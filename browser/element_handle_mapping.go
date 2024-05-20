package browser

import (
	"fmt"

	"github.com/dop251/goja"

	"github.com/grafana/xk6-browser/common"
	"github.com/grafana/xk6-browser/k6ext"
)

// mapElementHandle to the JS module.
//
//nolint:funlen
func mapElementHandle(vu moduleVU, eh *common.ElementHandle) mapping { //nolint:cyclop
	rt := vu.Runtime()
	maps := mapping{
		"boundingBox": func() *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return eh.BoundingBox(), nil
			})
		},
		"check": func(opts goja.Value) *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, eh.Check(opts) //nolint:wrapcheck
			})
		},
		"click": func(opts goja.Value) (*goja.Promise, error) {
			popts := common.NewElementHandleClickOptions(eh.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing element click options: %w", err)
			}

			return k6ext.Promise(vu.Context(), func() (any, error) {
				err := eh.Click(popts)
				return nil, err //nolint:wrapcheck
			}), nil
		},
		"contentFrame": func() *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				f, err := eh.ContentFrame()
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				return mapFrame(vu, f), nil
			})
		},
		"dblclick": func(opts goja.Value) *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, eh.Dblclick(opts) //nolint:wrapcheck
			})
		},
		"dispatchEvent": func(typ string, eventInit goja.Value) *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, eh.DispatchEvent(typ, exportArg(eventInit)) //nolint:wrapcheck
			})
		},
		"fill": func(value string, opts goja.Value) *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, eh.Fill(value, opts) //nolint:wrapcheck
			})
		},
		"focus": func() *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, eh.Focus() //nolint:wrapcheck
			})
		},
		"getAttribute": func(name string) *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return eh.GetAttribute(name) //nolint:wrapcheck
			})
		},
		"hover": func(opts goja.Value) *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, eh.Hover(opts) //nolint:wrapcheck
			})
		},
		"innerHTML": func() *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return eh.InnerHTML() //nolint:wrapcheck
			})
		},
		"innerText":  eh.InnerText,
		"inputValue": eh.InputValue,
		"isChecked":  eh.IsChecked,
		"isDisabled": eh.IsDisabled,
		"isEditable": eh.IsEditable,
		"isEnabled":  eh.IsEnabled,
		"isHidden":   eh.IsHidden,
		"isVisible":  eh.IsVisible,
		"ownerFrame": func() (mapping, error) {
			f, err := eh.OwnerFrame()
			if err != nil {
				return nil, err //nolint:wrapcheck
			}
			return mapFrame(vu, f), nil
		},
		"press": eh.Press,
		"screenshot": func(opts goja.Value) (*goja.ArrayBuffer, error) {
			ctx := vu.Context()

			popts := common.NewElementHandleScreenshotOptions(eh.Timeout())
			if err := popts.Parse(ctx, opts); err != nil {
				return nil, fmt.Errorf("parsing frame screenshot options: %w", err)
			}

			bb, err := eh.Screenshot(popts, vu.filePersister)
			if err != nil {
				return nil, err //nolint:wrapcheck
			}

			ab := rt.NewArrayBuffer(bb)

			return &ab, nil
		},
		"scrollIntoViewIfNeeded": eh.ScrollIntoViewIfNeeded,
		"selectOption":           eh.SelectOption,
		"selectText":             eh.SelectText,
		"setInputFiles":          eh.SetInputFiles,
		"tap": func(opts goja.Value) (*goja.Promise, error) {
			popts := common.NewElementHandleTapOptions(eh.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing element tap options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, eh.Tap(popts) //nolint:wrapcheck
			}), nil
		},
		"textContent":         eh.TextContent,
		"type":                eh.Type,
		"uncheck":             eh.Uncheck,
		"waitForElementState": eh.WaitForElementState,
		"waitForSelector": func(selector string, opts goja.Value) (mapping, error) {
			eh, err := eh.WaitForSelector(selector, opts)
			if err != nil {
				return nil, err //nolint:wrapcheck
			}
			return mapElementHandle(vu, eh), nil
		},
	}
	maps["$"] = func(selector string) (mapping, error) {
		eh, err := eh.Query(selector, common.StrictModeOff)
		if err != nil {
			return nil, err //nolint:wrapcheck
		}
		// ElementHandle can be null when the selector does not match any elements.
		// We do not want to map nil elementHandles since the expectation is a
		// null result in the test script for this case.
		if eh == nil {
			return nil, nil //nolint:nilnil
		}
		ehm := mapElementHandle(vu, eh)

		return ehm, nil
	}
	maps["$$"] = func(selector string) ([]mapping, error) {
		ehs, err := eh.QueryAll(selector)
		if err != nil {
			return nil, err //nolint:wrapcheck
		}
		var mehs []mapping
		for _, eh := range ehs {
			ehm := mapElementHandle(vu, eh)
			mehs = append(mehs, ehm)
		}
		return mehs, nil
	}

	jsHandleMap := mapJSHandle(vu, eh)
	for k, v := range jsHandleMap {
		maps[k] = v
	}

	return maps
}
