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
	"compress/gzip"
	"compress/zlib"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/textproto"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	ntlmssp "github.com/Azure/go-ntlmssp"
	digest "github.com/Soontao/goHttpDigestClient"
	"github.com/dop251/goja"
	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/lib/netext"
	"github.com/loadimpact/k6/stats"
	log "github.com/sirupsen/logrus"
	null "gopkg.in/guregu/null.v3"
)

// ErrHTTPForbiddenInInitContext is used when a http requests was made in the init context
var ErrHTTPForbiddenInInitContext = common.NewInitContextError("Making http requests in the init context is not supported")

// ErrBatchForbiddenInInitContext is used when batch was made in the init context
var ErrBatchForbiddenInInitContext = common.NewInitContextError("Using batch in the init context is not supported")

// Request represent an http request
type Request struct {
	Method  string                          `json:"method"`
	URL     string                          `json:"url"`
	Headers map[string][]string             `json:"headers"`
	Body    string                          `json:"body"`
	Cookies map[string][]*HTTPRequestCookie `json:"cookies"`
}

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

	return h.request(ctx, req)
}

// ResponseType is used in the request to specify how the response body should be treated
// The conversion and validation methods are auto-generated with https://github.com/alvaroloes/enumer:
//go:generate enumer -type=ResponseType -transform=snake -json -text -trimprefix ResponseType -output response_type_gen.go
type ResponseType uint

const (
	// ResponseTypeText causes k6 to return the response body as a string. It works
	// well for web pages and JSON documents, but it can cause issues with
	// binary files since their data could be lost when they're converted in the
	// UTF-16 strings JavaScript uses.
	// This is the default value for backwards-compatibility, unless the global
	// discardResponseBodies option is enabled.
	ResponseTypeText ResponseType = iota
	// ResponseTypeBinary causes k6 to return the response body as a []byte, suitable
	// for working with binary files without lost data and needless string conversions.
	ResponseTypeBinary
	// ResponseTypeNone causes k6 to fully read the response body while immediately
	// discarding the actual data - k6 would set the body of the returned HTTPResponse
	// to null. This saves CPU and memory and is suitable for HTTP requests that we just
	// want to  measure, but we don't care about their responses' contents. This is the
	// default value for all requests if the global discardResponseBodies is enablled.
	ResponseTypeNone
)

type parsedHTTPRequest struct {
	url           *URL
	body          *bytes.Buffer
	req           *http.Request
	timeout       time.Duration
	auth          string
	throw         bool
	responseType  ResponseType
	redirects     null.Int
	activeJar     *cookiejar.Jar
	cookies       map[string]*HTTPRequestCookie
	mergedCookies map[string][]*HTTPRequestCookie
	tags          map[string]string
}

