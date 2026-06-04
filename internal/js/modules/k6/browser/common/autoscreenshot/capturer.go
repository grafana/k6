// Package autoscreenshot provides the per-iteration screenshot capturer used
// by the browser module's auto-screenshot feature.
//
// A Capturer accepts capture requests on any goroutine and dispatches them to
// a single worker goroutine for execution. The worker invokes the supplied
// CaptureFunc to obtain encoded screenshot bytes, deduplicates identical
// consecutive frames via CRC32, and hands non-duplicate frames to a Persister
// for asynchronous storage. A bounded ring buffer with drop-oldest semantics
// protects against backpressure from slow capture or persist operations.
//
// Capturers are typically held by a Registry, which scopes them to a (VU,
// iteration) pair and tears them down at iteration end.
package autoscreenshot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/log"
)

// Persister persists captured screenshot bytes to a path. It is structurally
// identical to common.ScreenshotPersister and browser.filePersister; declaring
// it locally avoids importing common (which would create a dependency cycle
// once common code starts calling into this package).
type Persister interface {
	Persist(ctx context.Context, path string, data io.Reader) error
}

// CaptureFunc executes the actual screenshot operation. It runs on the
// Capturer's worker goroutine, so it must be safe to call there and must not
// block on the goroutine that called Capture.
//
// Declared as a type alias rather than a defined type so the Capture
// method's signature is structurally identical to the corresponding hook
// interface expected by lifecycle/change watchers in other packages.
type CaptureFunc = func(ctx context.Context) ([]byte, error)

// CapturerOptions holds the dependencies and per-iteration metadata used by a
// Capturer. All fields except Logger are required; an unset Logger defaults
// to a discarding logger.
type CapturerOptions struct {
	Persister  Persister
	Logger     *log.Logger
	TestName   string
	VU         uint64
	Iter       int64
	BufferSize int
}

// Capturer dispatches screenshot capture requests to a worker goroutine with
// bounded buffering, CRC32-based dedup of consecutive identical frames, and
// drop-oldest backpressure on buffer overflow.
//
// All exported methods are safe for concurrent use from any goroutine. All
// exported methods are also safe to call on a nil receiver, so callers that
// hold a possibly-disabled Capturer can call its methods without nil checks.
type Capturer struct {
	opts CapturerOptions

	buf *ringBuffer

	workerDone chan struct{}

	mu       sync.Mutex
	seq      uint64
	lastHash uint32
	// lastPath is the relative storage path of the most recently
	// persisted frame. Used to associate subsequent dedup-elided
	// frames with the screenshot they collapsed into.
	lastPath string
	// lastDedups accumulates the API-call events that have been
	// dedup-elided against the most recently persisted frame. It
	// is reset every time a new (non-dedup) frame persists and
	// flushed to a JSON sidecar next to lastPath on each append.
	lastDedups []dedupEntry
	dropped    uint64
}

// dedupEntry records one API call whose frame was elided because its
// bytes matched the previous persisted frame. Serialised as the
// `deduped[]` array in the per-screenshot JSON sidecar.
type dedupEntry struct {
	API    string `json:"api"`
	UnixMs int64  `json:"unix_ms"`
	Reason string `json:"reason"`
}

// captureReq is the unit of work passed from Capture callers to the worker.
type captureReq struct {
	ctx context.Context
	// reason is the trigger tag ("action" or "failure" today).
	reason string
	// apiName is the JS-visible browser API method that triggered the
	// capture (e.g. "Page.click", "Locator.waitFor"). Empty when the
	// caller hasn't propagated a name; downstream code substitutes a
	// sentinel ("unknown") so paths remain well-formed.
	apiName string
	fn      CaptureFunc
	// forced skips the CRC32 dedup check, persisting the frame even
	// when its bytes match the previous persisted frame. Used by the
	// failure-debugging path where the user needs the state at the
	// moment of failure regardless of whether it has visibly changed.
	forced bool
}

// NewCapturer constructs a Capturer and starts its worker goroutine. The
// caller MUST invoke Close to drain pending captures and release the worker.
//
// BufferSize must be positive; values <= 0 are treated as 1.
// Logger may be nil; a discarding logger is substituted.
func NewCapturer(opts CapturerOptions) *Capturer {
	if opts.BufferSize <= 0 {
		opts.BufferSize = 1
	}
	if opts.Logger == nil {
		opts.Logger = log.NewNullLogger()
	}
	c := &Capturer{
		opts:       opts,
		buf:        newRingBuffer(opts.BufferSize),
		workerDone: make(chan struct{}),
	}
	go c.work()
	return c
}

