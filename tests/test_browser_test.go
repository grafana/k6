package tests

import (
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTestBrowserAwaitWithTimeoutShortCircuit(t *testing.T) {
	t.Parallel()
	tb := newTestBrowser(t)
	start := time.Now()
	require.NoError(t, tb.awaitWithTimeout(time.Second*10, func() error {
		runtime.Goexit() // this is what happens when a `require` fails
		return nil
	}))
	require.Less(t, time.Since(start), time.Second)
}

// testingT is a wrapper around testing.TB.
type testingT struct {
	testing.TB
	fatalfCalled bool
}

// Fatalf skips the test immediately after a test is calling it.
// This is useful when a test is expected to fail, but we don't
// want to mark it as a failure since it's expected.
func (t *testingT) Fatalf(format string, args ...any) {
	t.fatalfCalled = true
	t.SkipNow()
}

func TestTestBrowserWithLookupFunc(t *testing.T) {
	tt := &testingT{TB: t}

	// this lookup is expected to fail because the remote debugging port is
	// invalid, practically testing that the InitEnv.LookupEnv is used.
	lookup := func(key string) (string, bool) {
		if key == "K6_BROWSER_ARGS" {
			return "remote-debugging-port=99999", true
		}
		return "", false
	}
	_ = newTestBrowser(tt, withLookupFunc(lookup))
	require.True(t, tt.fatalfCalled)
}
