// Package testfork hosts an in-tree JS module type in its own Go package so it
// resolves to a distinct module path from the k6/x/testimport extension. It
// stands in for a private fork registered under a public import name: the
// catalog advertises the real extension's module path for that name, while this
// fork resolves to its own path, so matching by module path drops it.
package testfork

import "go.k6.io/k6/v2/js/modules"

// Module is a no-op JS module used only to register a fork extension whose
// module path differs from the catalogued one.
type Module struct{}

// NewModuleInstance implements modules.Module.
func (*Module) NewModuleInstance(_ modules.VU) modules.Instance {
	return &instance{}
}

type instance struct{}

func (*instance) Exports() modules.Exports {
	return modules.Exports{Named: map[string]any{}}
}
