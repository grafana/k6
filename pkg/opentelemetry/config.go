package opentelemetry

import (
	"errors"
	"fmt"
	"time"

	"go.k6.io/k6/output"
)

const (
	grpcReceiverType = "grpc"
)

// Config is the config for the template collector
type Config struct {
	// MetricPrefix is the prefix to use for the metrics
	MetricPrefix string
	// ReceiverType is the type of the receiver to use
	ReceiverType string
	// GRPCReceiverEndpoint is the endpoint of the gRPC receiver
	GRPCReceiverEndpoint string
	// PushInterval is the interval at which to push metrics to the receiver
	PushInterval time.Duration
	// FlushInterval is the interval at which to flush metrics from the k6
	FlushInterval time.Duration
}

// NewConfig creates and validates a new config
func NewConfig(p output.Params) (Config, error) {
	cfg := Config{
		MetricPrefix:         "",
		ReceiverType:         grpcReceiverType,
		GRPCReceiverEndpoint: "localhost:4317",
		PushInterval:         1 * time.Second,
		FlushInterval:        1 * time.Second,
	}

	var err error
	for k, v := range p.Environment {
		switch k {
		case "K6_OTEL_PUSH_INTERVAL":
			cfg.PushInterval, err = time.ParseDuration(v)
			if err != nil {
				return cfg, fmt.Errorf("error parsing environment variable 'K6_OTEL_PUSH_INTERVAL': %w", err)
			}
		case "K6_OTEL_METRIC_PREFIX":
			cfg.MetricPrefix = v
		case "K6_OTEL_FLUSH_INTERVAL":
			cfg.FlushInterval, err = time.ParseDuration(v)
			if err != nil {
				return cfg, fmt.Errorf("error parsing environment variable 'K6_OTEL_FLUSH_INTERVAL': %w", err)
			}
		case "K6_OTEL_RECEIVER_TYPE":
			cfg.ReceiverType = v
		case "K6_OTEL_GRPC_RECEIVER_ENDPOINT":
			cfg.GRPCReceiverEndpoint = v
		}
	}

	// TDOO: consolidated config

	if err = cfg.Validate(); err != nil {
		return cfg, fmt.Errorf("error validating config: %w", err)
	}

	return cfg, nil
}

// Validate validates the config
func (c Config) Validate() error {
	if c.ReceiverType != grpcReceiverType {
		return fmt.Errorf("unsupported receiver type %q, currently only %q supported", c.ReceiverType, grpcReceiverType)
	}

	if c.GRPCReceiverEndpoint == "" {
		return errors.New("gRPC receiver endpoint is required")
	}

	return nil
}

// String returns a string representation of the config
func (c Config) String() string {
	return fmt.Sprintf("%s, %s", c.ReceiverType, c.GRPCReceiverEndpoint)
}
