package browser

import (
	"fmt"

	"github.com/dop251/goja"

	"github.com/grafana/xk6-browser/common"
	"github.com/grafana/xk6-browser/k6ext"
)

// mapLocator API to the JS module.
func mapLocator(vu moduleVU, lo *common.Locator) mapping { //nolint:funlen
	return mapping{
		"clear": func(opts goja.Value) (*goja.Promise, error) {
			copts := common.NewFrameFillOptions(lo.Timeout())
			if err := copts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing clear options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, lo.Clear(copts) //nolint:wrapcheck
			}), nil
		},
		"click": func(opts goja.Value) (*goja.Promise, error) {
			popts, err := parseFrameClickOptions(vu.Context(), opts, lo.Timeout())
			if err != nil {
				return nil, err
			}

			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, lo.Click(popts) //nolint:wrapcheck
			}), nil
		},
		"dblclick": func(opts goja.Value) *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, lo.Dblclick(opts) //nolint:wrapcheck
			})
		},
		"check": func(opts goja.Value) *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, lo.Check(opts) //nolint:wrapcheck
			})
		},
		"uncheck": func(opts goja.Value) *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, lo.Uncheck(opts) //nolint:wrapcheck
			})
		},
		"isChecked": func(opts goja.Value) *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return lo.IsChecked(opts) //nolint:wrapcheck
			})
		},
		"isEditable": func(opts goja.Value) *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return lo.IsEditable(opts) //nolint:wrapcheck
			})
		},
		"isEnabled": func(opts goja.Value) *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return lo.IsEnabled(opts) //nolint:wrapcheck
			})
		},
		"isDisabled": func(opts goja.Value) *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return lo.IsDisabled(opts) //nolint:wrapcheck
			})
		},
		"isVisible": func() *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return lo.IsVisible() //nolint:wrapcheck
			})
		},
		"isHidden": func() *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return lo.IsHidden() //nolint:wrapcheck
			})
		},
		"fill": func(value string, opts goja.Value) *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, lo.Fill(value, opts) //nolint:wrapcheck
			})
		},
		"focus": func(opts goja.Value) *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, lo.Focus(opts) //nolint:wrapcheck
			})
		},
		"getAttribute": func(name string, opts goja.Value) *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return lo.GetAttribute(name, opts) //nolint:wrapcheck
			})
		},
		"innerHTML": func(opts goja.Value) *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return lo.InnerHTML(opts) //nolint:wrapcheck
			})
		},
		"innerText":    lo.InnerText,
		"textContent":  lo.TextContent,
		"inputValue":   lo.InputValue,
		"selectOption": lo.SelectOption,
		"press":        lo.Press,
		"type":         lo.Type,
		"hover":        lo.Hover,
		"tap": func(opts goja.Value) (*goja.Promise, error) {
			copts := common.NewFrameTapOptions(lo.DefaultTimeout())
			if err := copts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing locator tap options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, lo.Tap(copts) //nolint:wrapcheck
			}), nil
		},
		"dispatchEvent": func(typ string, eventInit, opts goja.Value) error {
			popts := common.NewFrameDispatchEventOptions(lo.DefaultTimeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return fmt.Errorf("parsing locator dispatch event options: %w", err)
			}
			return lo.DispatchEvent(typ, exportArg(eventInit), popts) //nolint:wrapcheck
		},
		"waitFor": lo.WaitFor,
	}
}
