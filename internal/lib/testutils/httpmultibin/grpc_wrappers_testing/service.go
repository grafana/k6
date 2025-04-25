// Package grpc_wrappers_testing provides a test service that could be used for the testing gRPC wrappers
package grpc_wrappers_testing //nolint:revive // we want to be consistent with the other packages

import (
	context "context"

	_struct "github.com/golang/protobuf/ptypes/struct"
	wrappers "github.com/golang/protobuf/ptypes/wrappers"
	grpc "google.golang.org/grpc"
)

//go:generate protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative test.proto

// Register registers a test service that could be used for the testing gRPC wrappers
func Register(r grpc.ServiceRegistrar) *Service {
	s := &Service{}

	RegisterServiceServer(r, s)

	return s
}

// Service is the test service for different grpc values and how they are handled
type Service struct {
	UnimplementedServiceServer

	TestStringImplementation  func(context.Context, *wrappers.StringValue) (*wrappers.StringValue, error)
	TestIntegerImplementation func(context.Context, *wrappers.Int64Value) (*wrappers.Int64Value, error)
	TestBooleanImplementation func(context.Context, *wrappers.BoolValue) (*wrappers.BoolValue, error)
	TestDoubleImplementation  func(context.Context, *wrappers.DoubleValue) (*wrappers.DoubleValue, error)
	TestValueImplementation   func(context.Context, *_struct.Value) (*_struct.Value, error)
	TestStreamImplementation  func(Service_TestStreamServer) error
}

// TestString is getting and returning a string value
func (s *Service) TestString(ctx context.Context, in *wrappers.StringValue) (*wrappers.StringValue, error) {
	if s.TestStringImplementation != nil {
		return s.TestStringImplementation(ctx, in)
	}

	return s.UnimplementedServiceServer.TestString(ctx, in)
}

// TestInteger is getting and returning a integer value
func (s *Service) TestInteger(ctx context.Context, in *wrappers.Int64Value) (*wrappers.Int64Value, error) {
	if s.TestIntegerImplementation != nil {
		return s.TestIntegerImplementation(ctx, in)
	}

	return s.UnimplementedServiceServer.TestInteger(ctx, in)
}

// TestBoolean is getting and returning a boolean value
func (s *Service) TestBoolean(ctx context.Context, in *wrappers.BoolValue) (*wrappers.BoolValue, error) {
	if s.TestBooleanImplementation != nil {
		return s.TestBooleanImplementation(ctx, in)
	}

	return s.UnimplementedServiceServer.TestBoolean(ctx, in)
}

// TestDouble is getting and returning a double value
func (s *Service) TestDouble(ctx context.Context, in *wrappers.DoubleValue) (*wrappers.DoubleValue, error) {
	if s.TestDoubleImplementation != nil {
		return s.TestDoubleImplementation(ctx, in)
	}

	return s.UnimplementedServiceServer.TestDouble(ctx, in)
}

// TestValue is getting and returning a generic value
func (s *Service) TestValue(ctx context.Context, in *_struct.Value) (*_struct.Value, error) {
	if s.TestValueImplementation != nil {
		return s.TestValueImplementation(ctx, in)
	}

	return s.UnimplementedServiceServer.TestValue(ctx, in)
}

// TestStream is testing a stream of values
func (s *Service) TestStream(stream Service_TestStreamServer) error {
	if s.TestStreamImplementation != nil {
		return s.TestStreamImplementation(stream)
	}

	return s.UnimplementedServiceServer.TestStream(stream)
}
