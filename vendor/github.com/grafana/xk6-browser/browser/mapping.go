package browser

import (
	"context"
	"fmt"

	"github.com/dop251/goja"

	"github.com/grafana/xk6-browser/api"
	"github.com/grafana/xk6-browser/chromium"
	"github.com/grafana/xk6-browser/k6ext"

	k6common "go.k6.io/k6/js/common"
	k6modules "go.k6.io/k6/js/modules"
)

// moduleVU carries module specific VU information.
//
// Currently, it is used to carry the VU object to the
// inner objects and promises.
type moduleVU struct {
	k6modules.VU
}

func (vu moduleVU) Context() context.Context {
	// promises and inner objects need the VU object to be
	// able to use k6-core specific functionality.
	//
	// We should not cache the context (especially the init
	// context from the vu that is received from k6 in
	// NewModuleInstance).
	return k6ext.WithVU(vu.VU.Context(), vu.VU)
}

// mapping is a type of mapping between our API (api/) and the JS
// module. It acts like a bridge and allows adding wildcard methods
// and customization over our API.
//
// TODO
// We should put this type back in when the following issue is resolved
// on the Goja side:
// https://github.com/dop251/goja/issues/469
type mapping = map[string]any

// mapBrowserToGoja maps the browser API to the JS module.
// The motivation of this mapping was to support $ and $$ wildcard
// methods.
// See issue #661 for more details.
func mapBrowserToGoja(vu moduleVU) *goja.Object {
	var (
		rt          = vu.Runtime()
		obj         = rt.NewObject()
		browserType = chromium.NewBrowserType(vu)
	)
	for k, v := range mapBrowserType(vu, browserType) {
		err := obj.Set(k, rt.ToValue(v))
		if err != nil {
			k6common.Throw(rt, fmt.Errorf("mapping: %w", err))
		}
	}

	return obj
}

// mapRequest to the JS module.
func mapRequest(vu moduleVU, r api.Request) mapping {
	rt := vu.Runtime()
	maps := mapping{
		"allHeaders": r.AllHeaders,
		"failure":    r.Failure,
		"frame": func() *goja.Object {
			mf := mapFrame(vu, r.Frame())
			return rt.ToValue(mf).ToObject(rt)
		},
		"headerValue":         r.HeaderValue,
		"headers":             r.Headers,
		"headersArray":        r.HeadersArray,
		"isNavigationRequest": r.IsNavigationRequest,
		"method":              r.Method,
		"postData":            r.PostData,
		"postDataBuffer":      r.PostDataBuffer,
		"postDataJSON":        r.PostDataJSON,
		"redirectedFrom": func() *goja.Object {
			mr := mapRequest(vu, r.RedirectedFrom())
			return rt.ToValue(mr).ToObject(rt)
		},
		"redirectedTo": func() *goja.Object {
			mr := mapRequest(vu, r.RedirectedTo())
			return rt.ToValue(mr).ToObject(rt)
		},
		"resourceType": r.ResourceType,
		"response": func() *goja.Object {
			mr := mapResponse(vu, r.Response())
			return rt.ToValue(mr).ToObject(rt)
		},
		"size":   r.Size,
		"timing": r.Timing,
		"url":    r.URL,
	}

	return maps
}

// mapResponse to the JS module.
func mapResponse(vu moduleVU, r api.Response) mapping {
	if r == nil {
		return nil
	}
	rt := vu.Runtime()
	maps := mapping{
		"allHeaders": r.AllHeaders,
		"body":       r.Body,
		"finished":   r.Finished,
		"frame": func() *goja.Object {
			mf := mapFrame(vu, r.Frame())
			return rt.ToValue(mf).ToObject(rt)
		},
		"headerValue":  r.HeaderValue,
		"headerValues": r.HeaderValues,
		"headers":      r.Headers,
		"headersArray": r.HeadersArray,
		"jSON":         r.JSON,
		"ok":           r.Ok,
		"request": func() *goja.Object {
			mr := mapRequest(vu, r.Request())
			return rt.ToValue(mr).ToObject(rt)
		},
		"securityDetails": r.SecurityDetails,
		"serverAddr":      r.ServerAddr,
		"size":            r.Size,
		"status":          r.Status,
		"statusText":      r.StatusText,
		"url":             r.URL,
	}

	return maps
}

