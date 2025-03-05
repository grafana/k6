package browser

import (
	"fmt"

	"github.com/grafana/sobek"

	"go.k6.io/k6/internal/js/modules/k6/browser/common"
	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext"
)

// mapElementHandle to the JS module.
func mapElementHandle(vu moduleVU, eh *common.ElementHandle) mapping { //nolint:gocognit,funlen,cyclop
	rt := vu.Runtime()
	maps := mapping{
		"boundingBox": func() *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return eh.BoundingBox(), nil
			})
		},
		"check": func(opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewElementHandleSetCheckedOptions(eh.DefaultTimeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing check options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, eh.Check(popts) //nolint:wrapcheck
			}), nil
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
		"dblclick": func(opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewElementHandleDblclickOptions(eh.DefaultTimeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing element double click options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, eh.Dblclick(popts) //nolint:wrapcheck
			}), nil
		},
		"dispatchEvent": func(typ string, eventInit sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, eh.DispatchEvent(typ, exportArg(eventInit)) //nolint:wrapcheck
			})
		},
		"fill": func(value string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewElementHandleBaseOptions(eh.DefaultTimeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing element fill options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, eh.Fill(value, popts) //nolint:wrapcheck
			}), nil
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
		"hover": func(opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewElementHandleHoverOptions(eh.DefaultTimeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing element hover options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, eh.Hover(popts) //nolint:wrapcheck
			}), nil
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
		"inputValue": func(opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewElementHandleBaseOptions(eh.DefaultTimeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing element input value options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return eh.InputValue(popts) //nolint:wrapcheck
			}), nil
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
		"press": func(key string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewElementHandlePressOptions(eh.DefaultTimeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing press %q options: %w", key, err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, eh.Press(key, popts) //nolint:wrapcheck
			}), nil
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
		"scrollIntoViewIfNeeded": func(opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewElementHandleBaseOptions(eh.DefaultTimeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing scrollIntoViewIfNeeded options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, eh.ScrollIntoViewIfNeeded(popts) //nolint:wrapcheck
			}), nil
		},
		"selectOption": func(values sobek.Value, opts sobek.Value) (*sobek.Promise, error) {
			convValues, err := common.ConvertSelectOptionValues(vu.Runtime(), values)
			if err != nil {
				return nil, fmt.Errorf("parsing select options values: %w", err)
			}
			popts := common.NewElementHandleBaseOptions(eh.DefaultTimeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing selectOption options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return eh.SelectOption(convValues, popts) //nolint:wrapcheck
			}), nil
		},
		"selectText": func(opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewElementHandleBaseOptions(eh.DefaultTimeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing selectText options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, eh.SelectText(popts) //nolint:wrapcheck
			}), nil
		},
		"setChecked": func(checked bool, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewElementHandleSetCheckedOptions(eh.DefaultTimeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing setChecked options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, eh.SetChecked(checked, popts) //nolint:wrapcheck
			}), nil
		},
		"setInputFiles": func(files sobek.Value, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewElementHandleSetInputFilesOptions(eh.DefaultTimeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing setInputFiles options: %w", err)
			}
			var pfiles common.Files
			if err := pfiles.Parse(vu.Context(), files); err != nil {
				return nil, fmt.Errorf("parsing setInputFiles parameter: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, eh.SetInputFiles(&pfiles, popts) //nolint:wrapcheck
			}), nil
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
		"type": func(text string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewElementHandleTypeOptions(eh.DefaultTimeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing type options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, eh.Type(text, popts) //nolint:wrapcheck
			}), nil
		},
		"uncheck": func(opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewElementHandleSetCheckedOptions(eh.DefaultTimeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing uncheck options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, eh.Uncheck(popts) //nolint:wrapcheck
			}), nil
		},
		"waitForElementState": func(state string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewElementHandleWaitForElementStateOptions(eh.DefaultTimeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing waitForElementState options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, eh.WaitForElementState(state, popts) //nolint:wrapcheck
			}), nil
		},
		"waitForSelector": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameWaitForSelectorOptions(eh.DefaultTimeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing waitForSelector %q options: %w", selector, err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				eh, err := eh.WaitForSelector(selector, popts)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				return mapElementHandle(vu, eh), nil
			}), nil
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
