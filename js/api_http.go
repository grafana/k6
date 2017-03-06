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

package js

import (
	"encoding/json"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
	"github.com/robertkrimen/otto"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptrace"
	"strconv"
	"strings"
	"sync"
)

type HTTPResponse struct {
	Status int
}

type HTTPParams struct {
	Headers map[string]string `json:"headers"`
	Tags    map[string]string `json:"tags"`
}

func (a JSAPI) HTTPRequest(method, url, body string, paramData string) map[string]interface{} {
	bodyReader := io.Reader(nil)
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		throw(a.vu.vm, err)
	}

	var params HTTPParams
	if err := json.Unmarshal([]byte(paramData), &params); err != nil {
		throw(a.vu.vm, err)
	}

	for key, value := range params.Headers {
		req.Header.Set(key, value)
	}

	tags := map[string]string{
		"vu":     a.vu.IDString,
		"status": "0",
		"method": method,
		"url":    url,
		"group":  a.vu.group.Path,
	}
	for key, value := range params.Tags {
		tags[key] = value
	}

	tracer := lib.Tracer{}
	res, err := a.vu.HTTPClient.Do(req.WithContext(httptrace.WithClientTrace(a.vu.ctx, tracer.Trace())))
	if err != nil {
		a.vu.Samples = append(a.vu.Samples, tracer.Done().Samples(tags)...)
		throw(a.vu.vm, err)
	}
	tags["status"] = strconv.Itoa(res.StatusCode)

	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		a.vu.Samples = append(a.vu.Samples, tracer.Done().Samples(tags)...)
		throw(a.vu.vm, err)
	}
	_ = res.Body.Close()

	trail := tracer.Done()
	a.vu.Samples = append(a.vu.Samples, trail.Samples(tags)...)

	headers := make(map[string]string)
	for k, v := range res.Header {
		headers[k] = strings.Join(v, ", ")
	}
	return map[string]interface{}{
		"url":     res.Request.URL.String(),
		"status":  res.StatusCode,
		"body":    string(resBody),
		"headers": headers,
		"timings": map[string]float64{
			"duration":   stats.D(trail.Duration),
			"blocked":    stats.D(trail.Blocked),
			"looking_up": stats.D(trail.LookingUp),
			"connecting": stats.D(trail.Connecting),
			"sending":    stats.D(trail.Sending),
			"waiting":    stats.D(trail.Waiting),
			"receiving":  stats.D(trail.Receiving),
		},
	}
}

func (a JSAPI) BatchHTTPRequest(requests otto.Value) otto.Value {
	obj := requests.Object()

	wg := sync.WaitGroup{}
	mutex := sync.Mutex{}
	for _, key := range obj.Keys() {
		v, _ := obj.Get(key)

		var method string
		var url string
		var body string
		var params string

		o := v.Object()

		v, _ = o.Get("method")
		method = v.String()
		v, _ = o.Get("url")
		url = v.String()
		v, _ = o.Get("body")
		body = v.String()
		v, _ = o.Get("params")
		params = v.String()

		wg.Add(1)
		go func(tkey string) {
			defer wg.Done()

			res := a.HTTPRequest(method, url, body, params)

			mutex.Lock()
			defer mutex.Unlock()

			_ = obj.Set(tkey, res)
		}(key)
	}

	wg.Wait()
	return obj.Value()
}
