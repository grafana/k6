package tests

import (
	"net"
	"strings"
	"testing"

	"go.k6.io/k6/v2/internal/lib/testutils/grpcservice"

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

	lis, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", addr)
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	features := grpcservice.LoadFeatures("")
	routeGuide := grpcservice.NewRouteGuideServer(features...)
	routeGuide.Logf = t.Logf
	grpcservice.RegisterRouteGuideServer(grpcServer, routeGuide)
	featureExplorer := grpcservice.NewFeatureExplorerServer(features...)
	featureExplorer.Logf = t.Logf
	grpcservice.RegisterFeatureExplorerServer(grpcServer, featureExplorer)
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