// Capture schedules a screenshot. The capture function is invoked on the
// worker goroutine; this method returns immediately. If the buffer is full
// the oldest pending request is dropped to make room and the dropped counter
// is incremented.
//
// reason is recorded in the persist path so the consumer can correlate the
// frame with its trigger (e.g. "action", "change", "failure"). Identical
// consecutive frames are deduplicated via CRC32; if a caller needs the
// frame persisted even when its bytes match the previous frame, use
// CaptureForced instead.
func (c *Capturer) Capture(ctx context.Context, reason, apiName string, fn CaptureFunc) {
	if c == nil {
		return
	}
	if c.buf.push(captureReq{ctx: ctx, reason: reason, apiName: apiName, fn: fn}) {
		c.mu.Lock()
		c.dropped++
		c.mu.Unlock()
	}
}

// CaptureForced schedules a screenshot that bypasses the CRC32 dedup
// check. Use this when the frame must be persisted regardless of
// whether its bytes match the previous frame — typically for
// failure-debugging captures where the user needs the state at the
// moment of failure even when it visually matches the last successful
// action. Backpressure (drop-oldest) behaves the same as Capture.
func (c *Capturer) CaptureForced(ctx context.Context, reason, apiName string, fn CaptureFunc) {
	if c == nil {
		return
	}
	if c.buf.push(captureReq{ctx: ctx, reason: reason, apiName: apiName, fn: fn, forced: true}) {
		c.mu.Lock()
		c.dropped++
		c.mu.Unlock()
	}
}

// Close stops the worker after draining all pending captures. After Close
// returns, no further Capture calls will be honoured.
func (c *Capturer) Close() {
	if c == nil {
		return
	}
	c.buf.close()
	<-c.workerDone
}

// Dropped returns the total number of capture requests dropped due to
// backpressure since the Capturer was created.
func (c *Capturer) Dropped() uint64 {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.dropped
}

func (c *Capturer) work() {
	defer close(c.workerDone)
	for {
		req, ok := c.buf.pop()
		if !ok {
			return
		}
		c.process(req)
	}
}

func (c *Capturer) process(req captureReq) {
	started := time.Now()
	buf, err := req.fn(req.ctx)
	if err != nil {
		c.opts.Logger.Warnf("autoscreenshot",
			"capture failed (reason=%s): %v", req.reason, err)
		return
	}
	if len(buf) == 0 {
		return
	}

	hash := crc32.ChecksumIEEE(buf)

	apiForLog := req.apiName
	if apiForLog == "" {
		apiForLog = "unknown"
	}

	c.mu.Lock()
	if !req.forced && c.seq > 0 && hash == c.lastHash {
		// Dedup path. Record the elided event against the previous
		// persisted frame so downstream tooling can show the full
		// sequence of API calls that mapped to a single screenshot.
		c.lastDedups = append(c.lastDedups, dedupEntry{
			API:    apiForLog,
			UnixMs: started.UnixMilli(),
			Reason: req.reason,
		})
		dupeOfSeq := c.seq
		dupeOfPath := c.lastPath
		dedupsSnapshot := append([]dedupEntry(nil), c.lastDedups...)
		c.mu.Unlock()

		fmt.Fprintf(os.Stderr,
			"auto-screenshot: dedup api=%s reason=%s unix_ms=%d of_seq=%d of_path=%s\n",
			apiForLog, req.reason, started.UnixMilli(), dupeOfSeq, dupeOfPath)

		c.writeSidecar(req.ctx, dupeOfPath, dedupsSnapshot)
		return
	}
	c.lastHash = hash
	c.seq++
	seq := c.seq
	// New persisted frame opens a fresh dedup bucket.
	c.lastDedups = nil
	c.mu.Unlock()

	path := buildPath(c.opts.TestName, c.opts.VU, c.opts.Iter, seq, req.reason, req.apiName, started)
	if err := c.opts.Persister.Persist(req.ctx, path, bytes.NewReader(buf)); err != nil {
		c.opts.Logger.Warnf("autoscreenshot",
			"persist failed (path=%s): %v", path, err)
		return
	}

	// Commit lastPath only after a successful persist so any later
	// dedups associate with a frame that really exists on disk.
	c.mu.Lock()
	c.lastPath = path
	c.mu.Unlock()

	// Emit a structured marker per persisted capture so external
	// tooling (a wrapper script, the SM agent, a future plugin) can
	// discover screenshots without scanning the filesystem. We write
	// to stderr directly rather than through opts.Logger because
	// callers commonly wire a NullLogger (to suppress the existing
	// capture/persist warnings) and the marker must be visible
	// regardless of that choice. The `auto-screenshot:` prefix is
	// the stable contract; downstream parsers anchor on it. api= is
	// the JS-facing browser API method that triggered this capture
	// (e.g. Page.click); "unknown" when the caller didn't propagate
	// one.
	fmt.Fprintf(os.Stderr,
		"auto-screenshot: reason=%s api=%s seq=%d unix_ms=%d path=%s\n",
		req.reason, apiForLog, seq, started.UnixMilli(), path)
}

