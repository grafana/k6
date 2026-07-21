package browser

import (
	"errors"
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// awaitResult is a test helper that calls awaitHandlerResult and collects
// the outcome from the result channel, assuming the Promise callbacks fire
// synchronously (after one microtask flush via RunString).
func awaitResult(t *testing.T, rt *sobek.Runtime, retVal sobek.Value) error {
	t.Helper()
	result := make(chan error, 1)
	awaitHandlerResult(rt, retVal, result)
	// Flush microtasks: any pending .then callbacks are scheduled as microtasks
	// and run when the runtime executes the next statement.
	_, _ = rt.RunString("undefined")
	select {
	case err := <-result:
		return err
	default:
		t.Fatal("awaitHandlerResult did not send to result channel")
		return nil
	}
}

func TestAwaitHandlerResultNil(t *testing.T) {
	t.Parallel()
	rt := sobek.New()
	err := awaitResult(t, rt, nil)
	assert.NoError(t, err)
}

func TestAwaitHandlerResultUndefined(t *testing.T) {
	t.Parallel()
	rt := sobek.New()
	err := awaitResult(t, rt, sobek.Undefined())
	assert.NoError(t, err)
}

func TestAwaitHandlerResultNonPromise(t *testing.T) {
	t.Parallel()
	rt := sobek.New()
	v, err := rt.RunString(`42`)
	require.NoError(t, err)
	assert.NoError(t, awaitResult(t, rt, v))
}

func TestAwaitHandlerResultFulfilledPromise(t *testing.T) {
	t.Parallel()
	rt := sobek.New()
	v, err := rt.RunString(`Promise.resolve("ok")`)
	require.NoError(t, err)
	// Already-fulfilled Promise: awaitHandlerResult sends nil immediately.
	result := make(chan error, 1)
	awaitHandlerResult(rt, v, result)
	select {
	case err := <-result:
		assert.NoError(t, err)
	default:
		t.Fatal("expected result for already-fulfilled Promise")
	}
}

func TestAwaitHandlerResultRejectedPromise(t *testing.T) {
	t.Parallel()
	rt := sobek.New()
	// Suppress unhandled rejection panic from Sobek.
	rt.SetPromiseRejectionTracker(func(*sobek.Promise, sobek.PromiseRejectionOperation) {})
	v, err := rt.RunString(`Promise.reject(new Error("boom"))`)
	require.NoError(t, err)
	// Already-rejected Promise: awaitHandlerResult sends error immediately.
	result := make(chan error, 1)
	awaitHandlerResult(rt, v, result)
	select {
	case err := <-result:
		require.Error(t, err)
		assert.Contains(t, err.Error(), "boom")
	default:
		t.Fatal("expected result for already-rejected Promise")
	}
}

func TestAwaitHandlerResultPendingPromiseFulfilled(t *testing.T) {
	t.Parallel()
	rt := sobek.New()

	// Create a pending Promise and capture its resolve function.
	var resolveFn sobek.Callable
	v, err := rt.RunString(`
		var _resolve;
		new Promise(function(resolve) { _resolve = resolve; })
	`)
	require.NoError(t, err)

	resolveVal, err := rt.RunString(`_resolve`)
	require.NoError(t, err)
	var ok bool
	resolveFn, ok = sobek.AssertFunction(resolveVal)
	require.True(t, ok)

	result := make(chan error, 1)
	awaitHandlerResult(rt, v, result)

	// result channel is still empty — Promise is pending.
	select {
	case <-result:
		t.Fatal("result should not be sent while Promise is still pending")
	default:
	}

	// Resolve the Promise and flush microtasks.
	_, err = resolveFn(sobek.Undefined(), rt.ToValue("done"))
	require.NoError(t, err)
	_, _ = rt.RunString("undefined") // flush microtasks

	select {
	case err := <-result:
		assert.NoError(t, err)
	default:
		t.Fatal("awaitHandlerResult did not fire after Promise fulfilled")
	}
}

func TestAwaitHandlerResultPendingPromiseRejected(t *testing.T) {
	t.Parallel()
	rt := sobek.New()
	rt.SetPromiseRejectionTracker(func(*sobek.Promise, sobek.PromiseRejectionOperation) {})

	var rejectFn sobek.Callable
	v, err := rt.RunString(`
		var _reject;
		new Promise(function(_, reject) { _reject = reject; })
	`)
	require.NoError(t, err)

	rejectVal, err := rt.RunString(`_reject`)
	require.NoError(t, err)
	var ok bool
	rejectFn, ok = sobek.AssertFunction(rejectVal)
	require.True(t, ok)

	result := make(chan error, 1)
	awaitHandlerResult(rt, v, result)

	select {
	case <-result:
		t.Fatal("result should not be sent while Promise is still pending")
	default:
	}

	_, err = rejectFn(sobek.Undefined(), rt.ToValue(errors.New("async failure")))
	require.NoError(t, err)
	_, _ = rt.RunString("undefined")

	select {
	case err := <-result:
		require.Error(t, err)
		assert.Contains(t, err.Error(), "async failure")
	default:
		t.Fatal("awaitHandlerResult did not fire after Promise rejected")
	}
}

func TestSobekEmptyString(t *testing.T) {
	t.Parallel()
	// SobekEmpty string should return true if the argument
	// is an empty string or not defined in the Sobek runtime.
	rt := sobek.New()
	require.NoError(t, rt.Set("sobekEmptyString", sobekEmptyString))
	for _, s := range []string{"() => true", "'() => false'"} { // not empty
		v, err := rt.RunString(`sobekEmptyString(` + s + `)`)
		require.NoError(t, err)
		require.Falsef(t, v.ToBoolean(), "got: true, want: false for %q", s)
	}
	for _, s := range []string{"", "  ", "null", "undefined"} { // empty
		v, err := rt.RunString(`sobekEmptyString(` + s + `)`)
		require.NoError(t, err)
		require.Truef(t, v.ToBoolean(), "got: false, want: true for %q", s)
	}
}
