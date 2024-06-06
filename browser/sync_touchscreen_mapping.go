package browser

import (
	"github.com/grafana/xk6-browser/common"
)

// syncMapTouchscreen is like mapTouchscreen but returns synchronous functions.
func syncMapTouchscreen(_ moduleVU, _ *common.Touchscreen) mapping {
	return nil
}
