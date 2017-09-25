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
	"crypto/tls"
	"encoding/json"
	"io"
	"net"
	"net/http"
	neturl "net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ocsp"

	"github.com/dop251/goja"
	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/js/modules/k6/html"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/netext"
	"github.com/loadimpact/k6/stats"
	"github.com/pkg/errors"
)

var (
	typeString = reflect.TypeOf("")
	typeURLTag = reflect.TypeOf(URLTag{})
)

type OCSPStapledResponse struct {
	ProducedAt, ThisUpdate, NextUpdate, RevokedAt string
	RevocationReason                              string
	Status                                        string
}

type OCSP struct {
	StapledResponse OCSPStapledResponse
}

type HTTPResponseTimings struct {
	Duration, Blocked, LookingUp, Connecting, Sending, Waiting, Receiving float64
}

type HTTPResponse struct {
	ctx context.Context

	RemoteIP       string
	RemotePort     int
	URL            string
	Status         int
	Proto          string
	Headers        map[string]string
	Body           string
	Timings        HTTPResponseTimings
	TLSVersion     string
	TLSCipherSuite string
	OCSP           OCSP `js:"ocsp"`
	Error          string

	cachedJSON goja.Value
}

func (res *HTTPResponse) Json() goja.Value {
	if res.cachedJSON == nil {
		var v interface{}
		if err := json.Unmarshal([]byte(res.Body), &v); err != nil {
			common.Throw(common.GetRuntime(res.ctx), err)
		}
		res.cachedJSON = common.GetRuntime(res.ctx).ToValue(v)
	}
	return res.cachedJSON
}

func (res *HTTPResponse) Html(selector ...string) html.Selection {
	sel, err := html.HTML{}.ParseHTML(res.ctx, res.Body)
	if err != nil {
		common.Throw(common.GetRuntime(res.ctx), err)
	}
	if len(selector) > 0 {
		sel = sel.Find(selector[0])
	}
	return sel
}

type HTTP struct{}

func (*HTTP) request(ctx context.Context, rt *goja.Runtime, state *common.State, method string, url goja.Value, args ...goja.Value) (*HTTPResponse, []stats.Sample, error) {
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
	timeout := 60 * time.Second
	throw := state.Options.Throw.Bool

	if len(args) > 1 {
		paramsV := args[1]
		if !goja.IsUndefined(paramsV) && !goja.IsNull(paramsV) {
			params := paramsV.ToObject(rt)
			for _, k := range params.Keys() {
				switch k {
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

	resp := &HTTPResponse{
		ctx: ctx,
		URL: urlStr,
	}
	client := http.Client{
		Transport: state.HTTPTransport,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			max := int(state.Options.MaxRedirects.Int64)
			if len(via) >= max {
				return errors.Errorf("stopped after %d redirects", max)
			}
			return nil
		},
	}
	if state.CookieJar != nil {
		client.Jar = state.CookieJar
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
		resp.URL = res.Request.URL.String()
		resp.Status = res.StatusCode
		resp.Proto = res.Proto
		tags["url"] = resp.URL
		tags["status"] = strconv.Itoa(resp.Status)
		tags["proto"] = resp.Proto

		if res.TLS != nil {
			tlsState := res.TLS
			switch tlsState.Version {
			case tls.VersionSSL30:
				resp.TLSVersion = "ssl3.0"
			case tls.VersionTLS10:
				resp.TLSVersion = "tls1.0"
			case tls.VersionTLS11:
				resp.TLSVersion = "tls1.1"
			case tls.VersionTLS12:
				resp.TLSVersion = "tls1.2"
			}
			resp.TLSCipherSuite = lib.SupportedTLSCipherSuitesToString[tlsState.CipherSuite]
			resp.OCSP.StapledResponse = OCSPStapledResponse{}
			if ocspRes, err := ocsp.ParseResponse(tlsState.OCSPResponse, nil); err == nil {
				switch ocspRes.Status {
				case ocsp.Good:
					resp.OCSP.StapledResponse.Status = "good"
				case ocsp.Revoked:
					resp.OCSP.StapledResponse.Status = "revoked"
				case ocsp.Unknown:
					resp.OCSP.StapledResponse.Status = "unknown"
				case ocsp.ServerFailed:
					resp.OCSP.StapledResponse.Status = "server_failed"
				}
				switch ocspRes.RevocationReason {
				case ocsp.Unspecified:
					resp.OCSP.StapledResponse.RevocationReason = "unspecified"
				case ocsp.KeyCompromise:
					resp.OCSP.StapledResponse.RevocationReason = "key_compromise"
				case ocsp.CACompromise:
					resp.OCSP.StapledResponse.RevocationReason = "ca_compromise"
				case ocsp.AffiliationChanged:
					resp.OCSP.StapledResponse.RevocationReason = "affiliation_changed"
				case ocsp.Superseded:
					resp.OCSP.StapledResponse.RevocationReason = "superseded"
				case ocsp.CessationOfOperation:
					resp.OCSP.StapledResponse.RevocationReason = "cessation_of_operation"
				case ocsp.CertificateHold:
					resp.OCSP.StapledResponse.RevocationReason = "certificate_hold"
				case ocsp.RemoveFromCRL:
					resp.OCSP.StapledResponse.RevocationReason = "remove_from_crl"
				case ocsp.PrivilegeWithdrawn:
					resp.OCSP.StapledResponse.RevocationReason = "privilege_withdrawn"
				case ocsp.AACompromise:
					resp.OCSP.StapledResponse.RevocationReason = "aa_compromise"
				}
				resp.OCSP.StapledResponse.ProducedAt = ocspRes.ProducedAt.String()
				resp.OCSP.StapledResponse.ThisUpdate = ocspRes.ThisUpdate.String()
				resp.OCSP.StapledResponse.NextUpdate = ocspRes.NextUpdate.String()
				resp.OCSP.StapledResponse.RevokedAt = ocspRes.RevokedAt.String()
			}
		}

		resp.Headers = make(map[string]string, len(res.Header))
		for k, vs := range res.Header {
			resp.Headers[k] = strings.Join(vs, ", ")
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
