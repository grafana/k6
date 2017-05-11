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
	"encoding/json"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	neturl "net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/dop251/goja"
	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/js/modules/k6/html"
	"github.com/loadimpact/k6/lib/netext"
	"github.com/loadimpact/k6/stats"
	"github.com/pkg/errors"
)

type HTTPResponseTimings struct {
	Duration, Blocked, LookingUp, Connecting, Sending, Waiting, Receiving float64
}

type HTTPResponse struct {
	ctx context.Context

	RemoteIP   string
	RemotePort int
	URL        string
	Status     int
	Headers    map[string]string
	Body       string
	Timings    HTTPResponseTimings
	Error      string

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

func (*HTTP) Request(ctx context.Context, method, url string, args ...goja.Value) (*HTTPResponse, error) {
	rt := common.GetRuntime(ctx)
	state := common.GetState(ctx)

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

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if userAgent := state.Options.UserAgent; userAgent.Valid {
		req.Header.Set("User-Agent", userAgent.String)
	}

	tags := map[string]string{
		"status": "0",
		"method": method,
		"url":    url,
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
		URL: url,
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

	tracer := netext.Tracer{}
	res, resErr := client.Do(req.WithContext(netext.WithTracer(ctx, &tracer)))
	if res != nil {
		body, _ := ioutil.ReadAll(res.Body)
		_ = res.Body.Close()
		resp.Body = string(body)
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
		tags["url"] = resp.URL
		tags["status"] = strconv.Itoa(resp.Status)

		resp.Headers = make(map[string]string, len(res.Header))
		for k, vs := range res.Header {
			resp.Headers[k] = strings.Join(vs, ", ")
		}
	}

	state.Samples = append(state.Samples, trail.Samples(tags)...)
	if resErr != nil {
		log.WithField("error", resErr).Warn("Request Failed")
		if throw {
			return nil, resErr
		}
	}
	return resp, nil
}

func (http *HTTP) Get(ctx context.Context, url string, args ...goja.Value) (*HTTPResponse, error) {
	// The body argument is always undefined for GETs and HEADs.
	args = append([]goja.Value{goja.Undefined()}, args...)
	return http.Request(ctx, "GET", url, args...)
}

func (http *HTTP) Head(ctx context.Context, url string, args ...goja.Value) (*HTTPResponse, error) {
	// The body argument is always undefined for GETs and HEADs.
	args = append([]goja.Value{goja.Undefined()}, args...)
	return http.Request(ctx, "HEAD", url, args...)
}

func (http *HTTP) Post(ctx context.Context, url string, args ...goja.Value) (*HTTPResponse, error) {
	return http.Request(ctx, "POST", url, args...)
}

func (http *HTTP) Put(ctx context.Context, url string, args ...goja.Value) (*HTTPResponse, error) {
	return http.Request(ctx, "PUT", url, args...)
}

func (http *HTTP) Patch(ctx context.Context, url string, args ...goja.Value) (*HTTPResponse, error) {
	return http.Request(ctx, "PATCH", url, args...)
}

func (http *HTTP) Del(ctx context.Context, url string, args ...goja.Value) (*HTTPResponse, error) {
	return http.Request(ctx, "DELETE", url, args...)
}

func (http *HTTP) Batch(ctx context.Context, reqsV goja.Value) (goja.Value, error) {
	rt := common.GetRuntime(ctx)

	errs := make(chan error)
	retval := rt.NewObject()
	mutex := sync.Mutex{}

	reqs := reqsV.ToObject(rt)
	keys := reqs.Keys()
	for _, k := range keys {
		k := k
		v := reqs.Get(k)

		var method, url string
		var args []goja.Value

		// Shorthand: "http://example.com/" -> ["GET", "http://example.com/"]
		if v.ExportType().Kind() == reflect.String {
			method = "GET"
			url = v.String()
		} else {
			obj := v.ToObject(rt)
			objkeys := obj.Keys()
			for i, objk := range objkeys {
				objv := obj.Get(objk)
				switch i {
				case 0:
					method = strings.ToUpper(objv.String())
					if method == "GET" || method == "HEAD" {
						args = []goja.Value{goja.Undefined()}
					}
				case 1:
					url = objv.String()
				default:
					args = append(args, objv)
				}
			}
		}

		go func() {
			res, err := http.Request(ctx, method, url, args...)
			if err != nil {
				errs <- err
			}
			mutex.Lock()
			_ = retval.Set(k, res)
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
