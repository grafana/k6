package browser

import (
	"fmt"
	"time"

	"github.com/grafana/sobek"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/chromium"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/common"
)

// mapChromium maps the Chromium browser type API to the JS module.
func mapChromium(vu moduleVU, bt *chromium.BrowserType) mapping {
	return mapping{
		"connectOverCDP": func(endpoint string, opts sobek.Value) (*sobek.Promise, error) {
			// Parse the optional options argument on the JS thread, before the
			// promise callback runs on a background goroutine.
			cdpOpts, err := parseConnectOverCDPOptions(vu.Runtime(), opts)
			if err != nil {
				return nil, err
			}

			return promise(vu, func() (any, error) {
				iter := vu.State().Iteration

				// Clone the BrowserType for this call so concurrent
				// connectOverCDP calls in the same iteration (e.g., via
				// Promise.all) don't race on its mutable state.
				connBT := bt.Clone()

				// Link the connection to the iteration trace and connect.
				tracedCtx := vu.startConnectTrace(vu.Context(), iter)
				b, err := connBT.ConnectOverCDP(tracedCtx, endpoint, cdpOpts)
				if err != nil {
					return nil, fmt.Errorf("connecting to Chromium over CDP: %w", err)
				}

				// Register for guaranteed cleanup at IterEnd / Exit.
				vu.trackUserManagedBrowser(iter, b)

				return mapBrowser(vu, func() (*common.Browser, error) {
					return b, nil
				}), nil
			}), nil
		},
	}
}

// parseConnectOverCDPOptions parses the optional trailing options argument of
// chromium.connectOverCDP (e.g. { timeout }) into chromium.ConnectOverCDPOptions.
// A nil, undefined or null value yields the zero options.
func parseConnectOverCDPOptions(rt *sobek.Runtime, opts sobek.Value) (chromium.ConnectOverCDPOptions, error) {
	var o chromium.ConnectOverCDPOptions
	if opts == nil || sobek.IsUndefined(opts) || sobek.IsNull(opts) {
		return o, nil
	}

	obj := opts.ToObject(rt)
	for _, k := range obj.Keys() {
		switch k {
		case "timeout":
			o.Timeout = time.Duration(obj.Get(k).ToInteger()) * time.Millisecond
		}
	}

	return o, nil
}
