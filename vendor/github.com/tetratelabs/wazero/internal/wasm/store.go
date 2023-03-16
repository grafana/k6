package wasm

import (
	"context"
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/ieee754"
	"github.com/tetratelabs/wazero/internal/leb128"
	internalsys "github.com/tetratelabs/wazero/internal/sys"
	"github.com/tetratelabs/wazero/sys"
)

type (
	// Store is the runtime representation of "instantiated" Wasm module and objects.
	// Multiple modules can be instantiated within a single store, and each instance,
	// (e.g. function instance) can be referenced by other module instances in a Store via Module.ImportSection.
	//
	// Every type whose name ends with "Instance" suffix belongs to exactly one store.
	//
	// Note that store is not thread (concurrency) safe, meaning that using single Store
	// via multiple goroutines might result in race conditions. In that case, the invocation
	// and access to any methods and field of Store must be guarded by mutex.
	//
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#store%E2%91%A0
	Store struct {
		// moduleList ensures modules are closed in reverse initialization order.
		moduleList *moduleListNode // guarded by mux

		// nameToNode holds the instantiated Wasm modules by module name from Instantiate.
		// It ensures no race conditions instantiating two modules of the same name.
		nameToNode map[string]*moduleListNode // guarded by mux

		// EnabledFeatures are read-only to allow optimizations.
		EnabledFeatures api.CoreFeatures

		// Engine is a global context for a Store which is in responsible for compilation and execution of Wasm modules.
		Engine Engine

		// typeIDs maps each FunctionType.String() to a unique FunctionTypeID. This is used at runtime to
		// do type-checks on indirect function calls.
		typeIDs map[string]FunctionTypeID

		// functionMaxTypes represents the limit on the number of function types in a store.
		// Note: this is fixed to 2^27 but have this a field for testability.
		functionMaxTypes uint32

		// mux is used to guard the fields from concurrent access.
		mux sync.RWMutex
	}

	// ModuleInstance represents instantiated wasm module.
	// The difference from the spec is that in wazero, a ModuleInstance holds pointers
	// to the instances, rather than "addresses" (i.e. index to Store.Functions, Globals, etc) for convenience.
	//
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#syntax-moduleinst
	ModuleInstance struct {
		Name      string
		Exports   map[string]ExportInstance
		Functions []FunctionInstance
		Globals   []*GlobalInstance
		// Memory is set when Module.MemorySection had a memory, regardless of whether it was exported.
		Memory *MemoryInstance
		Tables []*TableInstance

		// CallCtx holds default function call context from this function instance.
		CallCtx *CallContext

		// Engine implements function calls for this module.
		Engine ModuleEngine

		// TypeIDs is index-correlated with types and holds typeIDs which is uniquely assigned to a type by store.
		// This is necessary to achieve fast runtime type checking for indirect function calls at runtime.
		TypeIDs []FunctionTypeID

		// DataInstances holds data segments bytes of the module.
		// This is only used by bulk memory operations.
		//
		// https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/exec/runtime.html#data-instances
		DataInstances []DataInstance

		// ElementInstances holds the element instance, and each holds the references to either functions
		// or external objects (unimplemented).
		ElementInstances []ElementInstance
	}

	// DataInstance holds bytes corresponding to the data segment in a module.
	//
	// https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/exec/runtime.html#data-instances
	DataInstance = []byte

	// ExportInstance represents an exported instance in a Store.
	// The difference from the spec is that in wazero, a ExportInstance holds pointers
	// to the instances, rather than "addresses" (i.e. index to Store.Functions, Globals, etc) for convenience.
	//
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#syntax-exportinst
	ExportInstance struct {
		Type  ExternType
		Index Index
	}

	// FunctionInstance represents a function instance in a Store.
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#function-instances%E2%91%A0
	FunctionInstance struct {
		// Type is the signature of this function.
		Type *FunctionType

		// Fields above here are settable prior to instantiation. Below are set by the Store during instantiation.

		// ModuleInstance holds the pointer to the module instance to which this function belongs.
		Module *ModuleInstance

		// TypeID is assigned by a store for FunctionType.
		TypeID FunctionTypeID

		// Idx holds the index of this function instance in the function index (beginning with imports).
		Idx Index

		// Definition is known at compile time.
		Definition api.FunctionDefinition
	}

	// GlobalInstance represents a global instance in a store.
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#global-instances%E2%91%A0
	GlobalInstance struct {
		Type *GlobalType
		// Val holds a 64-bit representation of the actual value.
		Val uint64
		// ValHi is only used for vector type globals, and holds the higher bits of the vector.
		ValHi uint64
		// ^^ TODO: this should be guarded with atomics when mutable
	}

	// FunctionTypeID is a uniquely assigned integer for a function type.
	// This is wazero specific runtime object and specific to a store,
	// and used at runtime to do type-checks on indirect function calls.
	FunctionTypeID uint32
)

