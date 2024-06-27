package modules

import (
	"sync"

	"github.com/grafana/sobek"
)

// This sobek.ModuleRecord wrapper for go/js module which conforms to modules.Module interface
type goModule struct {
	m             Module
	once          sync.Once
	exportedNames []string
}

func (gm *goModule) Link() error {
	return nil // TDOF fix
}

func (gm *goModule) RequestedModules() []string {
	return nil
}

func (gm *goModule) InitializeEnvironment() error {
	return nil
}

func (gm *goModule) Instantiate(rt *sobek.Runtime) (sobek.CyclicModuleInstance, error) {
	vu := rt.GlobalObject().Get("vubox").Export().(vubox).vu //nolint:forcetypeassert
	mi := gm.m.NewModuleInstance(vu)
	gm.once.Do(func() {
		named := mi.Exports().Named
		gm.exportedNames = make([]string, len(named))
		for name := range named {
			gm.exportedNames = append(gm.exportedNames, name)
		}
	})
	return &goModuleInstance{rt: rt, mi: mi}, nil
}

func (gm *goModule) Evaluate(_ *sobek.Runtime) *sobek.Promise {
	panic("this shouldn't happen")
}

func (gm *goModule) GetExportedNames(callback func([]string), _ ...sobek.ModuleRecord) bool {
	gm.once.Do(func() { panic("this shouldn't happen") })
	callback(gm.exportedNames)
	return true
}

func (gm *goModule) ResolveExport(exportName string, _ ...sobek.ResolveSetElement) (*sobek.ResolvedBinding, bool) {
	return &sobek.ResolvedBinding{
		Module:      gm,
		BindingName: exportName,
	}, false
}

type goModuleInstance struct {
	Instance
	mi            Instance
	rt            *sobek.Runtime
	defaultExport sobek.Value
}

func (gmi *goModuleInstance) ExecuteModule(_ *sobek.Runtime, _, _ func(any)) (sobek.CyclicModuleInstance, error) {
	return gmi, nil
}
func (gmi *goModuleInstance) HasTLA() bool { return false }

func (gmi *goModuleInstance) GetBindingValue(name string) (v sobek.Value) {
	if name == "default" {
		return gmi.getDefaultExport()
	}

	exports := gmi.mi.Exports()
	if exports.Named != nil {
		return gmi.rt.ToValue(exports.Named[name])
	}
	return gmi.getDefaultExport().ToObject(gmi.rt).Get(name)
}

func (gmi *goModuleInstance) getDefaultExport() sobek.Value {
	if gmi.defaultExport != nil {
		return gmi.defaultExport
	}

	exports := gmi.mi.Exports()
	if exports.Default != nil {
		gmi.defaultExport = gmi.rt.ToValue(exports.Default)
		return gmi.defaultExport
	}

	// if there are only named exports we make a default object out of them
	// this allows scripts to modify this acting similar to how it would act
	// if the default export was an object to begin with.
	o := gmi.rt.NewObject()
	gmi.defaultExport = o
	for name, value := range exports.Named {
		// TODO:maybe do something slightly smarter
		_ = o.Set(name, value)
	}

	return gmi.defaultExport
}