func (h *HTTP) parseRequest(ctx context.Context, method string, reqURL URL, body interface{}, params goja.Value) (*parsedHTTPRequest, error) {
	rt := common.GetRuntime(ctx)
	state := common.GetState(ctx)
	if state == nil {
		return nil, ErrHTTPForbiddenInInitContext
	}

	result := &parsedHTTPRequest{
		url: &reqURL,
		req: &http.Request{
			Method: method,
			URL:    reqURL.URL,
			Header: make(http.Header),
		},
		timeout:   60 * time.Second,
		throw:     state.Options.Throw.Bool,
		redirects: state.Options.MaxRedirects,
		cookies:   make(map[string]*HTTPRequestCookie),
		tags:      make(map[string]string),
	}
	if state.Options.DiscardResponseBodies.Bool {
		result.responseType = ResponseTypeNone
	} else {
		result.responseType = ResponseTypeText
	}

	formatFormVal := func(v interface{}) string {
		//TODO: handle/warn about unsupported/nested values
		return fmt.Sprintf("%v", v)
	}

	handleObjectBody := func(data map[string]interface{}) error {
		if !requestContainsFile(data) {
			bodyQuery := make(url.Values, len(data))
			for k, v := range data {
				bodyQuery.Set(k, formatFormVal(v))
			}
			result.body = bytes.NewBufferString(bodyQuery.Encode())
			result.req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			return nil
		}

		// handling multipart request
		result.body = &bytes.Buffer{}
		mpw := multipart.NewWriter(result.body)

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

		result.req.Header.Set("Content-Type", mpw.FormDataContentType())
		return nil
	}

	if body != nil {
		switch data := body.(type) {
		case map[string]goja.Value:
			//TODO: fix forms submission and serialization in k6/html before fixing this..
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
			result.body = bytes.NewBufferString(data)
		case []byte:
			result.body = bytes.NewBuffer(data)
		default:
			return nil, fmt.Errorf("unknown request body type %T", body)
		}
	}

	if result.body != nil {
		result.req.Body = ioutil.NopCloser(result.body)
		result.req.ContentLength = int64(result.body.Len())
	}

	if userAgent := state.Options.UserAgent; userAgent.String != "" {
		result.req.Header.Set("User-Agent", userAgent.String)
	}

	if state.CookieJar != nil {
		result.activeJar = state.CookieJar
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
						result.cookies[key] = &HTTPRequestCookie{Name: key, Value: "", Replace: false}
						cookie := cookieV.ToObject(rt)
						for _, attr := range cookie.Keys() {
							switch strings.ToLower(attr) {
							case "replace":
								result.cookies[key].Replace = cookie.Get(attr).ToBoolean()
							case "value":
								result.cookies[key].Value = cookie.Get(attr).String()
							}
						}
					default:
						result.cookies[key] = &HTTPRequestCookie{Name: key, Value: cookieV.String(), Replace: false}
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
					switch strings.ToLower(key) {
					case "host":
						result.req.Host = str
					default:
						result.req.Header.Set(key, str)
					}
				}
			case "jar":
				jarV := params.Get(k)
				if goja.IsUndefined(jarV) || goja.IsNull(jarV) {
					continue
				}
				switch v := jarV.Export().(type) {
				case *HTTPCookieJar:
					result.activeJar = v.jar
				}
			case "redirects":
				result.redirects = null.IntFrom(params.Get(k).ToInteger())
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
					result.tags[key] = tagObj.Get(key).String()
				}
			case "auth":
				result.auth = params.Get(k).String()
			case "timeout":
				result.timeout = time.Duration(params.Get(k).ToFloat() * float64(time.Millisecond))
			case "throw":
				result.throw = params.Get(k).ToBoolean()
			case "responseType":
				responseType, err := ResponseTypeString(params.Get(k).String())
				if err != nil {
					return nil, err
				}
				result.responseType = responseType
			}
		}
	}

	if result.activeJar != nil {
		result.mergedCookies = h.mergeCookies(result.req, result.activeJar, result.cookies)
		h.setRequestCookies(result.req, result.mergedCookies)
	}

	return result, nil
}

