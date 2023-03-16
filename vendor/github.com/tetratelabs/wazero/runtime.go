package wazero

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/tetratelabs/wazero/api"
	experimentalapi "github.com/tetratelabs/wazero/experimental"
	internalsys "github.com/tetratelabs/wazero/internal/sys"
	"github.com/tetratelabs/wazero/internal/wasm"
	binaryformat "github.com/tetratelabs/wazero/internal/wasm/binary"
	"github.com/tetratelabs/wazero/sys"
)

// Runtime allows embedding of WebAssembly modules.
//
// The below is an example of basic initialization:
//
//	ctx := context.Background()
//	r := wazero.NewRuntime(ctx)
//	defer r.Close(ctx) // This closes everything this Runtime created.
//
//	mod, _ := r.Instantiate(ctx, wasm)
type Runtime interface {
	// Instantiate instantiates a module from the WebAssembly binary (%.wasm)
	// with default configuration.
	//
	// Here's an example:
	//	ctx := context.Background()
	//	r := wazero.NewRuntime(ctx)
	//	defer r.Close(ctx) // This closes everything this Runtime created.
	//
	//	mod, _ := r.Instantiate(ctx, wasm)
	//
	// See InstantiateWithConfig for configuration overrides.
	Instantiate(ctx context.Context, source []byte) (api.Module, error)

	// InstantiateWithConfig instantiates a module from the WebAssembly binary
	// (%.wasm) or errs if invalid.
	//
	// Here's an example:
	//	ctx := context.Background()
	//	r := wazero.NewRuntime(ctx)
	//	defer r.Close(ctx) // This closes everything this Runtime created.
	//
	//	mod, _ := r.InstantiateWithConfig(ctx, wasm,
	//		wazero.NewModuleConfig().WithName("rotate"))
	//
	// # Notes
	//
	//   - This is a convenience utility that chains CompileModule with
	//     InstantiateModule. To instantiate the same source multiple times,
	//     use CompileModule as InstantiateModule avoids redundant decoding
	//     and/or compilation.
	//   - If you aren't overriding defaults, use Instantiate.
	InstantiateWithConfig(ctx context.Context, source []byte, config ModuleConfig) (api.Module, error)

	// NewHostModuleBuilder lets you create modules out of functions defined in Go.
	//
	// Below defines and instantiates a module named "env" with one function:
	//
	//	ctx := context.Background()
	//	hello := func() {
	//		fmt.Fprintln(stdout, "hello!")
	//	}
	//	_, err := r.NewHostModuleBuilder("env").
	//		NewFunctionBuilder().WithFunc(hello).Export("hello").
	//		Instantiate(ctx, r)
	NewHostModuleBuilder(moduleName string) HostModuleBuilder

	// CompileModule decodes the WebAssembly binary (%.wasm) or errs if invalid.
	// Any pre-compilation done after decoding wasm is dependent on RuntimeConfig.
	//
	// There are two main reasons to use CompileModule instead of Instantiate:
	//   - Improve performance when the same module is instantiated multiple times under different names
	//   - Reduce the amount of errors that can occur during InstantiateModule.
	//
	// # Notes
	//
	//   - The resulting module name defaults to what was binary from the custom name section.
	//   - Any pre-compilation done after decoding the source is dependent on RuntimeConfig.
	//
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#name-section%E2%91%A0
	CompileModule(ctx context.Context, binary []byte) (CompiledModule, error)

	// InstantiateModule instantiates the module or errs if the configuration was invalid.
	//
	// Here's an example:
	//	mod, _ := n.InstantiateModule(ctx, compiled, wazero.NewModuleConfig().WithName("prod"))
	//
	// While CompiledModule is pre-validated, there are a few situations which can cause an error:
	//   - The module name is already in use.
	//   - The module has a table element initializer that resolves to an index outside the Table minimum size.
	//   - The module has a start function, and it failed to execute.
	InstantiateModule(ctx context.Context, compiled CompiledModule, config ModuleConfig) (api.Module, error)

	// CloseWithExitCode closes all the modules that have been initialized in this Runtime with the provided exit code.
	// An error is returned if any module returns an error when closed.
	//
	// Here's an example:
	//	ctx := context.Background()
	//	r := wazero.NewRuntime(ctx)
	//	defer r.CloseWithExitCode(ctx, 2) // This closes everything this Runtime created.
	//
	//	// Everything below here can be closed, but will anyway due to above.
	//	_, _ = wasi_snapshot_preview1.InstantiateSnapshotPreview1(ctx, r)
	//	mod, _ := r.Instantiate(ctx, wasm)
	CloseWithExitCode(ctx context.Context, exitCode uint32) error

	// Module returns an instantiated module in this runtime or nil if there aren't any.
	Module(moduleName string) api.Module

	// Closer closes all compiled code by delegating to CloseWithExitCode with an exit code of zero.
	api.Closer
}

