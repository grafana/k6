package browser

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/dop251/goja"

	"github.com/grafana/xk6-browser/common"
	"github.com/grafana/xk6-browser/k6error"
	"github.com/grafana/xk6-browser/k6ext"

	k6common "go.k6.io/k6/js/common"
)

// mapping is a type for mapping our module API to Goja.
// It acts like a bridge and allows adding wildcard methods
// and customization over our API.
type mapping = map[string]any

// mapBrowserToGoja maps the browser API to the JS module.
// The motivation of this mapping was to support $ and $$ wildcard
// methods.
// See issue #661 for more details.
func mapBrowserToGoja(vu moduleVU) *goja.Object {
	var (
		rt  = vu.Runtime()
		obj = rt.NewObject()
	)
	for k, v := range mapBrowser(vu) {
		err := obj.Set(k, rt.ToValue(v))
		if err != nil {
			k6common.Throw(rt, fmt.Errorf("mapping: %w", err))
		}
	}

	return obj
}

func parseFrameClickOptions(
	ctx context.Context, opts goja.Value, defaultTimeout time.Duration,
) (*common.FrameClickOptions, error) {
	copts := common.NewFrameClickOptions(defaultTimeout)
	if err := copts.Parse(ctx, opts); err != nil {
		return nil, fmt.Errorf("parsing click options: %w", err)
	}
	return copts, nil
}

