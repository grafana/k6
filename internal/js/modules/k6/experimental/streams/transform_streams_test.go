package streams

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTransformStream(t *testing.T) {
	t.Parallel()
	if _, err := os.Stat(webPlatformTestSuite); err != nil { //nolint:forbidigo
		t.Skipf("If you want to run Streams tests, you need to run the 'checkout.sh` script in the directory to get "+
			"https://github.com/web-platform-tests/wpt at the correct last tested commit (%v)", err)
	}

	suites := []string{
		"backpressure.any.js",
		"cancel.any.js",
		"errors.any.js",
		"flush.any.js",
		"general.any.js",
		"lipfuzz.any.js",
		"patched-global.any.js",
		"properties.any.js",
		"reentrant-strategies.any.js",
		"strategies.any.js",
		"terminate.any.js",
	}

	for _, suite := range suites {
		t.Run(suite, func(t *testing.T) {
			t.Parallel()
			ts := newConfiguredRuntime(t)
			gotErr := ts.EventLoop.Start(func() error {
				return executeTestScript(ts.VU, webPlatformTestSuite+"/streams/transform-streams", suite)
			})
			assert.NoError(t, gotErr)
		})
	}
}