// mapJSHandle to the JS module.
func mapJSHandle(vu moduleVU, jsh api.JSHandle) mapping {
	rt := vu.Runtime()
	return mapping{
		"asElement": func() *goja.Object {
			m := mapElementHandle(vu, jsh.AsElement())
			return rt.ToValue(m).ToObject(rt)
		},
		"dispose":  jsh.Dispose,
		"evaluate": jsh.Evaluate,
		"evaluateHandle": func(pageFunc goja.Value, args ...goja.Value) *goja.Object {
			var (
				h = jsh.EvaluateHandle(pageFunc, args...)
				m = mapJSHandle(vu, h)
			)
			return rt.ToValue(m).ToObject(rt)
		},
		"getProperties": func() *goja.Object {
			dst := make(map[string]any)
			for k, v := range jsh.GetProperties() {
				dst[k] = mapJSHandle(vu, v)
			}
			return rt.ToValue(dst).ToObject(rt)
		},
		"getProperty": func(propertyName string) *goja.Object {
			var (
				h = jsh.GetProperty(propertyName)
				m = mapJSHandle(vu, h)
			)
			return rt.ToValue(m).ToObject(rt)
		},
		"jsonValue": jsh.JSONValue,
		"objectID":  jsh.ObjectID,
	}
}

// mapElementHandle to the JS module.
//
//nolint:funlen
func mapElementHandle(vu moduleVU, eh api.ElementHandle) mapping {
	rt := vu.Runtime()
	maps := mapping{
		"boundingBox": eh.BoundingBox,
		"check":       eh.Check,
		"click": func(opts goja.Value) *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				err := eh.Click(opts)
				return nil, err //nolint:wrapcheck
			})
		},
		"contentFrame": func() *goja.Object {
			f := eh.ContentFrame()
			mf := mapFrame(vu, f)
			return rt.ToValue(mf).ToObject(rt)
		},
		"dblclick":      eh.Dblclick,
		"dispatchEvent": eh.DispatchEvent,
		"fill":          eh.Fill,
		"focus":         eh.Focus,
		"getAttribute":  eh.GetAttribute,
		"hover":         eh.Hover,
		"innerHTML":     eh.InnerHTML,
		"innerText":     eh.InnerText,
		"inputValue":    eh.InputValue,
		"isChecked":     eh.IsChecked,
		"isDisabled":    eh.IsDisabled,
		"isEditable":    eh.IsEditable,
		"isEnabled":     eh.IsEnabled,
		"isHidden":      eh.IsHidden,
		"isVisible":     eh.IsVisible,
		"ownerFrame": func() *goja.Object {
			f := eh.OwnerFrame()
			mf := mapFrame(vu, f)
			return rt.ToValue(mf).ToObject(rt)
		},
		"press":                  eh.Press,
		"screenshot":             eh.Screenshot,
		"scrollIntoViewIfNeeded": eh.ScrollIntoViewIfNeeded,
		"selectOption":           eh.SelectOption,
		"selectText":             eh.SelectText,
		"setInputFiles":          eh.SetInputFiles,
		"tap":                    eh.Tap,
		"textContent":            eh.TextContent,
		"type":                   eh.Type,
		"uncheck":                eh.Uncheck,
		"waitForElementState":    eh.WaitForElementState,
		"waitForSelector": func(selector string, opts goja.Value) *goja.Object {
			eh := eh.WaitForSelector(selector, opts)
			ehm := mapElementHandle(vu, eh)
			return rt.ToValue(ehm).ToObject(rt)
		},
	}
	maps["$"] = func(selector string) *goja.Object {
		eh := eh.Query(selector)
		ehm := mapElementHandle(vu, eh)
		return rt.ToValue(ehm).ToObject(rt)
	}
	maps["$$"] = func(selector string) *goja.Object {
		var (
			mehs []mapping
			ehs  = eh.QueryAll(selector)
		)
		for _, eh := range ehs {
			ehm := mapElementHandle(vu, eh)
			mehs = append(mehs, ehm)
		}
		return rt.ToValue(mehs).ToObject(rt)
	}

	jsHandleMap := mapJSHandle(vu, eh)
	for k, v := range jsHandleMap {
		maps[k] = v
	}

	return maps
}

