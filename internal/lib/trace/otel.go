package trace

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.20.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"go.k6.io/k6/internal/lib/strvals"
)

const serviceName = "k6"

var (
	// ErrInvalidTracesOutput indicates that the defined traces output is not valid.
	ErrInvalidTracesOutput = errors.New("invalid traces output")
	// ErrInvalidProto indicates that the defined exporter protocol is not valid.
	ErrInvalidProto = errors.New("invalid protocol")
	// ErrInvalidURLScheme indicates that the defined exporter URL scheme is not valid.
	ErrInvalidURLScheme = errors.New("invalid URL scheme")
	// ErrInvalidGRPCWithURLPath indicates that an exporter using gRPC protocol does not support URL path.
	ErrInvalidGRPCWithURLPath = errors.New("grpc protocol does not support URL path")
)

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
	prov := noop.NewTracerProvider()
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

// TracerProviderFromConfigLine initializes a new TracerProvider based on the configuration
// specified through input line.
//
// Supported format is: otel[=<endpoint>:<port>,<other opts>]
// Where endpoint and port default to: 127.0.0.1:4317
// And other opts accept:
//   - proto: http or grpc (default).
//   - header.<header_name>
//
// Example: otel=127.0.0.1:4318/v1/traces,proto=http,header.Authorization=token ***
func TracerProviderFromConfigLine(ctx context.Context, line string) (*TracerProvider, error) {
	params, err := tracerProviderParamsFromConfigLine(line)
	if err != nil {
		return nil, err
	}

	return NewTracerProvider(ctx, params)
}

func tracerProviderParamsFromConfigLine(line string) (tracerProviderParams, error) {
	params := defaultTracerProviderParams()

	if line == "otel" {
		// Use default params
		return params, nil
	}

	traceOutput, _, _ := strings.Cut(line, "=")
	if traceOutput != "otel" {
		return params, fmt.Errorf("%w %q", ErrInvalidTracesOutput, traceOutput)
	}

	tokens, err := strvals.Parse(line)
	if err != nil {
		return params, fmt.Errorf("error while parsing otel configuration %w", err)
	}

	for _, token := range tokens {
		key := token.Key
		value := token.Value

		var err error
		switch key {
		case "otel":
			err = params.parseURL(value)
			if err != nil {
				return params, fmt.Errorf("couldn't parse the otel URL: %w", err)
			}
		case "proto":
			err = params.parseProto(value)
			if err != nil {
				return params, fmt.Errorf("couldn't parse the otel proto: %w", err)
			}
		default:
			if strings.HasPrefix(key, "header.") {
				params.parseHeader(key, value)
				continue
			}
			return params, fmt.Errorf("unknown otel config key %s", key)
		}
	}

	if params.proto == "grpc" && params.urlPath != "" {
		return params, ErrInvalidGRPCWithURLPath
	}

	return params, nil
}

func (p *tracerProviderParams) parseURL(s string) error {
	u, err := url.Parse(s)
	if err != nil {
		return err
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%w: %q", ErrInvalidURLScheme, u.Scheme)
	}

	p.proto = "http"
	p.endpoint = u.Host
	p.urlPath = u.Path
	p.insecure = u.Scheme == "http"

	return nil
}

func (p *tracerProviderParams) parseProto(proto string) error {
	if proto != "http" && proto != "grpc" {
		return fmt.Errorf("%w: %q", ErrInvalidProto, proto)
	}

	p.proto = proto

	return nil
}

func (p *tracerProviderParams) parseHeader(header, value string) {
	headerName := strings.TrimPrefix(header, "header.")
	p.headers[headerName] = value
}
