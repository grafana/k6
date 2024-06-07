package browser

import (
	"fmt"

	"github.com/grafana/sobek"

	"github.com/grafana/xk6-browser/common"
	"github.com/grafana/xk6-browser/k6ext"
)

// syncMapFrame is like mapFrame but returns synchronous functions.
func syncMapFrame(vu moduleVU, f *common.Frame) mapping { //nolint:gocognit,cyclop,funlen
	rt := vu.Runtime()
	maps := mapping{
		"check": f.Check,
		"childFrames": func() *sobek.Object {
			var (
				mcfs []mapping
				cfs  = f.ChildFrames()
			)
			for _, fr := range cfs {
				mcfs = append(mcfs, syncMapFrame(vu, fr))
			}
			return rt.ToValue(mcfs).ToObject(rt)
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
		"content":  f.Content,
		"dblclick": f.Dblclick,
		"dispatchEvent": func(selector, typ string, eventInit, opts sobek.Value) error {
			popts := common.NewFrameDispatchEventOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return fmt.Errorf("parsing frame dispatch event options: %w", err)
			}
			return f.DispatchEvent(selector, typ, exportArg(eventInit), popts) //nolint:wrapcheck
		},
		"evaluate": func(pageFunction sobek.Value, gargs ...sobek.Value) (any, error) {
			return f.Evaluate(pageFunction.String(), exportArgs(gargs)...) //nolint:wrapcheck
		},
		"evaluateHandle": func(pageFunction sobek.Value, gargs ...sobek.Value) (mapping, error) {
			jsh, err := f.EvaluateHandle(pageFunction.String(), exportArgs(gargs)...)
			if err != nil {
				return nil, err //nolint:wrapcheck
			}
			return syncMapJSHandle(vu, jsh), nil
		},
		"fill":  f.Fill,
		"focus": f.Focus,
		"frameElement": func() (mapping, error) {
			fe, err := f.FrameElement()
			if err != nil {
				return nil, err //nolint:wrapcheck
			}
			return syncMapElementHandle(vu, fe), nil
		},
		"getAttribute": func(selector, name string, opts sobek.Value) (any, error) {
			v, ok, err := f.GetAttribute(selector, name, opts)
			if err != nil {
				return nil, err //nolint:wrapcheck
			}
			if !ok {
				return nil, nil //nolint:nilnil
			}
			return v, nil
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

				return syncMapResponse(vu, resp), nil
			}), nil
		},
		"hover":      f.Hover,
		"innerHTML":  f.InnerHTML,
		"innerText":  f.InnerText,
		"inputValue": f.InputValue,
		"isChecked":  f.IsChecked,
		"isDetached": f.IsDetached,
		"isDisabled": f.IsDisabled,
		"isEditable": f.IsEditable,
		"isEnabled":  f.IsEnabled,
		"isHidden":   f.IsHidden,
		"isVisible":  f.IsVisible,
		"locator": func(selector string, opts sobek.Value) *sobek.Object {
			ml := syncMapLocator(vu, f.Locator(selector, opts))
			return rt.ToValue(ml).ToObject(rt)
		},
		"name": f.Name,
		"page": func() *sobek.Object {
			mp := syncMapPage(vu, f.Page())
			return rt.ToValue(mp).ToObject(rt)
		},
		"parentFrame": func() *sobek.Object {
			mf := syncMapFrame(vu, f.ParentFrame())
			return rt.ToValue(mf).ToObject(rt)
		},
		"press":         f.Press,
		"selectOption":  f.SelectOption,
		"setContent":    f.SetContent,
		"setInputFiles": f.SetInputFiles,
		"tap": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameTapOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing frame tap options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, f.Tap(selector, popts) //nolint:wrapcheck
			}), nil
		},
		"textContent": func(selector string, opts sobek.Value) (any, error) {
			v, ok, err := f.TextContent(selector, opts)
			if err != nil {
				return nil, err //nolint:wrapcheck
			}
			if !ok {
				return nil, nil //nolint:nilnil
			}
			return v, nil
		},
		"title":   f.Title,
		"type":    f.Type,
		"uncheck": f.Uncheck,
		"url":     f.URL,
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
		"waitForLoadState": f.WaitForLoadState,
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
				return syncMapResponse(vu, resp), nil
			}), nil
		},
		"waitForSelector": func(selector string, opts sobek.Value) (mapping, error) {
			eh, err := f.WaitForSelector(selector, opts)
			if err != nil {
				return nil, err //nolint:wrapcheck
			}
			return syncMapElementHandle(vu, eh), nil
		},
		"waitForTimeout": f.WaitForTimeout,
	}
	maps["$"] = func(selector string) (mapping, error) {
		eh, err := f.Query(selector, common.StrictModeOff)
		if err != nil {
			return nil, err //nolint:wrapcheck
		}
		// ElementHandle can be null when the selector does not match any elements.
		// We do not want to map nil elementHandles since the expectation is a
		// null result in the test script for this case.
		if eh == nil {
			return nil, nil //nolint:nilnil
		}
		ehm := syncMapElementHandle(vu, eh)

		return ehm, nil
	}
	maps["$$"] = func(selector string) ([]mapping, error) {
		ehs, err := f.QueryAll(selector)
		if err != nil {
			return nil, err //nolint:wrapcheck
		}
		var mehs []mapping
		for _, eh := range ehs {
			ehm := syncMapElementHandle(vu, eh)
			mehs = append(mehs, ehm)
		}
		return mehs, nil
	}

	return maps
}
