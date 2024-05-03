package browser

import (
	"github.com/dop251/goja"

	"github.com/grafana/xk6-browser/common"
	"github.com/grafana/xk6-browser/k6ext"
)

// mapTouchscreen to the JS module.
func mapTouchscreen(vu moduleVU, ts *common.Touchscreen) mapping {
	return mapping{
		"tap": func(x float64, y float64) *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (result any, reason error) {
				return nil, ts.Tap(x, y) //nolint:wrapcheck
			})
		},
	}
}
