package cloud

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/js/modulestest"
	"go.k6.io/k6/lib"
)

func setupCloudTestEnv(t *testing.T) *modulestest.Runtime {
	tRt := modulestest.NewRuntime(t)
	m, ok := New().NewModuleInstance(tRt.VU).(*ModuleInstance)
	require.True(t, ok)
	require.NoError(t, tRt.VU.Runtime().Set("cloud", m.Exports().Default))
	return tRt
}

func TestGetTestRunId(t *testing.T) {
	t.Parallel()

	t.Run("init context", func(t *testing.T) {
		t.Parallel()
		tRt := setupCloudTestEnv(t)
		_, err := tRt.VU.Runtime().RunString(`cloud.testRunId`)
		require.ErrorIs(t, err, errRunInInitContext)
	})

	t.Run("undefined", func(t *testing.T) {
		t.Parallel()
		tRt := setupCloudTestEnv(t)
		tRt.MoveToVUContext(&lib.State{
			Options: lib.Options{},
		})
		testRunId, err := tRt.VU.Runtime().RunString(`cloud.testRunId`)
		require.NoError(t, err)
		assert.Equal(t, "undefined", testRunId.String())
	})

	t.Run("defined", func(t *testing.T) {
		t.Parallel()
		tRt := setupCloudTestEnv(t)
		tRt.MoveToVUContext(&lib.State{
			Options: lib.Options{
				Cloud: []byte(`{"testRunId": "123"}`),
			},
		})
		testRunId, err := tRt.VU.Runtime().RunString(`cloud.testRunId`)
		require.NoError(t, err)
		assert.Equal(t, "123", testRunId.String())
	})
}
