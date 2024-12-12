package browser

import (
	"github.com/grafana/sobek"

	"go.k6.io/k6/js/modules/k6/browser/common"
	"go.k6.io/k6/js/modules/k6/browser/k6ext"
)

func mapMouse(vu moduleVU, m *common.Mouse) mapping {
	return mapping{
		"click": func(x float64, y float64, opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, m.Click(x, y, opts) //nolint:wrapcheck
			})
		},
		"dblClick": func(x float64, y float64, opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, m.DblClick(x, y, opts) //nolint:wrapcheck
			})
		},
		"down": func(opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, m.Down(opts) //nolint:wrapcheck
			})
		},
		"up": func(opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, m.Up(opts) //nolint:wrapcheck
			})
		},
		"move": func(x float64, y float64, opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, m.Move(x, y, opts) //nolint:wrapcheck
			})
		},
	}
}
