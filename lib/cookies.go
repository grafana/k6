/*

k6 - a next-generation load testing tool
Copyright (C) 2016 Load Impact

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.

*/

package lib

import (
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// CookieJar implements a simplified version of net/http/cookiejar, that most notably can be
// cleared without reinstancing the whole thing.
type CookieJar struct {
	mutex   sync.Mutex
	cookies map[string][]*http.Cookie
}

func NewCookieJar() *CookieJar {
	jar := &CookieJar{}
	jar.Clear()
	return jar
}

func (j *CookieJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	if len(cookies) == 0 {
		return
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return
	}
	j.cookies[cookieHostKey(u.Host)] = cookies
}

func (j *CookieJar) Cookies(u *url.URL) []*http.Cookie {
	return j.cookies[cookieHostKey(u.Host)]
}

func (j *CookieJar) Clear() {
	j.cookies = make(map[string][]*http.Cookie)
}

func cookieHostKey(host string) string {
	if strings.HasSuffix(host, ".") {
		host = host[:len(host)-1]
	}
	host, _, err := net.SplitHostPort(host)
	if err != nil {
		return host
	}
	return host
}
