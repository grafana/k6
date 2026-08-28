package mysql

import (
	"github.com/grafana/sobek"
	extensionapi "go.k6.io/k6-extension-api"
)

// rootModule is the global module object type.
type rootModule struct {
	driverID *sobek.Symbol
}

var _ extensionapi.Module = &rootModule{}

// NewModuleInstance implements the modules.Module interface to return
// a new instance for each VU.
func (root *rootModule) NewModuleInstance(_ extensionapi.VU) extensionapi.Instance {
	instance := &module{
		exports: extensionapi.Exports{
			Default: root.driverID,
			Named:   make(map[string]interface{}),
		},
	}

	instance.tlsExports()

	return instance
}

// module represents an instance of the JavaScript module for every VU.
type module struct {
	vu        extensionapi.VU
	tlsConfig TLSConfig
	exports   extensionapi.Exports
}

// Exports is representation of ESM exports of a module.
func (mod *module) Exports() extensionapi.Exports {
	return mod.exports
}
