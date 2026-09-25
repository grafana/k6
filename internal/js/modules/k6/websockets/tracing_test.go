package websockets

import (
	"context"
	"net/http"
	"net/url"
	"sync"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"

	k6trace "go.k6.io/k6/v2/internal/lib/trace"
	"go.k6.io/k6/v2/js/modulestest"
	"go.k6.io/k6/v2/lib"
	moduletrace "go.k6.io/k6/v2/lib/trace"
	"go.k6.io/k6/v2/metrics"
)

func TestModuleInstanceTracingDefault(t *testing.T) {
	t.Parallel()

	t.Run("enabled via --tracing", func(t *testing.T) {
		t.Parallel()
		runtime := modulestest.NewRuntime(t)
		set, err := moduletrace.ParseSet("websockets")
		require.NoError(t, err)
		runtime.VU.InitEnvField.Tracing = set

		module := New().NewModuleInstance(runtime.VU).(*WebSocketsAPI)
		require.True(t, module.tracingEnabled)
	})

	t.Run("disabled by default", func(t *testing.T) {
		t.Parallel()
		runtime := modulestest.NewRuntime(t)
		module := New().NewModuleInstance(runtime.VU).(*WebSocketsAPI)
		require.False(t, module.tracingEnabled)
	})
}

func TestTracingOption(t *testing.T) {
	t.Parallel()

	runtime := modulestest.NewRuntime(t)
	state := &lib.State{Tags: lib.NewVUStateTags(metrics.NewRegistry().RootTagSet())}
	runtime.MoveToVUContext(state)

	value, err := runtime.VU.Runtime().RunString(`({ tracing: false })`)
	require.NoError(t, err)
	params, err := buildParams(state, runtime.VU.Runtime(), value, true)
	require.NoError(t, err)
	require.False(t, params.tracing, "the constructor option should override module enablement")

	value, err = runtime.VU.Runtime().RunString(`({ tracing: "yes" })`)
	require.NoError(t, err)
	_, err = buildParams(state, runtime.VU.Runtime(), value, true)
	require.ErrorContains(t, err, "invalid tracing option: expected a boolean")
}

type websocketTraceExporter struct {
	mu    sync.Mutex
	spans []sdktrace.ReadOnlySpan
}

func (e *websocketTraceExporter) ExportSpans(_ context.Context, spans []sdktrace.ReadOnlySpan) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.spans = append(e.spans, spans...)
	return nil
}

func (*websocketTraceExporter) Shutdown(context.Context) error { return nil }

func (e *websocketTraceExporter) exportedSpans() []sdktrace.ReadOnlySpan {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]sdktrace.ReadOnlySpan(nil), e.spans...)
}

func TestSessionTrace(t *testing.T) {
	t.Parallel()

	exporter := &websocketTraceExporter{}
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	parentCtx, parent := provider.Tracer("test").Start(t.Context(), "iteration")
	target, err := url.Parse("wss://user:secret@example.test/socket?q=1")
	require.NoError(t, err)
	headers := make(http.Header)
	tagsAndMeta := &metrics.TagsAndMeta{}

	_, span := startSessionTrace(parentCtx, &lib.State{TracerProvider: provider}, target, headers, tagsAndMeta)
	require.NotEmpty(t, headers.Get("traceparent"))
	require.Equal(t, parent.SpanContext().TraceID().String(), tagsAndMeta.Metadata["trace_id"])
	endSessionTrace(span, http.StatusSwitchingProtocols, nil)

	spans := exporter.exportedSpans()
	require.Len(t, spans, 1)
	got := spans[0]
	require.Equal(t, "websocket.session", got.Name())
	require.Equal(t, oteltrace.SpanKindClient, got.SpanKind())
	require.Equal(t, parent.SpanContext().SpanID(), got.Parent().SpanID())
	require.Equal(t, "wss://user:xxxxx@example.test/socket?q=1", websocketTraceAttribute(got, "url.full"))
	require.Equal(t, "websocket", websocketTraceAttribute(got, "network.protocol.name"))
	require.Equal(t, int64(http.StatusSwitchingProtocols), websocketTraceAttribute(got, "http.response.status_code"))
	parent.End()
}

func TestSessionTraceUsesConfiguredPropagator(t *testing.T) {
	t.Parallel()

	provider := sdktrace.NewTracerProvider()
	parentCtx, parent := provider.Tracer("test").Start(t.Context(), "iteration")
	propagator, err := k6trace.PropagatorFromConfig("jaeger")
	require.NoError(t, err)
	state := &lib.State{TracerProvider: provider, TracePropagator: propagator}
	target, err := url.Parse("wss://example.test/socket")
	require.NoError(t, err)
	headers := make(http.Header)

	_, span := startSessionTrace(parentCtx, state, target, headers, &metrics.TagsAndMeta{})
	require.Contains(t, headers.Get("uber-trace-id"), parent.SpanContext().TraceID().String()+":")
	endSessionTrace(span, http.StatusSwitchingProtocols, nil)
	parent.End()
}

func TestWebSocketOperationTracing(t *testing.T) {
	t.Parallel()

	ts := newTestState(t)
	traceparent := make(chan string, 1)
	ts.addHandler("/ws-tracing", &websocket.Upgrader{
		CheckOrigin: func(req *http.Request) bool {
			traceparent <- req.Header.Get("traceparent")
			return true
		},
	}, &testMessage{kind: websocket.TextMessage, data: []byte("hello")})

	exporter := &websocketTraceExporter{}
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	ts.runtime.VU.State().TracerProvider = provider
	parentCtx, parent := provider.Tracer("test").Start(ts.runtime.VU.Context(), "iteration")
	ts.runtime.VU.CtxField = parentCtx
	ts.module.tracingEnabled = true

	err := runOnEventLoopWithTimeout(t, ts.runtime, ts.tb.Replacer.Replace(`
		const ws = new WebSocket("WSBIN_URL/ws-tracing");
		ws.onmessage = () => {
			call("message");
			ws.close();
		};
		ws.onclose = () => call("close");
	`))
	require.NoError(t, err)
	require.Equal(t, []string{"message", "close"}, ts.callRecorder.Recorded())
	require.NotEmpty(t, <-traceparent)
	require.Empty(t, ts.errors)

	spans := exporter.exportedSpans()
	require.Len(t, spans, 1)
	span := spans[0]
	require.Equal(t, "websocket.session", span.Name())
	require.Equal(t, parent.SpanContext().SpanID(), span.Parent().SpanID())
	require.Equal(t, oteltrace.SpanKindClient, span.SpanKind())
	require.Equal(t, int64(http.StatusSwitchingProtocols),
		websocketTraceAttribute(span, "http.response.status_code"))

	traceID := parent.SpanContext().TraceID().String()
	traceMetadataFound := false
	for _, container := range metrics.GetBufferedSamples(ts.samples) {
		for _, sample := range container.GetSamples() {
			if sample.Metadata["trace_id"] == traceID {
				traceMetadataFound = true
			}
		}
	}
	require.True(t, traceMetadataFound)
	parent.End()
}

func websocketTraceAttribute(span sdktrace.ReadOnlySpan, name string) any {
	for _, attr := range span.Attributes() {
		if string(attr.Key) == name {
			return attr.Value.AsInterface()
		}
	}
	return nil
}
