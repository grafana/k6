package wasm

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/tetratelabs/wazero/api"
	internalsys "github.com/tetratelabs/wazero/internal/sys"
	"github.com/tetratelabs/wazero/sys"
)

// compile time check to ensure CallContext implements api.Module
var _ api.Module = &CallContext{}

func NewCallContext(s *Store, instance *ModuleInstance, sys *internalsys.Context) *CallContext {
	zero := uint64(0)
	return &CallContext{memory: instance.Memory, module: instance, s: s, Sys: sys, Closed: &zero}
}

// CallContext is a function call context bound to a module. This is important as one module's functions can call
// imported functions, but all need to effect the same memory.
//
// Note: This does not include the context.Context because doing so risks caching the wrong context which can break
// functionality like trace propagation.
// Note: this also implements api.Module in order to simplify usage as a host function parameter.
type CallContext struct {
	// TODO: We've never found a great name for this. It is only used for function calls, hence CallContext, but it
	// moves on a different axis than, for example, the context.Context. context.Context is the same root for the whole
	// call stack, where the CallContext can change depending on where memory is defined and who defines the calling
	// function. When we rename this again, we should try to capture as many key points possible on the docs.

	module *ModuleInstance
	// memory is returned by Memory and overridden WithMemory
	memory api.Memory
	s      *Store

	// Sys is exposed for use in special imports such as WASI, assemblyscript
	// and gojs.
	//
	// # Notes
	//
	//   - This is a part of CallContext so that scope and Close is coherent.
	//   - This is not exposed outside this repository (as a host function
	//	  parameter) because we haven't thought through capabilities based
	//	  security implications.
	Sys *internalsys.Context

	// closed is the pointer used both to guard moduleEngine.CloseWithExitCode and to store the exit code.
	//
	// The update value is closedType + exitCode << 32. This ensures an exit code of zero isn't mistaken for never closed.
	//
	// Note: Exclusively reading and updating this with atomics guarantees cross-goroutine observations.
	// See /RATIONALE.md
	Closed *uint64

	// CodeCloser is non-nil when the code should be closed after this module.
	CodeCloser api.Closer
}

// FailIfClosed returns a sys.ExitError if CloseWithExitCode was called.
func (m *CallContext) FailIfClosed() (err error) {
	if closed := atomic.LoadUint64(m.Closed); closed != 0 {
		return sys.NewExitError(m.module.Name, uint32(closed>>32)) // Unpack the high order bits as the exit code.
	}
	return nil
}

// CloseModuleOnCanceledOrTimeout take a context `ctx`, which might be a Cancel or Timeout context,
// and spawns the Goroutine to check the context is canceled ot deadline exceeded. If it reaches
// one of the conditions, it sets the appropriate exit code.
//
// Callers of this function must invoke the returned context.CancelFunc to release the spawned Goroutine.
func (m *CallContext) CloseModuleOnCanceledOrTimeout(ctx context.Context) context.CancelFunc {
	// Creating an empty channel in this case is a bit more efficient than
	// creating a context.Context and canceling it with the same effect. We
	// really just need to be notified when to stop listening to the users
	// context. Closing the channel will unblock the select in the goroutine
	// causing it to return an stop listening to ctx.Done().
	cancelChan := make(chan struct{})
	go m.closeModuleOnCanceledOrTimeout(ctx, cancelChan)
	return func() { close(cancelChan) }
}

// closeModuleOnCanceledOrTimeout is extracted from CloseModuleOnCanceledOrTimeout for testing.
func (m *CallContext) closeModuleOnCanceledOrTimeout(ctx context.Context, cancelChan <-chan struct{}) {
	select {
	case <-ctx.Done():
		select {
		case <-cancelChan:
			// In some cases by the time this goroutine is scheduled, the caller
			// has already closed both the context and the cancelChan. In this
			// case go will randomize which branch of the outer select to enter
			// and we don't want to close the module.
		default:
			m.CloseWithCtxErr(ctx)
		}
	case <-cancelChan:
	}
}

// CloseWithExitCode closes the module with an exit code based on the type of
// error reported by the context.
//
// If the context's error is unknown or nil, the module does not close.
func (m *CallContext) CloseWithCtxErr(ctx context.Context) {
	switch {
	case errors.Is(ctx.Err(), context.Canceled):
		// TODO: figure out how to report error here.
		_ = m.CloseWithExitCode(ctx, sys.ExitCodeContextCanceled)
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		// TODO: figure out how to report error here.
		_ = m.CloseWithExitCode(ctx, sys.ExitCodeDeadlineExceeded)
	}
}

// Name implements the same method as documented on api.Module
func (m *CallContext) Name() string {
	return m.module.Name
}

// WithMemory allows overriding memory without re-allocation when the result would be the same.
func (m *CallContext) WithMemory(memory *MemoryInstance) *CallContext {
	if memory != nil && memory != m.memory { // only re-allocate if it will change the effective memory
		return &CallContext{module: m.module, memory: memory, Sys: m.Sys, Closed: m.Closed}
	}
	return m
}

// String implements the same method as documented on api.Module
func (m *CallContext) String() string {
	return fmt.Sprintf("Module[%s]", m.Name())
}

// Close implements the same method as documented on api.Module.
func (m *CallContext) Close(ctx context.Context) (err error) {
	return m.CloseWithExitCode(ctx, 0)
}

