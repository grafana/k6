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
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	neturl "net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"
	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/lib/netext"
	"github.com/loadimpact/k6/stats"
	log "github.com/sirupsen/logrus"
)

var (
	typeString                     = reflect.TypeOf("")
	typeURLTag                     = reflect.TypeOf(URLTag{})
	typeMapKeyStringValueInterface = reflect.TypeOf(map[string]interface{}{})
)

const SSL_3_0 = "ssl3.0"
const TLS_1_0 = "tls1.0"
const TLS_1_1 = "tls1.1"
const TLS_1_2 = "tls1.2"
const OCSP_STATUS_GOOD = "good"
const OCSP_STATUS_REVOKED = "revoked"
const OCSP_STATUS_SERVER_FAILED = "server_failed"
const OCSP_STATUS_UNKNOWN = "unknown"
const OCSP_REASON_UNSPECIFIED = "unspecified"
const OCSP_REASON_KEY_COMPROMISE = "key_compromise"
const OCSP_REASON_CA_COMPROMISE = "ca_compromise"
const OCSP_REASON_AFFILIATION_CHANGED = "affiliation_changed"
const OCSP_REASON_SUPERSEDED = "superseded"
const OCSP_REASON_CESSATION_OF_OPERATION = "cessation_of_operation"
const OCSP_REASON_CERTIFICATE_HOLD = "certificate_hold"
const OCSP_REASON_REMOVE_FROM_CRL = "remove_from_crl"
const OCSP_REASON_PRIVILEGE_WITHDRAWN = "privilege_withdrawn"
const OCSP_REASON_AA_COMPROMISE = "aa_compromise"

type HTTPCookie struct {
	Name, Value, Domain, Path string
	HttpOnly, Secure          bool
	MaxAge                    int
	Expires                   int64
}

type HTTPRequestCookie struct {
	Name, Value string
	Replace     bool
}

type HTTP struct {
	SSL_3_0                            string `js:"SSL_3_0"`
	TLS_1_0                            string `js:"TLS_1_0"`
	TLS_1_1                            string `js:"TLS_1_1"`
	TLS_1_2                            string `js:"TLS_1_2"`
	OCSP_STATUS_GOOD                   string `js:"OCSP_STATUS_GOOD"`
	OCSP_STATUS_REVOKED                string `js:"OCSP_STATUS_REVOKED"`
	OCSP_STATUS_SERVER_FAILED          string `js:"OCSP_STATUS_SERVER_FAILED"`
	OCSP_STATUS_UNKNOWN                string `js:"OCSP_STATUS_UNKNOWN"`
	OCSP_REASON_UNSPECIFIED            string `js:"OCSP_REASON_UNSPECIFIED"`
	OCSP_REASON_KEY_COMPROMISE         string `js:"OCSP_REASON_KEY_COMPROMISE"`
	OCSP_REASON_CA_COMPROMISE          string `js:"OCSP_REASON_CA_COMPROMISE"`
	OCSP_REASON_AFFILIATION_CHANGED    string `js:"OCSP_REASON_AFFILIATION_CHANGED"`
	OCSP_REASON_SUPERSEDED             string `js:"OCSP_REASON_SUPERSEDED"`
	OCSP_REASON_CESSATION_OF_OPERATION string `js:"OCSP_REASON_CESSATION_OF_OPERATION"`
	OCSP_REASON_CERTIFICATE_HOLD       string `js:"OCSP_REASON_CERTIFICATE_HOLD"`
	OCSP_REASON_REMOVE_FROM_CRL        string `js:"OCSP_REASON_REMOVE_FROM_CRL"`
	OCSP_REASON_PRIVILEGE_WITHDRAWN    string `js:"OCSP_REASON_PRIVILEGE_WITHDRAWN"`
	OCSP_REASON_AA_COMPROMISE          string `js:"OCSP_REASON_AA_COMPROMISE"`
}

