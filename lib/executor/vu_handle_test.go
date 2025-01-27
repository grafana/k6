package executor

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/lib/testutils"
	"go.k6.io/k6/internal/lib/testutils/minirunner"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
)

func mockNextIterations() (uint64, uint64) {
	return 12, 15
}

// this test is mostly interesting when -race is enabled
func TestVUHandleRace(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logHook := testutils.NewLogHook(logrus.DebugLevel)
	testLog := logrus.New()
	testLog.AddHook(logHook)
	testLog.SetOutput(testutils.NewTestOutput(t))
	// testLog.Level = logrus.DebugLevel
	logEntry := logrus.NewEntry(testLog)

	runner := &minirunner.MiniRunner{}
	runner.Fn = func(_ context.Context, _ *lib.State, _ chan<- metrics.SampleContainer) error {
		return nil
	}

	var getVUCount uint64
	var returnVUCount uint64
	getVU := func() (lib.InitializedVU, error) {
		return runner.NewVU(ctx, atomic.AddUint64(&getVUCount, 1), 0, nil)
	}

	returnVU := func(_ lib.InitializedVU) {
		atomic.AddUint64(&returnVUCount, 1)
		// do something
	}
	var interruptedIter int64
	var fullIterations int64

	runIter := func(ctx context.Context, vu lib.ActiveVU) bool {
		_ = vu.RunOnce()
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

	vuHandle := newStoppedVUHandle(ctx, getVU, returnVU, mockNextIterations, &BaseConfig{}, logEntry)
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
	time.Sleep(time.Millisecond * 50)
	interruptedBefore := atomic.LoadInt64(&interruptedIter)
	fullBefore := atomic.LoadInt64(&fullIterations)
	_ = vuHandle.start()
	time.Sleep(time.Millisecond * 50) // just to be sure an iteration will squeeze in
	cancel()
	time.Sleep(time.Millisecond * 50)
	interruptedAfter := atomic.LoadInt64(&interruptedIter)
	fullAfter := atomic.LoadInt64(&fullIterations)
	assert.True(t, interruptedBefore >= interruptedAfter-1,
		"too big of a difference %d >= %d - 1", interruptedBefore, interruptedAfter)
	assert.True(t, fullBefore+1 <= fullAfter,
		"too small of a difference %d + 1 <= %d", fullBefore, fullAfter)
	require.Equal(t, atomic.LoadUint64(&getVUCount), atomic.LoadUint64(&returnVUCount))
}

// this test is mostly interesting when -race is enabled
func TestVUHandleStartStopRace(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logHook := testutils.NewLogHook(logrus.DebugLevel)
	testLog := logrus.New()
	testLog.AddHook(logHook)
	testLog.SetOutput(testutils.NewTestOutput(t))
	// testLog.Level = logrus.DebugLevel
	logEntry := logrus.NewEntry(testLog)

	runner := &minirunner.MiniRunner{}
	runner.Fn = func(_ context.Context, _ *lib.State, _ chan<- metrics.SampleContainer) error {
		return nil
	}

	var vuID uint64
	testIterations := 10000
	returned := make(chan struct{})

	getVU := func() (lib.InitializedVU, error) {
		returned = make(chan struct{})
		return runner.NewVU(ctx, atomic.AddUint64(&vuID, 1), 0, nil)
	}

	returnVU := func(v lib.InitializedVU) {
		require.Equal(t, atomic.LoadUint64(&vuID), v.(*minirunner.VU).ID)
		close(returned)
	}
	var interruptedIter int64
	var fullIterations int64

	runIter := func(ctx context.Context, vu lib.ActiveVU) bool {
		_ = vu.RunOnce()
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

	vuHandle := newStoppedVUHandle(ctx, getVU, returnVU, mockNextIterations, &BaseConfig{}, logEntry)
	go vuHandle.runLoopsIfPossible(runIter)
	for i := 0; i < testIterations; i++ {
		err := vuHandle.start()
		vuHandle.gracefulStop()
		require.NoError(t, err)
		select {
		case <-returned:
		case <-time.After(100 * time.Millisecond):
			go panic("returning took too long")
			time.Sleep(time.Second)
		}
	}

	vuHandle.hardStop() // STOP it
	time.Sleep(time.Millisecond * 5)
	interruptedBefore := atomic.LoadInt64(&interruptedIter)
	fullBefore := atomic.LoadInt64(&fullIterations)
	_ = vuHandle.start()
	time.Sleep(time.Millisecond * 50) // just to be sure an iteration will squeeze in
	cancel()
	time.Sleep(time.Millisecond * 5)
	interruptedAfter := atomic.LoadInt64(&interruptedIter)
	fullAfter := atomic.LoadInt64(&fullIterations)
	assert.True(t, interruptedBefore >= interruptedAfter-1,
		"too big of a difference %d >= %d - 1", interruptedBefore, interruptedAfter)
	assert.True(t, fullBefore+1 <= fullAfter,
		"too small of a difference %d + 1 <= %d", fullBefore, fullAfter)
}

type handleVUTest struct {
	runner          *minirunner.MiniRunner
	getVUCount      uint32
	returnVUCount   uint32
	interruptedIter int64
	fullIterations  int64
}

func (h *handleVUTest) getVU() (lib.InitializedVU, error) {
	return h.runner.NewVU(context.Background(), uint64(atomic.AddUint32(&h.getVUCount, 1)), 0, nil)
}

func (h *handleVUTest) returnVU(_ lib.InitializedVU) {
	atomic.AddUint32(&h.returnVUCount, 1)
}

func (h *handleVUTest) runIter(ctx context.Context, _ lib.ActiveVU) bool {
	select {
	case <-time.After(time.Second):
	case <-ctx.Done():
	}

	select {
	case <-ctx.Done():
		// Don't log errors or emit iterations metrics from cancelled iterations
		atomic.AddInt64(&h.interruptedIter, 1)
		return false
	default:
		atomic.AddInt64(&h.fullIterations, 1)
		return true
	}
}

func TestVUHandleSimple(t *testing.T) {
	t.Parallel()

	t.Run("start before gracefulStop finishes", func(t *testing.T) {
		t.Parallel()
		logHook := testutils.NewLogHook(logrus.DebugLevel)
		testLog := logrus.New()
		testLog.AddHook(logHook)
		testLog.SetOutput(testutils.NewTestOutput(t))
		// testLog.Level = logrus.DebugLevel
		logEntry := logrus.NewEntry(testLog)
		test := &handleVUTest{runner: &minirunner.MiniRunner{}}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		vuHandle := newStoppedVUHandle(ctx, test.getVU, test.returnVU, mockNextIterations, &BaseConfig{}, logEntry)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			vuHandle.runLoopsIfPossible(test.runIter)
		}()
		err := vuHandle.start()
		require.NoError(t, err)
		time.Sleep(time.Millisecond * 50)
		vuHandle.gracefulStop()
		// time.Sleep(time.Millisecond * 5) // No sleep as we want to always not return the VU
		err = vuHandle.start()
		require.NoError(t, err)
		time.Sleep(time.Millisecond * 1500)
		assert.EqualValues(t, 1, atomic.LoadUint32(&test.getVUCount))
		assert.EqualValues(t, 0, atomic.LoadUint32(&test.returnVUCount))
		assert.EqualValues(t, 0, atomic.LoadInt64(&test.interruptedIter))
		assert.EqualValues(t, 1, atomic.LoadInt64(&test.fullIterations))
		cancel()
		wg.Wait()
		time.Sleep(time.Millisecond * 5)
		assert.EqualValues(t, 1, atomic.LoadUint32(&test.getVUCount))
		assert.EqualValues(t, 1, atomic.LoadUint32(&test.returnVUCount))
		assert.EqualValues(t, 1, atomic.LoadInt64(&test.interruptedIter))
		assert.EqualValues(t, 1, atomic.LoadInt64(&test.fullIterations))
	})

	t.Run("start after gracefulStop finishes", func(t *testing.T) {
		t.Parallel()
		logHook := testutils.NewLogHook(logrus.DebugLevel)
		testLog := logrus.New()
		testLog.AddHook(logHook)
		testLog.SetOutput(testutils.NewTestOutput(t))
		// testLog.Level = logrus.DebugLevel
		logEntry := logrus.NewEntry(testLog)
		test := &handleVUTest{runner: &minirunner.MiniRunner{}}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		vuHandle := newStoppedVUHandle(ctx, test.getVU, test.returnVU, mockNextIterations, &BaseConfig{}, logEntry)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			vuHandle.runLoopsIfPossible(test.runIter)
		}()
		err := vuHandle.start()
		require.NoError(t, err)
		time.Sleep(time.Millisecond * 50)
		vuHandle.gracefulStop()
		time.Sleep(time.Millisecond * 1500)
		assert.EqualValues(t, 1, atomic.LoadUint32(&test.getVUCount))
		assert.EqualValues(t, 1, atomic.LoadUint32(&test.returnVUCount))
		assert.EqualValues(t, 0, atomic.LoadInt64(&test.interruptedIter))
		assert.EqualValues(t, 1, atomic.LoadInt64(&test.fullIterations))
		err = vuHandle.start()
		require.NoError(t, err)
		time.Sleep(time.Millisecond * 1500)
		cancel()
		wg.Wait()

		time.Sleep(time.Millisecond * 50)
		assert.EqualValues(t, 2, atomic.LoadUint32(&test.getVUCount))
		assert.EqualValues(t, 2, atomic.LoadUint32(&test.returnVUCount))
		assert.EqualValues(t, 1, atomic.LoadInt64(&test.interruptedIter))
		assert.EqualValues(t, 2, atomic.LoadInt64(&test.fullIterations))
	})

	t.Run("start after hardStop", func(t *testing.T) {
		t.Parallel()
		logHook := testutils.NewLogHook(logrus.DebugLevel)
		testLog := logrus.New()
		testLog.AddHook(logHook)
		testLog.SetOutput(testutils.NewTestOutput(t))
		// testLog.Level = logrus.DebugLevel
		logEntry := logrus.NewEntry(testLog)
		test := &handleVUTest{runner: &minirunner.MiniRunner{}}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		vuHandle := newStoppedVUHandle(ctx, test.getVU, test.returnVU, mockNextIterations, &BaseConfig{}, logEntry)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			vuHandle.runLoopsIfPossible(test.runIter)
		}()
		err := vuHandle.start()
		require.NoError(t, err)
		time.Sleep(time.Millisecond * 5)
		vuHandle.hardStop()
		time.Sleep(time.Millisecond * 15)
		assert.EqualValues(t, 1, atomic.LoadUint32(&test.getVUCount))
		assert.EqualValues(t, 1, atomic.LoadUint32(&test.returnVUCount))
		assert.EqualValues(t, 1, atomic.LoadInt64(&test.interruptedIter))
		assert.EqualValues(t, 0, atomic.LoadInt64(&test.fullIterations))
		err = vuHandle.start()
		require.NoError(t, err)
		time.Sleep(time.Millisecond * 1500)
		cancel()
		wg.Wait()

		time.Sleep(time.Millisecond * 5)
		assert.EqualValues(t, 2, atomic.LoadUint32(&test.getVUCount))
		assert.EqualValues(t, 2, atomic.LoadUint32(&test.returnVUCount))
		assert.EqualValues(t, 2, atomic.LoadInt64(&test.interruptedIter))
		assert.EqualValues(t, 1, atomic.LoadInt64(&test.fullIterations))
	})
}

