package browser

import (
	"errors"
	"fmt"
	"time"

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
		"check": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameCheckOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing new frame check options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return nil, f.Check(selector, popts) //nolint:wrapcheck
			}), nil
		},
		"childFrames": func() []mapping {
			cfs := f.ChildFrames()
			mcfs := make([]mapping, 0, len(cfs))
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

			return promise(vu, func() (any, error) {
				err := f.Click(selector, popts)
				return nil, err //nolint:wrapcheck
			}), nil
		},
		"content": func() *sobek.Promise {
			return promise(vu, func() (any, error) {
				return f.Content() //nolint:wrapcheck
			})
		},
		"dblclick": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameDblClickOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing double click options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return nil, f.Dblclick(selector, popts) //nolint:wrapcheck
			}), nil
		},
		"dispatchEvent": func(selector, typ string, eventInit, opts sobek.Value) (*sobek.Promise, error) {
			popts, err := parseFrameDispatchEventOptions(rt, opts, f.Timeout())
			if err != nil {
				return nil, fmt.Errorf("parsing frame dispatch event options: %w", err)
			}
			earg := exportArg(eventInit)
			return promise(vu, func() (any, error) {
				return nil, f.DispatchEvent(selector, typ, earg, popts) //nolint:wrapcheck
			}), nil
		},
		"evaluate": func(pageFunc sobek.Value, gargs ...sobek.Value) (*sobek.Promise, error) {
			if sobekEmptyString(pageFunc) {
				return nil, fmt.Errorf("evaluate requires a page function")
			}
			funcString := pageFunc.String()
			gopts := exportArgs(gargs)
			return promise(vu, func() (any, error) {
				return f.Evaluate(funcString, gopts...)
			}), nil
		},
		"evaluateHandle": func(pageFunc sobek.Value, gargs ...sobek.Value) (*sobek.Promise, error) {
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
		},
		"fill": func(selector, value string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameFillOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing fill options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return nil, f.Fill(selector, value, popts) //nolint:wrapcheck
			}), nil
		},
		"focus": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameBaseOptions(f.Timeout())
			if err := parseFrameBaseOptions(popts, rt, opts); err != nil {
				return nil, fmt.Errorf("parsing focus options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return nil, f.Focus(selector, popts) //nolint:wrapcheck
			}), nil
		},
		"frameElement": func() *sobek.Promise {
			return promise(vu, func() (any, error) {
				fe, err := f.FrameElement()
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				return mapElementHandle(vu, fe), nil
			})
		},
		"getAttribute": func(selector, name string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameBaseOptions(f.Timeout())
			if err := parseFrameBaseOptions(popts, rt, opts); err != nil {
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
		},
		"getByAltText": func(alt sobek.Value, opts sobek.Value) (*sobek.Object, error) {
			if k6common.IsNullish(alt) {
				return nil, errors.New("missing required argument 'altText'")
			}
			palt, popts := parseGetByBaseOptions(vu.Context(), alt, false, opts)

			ml := mapLocator(vu, f.GetByAltText(palt, popts))
			return rt.ToValue(ml).ToObject(rt), nil
		},
		"getByLabel": func(label sobek.Value, opts sobek.Value) (*sobek.Object, error) {
			if k6common.IsNullish(label) {
				return nil, errors.New("missing required argument 'label'")
			}
			plabel, popts := parseGetByBaseOptions(vu.Context(), label, true, opts)

			ml := mapLocator(vu, f.GetByLabel(plabel, popts))
			return rt.ToValue(ml).ToObject(rt), nil
		},
		"getByPlaceholder": func(placeholder sobek.Value, opts sobek.Value) (*sobek.Object, error) {
			if k6common.IsNullish(placeholder) {
				return nil, errors.New("missing required argument 'placeholder'")
			}
			pplaceholder, popts := parseGetByBaseOptions(vu.Context(), placeholder, false, opts)

			ml := mapLocator(vu, f.GetByPlaceholder(pplaceholder, popts))
			return rt.ToValue(ml).ToObject(rt), nil
		},
		"getByRole": func(role sobek.Value, opts sobek.Value) (*sobek.Object, error) {
			if k6common.IsNullish(role) {
				return nil, errors.New("missing required argument 'role'")
			}
			popts := parseGetByRoleOptions(vu.Context(), opts)

			ml := mapLocator(vu, f.GetByRole(role.String(), popts))
			return rt.ToValue(ml).ToObject(rt), nil
		},
		"getByTestId": func(testID sobek.Value) (*sobek.Object, error) {
			if k6common.IsNullish(testID) {
				return nil, errors.New("missing required argument 'testId'")
			}
			ptestID := parseStringOrRegex(testID, false)

			ml := mapLocator(vu, f.GetByTestID(ptestID))
			return rt.ToValue(ml).ToObject(rt), nil
		},
		"getByText": func(text sobek.Value, opts sobek.Value) (*sobek.Object, error) {
			if k6common.IsNullish(text) {
				return nil, errors.New("missing required argument 'text'")
			}
			ptext, popts := parseGetByBaseOptions(vu.Context(), text, true, opts)

			ml := mapLocator(vu, f.GetByText(ptext, popts))
			return rt.ToValue(ml).ToObject(rt), nil
		},
		"getByTitle": func(title sobek.Value, opts sobek.Value) (*sobek.Object, error) {
			if k6common.IsNullish(title) {
				return nil, errors.New("missing required argument 'title'")
			}
			ptitle, popts := parseGetByBaseOptions(vu.Context(), title, false, opts)

			ml := mapLocator(vu, f.GetByTitle(ptitle, popts))
			return rt.ToValue(ml).ToObject(rt), nil
		},
		"goto": func(url string, opts sobek.Value) (*sobek.Promise, error) {
			gopts, err := parseFrameGotoOptions(rt, opts, f.Referrer(), f.NavigationTimeout())
			if err != nil {
				return nil, fmt.Errorf("parsing frame navigation options to %q: %w", url, err)
			}
			return promise(vu, func() (any, error) {
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
			return promise(vu, func() (any, error) {
				return nil, f.Hover(selector, popts) //nolint:wrapcheck
			}), nil
		},
		"innerHTML": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts, err := parseFrameInnerHTMLOptions(rt, opts, f.Timeout())
			if err != nil {
				return nil, fmt.Errorf("parsing inner HTML options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return f.InnerHTML(selector, popts) //nolint:wrapcheck
			}), nil
		},
		"innerText": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts, err := parseFrameInnerTextOptions(rt, opts, f.Timeout())
			if err != nil {
				return nil, fmt.Errorf("parsing inner text options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return f.InnerText(selector, popts) //nolint:wrapcheck
			}), nil
		},
		"inputValue": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts, err := parseFrameInputValueOptions(rt, opts, f.Timeout())
			if err != nil {
				return nil, fmt.Errorf("parsing input value options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return f.InputValue(selector, popts) //nolint:wrapcheck
			}), nil
		},
		"isChecked": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts, err := parseFrameIsCheckedOptions(rt, opts, f.Timeout())
			if err != nil {
				return nil, fmt.Errorf("parsing isChecked options of selector %q: %w", selector, err)
			}
			return promise(vu, func() (any, error) {
				return f.IsChecked(selector, popts) //nolint:wrapcheck
			}), nil
		},
		"isDetached": f.IsDetached,
		"isDisabled": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts, err := parseFrameIsDisabledOptions(rt, opts, f.Timeout())
			if err != nil {
				return nil, fmt.Errorf("parsing isDisabled options of selector %q: %w", selector, err)
			}
			return promise(vu, func() (any, error) {
				return f.IsDisabled(selector, popts) //nolint:wrapcheck
			}), nil
		},
		"isEditable": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts, err := parseFrameIsEditableOptions(rt, opts, f.Timeout())
			if err != nil {
				return nil, fmt.Errorf("parse isEditable options of selector %q: %w", selector, err)
			}
			return promise(vu, func() (any, error) {
				return f.IsEditable(selector, popts) //nolint:wrapcheck
			}), nil
		},
		"isEnabled": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts, err := parseFrameIsEnabledOptions(rt, opts, f.Timeout())
			if err != nil {
				return nil, fmt.Errorf("parse isEnabled options of selector %q: %w", selector, err)
			}
			return promise(vu, func() (any, error) {
				return f.IsEnabled(selector, popts) //nolint:wrapcheck
			}), nil
		},
		"isHidden": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts, err := parseFrameIsHiddenOptions(rt, opts)
			if err != nil {
				return nil, fmt.Errorf("parse isHidden options of selector %q: %w", selector, err)
			}
			return promise(vu, func() (any, error) {
				return f.IsHidden(selector, popts) //nolint:wrapcheck
			}), nil
		},
		"isVisible": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts, err := parseFrameIsVisibleOptions(rt, opts)
			if err != nil {
				return nil, fmt.Errorf("parse isVisible options of selector %q: %w", selector, err)
			}
			return promise(vu, func() (any, error) {
				return f.IsVisible(selector, popts) //nolint:wrapcheck
			}), nil
		},
		"locator": func(selector string, opts sobek.Value) mapping {
			return mapLocator(vu, f.Locator(selector, parseLocatorOptions(rt, opts)))
		},
		"frameLocator": func(selector string) *sobek.Object {
			mfl := mapFrameLocator(vu, f.FrameLocator(selector))
			return rt.ToValue(mfl).ToObject(rt)
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
			return promise(vu, func() (any, error) {
				return nil, f.Press(selector, key, popts) //nolint:wrapcheck
			}), nil
		},
		"selectOption": func(selector string, values sobek.Value, opts sobek.Value) (*sobek.Promise, error) {
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
		},
		"setChecked": func(selector string, checked bool, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameCheckOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing frame set check options: %w", err)
			}

			return promise(vu, func() (any, error) {
				return nil, f.SetChecked(selector, checked, popts) //nolint:wrapcheck
			}), nil
		},
		"setContent": func(html string, opts sobek.Value) (*sobek.Promise, error) {
			popts, err := parseFrameSetContentOptions(rt, opts, f.Page().NavigationTimeout())
			if err != nil {
				return nil, fmt.Errorf("parsing setContent options: %w", err)
			}
			return promise(vu, func() (any, error) {
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

			return promise(vu, func() (any, error) {
				return nil, f.SetInputFiles(selector, pfiles, popts) //nolint:wrapcheck
			}), nil
		},
		"tap": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameTapOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing frame tap options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return nil, f.Tap(selector, popts) //nolint:wrapcheck
			}), nil
		},
		"textContent": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts, err := parseFrameTextContentOptions(rt, opts, f.Timeout())
			if err != nil {
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
		},
		"title": func() *sobek.Promise {
			return promise(vu, func() (any, error) {
				return f.Title()
			})
		},
		"type": func(selector, text string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameTypeOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing type options: %w", err)
			}

			return promise(vu, func() (any, error) {
				return nil, f.Type(selector, text, popts) //nolint:wrapcheck
			}), nil
		},
		"uncheck": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameUncheckOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing frame uncheck options %q: %w", selector, err)
			}

			return promise(vu, func() (any, error) {
				return nil, f.Uncheck(selector, popts) //nolint:wrapcheck
			}), nil
		},
		"url": f.URL,
		"waitForFunction": func(pageFunc, opts sobek.Value, args ...sobek.Value) (*sobek.Promise, error) {
			js, popts, pargs, err := parseWaitForFunctionArgs(
				rt, f.Timeout(), pageFunc, opts, args...,
			)
			if err != nil {
				return nil, fmt.Errorf("frame waitForFunction: %w", err)
			}

			return promise(vu, func() (result any, reason error) {
				return f.WaitForFunction(js, popts, pargs...) //nolint:wrapcheck
			}), nil
		},
		"waitForLoadState": func(state string, opts sobek.Value) (*sobek.Promise, error) {
			popts, err := parseFrameWaitForLoadStateOptions(rt, opts, f.Timeout())
			if err != nil {
				return nil, fmt.Errorf("parsing waitForLoadState %q options: %w", state, err)
			}

			return promise(vu, func() (any, error) {
				return nil, f.WaitForLoadState(state, popts) //nolint:wrapcheck
			}), nil
		},
		"waitForNavigation": func(opts sobek.Value) (*sobek.Promise, error) {
			return mapWaitForNavigation(vu, f, opts)
		},
		"waitForSelector": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts, err := parseFrameWaitForSelectorOptions(rt, opts, f.Timeout())
			if err != nil {
				return nil, fmt.Errorf("parsing wait for selector %q options: %w", selector, err)
			}

			return promise(vu, func() (any, error) {
				eh, err := f.WaitForSelector(selector, popts)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				return mapElementHandle(vu, eh), nil
			}), nil
		},
		"waitForTimeout": func(timeout int64) *sobek.Promise {
			return promise(vu, func() (any, error) {
				f.WaitForTimeout(timeout)
				return nil, nil
			})
		},
		"waitForURL": func(url sobek.Value, opts sobek.Value) (*sobek.Promise, error) {
			return mapWaitForURL(rt, vu, f, url, opts)
		},
	}
	maps["$"] = func(selector string) *sobek.Promise {
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
	}
	maps["$$"] = func(selector string) *sobek.Promise {
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
	}

	return maps
}

