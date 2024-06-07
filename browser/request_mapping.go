package browser

import (
	"github.com/grafana/sobek"

	"github.com/grafana/xk6-browser/common"
	"github.com/grafana/xk6-browser/k6ext"
)

// mapRequest to the JS module.
func mapRequest(vu moduleVU, r *common.Request) mapping {
	rt := vu.Runtime()
	maps := mapping{
		"allHeaders": func() *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return r.AllHeaders(), nil
			})
		},
		"frame": func() *sobek.Object {
			mf := mapFrame(vu, r.Frame())
			return rt.ToValue(mf).ToObject(rt)
		},
		"headerValue": func(name string) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return r.HeaderValue(name), nil
			})
		},
		"headers": r.Headers,
		"headersArray": func() *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return r.HeadersArray(), nil
			})
		},
		"isNavigationRequest": r.IsNavigationRequest,
		"method":              r.Method,
		"postData": func() any {
			p := r.PostData()
			if p == "" {
				return nil
			}
			return p
		},
		"postDataBuffer": func() any {
			p := r.PostDataBuffer()
			if len(p) == 0 {
				return nil
			}
			return rt.NewArrayBuffer(p)
		},
		"resourceType": r.ResourceType,
		"response": func() *sobek.Promise {
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
