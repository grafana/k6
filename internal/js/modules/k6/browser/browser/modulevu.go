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

// skipAfterActionAPIs lists JS-facing browser APIs whose successful
// completion never produces meaningful page-state to capture. Today
// this is just BrowserContext.newPage (which returns a freshly-created
// blank page; the next user action — typically Page.goto — is the
// first thing worth seeing). Failures of these APIs still capture via
// onFailure, since a failure inside newPage is itself informative.
//
// Add additional entries only when the API provably produces a no-op
// frame; over-filtering hides real debugging signal.
var skipAfterActionAPIs = map[string]struct{}{
	"BrowserContext.newPage": {},
}

// manualScreenshotAPIs lists the explicit, user-driven screenshot APIs.
// Auto-screenshot never fires for these — on success or failure — because
// the user is already capturing deliberately; an auto-capture would be a
// redundant duplicate of the very frame the user just asked for. Skips are
// logged at debug level so the suppression is traceable.
//
// Page.screenshot does not currently flow through the promise() helper (it
// builds its own promise), so it never reaches afterAction/onFailure
// regardless. It is listed here so the policy is explicit and survives a
// future refactor onto promise().
var manualScreenshotAPIs = map[string]struct{}{ //nolint:gochecknoglobals
	"Page.screenshot":          {},
	"ElementHandle.screenshot": {},
}

// afterAction schedules a screenshot capture for the current iteration's
// open pages when auto-screenshot mode A (actions) is active. Called from
// promise() after a successful JS-facing browser API call. apiName is the
// JS-visible method (e.g. "Page.click") that has just completed. Safe to
// call from any goroutine, during any VU lifecycle phase, and on a
// moduleVU whose auto-screenshot is disabled.
//
// APIs listed in skipAfterActionAPIs short-circuit here so blank
// pre-state captures never reach the persister; APIs listed in
// manualScreenshotAPIs short-circuit because the user is already
// capturing deliberately.
func (vu moduleVU) afterAction(apiName string) {
	if vu.autoScreenshot.Mode() != autoscreenshot.ModeActions {
		return
	}
	if _, skip := skipAfterActionAPIs[apiName]; skip {
		return
	}
	if vu.manualScreenshotSkip(apiName) {
		return
	}
	vu.captureOpenPages("action", apiName, false /* allow dedup */)
}

// onFailure schedules a failure-tagged screenshot capture for the current
// iteration's open pages whenever a browser API call rejects its promise.
// Fires for any non-Off auto-screenshot mode; a failure capture is useful
// regardless of how the user chose to drive successful-path captures. Safe
// to call from any goroutine, during any VU lifecycle phase, and on a
// moduleVU whose auto-screenshot is disabled.
//
// Explicit user-driven screenshot APIs (see manualScreenshotAPIs) are
// excluded: a failed page.screenshot()/elementHandle.screenshot() is the
// user's own capture call, not page state worth auto-capturing.
//
// Failure captures bypass the CRC32 dedup so that a frame is always
// produced at the moment of failure, even when the page state matches
// the preceding successful action (the common case for selector
// timeouts on unchanged pages).
//
// Check-failure (k6 core check() returning false) is intentionally not
// covered: k6's check is in a separate module with no cross-module hook
// point. Browser API errors are the dominant failure source in browser
// scripts in practice.
func (vu moduleVU) onFailure(apiName string) {
	if vu.autoScreenshot.Mode() == autoscreenshot.ModeOff {
		return
	}
	if vu.manualScreenshotSkip(apiName) {
		return
	}
	vu.captureOpenPages("failure", apiName, true /* bypass dedup */)
}

// manualScreenshotSkip reports whether apiName is an explicit, user-driven
// screenshot API for which auto-screenshot must not fire. When it returns
// true it has already logged the skip at debug level. Safe to call during
// the init phase when vu.State() is nil; the debug log is then omitted.
func (vu moduleVU) manualScreenshotSkip(apiName string) bool {
	if _, ok := manualScreenshotAPIs[apiName]; !ok {
		return false
	}
	if state := vu.State(); state != nil {
		state.Logger.Debugf(
			"auto-screenshot: skipping capture for manual screenshot API %q",
			apiName,
		)
	}
	return true
}

// captureOpenPages enqueues a viewport capture for every currently-open
// page in the iteration's browser. No-op when the registry is disabled
// for the current iteration. Shared by the after-action and failure
// trigger paths; pass force=true to skip the dedup path so the frame
// persists regardless of whether its bytes match the previous frame.
func (vu moduleVU) captureOpenPages(reason, apiName string, force bool) {
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

	// Viewport-sized capture. Page.captureScreenshot's default
	// behaviour is to image the current visible viewport, which
	// follows any scrolling the script (or k6 browser's
	// scroll-into-view-if-needed click path) has done since the
	// previous capture. Below-the-fold content that the script
	// interacts with appears in the next screenshot because the
	// click auto-scrolls the page; below-the-fold content the
	// script never touches is intentionally not shown.
	//
	// FullPage was tried during the POC but produced disproportionate
	// outputs on long marketing-style pages (10MB+ per shot, viewport
	// resize mid-iteration) without delivering substantially more
	// debugging value than viewport-with-scroll-tracking.
	ctx := vu.Context()
	opts := &common.PageScreenshotOptions{
		Format:  common.ImageFormatPNG,
		Quality: 100,
	}
	for _, page := range pages {
		page := page // shadow so each iteration's closures capture
		// the right Page; without this every closure would see the
		// final value of the loop variable in Go versions older than
		// 1.22's loopvar semantics.
		fn := func(_ context.Context) ([]byte, error) {
			return page.Screenshot(opts, noopScreenshotPersister{})
		}

		// onPersisted is invoked on the autoscreenshot worker goroutine
		// once the capturer has decided the frame survives dedup. We
		// translate the raw outputs into a typed AutoScreenshotEvent
		// and hand it to the Page's event dispatcher, which queues
		// any registered page.on('auto-screenshot') handler onto the
		// VU's JS event loop. Fire-and-forget: Page.OnAutoScreenshot
		// itself does not block (the queueTask underneath returns
		// immediately for fire-and-forget event types).
		onPersisted := func(buf []byte, seq uint64, unixMs int64) {
			pageURL, _ := page.URL()
			page.OnAutoScreenshot(&common.AutoScreenshotEvent{
				Bytes:   buf,
				API:     apiName,
				Reason:  reason,
				Seq:     seq,
				UnixMs:  unixMs,
				PageURL: pageURL,
			})
		}

		if force {
			c.CaptureForced(ctx, reason, apiName, fn, onPersisted)
		} else {
			c.Capture(ctx, reason, apiName, fn, onPersisted)
		}
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
