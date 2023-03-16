package wasm

import (
	"context"

	"github.com/tetratelabs/wazero/experimental"
)

// Engine is a Store-scoped mechanism to compile functions declared or imported by a module.
// This is a top-level type implemented by an interpreter or compiler.
type Engine interface {
	// Close closes this engine, and releases all the compiled cache.
	Close() (err error)

	// CompileModule implements the same method as documented on wasm.Engine.
	CompileModule(ctx context.Context, module *Module, listeners []experimental.FunctionListener, ensureTermination bool) error

	// CompiledModuleCount is exported for testing, to track the size of the compilation cache.
	CompiledModuleCount() uint32

	// DeleteCompiledModule releases compilation caches for the given module (source).
	// Note: it is safe to call this function for a module from which module instances are instantiated even when these
	// module instances have outstanding calls.
	DeleteCompiledModule(module *Module)

	// NewModuleEngine compiles down the function instances in a module, and returns ModuleEngine for the module.
	//
	// * name is the name the module was instantiated with used for error handling.
	// * module is the source module from which moduleFunctions are instantiated. This is used for caching.
	// * functions: the list of FunctionInstance which exists in this module, including the imported ones.
	//
	// Note: Input parameters must be pre-validated with wasm.Module Validate, to ensure no fields are invalid
	// due to reasons such as out-of-bounds.
	NewModuleEngine(name string, module *Module, functions []FunctionInstance) (ModuleEngine, error)
}

// ModuleEngine implements function calls for a given module.
type ModuleEngine interface {
	// Name returns the name of the module this engine was compiled for.
	Name() string

	// NewCallEngine returns a CallEngine for the given FunctionInstance.
	NewCallEngine(callCtx *CallContext, f *FunctionInstance) (CallEngine, error)

	// LookupFunction returns the index of the function in the function table.
	LookupFunction(t *TableInstance, typeId FunctionTypeID, tableOffset Index) (Index, error)

	// FunctionInstanceReference returns Reference for the given Index for a FunctionInstance. The returned values are used by
	// the initialization via ElementSegment.
	FunctionInstanceReference(funcIndex Index) Reference
}

// CallEngine implements function calls for a FunctionInstance. It manages its own call frame stack and value stack,
// internally, and shouldn't be used concurrently.
type CallEngine interface {
	// Call invokes a function instance f with given parameters.
	Call(ctx context.Context, m *CallContext, params []uint64) (results []uint64, err error)
}
