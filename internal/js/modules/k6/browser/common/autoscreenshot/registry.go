package autoscreenshot

import (
	"sync"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/log"
)

// defaultBufferSize is the per-Capturer ring buffer capacity. With
// viewport-only captures running ~hundreds of KB each, this caps worst-case
// in-flight memory per page around ~100MB.
const defaultBufferSize = 200

// Mode selects how auto-screenshots are triggered. The value is parsed from
// the K6_BROWSER_AUTO_SCREENSHOT environment variable using ParseMode.
type Mode int

const (
	// ModeOff disables auto-screenshots.
	ModeOff Mode = iota
	// ModeActions captures after every browser API call.
	ModeActions
)

// String returns the canonical string form of the mode.
func (m Mode) String() string {
	switch m {
	case ModeActions:
		return "actions"
	default:
		return "off"
	}
}

// ParseMode converts the value of the K6_BROWSER_AUTO_SCREENSHOT environment
// variable to a Mode. Unknown or empty inputs map to ModeOff.
func ParseMode(s string) Mode {
	switch s {
	case "actions":
		return ModeActions
	default:
		return ModeOff
	}
}

// Registry holds per-(VU, iteration) Capturers for a single VU. Callers
// allocate a Capturer at IterStart, retrieve it during the iteration via
// Get, and release it at IterEnd.
//
// All methods are safe to call on a nil receiver. NewRegistry returns nil
// when constructed with ModeOff, so callers in disabled mode get
// no-op behaviour without nil checks.
type Registry struct {
	persister      Persister
	testName       string
	logger         *log.Logger
	mode           Mode
	dedupEnabled   bool
	persistEnabled bool

	mu sync.Mutex
	m  map[capturerKey]*Capturer
}

type capturerKey struct {
	vu   uint64
	iter int64
}

// NewRegistry constructs a Registry. Returns nil if mode is ModeOff so that
// downstream code can rely on nil-safety to disable auto-screenshot work
// entirely.
//
// dedupEnabled controls whether the Capturers it produces apply CRC32
// dedup; persistEnabled controls whether they write captured frames to
// the Persister. Both default to true at the env-var parsing layer
// (module.go); tests that construct the Registry directly pass these
// explicitly.
func NewRegistry(mode Mode, persister Persister, testName string, logger *log.Logger, dedupEnabled, persistEnabled bool) *Registry {
	if mode == ModeOff {
		return nil
	}
	return &Registry{
		persister:      persister,
		testName:       testName,
		logger:         logger,
		mode:           mode,
		dedupEnabled:   dedupEnabled,
		persistEnabled: persistEnabled,
		m:              make(map[capturerKey]*Capturer),
	}
}

// Mode returns the mode the registry was constructed with, or ModeOff if the
// registry is nil.
func (r *Registry) Mode() Mode {
	if r == nil {
		return ModeOff
	}
	return r.mode
}

// OnIterStart allocates a Capturer for the (vu, iter) pair. If a Capturer
// already exists for the pair it is returned as-is — useful when the same
// IterStart event is delivered to multiple subscribers.
func (r *Registry) OnIterStart(vu uint64, iter int64) *Capturer {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	k := capturerKey{vu: vu, iter: iter}
	if c, ok := r.m[k]; ok {
		return c
	}
	c := NewCapturer(CapturerOptions{
		Persister:      r.persister,
		Logger:         r.logger,
		TestName:       r.testName,
		VU:             vu,
		Iter:           iter,
		BufferSize:     defaultBufferSize,
		DedupEnabled:   r.dedupEnabled,
		PersistEnabled: r.persistEnabled,
	})
	r.m[k] = c
	return c
}

// OnIterEnd closes the Capturer for the (vu, iter) pair, draining any
// pending captures. Safe to call if no Capturer exists for the pair.
func (r *Registry) OnIterEnd(vu uint64, iter int64) {
	if r == nil {
		return
	}
	k := capturerKey{vu: vu, iter: iter}
	r.mu.Lock()
	c, ok := r.m[k]
	if ok {
		delete(r.m, k)
	}
	r.mu.Unlock()
	if ok {
		c.Close()
	}
}

// Get returns the Capturer for the (vu, iter) pair, or nil if none is
// active.
func (r *Registry) Get(vu uint64, iter int64) *Capturer {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.m[capturerKey{vu: vu, iter: iter}]
}

// Stop closes all outstanding Capturers. Intended for end-of-VU cleanup
// (e.g. invoked from the k6 Exit event handler).
func (r *Registry) Stop() {
	if r == nil {
		return
	}
	r.mu.Lock()
	capturers := make([]*Capturer, 0, len(r.m))
	for k, c := range r.m {
		capturers = append(capturers, c)
		delete(r.m, k)
	}
	r.mu.Unlock()
	for _, c := range capturers {
		c.Close()
	}
}
