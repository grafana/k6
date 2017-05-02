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
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/loadimpact/k6/lib/netext"
	"github.com/loadimpact/k6/stats"
	"github.com/robertkrimen/otto"
)

type HTTPResponse struct {
	Status int
}

type HTTPCookie struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Domain   string `json:"domain"`
	Path     string `json:"path"`
	Expires  string `json:"expires"`
	MaxAge   int    `json:"maxAge"`
	Secure   bool   `json:"secure"`
	HttpOnly bool   `json:"httpOnly"`
}

type HTTPParams struct {
	Headers map[string]string `json:"headers"`
	Cookies []HTTPCookie      `json:"cookies"`
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

	var cookiesList []*http.Cookie
	for _, c := range params.Cookies {
		var t time.Time
		if c.Expires != "" {
			t, err = time.Parse(time.RFC1123, c.Expires)
			if err != nil {
				throw(a.vu.vm, err)
			}
		}
		cookie := &http.Cookie{
			Name:     c.Key,
			Value:    c.Value,
			Path:     c.Path,
			Domain:   c.Domain,
			Expires:  t,
			MaxAge:   c.MaxAge,
			HttpOnly: c.HttpOnly,
			Secure:   c.Secure,
		}
		cookiesList = append(cookiesList, cookie)
	}
	a.vu.CookieJar.SetCookies(req.URL, cookiesList)

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

	tracer := netext.Tracer{}
	res, err := a.vu.HTTPClient.Do(req.WithContext(netext.WithTracer(a.vu.ctx, &tracer)))
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
	cookies := make(map[string]string)
	for _, v := range a.vu.CookieJar.Cookies(res.Request.URL) {
		cookies[v.Name] = v.Value
	}
	return map[string]interface{}{
		"url":     res.Request.URL.String(),
		"status":  res.StatusCode,
		"body":    string(resBody),
		"headers": headers,
		"cookies": cookies,
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
	mutex := sync.Mutex{}

	keys := obj.Keys()
	errs := make(chan interface{}, len(keys))
	for _, key := range keys {
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

		go func(tkey string) {
			defer func() { errs <- recover() }()
			res := a.HTTPRequest(method, url, body, params)

			mutex.Lock()
			_ = obj.Set(tkey, res)
			mutex.Unlock()
		}(key)
	}

	for i := 0; i < len(keys); i++ {
		if err := <-errs; err != nil {
			panic(err)
		}
	}

	return obj.Value()
}
