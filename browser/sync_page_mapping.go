package browser

import (
	"github.com/grafana/xk6-browser/common"
)

// syncMapPage is like mapPage but returns synchronous functions.
func syncMapPage(_ moduleVU, _ *common.Page) mapping {
	return nil
}
