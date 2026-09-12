package browser

import (
	"testing"

	"github.com/grafana/sobek"
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

// TestModuleInInitContext is the regression test for #6178: browser module APIs
// called in the init context used to nil-dereference VU.State(). For the
// promise-wrapped ones that happened in a plain goroutine, an unrecovered panic
// that crashed the whole k6 process. They must all fail as script errors now.
func TestModuleInInitContext(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"browser_async":  `await browser.newPage()`,
		"browser_sync":   `browser.isConnected()`,
		"chromium_async": `await chromium.connectOverCDP("ws://127.0.0.1:1/devtools/browser/none")`,
	}
	for name, script := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Build the module as k6 does at init time: the VU is not activated,
			// so its State is nil.
			vu := k6test.NewVU(t)
			exports := New().NewModuleInstance(vu).Exports().Default
			mod, ok := exports.(*JSModule)
			require.Truef(t, ok, "unexpected default export type %T", exports)
			vu.SetVar(t, "browser", mod.Browser)
			vu.SetVar(t, "chromium", mod.Chromium)

			// Getting this far without a crashed test binary already means the
			// unrecovered goroutine panic is gone. The assertions below pin down
			// the error users get instead.
			p := vu.RunPromise(t, `
				try {
					%s;
					return "no error";
				} catch (e) {
					return String(e);
				}
			`, script)
			require.Equal(t, sobek.PromiseStateFulfilled, p.State())
			require.Contains(t, p.Result().String(), errInitContext.Error())
		})
	}
}
