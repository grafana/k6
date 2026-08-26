// Package protocol implements the agent that injects disruptors in protocols.
// The protocol disruptors run as a proxy. The agent redirects the traffic
// to the proxy using iptables.
package protocol

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/agent"
	"github.com/grafana/xk6-disruptor/pkg/runtime"
)

// ErrNoRequests is returned when a proxy supports MetricRequests and returns a value of 0 for it.
var ErrNoRequests = errors.New("disruptor did not receive any request")

// TrafficRedirector defines the interface for a traffic redirector
type TrafficRedirector interface {
	// Start initiates the redirection of traffic and resets existing connections
	Start() error
	// Stop restores the traffic to the original target and resets existing connections
	// to the redirection target
	Stop() error
}

// Proxy defines an interface for a proxy
type Proxy interface {
	Start() error
	Stop() error
	// Metrics returns a map of counter-type metrics. Implementations may return zero or more of the metrics defined
	// below, as well as any number of implementation-defined metrics. Callers can check if a metric exists in the map
	// returned by Metrics to distinguish a counter with a value of zero from an unsupported metric.
	Metrics() map[string]uint
	Force() error
}

const (
	// MetricRequests is the total number of requests received by the proxy.
	MetricRequests = "requests_total"
	// MetricRequestsExcluded is the total number of requests passed through due to exclusion rules.
	MetricRequestsExcluded = "requests_excluded"
	// MetricRequestsDisrupted is the total number requests that the proxy altered in any way.
	MetricRequestsDisrupted = "requests_disrupted"
)

// disruptor is an instance of a Disruptor that applies a disruption
// to a target
type disruptor struct {
	proxy      Proxy
	redirector TrafficRedirector
	executor   runtime.Executor
}

// NewDisruptor creates a new instance of a Disruptor that applies a disruptions to a target
// The configuration controls how the disruptor operates.
func NewDisruptor(
	executor runtime.Executor,
	proxy Proxy,
	redirector TrafficRedirector,
) (agent.Disruptor, error) {
	if proxy == nil {
		return nil, fmt.Errorf("proxy cannot be null")
	}

	return &disruptor{
		proxy:      proxy,
		executor:   executor,
		redirector: redirector,
	}, nil
}

// Apply applies the Disruption to the target system
func (d *disruptor) Apply(ctx context.Context, duration time.Duration) error {
	if duration < time.Second {
		return fmt.Errorf("duration must be at least one second")
	}

	wc := make(chan error)
	go func() {
		wc <- d.proxy.Start()
	}()

	// On termination, restore traffic and stop proxy
	defer func() {
		_ = d.proxy.Stop()
	}()

	if err := d.redirector.Start(); err != nil {
		return fmt.Errorf(" failed traffic redirection: %w", err)
	}

	defer func() {
		_ = d.redirector.Stop()
	}()

	// Wait for request duration, context cancellation or proxy server error
	for {
		select {
		case err := <-wc:
			if err != nil {
				return fmt.Errorf(" proxy ended with error: %w", err)
			}
		case <-time.After(duration):
			requests, hasMetric := d.proxy.Metrics()[MetricRequests]
			if hasMetric && requests == 0 {
				return ErrNoRequests
			}

			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// noop is a no-op traffic redirector
type noop struct{}

// NoopTrafficRedirector returns a dummy traffic redirector that has no effect
func NoopTrafficRedirector() TrafficRedirector {
	return &noop{}
}

func (n *noop) Start() error {
	return nil
}

func (n *noop) Stop() error {
	return nil
}
