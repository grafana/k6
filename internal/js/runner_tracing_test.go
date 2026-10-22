package js

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"

	"go.k6.io/k6/v2/errext"
	"go.k6.io/k6/v2/internal/event"
	"go.k6.io/k6/v2/internal/features"
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
	r.preInitState.FeatureFlags = nativeTracing()

	samples := make(chan metrics.SampleContainer, 100)
	defer close(samples)

	// Emulate what the scheduler does for a scenario.
	scenarioCtx, scenarioSpan := k6trace.StartScenario(
		t.Context(), k6trace.Tracer(tp, nativeTracing()), k6trace.ScenarioInfo{Name: "sc", Executor: "per-vu-iterations"},
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
	r.preInitState.FeatureFlags = nativeTracing()

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

func TestNoSpansWhenNativeTracingIsDisabled(t *testing.T) {
	t.Parallel()

	tp, rec := tracetest.NewProvider()
	// Cleanup runs once the parallel subtests are done.
	t.Cleanup(func() { assert.Empty(t, rec.Ended()) })
	tests := []struct {
		name  string
		tp    *k6trace.TracerProvider
		flags *features.Flags
	}{
		{name: "no tracer provider", flags: nativeTracing()},
		{name: "noop tracer provider", tp: k6trace.NewNoopTracerProvider(), flags: nativeTracing()},
		{name: "no feature flag", tp: tp},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r, err := getSimpleRunner(t, "/script.js", `exports.default = function() {}`)
			require.NoError(t, err)
			r.preInitState.TracerProvider = tc.tp
			r.preInitState.FeatureFlags = tc.flags

			samples := make(chan metrics.SampleContainer, 100)
			defer close(samples)

			vu, err := r.newVU(t.Context(), 1, 1, samples)
			require.NoError(t, err)
			assert.Nil(t, vu.tracer, "no spans are started on the hot path when native tracing is disabled")
			seen := spanContextOnIterStart(vu)
			runCtx, cancel := context.WithCancel(t.Context())
			require.NoError(t, vu.Activate(&lib.VUActivationParams{RunContext: runCtx}).RunOnce())
			assert.False(t, (<-seen).IsValid(), "no iteration span in the VU context during the iteration")
			cancel()
		})
	}
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
	r.preInitState.FeatureFlags = nativeTracing()

	samples := make(chan metrics.SampleContainer, 100)
	defer close(samples)

	vu, err := r.newVU(t.Context(), 1, 1, samples)
	require.NoError(t, err)

	// Cancel the activation, as an executor does at the end of a scenario, while the
	// iteration is running.
	runCtx, cancel := context.WithCancel(t.Context())
	defer cancel()
	_, eventsCh := vu.moduleVUImpl.events.local.Subscribe(event.IterStart)
	go func() {
		e := <-eventsCh
		e.Done()
		time.AfterFunc(100*time.Millisecond, cancel)
	}()

	activeVU := vu.Activate(&lib.VUActivationParams{RunContext: runCtx, Scenario: "sc"})
	// Whether RunOnce returns an error depends on the runtime noticing the interruption before
	// the script returns from the interrupted sleep, the span is the same either way.
	_ = activeVU.RunOnce()

	iterations := rec.ByName(k6trace.SpanIteration)
	require.Len(t, iterations, 1)
	assert.Equal(t, codes.Unset, iterations[0].Status().Code)
	assert.Empty(t, iterations[0].Events())
	assert.Contains(t, iterations[0].Attributes(), attribute.Bool("k6.iteration.interrupted", true))
}

func TestIterationSpanIsOnlyInTheVUContextDuringTheIteration(t *testing.T) {
	t.Parallel()

	tp, rec := tracetest.NewProvider()
	r, err := getSimpleRunner(t, "/script.js", `exports.default = function() {}`)
	require.NoError(t, err)
	r.preInitState.TracerProvider = tp
	r.preInitState.FeatureFlags = nativeTracing()

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

	// Between iterations, modules see the VU span, not the iteration span that has ended, in
	// the iteration context, which is done.
	assert.Equal(t, vuSpanCtx, oteltrace.SpanContextFromContext(vu.moduleVUImpl.Context()))
	assert.ErrorIs(t, vu.moduleVUImpl.Context().Err(), context.Canceled)
}

// nativeTracing returns the feature flags enabling the k6 execution hierarchy spans.
func nativeTracing() *features.Flags {
	return &features.Flags{NativeTracing: true}
}

func TestIsInterrupted(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		isFullIteration bool
		err             error
		want            bool
	}{
		{name: "full iteration", isFullIteration: true, want: false},
		{name: "full iteration with an error", isFullIteration: true, err: errors.New("boom"), want: false},
		{name: "test aborted", isFullIteration: true, err: &errext.InterruptError{}, want: false},
		{name: "test aborted, context done", err: &errext.InterruptError{}, want: false},
		{name: "context done", want: true},
		{name: "context done, runtime interrupted", err: &sobek.InterruptedError{}, want: true},
		{name: "context done, canceled", err: fmt.Errorf("op: %w", context.Canceled), want: true},
		{name: "context done, failed", err: errors.New("boom"), want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, isInterrupted(tc.isFullIteration, tc.err))
		})
	}
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
