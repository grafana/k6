package webcrypto_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/js/modules/k6/webcrypto"
	"go.k6.io/k6/v2/js/modulestest"
)

func newCryptoRuntime(t testing.TB) *modulestest.Runtime {
	t.Helper()

	rt := modulestest.NewRuntime(t)
	require.NoError(t, webcrypto.SetupGlobally(rt.VU))

	m := new(webcrypto.RootModule).NewModuleInstance(rt.VU)
	require.NoError(t, rt.VU.Runtime().Set("crypto", m.Exports().Named["crypto"]))

	return rt
}

func TestGetRandomValuesFillsFullWidthElements(t *testing.T) {
	t.Parallel()

	rt := newCryptoRuntime(t)

	// The documented example uses Uint32Array. The old implementation assigned
	// one random byte per element, so every value was in 0–255.
	_, err := rt.RunOnEventLoop(`
		const array = new Uint32Array(64);
		const ret = crypto.getRandomValues(array);
		if (ret !== array) {
			throw new Error("getRandomValues should return the same typed array");
		}

		let sawAbove255 = false;
		for (const num of array) {
			if (num > 255) {
				sawAbove255 = true;
				break;
			}
		}
		if (!sawAbove255) {
			throw new Error("Uint32Array values were all <= 255; expected full-width random values");
		}
	`)
	require.NoError(t, err)
}

func TestGetRandomValuesUint16FullWidth(t *testing.T) {
	t.Parallel()

	rt := newCryptoRuntime(t)

	_, err := rt.RunOnEventLoop(`
		const array = new Uint16Array(64);
		crypto.getRandomValues(array);

		let sawAbove255 = false;
		for (const num of array) {
			if (num > 255) {
				sawAbove255 = true;
				break;
			}
		}
		if (!sawAbove255) {
			throw new Error("Uint16Array values were all <= 255; expected full-width random values");
		}
	`)
	require.NoError(t, err)
}

func TestGetRandomValuesHonorsViewBounds(t *testing.T) {
	t.Parallel()

	rt := newCryptoRuntime(t)

	_, err := rt.RunOnEventLoop(`
		const buf = new ArrayBuffer(16);
		const prefix = new Uint8Array(buf, 0, 4);
		const view = new Uint32Array(buf, 4, 2);
		const suffix = new Uint8Array(buf, 12, 4);

		prefix.fill(0x11);
		suffix.fill(0x22);

		crypto.getRandomValues(view);

		for (let i = 0; i < prefix.length; i++) {
			if (prefix[i] !== 0x11) {
				throw new Error("bytes before the view were modified");
			}
		}
		for (let i = 0; i < suffix.length; i++) {
			if (suffix[i] !== 0x22) {
				throw new Error("bytes after the view were modified");
			}
		}

		const viewBytes = new Uint8Array(buf, 4, 8);
		let changed = false;
		for (const b of viewBytes) {
			if (b !== 0) {
				changed = true;
				break;
			}
		}
		if (!changed) {
			throw new Error("view bytes were not filled");
		}
	`)
	require.NoError(t, err)
}

func TestGetRandomValuesQuotaUsesByteLength(t *testing.T) {
	t.Parallel()

	rt := newCryptoRuntime(t)

	_, err := rt.RunOnEventLoop(`
		// 16385 * 4 = 65540 bytes, which exceeds the 65536-byte quota.
		crypto.getRandomValues(new Uint32Array(16385));
	`)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), webcrypto.QuotaExceededError), "got %v", err)
}

func TestGetRandomValuesUint8ArrayStillWorks(t *testing.T) {
	t.Parallel()

	rt := newCryptoRuntime(t)

	_, err := rt.RunOnEventLoop(`
		const array = new Uint8Array(8);
		crypto.getRandomValues(array);
		if (array.byteLength !== 8) {
			throw new Error("Uint8Array byteLength changed");
		}
	`)
	require.NoError(t, err)
}

func TestGetRandomValuesRejectsNegativeLength(t *testing.T) {
	t.Parallel()

	rt := newCryptoRuntime(t)

	_, err := rt.RunOnEventLoop(`
		crypto.getRandomValues({ constructor: Uint8Array, length: -1 });
	`)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), webcrypto.TypeMismatchError), "got %v", err)
}

func TestGetRandomValuesEmptyArray(t *testing.T) {
	t.Parallel()

	rt := newCryptoRuntime(t)

	_, err := rt.RunOnEventLoop(`
		const array = new Uint32Array(0);
		const ret = crypto.getRandomValues(array);
		if (ret !== array || array.length !== 0) {
			throw new Error("empty typed array should be returned unchanged");
		}
	`)
	require.NoError(t, err)
}
