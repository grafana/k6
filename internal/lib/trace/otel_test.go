package trace

import (
	"context"
	"errors"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
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

func TestNilTracerProviderUsesNoopTracer(t *testing.T) {
	t.Parallel()

	var provider *TracerProvider
	runCtx, runSpan := StartTestRun(context.Background(), provider, logrus.StandardLogger())
	require.False(t, runSpan.SpanContext().IsValid())

	iterationCtx, iterationSpan := StartIteration(runCtx, provider, IterationInfo{}, false)
	require.False(t, iterationSpan.SpanContext().IsValid())
	require.False(t, oteltrace.SpanContextFromContext(iterationCtx).IsValid())
}

func TestTestRunSpanLifecycle(t *testing.T) {
	t.Parallel()

	exporter := &recordingExporter{}
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	parentCtx, parent := provider.Tracer("test").Start(context.Background(), "parent")
	defer parent.End()
	runCtx, span := StartTestRun(parentCtx, provider, logrus.StandardLogger())

	require.True(t, oteltrace.SpanContextFromContext(runCtx).IsValid())
	require.Empty(t, exporter.spans)

	runErr := errors.New("run failed")
	EndSpan(span, runErr)

	require.Len(t, exporter.spans, 1)
	require.Equal(t, "k6.run", exporter.spans[0].Name())
	require.False(t, exporter.spans[0].Parent().IsValid())
	require.Equal(t, codes.Error, exporter.spans[0].Status().Code)
	require.Equal(t, runErr.Error(), exporter.spans[0].Status().Description)
}

func TestTestRunUsesRemoteParent(t *testing.T) {
	t.Parallel()

	exporter := &recordingExporter{}
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	parentTraceID, err := oteltrace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	require.NoError(t, err)
	parentSpanID, err := oteltrace.SpanIDFromHex("00f067aa0ba902b7")
	require.NoError(t, err)
	parent := oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
		TraceID: parentTraceID, SpanID: parentSpanID, TraceFlags: oteltrace.FlagsSampled, Remote: true,
	})
	ctx := oteltrace.ContextWithRemoteSpanContext(t.Context(), parent)

	_, runSpan := StartTestRun(ctx, provider, logrus.StandardLogger())
	EndSpan(runSpan, nil)

	require.Len(t, exporter.spans, 1)
	require.Equal(t, parentTraceID, exporter.spans[0].SpanContext().TraceID())
	require.Equal(t, parentSpanID, exporter.spans[0].Parent().SpanID())
}

func TestIterationSpanLifecycleSplit(t *testing.T) {
	t.Parallel()

	exporter := &recordingExporter{}
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	runCtx, runSpan := StartTestRun(context.Background(), provider, logrus.StandardLogger())
	runSpanContext := runSpan.SpanContext()
	iterationCtx, iterationSpan := StartIteration(runCtx, provider, IterationInfo{
		Number:                      3,
		VUID:                        2,
		VUIDGlobal:                  7,
		VUIterationInScenario:       1,
		Scenario:                    "checkout",
		ScenarioIterationInInstance: 8,
		ScenarioIterationInTest:     13,
		HasScenarioIterationNumbers: true,
	}, true)
	iterationSpanContext := iterationSpan.SpanContext()

	require.NotEqual(t, runSpanContext.TraceID(), iterationSpanContext.TraceID())
	require.Equal(t, iterationSpanContext, oteltrace.SpanContextFromContext(iterationCtx))

	EndSpan(iterationSpan, nil)
	EndSpan(runSpan, nil)
	require.Len(t, exporter.spans, 2)

	iteration := exporter.spans[0]
	require.Equal(t, "iteration", iteration.Name())
	require.False(t, iteration.Parent().IsValid())
	require.Len(t, iteration.Links(), 1)
	require.Equal(t, runSpanContext, iteration.Links()[0].SpanContext)
	require.Equal(t, map[string]any{
		"test.iteration.number":             int64(3),
		"test.vu":                           int64(2),
		"test.scenario":                     "checkout",
		"k6.run.id":                         runSpanContext.TraceID().String(),
		"k6.vu.id_in_instance":              int64(2),
		"k6.vu.id_in_test":                  int64(7),
		"k6.vu.iteration_in_scenario":       int64(1),
		"k6.scenario.iteration_in_instance": int64(8),
		"k6.scenario.iteration_in_test":     int64(13),
	}, spanAttributes(iteration))
}