// The wazero specific limitations described at RATIONALE.md.
const maximumFunctionTypes = 1 << 27

// addSections adds section elements to the ModuleInstance
func (m *ModuleInstance) addSections(module *Module, importedGlobals, globals []*GlobalInstance, tables []*TableInstance, memory, importedMemory *MemoryInstance) {
	m.Globals = append(importedGlobals, globals...)
	m.Tables = tables

	if importedMemory != nil {
		m.Memory = importedMemory
	} else {
		m.Memory = memory
	}

	m.BuildExports(module.ExportSection)
}

func (m *ModuleInstance) buildElementInstances(elements []*ElementSegment) {
	m.ElementInstances = make([]ElementInstance, len(elements))
	for i, elm := range elements {
		if elm.Type == RefTypeFuncref && elm.Mode == ElementModePassive {
			// Only passive elements can be access as element instances.
			// See https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/syntax/modules.html#element-segments
			inits := elm.Init
			elemInst := &m.ElementInstances[i]
			elemInst.References = make([]Reference, len(inits))
			elemInst.Type = RefTypeFuncref
			for j, idxPtr := range inits {
				if idxPtr != nil {
					idx := *idxPtr
					elemInst.References[j] = m.Engine.FunctionInstanceReference(idx)
				}
			}
		}
	}
}

func (m *ModuleInstance) applyTableInits(tables []*TableInstance, tableInits []tableInitEntry) {
	for _, init := range tableInits {
		table := tables[init.tableIndex]
		references := table.References
		if int(init.offset)+len(init.functionIndexes) > len(references) ||
			int(init.offset)+init.nullExternRefCount > len(references) {
			// ErrElementOffsetOutOfBounds is the error raised when the active element offset exceeds the table length.
			// Before CoreFeatureReferenceTypes, this was checked statically before instantiation, after the proposal,
			// this must be raised as runtime error (as in assert_trap in spectest), not even an instantiation error.
			// https://github.com/WebAssembly/spec/blob/d39195773112a22b245ffbe864bab6d1182ccb06/test/core/linking.wast#L264-L274
			//
			// In wazero, we ignore it since in any way, the instantiated module and engines are fine and can be used
			// for function invocations.
			return
		}

		if table.Type == RefTypeExternref {
			for i := 0; i < init.nullExternRefCount; i++ {
				references[init.offset+uint32(i)] = Reference(0)
			}
		} else {
			for i, fnIndex := range init.functionIndexes {
				if fnIndex != nil {
					references[init.offset+uint32(i)] = m.Engine.FunctionInstanceReference(*fnIndex)
				}
			}
		}
	}
}

func (m *ModuleInstance) BuildExports(exports []*Export) {
	m.Exports = make(map[string]ExportInstance, len(exports))
	for _, exp := range exports {
		// We already validated the duplicates during module validation phase.
		m.Exports[exp.Name] = ExportInstance{Type: exp.Type, Index: exp.Index}
	}
}

// validateData ensures that data segments are valid in terms of memory boundary.
// Note: this is used only when bulk-memory/reference type feature is disabled.
func (m *ModuleInstance) validateData(data []*DataSegment) (err error) {
	for i, d := range data {
		if !d.IsPassive() {
			offset := int(executeConstExpression(m.Globals, d.OffsetExpression).(int32))
			ceil := offset + len(d.Init)
			if offset < 0 || ceil > len(m.Memory.Buffer) {
				return fmt.Errorf("%s[%d]: out of bounds memory access", SectionIDName(SectionIDData), i)
			}
		}
	}
	return
}

