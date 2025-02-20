package browser

import (
	"fmt"

	"github.com/grafana/sobek"

	"go.k6.io/k6/internal/js/modules/k6/browser/common"
	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext"
)

func mapMouse(vu moduleVU, m *common.Mouse) mapping {
	return mapping{
		"click": func(x float64, y float64, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewMouseClickOptions()
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing mouse click options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, m.Click(x, y, popts) //nolint:wrapcheck
			}), nil
		},
		"dblClick": func(x float64, y float64, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewMouseDblClickOptions()
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing double click options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, m.DblClick(x, y, popts) //nolint:wrapcheck
			}), nil
		},
		"down": func(opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewMouseDownUpOptions()
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing mouse down options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, m.Down(popts) //nolint:wrapcheck
			}), nil
		},
		"up": func(opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewMouseDownUpOptions()
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing mouse up options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, m.Up(popts) //nolint:wrapcheck
			}), nil
		},
		"move": func(x float64, y float64, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewMouseMoveOptions()
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing mouse move options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, m.Move(x, y, popts) //nolint:wrapcheck
			}), nil
		},
	}
}
