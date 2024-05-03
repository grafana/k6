package browser

import (
	"context"
	"fmt"
	"time"

	"github.com/dop251/goja"

	"github.com/grafana/xk6-browser/common"

	k6common "go.k6.io/k6/js/common"
)

// mapping is a type for mapping our module API to Goja.
// It acts like a bridge and allows adding wildcard methods
// and customization over our API.
type mapping = map[string]any

// mapBrowserToGoja maps the browser API to the JS module.
// The motivation of this mapping was to support $ and $$ wildcard
// methods.
// See issue #661 for more details.
func mapBrowserToGoja(vu moduleVU) *goja.Object {
	var (
		rt  = vu.Runtime()
		obj = rt.NewObject()
	)
	for k, v := range mapBrowser(vu) {
		err := obj.Set(k, rt.ToValue(v))
		if err != nil {
			k6common.Throw(rt, fmt.Errorf("mapping: %w", err))
		}
	}

	return obj
}

func parseFrameClickOptions(
	ctx context.Context, opts goja.Value, defaultTimeout time.Duration,
) (*common.FrameClickOptions, error) {
	copts := common.NewFrameClickOptions(defaultTimeout)
	if err := copts.Parse(ctx, opts); err != nil {
		return nil, fmt.Errorf("parsing click options: %w", err)
	}
	return copts, nil
}

func initBrowserContext(bctx *common.BrowserContext, testRunID string) error {
	// Setting a k6 object which will contain k6 specific metadata
	// on the current test run. This allows external applications
	// (such as Grafana Faro) to identify that the session is a k6
	// automated one and not one driven by a real person.
	if err := bctx.AddInitScript(
		fmt.Sprintf(`window.k6 = { testRunId: %q }`, testRunID),
	); err != nil {
		return fmt.Errorf("adding k6 object to new browser context: %w", err)
	}

	return nil
}
