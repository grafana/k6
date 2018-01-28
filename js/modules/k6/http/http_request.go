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
	// "fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/cookiejar"
	neturl "net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"
	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/lib/netext"
	"github.com/loadimpact/k6/stats"
	log "github.com/sirupsen/logrus"
	null "gopkg.in/guregu/null.v3"
)

type HTTPRequest struct {
	Method  string
	URL     string
	Headers map[string][]string
	Body    io.Closer
	Cookies map[string][]*HTTPRequestCookie
}

func (http *HTTP) Get(ctx context.Context, url goja.Value, args ...goja.Value) (*HTTPResponse, error) {
	// The body argument is always undefined for GETs and HEADs.
	args = append([]goja.Value{goja.Undefined()}, args...)
	return http.Request(ctx, HTTP_METHOD_GET, url, args...)
}

func (http *HTTP) Head(ctx context.Context, url goja.Value, args ...goja.Value) (*HTTPResponse, error) {
	// The body argument is always undefined for GETs and HEADs.
	args = append([]goja.Value{goja.Undefined()}, args...)
	return http.Request(ctx, HTTP_METHOD_HEAD, url, args...)
}

func (http *HTTP) Post(ctx context.Context, url goja.Value, args ...goja.Value) (*HTTPResponse, error) {
	return http.Request(ctx, HTTP_METHOD_POST, url, args...)
}

func (http *HTTP) Put(ctx context.Context, url goja.Value, args ...goja.Value) (*HTTPResponse, error) {
	return http.Request(ctx, HTTP_METHOD_PUT, url, args...)
}

func (http *HTTP) Patch(ctx context.Context, url goja.Value, args ...goja.Value) (*HTTPResponse, error) {
	return http.Request(ctx, HTTP_METHOD_PATCH, url, args...)
}

func (http *HTTP) Del(ctx context.Context, url goja.Value, args ...goja.Value) (*HTTPResponse, error) {
	return http.Request(ctx, HTTP_METHOD_DELETE, url, args...)
}

func (http *HTTP) Options(ctx context.Context, url goja.Value, args ...goja.Value) (*HTTPResponse, error) {
	return http.Request(ctx, HTTP_METHOD_OPTIONS, url, args...)
}

func (http *HTTP) Request(ctx context.Context, method string, url goja.Value, args ...goja.Value) (*HTTPResponse, error) {
	rt := common.GetRuntime(ctx)
	state := common.GetState(ctx)

	u, err := ToURL(url)
	if err != nil {
		return nil, err
	}
	res, samples, err := http.request(ctx, rt, state, method, u, args...)
	state.Samples = append(state.Samples, samples...)
	return res, err
}

