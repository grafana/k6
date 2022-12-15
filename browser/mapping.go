package browser

import (
	"fmt"

	"github.com/dop251/goja"

	"github.com/grafana/xk6-browser/api"
	"github.com/grafana/xk6-browser/chromium"

	k6common "go.k6.io/k6/js/common"
	k6modules "go.k6.io/k6/js/modules"
)

var _ = mapBrowserToGoja(nil)

// mapping is a type of mapping between our API (api/) and the JS
// module. It acts like a bridge and allows adding wildcard methods
// and customization over our API.
type mapping map[string]any

// mapBrowserToGoja maps the browser API to the JS module.
// The motivation of this mapping was to support $ and $$ wildcard
// methods.
// See issue #661 for more details.
func mapBrowserToGoja(vu k6modules.VU) *goja.Object {
	rt := vu.Runtime()

	var (
		obj         = rt.NewObject()
		browserType = chromium.NewBrowserType(vu)
	)
	for k, v := range mapBrowserType(rt, browserType) {
		err := obj.Set(k, rt.ToValue(v))
		if err != nil {
			k6common.Throw(rt, fmt.Errorf("mapping: %w", err))
		}
	}

	return obj
}

// mapFrame to the JS module.
//
//nolint:funlen
func mapFrame(rt *goja.Runtime, f api.Frame) mapping {
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
				mcfs = append(mcfs, mapFrame(rt, fr))
			}
			return rt.ToValue(mcfs).ToObject(rt)
		},
		"click":          f.Click,
		"content":        f.Content,
		"dblclick":       f.Dblclick,
		"dispatchEvent":  f.DispatchEvent,
		"evaluate":       f.Evaluate,
		"evaluateHandle": f.EvaluateHandle,
		"fill":           f.Fill,
		"focus":          f.Focus,
		"frameElement":   f.FrameElement,
		"getAttribute":   f.GetAttribute,
		"goto":           f.Goto,
		"hover":          f.Hover,
		"innerHTML":      f.InnerHTML,
		"innerText":      f.InnerText,
		"inputValue":     f.InputValue,
		"isChecked":      f.IsChecked,
		"isDetached":     f.IsDetached,
		"isDisabled":     f.IsDisabled,
		"isEditable":     f.IsEditable,
		"isEnabled":      f.IsEnabled,
		"isHidden":       f.IsHidden,
		"isVisible":      f.IsVisible,
		"iD":             f.ID,
		"loaderID":       f.LoaderID,
		"locator":        f.Locator,
		"name":           f.Name,
		"page": func() *goja.Object {
			mp := mapPage(rt, f.Page())
			return rt.ToValue(mp).ToObject(rt)
		},
		"parentFrame": func() *goja.Object {
			mf := mapFrame(rt, f.ParentFrame())
			return rt.ToValue(mf).ToObject(rt)
		},
		"press":             f.Press,
		"selectOption":      f.SelectOption,
		"setContent":        f.SetContent,
		"setInputFiles":     f.SetInputFiles,
		"tap":               f.Tap,
		"textContent":       f.TextContent,
		"title":             f.Title,
		"type":              f.Type,
		"uncheck":           f.Uncheck,
		"url":               f.URL,
		"waitForFunction":   f.WaitForFunction,
		"waitForLoadState":  f.WaitForLoadState,
		"waitForNavigation": f.WaitForNavigation,
		"waitForSelector":   f.WaitForSelector,
		"waitForTimeout":    f.WaitForTimeout,
	}

	return maps
}

