// Package grpcservice provides a gRPC test service for k6 tests.
package grpcservice

import (
	"go.k6.io/k6/internal/lib/testutils/grpcservice"
	"google.golang.org/grpc"
)

// LoadFeatures loads test features from a JSON file.
func LoadFeatures(jsonFile string) []*Feature {
	return grpcservice.LoadFeatures(jsonFile)
}

// NewRouteGuideServer creates a new RouteGuide server with the given features.
func NewRouteGuideServer(features ...*Feature) RouteGuideServer {
	return grpcservice.NewRouteGuideServer(features...)
}

// RegisterRouteGuideServer registers the RouteGuide server with a gRPC server.
func RegisterRouteGuideServer(s grpc.ServiceRegistrar, srv RouteGuideServer) {
	grpcservice.RegisterRouteGuideServer(s, srv)
}

// Feature represents a geographical feature.
type Feature = grpcservice.Feature

// RouteGuideServer is the server API for RouteGuide service.
type RouteGuideServer = grpcservice.RouteGuideServer
