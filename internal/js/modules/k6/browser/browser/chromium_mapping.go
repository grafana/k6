package browser

import (
	"fmt"

	"github.com/grafana/sobek"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/chromium"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/common"
)

// mapChromium maps the chromium browser type API.
func mapChromium(vu moduleVU, bt *chromium.BrowserType) mapping {
	return mapping{
		"connectOverCDP": func(wsEndpoint string) *sobek.Promise {
			return promise(vu, func() (any, error) {
				b, err := bt.ConnectOverCDP(vu.Context(), wsEndpoint)
				if err != nil {
					return nil, fmt.Errorf("connecting to Chromium over CDP: %w", err)
				}
				m := mapBrowser(vu, func() (*common.Browser, error) {
					return b, nil
				})
				m["close"] = func() *sobek.Promise {
					return promise(vu, func() (any, error) {
						b.Close()
						return nil, nil
					})
				}
				return m, nil
			})
		},
	}
}
