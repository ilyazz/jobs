package client

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "github.com/ilyazz/jobs/pkg/api/grpc/jobs/v1"
)

// New creates a new secure Job Server connection, using cert/ca data from the config
func New(cfg *Config) (pb.JobServiceClient, error) {

	if cfg.CAPath == "" {
		return nil, fmt.Errorf("CA cert required")
	}
	if cfg.CertPath == "" {
		return nil, fmt.Errorf("TLS cert required")
	}
	if cfg.KeyPath == "" {
		return nil, fmt.Errorf("TLS cert key required")
	}
	if cfg.Server == "" {
		return nil, fmt.Errorf("server endpoint required")
	}

	fmt.Printf("Using %q/%q/%q/%q\n", cfg.Server, cfg.CAPath, cfg.CertPath, cfg.KeyPath)

	creds, err := loadTLSCreds(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %w", err)
	}

	cl, err := grpc.Dial(cfg.Server,
		grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %w", err)
	}

	return pb.NewJobServiceClient(cl), nil
}

// loadTLSCreds
func loadTLSCreds(cfg *Config) (credentials.TransportCredentials, error) {
	ca, err := os.ReadFile(cfg.CAPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA cert: %w", err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(ca) {
		return nil, fmt.Errorf("failed to add CA cert")
	}

	cert, err := tls.LoadX509KeyPair(cfg.CertPath, cfg.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS cert: %w", err)
	}

	tlsCfg := &tls.Config{
		RootCAs:            pool,
		InsecureSkipVerify: false,
		Certificates:       []tls.Certificate{cert},
		MinVersion:         tls.VersionTLS13,
	}

	return credentials.NewTLS(tlsCfg), nil
}
