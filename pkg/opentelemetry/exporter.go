package opentelemetry

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/sdk/metric"
)

func getExporter(cfg Config) (metric.Exporter, error) {
	// at the point of writing this code
	// ctx isn't used at any point in the exporter
	// later on, it could be used for the connection timeout
	ctx := context.Background()
	exporterType := cfg.ExporterType.String

	if exporterType == grpcExporterType {
		return buildGRPCExporter(ctx, cfg)
	}

	if exporterType == httpExporterType {
		return buildHTTPExporter(ctx, cfg)
	}

	return nil, errors.New("unsupported exporter type " + exporterType + " specified")
}

func buildHTTPExporter(ctx context.Context, cfg Config) (metric.Exporter, error) {
	opts := []otlpmetrichttp.Option{
		otlpmetrichttp.WithEndpoint(cfg.HTTPExporterEndpoint.String),
		otlpmetrichttp.WithURLPath(cfg.HTTPExporterURLPath.String),
	}

	if cfg.HTTPExporterInsecure.Bool {
		opts = append(opts, otlpmetrichttp.WithInsecure())
	}

	return otlpmetrichttp.New(ctx, opts...)
}

func buildGRPCExporter(ctx context.Context, cfg Config) (metric.Exporter, error) {
	opt := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(cfg.GRPCExporterEndpoint.String),
	}

	// TODO: give priority to the TLS
	if cfg.GRPCExporterInsecure.Bool {
		opt = append(opt, otlpmetricgrpc.WithInsecure())
	}

	return otlpmetricgrpc.New(ctx, opt...)
}
