/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package http

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/dop251/goja"
	"gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/netext/httpext"
	"github.com/loadimpact/k6/lib/types"
)

// ErrHTTPForbiddenInInitContext is used when a http requests was made in the init context
var ErrHTTPForbiddenInInitContext = common.NewInitContextError("Making http requests in the init context is not supported")

// ErrBatchForbiddenInInitContext is used when batch was made in the init context
var ErrBatchForbiddenInInitContext = common.NewInitContextError("Using batch in the init context is not supported")

// Get makes an HTTP GET request and returns a corresponding response by taking goja.Values as arguments
func (h *HTTP) Get(ctx context.Context, url goja.Value, args ...goja.Value) (*Response, error) {
	// The body argument is always undefined for GETs and HEADs.
	args = append([]goja.Value{goja.Undefined()}, args...)
	return h.Request(ctx, HTTP_METHOD_GET, url, args...)
}

// Head makes an HTTP HEAD request and returns a corresponding response by taking goja.Values as arguments
func (h *HTTP) Head(ctx context.Context, url goja.Value, args ...goja.Value) (*Response, error) {
	// The body argument is always undefined for GETs and HEADs.
	args = append([]goja.Value{goja.Undefined()}, args...)
	return h.Request(ctx, HTTP_METHOD_HEAD, url, args...)
}

// Post makes an HTTP POST request and returns a corresponding response by taking goja.Values as arguments
func (h *HTTP) Post(ctx context.Context, url goja.Value, args ...goja.Value) (*Response, error) {
	return h.Request(ctx, HTTP_METHOD_POST, url, args...)
}

// Put makes an HTTP PUT request and returns a corresponding response by taking goja.Values as arguments
func (h *HTTP) Put(ctx context.Context, url goja.Value, args ...goja.Value) (*Response, error) {
	return h.Request(ctx, HTTP_METHOD_PUT, url, args...)
}

// Patch makes a patch request and returns a corresponding response by taking goja.Values as arguments
func (h *HTTP) Patch(ctx context.Context, url goja.Value, args ...goja.Value) (*Response, error) {
	return h.Request(ctx, HTTP_METHOD_PATCH, url, args...)
}

// Del makes an HTTP DELETE and returns a corresponding response by taking goja.Values as arguments
func (h *HTTP) Del(ctx context.Context, url goja.Value, args ...goja.Value) (*Response, error) {
	return h.Request(ctx, HTTP_METHOD_DELETE, url, args...)
}

// Options makes an HTTP OPTIONS request and returns a corresponding response by taking goja.Values as arguments
func (h *HTTP) Options(ctx context.Context, url goja.Value, args ...goja.Value) (*Response, error) {
	return h.Request(ctx, HTTP_METHOD_OPTIONS, url, args...)
}

// Request makes an http request of the provided `method` and returns a corresponding response by
// taking goja.Values as arguments
func (h *HTTP) Request(ctx context.Context, method string, url goja.Value, args ...goja.Value) (*Response, error) {
	u, err := ToURL(url)
	if err != nil {
		return nil, err
	}

	var body interface{}
	var params goja.Value

	if len(args) > 0 {
		body = args[0].Export()
	}
	if len(args) > 1 {
		params = args[1]
	}

	req, err := h.parseRequest(ctx, method, u, body, params)
	if err != nil {
		return nil, err
	}

	resp, err := httpext.MakeRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	return responseFromHttpext(resp), nil
}