func (h *HTTP) request(ctx context.Context, rt *goja.Runtime, state *common.State, method string, url URL, args ...goja.Value) (*HTTPResponse, []stats.Sample, error) {
	var bodyBuf *bytes.Buffer
	var contentType string
	if len(args) > 0 && !goja.IsUndefined(args[0]) && !goja.IsNull(args[0]) {
		var data map[string]goja.Value
		if rt.ExportTo(args[0], &data) == nil {
			bodyQuery := make(neturl.Values, len(data))
			for k, v := range data {
				if v != goja.Undefined() {
					bodyQuery.Set(k, v.String())
				}
			}
			bodyBuf = bytes.NewBufferString(bodyQuery.Encode())
			contentType = "application/x-www-form-urlencoded"
		} else {
			bodyBuf = bytes.NewBufferString(args[0].String())
		}
	}

	req := &http.Request{
		Method: method,
		URL:    url.URL,
		Header: make(http.Header),
	}
	respReq := &HTTPRequest{
		Method: req.Method,
		URL:    req.URL.String(),
	}
	if bodyBuf != nil {
		req.Body = ioutil.NopCloser(bodyBuf)
		req.ContentLength = int64(bodyBuf.Len())
		respReq.Body = req.Body
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if userAgent := state.Options.UserAgent; userAgent.String != "" {
		req.Header.Set("User-Agent", userAgent.String)
	}

	tags := map[string]string{
		"proto":  "",
		"status": "0",
		"method": method,
		"url":    url.URLString,
		"name":   url.Name,
		"group":  state.Group.Path,
		"vu":     strconv.FormatInt(state.Vu, 10),
		"iter":   strconv.FormatInt(state.Iteration, 10),
	}
	redirects := state.Options.MaxRedirects
	timeout := 60 * time.Second
	throw := state.Options.Throw.Bool

	var activeJar *cookiejar.Jar
	if state.CookieJar != nil {
		activeJar = state.CookieJar
	}
	reqCookies := make(map[string]*HTTPRequestCookie)

	if len(args) > 1 {
		paramsV := args[1]
		if !goja.IsUndefined(paramsV) && !goja.IsNull(paramsV) {
			params := paramsV.ToObject(rt)
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
						case typeMapKeyStringValueInterface:
							reqCookies[key] = &HTTPRequestCookie{Name: key, Value: "", Replace: false}
							cookie := cookieV.ToObject(rt)
							for _, attr := range cookie.Keys() {
								switch strings.ToLower(attr) {
								case "replace":
									reqCookies[key].Replace = cookie.Get(attr).ToBoolean()
								case "value":
									reqCookies[key].Value = cookie.Get(attr).String()
								}
							}
						default:
							reqCookies[key] = &HTTPRequestCookie{Name: key, Value: cookieV.String(), Replace: false}
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
							req.Host = str
						default:
							req.Header.Set(key, str)
						}
					}
				case "jar":
					jarV := params.Get(k)
					if goja.IsUndefined(jarV) || goja.IsNull(jarV) {
						continue
					}
					switch v := jarV.Export().(type) {
					case *HTTPCookieJar:
						activeJar = v.jar
					}
				case "redirects":
					redirects = null.IntFrom(params.Get(k).ToInteger())
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
						tags[key] = tagObj.Get(key).String()
					}
				case "timeout":
					timeout = time.Duration(params.Get(k).ToFloat() * float64(time.Millisecond))
				case "throw":
					throw = params.Get(k).ToBoolean()
				}
			}
		}
	}

	if activeJar != nil {
		mergedCookies := h.mergeCookies(req, activeJar, reqCookies)
		respReq.Cookies = mergedCookies
		h.setRequestCookies(req, mergedCookies)
	}

	// Check rate limit *after* we've prepared a request; no need to wait with that part.
	if rpsLimit := state.RPSLimit; rpsLimit != nil {
		if err := rpsLimit.Wait(ctx); err != nil {
			return nil, nil, err
		}
	}

	respReq.Headers = req.Header

	resp := &HTTPResponse{ctx: ctx, URL: url.URLString, Request: *respReq}
	client := http.Client{
		Transport: state.HTTPTransport,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Update active jar with cookies found in "Set-Cookie" header(s) of redirect response
			if activeJar != nil {
				if respCookies := req.Response.Cookies(); len(respCookies) > 0 {
					activeJar.SetCookies(req.URL, respCookies)
				}
				req.Header.Del("Cookie")
				mergedCookies := h.mergeCookies(req, activeJar, reqCookies)

				h.setRequestCookies(req, mergedCookies)
			}

			if l := len(via); int64(l) > redirects.Int64 {
				if !redirects.Valid {
					url := req.URL
					if l > 0 {
						url = via[0].URL
					}
					state.Logger.WithFields(log.Fields{"url": url.String()}).Warnf("Stopped after %d redirects and returned the redirection; pass { redirects: n } in request params or set global maxRedirects to silence this", l)
				}
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	tracer := netext.Tracer{}
	res, resErr := client.Do(req.WithContext(netext.WithTracer(ctx, &tracer)))
	if resErr == nil && res != nil {
		switch res.Header.Get("Content-Encoding") {
		case "deflate":
			res.Body, resErr = zlib.NewReader(res.Body)
		case "gzip":
			res.Body, resErr = gzip.NewReader(res.Body)
		}
	}
	if resErr == nil && res != nil {
		buf := state.BPool.Get()
		buf.Reset()
		defer state.BPool.Put(buf)
		_, err := io.Copy(buf, res.Body)
		if err != nil && err != io.EOF {
			resErr = err
		}
		resp.Body = buf.String()
		_ = res.Body.Close()
	}
	trail := tracer.Done()
	if trail.ConnRemoteAddr != nil {
		remoteHost, remotePortStr, _ := net.SplitHostPort(trail.ConnRemoteAddr.String())
		remotePort, _ := strconv.Atoi(remotePortStr)
		resp.RemoteIP = remoteHost
		resp.RemotePort = remotePort
	}
	resp.Timings = HTTPResponseTimings{
		Duration:       stats.D(trail.Duration),
		Blocked:        stats.D(trail.Blocked),
		Connecting:     stats.D(trail.Connecting),
		TLSHandshaking: stats.D(trail.TLSHandshaking),
		Sending:        stats.D(trail.Sending),
		Waiting:        stats.D(trail.Waiting),
		Receiving:      stats.D(trail.Receiving),
	}

	if resErr != nil {
		resp.Error = resErr.Error()
		tags["error"] = resp.Error
	} else {
		if activeJar != nil {
			if rc := res.Cookies(); len(rc) > 0 {
				activeJar.SetCookies(res.Request.URL, rc)
			}
		}

		resp.URL = res.Request.URL.String()
		resp.Status = res.StatusCode
		resp.Proto = res.Proto
		tags["url"] = resp.URL
		tags["status"] = strconv.Itoa(resp.Status)
		tags["proto"] = resp.Proto

		if res.TLS != nil {
			resp.setTLSInfo(res.TLS)
			tags["tls_version"] = resp.TLSVersion
			tags["ocsp_status"] = resp.OCSP.Status
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

		if throw {
			return nil, nil, resErr
		}
	}
	return resp, trail.Samples(tags), nil
}

func (http *HTTP) Batch(ctx context.Context, reqsV goja.Value) (goja.Value, error) {
	rt := common.GetRuntime(ctx)
	state := common.GetState(ctx)

	// Return values; retval must be guarded by the mutex.
	var mutex sync.Mutex
	retval := rt.NewObject()
	errs := make(chan error)

	// Concurrency limits.
	globalLimiter := NewSlotLimiter(int(state.Options.Batch.Int64))
	perHostLimiter := NewMultiSlotLimiter(int(state.Options.BatchPerHost.Int64))

	reqs := reqsV.ToObject(rt)
	keys := reqs.Keys()
	for _, k := range keys {
		k := k
		v := reqs.Get(k)

		method := HTTP_METHOD_GET
		var url URL
		var args []goja.Value

		// Shorthand: "http://example.com/" -> ["GET", "http://example.com/"]
		switch v.ExportType() {
		case typeURL:
			url = v.Export().(URL)
		case typeString:
			u, err := ToURL(v)
			if err != nil {
				return goja.Undefined(), err
			}
			url = u
		default:
			obj := v.ToObject(rt)
			objkeys := obj.Keys()
			for _, objk := range objkeys {
				objv := obj.Get(objk)
				switch objk {
				case "0", "method":
					method = strings.ToUpper(objv.String())
					if method == HTTP_METHOD_GET || method == HTTP_METHOD_HEAD {
						args = []goja.Value{goja.Undefined()}
					}
				case "1", "url":
					u, err := ToURL(objv)
					if err != nil {
						return goja.Undefined(), err
					}
					url = u
				default:
					args = append(args, objv)
				}
			}
		}

		go func() {
			globalLimiter.Begin()
			defer globalLimiter.End()

			if hl := perHostLimiter.Slot(url.URL.Host); hl != nil {
				hl.Begin()
				defer hl.End()
			}

			res, samples, err := http.request(ctx, rt, state, method, url, args...)
			if err != nil {
				errs <- err
				return
			}

			mutex.Lock()
			_ = retval.Set(k, res)
			state.Samples = append(state.Samples, samples...)
			mutex.Unlock()

			errs <- nil
		}()
	}

	var err error
	for range keys {
		if e := <-errs; e != nil {
			err = e
		}
	}
	return retval, err
}
