package webcrypto

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetRandomValues(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)

	gotScriptErr := ts.ev.Start(func() error {
		_, err := ts.rt.RunString(`
		var input = new Uint8Array(10);
		var output = crypto.getRandomValues(input);

		if (output.length != 10) {
			throw new Error("output.length != 10");
		}

		// Note that we're comparing references here, not values.
		// Thus we're testing that the same typed array is returned.
		if (input !== output) {
			throw new Error("input !== output");
		}
		`)

		return err
	})

	assert.NoError(t, gotScriptErr)
}

// TODO: Add tests for DataView

// TestGetRandomValues tests that crypto.getRandomValues() supports the expected types
// listed in the [specification]:
// - Int8Array
// - Int16Arrays
// - Int32Array
// - Uint8Array
// - Uint8ClampedArray
// - Uint16Array
// - Uint32Array
//
// It stands as the k6 counterpart of the [official test suite] on that topic.
//
// [specification]: https://www.w3.org/TR/WebCryptoAPI/#Crypto-method-getRandomValues
// [official test suite]: https://github.com/web-platform-tests/wpt/blob/master/WebCryptoAPI/getRandomValues.any.js#L1
func TestGetRandomValuesSupportedTypedArrays(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)

	type testCase struct {
		name       string
		typedArray string
		wantErr    bool
	}

	testCases := []testCase{
		{
			name:       "filling a Int8Array typed array with random values should succeed",
			typedArray: "Int8Array",
			wantErr:    false,
		},
		{
			name:       "filling a Int16Array typed array with random values should succeed",
			typedArray: "Int16Array",
			wantErr:    false,
		},
		{
			name:       "filling a Int32Array typed array with random values should succeed",
			typedArray: "Int32Array",
			wantErr:    false,
		},
		{
			name:       "filling a Uint8Array typed array with random values should succeed",
			typedArray: "Uint8Array",
			wantErr:    false,
		},
		{
			name:       "filling a Uint8ClampedArray typed array with random values should succeed",
			typedArray: "Uint8ClampedArray",
			wantErr:    false,
		},
		{
			name:       "filling a Uint16Array typed array with random values should succeed",
			typedArray: "Uint16Array",
			wantErr:    false,
		},
		{
			name:       "filling a Uint32Array typed array with random values should succeed",
			typedArray: "Uint32Array",
			wantErr:    false,
		},

		// Unsupported typed arrays
		{
			name:       "filling a BigInt64Array typed array with random values should succeed",
			typedArray: "BigInt64Array",
			wantErr:    true,
		},
		{
			name:       "filling a BigUint64Array typed array with random values should succeed",
			typedArray: "BigUint64Array",
			wantErr:    true,
		},
		{
			name:       "filling a Float32Array typed array with random values should fail",
			typedArray: "Float32Array",
			wantErr:    true,
		},
		{
			name:       "filling a Float64Array typed array with random values should fail",
			typedArray: "Float64Array",
			wantErr:    true,
		},
	}

	for _, tc := range testCases {
		gotScriptErr := ts.ev.Start(func() error {
			script := fmt.Sprintf(`
				var buf = new %s(10);
				crypto.getRandomValues(buf);

				if (buf.length != 10) {
					throw new Error("buf.length != 10");
				}
			`, tc.typedArray)

			_, err := ts.rt.RunString(script)
			return err
		})

		if tc.wantErr != (gotScriptErr != nil) {
			t.Fatalf("unexpected error: %v", gotScriptErr)
		}

		assert.Equal(t, tc.wantErr, gotScriptErr != nil, tc.name)
	}
}

// TestGetRandomValuesQuotaExceeded tests that crypto.getRandomValues() returns a
// QuotaExceededError when the requested size is too large. As described in the
// [specification], the maximum size is 65536 bytes.
//
// It stands as the k6 counterpart of the [official test suite] on that topic.
//
// [specification]: https://www.w3.org/TR/WebCryptoAPI/#Crypto-method-getRandomValues
// [official test suite]: https://github.com/web-platform-tests/wpt/blob/master/WebCryptoAPI/getRandomValues.any.js#L1
func TestGetRandomValuesQuotaExceeded(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)

	gotScriptErr := ts.ev.Start(func() error {
		_, err := ts.rt.RunString(`
		var buf = new Uint8Array(1000000000);
		crypto.getRandomValues(buf);
		`)

		return err
	})

	assert.Error(t, gotScriptErr)
	assert.Contains(t, gotScriptErr.Error(), "QuotaExceededError")
}
