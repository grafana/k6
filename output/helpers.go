package output

import (
	"fmt"
	"sync"
	"time"

	"go.k6.io/k6/metrics"
)

// SampleBuffer is a simple thread-safe buffer for metric samples. It should be
// used by most outputs, since we generally want to flush metric samples to the
// remote service asynchronously. We want to do it only every several seconds,
// and we don't want to block the Engine in the meantime.
type SampleBuffer struct {
	sync.Mutex
	buffer []metrics.SampleContainer
	maxLen int
}

// AddMetricSamples adds the given metric samples to the internal buffer.
func (sc *SampleBuffer) AddMetricSamples(samples []metrics.SampleContainer) {
	if len(samples) == 0 {
		return
	}
	sc.Lock()
	sc.buffer = append(sc.buffer, samples...)
	sc.Unlock()
}

// GetBufferedSamples returns the currently buffered metric samples and makes a
// new internal buffer with some hopefully realistic size. If the internal
// buffer is empty, it will return nil.
func (sc *SampleBuffer) GetBufferedSamples() []metrics.SampleContainer {
	sc.Lock()
	defer sc.Unlock()

	buffered, bufferedLen := sc.buffer, len(sc.buffer)
	if bufferedLen == 0 {
		return nil
	}
	if bufferedLen > sc.maxLen {
		sc.maxLen = bufferedLen
	}
	// Make the new buffer halfway between the previously allocated size and the
	// maximum buffer size we've seen so far, to hopefully reduce copying a bit.
	sc.buffer = make([]metrics.SampleContainer, 0, (bufferedLen+sc.maxLen)/2)

	return buffered
}

// PeriodicFlusher is a small helper for asynchronously flushing buffered metric
// samples on regular intervals. The biggest benefit is having a Stop() method
// that waits for one last flush before it returns.
type PeriodicFlusher struct {
	period        time.Duration
	flushCallback func()
	stop          chan struct{}
	stopped       chan struct{}
	once          *sync.Once
}

func (pf *PeriodicFlusher) run() {
	ticker := time.NewTicker(pf.period)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			pf.flushCallback()
		case <-pf.stop:
			pf.flushCallback()
			close(pf.stopped)
			return
		}
	}
}

// Stop waits for the periodic flusher flush one last time and exit. You can
// safely call Stop() multiple times from different goroutines, you just can't
// call it from inside of the flushing function.
func (pf *PeriodicFlusher) Stop() {
	pf.once.Do(func() {
		close(pf.stop)
	})
	<-pf.stopped
}

// NewPeriodicFlusher creates a new PeriodicFlusher and starts its goroutine.
func NewPeriodicFlusher(period time.Duration, flushCallback func()) (*PeriodicFlusher, error) {
	if period <= 0 {
		return nil, fmt.Errorf("metric flush period should be positive but was %s", period)
	}

	pf := &PeriodicFlusher{
		period:        period,
		flushCallback: flushCallback,
		stop:          make(chan struct{}),
		stopped:       make(chan struct{}),
		once:          &sync.Once{},
	}

	go pf.run()

	return pf, nil
}
