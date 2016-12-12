package lib

import (
	"net"
	"net/http"
	"net/url"
	"strings"
)

// CookieJar implements a simplified version of net/http/cookiejar, that most notably can be
// cleared without reinstancing the whole thing.
type CookieJar struct {
	cookies map[string][]*http.Cookie
}

func NewCookieJar() *CookieJar {
	jar := &CookieJar{}
	jar.Clear()
	return jar
}

func (j *CookieJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
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
