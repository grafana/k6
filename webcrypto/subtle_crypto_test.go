package webcrypto

import (
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSubtleDigest tests that the cryptographic digests produced by
// the crypto.digest() are conform with the specification's expectations.
//
// It stands as the k6 counterpart of the equivalent [WPT test].
//
// [WPT test]: https://github.com/web-platform-tests/wpt/blob/master/WebCryptoAPI/digest/digest.https.any.js
func TestSubtleDigest(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)
	digestTestScript, err := CompileFile("./tests", "digest.js")
	assert.NoError(t, err)

	gotScriptErr := ts.ev.Start(func() error {
		_, err := ts.rt.RunProgram(digestTestScript)
		return err
	})

	assert.NoError(t, gotScriptErr)
}

func TestSubtleCryptoGenerateKey(t *testing.T) {
	t.Parallel()

	t.Run("successes", func(t *testing.T) {
		t.Parallel()

		ts := newTestSetup(t)
		err := ts.rt.GlobalObject().Set("CryptoKey", CryptoKey{})
		require.NoError(t, err)

		gotScriptErr := ts.ev.Start(func() error {
			err := executeTestScripts(ts.rt, "./tests/generateKey", "successes.js")
			require.NoError(t, err)

			_, err = ts.rt.RunString(`run_test()`)

			return err
		})

		assert.NoError(t, gotScriptErr)
	})

	t.Run("failures", func(t *testing.T) {
		t.Parallel()

		ts := newTestSetup(t)
		err := ts.rt.GlobalObject().Set("CryptoKey", CryptoKey{})
		require.NoError(t, err)

		gotScriptErr := ts.ev.Start(func() error {
			err := executeTestScripts(ts.rt, "./tests/generateKey", "failures.js")
			require.NoError(t, err)

			_, err = ts.rt.RunString(`run_test()`)

			return err
		})

		assert.NoError(t, gotScriptErr)
	})
}

func executeTestScripts(rt *goja.Runtime, base string, scripts ...string) error {
	for _, script := range scripts {
		program, err := CompileFile(base, script)
		if err != nil {
			return err
		}

		if _, err = rt.RunProgram(program); err != nil {
			return err
		}
	}

	return nil
}