func TestIterationSpanLifecycleNested(t *testing.T) {
	t.Parallel()

	exporter := &recordingExporter{}
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	runCtx, runSpan := StartTestRun(context.Background(), provider, logrus.StandardLogger())
	runSpanContext := runSpan.SpanContext()
	iterationCtx, iterationSpan := StartIteration(runCtx, provider, IterationInfo{
		Number:                      3,
		VUID:                        2,
		VUIDGlobal:                  7,
		VUIterationInScenario:       1,
		Scenario:                    "checkout",
		ScenarioIterationInInstance: 8,
		ScenarioIterationInTest:     13,
		HasScenarioIterationNumbers: true,
	}, false)
	iterationSpanContext := iterationSpan.SpanContext()

	require.Equal(t, runSpanContext.TraceID(), iterationSpanContext.TraceID())
	require.Equal(t, iterationSpanContext, oteltrace.SpanContextFromContext(iterationCtx))

	EndSpan(iterationSpan, nil)
	EndSpan(runSpan, nil)
	require.Len(t, exporter.spans, 2)

	iteration := exporter.spans[0]
	require.Equal(t, "iteration", iteration.Name())
	require.True(t, iteration.Parent().IsValid())
	require.Equal(t, runSpanContext.SpanID(), iteration.Parent().SpanID())
	require.Empty(t, iteration.Links())
	require.Equal(t, map[string]any{
		"test.iteration.number":             int64(3),
		"test.vu":                           int64(2),
		"test.scenario":                     "checkout",
		"k6.vu.id_in_instance":              int64(2),
		"k6.vu.id_in_test":                  int64(7),
		"k6.vu.iteration_in_scenario":       int64(1),
		"k6.scenario.iteration_in_instance": int64(8),
		"k6.scenario.iteration_in_test":     int64(13),
	}, spanAttributes(iteration))
}

func spanAttributes(span sdktrace.ReadOnlySpan) map[string]any {
	attrs := make(map[string]any, len(span.Attributes()))
	for _, attr := range span.Attributes() {
		attrs[string(attr.Key)] = attr.Value.AsInterface()
	}
	return attrs
}

func TestTracerProviderParamsFromConfigLine(t *testing.T) {
	t.Parallel()

	testCases := [...]struct {
		name      string
		line      string
		expParams tracerProviderParams
		expErr    error
	}{
		{
			name:      "default otel",
			line:      "otel",
			expParams: defaultTracerProviderParams(),
		},
		{
			name: "otel with insecure url and custom path",
			line: "otel=http://localhost:4444/custom/traces",
			expParams: tracerProviderParams{
				proto:    "http",
				endpoint: "localhost:4444",
				urlPath:  "/custom/traces",
				insecure: true,
				headers:  make(map[string]string),
			},
		},
		{
			name: "otel with secure url and no port",
			line: "otel=https://localhost",
			expParams: tracerProviderParams{
				proto:    "http",
				endpoint: "localhost",
				headers:  make(map[string]string),
			},
		},
		{
			name: "otel with grpc proto",
			line: "otel=https://localhost,proto=grpc",
			expParams: tracerProviderParams{
				proto:    "grpc",
				endpoint: "localhost",
				headers:  make(map[string]string),
			},
		},
		{
			name: "otel with headers",
			line: "otel=https://localhost,header.Authorization=token ***,header.other=test",
			expParams: tracerProviderParams{
				proto:    "http",
				endpoint: "localhost",
				headers: map[string]string{
					"Authorization": "token ***",
					"other":         "test",
				},
			},
		},
		{
			name:   "error invalid output",
			line:   "invalid",
			expErr: ErrInvalidTracesOutput,
		},
		{
			name:   "error invalid scheme",
			line:   "otel=invalid://localhost:4444/traces",
			expErr: ErrInvalidURLScheme,
		},
		{
			name:   "error invalid proto",
			line:   "otel=http://localhost:4444,proto=invalid",
			expErr: ErrInvalidProto,
		},
		{
			name:   "error invalid grpc proto with URL path",
			line:   "otel=http://localhost:4444/url/path,proto=grpc",
			expErr: ErrInvalidGRPCWithURLPath,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			params, err := tracerProviderParamsFromConfigLine(tc.line)
			if err != nil {
				require.ErrorIs(t, err, tc.expErr)
				return
			}
			require.Equal(t, tc.expParams, params)
		})
	}
}
