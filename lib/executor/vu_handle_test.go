package executor

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/testutils"
	"github.com/loadimpact/k6/lib/testutils/minirunner"
	"github.com/loadimpact/k6/stats"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// this test is mostly interesting when -race is enabled
func TestVUHandleRace(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logHook := &testutils.SimpleLogrusHook{HookedLevels: []logrus.Level{logrus.DebugLevel}}
	testLog := logrus.New()
	testLog.AddHook(logHook)
	testLog.SetOutput(testutils.NewTestOutput(t))
	// testLog.Level = logrus.DebugLevel
	logEntry := logrus.NewEntry(testLog)

	getVU := func() (lib.InitializedVU, error) {
		return &minirunner.VU{
			R: &minirunner.MiniRunner{
				Fn: func(ctx context.Context, out chan<- stats.SampleContainer) error {
					// TODO: do something
					return nil
				},
			},
		}, nil
	}

	returnVU := func(_ lib.InitializedVU) {
		// do something
	}
	var interruptedIter int64
	var fullIterations int64

	runIter := func(ctx context.Context, vu lib.ActiveVU) bool {
		_ = vu.RunOnce()

		// TODO: track (non-ramp-down) errors from script iterations as a metric,
		// and have a default threshold that will abort the script when the error
		// rate exceeds a certain percentage

		select {
		case <-ctx.Done():
			// Don't log errors or emit iterations metrics from cancelled iterations
			atomic.AddInt64(&interruptedIter, 1)
			return false
		default:
			atomic.AddInt64(&fullIterations, 1)
			return true
		}
	}
	baseConfig := &BaseConfig{} // this is just being passed along
	vuHandle := newStoppedVUHandle(ctx, getVU, returnVU, baseConfig, logEntry)
	go vuHandle.runLoopsIfPossible(runIter)
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		for i := 0; i < 10000; i++ {
			err := vuHandle.start()
			require.NoError(t, err)
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			vuHandle.gracefulStop()
			time.Sleep(1 * time.Nanosecond)
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			vuHandle.hardStop()
			time.Sleep(10 * time.Nanosecond)
		}
	}()
	wg.Wait()
	vuHandle.hardStop() // STOP it
	time.Sleep(time.Millisecond)
	interruptedBefore := atomic.LoadInt64(&interruptedIter)
	fullBefore := atomic.LoadInt64(&fullIterations)
	_ = vuHandle.start()
	time.Sleep(time.Millisecond * 5) // just to be sure an iteration will squeeze in
	vuHandle.hardStop()
	time.Sleep(time.Millisecond)
	interruptedAfter := atomic.LoadInt64(&interruptedIter)
	fullAfter := atomic.LoadInt64(&fullIterations)
	assert.True(t, interruptedBefore >= interruptedAfter-1,
		"too big of a difference %d >= %d - 1", interruptedBefore, interruptedAfter)
	assert.True(t, fullBefore+1 <= fullAfter,
		"too small of a difference %d + 1 <= %d", fullBefore, fullAfter)
}
