// Package xk6icmp adapts the real xk6-icmp module to provide its embedded TypeScript declaration.
package xk6icmp

import (
	_ "embed"
	"slices"

	icmp "github.com/grafana/xk6-icmp/icmp"
	"go.k6.io/k6/v2/js/modules"
)

// ModuleName is the JavaScript import specifier registered by xk6-icmp.
const ModuleName = icmp.ImportPath

//go:embed index.d.ts
var typeScriptTypes []byte

// RootModule delegates runtime behavior to xk6-icmp and provides its declaration to k6 tooling.
type RootModule struct {
	module modules.Module
}

var (
	_ modules.Module                 = (*RootModule)(nil)
	_ modules.TypeScriptTypeProvider = (*RootModule)(nil)
)

func init() {
	modules.Register(ModuleName, &RootModule{module: icmp.New()})
}

// NewModuleInstance implements modules.Module.
func (m *RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	return m.module.NewModuleInstance(vu)
}

// TypeScriptTypes returns xk6-icmp's declaration compiled into the custom k6 binary.
func (*RootModule) TypeScriptTypes() []byte {
	return slices.Clone(typeScriptTypes)
}
