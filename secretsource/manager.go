package secretsource

import (
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"
)

// DefaultSourceName is the name for the default secret source
const DefaultSourceName = "default"

// SecretsManager manages secrets making certain for them to be redacted from logs
type SecretsManager struct {
	hook    *secretsHook
	sources map[string]SecretSource
	cache   map[string]*sync.Map
}

// NewSecretsManager returns a new NewSecretsManager with the provided secretsHook and will redact secrets from the hook
func NewSecretsManager(sources map[string]SecretSource) (*SecretsManager, logrus.Hook, error) {
	cache := make(map[string]*sync.Map, len(sources)-1)
	hook := &secretsHook{}
	if len(sources) == 0 {
		return &SecretsManager{
			hook:  hook,
			cache: cache,
		}, hook, nil
	}
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
	return sm, hook, nil
}

// Get is the way to get a secret for the provided source name and key of the secret.
// It can be used with the [DefaultSourceName].
// This automatically starts redacting the secret before returning it.
func (sm *SecretsManager) Get(sourceName, key string) (string, error) {
	sourceCache, ok := sm.cache[sourceName]
	if !ok {
		return "", fmt.Errorf("no source with name %s", sourceName)
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
