package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"

	pb "github.com/ilyazz/jobs/pkg/api/grpc/jobs/v1"
	"github.com/ilyazz/jobs/pkg/certloader"
	"github.com/ilyazz/jobs/pkg/server"
	"github.com/ilyazz/jobs/pkg/server/shim"
)

var address string
var config string
var mode string

var cmd string

var cgroup string
var uid int
var gid int

func init() {
	flag.StringVar(&address, "address", "localhost", "address")

	flag.StringVar(&config, "config", "jobserver.yaml", "")
	flag.StringVar(&mode, "mode", "", "")
	flag.StringVar(&cmd, "cmd", "", "")

	flag.StringVar(&cgroup, "cgroup", "", "")
	flag.IntVar(&uid, "uid", 0, "")
	flag.IntVar(&gid, "gid", 0, "")
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
