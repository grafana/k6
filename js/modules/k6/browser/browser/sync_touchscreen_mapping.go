package browser

import (
	"github.com/grafana/sobek"

	"go.k6.io/k6/js/modules/k6/browser/common"
	"go.k6.io/k6/js/modules/k6/browser/k6ext"
)

// syncMapTouchscreen is like mapTouchscreen but returns synchronous functions.
func syncMapTouchscreen(vu moduleVU, ts *common.Touchscreen) mapping {
	return mapping{
		"tap": func(x float64, y float64) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (result any, reason error) {
				return nil, ts.Tap(x, y) //nolint:wrapcheck
			})
		},
	}
}