// applyData uses the given data segments and mutate the memory according to the initial contents on it
// and populate the `DataInstances`. This is called after all the validation phase passes and out of
// bounds memory access error here is not a validation error, but rather a runtime error.
func (m *ModuleInstance) applyData(data []*DataSegment) error {
	m.DataInstances = make([][]byte, len(data))
	for i, d := range data {
		m.DataInstances[i] = d.Init
		if !d.IsPassive() {
			offset := executeConstExpression(m.Globals, d.OffsetExpression).(int32)
			if offset < 0 || int(offset)+len(d.Init) > len(m.Memory.Buffer) {
				return fmt.Errorf("%s[%d]: out of bounds memory access", SectionIDName(SectionIDData), i)
			}
			copy(m.Memory.Buffer[offset:], d.Init)
		}
	}
	return nil
}

// GetExport returns an export of the given name and type or errs if not exported or the wrong type.
func (m *ModuleInstance) getExport(name string, et ExternType) (ExportInstance, error) {
	exp, ok := m.Exports[name]
	if !ok {
		return ExportInstance{}, fmt.Errorf("%q is not exported in module %q", name, m.Name)
	}
	if exp.Type != et {
		return ExportInstance{}, fmt.Errorf("export %q in module %q is a %s, not a %s", name, m.Name, ExternTypeName(exp.Type), ExternTypeName(et))
	}
	return exp, nil
}

func NewStore(enabledFeatures api.CoreFeatures, engine Engine) *Store {
	typeIDs := make(map[string]FunctionTypeID, len(preAllocatedTypeIDs))
	for k, v := range preAllocatedTypeIDs {
		typeIDs[k] = v
	}
	return &Store{
		nameToNode:       map[string]*moduleListNode{},
		EnabledFeatures:  enabledFeatures,
		Engine:           engine,
		typeIDs:          typeIDs,
		functionMaxTypes: maximumFunctionTypes,
	}
}

// Instantiate uses name instead of the Module.NameSection ModuleName as it allows instantiating the same module under
// different names safely and concurrently.
//
// * ctx: the default context used for function calls.
// * name: the name of the module.
// * sys: the system context, which will be closed (SysContext.Close) on CallContext.Close.
//
// Note: Module.Validate must be called prior to instantiation.
func (s *Store) Instantiate(
	ctx context.Context,
	module *Module,
	name string,
	sys *internalsys.Context,
) (*CallContext, error) {
	// Collect any imported modules to avoid locking the store too long.
	importedModuleNames := map[string]struct{}{}
	for _, i := range module.ImportSection {
		importedModuleNames[i.Module] = struct{}{}
	}

	// Read-Lock the store and ensure imports needed are present.
	importedModules, err := s.requireModules(importedModuleNames)
	if err != nil {
		return nil, err
	}

	// Write-Lock the store and claim the name of the current module.
	if err = s.requireModuleName(name); err != nil {
		return nil, err
	}

	// Instantiate the module and add it to the store so that other modules can import it.
	if callCtx, err := s.instantiate(ctx, module, name, sys, importedModules); err != nil {
		_ = s.deleteModule(name)
		return nil, err
	} else {
		// Now that the instantiation is complete without error, add it.
		// This makes the module visible for import, and ensures it is closed when the store is.
		if err := s.setModule(callCtx.module); err != nil {
			callCtx.Close(ctx)
			return nil, err
		}
		return callCtx, nil
	}
}

