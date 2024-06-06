package browser

import (
	"github.com/grafana/xk6-browser/common"
)

// syncMapRequest is like mapRequest but returns synchronous functions.
func syncMapRequest(_ moduleVU, _ *common.Request) mapping {
	return nil
}
