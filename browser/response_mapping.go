package browser

import (
	"github.com/dop251/goja"

	"github.com/grafana/xk6-browser/common"
)

// mapResponse to the JS module.
func mapResponse(vu moduleVU, r *common.Response) mapping {
	if r == nil {
		return nil
	}
	rt := vu.Runtime()
	maps := mapping{
		"allHeaders": r.AllHeaders,
		"body":       r.Body,
		"frame": func() *goja.Object {
			mf := mapFrame(vu, r.Frame())
			return rt.ToValue(mf).ToObject(rt)
		},
		"headerValue":  r.HeaderValue,
		"headerValues": r.HeaderValues,
		"headers":      r.Headers,
		"headersArray": r.HeadersArray,
		"json":         r.JSON,
		"ok":           r.Ok,
		"request": func() *goja.Object {
			mr := mapRequest(vu, r.Request())
			return rt.ToValue(mr).ToObject(rt)
		},
		"securityDetails": r.SecurityDetails,
		"serverAddr":      r.ServerAddr,
		"size":            r.Size,
		"status":          r.Status,
		"statusText":      r.StatusText,
		"url":             r.URL,
	}

	return maps
}
