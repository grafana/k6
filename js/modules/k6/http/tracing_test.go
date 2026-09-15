package http

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"

	k6trace "go.k6.io/k6/v2/internal/lib/trace"
	"go.k6.io/k6/v2/js/modulestest"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/metrics"
)

type httpTraceExporter struct {
	mu    sync.Mutex
	spans []sdktrace.ReadOnlySpan
}

func (e *httpTraceExporter) ExportSpans(_ context.Context, spans []sdktrace.ReadOnlySpan) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.spans = append(e.spans, spans...)
	return nil
}

func (*httpTraceExporter) Shutdown(context.Context) error {
	return nil
}

func (e *httpTraceExporter) exportedSpans() []sdktrace.ReadOnlySpan {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]sdktrace.ReadOnlySpan(nil), e.spans...)
}

func TestEnableTracing(t *testing.T) {
	t.Parallel()

	t.Run("init context", func(t *testing.T) {
		t.Parallel()

		testRuntime := modulestest.NewRuntime(t)
		root := New()
		module := root.NewModuleInstance(testRuntime.VU).(*ModuleInstance) //nolint:forcetypeassert
		module.EnableTracing()
		require.True(t, module.tracingEnabled)
		require.True(t, root.tracingEnabled.Load())

		nextRuntime := modulestest.NewRuntime(t)
		next := root.NewModuleInstance(nextRuntime.VU).(*ModuleInstance) //nolint:forcetypeassert
		require.True(t, next.tracingEnabled)
	})

	t.Run("VU context", func(t *testing.T) {
		t.Parallel()

		testRuntime := modulestest.NewRuntime(t)
		root := New()
		module := root.NewModuleInstance(testRuntime.VU).(*ModuleInstance) //nolint:forcetypeassert
		state := &lib.State{}
		testRuntime.MoveToVUContext(state)
		module.EnableTracing()
		require.True(t, module.tracingEnabled)
		require.False(t, root.tracingEnabled.Load())
	})
}

func TestHTTPTracingOptIn(t *testing.T) {
	t.Parallel()

	testState := newTestCase(t)
	traceparents := make(chan string, 4)
	testState.tb.Mux.HandleFunc("/native-tracing", func(_ http.ResponseWriter, request *http.Request) {
		traceparents <- request.Header.Get("traceparent")
	})

	exporter := &httpTraceExporter{}
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	testState.runtime.VU.State().TracerProvider = provider
	iterationCtx, iterationSpan := provider.Tracer("test").Start(
		testState.runtime.VU.Context(), "iteration",
	)
	testState.runtime.VU.CtxField = iterationCtx
	url := testState.tb.Replacer.Replace("HTTPBIN_URL/native-tracing")

	request := func(params string) string {
		t.Helper()
		_, err := testState.runtime.VU.Runtime().RunString(`http.get("` + url + `"` + params + `)`)
		require.NoError(t, err)
		return <-traceparents
	}

	require.Empty(t, request(""), "HTTP tracing should be disabled by default")
	require.Empty(t, exporter.exportedSpans())

	traceparent := request(`, { tracing: true }`)
	parts := strings.Split(traceparent, "-")
	require.Len(t, parts, 4)
	require.Equal(t, iterationSpan.SpanContext().TraceID().String(), parts[1])

	spans := exporter.exportedSpans()
	require.Len(t, spans, 1)
	require.Equal(t, "http.request", spans[0].Name())
	require.Equal(t, oteltrace.SpanKindClient, spans[0].SpanKind())
	require.Equal(t, iterationSpan.SpanContext().SpanID(), spans[0].Parent().SpanID())
	require.Equal(t, parts[2], spans[0].SpanContext().SpanID().String())
	require.Equal(t, iterationSpan.SpanContext().TraceID(), spans[0].SpanContext().TraceID())
	require.Equal(t, "GET", traceAttribute(spans[0], "http.request.method"))
	require.Equal(t, int64(200), traceAttribute(spans[0], "http.response.status_code"))
	requireTraceIDMetadata(t, testState.samples, iterationSpan.SpanContext().TraceID().String())

	testState.instance.tracingEnabled = true
	require.Empty(t, request(`, { tracing: false }`), "per-request false should override VU enablement")
	require.Len(t, exporter.exportedSpans(), 1)
	require.NotEmpty(t, request(""), "VU enablement should trace requests by default")
	require.Len(t, exporter.exportedSpans(), 2)

	iterationSpan.End()
}

func TestHTTPTracingOptionRejectsNonBoolean(t *testing.T) {
	t.Parallel()

	testState := newTestCase(t)
	url := testState.tb.Replacer.Replace("HTTPBIN_URL/get")
	_, err := testState.runtime.VU.Runtime().RunString(`http.get("` + url + `", { tracing: "yes" })`)
	require.ErrorContains(t, err, "invalid tracing option: expected a boolean")
}

func TestHTTPTracingUsesConfiguredPropagator(t *testing.T) {
	t.Parallel()

	testState := newTestCase(t)
	traceHeaders := make(chan string, 1)
	testState.tb.Mux.HandleFunc("/jaeger-tracing", func(_ http.ResponseWriter, request *http.Request) {
		traceHeaders <- request.Header.Get("uber-trace-id")
	})

	provider := sdktrace.NewTracerProvider()
	testState.runtime.VU.State().TracerProvider = provider
	propagator, err := k6trace.PropagatorFromConfig("jaeger")
	require.NoError(t, err)
	testState.runtime.VU.State().TracePropagator = propagator
	iterationCtx, iterationSpan := provider.Tracer("test").Start(testState.runtime.VU.Context(), "iteration")
	testState.runtime.VU.CtxField = iterationCtx
	url := testState.tb.Replacer.Replace("HTTPBIN_URL/jaeger-tracing")

	_, err = testState.runtime.VU.Runtime().RunString(`http.get("` + url + `", { tracing: true })`)
	require.NoError(t, err)
	parts := strings.Split(<-traceHeaders, ":")
	require.Len(t, parts, 4)
	require.Equal(t, iterationSpan.SpanContext().TraceID().String(), parts[0])
	iterationSpan.End()
}

func traceAttribute(span sdktrace.ReadOnlySpan, name string) any {
	for _, attr := range span.Attributes() {
		if string(attr.Key) == name {
			return attr.Value.AsInterface()
		}
	}
	return nil
}

func requireTraceIDMetadata(t *testing.T, samples <-chan metrics.SampleContainer, traceID string) {
	t.Helper()
	for _, sampleContainer := range metrics.GetBufferedSamples(samples) {
		for _, sample := range sampleContainer.GetSamples() {
			if sample.Metadata["trace_id"] == traceID {
				return
			}
		}
	}
	t.Fatalf("trace_id %q was not found in HTTP sample metadata", traceID)
}
