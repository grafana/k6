// Part of the Web Platform Tests suite for the k6's WebCrypto API
//go:build wpt

package tests

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const webPlatformTestSuite = "./wpt/WebCryptoAPI/"

func TestWebPlatformTestSuite(t *testing.T) {
	t.Parallel()

	// check if the test is running in the correct environment
	info, err := os.Stat(webPlatformTestSuite)
	if os.IsNotExist(err) || err != nil || !info.IsDir() {
		t.Fatalf(
			"The Web Platform Test directory does not exist, err: %s. Please check webcrypto/tests/README.md how to setup it",
			err,
		)
	}

	tests := []struct {
		// catalog is the catalog relatively webPlatformTestSuite where to look files
		catalog string
		// files is the list of files to execute
		files []string
		// callFn is the function to call after the files are executed
		// if empty, no function will be called
		callFn string
	}{
		{
			catalog: "digest",
			files: []string{
				"digest.https.any.js",
			},
		},
		{
			catalog: "generateKey",
			files: []string{
				"successes.js",
			},
			callFn: "run_test",
		},
		{
			catalog: "generateKey",
			files: []string{
				"failures.js",
			},
			callFn: "run_test",
		},
		{
			catalog: "import_export",
			files: []string{
				"symmetric_importKey.https.any.js",
			},
		},
		{
			catalog: "import_export",
			files: []string{
				"ec_importKey.https.any.js",
			},
		},
		{
			catalog: "import_export",
			files: []string{
				"rsa_importKey.https.any.js",
			},
		},
		{
			catalog: "encrypt_decrypt",
			files: []string{
				"aes_cbc_vectors.js",
				"aes.js",
			},
			callFn: "run_test",
		},
		{
			catalog: "encrypt_decrypt",
			files: []string{
				"aes_ctr_vectors.js",
				"aes.js",
			},
			callFn: "run_test",
		},
		{
			// Note @oleiade: although the specification targets support
			// for various iv sizes, go AES GCM cipher only supports 96bits.
			// Thus, although the official WebPlatform test suite contains
			// vectors for various iv sizes, we only test the 96bits one.
			catalog: "encrypt_decrypt",
			files: []string{
				"aes_gcm_96_iv_fixtures.js",
				"aes_gcm_vectors.js",
				"aes.js",
			},
			callFn: "run_test",
		},
		{
			// RSA-OAEP
			catalog: "encrypt_decrypt",
			files: []string{
				"rsa_vectors.js",
				"rsa.js",
			},
			callFn: "run_test",
		},
		{
			catalog: "sign_verify",
			files: []string{
				"hmac_vectors.js", "hmac.js",
			},
			callFn: "run_test",
		},
		{
			catalog: "sign_verify",
			files: []string{
				"ecdsa_vectors.js",
				"ecdsa.js",
			},
			callFn: "run_test",
		},
		{
			catalog: "sign_verify",
			files: []string{
				"rsa_pss_vectors.js",
				"rsa.js",
			},
			callFn: "run_test",
		},
		{
			catalog: "sign_verify",
			files: []string{
				"rsa_pss_vectors.js", "rsa.js",
			},
			callFn: "run_test",
		},
		{
			catalog: "derive_bits_keys",
			files: []string{
				"ecdh_bits.js",
			},
			callFn: "define_tests",
		},
	}

	for _, tt := range tests {
		tt := tt
		testName := tt.catalog + "/" + strings.Join(tt.files, "_")

		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			ts := newConfiguredRuntime(t)

			gotErr := ts.EventLoop.Start(func() error {
				for _, script := range tt.files {
					compileAndRun(t, ts, webPlatformTestSuite+tt.catalog, script)
				}

				if tt.callFn == "" {
					return nil
				}

				_, err := ts.VU.Runtime().RunString(tt.callFn + `()`)
				return err
			})
			assert.NoError(t, gotErr)
		})
	}
}