func (s *Store) instantiate(
	ctx context.Context,
	module *Module,
	name string,
	sysCtx *internalsys.Context,
	modules map[string]*ModuleInstance,
) (*CallContext, error) {
	typeIDs, err := s.getFunctionTypeIDs(module.TypeSection)
	if err != nil {
		return nil, err
	}

	importedFunctions, importedGlobals, importedTables, importedMemory, err := resolveImports(module, modules)
	if err != nil {
		return nil, err
	}

	tables, tableInit, err := module.buildTables(importedTables, importedGlobals,
		// As of reference-types proposal, boundary check must be done after instantiation.
		s.EnabledFeatures.IsEnabled(api.CoreFeatureReferenceTypes))
	if err != nil {
		return nil, err
	}

	m := &ModuleInstance{Name: name, TypeIDs: typeIDs}
	functions := m.BuildFunctions(module, importedFunctions)

	// Plus, we are ready to compile functions.
	m.Engine, err = s.Engine.NewModuleEngine(name, module, functions)
	if err != nil {
		return nil, err
	}

	globals, memory := module.buildGlobals(importedGlobals, m.Engine.FunctionInstanceReference), module.buildMemory()

	// Now we have all instances from imports and local ones, so ready to create a new ModuleInstance.
	m.addSections(module, importedGlobals, globals, tables, importedMemory, memory)

	// As of reference types proposal, data segment validation must happen after instantiation,
	// and the side effect must persist even if there's out of bounds error after instantiation.
	// https://github.com/WebAssembly/spec/blob/d39195773112a22b245ffbe864bab6d1182ccb06/test/core/linking.wast#L395-L405
	if !s.EnabledFeatures.IsEnabled(api.CoreFeatureReferenceTypes) {
		if err = m.validateData(module.DataSection); err != nil {
			return nil, err
		}
	}

	// After engine creation, we can create the funcref element instances and initialize funcref type globals.
	m.buildElementInstances(module.ElementSection)

	// Now all the validation passes, we are safe to mutate memory instances (possibly imported ones).
	if err = m.applyData(module.DataSection); err != nil {
		return nil, err
	}

	m.applyTableInits(tables, tableInit)

	// Compile the default context for calls to this module.
	callCtx := NewCallContext(s, m, sysCtx)
	m.CallCtx = callCtx

	// Execute the start function.
	if module.StartSection != nil {
		funcIdx := *module.StartSection
		f := &m.Functions[funcIdx]

		ce, err := f.Module.Engine.NewCallEngine(callCtx, f)
		if err != nil {
			return nil, fmt.Errorf("create call engine for start function[%s]: %v",
				module.funcDesc(SectionIDFunction, funcIdx), err)
		}

		_, err = ce.Call(ctx, callCtx, nil)
		if exitErr, ok := err.(*sys.ExitError); ok { // Don't wrap an exit error!
			return nil, exitErr
		} else if err != nil {
			return nil, fmt.Errorf("start %s failed: %w", module.funcDesc(SectionIDFunction, funcIdx), err)
		}
	}

	return m.CallCtx, nil
}

func resolveImports(module *Module, modules map[string]*ModuleInstance) (
	importedFunctions []*FunctionInstance,
	importedGlobals []*GlobalInstance,
	importedTables []*TableInstance,
	importedMemory *MemoryInstance,
	err error,
) {
	for idx, i := range module.ImportSection {
		m, ok := modules[i.Module]
		if !ok {
			err = fmt.Errorf("module[%s] not instantiated", i.Module)
			return
		}

		var imported ExportInstance
		imported, err = m.getExport(i.Name, i.Type)
		if err != nil {
			return
		}

		switch i.Type {
		case ExternTypeFunc:
			typeIndex := i.DescFunc
			// TODO: this shouldn't be possible as invalid should fail validate
			if int(typeIndex) >= len(module.TypeSection) {
				err = errorInvalidImport(i, idx, fmt.Errorf("function type out of range"))
				return
			}
			expectedType := module.TypeSection[i.DescFunc]
			importedFunction := &m.Functions[imported.Index]

			d := importedFunction.Definition
			if !expectedType.EqualsSignature(d.ParamTypes(), d.ResultTypes()) {
				actualType := &FunctionType{Params: d.ParamTypes(), Results: d.ResultTypes()}
				err = errorInvalidImport(i, idx, fmt.Errorf("signature mismatch: %s != %s", expectedType, actualType))
				return
			}

			importedFunctions = append(importedFunctions, importedFunction)
		case ExternTypeTable:
			expected := i.DescTable
			importedTable := m.Tables[imported.Index]
			if expected.Type != importedTable.Type {
				err = errorInvalidImport(i, idx, fmt.Errorf("table type mismatch: %s != %s",
					RefTypeName(expected.Type), RefTypeName(importedTable.Type)))
			}

			if expected.Min > importedTable.Min {
				err = errorMinSizeMismatch(i, idx, expected.Min, importedTable.Min)
				return
			}

			if expected.Max != nil {
				expectedMax := *expected.Max
				if importedTable.Max == nil {
					err = errorNoMax(i, idx, expectedMax)
					return
				} else if expectedMax < *importedTable.Max {
					err = errorMaxSizeMismatch(i, idx, expectedMax, *importedTable.Max)
					return
				}
			}
			importedTables = append(importedTables, importedTable)
		case ExternTypeMemory:
			expected := i.DescMem
			importedMemory = m.Memory

			if expected.Min > memoryBytesNumToPages(uint64(len(importedMemory.Buffer))) {
				err = errorMinSizeMismatch(i, idx, expected.Min, importedMemory.Min)
				return
			}

			if expected.Max < importedMemory.Max {
				err = errorMaxSizeMismatch(i, idx, expected.Max, importedMemory.Max)
				return
			}
		case ExternTypeGlobal:
			expected := i.DescGlobal
			importedGlobal := m.Globals[imported.Index]

			if expected.Mutable != importedGlobal.Type.Mutable {
				err = errorInvalidImport(i, idx, fmt.Errorf("mutability mismatch: %t != %t",
					expected.Mutable, importedGlobal.Type.Mutable))
				return
			}

			if expected.ValType != importedGlobal.Type.ValType {
				err = errorInvalidImport(i, idx, fmt.Errorf("value type mismatch: %s != %s",
					ValueTypeName(expected.ValType), ValueTypeName(importedGlobal.Type.ValType)))
				return
			}
			importedGlobals = append(importedGlobals, importedGlobal)
		}
	}
	return
}

