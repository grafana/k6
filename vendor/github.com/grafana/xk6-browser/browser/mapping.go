package browser

import (
	"context"
	"errors"
	"fmt"

	"github.com/dop251/goja"

	"github.com/grafana/xk6-browser/api"
	"github.com/grafana/xk6-browser/k6error"
	"github.com/grafana/xk6-browser/k6ext"

	k6common "go.k6.io/k6/js/common"
)

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

// mapLocator API to the JS module.
func mapLocator(vu moduleVU, lo api.Locator) mapping {
	return mapping{
		"click": func(opts goja.Value) *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				err := lo.Click(opts)
				return nil, err //nolint:wrapcheck
			})
		},
		"dblclick":      lo.Dblclick,
		"check":         lo.Check,
		"uncheck":       lo.Uncheck,
		"isChecked":     lo.IsChecked,
		"isEditable":    lo.IsEditable,
		"isEnabled":     lo.IsEnabled,
		"isDisabled":    lo.IsDisabled,
		"isVisible":     lo.IsVisible,
		"isHidden":      lo.IsHidden,
		"fill":          lo.Fill,
		"focus":         lo.Focus,
		"getAttribute":  lo.GetAttribute,
		"innerHTML":     lo.InnerHTML,
		"innerText":     lo.InnerText,
		"textContent":   lo.TextContent,
		"inputValue":    lo.InputValue,
		"selectOption":  lo.SelectOption,
		"press":         lo.Press,
		"type":          lo.Type,
		"hover":         lo.Hover,
		"tap":           lo.Tap,
		"dispatchEvent": lo.DispatchEvent,
		"waitFor":       lo.WaitFor,
	}
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
		"json":         r.JSON,
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
		"evaluateHandle": func(pageFunc goja.Value, args ...goja.Value) (mapping, error) {
			h, err := jsh.EvaluateHandle(pageFunc, args...)
			if err != nil {
				return nil, err //nolint:wrapcheck
			}
			return mapJSHandle(vu, h), nil
		},
		"getProperties": func() (mapping, error) {
			props, err := jsh.GetProperties()
			if err != nil {
				return nil, err //nolint:wrapcheck
			}

			dst := make(map[string]any)
			for k, v := range props {
				dst[k] = mapJSHandle(vu, v)
			}
			return dst, nil
		},
		"getProperty": func(propertyName string) *goja.Object {
			var (
				h = jsh.GetProperty(propertyName)
				m = mapJSHandle(vu, h)
			)
			return rt.ToValue(m).ToObject(rt)
		},
		"jsonValue": jsh.JSONValue,
	}
}

// mapElementHandle to the JS module.
//
//nolint:funlen
func mapElementHandle(vu moduleVU, eh api.ElementHandle) mapping {
	maps := mapping{
		"boundingBox": eh.BoundingBox,
		"check":       eh.Check,
		"click": func(opts goja.Value) *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				err := eh.Click(opts)
				return nil, err //nolint:wrapcheck
			})
		},
		"contentFrame": func() (mapping, error) {
			f, err := eh.ContentFrame()
			if err != nil {
				return nil, err //nolint:wrapcheck
			}
			return mapFrame(vu, f), nil
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
		"ownerFrame": func() (mapping, error) {
			f, err := eh.OwnerFrame()
			if err != nil {
				return nil, err //nolint:wrapcheck
			}
			return mapFrame(vu, f), nil
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
		"waitForSelector": func(selector string, opts goja.Value) (mapping, error) {
			eh, err := eh.WaitForSelector(selector, opts)
			if err != nil {
				return nil, err //nolint:wrapcheck
			}
			return mapElementHandle(vu, eh), nil
		},
	}
	maps["$"] = func(selector string) (mapping, error) {
		eh, err := eh.Query(selector)
		if err != nil {
			return nil, err //nolint:wrapcheck
		}
		ehm := mapElementHandle(vu, eh)
		return ehm, nil
	}
	maps["$$"] = func(selector string) ([]mapping, error) {
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
		"evaluateHandle": func(pageFunction goja.Value, args ...goja.Value) (mapping, error) {
			jsh, err := f.EvaluateHandle(pageFunction, args...)
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
		eh, err := f.Query(selector)
		if err != nil {
			return nil, err //nolint:wrapcheck
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
		"evaluateHandle": func(pageFunc goja.Value, args ...goja.Value) (mapping, error) {
			jsh, err := p.EvaluateHandle(pageFunc, args...)
			if err != nil {
				return nil, err //nolint:wrapcheck
			}
			return mapJSHandle(vu, jsh), nil
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
		"goBack": func(opts goja.Value) *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				resp := p.GoBack(opts)
				return mapResponse(vu, resp), nil
			})
		},
		"goForward": p.GoForward,
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
			mapMsgAndHandleEvent := func(m *api.ConsoleMessage) error {
				mapping := mapConsoleMessage(vu, m)
				_, err := handler(goja.Undefined(), vu.Runtime().ToValue(mapping))
				return err
			}
			return p.On(event, mapMsgAndHandleEvent) //nolint:wrapcheck
		},
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

// mapWorker to the JS module.
func mapWorker(vu moduleVU, w api.Worker) mapping {
	return mapping{
		"evaluate": w.Evaluate,
		"evaluateHandle": func(pageFunc goja.Value, args ...goja.Value) (mapping, error) {
			h, err := w.EvaluateHandle(pageFunc, args...)
			if err != nil {
				panicIfFatalError(vu.Context(), err)
				return nil, err //nolint:wrapcheck
			}
			return mapJSHandle(vu, h), nil
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
		"setExtraHTTPHeaders": func(headers map[string]string) *goja.Promise {
			ctx := vu.Context()
			return k6ext.Promise(ctx, func() (result any, reason error) {
				err := bc.SetExtraHTTPHeaders(headers)
				panicIfFatalError(ctx, err)
				return nil, err //nolint:wrapcheck
			})
		},
		"setGeolocation":     bc.SetGeolocation,
		"setHTTPCredentials": bc.SetHTTPCredentials, //nolint:staticcheck
		"setOffline":         bc.SetOffline,
		"storageState":       bc.StorageState,
		"unroute":            bc.Unroute,
		"waitForEvent":       bc.WaitForEvent,
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
func mapConsoleMessage(vu moduleVU, cm *api.ConsoleMessage) mapping {
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
func mapBrowser(vu moduleVU) mapping {
	rt := vu.Runtime()
	return mapping{
		"context": func() (api.BrowserContext, error) {
			b, err := vu.browser()
			if err != nil {
				return nil, err
			}
			return b.Context(), nil
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
			return mapPage(vu, page), nil
		},
	}
}

func panicIfFatalError(ctx context.Context, err error) {
	if errors.Is(err, k6error.ErrFatal) {
		k6ext.Abort(ctx, err.Error())
	}
}
