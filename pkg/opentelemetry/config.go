package opentelemetry

import (
	"errors"
	"fmt"
	"time"

	k6Const "go.k6.io/k6/lib/consts"
	"go.k6.io/k6/output"
	"gopkg.in/guregu/null.v3"
)

const (
	// grpcExporterType GRPC exporter type
	grpcExporterType = "grpc"
	// httpExporterType HTTP exporter type
	httpExporterType = "http"
)

// Config is the config for the template collector
type Config struct {
	// ServiceName is the name of the service to use for the metrics
	// export, if not set it will use "k6"
	ServiceName string
	// ServiceVersion is the version of the service to use for the metrics
	// export, if not set it will use k6's library version
	ServiceVersion string
	// MetricPrefix is the prefix to use for the metrics
	MetricPrefix string
	// FlushInterval is the interval at which to flush metrics from the k6
	FlushInterval time.Duration

	// ExporterType sets the type of OpenTelemetry Exporter to use
	ExporterType string
	// ExportInterval configures the intervening time between metrics exports
	ExportInterval time.Duration

	// HTTPExporterInsecure disables client transport security for the Exporter's HTTP
	// connection.
	HTTPExporterInsecure null.Bool
	// HTTPExporterEndpoint sets the target endpoint the OpenTelemetry Exporter
	// will connect to.
	HTTPExporterEndpoint string
	// HTTPExporterURLPath sets the target URL path the OpenTelemetry Exporter
	HTTPExporterURLPath string

	// GRPCExporterEndpoint sets the target endpoint the OpenTelemetry Exporter
	// will connect to.
	GRPCExporterEndpoint string
	// GRPCExporterInsecure disables client transport security for the Exporter's gRPC
	// connection.
	GRPCExporterInsecure null.Bool
}

// NewConfig creates and validates a new config
func NewConfig(p output.Params) (Config, error) {
	cfg := Config{
		ServiceName:    "k6",
		ServiceVersion: k6Const.Version,
		MetricPrefix:   "",
		ExporterType:   grpcExporterType,

		HTTPExporterInsecure: null.BoolFrom(false),
		HTTPExporterEndpoint: "localhost:4318",
		HTTPExporterURLPath:  "/v1/metrics",

		GRPCExporterInsecure: null.BoolFrom(false),
		GRPCExporterEndpoint: "localhost:4317",

		ExportInterval: 1 * time.Second,
		FlushInterval:  1 * time.Second,
	}

	var err error
	for k, v := range p.Environment {
		switch k {
		case "K6_OTEL_SERVICE_NAME":
			cfg.ServiceName = v
		case "K6_OTEL_SERVICE_VERSION":
			cfg.ServiceVersion = v
		case "K6_OTEL_METRIC_PREFIX":
			cfg.MetricPrefix = v
		case "K6_OTEL_EXPORT_INTERVAL":
			cfg.ExportInterval, err = time.ParseDuration(v)
			if err != nil {
				return cfg, fmt.Errorf("error parsing environment variable 'K6_OTEL_EXPORT_INTERVAL': %w", err)
			}
		case "K6_OTEL_FLUSH_INTERVAL":
			cfg.FlushInterval, err = time.ParseDuration(v)
			if err != nil {
				return cfg, fmt.Errorf("error parsing environment variable 'K6_OTEL_FLUSH_INTERVAL': %w", err)
			}
		case "K6_OTEL_EXPORTER_TYPE":
			cfg.ExporterType = v
		case "K6_OTEL_GRPC_EXPORTER_ENDPOINT":
			cfg.GRPCExporterEndpoint = v
		case "K6_OTEL_HTTP_EXPORTER_ENDPOINT":
			cfg.HTTPExporterEndpoint = v
		case "K6_OTEL_HTTP_EXPORTER_URL_PATH":
			cfg.HTTPExporterURLPath = v
		case "K6_OTEL_HTTP_EXPORTER_INSECURE":
			cfg.HTTPExporterInsecure, err = parseBool(k, v)
			if err != nil {
				return cfg, err
			}
		case "K6_OTEL_GRPC_EXPORTER_INSECURE":
			cfg.GRPCExporterInsecure, err = parseBool(k, v)
			if err != nil {
				return cfg, err
			}
		}
	}

	// TDOO: consolidated config

	if err = cfg.Validate(); err != nil {
		return cfg, fmt.Errorf("error validating OpenTelemetry output config: %w", err)
	}

	return cfg, nil
}

func parseBool(k, v string) (null.Bool, error) {
	bv := null.NewBool(false, false)

	err := bv.UnmarshalText([]byte(v))
	if err != nil {
		return bv, fmt.Errorf("error parsing %q environment variable: %w", k, err)
	}

	return bv, nil
}

// Validate validates the config
func (c Config) Validate() error {
	if c.ServiceName == "" {
		return errors.New("providing service name is required")
	}

	if c.ServiceVersion == "" {
		return errors.New("providing service version is required")
	}

	if c.ExporterType != grpcExporterType && c.ExporterType != httpExporterType {
		return fmt.Errorf(
			"unsupported exporter type %q, currently only %q and %q are supported",
			c.ExporterType,
			grpcExporterType,
			httpExporterType,
		)
	}

	if c.ExporterType == grpcExporterType {
		if c.GRPCExporterEndpoint == "" {
			return errors.New("gRPC exporter endpoint is required")
		}
	}

	if c.ExporterType == httpExporterType {
		if c.HTTPExporterEndpoint == "" {
			return errors.New("HTTP exporter endpoint is required")
		}
	}

	return nil
}

// String returns a string representation of the config
func (c Config) String() string {
	var endpoint string
	exporter := c.ExporterType

	if c.ExporterType == httpExporterType {
		endpoint = "http"
		if !c.HTTPExporterInsecure.Bool {
			endpoint += "s"
		}

		endpoint += "://" + c.HTTPExporterEndpoint + c.HTTPExporterURLPath
	} else {
		endpoint = c.GRPCExporterEndpoint

		if c.GRPCExporterInsecure.Bool {
			exporter += " (insecure)"
		}
	}

	return fmt.Sprintf("%s, %s", exporter, endpoint)
}
