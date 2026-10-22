package trace_test

import (
	"context"
	"errors"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"

	"go.k6.io/k6/v2/internal/features"
	"go.k6.io/k6/v2/internal/lib/testutils"
	k6trace "go.k6.io/k6/v2/internal/lib/trace"
	"go.k6.io/k6/v2/internal/lib/trace/tracetest"
)

// nativeTracing returns the feature flags enabling the k6 execution hierarchy spans.
func nativeTracing() *features.Flags {
	return &features.Flags{NativeTracing: true}
}

func TestSpanHierarchy(t *testing.T) {
	t.Parallel()

	tp, rec := tracetest.NewProvider()
	tracer := k6trace.Tracer(tp, nativeTracing())
	logger := testutils.NewLogger(t)

	runCtx, runSpan := k6trace.StartTestRun(context.Background(), tracer, logger)
	scCtx, scSpan := k6trace.StartScenario(runCtx, tracer, k6trace.ScenarioInfo{Name: "sc", Executor: "constant-vus"})
	vuCtx, vuSpan := k6trace.StartVU(scCtx, tracer, k6trace.VUInfo{ID: 2, IDGlobal: 3, Scenario: "sc"})
	_, itSpan := k6trace.StartIteration(vuCtx, tracer, k6trace.IterationInfo{
		Number: 4, VUID: 2, VUIDGlobal: 3, VUIterationInScenario: 5, Scenario: "sc",
		ScenarioIterationInInstance: 6, ScenarioIterationInTest: 7, HasScenarioIterationNumbers: true,
	})
	for _, s := range []oteltrace.Span{itSpan, vuSpan, scSpan} {
		k6trace.EndSpan(s, nil)
	}
	runSpan.End()

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

func TestStartTestRunStartsNewTrace(t *testing.T) {
	t.Parallel()

	tp, _ := tracetest.NewProvider()
	logger := testutils.NewLogger(t)

	// A span already in the context is not a parent, the run is always a new trace.
	outerCtx, outer := tp.Tracer("test").Start(context.Background(), "outer")
	defer outer.End()

	_, run := k6trace.StartTestRun(outerCtx, k6trace.Tracer(tp, nativeTracing()), logger)
	defer run.End()

	assert.NotEqual(t, outer.SpanContext().TraceID(), run.SpanContext().TraceID())
}

func TestStartFunctionsWithoutTracer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		start func(context.Context, logrus.FieldLogger) (context.Context, oteltrace.Span)
	}{
		{name: "run", start: func(ctx context.Context, l logrus.FieldLogger) (context.Context, oteltrace.Span) {
			return k6trace.StartTestRun(ctx, nil, l)
		}},
		{name: "scenario", start: func(ctx context.Context, _ logrus.FieldLogger) (context.Context, oteltrace.Span) {
			return k6trace.StartScenario(ctx, nil, k6trace.ScenarioInfo{Name: "sc"})
		}},
		{name: "vu", start: func(ctx context.Context, _ logrus.FieldLogger) (context.Context, oteltrace.Span) {
			return k6trace.StartVU(ctx, nil, k6trace.VUInfo{ID: 1})
		}},
		{name: "iteration", start: func(ctx context.Context, _ logrus.FieldLogger) (context.Context, oteltrace.Span) {
			return k6trace.StartIteration(ctx, nil, k6trace.IterationInfo{Number: 1})
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			logger, hook := testutils.NewLoggerWithHook(t)
			ctx := context.Background()
			gotCtx, span := tc.start(ctx, logger)
			k6trace.EndSpan(span, errors.New("boom"))

			assert.Equal(t, ctx, gotCtx, "the context must be unchanged when tracing is disabled")
			assert.False(t, span.SpanContext().IsValid())
			assert.Empty(t, hook.Drain(), "nothing must be logged when tracing is disabled")
		})
	}
}

func TestEndSpanStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		end             func(oteltrace.Span)
		wantCode        codes.Code
		wantInterrupted bool
	}{
		{name: "no error", end: func(s oteltrace.Span) { k6trace.EndSpan(s, nil) }, wantCode: codes.Unset},
		{name: "error", end: func(s oteltrace.Span) { k6trace.EndSpan(s, errors.New("boom")) }, wantCode: codes.Error},
		{
			name:     "failed iteration",
			end:      func(s oteltrace.Span) { k6trace.EndIteration(s, errors.New("boom"), false) },
			wantCode: codes.Error,
		},
		{
			name:            "interrupted iteration",
			end:             func(s oteltrace.Span) { k6trace.EndIteration(s, context.Canceled, true) },
			wantCode:        codes.Unset,
			wantInterrupted: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tp, rec := tracetest.NewProvider()
			_, span := k6trace.StartIteration(context.Background(), k6trace.Tracer(tp, nativeTracing()), k6trace.IterationInfo{})
			tc.end(span)

			spans := rec.Ended()
			require.Len(t, spans, 1)
			assert.Equal(t, tc.wantCode, spans[0].Status().Code)
			if tc.wantCode == codes.Error {
				assert.Equal(t, "boom", spans[0].Status().Description)
				require.Len(t, spans[0].Events(), 1)
				assert.Equal(t, "exception", spans[0].Events()[0].Name)
			} else {
				assert.Empty(t, spans[0].Events())
			}
			interrupted := attribute.Bool("k6.iteration.interrupted", true)
			if tc.wantInterrupted {
				assert.Contains(t, spans[0].Attributes(), interrupted)
			} else {
				assert.NotContains(t, spans[0].Attributes(), interrupted)
			}
		})
	}
}

func TestTracer(t *testing.T) {
	t.Parallel()

	tp, _ := tracetest.NewProvider()
	var nilProvider *k6trace.TracerProvider
	tests := []struct {
		name    string
		tp      *k6trace.TracerProvider
		flags   *features.Flags
		wantNil bool
	}{
		{name: "enabled", tp: tp, flags: nativeTracing()},
		{name: "nil provider", tp: nilProvider, flags: nativeTracing(), wantNil: true},
		{name: "noop provider", tp: k6trace.NewNoopTracerProvider(), flags: nativeTracing(), wantNil: true},
		{name: "nil flags", tp: tp, flags: nil, wantNil: true},
		{name: "feature flag disabled", tp: tp, flags: &features.Flags{}, wantNil: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.wantNil, k6trace.Tracer(tc.tp, tc.flags) == nil)
		})
	}
}

func TestMarkIterationFailed(t *testing.T) {
	t.Parallel()

	tp, rec := tracetest.NewProvider()
	tracer := k6trace.Tracer(tp, nativeTracing())

	// Outside of an iteration, e.g. in setup, it does nothing.
	k6trace.MarkIterationFailed(context.Background(), "failed")

	ctx, iteration := k6trace.StartIteration(context.Background(), tracer, k6trace.IterationInfo{Number: 0})
	// The iteration span is found even when a module made a child span the active one.
	childCtx, child := tracer.Start(ctx, "child")
	k6trace.MarkIterationFailed(childCtx, "test marked as failed: oops")
	child.End()
	k6trace.EndIteration(iteration, nil, false)

	spans := rec.Ended()
	require.Len(t, spans, 2)
	assert.Equal(t, codes.Unset, spans[0].Status().Code)
	assert.Equal(t, codes.Error, spans[1].Status().Code)
	assert.Equal(t, "test marked as failed: oops", spans[1].Status().Description)
}
