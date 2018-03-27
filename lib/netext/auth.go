package netext

import (
	"sync"
)

type AuthCache struct {
	sync.Mutex

	entries map[string]string
}

func NewAuthCache() *AuthCache {
	return &AuthCache{
		entries: make(map[string]string),
	}
}

func (a *AuthCache) Set(key, value string) {
	a.Lock()
	defer a.Unlock()

	a.entries[key] = value
}

func (a AuthCache) Get(key string) (string, bool) {
	a.Lock()
	defer a.Unlock()

	value, ok := a.entries[key]
	return value, ok
}
