package js

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"

	"go.k6.io/k6/v2/internal/event"
	k6trace "go.k6.io/k6/v2/internal/lib/trace"
	"go.k6.io/k6/v2/internal/lib/trace/tracetest"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/metrics"
)

func TestVUAndIterationSpans(t *testing.T) {
	t.Parallel()

	tp, rec := tracetest.NewProvider()
	r, err := getSimpleRunner(t, "/script.js", `exports.default = function() {}`)
	require.NoError(t, err)
	r.preInitState.TracerProvider = tp

	samples := make(chan metrics.SampleContainer, 100)
	defer close(samples)

	// Emulate what the scheduler does for a scenario.
	scenarioCtx, scenarioSpan := k6trace.StartScenario(
		t.Context(), k6trace.Tracer(tp), k6trace.ScenarioInfo{Name: "sc", Executor: "per-vu-iterations"},
	)
	runCtx, cancel := context.WithCancel(scenarioCtx)

	vu, err := r.newVU(t.Context(), 7, 8, samples)
	require.NoError(t, err)
	activeVU := vu.Activate(&lib.VUActivationParams{RunContext: runCtx, Scenario: "sc"})
	require.NoError(t, activeVU.RunOnce())
	require.NoError(t, activeVU.RunOnce())

	// Spans of the activation end when its context is done.
	assert.Empty(t, rec.ByName(k6trace.SpanVU), "the VU span must outlive its iterations")
	require.Len(t, rec.ByName(k6trace.SpanIteration), 2)
	cancel()
	require.Eventually(t, func() bool { return len(rec.ByName(k6trace.SpanVU)) == 1 }, 5e9, 1e6)
	k6trace.EndSpan(scenarioSpan, nil)

	vuSpan := rec.ByName(k6trace.SpanVU)[0]
	assert.Equal(t, scenarioSpan.SpanContext().SpanID(), vuSpan.Parent().SpanID())
	assert.Equal(t, scenarioSpan.SpanContext().TraceID(), vuSpan.SpanContext().TraceID())

	for i, it := range rec.ByName(k6trace.SpanIteration) {
		assert.Equal(t, vuSpan.SpanContext().SpanID(), it.Parent().SpanID(), "iteration %d parent", i)
		assert.Equal(t, scenarioSpan.SpanContext().TraceID(), it.SpanContext().TraceID())
		attrs := map[string]any{}
		for _, kv := range it.Attributes() {
			attrs[string(kv.Key)] = kv.Value.AsInterface()
		}
		assert.Equal(t, int64(i), attrs["test.iteration.number"])
		assert.Equal(t, int64(7), attrs["test.vu"])
		assert.Equal(t, int64(8), attrs["k6.vu.id_in_test"])
		assert.Equal(t, "sc", attrs["test.scenario"])
		assert.Equal(t, int64(i), attrs["k6.vu.iteration_in_scenario"])
		assert.NotContains(t, attrs, "k6.scenario.iteration_in_test", "no counters without an executor")
	}
}

func TestIterationSpanIsInTheVUContext(t *testing.T) {
	t.Parallel()

	tp, rec := tracetest.NewProvider()
	r, err := getSimpleRunner(t, "/script.js", `exports.default = function() {}`)
	require.NoError(t, err)
	r.preInitState.TracerProvider = tp

	samples := make(chan metrics.SampleContainer, 100)
	defer close(samples)

	vu, err := r.newVU(t.Context(), 1, 1, samples)
	require.NoError(t, err)

	// Modules, e.g. the browser, read the iteration span from the VU context on IterStart.
	seen := spanContextOnIterStart(vu)

	activeVU := vu.Activate(&lib.VUActivationParams{RunContext: t.Context(), Scenario: "sc"})
	require.NoError(t, activeVU.RunOnce())

	iterations := rec.ByName(k6trace.SpanIteration)
	require.Len(t, iterations, 1)
	assert.Equal(t, iterations[0].SpanContext().SpanID(), (<-seen).SpanID())
}

func TestNoSpansWithoutTracerProvider(t *testing.T) {
	t.Parallel()

	r, err := getSimpleRunner(t, "/script.js", `exports.default = function() {}`)
	require.NoError(t, err)
	require.Nil(t, r.preInitState.TracerProvider)

	samples := make(chan metrics.SampleContainer, 100)
	defer close(samples)

	vu, err := r.newVU(t.Context(), 1, 1, samples)
	require.NoError(t, err)
	assert.Nil(t, vu.tracer)
	seen := spanContextOnIterStart(vu)
	require.NoError(t, vu.Activate(&lib.VUActivationParams{RunContext: t.Context()}).RunOnce())
	assert.False(t, (<-seen).IsValid(), "no iteration span in the VU context during the iteration")
}

