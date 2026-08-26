package ping

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// defaultPingServer is the canonical implementation of a TestServiceServer.
type defaultPingServer struct {
	UnsafePingServiceServer
}

// NewPingServer returns an instance of the default PingServiceServer
func NewPingServer() PingServiceServer {
	return &defaultPingServer{}
}

func (s *defaultPingServer) Ping(ctx context.Context, request *PingRequest) (*PingResponse, error) {
	if err := s.sendHeader(ctx, request.Headers); err != nil {
		return nil, err
	}
	if err := s.setTrailer(ctx, request.Trailers); err != nil {
		return nil, err
	}

	if request.Error != int32(codes.OK) {
		return nil, status.Error(codes.Code(request.Error), request.Message) //nolint:gosec // This is a test service
	}

	return &PingResponse{Message: request.Message}, nil
}

func (s *defaultPingServer) sendHeader(ctx context.Context, headers map[string]string) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		md = metadata.New(nil)
	}

	for header, value := range headers {
		valueList := strings.Split(value, ",")
		md.Append(header, valueList...)
	}

	if err := grpc.SendHeader(ctx, md); err != nil {
		return fmt.Errorf("error setting header: %w", err)
	}
	return nil
}

func (s *defaultPingServer) setTrailer(ctx context.Context, trailers map[string]string) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		md = metadata.New(nil)
	}

	for trailer, value := range trailers {
		valueList := strings.Split(value, ",")
		md.Append(trailer, valueList...)
	}

	if err := grpc.SetTrailer(ctx, md); err != nil {
		return fmt.Errorf("error setting trailer: %w", err)
	}

	return nil
}
