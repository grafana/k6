package webcrypto_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/js/modulestest"
)

// deriveBitsScript derives the full ECDH shared secret and a prefix of %[2]d bits of it
// from the same key pair, and stores both on globalThis so the test can compare them.
const deriveBitsScript = `
	(async function () {
		try {
			const algorithm = { name: "ECDH", namedCurve: "%[1]s" };
			const alice = await crypto.subtle.generateKey(algorithm, true, ["deriveBits"]);
			const bob = await crypto.subtle.generateKey(algorithm, true, ["deriveBits"]);
			const params = { name: "ECDH", public: bob.publicKey };

			globalThis.full = Array.from(new Uint8Array(
				await crypto.subtle.deriveBits(params, alice.privateKey, %[3]d)));
			globalThis.partial = Array.from(new Uint8Array(
				await crypto.subtle.deriveBits(params, alice.privateKey, %[2]d)));
		} catch (e) {
			globalThis.error = String(e);
		}
	})();
`

// TestECDHDeriveBitsNonMultipleOf8 asserts that ECDH deriveBits accepts a length that
// is not a multiple of 8, and returns the first length bits of the shared secret with
// the unused trailing bits of the last byte set to zero, as Chrome does.
//
// See https://github.com/grafana/k6/issues/4260
func TestECDHDeriveBitsNonMultipleOf8(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		curve string
		// size is the length of the curve's shared secret in bytes.
		size   int
		length int
	}{
		// The lengths used by the Web Platform Tests (8 * size - 11).
		{curve: "P-256", size: 32, length: 8*32 - 11},
		{curve: "P-384", size: 48, length: 8*48 - 11},
		{curve: "P-521", size: 66, length: 8*66 - 11},
		// A single bit, the smallest length allowed.
		{curve: "P-256", size: 32, length: 1},
		// One bit short of the full secret.
		{curve: "P-256", size: 32, length: 8*32 - 1},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s/%d bits", tc.curve, tc.length), func(t *testing.T) {
			t.Parallel()

			rt := modulestest.NewRuntime(t)
			_, err := rt.RunOnEventLoop(fmt.Sprintf(deriveBitsScript, tc.curve, tc.length, 8*tc.size))
			require.NoError(t, err)

			vm := rt.VU.Runtime()
			require.Nil(t, vm.Get("error"), "deriveBits failed: %v", vm.Get("error"))

			var full, partial []byte
			require.NoError(t, vm.ExportTo(vm.Get("full"), &full))
			require.NoError(t, vm.ExportTo(vm.Get("partial"), &partial))
			require.Len(t, full, tc.size)

			wantBytes := (tc.length + 7) / 8
			require.Len(t, partial, wantBytes)

			// All the bytes but the last one are copied as they are.
			assert.Equal(t, full[:wantBytes-1], partial[:wantBytes-1])

			// The last byte keeps only its leading length%8 bits.
			mask := byte(0xFF << (8 - tc.length%8))
			assert.Equal(t, full[wantBytes-1]&mask, partial[wantBytes-1],
				"last byte should be %08b masked with %08b", full[wantBytes-1], mask)
		})
	}
}

// TestPBKDF2DeriveBitsNonMultipleOf8 asserts that PBKDF2 keeps rejecting a length that
// is not a multiple of 8 with an OperationError, as the specification requires, now that
// the common check in SubtleCrypto.DeriveBits no longer does it for every algorithm.
func TestPBKDF2DeriveBitsNonMultipleOf8(t *testing.T) {
	t.Parallel()

	rt := modulestest.NewRuntime(t)
	_, err := rt.RunOnEventLoop(`
		(async function () {
			try {
				const key = await crypto.subtle.importKey(
					"raw", new TextEncoder().encode("password"), "PBKDF2", false, ["deriveBits"]);
				await crypto.subtle.deriveBits(
					{ name: "PBKDF2", salt: new Uint8Array(16), hash: "SHA-256", iterations: 1 }, key, 44);
			} catch (e) {
				globalThis.error = String(e);
			}
		})();
	`)
	require.NoError(t, err)

	vm := rt.VU.Runtime()
	require.NotNil(t, vm.Get("error"), "deriveBits was expected to fail")
	assert.Contains(t, vm.Get("error").String(), "OperationError")
}

// TestECDHDeriveBitsTooLong asserts that asking for more bits than the shared secret
// has is rejected, including when the extra bits do not add up to a whole byte.
func TestECDHDeriveBitsTooLong(t *testing.T) {
	t.Parallel()

	for _, length := range []int{8*32 + 1, 8*32 + 8} {
		t.Run(fmt.Sprintf("%d bits", length), func(t *testing.T) {
			t.Parallel()

			rt := modulestest.NewRuntime(t)
			_, err := rt.RunOnEventLoop(fmt.Sprintf(deriveBitsScript, "P-256", length, 8*32))
			require.NoError(t, err)

			vm := rt.VU.Runtime()
			require.NotNil(t, vm.Get("error"), "deriveBits was expected to fail")
			assert.Contains(t, vm.Get("error").String(), "OperationError")
			assert.Contains(t, vm.Get("error").String(), "length is too large")
		})
	}
}
