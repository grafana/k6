package execution

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/internal/execution/local"
	"go.k6.io/k6/v2/internal/lib/testutils"
	"go.k6.io/k6/v2/internal/lib/testutils/minirunner"
	"go.k6.io/k6/v2/internal/ui/pb"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/metrics"
)

func getBogusTestRunState(tb testing.TB) *lib.TestRunState {
	reg := metrics.NewRegistry()
	piState := &lib.TestPreInitState{
		Logger:         testutils.NewLogger(tb),
		RuntimeOptions: lib.RuntimeOptions{},
		Registry:       reg,
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(reg),
	}

	return &lib.TestRunState{
		TestPreInitState: piState,
		Options:          lib.Options{},
		Runner:           &minirunner.MiniRunner{},
		RunTags:          piState.Registry.RootTagSet(),
	}
}

// Just a lib.PausableExecutor implementation that can return an error
type pausableExecutor struct {
	lib.Executor
	err error
}

func (p pausableExecutor) SetPaused(bool) error {
	return p.err
}

// constantVUsExecutor is not pausable, like the constant-vus executor.
type constantVUsExecutor struct {
	name string
}

func (e constantVUsExecutor) GetConfig() lib.ExecutorConfig { return constantVUsConfig{name: e.name} }
func (constantVUsExecutor) GetProgress() *pb.ProgressBar    { return nil }
func (constantVUsExecutor) GetLogger() *logrus.Entry        { return nil }
func (constantVUsExecutor) Init(context.Context) error      { return nil }
func (constantVUsExecutor) Run(context.Context, chan<- metrics.SampleContainer) error {
	return nil
}

type constantVUsConfig struct {
	name string
}

func (c constantVUsConfig) Validate() []error            { return nil }
func (c constantVUsConfig) GetName() string              { return c.name }
func (constantVUsConfig) GetType() string                { return "constant-vus" }
func (constantVUsConfig) GetStartTime() time.Duration    { return 0 }
func (constantVUsConfig) GetGracefulStop() time.Duration { return 0 }
func (constantVUsConfig) IsDistributable() bool          { return true }
func (constantVUsConfig) GetEnv() map[string]string      { return nil }
func (constantVUsConfig) GetExec() string                { return "" }
func (constantVUsConfig) GetTags() map[string]string     { return nil }
func (constantVUsConfig) GetExecutionRequirements(*lib.ExecutionTuple) []lib.ExecutionStep {
	return nil
}
func (constantVUsConfig) GetScenarioOptions() *lib.ScenarioOptions { return nil }
func (constantVUsConfig) GetDescription(*lib.ExecutionTuple) string {
	return ""
}
func (constantVUsConfig) NewExecutor(*lib.ExecutionState, *logrus.Entry) (lib.Executor, error) {
	return nil, nil
}
func (constantVUsConfig) HasWork(*lib.ExecutionTuple) bool { return true }

func TestSetPaused(t *testing.T) {
	t.Parallel()
	t.Run("second pause is a no-op", func(t *testing.T) {
		t.Parallel()
		testRunState := getBogusTestRunState(t)
		sched, err := NewScheduler(testRunState, local.NewController())
		require.NoError(t, err)
		sched.executors = []lib.Executor{pausableExecutor{err: nil}}

		require.NoError(t, sched.SetPaused(true))
		require.NoError(t, sched.SetPaused(true))
		require.True(t, sched.state.IsPaused())
	})

	t.Run("unpause when not paused is a no-op", func(t *testing.T) {
		t.Parallel()
		testRunState := getBogusTestRunState(t)
		sched, err := NewScheduler(testRunState, local.NewController())
		require.NoError(t, err)
		sched.executors = []lib.Executor{pausableExecutor{err: nil}}
		require.NoError(t, sched.SetPaused(false))
		require.False(t, sched.state.IsPaused())
	})

	t.Run("second unpause is a no-op", func(t *testing.T) {
		t.Parallel()
		testRunState := getBogusTestRunState(t)
		sched, err := NewScheduler(testRunState, local.NewController())
		require.NoError(t, err)
		sched.executors = []lib.Executor{pausableExecutor{err: nil}}
		require.NoError(t, sched.SetPaused(true))
		require.NoError(t, sched.SetPaused(false))
		require.NoError(t, sched.SetPaused(false))
		require.False(t, sched.state.IsPaused())
	})

	t.Run("resume retry after start does not ask the executor", func(t *testing.T) {
		t.Parallel()
		testRunState := getBogusTestRunState(t)
		sched, err := NewScheduler(testRunState, local.NewController())
		require.NoError(t, err)
		ex := constantVUsExecutor{name: "default"}
		sched.executors = []lib.Executor{ex}

		require.NoError(t, sched.state.Pause())
		require.NoError(t, sched.SetPaused(false))
		sched.state.MarkStarted()
		require.NoError(t, sched.SetPaused(false))
		require.False(t, sched.state.IsPaused())

		err = sched.SetPaused(true)
		require.Error(t, err)
		require.Contains(t, err.Error(), "doesn't support pause and resume operations after its start")
	})

	t.Run("an error on pausing is propagated", func(t *testing.T) {
		t.Parallel()
		testRunState := getBogusTestRunState(t)
		sched, err := NewScheduler(testRunState, local.NewController())
		require.NoError(t, err)
		expectedErr := errors.New("testing pausable executor error")
		sched.executors = []lib.Executor{pausableExecutor{err: expectedErr}}
		err = sched.SetPaused(true)
		require.Error(t, err)
		require.Equal(t, expectedErr, err)
	})
}
