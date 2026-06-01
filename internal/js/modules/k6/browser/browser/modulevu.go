package browser

import (
	"context"
	"io"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/common"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/common/autoscreenshot"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/k6ext"

	k6modules "go.k6.io/k6/v2/js/modules"
)

// moduleVU carries module specific VU information.
//
// Currently, it is used to carry the VU object to the inner objects and
// promises.
type moduleVU struct {
	k6modules.VU

	*pidRegistry
	*browserRegistry

	*taskQueueRegistry

	filePersister

	// autoScreenshot tracks per-iteration screenshot Capturers when the
	// K6_BROWSER_AUTO_SCREENSHOT environment variable is set. nil when
	// the feature is disabled; nil-safe at all call sites.
	autoScreenshot *autoscreenshot.Registry

	testRunID string
}

// browser returns the VU browser instance for the current iteration.
func (vu moduleVU) browser() (*common.Browser, error) {
	return vu.getBrowser(vu.State().Iteration)
}

func (vu moduleVU) Context() context.Context {
	// promises and inner objects need the VU object to be
	// able to use k6-core specific functionality.
	//
	// We should not cache the context (especially the init
	// context from the vu that is received from k6 in
	// NewModuleInstance).
	return k6ext.WithVU(vu.VU.Context(), vu)
}

// afterAction schedules a screenshot capture for the current iteration's
// open pages when auto-screenshot mode A (actions) is active. Called from
// promise() after a successful JS-facing browser API call. Safe to call
// from any goroutine, during any VU lifecycle phase, and on a moduleVU
// whose auto-screenshot is disabled.
func (vu moduleVU) afterAction() {
	if vu.autoScreenshot.Mode() != autoscreenshot.ModeActions {
		return
	}
	vu.captureOpenPages("action")
}

// onFailure schedules a failure-tagged screenshot capture for the current
// iteration's open pages whenever a browser API call rejects its promise.
// Fires for any non-Off auto-screenshot mode (both ModeActions and
// ModeChanges); a failure capture is useful regardless of how the user
// chose to drive successful-path captures. Safe to call from any
// goroutine, during any VU lifecycle phase, and on a moduleVU whose
// auto-screenshot is disabled.
//
// Check-failure (k6 core check() returning false) is intentionally not
// covered: k6's check is in a separate module with no cross-module hook
// point. Browser API errors are the dominant failure source in browser
// scripts in practice.
func (vu moduleVU) onFailure() {
	if vu.autoScreenshot.Mode() == autoscreenshot.ModeOff {
		return
	}
	vu.captureOpenPages("failure")
}

// captureOpenPages enqueues a viewport capture for every currently-open
// page in the iteration's browser. No-op when the registry is disabled
// for the current iteration. Shared by the after-action and failure
// trigger paths.
func (vu moduleVU) captureOpenPages(reason string) {
	state := vu.State()
	if state == nil {
		return
	}

	c := vu.autoScreenshot.Get(state.VUID, state.Iteration)
	if c == nil {
		return
	}

	b, err := vu.browser()
	if err != nil {
		return
	}
	pages := b.Pages()
	if len(pages) == 0 {
		return
	}

	ctx := vu.Context()
	for _, page := range pages {
		c.Capture(ctx, reason, func(_ context.Context) ([]byte, error) {
			return page.Screenshot(&common.PageScreenshotOptions{}, noopScreenshotPersister{})
		})
	}
}

// noopScreenshotPersister discards persist calls. Used when invoking
// common.Page.Screenshot for auto-screenshot captures, where the bytes
// are returned to the autoscreenshot worker and persisted there rather
// than written by Page.Screenshot itself.
type noopScreenshotPersister struct{}

func (noopScreenshotPersister) Persist(_ context.Context, _ string, _ io.Reader) error {
	return nil
}
