package trace_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	oteltrace "go.opentelemetry.io/otel/trace"

	"go.k6.io/k6/v2/internal/lib/testutils"
	k6trace "go.k6.io/k6/v2/internal/lib/trace"
	"go.k6.io/k6/v2/internal/lib/trace/tracetest"
)

func TestSpanHierarchy(t *testing.T) {
	t.Parallel()

	tp, rec := tracetest.NewProvider()
	tracer := k6trace.Tracer(tp)
	logger := testutils.NewLogger(t)

	runCtx, runSpan := k6trace.StartTestRun(context.Background(), tracer, logger)
	scCtx, scSpan := k6trace.StartScenario(runCtx, tracer, k6trace.ScenarioInfo{Name: "sc", Executor: "constant-vus"})
	vuCtx, vuSpan := k6trace.StartVU(scCtx, tracer, k6trace.VUInfo{ID: 2, IDGlobal: 3, Scenario: "sc"})
	_, itSpan := k6trace.StartIteration(vuCtx, tracer, k6trace.IterationInfo{
		Number: 4, VUID: 2, VUIDGlobal: 3, VUIterationInScenario: 5, Scenario: "sc",
		ScenarioIterationInInstance: 6, ScenarioIterationInTest: 7, HasScenarioIterationNumbers: true,
	})
	for _, s := range []oteltrace.Span{itSpan, vuSpan, scSpan, runSpan} {
		k6trace.EndSpan(s, nil)
	}

	byName := map[string]oteltrace.SpanContext{}
	parents := map[string]oteltrace.SpanID{}
	attrs := map[string]map[string]attribute.Value{}
	for _, s := range rec.Ended() {
		byName[s.Name()] = s.SpanContext()
		parents[s.Name()] = s.Parent().SpanID()
		attrs[s.Name()] = map[string]attribute.Value{}
		for _, kv := range s.Attributes() {
			attrs[s.Name()][string(kv.Key)] = kv.Value
		}
	}
	require.Len(t, byName, 4)

	assert.False(t, rec.ByName(k6trace.SpanRun)[0].Parent().IsValid(), "the run span is a root")
	assert.Equal(t, byName[k6trace.SpanRun].SpanID(), parents[k6trace.SpanScenario])
	assert.Equal(t, byName[k6trace.SpanScenario].SpanID(), parents[k6trace.SpanVU])
	assert.Equal(t, byName[k6trace.SpanVU].SpanID(), parents[k6trace.SpanIteration])
	for name, sc := range byName {
		assert.Equal(t, byName[k6trace.SpanRun].TraceID(), sc.TraceID(), "%s must be in the run trace", name)
	}

	assert.Equal(t, "sc", attrs[k6trace.SpanScenario]["test.scenario"].AsString())
	assert.Equal(t, "constant-vus", attrs[k6trace.SpanScenario]["k6.scenario.executor"].AsString())
	assert.Equal(t, int64(2), attrs[k6trace.SpanVU]["test.vu"].AsInt64())
	assert.Equal(t, int64(2), attrs[k6trace.SpanVU]["k6.vu.id_in_instance"].AsInt64())
	assert.Equal(t, int64(3), attrs[k6trace.SpanVU]["k6.vu.id_in_test"].AsInt64())
	assert.Equal(t, int64(4), attrs[k6trace.SpanIteration]["test.iteration.number"].AsInt64())
	assert.Equal(t, int64(2), attrs[k6trace.SpanIteration]["test.vu"].AsInt64())
	assert.Equal(t, "sc", attrs[k6trace.SpanIteration]["test.scenario"].AsString())
	assert.Equal(t, int64(2), attrs[k6trace.SpanIteration]["k6.vu.id_in_instance"].AsInt64())
	assert.Equal(t, int64(3), attrs[k6trace.SpanIteration]["k6.vu.id_in_test"].AsInt64())
	assert.Equal(t, int64(5), attrs[k6trace.SpanIteration]["k6.vu.iteration_in_scenario"].AsInt64())
	assert.Equal(t, int64(6), attrs[k6trace.SpanIteration]["k6.scenario.iteration_in_instance"].AsInt64())
	assert.Equal(t, int64(7), attrs[k6trace.SpanIteration]["k6.scenario.iteration_in_test"].AsInt64())
}

func TestStartTestRunStartsNewTraceWithoutRemoteParent(t *testing.T) {
	t.Parallel()

	tp, _ := tracetest.NewProvider()
	logger := testutils.NewLogger(t)

	// A local span in the context is not an input parent, the run is still a new trace.
	outerCtx, outer := tp.Tracer("test").Start(context.Background(), "outer")
	defer outer.End()

	_, run := k6trace.StartTestRun(outerCtx, k6trace.Tracer(tp), logger)
	defer run.End()

	assert.NotEqual(t, outer.SpanContext().TraceID(), run.SpanContext().TraceID())
}

