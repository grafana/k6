package tests

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/core/local"
	"go.k6.io/k6/js"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/js/modules/k6/experimental/tracing"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/executor"
	"go.k6.io/k6/lib/testutils"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/loader"
	"go.k6.io/k6/metrics"
	"gopkg.in/guregu/null.v3"
)

func init() {
	modules.Register("k6/x/tracing", tracing.New())
}

func TestTracingInstrumentHTTP(t *testing.T) {
	t.Parallel()

	testScript := []byte(`
		tracing.instrumentHTTP({propagator: "w3c"});
	`)

	testHandle := func(ctx context.Context, r lib.Runner, err error, logHook *testutils.SimpleLogrusHook) {
		require.NoError(t, err)
	}

	tracingTest(t, testScript, testHandle)
}

func tracingTest(t *testing.T, script []byte, testHandle func(context.Context, lib.Runner, error, *testutils.SimpleLogrusHook)) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	logHook := &testutils.SimpleLogrusHook{HookedLevels: []logrus.Level{logrus.InfoLevel, logrus.WarnLevel, logrus.ErrorLevel}}
	logger.AddHook(logHook)

	registry := metrics.NewRegistry()
	preInitState := &lib.TestPreInitState{
		Logger:         logger,
		Registry:       registry,
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(registry),
	}

	script = []byte("import tracing from 'k6/x/tracing';\n" + string(script))
	runner, err := js.New(preInitState, &loader.SourceData{Data: script}, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	newOpts, err := executor.DeriveScenariosFromShortcuts(lib.Options{
		MetricSamplesBufferSize: null.NewInt(200, false),
		TeardownTimeout:         types.NullDurationFrom(time.Second),
		SetupTimeout:            types.NullDurationFrom(time.Second),
	}.Apply(runner.GetOptions()), nil)
	require.NoError(t, err)
	require.Empty(t, newOpts.Validate())
	require.NoError(t, runner.SetOptions(newOpts))

	testState := &lib.TestRunState{
		TestPreInitState: preInitState,
		Options:          newOpts,
		Runner:           runner,
		RunTags:          preInitState.Registry.RootTagSet().WithTagsFromMap(newOpts.RunTags),
	}

	execScheduler, err := local.NewExecutionScheduler(testState)
	require.NoError(t, err)

	samples := make(chan metrics.SampleContainer, newOpts.MetricSamplesBufferSize.Int64)
	go func() {
		for {
			select {
			case <-samples:
			case <-ctx.Done():
				return
			}
		}
	}()

	require.NoError(t, execScheduler.Init(ctx, samples))

	errCh := make(chan error, 1)
	go func() { errCh <- execScheduler.Run(ctx, ctx, samples) }()

	select {
	case err := <-errCh:
		testHandle(ctx, runner, err, logHook)
	case <-time.After(10 * time.Second):
		t.Fatal("timed out")
	}
}
