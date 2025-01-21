// Package usage implements usage tracking for k6 in order to figure what is being used within a given execution
package usage

import (
	"fmt"
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
// It also works out level of keys
func (u *Usage) Strings(originalKey, value string) error {
	u.l.Lock()
	defer u.l.Unlock()
	m, newKey, err := u.createLevel(originalKey)
	if err != nil {
		return err
	}
	oldV, ok := m[newKey]
	if !ok {
		m[newKey] = []string{value}
		return nil
	}
	switch oldValue := oldV.(type) {
	case []string:
		m[newKey] = append(oldValue, value)
	default:
		return fmt.Errorf("value of key %s is not []string as expected but %T", originalKey, oldValue)
	}
	return nil
}

// Uint64 adds the provided value to a given key. Creating the key if needed and working out levels of keys
func (u *Usage) Uint64(originalKey string, value uint64) error {
	u.l.Lock()
	defer u.l.Unlock()
	m, newKey, err := u.createLevel(originalKey)
	if err != nil {
		return err
	}

	oldValue, ok := m[newKey]
	if !ok {
		m[newKey] = value
		return nil
	}
	switch oldVUint64 := oldValue.(type) {
	case uint64:
		m[newKey] = oldVUint64 + value
	default:
		return fmt.Errorf("value of key %s is not uint64 as expected but %T", originalKey, oldValue)
	}
	return nil
}

func (u *Usage) createLevel(key string) (map[string]any, string, error) {
	levelKey, subLevelKey, found := strings.Cut(key, "/")
	if !found {
		return u.m, key, nil
	}
	if strings.Contains(subLevelKey, "/") {
		return nil, "", fmt.Errorf("only one level is permitted in usages: %q", key)
	}

	level, ok := u.m[levelKey]
	if !ok {
		level = make(map[string]any)
		u.m[levelKey] = level
	}
	levelMap, ok := level.(map[string]any)
	if !ok {
		return nil, "", fmt.Errorf("new level %q for key %q as the key was already used for %T", levelKey, key, level)
	}
	return levelMap, subLevelKey, nil
}

// Map returns a copy of the internal map
func (u *Usage) Map() map[string]any {
	u.l.Lock()
	defer u.l.Unlock()

	return deepClone(u.m)
}

func deepClone(m map[string]any) map[string]any {
	result := make(map[string]any, len(m))
	for k, v := range m {
		if newM, ok := v.(map[string]any); ok {
			v = deepClone(newM)
		}
		result[k] = v
	}
	return result
}
