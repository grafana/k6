// Package tracing implements a k6 JS module for instrumenting k6 scripts with tracing context information.
package tracing

import (
	"errors"
	"fmt"

	"github.com/dop251/goja"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
)

type (
	// RootModule is the global module instance that will create Client
	// instances for each VU.
	RootModule struct{}

	// ModuleInstance represents an instance of the JS module.
	ModuleInstance struct {
		vu modules.VU
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
	return &ModuleInstance{
		vu: vu,
	}
}

// Exports implements the modules.Instance interface and returns
// the exports of the JS module.
func (mi *ModuleInstance) Exports() modules.Exports {
	return modules.Exports{
		Named: map[string]interface{}{
			"Client": mi.newClient,
		},
	}
}

// NewClient is the JS constructor for the tracing.Client
//
// It expects a single configuration object as argument, which
// will be used to instantiate an `Object` instance internally,
// and will be used by the client to configure itself.
func (mi *ModuleInstance) newClient(cc goja.ConstructorCall) *goja.Object {
	rt := mi.vu.Runtime()

	if len(cc.Arguments) < 1 {
		common.Throw(rt, errors.New("Client constructor expects a single configuration object as argument; none given"))
	}

	var opts options
	if err := rt.ExportTo(cc.Arguments[0], &opts); err != nil {
		common.Throw(rt, fmt.Errorf("unable to parse options object; reason: %w", err))
	}

	return rt.ToValue(NewClient(mi.vu, opts)).ToObject(rt)
}
