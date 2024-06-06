package browser

import (
	"github.com/grafana/xk6-browser/common"
)

// syncMapResponse is like mapResponse but returns synchronous functions.
func syncMapResponse(vu moduleVU, r *common.Response) mapping {
	if r == nil {
		return nil
	}
	maps := mapping{
		"allHeaders":      r.AllHeaders,
		"body":            r.Body,
		"frame":           func() mapping { return syncMapFrame(vu, r.Frame()) },
		"headerValue":     r.HeaderValue,
		"headerValues":    r.HeaderValues,
		"headers":         r.Headers,
		"headersArray":    r.HeadersArray,
		"json":            r.JSON,
		"ok":              r.Ok,
		"request":         func() mapping { return syncMapRequest(vu, r.Request()) },
		"securityDetails": r.SecurityDetails,
		"serverAddr":      r.ServerAddr,
		"size":            r.Size,
		"status":          r.Status,
		"statusText":      r.StatusText,
		"url":             r.URL,
	}

	return maps
}
