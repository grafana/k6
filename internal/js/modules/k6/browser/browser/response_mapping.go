package browser

import (
	"github.com/grafana/sobek"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/common"
)

func mapResponseEvent(vu moduleVU, event common.PageEvent) mapping {
	return mapResponse(vu, event.Response)
}

// mapResponse to the JS module.
//
//nolint:funlen
func mapResponse(vu moduleVU, r *common.Response) mapping {
	if r == nil {
		return nil
	}
	maps := mapping{
		"allHeaders": passiveCall(func() *sobek.Promise {
			return promise(vu, func() (any, error) {
				r.WaitForRawHeaders()
				return r.AllHeaders(), nil
			})
		}),
		"body": passiveCall(func() *sobek.Promise {
			rt := vu.Runtime()
			promise, res, rej := rt.NewPromise()
			callback := vu.RegisterCallback()
			go func() {
				body, err := r.Body()
				if err != nil {
					callback(func() error {
						return rej(err)
					})
					return
				}
				callback(func() error {
					buf := vu.Runtime().NewArrayBuffer(body)
					return res(&buf)
				})
			}()
			return promise
		}),
		"frame": passiveCall(func() mapping {
			return mapFrame(vu, r.Frame())
		}),
		"headerValue": passiveCall(func(name string) *sobek.Promise {
			return promise(vu, func() (any, error) {
				r.WaitForRawHeaders()
				v, ok := r.HeaderValue(name)
				if !ok {
					return nil, nil
				}
				return v, nil
			})
		}),
		"headerValues": passiveCall(func(name string) *sobek.Promise {
			return promise(vu, func() (any, error) {
				r.WaitForRawHeaders()
				return r.HeaderValues(name), nil
			})
		}),
		"headers": passiveCall(r.Headers),
		"headersArray": passiveCall(func() *sobek.Promise {
			return promise(vu, func() (any, error) {
				r.WaitForRawHeaders()
				return r.HeadersArray(), nil
			})
		}),
		"json": passiveCall(func() *sobek.Promise {
			return promise(vu, func() (any, error) {
				return r.JSON() //nolint: wrapcheck
			})
		}),
		"ok": passiveCall(r.Ok),
		"request": passiveCall(func() mapping {
			return mapRequest(vu, r.Request())
		}),
		"securityDetails": passiveCall(func() *sobek.Promise {
			return promise(vu, func() (any, error) {
				return r.SecurityDetails(), nil
			})
		}),
		"serverAddr": passiveCall(func() *sobek.Promise {
			return promise(vu, func() (any, error) {
				return r.ServerAddr(), nil
			})
		}),
		"size": passiveCall(func() *sobek.Promise {
			return promise(vu, func() (any, error) {
				return r.Size(), nil
			})
		}),
		"status":     passiveCall(r.Status),
		"statusText": passiveCall(r.StatusText),
		"url":        passiveCall(r.URL),
		"text": passiveCall(func() *sobek.Promise {
			return promise(vu, func() (any, error) {
				return r.Text() //nolint:wrapcheck
			})
		}),
	}

	return finishMapping(maps)
}
