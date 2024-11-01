package modules

import (
	"github.com/grafana/sobek"
)

// This sobek.ModuleRecord wrapper for go/js module which does not conform to modules.Module interface
type basicGoModule struct {
	m                      any
	exportedNames          []string
	exportedNamesCallbacks []func([]string)
}

func (bgm *basicGoModule) Link() error { return nil }

func (bgm *basicGoModule) Evaluate(_ *sobek.Runtime) *sobek.Promise {
	panic("this shouldn't be called")
}

func (bgm *basicGoModule) InitializeEnvironment() error { return nil }

func (bgm *basicGoModule) Instantiate(rt *sobek.Runtime) (sobek.CyclicModuleInstance, error) {
	o := rt.ToValue(bgm.m).ToObject(rt)

	// this will be called multiple times we only need to update this on the first VU
	if bgm.exportedNames == nil {
		bgm.exportedNames = o.Keys()
		for _, callback := range bgm.exportedNamesCallbacks {
			callback(bgm.exportedNames)
		}
	}
	return &basicGoModuleInstance{
		v:      o,
		module: bgm,
	}, nil
}

func (bgm *basicGoModule) RequestedModules() []string { return nil }

func (bgm *basicGoModule) ResolveExport(name string, _ ...sobek.ResolveSetElement) (*sobek.ResolvedBinding, bool) {
	return &sobek.ResolvedBinding{
		Module:      bgm,
		BindingName: name,
	}, false
}

func (bgm *basicGoModule) GetExportedNames(callback func([]string), _ ...sobek.ModuleRecord) bool {
	if bgm.exportedNames != nil {
		callback(bgm.exportedNames)
		return true
	}
	bgm.exportedNamesCallbacks = append(bgm.exportedNamesCallbacks, callback)
	return false
}

type basicGoModuleInstance struct {
	v      *sobek.Object
	module *basicGoModule
}

func (bgmi *basicGoModuleInstance) GetBindingValue(n string) sobek.Value {
	if n == jsDefaultExportIdentifier {
		return bgmi.v
	}
	return bgmi.v.Get(n)
}

func (bgmi *basicGoModuleInstance) HasTLA() bool { return false }

func (bgmi *basicGoModuleInstance) ExecuteModule(
	_ *sobek.Runtime, _, _ func(any) error,
) (sobek.CyclicModuleInstance, error) {
	return bgmi, nil
}
