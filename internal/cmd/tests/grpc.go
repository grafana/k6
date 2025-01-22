package tests

import (
	"net"
	"strings"
	"testing"

	"go.k6.io/k6/internal/lib/testutils/grpcservice"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// GRPC .
type GRPC struct {
	Addr       string
	ServerGRPC *grpc.Server
	Replacer   *strings.Replacer
}

// NewGRPC .
func NewGRPC(t testing.TB) *GRPC {
	grpcServer := grpc.NewServer()

	addr := getFreeBindAddr(t)

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	features := grpcservice.LoadFeatures("")
	grpcservice.RegisterRouteGuideServer(grpcServer, grpcservice.NewRouteGuideServer(features...))
	grpcservice.RegisterFeatureExplorerServer(grpcServer, grpcservice.NewFeatureExplorerServer(features...))
	reflection.Register(grpcServer)

	go func() {
		_ = grpcServer.Serve(lis)
	}()

	t.Cleanup(func() {
		grpcServer.Stop()
	})

	return &GRPC{
		Addr:       addr,
		ServerGRPC: grpcServer,
		Replacer: strings.NewReplacer(
			"GRPCBIN_ADDR", addr,
		),
	}
}
