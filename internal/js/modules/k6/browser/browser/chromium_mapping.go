package browser

import (
	"errors"
	"fmt"

	"github.com/grafana/sobek"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/chromium"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/common"
)

// mapChromium maps the Chromium browser type API to the JS module.
func mapChromium(vu moduleVU, bt *chromium.BrowserType) mapping {
	return mapping{
		"connectOverCDP": func(wsEndpoint string) *sobek.Promise {
			return promise(vu, func() (any, error) {
				// connectOverCDP is gated behind options.browser.remote: true. This
				// keeps it inside k6's browser-scenario model (managed iteration
				// lifecycle, tracing) and tells k6 not to launch a managed browser
				// for this VU (see the IterStart handling in the registry).
				if !isRemoteScenario(vu) {
					return nil, errors.New(
						"chromium.connectOverCDP requires a browser scenario with " +
							"options.browser.remote set to true",
					)
				}

				iter := vu.State().Iteration

				// Clone the BrowserType for this call so concurrent
				// connectOverCDP calls in the same iteration (e.g., via
				// Promise.all) don't race on its mutable state.
				connBT := bt.Clone()

				// Link the connection to the iteration trace and connect.
				tracedCtx := vu.startConnectTrace(vu.Context(), iter)
				b, err := connBT.ConnectOverCDP(tracedCtx, wsEndpoint)
				if err != nil {
					return nil, fmt.Errorf("connecting to Chromium over CDP: %w", err)
				}

				// Register for guaranteed cleanup at IterEnd / Exit.
				vu.trackUserManagedBrowser(iter, b)

				return mapBrowser(vu, func() (*common.Browser, error) {
					return b, nil
				}), nil
			})
		},
	}
}