// request() shouldn't mess with the goja runtime or other thread-unsafe
// things because it's called concurrently by Batch()
func (h *HTTP) request(ctx context.Context, preq *parsedHTTPRequest) (*Response, error) {
	state := common.GetState(ctx)

	respReq := &Request{
		Method:  preq.req.Method,
		URL:     preq.req.URL.String(),
		Cookies: preq.mergedCookies,
		Headers: preq.req.Header,
	}
	if preq.body != nil {
		respReq.Body = preq.body.String()
	}

	tags := state.Options.RunTags.CloneTags()
	for k, v := range preq.tags {
		tags[k] = v
	}

	if state.Options.SystemTags["method"] {
		tags["method"] = preq.req.Method
	}
	if state.Options.SystemTags["url"] {
		tags["url"] = preq.url.URLString
	}

	// Only set the name system tag if the user didn't explicitly set it beforehand
	if _, ok := tags["name"]; !ok && state.Options.SystemTags["name"] {
		tags["name"] = preq.url.Name
	}
	if state.Options.SystemTags["group"] {
		tags["group"] = state.Group.Path
	}
	if state.Options.SystemTags["vu"] {
		tags["vu"] = strconv.FormatInt(state.Vu, 10)
	}
	if state.Options.SystemTags["iter"] {
		tags["iter"] = strconv.FormatInt(state.Iteration, 10)
	}

	// Check rate limit *after* we've prepared a request; no need to wait with that part.
	if rpsLimit := state.RPSLimit; rpsLimit != nil {
		if err := rpsLimit.Wait(ctx); err != nil {
			return nil, err
		}
	}

	tracerTransport := netext.NewTransport(state.Transport, state.Samples, &state.Options, tags)
	var transport http.RoundTripper = tracerTransport
	if preq.auth == "ntlm" {
		transport = ntlmssp.Negotiator{
			RoundTripper: tracerTransport,
		}
	}

	resp := &Response{ctx: ctx, URL: preq.url.URLString, Request: *respReq}
	client := http.Client{
		Transport: transport,
		Timeout:   preq.timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			h.debugResponse(state, req.Response, "RedirectResponse")

			// Update active jar with cookies found in "Set-Cookie" header(s) of redirect response
			if preq.activeJar != nil {
				if respCookies := req.Response.Cookies(); len(respCookies) > 0 {
					preq.activeJar.SetCookies(req.URL, respCookies)
				}
				req.Header.Del("Cookie")
				mergedCookies := h.mergeCookies(req, preq.activeJar, preq.cookies)

				h.setRequestCookies(req, mergedCookies)
			}

			if l := len(via); int64(l) > preq.redirects.Int64 {
				if !preq.redirects.Valid {
					url := req.URL
					if l > 0 {
						url = via[0].URL
					}
					state.Logger.WithFields(log.Fields{"url": url.String()}).Warnf("Stopped after %d redirects and returned the redirection; pass { redirects: n } in request params or set global maxRedirects to silence this", l)
				}
				return http.ErrUseLastResponse
			}
			h.debugRequest(state, req, "RedirectRequest")
			return nil
		},
	}

	// if digest authentication option is passed, make an initial request to get the authentication params to compute the authorization header
	if preq.auth == "digest" {
		username := preq.url.URL.User.Username()
		password, _ := preq.url.URL.User.Password()

		// removing user from URL to avoid sending the authorization header fo basic auth
		preq.req.URL.User = nil

		h.debugRequest(state, preq.req, "DigestRequest")
		res, err := client.Do(preq.req.WithContext(ctx))
		h.debugRequest(state, preq.req, "DigestResponse")
		if err != nil {
			// Do *not* log errors about the contex being cancelled.
			select {
			case <-ctx.Done():
			default:
				state.Logger.WithField("error", res).Warn("Digest request failed")
			}

			if preq.throw {
				return nil, err
			}

			resp.setError(err)
			return resp, nil
		}

		if res.StatusCode == http.StatusUnauthorized {
			body := ""
			if b, err := ioutil.ReadAll(res.Body); err == nil {
				body = string(b)
			}

			challenge := digest.GetChallengeFromHeader(&res.Header)
			challenge.ComputeResponse(preq.req.Method, preq.req.URL.RequestURI(), body, username, password)
			authorization := challenge.ToAuthorizationStr()
			preq.req.Header.Set(digest.KEY_AUTHORIZATION, authorization)
		}
	}

	h.debugRequest(state, preq.req, "Request")
	res, resErr := client.Do(preq.req.WithContext(ctx))
	h.debugResponse(state, res, "Response")
	if resErr == nil && res != nil {
		switch res.Header.Get("Content-Encoding") {
		case "deflate":
			res.Body, resErr = zlib.NewReader(res.Body)
		case "gzip":
			res.Body, resErr = gzip.NewReader(res.Body)
		}
	}
	if resErr == nil && res != nil {
		if preq.responseType == ResponseTypeNone {
			_, err := io.Copy(ioutil.Discard, res.Body)
			if err != nil && err != io.EOF {
				resErr = err
			}
			resp.Body = nil
		} else {
			// Binary or string
			buf := state.BPool.Get()
			buf.Reset()
			defer state.BPool.Put(buf)
			_, err := io.Copy(buf, res.Body)
			if err != nil && err != io.EOF {
				resErr = err
			}

			switch preq.responseType {
			case ResponseTypeText:
				resp.Body = buf.String()
			case ResponseTypeBinary:
				resp.Body = buf.Bytes()
			default:
				resErr = fmt.Errorf("unknown responseType %s", preq.responseType)
			}
		}
		_ = res.Body.Close()
	}

	trail := tracerTransport.GetTrail()

	if trail.ConnRemoteAddr != nil {
		remoteHost, remotePortStr, _ := net.SplitHostPort(trail.ConnRemoteAddr.String())
		remotePort, _ := strconv.Atoi(remotePortStr)
		resp.RemoteIP = remoteHost
		resp.RemotePort = remotePort
	}
	resp.Timings = ResponseTimings{
		Duration:       stats.D(trail.Duration),
		Blocked:        stats.D(trail.Blocked),
		Connecting:     stats.D(trail.Connecting),
		TLSHandshaking: stats.D(trail.TLSHandshaking),
		Sending:        stats.D(trail.Sending),
		Waiting:        stats.D(trail.Waiting),
		Receiving:      stats.D(trail.Receiving),
	}

	if resErr != nil {
		resp.setError(resErr)
	} else {
		if preq.activeJar != nil {
			if rc := res.Cookies(); len(rc) > 0 {
				preq.activeJar.SetCookies(res.Request.URL, rc)
			}
		}

		resp.URL = res.Request.URL.String()
		resp.setStatusCode(res.StatusCode)
		resp.Proto = res.Proto

		if res.TLS != nil {
			resp.setTLSInfo(res.TLS)
		}

		resp.Headers = make(map[string]string, len(res.Header))
		for k, vs := range res.Header {
			resp.Headers[k] = strings.Join(vs, ", ")
		}

		resCookies := res.Cookies()
		resp.Cookies = make(map[string][]*HTTPCookie, len(resCookies))
		for _, c := range resCookies {
			resp.Cookies[c.Name] = append(resp.Cookies[c.Name], &HTTPCookie{
				Name:     c.Name,
				Value:    c.Value,
				Domain:   c.Domain,
				Path:     c.Path,
				HttpOnly: c.HttpOnly,
				Secure:   c.Secure,
				MaxAge:   c.MaxAge,
				Expires:  c.Expires.UnixNano() / 1000000,
			})
		}
	}

	if resErr != nil {
		// Do *not* log errors about the contex being cancelled.
		select {
		case <-ctx.Done():
		default:
			state.Logger.WithField("error", resErr).Warn("Request Failed")
		}

		if preq.throw {
			return nil, resErr
		}
	}

	return resp, nil
}

