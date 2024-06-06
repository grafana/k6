package browser

import (
	"github.com/grafana/xk6-browser/common"
)

// syncMapLocator is like mapLocator but returns synchronous functions.
func syncMapLocator(_ moduleVU, _ *common.Locator) mapping {
	return nil
}
