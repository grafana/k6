package trace

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func TestNilProviderFallbacks(t *testing.T) {
	t.Parallel()

	scenarioCtx, scenarioSpan := StartScenario(context.Background(), nil, ScenarioInfo{})
	require.False(t, scenarioSpan.SpanContext().IsValid())
	require.False(t, oteltrace.SpanContextFromContext(scenarioCtx).IsValid())

	vuCtx, vuSpan := StartVU(context.Background(), nil, VUInfo{})
	require.False(t, vuSpan.SpanContext().IsValid())
	require.False(t, oteltrace.SpanContextFromContext(vuCtx).IsValid())
}

func TestScenarioSpanLifecycle(t *testing.T) {
	t.Parallel()

	exporter := &recordingExporter{}
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	runCtx, runSpan := StartTestRun(context.Background(), provider)
	runSpanContext := runSpan.SpanContext()

	scenarioCtx, scenarioSpan := StartScenario(runCtx, provider, ScenarioInfo{
		Name:     "checkout",
		Executor: "ramping-vus",
	})
	scenarioSpanContext := scenarioSpan.SpanContext()

	require.Equal(t, runSpanContext.TraceID(), scenarioSpanContext.TraceID())
	require.Equal(t, scenarioSpanContext, oteltrace.SpanContextFromContext(scenarioCtx))

	EndSpan(scenarioSpan, nil)
	EndSpan(runSpan, nil)
	require.Len(t, exporter.spans, 2)

	scenario := exporter.spans[0]
	require.Equal(t, "k6.scenario", scenario.Name())
	require.True(t, scenario.Parent().IsValid())
	require.Equal(t, runSpanContext.SpanID(), scenario.Parent().SpanID())
	require.Equal(t, map[string]any{
		"test.scenario":        "checkout",
		"k6.scenario.executor": "ramping-vus",
	}, spanAttributes(scenario))
}

func TestVUSpanLifecycle(t *testing.T) {
	t.Parallel()

	exporter := &recordingExporter{}
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	runCtx, runSpan := StartTestRun(context.Background(), provider)
	scenarioCtx, scenarioSpan := StartScenario(runCtx, provider, ScenarioInfo{Name: "checkout"})
	scenarioSpanContext := scenarioSpan.SpanContext()

	vuCtx, vuSpan := StartVU(scenarioCtx, provider, VUInfo{
		VUID:       2,
		VUIDGlobal: 7,
		Scenario:   "checkout",
	})
	vuSpanContext := vuSpan.SpanContext()

	require.Equal(t, scenarioSpanContext.TraceID(), vuSpanContext.TraceID())
	require.Equal(t, vuSpanContext, oteltrace.SpanContextFromContext(vuCtx))

	EndSpan(vuSpan, nil)
	EndSpan(scenarioSpan, nil)
	EndSpan(runSpan, nil)
	require.Len(t, exporter.spans, 3)

	vu := exporter.spans[0]
	require.Equal(t, "k6.vu", vu.Name())
	require.True(t, vu.Parent().IsValid())
	require.Equal(t, scenarioSpanContext.SpanID(), vu.Parent().SpanID())
	require.Equal(t, map[string]any{
		"test.vu":              int64(2),
		"k6.vu.id_in_instance": int64(2),
		"k6.vu.id_in_test":     int64(7),
		"test.scenario":        "checkout",
	}, spanAttributes(vu))
}

// TestSplitIterationLinksToVUSpanNotRunSpan proves --traces-split's iteration
// link points at the immediate ambient span (the VU span), not the run span,
// even though k6.run/k6.scenario/k6.vu all share one trace ID.
func TestSplitIterationLinksToVUSpanNotRunSpan(t *testing.T) {
	t.Parallel()

	exporter := &recordingExporter{}
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	runCtx, runSpan := StartTestRun(context.Background(), provider)
	runSpanContext := runSpan.SpanContext()
	scenarioCtx, scenarioSpan := StartScenario(runCtx, provider, ScenarioInfo{Name: "checkout"})
	vuCtx, vuSpan := StartVU(scenarioCtx, provider, VUInfo{VUID: 2, VUIDGlobal: 7, Scenario: "checkout"})
	vuSpanContext := vuSpan.SpanContext()

	_, iterationSpan := StartIteration(vuCtx, provider, IterationInfo{VUID: 2, VUIDGlobal: 7}, true)

	EndSpan(iterationSpan, nil)
	EndSpan(vuSpan, nil)
	EndSpan(scenarioSpan, nil)
	EndSpan(runSpan, nil)
	require.Len(t, exporter.spans, 4)

	iteration := exporter.spans[0]
	require.Equal(t, "iteration", iteration.Name())
	require.NotEqual(t, runSpanContext.TraceID(), iteration.SpanContext().TraceID(),
		"the iteration must still be an independent trace when split")
	require.Len(t, iteration.Links(), 1)
	link := iteration.Links()[0].SpanContext
	require.Equal(t, vuSpanContext.SpanID(), link.SpanID(),
		"the link must point at the VU span's SpanID, not the run span's")
	require.NotEqual(t, runSpanContext.SpanID(), link.SpanID())
	require.Equal(t, vuSpanContext.TraceID(), link.TraceID(),
		"k6.run/k6.scenario/k6.vu share one trace, so the linked TraceID matches the run's too")
}
