// Package modulestest contains helpers to test js modules
package modulestest

import (
	"context"
	"testing"

	"github.com/dop251/goja"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/eventloop"
	"go.k6.io/k6/lib"
)

// Runtime is a helper struct that contains what is needed to run a (simple) module test
type Runtime struct {
	VU            *VU
	EventLoop     *eventloop.EventLoop
	CancelContext func()
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
	eventloop := eventloop.New(vu)
	vu.RegisterCallbackField = eventloop.RegisterCallback
	result := &Runtime{
		VU:            vu,
		EventLoop:     eventloop,
		CancelContext: cancel,
	}
	// let's cancel again in case it has changed
	t.Cleanup(func() { result.CancelContext() })
	return result
}

// SetInitContext will set the SetInitContext of the Runtime
func (r *Runtime) SetInitContext(i *common.InitEnvironment) {
	r.VU.InitEnvField = i
}

// MoveToVUContext will set the state and nil the InitEnv just as a real VU
func (r *Runtime) MoveToVUContext(state *lib.State) {
	r.VU.InitEnvField = nil
	r.VU.StateField = state
}