// NewRuntime returns a runtime with a configuration assigned by NewRuntimeConfig.
func NewRuntime(ctx context.Context) Runtime {
	return NewRuntimeWithConfig(ctx, NewRuntimeConfig())
}

// NewRuntimeWithConfig returns a runtime with the given configuration.
func NewRuntimeWithConfig(ctx context.Context, rConfig RuntimeConfig) Runtime {
	config := rConfig.(*runtimeConfig)
	var engine wasm.Engine
	var cacheImpl *cache
	if c := config.cache; c != nil {
		// If the Cache is configured, we share the engine.
		cacheImpl = c.(*cache)
		engine = cacheImpl.initEngine(config.engineKind, config.newEngine, ctx, config.enabledFeatures)
	} else {
		// Otherwise, we create a new engine.
		engine = config.newEngine(ctx, config.enabledFeatures, nil)
	}
	store := wasm.NewStore(config.enabledFeatures, engine)
	zero := uint64(0)
	return &runtime{
		cache:                 cacheImpl,
		store:                 store,
		enabledFeatures:       config.enabledFeatures,
		memoryLimitPages:      config.memoryLimitPages,
		memoryCapacityFromMax: config.memoryCapacityFromMax,
		dwarfDisabled:         config.dwarfDisabled,
		storeCustomSections:   config.storeCustomSections,
		closed:                &zero,
		ensureTermination:     config.ensureTermination,
	}
}

// runtime allows decoupling of public interfaces from internal representation.
type runtime struct {
	store                 *wasm.Store
	cache                 *cache
	enabledFeatures       api.CoreFeatures
	memoryLimitPages      uint32
	memoryCapacityFromMax bool
	dwarfDisabled         bool
	storeCustomSections   bool

	// closed is the pointer used both to guard moduleEngine.CloseWithExitCode and to store the exit code.
	//
	// The update value is 1 + exitCode << 32. This ensures an exit code of zero isn't mistaken for never closed.
	//
	// Note: Exclusively reading and updating this with atomics guarantees cross-goroutine observations.
	// See /RATIONALE.md
	closed *uint64

	ensureTermination bool
}

// Module implements Runtime.Module.
func (r *runtime) Module(moduleName string) api.Module {
	return r.store.Module(moduleName)
}

// CompileModule implements Runtime.CompileModule
func (r *runtime) CompileModule(ctx context.Context, binary []byte) (CompiledModule, error) {
	if err := r.failIfClosed(); err != nil {
		return nil, err
	}

	if binary == nil {
		return nil, errors.New("binary == nil")
	}

	internal, err := binaryformat.DecodeModule(binary, r.enabledFeatures,
		r.memoryLimitPages, r.memoryCapacityFromMax, !r.dwarfDisabled, r.storeCustomSections)
	if err != nil {
		return nil, err
	} else if err = internal.Validate(r.enabledFeatures); err != nil {
		// TODO: decoders should validate before returning, as that allows
		// them to err with the correct position in the wasm binary.
		return nil, err
	}

	internal.AssignModuleID(binary)

	// Now that the module is validated, cache the function and memory definitions.
	internal.BuildFunctionDefinitions()
	internal.BuildMemoryDefinitions()

	c := &compiledModule{module: internal, compiledEngine: r.store.Engine}

	listeners, err := buildListeners(ctx, internal)
	if err != nil {
		return nil, err
	}

	if err = r.store.Engine.CompileModule(ctx, internal, listeners, r.ensureTermination); err != nil {
		return nil, err
	}
	return c, nil
}

