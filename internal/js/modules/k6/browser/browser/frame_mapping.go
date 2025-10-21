package browser

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/grafana/sobek"
	"github.com/mstoykov/k6-taskqueue-lib/taskqueue"

	"go.k6.io/k6/internal/js/modules/k6/browser/common"
	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext"
	k6common "go.k6.io/k6/js/common"
)

func cancelableTaskQueue(ctx context.Context, registerCallback func() func(func() error)) *taskqueue.TaskQueue {
	tq := taskqueue.New(registerCallback)

	go func() {
		<-ctx.Done()
		tq.Close()
	}()
	return tq
}

// mapFrame to the JS module.
//
//nolint:funlen,gocognit,cyclop
func mapFrame(vu moduleVU, f *common.Frame) mapping {
	rt := vu.Runtime()
	maps := mapping{
		"check": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts, err := parseFrameCheckOptions(vu.Context(), opts)
			if err != nil {
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
			popts, err := parseFrameClickOptions(vu.Context(), opts)
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
			popts, err := parseFrameDblClickOptions(vu.Context(), opts)
			if err != nil {
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
			funcString := pageFunc.String()
			gopts := exportArgs(gargs)
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return f.Evaluate(funcString, gopts...)
			}), nil
		},
		"evaluateHandle": func(pageFunc sobek.Value, gargs ...sobek.Value) (*sobek.Promise, error) {
			if sobekEmptyString(pageFunc) {
				return nil, fmt.Errorf("evaluateHandle requires a page function")
			}
			funcString := pageFunc.String()
			gopts := exportArgs(gargs)
			return k6ext.Promise(vu.Context(), func() (any, error) {
				jsh, err := f.EvaluateHandle(funcString, gopts...)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				return mapJSHandle(vu, jsh), nil
			}), nil
		},
		"fill": func(selector, value string, opts sobek.Value) (*sobek.Promise, error) {
			popts, err := parseFrameFillOptions(vu.Context(), opts)
			if err != nil {
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
			popts, err := parseFrameHoverOptions(vu.Context(), opts)
			if err != nil {
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
			return mapLocator(vu, f.Locator(selector, parseLocatorOptions(rt, opts)))
		},
		"name": f.Name,
		"page": func() mapping {
			return mapPage(vu, f.Page())
		},
		"parentFrame": func() mapping {
			return mapFrame(vu, f.ParentFrame())
		},
		"press": func(selector, key string, opts sobek.Value) (*sobek.Promise, error) {
			popts, err := parseFramePressOptions(vu.Context(), opts)
			if err != nil {
				return nil, fmt.Errorf("parse press options of selector %q on key %q: %w", selector, key, err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, f.Press(selector, key, popts) //nolint:wrapcheck
			}), nil
		},
		"selectOption": func(selector string, values sobek.Value, opts sobek.Value) (*sobek.Promise, error) {
			popts, err := parseFrameSelectOptionOptions(vu.Context(), opts)
			if err != nil {
				return nil, fmt.Errorf("parsing select option options: %w", err)
			}
			convValues, err := common.ConvertSelectOptionValues(rt, values)
			if err != nil {
				return nil, fmt.Errorf("parsing select options values: %w", err)
			}

			return k6ext.Promise(vu.Context(), func() (any, error) {
				return f.SelectOption(selector, convValues, popts) //nolint:wrapcheck
			}), nil
		},
		"setChecked": func(selector string, checked bool, opts sobek.Value) (*sobek.Promise, error) {
			popts, err := parseFrameCheckOptions(vu.Context(), opts)
			if err != nil {
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
			popts, err := parseFrameSetInputFilesOptions(vu.Context(), opts)
			if err != nil {
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
			popts, err := parseFrameTapOptions(vu.Context(), opts)
			if err != nil {
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
			popts, err := parseFrameTypeOptions(vu.Context(), opts)
			if err != nil {
				return nil, fmt.Errorf("parsing type options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, f.Type(selector, text, popts) //nolint:wrapcheck
			}), nil
		},
		"uncheck": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts, err := parseFrameUncheckOptions(vu.Context(), opts)
			if err != nil {
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
			return waitForNavigationBodyImpl(vu, f, opts)
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
		"waitForURL": func(url sobek.Value, opts sobek.Value) (*sobek.Promise, error) {
			return waitForURLBody(vu, f, url, opts)
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

func parseStrict(ctx context.Context, opts sobek.Value) bool {
	var strict bool

	rt := k6ext.Runtime(ctx)
	if !k6common.IsNullish(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			if k == "strict" {
				strict = opts.Get(k).ToBoolean()
			}
		}
	}

	return strict
}

// parseFrameCheckOptions parses FrameCheckOptions from opts.
func parseFrameCheckOptions(ctx context.Context, opts sobek.Value) (*common.FrameCheckOptions, error) {
	basePointerOpts, err := parseElementHandleBasePointerOptions(ctx, opts)
	if err != nil {
		return nil, err
	}

	o := &common.FrameCheckOptions{
		Strict: parseStrict(ctx, opts),
	}
	if basePointerOpts != nil {
		o.ElementHandleBasePointerOptions = *basePointerOpts
	}

	return o, nil
}

// parseFrameClickOptions parses FrameClickOptions from opts.
func parseFrameClickOptions(ctx context.Context, opts sobek.Value) (*common.FrameClickOptions, error) {
	clickOpts, err := parseElementHandleClickOptions(ctx, opts)
	if err != nil {
		return nil, err
	}

	o := &common.FrameClickOptions{
		Strict: parseStrict(ctx, opts),
	}
	if clickOpts != nil {
		o.ElementHandleClickOptions = *clickOpts
	}

	return o, nil
}

// parseFrameDblClickOptions parses FrameDblClickOptions from opts.
func parseFrameDblClickOptions(ctx context.Context, opts sobek.Value) (*common.FrameDblclickOptions, error) {
	dblclickOpts, err := parseElementHandleDblclickOptions(ctx, opts)
	if err != nil {
		return nil, err
	}

	o := &common.FrameDblclickOptions{
		Strict: parseStrict(ctx, opts),
	}
	if dblclickOpts != nil {
		o.ElementHandleDblclickOptions = *dblclickOpts
	}

	return o, nil
}

// parseFrameFillOptions parses FrameFillOptions from opts.
func parseFrameFillOptions(ctx context.Context, opts sobek.Value) (*common.FrameFillOptions, error) {
	baseOpts, err := parseElementHandleBaseOptions(ctx, opts)
	if err != nil {
		return nil, err
	}

	o := &common.FrameFillOptions{
		Strict: parseStrict(ctx, opts),
	}
	if baseOpts != nil {
		o.ElementHandleBaseOptions = *baseOpts
	}

	return o, nil
}

// parseFrameHoverOptions parses FrameHoverOptions from opts.
func parseFrameHoverOptions(ctx context.Context, opts sobek.Value) (*common.FrameHoverOptions, error) {
	hoverOpts, err := parseElementHandleHoverOptions(ctx, opts)
	if err != nil {
		return nil, err
	}

	o := &common.FrameHoverOptions{
		Strict: parseStrict(ctx, opts),
	}
	if hoverOpts != nil {
		o.ElementHandleHoverOptions = *hoverOpts
	}

	return o, nil
}

// parseFrameSelectOptionOptions parses FrameSelectOptionOptions from opts.
func parseFrameSelectOptionOptions(ctx context.Context, opts sobek.Value) (*common.FrameSelectOptionOptions, error) {
	baseOpts, err := parseElementHandleBaseOptions(ctx, opts)
	if err != nil {
		return nil, err
	}

	o := &common.FrameSelectOptionOptions{
		Strict: parseStrict(ctx, opts),
	}
	if baseOpts != nil {
		o.ElementHandleBaseOptions = *baseOpts
	}

	return o, nil
}

// parseFrameSetInputFilesOptions parses FrameSetInputFilesOptions from opts.
func parseFrameSetInputFilesOptions(ctx context.Context, opts sobek.Value) (*common.FrameSetInputFilesOptions, error) {
	inputOpts, err := parseElementHandleSetInputFilesOptions(ctx, opts)
	if err != nil {
		return nil, err
	}
	if inputOpts != nil {
		return &common.FrameSetInputFilesOptions{
			ElementHandleSetInputFilesOptions: *inputOpts,
		}, nil
	}
	return nil, nil
}

// parseFrameTapOptions parses FrameTapOptions from opts.
func parseFrameTapOptions(ctx context.Context, opts sobek.Value) (*common.FrameTapOptions, error) {
	basePointerOpts, err := parseElementHandleBasePointerOptions(ctx, opts)
	if err != nil {
		return nil, err
	}

	o := &common.FrameTapOptions{}
	if basePointerOpts != nil {
		o.ElementHandleBasePointerOptions = *basePointerOpts
	}
	rt := k6ext.Runtime(ctx)
	if !k6common.IsNullish(opts) {
		obj := opts.ToObject(rt)
		for _, k := range obj.Keys() {
			switch k {
			case "modifiers":
				var m []string
				if err := rt.ExportTo(obj.Get(k), &m); err != nil {
					return nil, err
				}
				o.Modifiers = m
			case "strict":
				o.Strict = obj.Get(k).ToBoolean()
			}
		}
	}

	return o, nil
}

// parseFrameUncheckOptions parses FrameUncheckOptions from opts.
func parseFrameUncheckOptions(ctx context.Context, opts sobek.Value) (*common.FrameUncheckOptions, error) {
	basePointerOpts, err := parseElementHandleBasePointerOptions(ctx, opts)
	if err != nil {
		return nil, err
	}

	o := &common.FrameUncheckOptions{
		Strict: parseStrict(ctx, opts),
	}
	if basePointerOpts != nil {
		o.ElementHandleBasePointerOptions = *basePointerOpts
	}

	return o, nil
}

func parseFrameTypeOptions(ctx context.Context, opts sobek.Value) (*common.FrameTypeOptions, error) {
	o := &common.FrameTypeOptions{
		ElementHandleTypeOptions: common.ElementHandleTypeOptions{}, // embed base struct
		Strict:                   false,
	}

	if k6common.IsNullish(opts) {
		return o, nil
	}
	rt := k6ext.Runtime(ctx)
	obj := opts.ToObject(rt)

	for _, k := range obj.Keys() {
		switch k {
		case "delay":
			o.Delay = obj.Get(k).ToInteger()
		case "noWaitAfter":
			o.NoWaitAfter = obj.Get(k).ToBoolean()
		case "timeout":
			o.Timeout = time.Duration(obj.Get(k).ToInteger()) * time.Millisecond
		case "strict":
			o.Strict = obj.Get(k).ToBoolean()
		}
	}

	return o, nil
}

// parseFramePressOptions parses FramePressOptions from opts.
func parseFramePressOptions(ctx context.Context, opts sobek.Value) (*common.FramePressOptions, error) {
	o := &common.FramePressOptions{
		ElementHandlePressOptions: common.ElementHandlePressOptions{},
		Strict:                    false,
	}

	if k6common.IsNullish(opts) {
		return o, nil
	}

	rt := k6ext.Runtime(ctx)
	obj := opts.ToObject(rt)

	for _, k := range obj.Keys() {
		switch k {
		case "delay":
			o.Delay = obj.Get(k).ToInteger()
		case "noWaitAfter":
			o.NoWaitAfter = obj.Get(k).ToBoolean()
		case "timeout":
			o.Timeout = time.Duration(obj.Get(k).ToInteger()) * time.Millisecond
		case "strict":
			o.Strict = obj.Get(k).ToBoolean()
		}
	}

	return o, nil
}
