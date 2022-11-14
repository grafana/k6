package webcrypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
