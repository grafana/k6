package modules

import "github.com/grafana/sobek"

type unknownModule struct {
	name      string
	requested map[string]struct{}
}

func (um *unknownModule) Link() error { return nil }

func (um *unknownModule) Evaluate(_ *sobek.Runtime) *sobek.Promise { panic("this shouldn't be called") }

func (um *unknownModule) InitializeEnvironment() error { return nil }

func (um *unknownModule) Instantiate(_ *sobek.Runtime) (sobek.CyclicModuleInstance, error) {
	return &unknownModuleInstance{module: um}, nil
}

func (um *unknownModule) RequestedModules() []string { return nil }

func (um *unknownModule) ResolveExport(name string, _ ...sobek.ResolveSetElement) (*sobek.ResolvedBinding, bool) {
	um.requested[name] = struct{}{}
	return &sobek.ResolvedBinding{
		Module:      um,
		BindingName: name,
	}, false
}

func (um *unknownModule) GetExportedNames(_ func([]string), _ ...sobek.ModuleRecord) bool {
	return false
}

type unknownModuleInstance struct {
	module *unknownModule
}

func (umi *unknownModuleInstance) GetBindingValue(_ string) sobek.Value {
	return nil
}

func (umi *unknownModuleInstance) HasTLA() bool { return false }

func (umi *unknownModuleInstance) ExecuteModule(_ *sobek.Runtime, _, _ func(any) error,
) (sobek.CyclicModuleInstance, error) {
	return umi, nil
}