//TODO break this function up
//nolint: gocyclo
func (h *HTTP) parseRequest(
	ctx context.Context, method string, reqURL httpext.URL, body interface{}, params goja.Value,
) (*httpext.ParsedHTTPRequest, error) {
	rt := common.GetRuntime(ctx)
	state := lib.GetState(ctx)
	if state == nil {
		return nil, ErrHTTPForbiddenInInitContext
	}

	result := &httpext.ParsedHTTPRequest{
		URL: &reqURL,
		Req: &http.Request{
			Method: method,
			URL:    reqURL.GetURL(),
			Header: make(http.Header),
		},
		Timeout:   60 * time.Second,
		Throw:     state.Options.Throw.Bool,
		Redirects: state.Options.MaxRedirects,
		Cookies:   make(map[string]*httpext.HTTPRequestCookie),
		Tags:      make(map[string]string),
	}
	if state.Options.DiscardResponseBodies.Bool {
		result.ResponseType = httpext.ResponseTypeNone
	} else {
		result.ResponseType = httpext.ResponseTypeText
	}

	formatFormVal := func(v interface{}) string {
		// TODO: handle/warn about unsupported/nested values
		return fmt.Sprintf("%v", v)
	}

	handleObjectBody := func(data map[string]interface{}) error {
		if !requestContainsFile(data) {
			bodyQuery := make(url.Values, len(data))
			for k, v := range data {
				bodyQuery.Set(k, formatFormVal(v))
			}
			result.Body = bytes.NewBufferString(bodyQuery.Encode())
			result.Req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			return nil
		}

		// handling multipart request
		result.Body = &bytes.Buffer{}
		mpw := multipart.NewWriter(result.Body)

		// For parameters of type common.FileData, created with open(file, "b"),
		// we write the file boundary to the body buffer.
		// Otherwise parameters are treated as standard form field.
		for k, v := range data {
			switch ve := v.(type) {
			case FileData:
				// writing our own part to handle receiving
				// different content-type than the default application/octet-stream
				h := make(textproto.MIMEHeader)
				escapedFilename := escapeQuotes(ve.Filename)
				h.Set("Content-Disposition",
					fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
						k, escapedFilename))
				h.Set("Content-Type", ve.ContentType)

				// this writer will be closed either by the next part or
				// the call to mpw.Close()
				fw, err := mpw.CreatePart(h)
				if err != nil {
					return err
				}

				if _, err := fw.Write(ve.Data); err != nil {
					return err
				}
			default:
				fw, err := mpw.CreateFormField(k)
				if err != nil {
					return err
				}

				if _, err := fw.Write([]byte(formatFormVal(v))); err != nil {
					return err
				}
			}
		}

		if err := mpw.Close(); err != nil {
			return err
		}

		result.Req.Header.Set("Content-Type", mpw.FormDataContentType())
		return nil
	}

	if body != nil {
		switch data := body.(type) {
		case map[string]goja.Value:
			// TODO: fix forms submission and serialization in k6/html before fixing this..
			newData := map[string]interface{}{}
			for k, v := range data {
				newData[k] = v.Export()
			}
			if err := handleObjectBody(newData); err != nil {
				return nil, err
			}
		case map[string]interface{}:
			if err := handleObjectBody(data); err != nil {
				return nil, err
			}
		case string:
			result.Body = bytes.NewBufferString(data)
		case []byte:
			result.Body = bytes.NewBuffer(data)
		default:
			return nil, fmt.Errorf("unknown request body type %T", body)
		}
	}

	result.Req.Header.Set("User-Agent", state.Options.UserAgent.String)

	if state.CookieJar != nil {
		result.ActiveJar = state.CookieJar
	}

	// TODO: ditch goja.Value, reflections and Object and use a simple go map and type assertions?
	if params != nil && !goja.IsUndefined(params) && !goja.IsNull(params) {
		params := params.ToObject(rt)
		for _, k := range params.Keys() {
			switch k {
			case "cookies":
				cookiesV := params.Get(k)
				if goja.IsUndefined(cookiesV) || goja.IsNull(cookiesV) {
					continue
				}
				cookies := cookiesV.ToObject(rt)
				if cookies == nil {
					continue
				}
				for _, key := range cookies.Keys() {
					cookieV := cookies.Get(key)
					if goja.IsUndefined(cookieV) || goja.IsNull(cookieV) {
						continue
					}
					switch cookieV.ExportType() {
					case reflect.TypeOf(map[string]interface{}{}):
						result.Cookies[key] = &httpext.HTTPRequestCookie{Name: key, Value: "", Replace: false}
						cookie := cookieV.ToObject(rt)
						for _, attr := range cookie.Keys() {
							switch strings.ToLower(attr) {
							case "replace":
								result.Cookies[key].Replace = cookie.Get(attr).ToBoolean()
							case "value":
								result.Cookies[key].Value = cookie.Get(attr).String()
							}
						}
					default:
						result.Cookies[key] = &httpext.HTTPRequestCookie{Name: key, Value: cookieV.String(), Replace: false}
					}
				}
			case "headers":
				headersV := params.Get(k)
				if goja.IsUndefined(headersV) || goja.IsNull(headersV) {
					continue
				}
				headers := headersV.ToObject(rt)
				if headers == nil {
					continue
				}
				for _, key := range headers.Keys() {
					str := headers.Get(key).String()
					if strings.ToLower(key) == "host" {
						result.Req.Host = str
					}
					result.Req.Header.Set(key, str)
				}
			case "jar":
				jarV := params.Get(k)
				if goja.IsUndefined(jarV) || goja.IsNull(jarV) {
					continue
				}
				switch v := jarV.Export().(type) {
				case *HTTPCookieJar:
					result.ActiveJar = v.jar
				}
			case "compression":
				algosString := strings.TrimSpace(params.Get(k).ToString().String())
				if algosString == "" {
					continue
				}
				algos := strings.Split(algosString, ",")
				var err error
				result.Compressions = make([]httpext.CompressionType, len(algos))
				for index, algo := range algos {
					algo = strings.TrimSpace(algo)
					result.Compressions[index], err = httpext.CompressionTypeString(algo)
					if err != nil {
						return nil, fmt.Errorf("unknown compression algorithm %s, supported algorithms are %s",
							algo, httpext.CompressionTypeValues())
					}
				}
			case "redirects":
				result.Redirects = null.IntFrom(params.Get(k).ToInteger())
			case "tags":
				tagsV := params.Get(k)
				if goja.IsUndefined(tagsV) || goja.IsNull(tagsV) {
					continue
				}
				tagObj := tagsV.ToObject(rt)
				if tagObj == nil {
					continue
				}
				for _, key := range tagObj.Keys() {
					result.Tags[key] = tagObj.Get(key).String()
				}
			case "auth":
				result.Auth = params.Get(k).String()
			case "timeout":
				t, err := types.GetDurationValue(params.Get(k).Export())
				if err != nil {
					return nil, fmt.Errorf("invalid timeout value: %w", err)
				}
				result.Timeout = t
			case "throw":
				result.Throw = params.Get(k).ToBoolean()
			case "responseType":
				responseType, err := httpext.ResponseTypeString(params.Get(k).String())
				if err != nil {
					return nil, err
				}
				result.ResponseType = responseType
			}
		}
	}

	if result.ActiveJar != nil {
		httpext.SetRequestCookies(result.Req, result.ActiveJar, result.Cookies)
	}

	return result, nil
}