// mapFrame to the JS module.
//
//nolint:funlen
func mapFrame(vu moduleVU, f api.Frame) mapping {
	rt := vu.Runtime()
	maps := mapping{
		"addScriptTag": f.AddScriptTag,
		"addStyleTag":  f.AddStyleTag,
		"check":        f.Check,
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
		"click": func(selector string, opts goja.Value) *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				err := f.Click(selector, opts)
				return nil, err //nolint:wrapcheck
			})
		},
		"content":       f.Content,
		"dblclick":      f.Dblclick,
		"dispatchEvent": f.DispatchEvent,
		"evaluate":      f.Evaluate,
		"evaluateHandle": func(pageFunction goja.Value, args ...goja.Value) *goja.Object {
			jsh := f.EvaluateHandle(pageFunction, args...)
			ehm := mapJSHandle(vu, jsh)
			return rt.ToValue(ehm).ToObject(rt)
		},
		"fill":  f.Fill,
		"focus": f.Focus,
		"frameElement": func() *goja.Object {
			eh := mapElementHandle(vu, f.FrameElement())
			return rt.ToValue(eh).ToObject(rt)
		},
		"getAttribute": f.GetAttribute,
		"goto": func(url string, opts goja.Value) *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				resp, err := f.Goto(url, opts)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}

				return mapResponse(vu, resp), nil
			})
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
		"iD":         f.ID,
		"loaderID":   f.LoaderID,
		"locator":    f.Locator,
		"name":       f.Name,
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
		"tap":           f.Tap,
		"textContent":   f.TextContent,
		"title":         f.Title,
		"type":          f.Type,
		"uncheck":       f.Uncheck,
		"url":           f.URL,
		"waitForFunction": func(pageFunc, opts goja.Value, args ...goja.Value) *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (result any, reason error) {
				return f.WaitForFunction(pageFunc, opts, args...) //nolint:wrapcheck
			})
		},
		"waitForLoadState": f.WaitForLoadState,
		"waitForNavigation": func(opts goja.Value) *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (result any, reason error) {
				resp, err := f.WaitForNavigation(opts)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				return mapResponse(vu, resp), nil
			})
		},
		"waitForSelector": func(selector string, opts goja.Value) *goja.Object {
			eh := f.WaitForSelector(selector, opts)
			ehm := mapElementHandle(vu, eh)
			return rt.ToValue(ehm).ToObject(rt)
		},
		"waitForTimeout": f.WaitForTimeout,
	}
	maps["$"] = func(selector string) *goja.Object {
		eh := f.Query(selector)
		ehm := mapElementHandle(vu, eh)
		return rt.ToValue(ehm).ToObject(rt)
	}
	maps["$$"] = func(selector string) *goja.Object {
		var (
			mehs []mapping
			ehs  = f.QueryAll(selector)
		)
		for _, eh := range ehs {
			ehm := mapElementHandle(vu, eh)
			mehs = append(mehs, ehm)
		}
		return rt.ToValue(mehs).ToObject(rt)
	}

	return maps
}

