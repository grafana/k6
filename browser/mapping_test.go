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
		wildcards = wildcards()
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

			wm, wok := isWildcard(wildcards, typ.Name(), m)
			// if the method is a wildcard method, it should not
			// be mapped to the module. so we should not find it
			// in the mapped methods.
			if _, ok := mapped[m]; wok && ok {
				t.Errorf("method %s should not be mapped", m)
			}
			// change the method name if it is mapped to a wildcard
			// method. these wildcard methods are not exist on our
			// API. so we need to use the mapped method instead.
			if wok {
				m = wm
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
				return mapPage(vu, &common.Page{})
			},
		},
		"elementHandle": {
			apiInterface: (*api.ElementHandle)(nil),
			mapp: func() mapping {
				return mapElementHandle(vu, &common.ElementHandle{})
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
