// Package usage implements usage tracking for k6 in order to figure what is being used within a given execution
package usage

import (
	"strings"
	"sync"
)

// Usage is a way to collect usage data for within k6
type Usage struct {
	l *sync.Mutex
	m map[string]any
}

// New returns a new empty Usage ready to be used
func New() *Usage {
	return &Usage{
		l: new(sync.Mutex),
		m: make(map[string]any),
	}
}

// String appends the provided value to a slice of strings that is the value.
// If called only a single time, the value will be just a string not a slice
func (u *Usage) String(k, v string) {
	u.l.Lock()
	defer u.l.Unlock()
	oldV, ok := u.m[k]
	if !ok {
		u.m[k] = v
		return
	}
	switch oldV := oldV.(type) {
	case string:
		u.m[k] = []string{oldV, v}
	case []string:
		u.m[k] = append(oldV, v)
	default:
		// TODO: error, panic?, nothing, log?
	}
}

// Strings appends the provided value to a slice of strings that is the value.
// Unlike String it
func (u *Usage) Strings(k, v string) {
	u.l.Lock()
	defer u.l.Unlock()
	oldV, ok := u.m[k]
	if !ok {
		u.m[k] = []string{v}
		return
	}
	switch oldV := oldV.(type) {
	case string:
		u.m[k] = []string{oldV, v}
	case []string:
		u.m[k] = append(oldV, v)
	default:
		// TODO: error, panic?, nothing, log?
	}
}

// Count adds the provided value to a given key. Creating the key if needed
func (u *Usage) Count(k string, v int64) {
	u.l.Lock()
	defer u.l.Unlock()
	oldV, ok := u.m[k]
	if !ok {
		u.m[k] = v
		return
	}
	switch oldV := oldV.(type) {
	case int64:
		u.m[k] = oldV + v
	default:
		// TODO: error, panic?, nothing, log?
	}
}

// Map returns a copy of the internal map plus making subusages from keys that have a slash in them
// only a single level is being respected
func (u *Usage) Map() map[string]any {
	u.l.Lock()
	defer u.l.Unlock()

	result := make(map[string]any, len(u.m))
	for k, v := range u.m {
		prefix, post, found := strings.Cut(k, "/")
		if !found {
			result[k] = v
			continue
		}

		topLevel, ok := result[prefix]
		if !ok {
			topLevel = make(map[string]any)
			result[prefix] = topLevel
		}
		topLevelMap, ok := topLevel.(map[string]any)
		if !ok {
			continue // TODO panic?, error?
		}
		keyLevel, ok := topLevelMap[post]
		switch value := v.(type) {
		case int64:
			switch i := keyLevel.(type) {
			case int64:
				keyLevel = i + value
			default:
				// TODO:panic? error?
			}
		case string:
			switch i := keyLevel.(type) {
			case string:
				keyLevel = append([]string(nil), i, value)
			case []string:
				keyLevel = append(i, value) //nolint:gocritic // we assign to the final value
			default:
				// TODO:panic? error?
			}
		case []string:
			switch i := keyLevel.(type) {
			case []string:
				keyLevel = append(i, value...) //nolint:gocritic // we assign to the final value
			default:
				// TODO:panic? error?
			}
		}
		if !ok {
			keyLevel = v
		}
		topLevelMap[post] = keyLevel
	}

	return result
}