// mapFrame to the JS module.
//
//nolint:funlen
func mapFrame(vu moduleVU, f *common.Frame) mapping {
	rt := vu.Runtime()
	maps := mapping{
		"check": f.Check,
		"childFrames": func() *goja.Object {
			var (
				mcfs []mapping
				cfs  = f.ChildFrames()
			)
			for _, fr := range cfs {
				mcfs = append(mcfs, mapFrame(vu, fr))
			}
			return rt.ToValue(mcfs).ToObject(rt)
		},
		"click": func(selector string, opts goja.Value) (*goja.Promise, error) {
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
		"dispatchEvent": func(selector, typ string, eventInit, opts goja.Value) error {
			popts := common.NewFrameDispatchEventOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return fmt.Errorf("parsing frame dispatch event options: %w", err)
			}
			return f.DispatchEvent(selector, typ, exportArg(eventInit), popts) //nolint:wrapcheck
		},
		"evaluate": func(pageFunction goja.Value, gargs ...goja.Value) any {
			return f.Evaluate(pageFunction.String(), exportArgs(gargs)...)
		},
		"evaluateHandle": func(pageFunction goja.Value, gargs ...goja.Value) (mapping, error) {
			jsh, err := f.EvaluateHandle(pageFunction.String(), exportArgs(gargs)...)
			if err != nil {
				return nil, err //nolint:wrapcheck
			}
			return mapJSHandle(vu, jsh), nil
		},
		"fill":  f.Fill,
		"focus": f.Focus,
		"frameElement": func() (mapping, error) {
			fe, err := f.FrameElement()
			if err != nil {
				return nil, err //nolint:wrapcheck
			}
			return mapElementHandle(vu, fe), nil
		},
		"getAttribute": f.GetAttribute,
		"goto": func(url string, opts goja.Value) (*goja.Promise, error) {
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
		"locator": func(selector string, opts goja.Value) *goja.Object {
			ml := mapLocator(vu, f.Locator(selector, opts))
			return rt.ToValue(ml).ToObject(rt)
		},
		"name": f.Name,
		"page": func() *goja.Object {
			mp := mapPage(vu, f.Page())
			return rt.ToValue(mp).ToObject(rt)
		},
		"parentFrame": func() *goja.Object {
			mf := mapFrame(vu, f.ParentFrame())
			return rt.ToValue(mf).ToObject(rt)
		},
		"press":         f.Press,
		"selectOption":  f.SelectOption,
		"setContent":    f.SetContent,
		"setInputFiles": f.SetInputFiles,
		"tap": func(selector string, opts goja.Value) (*goja.Promise, error) {
			popts := common.NewFrameTapOptions(f.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing frame tap options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, f.Tap(selector, popts) //nolint:wrapcheck
			}), nil
		},
		"textContent": f.TextContent,
		"title":       f.Title,
		"type":        f.Type,
		"uncheck":     f.Uncheck,
		"url":         f.URL,
		"waitForFunction": func(pageFunc, opts goja.Value, args ...goja.Value) (*goja.Promise, error) {
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
		"waitForNavigation": func(opts goja.Value) (*goja.Promise, error) {
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
		"waitForSelector": func(selector string, opts goja.Value) (mapping, error) {
			eh, err := f.WaitForSelector(selector, opts)
			if err != nil {
				return nil, err //nolint:wrapcheck
			}
			return mapElementHandle(vu, eh), nil
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
		ehm := mapElementHandle(vu, eh)
		return ehm, nil
	}
	maps["$$"] = func(selector string) ([]mapping, error) {
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
	}

	return maps
}

func parseWaitForFunctionArgs(
	ctx context.Context, timeout time.Duration, pageFunc, opts goja.Value, gargs ...goja.Value,
) (string, *common.FrameWaitForFunctionOptions, []any, error) {
	popts := common.NewFrameWaitForFunctionOptions(timeout)
	err := popts.Parse(ctx, opts)
	if err != nil {
		return "", nil, nil, fmt.Errorf("parsing waitForFunction options: %w", err)
	}

	js := pageFunc.ToString().String()
	_, isCallable := goja.AssertFunction(pageFunc)
	if !isCallable {
		js = fmt.Sprintf("() => (%s)", js)
	}

	return js, popts, exportArgs(gargs), nil
}

// mapPage to the JS module.
//
//nolint:funlen
func mapPage(vu moduleVU, p *common.Page) mapping {
	rt := vu.Runtime()
	maps := mapping{
		"bringToFront": p.BringToFront,
		"check":        p.Check,
		"click": func(selector string, opts goja.Value) (*goja.Promise, error) {
			popts, err := parseFrameClickOptions(vu.Context(), opts, p.Timeout())
			if err != nil {
				return nil, err
			}

			return k6ext.Promise(vu.Context(), func() (any, error) {
				err := p.Click(selector, popts)
				return nil, err //nolint:wrapcheck
			}), nil
		},
		"close": func(opts goja.Value) error {
			vu.taskQueueRegistry.close(p.TargetID())

			return p.Close(opts) //nolint:wrapcheck
		},
		"content":  p.Content,
		"context":  p.Context,
		"dblclick": p.Dblclick,
		"dispatchEvent": func(selector, typ string, eventInit, opts goja.Value) error {
			popts := common.NewFrameDispatchEventOptions(p.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return fmt.Errorf("parsing page dispatch event options: %w", err)
			}
			return p.DispatchEvent(selector, typ, exportArg(eventInit), popts) //nolint:wrapcheck
		},
		"emulateMedia":            p.EmulateMedia,
		"emulateVisionDeficiency": p.EmulateVisionDeficiency,
		"evaluate": func(pageFunction goja.Value, gargs ...goja.Value) any {
			return p.Evaluate(pageFunction.String(), exportArgs(gargs)...)
		},
		"evaluateHandle": func(pageFunc goja.Value, gargs ...goja.Value) (mapping, error) {
			jsh, err := p.EvaluateHandle(pageFunc.String(), exportArgs(gargs)...)
			if err != nil {
				return nil, err //nolint:wrapcheck
			}
			return mapJSHandle(vu, jsh), nil
		},
		"fill":  p.Fill,
		"focus": p.Focus,
		"frames": func() *goja.Object {
			var (
				mfrs []mapping
				frs  = p.Frames()
			)
			for _, fr := range frs {
				mfrs = append(mfrs, mapFrame(vu, fr))
			}
			return rt.ToValue(mfrs).ToObject(rt)
		},
		"getAttribute": p.GetAttribute,
		"goto": func(url string, opts goja.Value) (*goja.Promise, error) {
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

				return mapResponse(vu, resp), nil
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
		"locator": func(selector string, opts goja.Value) *goja.Object {
			ml := mapLocator(vu, p.Locator(selector, opts))
			return rt.ToValue(ml).ToObject(rt)
		},
		"mainFrame": func() *goja.Object {
			mf := mapFrame(vu, p.MainFrame())
			return rt.ToValue(mf).ToObject(rt)
		},
		"mouse": rt.ToValue(p.GetMouse()).ToObject(rt),
		"on": func(event string, handler goja.Callable) error {
			tq := vu.taskQueueRegistry.get(p.TargetID())

			mapMsgAndHandleEvent := func(m *common.ConsoleMessage) error {
				mapping := mapConsoleMessage(vu, m)
				_, err := handler(goja.Undefined(), vu.Runtime().ToValue(mapping))
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
		"reload": func(opts goja.Value) (*goja.Object, error) {
			resp, err := p.Reload(opts)
			if err != nil {
				return nil, err //nolint:wrapcheck
			}

			r := mapResponse(vu, resp)

			return rt.ToValue(r).ToObject(rt), nil
		},
		"screenshot": func(opts goja.Value) (*goja.ArrayBuffer, error) {
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
		"tap": func(selector string, opts goja.Value) (*goja.Promise, error) {
			popts := common.NewFrameTapOptions(p.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing page tap options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, p.Tap(selector, popts) //nolint:wrapcheck
			}), nil
		},
		"textContent":     p.TextContent,
		"throttleCPU":     p.ThrottleCPU,
		"throttleNetwork": p.ThrottleNetwork,
		"title":           p.Title,
		"touchscreen":     mapTouchscreen(vu, p.GetTouchscreen()),
		"type":            p.Type,
		"uncheck":         p.Uncheck,
		"url":             p.URL,
		"viewportSize":    p.ViewportSize,
		"waitForFunction": func(pageFunc, opts goja.Value, args ...goja.Value) (*goja.Promise, error) {
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
		"waitForNavigation": func(opts goja.Value) (*goja.Promise, error) {
			popts := common.NewFrameWaitForNavigationOptions(p.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing page wait for navigation options: %w", err)
			}

			return k6ext.Promise(vu.Context(), func() (result any, reason error) {
				resp, err := p.WaitForNavigation(popts)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				return mapResponse(vu, resp), nil
			}), nil
		},
		"waitForSelector": func(selector string, opts goja.Value) (mapping, error) {
			eh, err := p.WaitForSelector(selector, opts)
			if err != nil {
				return nil, err //nolint:wrapcheck
			}
			return mapElementHandle(vu, eh), nil
		},
		"waitForTimeout": p.WaitForTimeout,
		"workers": func() *goja.Object {
			var mws []mapping
			for _, w := range p.Workers() {
				mw := mapWorker(vu, w)
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
		ehm := mapElementHandle(vu, eh)
		return ehm, nil
	}
	maps["$$"] = func(selector string) ([]mapping, error) {
		ehs, err := p.QueryAll(selector)
		if err != nil {
			return nil, err //nolint:wrapcheck
		}
		var mehs []mapping
		for _, eh := range ehs {
			ehm := mapElementHandle(vu, eh)
			mehs = append(mehs, ehm)
		}
		return mehs, nil
	}

	return maps
}

// mapTouchscreen to the JS module.
func mapTouchscreen(vu moduleVU, ts *common.Touchscreen) mapping {
	return mapping{
		"tap": func(x float64, y float64) *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (result any, reason error) {
				return nil, ts.Tap(x, y) //nolint:wrapcheck
			})
		},
	}
}

// mapWorker to the JS module.
func mapWorker(vu moduleVU, w *common.Worker) mapping {
	return mapping{
		"url": w.URL(),
	}
}

// mapBrowserContext to the JS module.
func mapBrowserContext(vu moduleVU, bc *common.BrowserContext) mapping { //nolint:funlen
	rt := vu.Runtime()
	return mapping{
		"addCookies": bc.AddCookies,
		"addInitScript": func(script goja.Value) error {
			if !gojaValueExists(script) {
				return nil
			}

			source := ""
			switch script.ExportType() {
			case reflect.TypeOf(string("")):
				source = script.String()
			case reflect.TypeOf(goja.Object{}):
				opts := script.ToObject(rt)
				for _, k := range opts.Keys() {
					if k == "content" {
						source = opts.Get(k).String()
					}
				}
			default:
				_, isCallable := goja.AssertFunction(script)
				if !isCallable {
					source = fmt.Sprintf("(%s);", script.ToString().String())
				} else {
					source = fmt.Sprintf("(%s)(...args);", script.ToString().String())
				}
			}

			return bc.AddInitScript(source) //nolint:wrapcheck
		},
		"browser":          bc.Browser,
		"clearCookies":     bc.ClearCookies,
		"clearPermissions": bc.ClearPermissions,
		"close":            bc.Close,
		"cookies":          bc.Cookies,
		"grantPermissions": func(permissions []string, opts goja.Value) error {
			pOpts := common.NewGrantPermissionsOptions()
			pOpts.Parse(vu.Context(), opts)

			return bc.GrantPermissions(permissions, pOpts) //nolint:wrapcheck
		},
		"setDefaultNavigationTimeout": bc.SetDefaultNavigationTimeout,
		"setDefaultTimeout":           bc.SetDefaultTimeout,
		"setGeolocation":              bc.SetGeolocation,
		"setHTTPCredentials":          bc.SetHTTPCredentials, //nolint:staticcheck
		"setOffline":                  bc.SetOffline,
		"waitForEvent": func(event string, optsOrPredicate goja.Value) (*goja.Promise, error) {
			ctx := vu.Context()
			popts := common.NewWaitForEventOptions(
				bc.Timeout(),
			)
			if err := popts.Parse(ctx, optsOrPredicate); err != nil {
				return nil, fmt.Errorf("parsing waitForEvent options: %w", err)
			}

			return k6ext.Promise(ctx, func() (result any, reason error) {
				var runInTaskQueue func(p *common.Page) (bool, error)
				if popts.PredicateFn != nil {
					runInTaskQueue = func(p *common.Page) (bool, error) {
						tq := vu.taskQueueRegistry.get(p.TargetID())

						var rtn bool
						var err error
						// The function on the taskqueue runs in its own goroutine
						// so we need to use a channel to wait for it to complete
						// before returning the result to the caller.
						c := make(chan bool)
						tq.Queue(func() error {
							var resp goja.Value
							resp, err = popts.PredicateFn(vu.Runtime().ToValue(p))
							rtn = resp.ToBoolean()
							close(c)
							return nil
						})
						<-c

						return rtn, err //nolint:wrapcheck
					}
				}

				resp, err := bc.WaitForEvent(event, runInTaskQueue, popts.Timeout)
				panicIfFatalError(ctx, err)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				p, ok := resp.(*common.Page)
				if !ok {
					panicIfFatalError(ctx, fmt.Errorf("response object is not a page: %w", k6error.ErrFatal))
				}

				return mapPage(vu, p), nil
			}), nil
		},
		"pages": func() *goja.Object {
			var (
				mpages []mapping
				pages  = bc.Pages()
			)
			for _, page := range pages {
				if page == nil {
					continue
				}
				m := mapPage(vu, page)
				mpages = append(mpages, m)
			}

			return rt.ToValue(mpages).ToObject(rt)
		},
		"newPage": func() (mapping, error) {
			page, err := bc.NewPage()
			if err != nil {
				return nil, err //nolint:wrapcheck
			}
			return mapPage(vu, page), nil
		},
	}
}

// mapConsoleMessage to the JS module.
func mapConsoleMessage(vu moduleVU, cm *common.ConsoleMessage) mapping {
	rt := vu.Runtime()
	return mapping{
		"args": func() *goja.Object {
			var (
				margs []mapping
				args  = cm.Args
			)
			for _, arg := range args {
				a := mapJSHandle(vu, arg)
				margs = append(margs, a)
			}

			return rt.ToValue(margs).ToObject(rt)
		},
		// page(), text() and type() are defined as
		// functions in order to match Playwright's API
		"page": func() *goja.Object {
			mp := mapPage(vu, cm.Page)
			return rt.ToValue(mp).ToObject(rt)
		},
		"text": func() *goja.Object {
			return rt.ToValue(cm.Text).ToObject(rt)
		},
		"type": func() *goja.Object {
			return rt.ToValue(cm.Type).ToObject(rt)
		},
	}
}

// mapBrowser to the JS module.
func mapBrowser(vu moduleVU) mapping { //nolint:funlen
	rt := vu.Runtime()
	return mapping{
		"context": func() (*common.BrowserContext, error) {
			b, err := vu.browser()
			if err != nil {
				return nil, err
			}
			return b.Context(), nil
		},
		"closeContext": func() error {
			b, err := vu.browser()
			if err != nil {
				return err
			}
			return b.CloseContext() //nolint:wrapcheck
		},
		"isConnected": func() (bool, error) {
			b, err := vu.browser()
			if err != nil {
				return false, err
			}
			return b.IsConnected(), nil
		},
		"newContext": func(opts goja.Value) (*goja.Object, error) {
			b, err := vu.browser()
			if err != nil {
				return nil, err
			}
			bctx, err := b.NewContext(opts)
			if err != nil {
				return nil, err //nolint:wrapcheck
			}

			if err := initBrowserContext(bctx, vu.testRunID); err != nil {
				return nil, err
			}

			m := mapBrowserContext(vu, bctx)
			return rt.ToValue(m).ToObject(rt), nil
		},
		"userAgent": func() (string, error) {
			b, err := vu.browser()
			if err != nil {
				return "", err
			}
			return b.UserAgent(), nil
		},
		"version": func() (string, error) {
			b, err := vu.browser()
			if err != nil {
				return "", err
			}
			return b.Version(), nil
		},
		"newPage": func(opts goja.Value) (mapping, error) {
			b, err := vu.browser()
			if err != nil {
				return nil, err
			}
			page, err := b.NewPage(opts)
			if err != nil {
				return nil, err //nolint:wrapcheck
			}

			if err := initBrowserContext(b.Context(), vu.testRunID); err != nil {
				return nil, err
			}

			return mapPage(vu, page), nil
		},
	}
}

func initBrowserContext(bctx *common.BrowserContext, testRunID string) error {
	// Setting a k6 object which will contain k6 specific metadata
	// on the current test run. This allows external applications
	// (such as Grafana Faro) to identify that the session is a k6
	// automated one and not one driven by a real person.
	if err := bctx.AddInitScript(
		fmt.Sprintf(`window.k6 = { testRunId: %q }`, testRunID),
	); err != nil {
		return fmt.Errorf("adding k6 object to new browser context: %w", err)
	}

	return nil
}