// mapPage to the JS module.
//
//nolint:funlen
func mapPage(vu moduleVU, p api.Page) mapping {
	rt := vu.Runtime()
	maps := mapping{
		"addInitScript": p.AddInitScript,
		"addScriptTag":  p.AddScriptTag,
		"addStyleTag":   p.AddStyleTag,
		"bringToFront":  p.BringToFront,
		"check":         p.Check,
		"click": func(selector string, opts goja.Value) *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				err := p.Click(selector, opts)
				return nil, err //nolint:wrapcheck
			})
		},
		"close":                   p.Close,
		"content":                 p.Content,
		"context":                 p.Context,
		"dblclick":                p.Dblclick,
		"dispatchEvent":           p.DispatchEvent,
		"dragAndDrop":             p.DragAndDrop,
		"emulateMedia":            p.EmulateMedia,
		"emulateVisionDeficiency": p.EmulateVisionDeficiency,
		"evaluate":                p.Evaluate,
		"evaluateHandle": func(pageFunc goja.Value, args ...goja.Value) *goja.Object {
			var (
				jsh = p.EvaluateHandle(pageFunc, args...)
				m   = mapJSHandle(vu, jsh)
			)
			return rt.ToValue(m).ToObject(rt)
		},
		"exposeBinding":  p.ExposeBinding,
		"exposeFunction": p.ExposeFunction,
		"fill":           p.Fill,
		"focus":          p.Focus,
		"frame":          p.Frame,
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
		"goBack":       p.GoBack,
		"goForward":    p.GoForward,
		"goto": func(url string, opts goja.Value) *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				resp, err := p.Goto(url, opts)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}

				return mapResponse(vu, resp), nil
			})
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
		"locator":    p.Locator,
		"mainFrame": func() *goja.Object {
			mf := mapFrame(vu, p.MainFrame())
			return rt.ToValue(mf).ToObject(rt)
		},
		"mouse":  rt.ToValue(p.GetMouse()).ToObject(rt),
		"opener": p.Opener,
		"pause":  p.Pause,
		"pdf":    p.Pdf,
		"press":  p.Press,
		"reload": func(opts goja.Value) *goja.Object {
			r := mapResponse(vu, p.Reload(opts))
			return rt.ToValue(r).ToObject(rt)
		},
		"route":                       p.Route,
		"screenshot":                  p.Screenshot,
		"selectOption":                p.SelectOption,
		"setContent":                  p.SetContent,
		"setDefaultNavigationTimeout": p.SetDefaultNavigationTimeout,
		"setDefaultTimeout":           p.SetDefaultTimeout,
		"setExtraHTTPHeaders":         p.SetExtraHTTPHeaders,
		"setInputFiles":               p.SetInputFiles,
		"setViewportSize":             p.SetViewportSize,
		"tap":                         p.Tap,
		"textContent":                 p.TextContent,
		"title":                       p.Title,
		"touchscreen":                 rt.ToValue(p.GetTouchscreen()).ToObject(rt),
		"type":                        p.Type,
		"uncheck":                     p.Uncheck,
		"unroute":                     p.Unroute,
		"url":                         p.URL,
		"video":                       p.Video,
		"viewportSize":                p.ViewportSize,
		"waitForEvent":                p.WaitForEvent,
		"waitForFunction": func(pageFunc, opts goja.Value, args ...goja.Value) *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (result any, reason error) {
				return p.WaitForFunction(pageFunc, opts, args...) //nolint:wrapcheck
			})
		},
		"waitForLoadState": p.WaitForLoadState,
		"waitForNavigation": func(opts goja.Value) *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (result any, reason error) {
				resp, err := p.WaitForNavigation(opts)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				return mapResponse(vu, resp), nil
			})
		},
		"waitForRequest":  p.WaitForRequest,
		"waitForResponse": p.WaitForResponse,
		"waitForSelector": p.WaitForSelector,
		"waitForTimeout":  p.WaitForTimeout,
		"workers": func() *goja.Object {
			var mws []mapping
			for _, w := range p.Workers() {
				mw := mapWorker(vu, w)
				mws = append(mws, mw)
			}
			return rt.ToValue(mws).ToObject(rt)
		},
	}
	maps["$"] = func(selector string) *goja.Object {
		eh := p.Query(selector)
		ehm := mapElementHandle(vu, eh)
		return rt.ToValue(ehm).ToObject(rt)
	}
	maps["$$"] = func(selector string) *goja.Object {
		var (
			mehs []mapping
			ehs  = p.QueryAll(selector)
		)
		for _, eh := range ehs {
			ehm := mapElementHandle(vu, eh)
			mehs = append(mehs, ehm)
		}
		return rt.ToValue(mehs).ToObject(rt)
	}

	return maps
}

