package browser

import (
	"context"
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	k6common "go.k6.io/k6/js/common"
	k6modulestest "go.k6.io/k6/js/modulestest"
	k6metrics "go.k6.io/k6/metrics"
)

// TestModuleNew tests registering the module.
// It doesn't test the module's remaining functionality as it is
// already tested in the tests/ integration tests.
func TestModuleNew(t *testing.T) {
	t.Parallel()

	vu := &k6modulestest.VU{
		RuntimeField: goja.New(),
		InitEnvField: &k6common.InitEnvironment{
			Registry: k6metrics.NewRegistry(),
		},
		CtxField: context.Background(),
	}
	m, ok := New().NewModuleInstance(vu).(*ModuleInstance)
	require.True(t, ok, "NewModuleInstance should return a ModuleInstance")
	require.NotNil(t, m.mod, "Module should be set")
	require.NotNil(t, m.mod.Chromium, "Chromium should be set")
	require.NotNil(t, m.mod.Devices, "Devices should be set")
	require.Equal(t, version, m.mod.Version, "Incorrect version")
}

func TestModuleNewDisabled(t *testing.T) {
	t.Setenv("K6_BROWSER_DISABLE_RUN", "")

	vu := &k6modulestest.VU{
		RuntimeField: goja.New(),
		InitEnvField: &k6common.InitEnvironment{
			Registry: k6metrics.NewRegistry(),
		},
	}
	assert.Panics(t, func() { New().NewModuleInstance(vu) })
}
