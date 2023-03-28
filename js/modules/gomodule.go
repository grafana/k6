package modules

import (
	"github.com/dop251/goja"
)

// baseGoModule is a go module that does not implement modules.Module interface
// TODO maybe depracate those in the future
type baseGoModule struct {
	mod interface{}
}

var _ module = &baseGoModule{}

func (b *baseGoModule) instantiate(vu VU) moduleInstance {
	return &baseGoModuleInstance{mod: b.mod, vu: vu}
}

type baseGoModuleInstance struct {
	mod      interface{}
	vu       VU
	exportsO *goja.Object // this is so we only initialize the exports once per instance
}

func (b *baseGoModuleInstance) execute() error {
	return nil
}

func (b *baseGoModuleInstance) exports() *goja.Object {
	if b.exportsO == nil {
		// TODO check this does not panic a lot
		rt := b.vu.Runtime()
		b.exportsO = rt.ToValue(b.mod).ToObject(rt)
	}
	return b.exportsO
}

// goModule is a go module which implements Module
type goModule struct {
	Module
}

var _ module = &goModule{}

func (g *goModule) instantiate(vu VU) moduleInstance {
	return &goModuleInstance{vu: vu, module: g}
}

type goModuleInstance struct {
	Instance
	module   *goModule
	vu       VU
	exportsO *goja.Object // this is so we only initialize the exports once per instance
}

var _ moduleInstance = &goModuleInstance{}

func (gi *goModuleInstance) execute() error {
	gi.Instance = gi.module.NewModuleInstance(gi.vu)
	return nil
}

func (gi *goModuleInstance) exports() *goja.Object {
	if gi.exportsO == nil {
		rt := gi.vu.Runtime()
		gi.exportsO = rt.ToValue(toESModuleExports(gi.Instance.Exports())).ToObject(rt)
	}
	return gi.exportsO
}

func toESModuleExports(exp Exports) interface{} {
	if exp.Named == nil {
		return exp.Default
	}
	if exp.Default == nil {
		return exp.Named
	}

	result := make(map[string]interface{}, len(exp.Named)+2)

	for k, v := range exp.Named {
		result[k] = v
	}
	// Maybe check that those weren't set
	result["default"] = exp.Default
	// this so babel works with the `default` when it transpiles from ESM to commonjs.
	// This should probably be removed once we have support for ESM directly. So that require doesn't get support for
	// that while ESM has.
	result["__esModule"] = true

	return result
}
