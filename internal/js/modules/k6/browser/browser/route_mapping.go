package browser

import (
	"context"
	"fmt"

	"github.com/grafana/sobek"

	"go.k6.io/k6/internal/js/modules/k6/browser/common"
	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext"
	jsCommon "go.k6.io/k6/js/common"
)

// mapRoute to the JS module.
func mapRoute(vu moduleVU, route *common.Route) mapping {
	return mapping{
		"abort": func(reason string) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, route.Abort(reason)
			})
		},
		"continue": func(opts sobek.Value) *sobek.Promise {
			copts, err := parseContinueOptions(vu.Context(), opts)
			return k6ext.Promise(vu.Context(), func() (any, error) {
				if err != nil {
					return nil, err
				}
				return nil, route.Continue(copts)
			})
		},
		"fulfill": func(opts sobek.Value) *sobek.Promise {
			fopts, err := parseFulfillOptions(vu.Context(), opts)
			return k6ext.Promise(vu.Context(), func() (any, error) {
				if err != nil {
					return nil, err
				}
				return nil, route.Fulfill(fopts)
			})
		},
		"request": func() mapping {
			return mapRequest(vu, route.Request())
		},
	}
}

func parseContinueOptions(ctx context.Context, opts sobek.Value) (common.ContinueOptions, error) {
	copts := common.ContinueOptions{}
	if !sobekValueExists(opts) {
		return copts, nil
	}

	rt := k6ext.Runtime(ctx)
	obj := opts.ToObject(rt)
	for _, k := range obj.Keys() {
		switch k {
		case "headers":
			copts.Headers = parseHeaders(obj.Get(k).ToObject(rt))
		case "method":
			copts.Method = obj.Get(k).String()
		case "postData":
			bytesData, err := jsCommon.ToBytes(obj.Get(k).Export())
			if err != nil {
				return copts, err
			}
			copts.PostData = bytesData
		case "url":
			copts.URL = obj.Get(k).String()
		}
	}

	return copts, nil
}

func parseFulfillOptions(ctx context.Context, opts sobek.Value) (common.FulfillOptions, error) {
	fopts := common.FulfillOptions{}
	if !sobekValueExists(opts) {
		return fopts, nil
	}

	rt := k6ext.Runtime(ctx)
	obj := opts.ToObject(rt)
	for _, k := range obj.Keys() {
		switch k {
		case "body":
			bytesBody, err := jsCommon.ToBytes(obj.Get(k).Export())
			if err != nil {
				return fopts, err
			}
			fopts.Body = bytesBody
		case "contentType":
			fopts.ContentType = obj.Get(k).String()
		case "headers":
			fopts.Headers = parseHeaders(obj.Get(k).ToObject(rt))
		case "status":
			fopts.Status = obj.Get(k).ToInteger()
		// As we don't support all fields that PW supports, we return an error to inform the user
		default:
			return fopts, fmt.Errorf("unsupported fulfill option: '%s'", k)
		}
	}

	return fopts, nil
}

func parseHeaders(headers *sobek.Object) []common.HTTPHeader {
	headersKeys := headers.Keys()
	result := make([]common.HTTPHeader, len(headersKeys))
	for i, hk := range headersKeys {
		result[i] = common.HTTPHeader{
			Name:  hk,
			Value: headers.Get(hk).String(),
		}
	}
	return result
}
