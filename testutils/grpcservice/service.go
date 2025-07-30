// Package grpcservice provides a thin wrapper around the internal gRPC service for external use.
package grpcservice

import "go.k6.io/k6/internal/lib/testutils/grpcservice"

// Re-export types and functions from internal package
type Feature = grpcservice.Feature
type RouteGuideServer = grpcservice.RouteGuideServer

var (
	LoadFeatures           = grpcservice.LoadFeatures
	NewRouteGuideServer    = grpcservice.NewRouteGuideServer
	RegisterRouteGuideServer = grpcservice.RegisterRouteGuideServer
)
