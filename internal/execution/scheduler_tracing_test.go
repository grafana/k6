package execution_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/v2/internal/execution"
	"go.k6.io/k6/v2/internal/execution/local"
	"go.k6.io/k6/v2/internal/lib/testutils/minirunner"
	k6trace "go.k6.io/k6/v2/internal/lib/trace"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/lib/executor"
	"go.k6.io/k6/v2/metrics"
)

type tracingRecordingExporter struct {
	spans []sdktrace.ReadOnlySpan
}

func (e *tracingRecordingExporter) ExportSpans(_ context.Context, spans []sdktrace.ReadOnlySpan) error {
	e.spans = append(e.spans, spans...)
	return nil
}

func (*tracingRecordingExporter) Shutdown(context.Context) error { return nil }

func (e *tracingRecordingExporter) findByName(name string) sdktrace.ReadOnlySpan {
	for _, s := range e.spans {
		if s.Name() == name {
			return s
		}
	}
	return nil
}

// runSchedulerWithTracing runs the scheduler's default scenario (1 VU, 1
// iteration, via MiniRunner) under a manually-started "k6.run" span, and
// returns the exported spans plus that run span. Note MiniRunner has its own
// independent Activate/RunOnce mock that never calls into internal/js, so it
// can only exercise the scenario span created in runExecutor (scheduler.go)
// -- VU and iteration span nesting is covered separately in internal/js,
// where the real VU.Activate/ActiveVU.RunOnce implementation lives.
func runSchedulerWithTracing(t *testing.T, tracesSplit bool) (*tracingRecordingExporter, sdktrace.ReadOnlySpan) {
	t.Helper()

	exporter := &tracingRecordingExporter{}
	sdkProvider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracerProvider := &k6trace.TracerProvider{TracerProvider: sdkProvider}

	piState := getTestPreInitState(t)
	piState.TracerProvider = tracerProvider
	piState.TracesSplit = tracesSplit

	runner := &minirunner.MiniRunner{}
	newOpts, err := executor.DeriveScenariosFromShortcuts(lib.Options{
		MetricSamplesBufferSize: null.NewInt(200, false),
	}.Apply(runner.GetOptions()), nil)
	require.NoError(t, err)

	testRunState := getTestRunState(t, piState, newOpts, runner)

	execScheduler, err := execution.NewScheduler(testRunState, local.NewController())
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	samples := make(chan metrics.SampleContainer, 200)
	go func() {
		for {
			select {
			case <-samples:
			case <-ctx.Done():
				return
			}
		}
	}()

	stopEmission, err := execScheduler.Init(ctx, samples)
	require.NoError(t, err)
	t.Cleanup(func() {
		stopEmission()
		close(samples)
	})

	runCtx, runSpan := sdkProvider.Tracer("test").Start(ctx, "k6.run")
	require.NoError(t, execScheduler.Run(ctx, runCtx, samples))
	runSpan.End()

	return exporter, exporter.findByName("k6.run")
}

// TestSchedulerScenarioSpanAlwaysNested verifies the k6.scenario span is
// always created as a real child of k6.run, regardless of --traces-split:
// that flag only affects the iteration boundary (see internal/js/runner_test.go),
// not whether scenario/VU spans exist.
func TestSchedulerScenarioSpanAlwaysNested(t *testing.T) {
	t.Parallel()

	for _, tracesSplit := range []bool{false, true} {
		exporter, runSpan := runSchedulerWithTracing(t, tracesSplit)
		require.NotNil(t, runSpan, "expected a k6.run span to have been exported")

		scenario := exporter.findByName("k6.scenario")
		require.NotNil(t, scenario, "expected a k6.scenario span to have been exported")
		require.True(t, scenario.Parent().IsValid())
		require.Equal(t, runSpan.SpanContext().SpanID(), scenario.Parent().SpanID())
		require.Equal(t, runSpan.SpanContext().TraceID(), scenario.SpanContext().TraceID())
	}
}
