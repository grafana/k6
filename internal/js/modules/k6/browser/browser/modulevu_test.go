package browser

import (
	"sync/atomic"
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/k6ext/k6test"
)

const errInitContextMsg = "the browser module can only be used in the iteration context"

// TestModuleVUBrowserInitContext verifies that resolving the VU browser in the
// init context returns a friendly error instead of nil-dereferencing
// VU.State(). This is the shared path behind the synchronous browser APIs
// (isConnected, userAgent, version). See #6178.
func TestModuleVUBrowserInitContext(t *testing.T) {
	t.Parallel()

	// A VU that has not been activated has a nil State(): the init context.
	vu := k6test.NewVU(t)
	mvu := moduleVU{VU: vu}

	_, err := mvu.browser()
	require.ErrorIs(t, err, errInitContext)
}

// TestBrowserAsyncAPIInInitContextRejects is the regression test for #6178: a
// promise-wrapped browser API called in the init context used to nil-deref
// VU.State() inside the promise() goroutine, an unrecovered panic that crashed
// the whole k6 process. It must now reject the promise with a friendly error.
func TestBrowserAsyncAPIInInitContextRejects(t *testing.T) {
	t.Parallel()

	// Init context: the VU is not activated, so State() is nil. Build the module
	// as k6 does at init and call an async browser API at the top level.
	vu := k6test.NewVU(t)
	mod := New().NewModuleInstance(vu)
	jsMod, ok := mod.Exports().Default.(*JSModule)
	require.Truef(t, ok, "unexpected default export type %T", mod.Exports().Default)
	vu.SetVar(t, "browser", jsMod.Browser)

	// If the process survives to here without a SIGSEGV, the goroutine panic is
	// already gone; the assertions below pin down the friendly rejection.
	p := vu.RunPromise(t, `
		try {
			await browser.newPage();
			return "NO_REJECTION";
		} catch (e) {
			return String(e);
		}
	`)
	require.Equal(t, sobek.PromiseStateFulfilled, p.State())
	require.Contains(t, p.Result().String(), errInitContextMsg)
}

// TestBrowserSyncAPIInInitContextThrows verifies the synchronous symptom from
// #6178: calling a sync browser API in the init context now throws a friendly
// JS error instead of surfacing a recovered Go panic.
func TestBrowserSyncAPIInInitContextThrows(t *testing.T) {
	t.Parallel()

	vu := k6test.NewVU(t)
	mod := New().NewModuleInstance(vu)
	jsMod, ok := mod.Exports().Default.(*JSModule)
	require.Truef(t, ok, "unexpected default export type %T", mod.Exports().Default)
	vu.SetVar(t, "browser", jsMod.Browser)

	_, err := vu.RunOnEventLoop(t, `browser.isConnected()`)
	require.ErrorContains(t, err, errInitContextMsg)
}

// TestPromiseInitContextDoesNotRunFn pins the promise() guard specifically: in
// the init context it must reject before spawning the goroutine, so fn never
// runs. This is what protects promise-wrapped APIs that dereference VU.State()
// directly in their fn (e.g. chromium.connectOverCDP), which would otherwise be
// an unrecovered goroutine panic. fn here deliberately does NOT touch State, so
// removing the guard surfaces as "fn ran" rather than a process crash.
func TestPromiseInitContextDoesNotRunFn(t *testing.T) {
	t.Parallel()

	vu := k6test.NewVU(t) // init context: State() is nil
	mvu := moduleVU{VU: vu}

	var ran atomic.Bool
	require.NoError(t, vu.Runtime().Set("callGuardedPromise", func() *sobek.Promise {
		return promise(mvu, func() (any, error) {
			ran.Store(true)
			return nil, nil
		})
	}))

	p := vu.RunPromise(t, `
		try {
			await callGuardedPromise();
			return "RESOLVED";
		} catch (e) {
			return String(e);
		}
	`)
	require.Equal(t, sobek.PromiseStateFulfilled, p.State())
	require.Contains(t, p.Result().String(), errInitContextMsg)
	require.False(t, ran.Load(), "fn must not run in the init context")
}
