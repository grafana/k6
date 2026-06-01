package common

import (
	"context"
	"io"
	"sync"
	"time"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/log"
)

// CapturerHook is the subset of the auto-screenshot Capturer API used by
// LifecycleWatcher to enqueue change-triggered screenshots. Declared here
// to avoid an upward dependency from common into the browser/autoscreenshot
// package.
type CapturerHook interface {
	Capture(ctx context.Context, reason string, fn func(context.Context) ([]byte, error))
}

// LifecycleWatcher subscribes to page-attachment and frame lifecycle events
// on a BrowserContext and asks the supplied CapturerHook to capture a
// viewport screenshot whenever a change settles. Settling is determined by
// a debounce window applied per-page: a burst of lifecycle events within
// the window collapses into a single capture fired after the last event.
type LifecycleWatcher struct {
	bc       *BrowserContext
	hook     CapturerHook
	debounce time.Duration
	logger   *log.Logger
}

// NewLifecycleWatcher constructs a watcher. debounce must be positive; the
// zero value defaults to 300ms.
func NewLifecycleWatcher(
	bc *BrowserContext,
	hook CapturerHook,
	debounce time.Duration,
	logger *log.Logger,
) *LifecycleWatcher {
	if debounce <= 0 {
		debounce = 300 * time.Millisecond
	}
	if logger == nil {
		logger = log.NewNullLogger()
	}
	return &LifecycleWatcher{bc: bc, hook: hook, debounce: debounce, logger: logger}
}

// Watch blocks until ctx is canceled, fanning out a per-page goroutine for
// every page already attached to the BrowserContext and every page attached
// later. Each per-page goroutine debounces lifecycle events and triggers
// the CapturerHook on settle.
func (w *LifecycleWatcher) Watch(ctx context.Context) {
	pagesCh := make(chan Event, 4)
	w.bc.on(ctx, []string{EventBrowserContextPage}, pagesCh)

	// Existing pages.
	for _, p := range w.bc.Pages() {
		go w.watchPage(ctx, p)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-pagesCh:
			p, ok := ev.data.(*Page)
			if !ok {
				continue
			}
			go w.watchPage(ctx, p)
		}
	}
}

// watchPage subscribes to the page's main-frame lifecycle events and
// drives the debouncer for that page.
func (w *LifecycleWatcher) watchPage(ctx context.Context, p *Page) {
	lifeCh := make(chan Event, 4)
	p.MainFrame().on(ctx, []string{EventFrameAddLifecycle}, lifeCh)

	page := p
	d := newDebouncer(w.debounce, func() {
		w.hook.Capture(ctx, "lifecycle", func(_ context.Context) ([]byte, error) {
			return page.Screenshot(&PageScreenshotOptions{}, lifecycleNoopPersister{})
		})
	})
	defer d.stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-lifeCh:
			d.trigger()
		}
	}
}

// debouncer collapses bursty triggers into a single fire at the trailing
// edge of a quiet window. All methods are safe for concurrent use.
type debouncer struct {
	delay time.Duration
	fire  func()

	mu    sync.Mutex
	timer *time.Timer
}

func newDebouncer(delay time.Duration, fire func()) *debouncer {
	return &debouncer{delay: delay, fire: fire}
}

// trigger schedules fire to run after delay, cancelling any previous
// pending fire scheduled by this debouncer.
func (d *debouncer) trigger() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.timer != nil {
		d.timer.Stop()
	}
	d.timer = time.AfterFunc(d.delay, d.fire)
}

// stop cancels any pending fire. Subsequent triggers are still honoured.
func (d *debouncer) stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}
}

// lifecycleNoopPersister discards Persist calls. The page screenshot's
// bytes are returned to the Capturer, which handles persistence through
// its own configured Persister.
type lifecycleNoopPersister struct{}

func (lifecycleNoopPersister) Persist(_ context.Context, _ string, _ io.Reader) error {
	return nil
}
