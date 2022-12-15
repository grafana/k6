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

// mapBrowserType to the JS module.
func mapBrowserType(rt *goja.Runtime, bt api.BrowserType) mapping {
	_ = rt
	return mapping{
		"connect":                 bt.Connect,
		"executablePath":          bt.ExecutablePath,
		"launchPersistentContext": bt.LaunchPersistentContext,
		"name":                    bt.Name,
		"launch":                  bt.Launch,
	}
}
