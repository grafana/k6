package browser

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/grafana/sobek"

	"go.k6.io/k6/internal/js/modules/k6/browser/common"
	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext"
	k6common "go.k6.io/k6/js/common"
)

const (
	optionButton     = "button"
	optionClickCount = "clickCount"
	optionDelay      = "delay"
	optionModifiers  = "modifiers"
)

const noWaitAfterOption = "noWaitAfter"

var imageFormatToID = map[string]common.ImageFormat{ //nolint:gochecknoglobals
	"jpeg": common.ImageFormatJPEG,
	"png":  common.ImageFormatPNG,
}

// mapElementHandle to the JS module.
func mapElementHandle(vu moduleVU, eh *common.ElementHandle) mapping { //nolint:gocognit,funlen,cyclop
	rt := vu.Runtime()
	maps := mapping{
		"boundingBox": func() *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				box, err := eh.BoundingBox()
				// We want to avoid errors when an element is not visible and instead
				// opt to return a nil rectangle -- this matches Playwright's behaviour.
				if errors.Is(err, common.ErrElementNotVisible) {
					return nil, nil
				}
				return box, err
			})
		},
		"check": func(opts sobek.Value) (*sobek.Promise, error) {
			popts, err := parseElementHandleSetCheckedOptions(vu.Context(), opts)
			if err != nil {
				return nil, fmt.Errorf("parsing check options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, eh.Check(popts) //nolint:wrapcheck
			}), nil
		},
		"click": func(opts sobek.Value) (*sobek.Promise, error) {
			popts, err := parseElementHandleClickOptions(vu.Context(), opts)
			if err != nil {
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
			popts, err := parseElementHandleDblclickOptions(vu.Context(), opts)
			if err != nil {
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
			popts, err := parseElementHandleBaseOptions(vu.Context(), opts)
			if err != nil {
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
			popts, err := parseElementHandleHoverOptions(vu.Context(), opts)
			if err != nil {
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
			popts, err := parseElementHandleBaseOptions(vu.Context(), opts)
			if err != nil {
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
			popts, err := parseElementHandlePressOptions(vu.Context(), opts)
			if err != nil {
				return nil, fmt.Errorf("parsing press %q options: %w", key, err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, eh.Press(key, popts) //nolint:wrapcheck
			}), nil
		},
		"screenshot": func(opts sobek.Value) (*sobek.Promise, error) {
			popts, err := parseElementHandleScreenshotOptions(vu.Context(), opts)
			if err != nil {
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
			popts, err := parseElementHandleBaseOptions(vu.Context(), opts)
			if err != nil {
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
			popts, err := parseElementHandleBaseOptions(vu.Context(), opts)
			if err != nil {
				return nil, fmt.Errorf("parsing selectOption options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return eh.SelectOption(convValues, popts) //nolint:wrapcheck
			}), nil
		},
		"selectText": func(opts sobek.Value) (*sobek.Promise, error) {
			popts, err := parseElementHandleBaseOptions(vu.Context(), opts)
			if err != nil {
				return nil, fmt.Errorf("parsing selectText options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, eh.SelectText(popts) //nolint:wrapcheck
			}), nil
		},
		"setChecked": func(checked bool, opts sobek.Value) (*sobek.Promise, error) {
			popts, err := parseElementHandleSetCheckedOptions(vu.Context(), opts)
			if err != nil {
				return nil, fmt.Errorf("parsing setChecked options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, eh.SetChecked(checked, popts) //nolint:wrapcheck
			}), nil
		},
		"setInputFiles": func(files sobek.Value, opts sobek.Value) (*sobek.Promise, error) {
			popts, err := parseElementHandleSetInputFilesOptions(vu.Context(), opts)
			if err != nil {
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
			popts, err := parseElementHandleTapOptions(vu.Context(), opts)
			if err != nil {
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
			popts, err := parseElementHandleTypeOptions(vu.Context(), opts)
			if err != nil {
				return nil, fmt.Errorf("parsing type options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, eh.Type(text, popts) //nolint:wrapcheck
			}), nil
		},
		"uncheck": func(opts sobek.Value) (*sobek.Promise, error) {
			popts, err := parseElementHandleSetCheckedOptions(vu.Context(), opts)
			if err != nil {
				return nil, fmt.Errorf("parsing uncheck options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, eh.Uncheck(popts) //nolint:wrapcheck
			}), nil
		},
		"waitForElementState": func(state string, opts sobek.Value) (*sobek.Promise, error) {
			popts, err := parseElementHandleWaitForElementStateOptions(vu.Context(), opts)
			if err != nil {
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

//nolint:unparam // keeping error for consistency with other parse functions
func parseElementHandleBaseOptions(ctx context.Context, opts sobek.Value) (*common.ElementHandleBaseOptions, error) {
	if k6common.IsNullish(opts) {
		//nolint:nilnil // returning (nil, nil) intentionally means "no options provided"
		return nil, nil
	}

	parsed := &common.ElementHandleBaseOptions{
		Force:       false,
		NoWaitAfter: false,
		Timeout:     0,
	}
	rt := k6ext.Runtime(ctx)
	obj := opts.ToObject(rt)
	for _, k := range obj.Keys() {
		switch k {
		case "force":
			parsed.Force = obj.Get(k).ToBoolean()
		case noWaitAfterOption:
			parsed.NoWaitAfter = obj.Get(k).ToBoolean()
		case "timeout":
			parsed.Timeout = time.Duration(obj.Get(k).ToInteger()) * time.Millisecond
		}
	}

	return parsed, nil
}

func parseElementHandleBasePointerOptions(
	ctx context.Context,
	opts sobek.Value,
) (*common.ElementHandleBasePointerOptions, error) {
	o := &common.ElementHandleBasePointerOptions{}

	baseOpts, err := parseElementHandleBaseOptions(ctx, opts)
	if err != nil {
		return nil, err
	}
	if baseOpts != nil {
		o.ElementHandleBaseOptions = *baseOpts
	}

	rt := k6ext.Runtime(ctx)
	if !k6common.IsNullish(opts) {
		optsObj := opts.ToObject(rt)
		for _, k := range optsObj.Keys() {
			switch k {
			case "position":
				var p map[string]float64
				o.Position = &common.Position{}
				if rt.ExportTo(optsObj.Get(k), &p) == nil {
					o.Position.X = p["x"]
					o.Position.Y = p["y"]
				}
			case "trial":
				o.Trial = optsObj.Get(k).ToBoolean()
			}
		}
	}

	return o, nil
}

func parseElementHandleSetInputFilesOptions(
	ctx context.Context,
	opts sobek.Value,
) (*common.ElementHandleSetInputFilesOptions, error) {
	baseOpts, err := parseElementHandleBaseOptions(ctx, opts)
	if err != nil {
		return nil, err
	}
	if baseOpts == nil {
		//nolint:nilnil // returning (nil, nil) intentionally means "no options provided"
		return nil, nil
	}

	o := &common.ElementHandleSetInputFilesOptions{
		ElementHandleBaseOptions: *baseOpts,
	}

	return o, nil
}

func parseElementHandleClickOptions(ctx context.Context, opts sobek.Value) (*common.ElementHandleClickOptions, error) {
	if k6common.IsNullish(opts) {
		//nolint:nilnil // returning (nil, nil) intentionally means "no options provided"
		return nil, nil
	}
	basePointer, err := parseElementHandleBasePointerOptions(ctx, opts)
	if err != nil {
		return nil, err
	}

	o := &common.ElementHandleClickOptions{}
	if basePointer != nil {
		o.ElementHandleBasePointerOptions = *basePointer
	}
	rt := k6ext.Runtime(ctx)
	optsObj := opts.ToObject(rt)
	for _, k := range optsObj.Keys() {
		switch k {
		case optionButton:
			o.Button = optsObj.Get(k).String()
		case optionClickCount:
			o.ClickCount = optsObj.Get(k).ToInteger()
		case optionDelay:
			o.Delay = optsObj.Get(k).ToInteger()
		case optionModifiers:
			var m []string
			if err := rt.ExportTo(optsObj.Get(k), &m); err == nil {
				o.Modifiers = m
			}
		}
	}

	return o, nil
}

func parseElementHandleDblclickOptions(
	ctx context.Context,
	opts sobek.Value,
) (*common.ElementHandleDblclickOptions, error) {
	o := &common.ElementHandleDblclickOptions{}

	// Parse base pointer options first, propagating error if any
	basePointer, err := parseElementHandleBasePointerOptions(ctx, opts)
	if err != nil {
		return nil, err
	}
	if basePointer != nil {
		o.ElementHandleBasePointerOptions = *basePointer
	}
	rt := k6ext.Runtime(ctx)
	if !k6common.IsNullish(opts) {
		optsObj := opts.ToObject(rt)
		for _, k := range optsObj.Keys() {
			switch k {
			case "button":
				o.Button = optsObj.Get(k).String()
			case "delay":
				o.Delay = optsObj.Get(k).ToInteger()
			case "modifiers":
				var m []string
				if err := rt.ExportTo(optsObj.Get(k), &m); err != nil {
					return nil, err
				}
				o.Modifiers = m
			}
		}
	}

	return o, nil
}

func parseElementHandleHoverOptions(ctx context.Context, opts sobek.Value) (*common.ElementHandleHoverOptions, error) {
	o := &common.ElementHandleHoverOptions{}

	basePointer, err := parseElementHandleBasePointerOptions(ctx, opts)
	if err != nil {
		return nil, err
	}
	if basePointer != nil {
		o.ElementHandleBasePointerOptions = *basePointer
	}
	rt := k6ext.Runtime(ctx)
	if !k6common.IsNullish(opts) {
		optsObj := opts.ToObject(rt)
		for _, k := range optsObj.Keys() {
			if k == "modifiers" {
				var m []string
				if err := rt.ExportTo(optsObj.Get(k), &m); err != nil {
					return nil, err
				}
				o.Modifiers = m
			}
		}
	}

	return o, nil
}

func parseElementHandleSetCheckedOptions(
	ctx context.Context,
	opts sobek.Value,
) (*common.ElementHandleSetCheckedOptions, error) {
	o := &common.ElementHandleSetCheckedOptions{}

	basePointerOpts, err := parseElementHandleBasePointerOptions(ctx, opts)
	if err != nil {
		return nil, err
	}
	if basePointerOpts != nil {
		o.ElementHandleBasePointerOptions = *basePointerOpts
	}
	rt := k6ext.Runtime(ctx)
	if !k6common.IsNullish(opts) {
		optsObj := opts.ToObject(rt)
		for _, k := range optsObj.Keys() {
			if k == "strict" {
				o.Strict = optsObj.Get(k).ToBoolean()
			}
		}
	}

	return o, nil
}

func parseElementHandleTapOptions(ctx context.Context, opts sobek.Value) (*common.ElementHandleTapOptions, error) {
	o := &common.ElementHandleTapOptions{}

	basePointerOpts, err := parseElementHandleBasePointerOptions(ctx, opts)
	if err != nil {
		return nil, err
	}
	if basePointerOpts != nil {
		o.ElementHandleBasePointerOptions = *basePointerOpts
	}

	rt := k6ext.Runtime(ctx)
	if !k6common.IsNullish(opts) {
		optsObj := opts.ToObject(rt)
		for _, k := range optsObj.Keys() {
			if k == "modifiers" {
				var m []string
				if err := rt.ExportTo(optsObj.Get(k), &m); err != nil {
					return nil, err
				}
				o.Modifiers = m
			}
		}
	}

	return o, nil
}

//nolint:unparam // keeping error for consistency with other parse functions
func parseElementHandlePressOptions(ctx context.Context, opts sobek.Value) (*common.ElementHandlePressOptions, error) {
	o := &common.ElementHandlePressOptions{}
	rt := k6ext.Runtime(ctx)
	if !k6common.IsNullish(opts) {
		obj := opts.ToObject(rt)
		for _, k := range obj.Keys() {
			switch k {
			case "delay":
				o.Delay = obj.Get(k).ToInteger()
			case noWaitAfterOption:
				o.NoWaitAfter = obj.Get(k).ToBoolean()
			case "timeout":
				o.Timeout = time.Duration(obj.Get(k).ToInteger()) * time.Millisecond
			}
		}
	}

	return o, nil
}

//nolint:unparam // keeping error for consistency with other parse functions
func parseElementHandleScreenshotOptions(
	ctx context.Context,
	opts sobek.Value,
) (*common.ElementHandleScreenshotOptions, error) {
	o := &common.ElementHandleScreenshotOptions{}

	if k6common.IsNullish(opts) {
		return o, nil
	}
	rt := k6ext.Runtime(ctx)
	obj := opts.ToObject(rt)
	formatSpecified := false
	for _, k := range obj.Keys() {
		switch k {
		case "omitBackground":
			o.OmitBackground = obj.Get(k).ToBoolean()
		case "path":
			o.Path = obj.Get(k).String()
		case "quality":
			o.Quality = obj.Get(k).ToInteger()
		case "type":
			if f, ok := imageFormatToID[obj.Get(k).String()]; ok {
				o.Format = f
				formatSpecified = true
			}
		case "timeout":
			o.Timeout = time.Duration(obj.Get(k).ToInteger()) * time.Millisecond
		}
	}

	// Infer file format by path if not specified explicitly (default PNG)
	if o.Path != "" && !formatSpecified {
		if strings.HasSuffix(o.Path, ".jpg") || strings.HasSuffix(o.Path, ".jpeg") {
			o.Format = common.ImageFormatJPEG
		}
	}

	return o, nil
}

//nolint:unparam // keeping error for consistency with other parse functions
func parseElementHandleTypeOptions(ctx context.Context, opts sobek.Value) (*common.ElementHandleTypeOptions, error) {
	o := &common.ElementHandleTypeOptions{}
	rt := k6ext.Runtime(ctx)
	if !k6common.IsNullish(opts) {
		obj := opts.ToObject(rt)
		for _, k := range obj.Keys() {
			switch k {
			case "delay":
				o.Delay = obj.Get(k).ToInteger()
			case noWaitAfterOption:
				o.NoWaitAfter = obj.Get(k).ToBoolean()
			case "timeout":
				o.Timeout = time.Duration(obj.Get(k).ToInteger()) * time.Millisecond
			}
		}
	}

	return o, nil
}

//nolint:unparam // keeping error for consistency with other parse functions
func parseElementHandleWaitForElementStateOptions(
	ctx context.Context,
	opts sobek.Value,
) (*common.ElementHandleWaitForElementStateOptions, error) {
	o := &common.ElementHandleWaitForElementStateOptions{}
	rt := k6ext.Runtime(ctx)
	if !k6common.IsNullish(opts) {
		obj := opts.ToObject(rt)
		for _, k := range obj.Keys() {
			if k == "timeout" {
				o.Timeout = time.Duration(obj.Get(k).ToInteger()) * time.Millisecond
			}
		}
	}

	return o, nil
}
