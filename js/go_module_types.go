package js

import (
	"sync"

	"github.com/dop251/goja"
	"go.k6.io/k6/js/modules"
)

func wrapGoModule(mod interface{}) goja.ModuleRecord {
	k6m, ok := mod.(modules.Module)
	if !ok {
		return &wrappedBasicGoModule{m: mod}
	}
	return &wrappedGoModule{m: k6m}
}

// This goja.ModuleRecord wrapper for go/js module which does not conform to modules.Module interface
type wrappedBasicGoModule struct {
	m             interface{}
	once          sync.Once
	exportedNames []string
}

var (
	_ goja.CyclicModuleRecord = &wrappedBasicGoModule{}
	_ goja.CyclicModuleRecord = &wrappedGoModule{}
)

func (w *wrappedBasicGoModule) Link() error { return nil }
func (w *wrappedBasicGoModule) Evaluate(rt *goja.Runtime) *goja.Promise {
	panic("this shouldn't be called")
}

func (w *wrappedBasicGoModule) InitializeEnvironment() error {
	return nil
}

func (w *wrappedGoModule) RequestedModules() []string {
	return nil
}

func (w *wrappedGoModule) InitializeEnvironment() error {
	return nil
}

func (w *wrappedBasicGoModule) Instantiate(rt *goja.Runtime) (goja.CyclicModuleInstance, error) {
	o := rt.ToValue(w.m).ToObject(rt) // TODO checks
	w.once.Do(func() { w.exportedNames = o.Keys() })
	return &wrappedBasicGoModuleInstance{
		v: o,
	}, nil
}

func (w *wrappedBasicGoModuleInstance) HasTLA() bool { return false }
func (w *wrappedBasicGoModuleInstance) ExecuteModule(rt *goja.Runtime, _, _ func(interface{})) (goja.CyclicModuleInstance, error) {
	return w, nil
}
func (w *wrappedBasicGoModule) RequestedModules() []string { return nil }

func (w *wrappedBasicGoModule) ResolveExport(
	exportName string, set ...goja.ResolveSetElement,
) (*goja.ResolvedBinding, bool) {
	return &goja.ResolvedBinding{
		Module:      w,
		BindingName: exportName,
	}, false
}

func (w *wrappedBasicGoModule) GetExportedNames(set ...goja.ModuleRecord) []string {
	w.once.Do(func() { panic("this shouldn't happen") })
	return w.exportedNames
}

type wrappedBasicGoModuleInstance struct {
	v *goja.Object
}

func (wmi *wrappedBasicGoModuleInstance) GetBindingValue(n string) goja.Value {
	if n == "default" {
		return wmi.v
	}
	return wmi.v.Get(n)
}

// This goja.ModuleRecord wrapper for go/js module which conforms to modules.Module interface
type wrappedGoModule struct {
	m             modules.Module
	once          sync.Once
	exportedNames []string
}

func (w *wrappedGoModule) Link() error {
	return nil // TDOF fix
}

func (w *wrappedGoModule) Instantiate(rt *goja.Runtime) (goja.CyclicModuleInstance, error) {
	vu := rt.GlobalObject().Get("vugetter").Export().(vugetter).get() //nolint:forcetypeassert
	mi := w.m.NewModuleInstance(vu)
	w.once.Do(func() {
		named := mi.Exports().Named
		w.exportedNames = make([]string, len(named))
		for name := range named {
			w.exportedNames = append(w.exportedNames, name)
		}
	})
	return &wrappedGoModuleInstance{rt: rt, mi: mi}, nil
}

func (w *wrappedGoModule) Evaluate(rt *goja.Runtime) *goja.Promise {
	panic("this shouldn't happen")
}

func (w *wrappedGoModule) GetExportedNames(set ...goja.ModuleRecord) []string {
	w.once.Do(func() { panic("this shouldn't happen") })
	return w.exportedNames
}

func (w *wrappedGoModule) ResolveExport(exportName string, set ...goja.ResolveSetElement) (*goja.ResolvedBinding, bool) {
	return &goja.ResolvedBinding{
		Module:      w,
		BindingName: exportName,
	}, false
}

type wrappedGoModuleInstance struct {
	mi modules.Instance
	rt *goja.Runtime
}

func (wmi *wrappedGoModuleInstance) ExecuteModule(rt *goja.Runtime, _, _ func(interface{})) (goja.CyclicModuleInstance, error) {
	return wmi, nil
}
func (wmi *wrappedGoModuleInstance) HasTLA() bool { return false }

func (wmi *wrappedGoModuleInstance) GetBindingValue(name string) goja.Value {
	exports := wmi.mi.Exports()
	if name == "default" {
		if exports.Default == nil {
			return wmi.rt.ToValue(exports.Named)
		}
		return wmi.rt.ToValue(exports.Default)
	}
	return wmi.rt.ToValue(exports.Named[name])
}
