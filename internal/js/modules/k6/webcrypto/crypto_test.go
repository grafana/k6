// Note: this test lives in the webcrypto_test package because js/modulestest
// imports webcrypto, so an in-package test would create an import cycle.
package webcrypto_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/js/modulestest"
)

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
		// wantError is the error name the script should see.
		wantError string
	}{
		{
			// GetRandomValues receives a nil sobek.Value, and IsInstanceOf
			// dereferences it. The WebIDL argument is required, so browsers
			// throw a TypeError here.
			name:      "no argument",
			script:    `crypto.getRandomValues()`,
			wantError: "TypeError",
		},
		{
			name:      "null argument",
			script:    `crypto.getRandomValues(null)`,
			wantError: "TypeError",
		},
		{
			// Not an ArrayBufferView at all. Today k6 reports this as a
			// TypeMismatchError, which the spec reserves for views of the
			// wrong element type (Float32Array, DataView, ...).
			name:      "not a TypedArray",
			script:    `crypto.getRandomValues({})`,
			wantError: "TypeMismatchError",
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
			wantError: "TypeError",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// NewRuntime already calls webcrypto.SetupGlobally, so the
			// crypto object is available as a global.
			rt := modulestest.NewRuntime(t)

			var err error
			require.NotPanics(t, func() {
				_, err = rt.VU.Runtime().RunString(tc.script)
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantError)
		})
	}
}
