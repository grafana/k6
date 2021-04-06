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
	"context"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	neturl "net/url"
	"strings"
	"time"

	"github.com/dop251/goja"

	"github.com/loadimpact/k6/js/common"
)

// HTTPCookieJar is cookiejar.Jar wrapper to be used in js scripts
type HTTPCookieJar struct {
	jar *cookiejar.Jar
	ctx *context.Context
}

func newCookieJar(ctxPtr *context.Context) *HTTPCookieJar {
	jar, err := cookiejar.New(nil)
	if err != nil {
		common.Throw(common.GetRuntime(*ctxPtr), err)
	}
	return &HTTPCookieJar{jar, ctxPtr}
}

// CookiesForURL return the cookies for a given url as a map of key and values
func (j HTTPCookieJar) CookiesForURL(url string) map[string][]string {
	u, err := neturl.Parse(url)
	if err != nil {
		panic(err)
	}

	cookies := j.jar.Cookies(u)
	objs := make(map[string][]string, len(cookies))
	for _, c := range cookies {
		objs[c.Name] = append(objs[c.Name], c.Value)
	}
	return objs
}

// Set sets a cookie for a particular url with the given name value and additional opts
func (j HTTPCookieJar) Set(url, name, value string, opts goja.Value) (bool, error) {
	rt := common.GetRuntime(*j.ctx)

	u, err := neturl.Parse(url)
	if err != nil {
		return false, err
	}

	c := http.Cookie{Name: name, Value: value}
	paramsV := opts
	if paramsV != nil && !goja.IsUndefined(paramsV) && !goja.IsNull(paramsV) {
		params := paramsV.ToObject(rt)
		for _, k := range params.Keys() {
			switch strings.ToLower(k) {
			case "path":
				c.Path = params.Get(k).String()
			case "domain":
				c.Domain = params.Get(k).String()
			case "expires":
				var t time.Time
				expires := params.Get(k).String()
				if expires != "" {
					t, err = time.Parse(time.RFC1123, expires)
					if err != nil {
						return false, fmt.Errorf(`unable to parse "expires" date string "%s": %w`, expires, err)
					}
				}
				c.Expires = t
			case "max_age":
				c.MaxAge = int(params.Get(k).ToInteger())
			case "secure":
				c.Secure = params.Get(k).ToBoolean()
			case "http_only":
				c.HttpOnly = params.Get(k).ToBoolean()
			}
		}
	}
	j.jar.SetCookies(u, []*http.Cookie{&c})
	return true, nil
}
