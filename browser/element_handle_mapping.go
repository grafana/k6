package browser

import (
	"fmt"

	"github.com/grafana/sobek"

	"github.com/grafana/xk6-browser/common"
	"github.com/grafana/xk6-browser/k6ext"
)

// mapElementHandle to the JS module.
func mapElementHandle(vu moduleVU, eh *common.ElementHandle) mapping { //nolint:gocognit,cyclop,funlen
	rt := vu.Runtime()
	maps := mapping{
		"boundingBox": func() *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return eh.BoundingBox(), nil
			})
		},
		"check": func(opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, eh.Check(opts) //nolint:wrapcheck
			})
		},
		"click": func(opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewElementHandleClickOptions(eh.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing element click options: %w", err)
			}

			return k6ext.Promise(vu.Context(), func() (any, error) {
				err := eh.Click(popts)
				return nil, err //nolint:wrapcheck
			}), nil
		},
		"contentFrame": func() *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				f, err := eh.ContentFrame()
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				return mapFrame(vu, f), nil
			})
		},
		"dblclick": func(opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, eh.Dblclick(opts) //nolint:wrapcheck
			})
		},
		"dispatchEvent": func(typ string, eventInit sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, eh.DispatchEvent(typ, exportArg(eventInit)) //nolint:wrapcheck
			})
		},
		"fill": func(value string, opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, eh.Fill(value, opts) //nolint:wrapcheck
			})
		},
		"focus": func() *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, eh.Focus() //nolint:wrapcheck
			})
		},
		"getAttribute": func(name string) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				s, ok, err := eh.GetAttribute(name)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				if !ok {
					return nil, nil
				}
				return s, nil
			})
		},
		"hover": func(opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, eh.Hover(opts) //nolint:wrapcheck
			})
		},
		"innerHTML": func() *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return eh.InnerHTML() //nolint:wrapcheck
			})
		},
		"innerText": func() *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return eh.InnerText() //nolint:wrapcheck
			})
		},
		"inputValue": func(opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return eh.InputValue(opts) //nolint:wrapcheck
			})
		},
		"isChecked": func() *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return eh.IsChecked() //nolint:wrapcheck
			})
		},
		"isDisabled": func() *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return eh.IsDisabled() //nolint:wrapcheck
			})
		},
		"isEditable": func() *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return eh.IsEditable() //nolint:wrapcheck
			})
		},
		"isEnabled": func() *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return eh.IsEnabled() //nolint:wrapcheck
			})
		},
		"isHidden": func() *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return eh.IsHidden() //nolint:wrapcheck
			})
		},
		"isVisible": func() *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return eh.IsVisible() //nolint:wrapcheck
			})
		},
		"ownerFrame": func() *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				f, err := eh.OwnerFrame()
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				return mapFrame(vu, f), nil
			})
		},
		"press": func(key string, opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, eh.Press(key, opts) //nolint:wrapcheck
			})
		},
		"screenshot": func(opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewElementHandleScreenshotOptions(eh.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing element handle screenshot options: %w", err)
			}

			return k6ext.Promise(vu.Context(), func() (any, error) {
				bb, err := eh.Screenshot(popts, vu.filePersister)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}

				ab := rt.NewArrayBuffer(bb)

				return &ab, nil
			}), nil
		},
		"scrollIntoViewIfNeeded": func(opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, eh.ScrollIntoViewIfNeeded(opts) //nolint:wrapcheck
			})
		},
		"selectOption": func(values sobek.Value, opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return eh.SelectOption(values, opts) //nolint:wrapcheck
			})
		},
		"selectText": func(opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, eh.SelectText(opts) //nolint:wrapcheck
			})
		},
		"setInputFiles": func(files sobek.Value, opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, eh.SetInputFiles(files, opts) //nolint:wrapcheck
			})
		},
		"tap": func(opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewElementHandleTapOptions(eh.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing element tap options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, eh.Tap(popts) //nolint:wrapcheck
			}), nil
		},
		"textContent": func() *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				s, ok, err := eh.TextContent()
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				if !ok {
					return nil, nil
				}
				return s, nil
			})
		},
		"type": func(text string, opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, eh.Type(text, opts) //nolint:wrapcheck
			})
		},
		"uncheck": func(opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, eh.Uncheck(opts) //nolint:wrapcheck
			})
		},
		"waitForElementState": func(state string, opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, eh.WaitForElementState(state, opts) //nolint:wrapcheck
			})
		},
		"waitForSelector": func(selector string, opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				eh, err := eh.WaitForSelector(selector, opts)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				return mapElementHandle(vu, eh), nil
			})
		},
	}
	maps["$"] = func(selector string) *sobek.Promise {
		return k6ext.Promise(vu.Context(), func() (any, error) {
			eh, err := eh.Query(selector, common.StrictModeOff)
			if err != nil {
				return nil, err //nolint:wrapcheck
			}
			// ElementHandle can be null when the selector does not match any elements.
			// We do not want to map nil elementHandles since the expectation is a
			// null result in the test script for this case.
			if eh == nil {
				return nil, nil
			}
			ehm := mapElementHandle(vu, eh)

			return ehm, nil
		})
	}
	maps["$$"] = func(selector string) *sobek.Promise {
		return k6ext.Promise(vu.Context(), func() (any, error) {
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
		})
	}

	jsHandleMap := mapJSHandle(vu, eh)
	for k, v := range jsHandleMap {
		maps[k] = v
	}

	return maps
}
