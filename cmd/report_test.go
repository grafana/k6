package cmd

import (
	"testing"
	"time"

	"github.com/liuxd6825/k6server/execution"
	"github.com/liuxd6825/k6server/execution/local"
	"github.com/liuxd6825/k6server/lib"
	"github.com/liuxd6825/k6server/lib/consts"
	"github.com/liuxd6825/k6server/lib/executor"
	"github.com/liuxd6825/k6server/lib/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"
)

func TestCreateReport(t *testing.T) {
	t.Parallel()
	importedModules := []string{
		"k6/http",
		"my-custom-module",
		"k6/experimental/webcrypto",
		"file:custom-from-file-system",
		"k6",
		"k6/x/custom-extension",
	}

	outputs := []string{
		"json",
		"xk6-output-custom-example",
	}

	logger := testutils.NewLogger(t)
	opts, err := executor.DeriveScenariosFromShortcuts(lib.Options{
		VUs:        null.IntFrom(10),
		Iterations: null.IntFrom(170),
	}, logger)
	require.NoError(t, err)

	s, err := execution.NewScheduler(&lib.TestRunState{
		TestPreInitState: &lib.TestPreInitState{
			Logger: logger,
		},
		Options: opts,
	}, local.NewController())
	require.NoError(t, err)
	s.GetState().ModInitializedVUsCount(6)
	s.GetState().AddFullIterations(uint64(opts.Iterations.Int64))
	s.GetState().MarkStarted()
	time.Sleep(10 * time.Millisecond)
	s.GetState().MarkEnded()

	r := createReport(s, importedModules, outputs)
	assert.Equal(t, consts.Version, r.Version)
	assert.Equal(t, map[string]int{"shared-iterations": 1}, r.Executors)
	assert.Equal(t, 6, int(r.VUsMax))
	assert.Equal(t, 170, int(r.Iterations))
	assert.NotEqual(t, "0s", r.Duration)
	assert.ElementsMatch(t, []string{"k6", "k6/http", "k6/experimental/webcrypto"}, r.Modules)
	assert.ElementsMatch(t, []string{"json"}, r.Outputs)
}