// Batch makes multiple simultaneous HTTP requests. The provideds reqsV should be an array of request
// objects. Batch returns an array of responses and/or error
func (h *HTTP) Batch(ctx context.Context, reqsV goja.Value) (goja.Value, error) {
	state := common.GetState(ctx)
	if state == nil {
		return nil, ErrBatchForbiddenInInitContext
	}
	rt := common.GetRuntime(ctx)

	reqs := reqsV.ToObject(rt)
	keys := reqs.Keys()
	parsedReqs := map[string]*parsedHTTPRequest{}
	for _, key := range keys {
		parsedReq, err := h.parseBatchRequest(ctx, key, reqs.Get(key))
		if err != nil {
			return nil, err
		}
		parsedReqs[key] = parsedReq
	}

	var (
		// Return values; retval must be guarded by the mutex.
		mutex  sync.Mutex
		retval = rt.NewObject()
		errs   = make(chan error)

		// Concurrency limits.
		globalLimiter  = NewSlotLimiter(int(state.Options.Batch.Int64))
		perHostLimiter = NewMultiSlotLimiter(int(state.Options.BatchPerHost.Int64))
	)
	for k, pr := range parsedReqs {
		go func(key string, parsedReq *parsedHTTPRequest) {
			globalLimiter.Begin()
			defer globalLimiter.End()

			if hl := perHostLimiter.Slot(parsedReq.url.URL.Host); hl != nil {
				hl.Begin()
				defer hl.End()
			}

			res, err := h.request(ctx, parsedReq)
			if err != nil {
				errs <- err
				return
			}

			mutex.Lock()
			_ = retval.Set(key, res)
			mutex.Unlock()

			errs <- nil
		}(k, pr)
	}

	var err error
	for range keys {
		if e := <-errs; e != nil {
			err = e
		}
	}
	return retval, err
}

func (h *HTTP) parseBatchRequest(ctx context.Context, key string, val goja.Value) (*parsedHTTPRequest, error) {
	var (
		method = HTTP_METHOD_GET
		ok     bool
		err    error
		reqURL URL
		body   interface{}
		params goja.Value
		rt     = common.GetRuntime(ctx)
	)

	switch data := val.Export().(type) {
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
		// Handling of {method: "GET", url: "http://test.loadimpact.com"}
		if murl, ok := data["url"]; !ok {
			return nil, fmt.Errorf("batch request %s doesn't have an url key", key)
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
