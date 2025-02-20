package browser

import (
	"fmt"

	"github.com/grafana/sobek"

	"go.k6.io/k6/internal/js/modules/k6/browser/common"
	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext"
)

// mapFrame to the JS module.
//
//nolint:funlen,gocognit,cyclop
func mapFrame(vu moduleVU, f *common.Frame) mapping {
	maps := mapping{
		"check": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameCheckOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing new frame check options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, f.Check(selector, popts) //nolint:wrapcheck
			}), nil
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
		"dblclick": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameDblClickOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing double click options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, f.Dblclick(selector, popts) //nolint:wrapcheck
			}), nil
		},
		"dispatchEvent": func(selector, typ string, eventInit, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameDispatchEventOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing frame dispatch event options: %w", err)
			}
			earg := exportArg(eventInit)
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, f.DispatchEvent(selector, typ, earg, popts) //nolint:wrapcheck
			}), nil
		},
		"evaluate": func(pageFunc sobek.Value, gargs ...sobek.Value) (*sobek.Promise, error) {
			if sobekEmptyString(pageFunc) {
				return nil, fmt.Errorf("evaluate requires a page function")
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return f.Evaluate(pageFunc.String(), exportArgs(gargs)...)
			}), nil
		},
		"evaluateHandle": func(pageFunc sobek.Value, gargs ...sobek.Value) (*sobek.Promise, error) {
			if sobekEmptyString(pageFunc) {
				return nil, fmt.Errorf("evaluateHandle requires a page function")
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				jsh, err := f.EvaluateHandle(pageFunc.String(), exportArgs(gargs)...)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				return mapJSHandle(vu, jsh), nil
			}), nil
		},
		"fill": func(selector, value string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameFillOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing fill options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, f.Fill(selector, value, popts) //nolint:wrapcheck
			}), nil
		},
		"focus": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameBaseOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing focus options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, f.Focus(selector, popts) //nolint:wrapcheck
			}), nil
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
		"getAttribute": func(selector, name string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameBaseOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing getAttribute options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				s, ok, err := f.GetAttribute(selector, name, popts)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				if !ok {
					return nil, nil //nolint:nilnil
				}
				return s, nil
			}), nil
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
		"hover": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameHoverOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing hover options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, f.Hover(selector, popts) //nolint:wrapcheck
			}), nil
		},
		"innerHTML": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameInnerHTMLOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing inner HTML options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return f.InnerHTML(selector, popts) //nolint:wrapcheck
			}), nil
		},
		"innerText": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameInnerTextOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing inner text options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return f.InnerText(selector, popts) //nolint:wrapcheck
			}), nil
		},
		"inputValue": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameInputValueOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing input value options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return f.InputValue(selector, popts) //nolint:wrapcheck
			}), nil
		},
		"isChecked": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameIsCheckedOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing isChecked options of selector %q: %w", selector, err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return f.IsChecked(selector, popts) //nolint:wrapcheck
			}), nil
		},
		"isDetached": f.IsDetached,
		"isDisabled": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameIsDisabledOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing isDisabled options of selector %q: %w", selector, err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return f.IsDisabled(selector, popts) //nolint:wrapcheck
			}), nil
		},
		"isEditable": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameIsEditableOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parse isEditable options of selector %q: %w", selector, err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return f.IsEditable(selector, popts) //nolint:wrapcheck
			}), nil
		},
		"isEnabled": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameIsEnabledOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parse isEnabled options of selector %q: %w", selector, err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return f.IsEnabled(selector, popts) //nolint:wrapcheck
			}), nil
		},
		"isHidden": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameIsHiddenOptions()
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parse isHidden options of selector %q: %w", selector, err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return f.IsHidden(selector, popts) //nolint:wrapcheck
			}), nil
		},
		"isVisible": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameIsVisibleOptions()
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parse isVisible options of selector %q: %w", selector, err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return f.IsVisible(selector, popts) //nolint:wrapcheck
			}), nil
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
		"press": func(selector, key string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFramePressOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parse press options of selector %q on key %q: %w", selector, key, err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, f.Press(selector, key, popts) //nolint:wrapcheck
			}), nil
		},
		"selectOption": func(selector string, values sobek.Value, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameSelectOptionOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing select option options: %w", err)
			}

			convValues, err := common.ConvertSelectOptionValues(vu.Runtime(), values)
			if err != nil {
				return nil, fmt.Errorf("parsing select options values: %w", err)
			}

			return k6ext.Promise(vu.Context(), func() (any, error) {
				return f.SelectOption(selector, convValues, popts) //nolint:wrapcheck
			}), nil
		},
		"setChecked": func(selector string, checked bool, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameCheckOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing frame set check options: %w", err)
			}

			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, f.SetChecked(selector, checked, popts) //nolint:wrapcheck
			}), nil
		},
		"setContent": func(html string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameSetContentOptions(f.Page().NavigationTimeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing setContent options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, f.SetContent(html, popts) //nolint:wrapcheck
			}), nil
		},
		"setInputFiles": func(selector string, files sobek.Value, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameSetInputFilesOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing setInputFiles options: %w", err)
			}

			pfiles := new(common.Files)
			if err := pfiles.Parse(vu.Context(), files); err != nil {
				return nil, fmt.Errorf("parsing setInputFiles parameter: %w", err)
			}

			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, f.SetInputFiles(selector, pfiles, popts) //nolint:wrapcheck
			}), nil
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
		"textContent": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameTextContentOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing text content options: %w", err)
			}

			return k6ext.Promise(vu.Context(), func() (any, error) {
				s, ok, err := f.TextContent(selector, popts)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				if !ok {
					return nil, nil //nolint:nilnil
				}
				return s, nil
			}), nil
		},
		"title": func() *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return f.Title()
			})
		},
		"type": func(selector, text string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameTypeOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing type options: %w", err)
			}

			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, f.Type(selector, text, popts) //nolint:wrapcheck
			}), nil
		},
		"uncheck": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameUncheckOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing frame uncheck options %q: %w", selector, err)
			}

			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, f.Uncheck(selector, popts) //nolint:wrapcheck
			}), nil
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
		"waitForLoadState": func(state string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameWaitForLoadStateOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing waitForLoadState %q options: %w", state, err)
			}

			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, f.WaitForLoadState(state, popts) //nolint:wrapcheck
			}), nil
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
		"waitForSelector": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameWaitForSelectorOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing wait for selector %q options: %w", selector, err)
			}

			return k6ext.Promise(vu.Context(), func() (any, error) {
				eh, err := f.WaitForSelector(selector, popts)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				return mapElementHandle(vu, eh), nil
			}), nil
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
