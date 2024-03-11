package httprc

import "regexp"

// Whitelist is an interface for a set of URL whitelists. When provided
// to fetching operations, urls are checked against this object, and
// the object must return true for urls to be fetched.
type Whitelist interface {
	IsAllowed(string) bool
}

// InsecureWhitelist allows any URLs to be fetched.
type InsecureWhitelist struct{}

func (InsecureWhitelist) IsAllowed(string) bool {
	return true
}

// RegexpWhitelist is a httprc.Whitelist object comprised of a list of *regexp.Regexp
// objects. All entries in the list are tried until one matches. If none of the
// *regexp.Regexp objects match, then the URL is deemed unallowed.
type RegexpWhitelist struct {
	patterns []*regexp.Regexp
}

func NewRegexpWhitelist() *RegexpWhitelist {
	return &RegexpWhitelist{}
}

func (w *RegexpWhitelist) Add(pat *regexp.Regexp) *RegexpWhitelist {
	w.patterns = append(w.patterns, pat)
	return w
}

// IsAlloed returns true if any of the patterns in the whitelist
// returns true.
func (w *RegexpWhitelist) IsAllowed(u string) bool {
	for _, pat := range w.patterns {
		if pat.MatchString(u) {
			return true
		}
	}
	return false
}

// MapWhitelist is a httprc.Whitelist object comprised of a map of strings.
// If the URL exists in the map, then the URL is allowed to be fetched.
type MapWhitelist struct {
	store map[string]struct{}
}

func NewMapWhitelist() *MapWhitelist {
	return &MapWhitelist{store: make(map[string]struct{})}
}

func (w *MapWhitelist) Add(pat string) *MapWhitelist {
	w.store[pat] = struct{}{}
	return w
}

func (w *MapWhitelist) IsAllowed(u string) bool {
	_, b := w.store[u]
	return b
}

// WhitelistFunc is a httprc.Whitelist object based on a function.
// You can perform any sort of check against the given URL to determine
// if it can be fetched or not.
type WhitelistFunc func(string) bool

func (w WhitelistFunc) IsAllowed(u string) bool {
	return w(u)
}