func TestStartTestRunContinuesRemoteParent(t *testing.T) {
	t.Parallel()

	tp, rec := tracetest.NewProvider()
	tracer := k6trace.Tracer(tp)
	logger := testutils.NewLogger(t)

	parentCtx, err := k6trace.ContextWithRemoteParent(
		context.Background(),
		"00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
		propagation.TraceContext{},
	)
	require.NoError(t, err)

	runCtx, run := k6trace.StartTestRun(parentCtx, tracer, logger)
	_, scenario := k6trace.StartScenario(runCtx, tracer, k6trace.ScenarioInfo{Name: "sc"})
	k6trace.EndSpan(scenario, nil)
	k6trace.EndSpan(run, nil)

	runSpan := rec.ByName(k6trace.SpanRun)[0]
	assert.Equal(t, "4bf92f3577b34da6a3ce929d0e0e4736", runSpan.SpanContext().TraceID().String())
	assert.Equal(t, "00f067aa0ba902b7", runSpan.Parent().SpanID().String())
	assert.True(t, runSpan.Parent().IsRemote())
	assert.True(t, runSpan.SpanContext().IsSampled())

	scenarioSpan := rec.ByName(k6trace.SpanScenario)[0]
	assert.Equal(t, runSpan.SpanContext().TraceID(), scenarioSpan.SpanContext().TraceID())
}

func TestEndSpanRecordsError(t *testing.T) {
	t.Parallel()

	tp, rec := tracetest.NewProvider()
	tracer := k6trace.Tracer(tp)
	_, okSpan := k6trace.StartScenario(context.Background(), tracer, k6trace.ScenarioInfo{Name: "ok"})
	k6trace.EndSpan(okSpan, nil)
	_, failSpan := k6trace.StartScenario(context.Background(), tracer, k6trace.ScenarioInfo{Name: "fail"})
	k6trace.EndSpan(failSpan, errors.New("boom"))

	spans := rec.Ended()
	require.Len(t, spans, 2)
	assert.Equal(t, codes.Unset, spans[0].Status().Code)
	assert.Equal(t, codes.Error, spans[1].Status().Code)
	assert.Equal(t, "boom", spans[1].Status().Description)
	require.Len(t, spans[1].Events(), 1)
	assert.Equal(t, "exception", spans[1].Events()[0].Name)
}

func TestTracerIsNilWhenTracingIsDisabled(t *testing.T) {
	t.Parallel()

	var nilProvider *k6trace.TracerProvider
	assert.Nil(t, k6trace.Tracer(nil))
	assert.Nil(t, k6trace.Tracer(nilProvider))
	assert.Nil(t, k6trace.Tracer(k6trace.NewNoopTracerProvider()))

	tp, _ := tracetest.NewProvider()
	assert.NotNil(t, k6trace.Tracer(tp))
}

func TestEndIterationDoesNotRecordInterruptionsAsErrors(t *testing.T) {
	t.Parallel()

	tp, rec := tracetest.NewProvider()
	tracer := k6trace.Tracer(tp)
	_, failed := k6trace.StartIteration(context.Background(), tracer, k6trace.IterationInfo{Number: 0})
	k6trace.EndIteration(failed, errors.New("boom"), false)
	_, interrupted := k6trace.StartIteration(context.Background(), tracer, k6trace.IterationInfo{Number: 1})
	k6trace.EndIteration(interrupted, context.Canceled, true)

	spans := rec.Ended()
	require.Len(t, spans, 2)
	assert.Equal(t, codes.Error, spans[0].Status().Code)
	assert.Equal(t, codes.Unset, spans[1].Status().Code)
	assert.Empty(t, spans[1].Events())
	assert.Contains(t, spans[1].Attributes(), attribute.Bool("k6.iteration.interrupted", true))
}

func TestIterationWithoutScenarioIterationNumbers(t *testing.T) {
	t.Parallel()

	tp, rec := tracetest.NewProvider()
	_, span := k6trace.StartIteration(context.Background(), k6trace.Tracer(tp), k6trace.IterationInfo{Number: 1})
	span.End()

	for _, kv := range rec.Ended()[0].Attributes() {
		assert.NotContains(t, []string{
			"k6.scenario.iteration_in_instance", "k6.scenario.iteration_in_test",
		}, string(kv.Key), "the scenario counters are left out when the executor doesn't provide them")
	}
}
