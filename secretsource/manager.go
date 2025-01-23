package secretsource

import (
	"fmt"
	"sync"

	"go.k6.io/k6/internal/log"
)

// SecretsManager manages secrets making certain for them to be redacted from logs
type SecretsManager struct {
	hook    *log.SecretsHook
	sources map[string]SecretSource
	cache   map[string]*sync.Map
}

// NewSecretsManager returns a new NewSecretsManager with the provided secretsHook and will redact secrets from the hook
func NewSecretsManager(hook *log.SecretsHook, sources map[string]SecretSource) (*SecretsManager, error) {
	cache := make(map[string]*sync.Map, len(sources)-1)
	defaultSource := sources["default"]
	cache["default"] = new(sync.Map)
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
	sm := &SecretsManager{
		hook:    hook,
		sources: sources,
		cache:   cache,
	}
	return sm, nil
}

// Get is the way to get a secret for the provided source and key
// the default source key is "default" // TODO move to const
// This automatically starts redacting the secret after getting it
func (sm *SecretsManager) Get(sourceKey, key string) (string, error) {
	sourceCache, ok := sm.cache[sourceKey]
	if !ok {
		return "", fmt.Errorf("no source with name %s", sourceKey)
	}
	v, ok := sourceCache.Load(key)
	if ok {
		return v.(string), nil //nolint:forcetypeassert
	}
	source := sm.sources[sourceKey]
	value, err := source.Get(key)
	if err != nil {
		return "", err
	}
	sourceCache.Store(key, value)
	sm.hook.Add(value)
	return value, err
}
