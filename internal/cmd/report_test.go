package cmd

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/internal/build"
	"go.k6.io/k6/internal/execution"
	"go.k6.io/k6/internal/execution/local"
	"go.k6.io/k6/internal/lib/testutils"
	"go.k6.io/k6/internal/usage"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/executor"
	"gopkg.in/guregu/null.v3"
)

func TestCreateReport(t *testing.T) {
	t.Parallel()

	logger := testutils.NewLogger(t)
	opts, err := executor.DeriveScenariosFromShortcuts(lib.Options{
		VUs:        null.IntFrom(10),
		Iterations: null.IntFrom(170),
	}, logger)
	require.NoError(t, err)

	initSchedulerWithEnv := func(lookupEnv func(string) (string, bool)) (*execution.Scheduler, error) {
		return execution.NewScheduler(&lib.TestRunState{
			TestPreInitState: &lib.TestPreInitState{
				Logger:    logger,
				LookupEnv: lookupEnv,
			},
			Options: opts,
		}, local.NewController())
	}

	t.Run("default (no env)", func(t *testing.T) {
		t.Parallel()

		s, err := initSchedulerWithEnv(func(_ string) (val string, ok bool) {
			return "", false
		})
		require.NoError(t, err)

		s.GetState().ModInitializedVUsCount(6)
		s.GetState().AddFullIterations(uint64(opts.Iterations.Int64)) //nolint:gosec
		s.GetState().MarkStarted()
		time.Sleep(10 * time.Millisecond)
		s.GetState().MarkEnded()

		m := createReport(usage.New(), s)
		require.NoError(t, err)

		assert.Equal(t, build.Version, m["k6_version"])
		assert.EqualValues(t, map[string]int{"shared-iterations": 1}, m["executors"])
		assert.EqualValues(t, 6, m["vus_max"])
		assert.EqualValues(t, 170, m["iterations"])
		assert.NotEqual(t, "0s", m["duration"])
		assert.EqualValues(t, false, m["is_ci"])
	})

	t.Run("CI=false", func(t *testing.T) {
		t.Parallel()

		s, err := initSchedulerWithEnv(func(envVar string) (val string, ok bool) {
			if envVar == "CI" {
				return "false", true
			}
			return "", false
		})
		require.NoError(t, err)

		m := createReport(usage.New(), s)
		require.NoError(t, err)

		assert.Equal(t, build.Version, m["k6_version"])
		assert.EqualValues(t, map[string]int{"shared-iterations": 1}, m["executors"])
		assert.EqualValues(t, 0, m["vus_max"])
		assert.EqualValues(t, 0, m["iterations"])
		assert.Equal(t, "0s", m["duration"])
		assert.EqualValues(t, false, m["is_ci"])
	})

	t.Run("CI=true", func(t *testing.T) {
		t.Parallel()

		s, err := initSchedulerWithEnv(func(envVar string) (val string, ok bool) {
			if envVar == "CI" {
				return "true", true
			}
			return "", false
		})
		require.NoError(t, err)

		m := createReport(usage.New(), s)
		require.NoError(t, err)

		assert.Equal(t, build.Version, m["k6_version"])
		assert.EqualValues(t, map[string]int{"shared-iterations": 1}, m["executors"])
		assert.EqualValues(t, 0, m["vus_max"])
		assert.EqualValues(t, 0, m["iterations"])
		assert.Equal(t, "0s", m["duration"])
		assert.EqualValues(t, true, m["is_ci"])
	})
}
