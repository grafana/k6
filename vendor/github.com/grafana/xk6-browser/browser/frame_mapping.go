package browser

import (
	"fmt"

	"github.com/grafana/sobek"

	"github.com/grafana/xk6-browser/common"
	"github.com/grafana/xk6-browser/k6ext"
)

// mapFrame to the JS module.
//
//nolint:funlen
func mapFrame(vu moduleVU, f *common.Frame) mapping { //nolint:gocognit,cyclop
	maps := mapping{
		"check": func(selector string, opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, f.Check(selector, opts) //nolint:wrapcheck
			})
		},
		"childFrames": func() []mapping {
			var (
				mcfs []mapping
				cfs  = f.ChildFrames()
			)
			for _, fr := range cfs {
				mcfs = append(mcfs, mapFrame(vu, fr))
			}
			return mcfs
		},
		"click": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts, err := parseFrameClickOptions(vu.Context(), opts, f.Timeout())
			if err != nil {
				return nil, err
			}

			return k6ext.Promise(vu.Context(), func() (any, error) {
				err := f.Click(selector, popts)
				return nil, err //nolint:wrapcheck
			}), nil
		},
		"content": func() *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return f.Content() //nolint:wrapcheck
			})
		},
		"dblclick": func(selector string, opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, f.Dblclick(selector, opts) //nolint:wrapcheck
			})
		},
		"dispatchEvent": func(selector, typ string, eventInit, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameDispatchEventOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing frame dispatch event options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, f.DispatchEvent(selector, typ, exportArg(eventInit), popts) //nolint:wrapcheck
			}), nil
		},
		"evaluate": func(pageFunction sobek.Value, gargs ...sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return f.Evaluate(pageFunction.String(), exportArgs(gargs)...) //nolint:wrapcheck
			})
		},
		"evaluateHandle": func(pageFunction sobek.Value, gargs ...sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				jsh, err := f.EvaluateHandle(pageFunction.String(), exportArgs(gargs)...)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				return mapJSHandle(vu, jsh), nil
			})
		},
		"fill": func(selector, value string, opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, f.Fill(selector, value, opts) //nolint:wrapcheck
			})
		},
		"focus": func(selector string, opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, f.Focus(selector, opts) //nolint:wrapcheck
			})
		},
		"frameElement": func() *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				fe, err := f.FrameElement()
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				return mapElementHandle(vu, fe), nil
			})
		},
		"getAttribute": func(selector, name string, opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				s, ok, err := f.GetAttribute(selector, name, opts)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				if !ok {
					return nil, nil
				}
				return s, nil
			})
		},
		"goto": func(url string, opts sobek.Value) (*sobek.Promise, error) {
			gopts := common.NewFrameGotoOptions(
				f.Referrer(),
				f.NavigationTimeout(),
			)
			if err := gopts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing frame navigation options to %q: %w", url, err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				resp, err := f.Goto(url, gopts)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}

				return mapResponse(vu, resp), nil
			}), nil
		},
		"hover": func(selector string, opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, f.Hover(selector, opts) //nolint:wrapcheck
			})
		},
		"innerHTML": func(selector string, opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return f.InnerHTML(selector, opts) //nolint:wrapcheck
			})
		},
		"innerText": func(selector string, opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return f.InnerText(selector, opts) //nolint:wrapcheck
			})
		},
		"inputValue": func(selector string, opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return f.InputValue(selector, opts) //nolint:wrapcheck
			})
		},
		"isChecked": func(selector string, opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return f.IsChecked(selector, opts) //nolint:wrapcheck
			})
		},
		"isDetached": f.IsDetached,
		"isDisabled": func(selector string, opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return f.IsDisabled(selector, opts) //nolint:wrapcheck
			})
		},
		"isEditable": func(selector string, opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return f.IsEditable(selector, opts) //nolint:wrapcheck
			})
		},
		"isEnabled": func(selector string, opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return f.IsEnabled(selector, opts) //nolint:wrapcheck
			})
		},
		"isHidden": func(selector string, opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return f.IsHidden(selector, opts) //nolint:wrapcheck
			})
		},
		"isVisible": func(selector string, opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return f.IsVisible(selector, opts) //nolint:wrapcheck
			})
		},
		"locator": func(selector string, opts sobek.Value) mapping {
			return mapLocator(vu, f.Locator(selector, opts))
		},
		"name": f.Name,
		"page": func() mapping {
			return mapPage(vu, f.Page())
		},
		"parentFrame": func() mapping {
			return mapFrame(vu, f.ParentFrame())
		},
		"press": func(selector, key string, opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, f.Press(selector, key, opts) //nolint:wrapcheck
			})
		},
		"selectOption": func(selector string, values sobek.Value, opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return f.SelectOption(selector, values, opts) //nolint:wrapcheck
			})
		},
		"setContent": func(html string, opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, f.SetContent(html, opts) //nolint:wrapcheck
			})
		},
		"setInputFiles": func(selector string, files sobek.Value, opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, f.SetInputFiles(selector, files, opts) //nolint:wrapcheck
			})
		},
		"tap": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameTapOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing frame tap options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, f.Tap(selector, popts) //nolint:wrapcheck
			}), nil
		},
		"textContent": func(selector string, opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				s, ok, err := f.TextContent(selector, opts)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				if !ok {
					return nil, nil
				}
				return s, nil
			})
		},
		"title": func() *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return f.Title(), nil
			})
		},
		"type": func(selector, text string, opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, f.Type(selector, text, opts) //nolint:wrapcheck
			})
		},
		"uncheck": func(selector string, opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, f.Uncheck(selector, opts) //nolint:wrapcheck
			})
		},
		"url": f.URL,
		"waitForFunction": func(pageFunc, opts sobek.Value, args ...sobek.Value) (*sobek.Promise, error) {
			js, popts, pargs, err := parseWaitForFunctionArgs(
				vu.Context(), f.Timeout(), pageFunc, opts, args...,
			)
			if err != nil {
				return nil, fmt.Errorf("frame waitForFunction: %w", err)
			}

			return k6ext.Promise(vu.Context(), func() (result any, reason error) {
				return f.WaitForFunction(js, popts, pargs...) //nolint:wrapcheck
			}), nil
		},
		"waitForLoadState": func(state string, opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, f.WaitForLoadState(state, opts) //nolint:wrapcheck
			})
		},
		"waitForNavigation": func(opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameWaitForNavigationOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing frame wait for navigation options: %w", err)
			}

			return k6ext.Promise(vu.Context(), func() (result any, reason error) {
				resp, err := f.WaitForNavigation(popts)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				return mapResponse(vu, resp), nil
			}), nil
		},
		"waitForSelector": func(selector string, opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				eh, err := f.WaitForSelector(selector, opts)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				return mapElementHandle(vu, eh), nil
			})
		},
		"waitForTimeout": func(timeout int64) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				f.WaitForTimeout(timeout)
				return nil, nil
			})
		},
	}
	maps["$"] = func(selector string) *sobek.Promise {
		return k6ext.Promise(vu.Context(), func() (any, error) {
			eh, err := f.Query(selector, common.StrictModeOff)
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
			ehs, err := f.QueryAll(selector)
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

	return maps
}
