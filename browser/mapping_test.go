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

// TestMappings tests that all the methods of the API (api/) are
// to the module. This is to ensure that we don't forget to map
// a new method to the module.
func TestMappings(t *testing.T) {
	t.Parallel()

	vu := &k6modulestest.VU{
		RuntimeField: goja.New(),
		InitEnvField: &k6common.InitEnvironment{
			Registry: k6metrics.NewRegistry(),
		},
	}
	rt := vu.Runtime()

	type test struct {
		apiObj any
		mapp   func() mapping
	}
	mappers := map[string]test{
		"browserType": {
			apiObj: (*api.BrowserType)(nil),
			mapp: func() mapping {
				return mapBrowserType(rt, &chromium.BrowserType{})
			},
		},
		"browser": {
			apiObj: (*api.Browser)(nil),
			mapp: func() mapping {
				return mapBrowser(rt, &chromium.Browser{})
			},
		},
		"browserContext": {
			apiObj: (*api.BrowserContext)(nil),
			mapp: func() mapping {
				return mapBrowserContext(rt, &common.BrowserContext{})
			},
		},
		"page": {
			apiObj: (*api.Page)(nil),
			mapp: func() mapping {
				return mapPage(rt, &common.Page{})
			},
		},
		"elementHandle": {
			apiObj: (*api.ElementHandle)(nil),
			mapp: func() mapping {
				return mapElementHandle(rt, &common.ElementHandle{})
			},
		},
		"frame": {
			apiObj: (*api.Frame)(nil),
			mapp: func() mapping {
				return mapFrame(rt, &common.Frame{})
			},
		},
		"mapRequest": {
			apiObj: (*api.Request)(nil),
			mapp: func() mapping {
				return mapRequest(rt, &common.Request{})
			},
		},
		"mapResponse": {
			apiObj: (*api.Response)(nil),
			mapp: func() mapping {
				return mapResponse(rt, &common.Response{})
			},
		},
	}

	wildcards := wildcards()

	for name, tt := range mappers {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var (
				typ    = reflect.TypeOf(tt.apiObj).Elem()
				mapped = tt.mapp()
			)
			for i := 0; i < typ.NumMethod(); i++ {
				method := typ.Method(i)
				require.NotNil(t, method)

				m := toFirstLetterLower(method.Name)
				// change the method name if it is mapped to a wildcard
				// method. these wildcard methods are not exist on our
				// API. so we need to use the mapped method instead.
				if wm, ok := isWildcard(wildcards, typ.Name(), m); ok {
					m = wm
				}
				if _, ok := mapped[m]; !ok {
					t.Errorf("method %s not found", m)
				}
			}
		})
	}
}

// toFirstLetterLower converts the first letter of the string to lower case.
func toFirstLetterLower(s string) string {
	// Special case for URL.
	// Instead of loading up an acronyms list, just do this.
	// Good enough for our purposes.
	if s == "URL" {
		return "url"
	}
	return strings.ToLower(s[:1]) + s[1:]
}

// isWildcard returns true if the method is a wildcard method and
// returns the name of the method to be called instead of the original
// method.
func isWildcard(wildcards map[string]string, typ, method string) (string, bool) {
	name := typ + "." + method
	s, ok := wildcards[name]
	return s, ok
}
