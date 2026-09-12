package webcrypto_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/internal/js/modules/k6/webcrypto"
	"go.k6.io/k6/v2/js/modulestest"
)

func setupCryptoRuntime(t *testing.T) *modulestest.Runtime {
	t.Helper()

	rt := modulestest.NewRuntime(t)
	require.NoError(t, webcrypto.SetupGlobally(rt.VU))
	m := new(webcrypto.RootModule).NewModuleInstance(rt.VU)
	require.NoError(t, rt.VU.Runtime().Set("crypto", m.Exports().Named["crypto"]))

	return rt
}

func TestGetRandomValuesRejectsNegativeLength(t *testing.T) {
	t.Parallel()

	rt := setupCryptoRuntime(t)

	require.NotPanics(t, func() {
		_, err := rt.VU.Runtime().RunString(`
			const o = { length: -1, constructor: Uint8Array };
			crypto.getRandomValues(o);
		`)
		require.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "TypeMismatchError") ||
			strings.Contains(err.Error(), "invalid length"), "got: %v", err)
	})
}

func TestGetRandomValuesAcceptsEmptyTypedArray(t *testing.T) {
	t.Parallel()

	rt := setupCryptoRuntime(t)

	_, err := rt.VU.Runtime().RunString(`
		const a = new Uint8Array(0);
		const out = crypto.getRandomValues(a);
		if (out.length !== 0) throw new Error('expected empty result');
	`)
	require.NoError(t, err)
}
