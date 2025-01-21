package browser

import (
	"fmt"

	"github.com/grafana/sobek"

	"go.k6.io/k6/internal/js/modules/k6/browser/common"
	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext"
)

// mapLocator API to the JS module.
func mapLocator(vu moduleVU, lo *common.Locator) mapping { //nolint:funlen
	return mapping{
		"clear": func(opts sobek.Value) (*sobek.Promise, error) {
			copts := common.NewFrameFillOptions(lo.Timeout())
			if err := copts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing clear options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, lo.Clear(copts) //nolint:wrapcheck
			}), nil
		},
		"click": func(opts sobek.Value) (*sobek.Promise, error) {
			popts, err := parseFrameClickOptions(vu.Context(), opts, lo.Timeout())
			if err != nil {
				return nil, err
			}

			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, lo.Click(popts) //nolint:wrapcheck
			}), nil
		},
		"dblclick": func(opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, lo.Dblclick(opts) //nolint:wrapcheck
			})
		},
		"setChecked": func(checked bool, opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, lo.SetChecked(checked, opts) //nolint:wrapcheck
			})
		},
		"check": func(opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, lo.Check(opts) //nolint:wrapcheck
			})
		},
		"uncheck": func(opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, lo.Uncheck(opts) //nolint:wrapcheck
			})
		},
		"isChecked": func(opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return lo.IsChecked(opts) //nolint:wrapcheck
			})
		},
		"isEditable": func(opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return lo.IsEditable(opts) //nolint:wrapcheck
			})
		},
		"isEnabled": func(opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return lo.IsEnabled(opts) //nolint:wrapcheck
			})
		},
		"isDisabled": func(opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return lo.IsDisabled(opts) //nolint:wrapcheck
			})
		},
		"isVisible": func() *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return lo.IsVisible() //nolint:wrapcheck
			})
		},
		"isHidden": func() *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return lo.IsHidden() //nolint:wrapcheck
			})
		},
		"fill": func(value string, opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, lo.Fill(value, opts) //nolint:wrapcheck
			})
		},
		"focus": func(opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, lo.Focus(opts) //nolint:wrapcheck
			})
		},
		"getAttribute": func(name string, opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				s, ok, err := lo.GetAttribute(name, opts)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				if !ok {
					return nil, nil
				}
				return s, nil
			})
		},
		"innerHTML": func(opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return lo.InnerHTML(opts) //nolint:wrapcheck
			})
		},
		"innerText": func(opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return lo.InnerText(opts) //nolint:wrapcheck
			})
		},
		"textContent": func(opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				s, ok, err := lo.TextContent(opts)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				if !ok {
					return nil, nil
				}
				return s, nil
			})
		},
		"inputValue": func(opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return lo.InputValue(opts) //nolint:wrapcheck
			})
		},
		"selectOption": func(values sobek.Value, opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return lo.SelectOption(values, opts) //nolint:wrapcheck
			})
		},
		"press": func(key string, opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, lo.Press(key, opts) //nolint:wrapcheck
			})
		},
		"type": func(text string, opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, lo.Type(text, opts) //nolint:wrapcheck
			})
		},
		"hover": func(opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, lo.Hover(opts) //nolint:wrapcheck
			})
		},
		"tap": func(opts sobek.Value) (*sobek.Promise, error) {
			copts := common.NewFrameTapOptions(lo.DefaultTimeout())
			if err := copts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing locator tap options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, lo.Tap(copts) //nolint:wrapcheck
			}), nil
		},
		"dispatchEvent": func(typ string, eventInit, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameDispatchEventOptions(lo.DefaultTimeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing locator dispatch event options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, lo.DispatchEvent(typ, exportArg(eventInit), popts) //nolint:wrapcheck
			}), nil
		},
		"waitFor": func(opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, lo.WaitFor(opts) //nolint:wrapcheck
			})
		},
	}
}
