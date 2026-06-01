package browser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/common/autoscreenshot"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/env"
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
}

// TestRootModule_AutoScreenshotMode verifies that the K6_BROWSER_AUTO_SCREENSHOT
// environment variable is parsed into RootModule.autoScreenshotMode during
// initialize().
func TestRootModule_AutoScreenshotMode(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		env  env.LookupFunc
		want autoscreenshot.Mode
	}{
		{
			name: "unset",
			env:  env.EmptyLookup,
			want: autoscreenshot.ModeOff,
		},
		{
			name: "actions",
			env:  env.ConstLookup(env.AutoScreenshot, "actions"),
			want: autoscreenshot.ModeActions,
		},
		{
			name: "unknown_value_disables",
			env:  env.ConstLookup(env.AutoScreenshot, "nonsense"),
			want: autoscreenshot.ModeOff,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t, tc.env)
			rm := New()
			_ = rm.NewModuleInstance(vu)

			assert.Equal(t, tc.want, rm.autoScreenshotMode)
		})
	}
}
