package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"time"

	pb "github.com/ilyazz/jobs/pkg/api/grpc/jobs/v1"
	"github.com/ilyazz/jobs/pkg/certloader"
	"github.com/ilyazz/jobs/pkg/server"
	"github.com/ilyazz/jobs/pkg/server/shim"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

var address string
var config string
var mode string

var cmd string

var cgroup string
var uid int
var gid int

var pidfile string

func init() {
	flag.StringVar(&address, "address", "localhost", "address")

	flag.StringVar(&config, "config", "jobserver.yaml", "")
	flag.StringVar(&mode, "mode", "", "")
	flag.StringVar(&cmd, "cmd", "", "")

	flag.StringVar(&cgroup, "cgroup", "", "")
	flag.IntVar(&uid, "uid", 0, "")
	flag.IntVar(&gid, "gid", 0, "")

	flag.StringVar(&pidfile, "pid", "", "")
}

func main() {

	if os.Getuid() != 0 {
		fmt.Fprintf(os.Stderr, "Please run as root\n")
		os.Exit(1)
	}

	flag.Parse()

	if mode == "shim" {
		shim.Main(cmd, flag.Args(), cgroup, uid, gid)
		return
	}

	if pidfile != "" {
		_ = os.WriteFile(pidfile, []byte(fmt.Sprintf("%d", os.Getpid())), 0666)
	}

	log.Logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).With().Timestamp().Logger()

	//TODO check shim flags are not set

	cfg, err := server.FindConfig(config)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to start the server: %v\n", err)
		os.Exit(1)
	}

	js, err := server.New(cfg)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to start the server: %v\n", err)
		os.Exit(1)
	}

	cl := certloader.Loader{
		KeyPath:  cfg.TLS.KeyPath,
		CertPath: cfg.TLS.CertPath,
		Reload:   time.Duration(cfg.TLS.ReloadSec) * time.Second,
	}

	err = cl.Start()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to start cert loader: %v", err)
		os.Exit(1)
	}

	cas := x509.NewCertPool()
	ca, err := os.ReadFile(cfg.TLS.CAPath)
	if err != nil {
		log.Warn().Err(err).Msg("failed to load CA cert")
		os.Exit(1)
	}
	cas.AppendCertsFromPEM(ca)

	tlsCfg := &tls.Config{
		ClientCAs:  cas,
		MinVersion: tls.VersionTLS13,
		ClientAuth: tls.RequireAndVerifyClientCert,
		GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
			return cl.Cert(), nil
		},
	}

	opts := []grpc.ServerOption{
		grpc.Creds(credentials.NewTLS(tlsCfg)),
		grpc.UnaryInterceptor(interceptor),
		grpc.StreamInterceptor(streamInterceptor),
	}

	srv := grpc.NewServer(opts...)

	reflection.Register(srv)

	pb.RegisterJobServiceServer(srv, js)

	l, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		log.Error().Err(err).Msg("failed to start server")
		os.Exit(1)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	go func() {
		<-sigCh
		log.Info().Msg("stopping the server ...")
		js.StopServer()
		log.Info().Msg("server stopped")
		os.Exit(0)
	}()

	log.Info().Msgf("listening on %v", cfg.Address)
	err = srv.Serve(l)
	if err != nil {
		log.Error().Err(err).Msg("failed to serve")
		os.Exit(1)
	}
}

// clientID extracts a client ID from GRPC context
func clientID(ctx context.Context) (string, error) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return "", fmt.Errorf("no client ID")
	}
	tlsInfo := p.AuthInfo.(credentials.TLSInfo)
	if (len(tlsInfo.State.VerifiedChains) < 1) ||
		(len(tlsInfo.State.VerifiedChains[0]) < 1) ||
		(len(tlsInfo.State.VerifiedChains[0][0].DNSNames) < 1) {
		return "", fmt.Errorf("no ID provided")
	}

	return tlsInfo.State.VerifiedChains[0][0].DNSNames[0], nil
}

// wrapper is a simple stream wrapper for stream methods
type wrapper struct {
	grpc.ServerStream
	ctx context.Context
}

// Context returns the wrapped context
func (w wrapper) Context() context.Context {
	return w.ctx
}

// streamInterceptor provides timing, panic recovery, and adds client ID to the context
func streamInterceptor(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Warn().Msgf("panic recovered: %v", r)
			err = status.Error(codes.Internal, "internal error")
		}
	}()

	t0 := time.Now()

	cid, err := clientID(stream.Context())
	if err != nil {
		return status.Error(codes.Unauthenticated, "no DN provided")
	}

	wctx := &wrapper{
		ServerStream: stream,
		ctx:          server.StoreAuthID(stream.Context(), cid),
	}

	lg := log.With().Str("client", cid).Logger()

	defer func() {
		lg.Info().Msgf("Method %v took %v", info.FullMethod, time.Since(t0))
	}()

	lg.Info().Msgf("Calling %v", info.FullMethod)

	return handler(srv, wctx)
}

// interceptor provides timing, panic recovery, and adds client ID to the context
func interceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Warn().Msgf("panic recovered: %v", r)
			err = status.Error(codes.Internal, "internal error")
			resp = nil
		}
	}()

	t0 := time.Now()

	cid, err := clientID(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "no DN provided")
	}

	ctx = server.StoreAuthID(ctx, cid)

	lg := log.With().Str("client", cid).Logger()

	defer func() {
		lg.Info().Msgf("Method %v took %v", info.FullMethod, time.Since(t0))
	}()

	log.Info().Str("client", cid).Msgf("Calling %v", info.FullMethod)

	return handler(ctx, req)
}
