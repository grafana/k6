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

// TestModuleInitContext tests that using the browser APIs in the init context
// (where the VU has no state) fails with an error instead of panicking.
func TestModuleInitContext(t *testing.T) {
	t.Parallel()

	vu := k6test.NewVU(t)
	require.Nil(t, vu.State(), "the VU must be in the init context")

	m, ok := New().NewModuleInstance(vu).(*ModuleInstance)
	require.True(t, ok)
	rt := vu.Runtime()
	require.NoError(t, rt.Set("browser", m.mod.Browser))
	require.NoError(t, rt.Set("chromium", m.mod.Chromium))

	// Sync API: the error is thrown on the JS thread.
	_, err := rt.RunString(`browser.isConnected()`)
	require.ErrorContains(t, err, errInitContext.Error())

	// Async APIs: the promise is rejected instead of panicking in a goroutine.
	for _, call := range []string{
		`browser.newPage()`,
		`chromium.connectOverCDP('ws://localhost:0')`,
	} {
		err = vu.Loop.Start(func() error {
			_, err := rt.RunString(`globalThis.reason = ''; ` + call +
				`.catch(e => { globalThis.reason = String(e) })`)
			return err
		})
		require.NoError(t, err, call)
		require.Contains(t, rt.Get("reason").String(), errInitContext.Error(), call)
	}
}
