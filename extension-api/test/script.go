package extensionapitest

import (
	"fmt"
	"os"

	"github.com/grafana/sobek"
	extensionapi "go.k6.io/k6-extension-api"
)

// ScriptRuntime is a small CommonJS test runtime for standalone extension
// modules. It intentionally provides only explicitly supplied modules; it is
// not a replacement for k6's module resolver.
type ScriptRuntime struct {
	*Runtime
	modules map[string]any
}

// NewScriptRuntime creates a standalone script runtime with the supplied
// CommonJS require() modules.
func NewScriptRuntime(modules map[string]any) *ScriptRuntime {
	scriptRuntime := &ScriptRuntime{Runtime: NewRuntime(), modules: make(map[string]any, len(modules))}
	for name, exports := range modules {
		scriptRuntime.SetModule(name, exports)
	}
	if err := scriptRuntime.VU.Runtime().Set("require", scriptRuntime.require); err != nil {
		panic(fmt.Errorf("set script require: %w", err))
	}
	return scriptRuntime
}

// SetModule adds or replaces a CommonJS module export.
func (r *ScriptRuntime) SetModule(name string, exports any) { r.modules[name] = exports }

// SetExtension adds the exports of an extension API module as a CommonJS
// module. It creates one instance for this script runtime's VU.
func (r *ScriptRuntime) SetExtension(name string, module extensionapi.Module) {
	exports := module.NewModuleInstance(r.VU).Exports()
	if exports.Default == nil {
		r.SetModule(name, exports.Named)
		return
	}
	if len(exports.Named) == 0 {
		r.SetModule(name, exports.Default)
		return
	}
	combined := make(map[string]any, len(exports.Named)+1)
	combined["default"] = exports.Default
	for exportName, value := range exports.Named {
		combined[exportName] = value
	}
	r.SetModule(name, combined)
}

func (r *ScriptRuntime) require(call sobek.FunctionCall) sobek.Value {
	name := call.Argument(0).String()
	exports, ok := r.modules[name]
	if !ok {
		panic(r.VU.Runtime().NewGoError(fmt.Errorf("unknown test module %q", name)))
	}
	return r.VU.Runtime().ToValue(exports)
}

// RunSource runs a CommonJS source file and returns module.exports.
func (r *ScriptRuntime) RunSource(filename, source string) (sobek.Value, error) {
	runtime := r.VU.Runtime()
	exports := runtime.NewObject()
	module := runtime.NewObject()
	if err := module.Set("exports", exports); err != nil {
		return nil, err
	}
	if err := runtime.Set("module", module); err != nil {
		return nil, err
	}
	if err := runtime.Set("exports", exports); err != nil {
		return nil, err
	}
	program, err := sobek.Compile(filename, source, false)
	if err != nil {
		return nil, err
	}
	if _, err = runtime.RunProgram(program); err != nil {
		return nil, err
	}
	return module.Get("exports"), nil
}

// RunFile reads and runs a CommonJS source file.
func (r *ScriptRuntime) RunFile(filename string) (sobek.Value, error) {
	source, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return r.RunSource(filename, string(source))
}

// Call invokes fn on the VU's JavaScript event loop and waits for every
// callback reserved by the extension.
func (r *ScriptRuntime) Call(fn sobek.Callable, args ...sobek.Value) (sobek.Value, error) {
	var result sobek.Value
	err := r.EventLoop.Start(func() error {
		var err error
		result, err = fn(sobek.Undefined(), args...)
		return err
	})
	return result, err
}
