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
			// The array is a real Uint8Array, so the element type is fine,
			// but make([]byte, -1) panics.
			name: "length overridden with a negative value",
			script: `
				const a = new Uint8Array(4);
				Object.defineProperty(a, "length", { value: -1 });
				crypto.getRandomValues(a);
			`,
			wantTypeError: true,
			wantMessage:   "typedArray parameter's length is negative",
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
