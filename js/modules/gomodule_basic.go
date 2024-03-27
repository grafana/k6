package modules

import (
	"sync"

	"github.com/dop251/goja"
)

// This goja.ModuleRecord wrapper for go/js module which does not conform to modules.Module interface
type baseGoModule struct {
	m             any
	once          sync.Once
	exportedNames []string
}

var (
	_ goja.CyclicModuleRecord = &baseGoModule{}
	_ goja.CyclicModuleRecord = &goModule{}
)

func (bgm *baseGoModule) Link() error { return nil }
func (bgm *baseGoModule) Evaluate(_ *goja.Runtime) *goja.Promise {
	panic("this shouldn't be called")
}

func (bgm *baseGoModule) InitializeEnvironment() error {
	return nil
}

func (bgm *baseGoModule) Instantiate(rt *goja.Runtime) (goja.CyclicModuleInstance, error) {
	o := rt.ToValue(bgm.m).ToObject(rt) // TODO checks
	bgm.once.Do(func() { bgm.exportedNames = o.Keys() })
	return &basicGoModuleInstance{
		v: o,
	}, nil
}

func (bgm *baseGoModule) RequestedModules() []string { return nil }

func (bgm *baseGoModule) ResolveExport(exportName string, _ ...goja.ResolveSetElement) (*goja.ResolvedBinding, bool) {
	return &goja.ResolvedBinding{
		Module:      bgm,
		BindingName: exportName,
	}, false
}

func (bgm *baseGoModule) GetExportedNames(_ ...goja.ModuleRecord) []string {
	bgm.once.Do(func() { panic("this shouldn't happen") })
	return bgm.exportedNames
}

type basicGoModuleInstance struct {
	v *goja.Object
}

func (bgmi *basicGoModuleInstance) GetBindingValue(n string) goja.Value {
	if n == "default" {
		return bgmi.v
	}
	return bgmi.v.Get(n)
}

func (bgmi *basicGoModuleInstance) HasTLA() bool { return false }

func (bgmi *basicGoModuleInstance) ExecuteModule(_ *goja.Runtime, _, _ func(any)) (goja.CyclicModuleInstance, error) {
	return bgmi, nil
}
