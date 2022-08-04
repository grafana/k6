package httpext

import (
	"fmt"
	"net/url"
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

	if password, passwordOk := u.u.User.Password(); passwordOk {
		// here 3 is for the '://' and 4 is because of '://' and ':' between the credentials
		return u.URL[:len(u.u.Scheme)+3] + "****:****" + u.URL[len(u.u.Scheme)+4+len(u.u.User.Username())+len(password):]
	}

	// here 3 in both places is for the '://'
	return u.URL[:len(u.u.Scheme)+3] + "****" + u.URL[len(u.u.Scheme)+3+len(u.u.User.Username()):]
}

// GetURL returns the internal url.URL
func (u URL) GetURL() *url.URL {
	return u.u
}

// ToURL tries to convert anything passed to it to a k6 URL struct
func ToURL(u interface{}) (URL, error) {
	switch tu := u.(type) {
	case URL:
		// Handling of http.url`http://example.com/{$id}`
		return tu, nil
	case string:
		// Handling of "http://example.com/"
		return NewURL(tu, tu)
	default:
		return URL{}, NewK6Error(invalidURLErrorCode,
			fmt.Sprintf("%s: '#%v'", invalidURLErrorCodeMsg, u), nil)
	}
}
