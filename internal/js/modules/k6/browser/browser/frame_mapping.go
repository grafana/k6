package browser

import (
	"errors"
	"fmt"

	"github.com/grafana/sobek"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/common"
	k6common "go.k6.io/k6/v2/js/common"
)

// mapFrame to the JS module.
//
//nolint:funlen,gocognit,cyclop
func mapFrame(vu moduleVU, f *common.Frame) mapping {
	rt := vu.Runtime()
	maps := mapping{
		"check": networkCall(func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameCheckOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing new frame check options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return nil, f.Check(selector, popts) //nolint:wrapcheck
			}), nil
		}),
		"childFrames": passiveCall(func() []mapping {
			cfs := f.ChildFrames()
			mcfs := make([]mapping, 0, len(cfs))
			for _, fr := range cfs {
				mcfs = append(mcfs, mapFrame(vu, fr))
			}
			return mcfs
		}),
		"click": networkCall(func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts, err := parseFrameClickOptions(vu.Context(), opts, f.Timeout())
			if err != nil {
				return nil, err
			}

			return promise(vu, func() (any, error) {
				err := f.Click(selector, popts)
				return nil, err //nolint:wrapcheck
			}), nil
		}),
		"content": passiveCall(func() *sobek.Promise {
			return promise(vu, func() (any, error) {
				return f.Content() //nolint:wrapcheck
			})
		}),
		"dblclick": networkCall(func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameDblClickOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing double click options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return nil, f.Dblclick(selector, popts) //nolint:wrapcheck
			}), nil
		}),
		"dispatchEvent": networkCall(func(selector, typ string, eventInit, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameDispatchEventOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing frame dispatch event options: %w", err)
			}
			earg := exportArg(eventInit)
			return promise(vu, func() (any, error) {
				return nil, f.DispatchEvent(selector, typ, earg, popts) //nolint:wrapcheck
			}), nil
		}),
		"evaluate": networkCall(func(pageFunc sobek.Value, gargs ...sobek.Value) (*sobek.Promise, error) {
			if sobekEmptyString(pageFunc) {
				return nil, fmt.Errorf("evaluate requires a page function")
			}
			funcString := pageFunc.String()
			gopts := exportArgs(gargs)
			return promise(vu, func() (any, error) {
				return f.Evaluate(funcString, gopts...)
			}), nil
		}),
		"evaluateHandle": networkCall(func(pageFunc sobek.Value, gargs ...sobek.Value) (*sobek.Promise, error) {
			if sobekEmptyString(pageFunc) {
				return nil, fmt.Errorf("evaluateHandle requires a page function")
			}
			funcString := pageFunc.String()
			gopts := exportArgs(gargs)
			return promise(vu, func() (any, error) {
				jsh, err := f.EvaluateHandle(funcString, gopts...)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				return mapJSHandle(vu, jsh), nil
			}), nil
		}),
		"fill": networkCall(func(selector, value string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameFillOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing fill options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return nil, f.Fill(selector, value, popts) //nolint:wrapcheck
			}), nil
		}),
		"focus": networkCall(func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameBaseOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing focus options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return nil, f.Focus(selector, popts) //nolint:wrapcheck
			}), nil
		}),
		"frameElement": passiveCall(func() *sobek.Promise {
			return promise(vu, func() (any, error) {
				fe, err := f.FrameElement()
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				return mapElementHandle(vu, fe), nil
			})
		}),
		"getAttribute": passiveCall(func(selector, name string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameBaseOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing getAttribute options: %w", err)
			}
			return promise(vu, func() (any, error) {
				s, ok, err := f.GetAttribute(selector, name, popts)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				if !ok {
					return nil, nil //nolint:nilnil
				}
				return s, nil
			}), nil
		}),
		"getByAltText": passiveCall(func(alt sobek.Value, opts sobek.Value) (*sobek.Object, error) {
			if k6common.IsNullish(alt) {
				return nil, errors.New("missing required argument 'altText'")
			}
			palt, popts := parseGetByBaseOptions(vu.Context(), alt, false, opts)

			ml := mapLocator(vu, f.GetByAltText(palt, popts))
			return rt.ToValue(ml).ToObject(rt), nil
		}),
		"getByLabel": passiveCall(func(label sobek.Value, opts sobek.Value) (*sobek.Object, error) {
			if k6common.IsNullish(label) {
				return nil, errors.New("missing required argument 'label'")
			}
			plabel, popts := parseGetByBaseOptions(vu.Context(), label, true, opts)

			ml := mapLocator(vu, f.GetByLabel(plabel, popts))
			return rt.ToValue(ml).ToObject(rt), nil
		}),
		"getByPlaceholder": passiveCall(func(placeholder sobek.Value, opts sobek.Value) (*sobek.Object, error) {
			if k6common.IsNullish(placeholder) {
				return nil, errors.New("missing required argument 'placeholder'")
			}
			pplaceholder, popts := parseGetByBaseOptions(vu.Context(), placeholder, false, opts)

			ml := mapLocator(vu, f.GetByPlaceholder(pplaceholder, popts))
			return rt.ToValue(ml).ToObject(rt), nil
		}),
		"getByRole": passiveCall(func(role sobek.Value, opts sobek.Value) (*sobek.Object, error) {
			if k6common.IsNullish(role) {
				return nil, errors.New("missing required argument 'role'")
			}
			popts := parseGetByRoleOptions(vu.Context(), opts)

			ml := mapLocator(vu, f.GetByRole(role.String(), popts))
			return rt.ToValue(ml).ToObject(rt), nil
		}),
		"getByTestId": passiveCall(func(testID sobek.Value) (*sobek.Object, error) {
			if k6common.IsNullish(testID) {
				return nil, errors.New("missing required argument 'testId'")
			}
			ptestID := parseStringOrRegex(testID, false)

			ml := mapLocator(vu, f.GetByTestID(ptestID))
			return rt.ToValue(ml).ToObject(rt), nil
		}),
		"getByText": passiveCall(func(text sobek.Value, opts sobek.Value) (*sobek.Object, error) {
			if k6common.IsNullish(text) {
				return nil, errors.New("missing required argument 'text'")
			}
			ptext, popts := parseGetByBaseOptions(vu.Context(), text, true, opts)

			ml := mapLocator(vu, f.GetByText(ptext, popts))
			return rt.ToValue(ml).ToObject(rt), nil
		}),
		"getByTitle": passiveCall(func(title sobek.Value, opts sobek.Value) (*sobek.Object, error) {
			if k6common.IsNullish(title) {
				return nil, errors.New("missing required argument 'title'")
			}
			ptitle, popts := parseGetByBaseOptions(vu.Context(), title, false, opts)

			ml := mapLocator(vu, f.GetByTitle(ptitle, popts))
			return rt.ToValue(ml).ToObject(rt), nil
		}),
		"goto": networkCall(func(url string, opts sobek.Value) (*sobek.Promise, error) {
			gopts := common.NewFrameGotoOptions(
				f.Referrer(),
				f.NavigationTimeout(),
			)
			if err := gopts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing frame navigation options to %q: %w", url, err)
			}
			return promise(vu, func() (any, error) {
				resp, err := f.Goto(url, gopts)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}

				return mapResponse(vu, resp), nil
			}), nil
		}),
		"hover": networkCall(func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameHoverOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing hover options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return nil, f.Hover(selector, popts) //nolint:wrapcheck
			}), nil
		}),
		"innerHTML": passiveCall(func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameInnerHTMLOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing inner HTML options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return f.InnerHTML(selector, popts) //nolint:wrapcheck
			}), nil
		}),
		"innerText": passiveCall(func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameInnerTextOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing inner text options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return f.InnerText(selector, popts) //nolint:wrapcheck
			}), nil
		}),
		"inputValue": passiveCall(func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameInputValueOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing input value options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return f.InputValue(selector, popts) //nolint:wrapcheck
			}), nil
		}),
		"isChecked": passiveCall(func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameIsCheckedOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing isChecked options of selector %q: %w", selector, err)
			}
			return promise(vu, func() (any, error) {
				return f.IsChecked(selector, popts) //nolint:wrapcheck
			}), nil
		}),
		"isDetached": passiveCall(f.IsDetached),
		"isDisabled": passiveCall(func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameIsDisabledOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing isDisabled options of selector %q: %w", selector, err)
			}
			return promise(vu, func() (any, error) {
				return f.IsDisabled(selector, popts) //nolint:wrapcheck
			}), nil
		}),
		"isEditable": passiveCall(func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameIsEditableOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parse isEditable options of selector %q: %w", selector, err)
			}
			return promise(vu, func() (any, error) {
				return f.IsEditable(selector, popts) //nolint:wrapcheck
			}), nil
		}),
		"isEnabled": passiveCall(func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameIsEnabledOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parse isEnabled options of selector %q: %w", selector, err)
			}
			return promise(vu, func() (any, error) {
				return f.IsEnabled(selector, popts) //nolint:wrapcheck
			}), nil
		}),
		"isHidden": passiveCall(func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameIsHiddenOptions()
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parse isHidden options of selector %q: %w", selector, err)
			}
			return promise(vu, func() (any, error) {
				return f.IsHidden(selector, popts) //nolint:wrapcheck
			}), nil
		}),
		"isVisible": passiveCall(func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameIsVisibleOptions()
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parse isVisible options of selector %q: %w", selector, err)
			}
			return promise(vu, func() (any, error) {
				return f.IsVisible(selector, popts) //nolint:wrapcheck
			}), nil
		}),
		"locator": passiveCall(func(selector string, opts sobek.Value) mapping {
			return mapLocator(vu, f.Locator(selector, parseLocatorOptions(rt, opts)))
		}),
		"frameLocator": passiveCall(func(selector string) *sobek.Object {
			mfl := mapFrameLocator(vu, f.FrameLocator(selector))
			return rt.ToValue(mfl).ToObject(rt)
		}),
		"name": passiveCall(f.Name),
		"page": passiveCall(func() mapping {
			return mapPage(vu, f.Page())
		}),
		"parentFrame": passiveCall(func() mapping {
			return mapFrame(vu, f.ParentFrame())
		}),
		"press": networkCall(func(selector, key string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFramePressOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parse press options of selector %q on key %q: %w", selector, key, err)
			}
			return promise(vu, func() (any, error) {
				return nil, f.Press(selector, key, popts) //nolint:wrapcheck
			}), nil
		}),
		"selectOption": networkCall(func(selector string, values sobek.Value, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameSelectOptionOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing select option options: %w", err)
			}

			convValues, err := ConvertSelectOptionValues(rt, values)
			if err != nil {
				return nil, fmt.Errorf("parsing select options values: %w", err)
			}

			return promise(vu, func() (any, error) {
				return f.SelectOption(selector, convValues, popts) //nolint:wrapcheck
			}), nil
		}),
		"setChecked": networkCall(func(selector string, checked bool, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameCheckOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing frame set check options: %w", err)
			}

			return promise(vu, func() (any, error) {
				return nil, f.SetChecked(selector, checked, popts) //nolint:wrapcheck
			}), nil
		}),
		"setContent": networkCall(func(html string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameSetContentOptions(f.Page().NavigationTimeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing setContent options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return nil, f.SetContent(html, popts) //nolint:wrapcheck
			}), nil
		}),
		"setInputFiles": networkCall(func(selector string, files sobek.Value, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameSetInputFilesOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing setInputFiles options: %w", err)
			}

			pfiles := new(common.Files)
			if err := pfiles.Parse(vu.Context(), files); err != nil {
				return nil, fmt.Errorf("parsing setInputFiles parameter: %w", err)
			}

			return promise(vu, func() (any, error) {
				return nil, f.SetInputFiles(selector, pfiles, popts) //nolint:wrapcheck
			}), nil
		}),
		"tap": networkCall(func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameTapOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing frame tap options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return nil, f.Tap(selector, popts) //nolint:wrapcheck
			}), nil
		}),
		"textContent": passiveCall(func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameTextContentOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing text content options: %w", err)
			}

			return promise(vu, func() (any, error) {
				s, ok, err := f.TextContent(selector, popts)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				if !ok {
					return nil, nil //nolint:nilnil
				}
				return s, nil
			}), nil
		}),
		"title": passiveCall(func() *sobek.Promise {
			return promise(vu, func() (any, error) {
				return f.Title()
			})
		}),
		"type": networkCall(func(selector, text string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameTypeOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing type options: %w", err)
			}

			return promise(vu, func() (any, error) {
				return nil, f.Type(selector, text, popts) //nolint:wrapcheck
			}), nil
		}),
		"uncheck": networkCall(func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameUncheckOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing frame uncheck options %q: %w", selector, err)
			}

			return promise(vu, func() (any, error) {
				return nil, f.Uncheck(selector, popts) //nolint:wrapcheck
			}), nil
		}),
		"url": passiveCall(f.URL),
		"waitForFunction": networkCall(func(pageFunc, opts sobek.Value, args ...sobek.Value) (*sobek.Promise, error) {
			js, popts, pargs, err := parseWaitForFunctionArgs(
				vu.Context(), f.Timeout(), pageFunc, opts, args...,
			)
			if err != nil {
				return nil, fmt.Errorf("frame waitForFunction: %w", err)
			}

			return promise(vu, func() (result any, reason error) {
				return f.WaitForFunction(js, popts, pargs...) //nolint:wrapcheck
			}), nil
		}),
		"waitForLoadState": passiveCall(func(state string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameWaitForLoadStateOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing waitForLoadState %q options: %w", state, err)
			}

			return promise(vu, func() (any, error) {
				return nil, f.WaitForLoadState(state, popts) //nolint:wrapcheck
			}), nil
		}),
		"waitForNavigation": passiveCall(func(opts sobek.Value) (*sobek.Promise, error) {
			return mapWaitForNavigation(vu, f, opts)
		}),
		"waitForSelector": passiveCall(func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameWaitForSelectorOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing wait for selector %q options: %w", selector, err)
			}

			return promise(vu, func() (any, error) {
				eh, err := f.WaitForSelector(selector, popts)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				return mapElementHandle(vu, eh), nil
			}), nil
		}),
		"waitForTimeout": passiveCall(func(timeout int64) *sobek.Promise {
			return promise(vu, func() (any, error) {
				f.WaitForTimeout(timeout)
				return nil, nil
			})
		}),
		"waitForURL": passiveCall(func(url sobek.Value, opts sobek.Value) (*sobek.Promise, error) {
			return mapWaitForURL(vu, f, url, opts)
		}),
	}
	maps["$"] = passiveCall(func(selector string) *sobek.Promise {
		return promise(vu, func() (any, error) {
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
	})
	maps["$$"] = passiveCall(func(selector string) *sobek.Promise {
		return promise(vu, func() (any, error) {
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
	})

	return withPageNetworkCalls(vu, f.Page(), maps)
}
