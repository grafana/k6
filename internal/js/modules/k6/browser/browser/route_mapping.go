package browser

import (
	"context"

	"github.com/grafana/sobek"

	"go.k6.io/k6/internal/js/modules/k6/browser/common"
	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext"
)

// mapRoute to the JS module.
func mapRoute(vu moduleVU, route *common.Route) mapping {
	return mapping{
		"abort": func(reason string) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, route.Abort(reason)
			})
		},
		"fulfill": func(opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				fopts := parseFulfillOptions(vu.Context(), opts)
				return nil, route.Fulfill(fopts)
			})
		},
		"request": func() mapping {
			return mapRequest(vu, route.Request())
		},
	}
}

func parseFulfillOptions(ctx context.Context, opts sobek.Value) *common.FulfillOptions {
	if !sobekValueExists(opts) {
		return nil
	}

	rt := k6ext.Runtime(ctx)
	copts := &common.FulfillOptions{}

	obj := opts.ToObject(rt)
	for _, k := range obj.Keys() {
		switch k {
		case "body":
			copts.Body = obj.Get(k).String()
		case "contentType":
			copts.ContentType = obj.Get(k).String()
		case "headers":
			headers := obj.Get(k).ToObject(rt)
			headersKeys := headers.Keys()
			copts.Headers = make([]common.HTTPHeader, len(headersKeys))
			for i, hk := range headersKeys {
				copts.Headers[i] = common.HTTPHeader{
					Name:  hk,
					Value: headers.Get(hk).String(),
				}
			}
		case "status":
			copts.Status = obj.Get(k).ToInteger()
		}
	}

	return copts
}
