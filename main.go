package main

//go:generate mkdir -p internal/goldmane
//go:generate curl -sSL -o internal/goldmane/api.pb.go https://raw.githubusercontent.com/projectcalico/calico/v3.30.3/goldmane/proto/api.pb.go
//go:generate curl -sSL -o internal/goldmane/api_grpc.pb.go https://raw.githubusercontent.com/projectcalico/calico/v3.30.3/goldmane/proto/api_grpc.pb.go

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	goldmane "github.com/simwr872/goldmane-streamer/internal/goldmane"
)

func getEnv(k, def string) string {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	return v
}

func getLogWriter(logPath string) (*os.File, error) {
	if logPath == "-" {
		return os.Stdout, nil
	}

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func loadTLSCredentials(caPath, crtPath, keyPath string) (credentials.TransportCredentials, error) {
	caPEM, err := os.ReadFile(caPath)
	if err != nil {
		return nil, err
	}

	cert, err := tls.LoadX509KeyPair(crtPath, keyPath)
	if err != nil {
		return nil, err
	}

	certPool := x509.NewCertPool()
	if ok := certPool.AppendCertsFromPEM(caPEM); !ok {
		return nil, fmt.Errorf("bad CA")
	}

	return credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      certPool,
		MinVersion:   tls.VersionTLS12,
	}), nil
}

func main() {
	addr := getEnv("GOLDMANE_ADDR", "goldmane.calico-system.svc:7443")
	caPath := getEnv("GOLDMANE_CA", "/etc/goldmane/certs/tls.crt")
	crtPath := getEnv("GOLDMANE_CERT", "/etc/goldmane/certs/tls.crt")
	keyPath := getEnv("GOLDMANE_KEY", "/etc/goldmane/certs/tls.key")
	logPath := getEnv("LOG", "-")

	out, err := getLogWriter(logPath)
	if err != nil {
		log.Fatalf("failed to open log writer: %v", err)
	}
	if out != os.Stdout {
		defer out.Close()
	}

	creds, err := loadTLSCredentials(caPath, crtPath, keyPath)
	if err != nil {
		log.Fatalf("failed to load TLS credentials: %v", err)
	}

	client, err := grpc.NewClient(addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		log.Fatalf("failed to create grpc client: %v", err)
	}
	defer client.Close()

	flowsClient := goldmane.NewFlowsClient(client)

	ctx := context.Background()
	stream, err := flowsClient.Stream(ctx, &goldmane.FlowStreamRequest{})
	if err != nil {
		log.Fatalf("failed to open stream: %v", err)
	}

	fmt.Println("goldmane-streamer started...")

	for {
		res, err := stream.Recv()
		if err != nil {
			log.Fatalf("failed to stream recv: %v", err)
		}
		flow, err := json.Marshal(res.Flow)
		if err != nil {
			log.Fatalf("failed to marshal flow: %v", err)
		}
		_, err = fmt.Fprintln(out, string(flow))
		if err != nil {
			log.Fatalf("failed to write log: %v", err)
		}
	}
}