func (h *HTTP) prepareBatchArray(
	ctx context.Context, requests []interface{},
) ([]httpext.BatchParsedHTTPRequest, []*Response, error) {
	reqCount := len(requests)
	batchReqs := make([]httpext.BatchParsedHTTPRequest, reqCount)
	results := make([]*Response, reqCount)

	for i, req := range requests {
		parsedReq, err := h.parseBatchRequest(ctx, i, req)
		if err != nil {
			return nil, nil, err
		}
		response := new(httpext.Response)
		batchReqs[i] = httpext.BatchParsedHTTPRequest{
			ParsedHTTPRequest: parsedReq,
			Response:          response,
		}
		results[i] = &Response{response}
	}

	return batchReqs, results, nil
}

func (h *HTTP) prepareBatchObject(
	ctx context.Context, requests map[string]interface{},
) ([]httpext.BatchParsedHTTPRequest, map[string]*Response, error) {
	reqCount := len(requests)
	batchReqs := make([]httpext.BatchParsedHTTPRequest, reqCount)
	results := make(map[string]*Response, reqCount)

	i := 0
	for key, req := range requests {
		parsedReq, err := h.parseBatchRequest(ctx, key, req)
		if err != nil {
			return nil, nil, err
		}
		response := new(httpext.Response)
		batchReqs[i] = httpext.BatchParsedHTTPRequest{
			ParsedHTTPRequest: parsedReq,
			Response:          response,
		}
		results[key] = &Response{response}
		i++
	}

	return batchReqs, results, nil
}

