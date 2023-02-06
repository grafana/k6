package browser

import (
	"reflect"
	"strings"
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/require"

	"github.com/grafana/xk6-browser/api"
	"github.com/grafana/xk6-browser/chromium"
	"github.com/grafana/xk6-browser/common"

	k6common "go.k6.io/k6/js/common"
	k6modulestest "go.k6.io/k6/js/modulestest"
	k6metrics "go.k6.io/k6/metrics"
)

// customMappings is a list of custom mappings for our API (api/).
// Some of them are wildcards, such as query to $ mapping; and
// others are for publicly accessible fields, such as mapping
// of page.keyboard to Page.getKeyboard.
func customMappings() map[string]string {
	return map[string]string{
		// wildcards
		"Page.query":             "$",
		"Page.queryAll":          "$$",
		"Frame.query":            "$",
		"Frame.queryAll":         "$$",
		"ElementHandle.query":    "$",
		"ElementHandle.queryAll": "$$",
		// getters
		"Page.getKeyboard":    "keyboard",
		"Page.getMouse":       "mouse",
		"Page.getTouchscreen": "touchscreen",
	}
}

// TestMappings tests that all the methods of the API (api/) are
// to the module. This is to ensure that we don't forget to map
// a new method to the module.
func TestMappings(t *testing.T) {
	t.Parallel()

	type test struct {
		apiInterface any
		mapp         func() mapping
	}

	var (
		vu = &k6modulestest.VU{
			RuntimeField: goja.New(),
			InitEnvField: &k6common.InitEnvironment{
				Registry: k6metrics.NewRegistry(),
			},
		}
		customMappings = customMappings()
	)

	// testMapping tests that all the methods of an API are mapped
	// to the module. And wildcards are mapped correctly and their
	// methods are not mapped.
	testMapping := func(tt test) {
		var (
			typ    = reflect.TypeOf(tt.apiInterface).Elem()
			mapped = tt.mapp()
		)
		for i := 0; i < typ.NumMethod(); i++ {
			method := typ.Method(i)
			require.NotNil(t, method)

			// goja uses methods that starts with lowercase.
			// so we need to convert the first letter to lowercase.
			m := toFirstLetterLower(method.Name)

			cm, cmok := isCustomMapping(customMappings, typ.Name(), m)
			// if the method is a custom mapping, it should not be
			// mapped to the module. so we should not find it in
			// the mapped methods.
			if _, ok := mapped[m]; cmok && ok {
				t.Errorf("method %s should not be mapped", m)
			}
			// change the method name if it is mapped to a custom
			// method. these custom methods are not exist on our
			// API. so we need to use the mapped method instead.
			if cmok {
				m = cm
			}
			if _, ok := mapped[m]; !ok {
				t.Errorf("method %s not found", m)
			}
		}
	}

	for name, tt := range map[string]test{
		"browserType": {
			apiInterface: (*api.BrowserType)(nil),
			mapp: func() mapping {
				return mapBrowserType(vu, &chromium.BrowserType{})
			},
		},
		"browser": {
			apiInterface: (*api.Browser)(nil),
			mapp: func() mapping {
				return mapBrowser(vu, &chromium.Browser{})
			},
		},
		"browserContext": {
			apiInterface: (*api.BrowserContext)(nil),
			mapp: func() mapping {
				return mapBrowserContext(vu, &common.BrowserContext{})
			},
		},
		"page": {
			apiInterface: (*api.Page)(nil),
			mapp: func() mapping {
				return mapPage(vu, &common.Page{
					Keyboard:    &common.Keyboard{},
					Mouse:       &common.Mouse{},
					Touchscreen: &common.Touchscreen{},
				})
			},
		},
		"elementHandle": {
			apiInterface: (*api.ElementHandle)(nil),
			mapp: func() mapping {
				return mapElementHandle(vu, &common.ElementHandle{})
			},
		},
		"jsHandle": {
			apiInterface: (*api.JSHandle)(nil),
			mapp: func() mapping {
				return mapJSHandle(vu, &common.BaseJSHandle{})
			},
		},
		"frame": {
			apiInterface: (*api.Frame)(nil),
			mapp: func() mapping {
				return mapFrame(vu, &common.Frame{})
			},
		},
		"mapRequest": {
			apiInterface: (*api.Request)(nil),
			mapp: func() mapping {
				return mapRequest(vu, &common.Request{})
			},
		},
		"mapResponse": {
			apiInterface: (*api.Response)(nil),
			mapp: func() mapping {
				return mapResponse(vu, &common.Response{})
			},
		},
		"mapWorker": {
			apiInterface: (*api.Worker)(nil),
			mapp: func() mapping {
				return mapWorker(vu, &common.Worker{})
			},
		},
	} {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			testMapping(tt)
		})
	}
}

// toFirstLetterLower converts the first letter of the string to lower case.
func toFirstLetterLower(s string) string {
	// Special cases.
	// Instead of loading up an acronyms list, just do this.
	// Good enough for our purposes.
	switch s {
	default:
		return strings.ToLower(s[:1]) + s[1:]
	case "URL":
		return "url"
	case "JSONValue":
		return "jsonValue"
	}
}

// isCustomMapping returns true if the method is a custom mapping
// and returns the name of the method to be called instead of the
// original one.
func isCustomMapping(customMappings map[string]string, typ, method string) (string, bool) {
	name := typ + "." + method

	if s, ok := customMappings[name]; ok {
		return s, ok
	}

	return "", false
}
