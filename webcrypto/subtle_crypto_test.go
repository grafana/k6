package webcrypto

import (
	"testing"

	"github.com/grafana/sobek"
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

	digestTestScript, err := CompileFile("./tests", "digest.js")
	assert.NoError(t, err)

	ts := newConfiguredRuntime(t)
	gotErr := ts.EventLoop.Start(func() error {
		_, programErr := ts.VU.Runtime().RunProgram(digestTestScript)
		return programErr
	})

	assert.NoError(t, gotErr)
}

func TestSubtleCryptoGenerateKey(t *testing.T) {
	t.Parallel()

	t.Run("successes", func(t *testing.T) {
		t.Parallel()

		ts := newConfiguredRuntime(t)

		gotErr := ts.EventLoop.Start(func() error {
			err := executeTestScripts(ts.VU.Runtime(), "./tests/generateKey", "successes.js")
			require.NoError(t, err)

			_, err = ts.VU.Runtime().RunString(`run_test()`)

			return err
		})

		assert.NoError(t, gotErr)
	})

	t.Run("failures", func(t *testing.T) {
		t.Parallel()

		ts := newConfiguredRuntime(t)

		gotErr := ts.EventLoop.Start(func() error {
			err := executeTestScripts(ts.VU.Runtime(), "./tests/generateKey", "failures.js")
			require.NoError(t, err)

			_, err = ts.VU.Runtime().RunString(`run_test()`)

			return err
		})

		assert.NoError(t, gotErr)
	})
}

func TestSubtleCryptoImportExportKey(t *testing.T) {
	t.Parallel()

	t.Run("symmetric", func(t *testing.T) {
		t.Parallel()

		ts := newConfiguredRuntime(t)

		gotErr := ts.EventLoop.Start(func() error {
			err := executeTestScripts(ts.VU.Runtime(), "./tests/import_export", "symmetric.js")

			return err
		})

		assert.NoError(t, gotErr)
	})

	t.Run("elliptic-curves", func(t *testing.T) {
		t.Parallel()

		ts := newConfiguredRuntime(t)

		gotErr := ts.EventLoop.Start(func() error {
			err := executeTestScripts(ts.VU.Runtime(), "./tests/import_export", "ec_importKey.js")

			return err
		})

		assert.NoError(t, gotErr)
	})
}

func TestSubtleCryptoEncryptDecrypt(t *testing.T) {
	t.Parallel()

	t.Run("AES CBC", func(t *testing.T) {
		t.Parallel()

		ts := newConfiguredRuntime(t)

		gotErr := ts.EventLoop.Start(func() error {
			err := executeTestScripts(ts.VU.Runtime(), "./tests/encrypt_decrypt", "aes_cbc_vectors.js", "aes.js")
			require.NoError(t, err)

			_, err = ts.VU.Runtime().RunString(`run_test()`)

			return err
		})

		assert.NoError(t, gotErr)
	})

	t.Run("AES CTR", func(t *testing.T) {
		t.Parallel()

		ts := newConfiguredRuntime(t)

		gotErr := ts.EventLoop.Start(func() error {
			err := executeTestScripts(ts.VU.Runtime(), "./tests/encrypt_decrypt", "aes_ctr_vectors.js", "aes.js")
			require.NoError(t, err)

			_, err = ts.VU.Runtime().RunString(`run_test()`)

			return err
		})

		assert.NoError(t, gotErr)
	})

	// Note @oleiade: although the specification targets support
	// for various iv sizes, go AES GCM cipher only supports 96bits.
	// Thus, although the official WebPlatform test suite contains
	// vectors for various iv sizes, we only test the 96bits one.
	t.Run("AES GCM 96bits iv", func(t *testing.T) {
		t.Parallel()

		ts := newConfiguredRuntime(t)

		gotErr := ts.EventLoop.Start(func() error {
			err := executeTestScripts(ts.VU.Runtime(), "./tests/encrypt_decrypt", "aes_gcm_96_iv_fixtures.js", "aes_gcm_vectors.js", "aes.js")
			require.NoError(t, err)

			_, err = ts.VU.Runtime().RunString(`run_test()`)

			return err
		})

		assert.NoError(t, gotErr)
	})
}

func TestSubtleCryptoSignVerify(t *testing.T) {
	t.Parallel()

	t.Run("HMAC", func(t *testing.T) {
		t.Parallel()

		ts := newConfiguredRuntime(t)

		gotErr := ts.EventLoop.Start(func() error {
			err := executeTestScripts(ts.VU.Runtime(), "./tests/sign_verify", "hmac_vectors.js", "hmac.js")
			require.NoError(t, err)

			_, err = ts.VU.Runtime().RunString(`run_test()`)

			return err
		})

		assert.NoError(t, gotErr)
	})

	t.Run("ECDSA", func(t *testing.T) {
		t.Parallel()

		ts := newConfiguredRuntime(t)

		gotErr := ts.EventLoop.Start(func() error {
			err := executeTestScripts(ts.VU.Runtime(), "./tests/sign_verify", "ecdsa_vectors.js", "ecdsa.js")
			require.NoError(t, err)

			_, err = ts.VU.Runtime().RunString(`run_test()`)

			return err
		})

		assert.NoError(t, gotErr)
	})
}

func TestSubtleCryptoDeriveBitsKeys(t *testing.T) {
	t.Parallel()

	t.Run("ecdh", func(t *testing.T) {
		t.Parallel()

		ts := newConfiguredRuntime(t)

		gotErr := ts.EventLoop.Start(func() error {
			err := executeTestScripts(ts.VU.Runtime(), "./tests/derive_bits_keys", "ecdh_bits.js")

			return err
		})

		assert.NoError(t, gotErr)
	})
}

func executeTestScripts(rt *sobek.Runtime, base string, scripts ...string) error {
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
