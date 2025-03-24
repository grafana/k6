// Package secrets implements `k6/secrets` giving access to secrets from secret sources to js code.
package secrets

import (
	"github.com/grafana/sobek"

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/js/promises"
	"go.k6.io/k6/secretsource"
)

type (
	// RootModule is the global module instance that will create module
	// instances for each VU.
	RootModule struct{}

	// Secrets represents an instance of the k6 module.
	Secrets struct {
		vu             modules.VU
		secretsManager *secretsource.Manager
	}
)

var (
	_ modules.Module   = &RootModule{}
	_ modules.Instance = &Secrets{}
)

// New returns a pointer to a new RootModule instance.
func New() *RootModule {
	return &RootModule{}
}

// NewModuleInstance implements the modules.Module interface to return
// a new instance for each VU.
func (*RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	return &Secrets{vu: vu, secretsManager: vu.InitEnv().SecretsManager}
}

// Exports returns the exports of the k6 module.
func (mi *Secrets) Exports() modules.Exports {
	s, err := mi.secrets()
	if err != nil {
		common.Throw(mi.vu.Runtime(), err)
	}
	return modules.Exports{
		Default: s,
		Named:   make(map[string]any), // this is intentionally not nil so it doesn't export anything as named exports
	}
}

func (mi *Secrets) secrets() (*sobek.Object, error) {
	obj, err := secretSourceObjectForSourceName(mi.vu, mi.secretsManager, secretsource.DefaultSourceName)
	if err != nil {
		return nil, err
	}

	err = obj.Set("source", func(sourceName string) (*sobek.Object, error) {
		return secretSourceObjectForSourceName(mi.vu, mi.secretsManager, sourceName)
	})
	if err != nil {
		return nil, err
	}

	return obj, nil
}

func secretSourceObjectForSourceName(
	vu modules.VU, manager *secretsource.Manager, sourceName string,
) (*sobek.Object, error) {
	obj := vu.Runtime().NewObject()
	err := obj.Set("get", func(key string) *sobek.Promise {
		p, resolve, reject := promises.New(vu)
		go func() {
			res, err := manager.Get(sourceName, key)
			if err != nil {
				reject(err)
				return
			}
			resolve(res)
		}()
		return p
	})
	if err != nil {
		return nil, err
	}
	return obj, nil
}
