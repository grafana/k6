package execution_test

import (
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"

	"go.k6.io/k6/v2/internal/execution"
	"go.k6.io/k6/v2/internal/execution/local"
	"go.k6.io/k6/v2/internal/js"
	k6trace "go.k6.io/k6/v2/internal/lib/trace"
	"go.k6.io/k6/v2/internal/lib/trace/tracetest"
	"go.k6.io/k6/v2/internal/loader"
	"go.k6.io/k6/v2/metrics"
)

func TestSchedulerTracesScenariosVUsAndIterations(t *testing.T) {
	t.Parallel()

	tp, rec := tracetest.NewProvider()
	piState := getTestPreInitState(t)
	piState.TracerProvider = tp

	sourceData := &loader.SourceData{URL: &url.URL{Path: "/script.js"}, Data: []byte(`
		export const options = {
			scenarios: {
				first: { executor: "per-vu-iterations", vus: 2, iterations: 2 },
				second: { executor: "shared-iterations", vus: 1, iterations: 1, exec: "other" },
			},
		};
		export default function () {}
		export function other() {}
	`)}
	moduleResolver := js.NewModuleResolver(loader.Dir(sourceData.URL), piState, nil)
	runner, err := js.New(piState, sourceData, nil, moduleResolver)
	require.NoError(t, err)

	testRunState := getTestRunState(t, piState, runner.GetOptions(), runner)
	execScheduler, err := execution.NewScheduler(testRunState, local.NewController())
	require.NoError(t, err)

	// The run span is created outside of the scheduler and carried by the run context.
	runCtx, runSpan := k6trace.StartTestRun(t.Context(), k6trace.Tracer(tp), testRunState.Logger)

	samples := make(chan metrics.SampleContainer, 100)
	go func() {
		for range samples { //nolint:revive
		}
	}()
	defer close(samples)

	stopEmission, err := execScheduler.Init(runCtx, samples)
	require.NoError(t, err)
	defer stopEmission()
	require.NoError(t, execScheduler.Run(runCtx, runCtx, samples))
	k6trace.EndSpan(runSpan, nil)

	// VU spans end asynchronously, once the executor cancels the activation context.
	require.Eventually(t, func() bool {
		return len(rec.ByName(k6trace.SpanVU)) == 3
	}, 5*time.Second, 5*time.Millisecond)

	scenarios := rec.ByName(k6trace.SpanScenario)
	vus := rec.ByName(k6trace.SpanVU)
	iterations := rec.ByName(k6trace.SpanIteration)
	require.Len(t, scenarios, 2)
	require.Len(t, vus, 3)
	require.Len(t, iterations, 5)

	scenarioByID := map[oteltrace.SpanID]string{}
	for _, s := range scenarios {
		assert.Equal(t, runSpan.SpanContext().SpanID(), s.Parent().SpanID(), "scenarios are children of the run")
		assert.Equal(t, runSpan.SpanContext().TraceID(), s.SpanContext().TraceID())
		scenarioByID[s.SpanContext().SpanID()] = attrString(s.Attributes(), "test.scenario")
	}
	assert.ElementsMatch(t, []string{"first", "second"}, []string{
		scenarioByID[scenarios[0].SpanContext().SpanID()], scenarioByID[scenarios[1].SpanContext().SpanID()],
	})

	vuScenario := map[oteltrace.SpanID]string{}
	vusPerScenario := map[string]int{}
	for _, s := range vus {
		name, ok := scenarioByID[s.Parent().SpanID()]
		require.True(t, ok, "VU spans are children of a scenario span")
		assert.Equal(t, name, attrString(s.Attributes(), "test.scenario"))
		assert.Equal(t, runSpan.SpanContext().TraceID(), s.SpanContext().TraceID())
		vuScenario[s.SpanContext().SpanID()] = name
		vusPerScenario[name]++
	}
	assert.Equal(t, map[string]int{"first": 2, "second": 1}, vusPerScenario)

	itersPerScenario := map[string]int{}
	for _, s := range iterations {
		name, ok := vuScenario[s.Parent().SpanID()]
		require.True(t, ok, "iteration spans are children of a VU span")
		assert.Equal(t, name, attrString(s.Attributes(), "test.scenario"))
		assert.Equal(t, runSpan.SpanContext().TraceID(), s.SpanContext().TraceID())
		itersPerScenario[name]++
		hasCounter := false
		for _, kv := range s.Attributes() {
			hasCounter = hasCounter || string(kv.Key) == "k6.scenario.iteration_in_test"
		}
		assert.True(t, hasCounter, "executors provide the scenario iteration counters")
	}
	assert.Equal(t, map[string]int{"first": 4, "second": 1}, itersPerScenario)
}

func attrString(attrs []attribute.KeyValue, key string) string {
	for _, kv := range attrs {
		if string(kv.Key) == key {
			return kv.Value.AsString()
		}
	}
	return ""
}
