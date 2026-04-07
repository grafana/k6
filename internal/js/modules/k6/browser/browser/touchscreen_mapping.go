package browser

import (
	"github.com/grafana/sobek"

	"go.k6.io/k6/internal/js/modules/k6/browser/common"
)

// mapTouchscreen to the JS module.
func mapTouchscreen(vu moduleVU, ts *common.Touchscreen) mapping {
	return mapping{
		"tap": func(x float64, y float64) *sobek.Promise {
			return promise(vu, func() (result any, reason error) {
				return nil, ts.Tap(x, y) //nolint:wrapcheck
			})
		},
	}
}
