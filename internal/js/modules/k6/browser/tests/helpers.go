package tests

import (
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/js/modules/k6/browser/browser"
	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext/k6test"
)

// startIteration will work with the event system to start chrome and
// (more importantly) set browser as the mapped browser instance which will
// force all tests that work with this to go through the mapping layer.
// This returns a cleanup function which should be deferred.
// The opts are passed to k6test.NewVU as is without any modification.
func startIteration(t *testing.T, opts ...any) (*k6test.VU, *sobek.Runtime, *[]string, func()) {
	t.Helper()

	vu := k6test.NewVU(t, opts...)
	rt := vu.Runtime()

	mod := browser.New().NewModuleInstance(vu)
	jsMod, ok := mod.Exports().Default.(*browser.JSModule)
	require.Truef(t, ok, "unexpected default mod export type %T", mod.Exports().Default)

	// Setting the mapped browser into the vu's sobek runtime.
	require.NoError(t, rt.Set("browser", jsMod.Browser))

	// Setting log, which is used by the callers to assert that certain actions
	// have been made.
	var log []string
	require.NoError(t, rt.Set("log", func(s string) { log = append(log, s) }))

	vu.ActivateVU()
	vu.StartIteration(t)

	return vu, rt, &log, func() { t.Helper(); vu.EndIteration(t) }
}
