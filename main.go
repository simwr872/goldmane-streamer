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

func mustEnv(k, def string) string {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	return v
}

func main() {
	addr := mustEnv("GOLDMANE_ADDR", "goldmane.calico-system.svc:7443")
	caPath := mustEnv("GOLDMANE_CA", "/etc/goldmane/certs/tls.crt")
	crtPath := mustEnv("GOLDMANE_CERT", "/etc/goldmane/certs/tls.crt")
	keyPath := mustEnv("GOLDMANE_KEY", "/etc/goldmane/certs/tls.key")

	caPEM, err := os.ReadFile(caPath)
	if err != nil {
		log.Fatalf("read CA: %v", err)
	}
	cert, err := tls.LoadX509KeyPair(crtPath, keyPath)
	if err != nil {
		log.Fatalf("load client cert/key: %v", err)
	}
	cp := x509.NewCertPool()
	if ok := cp.AppendCertsFromPEM(caPEM); !ok {
		log.Fatalf("bad CA")
	}
	creds := credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      cp,
		MinVersion:   tls.VersionTLS12,
	})

	client, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(creds),
	)
	if err != nil {
		log.Fatalf("failed to create grpc client: %v", err)
	}
	defer client.Close()

	flowsClient := goldmane.NewFlowsClient(client)

	ctx := context.Background()
	stream, err := flowsClient.Stream(ctx, &goldmane.FlowStreamRequest{})
	if err != nil {
		log.Fatalf("open stream: %v", err)
	}

	for {
		res, err := stream.Recv()
		if err != nil {
			log.Fatalf("stream recv: %v", err)
		}
		b, err := json.Marshal(res.Flow)
		if err != nil {
			log.Printf("marshal flow: %v", err)
			continue
		}
		fmt.Println(string(b))
	}
}
