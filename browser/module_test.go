package browser

import (
	"context"
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/require"

	k6common "go.k6.io/k6/js/common"
	k6modulestest "go.k6.io/k6/js/modulestest"
	k6lib "go.k6.io/k6/lib"
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
			TestPreInitState: &k6lib.TestPreInitState{
				Registry: k6metrics.NewRegistry(),
			},
		},
		CtxField: context.Background(),
	}
	m, ok := New().NewModuleInstance(vu).(*ModuleInstance)
	require.True(t, ok, "NewModuleInstance should return a ModuleInstance")
	require.NotNil(t, m.mod, "Module should be set")
	require.NotNil(t, m.mod.Chromium, "Chromium should be set")
	require.NotNil(t, m.mod.Devices, "Devices should be set")
}