// parseFrameGotoOptions parses the frame goto options from a Sobek value.
func parseFrameGotoOptions(
	rt *sobek.Runtime, opts sobek.Value, defaultReferrer string, defaultTimeout time.Duration,
) (*common.FrameGotoOptions, error) {
	gopts := common.NewFrameGotoOptions(defaultReferrer, defaultTimeout)
	if k6common.IsNullish(opts) {
		return gopts, nil
	}
	obj := opts.ToObject(rt)
	for _, k := range obj.Keys() {
		switch k {
		case "referer":
			gopts.Referer = obj.Get(k).String()
		case "timeout":
			gopts.Timeout = time.Duration(obj.Get(k).ToInteger()) * time.Millisecond
		case "waitUntil":
			lifeCycle := obj.Get(k).String()
			if err := gopts.WaitUntil.UnmarshalText([]byte(lifeCycle)); err != nil {
				return gopts, fmt.Errorf("parsing goto options: %w", err)
			}
		}
	}
	return gopts, nil
}

// parseFrameSetContentOptions parses the frame setContent options from a Sobek value.
func parseFrameSetContentOptions(
	rt *sobek.Runtime, opts sobek.Value, defaultTimeout time.Duration,
) (*common.FrameSetContentOptions, error) {
	scopts := common.NewFrameSetContentOptions(defaultTimeout)
	if k6common.IsNullish(opts) {
		return scopts, nil
	}
	obj := opts.ToObject(rt)
	for _, k := range obj.Keys() {
		switch k {
		case "timeout":
			scopts.Timeout = time.Duration(obj.Get(k).ToInteger()) * time.Millisecond
		case "waitUntil":
			lifeCycle := obj.Get(k).String()
			if err := scopts.WaitUntil.UnmarshalText([]byte(lifeCycle)); err != nil {
				return scopts, fmt.Errorf("parsing setContent options: %w", err)
			}
		}
	}
	return scopts, nil
}

