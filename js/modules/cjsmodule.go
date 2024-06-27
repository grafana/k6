package modules

import (
	"errors"
	"net/url"
	"sync"

	"github.com/grafana/sobek"
	"go.k6.io/k6/js/compiler"
)

// cjsModule represents a commonJS module
type cjsModule struct {
	prg                    *sobek.Program
	main                   bool
	exportedNamesCallbacks []func([]string)
	exportedNames          []string
	o                      sync.Once
}

var _ sobek.ModuleRecord = &cjsModule{}

func newCjsModule(prg *sobek.Program, main bool) sobek.ModuleRecord {
	return &cjsModule{prg: prg, main: main}
}

func (cm *cjsModule) Link() error { return nil }

func (cm *cjsModule) InitializeEnvironment() error { return nil }

func (cm *cjsModule) Instantiate(rt *sobek.Runtime) (sobek.CyclicModuleInstance, error) {
	return &cjsModuleInstance{rt: rt, w: cm}, nil
}

func (cm *cjsModule) RequestedModules() []string { return nil }

func (cm *cjsModule) Evaluate(_ *sobek.Runtime) *sobek.Promise {
	panic("this shouldn't be called in the current implementation")
}

func (cm *cjsModule) GetExportedNames(callback func([]string), _ ...sobek.ModuleRecord) bool {
	if cm.exportedNames != nil {
		callback(cm.exportedNames)
		return true
	}
	cm.exportedNamesCallbacks = append(cm.exportedNamesCallbacks, callback)
	return false
}

func (cm *cjsModule) ResolveExport(exportName string, _ ...sobek.ResolveSetElement) (*sobek.ResolvedBinding, bool) {
	return &sobek.ResolvedBinding{
		Module:      cm,
		BindingName: exportName,
	}, false
}

type cjsModuleInstance struct {
	exports          *sobek.Object
	rt               *sobek.Runtime
	w                *cjsModule
	isEsModuleMarked bool
}

func (cmi *cjsModuleInstance) HasTLA() bool { return false }
func (cmi *cjsModuleInstance) RequestedModules() []string {
	return cmi.w.RequestedModules()
}

func (cmi *cjsModuleInstance) ExecuteModule(rt *sobek.Runtime, _, _ func(any)) (sobek.CyclicModuleInstance, error) {
	v, err := rt.RunProgram(cmi.w.prg)
	if err != nil {
		return nil, err
	}

	module := rt.NewObject()
	cmi.exports = rt.NewObject()
	_ = module.Set("exports", cmi.exports)
	call, ok := sobek.AssertFunction(v)
	if !ok {
		panic("Somehow a commonjs module is not wrapped in a function - this is a k6 bug")
	}
	if _, err = call(cmi.exports, module, cmi.exports); err != nil {
		return nil, err
	}

	exportsV := module.Get("exports")
	if sobek.IsNull(exportsV) {
		return nil, errors.New("exports must be an object") // TODO make this message more specific for commonjs
	}
	cmi.exports = exportsV.ToObject(rt)

	cmi.w.o.Do(func() {
		cmi.w.exportedNames = cmi.exports.Keys()
		if cmi.w.exportedNames == nil {
			cmi.w.exportedNames = make([]string, 0)
		}
		for _, callback := range cmi.w.exportedNamesCallbacks {
			callback(cmi.w.exportedNames)
		}
	})
	__esModule := cmi.exports.Get("__esModule") //nolint:revive,stylecheck
	cmi.isEsModuleMarked = __esModule != nil && __esModule.ToBoolean()
	return cmi, nil
}

func (cmi *cjsModuleInstance) GetBindingValue(name string) sobek.Value {
	if name == "default" {
		// if wmi.w.main || wmi.isEsModuleMarked { // hack for just the main file as it worked like that before :facepalm:
		d := cmi.exports.Get("default")
		if d != nil {
			return d
		}
		//}
		return cmi.exports
	}

	return cmi.exports.Get(name)
}

// cjsModuleFromString is a helper function which returns CJSModule given the argument it has.
// It is mostly a wrapper around compiler.Compiler@Compile
//
// TODO: extract this to not make this package dependant on compilers.
// this is potentially mute point after ESM when the compiler will likely get mostly dropped.
func cjsModuleFromString(fileURL *url.URL, data []byte, c *compiler.Compiler) (sobek.ModuleRecord, error) {
	pgm, _, err := c.Compile(string(data), fileURL.String(), false)
	if err != nil {
		return nil, err
	}
	return newCjsModule(pgm, false), nil
}