func BenchmarkVUHandleIterations(b *testing.B) {
	logHook := testutils.NewLogHook(logrus.DebugLevel)
	testLog := logrus.New()
	testLog.AddHook(logHook)
	// testLog.Level = logrus.DebugLevel
	logEntry := logrus.NewEntry(testLog)

	var (
		getVUCount      uint32
		returnVUCount   uint32
		interruptedIter int64
		fullIterations  int64
	)
	reset := func() {
		getVUCount = 0
		returnVUCount = 0
		interruptedIter = 0
		fullIterations = 0
	}

	runner := &minirunner.MiniRunner{}
	runner.Fn = func(_ context.Context, _ *lib.State, _ chan<- metrics.SampleContainer) error {
		return nil
	}
	getVU := func() (lib.InitializedVU, error) {
		return runner.NewVU(context.Background(), uint64(atomic.AddUint32(&getVUCount, 1)), 0, nil)
	}

	returnVU := func(_ lib.InitializedVU) {
		atomic.AddUint32(&returnVUCount, 1)
	}

	runIter := func(ctx context.Context, _ lib.ActiveVU) bool {
		// Do nothing
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

	reset()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	vuHandle := newStoppedVUHandle(ctx, getVU, returnVU, mockNextIterations, &BaseConfig{}, logEntry)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		vuHandle.runLoopsIfPossible(runIter)
	}()
	start := time.Now()
	b.ResetTimer()
	err := vuHandle.start()
	require.NoError(b, err)
	time.Sleep(time.Second)
	cancel()
	wg.Wait()
	b.StopTimer()
	took := time.Since(start)
	b.ReportMetric(float64(atomic.LoadInt64(&fullIterations))/float64(took), "iterations/ns")
}
