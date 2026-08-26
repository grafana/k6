// Package module contains k6 faker JavaScript module.
package module

import (
	"strconv"

	"github.com/grafana/xk6-faker/faker"
	"go.k6.io/k6/v2/js/modules"
)

// rootModule is k6 JavaScript module.
type rootModule struct{}

// ImportPath contains module's JavaScript import path.
const ImportPath = "k6/x/faker"

// New creates new root module.
func New() modules.Module {
	return &rootModule{}
}

func getseed(vu modules.VU) int64 {
	if vu == nil || vu.InitEnv() == nil || vu.InitEnv().LookupEnv == nil {
		return 0
	}

	str, ok := vu.InitEnv().LookupEnv("XK6_FAKER_SEED")
	if !ok {
		return 0
	}

	val, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return 0
	}

	return val
}

// NewModuleInstance creates new module instance.
func (root *rootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	mod := &module{exports: modules.Exports{
		Named:   make(map[string]any),
		Default: faker.New(getseed(vu), vu.Runtime()),
	}}

	mod.exports.Named["Faker"] = faker.Constructor

	return mod
}

// module is a k6 JavaScript module instance.
type module struct {
	exports modules.Exports
}

// Exports is representation of ESM exports of a module.
func (mod *module) Exports() modules.Exports {
	return mod.exports
}

var (
	_ modules.Module   = (*rootModule)(nil)
	_ modules.Instance = (*module)(nil)
)
