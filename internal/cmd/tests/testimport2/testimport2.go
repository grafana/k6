// Package testimport2 hosts a second in-tree JS module type in its own Go
// package so it resolves to a distinct module path from the k6/x/testimport
// extension. Two module types in the same package would share one path and get
// de-duplicated, which would hide slot-overwrite or over-dedup bugs.
package testimport2

import "go.k6.io/k6/v2/js/modules"

// Module is a no-op JS module used only to register a second distinct extension.
type Module struct{}

// NewModuleInstance implements modules.Module.
func (*Module) NewModuleInstance(_ modules.VU) modules.Instance {
	return &instance{}
}

type instance struct{}

func (*instance) Exports() modules.Exports {
	return modules.Exports{Named: map[string]any{}}
}