// writeSidecar persists the current dedup bucket for screenshotPath as
// a JSON file at the same name with the ".json" extension. Overwrites
// each time a new dedup event is appended; idempotent for a given
// (screenshotPath, dedups) input.
//
// Errors are intentionally swallowed at warn level: the sidecar is a
// dev-loop convenience and must not fail captures.
func (c *Capturer) writeSidecar(ctx context.Context, screenshotPath string, dedups []dedupEntry) {
	if screenshotPath == "" {
		return
	}
	sidecarPath := strings.TrimSuffix(screenshotPath, ".png") + ".json"

	payload := struct {
		Deduped []dedupEntry `json:"deduped"`
	}{Deduped: dedups}

	data, err := json.Marshal(payload)
	if err != nil {
		c.opts.Logger.Warnf("autoscreenshot",
			"marshal dedup sidecar (path=%s): %v", sidecarPath, err)
		return
	}

	if err := c.opts.Persister.Persist(ctx, sidecarPath, bytes.NewReader(data)); err != nil {
		c.opts.Logger.Warnf("autoscreenshot",
			"persist dedup sidecar (path=%s): %v", sidecarPath, err)
	}
}

// buildPath produces the storage path for one captured frame.
//
// Layout: auto-screenshots/{testName}/vu-{vu}/iter-{iter}/{seq:06d}-{reason}-{api}-{unix_ms}.png
//
// {api} is the sanitised name of the JS-facing browser API call that
// triggered the capture (e.g. "Page.click"); "unknown" when the caller
// didn't propagate one. The format is stable enough that consumers can
// parse it; any future change should preserve segment order so callers
// can keep relying on `vu-` and `iter-` prefixes.
func buildPath(testName string, vu uint64, iter int64, seq uint64, reason, apiName string, t time.Time) string {
	api := sanitize(apiName)
	if api == "" {
		api = "unknown"
	}
	return fmt.Sprintf(
		"auto-screenshots/%s/vu-%d/iter-%d/%06d-%s-%s-%d.png",
		sanitize(testName), vu, iter, seq, sanitize(reason), api, t.UnixMilli(),
	)
}

// sanitize replaces characters that are unsafe in file paths with hyphens.
// Empty input collapses to "k6-test".
func sanitize(s string) string {
	if s == "" {
		return "k6-test"
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	return b.String()
}

// ringBuffer is a bounded FIFO queue with drop-oldest semantics on overflow.
// Concurrency: all operations take the embedded mutex; the condition variable
// signals pop callers when items become available or the buffer closes.
type ringBuffer struct {
	mu     sync.Mutex
	cond   *sync.Cond
	items  []captureReq
	head   int
	size   int
	closed bool
}

func newRingBuffer(capacity int) *ringBuffer {
	r := &ringBuffer{items: make([]captureReq, capacity)}
	r.cond = sync.NewCond(&r.mu)
	return r
}

// push appends req to the tail of the buffer. If the buffer is full, the
// item at the head is overwritten and the head pointer advances, dropping
// the oldest entry. Returns true if a drop occurred. Returns false if the
// buffer is closed (the push is silently discarded in that case).
func (r *ringBuffer) push(req captureReq) (dropped bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return false
	}
	capN := len(r.items)
	if r.size == capN {
		r.items[r.head] = req
		r.head = (r.head + 1) % capN
		dropped = true
	} else {
		tail := (r.head + r.size) % capN
		r.items[tail] = req
		r.size++
	}
	r.cond.Signal()
	return dropped
}

// pop blocks until an item is available or the buffer closes. When closed
// and empty, returns ok=false so the worker can exit.
func (r *ringBuffer) pop() (captureReq, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for r.size == 0 && !r.closed {
		r.cond.Wait()
	}
	if r.size == 0 {
		return captureReq{}, false
	}
	req := r.items[r.head]
	r.items[r.head] = captureReq{} // help GC release the closure
	r.head = (r.head + 1) % len(r.items)
	r.size--
	return req, true
}

// close marks the buffer closed and wakes any pop callers so they can
// observe the new state. Items already in the buffer are still drained.
func (r *ringBuffer) close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	r.cond.Broadcast()
}
