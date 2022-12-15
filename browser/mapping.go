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

// mapBrowserContext to the JS module.
func mapBrowserContext(rt *goja.Runtime, bc api.BrowserContext) mapping {
	_ = rt
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
		"pages":                       bc.Pages,
		"newPage":                     bc.NewPage,
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
		"newPage": b.NewPage,
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