// mapPage to the JS module.
//
//nolint:funlen
func mapPage(rt *goja.Runtime, p api.Page) mapping {
	maps := mapping{
		"addInitScript":           p.AddInitScript,
		"addScriptTag":            p.AddScriptTag,
		"addStyleTag":             p.AddStyleTag,
		"bringToFront":            p.BringToFront,
		"check":                   p.Check,
		"click":                   p.Click,
		"close":                   p.Close,
		"content":                 p.Content,
		"context":                 p.Context,
		"dblclick":                p.Dblclick,
		"dispatchEvent":           p.DispatchEvent,
		"dragAndDrop":             p.DragAndDrop,
		"emulateMedia":            p.EmulateMedia,
		"emulateVisionDeficiency": p.EmulateVisionDeficiency,
		"evaluate":                p.Evaluate,
		"evaluateHandle":          p.EvaluateHandle,
		"exposeBinding":           p.ExposeBinding,
		"exposeFunction":          p.ExposeFunction,
		"fill":                    p.Fill,
		"focus":                   p.Focus,
		"frame":                   p.Frame,
		"frames": func() *goja.Object {
			var (
				mfrs []mapping
				frs  = p.Frames()
			)
			for _, fr := range frs {
				mfrs = append(mfrs, mapFrame(rt, fr))
			}
			return rt.ToValue(mfrs).ToObject(rt)
		},
		"getAttribute": p.GetAttribute,
		"goBack":       p.GoBack,
		"goForward":    p.GoForward,
		"goto":         p.Goto,
		"hover":        p.Hover,
		"innerHTML":    p.InnerHTML,
		"innerText":    p.InnerText,
		"inputValue":   p.InputValue,
		"isChecked":    p.IsChecked,
		"isClosed":     p.IsClosed,
		"isDisabled":   p.IsDisabled,
		"isEditable":   p.IsEditable,
		"isEnabled":    p.IsEnabled,
		"isHidden":     p.IsHidden,
		"isVisible":    p.IsVisible,
		"locator":      p.Locator,
		"mainFrame": func() *goja.Object {
			mf := mapFrame(rt, p.MainFrame())
			return rt.ToValue(mf).ToObject(rt)
		},
		"opener":                      p.Opener,
		"pause":                       p.Pause,
		"pdf":                         p.Pdf,
		"press":                       p.Press,
		"reload":                      p.Reload,
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
		"type":                        p.Type,
		"uncheck":                     p.Uncheck,
		"unroute":                     p.Unroute,
		"url":                         p.URL,
		"video":                       p.Video,
		"viewportSize":                p.ViewportSize,
		"waitForEvent":                p.WaitForEvent,
		"waitForFunction":             p.WaitForFunction,
		"waitForLoadState":            p.WaitForLoadState,
		"waitForNavigation":           p.WaitForNavigation,
		"waitForRequest":              p.WaitForRequest,
		"waitForResponse":             p.WaitForResponse,
		"waitForSelector":             p.WaitForSelector,
		"waitForTimeout":              p.WaitForTimeout,
		"workers":                     p.Workers,
	}

	return maps
}

// mapBrowserContext to the JS module.
func mapBrowserContext(rt *goja.Runtime, bc api.BrowserContext) mapping {
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
				m := mapPage(rt, page)
				mpages = append(mpages, m)
			}

			return rt.ToValue(mpages).ToObject(rt)
		},
		"newPage": func() *goja.Object {
			page := bc.NewPage()
			m := mapPage(rt, page)
			return rt.ToValue(m).ToObject(rt)
		},
	}
}

// mapBrowser to the JS module.
func mapBrowser(rt *goja.Runtime, b api.Browser) mapping {
	return mapping{
		"close":       b.Close,
		"contexts":    b.Contexts,
		"isConnected": b.IsConnected,
		"on":          b.On,
		"userAgent":   b.UserAgent,
		"version":     b.Version,
		"newContext": func(opts goja.Value) *goja.Object {
			bctx := b.NewContext(opts)
			m := mapBrowserContext(rt, bctx)
			return rt.ToValue(m).ToObject(rt)
		},
		"newPage": func(opts goja.Value) *goja.Object {
			page := b.NewPage(opts)
			m := mapPage(rt, page)
			return rt.ToValue(m).ToObject(rt)
		},
	}
}

// mapBrowserType to the JS module.
func mapBrowserType(rt *goja.Runtime, bt api.BrowserType) mapping {
	return mapping{
		"connect":                 bt.Connect,
		"executablePath":          bt.ExecutablePath,
		"launchPersistentContext": bt.LaunchPersistentContext,
		"name":                    bt.Name,
		"launch": func(opts goja.Value) *goja.Object {
			m := mapBrowser(rt, bt.Launch(opts))
			return rt.ToValue(m).ToObject(rt)
		},
	}
}
