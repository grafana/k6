// Package grpc implements a proxy that applies disruptions to gRPC requests
// This package is inspired by and extensively copies code from https://github.com/mwitkow/grpc-proxy
package grpc

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/agent/protocol"

	"google.golang.org/grpc"
)

// Disruption specifies disruptions in grpc requests
type Disruption struct {
	// Average delay introduced to requests
	AverageDelay time.Duration
	// Variation in the delay (with respect of the average delay)
	DelayVariation time.Duration
	// Fraction (in the range 0.0 to 1.0) of requests that will return an error
	ErrorRate float32
	// Status code to be returned by requests selected to return an error
	StatusCode uint32
	// Status message to be returned in requests selected to return an error
	StatusMessage string
	// List of grpc services to be excluded from disruptions
	Excluded []string
}

// Proxy defines the parameters used by the proxy for processing grpc requests and its execution state
type proxy struct {
	listener net.Listener
	srv      *grpc.Server
	cancel   func()
	metrics  *protocol.MetricMap
}

// NewProxy return a new Proxy
func NewProxy(listener net.Listener, upstreamAddress string, d Disruption) (protocol.Proxy, error) {
	if upstreamAddress == "" {
		return nil, fmt.Errorf("proxy's forwarding address must be provided")
	}

	if d.DelayVariation > d.AverageDelay {
		return nil, fmt.Errorf("variation must be less that average delay")
	}

	if d.ErrorRate < 0.0 || d.ErrorRate > 1.0 {
		return nil, fmt.Errorf("error rate must be in the range [0.0, 1.0]")
	}

	if d.ErrorRate > 0.0 && d.StatusCode == 0 {
		return nil, fmt.Errorf("status code cannot be 0 (OK)")
	}

	ctx, cancel := context.WithCancel(context.Background())
	conn, err := grpc.DialContext(
		ctx,
		upstreamAddress,
		grpc.WithInsecure(),
	)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("error dialing %s: %w", upstreamAddress, err)
	}

	metrics := protocol.NewMetricMap(
		protocol.MetricRequests,
		protocol.MetricRequestsExcluded,
		protocol.MetricRequestsDisrupted,
	)

	handler := NewHandler(d, conn, metrics)

	srv := grpc.NewServer(
		grpc.UnknownServiceHandler(handler),
	)

	return &proxy{
		listener: listener,
		srv:      srv,
		cancel:   cancel,
		metrics:  metrics,
	}, nil
}

// Start starts the execution of the proxy
func (p *proxy) Start() error {
	err := p.srv.Serve(p.listener)
	if err != nil {
		return fmt.Errorf("proxy terminated with error: %w", err)
	}

	return nil
}

// Stop stops the execution of the proxy
func (p *proxy) Stop() error {
	p.cancel()
	p.srv.GracefulStop()

	return nil
}

// Metrics returns runtime metrics for the proxy.
func (p *proxy) Metrics() map[string]uint {
	return p.metrics.Map()
}

// Force stops the proxy without waiting for connections to drain
// In grpc this action is a nop
func (p *proxy) Force() error {
	p.cancel()
	p.srv.Stop()

	return nil
}