func New() *HTTP {
	return &HTTP{
		SSL_3_0:                            SSL_3_0,
		TLS_1_0:                            TLS_1_0,
		TLS_1_1:                            TLS_1_1,
		TLS_1_2:                            TLS_1_2,
		OCSP_STATUS_GOOD:                   OCSP_STATUS_GOOD,
		OCSP_STATUS_REVOKED:                OCSP_STATUS_REVOKED,
		OCSP_STATUS_SERVER_FAILED:          OCSP_STATUS_SERVER_FAILED,
		OCSP_STATUS_UNKNOWN:                OCSP_STATUS_UNKNOWN,
		OCSP_REASON_UNSPECIFIED:            OCSP_REASON_UNSPECIFIED,
		OCSP_REASON_KEY_COMPROMISE:         OCSP_REASON_KEY_COMPROMISE,
		OCSP_REASON_CA_COMPROMISE:          OCSP_REASON_CA_COMPROMISE,
		OCSP_REASON_AFFILIATION_CHANGED:    OCSP_REASON_AFFILIATION_CHANGED,
		OCSP_REASON_SUPERSEDED:             OCSP_REASON_SUPERSEDED,
		OCSP_REASON_CESSATION_OF_OPERATION: OCSP_REASON_CESSATION_OF_OPERATION,
		OCSP_REASON_CERTIFICATE_HOLD:       OCSP_REASON_CERTIFICATE_HOLD,
		OCSP_REASON_REMOVE_FROM_CRL:        OCSP_REASON_REMOVE_FROM_CRL,
		OCSP_REASON_PRIVILEGE_WITHDRAWN:    OCSP_REASON_PRIVILEGE_WITHDRAWN,
		OCSP_REASON_AA_COMPROMISE:          OCSP_REASON_AA_COMPROMISE,
	}
}

func (*HTTP) XCookieJar(ctx *context.Context) *HTTPCookieJar {
	return newCookieJar(ctx)
}

func (*HTTP) CookieJar(ctx context.Context) *HTTPCookieJar {
	state := common.GetState(ctx)
	return &HTTPCookieJar{state.CookieJar, &ctx}
}

func (*HTTP) setRequestCookies(req *http.Request, jar *cookiejar.Jar, reqCookies map[string]*HTTPRequestCookie) {
	jarCookies := make(map[string][]*http.Cookie)
	for _, c := range jar.Cookies(req.URL) {
		jarCookies[c.Name] = append(jarCookies[c.Name], c)
	}
	for key, reqCookie := range reqCookies {
		if jc := jarCookies[key]; jc != nil && reqCookie.Replace {
			jarCookies[key] = []*http.Cookie{{Name: key, Value: reqCookie.Value}}
		} else {
			jarCookies[key] = append(jarCookies[key], &http.Cookie{Name: key, Value: reqCookie.Value})
		}
	}
	for _, cookies := range jarCookies {
		for _, c := range cookies {
			req.AddCookie(c)
		}
	}
}

