// Package mock implements a secret source that is just taking secrets on the cli
package mock

import (
	"errors"
	"fmt"
	"strings"

	"go.k6.io/k6/secretsource"
)

func init() {
	secretsource.RegisterExtension("mock", newMockSecretSourceFromParams)
}

func newMockSecretSourceFromParams(params secretsource.Params) (secretsource.Source, error) {
	list := strings.Split(params.ConfigArgument, ",")
	secrets := make(map[string]string, len(list))
	for _, kv := range list {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			return nil, fmt.Errorf("parsing %q, needs =", kv)
		}

		secrets[k] = v
	}
	return NewMockSecretSource(secrets), nil
}

// NewMockSecretSource returns a new secret source mock with the provided name and map of secrets
func NewMockSecretSource(secrets map[string]string) secretsource.Source {
	return &mockSecretSource{
		internal: secrets,
	}
}

type mockSecretSource struct {
	internal map[string]string
}

func (mss *mockSecretSource) Description() string {
	return "this is a mock secret source"
}

func (mss *mockSecretSource) Get(key string) (string, error) {
	v, ok := mss.internal[key]
	if !ok {
		return "", errors.New("no value")
	}
	return v, nil
}
