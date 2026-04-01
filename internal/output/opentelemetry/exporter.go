package opentelemetry

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/sdk/metric"
	"google.golang.org/grpc/credentials"
)

func getExporter(cfg Config) (metric.Exporter, error) {
	// at the point of writing this code
	// ctx isn't used at any point in the exporter
	// later on, it could be used for the connection timeout
	ctx := context.Background()

	tlsConfig, err := buildTLSConfig(
		cfg.TLSInsecureSkipVerify,
		cfg.TLSCertificate,
		cfg.TLSClientCertificate,
		cfg.TLSClientKey,
	)
	if err != nil {
		return nil, err
	}

	var headers map[string]string
	if cfg.Headers.Valid {
		headers, err = parseHeaders(cfg.Headers.String)
		if err != nil {
			return nil, fmt.Errorf("failed to parse headers: %w", err)
		}
	}

	// TODO: drop the merge when the deprecated exporter type is fully removed
	protocol := mergeExporterTypeAndProtocol(cfg)

	switch protocol {
	case grpcExporterProtocol:
		return buildGRPCExporter(ctx, cfg, tlsConfig, headers)
	case httpExporterProtocol:
		return buildHTTPExporter(ctx, cfg, tlsConfig, headers)
	default:
		return nil, errors.New("unsupported exporter protocol " + cfg.ExporterProtocol.String)
	}
}

func mergeExporterTypeAndProtocol(cfg Config) string {
	if cfg.ExporterProtocol.Valid {
		return cfg.ExporterProtocol.String
	}
	if cfg.ExporterType.Valid {
		switch cfg.ExporterType.String {
		case httpExporterType:
			return httpExporterProtocol
		case grpcExporterType:
			return grpcExporterProtocol
		default:
			return cfg.ExporterType.String
		}
	}
	// return the default value
	return cfg.ExporterProtocol.String
}

func buildHTTPExporter(
	ctx context.Context,
	cfg Config,
	tlsConfig *tls.Config,
	headers map[string]string,
) (metric.Exporter, error) {
	opts := []otlpmetrichttp.Option{
		otlpmetrichttp.WithEndpoint(cfg.HTTPExporterEndpoint.String),
		otlpmetrichttp.WithURLPath(cfg.HTTPExporterURLPath.String),
	}

	if cfg.HTTPExporterInsecure.Bool {
		opts = append(opts, otlpmetrichttp.WithInsecure())
	}

	if len(headers) > 0 {
		opts = append(opts, otlpmetrichttp.WithHeaders(headers))
	}

	if tlsConfig != nil {
		opts = append(opts, otlpmetrichttp.WithTLSClientConfig(tlsConfig))
	}

	return otlpmetrichttp.New(ctx, opts...)
}

func buildGRPCExporter(
	ctx context.Context,
	cfg Config,
	tlsConfig *tls.Config,
	headers map[string]string,
) (metric.Exporter, error) {
	opt := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(cfg.GRPCExporterEndpoint.String),
	}

	if cfg.GRPCExporterInsecure.Bool {
		opt = append(opt, otlpmetricgrpc.WithInsecure())
	}

	if len(headers) > 0 {
		opt = append(opt, otlpmetricgrpc.WithHeaders(headers))
	}

	if tlsConfig != nil {
		opt = append(opt, otlpmetricgrpc.WithTLSCredentials(credentials.NewTLS(tlsConfig)))
	}

	return otlpmetricgrpc.New(ctx, opt...)
}

func parseHeaders(raw string) (map[string]string, error) {
	headers := make(map[string]string)
	for header := range strings.SplitSeq(raw, ",") {
		rawKey, rawValue, ok := strings.Cut(header, "=")
		if !ok {
			return nil, fmt.Errorf("invalid header %q, expected format key=value", header)
		}

		key, err := url.PathUnescape(rawKey)
		if err != nil {
			return nil, fmt.Errorf("failed to unescape header key %q: %w", rawKey, err)
		}

		value, err := url.PathUnescape(rawValue)
		if err != nil {
			return nil, fmt.Errorf("failed to unescape header value %q: %w", rawValue, err)
		}

		headers[key] = value
	}

	return headers, nil
}
