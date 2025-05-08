// Package tests runs part of the Web Platform Tests suite for the k6's WebCrypto API
package tests

import (
	"os"
	"strings"
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const webPlatformTestSuite = "./wpt/WebCryptoAPI/"

func TestWebPlatformTestSuite(t *testing.T) {
	t.Parallel()

	if _, err := os.Stat(webPlatformTestSuite); err != nil { //nolint:forbidigo
		t.Skipf("If you want to run WebCrypto tests, you need to run the 'checkout.sh` script in the directory to get "+
			"https://github.com/web-platform-tests/wpt at the correct last tested commit (%v)", err)
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
		testName := tt.catalog + "/" + strings.Join(tt.files, "_")

		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			ts := newConfiguredRuntime(t)
			// We compile the Web Platform testharness script into a sobek.Program
			compileAndRun(t, ts, "./wpt/resources", "testharness.js")

			gotErr := ts.EventLoop.Start(func() error {
				rt := ts.VU.Runtime()
				// https://web-platform-tests.org/writing-tests/testharness-api.html#callbacks
				// gets back the Test instance for each test when it finishes
				cal, ok := sobek.AssertFunction(rt.Get("add_result_callback"))
				require.True(t, ok)
				_, err := cal(sobek.Undefined(), rt.ToValue(func(test *sobek.Object) {
					// TODO(@mstoykov): In some future place do this better and potentially record
					// and expect failures on unimplemented stuff
					status := test.Get("status").ToInteger()
					t.Run(test.Get("name").String(), func(t *testing.T) {
						// Report issues
						assert.Equal(t, "null", test.Get("message").String())
						assert.Equal(t, "null", test.Get("stack").String())
						require.EqualValues(t, 0, status) // 0 is a PASS, all other values are some kind of failures
					})
				}))
				require.NoError(t, err)
				for _, script := range tt.files {
					compileAndRun(t, ts, webPlatformTestSuite+tt.catalog, script)
				}

				if tt.callFn == "" {
					return nil
				}

				_, err = rt.RunString(tt.callFn + `()`)
				return err
			})
			require.NoError(t, gotErr)
		})
	}
}
