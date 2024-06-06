package browser

import (
	"github.com/grafana/xk6-browser/common"
)

// syncMapBrowserContext is like mapBrowserContext but returns synchronous functions.
func syncMapBrowserContext(_ moduleVU, _ *common.BrowserContext) mapping {
	return nil
}