// parseFrameWaitForLoadStateOptions parses the frame waitForLoadState options from a Sobek value.
//
//nolint:unparam
func parseFrameWaitForLoadStateOptions(
	rt *sobek.Runtime, opts sobek.Value, defaultTimeout time.Duration,
) (*common.FrameWaitForLoadStateOptions, error) {
	wlsopts := common.NewFrameWaitForLoadStateOptions(defaultTimeout)
	if k6common.IsNullish(opts) {
		return wlsopts, nil
	}
	obj := opts.ToObject(rt)
	for _, k := range obj.Keys() {
		if k == "timeout" {
			wlsopts.Timeout = time.Duration(obj.Get(k).ToInteger()) * time.Millisecond
		}
	}
	return wlsopts, nil
}

// parseFrameInnerTextOptions parses the frame innerText options from a Sobek value.
func parseFrameInnerTextOptions(
	rt *sobek.Runtime, opts sobek.Value,
	defaultTimeout time.Duration,
) (*common.FrameInnerTextOptions, error) {
	itopts := common.NewFrameInnerTextOptions(defaultTimeout)
	if k6common.IsNullish(opts) {
		return itopts, nil
	}
	if err := parseFrameBaseOptions(&itopts.FrameBaseOptions, rt, opts); err != nil {
		return itopts, err
	}
	return itopts, nil
}

