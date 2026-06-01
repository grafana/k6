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
		c.Capture(ctx, "action", func(_ context.Context) ([]byte, error) {
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
