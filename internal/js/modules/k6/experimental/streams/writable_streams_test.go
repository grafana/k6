package streams

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWritableStream(t *testing.T) {
	t.Parallel()
	if _, err := os.Stat(webPlatformTestSuite); err != nil { //nolint:forbidigo
		t.Skipf("If you want to run Streams tests, you need to run the 'checkout.sh` script in the directory to get "+
			"https://github.com/web-platform-tests/wpt at the correct last tested commit (%v)", err)
	}

	suites := []string{
		"aborting.any.js",
		"bad-strategies.any.js",
		"bad-underlying-sinks.any.js",
		"close.any.js",
		"constructor.any.js",
		"count-queuing-strategy.any.js",
		"error.any.js",
		"floating-point-total-queue-size.any.js",
		"general.any.js",
		"properties.any.js",
		"reentrant-strategy.any.js",
		"start.any.js",
		"write.any.js",
	}

	for _, suite := range suites {
		t.Run(suite, func(t *testing.T) {
			t.Parallel()
			ts := newConfiguredRuntime(t)
			gotErr := ts.EventLoop.Start(func() error {
				return executeTestScript(ts.VU, webPlatformTestSuite+"/streams/writable-streams", suite)
			})
			assert.NoError(t, gotErr)
		})
	}
}
