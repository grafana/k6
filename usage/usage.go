// Package usage implements usage tracking for k6 in order to figure what is being used within a given execution
package usage

import (
	"errors"
	"fmt"
	"sort"
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

// Strings appends the provided value to a slice of strings that is the value.
// Appending to the slice if the key is already there.
func (u *Usage) Strings(k, v string) error {
	u.l.Lock()
	defer u.l.Unlock()
	oldV, ok := u.m[k]
	if !ok {
		u.m[k] = []string{v}
		return nil
	}
	switch oldV := oldV.(type) {
	case []string:
		u.m[k] = append(oldV, v)
	default:
		return fmt.Errorf("value of key %s is not []string as expected but %T", k, oldV)
	}
	return nil
}

// Uint64 adds the provided value to a given key. Creating the key if needed
func (u *Usage) Uint64(k string, v uint64) error {
	u.l.Lock()
	defer u.l.Unlock()
	oldV, ok := u.m[k]
	if !ok {
		u.m[k] = v
		return nil
	}
	switch oldVUint64 := oldV.(type) {
	case uint64:
		u.m[k] = oldVUint64 + v
	default:
		return fmt.Errorf("!value of key %s is not uint64 as expected but %T", k, oldV)
		// TODO: error, panic?, nothing, log?
	}
	return nil
}

// Map returns a copy of the internal map plus making subusages from keys that have a slash in them
// only a single level is being respected
func (u *Usage) Map() (map[string]any, error) {
	u.l.Lock()
	defer u.l.Unlock()
	var errs []error

	keys := mapKeys(u.m)
	sort.Strings(keys)
	result := make(map[string]any, len(u.m))
	for _, k := range keys {
		v := u.m[k]
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
			errs = append(errs, fmt.Errorf("key %s was expected to be a map[string]any but was %T", prefix, topLevel))
			continue
		}
		topLevelMap[post] = v
	}

	return result, errors.Join(errs...)
}

// replace with map.Keys from go 1.23 after that is the minimal version
func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
