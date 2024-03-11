//go:generate tools/genoptions.sh

// Package httprc implements a cache for resources available
// over http(s). Its aim is not only to cache these resources so
// that it saves on HTTP roundtrips, but it also periodically
// attempts to auto-refresh these resources once they are cached
// based on the user-specified intervals and HTTP `Expires` and
// `Cache-Control` headers, thus keeping the entries _relatively_ fresh.
package httprc

import "fmt"

// RefreshError is the underlying error type that is sent to
// the `httprc.ErrSink` objects
type RefreshError struct {
	URL string
	Err error
}

func (re *RefreshError) Error() string {
	return fmt.Sprintf(`refresh error (%q): %s`, re.URL, re.Err)
}
