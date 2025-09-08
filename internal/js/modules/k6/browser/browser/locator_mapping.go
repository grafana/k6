package browser

import (
	"errors"
	"fmt"

	"github.com/grafana/sobek"

	"go.k6.io/k6/internal/js/modules/k6/browser/common"
	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext"
	k6common "go.k6.io/k6/js/common"
)

// mapLocator API to the JS module.
//
//nolint:gocognit,funlen
func mapLocator(vu moduleVU, lo *common.Locator) mapping {
	rt := vu.Runtime()
	return mapping{
		"all": func() *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
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
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return lo.BoundingBox(popts) //nolint:wrapcheck
			}), nil
		},
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
		"contentFrame": func() *sobek.Object {
			ml := mapFrameLocator(vu, lo.ContentFrame())
			return rt.ToValue(ml).ToObject(rt)
		},
		"count": func() *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return lo.Count() //nolint:wrapcheck
			})
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
		"filter": func(opts sobek.Value) mapping {
			return mapLocator(vu, lo.Filter(&common.LocatorFilterOptions{
				LocatorOptions: parseLocatorOptions(rt, opts),
			}))
		},
		"first": func() *sobek.Object {
			ml := mapLocator(vu, lo.First())
			return rt.ToValue(ml).ToObject(rt)
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
		"locator": func(selector string) *sobek.Object {
			ml := mapLocator(vu, lo.Locator(selector))
			return rt.ToValue(ml).ToObject(rt)
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
		"last": func() *sobek.Object {
			ml := mapLocator(vu, lo.Last())
			return rt.ToValue(ml).ToObject(rt)
		},
		"nth": func(nth int) *sobek.Object {
			ml := mapLocator(vu, lo.Nth(nth))
			return rt.ToValue(ml).ToObject(rt)
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
