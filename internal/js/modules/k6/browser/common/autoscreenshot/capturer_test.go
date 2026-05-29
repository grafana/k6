package autoscreenshot

import (
	"context"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/log"
)

// persistedFrame records a single call to Persist for assertion in tests.
type persistedFrame struct {
	path  string
	bytes []byte
	at    time.Time
}

// recordingPersister captures every Persist call. When startCh is non-nil,
// the first Persist call closes it to signal that the worker has reached
// the persister. When block is non-nil, Persist blocks on it until closed.
type recordingPersister struct {
	startOnce sync.Once
	startCh   chan struct{}
	block     chan struct{}

	mu     sync.Mutex
	frames []persistedFrame
}

func newRecordingPersister() *recordingPersister {
	return &recordingPersister{}
}

func newBlockingPersister() *recordingPersister {
	return &recordingPersister{
		startCh: make(chan struct{}),
		block:   make(chan struct{}),
	}
}

func (p *recordingPersister) release() {
	if p.block != nil {
		close(p.block)
	}
}

func (p *recordingPersister) waitStart(t *testing.T) {
	t.Helper()
	select {
	case <-p.startCh:
	case <-time.After(2 * time.Second):
		t.Fatal("persister never received its first Persist call")
	}
}

func (p *recordingPersister) Persist(_ context.Context, path string, data io.Reader) error {
	if p.startCh != nil {
		p.startOnce.Do(func() { close(p.startCh) })
	}
	if p.block != nil {
		<-p.block
	}
	b, err := io.ReadAll(data)
	if err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.frames = append(p.frames, persistedFrame{path: path, bytes: b, at: time.Now()})
	return nil
}

func (p *recordingPersister) snapshot() []persistedFrame {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]persistedFrame, len(p.frames))
	copy(out, p.frames)
	return out
}

func TestCapturer(t *testing.T) {
	t.Parallel()

	p := newRecordingPersister()
	c := NewCapturer(CapturerOptions{
		Persister:  p,
		Logger:     log.NewNullLogger(),
		TestName:   "demo",
		VU:         1,
		Iter:       0,
		BufferSize: 10,
	})

	wantBytes := []byte{0xde, 0xad, 0xbe, 0xef}
	start := time.Now()
	c.Capture(context.Background(), "action", func(_ context.Context) ([]byte, error) {
		return wantBytes, nil
	})

	c.Close()

	frames := p.snapshot()
	require.Len(t, frames, 1)
	assert.Equal(t, wantBytes, frames[0].bytes)
	assert.Contains(t, frames[0].path, "demo")
	assert.Contains(t, frames[0].path, "vu-1")
	assert.Contains(t, frames[0].path, "iter-0")
	assert.Contains(t, frames[0].path, "action")
	assert.True(t, strings.HasSuffix(frames[0].path, ".png"))
	assert.False(t, frames[0].at.Before(start.Truncate(time.Millisecond)))
	assert.Equal(t, uint64(0), c.Dropped())
}

func TestCapturer_DedupsIdenticalFrames(t *testing.T) {
	t.Parallel()

	p := newRecordingPersister()
	c := NewCapturer(CapturerOptions{
		Persister:  p,
		Logger:     log.NewNullLogger(),
		TestName:   "demo",
		VU:         1,
		Iter:       0,
		BufferSize: 10,
	})

	same := []byte{1, 2, 3, 4, 5}
	fn := func(_ context.Context) ([]byte, error) { return same, nil }

	c.Capture(context.Background(), "action", fn)
	c.Capture(context.Background(), "action", fn)
	c.Capture(context.Background(), "action", fn)

	c.Close()

	frames := p.snapshot()
	assert.Len(t, frames, 1)
	assert.Equal(t, uint64(0), c.Dropped(), "dedup is not a drop")
}

func TestCapturer_DropsOldestOnBackpressure(t *testing.T) {
	t.Parallel()

	p := newBlockingPersister()
	c := NewCapturer(CapturerOptions{
		Persister:  p,
		Logger:     log.NewNullLogger(),
		TestName:   "demo",
		VU:         1,
		Iter:       0,
		BufferSize: 3,
	})

	// First capture is pulled by the worker and blocks in the persister.
	// Wait for that to happen so the buffer state is deterministic.
	var counter atomic.Uint64
	makeFn := func() CaptureFunc {
		n := counter.Add(1)
		return func(_ context.Context) ([]byte, error) {
			// Distinct bytes so the dedup path doesn't interfere.
			return []byte{byte(n)}, nil
		}
	}

	c.Capture(context.Background(), "action", makeFn())
	p.waitStart(t)

	// The worker is now blocked in Persist. The buffer can hold up to 3.
	// Push 6 more: 3 fit, 3 must drop the oldest.
	for range 6 {
		c.Capture(context.Background(), "action", makeFn())
	}

	// Give the buffer a moment to settle. All push operations are
	// synchronous (no goroutines spawned per Capture), so this is just a
	// safety margin against scheduling noise.
	time.Sleep(20 * time.Millisecond)

	require.Equal(t, uint64(3), c.Dropped(), "expected 3 oldest drops")

	p.release()
	c.Close()

	frames := p.snapshot()
	// 1 in-flight + 3 buffered = 4 persisted. 3 dropped. Total = 7.
	assert.Len(t, frames, 4)
	assert.Equal(t, uint64(7), uint64(len(frames))+c.Dropped())
}

func TestCapturer_CloseDrainsPending(t *testing.T) {
	t.Parallel()

	p := newRecordingPersister()
	c := NewCapturer(CapturerOptions{
		Persister:  p,
		Logger:     log.NewNullLogger(),
		TestName:   "demo",
		VU:         1,
		Iter:       0,
		BufferSize: 100,
	})

	const n = 5
	for i := range n {
		c.Capture(context.Background(), "action", func(_ context.Context) ([]byte, error) {
			return []byte{byte(i), 0xff}, nil
		})
	}

	c.Close()

	assert.Len(t, p.snapshot(), n)
}

func TestCapturer_NilSafe(t *testing.T) {
	t.Parallel()

	var c *Capturer
	c.Capture(context.Background(), "action", func(_ context.Context) ([]byte, error) {
		return nil, nil
	})
	c.Close()
	assert.Equal(t, uint64(0), c.Dropped())
}
