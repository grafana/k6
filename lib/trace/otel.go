package trace

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.20.0"
	"go.opentelemetry.io/otel/trace"
)

const serviceName = "k6"

// ErrInvalidProto indicates that the defined exporter protocol is not valid.
var ErrInvalidProto = errors.New("invalid protocol")

type (
	tracerProvShutdownFunc func(ctx context.Context) error
)

// TracerProvider provides methods for tracers initialization and shutdown of the
// processing pipeline.
type TracerProvider struct {
	trace.TracerProvider
	shutdown tracerProvShutdownFunc
}

type tracerProviderParams struct {
	proto    string
	endpoint string
	urlPath  string
	insecure bool
	headers  map[string]string
}

func defaultTracerProviderParams() tracerProviderParams {
	return tracerProviderParams{
		proto:    "grpc",
		endpoint: "127.0.0.1:4317",
		insecure: true,
		headers:  make(map[string]string),
	}
}

// NewTracerProvider creates a new tracer provider.
func NewTracerProvider(ctx context.Context, params tracerProviderParams) (*TracerProvider, error) {
	client, err := newClient(params)
	if err != nil {
		return nil, fmt.Errorf("creating TracerProvider exporter client: %w", err)
	}

	exporter, err := otlptrace.New(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("creating TracerProvider exporter: %w", err)
	}

	prov := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(newResource()),
	)

	// Set a noop TracerProvider globally so usage of tracing
	// instrumentation is restricted to our own implementation
	otel.SetTracerProvider(NewNoopTracerProvider())

	return &TracerProvider{
		TracerProvider: prov,
		shutdown:       prov.Shutdown,
	}, nil
}

func newResource() *resource.Resource {
	return resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(serviceName),
	)
}

func newClient(params tracerProviderParams) (otlptrace.Client, error) {
	switch params.proto {
	case "http":
		return newHTTPClient(params.endpoint, params.urlPath, params.insecure, params.headers), nil
	case "grpc":
		return newGRPCClient(params.endpoint, params.insecure, params.headers), nil
	default:
		return nil, ErrInvalidProto
	}
}

func newHTTPClient(endpoint, urlPath string, insecure bool, headers map[string]string) otlptrace.Client {
	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithURLPath(urlPath),
		otlptracehttp.WithHeaders(headers),
	}
	if insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}
	return otlptracehttp.NewClient(opts...)
}

func newGRPCClient(endpoint string, insecure bool, headers map[string]string) otlptrace.Client {
	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithHeaders(headers),
	}
	if insecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}
	return otlptracegrpc.NewClient(opts...)
}

// NewNoopTracerProvider creates a new noop TracerProvider.
func NewNoopTracerProvider() *TracerProvider {
	prov := trace.NewNoopTracerProvider()
	noopShutdown := func(context.Context) error { return nil }

	otel.SetTracerProvider(prov)

	return &TracerProvider{
		TracerProvider: prov,
		shutdown:       noopShutdown,
	}
}

// Shutdown shuts down the TracerProvider releasing any held computational resources.
// After Shutdown is called, all methods are no-ops.
func (tp *TracerProvider) Shutdown(ctx context.Context) error {
	return tp.shutdown(ctx)
}
