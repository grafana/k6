package browser

import (
	"testing"

	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/k6ext/k6test"
	moduletrace "go.k6.io/k6/v2/lib/trace"
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

// TestModuleInstanceTracingReadFreshPerInstance guards against a regression
// where the tracing-enabled flag was cached once (alongside the module's
// other sync.Once-gated, genuinely-shared state) on the very first
// NewModuleInstance call. In a real k6 run that first call happens while the
// script is being loaded, before the resolved --tracing value is attached to
// the test's TestPreInitState (see internal/cmd/run.go), so caching it there
// would silently disable tracing for the whole run regardless of the flag.
// Each VU's own instantiation must read the current value instead.
//
// This only exercises the RootModule/NewModuleInstance side; the end-to-end
// regression coverage (verifying actual spans through a real script load +
// VU run, in that exact order) lives in
// internal/execution's TestBrowserSpansSurviveLateTracingResolution.
func TestModuleInstanceTracingReadFreshPerInstance(t *testing.T) {
	t.Parallel()

	root := New()

	// First instantiation: Tracing not resolved yet (as at script-load time).
	// This must not panic, and must not latch tracing off for later VUs.
	earlyVU := k6test.NewVU(t)
	root.NewModuleInstance(earlyVU)

	// Tracing gets resolved afterwards (as it does once loading finishes),
	// then a later, real VU is instantiated: it must see it enabled.
	set, err := moduletrace.ParseSet("browser")
	require.NoError(t, err)
	laterVU := k6test.NewVU(t)
	laterVU.InitEnvField.Tracing = set

	require.True(t, laterVU.InitEnv().Tracing.Enabled("browser"),
		"sanity check: the VU's own InitEnv should report tracing enabled")
	m, ok := root.NewModuleInstance(laterVU).(*ModuleInstance)
	require.True(t, ok)
	require.NotNil(t, m.mod.Browser)
}
