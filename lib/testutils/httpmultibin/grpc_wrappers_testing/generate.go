// Package grpc_wrappers_testing contains the test service that helps to test gRPC wrappers.
package grpc_wrappers_testing //nolint:revive,stylecheck

//go:generate protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative test.proto
