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

// filterPNGs returns only the frames whose path ends in ".png". Tests
// that exercise the dedup path get JSON sidecar writes interleaved
// with screenshot writes; use this helper when only the screenshots
// matter for the assertion.
func filterPNGs(frames []persistedFrame) []persistedFrame {
	out := make([]persistedFrame, 0, len(frames))
	for _, f := range frames {
		if strings.HasSuffix(f.path, ".png") {
			out = append(out, f)
		}
	}
	return out
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
		DedupEnabled: true,
	})

	wantBytes := []byte{0xde, 0xad, 0xbe, 0xef}
	start := time.Now()
	c.Capture(context.Background(), "action", "Test.action", func(_ context.Context) ([]byte, error) {
		return wantBytes, nil
	}, nil)

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
		DedupEnabled: true,
	})

	same := []byte{1, 2, 3, 4, 5}
	fn := func(_ context.Context) ([]byte, error) { return same, nil }

	c.Capture(context.Background(), "action", "Test.action", fn, nil)
	c.Capture(context.Background(), "action", "Test.action", fn, nil)
	c.Capture(context.Background(), "action", "Test.action", fn, nil)

	c.Close()

	// Filter out the dedup sidecar (.json) writes; we're asserting on
	// persisted screenshots only.
	pngs := filterPNGs(p.snapshot())
	assert.Len(t, pngs, 1)
	assert.Equal(t, uint64(0), c.Dropped(), "dedup is not a drop")
}

// When DedupEnabled is false, byte-identical consecutive frames must
// all persist. Mirrors K6_BROWSER_AUTO_SCREENSHOT_DEDUP=false at the
// Capturer layer. Sidecar JSONs MUST NOT be written in this mode
// because the dedup bucket is never populated.
func TestCapturer_DedupDisabled_PersistsEveryFrame(t *testing.T) {
	t.Parallel()

	p := newRecordingPersister()
	c := NewCapturer(CapturerOptions{
		Persister:    p,
		Logger:       log.NewNullLogger(),
		TestName:     "demo",
		VU:           1,
		Iter:         0,
		BufferSize:   10,
		DedupEnabled: false,
	})

	same := []byte{1, 2, 3, 4, 5}
	fn := func(_ context.Context) ([]byte, error) { return same, nil }

	c.Capture(context.Background(), "action", "Test.action", fn, nil)
	c.Capture(context.Background(), "action", "Test.action", fn, nil)
	c.Capture(context.Background(), "action", "Test.action", fn, nil)

	c.Close()

	pngs := filterPNGs(p.snapshot())
	assert.Len(t, pngs, 3, "every triggered frame must persist when dedup is off")

	for _, f := range p.snapshot() {
		assert.False(t, strings.HasSuffix(f.path, ".json"),
			"no sidecars should be written when dedup is disabled, got %q", f.path)
	}

	assert.Equal(t, uint64(0), c.Dropped())
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
		DedupEnabled: true,
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

	c.Capture(context.Background(), "action", "Test.action", makeFn(), nil)
	p.waitStart(t)

	// The worker is now blocked in Persist. The buffer can hold up to 3.
	// Push 6 more: 3 fit, 3 must drop the oldest.
	for range 6 {
		c.Capture(context.Background(), "action", "Test.action", makeFn(), nil)
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
		DedupEnabled: true,
	})

	const n = 5
	for i := range n {
		c.Capture(context.Background(), "action", "Test.action", func(_ context.Context) ([]byte, error) {
			return []byte{byte(i), 0xff}, nil
		}, nil)
	}

	c.Close()

	assert.Len(t, p.snapshot(), n)
}

func TestCapturer_NilSafe(t *testing.T) {
	t.Parallel()

	var c *Capturer
	c.Capture(context.Background(), "action", "Test.action", func(_ context.Context) ([]byte, error) {
		return nil, nil
	}, nil)
	c.CaptureForced(context.Background(), "failure", "Test.failure", func(_ context.Context) ([]byte, error) {
		return nil, nil
	}, nil)
	c.Close()
	assert.Equal(t, uint64(0), c.Dropped())
}

func TestCapturer_CaptureForced_BypassesDedup(t *testing.T) {
	t.Parallel()

	p := newRecordingPersister()
	c := NewCapturer(CapturerOptions{
		Persister:  p,
		Logger:     log.NewNullLogger(),
		TestName:   "demo",
		VU:         1,
		Iter:       0,
		BufferSize: 10,
		DedupEnabled: true,
	})

	// All four calls return byte-identical screenshots so the dedup
	// path would normally collapse three of the four. CaptureForced
	// must persist regardless: the failure-debugging use case needs
	// the "state at the moment of failure" even when it matches the
	// preceding successful action.
	same := []byte{0xaa, 0xbb, 0xcc}
	fn := func(_ context.Context) ([]byte, error) { return same, nil }

	c.Capture(context.Background(), "action", "Test.action", fn, nil)        // persist
	c.Capture(context.Background(), "action", "Test.action", fn, nil)        // dedup
	c.CaptureForced(context.Background(), "failure", "Test.failure", fn, nil) // persist (forced)
	c.Capture(context.Background(), "action", "Test.action", fn, nil)        // dedup

	c.Close()

	// Filter out the dedup sidecar (.json) writes; we're asserting on
	// persisted screenshots only.
	frames := filterPNGs(p.snapshot())
	require.Len(t, frames, 2)
	assert.Contains(t, frames[0].path, "action")
	assert.Contains(t, frames[1].path, "failure")
}