// Batch makes multiple simultaneous HTTP requests. The provideds reqsV should be an array of request
// objects. Batch returns an array of responses and/or error
func (h *HTTP) Batch(ctx context.Context, reqsV goja.Value) (goja.Value, error) {
	state := lib.GetState(ctx)
	if state == nil {
		return nil, ErrBatchForbiddenInInitContext
	}

	var (
		err       error
		batchReqs []httpext.BatchParsedHTTPRequest
		results   interface{} // either []*Response or map[string]*Response
	)

	switch v := reqsV.Export().(type) {
	case []interface{}:
		batchReqs, results, err = h.prepareBatchArray(ctx, v)
	case map[string]interface{}:
		batchReqs, results, err = h.prepareBatchObject(ctx, v)
	default:
		return nil, fmt.Errorf("invalid http.batch() argument type %T", v)
	}

	if err != nil {
		return nil, err
	}

	reqCount := len(batchReqs)
	errs := httpext.MakeBatchRequests(
		ctx, batchReqs, reqCount,
		int(state.Options.Batch.Int64), int(state.Options.BatchPerHost.Int64),
	)

	for i := 0; i < reqCount; i++ {
		if e := <-errs; e != nil && err == nil { // Save only the first error
			err = e
		}
	}
	return common.GetRuntime(ctx).ToValue(results), err
}

func (h *HTTP) parseBatchRequest(
	ctx context.Context, key interface{}, val interface{},
) (*httpext.ParsedHTTPRequest, error) {
	var (
		method = HTTP_METHOD_GET
		ok     bool
		err    error
		reqURL httpext.URL
		body   interface{}
		params goja.Value
		rt     = common.GetRuntime(ctx)
	)

	switch data := val.(type) {
	case []interface{}:
		// Handling of ["GET", "http://example.com/"]
		dataLen := len(data)
		if dataLen < 2 {
			return nil, fmt.Errorf("invalid batch request '%#v'", data)
		}
		method, ok = data[0].(string)
		if !ok {
			return nil, fmt.Errorf("invalid method type '%#v'", data[0])
		}
		reqURL, err = ToURL(data[1])
		if err != nil {
			return nil, err
		}
		if dataLen > 2 {
			body = data[2]
		}
		if dataLen > 3 {
			params = rt.ToValue(data[3])
		}

	case map[string]interface{}:
		// Handling of {method: "GET", url: "https://test.k6.io"}
		if murl, ok := data["url"]; !ok {
			return nil, fmt.Errorf("batch request %q doesn't have an url key", key)
		} else if reqURL, err = ToURL(murl); err != nil {
			return nil, err
		}

		body = data["body"] // It's fine if it's missing, the map lookup will return

		if newMethod, ok := data["method"]; ok {
			if method, ok = newMethod.(string); !ok {
				return nil, fmt.Errorf("invalid method type '%#v'", newMethod)
			}
			method = strings.ToUpper(method)
			if method == HTTP_METHOD_GET || method == HTTP_METHOD_HEAD {
				body = nil
			}
		}

		if p, ok := data["params"]; ok {
			params = rt.ToValue(p)
		}

	default:
		// Handling of "http://example.com/" or http.url`http://example.com/{$id}`
		reqURL, err = ToURL(data)
		if err != nil {
			return nil, err
		}
	}

	return h.parseRequest(ctx, method, reqURL, body, params)
}

func requestContainsFile(data map[string]interface{}) bool {
	for _, v := range data {
		switch v.(type) {
		case FileData:
			return true
		}
	}
	return false
}
