package browser

import (
	"github.com/grafana/sobek"

	"go.k6.io/k6/internal/js/modules/k6/browser/common"
	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext"
)

// mapRoute to the JS module.
func mapRoute(vu moduleVU, route *common.Route) mapping {
	return mapping{
		"abort": func(reason string) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, route.Abort(reason)
			})
		},
		"request": route.Request,
	}
}