// mapWorker to the JS module.
func mapWorker(vu moduleVU, w api.Worker) mapping {
	rt := vu.Runtime()
	return mapping{
		"evaluate": w.Evaluate,
		"evaluateHandle": func(pageFunc goja.Value, args ...goja.Value) *goja.Object {
			h := w.EvaluateHandle(pageFunc, args...)
			m := mapJSHandle(vu, h)
			return rt.ToValue(m).ToObject(rt)
		},
		"url": w.URL(),
	}
}

// mapBrowserContext to the JS module.
func mapBrowserContext(vu moduleVU, bc api.BrowserContext) mapping {
	rt := vu.Runtime()
	return mapping{
		"addCookies":                  bc.AddCookies,
		"addInitScript":               bc.AddInitScript,
		"browser":                     bc.Browser,
		"clearCookies":                bc.ClearCookies,
		"clearPermissions":            bc.ClearPermissions,
		"close":                       bc.Close,
		"cookies":                     bc.Cookies,
		"exposeBinding":               bc.ExposeBinding,
		"exposeFunction":              bc.ExposeFunction,
		"grantPermissions":            bc.GrantPermissions,
		"newCDPSession":               bc.NewCDPSession,
		"route":                       bc.Route,
		"setDefaultNavigationTimeout": bc.SetDefaultNavigationTimeout,
		"setDefaultTimeout":           bc.SetDefaultTimeout,
		"setExtraHTTPHeaders":         bc.SetExtraHTTPHeaders,
		"setGeolocation":              bc.SetGeolocation,
		"setHTTPCredentials":          bc.SetHTTPCredentials, //nolint:staticcheck
		"setOffline":                  bc.SetOffline,
		"storageState":                bc.StorageState,
		"unroute":                     bc.Unroute,
		"waitForEvent":                bc.WaitForEvent,
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
		"newPage": func() *goja.Object {
			page := bc.NewPage()
			m := mapPage(vu, page)
			return rt.ToValue(m).ToObject(rt)
		},
	}
}

// mapBrowser to the JS module.
func mapBrowser(vu moduleVU, b api.Browser) mapping {
	rt := vu.Runtime()
	return mapping{
		"close":       b.Close,
		"contexts":    b.Contexts,
		"isConnected": b.IsConnected,
		"on": func(event string) *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (result any, reason error) {
				return b.On(event) //nolint:wrapcheck
			})
		},
		"userAgent": b.UserAgent,
		"version":   b.Version,
		"newContext": func(opts goja.Value) *goja.Object {
			bctx := b.NewContext(opts)
			m := mapBrowserContext(vu, bctx)
			return rt.ToValue(m).ToObject(rt)
		},
		"newPage": func(opts goja.Value) *goja.Object {
			page := b.NewPage(opts)
			m := mapPage(vu, page)
			return rt.ToValue(m).ToObject(rt)
		},
	}
}

// mapBrowserType to the JS module.
func mapBrowserType(vu moduleVU, bt api.BrowserType) mapping {
	rt := vu.Runtime()
	return mapping{
		"connect":                 bt.Connect,
		"executablePath":          bt.ExecutablePath,
		"launchPersistentContext": bt.LaunchPersistentContext,
		"name":                    bt.Name,
		"launch": func(opts goja.Value) *goja.Object {
			m := mapBrowser(vu, bt.Launch(opts))
			return rt.ToValue(m).ToObject(rt)
		},
	}
}
