package opentelemetry

import (
	"context"
	"crypto/tls"
	"errors"

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

	exporterType := cfg.ExporterType.String

	if exporterType == grpcExporterType {
		return buildGRPCExporter(ctx, cfg, tlsConfig)
	}

	if exporterType == httpExporterType {
		return buildHTTPExporter(ctx, cfg, tlsConfig)
	}

	return nil, errors.New("unsupported exporter type " + exporterType + " specified")
}

func buildHTTPExporter(ctx context.Context, cfg Config, tlsConfig *tls.Config) (metric.Exporter, error) {
	opts := []otlpmetrichttp.Option{
		otlpmetrichttp.WithEndpoint(cfg.HTTPExporterEndpoint.String),
		otlpmetrichttp.WithURLPath(cfg.HTTPExporterURLPath.String),
	}

	if cfg.HTTPExporterInsecure.Bool {
		opts = append(opts, otlpmetrichttp.WithInsecure())
	}

	if tlsConfig != nil {
		opts = append(opts, otlpmetrichttp.WithTLSClientConfig(tlsConfig))
	}

	return otlpmetrichttp.New(ctx, opts...)
}

func buildGRPCExporter(ctx context.Context, cfg Config, tlsConfig *tls.Config) (metric.Exporter, error) {
	opt := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(cfg.GRPCExporterEndpoint.String),
	}

	if cfg.GRPCExporterInsecure.Bool {
		opt = append(opt, otlpmetricgrpc.WithInsecure())
	}

	if tlsConfig != nil {
		opt = append(opt, otlpmetricgrpc.WithTLSCredentials(credentials.NewTLS(tlsConfig)))
	}

	return otlpmetricgrpc.New(ctx, opt...)
}
