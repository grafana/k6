package browser

import (
	"github.com/grafana/xk6-browser/common"
)

// syncMapFrame is like mapFrame but returns synchronous functions.
func syncMapFrame(_ moduleVU, _ *common.Frame) mapping {
	return nil
}
