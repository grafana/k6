// Package tracing implements a k6 JS module for instrumenting k6 scripts with tracing context information.
package tracing

import (
	"go.k6.io/k6/js/modules"

	"github.com/dop251/goja"
)

type (
	// RootModule is the global module instance that will create Client
	// instances for each VU.
	RootModule struct{}

	// ModuleInstance represents an instance of the JS module.
	ModuleInstance struct {
		vu modules.VU

		*Tracing
	}
)

// Ensure the interfaces are implemented correctly
var (
	_ modules.Instance = &ModuleInstance{}
	_ modules.Module   = &RootModule{}
)

// New returns a pointer to a new RootModule instance
func New() *RootModule {
	return &RootModule{}
}

// NewModuleInstance implements the modules.Module interface and returns
// a new instance for each VU.
func (*RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	// Any goja.Value exported to a Go struct will be mapped
	// to the matching fields using the `js:""` tags.
	vu.Runtime().SetFieldNameMapper(goja.TagFieldNameMapper("js", true))

	return &ModuleInstance{
		vu: vu,
		Tracing: &Tracing{
			vu: vu,
		},
	}
}

// Exports implements the modules.Instance interface and returns
// the exports of the JS module.
func (mi *ModuleInstance) Exports() modules.Exports {
	return modules.Exports{
		Named: map[string]interface{}{
			"Client": mi.NewClient,
		},
	}
}
