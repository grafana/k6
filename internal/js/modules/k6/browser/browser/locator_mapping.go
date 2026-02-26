package browser

import (
	"errors"
	"fmt"

	"github.com/grafana/sobek"

	"go.k6.io/k6/internal/js/modules/k6/browser/common"
	k6common "go.k6.io/k6/js/common"
)

// mapLocator API to the JS module.
//
//nolint:gocognit,funlen,cyclop
func mapLocator(vu moduleVU, lo *common.Locator) mapping {
	rt := vu.Runtime()
	return mapping{
		"all": func() *sobek.Promise {
			return promise(vu, func() (any, error) {
				all, err := lo.All()
				if err != nil {
					return nil, err
				}

				res := make([]mapping, len(all))
				for i, el := range all {
					res[i] = mapLocator(vu, el)
				}
				return res, nil
			})
		},
		"boundingBox": func(opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameBaseOptions(lo.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing locator bounding box options: %w", err)
			}
			return promise(vu, func() (any, error) {
				box, err := lo.BoundingBox(popts)
				// We want to avoid errors when an element is not visible and instead
				// opt to return a nil rectangle -- this matches Playwright's behaviour.
				if errors.Is(err, common.ErrElementNotVisible) {
					return nil, nil
				}
				return box, err
			}), nil
		},
		"clear": func(opts sobek.Value) (*sobek.Promise, error) {
			copts := common.NewFrameFillOptions(lo.Timeout())
			if err := copts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing clear options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return nil, lo.Clear(copts) //nolint:wrapcheck
			}), nil
		},
		"click": func(opts sobek.Value) (*sobek.Promise, error) {
			popts, err := parseFrameClickOptions(vu.Context(), opts, lo.Timeout())
			if err != nil {
				return nil, err
			}

			return promise(vu, func() (any, error) {
				return nil, lo.Click(popts) //nolint:wrapcheck
			}), nil
		},
		"contentFrame": func() *sobek.Object {
			ml := mapFrameLocator(vu, lo.ContentFrame())
			return rt.ToValue(ml).ToObject(rt)
		},
		"count": func() *sobek.Promise {
			return promise(vu, func() (any, error) {
				return lo.Count() //nolint:wrapcheck
			})
		},
		"dblclick": func(opts sobek.Value) (*sobek.Promise, error) {
			copts := common.NewFrameDblClickOptions(lo.Timeout())
			if err := copts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing double click options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return nil, lo.Dblclick(copts) //nolint:wrapcheck
			}), nil
		},
		"evaluate": func(pageFunc sobek.Value, gargs ...sobek.Value) (*sobek.Promise, error) {
			if sobekEmptyString(pageFunc) {
				return nil, fmt.Errorf("evaluate requires a page function")
			}
			funcString := pageFunc.String()
			gopts := exportArgs(gargs)
			return promise(vu, func() (any, error) {
				return lo.Evaluate(funcString, gopts...)
			}), nil
		},
		"evaluateHandle": func(pageFunc sobek.Value, gargs ...sobek.Value) (*sobek.Promise, error) {
			if sobekEmptyString(pageFunc) {
				return nil, fmt.Errorf("evaluateHandle requires a page function")
			}
			funcString := pageFunc.String()
			gopts := exportArgs(gargs)
			return promise(vu, func() (any, error) {
				jsh, err := lo.EvaluateHandle(funcString, gopts...)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				return mapJSHandle(vu, jsh), nil
			}), nil
		},
		"setChecked": func(checked bool, opts sobek.Value) (*sobek.Promise, error) {
			copts := common.NewFrameCheckOptions(lo.Timeout())
			if err := copts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing set checked options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return nil, lo.SetChecked(checked, copts) //nolint:wrapcheck
			}), nil
		},
		"check": func(opts sobek.Value) (*sobek.Promise, error) {
			copts := common.NewFrameCheckOptions(lo.Timeout())
			if err := copts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing check options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return nil, lo.Check(copts) //nolint:wrapcheck
			}), nil
		},
		"uncheck": func(opts sobek.Value) (*sobek.Promise, error) {
			copts := common.NewFrameUncheckOptions(lo.Timeout())
			if err := copts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing uncheck options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return nil, lo.Uncheck(copts) //nolint:wrapcheck
			}), nil
		},
		"isChecked": func(opts sobek.Value) (*sobek.Promise, error) {
			copts := common.NewFrameIsCheckedOptions(lo.Timeout())
			if err := copts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing is checked options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return lo.IsChecked(copts) //nolint:wrapcheck
			}), nil
		},
		"isEditable": func(opts sobek.Value) (*sobek.Promise, error) {
			copts := common.NewFrameIsEditableOptions(lo.Timeout())
			if err := copts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing is editable options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return lo.IsEditable(copts) //nolint:wrapcheck
			}), nil
		},
		"isEnabled": func(opts sobek.Value) (*sobek.Promise, error) {
			copts := common.NewFrameIsEnabledOptions(lo.Timeout())
			if err := copts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing is enabled options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return lo.IsEnabled(copts) //nolint:wrapcheck
			}), nil
		},
		"isDisabled": func(opts sobek.Value) (*sobek.Promise, error) {
			copts := common.NewFrameIsDisabledOptions(lo.Timeout())
			if err := copts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing is disabled options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return lo.IsDisabled(copts) //nolint:wrapcheck
			}), nil
		},
		"isVisible": func() *sobek.Promise {
			return promise(vu, func() (any, error) {
				return lo.IsVisible() //nolint:wrapcheck
			})
		},
		"isHidden": func() *sobek.Promise {
			return promise(vu, func() (any, error) {
				return lo.IsHidden() //nolint:wrapcheck
			})
		},
		"fill": func(value string, opts sobek.Value) (*sobek.Promise, error) {
			copts := common.NewFrameFillOptions(lo.Timeout())
			if err := copts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing fill options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return nil, lo.Fill(value, copts) //nolint:wrapcheck
			}), nil
		},
		"filter": func(opts sobek.Value) mapping {
			return mapLocator(vu, lo.Filter(&common.LocatorFilterOptions{
				LocatorOptions: parseLocatorOptions(rt, opts),
			}))
		},
		"first": func() *sobek.Object {
			ml := mapLocator(vu, lo.First())
			return rt.ToValue(ml).ToObject(rt)
		},
		"focus": func(opts sobek.Value) (*sobek.Promise, error) {
			copts := common.NewFrameBaseOptions(lo.Timeout())
			if err := copts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing focus options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return nil, lo.Focus(copts) //nolint:wrapcheck
			}), nil
		},
		"getAttribute": func(name string, opts sobek.Value) (*sobek.Promise, error) {
			copts := common.NewFrameBaseOptions(lo.Timeout())
			if err := copts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing get attribute options: %w", err)
			}
			return promise(vu, func() (any, error) {
				s, ok, err := lo.GetAttribute(name, copts)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				if !ok {
					return nil, nil
				}
				return s, nil
			}), nil
		},
		"getByAltText": func(alt sobek.Value, opts sobek.Value) (*sobek.Object, error) {
			if k6common.IsNullish(alt) {
				return nil, errors.New("missing required argument 'altText'")
			}
			palt, popts := parseGetByBaseOptions(vu.Context(), alt, false, opts)

			ml := mapLocator(vu, lo.GetByAltText(palt, popts))
			return rt.ToValue(ml).ToObject(rt), nil
		},
		"getByLabel": func(label sobek.Value, opts sobek.Value) (*sobek.Object, error) {
			if k6common.IsNullish(label) {
				return nil, errors.New("missing required argument 'label'")
			}
			plabel, popts := parseGetByBaseOptions(vu.Context(), label, true, opts)

			ml := mapLocator(vu, lo.GetByLabel(plabel, popts))
			return rt.ToValue(ml).ToObject(rt), nil
		},
		"getByPlaceholder": func(placeholder sobek.Value, opts sobek.Value) (*sobek.Object, error) {
			if k6common.IsNullish(placeholder) {
				return nil, errors.New("missing required argument 'placeholder'")
			}
			pplaceholder, popts := parseGetByBaseOptions(vu.Context(), placeholder, false, opts)

			ml := mapLocator(vu, lo.GetByPlaceholder(pplaceholder, popts))
			return rt.ToValue(ml).ToObject(rt), nil
		},
		"getByRole": func(role sobek.Value, opts sobek.Value) (*sobek.Object, error) {
			if k6common.IsNullish(role) {
				return nil, errors.New("missing required argument 'role'")
			}
			popts := parseGetByRoleOptions(vu.Context(), opts)

			ml := mapLocator(vu, lo.GetByRole(role.String(), popts))
			return rt.ToValue(ml).ToObject(rt), nil
		},
		"getByTestId": func(testID sobek.Value) (*sobek.Object, error) {
			if k6common.IsNullish(testID) {
				return nil, errors.New("missing required argument 'testId'")
			}
			ptestID := parseStringOrRegex(testID, false)

			ml := mapLocator(vu, lo.GetByTestID(ptestID))
			return rt.ToValue(ml).ToObject(rt), nil
		},
		"getByText": func(text sobek.Value, opts sobek.Value) (*sobek.Object, error) {
			if k6common.IsNullish(text) {
				return nil, errors.New("missing required argument 'text'")
			}
			ptext, popts := parseGetByBaseOptions(vu.Context(), text, true, opts)

			ml := mapLocator(vu, lo.GetByText(ptext, popts))
			return rt.ToValue(ml).ToObject(rt), nil
		},
		"getByTitle": func(title sobek.Value, opts sobek.Value) (*sobek.Object, error) {
			if k6common.IsNullish(title) {
				return nil, errors.New("missing required argument 'title'")
			}
			ptitle, popts := parseGetByBaseOptions(vu.Context(), title, false, opts)

			ml := mapLocator(vu, lo.GetByTitle(ptitle, popts))
			return rt.ToValue(ml).ToObject(rt), nil
		},
		"locator": func(selector string, opts sobek.Value) mapping {
			return mapLocator(vu, lo.Locator(selector, parseLocatorOptions(rt, opts)))
		},
		"frameLocator": func(selector string) *sobek.Object {
			mfl := mapFrameLocator(vu, lo.FrameLocator(selector))
			return rt.ToValue(mfl).ToObject(rt)
		},
		"innerHTML": func(opts sobek.Value) (*sobek.Promise, error) {
			copts := common.NewFrameInnerHTMLOptions(lo.Timeout())
			if err := copts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing inner HTML options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return lo.InnerHTML(copts) //nolint:wrapcheck
			}), nil
		},
		"innerText": func(opts sobek.Value) (*sobek.Promise, error) {
			copts := common.NewFrameInnerTextOptions(lo.Timeout())
			if err := copts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing inner text options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return lo.InnerText(copts) //nolint:wrapcheck
			}), nil
		},
		"last": func() *sobek.Object {
			ml := mapLocator(vu, lo.Last())
			return rt.ToValue(ml).ToObject(rt)
		},
		"nth": func(nth int) *sobek.Object {
			ml := mapLocator(vu, lo.Nth(nth))
			return rt.ToValue(ml).ToObject(rt)
		},
		"textContent": func(opts sobek.Value) (*sobek.Promise, error) {
			copts := common.NewFrameTextContentOptions(lo.Timeout())
			if err := copts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing text content options: %w", err)
			}
			return promise(vu, func() (any, error) {
				s, ok, err := lo.TextContent(copts)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				if !ok {
					return nil, nil
				}
				return s, nil
			}), nil
		},
		"inputValue": func(opts sobek.Value) (*sobek.Promise, error) {
			copts := common.NewFrameInputValueOptions(lo.Timeout())
			if err := copts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing input value options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return lo.InputValue(copts) //nolint:wrapcheck
			}), nil
		},
		"selectOption": func(values sobek.Value, opts sobek.Value) (*sobek.Promise, error) {
			copts := common.NewFrameSelectOptionOptions(lo.Timeout())
			if err := copts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing select option options: %w", err)
			}
			convValues, err := ConvertSelectOptionValues(vu.Runtime(), values)
			if err != nil {
				return nil, fmt.Errorf("parsing select option values: %w", err)
			}
			return promise(vu, func() (any, error) {
				return lo.SelectOption(convValues, copts) //nolint:wrapcheck
			}), nil
		},
		"press": func(key string, opts sobek.Value) (*sobek.Promise, error) {
			copts := common.NewFramePressOptions(lo.Timeout())
			if err := copts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing press options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return nil, lo.Press(key, copts) //nolint:wrapcheck
			}), nil
		},

		"pressSequentially": func(text string, opts sobek.Value) (*sobek.Promise, error) {
			copts := common.NewFrameTypeOptions(lo.Timeout())
			if err := copts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing locator press sequentially options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return nil, lo.PressSequentially(text, copts) //nolint:wrapcheck
			}), nil
		},

		"type": func(text string, opts sobek.Value) (*sobek.Promise, error) {
			copts := common.NewFrameTypeOptions(lo.Timeout())
			if err := copts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing type options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return nil, lo.Type(text, copts) //nolint:wrapcheck
			}), nil
		},
		"hover": func(opts sobek.Value) (*sobek.Promise, error) {
			copts := common.NewFrameHoverOptions(lo.Timeout())
			if err := copts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing hover options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return nil, lo.Hover(copts) //nolint:wrapcheck
			}), nil
		},
		"tap": func(opts sobek.Value) (*sobek.Promise, error) {
			copts := common.NewFrameTapOptions(lo.DefaultTimeout())
			if err := copts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing locator tap options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return nil, lo.Tap(copts) //nolint:wrapcheck
			}), nil
		},
		"dispatchEvent": func(typ string, eventInit, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameDispatchEventOptions(lo.DefaultTimeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing locator dispatch event options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return nil, lo.DispatchEvent(typ, exportArg(eventInit), popts) //nolint:wrapcheck
			}), nil
		},
		"waitFor": func(opts sobek.Value) (*sobek.Promise, error) {
			copts := common.NewFrameWaitForSelectorOptions(lo.Timeout())
			if err := copts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing wait for options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return nil, lo.WaitFor(copts) //nolint:wrapcheck
			}), nil
		},
	}
}

func parseLocatorOptions(rt *sobek.Runtime, opts sobek.Value) *common.LocatorOptions {
	if k6common.IsNullish(opts) {
		return nil
	}

	var popts common.LocatorOptions

	obj := opts.ToObject(rt)
	for _, k := range obj.Keys() {
		switch k {
		case "hasText":
			popts.HasText = obj.Get(k).String()
		case "hasNotText":
			popts.HasNotText = obj.Get(k).String()
		}
	}

	return &popts
}
