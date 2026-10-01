package tracing

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"go.k6.io/k6/v2/js/modulestest"
	"go.k6.io/k6/v2/lib"
)

type recordingExporter struct {
	spans []sdktrace.ReadOnlySpan
}

func (e *recordingExporter) ExportSpans(_ context.Context, spans []sdktrace.ReadOnlySpan) error {
	e.spans = append(e.spans, spans...)
	return nil
}

func (*recordingExporter) Shutdown(context.Context) error {
	return nil
}

func TestCurrentSpan(t *testing.T) {
	t.Parallel()

	testRuntime := modulestest.NewRuntime(t)
	module := New().NewModuleInstance(testRuntime.VU)
	require.NoError(t, testRuntime.VU.Runtime().Set("tracing", module.Exports().Named))

	exporter := &recordingExporter{}
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	spanCtx, span := provider.Tracer("test").Start(testRuntime.VU.Context(), "iteration")
	testRuntime.VU.CtxField = spanCtx
	testRuntime.MoveToVUContext(&lib.State{})

	result, err := testRuntime.VU.Runtime().RunString(`
		const span = tracing.currentSpan();
		span.setAttribute("string", "value");
		span.setAttribute("boolean", true);
		span.setAttribute("integer", 42);
		span.setAttribute("float", 1.5);
		span.addEvent("checkpoint");
		JSON.stringify({
			traceId: span.traceId,
			spanId: span.spanId,
			sampled: span.sampled,
			recording: span.recording,
		});
	`)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"traceId": "`+span.SpanContext().TraceID().String()+`",
		"spanId": "`+span.SpanContext().SpanID().String()+`",
		"sampled": true,
		"recording": true
	}`, result.String())

	span.End()
	require.Len(t, exporter.spans, 1)
	require.Equal(t, map[string]any{
		"string":  "value",
		"boolean": true,
		"integer": int64(42),
		"float":   1.5,
	}, attributes(exporter.spans[0]))
	require.Len(t, exporter.spans[0].Events(), 1)
	require.Equal(t, "checkpoint", exporter.spans[0].Events()[0].Name)
}

func TestCurrentSpanWithoutTracingOutput(t *testing.T) {
	t.Parallel()

	testRuntime := modulestest.NewRuntime(t)
	module := New().NewModuleInstance(testRuntime.VU)
	require.NoError(t, testRuntime.VU.Runtime().Set("tracing", module.Exports().Named))
	testRuntime.MoveToVUContext(&lib.State{})

	result, err := testRuntime.VU.Runtime().RunString(`
		const span = tracing.currentSpan();
		span.setAttribute("ignored", true);
		span.addEvent("ignored");
		JSON.stringify(span);
	`)
	require.NoError(t, err)
	require.JSONEq(t, `{"traceId":"","spanId":"","sampled":false,"recording":false}`, result.String())
}

func TestCurrentSpanInInitContext(t *testing.T) {
	t.Parallel()

	testRuntime := modulestest.NewRuntime(t)
	module := New().NewModuleInstance(testRuntime.VU)
	moduleInstance, ok := module.(*ModuleInstance)
	require.True(t, ok)
	_, err := moduleInstance.CurrentSpan()
	require.ErrorIs(t, err, ErrCurrentSpanInInitContext)
}

func attributes(span sdktrace.ReadOnlySpan) map[string]any {
	attrs := make(map[string]any, len(span.Attributes()))
	for _, attr := range span.Attributes() {
		attrs[string(attr.Key)] = attr.Value.AsInterface()
	}
	return attrs
}
