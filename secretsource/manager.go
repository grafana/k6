package secretsource

import (
	"errors"
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"
)

// DefaultSourceName is the name for the default secret source
const DefaultSourceName = "default"

// Manager manages secrets making certain for them to be redacted from logs
type Manager struct {
	hook    *secretsHook
	sources map[string]Source
	cache   map[string]*sync.Map
}

// NewManager returns a new NewManager with the provided secretsHook and will redact secrets from the hook
func NewManager(sources map[string]Source) (*Manager, logrus.Hook, error) {
	cache := make(map[string]*sync.Map, len(sources)-1)
	hook := &secretsHook{}
	if len(sources) == 0 {
		return &Manager{
			hook:  hook,
			cache: cache,
		}, hook, nil
	}
	defaultSource := sources["default"]
	if defaultSource != nil {
		cache["default"] = new(sync.Map)
	}
	for k, source := range sources {
		if k == "default" {
			continue
		}
		if source == defaultSource {
			cache[k] = cache["default"]
			continue
		}
		cache[k] = new(sync.Map)
	}
	sm := &Manager{
		hook:    hook,
		sources: sources,
		cache:   cache,
	}
	return sm, hook, nil
}

// Get is the way to get a secret for the provided source name and key of the secret.
// It can be used with the [DefaultSourceName].
// This automatically starts redacting the secret before returning it.
func (sm *Manager) Get(sourceName, key string) (string, error) {
	if len(sm.cache) == 0 {
		return "", errors.New("no secret sources are configured")
	}
	sourceCache, ok := sm.cache[sourceName]
	if !ok {
		return "", UnknownSourceError(sourceName)
	}
	v, ok := sourceCache.Load(key)
	if ok {
		return v.(string), nil //nolint:forcetypeassert
	}
	source := sm.sources[sourceName]
	value, err := source.Get(key)
	if err != nil {
		return "", err
	}
	sourceCache.Store(key, value)
	sm.hook.add(value)
	return value, err
}

// UnknownSourceError is returned when a unknown source is requested
type UnknownSourceError string

func (u UnknownSourceError) Error() string {
	return fmt.Sprintf("no secret source with name %q is configured", (string)(u))
}
