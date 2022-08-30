package js

import (
	"errors"
	"sync"

	"github.com/dop251/goja"
)

// TODO a lot of tests around this
type wrappedCommonJS struct {
	prg           *goja.Program
	main          bool
	exportedNames []string
	o             sync.Once
}

var _ goja.CyclicModuleRecord = &wrappedCommonJS{}

func wrapCommonJS(prg *goja.Program, main bool) goja.ModuleRecord {
	return &wrappedCommonJS{prg: prg, main: main}
}

func (w *wrappedCommonJS) Link() error {
	return nil // TODO fix
}

func (w *wrappedCommonJS) InitializeEnvironment() error {
	return nil // TODO fix
}

func (w *wrappedCommonJS) Instantiate(rt *goja.Runtime) (goja.CyclicModuleInstance, error) {
	return &wrappedCommonJSInstance{w: w, rt: rt}, nil
}

func (w *wrappedCommonJS) RequestedModules() []string {
	return nil // TODO fix
}

func (w *wrappedCommonJS) Evaluate(rt *goja.Runtime) *goja.Promise {
	panic("this should be called in the current implementation")
}

func (w wrappedCommonJS) GetExportedNames(set ...goja.ModuleRecord) []string {
	w.o.Do(func() {
		panic("some how we first got to GetExportedNames of a commonjs module before they were set" +
			"- this should never happen and is some kind of a bug")
	})
	return w.exportedNames
}

func (w *wrappedCommonJS) ResolveExport(
	exportName string, set ...goja.ResolveSetElement,
) (*goja.ResolvedBinding, bool) {
	return &goja.ResolvedBinding{
		Module:      w,
		BindingName: exportName,
	}, false
}

type wrappedCommonJSInstance struct {
	exports          *goja.Object
	rt               *goja.Runtime
	w                *wrappedCommonJS
	isEsModuleMarked bool
}

func (wmi *wrappedCommonJSInstance) HasTLA() bool { return false }
func (wmi *wrappedCommonJSInstance) RequestedModules() []string {
	return wmi.w.RequestedModules()
}

func (wmi *wrappedCommonJSInstance) ExecuteModule(rt *goja.Runtime, _, _ func(interface{})) (goja.CyclicModuleInstance, error) {
	v, err := rt.RunProgram(wmi.w.prg)
	if err != nil {
		return nil, err
	}

	module := rt.NewObject()
	wmi.exports = rt.NewObject()
	_ = module.Set("exports", wmi.exports)
	call, ok := goja.AssertFunction(v)
	if !ok {
		panic("Somehow a commonjs module is not wrapped in a function - this is a k6 bug")
	}
	if _, err = call(wmi.exports, module, wmi.exports); err != nil {
		return nil, err
	}
	exportsV := module.Get("exports")
	if goja.IsNull(exportsV) {
		return nil, errors.New("commonjs exports must be an object")
	}
	wmi.exports = exportsV.ToObject(rt)

	wmi.w.o.Do(func() {
		wmi.w.exportedNames = wmi.exports.Keys()
	})
	__esModule := wmi.exports.Get("__esModule")
	wmi.isEsModuleMarked = __esModule != nil && __esModule.ToBoolean()
	return wmi, nil
}

func (wmi *wrappedCommonJSInstance) GetBindingValue(name string) goja.Value {
	if name == "default" {
		if wmi.w.main || wmi.isEsModuleMarked { // hack for just the main file as it worked like that before :facepalm:
			d := wmi.exports.Get("default")
			if d != nil {
				return d
			}
		}
		return wmi.exports
	}
	return wmi.exports.Get(name)
}
