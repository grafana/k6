package browser

import (
	"github.com/grafana/xk6-browser/common"
)

// syncMapJSHandle is like mapJSHandle but returns synchronous functions.
func syncMapJSHandle(_ moduleVU, _ common.JSHandleAPI) mapping {
	return nil
}
