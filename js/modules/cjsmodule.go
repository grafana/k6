package modules

import (
	"errors"

	"github.com/grafana/sobek"
	"github.com/grafana/sobek/ast"
)

// cjsModule represents a commonJS module
type cjsModule struct {
	prg                    *sobek.Program
	exportedNames          []string
	exportedNamesCallbacks []func([]string)
}

func newCjsModule(prg *sobek.Program) sobek.ModuleRecord {
	return &cjsModule{prg: prg}
}

func (cm *cjsModule) Link() error { return nil }

func (cm *cjsModule) InitializeEnvironment() error { return nil }

func (cm *cjsModule) Instantiate(_ *sobek.Runtime) (sobek.CyclicModuleInstance, error) {
	return &cjsModuleInstance{w: cm}, nil
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
	w                *cjsModule
	exports          *sobek.Object
	isEsModuleMarked bool
}

func (cmi *cjsModuleInstance) HasTLA() bool { return false }

func (cmi *cjsModuleInstance) RequestedModules() []string { return cmi.w.RequestedModules() }

func (cmi *cjsModuleInstance) ExecuteModule(
	rt *sobek.Runtime, _, _ func(any) error,
) (sobek.CyclicModuleInstance, error) {
	v, err := rt.RunProgram(cmi.w.prg)
	if err != nil {
		return nil, err
	}

	module := rt.NewObject()
	cmi.exports = rt.NewObject()
	_ = module.Set("exports", cmi.exports)
	call, ok := sobek.AssertFunction(v)
	if !ok {
		panic("Somehow a CommonJS module is not wrapped in a function - " +
			"this is a k6 bug, please report it (https://github.com/grafana/k6/issues)")
	}
	if _, err = call(cmi.exports, module, cmi.exports); err != nil {
		return nil, err
	}

	exportsV := module.Get("exports")
	if sobek.IsNull(exportsV) {
		return nil, errors.New("CommonJS's exports must not be null")
	}
	cmi.exports = exportsV.ToObject(rt)

	// this will be called multiple times we only need to update this on the first VU
	if cmi.w.exportedNames == nil {
		cmi.w.exportedNames = cmi.exports.Keys()
		if cmi.w.exportedNames == nil { // workaround if a CommonJS module does not have exports
			cmi.w.exportedNames = make([]string, 0)
		}
		for _, callback := range cmi.w.exportedNamesCallbacks {
			callback(cmi.w.exportedNames)
		}
	}
	__esModule := cmi.exports.Get("__esModule") //nolint:revive,stylecheck
	cmi.isEsModuleMarked = __esModule != nil && __esModule.ToBoolean()
	return cmi, nil
}

func (cmi *cjsModuleInstance) GetBindingValue(name string) sobek.Value {
	if name == jsDefaultExportIdentifier {
		d := cmi.exports.Get(jsDefaultExportIdentifier)
		if d != nil {
			return d
		}
		return cmi.exports
	}

	return cmi.exports.Get(name)
}

// cjsModuleFromString is a helper function which returns CJSModule given the argument it has.
func cjsModuleFromString(prg *ast.Program) (sobek.ModuleRecord, error) {
	pgm, err := sobek.CompileAST(prg, true)
	if err != nil {
		return nil, err
	}
	return newCjsModule(pgm), nil
}
