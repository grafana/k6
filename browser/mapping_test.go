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
		// internal methods
		"ElementHandle.objectID": "",
		"Frame.id":               "",
		"Frame.loaderID":         "",
		"JSHandle.objectID":      "",
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
			typ     = reflect.TypeOf(tt.apiInterface).Elem()
			typName = typ.Name()
			mapped  = tt.mapp()
			tested  = make(map[string]bool)
		)
		for i := 0; i < typ.NumMethod(); i++ {
			method := typ.Method(i)
			require.NotNil(t, method)

			// goja uses methods that starts with lowercase.
			// so we need to convert the first letter to lowercase.
			m := toFirstLetterLower(method.Name)

			cm, cmok := isCustomMapping(customMappings, typName, m)
			// if the method is a custom mapping, it should not be
			// mapped to the module. so we should not find it in
			// the mapped methods.
			if _, ok := mapped[m]; cmok && ok {
				t.Errorf("method %s should not be mapped for %s", m, typName)
			}
			// a custom mapping with an empty string means that
			// the method should not exist on the API.
			if cmok && cm == "" {
				continue
			}
			// change the method name if it is mapped to a custom
			// method. these custom methods are not exist on our
			// API. so we need to use the mapped method instead.
			if cmok {
				m = cm
			}
			if _, ok := mapped[m]; !ok {
				t.Errorf("method %s for %s not found", m, typName)
			}
			// to detect if a method is redundantly mapped.
			tested[m] = true
		}
		// detect redundant mappings.
		for m := range mapped {
			if !tested[m] {
				t.Errorf("method %s is redundant for %s", m, typName)
			}
		}
	}

	for name, tt := range map[string]test{
		"browserType": {
			apiInterface: (*api.BrowserType)(nil),
			mapp: func() mapping {
				return mapBrowserType(moduleVU{VU: vu}, &chromium.BrowserType{})
			},
		},
		"browser": {
			apiInterface: (*api.Browser)(nil),
			mapp: func() mapping {
				return mapBrowser(moduleVU{VU: vu}, &chromium.Browser{})
			},
		},
		"browserContext": {
			apiInterface: (*api.BrowserContext)(nil),
			mapp: func() mapping {
				return mapBrowserContext(moduleVU{VU: vu}, &common.BrowserContext{})
			},
		},
		"page": {
			apiInterface: (*api.Page)(nil),
			mapp: func() mapping {
				return mapPage(moduleVU{VU: vu}, &common.Page{
					Keyboard:    &common.Keyboard{},
					Mouse:       &common.Mouse{},
					Touchscreen: &common.Touchscreen{},
				})
			},
		},
		"elementHandle": {
			apiInterface: (*api.ElementHandle)(nil),
			mapp: func() mapping {
				return mapElementHandle(moduleVU{VU: vu}, &common.ElementHandle{})
			},
		},
		"jsHandle": {
			apiInterface: (*api.JSHandle)(nil),
			mapp: func() mapping {
				return mapJSHandle(moduleVU{VU: vu}, &common.BaseJSHandle{})
			},
		},
		"frame": {
			apiInterface: (*api.Frame)(nil),
			mapp: func() mapping {
				return mapFrame(moduleVU{VU: vu}, &common.Frame{})
			},
		},
		"mapRequest": {
			apiInterface: (*api.Request)(nil),
			mapp: func() mapping {
				return mapRequest(moduleVU{VU: vu}, &common.Request{})
			},
		},
		"mapResponse": {
			apiInterface: (*api.Response)(nil),
			mapp: func() mapping {
				return mapResponse(moduleVU{VU: vu}, &common.Response{})
			},
		},
		"mapWorker": {
			apiInterface: (*api.Worker)(nil),
			mapp: func() mapping {
				return mapWorker(moduleVU{VU: vu}, &common.Worker{})
			},
		},
		"mapLocator": {
			apiInterface: (*api.Locator)(nil),
			mapp: func() mapping {
				return mapLocator(moduleVU{VU: vu}, &common.Locator{})
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
	special := map[string]string{
		"ID":        "id",
		"JSON":      "json",
		"JSONValue": "jsonValue",
		"URL":       "url",
	}
	if v, ok := special[s]; ok {
		return v
	}
	if s == "" {
		return ""
	}

	return strings.ToLower(s[:1]) + s[1:]
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
