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

func TestSubtleCryptoImportExportKey(t *testing.T) {
	t.Parallel()

	t.Run("symmetric", func(t *testing.T) {
		t.Parallel()

		ts := newTestSetup(t)
		err := ts.rt.GlobalObject().Set("CryptoKey", CryptoKey{})
		require.NoError(t, err)

		gotScriptErr := ts.ev.Start(func() error {
			err := executeTestScripts(ts.rt, "./tests/import_export", "symmetric.js")

			return err
		})

		assert.NoError(t, gotScriptErr)
	})
}

func TestSubtleCryptoEncryptDecrypt(t *testing.T) {
	t.Parallel()

	t.Run("AES CBC", func(t *testing.T) {
		t.Parallel()

		ts := newTestSetup(t)

		gotScriptErr := ts.ev.Start(func() error {
			err := executeTestScripts(ts.rt, "./tests/encrypt_decrypt", "aes_cbc_vectors.js", "aes.js")
			require.NoError(t, err)

			_, err = ts.rt.RunString(`run_test()`)

			return err
		})

		assert.NoError(t, gotScriptErr)
	})

	t.Run("AES CTR", func(t *testing.T) {
		t.Parallel()

		ts := newTestSetup(t)

		gotScriptErr := ts.ev.Start(func() error {
			err := executeTestScripts(ts.rt, "./tests/encrypt_decrypt", "aes_ctr_vectors.js", "aes.js")
			require.NoError(t, err)

			_, err = ts.rt.RunString(`run_test()`)

			return err
		})

		assert.NoError(t, gotScriptErr)
	})

	// Note @oleiade: although the specification targets support
	// for various iv sizes, go AES GCM cipher only supports 96bits.
	// Thus, alghought the official WebPlatform test suite contains
	// vectors for various iv sizes, we only test the 96bits one.
	t.Run("AES GCM 96bits iv", func(t *testing.T) {
		t.Parallel()

		ts := newTestSetup(t)

		gotScriptErr := ts.ev.Start(func() error {
			err := executeTestScripts(ts.rt, "./tests/encrypt_decrypt", "aes_gcm_96_iv_fixtures.js", "aes_gcm_vectors.js", "aes.js")
			require.NoError(t, err)

			_, err = ts.rt.RunString(`run_test()`)

			return err
		})

		assert.NoError(t, gotScriptErr)
	})
}

func TestSubtleCryptoSignVerify(t *testing.T) {
	t.Parallel()

	t.Run("HMAC", func(t *testing.T) {
		t.Parallel()

		ts := newTestSetup(t)

		gotScriptErr := ts.ev.Start(func() error {
			err := executeTestScripts(ts.rt, "./tests/sign_verify", "hmac_vectors.js", "hmac.js")
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
