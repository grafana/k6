package executor

import (
	"context"
	"io"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/testutils"
	"go.k6.io/k6/lib/testutils/minirunner"
	"go.k6.io/k6/metrics"
)

func simpleRunner(vuFn func(context.Context, *lib.State) error) lib.Runner {
	return &minirunner.MiniRunner{
		Fn: func(ctx context.Context, state *lib.State, _ chan<- metrics.SampleContainer) error {
			return vuFn(ctx, state)
		},
	}
}

func getTestRunState(tb testing.TB, options lib.Options, runner lib.Runner) *lib.TestRunState {
	reg := metrics.NewRegistry()
	piState := &lib.TestPreInitState{
		Logger:         testutils.NewLogger(tb),
		RuntimeOptions: lib.RuntimeOptions{},
		Registry:       reg,
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(reg),
	}

	require.NoError(tb, runner.SetOptions(options))

	return &lib.TestRunState{
		TestPreInitState: piState,
		Options:          options,
		Runner:           runner,
		RunTags:          piState.Registry.RootTagSet().WithTagsFromMap(options.RunTags),
	}
}

func setupExecutor(t testing.TB, config lib.ExecutorConfig, es *lib.ExecutionState) (
	context.Context, context.CancelFunc, lib.Executor, *testutils.SimpleLogrusHook,
) {
	ctx, cancel := context.WithCancel(context.Background())
	engineOut := make(chan metrics.SampleContainer, 100) // TODO: return this for more complicated tests?

	logHook := testutils.NewLogHook(logrus.WarnLevel)
	testLog := logrus.New()
	testLog.AddHook(logHook)
	testLog.SetOutput(io.Discard)
	logEntry := logrus.NewEntry(testLog)

	initVUFunc := func(_ context.Context, logger *logrus.Entry) (lib.InitializedVU, error) {
		idl, idg := es.GetUniqueVUIdentifiers()
		return es.Test.Runner.NewVU(ctx, idl, idg, engineOut)
	}
	es.SetInitVUFunc(initVUFunc)

	maxPlannedVUs := lib.GetMaxPlannedVUs(config.GetExecutionRequirements(es.ExecutionTuple))
	initializeVUs(ctx, t, logEntry, es, maxPlannedVUs, initVUFunc)

	executor, err := config.NewExecutor(es, logEntry)
	require.NoError(t, err)

	err = executor.Init(ctx)
	require.NoError(t, err)
	return ctx, cancel, executor, logHook
}

func initializeVUs(
	ctx context.Context, t testing.TB, logEntry *logrus.Entry, es *lib.ExecutionState, number uint64, initVU lib.InitVUFunc,
) {
	// This is not how the local ExecutionScheduler initializes VUs, but should do the same job
	for i := uint64(0); i < number; i++ {
		// Not calling es.InitializeNewVU() here to avoid a double increment of initializedVUs,
		// which is done in es.AddInitializedVU().
		vu, err := initVU(ctx, logEntry)
		require.NoError(t, err)
		es.AddInitializedVU(vu)
	}
}

type executorTest struct {
	options lib.Options
	state   *lib.ExecutionState

	ctx      context.Context
	cancel   context.CancelFunc
	executor lib.Executor
	logHook  *testutils.SimpleLogrusHook
}

func setupExecutorTest(
	t testing.TB, segmentStr, sequenceStr string, extraOptions lib.Options,
	runner lib.Runner, config lib.ExecutorConfig,
) *executorTest {
	var err error
	var segment *lib.ExecutionSegment
	if segmentStr != "" {
		segment, err = lib.NewExecutionSegmentFromString(segmentStr)
		require.NoError(t, err)
	}

	var sequence lib.ExecutionSegmentSequence
	if sequenceStr != "" {
		sequence, err = lib.NewExecutionSegmentSequenceFromString(sequenceStr)
		require.NoError(t, err)
	}

	et, err := lib.NewExecutionTuple(segment, &sequence)
	require.NoError(t, err)

	options := lib.Options{
		ExecutionSegment:         segment,
		ExecutionSegmentSequence: &sequence,
	}.Apply(runner.GetOptions()).Apply(extraOptions)

	testRunState := getTestRunState(t, options, runner)

	execReqs := config.GetExecutionRequirements(et)
	es := lib.NewExecutionState(testRunState, et, lib.GetMaxPlannedVUs(execReqs), lib.GetMaxPossibleVUs(execReqs))
	ctx, cancel, executor, logHook := setupExecutor(t, config, es)

	return &executorTest{
		options:  options,
		state:    es,
		ctx:      ctx,
		cancel:   cancel,
		executor: executor,
		logHook:  logHook,
	}
}