// CloseWithExitCode implements the same method as documented on api.Module.
func (m *CallContext) CloseWithExitCode(ctx context.Context, exitCode uint32) (err error) {
	if !m.setExitCode(exitCode) {
		return nil // not an error to have already closed
	}
	_ = m.s.deleteModule(m.Name())
	return m.ensureResourcesClosed(ctx)
}

// closeWithExitCode is the same as CloseWithExitCode besides this doesn't delete it from Store.moduleList.
func (m *CallContext) closeWithExitCode(ctx context.Context, exitCode uint32) (err error) {
	if !m.setExitCode(exitCode) {
		return nil // not an error to have already closed
	}
	return m.ensureResourcesClosed(ctx)
}

func (m *CallContext) setExitCode(exitCode uint32) bool {
	closed := uint64(1) + uint64(exitCode)<<32 // Store exitCode as high-order bits.
	return atomic.CompareAndSwapUint64(m.Closed, 0, closed)
}

// ensureResourcesClosed ensures that resources assigned to CallContext is released.
// Multiple calls to this function is safe.
func (m *CallContext) ensureResourcesClosed(ctx context.Context) (err error) {
	if sysCtx := m.Sys; sysCtx != nil { // nil if from HostModuleBuilder
		if err = sysCtx.FS().Close(ctx); err != nil {
			return err
		}
		m.Sys = nil
	}

	if m.CodeCloser == nil {
		return
	}
	if e := m.CodeCloser.Close(ctx); e != nil && err == nil {
		err = e
	}
	m.CodeCloser = nil
	return
}

// Memory implements the same method as documented on api.Module.
func (m *CallContext) Memory() api.Memory {
	return m.module.Memory
}

// ExportedMemory implements the same method as documented on api.Module.
func (m *CallContext) ExportedMemory(name string) api.Memory {
	_, err := m.module.getExport(name, ExternTypeMemory)
	if err != nil {
		return nil
	}
	// We Assume that we have at most one memory.
	return m.memory
}

// ExportedMemoryDefinitions implements the same method as documented on
// api.Module.
func (m *CallContext) ExportedMemoryDefinitions() map[string]api.MemoryDefinition {
	// Special case as we currently only support one memory.
	if mem := m.module.Memory; mem != nil {
		// Now, find out if it is exported
		for name, exp := range m.module.Exports {
			if exp.Type == ExternTypeMemory {
				return map[string]api.MemoryDefinition{name: mem.definition}
			}
		}
	}
	return map[string]api.MemoryDefinition{}
}

// ExportedFunction implements the same method as documented on api.Module.
func (m *CallContext) ExportedFunction(name string) api.Function {
	exp, err := m.module.getExport(name, ExternTypeFunc)
	if err != nil {
		return nil
	}

	return m.function(&m.module.Functions[exp.Index])
}

// ExportedFunctionDefinitions implements the same method as documented on
// api.Module.
func (m *CallContext) ExportedFunctionDefinitions() map[string]api.FunctionDefinition {
	result := map[string]api.FunctionDefinition{}
	for name, exp := range m.module.Exports {
		if exp.Type == ExternTypeFunc {
			result[name] = m.module.Functions[exp.Index].Definition
		}
	}
	return result
}

// Module is exposed for emscripten.
func (m *CallContext) Module() *ModuleInstance {
	return m.module
}

func (m *CallContext) Function(funcIdx Index) api.Function {
	if uint32(len(m.module.Functions)) < funcIdx {
		return nil
	}
	return m.function(&m.module.Functions[funcIdx])
}

func (m *CallContext) function(f *FunctionInstance) api.Function {
	ce, err := f.Module.Engine.NewCallEngine(m, f)
	if err != nil {
		return nil
	}
	return &function{fi: f, ce: ce}
}

// function implements api.Function. This couples FunctionInstance with CallEngine so that
// it can be used to make function calls originating from the FunctionInstance.
type function struct {
	fi *FunctionInstance
	ce CallEngine
}

// Definition implements the same method as documented on api.FunctionDefinition.
func (f *function) Definition() api.FunctionDefinition {
	return f.fi.Definition
}

// Call implements the same method as documented on api.Function.
func (f *function) Call(ctx context.Context, params ...uint64) (ret []uint64, err error) {
	return f.ce.Call(ctx, f.fi.Module.CallCtx, params)
}

// GlobalVal is an internal hack to get the lower 64 bits of a global.
func (m *CallContext) GlobalVal(idx Index) uint64 {
	return m.module.Globals[idx].Val
}

// ExportedGlobal implements the same method as documented on api.Module.
func (m *CallContext) ExportedGlobal(name string) api.Global {
	exp, err := m.module.getExport(name, ExternTypeGlobal)
	if err != nil {
		return nil
	}
	g := m.module.Globals[exp.Index]
	if g.Type.Mutable {
		return &mutableGlobal{g}
	}
	valType := g.Type.ValType
	switch valType {
	case ValueTypeI32:
		return globalI32(g.Val)
	case ValueTypeI64:
		return globalI64(g.Val)
	case ValueTypeF32:
		return globalF32(g.Val)
	case ValueTypeF64:
		return globalF64(g.Val)
	default:
		panic(fmt.Errorf("BUG: unknown value type %X", valType))
	}
}