func errorMinSizeMismatch(i *Import, idx int, expected, actual uint32) error {
	return errorInvalidImport(i, idx, fmt.Errorf("minimum size mismatch: %d > %d", expected, actual))
}

func errorNoMax(i *Import, idx int, expected uint32) error {
	return errorInvalidImport(i, idx, fmt.Errorf("maximum size mismatch: %d, but actual has no max", expected))
}

func errorMaxSizeMismatch(i *Import, idx int, expected, actual uint32) error {
	return errorInvalidImport(i, idx, fmt.Errorf("maximum size mismatch: %d < %d", expected, actual))
}

func errorInvalidImport(i *Import, idx int, err error) error {
	return fmt.Errorf("import[%d] %s[%s.%s]: %w", idx, ExternTypeName(i.Type), i.Module, i.Name, err)
}

// Global initialization constant expression can only reference the imported globals.
// See the note on https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#constant-expressions%E2%91%A0
func executeConstExpression(importedGlobals []*GlobalInstance, expr *ConstantExpression) (v interface{}) {
	switch expr.Opcode {
	case OpcodeI32Const:
		// Treat constants as signed as their interpretation is not yet known per /RATIONALE.md
		v, _, _ = leb128.LoadInt32(expr.Data)
	case OpcodeI64Const:
		// Treat constants as signed as their interpretation is not yet known per /RATIONALE.md
		v, _, _ = leb128.LoadInt64(expr.Data)
	case OpcodeF32Const:
		v, _ = ieee754.DecodeFloat32(expr.Data)
	case OpcodeF64Const:
		v, _ = ieee754.DecodeFloat64(expr.Data)
	case OpcodeGlobalGet:
		id, _, _ := leb128.LoadUint32(expr.Data)
		g := importedGlobals[id]
		switch g.Type.ValType {
		case ValueTypeI32:
			v = int32(g.Val)
		case ValueTypeI64:
			v = int64(g.Val)
		case ValueTypeF32:
			v = api.DecodeF32(g.Val)
		case ValueTypeF64:
			v = api.DecodeF64(g.Val)
		case ValueTypeV128:
			v = [2]uint64{g.Val, g.ValHi}
		case ValueTypeFuncref, ValueTypeExternref:
			v = int64(g.Val)
		}
	case OpcodeRefNull:
		switch expr.Data[0] {
		case ValueTypeExternref, ValueTypeFuncref:
			v = int64(0) // Reference types are opaque 64bit pointer at runtime.
		}
	case OpcodeRefFunc:
		// For ref.func const expression, we temporarily store the index as value,
		// and if this is the const expr for global, the value will be further downed to
		// opaque pointer of the engine-specific compiled function.
		v, _, _ = leb128.LoadUint32(expr.Data)
	case OpcodeVecV128Const:
		v = [2]uint64{binary.LittleEndian.Uint64(expr.Data[0:8]), binary.LittleEndian.Uint64(expr.Data[8:16])}
	}
	return
}

