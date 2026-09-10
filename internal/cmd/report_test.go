package cmd

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/internal/build"
	"go.k6.io/k6/v2/internal/execution"
	"go.k6.io/k6/v2/internal/execution/local"
	"go.k6.io/k6/v2/internal/lib/testutils"
	"go.k6.io/k6/v2/internal/usage"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/lib/executor"
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

	// A nil func would also work while no case records an extension, but this
	// stays fail-closed if one ever does: filtering against a nil catalog
	// drops the extensions instead of panicking.
	noCatalog := func() map[string]struct{} { return nil }

	t.Run("default (no env)", func(t *testing.T) {
		t.Parallel()

		s, err := initSchedulerWithEnv(func(_ string) (val string, ok bool) {
			return "", false
		})
		require.NoError(t, err)

		s.GetState().ModInitializedVUsCount(6)
		s.GetState().AddFullIterations(uint64(opts.Iterations.Int64))
		s.GetState().MarkStarted()
		time.Sleep(10 * time.Millisecond)
		s.GetState().MarkEnded()

		m := createReport(usage.New(), s, noCatalog)

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

		m := createReport(usage.New(), s, noCatalog)

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

		m := createReport(usage.New(), s, noCatalog)

		assert.Equal(t, build.Version, m["k6_version"])
		assert.EqualValues(t, map[string]int{"shared-iterations": 1}, m["executors"])
		assert.EqualValues(t, 0, m["vus_max"])
		assert.EqualValues(t, 0, m["iterations"])
		assert.Equal(t, "0s", m["duration"])
		assert.EqualValues(t, true, m["is_ci"])
	})
}

//nolint:paralleltest // swaps the package-global build.BuildOrigin, so the subtests must run serially.
func TestCreateReportBuildOrigin(t *testing.T) {
	logger := testutils.NewLogger(t)
	opts, err := executor.DeriveScenariosFromShortcuts(lib.Options{
		VUs:        null.IntFrom(1),
		Iterations: null.IntFrom(1),
	}, logger)
	require.NoError(t, err)

	newScheduler := func() *execution.Scheduler {
		s, err := execution.NewScheduler(&lib.TestRunState{
			TestPreInitState: &lib.TestPreInitState{
				Logger:    logger,
				LookupEnv: func(string) (string, bool) { return "", false },
			},
			Options: opts,
		}, local.NewController())
		require.NoError(t, err)
		return s
	}

	noCatalog := func() map[string]struct{} { return nil }

	t.Run("empty origin omits the field", func(t *testing.T) {
		original := build.BuildOrigin
		build.BuildOrigin = ""
		t.Cleanup(func() { build.BuildOrigin = original })

		m := createReport(usage.New(), newScheduler(), noCatalog)

		_, ok := m["build_origin"]
		assert.False(t, ok)
	})

	t.Run("recorded origin appears in the report", func(t *testing.T) {
		original := build.BuildOrigin
		build.BuildOrigin = "release"
		t.Cleanup(func() { build.BuildOrigin = original })

		m := createReport(usage.New(), newScheduler(), noCatalog)

		assert.Equal(t, "release", m["build_origin"])
	})
}
