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

	if cfg.ExporterType == grpcExporterType {
		return buildGRPCExporter(ctx, cfg)
	}

	if cfg.ExporterType == httpExporterType {
		return buildHTTPExporter(ctx, cfg)
	}

	return nil, errors.New("unsupported exporter type " + cfg.ExporterType + " specified")
}

func buildHTTPExporter(ctx context.Context, cfg Config) (metric.Exporter, error) {
	opts := []otlpmetrichttp.Option{
		otlpmetrichttp.WithEndpoint(cfg.HTTPExporterEndpoint),
		otlpmetrichttp.WithURLPath(cfg.HTTPExporterURLPath),
	}

	if cfg.HTTPExporterInsecure.Bool {
		opts = append(opts, otlpmetrichttp.WithInsecure())
	}

	return otlpmetrichttp.New(ctx, opts...)
}

func buildGRPCExporter(ctx context.Context, cfg Config) (metric.Exporter, error) {
	opt := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(cfg.GRPCExporterEndpoint),
	}

	// TODO: give priority to the TLS
	if cfg.GRPCExporterInsecure.Bool {
		opt = append(opt, otlpmetricgrpc.WithInsecure())
	}

	return otlpmetricgrpc.New(ctx, opt...)
}