func TestInterruptedIterationSpanIsNotAnError(t *testing.T) {
	t.Parallel()

	tp, rec := tracetest.NewProvider()
	r, err := getSimpleRunner(t, "/script.js", `
		const k6 = require("k6");
		exports.default = function() { k6.sleep(10); }
	`)
	require.NoError(t, err)
	r.preInitState.TracerProvider = tp

	samples := make(chan metrics.SampleContainer, 100)
	defer close(samples)

	vu, err := r.newVU(t.Context(), 1, 1, samples)
	require.NoError(t, err)

	// Cancel the activation, as an executor does at the end of a scenario, while the
	// iteration is running.
	runCtx, cancel := context.WithCancel(t.Context())
	_, eventsCh := vu.moduleVUImpl.events.local.Subscribe(event.IterStart)
	go func() {
		e := <-eventsCh
		e.Done()
		cancel()
	}()

	activeVU := vu.Activate(&lib.VUActivationParams{RunContext: runCtx, Scenario: "sc"})
	require.Error(t, activeVU.RunOnce())

	iterations := rec.ByName(k6trace.SpanIteration)
	require.Len(t, iterations, 1)
	assert.Equal(t, codes.Unset, iterations[0].Status().Code)
	assert.Empty(t, iterations[0].Events())
	assert.Contains(t, iterations[0].Attributes(), attribute.Bool("k6.iteration.interrupted", true))
}

func TestNoTracerWithNoopTracerProvider(t *testing.T) {
	t.Parallel()

	r, err := getSimpleRunner(t, "/script.js", `exports.default = function() {}`)
	require.NoError(t, err)
	r.preInitState.TracerProvider = k6trace.NewNoopTracerProvider()

	samples := make(chan metrics.SampleContainer, 100)
	defer close(samples)

	vu, err := r.newVU(t.Context(), 1, 1, samples)
	require.NoError(t, err)
	assert.Nil(t, vu.tracer, "no spans are started on the hot path when tracing is disabled")
	seen := spanContextOnIterStart(vu)
	require.NoError(t, vu.Activate(&lib.VUActivationParams{RunContext: t.Context()}).RunOnce())
	assert.False(t, (<-seen).IsValid(), "no iteration span in the VU context during the iteration")
}

func TestIterationSpanIsOnlyInTheVUContextDuringTheIteration(t *testing.T) {
	t.Parallel()

	tp, rec := tracetest.NewProvider()
	r, err := getSimpleRunner(t, "/script.js", `exports.default = function() {}`)
	require.NoError(t, err)
	r.preInitState.TracerProvider = tp

	samples := make(chan metrics.SampleContainer, 100)
	defer close(samples)

	vu, err := r.newVU(t.Context(), 1, 1, samples)
	require.NoError(t, err)

	runCtx, cancel := context.WithCancel(t.Context())
	defer cancel()
	activeVU := vu.Activate(&lib.VUActivationParams{RunContext: runCtx, Scenario: "sc"})
	vuSpanCtx := oteltrace.SpanContextFromContext(vu.moduleVUImpl.Context())
	require.True(t, vuSpanCtx.IsValid())

	require.NoError(t, activeVU.RunOnce())
	require.Len(t, rec.ByName(k6trace.SpanIteration), 1)

	// Between iterations, modules see the VU span, not the iteration span that has ended.
	assert.Equal(t, vuSpanCtx, oteltrace.SpanContextFromContext(vu.moduleVUImpl.Context()))
}

// spanContextOnIterStart returns the span context the modules see in the VU context when they
// handle the IterStart event of the next iteration.
func spanContextOnIterStart(vu *VU) <-chan oteltrace.SpanContext {
	_, eventsCh := vu.moduleVUImpl.events.local.Subscribe(event.IterStart)
	seen := make(chan oteltrace.SpanContext, 1)
	go func() {
		e := <-eventsCh
		seen <- oteltrace.SpanContextFromContext(vu.moduleVUImpl.Context())
		e.Done()
	}()
	return seen
}