func (h *HTTP) request(ctx context.Context, rt *goja.Runtime, state *common.State, method string, url goja.Value, args ...goja.Value) (*HTTPResponse, []stats.Sample, error) {
	var bodyReader io.Reader
	var contentType string
	if len(args) > 0 && !goja.IsUndefined(args[0]) && !goja.IsNull(args[0]) {
		var data map[string]goja.Value
		if rt.ExportTo(args[0], &data) == nil {
			bodyQuery := make(neturl.Values, len(data))
			for k, v := range data {
				bodyQuery.Set(k, v.String())
			}
			bodyReader = bytes.NewBufferString(bodyQuery.Encode())
			contentType = "application/x-www-form-urlencoded"
		} else {
			bodyReader = bytes.NewBufferString(args[0].String())
		}
	}

	// The provided URL can be either a string (or at least something stringable) or a URLTag.
	var urlStr string
	var nameTag string
	switch v := url.Export().(type) {
	case URLTag:
		urlStr = v.URL
		nameTag = v.Name
	default:
		urlStr = url.String()
		nameTag = urlStr
	}

	req, err := http.NewRequest(method, urlStr, bodyReader)
	if err != nil {
		return nil, nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if userAgent := state.Options.UserAgent; userAgent.Valid {
		req.Header.Set("User-Agent", userAgent.String)
	}

	tags := map[string]string{
		"proto":  "",
		"status": "0",
		"method": method,
		"url":    urlStr,
		"name":   nameTag,
		"group":  state.Group.Path,
	}
	redirects := -1
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
						req.Header.Set(key, headers.Get(key).String())
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
					redirects = int(params.Get(k).ToInteger())
					if redirects < 0 {
						redirects = 0
					}
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
		h.setRequestCookies(req, activeJar, reqCookies)
	}

	resp := &HTTPResponse{
		ctx: ctx,
		URL: urlStr,
	}
	client := http.Client{
		Transport: state.HTTPTransport,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Update active jar with cookies found in "Set-Cookie" header(s) of redirect response
			if activeJar != nil {
				if respCookies := req.Response.Cookies(); len(respCookies) > 0 {
					activeJar.SetCookies(req.URL, respCookies)
					h.setRequestCookies(req, activeJar, reqCookies)
				}
			}

			max := int(state.Options.MaxRedirects.Int64)
			if redirects >= 0 {
				max = redirects
			}
			if len(via) > max {
				if redirects < 0 {
					log.Println(via[0].Response)
					state.Logger.WithFields(log.Fields{
						"error": fmt.Sprintf("Possible redirect loop, %d response returned last, %d redirects followed; pass { redirects: n } in request params to silence this", via[len(via)-1].Response.StatusCode, max),
						"url":   via[0].URL.String(),
					}).Warn("Redirect Limit")
				}
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	tracer := netext.Tracer{}
	res, resErr := client.Do(req.WithContext(netext.WithTracer(ctx, &tracer)))
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
		Duration:   stats.D(trail.Duration),
		Blocked:    stats.D(trail.Blocked),
		Connecting: stats.D(trail.Connecting),
		Sending:    stats.D(trail.Sending),
		Waiting:    stats.D(trail.Waiting),
		Receiving:  stats.D(trail.Receiving),
	}

	if resErr != nil {
		resp.Error = resErr.Error()
		tags["error"] = resp.Error
	} else {
		if activeJar != nil {
			if rc := res.Cookies(); len(rc) > 0 {
				activeJar.SetCookies(req.URL, rc)
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

		var resCookies []*http.Cookie
		if client.Jar != nil {
			resCookies = res.Cookies()
		}
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

func (http *HTTP) Request(ctx context.Context, method string, url goja.Value, args ...goja.Value) (*HTTPResponse, error) {
	rt := common.GetRuntime(ctx)
	state := common.GetState(ctx)

	res, samples, err := http.request(ctx, rt, state, method, url, args...)
	state.Samples = append(state.Samples, samples...)
	return res, err
}

func (http *HTTP) Get(ctx context.Context, url goja.Value, args ...goja.Value) (*HTTPResponse, error) {
	// The body argument is always undefined for GETs and HEADs.
	args = append([]goja.Value{goja.Undefined()}, args...)
	return http.Request(ctx, "GET", url, args...)
}

func (http *HTTP) Head(ctx context.Context, url goja.Value, args ...goja.Value) (*HTTPResponse, error) {
	// The body argument is always undefined for GETs and HEADs.
	args = append([]goja.Value{goja.Undefined()}, args...)
	return http.Request(ctx, "HEAD", url, args...)
}

func (http *HTTP) Post(ctx context.Context, url goja.Value, args ...goja.Value) (*HTTPResponse, error) {
	return http.Request(ctx, "POST", url, args...)
}

func (http *HTTP) Put(ctx context.Context, url goja.Value, args ...goja.Value) (*HTTPResponse, error) {
	return http.Request(ctx, "PUT", url, args...)
}

func (http *HTTP) Patch(ctx context.Context, url goja.Value, args ...goja.Value) (*HTTPResponse, error) {
	return http.Request(ctx, "PATCH", url, args...)
}

func (http *HTTP) Del(ctx context.Context, url goja.Value, args ...goja.Value) (*HTTPResponse, error) {
	return http.Request(ctx, "DELETE", url, args...)
}

func (http *HTTP) Batch(ctx context.Context, reqsV goja.Value) (goja.Value, error) {
	rt := common.GetRuntime(ctx)
	state := common.GetState(ctx)

	errs := make(chan error)
	retval := rt.NewObject()
	mutex := sync.Mutex{}

	reqs := reqsV.ToObject(rt)
	keys := reqs.Keys()
	for _, k := range keys {
		k := k
		v := reqs.Get(k)

		var method string
		var url goja.Value
		var args []goja.Value

		// Shorthand: "http://example.com/" -> ["GET", "http://example.com/"]
		switch v.ExportType() {
		case typeString, typeURLTag:
			method = "GET"
			url = v
		default:
			obj := v.ToObject(rt)
			objkeys := obj.Keys()
			for _, objk := range objkeys {
				objv := obj.Get(objk)
				switch objk {
				case "0", "method":
					method = strings.ToUpper(objv.String())
					if method == "GET" || method == "HEAD" {
						args = []goja.Value{goja.Undefined()}
					}
				case "1", "url":
					url = objv
				default:
					args = append(args, objv)
				}
			}
		}

		go func() {
			res, samples, err := http.request(ctx, rt, state, method, url, args...)
			if err != nil {
				errs <- err
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

func (http *HTTP) Url(parts []string, pieces ...string) URLTag {
	var tag URLTag
	for i, part := range parts {
		tag.Name += part
		tag.URL += part
		if i < len(pieces) {
			tag.Name += "${}"
			tag.URL += pieces[i]
		}
	}
	return tag
}
