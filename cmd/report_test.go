package cmd

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/execution"
	"go.k6.io/k6/execution/local"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/consts"
	"go.k6.io/k6/lib/executor"
	"go.k6.io/k6/lib/testutils"
	"go.k6.io/k6/usage"
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

	m := createReport(usage.New(), s)
	require.NoError(t, err)

	assert.Equal(t, consts.Version, m["k6_version"])
	assert.EqualValues(t, map[string]int{"shared-iterations": 1}, m["executors"])
	assert.EqualValues(t, 6, m["vus_max"])
	assert.EqualValues(t, 170, m["iterations"])
	assert.NotEqual(t, "0s", m["duration"])
}