func (s *Store) getFunctionTypeIDs(ts []*FunctionType) ([]FunctionTypeID, error) {
	ret := make([]FunctionTypeID, len(ts))
	for i, t := range ts {
		inst, err := s.getFunctionTypeID(t)
		if err != nil {
			return nil, err
		}
		ret[i] = inst
	}
	return ret, nil
}

// preAllocatedTypeIDs maps several "well-known" FunctionType strings to the pre allocated FunctionID.
// This is used by emscripten integration, but it is harmless to have this all the time as it's only
// used during Store creation.
var preAllocatedTypeIDs = map[string]FunctionTypeID{
	"i32i32i32i32_v":   PreAllocatedTypeID_i32i32i32i32_v,
	"i32i32i32_v":      PreAllocatedTypeID_i32i32i32_v,
	"i32i32_v":         PreAllocatedTypeID_i32i32_v,
	"i32_v":            PreAllocatedTypeID_i32_v,
	"v_v":              PreAllocatedTypeID_v_v,
	"i32i32i32i32_i32": PreAllocatedTypeID_i32i32i32i32_i32,
	"i32i32i32_i32":    PreAllocatedTypeID_i32i32i32_i32,
	"i32i32_i32":       PreAllocatedTypeID_i32i32_i32,
	"i32_i32":          PreAllocatedTypeID_i32_i32,
	"v_i32":            PreAllocatedTypeID_v_i32,
}

const (
	// PreAllocatedTypeID_i32i32i32i32_v is FunctionTypeID for i32i32i32i32_v.
	PreAllocatedTypeID_i32i32i32i32_v FunctionTypeID = iota
	// PreAllocatedTypeID_i32i32i32_v is FunctionTypeID for i32i32i32_v
	PreAllocatedTypeID_i32i32i32_v
	// PreAllocatedTypeID_i32i32_v is FunctionTypeID for i32i32_v
	PreAllocatedTypeID_i32i32_v
	// PreAllocatedTypeID_i32_v is FunctionTypeID for i32_v
	PreAllocatedTypeID_i32_v
	// PreAllocatedTypeID_v_v is FunctionTypeID for v_v
	PreAllocatedTypeID_v_v
	// PreAllocatedTypeID_i32i32i32i32_i32 is FunctionTypeID for i32i32i32i32_i32
	PreAllocatedTypeID_i32i32i32i32_i32
	// PreAllocatedTypeID_i32i32i32_i32 is FunctionTypeID for i32i32i32_i32
	PreAllocatedTypeID_i32i32i32_i32
	// PreAllocatedTypeID_i32i32_i32 is FunctionTypeID for i32i32_i32
	PreAllocatedTypeID_i32i32_i32
	// PreAllocatedTypeID_i32_i32 is FunctionTypeID for i32_i32
	PreAllocatedTypeID_i32_i32
	// PreAllocatedTypeID_v_i32 is FunctionTypeID for v_i32
	PreAllocatedTypeID_v_i32
)

func (s *Store) getFunctionTypeID(t *FunctionType) (FunctionTypeID, error) {
	s.mux.RLock()
	key := t.key()
	id, ok := s.typeIDs[key]
	s.mux.RUnlock()
	if !ok {
		s.mux.Lock()
		defer s.mux.Unlock()
		// Check again in case another goroutine has already added the type.
		if id, ok = s.typeIDs[key]; ok {
			return id, nil
		}
		l := len(s.typeIDs)
		if uint32(l) >= s.functionMaxTypes {
			return 0, fmt.Errorf("too many function types in a store")
		}
		id = FunctionTypeID(l)
		s.typeIDs[key] = id
	}
	return id, nil
}

// CloseWithExitCode implements the same method as documented on wazero.Runtime.
func (s *Store) CloseWithExitCode(ctx context.Context, exitCode uint32) (err error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	// Close modules in reverse initialization order.
	for node := s.moduleList; node != nil; node = node.next {
		// If closing this module errs, proceed anyway to close the others.
		if m := node.module; m != nil {
			if e := m.CallCtx.closeWithExitCode(ctx, exitCode); e != nil && err == nil {
				// TODO: use multiple errors handling in Go 1.20.
				err = e // first error
			}
		}
	}
	s.moduleList = nil
	s.nameToNode = nil
	s.typeIDs = nil
	return
}
