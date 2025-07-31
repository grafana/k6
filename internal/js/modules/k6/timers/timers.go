// Package timers exposes setInterval setTimeout and co. as a module
package timers

import (
	"go.k6.io/k6/js/modules"
)

// RootModule is the global module instance that will create module
// instances for each VU.
type RootModule struct{}

// Timers represents an instance of the timers module.
type Timers struct {
	vu modules.VU
}

var (
	_ modules.Module   = &RootModule{}
	_ modules.Instance = &Timers{}
)

// New returns a pointer to a new RootModule instance.
func New() *RootModule {
	return &RootModule{}
}

// NewModuleInstance implements the modules.Module interface to return
// a new instance for each VU.
func (*RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	return &Timers{
		vu: vu,
	}
}

// Exports returns the exports of the k6 module.
func (e *Timers) Exports() modules.Exports {
	globalThis := e.vu.Runtime().GlobalObject()
	return modules.Exports{
		Named: map[string]any{
			"setTimeout":    globalThis.Get("setTimeout"),
			"clearTimeout":  globalThis.Get("clearTimeout"),
			"setInterval":   globalThis.Get("setInterval"),
			"clearInterval": globalThis.Get("clearInterval"),
		},
	}
}
