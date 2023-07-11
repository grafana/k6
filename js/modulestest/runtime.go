// Package modulestest contains helpers to test js modules
package modulestest

import (
	"context"
	"net/url"
	"testing"

	"github.com/dop251/goja"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/compiler"
	"go.k6.io/k6/js/eventloop"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/testutils"
	"go.k6.io/k6/metrics"
)

// Runtime is a helper struct that contains what is needed to run a (simple) module test
type Runtime struct {
	VU             *VU
	EventLoop      *eventloop.EventLoop
	CancelContext  func()
	BuiltinMetrics *metrics.BuiltinMetrics

	mr *modules.ModuleResolver
}

// NewRuntime will create a new test runtime and will cancel the context on test/benchmark end
func NewRuntime(t testing.TB) *Runtime {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	vu := &VU{
		CtxField:     ctx,
		RuntimeField: goja.New(),
	}
	vu.RuntimeField.SetFieldNameMapper(common.FieldNameMapper{})
	vu.InitEnvField = &common.InitEnvironment{
		TestPreInitState: &lib.TestPreInitState{
			Logger:   testutils.NewLogger(t),
			Registry: metrics.NewRegistry(),
		},
	}

	eventloop := eventloop.New(vu)
	vu.RegisterCallbackField = eventloop.RegisterCallback
	result := &Runtime{
		VU:             vu,
		EventLoop:      eventloop,
		CancelContext:  cancel,
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(vu.InitEnvField.Registry),
	}
	// let's cancel again in case it has changed
	t.Cleanup(func() { result.CancelContext() })
	return result
}

// MoveToVUContext will set the state and nil the InitEnv just as a real VU
func (r *Runtime) MoveToVUContext(state *lib.State) {
	r.VU.InitEnvField = nil
	r.VU.StateField = state
}

// SetupModuleSystem sets up the modules system for the Runtime.
// See [modules.NewModuleResolver] for the meaning of the parameters.
func (r *Runtime) SetupModuleSystem(goModules map[string]any, loader modules.FileLoader, c *compiler.Compiler) error {
	r.mr = modules.NewModuleResolver(goModules, loader, c)
	return r.innerSetupModuleSystem()
}

// SetupModuleSystemFromAnother sets up the modules system for the Runtime by using the resolver of another runtime.
func (r *Runtime) SetupModuleSystemFromAnother(another *Runtime) error {
	r.mr = another.mr
	return r.innerSetupModuleSystem()
}

// RunOnEventLoop will run the given code on the event loop.
//
// It is meant as a helper to test code that is expected to be run on the event loop, such
// as code that returns a promise.
//
// A typical usage is to facilitate writing tests for asynchrounous code:
//
//	func TestSomething(t *testing.T) {
//	    runtime := modulestest.NewRuntime(t)
//
//	    err := runtime.RunOnEventLoop(`
//	        doSomethingAsync().then(() => {
//	            // do some assertions
//	        });
//	    `)
//	    require.NoError(t, err)
//	}
func (r *Runtime) RunOnEventLoop(code string) (value goja.Value, err error) {
	defer r.EventLoop.WaitOnRegistered()

	err = r.EventLoop.Start(func() error {
		value, err = r.VU.Runtime().RunString(code)
		return err
	})

	return value, err
}

func (r *Runtime) innerSetupModuleSystem() error {
	ms := modules.NewModuleSystem(r.mr, r.VU)
	impl := modules.NewLegacyRequireImpl(r.VU, ms, url.URL{})
	return r.VU.RuntimeField.Set("require", impl.Require)
}
