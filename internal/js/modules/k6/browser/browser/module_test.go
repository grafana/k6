package browser

import (
	"testing"

	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/k6ext/k6test"
)

// TestModuleNew tests registering the module.
// It doesn't test the module's remaining functionality as it is
// already tested in the tests/ integration tests.
func TestModuleNew(t *testing.T) {
	t.Parallel()

	vu := k6test.NewVU(t)
	m, ok := New().NewModuleInstance(vu).(*ModuleInstance)
	require.True(t, ok, "NewModuleInstance should return a ModuleInstance")
	require.NotNil(t, m.mod, "Module should be set")
	require.NotNil(t, m.mod.Browser, "Browser should be set")
	require.NotNil(t, m.mod.Devices, "Devices should be set")
	require.NotNil(t, m.mod.NetworkProfiles, "Profiles should be set")
	require.NotNil(t, m.mod.Chromium, "Chromium should be set")
}

func TestEnableTracing(t *testing.T) {
	t.Parallel()

	vu := k6test.NewVU(t)
	root := New()
	m := root.NewModuleInstance(vu).(*ModuleInstance) //nolint:forcetypeassert
	require.False(t, root.tracingEnabled.Load())
	require.NoError(t, vu.Runtime().Set("browser", m.mod.Browser))

	_, err := vu.Runtime().RunString("browser.enableTracing()")
	require.NoError(t, err)
	require.True(t, root.tracingEnabled.Load())
}

func TestEnableTracingRequiresInitContext(t *testing.T) {
	t.Parallel()

	vu := k6test.NewVU(t)
	root := New()
	m := root.NewModuleInstance(vu).(*ModuleInstance) //nolint:forcetypeassert
	require.NoError(t, vu.Runtime().Set("browser", m.mod.Browser))
	vu.ActivateVU()

	_, err := vu.Runtime().RunString("browser.enableTracing()")
	require.ErrorContains(t, err, "browser.enableTracing() must be called in the init context")
	require.False(t, root.tracingEnabled.Load())
}
