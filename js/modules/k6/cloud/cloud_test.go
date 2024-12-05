package cloud

import (
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/js/modulestest"
	"go.k6.io/k6/lib"
)

func setupCloudTestEnv(t *testing.T, env map[string]string) *modulestest.Runtime {
	tRt := modulestest.NewRuntime(t)
	tRt.VU.InitEnv().LookupEnv = func(key string) (string, bool) {
		v, ok := env[key]
		return v, ok
	}
	m, ok := New().NewModuleInstance(tRt.VU).(*ModuleInstance)
	require.True(t, ok)
	require.NoError(t, tRt.VU.Runtime().Set("cloud", m.Exports().Default))
	return tRt
}

func TestGetTestRunId(t *testing.T) {
	t.Parallel()

	t.Run("Cloud execution", func(t *testing.T) {
		t.Parallel()

		t.Run("Not defined", func(t *testing.T) {
			t.Parallel()
			tRt := setupCloudTestEnv(t, nil)
			testRunId, err := tRt.VU.Runtime().RunString(`cloud.testRunId`)
			require.NoError(t, err)
			assert.Equal(t, sobek.Undefined(), testRunId)
		})

		t.Run("Defined", func(t *testing.T) {
			t.Parallel()
			tRt := setupCloudTestEnv(t, map[string]string{"K6_CLOUD_TEST_RUN_ID": "123"})
			testRunId, err := tRt.VU.Runtime().RunString(`cloud.testRunId`)
			require.NoError(t, err)
			assert.Equal(t, "123", testRunId.String())
		})
	})

	t.Run("Local execution", func(t *testing.T) {
		t.Parallel()

		t.Run("Init context", func(t *testing.T) {
			t.Parallel()
			tRt := setupCloudTestEnv(t, nil)
			testRunId, err := tRt.VU.Runtime().RunString(`cloud.testRunId`)
			require.NoError(t, err)
			assert.Equal(t, sobek.Undefined(), testRunId)
		})

		t.Run("Not defined", func(t *testing.T) {
			t.Parallel()
			tRt := setupCloudTestEnv(t, nil)
			tRt.MoveToVUContext(&lib.State{
				Options: lib.Options{},
			})
			testRunId, err := tRt.VU.Runtime().RunString(`cloud.testRunId`)
			require.NoError(t, err)
			assert.Equal(t, sobek.Undefined(), testRunId)
		})

		t.Run("Defined", func(t *testing.T) {
			t.Parallel()
			tRt := setupCloudTestEnv(t, nil)
			tRt.MoveToVUContext(&lib.State{
				Options: lib.Options{
					Cloud: []byte(`{"testRunID": "123"}`),
				},
			})
			testRunId, err := tRt.VU.Runtime().RunString(`cloud.testRunId`)
			require.NoError(t, err)
			assert.Equal(t, "123", testRunId.String())
		})
	})
}
