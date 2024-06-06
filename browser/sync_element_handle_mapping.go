package browser

import (
	"github.com/grafana/xk6-browser/common"
)

// syncMapElementHandle is like mapElementHandle but returns synchronous functions.
func syncMapElementHandle(_ moduleVU, _ *common.ElementHandle) mapping {
	return nil
}
