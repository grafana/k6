package modules

import (
	"sync"

	"github.com/grafana/sobek"
)

// This sobek.ModuleRecord wrapper for go/js module which does not conform to modules.Module interface
type baseGoModule struct {
	m             any
	once          sync.Once
	exportedNames []string
}

var (
	_ sobek.CyclicModuleRecord = &baseGoModule{}
	_ sobek.CyclicModuleRecord = &goModule{}
)

func (bgm *baseGoModule) Link() error { return nil }
func (bgm *baseGoModule) Evaluate(_ *sobek.Runtime) *sobek.Promise {
	panic("this shouldn't be called")
}

func (bgm *baseGoModule) InitializeEnvironment() error {
	return nil
}

func (bgm *baseGoModule) Instantiate(rt *sobek.Runtime) (sobek.CyclicModuleInstance, error) {
	o := rt.ToValue(bgm.m).ToObject(rt) // TODO checks
	bgm.once.Do(func() { bgm.exportedNames = o.Keys() })
	return &basicGoModuleInstance{
		v:      o,
		module: bgm,
	}, nil
}

func (bgm *baseGoModule) RequestedModules() []string { return nil }

func (bgm *baseGoModule) ResolveExport(exportName string, _ ...sobek.ResolveSetElement) (*sobek.ResolvedBinding, bool) {
	return &sobek.ResolvedBinding{
		Module:      bgm,
		BindingName: exportName,
	}, false
}

func (bgm *baseGoModule) GetExportedNames(callback func([]string), _ ...sobek.ModuleRecord) bool {
	bgm.once.Do(func() { panic("this shouldn't happen") })
	callback(bgm.exportedNames)
	return true
}

type basicGoModuleInstance struct {
	v      *sobek.Object
	module *baseGoModule
}

func (bgmi *basicGoModuleInstance) GetBindingValue(n string) sobek.Value {
	if n == "default" {
		return bgmi.v
	}
	return bgmi.v.Get(n)
}

func (bgmi *basicGoModuleInstance) HasTLA() bool { return false }

func (bgmi *basicGoModuleInstance) ExecuteModule(_ *sobek.Runtime, _, _ func(any)) (sobek.CyclicModuleInstance, error) {
	return bgmi, nil
}
