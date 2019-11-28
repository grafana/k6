package executor

import (
	"context"
	"io/ioutil"
	"math/rand"
	"testing"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/testutils"
	"github.com/loadimpact/k6/stats"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func setupExecutor(
	t *testing.T, config lib.ExecutorConfig,
	vuFn func(context.Context, chan<- stats.SampleContainer) error,
) (context.Context, context.CancelFunc, lib.Executor, *testutils.SimpleLogrusHook) {
	ctx, cancel := context.WithCancel(context.Background())
	logHook := &testutils.SimpleLogrusHook{HookedLevels: []logrus.Level{logrus.WarnLevel}}
	testLog := logrus.New()
	testLog.AddHook(logHook)
	testLog.SetOutput(ioutil.Discard)
	logEntry := logrus.NewEntry(testLog)
	es := lib.NewExecutionState(lib.Options{}, 10, 50)
	runner := lib.MiniRunner{
		Fn: vuFn,
	}

	es.SetInitVUFunc(func(_ context.Context, logger *logrus.Entry) (lib.VU, error) {
		return &lib.MiniRunnerVU{R: runner, ID: rand.Int63()}, nil
	})

	initializeVUs(ctx, t, logEntry, es, 10)

	executor, err := config.NewExecutor(es, logEntry)
	require.NoError(t, err)
	err = executor.Init(ctx)
	require.NoError(t, err)
	return ctx, cancel, executor, logHook
}

func initializeVUs(
	ctx context.Context, t testing.TB, logEntry *logrus.Entry, es *lib.ExecutionState, number int,
) {
	for i := 0; i < number; i++ {
		require.EqualValues(t, i, es.GetInitializedVUsCount())
		vu, err := es.InitializeNewVU(ctx, logEntry)
		require.NoError(t, err)
		require.EqualValues(t, i+1, es.GetInitializedVUsCount())
		es.ReturnVU(vu, false)
		require.EqualValues(t, 0, es.GetCurrentlyActiveVUsCount())
		require.EqualValues(t, i+1, es.GetInitializedVUsCount())
	}
}
