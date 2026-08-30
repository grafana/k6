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

	res := applyOnce(opts)
	require.Len(t, res.Scenarios, 2)

	for _, name := range []string{"ui", "api"} {
		sc, ok := res.Scenarios[name].(executor.SharedIterationsConfig)
		require.True(t, ok)
		assert.Equal(t, name, sc.GetName())
		assert.Equal(t, null.IntFrom(1), sc.VUs)
		assert.Equal(t, null.IntFrom(1), sc.Iterations)
	}
}
