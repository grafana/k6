package modules

import (
	"context"

	"github.com/grafana/sobek"
	extensionapi "go.k6.io/k6-extension-api"
)

// extensionAPIModuleAdapter is the k6-owned translation layer between the
// standalone extension API and the legacy module resolver. No k6 type is
// exposed from the standalone API.
type extensionAPIModuleAdapter struct {
	module extensionapi.Module
}

func (a extensionAPIModuleAdapter) NewModuleInstance(vu VU) Instance {
	return extensionAPIInstanceAdapter{instance: a.module.NewModuleInstance(extensionAPIVU{vu: vu})}
}

type extensionAPIVU struct {
	vu VU
}

func (v extensionAPIVU) Context() context.Context {
	return v.vu.Context()
}

func (v extensionAPIVU) Runtime() *sobek.Runtime {
	return v.vu.Runtime()
}

func (v extensionAPIVU) LookupEnv(key string) (string, bool) {
	initEnv := v.vu.InitEnv()
	if initEnv == nil || initEnv.LookupEnv == nil {
		return "", false
	}

	return initEnv.LookupEnv(key)
}

type extensionAPIInstanceAdapter struct {
	instance extensionapi.Instance
}

func (a extensionAPIInstanceAdapter) Exports() Exports {
	exports := a.instance.Exports()
	return Exports{Default: exports.Default, Named: exports.Named}
}
