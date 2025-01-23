package secretsource

import (
	"errors"
	"fmt"
	"strings"

	"go.k6.io/k6/secretsource"
)

func init() {
	secretsource.RegisterExtension("mock", func(params secretsource.Params) (secretsource.SecretSource, error) {
		list := strings.Split(params.ConfigArgument, ":")
		r := make(map[string]string, len(list))
		for _, kv := range list {
			k, v, ok := strings.Cut(kv, "=")
			if !ok {
				return nil, fmt.Errorf("parsing %q, needs =", kv)
			}

			r[k] = v
		}
		return &mockSecretSource{
			internal: r,
		}, nil
	})
}

// TODO remove - this was only for quicker testing
type mockSecretSource struct {
	internal map[string]string
}

func (mss *mockSecretSource) Name() string {
	return "some"
}

func (mss *mockSecretSource) Description() string {
	return "something cool for description"
}

func (mss *mockSecretSource) Get(key string) (string, error) {
	v, ok := mss.internal[key]
	if !ok {
		return "", errors.New("no value")
	}
	return v, nil
}
