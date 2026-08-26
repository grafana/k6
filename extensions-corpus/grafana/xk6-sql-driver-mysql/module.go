package mysql

import (
	"github.com/grafana/sobek"
	"go.k6.io/k6/v2/js/modules"
)

// rootModule is the global module object type.
type rootModule struct {
	driverID *sobek.Symbol
}

var _ modules.Module = &rootModule{}

// NewModuleInstance implements the modules.Module interface to return
// a new instance for each VU.
func (root *rootModule) NewModuleInstance(_ modules.VU) modules.Instance {
	instance := &module{
		exports: modules.Exports{
			Default: root.driverID,
			Named:   make(map[string]interface{}),
		},
	}

	instance.tlsExports()

	return instance
}

// module represents an instance of the JavaScript module for every VU.
type module struct {
	vu        modules.VU
	tlsConfig TLSConfig
	exports   modules.Exports
}

// Exports is representation of ESM exports of a module.
func (mod *module) Exports() modules.Exports {
	return mod.exports
}
