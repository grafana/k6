package encoding

import (
	"testing"

	"github.com/stretchr/testify/require"
)

//
// [WPT test]: https://github.com/web-platform-tests/wpt/blob/b5e12f331494f9533ef6211367dace2c88131fd7/encoding/
func TestTextEncoder(t *testing.T) {
	t.Parallel()
	scripts := []testScript{
		{base: "./tests", path: "textencoder-constructor-non-utf.js"},
		{base: "./tests", path: "textencoder-utf16-surrogates.js"},
	}

	ts := newTestSetup(t)
	err := executeTestScripts(ts, scripts)
	require.NoError(t, err)
}