// parseFrameInnerHTMLOptions parses the frame innerHTML options from a Sobek value.
func parseFrameInnerHTMLOptions(
	rt *sobek.Runtime, opts sobek.Value,
	defaultTimeout time.Duration,
) (*common.FrameInnerHTMLOptions, error) {
	ihopts := common.NewFrameInnerHTMLOptions(defaultTimeout)
	if k6common.IsNullish(opts) {
		return ihopts, nil
	}
	if err := parseFrameBaseOptions(&ihopts.FrameBaseOptions, rt, opts); err != nil {
		return ihopts, err
	}
	return ihopts, nil
}

// parseFrameIsCheckedOptions parses the frame isChecked options from a Sobek value.
func parseFrameIsCheckedOptions(
	rt *sobek.Runtime, opts sobek.Value,
	defaultTimeout time.Duration,
) (*common.FrameIsCheckedOptions, error) {
	icopts := common.NewFrameIsCheckedOptions(defaultTimeout)
	if k6common.IsNullish(opts) {
		return icopts, nil
	}
	if err := parseFrameBaseOptions(&icopts.FrameBaseOptions, rt, opts); err != nil {
		return icopts, err
	}
	return icopts, nil
}

// parseFrameIsDisabledOptions parses the frame isDisabled options from a Sobek value.
func parseFrameIsDisabledOptions(
	rt *sobek.Runtime, opts sobek.Value,
	defaultTimeout time.Duration,
) (*common.FrameIsDisabledOptions, error) {
	idopts := common.NewFrameIsDisabledOptions(defaultTimeout)
	if k6common.IsNullish(opts) {
		return idopts, nil
	}
	if err := parseFrameBaseOptions(&idopts.FrameBaseOptions, rt, opts); err != nil {
		return idopts, err
	}
	return idopts, nil
}

