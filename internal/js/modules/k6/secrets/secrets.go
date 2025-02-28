// Package secrets implements `k6/secrets` giving access to secrets from secret sources to js code.
package secrets

import (
	"github.com/grafana/sobek"

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/secretsource"
)

type (
	// RootModule is the global module instance that will create module
	// instances for each VU.
	RootModule struct{}

	// Secrets represents an instance of the k6 module.
	Secrets struct {
		vu             modules.VU
		secretsManager *secretsource.SecretsManager
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
		Named:   make(map[string]any), // this is intentially not nil so it doesn't export anything as named expeorts
	}
}

func (mi *Secrets) secrets() (*sobek.Object, error) {
	obj, err := secretSourceObjectForSourceName(mi.vu.Runtime(), mi.secretsManager, secretsource.DefaultSourceName)
	if err != nil {
		return nil, err
	}

	err = obj.Set("source", func(sourceName string) (*sobek.Object, error) {
		return secretSourceObjectForSourceName(mi.vu.Runtime(), mi.secretsManager, sourceName)
	})
	if err != nil {
		return nil, err
	}

	return obj, nil
}

func secretSourceObjectForSourceName(
	rt *sobek.Runtime, manager *secretsource.SecretsManager, sourceName string,
) (*sobek.Object, error) {
	obj := rt.NewObject()
	err := obj.Set("get", func(key string) (string, error) {
		return manager.Get(sourceName, key)
	})
	if err != nil {
		return nil, err
	}
	return obj, nil
}
