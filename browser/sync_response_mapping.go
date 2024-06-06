package browser

import (
	"github.com/grafana/xk6-browser/common"
)

// syncMapResponse is like mapResponse but returns synchronous functions.
func syncMapResponse(_ moduleVU, _ *common.Response) mapping {
	return nil
}