// parseFrameInputValueOptions parses the frame inputValue options from a Sobek value.
func parseFrameInputValueOptions(
	rt *sobek.Runtime, opts sobek.Value,
	defaultTimeout time.Duration,
) (*common.FrameInputValueOptions, error) {
	ivopts := common.NewFrameInputValueOptions(defaultTimeout)
	if k6common.IsNullish(opts) {
		return ivopts, nil
	}
	if err := parseFrameBaseOptions(&ivopts.FrameBaseOptions, rt, opts); err != nil {
		return ivopts, err
	}
	return ivopts, nil
}

// parseFrameIsEditableOptions parses the frame isEditable options from a Sobek value.
func parseFrameIsEditableOptions(
	rt *sobek.Runtime, opts sobek.Value,
	defaultTimeout time.Duration,
) (*common.FrameIsEditableOptions, error) {
	ieopts := common.NewFrameIsEditableOptions(defaultTimeout)
	if k6common.IsNullish(opts) {
		return ieopts, nil
	}
	if err := parseFrameBaseOptions(&ieopts.FrameBaseOptions, rt, opts); err != nil {
		return ieopts, err
	}
	return ieopts, nil
}

// parseFrameIsEnabledOptions parses the frame isEnabled options from a Sobek value.
func parseFrameIsEnabledOptions(
	rt *sobek.Runtime, opts sobek.Value,
	defaultTimeout time.Duration,
) (*common.FrameIsEnabledOptions, error) {
	ieopts := common.NewFrameIsEnabledOptions(defaultTimeout)
	if k6common.IsNullish(opts) {
		return ieopts, nil
	}
	if err := parseFrameBaseOptions(&ieopts.FrameBaseOptions, rt, opts); err != nil {
		return ieopts, err
	}
	return ieopts, nil
}

