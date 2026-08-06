// Command testserver is a standalone, plaintext gRPC server used by the gRPC
// end-to-end tests (see internal/e2e/grpc). It is intentionally a separate
// program so that it runs in its own OS process: the process-global protobuf
// registries it populates (by linking the generated grpcservice types) are NOT
// shared with the k6 process under test. This is what lets the e2e tests verify
// that k6 loads proto definitions at runtime purely from the .proto file (or
// server reflection), rather than piggybacking on the server's registrations.
//
// See https://github.com/grafana/k6/issues/3552 for the motivation.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"go.k6.io/k6/v2/internal/lib/testutils/grpcservice"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:0", "address to listen on (use :0 for a random free port)")
	flag.Parse()

	lis, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", *addr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	server := grpc.NewServer()
	features := grpcservice.LoadFeatures("")
	grpcservice.RegisterRouteGuideServer(server, grpcservice.NewRouteGuideServer(features...))
	grpcservice.RegisterFeatureExplorerServer(server, grpcservice.NewFeatureExplorerServer(features...))

	healthcheck := health.NewServer()
	healthpb.RegisterHealthServer(server, healthcheck)
	healthcheck.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	reflection.Register(server)

	// Print the resolved address on a dedicated line so the parent test can
	// discover the port when listening on :0. Flush happens on newline.
	fmt.Printf("LISTENING %s\n", lis.Addr().String())

	if err := server.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