func buildListeners(ctx context.Context, internal *wasm.Module) ([]experimentalapi.FunctionListener, error) {
	// Test to see if internal code are using an experimental feature.
	fnlf := ctx.Value(experimentalapi.FunctionListenerFactoryKey{})
	if fnlf == nil {
		return nil, nil
	}
	factory := fnlf.(experimentalapi.FunctionListenerFactory)
	importCount := internal.ImportFuncCount()
	listeners := make([]experimentalapi.FunctionListener, len(internal.FunctionSection))
	for i := 0; i < len(listeners); i++ {
		listeners[i] = factory.NewListener(internal.FunctionDefinitionSection[uint32(i)+importCount])
	}
	return listeners, nil
}

// failIfClosed returns an error if CloseWithExitCode was called implicitly (by Close) or explicitly.
func (r *runtime) failIfClosed() error {
	if closed := atomic.LoadUint64(r.closed); closed != 0 {
		return fmt.Errorf("runtime closed with exit_code(%d)", uint32(closed>>32))
	}
	return nil
}

// Instantiate implements Runtime.Instantiate
func (r *runtime) Instantiate(ctx context.Context, binary []byte) (api.Module, error) {
	return r.InstantiateWithConfig(ctx, binary, NewModuleConfig())
}

// InstantiateWithConfig implements Runtime.InstantiateWithConfig
func (r *runtime) InstantiateWithConfig(ctx context.Context, binary []byte, config ModuleConfig) (api.Module, error) {
	if compiled, err := r.CompileModule(ctx, binary); err != nil {
		return nil, err
	} else {
		compiled.(*compiledModule).closeWithModule = true
		return r.InstantiateModule(ctx, compiled, config)
	}
}

// InstantiateModule implements Runtime.InstantiateModule.
func (r *runtime) InstantiateModule(
	ctx context.Context,
	compiled CompiledModule,
	mConfig ModuleConfig,
) (mod api.Module, err error) {
	if err = r.failIfClosed(); err != nil {
		return nil, err
	}

	code := compiled.(*compiledModule)
	config := mConfig.(*moduleConfig)

	var sysCtx *internalsys.Context
	if sysCtx, err = config.toSysContext(); err != nil {
		return
	}

	name := config.name
	if name == "" && code.module.NameSection != nil && code.module.NameSection.ModuleName != "" {
		name = code.module.NameSection.ModuleName
	}

	// Instantiate the module.
	mod, err = r.store.Instantiate(ctx, code.module, name, sysCtx)
	if err != nil {
		// If there was an error, don't leak the compiled module.
		if code.closeWithModule {
			_ = code.Close(ctx) // don't overwrite the error
		}
		return
	}

	// Attach the code closer so that anything afterwards closes the compiled code when closing the module.
	if code.closeWithModule {
		mod.(*wasm.CallContext).CodeCloser = code
	}

	// Now, invoke any start functions, failing at first error.
	for _, fn := range config.startFunctions {
		start := mod.ExportedFunction(fn)
		if start == nil {
			continue
		}
		if _, err = start.Call(ctx); err != nil {
			_ = mod.Close(ctx) // Don't leak the module on error.
			if _, ok := err.(*sys.ExitError); ok {
				return // Don't wrap an exit error
			}
			err = fmt.Errorf("module[%s] function[%s] failed: %w", name, fn, err)
			return
		}
	}
	return
}

// Close implements api.Closer embedded in Runtime.
func (r *runtime) Close(ctx context.Context) error {
	return r.CloseWithExitCode(ctx, 0)
}

// CloseWithExitCode implements Runtime.CloseWithExitCode
//
// Note: it also marks the internal `closed` field
func (r *runtime) CloseWithExitCode(ctx context.Context, exitCode uint32) error {
	closed := uint64(1) + uint64(exitCode)<<32 // Store exitCode as high-order bits.
	if !atomic.CompareAndSwapUint64(r.closed, 0, closed) {
		return nil
	}
	err := r.store.CloseWithExitCode(ctx, exitCode)
	if r.cache == nil {
		// Close the engine if the cache is not configured, which means that this engine is scoped in this runtime.
		if errCloseEngine := r.store.Engine.Close(); errCloseEngine != nil {
			return errCloseEngine
		}
	}
	return err
}
