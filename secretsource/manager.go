package secretsource

import (
	"fmt"

	"go.k6.io/k6/internal/log"
)

// SecretsManager manages secrets making certain for them to be redacted from logs
type SecretsManager struct {
	hook    *log.SecretsHook
	sources map[string]SecretSource
}

// NewSecretsManager returns a new NewSecretsManager with the provided secretsHook and will redact secrets from the hook
func NewSecretsManager(hook *log.SecretsHook, sources map[string]SecretSource) (*SecretsManager, error) {
	sm := &SecretsManager{
		hook:    hook,
		sources: sources,
	}
	return sm, nil
}

// Get is the way to get a secret for the provided source and key
// the default source key is "default" // TODO move to const
// This automatically starts redacting the secret after getting it
func (sm *SecretsManager) Get(sourceKey, key string) (string, error) {
	// TODO check a cache
	source, ok := sm.sources[sourceKey]
	if !ok {
		return "", fmt.Errorf("no source with name %s", sourceKey)
	}
	value, err := source.Get(key)
	if err != nil {
		return "", err
	}
	// TODO add to cache
	sm.hook.Add(value)
	return value, err
}
