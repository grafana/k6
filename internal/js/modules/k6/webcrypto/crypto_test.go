// Note: this test lives in the webcrypto_test package because js/modulestest
// imports webcrypto, so an in-package test would create an import cycle.
package webcrypto_test

import (
	"fmt"
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/js/modulestest"
)

// catchScript runs the given statements and reports what the script saw, so the
// assertions can look at the thrown value the way a user's test would, instead
// of at the Go error RunString wraps it in.
const catchScript = `
	(function () {
		try {
			%s
			return { threw: false };
		} catch (e) {
			return { threw: true, isTypeError: e instanceof TypeError, text: String(e) };
		}
	})()
`

// TestGetRandomValuesRejectsBadInput asserts that bad input to
// crypto.getRandomValues() is reported to the script as a catchable error,
// instead of taking the whole k6 process down with a Go panic.
//
// See https://github.com/grafana/k6/issues/6320
func TestGetRandomValuesRejectsBadInput(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		script string
		// wantTypeError is true when the script should see a genuine ECMAScript
		// TypeError, so that `e instanceof TypeError` holds the way it does in
		// browsers and in Node. The WebCrypto error names are not ECMAScript
		// errors, so they are reported as GoError instead.
		wantTypeError bool
		// wantMessage is a substring of the thrown value's string form.
		wantMessage string
	}{
		{
			// GetRandomValues receives a nil sobek.Value, and IsInstanceOf
			// dereferences it. The WebIDL argument is required, so browsers
			// throw a TypeError here.
			name:          "no argument",
			script:        `crypto.getRandomValues()`,
			wantTypeError: true,
			wantMessage:   "typedArray parameter is required",
		},
		{
			// A nullish argument already threw a TypeError before the check
			// that fixes the case above existed, so it has to keep throwing
			// one, or scripts branching on `e instanceof TypeError` break.
			name:          "null argument",
			script:        `crypto.getRandomValues(null)`,
			wantTypeError: true,
			wantMessage:   "typedArray parameter is required",
		},
		{
			name:          "undefined argument",
			script:        `crypto.getRandomValues(undefined)`,
			wantTypeError: true,
			wantMessage:   "typedArray parameter is required",
		},
		{
			// Not an ArrayBufferView at all. Today k6 reports this as a
			// TypeMismatchError, which the spec reserves for views of the
			// wrong element type (Float32Array, DataView, ...). Left as it is,
			// since it never panicked and changing it is a separate concern.
			name:          "not a TypedArray",
			script:        `crypto.getRandomValues({})`,
			wantTypeError: false,
			wantMessage:   "TypeMismatchError",
		},
		{
			// Quota is on byteLength. 16385 Uint32 elements is 65540 bytes.
			name:          "Uint32Array over the byte quota",
			script:        `crypto.getRandomValues(new Uint32Array(16385))`,
			wantTypeError: false,
			wantMessage:   "QuotaExceededError",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// NewRuntime already calls webcrypto.SetupGlobally, so the
			// crypto object is available as a global.
			rt := modulestest.NewRuntime(t)

			var (
				got sobek.Value
				err error
			)
			require.NotPanics(t, func() {
				got, err = rt.VU.Runtime().RunString(fmt.Sprintf(catchScript, tc.script))
			})
			require.NoError(t, err)

			result := got.ToObject(rt.VU.Runtime())
			require.True(t, result.Get("threw").ToBoolean(), "the script was expected to throw")
			assert.Equal(t, tc.wantTypeError, result.Get("isTypeError").ToBoolean())
			assert.Contains(t, result.Get("text").String(), tc.wantMessage)
		})
	}
}

// TestGetRandomValuesFillsFullElementWidth asserts that 16- and 32-bit typed
// arrays get a full-width random value in each element, not a single byte.
//
// See https://github.com/grafana/k6/issues/6318
func TestGetRandomValuesFillsFullElementWidth(t *testing.T) {
	t.Parallel()

	rt := modulestest.NewRuntime(t)
	got, err := rt.VU.Runtime().RunString(`
		(function () {
			const u16 = new Uint16Array(256);
			const u32 = new Uint32Array(256);
			const viewHost = new Uint32Array(4);
			viewHost.fill(0xdeadbeef);
			const view = new Uint32Array(viewHost.buffer, 4, 2);

			crypto.getRandomValues(u16);
			crypto.getRandomValues(u32);
			crypto.getRandomValues(view);

			let u16Max = 0;
			let u32Max = 0;
			for (const n of u16) {
				u16Max = Math.max(u16Max, n);
			}
			for (const n of u32) {
				u32Max = Math.max(u32Max, n);
			}

			return {
				u16Max: u16Max,
				u32Max: u32Max,
				viewHost0: viewHost[0],
				viewHost3: viewHost[3],
				viewChanged: viewHost[1] !== 0xdeadbeef || viewHost[2] !== 0xdeadbeef,
			};
		})()
	`)
	require.NoError(t, err)

	result := got.ToObject(rt.VU.Runtime())
	assert.Greater(t, result.Get("u16Max").ToInteger(), int64(255))
	assert.Greater(t, result.Get("u32Max").ToInteger(), int64(65535))
	assert.Equal(t, int64(0xdeadbeef), result.Get("viewHost0").ToInteger())
	assert.Equal(t, int64(0xdeadbeef), result.Get("viewHost3").ToInteger())
	assert.True(t, result.Get("viewChanged").ToBoolean())
}

// TestGetRandomValuesIgnoresOverriddenLength keeps the #6320 guarantee that a
// script-controlled length cannot take the process down, now that filling uses
// the view's real bytes instead of the length property.
func TestGetRandomValuesIgnoresOverriddenLength(t *testing.T) {
	t.Parallel()

	rt := modulestest.NewRuntime(t)
	require.NotPanics(t, func() {
		_, err := rt.VU.Runtime().RunString(`
			const a = new Uint8Array(4);
			Object.defineProperty(a, "length", { value: -1 });
			crypto.getRandomValues(a);
		`)
		require.NoError(t, err)
	})
}
