package cmd

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/cmd/state"
	"go.k6.io/k6/v2/internal/build"
	"go.k6.io/k6/v2/internal/execution"
	"go.k6.io/k6/v2/internal/execution/local"
	"go.k6.io/k6/v2/internal/lib/testutils"
	"go.k6.io/k6/v2/internal/usage"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/lib/executor"
	"go.k6.io/k6/v2/lib/fsext"
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

func TestInstallationID(t *testing.T) {
	t.Parallel()

	const storedID = "123e4567-e89b-42d3-a456-426614174000"

	tests := []struct {
		name   string
		stored string
		want   string
	}{
		{name: "creates a UUID when none is stored"},
		{name: "reuses the stored UUID", stored: storedID, want: storedID},
		{name: "replaces an invalid stored value", stored: "not-a-uuid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			configDir := t.TempDir()
			path := filepath.Join(configDir, "k6", "installation-id")
			gs := &state.GlobalState{FS: fsext.NewOsFs(), UserOSConfigDir: configDir}
			if tt.stored != "" {
				require.NoError(t, gs.FS.MkdirAll(filepath.Dir(path), configDirMode))
				require.NoError(t, fsext.WriteFile(gs.FS, path, []byte(tt.stored), configFileMode))
			}

			got, err := installationID(gs)

			require.NoError(t, err)
			require.NoError(t, uuid.Validate(got))
			if tt.want != "" {
				require.Equal(t, tt.want, got)
			}
			saved, err := fsext.ReadFile(gs.FS, path)
			require.NoError(t, err)
			require.Equal(t, got, string(saved))
		})
	}
}
