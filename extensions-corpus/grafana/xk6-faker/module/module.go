// Package module contains k6 faker JavaScript module.
package module

import (
	"strconv"

	"github.com/grafana/xk6-faker/faker"
	extensionapi "go.k6.io/k6-extension-api"
)

// rootModule is k6 JavaScript module.
type rootModule struct{}

// ImportPath contains module's JavaScript import path.
const ImportPath = "k6/x/faker"

// New creates new root module.
func New() extensionapi.Module {
	return &rootModule{}
}

func getseed(vu extensionapi.VU) int64 {
	environment, ok := vu.(extensionapi.Environment)
	if !ok {
		return 0
	}

	str, ok := environment.LookupEnv("XK6_FAKER_SEED")
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
func (root *rootModule) NewModuleInstance(vu extensionapi.VU) extensionapi.Instance {
	mod := &module{exports: extensionapi.Exports{
		Named:   make(map[string]any),
		Default: faker.New(getseed(vu), vu.Runtime()),
	}}

	mod.exports.Named["Faker"] = faker.Constructor

	return mod
}

// module is a k6 JavaScript module instance.
type module struct {
	exports extensionapi.Exports
}

// Exports is representation of ESM exports of a module.
func (mod *module) Exports() extensionapi.Exports {
	return mod.exports
}

var (
	_ extensionapi.Module   = (*rootModule)(nil)
	_ extensionapi.Instance = (*module)(nil)
)
