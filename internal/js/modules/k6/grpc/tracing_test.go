package grpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
	grpcCodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"

	"go.k6.io/k6/v2/internal/lib/netext/grpcext"
	k6trace "go.k6.io/k6/v2/internal/lib/trace"
	"go.k6.io/k6/v2/js/modulestest"
	"go.k6.io/k6/v2/lib"
	moduletrace "go.k6.io/k6/v2/lib/trace"
	"go.k6.io/k6/v2/metrics"
)

type grpcTraceExporter struct {
	spans []sdktrace.ReadOnlySpan
}

func (e *grpcTraceExporter) ExportSpans(_ context.Context, spans []sdktrace.ReadOnlySpan) error {
	e.spans = append(e.spans, spans...)
	return nil
}

func (*grpcTraceExporter) Shutdown(context.Context) error { return nil }

func TestModuleInstanceTracingDefault(t *testing.T) {
	t.Parallel()

	t.Run("enabled via --tracing", func(t *testing.T) {
		t.Parallel()
		runtime := modulestest.NewRuntime(t)
		set, err := moduletrace.ParseSet("grpc")
		require.NoError(t, err)
		runtime.VU.InitEnvField.Tracing = set

		module := New().NewModuleInstance(runtime.VU).(*ModuleInstance)
		require.True(t, module.tracingEnabled)
	})

	t.Run("disabled by default", func(t *testing.T) {
		t.Parallel()
		runtime := modulestest.NewRuntime(t)
		module := New().NewModuleInstance(runtime.VU).(*ModuleInstance)
		require.False(t, module.tracingEnabled)
	})
}

func TestGRPCTrace(t *testing.T) {
	t.Parallel()

	exporter := &grpcTraceExporter{}
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	parentCtx, parent := provider.Tracer("test").Start(t.Context(), "iteration")
	state := &lib.State{TracerProvider: provider}
	md := metadata.New(nil)
	tagsAndMeta := &metrics.TagsAndMeta{}

	_, span := startGRPCTrace(
		parentCtx, state, "localhost:6565", "/example.Service/Call", md, tagsAndMeta,
	)
	require.NotEmpty(t, md.Get("traceparent"))
	require.Equal(t, parent.SpanContext().TraceID().String(), tagsAndMeta.Metadata["trace_id"])
	endGRPCTrace(span, &grpcext.InvokeResponse{Status: grpcCodes.Unavailable}, nil)

	require.Len(t, exporter.spans, 1)
	got := exporter.spans[0]
	require.Equal(t, "example.Service/Call", got.Name())
	require.Equal(t, oteltrace.SpanKindClient, got.SpanKind())
	require.Equal(t, parent.SpanContext().SpanID(), got.Parent().SpanID())
	require.Equal(t, "grpc", grpcTraceAttribute(got, "rpc.system.name"))
	require.Equal(t, "example.Service", grpcTraceAttribute(got, "rpc.service"))
	require.Equal(t, "Call", grpcTraceAttribute(got, "rpc.method"))
	require.Equal(t, int64(grpcCodes.Unavailable), grpcTraceAttribute(got, "rpc.grpc.status_code"))
	require.Equal(t, codes.Error, got.Status().Code)
	require.Equal(t, "Unavailable", got.Status().Description)
	parent.End()
}

func TestGRPCTraceUsesConfiguredPropagator(t *testing.T) {
	t.Parallel()

	provider := sdktrace.NewTracerProvider()
	parentCtx, parent := provider.Tracer("test").Start(t.Context(), "iteration")
	propagator, err := k6trace.PropagatorFromConfig("jaeger")
	require.NoError(t, err)
	state := &lib.State{TracerProvider: provider, TracePropagator: propagator}
	md := metadata.New(nil)

	_, span := startGRPCTrace(
		parentCtx, state, "localhost:6565", "/example.Service/Call", md, &metrics.TagsAndMeta{},
	)
	require.Len(t, md.Get("uber-trace-id"), 1)
	require.Contains(t, md.Get("uber-trace-id")[0], parent.SpanContext().TraceID().String()+":")
	endGRPCTrace(span, nil, nil)
	parent.End()
}

func grpcTraceAttribute(span sdktrace.ReadOnlySpan, name string) any {
	for _, attr := range span.Attributes() {
		if string(attr.Key) == name {
			return attr.Value.AsInterface()
		}
	}
	return nil
}
