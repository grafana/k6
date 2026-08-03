// Package xk6types is an example k6 extension that embeds its TypeScript declaration.
package xk6types

import (
	_ "embed"
	"slices"

	"go.k6.io/k6/v2/js/modules"
)

// ModuleName is the JavaScript import specifier registered by this extension.
const ModuleName = "k6/x/types-example"

//go:embed index.d.ts
var typeScriptTypes []byte

// RootModule creates one extension instance per VU.
type RootModule struct{}

// ModuleInstance exposes the extension's JavaScript API.
type ModuleInstance struct{}

var (
	_ modules.Module                 = (*RootModule)(nil)
	_ modules.TypeScriptTypeProvider = (*RootModule)(nil)
	_ modules.Instance               = (*ModuleInstance)(nil)
)

func init() {
	modules.Register(ModuleName, new(RootModule))
}

// NewModuleInstance implements modules.Module.
func (*RootModule) NewModuleInstance(modules.VU) modules.Instance {
	return new(ModuleInstance)
}

// TypeScriptTypes returns the declaration compiled into the k6 binary.
func (*RootModule) TypeScriptTypes() []byte {
	return slices.Clone(typeScriptTypes)
}

// Exports implements modules.Instance.
func (*ModuleInstance) Exports() modules.Exports {
	greet := func(name string) string { return "Hello, " + name + "!" }
	return modules.Exports{
		Default: map[string]any{"greet": greet},
		Named:   map[string]any{"greet": greet},
	}
}
