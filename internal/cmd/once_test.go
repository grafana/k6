package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/lib/executor"
)

func TestApplyOncePreservesScenarioName(t *testing.T) {
	t.Parallel()

	opts := lib.Options{Scenarios: lib.ScenarioConfigs{
		"ui":  executor.NewConstantVUsConfig("ui"),
		"api": executor.NewConstantVUsConfig("api"),
	}}

	res, err := applyOnce(opts)
	require.NoError(t, err)
	require.Len(t, res.Scenarios, 2)

	for _, name := range []string{"ui", "api"} {
		sc, ok := res.Scenarios[name].(executor.SharedIterationsConfig)
		require.True(t, ok)
		assert.Equal(t, name, sc.GetName())
		assert.Equal(t, null.IntFrom(1), sc.VUs)
		assert.Equal(t, null.IntFrom(1), sc.Iterations)
	}
}

func mustExtractSingleSharedIterScenario(t *testing.T, opts lib.Options) executor.SharedIterationsConfig {
	t.Helper()

	require.Len(t, opts.Scenarios, 1)
	for _, sc := range opts.Scenarios {
		shared, ok := sc.(executor.SharedIterationsConfig)
		require.True(t, ok)
		return shared
	}
	return executor.SharedIterationsConfig{}
}

func TestApplyOncePreservesScenarioExec(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		exec null.String
	}{
		{"custom", null.StringFrom("api")},
		{"unset", null.String{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			src := executor.NewSharedIterationsConfig("ui")
			src.Exec = tt.exec
			opts := lib.Options{Scenarios: lib.ScenarioConfigs{"ui": src}}

			res, err := applyOnce(opts)
			require.NoError(t, err)
			assert.Equal(t, tt.exec, mustExtractSingleSharedIterScenario(t, res).Exec)
		})
	}
}
