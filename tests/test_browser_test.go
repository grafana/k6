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
