package httpext

import (
	"fmt"
	"net/url"
	"strings"
)

// A URL wraps net.URL, and preserves the template (if any) the URL was constructed from.
type URL struct {
	u        *url.URL
	Name     string // http://example.com/thing/${}/
	URL      string // http://example.com/thing/1234/
	CleanURL string // URL with masked user credentials, used for output
}

// NewURL returns a new URL for the provided url and name. The error is returned if the url provided
// can't be parsed
func NewURL(urlString, name string) (URL, error) {
	u, err := url.Parse(urlString)
	if err != nil {
		return URL{}, NewK6Error(invalidURLErrorCode,
			fmt.Sprintf("%s: %s", invalidURLErrorCodeMsg, err), err)
	}
	newURL := URL{u: u, Name: name, URL: urlString}
	newURL.CleanURL = newURL.Clean()
	if urlString == name {
		newURL.Name = newURL.CleanURL
	}
	return newURL, nil
}

// Clean returns an output-safe representation of URL
func (u URL) Clean() string {
	if u.CleanURL != "" {
		return u.CleanURL
	}

	if u.u == nil || u.u.User == nil {
		return u.URL
	}

	// Mask the credentials in the raw URL without relying on the decoded
	// username/password lengths. Those lengths differ from the bytes actually
	// present in u.URL whenever the userinfo contains percent-encoded characters
	// (e.g. a password with a '@', ':' or space), and using them would slice in
	// the wrong place and leak part of the credentials into the "clean" output.
	// Instead locate the userinfo in the raw string: it spans from right after
	// the "scheme://" prefix up to the '@' that separates it from the host.
	prefixLen := len(u.u.Scheme) + len("://")
	authority := u.URL[prefixLen:]
	if end := strings.IndexAny(authority, "/?#"); end >= 0 {
		authority = authority[:end]
	}
	// url.Parse splits the userinfo from the host at the last '@' in the
	// authority, so mirror that here.
	at := strings.LastIndex(authority, "@")
	if at < 0 {
		return u.URL
	}

	mask := "****"
	if _, passwordOk := u.u.User.Password(); passwordOk {
		mask = "****:****"
	}
	return u.URL[:prefixLen] + mask + u.URL[prefixLen+at:]
}

// GetURL returns the internal url.URL
func (u URL) GetURL() *url.URL {
	return u.u
}

// ToURL tries to convert anything passed to it to a k6 URL struct.
//
// The returned URL always owns its underlying *url.URL: values coming from
// http.url are reused by every request that references them, so the struct is
// copied here and callers may freely mutate their copy (e.g. to clear the user
// info for digest auth, #6397) without affecting the shared object.
func ToURL(u any) (URL, error) {
	switch tu := u.(type) {
	case URL:
		// Handling of http.url`http://example.com/{$id}`
		if tu.u != nil {
			requestURL := *tu.u
			tu.u = &requestURL
		}
		return tu, nil
	case string:
		// Handling of "http://example.com/"
		return NewURL(tu, tu)
	default:
		return URL{}, NewK6Error(invalidURLErrorCode,
			fmt.Sprintf("%s: '#%v'", invalidURLErrorCodeMsg, u), nil)
	}
}
