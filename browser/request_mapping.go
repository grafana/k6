package browser

import (
	"github.com/dop251/goja"

	"github.com/grafana/xk6-browser/common"
	"github.com/grafana/xk6-browser/k6ext"
)

// mapRequest to the JS module.
func mapRequest(vu moduleVU, r *common.Request) mapping {
	rt := vu.Runtime()
	maps := mapping{
		"allHeaders": func() *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return r.AllHeaders(), nil
			})
		},
		"frame": func() *goja.Object {
			mf := mapFrame(vu, r.Frame())
			return rt.ToValue(mf).ToObject(rt)
		},
		"headerValue": func(name string) *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return r.HeaderValue(name), nil
			})
		},
		"headers": func(name string) *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return r.Headers(), nil
			})
		},
		"headersArray": func(name string) *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return r.HeadersArray(), nil
			})
		},
		"isNavigationRequest": r.IsNavigationRequest,
		"method":              r.Method,
		"postData":            r.PostData,
		"postDataBuffer":      r.PostDataBuffer,
		"resourceType":        r.ResourceType,
		"response": func(name string) *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				resp := r.Response()
				if resp == nil {
					return nil, nil
				}
				return mapResponse(vu, resp), nil
			})
		},
		"size":   r.Size,
		"timing": r.Timing,
		"url":    r.URL,
	}

	return maps
}
