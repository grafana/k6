package browser

import (
	"fmt"

	"github.com/grafana/sobek"

	"github.com/grafana/xk6-browser/common"
	"github.com/grafana/xk6-browser/k6ext"
)

// syncMapPage is like mapPage but returns synchronous functions.
func syncMapPage(vu moduleVU, p *common.Page) mapping { //nolint:gocognit,cyclop,funlen
	rt := vu.Runtime()
	maps := mapping{
		"bringToFront": p.BringToFront,
		"check":        p.Check,
		"click": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts, err := parseFrameClickOptions(vu.Context(), opts, p.Timeout())
			if err != nil {
				return nil, err
			}

			return k6ext.Promise(vu.Context(), func() (any, error) {
				err := p.Click(selector, popts)
				return nil, err //nolint:wrapcheck
			}), nil
		},
		"close": func(opts sobek.Value) error {
			vu.taskQueueRegistry.close(p.TargetID())

			return p.Close(opts) //nolint:wrapcheck
		},
		"content":  p.Content,
		"context":  p.Context,
		"dblclick": p.Dblclick,
		"dispatchEvent": func(selector, typ string, eventInit, opts sobek.Value) error {
			popts := common.NewFrameDispatchEventOptions(p.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return fmt.Errorf("parsing page dispatch event options: %w", err)
			}
			return p.DispatchEvent(selector, typ, exportArg(eventInit), popts) //nolint:wrapcheck
		},
		"emulateMedia":            p.EmulateMedia,
		"emulateVisionDeficiency": p.EmulateVisionDeficiency,
		"evaluate": func(pageFunction sobek.Value, gargs ...sobek.Value) (any, error) {
			return p.Evaluate(pageFunction.String(), exportArgs(gargs)...) //nolint:wrapcheck
		},
		"evaluateHandle": func(pageFunc sobek.Value, gargs ...sobek.Value) (mapping, error) {
			jsh, err := p.EvaluateHandle(pageFunc.String(), exportArgs(gargs)...)
			if err != nil {
				return nil, err //nolint:wrapcheck
			}
			return syncMapJSHandle(vu, jsh), nil
		},
		"fill":  p.Fill,
		"focus": p.Focus,
		"frames": func() *sobek.Object {
			var (
				mfrs []mapping
				frs  = p.Frames()
			)
			for _, fr := range frs {
				mfrs = append(mfrs, syncMapFrame(vu, fr))
			}
			return rt.ToValue(mfrs).ToObject(rt)
		},
		"getAttribute": func(selector string, name string, opts sobek.Value) (any, error) {
			v, ok, err := p.GetAttribute(selector, name, opts)
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
				p.Referrer(),
				p.NavigationTimeout(),
			)
			if err := gopts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing page navigation options to %q: %w", url, err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				resp, err := p.Goto(url, gopts)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}

				return syncMapResponse(vu, resp), nil
			}), nil
		},
		"hover":      p.Hover,
		"innerHTML":  p.InnerHTML,
		"innerText":  p.InnerText,
		"inputValue": p.InputValue,
		"isChecked":  p.IsChecked,
		"isClosed":   p.IsClosed,
		"isDisabled": p.IsDisabled,
		"isEditable": p.IsEditable,
		"isEnabled":  p.IsEnabled,
		"isHidden":   p.IsHidden,
		"isVisible":  p.IsVisible,
		"keyboard":   rt.ToValue(p.GetKeyboard()).ToObject(rt),
		"locator": func(selector string, opts sobek.Value) *sobek.Object {
			ml := syncMapLocator(vu, p.Locator(selector, opts))
			return rt.ToValue(ml).ToObject(rt)
		},
		"mainFrame": func() *sobek.Object {
			mf := syncMapFrame(vu, p.MainFrame())
			return rt.ToValue(mf).ToObject(rt)
		},
		"mouse": rt.ToValue(p.GetMouse()).ToObject(rt),
		"on": func(event string, handler sobek.Callable) error {
			tq := vu.taskQueueRegistry.get(p.TargetID())

			mapMsgAndHandleEvent := func(m *common.ConsoleMessage) error {
				mapping := syncMapConsoleMessage(vu, m)
				_, err := handler(sobek.Undefined(), vu.Runtime().ToValue(mapping))
				return err
			}
			runInTaskQueue := func(m *common.ConsoleMessage) {
				tq.Queue(func() error {
					if err := mapMsgAndHandleEvent(m); err != nil {
						return fmt.Errorf("executing page.on handler: %w", err)
					}
					return nil
				})
			}

			return p.On(event, runInTaskQueue) //nolint:wrapcheck
		},
		"opener": p.Opener,
		"press":  p.Press,
		"reload": func(opts sobek.Value) (*sobek.Object, error) {
			resp, err := p.Reload(opts)
			if err != nil {
				return nil, err //nolint:wrapcheck
			}

			r := syncMapResponse(vu, resp)

			return rt.ToValue(r).ToObject(rt), nil
		},
		"screenshot": func(opts sobek.Value) (*sobek.ArrayBuffer, error) {
			ctx := vu.Context()

			popts := common.NewPageScreenshotOptions()
			if err := popts.Parse(ctx, opts); err != nil {
				return nil, fmt.Errorf("parsing page screenshot options: %w", err)
			}

			bb, err := p.Screenshot(popts, vu.filePersister)
			if err != nil {
				return nil, err //nolint:wrapcheck
			}

			ab := rt.NewArrayBuffer(bb)

			return &ab, nil
		},
		"selectOption":                p.SelectOption,
		"setContent":                  p.SetContent,
		"setDefaultNavigationTimeout": p.SetDefaultNavigationTimeout,
		"setDefaultTimeout":           p.SetDefaultTimeout,
		"setExtraHTTPHeaders":         p.SetExtraHTTPHeaders,
		"setInputFiles":               p.SetInputFiles,
		"setViewportSize":             p.SetViewportSize,
		"tap": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameTapOptions(p.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing page tap options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, p.Tap(selector, popts) //nolint:wrapcheck
			}), nil
		},
		"textContent": func(selector string, opts sobek.Value) (any, error) {
			v, ok, err := p.TextContent(selector, opts)
			if err != nil {
				return nil, err //nolint:wrapcheck
			}
			if !ok {
				return nil, nil //nolint:nilnil
			}
			return v, nil
		},
		"throttleCPU":     p.ThrottleCPU,
		"throttleNetwork": p.ThrottleNetwork,
		"title":           p.Title,
		"touchscreen":     syncMapTouchscreen(vu, p.GetTouchscreen()),
		"type":            p.Type,
		"uncheck":         p.Uncheck,
		"url":             p.URL,
		"viewportSize":    p.ViewportSize,
		"waitForFunction": func(pageFunc, opts sobek.Value, args ...sobek.Value) (*sobek.Promise, error) {
			js, popts, pargs, err := parseWaitForFunctionArgs(
				vu.Context(), p.Timeout(), pageFunc, opts, args...,
			)
			if err != nil {
				return nil, fmt.Errorf("page waitForFunction: %w", err)
			}

			return k6ext.Promise(vu.Context(), func() (result any, reason error) {
				return p.WaitForFunction(js, popts, pargs...) //nolint:wrapcheck
			}), nil
		},
		"waitForLoadState": p.WaitForLoadState,
		"waitForNavigation": func(opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameWaitForNavigationOptions(p.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing page wait for navigation options: %w", err)
			}

			return k6ext.Promise(vu.Context(), func() (result any, reason error) {
				resp, err := p.WaitForNavigation(popts)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				return syncMapResponse(vu, resp), nil
			}), nil
		},
		"waitForSelector": func(selector string, opts sobek.Value) (mapping, error) {
			eh, err := p.WaitForSelector(selector, opts)
			if err != nil {
				return nil, err //nolint:wrapcheck
			}
			return syncMapElementHandle(vu, eh), nil
		},
		"waitForTimeout": p.WaitForTimeout,
		"workers": func() *sobek.Object {
			var mws []mapping
			for _, w := range p.Workers() {
				mw := syncMapWorker(vu, w)
				mws = append(mws, mw)
			}
			return rt.ToValue(mws).ToObject(rt)
		},
	}
	maps["$"] = func(selector string) (mapping, error) {
		eh, err := p.Query(selector)
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
		ehs, err := p.QueryAll(selector)
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