// parseFrameTextContentOptions parses the frame textContent options from a Sobek value.
func parseFrameTextContentOptions(
	rt *sobek.Runtime, opts sobek.Value,
	defaultTimeout time.Duration,
) (*common.FrameTextContentOptions, error) {
	tcopts := common.NewFrameTextContentOptions(defaultTimeout)
	if k6common.IsNullish(opts) {
		return tcopts, nil
	}
	if err := parseFrameBaseOptions(&tcopts.FrameBaseOptions, rt, opts); err != nil {
		return tcopts, err
	}
	return tcopts, nil
}

// parseFrameDispatchEventOptions parses the frame dispatchEvent options from a Sobek value.
func parseFrameDispatchEventOptions(
	rt *sobek.Runtime, opts sobek.Value,
	defaultTimeout time.Duration,
) (*common.FrameDispatchEventOptions, error) {
	deopts := common.NewFrameDispatchEventOptions(defaultTimeout)
	if k6common.IsNullish(opts) {
		return deopts, nil
	}
	if err := parseFrameBaseOptions(&deopts.FrameBaseOptions, rt, opts); err != nil {
		return deopts, err
	}
	return deopts, nil
}

// parseFrameWaitForSelectorOptions parses the frame waitForSelector options from a Sobek value.
func parseFrameWaitForSelectorOptions(
	rt *sobek.Runtime, opts sobek.Value,
	defaultTimeout time.Duration,
) (*common.FrameWaitForSelectorOptions, error) {
	wsopts := common.NewFrameWaitForSelectorOptions(defaultTimeout)
	if k6common.IsNullish(opts) {
		return wsopts, nil
	}
	obj := opts.ToObject(rt)
	for _, k := range obj.Keys() {
		switch k {
		case "state":
			state := obj.Get(k).String()
			if s, ok := common.DOMElementStateIDFromString(state); ok {
				wsopts.State = s
			} else {
				return wsopts, fmt.Errorf("%q is not a valid DOM state", state)
			}
		case "strict":
			wsopts.Strict = obj.Get(k).ToBoolean()
		case "timeout":
			wsopts.Timeout = time.Duration(obj.Get(k).ToInteger()) * time.Millisecond
		}
	}
	return wsopts, nil
}

// parseFrameIsHiddenOptions parses the frame isHidden options from a Sobek value.
//
//nolint:unparam
func parseFrameIsHiddenOptions(
	rt *sobek.Runtime, opts sobek.Value,
) (*common.FrameIsHiddenOptions, error) {
	tcopts := common.NewFrameIsHiddenOptions()
	tcopts.Strict = parseStrict(rt, opts)
	return tcopts, nil
}

// parseFrameIsVisibleOptions parses the frame isVisible options from a Sobek value.
//
//nolint:unparam
func parseFrameIsVisibleOptions(
	rt *sobek.Runtime, opts sobek.Value,
) (*common.FrameIsVisibleOptions, error) {
	tcopts := common.NewFrameIsVisibleOptions()
	tcopts.Strict = parseStrict(rt, opts)
	return tcopts, nil
}

// parseFrameBaseOptions parses the frame base options from a Sobek value into o.
//
//nolint:unparam
func parseFrameBaseOptions(
	o *common.FrameBaseOptions, rt *sobek.Runtime, opts sobek.Value,
) error {
	if k6common.IsNullish(opts) {
		return nil
	}
	obj := opts.ToObject(rt)
	for _, k := range obj.Keys() {
		switch k {
		case "strict":
			o.Strict = obj.Get(k).ToBoolean()
		case "timeout":
			o.Timeout = time.Duration(obj.Get(k).ToInteger()) * time.Millisecond
		}
	}
	return nil
}

func parseStrict(rt *sobek.Runtime, opts sobek.Value) bool {
	var strict bool

	if !k6common.IsNullish(opts) {
		obj := opts.ToObject(rt)
		for _, k := range obj.Keys() {
			if k == "strict" {
				strict = obj.Get(k).ToBoolean()
			}
		}
	}

	return strict
}
