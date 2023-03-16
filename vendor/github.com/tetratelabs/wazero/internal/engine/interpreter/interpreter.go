package interpreter

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"math/bits"
	"sync"
	"unsafe"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/experimental"
	"github.com/tetratelabs/wazero/internal/filecache"
	"github.com/tetratelabs/wazero/internal/moremath"
	"github.com/tetratelabs/wazero/internal/wasm"
	"github.com/tetratelabs/wazero/internal/wasmdebug"
	"github.com/tetratelabs/wazero/internal/wasmruntime"
	"github.com/tetratelabs/wazero/internal/wazeroir"
)

// callStackCeiling is the maximum WebAssembly call frame stack height. This allows wazero to raise
// wasm.ErrCallStackOverflow instead of overflowing the Go runtime.
//
// The default value should suffice for most use cases. Those wishing to change this can via `go build -ldflags`.
var callStackCeiling = 2000

// engine is an interpreter implementation of wasm.Engine
type engine struct {
	enabledFeatures api.CoreFeatures
	codes           map[wasm.ModuleID][]*code // guarded by mutex.
	mux             sync.RWMutex
}

func NewEngine(_ context.Context, enabledFeatures api.CoreFeatures, _ filecache.Cache) wasm.Engine {
	return &engine{
		enabledFeatures: enabledFeatures,
		codes:           map[wasm.ModuleID][]*code{},
	}
}

// Close implements the same method as documented on wasm.Engine.
func (e *engine) Close() (err error) {
	return
}

// CompiledModuleCount implements the same method as documented on wasm.Engine.
func (e *engine) CompiledModuleCount() uint32 {
	return uint32(len(e.codes))
}

// DeleteCompiledModule implements the same method as documented on wasm.Engine.
func (e *engine) DeleteCompiledModule(m *wasm.Module) {
	e.deleteCodes(m)
}

func (e *engine) deleteCodes(module *wasm.Module) {
	e.mux.Lock()
	defer e.mux.Unlock()
	delete(e.codes, module.ID)
}

func (e *engine) addCodes(module *wasm.Module, fs []*code) {
	e.mux.Lock()
	defer e.mux.Unlock()
	e.codes[module.ID] = fs
}

func (e *engine) getCodes(module *wasm.Module) (fs []*code, ok bool) {
	e.mux.RLock()
	defer e.mux.RUnlock()
	fs, ok = e.codes[module.ID]
	return
}

// moduleEngine implements wasm.ModuleEngine
type moduleEngine struct {
	// name is the name the module was instantiated with used for error handling.
	name string

	// codes are the compiled functions in a module instances.
	// The index is module instance-scoped.
	functions []function

	// parentEngine holds *engine from which this module engine is created from.
	parentEngine *engine
}

// callEngine holds context per moduleEngine.Call, and shared across all the
// function calls originating from the same moduleEngine.Call execution.
type callEngine struct {
	// stack contains the operands.
	// Note that all the values are represented as uint64.
	stack []uint64

	// frames are the function call stack.
	frames []*callFrame

	// compiled is the initial function for this call engine.
	compiled *function
	// source is the FunctionInstance from which compiled is created from.
	source *wasm.FunctionInstance
}

func (e *moduleEngine) newCallEngine(source *wasm.FunctionInstance, compiled *function) *callEngine {
	return &callEngine{source: source, compiled: compiled}
}

func (ce *callEngine) pushValue(v uint64) {
	ce.stack = append(ce.stack, v)
}

func (ce *callEngine) popValue() (v uint64) {
	// No need to check stack bound
	// as we can assume that all the operations
	// are valid thanks to validateFunction
	// at module validation phase
	// and wazeroir translation
	// before compilation.
	stackTopIndex := len(ce.stack) - 1
	v = ce.stack[stackTopIndex]
	ce.stack = ce.stack[:stackTopIndex]
	return
}

// peekValues peeks api.ValueType values from the stack and returns them.
func (ce *callEngine) peekValues(count int) []uint64 {
	if count == 0 {
		return nil
	}
	stackLen := len(ce.stack)
	return ce.stack[stackLen-count : stackLen]
}

func (ce *callEngine) drop(r *wazeroir.InclusiveRange) {
	// No need to check stack bound
	// as we can assume that all the operations
	// are valid thanks to validateFunction
	// at module validation phase
	// and wazeroir translation
	// before compilation.
	if r == nil {
		return
	} else if r.Start == 0 {
		ce.stack = ce.stack[:len(ce.stack)-1-r.End]
	} else {
		newStack := ce.stack[:len(ce.stack)-1-r.End]
		newStack = append(newStack, ce.stack[len(ce.stack)-r.Start:]...)
		ce.stack = newStack
	}
}

func (ce *callEngine) pushFrame(frame *callFrame) {
	if callStackCeiling <= len(ce.frames) {
		panic(wasmruntime.ErrRuntimeStackOverflow)
	}
	ce.frames = append(ce.frames, frame)
}

func (ce *callEngine) popFrame() (frame *callFrame) {
	// No need to check stack bound as we can assume that all the operations are valid thanks to validateFunction at
	// module validation phase and wazeroir translation before compilation.
	oneLess := len(ce.frames) - 1
	frame = ce.frames[oneLess]
	ce.frames = ce.frames[:oneLess]
	return
}

type callFrame struct {
	// pc is the program counter representing the current position in code.body.
	pc uint64
	// f is the compiled function used in this function frame.
	f *function
}

type code struct {
	source            *wasm.Module
	body              []*interpreterOp
	listener          experimental.FunctionListener
	hostFn            interface{}
	isHostFunction    bool
	ensureTermination bool
}

type function struct {
	source *wasm.FunctionInstance
	parent *code
}

// functionFromUintptr resurrects the original *function from the given uintptr
// which comes from either funcref table or OpcodeRefFunc instruction.
func functionFromUintptr(ptr uintptr) *function {
	// Wraps ptrs as the double pointer in order to avoid the unsafe access as detected by race detector.
	//
	// For example, if we have (*function)(unsafe.Pointer(ptr)) instead, then the race detector's "checkptr"
	// subroutine wanrs as "checkptr: pointer arithmetic result points to invalid allocation"
	// https://github.com/golang/go/blob/1ce7fcf139417d618c2730010ede2afb41664211/src/runtime/checkptr.go#L69
	var wrapped *uintptr = &ptr
	return *(**function)(unsafe.Pointer(wrapped))
}

// interpreterOp is the compilation (engine.lowerIR) result of a wazeroir.Operation.
//
// Not all operations result in an interpreterOp, e.g. wazeroir.OperationI32ReinterpretFromF32, and some operations are
// more complex than others, e.g. wazeroir.OperationBrTable.
//
// Note: This is a form of union type as it can store fields needed for any operation. Hence, most fields are opaque and
// only relevant when in context of its kind.
type interpreterOp struct {
	// kind determines how to interpret the other fields in this struct.
	kind     wazeroir.OperationKind
	b1, b2   byte
	b3       bool
	us       []uint64
	rs       []*wazeroir.InclusiveRange
	sourcePC uint64
}

// interpreter mode doesn't maintain call frames in the stack, so pass the zero size to the IR.
const callFrameStackSize = 0

// CompileModule implements the same method as documented on wasm.Engine.
func (e *engine) CompileModule(ctx context.Context, module *wasm.Module, listeners []experimental.FunctionListener, ensureTermination bool) error {
	if _, ok := e.getCodes(module); ok { // cache hit!
		return nil
	}

	funcs := make([]*code, len(module.FunctionSection))
	irs, err := wazeroir.CompileFunctions(e.enabledFeatures, callFrameStackSize, module, ensureTermination)
	if err != nil {
		return err
	}
	for i, ir := range irs {
		var lsn experimental.FunctionListener
		if i < len(listeners) {
			lsn = listeners[i]
		}

		// If this is the host function, there's nothing to do as the runtime representation of
		// host function in interpreter is its Go function itself as opposed to Wasm functions,
		// which need to be compiled down to wazeroir.
		var compiled *code
		if ir.GoFunc != nil {
			compiled = &code{hostFn: ir.GoFunc, listener: lsn}
		} else {
			compiled, err = e.lowerIR(ir)
			if err != nil {
				def := module.FunctionDefinitionSection[uint32(i)+module.ImportFuncCount()]
				return fmt.Errorf("failed to lower func[%s] to wazeroir: %w", def.DebugName(), err)
			}
			compiled.listener = lsn
		}
		compiled.source = module
		compiled.isHostFunction = ir.IsHostFunction
		compiled.ensureTermination = ir.EnsureTermination
		funcs[i] = compiled
	}
	e.addCodes(module, funcs)
	return nil
}

// NewModuleEngine implements the same method as documented on wasm.Engine.
func (e *engine) NewModuleEngine(name string, module *wasm.Module, functions []wasm.FunctionInstance) (wasm.ModuleEngine, error) {
	me := &moduleEngine{
		name:         name,
		parentEngine: e,
		functions:    make([]function, len(functions)),
	}

	imported := int(module.ImportFuncCount())
	for i, f := range functions[:imported] {
		cf := f.Module.Engine.(*moduleEngine).functions[f.Idx]
		me.functions[i] = cf
	}

	codes, ok := e.getCodes(module)
	if !ok {
		return nil, fmt.Errorf("source module for %s must be compiled before instantiation", name)
	}

	for i, c := range codes {
		offset := i + imported
		f := &functions[offset]
		me.functions[offset] = function{source: f, parent: c}
	}
	return me, nil
}

// lowerIR lowers the wazeroir operations to engine friendly struct.
func (e *engine) lowerIR(ir *wazeroir.CompilationResult) (*code, error) {
	hasSourcePCs := len(ir.IROperationSourceOffsetsInWasmBinary) > 0
	ops := ir.Operations
	ret := &code{}
	labelAddress := map[wazeroir.LabelID]uint64{}
	onLabelAddressResolved := map[wazeroir.LabelID][]func(addr uint64){}
	for i, original := range ops {
		op := &interpreterOp{kind: original.Kind()}
		if hasSourcePCs {
			op.sourcePC = ir.IROperationSourceOffsetsInWasmBinary[i]
		}
		switch o := original.(type) {
		case wazeroir.OperationBuiltinFunctionCheckExitCode:
		case wazeroir.OperationUnreachable:
		case wazeroir.OperationLabel:
			labelID := o.Label.ID()
			address := uint64(len(ret.body))
			labelAddress[labelID] = address
			for _, cb := range onLabelAddressResolved[labelID] {
				cb(address)
			}
			delete(onLabelAddressResolved, labelID)
			// We just ignore the label operation
			// as we translate branch operations to the direct address jmp.
			continue
		case wazeroir.OperationBr:
			op.us = make([]uint64, 1)
			if o.Target.IsReturnTarget() {
				// Jmp to the end of the possible binary.
				op.us[0] = math.MaxUint64
			} else {
				labelID := o.Target.ID()
				addr, ok := labelAddress[labelID]
				if !ok {
					// If this is the forward jump (e.g. to the continuation of if, etc.),
					// the target is not emitted yet, so resolve the address later.
					onLabelAddressResolved[labelID] = append(onLabelAddressResolved[labelID],
						func(addr uint64) {
							op.us[0] = addr
						},
					)
				} else {
					op.us[0] = addr
				}
			}
		case wazeroir.OperationBrIf:
			op.rs = make([]*wazeroir.InclusiveRange, 2)
			op.us = make([]uint64, 2)
			for i, target := range []wazeroir.BranchTargetDrop{o.Then, o.Else} {
				op.rs[i] = target.ToDrop
				if target.Target.IsReturnTarget() {
					// Jmp to the end of the possible binary.
					op.us[i] = math.MaxUint64
				} else {
					labelID := target.Target.ID()
					addr, ok := labelAddress[labelID]
					if !ok {
						i := i
						// If this is the forward jump (e.g. to the continuation of if, etc.),
						// the target is not emitted yet, so resolve the address later.
						onLabelAddressResolved[labelID] = append(onLabelAddressResolved[labelID],
							func(addr uint64) {
								op.us[i] = addr
							},
						)
					} else {
						op.us[i] = addr
					}
				}
			}
		case wazeroir.OperationBrTable:
			targets := append([]*wazeroir.BranchTargetDrop{o.Default}, o.Targets...)
			op.rs = make([]*wazeroir.InclusiveRange, len(targets))
			op.us = make([]uint64, len(targets))
			for i, target := range targets {
				op.rs[i] = target.ToDrop
				if target.Target.IsReturnTarget() {
					// Jmp to the end of the possible binary.
					op.us[i] = math.MaxUint64
				} else {
					labelID := target.Target.ID()
					addr, ok := labelAddress[labelID]
					if !ok {
						i := i // pin index for later resolution
						// If this is the forward jump (e.g. to the continuation of if, etc.),
						// the target is not emitted yet, so resolve the address later.
						onLabelAddressResolved[labelID] = append(onLabelAddressResolved[labelID],
							func(addr uint64) {
								op.us[i] = addr
							},
						)
					} else {
						op.us[i] = addr
					}
				}
			}
		case wazeroir.OperationCall:
			op.us = make([]uint64, 1)
			op.us = []uint64{uint64(o.FunctionIndex)}
		case wazeroir.OperationCallIndirect:
			op.us = make([]uint64, 2)
			op.us[0] = uint64(o.TypeIndex)
			op.us[1] = uint64(o.TableIndex)
		case wazeroir.OperationDrop:
			op.rs = make([]*wazeroir.InclusiveRange, 1)
			op.rs[0] = o.Depth
		case wazeroir.OperationSelect:
			op.b3 = o.IsTargetVector
		case wazeroir.OperationPick:
			op.us = make([]uint64, 1)
			op.us[0] = uint64(o.Depth)
			op.b3 = o.IsTargetVector
		case wazeroir.OperationSet:
			op.us = make([]uint64, 1)
			op.us[0] = uint64(o.Depth)
			op.b3 = o.IsTargetVector
		case wazeroir.OperationGlobalGet:
			op.us = make([]uint64, 1)
			op.us[0] = uint64(o.Index)
		case wazeroir.OperationGlobalSet:
			op.us = make([]uint64, 1)
			op.us[0] = uint64(o.Index)
		case wazeroir.OperationLoad:
			op.b1 = byte(o.Type)
			op.us = make([]uint64, 2)
			op.us[0] = uint64(o.Arg.Alignment)
			op.us[1] = uint64(o.Arg.Offset)
		case wazeroir.OperationLoad8:
			op.b1 = byte(o.Type)
			op.us = make([]uint64, 2)
			op.us[0] = uint64(o.Arg.Alignment)
			op.us[1] = uint64(o.Arg.Offset)
		case wazeroir.OperationLoad16:
			op.b1 = byte(o.Type)
			op.us = make([]uint64, 2)
			op.us[0] = uint64(o.Arg.Alignment)
			op.us[1] = uint64(o.Arg.Offset)
		case wazeroir.OperationLoad32:
			if o.Signed {
				op.b1 = 1
			}
			op.us = make([]uint64, 2)
			op.us[0] = uint64(o.Arg.Alignment)
			op.us[1] = uint64(o.Arg.Offset)
		case wazeroir.OperationStore:
			op.b1 = byte(o.Type)
			op.us = make([]uint64, 2)
			op.us[0] = uint64(o.Arg.Alignment)
			op.us[1] = uint64(o.Arg.Offset)
		case wazeroir.OperationStore8:
			op.us = make([]uint64, 2)
			op.us[0] = uint64(o.Arg.Alignment)
			op.us[1] = uint64(o.Arg.Offset)
		case wazeroir.OperationStore16:
			op.us = make([]uint64, 2)
			op.us[0] = uint64(o.Arg.Alignment)
			op.us[1] = uint64(o.Arg.Offset)
		case wazeroir.OperationStore32:
			op.us = make([]uint64, 2)
			op.us[0] = uint64(o.Arg.Alignment)
			op.us[1] = uint64(o.Arg.Offset)
		case wazeroir.OperationMemorySize:
		case wazeroir.OperationMemoryGrow:
		case wazeroir.OperationConstI32:
			op.us = make([]uint64, 1)
			op.us[0] = uint64(o.Value)
		case wazeroir.OperationConstI64:
			op.us = make([]uint64, 1)
			op.us[0] = o.Value
		case wazeroir.OperationConstF32:
			op.us = make([]uint64, 1)
			op.us[0] = uint64(math.Float32bits(o.Value))
		case wazeroir.OperationConstF64:
			op.us = make([]uint64, 1)
			op.us[0] = math.Float64bits(o.Value)
		case wazeroir.OperationEq:
			op.b1 = byte(o.Type)
		case wazeroir.OperationNe:
			op.b1 = byte(o.Type)
		case wazeroir.OperationEqz:
			op.b1 = byte(o.Type)
		case wazeroir.OperationLt:
			op.b1 = byte(o.Type)
		case wazeroir.OperationGt:
			op.b1 = byte(o.Type)
		case wazeroir.OperationLe:
			op.b1 = byte(o.Type)
		case wazeroir.OperationGe:
			op.b1 = byte(o.Type)
		case wazeroir.OperationAdd:
			op.b1 = byte(o.Type)
		case wazeroir.OperationSub:
			op.b1 = byte(o.Type)
		case wazeroir.OperationMul:
			op.b1 = byte(o.Type)
		case wazeroir.OperationClz:
			op.b1 = byte(o.Type)
		case wazeroir.OperationCtz:
			op.b1 = byte(o.Type)
		case wazeroir.OperationPopcnt:
			op.b1 = byte(o.Type)
		case wazeroir.OperationDiv:
			op.b1 = byte(o.Type)
		case wazeroir.OperationRem:
			op.b1 = byte(o.Type)
		case wazeroir.OperationAnd:
			op.b1 = byte(o.Type)
		case wazeroir.OperationOr:
			op.b1 = byte(o.Type)
		case wazeroir.OperationXor:
			op.b1 = byte(o.Type)
		case wazeroir.OperationShl:
			op.b1 = byte(o.Type)
		case wazeroir.OperationShr:
			op.b1 = byte(o.Type)
		case wazeroir.OperationRotl:
			op.b1 = byte(o.Type)
		case wazeroir.OperationRotr:
			op.b1 = byte(o.Type)
		case wazeroir.OperationAbs:
			op.b1 = byte(o.Type)
		case wazeroir.OperationNeg:
			op.b1 = byte(o.Type)
		case wazeroir.OperationCeil:
			op.b1 = byte(o.Type)
		case wazeroir.OperationFloor:
			op.b1 = byte(o.Type)
		case wazeroir.OperationTrunc:
			op.b1 = byte(o.Type)
		case wazeroir.OperationNearest:
			op.b1 = byte(o.Type)
		case wazeroir.OperationSqrt:
			op.b1 = byte(o.Type)
		case wazeroir.OperationMin:
			op.b1 = byte(o.Type)
		case wazeroir.OperationMax:
			op.b1 = byte(o.Type)
		case wazeroir.OperationCopysign:
			op.b1 = byte(o.Type)
		case wazeroir.OperationI32WrapFromI64:
		case wazeroir.OperationITruncFromF:
			op.b1 = byte(o.InputType)
			op.b2 = byte(o.OutputType)
			op.b3 = o.NonTrapping
		case wazeroir.OperationFConvertFromI:
			op.b1 = byte(o.InputType)
			op.b2 = byte(o.OutputType)
		case wazeroir.OperationF32DemoteFromF64:
		case wazeroir.OperationF64PromoteFromF32:
		case wazeroir.OperationI32ReinterpretFromF32,
			wazeroir.OperationI64ReinterpretFromF64,
			wazeroir.OperationF32ReinterpretFromI32,
			wazeroir.OperationF64ReinterpretFromI64:
			// Reinterpret ops are essentially nop for engine mode
			// because we treat all values as uint64, and Reinterpret* is only used at module
			// validation phase where we check type soundness of all the operations.
			// So just eliminate the ops.
			continue
		case wazeroir.OperationExtend:
			if o.Signed {
				op.b1 = 1
			}
		case wazeroir.OperationSignExtend32From8, wazeroir.OperationSignExtend32From16, wazeroir.OperationSignExtend64From8,
			wazeroir.OperationSignExtend64From16, wazeroir.OperationSignExtend64From32:
		case wazeroir.OperationMemoryInit:
			op.us = make([]uint64, 1)
			op.us[0] = uint64(o.DataIndex)
		case wazeroir.OperationDataDrop:
			op.us = make([]uint64, 1)
			op.us[0] = uint64(o.DataIndex)
		case wazeroir.OperationMemoryCopy:
		case wazeroir.OperationMemoryFill:
		case wazeroir.OperationTableInit:
			op.us = make([]uint64, 2)
			op.us[0] = uint64(o.ElemIndex)
			op.us[1] = uint64(o.TableIndex)
		case wazeroir.OperationElemDrop:
			op.us = make([]uint64, 1)
			op.us[0] = uint64(o.ElemIndex)
		case wazeroir.OperationTableCopy:
			op.us = make([]uint64, 2)
			op.us[0] = uint64(o.SrcTableIndex)
			op.us[1] = uint64(o.DstTableIndex)
		case wazeroir.OperationRefFunc:
			op.us = make([]uint64, 1)
			op.us[0] = uint64(o.FunctionIndex)
		case wazeroir.OperationTableGet:
			op.us = make([]uint64, 1)
			op.us[0] = uint64(o.TableIndex)
		case wazeroir.OperationTableSet:
			op.us = make([]uint64, 1)
			op.us[0] = uint64(o.TableIndex)
		case wazeroir.OperationTableSize:
			op.us = make([]uint64, 1)
			op.us[0] = uint64(o.TableIndex)
		case wazeroir.OperationTableGrow:
			op.us = make([]uint64, 1)
			op.us[0] = uint64(o.TableIndex)
		case wazeroir.OperationTableFill:
			op.us = make([]uint64, 1)
			op.us[0] = uint64(o.TableIndex)
		case wazeroir.OperationV128Const:
			op.us = make([]uint64, 2)
			op.us[0] = o.Lo
			op.us[1] = o.Hi
		case wazeroir.OperationV128Add:
			op.b1 = o.Shape
		case wazeroir.OperationV128Sub:
			op.b1 = o.Shape
		case wazeroir.OperationV128Load:
			op.b1 = o.Type
			op.us = make([]uint64, 2)
			op.us[0] = uint64(o.Arg.Alignment)
			op.us[1] = uint64(o.Arg.Offset)
		case wazeroir.OperationV128LoadLane:
			op.b1 = o.LaneSize
			op.b2 = o.LaneIndex
			op.us = make([]uint64, 2)
			op.us[0] = uint64(o.Arg.Alignment)
			op.us[1] = uint64(o.Arg.Offset)
		case wazeroir.OperationV128Store:
			op.us = make([]uint64, 2)
			op.us[0] = uint64(o.Arg.Alignment)
			op.us[1] = uint64(o.Arg.Offset)
		case wazeroir.OperationV128StoreLane:
			op.b1 = o.LaneSize
			op.b2 = o.LaneIndex
			op.us = make([]uint64, 2)
			op.us[0] = uint64(o.Arg.Alignment)
			op.us[1] = uint64(o.Arg.Offset)
		case wazeroir.OperationV128ExtractLane:
			op.b1 = o.Shape
			op.b2 = o.LaneIndex
			op.b3 = o.Signed
		case wazeroir.OperationV128ReplaceLane:
			op.b1 = o.Shape
			op.b2 = o.LaneIndex
		case wazeroir.OperationV128Splat:
			op.b1 = o.Shape
		case wazeroir.OperationV128Shuffle:
			op.us = make([]uint64, 16)
			for i, l := range o.Lanes {
				op.us[i] = uint64(l)
			}
		case wazeroir.OperationV128Swizzle:
		case wazeroir.OperationV128AnyTrue:
		case wazeroir.OperationV128AllTrue:
			op.b1 = o.Shape
		case wazeroir.OperationV128BitMask:
			op.b1 = o.Shape
		case wazeroir.OperationV128And:
		case wazeroir.OperationV128Not:
		case wazeroir.OperationV128Or:
		case wazeroir.OperationV128Xor:
		case wazeroir.OperationV128Bitselect:
		case wazeroir.OperationV128AndNot:
		case wazeroir.OperationV128Shr:
			op.b1 = o.Shape
			op.b3 = o.Signed
		case wazeroir.OperationV128Shl:
			op.b1 = o.Shape
		case wazeroir.OperationV128Cmp:
			op.b1 = o.Type
		case wazeroir.OperationV128AddSat:
			op.b1 = o.Shape
			op.b3 = o.Signed
		case wazeroir.OperationV128SubSat:
			op.b1 = o.Shape
			op.b3 = o.Signed
		case wazeroir.OperationV128Mul:
			op.b1 = o.Shape
		case wazeroir.OperationV128Div:
			op.b1 = o.Shape
		case wazeroir.OperationV128Neg:
			op.b1 = o.Shape
		case wazeroir.OperationV128Sqrt:
			op.b1 = o.Shape
		case wazeroir.OperationV128Abs:
			op.b1 = o.Shape
		case wazeroir.OperationV128Popcnt:
		case wazeroir.OperationV128Min:
			op.b1 = o.Shape
			op.b3 = o.Signed
		case wazeroir.OperationV128Max:
			op.b1 = o.Shape
			op.b3 = o.Signed
		case wazeroir.OperationV128AvgrU:
			op.b1 = o.Shape
		case wazeroir.OperationV128Pmin:
			op.b1 = o.Shape
		case wazeroir.OperationV128Pmax:
			op.b1 = o.Shape
		case wazeroir.OperationV128Ceil:
			op.b1 = o.Shape
		case wazeroir.OperationV128Floor:
			op.b1 = o.Shape
		case wazeroir.OperationV128Trunc:
			op.b1 = o.Shape
		case wazeroir.OperationV128Nearest:
			op.b1 = o.Shape
		case wazeroir.OperationV128Extend:
			op.b1 = o.OriginShape
			if o.Signed {
				op.b2 = 1
			}
			op.b3 = o.UseLow
		case wazeroir.OperationV128ExtMul:
			op.b1 = o.OriginShape
			if o.Signed {
				op.b2 = 1
			}
			op.b3 = o.UseLow
		case wazeroir.OperationV128Q15mulrSatS:
		case wazeroir.OperationV128ExtAddPairwise:
			op.b1 = o.OriginShape
			op.b3 = o.Signed
		case wazeroir.OperationV128FloatPromote:
		case wazeroir.OperationV128FloatDemote:
		case wazeroir.OperationV128FConvertFromI:
			op.b1 = o.DestinationShape
			op.b3 = o.Signed
		case wazeroir.OperationV128Dot:
		case wazeroir.OperationV128Narrow:
			op.b1 = o.OriginShape
			op.b3 = o.Signed
		case wazeroir.OperationV128ITruncSatFromF:
			op.b1 = o.OriginShape
			op.b3 = o.Signed
		default:
			panic(fmt.Errorf("BUG: unimplemented operation %s", op.kind.String()))
		}
		ret.body = append(ret.body, op)
	}

	if len(onLabelAddressResolved) > 0 {
		keys := make([]wazeroir.LabelID, 0, len(onLabelAddressResolved))
		for id := range onLabelAddressResolved {
			keys = append(keys, id)
		}
		return nil, fmt.Errorf("labels are not defined: %v", keys)
	}
	return ret, nil
}

// Name implements the same method as documented on wasm.ModuleEngine.
func (e *moduleEngine) Name() string {
	return e.name
}

// FunctionInstanceReference implements the same method as documented on wasm.ModuleEngine.
func (e *moduleEngine) FunctionInstanceReference(funcIndex wasm.Index) wasm.Reference {
	return uintptr(unsafe.Pointer(&e.functions[funcIndex]))
}

// NewCallEngine implements the same method as documented on wasm.ModuleEngine.
func (e *moduleEngine) NewCallEngine(_ *wasm.CallContext, f *wasm.FunctionInstance) (ce wasm.CallEngine, err error) {
	// Note: The input parameters are pre-validated, so a compiled function is only absent on close. Updates to
	// code on close aren't locked, neither is this read.
	compiled := &e.functions[f.Idx]
	return e.newCallEngine(f, compiled), nil
}

// LookupFunction implements the same method as documented on wasm.ModuleEngine.
func (e *moduleEngine) LookupFunction(t *wasm.TableInstance, typeId wasm.FunctionTypeID, tableOffset wasm.Index) (idx wasm.Index, err error) {
	if tableOffset >= uint32(len(t.References)) {
		err = wasmruntime.ErrRuntimeInvalidTableAccess
		return
	}
	rawPtr := t.References[tableOffset]
	if rawPtr == 0 {
		err = wasmruntime.ErrRuntimeInvalidTableAccess
		return
	}

	tf := functionFromUintptr(rawPtr)
	if tf.source.TypeID != typeId {
		err = wasmruntime.ErrRuntimeIndirectCallTypeMismatch
		return
	}
	idx = tf.source.Idx

	return
}

// Call implements the same method as documented on wasm.CallEngine.
func (ce *callEngine) Call(ctx context.Context, m *wasm.CallContext, params []uint64) (results []uint64, err error) {
	return ce.call(ctx, m, ce.compiled, params)
}

func (ce *callEngine) call(ctx context.Context, callCtx *wasm.CallContext, tf *function, params []uint64) (results []uint64, err error) {
	if ce.compiled.parent.ensureTermination {
		select {
		case <-ctx.Done():
			// If the provided context is already done, close the call context
			// and return the error.
			callCtx.CloseWithCtxErr(ctx)
			return nil, callCtx.FailIfClosed()
		default:
		}
	}

	ft := tf.source.Type
	paramSignature := ft.ParamNumInUint64
	paramCount := len(params)
	if paramSignature != paramCount {
		return nil, fmt.Errorf("expected %d params, but passed %d", paramSignature, paramCount)
	}

	defer func() {
		// If the module closed during the call, and the call didn't err for another reason, set an ExitError.
		if err == nil {
			err = callCtx.FailIfClosed()
		}
		// TODO: ^^ Will not fail if the function was imported from a closed module.

		if v := recover(); v != nil {
			err = ce.recoverOnCall(v)
		}
	}()

	for _, param := range params {
		ce.pushValue(param)
	}

	if ce.compiled.parent.ensureTermination {
		done := callCtx.CloseModuleOnCanceledOrTimeout(ctx)
		defer done()
	}

	ce.callFunction(ctx, callCtx, tf)

	// This returns a safe copy of the results, instead of a slice view. If we
	// returned a re-slice, the caller could accidentally or purposefully
	// corrupt the stack of subsequent calls.
	results = wasm.PopValues(ft.ResultNumInUint64, ce.popValue)
	return
}

// recoverOnCall takes the recovered value `recoverOnCall`, and wraps it
// with the call frame stack traces. Also, reset the state of callEngine
// so that it can be used for the subsequent calls.
func (ce *callEngine) recoverOnCall(v interface{}) (err error) {
	builder := wasmdebug.NewErrorBuilder()
	frameCount := len(ce.frames)
	for i := 0; i < frameCount; i++ {
		frame := ce.popFrame()
		def := frame.f.source.Definition
		var sources []string
		if body := frame.f.parent.body; body != nil {
			sources = frame.f.parent.source.DWARFLines.Line(body[frame.pc].sourcePC)
		}
		builder.AddFrame(def.DebugName(), def.ParamTypes(), def.ResultTypes(), sources)
	}
	err = builder.FromRecovered(v)

	// Allows the reuse of CallEngine.
	ce.stack, ce.frames = ce.stack[:0], ce.frames[:0]
	return
}

func (ce *callEngine) callFunction(ctx context.Context, callCtx *wasm.CallContext, f *function) {
	if f.parent.hostFn != nil {
		ce.callGoFuncWithStack(ctx, callCtx, f)
	} else if lsn := f.parent.listener; lsn != nil {
		ce.callNativeFuncWithListener(ctx, callCtx, f, lsn)
	} else {
		ce.callNativeFunc(ctx, callCtx, f)
	}
}

func (ce *callEngine) callGoFunc(ctx context.Context, callCtx *wasm.CallContext, f *function, stack []uint64) {
	lsn := f.parent.listener
	callCtx = callCtx.WithMemory(ce.callerMemory())
	if lsn != nil {
		params := stack[:f.source.Type.ParamNumInUint64]
		ctx = lsn.Before(ctx, callCtx, f.source.Definition, params)
	}
	frame := &callFrame{f: f}
	ce.pushFrame(frame)

	fn := f.parent.hostFn
	switch fn := fn.(type) {
	case api.GoModuleFunction:
		fn.Call(ctx, callCtx, stack)
	case api.GoFunction:
		fn.Call(ctx, stack)
	}

	ce.popFrame()
	if lsn != nil {
		// TODO: This doesn't get the error due to use of panic to propagate them.
		results := stack[:f.source.Type.ResultNumInUint64]
		lsn.After(ctx, callCtx, f.source.Definition, nil, results)
	}
}

func (ce *callEngine) callNativeFunc(ctx context.Context, callCtx *wasm.CallContext, f *function) {
	frame := &callFrame{f: f}
	moduleInst := f.source.Module
	functions := moduleInst.Engine.(*moduleEngine).functions
	var memoryInst *wasm.MemoryInstance
	if f.parent.isHostFunction {
		memoryInst = ce.callerMemory()
	} else {
		memoryInst = moduleInst.Memory
	}
	globals := moduleInst.Globals
	tables := moduleInst.Tables
	typeIDs := f.source.Module.TypeIDs
	dataInstances := f.source.Module.DataInstances
	elementInstances := f.source.Module.ElementInstances
	ce.pushFrame(frame)
	body := frame.f.parent.body
	bodyLen := uint64(len(body))
	for frame.pc < bodyLen {
		op := body[frame.pc]
		// TODO: add description of each operation/case
		// on, for example, how many args are used,
		// how the stack is modified, etc.
		switch op.kind {
		case wazeroir.OperationKindBuiltinFunctionCheckExitCode:
			if err := callCtx.FailIfClosed(); err != nil {
				panic(err)
			}
			frame.pc++
		case wazeroir.OperationKindUnreachable:
			panic(wasmruntime.ErrRuntimeUnreachable)
		case wazeroir.OperationKindBr:
			frame.pc = op.us[0]
		case wazeroir.OperationKindBrIf:
			if ce.popValue() > 0 {
				ce.drop(op.rs[0])
				frame.pc = op.us[0]
			} else {
				ce.drop(op.rs[1])
				frame.pc = op.us[1]
			}
		case wazeroir.OperationKindBrTable:
			if v := uint64(ce.popValue()); v < uint64(len(op.us)-1) {
				ce.drop(op.rs[v+1])
				frame.pc = op.us[v+1]
			} else {
				// Default branch.
				ce.drop(op.rs[0])
				frame.pc = op.us[0]
			}
		case wazeroir.OperationKindCall:
			ce.callFunction(ctx, callCtx, &functions[op.us[0]])
			frame.pc++
		case wazeroir.OperationKindCallIndirect:
			offset := ce.popValue()
			table := tables[op.us[1]]
			if offset >= uint64(len(table.References)) {
				panic(wasmruntime.ErrRuntimeInvalidTableAccess)
			}
			rawPtr := table.References[offset]
			if rawPtr == 0 {
				panic(wasmruntime.ErrRuntimeInvalidTableAccess)
			}

			tf := functionFromUintptr(rawPtr)
			if tf.source.TypeID != typeIDs[op.us[0]] {
				panic(wasmruntime.ErrRuntimeIndirectCallTypeMismatch)
			}

			ce.callFunction(ctx, callCtx, tf)
			frame.pc++
		case wazeroir.OperationKindDrop:
			ce.drop(op.rs[0])
			frame.pc++
		case wazeroir.OperationKindSelect:
			c := ce.popValue()
			if op.b3 { // Target is vector.
				x2Hi, x2Lo := ce.popValue(), ce.popValue()
				if c == 0 {
					_, _ = ce.popValue(), ce.popValue() // discard the x1's lo and hi bits.
					ce.pushValue(x2Lo)
					ce.pushValue(x2Hi)
				}
			} else {
				v2 := ce.popValue()
				if c == 0 {
					_ = ce.popValue()
					ce.pushValue(v2)
				}
			}
			frame.pc++
		case wazeroir.OperationKindPick:
			index := len(ce.stack) - 1 - int(op.us[0])
			ce.pushValue(ce.stack[index])
			if op.b3 { // V128 value target.
				ce.pushValue(ce.stack[index+1])
			}
			frame.pc++
		case wazeroir.OperationKindSet:
			if op.b3 { // V128 value target.
				lowIndex := len(ce.stack) - 1 - int(op.us[0])
				highIndex := lowIndex + 1
				hi, lo := ce.popValue(), ce.popValue()
				ce.stack[lowIndex], ce.stack[highIndex] = lo, hi
			} else {
				index := len(ce.stack) - 1 - int(op.us[0])
				ce.stack[index] = ce.popValue()
			}
			frame.pc++
		case wazeroir.OperationKindGlobalGet:
			g := globals[op.us[0]]
			ce.pushValue(g.Val)
			if g.Type.ValType == wasm.ValueTypeV128 {
				ce.pushValue(g.ValHi)
			}
			frame.pc++
		case wazeroir.OperationKindGlobalSet:
			g := globals[op.us[0]]
			if g.Type.ValType == wasm.ValueTypeV128 {
				g.ValHi = ce.popValue()
			}
			g.Val = ce.popValue()
			frame.pc++
		case wazeroir.OperationKindLoad:
			offset := ce.popMemoryOffset(op)
			switch wazeroir.UnsignedType(op.b1) {
			case wazeroir.UnsignedTypeI32, wazeroir.UnsignedTypeF32:
				if val, ok := memoryInst.ReadUint32Le(offset); !ok {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				} else {
					ce.pushValue(uint64(val))
				}
			case wazeroir.UnsignedTypeI64, wazeroir.UnsignedTypeF64:
				if val, ok := memoryInst.ReadUint64Le(offset); !ok {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				} else {
					ce.pushValue(val)
				}
			}
			frame.pc++
		case wazeroir.OperationKindLoad8:
			val, ok := memoryInst.ReadByte(ce.popMemoryOffset(op))
			if !ok {
				panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
			}

			switch wazeroir.SignedInt(op.b1) {
			case wazeroir.SignedInt32:
				ce.pushValue(uint64(uint32(int8(val))))
			case wazeroir.SignedInt64:
				ce.pushValue(uint64(int8(val)))
			case wazeroir.SignedUint32, wazeroir.SignedUint64:
				ce.pushValue(uint64(val))
			}
			frame.pc++
		case wazeroir.OperationKindLoad16:

			val, ok := memoryInst.ReadUint16Le(ce.popMemoryOffset(op))
			if !ok {
				panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
			}

			switch wazeroir.SignedInt(op.b1) {
			case wazeroir.SignedInt32:
				ce.pushValue(uint64(uint32(int16(val))))
			case wazeroir.SignedInt64:
				ce.pushValue(uint64(int16(val)))
			case wazeroir.SignedUint32, wazeroir.SignedUint64:
				ce.pushValue(uint64(val))
			}
			frame.pc++
		case wazeroir.OperationKindLoad32:
			val, ok := memoryInst.ReadUint32Le(ce.popMemoryOffset(op))
			if !ok {
				panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
			}

			if op.b1 == 1 { // Signed
				ce.pushValue(uint64(int32(val)))
			} else {
				ce.pushValue(uint64(val))
			}
			frame.pc++
		case wazeroir.OperationKindStore:
			val := ce.popValue()
			offset := ce.popMemoryOffset(op)
			switch wazeroir.UnsignedType(op.b1) {
			case wazeroir.UnsignedTypeI32, wazeroir.UnsignedTypeF32:
				if !memoryInst.WriteUint32Le(offset, uint32(val)) {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
			case wazeroir.UnsignedTypeI64, wazeroir.UnsignedTypeF64:
				if !memoryInst.WriteUint64Le(offset, val) {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
			}
			frame.pc++
		case wazeroir.OperationKindStore8:
			val := byte(ce.popValue())
			offset := ce.popMemoryOffset(op)
			if !memoryInst.WriteByte(offset, val) {
				panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
			}
			frame.pc++
		case wazeroir.OperationKindStore16:
			val := uint16(ce.popValue())
			offset := ce.popMemoryOffset(op)
			if !memoryInst.WriteUint16Le(offset, val) {
				panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
			}
			frame.pc++
		case wazeroir.OperationKindStore32:
			val := uint32(ce.popValue())
			offset := ce.popMemoryOffset(op)
			if !memoryInst.WriteUint32Le(offset, val) {
				panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
			}
			frame.pc++
		case wazeroir.OperationKindMemorySize:
			ce.pushValue(uint64(memoryInst.PageSize()))
			frame.pc++
		case wazeroir.OperationKindMemoryGrow:
			n := ce.popValue()
			if res, ok := memoryInst.Grow(uint32(n)); !ok {
				ce.pushValue(uint64(0xffffffff)) // = -1 in signed 32-bit integer.
			} else {
				ce.pushValue(uint64(res))
			}
			frame.pc++
		case wazeroir.OperationKindConstI32, wazeroir.OperationKindConstI64,
			wazeroir.OperationKindConstF32, wazeroir.OperationKindConstF64:
			ce.pushValue(op.us[0])
			frame.pc++
		case wazeroir.OperationKindEq:
			var b bool
			switch wazeroir.UnsignedType(op.b1) {
			case wazeroir.UnsignedTypeI32:
				v2, v1 := ce.popValue(), ce.popValue()
				b = uint32(v1) == uint32(v2)
			case wazeroir.UnsignedTypeI64:
				v2, v1 := ce.popValue(), ce.popValue()
				b = v1 == v2
			case wazeroir.UnsignedTypeF32:
				v2, v1 := ce.popValue(), ce.popValue()
				b = math.Float32frombits(uint32(v2)) == math.Float32frombits(uint32(v1))
			case wazeroir.UnsignedTypeF64:
				v2, v1 := ce.popValue(), ce.popValue()
				b = math.Float64frombits(v2) == math.Float64frombits(v1)
			}
			if b {
				ce.pushValue(1)
			} else {
				ce.pushValue(0)
			}
			frame.pc++
		case wazeroir.OperationKindNe:
			var b bool
			switch wazeroir.UnsignedType(op.b1) {
			case wazeroir.UnsignedTypeI32, wazeroir.UnsignedTypeI64:
				v2, v1 := ce.popValue(), ce.popValue()
				b = v1 != v2
			case wazeroir.UnsignedTypeF32:
				v2, v1 := ce.popValue(), ce.popValue()
				b = math.Float32frombits(uint32(v2)) != math.Float32frombits(uint32(v1))
			case wazeroir.UnsignedTypeF64:
				v2, v1 := ce.popValue(), ce.popValue()
				b = math.Float64frombits(v2) != math.Float64frombits(v1)
			}
			if b {
				ce.pushValue(1)
			} else {
				ce.pushValue(0)
			}
			frame.pc++
		case wazeroir.OperationKindEqz:
			if ce.popValue() == 0 {
				ce.pushValue(1)
			} else {
				ce.pushValue(0)
			}
			frame.pc++
		case wazeroir.OperationKindLt:
			v2 := ce.popValue()
			v1 := ce.popValue()
			var b bool
			switch wazeroir.SignedType(op.b1) {
			case wazeroir.SignedTypeInt32:
				b = int32(v1) < int32(v2)
			case wazeroir.SignedTypeInt64:
				b = int64(v1) < int64(v2)
			case wazeroir.SignedTypeUint32, wazeroir.SignedTypeUint64:
				b = v1 < v2
			case wazeroir.SignedTypeFloat32:
				b = math.Float32frombits(uint32(v1)) < math.Float32frombits(uint32(v2))
			case wazeroir.SignedTypeFloat64:
				b = math.Float64frombits(v1) < math.Float64frombits(v2)
			}
			if b {
				ce.pushValue(1)
			} else {
				ce.pushValue(0)
			}
			frame.pc++
		case wazeroir.OperationKindGt:
			v2 := ce.popValue()
			v1 := ce.popValue()
			var b bool
			switch wazeroir.SignedType(op.b1) {
			case wazeroir.SignedTypeInt32:
				b = int32(v1) > int32(v2)
			case wazeroir.SignedTypeInt64:
				b = int64(v1) > int64(v2)
			case wazeroir.SignedTypeUint32, wazeroir.SignedTypeUint64:
				b = v1 > v2
			case wazeroir.SignedTypeFloat32:
				b = math.Float32frombits(uint32(v1)) > math.Float32frombits(uint32(v2))
			case wazeroir.SignedTypeFloat64:
				b = math.Float64frombits(v1) > math.Float64frombits(v2)
			}
			if b {
				ce.pushValue(1)
			} else {
				ce.pushValue(0)
			}
			frame.pc++
		case wazeroir.OperationKindLe:
			v2 := ce.popValue()
			v1 := ce.popValue()
			var b bool
			switch wazeroir.SignedType(op.b1) {
			case wazeroir.SignedTypeInt32:
				b = int32(v1) <= int32(v2)
			case wazeroir.SignedTypeInt64:
				b = int64(v1) <= int64(v2)
			case wazeroir.SignedTypeUint32, wazeroir.SignedTypeUint64:
				b = v1 <= v2
			case wazeroir.SignedTypeFloat32:
				b = math.Float32frombits(uint32(v1)) <= math.Float32frombits(uint32(v2))
			case wazeroir.SignedTypeFloat64:
				b = math.Float64frombits(v1) <= math.Float64frombits(v2)
			}
			if b {
				ce.pushValue(1)
			} else {
				ce.pushValue(0)
			}
			frame.pc++
		case wazeroir.OperationKindGe:
			v2 := ce.popValue()
			v1 := ce.popValue()
			var b bool
			switch wazeroir.SignedType(op.b1) {
			case wazeroir.SignedTypeInt32:
				b = int32(v1) >= int32(v2)
			case wazeroir.SignedTypeInt64:
				b = int64(v1) >= int64(v2)
			case wazeroir.SignedTypeUint32, wazeroir.SignedTypeUint64:
				b = v1 >= v2
			case wazeroir.SignedTypeFloat32:
				b = math.Float32frombits(uint32(v1)) >= math.Float32frombits(uint32(v2))
			case wazeroir.SignedTypeFloat64:
				b = math.Float64frombits(v1) >= math.Float64frombits(v2)
			}
			if b {
				ce.pushValue(1)
			} else {
				ce.pushValue(0)
			}
			frame.pc++
		case wazeroir.OperationKindAdd:
			v2 := ce.popValue()
			v1 := ce.popValue()
			switch wazeroir.UnsignedType(op.b1) {
			case wazeroir.UnsignedTypeI32:
				v := uint32(v1) + uint32(v2)
				ce.pushValue(uint64(v))
			case wazeroir.UnsignedTypeI64:
				ce.pushValue(v1 + v2)
			case wazeroir.UnsignedTypeF32:
				ce.pushValue(addFloat32bits(uint32(v1), uint32(v2)))
			case wazeroir.UnsignedTypeF64:
				v := math.Float64frombits(v1) + math.Float64frombits(v2)
				ce.pushValue(math.Float64bits(v))
			}
			frame.pc++
		case wazeroir.OperationKindSub:
			v2 := ce.popValue()
			v1 := ce.popValue()
			switch wazeroir.UnsignedType(op.b1) {
			case wazeroir.UnsignedTypeI32:
				ce.pushValue(uint64(uint32(v1) - uint32(v2)))
			case wazeroir.UnsignedTypeI64:
				ce.pushValue(v1 - v2)
			case wazeroir.UnsignedTypeF32:
				ce.pushValue(subFloat32bits(uint32(v1), uint32(v2)))
			case wazeroir.UnsignedTypeF64:
				v := math.Float64frombits(v1) - math.Float64frombits(v2)
				ce.pushValue(math.Float64bits(v))
			}
			frame.pc++
		case wazeroir.OperationKindMul:
			v2 := ce.popValue()
			v1 := ce.popValue()
			switch wazeroir.UnsignedType(op.b1) {
			case wazeroir.UnsignedTypeI32:
				ce.pushValue(uint64(uint32(v1) * uint32(v2)))
			case wazeroir.UnsignedTypeI64:
				ce.pushValue(v1 * v2)
			case wazeroir.UnsignedTypeF32:
				ce.pushValue(mulFloat32bits(uint32(v1), uint32(v2)))
			case wazeroir.UnsignedTypeF64:
				v := math.Float64frombits(v2) * math.Float64frombits(v1)
				ce.pushValue(math.Float64bits(v))
			}
			frame.pc++
		case wazeroir.OperationKindClz:
			v := ce.popValue()
			if op.b1 == 0 {
				// UnsignedInt32
				ce.pushValue(uint64(bits.LeadingZeros32(uint32(v))))
			} else {
				// UnsignedInt64
				ce.pushValue(uint64(bits.LeadingZeros64(v)))
			}
			frame.pc++
		case wazeroir.OperationKindCtz:
			v := ce.popValue()
			if op.b1 == 0 {
				// UnsignedInt32
				ce.pushValue(uint64(bits.TrailingZeros32(uint32(v))))
			} else {
				// UnsignedInt64
				ce.pushValue(uint64(bits.TrailingZeros64(v)))
			}
			frame.pc++
		case wazeroir.OperationKindPopcnt:
			v := ce.popValue()
			if op.b1 == 0 {
				// UnsignedInt32
				ce.pushValue(uint64(bits.OnesCount32(uint32(v))))
			} else {
				// UnsignedInt64
				ce.pushValue(uint64(bits.OnesCount64(v)))
			}
			frame.pc++
		case wazeroir.OperationKindDiv:
			// If an integer, check we won't divide by zero.
			t := wazeroir.SignedType(op.b1)
			v2, v1 := ce.popValue(), ce.popValue()
			switch t {
			case wazeroir.SignedTypeFloat32, wazeroir.SignedTypeFloat64: // not integers
			default:
				if v2 == 0 {
					panic(wasmruntime.ErrRuntimeIntegerDivideByZero)
				}
			}

			switch t {
			case wazeroir.SignedTypeInt32:
				d := int32(v2)
				n := int32(v1)
				if n == math.MinInt32 && d == -1 {
					panic(wasmruntime.ErrRuntimeIntegerOverflow)
				}
				ce.pushValue(uint64(uint32(n / d)))
			case wazeroir.SignedTypeInt64:
				d := int64(v2)
				n := int64(v1)
				if n == math.MinInt64 && d == -1 {
					panic(wasmruntime.ErrRuntimeIntegerOverflow)
				}
				ce.pushValue(uint64(n / d))
			case wazeroir.SignedTypeUint32:
				d := uint32(v2)
				n := uint32(v1)
				ce.pushValue(uint64(n / d))
			case wazeroir.SignedTypeUint64:
				d := v2
				n := v1
				ce.pushValue(n / d)
			case wazeroir.SignedTypeFloat32:
				ce.pushValue(divFloat32bits(uint32(v1), uint32(v2)))
			case wazeroir.SignedTypeFloat64:
				ce.pushValue(math.Float64bits(math.Float64frombits(v1) / math.Float64frombits(v2)))
			}
			frame.pc++
		case wazeroir.OperationKindRem:
			v2, v1 := ce.popValue(), ce.popValue()
			if v2 == 0 {
				panic(wasmruntime.ErrRuntimeIntegerDivideByZero)
			}
			switch wazeroir.SignedInt(op.b1) {
			case wazeroir.SignedInt32:
				d := int32(v2)
				n := int32(v1)
				ce.pushValue(uint64(uint32(n % d)))
			case wazeroir.SignedInt64:
				d := int64(v2)
				n := int64(v1)
				ce.pushValue(uint64(n % d))
			case wazeroir.SignedUint32:
				d := uint32(v2)
				n := uint32(v1)
				ce.pushValue(uint64(n % d))
			case wazeroir.SignedUint64:
				d := v2
				n := v1
				ce.pushValue(n % d)
			}
			frame.pc++
		case wazeroir.OperationKindAnd:
			v2 := ce.popValue()
			v1 := ce.popValue()
			if op.b1 == 0 {
				// UnsignedInt32
				ce.pushValue(uint64(uint32(v2) & uint32(v1)))
			} else {
				// UnsignedInt64
				ce.pushValue(uint64(v2 & v1))
			}
			frame.pc++
		case wazeroir.OperationKindOr:
			v2 := ce.popValue()
			v1 := ce.popValue()
			if op.b1 == 0 {
				// UnsignedInt32
				ce.pushValue(uint64(uint32(v2) | uint32(v1)))
			} else {
				// UnsignedInt64
				ce.pushValue(uint64(v2 | v1))
			}
			frame.pc++
		case wazeroir.OperationKindXor:
			v2 := ce.popValue()
			v1 := ce.popValue()
			if op.b1 == 0 {
				// UnsignedInt32
				ce.pushValue(uint64(uint32(v2) ^ uint32(v1)))
			} else {
				// UnsignedInt64
				ce.pushValue(uint64(v2 ^ v1))
			}
			frame.pc++
		case wazeroir.OperationKindShl:
			v2 := ce.popValue()
			v1 := ce.popValue()
			if op.b1 == 0 {
				// UnsignedInt32
				ce.pushValue(uint64(uint32(v1) << (uint32(v2) % 32)))
			} else {
				// UnsignedInt64
				ce.pushValue(v1 << (v2 % 64))
			}
			frame.pc++
		case wazeroir.OperationKindShr:
			v2 := ce.popValue()
			v1 := ce.popValue()
			switch wazeroir.SignedInt(op.b1) {
			case wazeroir.SignedInt32:
				ce.pushValue(uint64(uint32(int32(v1) >> (uint32(v2) % 32))))
			case wazeroir.SignedInt64:
				ce.pushValue(uint64(int64(v1) >> (v2 % 64)))
			case wazeroir.SignedUint32:
				ce.pushValue(uint64(uint32(v1) >> (uint32(v2) % 32)))
			case wazeroir.SignedUint64:
				ce.pushValue(v1 >> (v2 % 64))
			}
			frame.pc++
		case wazeroir.OperationKindRotl:
			v2 := ce.popValue()
			v1 := ce.popValue()
			if op.b1 == 0 {
				// UnsignedInt32
				ce.pushValue(uint64(bits.RotateLeft32(uint32(v1), int(v2))))
			} else {
				// UnsignedInt64
				ce.pushValue(uint64(bits.RotateLeft64(v1, int(v2))))
			}
			frame.pc++
		case wazeroir.OperationKindRotr:
			v2 := ce.popValue()
			v1 := ce.popValue()
			if op.b1 == 0 {
				// UnsignedInt32
				ce.pushValue(uint64(bits.RotateLeft32(uint32(v1), -int(v2))))
			} else {
				// UnsignedInt64
				ce.pushValue(uint64(bits.RotateLeft64(v1, -int(v2))))
			}
			frame.pc++
		case wazeroir.OperationKindAbs:
			if op.b1 == 0 {
				// Float32
				const mask uint32 = 1 << 31
				ce.pushValue(uint64(uint32(ce.popValue()) &^ mask))
			} else {
				// Float64
				const mask uint64 = 1 << 63
				ce.pushValue(ce.popValue() &^ mask)
			}
			frame.pc++
		case wazeroir.OperationKindNeg:
			if op.b1 == 0 {
				// Float32
				v := -math.Float32frombits(uint32(ce.popValue()))
				ce.pushValue(uint64(math.Float32bits(v)))
			} else {
				// Float64
				v := -math.Float64frombits(ce.popValue())
				ce.pushValue(math.Float64bits(v))
			}
			frame.pc++
		case wazeroir.OperationKindCeil:
			if op.b1 == 0 {
				// Float32
				v := moremath.WasmCompatCeilF32(math.Float32frombits(uint32(ce.popValue())))
				ce.pushValue(uint64(math.Float32bits(v)))
			} else {
				// Float64
				v := moremath.WasmCompatCeilF64(math.Float64frombits(ce.popValue()))
				ce.pushValue(math.Float64bits(v))
			}
			frame.pc++
		case wazeroir.OperationKindFloor:
			if op.b1 == 0 {
				// Float32
				v := moremath.WasmCompatFloorF32(math.Float32frombits(uint32(ce.popValue())))
				ce.pushValue(uint64(math.Float32bits(v)))
			} else {
				// Float64
				v := moremath.WasmCompatFloorF64(math.Float64frombits(ce.popValue()))
				ce.pushValue(math.Float64bits(v))
			}
			frame.pc++
		case wazeroir.OperationKindTrunc:
			if op.b1 == 0 {
				// Float32
				v := moremath.WasmCompatTruncF32(math.Float32frombits(uint32(ce.popValue())))
				ce.pushValue(uint64(math.Float32bits(v)))
			} else {
				// Float64
				v := moremath.WasmCompatTruncF64(math.Float64frombits(ce.popValue()))
				ce.pushValue(math.Float64bits(v))
			}
			frame.pc++
		case wazeroir.OperationKindNearest:
			if op.b1 == 0 {
				// Float32
				f := math.Float32frombits(uint32(ce.popValue()))
				ce.pushValue(uint64(math.Float32bits(moremath.WasmCompatNearestF32(f))))
			} else {
				// Float64
				f := math.Float64frombits(ce.popValue())
				ce.pushValue(math.Float64bits(moremath.WasmCompatNearestF64(f)))
			}
			frame.pc++
		case wazeroir.OperationKindSqrt:
			if op.b1 == 0 {
				// Float32
				v := math.Sqrt(float64(math.Float32frombits(uint32(ce.popValue()))))
				ce.pushValue(uint64(math.Float32bits(float32(v))))
			} else {
				// Float64
				v := math.Sqrt(math.Float64frombits(ce.popValue()))
				ce.pushValue(math.Float64bits(v))
			}
			frame.pc++
		case wazeroir.OperationKindMin:
			if op.b1 == 0 {
				// Float32
				ce.pushValue(WasmCompatMin32bits(uint32(ce.popValue()), uint32(ce.popValue())))
			} else {
				v2 := math.Float64frombits(ce.popValue())
				v1 := math.Float64frombits(ce.popValue())
				ce.pushValue(math.Float64bits(moremath.WasmCompatMin64(v1, v2)))
			}
			frame.pc++
		case wazeroir.OperationKindMax:
			if op.b1 == 0 {
				ce.pushValue(WasmCompatMax32bits(uint32(ce.popValue()), uint32(ce.popValue())))
			} else {
				// Float64
				v2 := math.Float64frombits(ce.popValue())
				v1 := math.Float64frombits(ce.popValue())
				ce.pushValue(math.Float64bits(moremath.WasmCompatMax64(v1, v2)))
			}
			frame.pc++
		case wazeroir.OperationKindCopysign:
			if op.b1 == 0 {
				// Float32
				v2 := uint32(ce.popValue())
				v1 := uint32(ce.popValue())
				const signbit = 1 << 31
				ce.pushValue(uint64(v1&^signbit | v2&signbit))
			} else {
				// Float64
				v2 := ce.popValue()
				v1 := ce.popValue()
				const signbit = 1 << 63
				ce.pushValue(v1&^signbit | v2&signbit)
			}
			frame.pc++
		case wazeroir.OperationKindI32WrapFromI64:
			ce.pushValue(uint64(uint32(ce.popValue())))
			frame.pc++
		case wazeroir.OperationKindITruncFromF:
			if op.b1 == 0 {
				// Float32
				switch wazeroir.SignedInt(op.b2) {
				case wazeroir.SignedInt32:
					v := math.Trunc(float64(math.Float32frombits(uint32(ce.popValue()))))
					if math.IsNaN(v) { // NaN cannot be compared with themselves, so we have to use IsNaN
						if op.b3 {
							// non-trapping conversion must cast nan to zero.
							v = 0
						} else {
							panic(wasmruntime.ErrRuntimeInvalidConversionToInteger)
						}
					} else if v < math.MinInt32 || v > math.MaxInt32 {
						if op.b3 {
							// non-trapping conversion must "saturate" the value for overflowing sources.
							if v < 0 {
								v = math.MinInt32
							} else {
								v = math.MaxInt32
							}
						} else {
							panic(wasmruntime.ErrRuntimeIntegerOverflow)
						}
					}
					ce.pushValue(uint64(uint32(int32(v))))
				case wazeroir.SignedInt64:
					v := math.Trunc(float64(math.Float32frombits(uint32(ce.popValue()))))
					res := int64(v)
					if math.IsNaN(v) { // NaN cannot be compared with themselves, so we have to use IsNaN
						if op.b3 {
							// non-trapping conversion must cast nan to zero.
							res = 0
						} else {
							panic(wasmruntime.ErrRuntimeInvalidConversionToInteger)
						}
					} else if v < math.MinInt64 || v >= math.MaxInt64 {
						// Note: math.MaxInt64 is rounded up to math.MaxInt64+1 in 64-bit float representation,
						// and that's why we use '>=' not '>' to check overflow.
						if op.b3 {
							// non-trapping conversion must "saturate" the value for overflowing sources.
							if v < 0 {
								res = math.MinInt64
							} else {
								res = math.MaxInt64
							}
						} else {
							panic(wasmruntime.ErrRuntimeIntegerOverflow)
						}
					}
					ce.pushValue(uint64(res))
				case wazeroir.SignedUint32:
					v := math.Trunc(float64(math.Float32frombits(uint32(ce.popValue()))))
					if math.IsNaN(v) { // NaN cannot be compared with themselves, so we have to use IsNaN
						if op.b3 {
							// non-trapping conversion must cast nan to zero.
							v = 0
						} else {
							panic(wasmruntime.ErrRuntimeInvalidConversionToInteger)
						}
					} else if v < 0 || v > math.MaxUint32 {
						if op.b3 {
							// non-trapping conversion must "saturate" the value for overflowing source.
							if v < 0 {
								v = 0
							} else {
								v = math.MaxUint32
							}
						} else {
							panic(wasmruntime.ErrRuntimeIntegerOverflow)
						}
					}
					ce.pushValue(uint64(uint32(v)))
				case wazeroir.SignedUint64:
					v := math.Trunc(float64(math.Float32frombits(uint32(ce.popValue()))))
					res := uint64(v)
					if math.IsNaN(v) { // NaN cannot be compared with themselves, so we have to use IsNaN
						if op.b3 {
							// non-trapping conversion must cast nan to zero.
							res = 0
						} else {
							panic(wasmruntime.ErrRuntimeInvalidConversionToInteger)
						}
					} else if v < 0 || v >= math.MaxUint64 {
						// Note: math.MaxUint64 is rounded up to math.MaxUint64+1 in 64-bit float representation,
						// and that's why we use '>=' not '>' to check overflow.
						if op.b3 {
							// non-trapping conversion must "saturate" the value for overflowing source.
							if v < 0 {
								res = 0
							} else {
								res = math.MaxUint64
							}
						} else {
							panic(wasmruntime.ErrRuntimeIntegerOverflow)
						}
					}
					ce.pushValue(res)
				}
			} else {
				// Float64
				switch wazeroir.SignedInt(op.b2) {
				case wazeroir.SignedInt32:
					v := math.Trunc(math.Float64frombits(ce.popValue()))
					if math.IsNaN(v) { // NaN cannot be compared with themselves, so we have to use IsNaN
						if op.b3 {
							// non-trapping conversion must cast nan to zero.
							v = 0
						} else {
							panic(wasmruntime.ErrRuntimeInvalidConversionToInteger)
						}
					} else if v < math.MinInt32 || v > math.MaxInt32 {
						if op.b3 {
							// non-trapping conversion must "saturate" the value for overflowing source.
							if v < 0 {
								v = math.MinInt32
							} else {
								v = math.MaxInt32
							}
						} else {
							panic(wasmruntime.ErrRuntimeIntegerOverflow)
						}
					}
					ce.pushValue(uint64(uint32(int32(v))))
				case wazeroir.SignedInt64:
					v := math.Trunc(math.Float64frombits(ce.popValue()))
					res := int64(v)
					if math.IsNaN(v) { // NaN cannot be compared with themselves, so we have to use IsNaN
						if op.b3 {
							// non-trapping conversion must cast nan to zero.
							res = 0
						} else {
							panic(wasmruntime.ErrRuntimeInvalidConversionToInteger)
						}
					} else if v < math.MinInt64 || v >= math.MaxInt64 {
						// Note: math.MaxInt64 is rounded up to math.MaxInt64+1 in 64-bit float representation,
						// and that's why we use '>=' not '>' to check overflow.
						if op.b3 {
							// non-trapping conversion must "saturate" the value for overflowing source.
							if v < 0 {
								res = math.MinInt64
							} else {
								res = math.MaxInt64
							}
						} else {
							panic(wasmruntime.ErrRuntimeIntegerOverflow)
						}
					}
					ce.pushValue(uint64(res))
				case wazeroir.SignedUint32:
					v := math.Trunc(math.Float64frombits(ce.popValue()))
					if math.IsNaN(v) { // NaN cannot be compared with themselves, so we have to use IsNaN
						if op.b3 {
							// non-trapping conversion must cast nan to zero.
							v = 0
						} else {
							panic(wasmruntime.ErrRuntimeInvalidConversionToInteger)
						}
					} else if v < 0 || v > math.MaxUint32 {
						if op.b3 {
							// non-trapping conversion must "saturate" the value for overflowing source.
							if v < 0 {
								v = 0
							} else {
								v = math.MaxUint32
							}
						} else {
							panic(wasmruntime.ErrRuntimeIntegerOverflow)
						}
					}
					ce.pushValue(uint64(uint32(v)))
				case wazeroir.SignedUint64:
					v := math.Trunc(math.Float64frombits(ce.popValue()))
					res := uint64(v)
					if math.IsNaN(v) { // NaN cannot be compared with themselves, so we have to use IsNaN
						if op.b3 {
							// non-trapping conversion must cast nan to zero.
							res = 0
						} else {
							panic(wasmruntime.ErrRuntimeInvalidConversionToInteger)
						}
					} else if v < 0 || v >= math.MaxUint64 {
						// Note: math.MaxUint64 is rounded up to math.MaxUint64+1 in 64-bit float representation,
						// and that's why we use '>=' not '>' to check overflow.
						if op.b3 {
							// non-trapping conversion must "saturate" the value for overflowing source.
							if v < 0 {
								res = 0
							} else {
								res = math.MaxUint64
							}
						} else {
							panic(wasmruntime.ErrRuntimeIntegerOverflow)
						}
					}
					ce.pushValue(res)
				}
			}
			frame.pc++
		case wazeroir.OperationKindFConvertFromI:
			switch wazeroir.SignedInt(op.b1) {
			case wazeroir.SignedInt32:
				if op.b2 == 0 {
					// Float32
					v := float32(int32(ce.popValue()))
					ce.pushValue(uint64(math.Float32bits(v)))
				} else {
					// Float64
					v := float64(int32(ce.popValue()))
					ce.pushValue(math.Float64bits(v))
				}
			case wazeroir.SignedInt64:
				if op.b2 == 0 {
					// Float32
					v := float32(int64(ce.popValue()))
					ce.pushValue(uint64(math.Float32bits(v)))
				} else {
					// Float64
					v := float64(int64(ce.popValue()))
					ce.pushValue(math.Float64bits(v))
				}
			case wazeroir.SignedUint32:
				if op.b2 == 0 {
					// Float32
					v := float32(uint32(ce.popValue()))
					ce.pushValue(uint64(math.Float32bits(v)))
				} else {
					// Float64
					v := float64(uint32(ce.popValue()))
					ce.pushValue(math.Float64bits(v))
				}
			case wazeroir.SignedUint64:
				if op.b2 == 0 {
					// Float32
					v := float32(ce.popValue())
					ce.pushValue(uint64(math.Float32bits(v)))
				} else {
					// Float64
					v := float64(ce.popValue())
					ce.pushValue(math.Float64bits(v))
				}
			}
			frame.pc++
		case wazeroir.OperationKindF32DemoteFromF64:
			v := float32(math.Float64frombits(ce.popValue()))
			ce.pushValue(uint64(math.Float32bits(v)))
			frame.pc++
		case wazeroir.OperationKindF64PromoteFromF32:
			v := float64(math.Float32frombits(uint32(ce.popValue())))
			ce.pushValue(math.Float64bits(v))
			frame.pc++
		case wazeroir.OperationKindExtend:
			if op.b1 == 1 {
				// Signed.
				v := int64(int32(ce.popValue()))
				ce.pushValue(uint64(v))
			} else {
				v := uint64(uint32(ce.popValue()))
				ce.pushValue(v)
			}
			frame.pc++
		case wazeroir.OperationKindSignExtend32From8:
			v := uint32(int8(ce.popValue()))
			ce.pushValue(uint64(v))
			frame.pc++
		case wazeroir.OperationKindSignExtend32From16:
			v := uint32(int16(ce.popValue()))
			ce.pushValue(uint64(v))
			frame.pc++
		case wazeroir.OperationKindSignExtend64From8:
			v := int64(int8(ce.popValue()))
			ce.pushValue(uint64(v))
			frame.pc++
		case wazeroir.OperationKindSignExtend64From16:
			v := int64(int16(ce.popValue()))
			ce.pushValue(uint64(v))
			frame.pc++
		case wazeroir.OperationKindSignExtend64From32:
			v := int64(int32(ce.popValue()))
			ce.pushValue(uint64(v))
			frame.pc++
		case wazeroir.OperationKindMemoryInit:
			dataInstance := dataInstances[op.us[0]]
			copySize := ce.popValue()
			inDataOffset := ce.popValue()
			inMemoryOffset := ce.popValue()
			if inDataOffset+copySize > uint64(len(dataInstance)) ||
				inMemoryOffset+copySize > uint64(len(memoryInst.Buffer)) {
				panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
			} else if copySize != 0 {
				copy(memoryInst.Buffer[inMemoryOffset:inMemoryOffset+copySize], dataInstance[inDataOffset:])
			}
			frame.pc++
		case wazeroir.OperationKindDataDrop:
			dataInstances[op.us[0]] = nil
			frame.pc++
		case wazeroir.OperationKindMemoryCopy:
			memLen := uint64(len(memoryInst.Buffer))
			copySize := ce.popValue()
			sourceOffset := ce.popValue()
			destinationOffset := ce.popValue()
			if sourceOffset+copySize > memLen || destinationOffset+copySize > memLen {
				panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
			} else if copySize != 0 {
				copy(memoryInst.Buffer[destinationOffset:],
					memoryInst.Buffer[sourceOffset:sourceOffset+copySize])
			}
			frame.pc++
		case wazeroir.OperationKindMemoryFill:
			fillSize := ce.popValue()
			value := byte(ce.popValue())
			offset := ce.popValue()
			if fillSize+offset > uint64(len(memoryInst.Buffer)) {
				panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
			} else if fillSize != 0 {
				// Uses the copy trick for faster filling buffer.
				// https://gist.github.com/taylorza/df2f89d5f9ab3ffd06865062a4cf015d
				buf := memoryInst.Buffer[offset : offset+fillSize]
				buf[0] = value
				for i := 1; i < len(buf); i *= 2 {
					copy(buf[i:], buf[:i])
				}
			}
			frame.pc++
		case wazeroir.OperationKindTableInit:
			elementInstance := elementInstances[op.us[0]]
			copySize := ce.popValue()
			inElementOffset := ce.popValue()
			inTableOffset := ce.popValue()
			table := tables[op.us[1]]
			if inElementOffset+copySize > uint64(len(elementInstance.References)) ||
				inTableOffset+copySize > uint64(len(table.References)) {
				panic(wasmruntime.ErrRuntimeInvalidTableAccess)
			} else if copySize != 0 {
				copy(table.References[inTableOffset:inTableOffset+copySize], elementInstance.References[inElementOffset:])
			}
			frame.pc++
		case wazeroir.OperationKindElemDrop:
			elementInstances[op.us[0]].References = nil
			frame.pc++
		case wazeroir.OperationKindTableCopy:
			srcTable, dstTable := tables[op.us[0]].References, tables[op.us[1]].References
			copySize := ce.popValue()
			sourceOffset := ce.popValue()
			destinationOffset := ce.popValue()
			if sourceOffset+copySize > uint64(len(srcTable)) || destinationOffset+copySize > uint64(len(dstTable)) {
				panic(wasmruntime.ErrRuntimeInvalidTableAccess)
			} else if copySize != 0 {
				copy(dstTable[destinationOffset:], srcTable[sourceOffset:sourceOffset+copySize])
			}
			frame.pc++
		case wazeroir.OperationKindRefFunc:
			ce.pushValue(uint64(uintptr(unsafe.Pointer(&functions[op.us[0]]))))
			frame.pc++
		case wazeroir.OperationKindTableGet:
			table := tables[op.us[0]]

			offset := ce.popValue()
			if offset >= uint64(len(table.References)) {
				panic(wasmruntime.ErrRuntimeInvalidTableAccess)
			}

			ce.pushValue(uint64(table.References[offset]))
			frame.pc++
		case wazeroir.OperationKindTableSet:
			table := tables[op.us[0]]
			ref := ce.popValue()

			offset := ce.popValue()
			if offset >= uint64(len(table.References)) {
				panic(wasmruntime.ErrRuntimeInvalidTableAccess)
			}

			table.References[offset] = uintptr(ref) // externrefs are opaque uint64.
			frame.pc++
		case wazeroir.OperationKindTableSize:
			table := tables[op.us[0]]
			ce.pushValue(uint64(len(table.References)))
			frame.pc++
		case wazeroir.OperationKindTableGrow:
			table := tables[op.us[0]]
			num, ref := ce.popValue(), ce.popValue()
			ret := table.Grow(uint32(num), uintptr(ref))
			ce.pushValue(uint64(ret))
			frame.pc++
		case wazeroir.OperationKindTableFill:
			table := tables[op.us[0]]
			num := ce.popValue()
			ref := uintptr(ce.popValue())
			offset := ce.popValue()
			if num+offset > uint64(len(table.References)) {
				panic(wasmruntime.ErrRuntimeInvalidTableAccess)
			} else if num > 0 {
				// Uses the copy trick for faster filling the region with the value.
				// https://gist.github.com/taylorza/df2f89d5f9ab3ffd06865062a4cf015d
				targetRegion := table.References[offset : offset+num]
				targetRegion[0] = ref
				for i := 1; i < len(targetRegion); i *= 2 {
					copy(targetRegion[i:], targetRegion[:i])
				}
			}
			frame.pc++
		case wazeroir.OperationKindV128Const:
			lo, hi := op.us[0], op.us[1]
			ce.pushValue(lo)
			ce.pushValue(hi)
			frame.pc++
		case wazeroir.OperationKindV128Add:
			yHigh, yLow := ce.popValue(), ce.popValue()
			xHigh, xLow := ce.popValue(), ce.popValue()
			switch op.b1 {
			case wazeroir.ShapeI8x16:
				ce.pushValue(
					uint64(uint8(xLow>>8)+uint8(yLow>>8))<<8 | uint64(uint8(xLow)+uint8(yLow)) |
						uint64(uint8(xLow>>24)+uint8(yLow>>24))<<24 | uint64(uint8(xLow>>16)+uint8(yLow>>16))<<16 |
						uint64(uint8(xLow>>40)+uint8(yLow>>40))<<40 | uint64(uint8(xLow>>32)+uint8(yLow>>32))<<32 |
						uint64(uint8(xLow>>56)+uint8(yLow>>56))<<56 | uint64(uint8(xLow>>48)+uint8(yLow>>48))<<48,
				)
				ce.pushValue(
					uint64(uint8(xHigh>>8)+uint8(yHigh>>8))<<8 | uint64(uint8(xHigh)+uint8(yHigh)) |
						uint64(uint8(xHigh>>24)+uint8(yHigh>>24))<<24 | uint64(uint8(xHigh>>16)+uint8(yHigh>>16))<<16 |
						uint64(uint8(xHigh>>40)+uint8(yHigh>>40))<<40 | uint64(uint8(xHigh>>32)+uint8(yHigh>>32))<<32 |
						uint64(uint8(xHigh>>56)+uint8(yHigh>>56))<<56 | uint64(uint8(xHigh>>48)+uint8(yHigh>>48))<<48,
				)
			case wazeroir.ShapeI16x8:
				ce.pushValue(
					uint64(uint16(xLow>>16+yLow>>16))<<16 | uint64(uint16(xLow)+uint16(yLow)) |
						uint64(uint16(xLow>>48+yLow>>48))<<48 | uint64(uint16(xLow>>32+yLow>>32))<<32,
				)
				ce.pushValue(
					uint64(uint16(xHigh>>16)+uint16(yHigh>>16))<<16 | uint64(uint16(xHigh)+uint16(yHigh)) |
						uint64(uint16(xHigh>>48)+uint16(yHigh>>48))<<48 | uint64(uint16(xHigh>>32)+uint16(yHigh>>32))<<32,
				)
			case wazeroir.ShapeI32x4:
				ce.pushValue(uint64(uint32(xLow>>32)+uint32(yLow>>32))<<32 | uint64(uint32(xLow)+uint32(yLow)))
				ce.pushValue(uint64(uint32(xHigh>>32)+uint32(yHigh>>32))<<32 | uint64(uint32(xHigh)+uint32(yHigh)))
			case wazeroir.ShapeI64x2:
				ce.pushValue(xLow + yLow)
				ce.pushValue(xHigh + yHigh)
			case wazeroir.ShapeF32x4:
				ce.pushValue(
					addFloat32bits(uint32(xLow), uint32(yLow)) | addFloat32bits(uint32(xLow>>32), uint32(yLow>>32))<<32,
				)
				ce.pushValue(
					addFloat32bits(uint32(xHigh), uint32(yHigh)) | addFloat32bits(uint32(xHigh>>32), uint32(yHigh>>32))<<32,
				)
			case wazeroir.ShapeF64x2:
				ce.pushValue(math.Float64bits(math.Float64frombits(xLow) + math.Float64frombits(yLow)))
				ce.pushValue(math.Float64bits(math.Float64frombits(xHigh) + math.Float64frombits(yHigh)))
			}
			frame.pc++
		case wazeroir.OperationKindV128Sub:
			yHigh, yLow := ce.popValue(), ce.popValue()
			xHigh, xLow := ce.popValue(), ce.popValue()
			switch op.b1 {
			case wazeroir.ShapeI8x16:
				ce.pushValue(
					uint64(uint8(xLow>>8)-uint8(yLow>>8))<<8 | uint64(uint8(xLow)-uint8(yLow)) |
						uint64(uint8(xLow>>24)-uint8(yLow>>24))<<24 | uint64(uint8(xLow>>16)-uint8(yLow>>16))<<16 |
						uint64(uint8(xLow>>40)-uint8(yLow>>40))<<40 | uint64(uint8(xLow>>32)-uint8(yLow>>32))<<32 |
						uint64(uint8(xLow>>56)-uint8(yLow>>56))<<56 | uint64(uint8(xLow>>48)-uint8(yLow>>48))<<48,
				)
				ce.pushValue(
					uint64(uint8(xHigh>>8)-uint8(yHigh>>8))<<8 | uint64(uint8(xHigh)-uint8(yHigh)) |
						uint64(uint8(xHigh>>24)-uint8(yHigh>>24))<<24 | uint64(uint8(xHigh>>16)-uint8(yHigh>>16))<<16 |
						uint64(uint8(xHigh>>40)-uint8(yHigh>>40))<<40 | uint64(uint8(xHigh>>32)-uint8(yHigh>>32))<<32 |
						uint64(uint8(xHigh>>56)-uint8(yHigh>>56))<<56 | uint64(uint8(xHigh>>48)-uint8(yHigh>>48))<<48,
				)
			case wazeroir.ShapeI16x8:
				ce.pushValue(
					uint64(uint16(xLow>>16)-uint16(yLow>>16))<<16 | uint64(uint16(xLow)-uint16(yLow)) |
						uint64(uint16(xLow>>48)-uint16(yLow>>48))<<48 | uint64(uint16(xLow>>32)-uint16(yLow>>32))<<32,
				)
				ce.pushValue(
					uint64(uint16(xHigh>>16)-uint16(yHigh>>16))<<16 | uint64(uint16(xHigh)-uint16(yHigh)) |
						uint64(uint16(xHigh>>48)-uint16(yHigh>>48))<<48 | uint64(uint16(xHigh>>32)-uint16(yHigh>>32))<<32,
				)
			case wazeroir.ShapeI32x4:
				ce.pushValue(uint64(uint32(xLow>>32-yLow>>32))<<32 | uint64(uint32(xLow)-uint32(yLow)))
				ce.pushValue(uint64(uint32(xHigh>>32-yHigh>>32))<<32 | uint64(uint32(xHigh)-uint32(yHigh)))
			case wazeroir.ShapeI64x2:
				ce.pushValue(xLow - yLow)
				ce.pushValue(xHigh - yHigh)
			case wazeroir.ShapeF32x4:
				ce.pushValue(
					subFloat32bits(uint32(xLow), uint32(yLow)) | subFloat32bits(uint32(xLow>>32), uint32(yLow>>32))<<32,
				)
				ce.pushValue(
					subFloat32bits(uint32(xHigh), uint32(yHigh)) | subFloat32bits(uint32(xHigh>>32), uint32(yHigh>>32))<<32,
				)
			case wazeroir.ShapeF64x2:
				ce.pushValue(math.Float64bits(math.Float64frombits(xLow) - math.Float64frombits(yLow)))
				ce.pushValue(math.Float64bits(math.Float64frombits(xHigh) - math.Float64frombits(yHigh)))
			}
			frame.pc++
		case wazeroir.OperationKindV128Load:
			offset := ce.popMemoryOffset(op)
			switch op.b1 {
			case wazeroir.V128LoadType128:
				lo, ok := memoryInst.ReadUint64Le(offset)
				if !ok {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
				ce.pushValue(lo)
				hi, ok := memoryInst.ReadUint64Le(offset + 8)
				if !ok {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
				ce.pushValue(hi)
			case wazeroir.V128LoadType8x8s:
				data, ok := memoryInst.Read(offset, 8)
				if !ok {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
				ce.pushValue(
					uint64(uint16(int8(data[3])))<<48 | uint64(uint16(int8(data[2])))<<32 | uint64(uint16(int8(data[1])))<<16 | uint64(uint16(int8(data[0]))),
				)
				ce.pushValue(
					uint64(uint16(int8(data[7])))<<48 | uint64(uint16(int8(data[6])))<<32 | uint64(uint16(int8(data[5])))<<16 | uint64(uint16(int8(data[4]))),
				)
			case wazeroir.V128LoadType8x8u:
				data, ok := memoryInst.Read(offset, 8)
				if !ok {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
				ce.pushValue(
					uint64(data[3])<<48 | uint64(data[2])<<32 | uint64(data[1])<<16 | uint64(data[0]),
				)
				ce.pushValue(
					uint64(data[7])<<48 | uint64(data[6])<<32 | uint64(data[5])<<16 | uint64(data[4]),
				)
			case wazeroir.V128LoadType16x4s:
				data, ok := memoryInst.Read(offset, 8)
				if !ok {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
				ce.pushValue(
					uint64(int16(binary.LittleEndian.Uint16(data[2:])))<<32 |
						uint64(uint32(int16(binary.LittleEndian.Uint16(data)))),
				)
				ce.pushValue(
					uint64(uint32(int16(binary.LittleEndian.Uint16(data[6:]))))<<32 |
						uint64(uint32(int16(binary.LittleEndian.Uint16(data[4:])))),
				)
			case wazeroir.V128LoadType16x4u:
				data, ok := memoryInst.Read(offset, 8)
				if !ok {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
				ce.pushValue(
					uint64(binary.LittleEndian.Uint16(data[2:]))<<32 | uint64(binary.LittleEndian.Uint16(data)),
				)
				ce.pushValue(
					uint64(binary.LittleEndian.Uint16(data[6:]))<<32 | uint64(binary.LittleEndian.Uint16(data[4:])),
				)
			case wazeroir.V128LoadType32x2s:
				data, ok := memoryInst.Read(offset, 8)
				if !ok {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
				ce.pushValue(uint64(int32(binary.LittleEndian.Uint32(data))))
				ce.pushValue(uint64(int32(binary.LittleEndian.Uint32(data[4:]))))
			case wazeroir.V128LoadType32x2u:
				data, ok := memoryInst.Read(offset, 8)
				if !ok {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
				ce.pushValue(uint64(binary.LittleEndian.Uint32(data)))
				ce.pushValue(uint64(binary.LittleEndian.Uint32(data[4:])))
			case wazeroir.V128LoadType8Splat:
				v, ok := memoryInst.ReadByte(offset)
				if !ok {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
				v8 := uint64(v)<<56 | uint64(v)<<48 | uint64(v)<<40 | uint64(v)<<32 |
					uint64(v)<<24 | uint64(v)<<16 | uint64(v)<<8 | uint64(v)
				ce.pushValue(v8)
				ce.pushValue(v8)
			case wazeroir.V128LoadType16Splat:
				v, ok := memoryInst.ReadUint16Le(offset)
				if !ok {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
				v4 := uint64(v)<<48 | uint64(v)<<32 | uint64(v)<<16 | uint64(v)
				ce.pushValue(v4)
				ce.pushValue(v4)
			case wazeroir.V128LoadType32Splat:
				v, ok := memoryInst.ReadUint32Le(offset)
				if !ok {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
				vv := uint64(v)<<32 | uint64(v)
				ce.pushValue(vv)
				ce.pushValue(vv)
			case wazeroir.V128LoadType64Splat:
				lo, ok := memoryInst.ReadUint64Le(offset)
				if !ok {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
				ce.pushValue(lo)
				ce.pushValue(lo)
			case wazeroir.V128LoadType32zero:
				lo, ok := memoryInst.ReadUint32Le(offset)
				if !ok {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
				ce.pushValue(uint64(lo))
				ce.pushValue(0)
			case wazeroir.V128LoadType64zero:
				lo, ok := memoryInst.ReadUint64Le(offset)
				if !ok {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
				ce.pushValue(lo)
				ce.pushValue(0)
			}
			frame.pc++
		case wazeroir.OperationKindV128LoadLane:
			hi, lo := ce.popValue(), ce.popValue()
			offset := ce.popMemoryOffset(op)
			switch op.b1 {
			case 8:
				b, ok := memoryInst.ReadByte(offset)
				if !ok {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
				if op.b2 < 8 {
					s := op.b2 << 3
					lo = (lo & ^(0xff << s)) | uint64(b)<<s
				} else {
					s := (op.b2 - 8) << 3
					hi = (hi & ^(0xff << s)) | uint64(b)<<s
				}
			case 16:
				b, ok := memoryInst.ReadUint16Le(offset)
				if !ok {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
				if op.b2 < 4 {
					s := op.b2 << 4
					lo = (lo & ^(0xff_ff << s)) | uint64(b)<<s
				} else {
					s := (op.b2 - 4) << 4
					hi = (hi & ^(0xff_ff << s)) | uint64(b)<<s
				}
			case 32:
				b, ok := memoryInst.ReadUint32Le(offset)
				if !ok {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
				if op.b2 < 2 {
					s := op.b2 << 5
					lo = (lo & ^(0xff_ff_ff_ff << s)) | uint64(b)<<s
				} else {
					s := (op.b2 - 2) << 5
					hi = (hi & ^(0xff_ff_ff_ff << s)) | uint64(b)<<s
				}
			case 64:
				b, ok := memoryInst.ReadUint64Le(offset)
				if !ok {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
				if op.b2 == 0 {
					lo = b
				} else {
					hi = b
				}
			}
			ce.pushValue(lo)
			ce.pushValue(hi)
			frame.pc++
		case wazeroir.OperationKindV128Store:
			hi, lo := ce.popValue(), ce.popValue()
			offset := ce.popMemoryOffset(op)
			if ok := memoryInst.WriteUint64Le(offset, lo); !ok {
				panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
			}
			if ok := memoryInst.WriteUint64Le(offset+8, hi); !ok {
				panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
			}
			frame.pc++
		case wazeroir.OperationKindV128StoreLane:
			hi, lo := ce.popValue(), ce.popValue()
			offset := ce.popMemoryOffset(op)
			var ok bool
			switch op.b1 {
			case 8:
				if op.b2 < 8 {
					ok = memoryInst.WriteByte(offset, byte(lo>>(op.b2*8)))
				} else {
					ok = memoryInst.WriteByte(offset, byte(hi>>((op.b2-8)*8)))
				}
			case 16:
				if op.b2 < 4 {
					ok = memoryInst.WriteUint16Le(offset, uint16(lo>>(op.b2*16)))
				} else {
					ok = memoryInst.WriteUint16Le(offset, uint16(hi>>((op.b2-4)*16)))
				}
			case 32:
				if op.b2 < 2 {
					ok = memoryInst.WriteUint32Le(offset, uint32(lo>>(op.b2*32)))
				} else {
					ok = memoryInst.WriteUint32Le(offset, uint32(hi>>((op.b2-2)*32)))
				}
			case 64:
				if op.b2 == 0 {
					ok = memoryInst.WriteUint64Le(offset, lo)
				} else {
					ok = memoryInst.WriteUint64Le(offset, hi)
				}
			}
			if !ok {
				panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
			}
			frame.pc++
		case wazeroir.OperationKindV128ReplaceLane:
			v := ce.popValue()
			hi, lo := ce.popValue(), ce.popValue()
			switch op.b1 {
			case wazeroir.ShapeI8x16:
				if op.b2 < 8 {
					s := op.b2 << 3
					lo = (lo & ^(0xff << s)) | uint64(byte(v))<<s
				} else {
					s := (op.b2 - 8) << 3
					hi = (hi & ^(0xff << s)) | uint64(byte(v))<<s
				}
			case wazeroir.ShapeI16x8:
				if op.b2 < 4 {
					s := op.b2 << 4
					lo = (lo & ^(0xff_ff << s)) | uint64(uint16(v))<<s
				} else {
					s := (op.b2 - 4) << 4
					hi = (hi & ^(0xff_ff << s)) | uint64(uint16(v))<<s
				}
			case wazeroir.ShapeI32x4, wazeroir.ShapeF32x4:
				if op.b2 < 2 {
					s := op.b2 << 5
					lo = (lo & ^(0xff_ff_ff_ff << s)) | uint64(uint32(v))<<s
				} else {
					s := (op.b2 - 2) << 5
					hi = (hi & ^(0xff_ff_ff_ff << s)) | uint64(uint32(v))<<s
				}
			case wazeroir.ShapeI64x2, wazeroir.ShapeF64x2:
				if op.b2 == 0 {
					lo = v
				} else {
					hi = v
				}
			}
			ce.pushValue(lo)
			ce.pushValue(hi)
			frame.pc++
		case wazeroir.OperationKindV128ExtractLane:
			hi, lo := ce.popValue(), ce.popValue()
			var v uint64
			switch op.b1 {
			case wazeroir.ShapeI8x16:
				var u8 byte
				if op.b2 < 8 {
					u8 = byte(lo >> (op.b2 * 8))
				} else {
					u8 = byte(hi >> ((op.b2 - 8) * 8))
				}
				if op.b3 {
					// sign-extend.
					v = uint64(uint32(int8(u8)))
				} else {
					v = uint64(u8)
				}
			case wazeroir.ShapeI16x8:
				var u16 uint16
				if op.b2 < 4 {
					u16 = uint16(lo >> (op.b2 * 16))
				} else {
					u16 = uint16(hi >> ((op.b2 - 4) * 16))
				}
				if op.b3 {
					// sign-extend.
					v = uint64(uint32(int16(u16)))
				} else {
					v = uint64(u16)
				}
			case wazeroir.ShapeI32x4, wazeroir.ShapeF32x4:
				if op.b2 < 2 {
					v = uint64(uint32(lo >> (op.b2 * 32)))
				} else {
					v = uint64(uint32(hi >> ((op.b2 - 2) * 32)))
				}
			case wazeroir.ShapeI64x2, wazeroir.ShapeF64x2:
				if op.b2 == 0 {
					v = lo
				} else {
					v = hi
				}
			}
			ce.pushValue(v)
			frame.pc++
		case wazeroir.OperationKindV128Splat:
			v := ce.popValue()
			var hi, lo uint64
			switch op.b1 {
			case wazeroir.ShapeI8x16:
				v8 := uint64(byte(v))<<56 | uint64(byte(v))<<48 | uint64(byte(v))<<40 | uint64(byte(v))<<32 |
					uint64(byte(v))<<24 | uint64(byte(v))<<16 | uint64(byte(v))<<8 | uint64(byte(v))
				hi, lo = v8, v8
			case wazeroir.ShapeI16x8:
				v4 := uint64(uint16(v))<<48 | uint64(uint16(v))<<32 | uint64(uint16(v))<<16 | uint64(uint16(v))
				hi, lo = v4, v4
			case wazeroir.ShapeI32x4, wazeroir.ShapeF32x4:
				v2 := uint64(uint32(v))<<32 | uint64(uint32(v))
				lo, hi = v2, v2
			case wazeroir.ShapeI64x2, wazeroir.ShapeF64x2:
				lo, hi = v, v
			}
			ce.pushValue(lo)
			ce.pushValue(hi)
			frame.pc++
		case wazeroir.OperationKindV128Swizzle:
			idxHi, idxLo := ce.popValue(), ce.popValue()
			baseHi, baseLo := ce.popValue(), ce.popValue()
			var newVal [16]byte
			for i := 0; i < 16; i++ {
				var id byte
				if i < 8 {
					id = byte(idxLo >> (i * 8))
				} else {
					id = byte(idxHi >> ((i - 8) * 8))
				}
				if id < 8 {
					newVal[i] = byte(baseLo >> (id * 8))
				} else if id < 16 {
					newVal[i] = byte(baseHi >> ((id - 8) * 8))
				}
			}
			ce.pushValue(binary.LittleEndian.Uint64(newVal[:8]))
			ce.pushValue(binary.LittleEndian.Uint64(newVal[8:]))
			frame.pc++
		case wazeroir.OperationKindV128Shuffle:
			xHi, xLo, yHi, yLo := ce.popValue(), ce.popValue(), ce.popValue(), ce.popValue()
			var newVal [16]byte
			for i, l := range op.us {
				if l < 8 {
					newVal[i] = byte(yLo >> (l * 8))
				} else if l < 16 {
					newVal[i] = byte(yHi >> ((l - 8) * 8))
				} else if l < 24 {
					newVal[i] = byte(xLo >> ((l - 16) * 8))
				} else if l < 32 {
					newVal[i] = byte(xHi >> ((l - 24) * 8))
				}
			}
			ce.pushValue(binary.LittleEndian.Uint64(newVal[:8]))
			ce.pushValue(binary.LittleEndian.Uint64(newVal[8:]))
			frame.pc++
		case wazeroir.OperationKindV128AnyTrue:
			hi, lo := ce.popValue(), ce.popValue()
			if hi != 0 || lo != 0 {
				ce.pushValue(1)
			} else {
				ce.pushValue(0)
			}
			frame.pc++
		case wazeroir.OperationKindV128AllTrue:
			hi, lo := ce.popValue(), ce.popValue()
			var ret bool
			switch op.b1 {
			case wazeroir.ShapeI8x16:
				ret = (uint8(lo) != 0) && (uint8(lo>>8) != 0) && (uint8(lo>>16) != 0) && (uint8(lo>>24) != 0) &&
					(uint8(lo>>32) != 0) && (uint8(lo>>40) != 0) && (uint8(lo>>48) != 0) && (uint8(lo>>56) != 0) &&
					(uint8(hi) != 0) && (uint8(hi>>8) != 0) && (uint8(hi>>16) != 0) && (uint8(hi>>24) != 0) &&
					(uint8(hi>>32) != 0) && (uint8(hi>>40) != 0) && (uint8(hi>>48) != 0) && (uint8(hi>>56) != 0)
			case wazeroir.ShapeI16x8:
				ret = (uint16(lo) != 0) && (uint16(lo>>16) != 0) && (uint16(lo>>32) != 0) && (uint16(lo>>48) != 0) &&
					(uint16(hi) != 0) && (uint16(hi>>16) != 0) && (uint16(hi>>32) != 0) && (uint16(hi>>48) != 0)
			case wazeroir.ShapeI32x4:
				ret = (uint32(lo) != 0) && (uint32(lo>>32) != 0) &&
					(uint32(hi) != 0) && (uint32(hi>>32) != 0)
			case wazeroir.ShapeI64x2:
				ret = (lo != 0) &&
					(hi != 0)
			}
			if ret {
				ce.pushValue(1)
			} else {
				ce.pushValue(0)
			}
			frame.pc++
		case wazeroir.OperationKindV128BitMask:
			// https://github.com/WebAssembly/spec/blob/wg-2.0.draft1/proposals/simd/SIMD.md#bitmask-extraction
			hi, lo := ce.popValue(), ce.popValue()
			var res uint64
			switch op.b1 {
			case wazeroir.ShapeI8x16:
				for i := 0; i < 8; i++ {
					if int8(lo>>(i*8)) < 0 {
						res |= 1 << i
					}
				}
				for i := 0; i < 8; i++ {
					if int8(hi>>(i*8)) < 0 {
						res |= 1 << (i + 8)
					}
				}
			case wazeroir.ShapeI16x8:
				for i := 0; i < 4; i++ {
					if int16(lo>>(i*16)) < 0 {
						res |= 1 << i
					}
				}
				for i := 0; i < 4; i++ {
					if int16(hi>>(i*16)) < 0 {
						res |= 1 << (i + 4)
					}
				}
			case wazeroir.ShapeI32x4:
				for i := 0; i < 2; i++ {
					if int32(lo>>(i*32)) < 0 {
						res |= 1 << i
					}
				}
				for i := 0; i < 2; i++ {
					if int32(hi>>(i*32)) < 0 {
						res |= 1 << (i + 2)
					}
				}
			case wazeroir.ShapeI64x2:
				if int64(lo) < 0 {
					res |= 0b01
				}
				if int(hi) < 0 {
					res |= 0b10
				}
			}
			ce.pushValue(res)
			frame.pc++
		case wazeroir.OperationKindV128And:
			x2Hi, x2Lo := ce.popValue(), ce.popValue()
			x1Hi, x1Lo := ce.popValue(), ce.popValue()
			ce.pushValue(x1Lo & x2Lo)
			ce.pushValue(x1Hi & x2Hi)
			frame.pc++
		case wazeroir.OperationKindV128Not:
			hi, lo := ce.popValue(), ce.popValue()
			ce.pushValue(^lo)
			ce.pushValue(^hi)
			frame.pc++
		case wazeroir.OperationKindV128Or:
			x2Hi, x2Lo := ce.popValue(), ce.popValue()
			x1Hi, x1Lo := ce.popValue(), ce.popValue()
			ce.pushValue(x1Lo | x2Lo)
			ce.pushValue(x1Hi | x2Hi)
			frame.pc++
		case wazeroir.OperationKindV128Xor:
			x2Hi, x2Lo := ce.popValue(), ce.popValue()
			x1Hi, x1Lo := ce.popValue(), ce.popValue()
			ce.pushValue(x1Lo ^ x2Lo)
			ce.pushValue(x1Hi ^ x2Hi)
			frame.pc++
		case wazeroir.OperationKindV128Bitselect:
			// https://github.com/WebAssembly/spec/blob/wg-2.0.draft1/proposals/simd/SIMD.md#bitwise-select
			cHi, cLo := ce.popValue(), ce.popValue()
			x2Hi, x2Lo := ce.popValue(), ce.popValue()
			x1Hi, x1Lo := ce.popValue(), ce.popValue()
			// v128.or(v128.and(v1, c), v128.and(v2, v128.not(c)))
			ce.pushValue((x1Lo & cLo) | (x2Lo & (^cLo)))
			ce.pushValue((x1Hi & cHi) | (x2Hi & (^cHi)))
			frame.pc++
		case wazeroir.OperationKindV128AndNot:
			x2Hi, x2Lo := ce.popValue(), ce.popValue()
			x1Hi, x1Lo := ce.popValue(), ce.popValue()
			ce.pushValue(x1Lo & (^x2Lo))
			ce.pushValue(x1Hi & (^x2Hi))
			frame.pc++
		case wazeroir.OperationKindV128Shl:
			s := ce.popValue()
			hi, lo := ce.popValue(), ce.popValue()
			switch op.b1 {
			case wazeroir.ShapeI8x16:
				s = s % 8
				lo = uint64(uint8(lo<<s)) |
					uint64(uint8((lo>>8)<<s))<<8 |
					uint64(uint8((lo>>16)<<s))<<16 |
					uint64(uint8((lo>>24)<<s))<<24 |
					uint64(uint8((lo>>32)<<s))<<32 |
					uint64(uint8((lo>>40)<<s))<<40 |
					uint64(uint8((lo>>48)<<s))<<48 |
					uint64(uint8((lo>>56)<<s))<<56
				hi = uint64(uint8(hi<<s)) |
					uint64(uint8((hi>>8)<<s))<<8 |
					uint64(uint8((hi>>16)<<s))<<16 |
					uint64(uint8((hi>>24)<<s))<<24 |
					uint64(uint8((hi>>32)<<s))<<32 |
					uint64(uint8((hi>>40)<<s))<<40 |
					uint64(uint8((hi>>48)<<s))<<48 |
					uint64(uint8((hi>>56)<<s))<<56
			case wazeroir.ShapeI16x8:
				s = s % 16
				lo = uint64(uint16(lo<<s)) |
					uint64(uint16((lo>>16)<<s))<<16 |
					uint64(uint16((lo>>32)<<s))<<32 |
					uint64(uint16((lo>>48)<<s))<<48
				hi = uint64(uint16(hi<<s)) |
					uint64(uint16((hi>>16)<<s))<<16 |
					uint64(uint16((hi>>32)<<s))<<32 |
					uint64(uint16((hi>>48)<<s))<<48
			case wazeroir.ShapeI32x4:
				s = s % 32
				lo = uint64(uint32(lo<<s)) | uint64(uint32((lo>>32)<<s))<<32
				hi = uint64(uint32(hi<<s)) | uint64(uint32((hi>>32)<<s))<<32
			case wazeroir.ShapeI64x2:
				s = s % 64
				lo = lo << s
				hi = hi << s
			}
			ce.pushValue(lo)
			ce.pushValue(hi)
			frame.pc++
		case wazeroir.OperationKindV128Shr:
			s := ce.popValue()
			hi, lo := ce.popValue(), ce.popValue()
			switch op.b1 {
			case wazeroir.ShapeI8x16:
				s = s % 8
				if op.b3 { // signed
					lo = uint64(uint8(int8(lo)>>s)) |
						uint64(uint8(int8(lo>>8)>>s))<<8 |
						uint64(uint8(int8(lo>>16)>>s))<<16 |
						uint64(uint8(int8(lo>>24)>>s))<<24 |
						uint64(uint8(int8(lo>>32)>>s))<<32 |
						uint64(uint8(int8(lo>>40)>>s))<<40 |
						uint64(uint8(int8(lo>>48)>>s))<<48 |
						uint64(uint8(int8(lo>>56)>>s))<<56
					hi = uint64(uint8(int8(hi)>>s)) |
						uint64(uint8(int8(hi>>8)>>s))<<8 |
						uint64(uint8(int8(hi>>16)>>s))<<16 |
						uint64(uint8(int8(hi>>24)>>s))<<24 |
						uint64(uint8(int8(hi>>32)>>s))<<32 |
						uint64(uint8(int8(hi>>40)>>s))<<40 |
						uint64(uint8(int8(hi>>48)>>s))<<48 |
						uint64(uint8(int8(hi>>56)>>s))<<56
				} else {
					lo = uint64(uint8(lo)>>s) |
						uint64(uint8(lo>>8)>>s)<<8 |
						uint64(uint8(lo>>16)>>s)<<16 |
						uint64(uint8(lo>>24)>>s)<<24 |
						uint64(uint8(lo>>32)>>s)<<32 |
						uint64(uint8(lo>>40)>>s)<<40 |
						uint64(uint8(lo>>48)>>s)<<48 |
						uint64(uint8(lo>>56)>>s)<<56
					hi = uint64(uint8(hi)>>s) |
						uint64(uint8(hi>>8)>>s)<<8 |
						uint64(uint8(hi>>16)>>s)<<16 |
						uint64(uint8(hi>>24)>>s)<<24 |
						uint64(uint8(hi>>32)>>s)<<32 |
						uint64(uint8(hi>>40)>>s)<<40 |
						uint64(uint8(hi>>48)>>s)<<48 |
						uint64(uint8(hi>>56)>>s)<<56
				}
			case wazeroir.ShapeI16x8:
				s = s % 16
				if op.b3 { // signed
					lo = uint64(uint16(int16(lo)>>s)) |
						uint64(uint16(int16(lo>>16)>>s))<<16 |
						uint64(uint16(int16(lo>>32)>>s))<<32 |
						uint64(uint16(int16(lo>>48)>>s))<<48
					hi = uint64(uint16(int16(hi)>>s)) |
						uint64(uint16(int16(hi>>16)>>s))<<16 |
						uint64(uint16(int16(hi>>32)>>s))<<32 |
						uint64(uint16(int16(hi>>48)>>s))<<48
				} else {
					lo = uint64(uint16(lo)>>s) |
						uint64(uint16(lo>>16)>>s)<<16 |
						uint64(uint16(lo>>32)>>s)<<32 |
						uint64(uint16(lo>>48)>>s)<<48
					hi = uint64(uint16(hi)>>s) |
						uint64(uint16(hi>>16)>>s)<<16 |
						uint64(uint16(hi>>32)>>s)<<32 |
						uint64(uint16(hi>>48)>>s)<<48
				}
			case wazeroir.ShapeI32x4:
				s = s % 32
				if op.b3 {
					lo = uint64(uint32(int32(lo)>>s)) | uint64(uint32(int32(lo>>32)>>s))<<32
					hi = uint64(uint32(int32(hi)>>s)) | uint64(uint32(int32(hi>>32)>>s))<<32
				} else {
					lo = uint64(uint32(lo)>>s) | uint64(uint32(lo>>32)>>s)<<32
					hi = uint64(uint32(hi)>>s) | uint64(uint32(hi>>32)>>s)<<32
				}
			case wazeroir.ShapeI64x2:
				s = s % 64
				if op.b3 { // signed
					lo = uint64(int64(lo) >> s)
					hi = uint64(int64(hi) >> s)
				} else {
					lo = lo >> s
					hi = hi >> s
				}

			}
			ce.pushValue(lo)
			ce.pushValue(hi)
			frame.pc++
		case wazeroir.OperationKindV128Cmp:
			x2Hi, x2Lo := ce.popValue(), ce.popValue()
			x1Hi, x1Lo := ce.popValue(), ce.popValue()
			var result []bool
			switch op.b1 {
			case wazeroir.V128CmpTypeI8x16Eq:
				result = []bool{
					byte(x1Lo>>0) == byte(x2Lo>>0), byte(x1Lo>>8) == byte(x2Lo>>8),
					byte(x1Lo>>16) == byte(x2Lo>>16), byte(x1Lo>>24) == byte(x2Lo>>24),
					byte(x1Lo>>32) == byte(x2Lo>>32), byte(x1Lo>>40) == byte(x2Lo>>40),
					byte(x1Lo>>48) == byte(x2Lo>>48), byte(x1Lo>>56) == byte(x2Lo>>56),
					byte(x1Hi>>0) == byte(x2Hi>>0), byte(x1Hi>>8) == byte(x2Hi>>8),
					byte(x1Hi>>16) == byte(x2Hi>>16), byte(x1Hi>>24) == byte(x2Hi>>24),
					byte(x1Hi>>32) == byte(x2Hi>>32), byte(x1Hi>>40) == byte(x2Hi>>40),
					byte(x1Hi>>48) == byte(x2Hi>>48), byte(x1Hi>>56) == byte(x2Hi>>56),
				}
			case wazeroir.V128CmpTypeI8x16Ne:
				result = []bool{
					byte(x1Lo>>0) != byte(x2Lo>>0), byte(x1Lo>>8) != byte(x2Lo>>8),
					byte(x1Lo>>16) != byte(x2Lo>>16), byte(x1Lo>>24) != byte(x2Lo>>24),
					byte(x1Lo>>32) != byte(x2Lo>>32), byte(x1Lo>>40) != byte(x2Lo>>40),
					byte(x1Lo>>48) != byte(x2Lo>>48), byte(x1Lo>>56) != byte(x2Lo>>56),
					byte(x1Hi>>0) != byte(x2Hi>>0), byte(x1Hi>>8) != byte(x2Hi>>8),
					byte(x1Hi>>16) != byte(x2Hi>>16), byte(x1Hi>>24) != byte(x2Hi>>24),
					byte(x1Hi>>32) != byte(x2Hi>>32), byte(x1Hi>>40) != byte(x2Hi>>40),
					byte(x1Hi>>48) != byte(x2Hi>>48), byte(x1Hi>>56) != byte(x2Hi>>56),
				}
			case wazeroir.V128CmpTypeI8x16LtS:
				result = []bool{
					int8(x1Lo>>0) < int8(x2Lo>>0), int8(x1Lo>>8) < int8(x2Lo>>8),
					int8(x1Lo>>16) < int8(x2Lo>>16), int8(x1Lo>>24) < int8(x2Lo>>24),
					int8(x1Lo>>32) < int8(x2Lo>>32), int8(x1Lo>>40) < int8(x2Lo>>40),
					int8(x1Lo>>48) < int8(x2Lo>>48), int8(x1Lo>>56) < int8(x2Lo>>56),
					int8(x1Hi>>0) < int8(x2Hi>>0), int8(x1Hi>>8) < int8(x2Hi>>8),
					int8(x1Hi>>16) < int8(x2Hi>>16), int8(x1Hi>>24) < int8(x2Hi>>24),
					int8(x1Hi>>32) < int8(x2Hi>>32), int8(x1Hi>>40) < int8(x2Hi>>40),
					int8(x1Hi>>48) < int8(x2Hi>>48), int8(x1Hi>>56) < int8(x2Hi>>56),
				}
			case wazeroir.V128CmpTypeI8x16LtU:
				result = []bool{
					byte(x1Lo>>0) < byte(x2Lo>>0), byte(x1Lo>>8) < byte(x2Lo>>8),
					byte(x1Lo>>16) < byte(x2Lo>>16), byte(x1Lo>>24) < byte(x2Lo>>24),
					byte(x1Lo>>32) < byte(x2Lo>>32), byte(x1Lo>>40) < byte(x2Lo>>40),
					byte(x1Lo>>48) < byte(x2Lo>>48), byte(x1Lo>>56) < byte(x2Lo>>56),
					byte(x1Hi>>0) < byte(x2Hi>>0), byte(x1Hi>>8) < byte(x2Hi>>8),
					byte(x1Hi>>16) < byte(x2Hi>>16), byte(x1Hi>>24) < byte(x2Hi>>24),
					byte(x1Hi>>32) < byte(x2Hi>>32), byte(x1Hi>>40) < byte(x2Hi>>40),
					byte(x1Hi>>48) < byte(x2Hi>>48), byte(x1Hi>>56) < byte(x2Hi>>56),
				}
			case wazeroir.V128CmpTypeI8x16GtS:
				result = []bool{
					int8(x1Lo>>0) > int8(x2Lo>>0), int8(x1Lo>>8) > int8(x2Lo>>8),
					int8(x1Lo>>16) > int8(x2Lo>>16), int8(x1Lo>>24) > int8(x2Lo>>24),
					int8(x1Lo>>32) > int8(x2Lo>>32), int8(x1Lo>>40) > int8(x2Lo>>40),
					int8(x1Lo>>48) > int8(x2Lo>>48), int8(x1Lo>>56) > int8(x2Lo>>56),
					int8(x1Hi>>0) > int8(x2Hi>>0), int8(x1Hi>>8) > int8(x2Hi>>8),
					int8(x1Hi>>16) > int8(x2Hi>>16), int8(x1Hi>>24) > int8(x2Hi>>24),
					int8(x1Hi>>32) > int8(x2Hi>>32), int8(x1Hi>>40) > int8(x2Hi>>40),
					int8(x1Hi>>48) > int8(x2Hi>>48), int8(x1Hi>>56) > int8(x2Hi>>56),
				}
			case wazeroir.V128CmpTypeI8x16GtU:
				result = []bool{
					byte(x1Lo>>0) > byte(x2Lo>>0), byte(x1Lo>>8) > byte(x2Lo>>8),
					byte(x1Lo>>16) > byte(x2Lo>>16), byte(x1Lo>>24) > byte(x2Lo>>24),
					byte(x1Lo>>32) > byte(x2Lo>>32), byte(x1Lo>>40) > byte(x2Lo>>40),
					byte(x1Lo>>48) > byte(x2Lo>>48), byte(x1Lo>>56) > byte(x2Lo>>56),
					byte(x1Hi>>0) > byte(x2Hi>>0), byte(x1Hi>>8) > byte(x2Hi>>8),
					byte(x1Hi>>16) > byte(x2Hi>>16), byte(x1Hi>>24) > byte(x2Hi>>24),
					byte(x1Hi>>32) > byte(x2Hi>>32), byte(x1Hi>>40) > byte(x2Hi>>40),
					byte(x1Hi>>48) > byte(x2Hi>>48), byte(x1Hi>>56) > byte(x2Hi>>56),
				}
			case wazeroir.V128CmpTypeI8x16LeS:
				result = []bool{
					int8(x1Lo>>0) <= int8(x2Lo>>0), int8(x1Lo>>8) <= int8(x2Lo>>8),
					int8(x1Lo>>16) <= int8(x2Lo>>16), int8(x1Lo>>24) <= int8(x2Lo>>24),
					int8(x1Lo>>32) <= int8(x2Lo>>32), int8(x1Lo>>40) <= int8(x2Lo>>40),
					int8(x1Lo>>48) <= int8(x2Lo>>48), int8(x1Lo>>56) <= int8(x2Lo>>56),
					int8(x1Hi>>0) <= int8(x2Hi>>0), int8(x1Hi>>8) <= int8(x2Hi>>8),
					int8(x1Hi>>16) <= int8(x2Hi>>16), int8(x1Hi>>24) <= int8(x2Hi>>24),
					int8(x1Hi>>32) <= int8(x2Hi>>32), int8(x1Hi>>40) <= int8(x2Hi>>40),
					int8(x1Hi>>48) <= int8(x2Hi>>48), int8(x1Hi>>56) <= int8(x2Hi>>56),
				}
			case wazeroir.V128CmpTypeI8x16LeU:
				result = []bool{
					byte(x1Lo>>0) <= byte(x2Lo>>0), byte(x1Lo>>8) <= byte(x2Lo>>8),
					byte(x1Lo>>16) <= byte(x2Lo>>16), byte(x1Lo>>24) <= byte(x2Lo>>24),
					byte(x1Lo>>32) <= byte(x2Lo>>32), byte(x1Lo>>40) <= byte(x2Lo>>40),
					byte(x1Lo>>48) <= byte(x2Lo>>48), byte(x1Lo>>56) <= byte(x2Lo>>56),
					byte(x1Hi>>0) <= byte(x2Hi>>0), byte(x1Hi>>8) <= byte(x2Hi>>8),
					byte(x1Hi>>16) <= byte(x2Hi>>16), byte(x1Hi>>24) <= byte(x2Hi>>24),
					byte(x1Hi>>32) <= byte(x2Hi>>32), byte(x1Hi>>40) <= byte(x2Hi>>40),
					byte(x1Hi>>48) <= byte(x2Hi>>48), byte(x1Hi>>56) <= byte(x2Hi>>56),
				}
			case wazeroir.V128CmpTypeI8x16GeS:
				result = []bool{
					int8(x1Lo>>0) >= int8(x2Lo>>0), int8(x1Lo>>8) >= int8(x2Lo>>8),
					int8(x1Lo>>16) >= int8(x2Lo>>16), int8(x1Lo>>24) >= int8(x2Lo>>24),
					int8(x1Lo>>32) >= int8(x2Lo>>32), int8(x1Lo>>40) >= int8(x2Lo>>40),
					int8(x1Lo>>48) >= int8(x2Lo>>48), int8(x1Lo>>56) >= int8(x2Lo>>56),
					int8(x1Hi>>0) >= int8(x2Hi>>0), int8(x1Hi>>8) >= int8(x2Hi>>8),
					int8(x1Hi>>16) >= int8(x2Hi>>16), int8(x1Hi>>24) >= int8(x2Hi>>24),
					int8(x1Hi>>32) >= int8(x2Hi>>32), int8(x1Hi>>40) >= int8(x2Hi>>40),
					int8(x1Hi>>48) >= int8(x2Hi>>48), int8(x1Hi>>56) >= int8(x2Hi>>56),
				}
			case wazeroir.V128CmpTypeI8x16GeU:
				result = []bool{
					byte(x1Lo>>0) >= byte(x2Lo>>0), byte(x1Lo>>8) >= byte(x2Lo>>8),
					byte(x1Lo>>16) >= byte(x2Lo>>16), byte(x1Lo>>24) >= byte(x2Lo>>24),
					byte(x1Lo>>32) >= byte(x2Lo>>32), byte(x1Lo>>40) >= byte(x2Lo>>40),
					byte(x1Lo>>48) >= byte(x2Lo>>48), byte(x1Lo>>56) >= byte(x2Lo>>56),
					byte(x1Hi>>0) >= byte(x2Hi>>0), byte(x1Hi>>8) >= byte(x2Hi>>8),
					byte(x1Hi>>16) >= byte(x2Hi>>16), byte(x1Hi>>24) >= byte(x2Hi>>24),
					byte(x1Hi>>32) >= byte(x2Hi>>32), byte(x1Hi>>40) >= byte(x2Hi>>40),
					byte(x1Hi>>48) >= byte(x2Hi>>48), byte(x1Hi>>56) >= byte(x2Hi>>56),
				}
			case wazeroir.V128CmpTypeI16x8Eq:
				result = []bool{
					uint16(x1Lo>>0) == uint16(x2Lo>>0), uint16(x1Lo>>16) == uint16(x2Lo>>16),
					uint16(x1Lo>>32) == uint16(x2Lo>>32), uint16(x1Lo>>48) == uint16(x2Lo>>48),
					uint16(x1Hi>>0) == uint16(x2Hi>>0), uint16(x1Hi>>16) == uint16(x2Hi>>16),
					uint16(x1Hi>>32) == uint16(x2Hi>>32), uint16(x1Hi>>48) == uint16(x2Hi>>48),
				}
			case wazeroir.V128CmpTypeI16x8Ne:
				result = []bool{
					uint16(x1Lo>>0) != uint16(x2Lo>>0), uint16(x1Lo>>16) != uint16(x2Lo>>16),
					uint16(x1Lo>>32) != uint16(x2Lo>>32), uint16(x1Lo>>48) != uint16(x2Lo>>48),
					uint16(x1Hi>>0) != uint16(x2Hi>>0), uint16(x1Hi>>16) != uint16(x2Hi>>16),
					uint16(x1Hi>>32) != uint16(x2Hi>>32), uint16(x1Hi>>48) != uint16(x2Hi>>48),
				}
			case wazeroir.V128CmpTypeI16x8LtS:
				result = []bool{
					int16(x1Lo>>0) < int16(x2Lo>>0), int16(x1Lo>>16) < int16(x2Lo>>16),
					int16(x1Lo>>32) < int16(x2Lo>>32), int16(x1Lo>>48) < int16(x2Lo>>48),
					int16(x1Hi>>0) < int16(x2Hi>>0), int16(x1Hi>>16) < int16(x2Hi>>16),
					int16(x1Hi>>32) < int16(x2Hi>>32), int16(x1Hi>>48) < int16(x2Hi>>48),
				}
			case wazeroir.V128CmpTypeI16x8LtU:
				result = []bool{
					uint16(x1Lo>>0) < uint16(x2Lo>>0), uint16(x1Lo>>16) < uint16(x2Lo>>16),
					uint16(x1Lo>>32) < uint16(x2Lo>>32), uint16(x1Lo>>48) < uint16(x2Lo>>48),
					uint16(x1Hi>>0) < uint16(x2Hi>>0), uint16(x1Hi>>16) < uint16(x2Hi>>16),
					uint16(x1Hi>>32) < uint16(x2Hi>>32), uint16(x1Hi>>48) < uint16(x2Hi>>48),
				}
			case wazeroir.V128CmpTypeI16x8GtS:
				result = []bool{
					int16(x1Lo>>0) > int16(x2Lo>>0), int16(x1Lo>>16) > int16(x2Lo>>16),
					int16(x1Lo>>32) > int16(x2Lo>>32), int16(x1Lo>>48) > int16(x2Lo>>48),
					int16(x1Hi>>0) > int16(x2Hi>>0), int16(x1Hi>>16) > int16(x2Hi>>16),
					int16(x1Hi>>32) > int16(x2Hi>>32), int16(x1Hi>>48) > int16(x2Hi>>48),
				}
			case wazeroir.V128CmpTypeI16x8GtU:
				result = []bool{
					uint16(x1Lo>>0) > uint16(x2Lo>>0), uint16(x1Lo>>16) > uint16(x2Lo>>16),
					uint16(x1Lo>>32) > uint16(x2Lo>>32), uint16(x1Lo>>48) > uint16(x2Lo>>48),
					uint16(x1Hi>>0) > uint16(x2Hi>>0), uint16(x1Hi>>16) > uint16(x2Hi>>16),
					uint16(x1Hi>>32) > uint16(x2Hi>>32), uint16(x1Hi>>48) > uint16(x2Hi>>48),
				}
			case wazeroir.V128CmpTypeI16x8LeS:
				result = []bool{
					int16(x1Lo>>0) <= int16(x2Lo>>0), int16(x1Lo>>16) <= int16(x2Lo>>16),
					int16(x1Lo>>32) <= int16(x2Lo>>32), int16(x1Lo>>48) <= int16(x2Lo>>48),
					int16(x1Hi>>0) <= int16(x2Hi>>0), int16(x1Hi>>16) <= int16(x2Hi>>16),
					int16(x1Hi>>32) <= int16(x2Hi>>32), int16(x1Hi>>48) <= int16(x2Hi>>48),
				}
			case wazeroir.V128CmpTypeI16x8LeU:
				result = []bool{
					uint16(x1Lo>>0) <= uint16(x2Lo>>0), uint16(x1Lo>>16) <= uint16(x2Lo>>16),
					uint16(x1Lo>>32) <= uint16(x2Lo>>32), uint16(x1Lo>>48) <= uint16(x2Lo>>48),
					uint16(x1Hi>>0) <= uint16(x2Hi>>0), uint16(x1Hi>>16) <= uint16(x2Hi>>16),
					uint16(x1Hi>>32) <= uint16(x2Hi>>32), uint16(x1Hi>>48) <= uint16(x2Hi>>48),
				}
			case wazeroir.V128CmpTypeI16x8GeS:
				result = []bool{
					int16(x1Lo>>0) >= int16(x2Lo>>0), int16(x1Lo>>16) >= int16(x2Lo>>16),
					int16(x1Lo>>32) >= int16(x2Lo>>32), int16(x1Lo>>48) >= int16(x2Lo>>48),
					int16(x1Hi>>0) >= int16(x2Hi>>0), int16(x1Hi>>16) >= int16(x2Hi>>16),
					int16(x1Hi>>32) >= int16(x2Hi>>32), int16(x1Hi>>48) >= int16(x2Hi>>48),
				}
			case wazeroir.V128CmpTypeI16x8GeU:
				result = []bool{
					uint16(x1Lo>>0) >= uint16(x2Lo>>0), uint16(x1Lo>>16) >= uint16(x2Lo>>16),
					uint16(x1Lo>>32) >= uint16(x2Lo>>32), uint16(x1Lo>>48) >= uint16(x2Lo>>48),
					uint16(x1Hi>>0) >= uint16(x2Hi>>0), uint16(x1Hi>>16) >= uint16(x2Hi>>16),
					uint16(x1Hi>>32) >= uint16(x2Hi>>32), uint16(x1Hi>>48) >= uint16(x2Hi>>48),
				}
			case wazeroir.V128CmpTypeI32x4Eq:
				result = []bool{
					uint32(x1Lo>>0) == uint32(x2Lo>>0), uint32(x1Lo>>32) == uint32(x2Lo>>32),
					uint32(x1Hi>>0) == uint32(x2Hi>>0), uint32(x1Hi>>32) == uint32(x2Hi>>32),
				}
			case wazeroir.V128CmpTypeI32x4Ne:
				result = []bool{
					uint32(x1Lo>>0) != uint32(x2Lo>>0), uint32(x1Lo>>32) != uint32(x2Lo>>32),
					uint32(x1Hi>>0) != uint32(x2Hi>>0), uint32(x1Hi>>32) != uint32(x2Hi>>32),
				}
			case wazeroir.V128CmpTypeI32x4LtS:
				result = []bool{
					int32(x1Lo>>0) < int32(x2Lo>>0), int32(x1Lo>>32) < int32(x2Lo>>32),
					int32(x1Hi>>0) < int32(x2Hi>>0), int32(x1Hi>>32) < int32(x2Hi>>32),
				}
			case wazeroir.V128CmpTypeI32x4LtU:
				result = []bool{
					uint32(x1Lo>>0) < uint32(x2Lo>>0), uint32(x1Lo>>32) < uint32(x2Lo>>32),
					uint32(x1Hi>>0) < uint32(x2Hi>>0), uint32(x1Hi>>32) < uint32(x2Hi>>32),
				}
			case wazeroir.V128CmpTypeI32x4GtS:
				result = []bool{
					int32(x1Lo>>0) > int32(x2Lo>>0), int32(x1Lo>>32) > int32(x2Lo>>32),
					int32(x1Hi>>0) > int32(x2Hi>>0), int32(x1Hi>>32) > int32(x2Hi>>32),
				}
			case wazeroir.V128CmpTypeI32x4GtU:
				result = []bool{
					uint32(x1Lo>>0) > uint32(x2Lo>>0), uint32(x1Lo>>32) > uint32(x2Lo>>32),
					uint32(x1Hi>>0) > uint32(x2Hi>>0), uint32(x1Hi>>32) > uint32(x2Hi>>32),
				}
			case wazeroir.V128CmpTypeI32x4LeS:
				result = []bool{
					int32(x1Lo>>0) <= int32(x2Lo>>0), int32(x1Lo>>32) <= int32(x2Lo>>32),
					int32(x1Hi>>0) <= int32(x2Hi>>0), int32(x1Hi>>32) <= int32(x2Hi>>32),
				}
			case wazeroir.V128CmpTypeI32x4LeU:
				result = []bool{
					uint32(x1Lo>>0) <= uint32(x2Lo>>0), uint32(x1Lo>>32) <= uint32(x2Lo>>32),
					uint32(x1Hi>>0) <= uint32(x2Hi>>0), uint32(x1Hi>>32) <= uint32(x2Hi>>32),
				}
			case wazeroir.V128CmpTypeI32x4GeS:
				result = []bool{
					int32(x1Lo>>0) >= int32(x2Lo>>0), int32(x1Lo>>32) >= int32(x2Lo>>32),
					int32(x1Hi>>0) >= int32(x2Hi>>0), int32(x1Hi>>32) >= int32(x2Hi>>32),
				}
			case wazeroir.V128CmpTypeI32x4GeU:
				result = []bool{
					uint32(x1Lo>>0) >= uint32(x2Lo>>0), uint32(x1Lo>>32) >= uint32(x2Lo>>32),
					uint32(x1Hi>>0) >= uint32(x2Hi>>0), uint32(x1Hi>>32) >= uint32(x2Hi>>32),
				}
			case wazeroir.V128CmpTypeI64x2Eq:
				result = []bool{x1Lo == x2Lo, x1Hi == x2Hi}
			case wazeroir.V128CmpTypeI64x2Ne:
				result = []bool{x1Lo != x2Lo, x1Hi != x2Hi}
			case wazeroir.V128CmpTypeI64x2LtS:
				result = []bool{int64(x1Lo) < int64(x2Lo), int64(x1Hi) < int64(x2Hi)}
			case wazeroir.V128CmpTypeI64x2GtS:
				result = []bool{int64(x1Lo) > int64(x2Lo), int64(x1Hi) > int64(x2Hi)}
			case wazeroir.V128CmpTypeI64x2LeS:
				result = []bool{int64(x1Lo) <= int64(x2Lo), int64(x1Hi) <= int64(x2Hi)}
			case wazeroir.V128CmpTypeI64x2GeS:
				result = []bool{int64(x1Lo) >= int64(x2Lo), int64(x1Hi) >= int64(x2Hi)}
			case wazeroir.V128CmpTypeF32x4Eq:
				result = []bool{
					math.Float32frombits(uint32(x1Lo>>0)) == math.Float32frombits(uint32(x2Lo>>0)),
					math.Float32frombits(uint32(x1Lo>>32)) == math.Float32frombits(uint32(x2Lo>>32)),
					math.Float32frombits(uint32(x1Hi>>0)) == math.Float32frombits(uint32(x2Hi>>0)),
					math.Float32frombits(uint32(x1Hi>>32)) == math.Float32frombits(uint32(x2Hi>>32)),
				}
			case wazeroir.V128CmpTypeF32x4Ne:
				result = []bool{
					math.Float32frombits(uint32(x1Lo>>0)) != math.Float32frombits(uint32(x2Lo>>0)),
					math.Float32frombits(uint32(x1Lo>>32)) != math.Float32frombits(uint32(x2Lo>>32)),
					math.Float32frombits(uint32(x1Hi>>0)) != math.Float32frombits(uint32(x2Hi>>0)),
					math.Float32frombits(uint32(x1Hi>>32)) != math.Float32frombits(uint32(x2Hi>>32)),
				}
			case wazeroir.V128CmpTypeF32x4Lt:
				result = []bool{
					math.Float32frombits(uint32(x1Lo>>0)) < math.Float32frombits(uint32(x2Lo>>0)),
					math.Float32frombits(uint32(x1Lo>>32)) < math.Float32frombits(uint32(x2Lo>>32)),
					math.Float32frombits(uint32(x1Hi>>0)) < math.Float32frombits(uint32(x2Hi>>0)),
					math.Float32frombits(uint32(x1Hi>>32)) < math.Float32frombits(uint32(x2Hi>>32)),
				}
			case wazeroir.V128CmpTypeF32x4Gt:
				result = []bool{
					math.Float32frombits(uint32(x1Lo>>0)) > math.Float32frombits(uint32(x2Lo>>0)),
					math.Float32frombits(uint32(x1Lo>>32)) > math.Float32frombits(uint32(x2Lo>>32)),
					math.Float32frombits(uint32(x1Hi>>0)) > math.Float32frombits(uint32(x2Hi>>0)),
					math.Float32frombits(uint32(x1Hi>>32)) > math.Float32frombits(uint32(x2Hi>>32)),
				}
			case wazeroir.V128CmpTypeF32x4Le:
				result = []bool{
					math.Float32frombits(uint32(x1Lo>>0)) <= math.Float32frombits(uint32(x2Lo>>0)),
					math.Float32frombits(uint32(x1Lo>>32)) <= math.Float32frombits(uint32(x2Lo>>32)),
					math.Float32frombits(uint32(x1Hi>>0)) <= math.Float32frombits(uint32(x2Hi>>0)),
					math.Float32frombits(uint32(x1Hi>>32)) <= math.Float32frombits(uint32(x2Hi>>32)),
				}
			case wazeroir.V128CmpTypeF32x4Ge:
				result = []bool{
					math.Float32frombits(uint32(x1Lo>>0)) >= math.Float32frombits(uint32(x2Lo>>0)),
					math.Float32frombits(uint32(x1Lo>>32)) >= math.Float32frombits(uint32(x2Lo>>32)),
					math.Float32frombits(uint32(x1Hi>>0)) >= math.Float32frombits(uint32(x2Hi>>0)),
					math.Float32frombits(uint32(x1Hi>>32)) >= math.Float32frombits(uint32(x2Hi>>32)),
				}
			case wazeroir.V128CmpTypeF64x2Eq:
				result = []bool{
					math.Float64frombits(x1Lo) == math.Float64frombits(x2Lo),
					math.Float64frombits(x1Hi) == math.Float64frombits(x2Hi),
				}
			case wazeroir.V128CmpTypeF64x2Ne:
				result = []bool{
					math.Float64frombits(x1Lo) != math.Float64frombits(x2Lo),
					math.Float64frombits(x1Hi) != math.Float64frombits(x2Hi),
				}
			case wazeroir.V128CmpTypeF64x2Lt:
				result = []bool{
					math.Float64frombits(x1Lo) < math.Float64frombits(x2Lo),
					math.Float64frombits(x1Hi) < math.Float64frombits(x2Hi),
				}
			case wazeroir.V128CmpTypeF64x2Gt:
				result = []bool{
					math.Float64frombits(x1Lo) > math.Float64frombits(x2Lo),
					math.Float64frombits(x1Hi) > math.Float64frombits(x2Hi),
				}
			case wazeroir.V128CmpTypeF64x2Le:
				result = []bool{
					math.Float64frombits(x1Lo) <= math.Float64frombits(x2Lo),
					math.Float64frombits(x1Hi) <= math.Float64frombits(x2Hi),
				}
			case wazeroir.V128CmpTypeF64x2Ge:
				result = []bool{
					math.Float64frombits(x1Lo) >= math.Float64frombits(x2Lo),
					math.Float64frombits(x1Hi) >= math.Float64frombits(x2Hi),
				}
			}

			var retLo, retHi uint64
			laneNum := len(result)
			switch laneNum {
			case 16:
				for i, b := range result {
					if b {
						if i < 8 {
							retLo |= 0xff << (i * 8)
						} else {
							retHi |= 0xff << ((i - 8) * 8)
						}
					}
				}
			case 8:
				for i, b := range result {
					if b {
						if i < 4 {
							retLo |= 0xffff << (i * 16)
						} else {
							retHi |= 0xffff << ((i - 4) * 16)
						}
					}
				}
			case 4:
				for i, b := range result {
					if b {
						if i < 2 {
							retLo |= 0xffff_ffff << (i * 32)
						} else {
							retHi |= 0xffff_ffff << ((i - 2) * 32)
						}
					}
				}
			case 2:
				if result[0] {
					retLo = ^uint64(0)
				}
				if result[1] {
					retHi = ^uint64(0)
				}
			}

			ce.pushValue(retLo)
			ce.pushValue(retHi)
			frame.pc++
		case wazeroir.OperationKindV128AddSat:
			x2hi, x2Lo := ce.popValue(), ce.popValue()
			x1hi, x1Lo := ce.popValue(), ce.popValue()

			var retLo, retHi uint64

			// Lane-wise addition while saturating the overflowing values.
			// https://github.com/WebAssembly/spec/blob/wg-2.0.draft1/proposals/simd/SIMD.md#saturating-integer-addition
			switch op.b1 {
			case wazeroir.ShapeI8x16:
				for i := 0; i < 16; i++ {
					var v, w byte
					if i < 8 {
						v, w = byte(x1Lo>>(i*8)), byte(x2Lo>>(i*8))
					} else {
						v, w = byte(x1hi>>((i-8)*8)), byte(x2hi>>((i-8)*8))
					}

					var uv uint64
					if op.b3 { // signed
						if subbed := int64(int8(v)) + int64(int8(w)); subbed < math.MinInt8 {
							uv = uint64(byte(0x80))
						} else if subbed > math.MaxInt8 {
							uv = uint64(byte(0x7f))
						} else {
							uv = uint64(byte(int8(subbed)))
						}
					} else {
						if subbed := int64(v) + int64(w); subbed < 0 {
							uv = uint64(byte(0))
						} else if subbed > math.MaxUint8 {
							uv = uint64(byte(0xff))
						} else {
							uv = uint64(byte(subbed))
						}
					}

					if i < 8 { // first 8 lanes are on lower 64bits.
						retLo |= uv << (i * 8)
					} else {
						retHi |= uv << ((i - 8) * 8)
					}
				}
			case wazeroir.ShapeI16x8:
				for i := 0; i < 8; i++ {
					var v, w uint16
					if i < 4 {
						v, w = uint16(x1Lo>>(i*16)), uint16(x2Lo>>(i*16))
					} else {
						v, w = uint16(x1hi>>((i-4)*16)), uint16(x2hi>>((i-4)*16))
					}

					var uv uint64
					if op.b3 { // signed
						if added := int64(int16(v)) + int64(int16(w)); added < math.MinInt16 {
							uv = uint64(uint16(0x8000))
						} else if added > math.MaxInt16 {
							uv = uint64(uint16(0x7fff))
						} else {
							uv = uint64(uint16(int16(added)))
						}
					} else {
						if added := int64(v) + int64(w); added < 0 {
							uv = uint64(uint16(0))
						} else if added > math.MaxUint16 {
							uv = uint64(uint16(0xffff))
						} else {
							uv = uint64(uint16(added))
						}
					}

					if i < 4 { // first 4 lanes are on lower 64bits.
						retLo |= uv << (i * 16)
					} else {
						retHi |= uv << ((i - 4) * 16)
					}
				}
			}

			ce.pushValue(retLo)
			ce.pushValue(retHi)
			frame.pc++
		case wazeroir.OperationKindV128SubSat:
			x2hi, x2Lo := ce.popValue(), ce.popValue()
			x1hi, x1Lo := ce.popValue(), ce.popValue()

			var retLo, retHi uint64

			// Lane-wise subtraction while saturating the overflowing values.
			// https://github.com/WebAssembly/spec/blob/wg-2.0.draft1/proposals/simd/SIMD.md#saturating-integer-subtraction
			switch op.b1 {
			case wazeroir.ShapeI8x16:
				for i := 0; i < 16; i++ {
					var v, w byte
					if i < 8 {
						v, w = byte(x1Lo>>(i*8)), byte(x2Lo>>(i*8))
					} else {
						v, w = byte(x1hi>>((i-8)*8)), byte(x2hi>>((i-8)*8))
					}

					var uv uint64
					if op.b3 { // signed
						if subbed := int64(int8(v)) - int64(int8(w)); subbed < math.MinInt8 {
							uv = uint64(byte(0x80))
						} else if subbed > math.MaxInt8 {
							uv = uint64(byte(0x7f))
						} else {
							uv = uint64(byte(int8(subbed)))
						}
					} else {
						if subbed := int64(v) - int64(w); subbed < 0 {
							uv = uint64(byte(0))
						} else if subbed > math.MaxUint8 {
							uv = uint64(byte(0xff))
						} else {
							uv = uint64(byte(subbed))
						}
					}

					if i < 8 {
						retLo |= uv << (i * 8)
					} else {
						retHi |= uv << ((i - 8) * 8)
					}
				}
			case wazeroir.ShapeI16x8:
				for i := 0; i < 8; i++ {
					var v, w uint16
					if i < 4 {
						v, w = uint16(x1Lo>>(i*16)), uint16(x2Lo>>(i*16))
					} else {
						v, w = uint16(x1hi>>((i-4)*16)), uint16(x2hi>>((i-4)*16))
					}

					var uv uint64
					if op.b3 { // signed
						if subbed := int64(int16(v)) - int64(int16(w)); subbed < math.MinInt16 {
							uv = uint64(uint16(0x8000))
						} else if subbed > math.MaxInt16 {
							uv = uint64(uint16(0x7fff))
						} else {
							uv = uint64(uint16(int16(subbed)))
						}
					} else {
						if subbed := int64(v) - int64(w); subbed < 0 {
							uv = uint64(uint16(0))
						} else if subbed > math.MaxUint16 {
							uv = uint64(uint16(0xffff))
						} else {
							uv = uint64(uint16(subbed))
						}
					}

					if i < 4 {
						retLo |= uv << (i * 16)
					} else {
						retHi |= uv << ((i - 4) * 16)
					}
				}
			}

			ce.pushValue(retLo)
			ce.pushValue(retHi)
			frame.pc++
		case wazeroir.OperationKindV128Mul:
			x2hi, x2lo := ce.popValue(), ce.popValue()
			x1hi, x1lo := ce.popValue(), ce.popValue()
			var retLo, retHi uint64
			switch op.b1 {
			case wazeroir.ShapeI16x8:
				retHi = uint64(uint16(x1hi)*uint16(x2hi)) | (uint64(uint16(x1hi>>16)*uint16(x2hi>>16)) << 16) |
					(uint64(uint16(x1hi>>32)*uint16(x2hi>>32)) << 32) | (uint64(uint16(x1hi>>48)*uint16(x2hi>>48)) << 48)
				retLo = uint64(uint16(x1lo)*uint16(x2lo)) | (uint64(uint16(x1lo>>16)*uint16(x2lo>>16)) << 16) |
					(uint64(uint16(x1lo>>32)*uint16(x2lo>>32)) << 32) | (uint64(uint16(x1lo>>48)*uint16(x2lo>>48)) << 48)
			case wazeroir.ShapeI32x4:
				retHi = uint64(uint32(x1hi)*uint32(x2hi)) | (uint64(uint32(x1hi>>32)*uint32(x2hi>>32)) << 32)
				retLo = uint64(uint32(x1lo)*uint32(x2lo)) | (uint64(uint32(x1lo>>32)*uint32(x2lo>>32)) << 32)
			case wazeroir.ShapeI64x2:
				retHi = x1hi * x2hi
				retLo = x1lo * x2lo
			case wazeroir.ShapeF32x4:
				retHi = mulFloat32bits(uint32(x1hi), uint32(x2hi)) | mulFloat32bits(uint32(x1hi>>32), uint32(x2hi>>32))<<32
				retLo = mulFloat32bits(uint32(x1lo), uint32(x2lo)) | mulFloat32bits(uint32(x1lo>>32), uint32(x2lo>>32))<<32
			case wazeroir.ShapeF64x2:
				retHi = math.Float64bits(math.Float64frombits(x1hi) * math.Float64frombits(x2hi))
				retLo = math.Float64bits(math.Float64frombits(x1lo) * math.Float64frombits(x2lo))
			}
			ce.pushValue(retLo)
			ce.pushValue(retHi)
			frame.pc++
		case wazeroir.OperationKindV128Div:
			x2hi, x2lo := ce.popValue(), ce.popValue()
			x1hi, x1lo := ce.popValue(), ce.popValue()
			var retLo, retHi uint64
			if op.b1 == wazeroir.ShapeF64x2 {
				retHi = math.Float64bits(math.Float64frombits(x1hi) / math.Float64frombits(x2hi))
				retLo = math.Float64bits(math.Float64frombits(x1lo) / math.Float64frombits(x2lo))
			} else {
				retHi = divFloat32bits(uint32(x1hi), uint32(x2hi)) | divFloat32bits(uint32(x1hi>>32), uint32(x2hi>>32))<<32
				retLo = divFloat32bits(uint32(x1lo), uint32(x2lo)) | divFloat32bits(uint32(x1lo>>32), uint32(x2lo>>32))<<32
			}
			ce.pushValue(retLo)
			ce.pushValue(retHi)
			frame.pc++
		case wazeroir.OperationKindV128Neg:
			hi, lo := ce.popValue(), ce.popValue()
			switch op.b1 {
			case wazeroir.ShapeI8x16:
				lo = uint64(-byte(lo)) | (uint64(-byte(lo>>8)) << 8) |
					(uint64(-byte(lo>>16)) << 16) | (uint64(-byte(lo>>24)) << 24) |
					(uint64(-byte(lo>>32)) << 32) | (uint64(-byte(lo>>40)) << 40) |
					(uint64(-byte(lo>>48)) << 48) | (uint64(-byte(lo>>56)) << 56)
				hi = uint64(-byte(hi)) | (uint64(-byte(hi>>8)) << 8) |
					(uint64(-byte(hi>>16)) << 16) | (uint64(-byte(hi>>24)) << 24) |
					(uint64(-byte(hi>>32)) << 32) | (uint64(-byte(hi>>40)) << 40) |
					(uint64(-byte(hi>>48)) << 48) | (uint64(-byte(hi>>56)) << 56)
			case wazeroir.ShapeI16x8:
				hi = uint64(-uint16(hi)) | (uint64(-uint16(hi>>16)) << 16) |
					(uint64(-uint16(hi>>32)) << 32) | (uint64(-uint16(hi>>48)) << 48)
				lo = uint64(-uint16(lo)) | (uint64(-uint16(lo>>16)) << 16) |
					(uint64(-uint16(lo>>32)) << 32) | (uint64(-uint16(lo>>48)) << 48)
			case wazeroir.ShapeI32x4:
				hi = uint64(-uint32(hi)) | (uint64(-uint32(hi>>32)) << 32)
				lo = uint64(-uint32(lo)) | (uint64(-uint32(lo>>32)) << 32)
			case wazeroir.ShapeI64x2:
				hi = -hi
				lo = -lo
			case wazeroir.ShapeF32x4:
				hi = uint64(math.Float32bits(-math.Float32frombits(uint32(hi)))) |
					(uint64(math.Float32bits(-math.Float32frombits(uint32(hi>>32)))) << 32)
				lo = uint64(math.Float32bits(-math.Float32frombits(uint32(lo)))) |
					(uint64(math.Float32bits(-math.Float32frombits(uint32(lo>>32)))) << 32)
			case wazeroir.ShapeF64x2:
				hi = math.Float64bits(-math.Float64frombits(hi))
				lo = math.Float64bits(-math.Float64frombits(lo))
			}
			ce.pushValue(lo)
			ce.pushValue(hi)
			frame.pc++
		case wazeroir.OperationKindV128Sqrt:
			hi, lo := ce.popValue(), ce.popValue()
			if op.b1 == wazeroir.ShapeF64x2 {
				hi = math.Float64bits(math.Sqrt(math.Float64frombits(hi)))
				lo = math.Float64bits(math.Sqrt(math.Float64frombits(lo)))
			} else {
				hi = uint64(math.Float32bits(float32(math.Sqrt(float64(math.Float32frombits(uint32(hi))))))) |
					(uint64(math.Float32bits(float32(math.Sqrt(float64(math.Float32frombits(uint32(hi>>32))))))) << 32)
				lo = uint64(math.Float32bits(float32(math.Sqrt(float64(math.Float32frombits(uint32(lo))))))) |
					(uint64(math.Float32bits(float32(math.Sqrt(float64(math.Float32frombits(uint32(lo>>32))))))) << 32)
			}
			ce.pushValue(lo)
			ce.pushValue(hi)
			frame.pc++
		case wazeroir.OperationKindV128Abs:
			hi, lo := ce.popValue(), ce.popValue()
			switch op.b1 {
			case wazeroir.ShapeI8x16:
				lo = uint64(i8Abs(byte(lo))) | (uint64(i8Abs(byte(lo>>8))) << 8) |
					(uint64(i8Abs(byte(lo>>16))) << 16) | (uint64(i8Abs(byte(lo>>24))) << 24) |
					(uint64(i8Abs(byte(lo>>32))) << 32) | (uint64(i8Abs(byte(lo>>40))) << 40) |
					(uint64(i8Abs(byte(lo>>48))) << 48) | (uint64(i8Abs(byte(lo>>56))) << 56)
				hi = uint64(i8Abs(byte(hi))) | (uint64(i8Abs(byte(hi>>8))) << 8) |
					(uint64(i8Abs(byte(hi>>16))) << 16) | (uint64(i8Abs(byte(hi>>24))) << 24) |
					(uint64(i8Abs(byte(hi>>32))) << 32) | (uint64(i8Abs(byte(hi>>40))) << 40) |
					(uint64(i8Abs(byte(hi>>48))) << 48) | (uint64(i8Abs(byte(hi>>56))) << 56)
			case wazeroir.ShapeI16x8:
				hi = uint64(i16Abs(uint16(hi))) | (uint64(i16Abs(uint16(hi>>16))) << 16) |
					(uint64(i16Abs(uint16(hi>>32))) << 32) | (uint64(i16Abs(uint16(hi>>48))) << 48)
				lo = uint64(i16Abs(uint16(lo))) | (uint64(i16Abs(uint16(lo>>16))) << 16) |
					(uint64(i16Abs(uint16(lo>>32))) << 32) | (uint64(i16Abs(uint16(lo>>48))) << 48)
			case wazeroir.ShapeI32x4:
				hi = uint64(i32Abs(uint32(hi))) | (uint64(i32Abs(uint32(hi>>32))) << 32)
				lo = uint64(i32Abs(uint32(lo))) | (uint64(i32Abs(uint32(lo>>32))) << 32)
			case wazeroir.ShapeI64x2:
				if int64(hi) < 0 {
					hi = -hi
				}
				if int64(lo) < 0 {
					lo = -lo
				}
			case wazeroir.ShapeF32x4:
				hi = hi &^ (1<<31 | 1<<63)
				lo = lo &^ (1<<31 | 1<<63)
			case wazeroir.ShapeF64x2:
				hi = hi &^ (1 << 63)
				lo = lo &^ (1 << 63)
			}
			ce.pushValue(lo)
			ce.pushValue(hi)
			frame.pc++
		case wazeroir.OperationKindV128Popcnt:
			hi, lo := ce.popValue(), ce.popValue()
			var retLo, retHi uint64
			for i := 0; i < 16; i++ {
				var v byte
				if i < 8 {
					v = byte(lo >> (i * 8))
				} else {
					v = byte(hi >> ((i - 8) * 8))
				}

				var cnt uint64
				for i := 0; i < 8; i++ {
					if (v>>i)&0b1 != 0 {
						cnt++
					}
				}

				if i < 8 {
					retLo |= cnt << (i * 8)
				} else {
					retHi |= cnt << ((i - 8) * 8)
				}
			}
			ce.pushValue(retLo)
			ce.pushValue(retHi)
			frame.pc++
		case wazeroir.OperationKindV128Min:
			x2hi, x2lo := ce.popValue(), ce.popValue()
			x1hi, x1lo := ce.popValue(), ce.popValue()
			var retLo, retHi uint64
			switch op.b1 {
			case wazeroir.ShapeI8x16:
				if op.b3 { // signed
					retLo = uint64(i8MinS(uint8(x1lo>>8), uint8(x2lo>>8)))<<8 | uint64(i8MinS(uint8(x1lo), uint8(x2lo))) |
						uint64(i8MinS(uint8(x1lo>>24), uint8(x2lo>>24)))<<24 | uint64(i8MinS(uint8(x1lo>>16), uint8(x2lo>>16)))<<16 |
						uint64(i8MinS(uint8(x1lo>>40), uint8(x2lo>>40)))<<40 | uint64(i8MinS(uint8(x1lo>>32), uint8(x2lo>>32)))<<32 |
						uint64(i8MinS(uint8(x1lo>>56), uint8(x2lo>>56)))<<56 | uint64(i8MinS(uint8(x1lo>>48), uint8(x2lo>>48)))<<48
					retHi = uint64(i8MinS(uint8(x1hi>>8), uint8(x2hi>>8)))<<8 | uint64(i8MinS(uint8(x1hi), uint8(x2hi))) |
						uint64(i8MinS(uint8(x1hi>>24), uint8(x2hi>>24)))<<24 | uint64(i8MinS(uint8(x1hi>>16), uint8(x2hi>>16)))<<16 |
						uint64(i8MinS(uint8(x1hi>>40), uint8(x2hi>>40)))<<40 | uint64(i8MinS(uint8(x1hi>>32), uint8(x2hi>>32)))<<32 |
						uint64(i8MinS(uint8(x1hi>>56), uint8(x2hi>>56)))<<56 | uint64(i8MinS(uint8(x1hi>>48), uint8(x2hi>>48)))<<48
				} else {
					retLo = uint64(i8MinU(uint8(x1lo>>8), uint8(x2lo>>8)))<<8 | uint64(i8MinU(uint8(x1lo), uint8(x2lo))) |
						uint64(i8MinU(uint8(x1lo>>24), uint8(x2lo>>24)))<<24 | uint64(i8MinU(uint8(x1lo>>16), uint8(x2lo>>16)))<<16 |
						uint64(i8MinU(uint8(x1lo>>40), uint8(x2lo>>40)))<<40 | uint64(i8MinU(uint8(x1lo>>32), uint8(x2lo>>32)))<<32 |
						uint64(i8MinU(uint8(x1lo>>56), uint8(x2lo>>56)))<<56 | uint64(i8MinU(uint8(x1lo>>48), uint8(x2lo>>48)))<<48
					retHi = uint64(i8MinU(uint8(x1hi>>8), uint8(x2hi>>8)))<<8 | uint64(i8MinU(uint8(x1hi), uint8(x2hi))) |
						uint64(i8MinU(uint8(x1hi>>24), uint8(x2hi>>24)))<<24 | uint64(i8MinU(uint8(x1hi>>16), uint8(x2hi>>16)))<<16 |
						uint64(i8MinU(uint8(x1hi>>40), uint8(x2hi>>40)))<<40 | uint64(i8MinU(uint8(x1hi>>32), uint8(x2hi>>32)))<<32 |
						uint64(i8MinU(uint8(x1hi>>56), uint8(x2hi>>56)))<<56 | uint64(i8MinU(uint8(x1hi>>48), uint8(x2hi>>48)))<<48
				}
			case wazeroir.ShapeI16x8:
				if op.b3 { // signed
					retLo = uint64(i16MinS(uint16(x1lo), uint16(x2lo))) |
						uint64(i16MinS(uint16(x1lo>>16), uint16(x2lo>>16)))<<16 |
						uint64(i16MinS(uint16(x1lo>>32), uint16(x2lo>>32)))<<32 |
						uint64(i16MinS(uint16(x1lo>>48), uint16(x2lo>>48)))<<48
					retHi = uint64(i16MinS(uint16(x1hi), uint16(x2hi))) |
						uint64(i16MinS(uint16(x1hi>>16), uint16(x2hi>>16)))<<16 |
						uint64(i16MinS(uint16(x1hi>>32), uint16(x2hi>>32)))<<32 |
						uint64(i16MinS(uint16(x1hi>>48), uint16(x2hi>>48)))<<48
				} else {
					retLo = uint64(i16MinU(uint16(x1lo), uint16(x2lo))) |
						uint64(i16MinU(uint16(x1lo>>16), uint16(x2lo>>16)))<<16 |
						uint64(i16MinU(uint16(x1lo>>32), uint16(x2lo>>32)))<<32 |
						uint64(i16MinU(uint16(x1lo>>48), uint16(x2lo>>48)))<<48
					retHi = uint64(i16MinU(uint16(x1hi), uint16(x2hi))) |
						uint64(i16MinU(uint16(x1hi>>16), uint16(x2hi>>16)))<<16 |
						uint64(i16MinU(uint16(x1hi>>32), uint16(x2hi>>32)))<<32 |
						uint64(i16MinU(uint16(x1hi>>48), uint16(x2hi>>48)))<<48
				}
			case wazeroir.ShapeI32x4:
				if op.b3 { // signed
					retLo = uint64(i32MinS(uint32(x1lo), uint32(x2lo))) |
						uint64(i32MinS(uint32(x1lo>>32), uint32(x2lo>>32)))<<32
					retHi = uint64(i32MinS(uint32(x1hi), uint32(x2hi))) |
						uint64(i32MinS(uint32(x1hi>>32), uint32(x2hi>>32)))<<32
				} else {
					retLo = uint64(i32MinU(uint32(x1lo), uint32(x2lo))) |
						uint64(i32MinU(uint32(x1lo>>32), uint32(x2lo>>32)))<<32
					retHi = uint64(i32MinU(uint32(x1hi), uint32(x2hi))) |
						uint64(i32MinU(uint32(x1hi>>32), uint32(x2hi>>32)))<<32
				}
			case wazeroir.ShapeF32x4:
				retHi = WasmCompatMin32bits(uint32(x1hi), uint32(x2hi)) |
					WasmCompatMin32bits(uint32(x1hi>>32), uint32(x2hi>>32))<<32
				retLo = WasmCompatMin32bits(uint32(x1lo), uint32(x2lo)) |
					WasmCompatMin32bits(uint32(x1lo>>32), uint32(x2lo>>32))<<32
			case wazeroir.ShapeF64x2:
				retHi = math.Float64bits(moremath.WasmCompatMin64(
					math.Float64frombits(x1hi),
					math.Float64frombits(x2hi),
				))
				retLo = math.Float64bits(moremath.WasmCompatMin64(
					math.Float64frombits(x1lo),
					math.Float64frombits(x2lo),
				))
			}
			ce.pushValue(retLo)
			ce.pushValue(retHi)
			frame.pc++
		case wazeroir.OperationKindV128Max:
			x2hi, x2lo := ce.popValue(), ce.popValue()
			x1hi, x1lo := ce.popValue(), ce.popValue()
			var retLo, retHi uint64
			switch op.b1 {
			case wazeroir.ShapeI8x16:
				if op.b3 { // signed
					retLo = uint64(i8MaxS(uint8(x1lo>>8), uint8(x2lo>>8)))<<8 | uint64(i8MaxS(uint8(x1lo), uint8(x2lo))) |
						uint64(i8MaxS(uint8(x1lo>>24), uint8(x2lo>>24)))<<24 | uint64(i8MaxS(uint8(x1lo>>16), uint8(x2lo>>16)))<<16 |
						uint64(i8MaxS(uint8(x1lo>>40), uint8(x2lo>>40)))<<40 | uint64(i8MaxS(uint8(x1lo>>32), uint8(x2lo>>32)))<<32 |
						uint64(i8MaxS(uint8(x1lo>>56), uint8(x2lo>>56)))<<56 | uint64(i8MaxS(uint8(x1lo>>48), uint8(x2lo>>48)))<<48
					retHi = uint64(i8MaxS(uint8(x1hi>>8), uint8(x2hi>>8)))<<8 | uint64(i8MaxS(uint8(x1hi), uint8(x2hi))) |
						uint64(i8MaxS(uint8(x1hi>>24), uint8(x2hi>>24)))<<24 | uint64(i8MaxS(uint8(x1hi>>16), uint8(x2hi>>16)))<<16 |
						uint64(i8MaxS(uint8(x1hi>>40), uint8(x2hi>>40)))<<40 | uint64(i8MaxS(uint8(x1hi>>32), uint8(x2hi>>32)))<<32 |
						uint64(i8MaxS(uint8(x1hi>>56), uint8(x2hi>>56)))<<56 | uint64(i8MaxS(uint8(x1hi>>48), uint8(x2hi>>48)))<<48
				} else {
					retLo = uint64(i8MaxU(uint8(x1lo>>8), uint8(x2lo>>8)))<<8 | uint64(i8MaxU(uint8(x1lo), uint8(x2lo))) |
						uint64(i8MaxU(uint8(x1lo>>24), uint8(x2lo>>24)))<<24 | uint64(i8MaxU(uint8(x1lo>>16), uint8(x2lo>>16)))<<16 |
						uint64(i8MaxU(uint8(x1lo>>40), uint8(x2lo>>40)))<<40 | uint64(i8MaxU(uint8(x1lo>>32), uint8(x2lo>>32)))<<32 |
						uint64(i8MaxU(uint8(x1lo>>56), uint8(x2lo>>56)))<<56 | uint64(i8MaxU(uint8(x1lo>>48), uint8(x2lo>>48)))<<48
					retHi = uint64(i8MaxU(uint8(x1hi>>8), uint8(x2hi>>8)))<<8 | uint64(i8MaxU(uint8(x1hi), uint8(x2hi))) |
						uint64(i8MaxU(uint8(x1hi>>24), uint8(x2hi>>24)))<<24 | uint64(i8MaxU(uint8(x1hi>>16), uint8(x2hi>>16)))<<16 |
						uint64(i8MaxU(uint8(x1hi>>40), uint8(x2hi>>40)))<<40 | uint64(i8MaxU(uint8(x1hi>>32), uint8(x2hi>>32)))<<32 |
						uint64(i8MaxU(uint8(x1hi>>56), uint8(x2hi>>56)))<<56 | uint64(i8MaxU(uint8(x1hi>>48), uint8(x2hi>>48)))<<48
				}
			case wazeroir.ShapeI16x8:
				if op.b3 { // signed
					retLo = uint64(i16MaxS(uint16(x1lo), uint16(x2lo))) |
						uint64(i16MaxS(uint16(x1lo>>16), uint16(x2lo>>16)))<<16 |
						uint64(i16MaxS(uint16(x1lo>>32), uint16(x2lo>>32)))<<32 |
						uint64(i16MaxS(uint16(x1lo>>48), uint16(x2lo>>48)))<<48
					retHi = uint64(i16MaxS(uint16(x1hi), uint16(x2hi))) |
						uint64(i16MaxS(uint16(x1hi>>16), uint16(x2hi>>16)))<<16 |
						uint64(i16MaxS(uint16(x1hi>>32), uint16(x2hi>>32)))<<32 |
						uint64(i16MaxS(uint16(x1hi>>48), uint16(x2hi>>48)))<<48
				} else {
					retLo = uint64(i16MaxU(uint16(x1lo), uint16(x2lo))) |
						uint64(i16MaxU(uint16(x1lo>>16), uint16(x2lo>>16)))<<16 |
						uint64(i16MaxU(uint16(x1lo>>32), uint16(x2lo>>32)))<<32 |
						uint64(i16MaxU(uint16(x1lo>>48), uint16(x2lo>>48)))<<48
					retHi = uint64(i16MaxU(uint16(x1hi), uint16(x2hi))) |
						uint64(i16MaxU(uint16(x1hi>>16), uint16(x2hi>>16)))<<16 |
						uint64(i16MaxU(uint16(x1hi>>32), uint16(x2hi>>32)))<<32 |
						uint64(i16MaxU(uint16(x1hi>>48), uint16(x2hi>>48)))<<48
				}
			case wazeroir.ShapeI32x4:
				if op.b3 { // signed
					retLo = uint64(i32MaxS(uint32(x1lo), uint32(x2lo))) |
						uint64(i32MaxS(uint32(x1lo>>32), uint32(x2lo>>32)))<<32
					retHi = uint64(i32MaxS(uint32(x1hi), uint32(x2hi))) |
						uint64(i32MaxS(uint32(x1hi>>32), uint32(x2hi>>32)))<<32
				} else {
					retLo = uint64(i32MaxU(uint32(x1lo), uint32(x2lo))) |
						uint64(i32MaxU(uint32(x1lo>>32), uint32(x2lo>>32)))<<32
					retHi = uint64(i32MaxU(uint32(x1hi), uint32(x2hi))) |
						uint64(i32MaxU(uint32(x1hi>>32), uint32(x2hi>>32)))<<32
				}
			case wazeroir.ShapeF32x4:
				retHi = WasmCompatMax32bits(uint32(x1hi), uint32(x2hi)) |
					WasmCompatMax32bits(uint32(x1hi>>32), uint32(x2hi>>32))<<32
				retLo = WasmCompatMax32bits(uint32(x1lo), uint32(x2lo)) |
					WasmCompatMax32bits(uint32(x1lo>>32), uint32(x2lo>>32))<<32
			case wazeroir.ShapeF64x2:
				retHi = math.Float64bits(moremath.WasmCompatMax64(
					math.Float64frombits(x1hi),
					math.Float64frombits(x2hi),
				))
				retLo = math.Float64bits(moremath.WasmCompatMax64(
					math.Float64frombits(x1lo),
					math.Float64frombits(x2lo),
				))
			}
			ce.pushValue(retLo)
			ce.pushValue(retHi)
			frame.pc++
		case wazeroir.OperationKindV128AvgrU:
			x2hi, x2lo := ce.popValue(), ce.popValue()
			x1hi, x1lo := ce.popValue(), ce.popValue()
			var retLo, retHi uint64
			switch op.b1 {
			case wazeroir.ShapeI8x16:
				retLo = uint64(i8RoundingAverage(uint8(x1lo>>8), uint8(x2lo>>8)))<<8 | uint64(i8RoundingAverage(uint8(x1lo), uint8(x2lo))) |
					uint64(i8RoundingAverage(uint8(x1lo>>24), uint8(x2lo>>24)))<<24 | uint64(i8RoundingAverage(uint8(x1lo>>16), uint8(x2lo>>16)))<<16 |
					uint64(i8RoundingAverage(uint8(x1lo>>40), uint8(x2lo>>40)))<<40 | uint64(i8RoundingAverage(uint8(x1lo>>32), uint8(x2lo>>32)))<<32 |
					uint64(i8RoundingAverage(uint8(x1lo>>56), uint8(x2lo>>56)))<<56 | uint64(i8RoundingAverage(uint8(x1lo>>48), uint8(x2lo>>48)))<<48
				retHi = uint64(i8RoundingAverage(uint8(x1hi>>8), uint8(x2hi>>8)))<<8 | uint64(i8RoundingAverage(uint8(x1hi), uint8(x2hi))) |
					uint64(i8RoundingAverage(uint8(x1hi>>24), uint8(x2hi>>24)))<<24 | uint64(i8RoundingAverage(uint8(x1hi>>16), uint8(x2hi>>16)))<<16 |
					uint64(i8RoundingAverage(uint8(x1hi>>40), uint8(x2hi>>40)))<<40 | uint64(i8RoundingAverage(uint8(x1hi>>32), uint8(x2hi>>32)))<<32 |
					uint64(i8RoundingAverage(uint8(x1hi>>56), uint8(x2hi>>56)))<<56 | uint64(i8RoundingAverage(uint8(x1hi>>48), uint8(x2hi>>48)))<<48
			case wazeroir.ShapeI16x8:
				retLo = uint64(i16RoundingAverage(uint16(x1lo), uint16(x2lo))) |
					uint64(i16RoundingAverage(uint16(x1lo>>16), uint16(x2lo>>16)))<<16 |
					uint64(i16RoundingAverage(uint16(x1lo>>32), uint16(x2lo>>32)))<<32 |
					uint64(i16RoundingAverage(uint16(x1lo>>48), uint16(x2lo>>48)))<<48
				retHi = uint64(i16RoundingAverage(uint16(x1hi), uint16(x2hi))) |
					uint64(i16RoundingAverage(uint16(x1hi>>16), uint16(x2hi>>16)))<<16 |
					uint64(i16RoundingAverage(uint16(x1hi>>32), uint16(x2hi>>32)))<<32 |
					uint64(i16RoundingAverage(uint16(x1hi>>48), uint16(x2hi>>48)))<<48
			}
			ce.pushValue(retLo)
			ce.pushValue(retHi)
			frame.pc++
		case wazeroir.OperationKindV128Pmin:
			x2hi, x2lo := ce.popValue(), ce.popValue()
			x1hi, x1lo := ce.popValue(), ce.popValue()
			var retLo, retHi uint64
			if op.b1 == wazeroir.ShapeF32x4 {
				if flt32(math.Float32frombits(uint32(x2lo)), math.Float32frombits(uint32(x1lo))) {
					retLo = x2lo & 0x00000000_ffffffff
				} else {
					retLo = x1lo & 0x00000000_ffffffff
				}
				if flt32(math.Float32frombits(uint32(x2lo>>32)), math.Float32frombits(uint32(x1lo>>32))) {
					retLo |= x2lo & 0xffffffff_00000000
				} else {
					retLo |= x1lo & 0xffffffff_00000000
				}
				if flt32(math.Float32frombits(uint32(x2hi)), math.Float32frombits(uint32(x1hi))) {
					retHi = x2hi & 0x00000000_ffffffff
				} else {
					retHi = x1hi & 0x00000000_ffffffff
				}
				if flt32(math.Float32frombits(uint32(x2hi>>32)), math.Float32frombits(uint32(x1hi>>32))) {
					retHi |= x2hi & 0xffffffff_00000000
				} else {
					retHi |= x1hi & 0xffffffff_00000000
				}
			} else {
				if flt64(math.Float64frombits(x2lo), math.Float64frombits(x1lo)) {
					retLo = x2lo
				} else {
					retLo = x1lo
				}
				if flt64(math.Float64frombits(x2hi), math.Float64frombits(x1hi)) {
					retHi = x2hi
				} else {
					retHi = x1hi
				}
			}
			ce.pushValue(retLo)
			ce.pushValue(retHi)
			frame.pc++
		case wazeroir.OperationKindV128Pmax:
			x2hi, x2lo := ce.popValue(), ce.popValue()
			x1hi, x1lo := ce.popValue(), ce.popValue()
			var retLo, retHi uint64
			if op.b1 == wazeroir.ShapeF32x4 {
				if flt32(math.Float32frombits(uint32(x1lo)), math.Float32frombits(uint32(x2lo))) {
					retLo = x2lo & 0x00000000_ffffffff
				} else {
					retLo = x1lo & 0x00000000_ffffffff
				}
				if flt32(math.Float32frombits(uint32(x1lo>>32)), math.Float32frombits(uint32(x2lo>>32))) {
					retLo |= x2lo & 0xffffffff_00000000
				} else {
					retLo |= x1lo & 0xffffffff_00000000
				}
				if flt32(math.Float32frombits(uint32(x1hi)), math.Float32frombits(uint32(x2hi))) {
					retHi = x2hi & 0x00000000_ffffffff
				} else {
					retHi = x1hi & 0x00000000_ffffffff
				}
				if flt32(math.Float32frombits(uint32(x1hi>>32)), math.Float32frombits(uint32(x2hi>>32))) {
					retHi |= x2hi & 0xffffffff_00000000
				} else {
					retHi |= x1hi & 0xffffffff_00000000
				}
			} else {
				if flt64(math.Float64frombits(x1lo), math.Float64frombits(x2lo)) {
					retLo = x2lo
				} else {
					retLo = x1lo
				}
				if flt64(math.Float64frombits(x1hi), math.Float64frombits(x2hi)) {
					retHi = x2hi
				} else {
					retHi = x1hi
				}
			}
			ce.pushValue(retLo)
			ce.pushValue(retHi)
			frame.pc++
		case wazeroir.OperationKindV128Ceil:
			hi, lo := ce.popValue(), ce.popValue()
			if op.b1 == wazeroir.ShapeF32x4 {
				lo = uint64(math.Float32bits(moremath.WasmCompatCeilF32(math.Float32frombits(uint32(lo))))) |
					(uint64(math.Float32bits(moremath.WasmCompatCeilF32(math.Float32frombits(uint32(lo>>32))))) << 32)
				hi = uint64(math.Float32bits(moremath.WasmCompatCeilF32(math.Float32frombits(uint32(hi))))) |
					(uint64(math.Float32bits(moremath.WasmCompatCeilF32(math.Float32frombits(uint32(hi>>32))))) << 32)
			} else {
				lo = math.Float64bits(moremath.WasmCompatCeilF64(math.Float64frombits(lo)))
				hi = math.Float64bits(moremath.WasmCompatCeilF64(math.Float64frombits(hi)))
			}
			ce.pushValue(lo)
			ce.pushValue(hi)
			frame.pc++
		case wazeroir.OperationKindV128Floor:
			hi, lo := ce.popValue(), ce.popValue()
			if op.b1 == wazeroir.ShapeF32x4 {
				lo = uint64(math.Float32bits(moremath.WasmCompatFloorF32(math.Float32frombits(uint32(lo))))) |
					(uint64(math.Float32bits(moremath.WasmCompatFloorF32(math.Float32frombits(uint32(lo>>32))))) << 32)
				hi = uint64(math.Float32bits(moremath.WasmCompatFloorF32(math.Float32frombits(uint32(hi))))) |
					(uint64(math.Float32bits(moremath.WasmCompatFloorF32(math.Float32frombits(uint32(hi>>32))))) << 32)
			} else {
				lo = math.Float64bits(moremath.WasmCompatFloorF64(math.Float64frombits(lo)))
				hi = math.Float64bits(moremath.WasmCompatFloorF64(math.Float64frombits(hi)))
			}
			ce.pushValue(lo)
			ce.pushValue(hi)
			frame.pc++
		case wazeroir.OperationKindV128Trunc:
			hi, lo := ce.popValue(), ce.popValue()
			if op.b1 == wazeroir.ShapeF32x4 {
				lo = uint64(math.Float32bits(moremath.WasmCompatTruncF32(math.Float32frombits(uint32(lo))))) |
					(uint64(math.Float32bits(moremath.WasmCompatTruncF32(math.Float32frombits(uint32(lo>>32))))) << 32)
				hi = uint64(math.Float32bits(moremath.WasmCompatTruncF32(math.Float32frombits(uint32(hi))))) |
					(uint64(math.Float32bits(moremath.WasmCompatTruncF32(math.Float32frombits(uint32(hi>>32))))) << 32)
			} else {
				lo = math.Float64bits(moremath.WasmCompatTruncF64(math.Float64frombits(lo)))
				hi = math.Float64bits(moremath.WasmCompatTruncF64(math.Float64frombits(hi)))
			}
			ce.pushValue(lo)
			ce.pushValue(hi)
			frame.pc++
		case wazeroir.OperationKindV128Nearest:
			hi, lo := ce.popValue(), ce.popValue()
			if op.b1 == wazeroir.ShapeF32x4 {
				lo = uint64(math.Float32bits(moremath.WasmCompatNearestF32(math.Float32frombits(uint32(lo))))) |
					(uint64(math.Float32bits(moremath.WasmCompatNearestF32(math.Float32frombits(uint32(lo>>32))))) << 32)
				hi = uint64(math.Float32bits(moremath.WasmCompatNearestF32(math.Float32frombits(uint32(hi))))) |
					(uint64(math.Float32bits(moremath.WasmCompatNearestF32(math.Float32frombits(uint32(hi>>32))))) << 32)
			} else {
				lo = math.Float64bits(moremath.WasmCompatNearestF64(math.Float64frombits(lo)))
				hi = math.Float64bits(moremath.WasmCompatNearestF64(math.Float64frombits(hi)))
			}
			ce.pushValue(lo)
			ce.pushValue(hi)
			frame.pc++
		case wazeroir.OperationKindV128Extend:
			hi, lo := ce.popValue(), ce.popValue()
			var origin uint64
			if op.b3 { // use lower 64 bits
				origin = lo
			} else {
				origin = hi
			}

			signed := op.b2 == 1

			var retHi, retLo uint64
			switch op.b1 {
			case wazeroir.ShapeI8x16:
				for i := 0; i < 8; i++ {
					v8 := byte(origin >> (i * 8))

					var v16 uint16
					if signed {
						v16 = uint16(int8(v8))
					} else {
						v16 = uint16(v8)
					}

					if i < 4 {
						retLo |= uint64(v16) << (i * 16)
					} else {
						retHi |= uint64(v16) << ((i - 4) * 16)
					}
				}
			case wazeroir.ShapeI16x8:
				for i := 0; i < 4; i++ {
					v16 := uint16(origin >> (i * 16))

					var v32 uint32
					if signed {
						v32 = uint32(int16(v16))
					} else {
						v32 = uint32(v16)
					}

					if i < 2 {
						retLo |= uint64(v32) << (i * 32)
					} else {
						retHi |= uint64(v32) << ((i - 2) * 32)
					}
				}
			case wazeroir.ShapeI32x4:
				v32Lo := uint32(origin)
				v32Hi := uint32(origin >> 32)
				if signed {
					retLo = uint64(int32(v32Lo))
					retHi = uint64(int32(v32Hi))
				} else {
					retLo = uint64(v32Lo)
					retHi = uint64(v32Hi)
				}
			}
			ce.pushValue(retLo)
			ce.pushValue(retHi)
			frame.pc++
		case wazeroir.OperationKindV128ExtMul:
			x2Hi, x2Lo := ce.popValue(), ce.popValue()
			x1Hi, x1Lo := ce.popValue(), ce.popValue()
			var x1, x2 uint64
			if op.b3 { // use lower 64 bits
				x1, x2 = x1Lo, x2Lo
			} else {
				x1, x2 = x1Hi, x2Hi
			}

			signed := op.b2 == 1

			var retLo, retHi uint64
			switch op.b1 {
			case wazeroir.ShapeI8x16:
				for i := 0; i < 8; i++ {
					v1, v2 := byte(x1>>(i*8)), byte(x2>>(i*8))

					var v16 uint16
					if signed {
						v16 = uint16(int16(int8(v1)) * int16(int8(v2)))
					} else {
						v16 = uint16(v1) * uint16(v2)
					}

					if i < 4 {
						retLo |= uint64(v16) << (i * 16)
					} else {
						retHi |= uint64(v16) << ((i - 4) * 16)
					}
				}
			case wazeroir.ShapeI16x8:
				for i := 0; i < 4; i++ {
					v1, v2 := uint16(x1>>(i*16)), uint16(x2>>(i*16))

					var v32 uint32
					if signed {
						v32 = uint32(int32(int16(v1)) * int32(int16(v2)))
					} else {
						v32 = uint32(v1) * uint32(v2)
					}

					if i < 2 {
						retLo |= uint64(v32) << (i * 32)
					} else {
						retHi |= uint64(v32) << ((i - 2) * 32)
					}
				}
			case wazeroir.ShapeI32x4:
				v1Lo, v2Lo := uint32(x1), uint32(x2)
				v1Hi, v2Hi := uint32(x1>>32), uint32(x2>>32)
				if signed {
					retLo = uint64(int64(int32(v1Lo)) * int64(int32(v2Lo)))
					retHi = uint64(int64(int32(v1Hi)) * int64(int32(v2Hi)))
				} else {
					retLo = uint64(v1Lo) * uint64(v2Lo)
					retHi = uint64(v1Hi) * uint64(v2Hi)
				}
			}

			ce.pushValue(retLo)
			ce.pushValue(retHi)
			frame.pc++
		case wazeroir.OperationKindV128Q15mulrSatS:
			x2hi, x2Lo := ce.popValue(), ce.popValue()
			x1hi, x1Lo := ce.popValue(), ce.popValue()
			var retLo, retHi uint64
			for i := 0; i < 8; i++ {
				var v, w int16
				if i < 4 {
					v, w = int16(uint16(x1Lo>>(i*16))), int16(uint16(x2Lo>>(i*16)))
				} else {
					v, w = int16(uint16(x1hi>>((i-4)*16))), int16(uint16(x2hi>>((i-4)*16)))
				}

				var uv uint64
				// https://github.com/WebAssembly/spec/blob/wg-2.0.draft1/proposals/simd/SIMD.md#saturating-integer-q-format-rounding-multiplication
				if calc := ((int32(v) * int32(w)) + 0x4000) >> 15; calc < math.MinInt16 {
					uv = uint64(uint16(0x8000))
				} else if calc > math.MaxInt16 {
					uv = uint64(uint16(0x7fff))
				} else {
					uv = uint64(uint16(int16(calc)))
				}

				if i < 4 {
					retLo |= uv << (i * 16)
				} else {
					retHi |= uv << ((i - 4) * 16)
				}
			}

			ce.pushValue(retLo)
			ce.pushValue(retHi)
			frame.pc++
		case wazeroir.OperationKindV128ExtAddPairwise:
			hi, lo := ce.popValue(), ce.popValue()

			signed := op.b3

			var retLo, retHi uint64
			switch op.b1 {
			case wazeroir.ShapeI8x16:
				for i := 0; i < 8; i++ {
					var v1, v2 byte
					if i < 4 {
						v1, v2 = byte(lo>>((i*2)*8)), byte(lo>>((i*2+1)*8))
					} else {
						v1, v2 = byte(hi>>(((i-4)*2)*8)), byte(hi>>(((i-4)*2+1)*8))
					}

					var v16 uint16
					if signed {
						v16 = uint16(int16(int8(v1)) + int16(int8(v2)))
					} else {
						v16 = uint16(v1) + uint16(v2)
					}

					if i < 4 {
						retLo |= uint64(v16) << (i * 16)
					} else {
						retHi |= uint64(v16) << ((i - 4) * 16)
					}
				}
			case wazeroir.ShapeI16x8:
				for i := 0; i < 4; i++ {
					var v1, v2 uint16
					if i < 2 {
						v1, v2 = uint16(lo>>((i*2)*16)), uint16(lo>>((i*2+1)*16))
					} else {
						v1, v2 = uint16(hi>>(((i-2)*2)*16)), uint16(hi>>(((i-2)*2+1)*16))
					}

					var v32 uint32
					if signed {
						v32 = uint32(int32(int16(v1)) + int32(int16(v2)))
					} else {
						v32 = uint32(v1) + uint32(v2)
					}

					if i < 2 {
						retLo |= uint64(v32) << (i * 32)
					} else {
						retHi |= uint64(v32) << ((i - 2) * 32)
					}
				}
			}
			ce.pushValue(retLo)
			ce.pushValue(retHi)
			frame.pc++
		case wazeroir.OperationKindV128FloatPromote:
			_, toPromote := ce.popValue(), ce.popValue()
			ce.pushValue(math.Float64bits(float64(math.Float32frombits(uint32(toPromote)))))
			ce.pushValue(math.Float64bits(float64(math.Float32frombits(uint32(toPromote >> 32)))))
			frame.pc++
		case wazeroir.OperationKindV128FloatDemote:
			hi, lo := ce.popValue(), ce.popValue()
			ce.pushValue(
				uint64(math.Float32bits(float32(math.Float64frombits(lo)))) |
					(uint64(math.Float32bits(float32(math.Float64frombits(hi)))) << 32),
			)
			ce.pushValue(0)
			frame.pc++
		case wazeroir.OperationKindV128FConvertFromI:
			hi, lo := ce.popValue(), ce.popValue()
			v1, v2, v3, v4 := uint32(lo), uint32(lo>>32), uint32(hi), uint32(hi>>32)
			signed := op.b3

			var retLo, retHi uint64
			switch op.b1 { // Destination shape.
			case wazeroir.ShapeF32x4: // f32x4 from signed/unsigned i32x4
				if signed {
					retLo = uint64(math.Float32bits(float32(int32(v1)))) |
						(uint64(math.Float32bits(float32(int32(v2)))) << 32)
					retHi = uint64(math.Float32bits(float32(int32(v3)))) |
						(uint64(math.Float32bits(float32(int32(v4)))) << 32)
				} else {
					retLo = uint64(math.Float32bits(float32(v1))) |
						(uint64(math.Float32bits(float32(v2))) << 32)
					retHi = uint64(math.Float32bits(float32(v3))) |
						(uint64(math.Float32bits(float32(v4))) << 32)
				}
			case wazeroir.ShapeF64x2: // f64x2 from signed/unsigned i32x4
				if signed {
					retLo, retHi = math.Float64bits(float64(int32(v1))), math.Float64bits(float64(int32(v2)))
				} else {
					retLo, retHi = math.Float64bits(float64(v1)), math.Float64bits(float64(v2))
				}
			}

			ce.pushValue(retLo)
			ce.pushValue(retHi)
			frame.pc++
		case wazeroir.OperationKindV128Narrow:
			x2Hi, x2Lo := ce.popValue(), ce.popValue()
			x1Hi, x1Lo := ce.popValue(), ce.popValue()
			signed := op.b3

			var retLo, retHi uint64
			switch op.b1 {
			case wazeroir.ShapeI16x8: // signed/unsigned i16x8 to i8x16
				for i := 0; i < 8; i++ {
					var v16 uint16
					if i < 4 {
						v16 = uint16(x1Lo >> (i * 16))
					} else {
						v16 = uint16(x1Hi >> ((i - 4) * 16))
					}

					var v byte
					if signed {
						if s := int16(v16); s > math.MaxInt8 {
							v = math.MaxInt8
						} else if s < math.MinInt8 {
							s = math.MinInt8
							v = byte(s)
						} else {
							v = byte(v16)
						}
					} else {
						if s := int16(v16); s > math.MaxUint8 {
							v = math.MaxUint8
						} else if s < 0 {
							v = 0
						} else {
							v = byte(v16)
						}
					}
					retLo |= uint64(v) << (i * 8)
				}
				for i := 0; i < 8; i++ {
					var v16 uint16
					if i < 4 {
						v16 = uint16(x2Lo >> (i * 16))
					} else {
						v16 = uint16(x2Hi >> ((i - 4) * 16))
					}

					var v byte
					if signed {
						if s := int16(v16); s > math.MaxInt8 {
							v = math.MaxInt8
						} else if s < math.MinInt8 {
							s = math.MinInt8
							v = byte(s)
						} else {
							v = byte(v16)
						}
					} else {
						if s := int16(v16); s > math.MaxUint8 {
							v = math.MaxUint8
						} else if s < 0 {
							v = 0
						} else {
							v = byte(v16)
						}
					}
					retHi |= uint64(v) << (i * 8)
				}
			case wazeroir.ShapeI32x4: // signed/unsigned i32x4 to i16x8
				for i := 0; i < 4; i++ {
					var v32 uint32
					if i < 2 {
						v32 = uint32(x1Lo >> (i * 32))
					} else {
						v32 = uint32(x1Hi >> ((i - 2) * 32))
					}

					var v uint16
					if signed {
						if s := int32(v32); s > math.MaxInt16 {
							v = math.MaxInt16
						} else if s < math.MinInt16 {
							s = math.MinInt16
							v = uint16(s)
						} else {
							v = uint16(v32)
						}
					} else {
						if s := int32(v32); s > math.MaxUint16 {
							v = math.MaxUint16
						} else if s < 0 {
							v = 0
						} else {
							v = uint16(v32)
						}
					}
					retLo |= uint64(v) << (i * 16)
				}

				for i := 0; i < 4; i++ {
					var v32 uint32
					if i < 2 {
						v32 = uint32(x2Lo >> (i * 32))
					} else {
						v32 = uint32(x2Hi >> ((i - 2) * 32))
					}

					var v uint16
					if signed {
						if s := int32(v32); s > math.MaxInt16 {
							v = math.MaxInt16
						} else if s < math.MinInt16 {
							s = math.MinInt16
							v = uint16(s)
						} else {
							v = uint16(v32)
						}
					} else {
						if s := int32(v32); s > math.MaxUint16 {
							v = math.MaxUint16
						} else if s < 0 {
							v = 0
						} else {
							v = uint16(v32)
						}
					}
					retHi |= uint64(v) << (i * 16)
				}
			}
			ce.pushValue(retLo)
			ce.pushValue(retHi)
			frame.pc++
		case wazeroir.OperationKindV128Dot:
			x2Hi, x2Lo := ce.popValue(), ce.popValue()
			x1Hi, x1Lo := ce.popValue(), ce.popValue()
			ce.pushValue(
				uint64(uint32(int32(int16(x1Lo>>0))*int32(int16(x2Lo>>0))+int32(int16(x1Lo>>16))*int32(int16(x2Lo>>16)))) |
					(uint64(uint32(int32(int16(x1Lo>>32))*int32(int16(x2Lo>>32))+int32(int16(x1Lo>>48))*int32(int16(x2Lo>>48)))) << 32),
			)
			ce.pushValue(
				uint64(uint32(int32(int16(x1Hi>>0))*int32(int16(x2Hi>>0))+int32(int16(x1Hi>>16))*int32(int16(x2Hi>>16)))) |
					(uint64(uint32(int32(int16(x1Hi>>32))*int32(int16(x2Hi>>32))+int32(int16(x1Hi>>48))*int32(int16(x2Hi>>48)))) << 32),
			)
			frame.pc++
		case wazeroir.OperationKindV128ITruncSatFromF:
			hi, lo := ce.popValue(), ce.popValue()
			signed := op.b3
			var retLo, retHi uint64

			switch op.b1 {
			case wazeroir.ShapeF32x4: // f32x4 to i32x4
				for i, f64 := range [4]float64{
					math.Trunc(float64(math.Float32frombits(uint32(lo)))),
					math.Trunc(float64(math.Float32frombits(uint32(lo >> 32)))),
					math.Trunc(float64(math.Float32frombits(uint32(hi)))),
					math.Trunc(float64(math.Float32frombits(uint32(hi >> 32)))),
				} {

					var v uint32
					if math.IsNaN(f64) {
						v = 0
					} else if signed {
						if f64 < math.MinInt32 {
							f64 = math.MinInt32
						} else if f64 > math.MaxInt32 {
							f64 = math.MaxInt32
						}
						v = uint32(int32(f64))
					} else {
						if f64 < 0 {
							f64 = 0
						} else if f64 > math.MaxUint32 {
							f64 = math.MaxUint32
						}
						v = uint32(f64)
					}

					if i < 2 {
						retLo |= uint64(v) << (i * 32)
					} else {
						retHi |= uint64(v) << ((i - 2) * 32)
					}
				}

			case wazeroir.ShapeF64x2: // f64x2 to i32x4
				for i, f := range [2]float64{
					math.Trunc(math.Float64frombits(lo)),
					math.Trunc(math.Float64frombits(hi)),
				} {
					var v uint32
					if math.IsNaN(f) {
						v = 0
					} else if signed {
						if f < math.MinInt32 {
							f = math.MinInt32
						} else if f > math.MaxInt32 {
							f = math.MaxInt32
						}
						v = uint32(int32(f))
					} else {
						if f < 0 {
							f = 0
						} else if f > math.MaxUint32 {
							f = math.MaxUint32
						}
						v = uint32(f)
					}

					retLo |= uint64(v) << (i * 32)
				}
			}

			ce.pushValue(retLo)
			ce.pushValue(retHi)
			frame.pc++
		}
	}
	ce.popFrame()
}

// callerMemory returns the caller context memory.
func (ce *callEngine) callerMemory() *wasm.MemoryInstance {
	// Search through the call frame stack from the top until we find a non host function.
	for i := len(ce.frames) - 1; i >= 0; i-- {
		f := ce.frames[i].f
		if !f.parent.isHostFunction {
			return f.source.Module.Memory
		}
	}
	return nil
}

func WasmCompatMax32bits(v1, v2 uint32) uint64 {
	return uint64(math.Float32bits(moremath.WasmCompatMax32(
		math.Float32frombits(v1),
		math.Float32frombits(v2),
	)))
}

func WasmCompatMin32bits(v1, v2 uint32) uint64 {
	return uint64(math.Float32bits(moremath.WasmCompatMin32(
		math.Float32frombits(v1),
		math.Float32frombits(v2),
	)))
}

func addFloat32bits(v1, v2 uint32) uint64 {
	return uint64(math.Float32bits(math.Float32frombits(v1) + math.Float32frombits(v2)))
}

func subFloat32bits(v1, v2 uint32) uint64 {
	return uint64(math.Float32bits(math.Float32frombits(v1) - math.Float32frombits(v2)))
}

func mulFloat32bits(v1, v2 uint32) uint64 {
	return uint64(math.Float32bits(math.Float32frombits(v1) * math.Float32frombits(v2)))
}

func divFloat32bits(v1, v2 uint32) uint64 {
	return uint64(math.Float32bits(math.Float32frombits(v1) / math.Float32frombits(v2)))
}

// https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/exec/numerics.html#xref-exec-numerics-op-flt-mathrm-flt-n-z-1-z-2
func flt32(z1, z2 float32) bool {
	if z1 != z1 || z2 != z2 {
		return false
	} else if z1 == z2 {
		return false
	} else if math.IsInf(float64(z1), 1) {
		return false
	} else if math.IsInf(float64(z1), -1) {
		return true
	} else if math.IsInf(float64(z2), 1) {
		return true
	} else if math.IsInf(float64(z2), -1) {
		return false
	}
	return z1 < z2
}

// https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/exec/numerics.html#xref-exec-numerics-op-flt-mathrm-flt-n-z-1-z-2
func flt64(z1, z2 float64) bool {
	if z1 != z1 || z2 != z2 {
		return false
	} else if z1 == z2 {
		return false
	} else if math.IsInf(z1, 1) {
		return false
	} else if math.IsInf(z1, -1) {
		return true
	} else if math.IsInf(z2, 1) {
		return true
	} else if math.IsInf(z2, -1) {
		return false
	}
	return z1 < z2
}

func i8RoundingAverage(v1, v2 byte) byte {
	// https://github.com/WebAssembly/spec/blob/wg-2.0.draft1/proposals/simd/SIMD.md#lane-wise-integer-rounding-average
	return byte((uint16(v1) + uint16(v2) + uint16(1)) / 2)
}

func i16RoundingAverage(v1, v2 uint16) uint16 {
	// https://github.com/WebAssembly/spec/blob/wg-2.0.draft1/proposals/simd/SIMD.md#lane-wise-integer-rounding-average
	return uint16((uint32(v1) + uint32(v2) + 1) / 2)
}

func i8Abs(v byte) byte {
	if i := int8(v); i < 0 {
		return byte(-i)
	} else {
		return byte(i)
	}
}

func i8MaxU(v1, v2 byte) byte {
	if v1 < v2 {
		return v2
	} else {
		return v1
	}
}

func i8MinU(v1, v2 byte) byte {
	if v1 > v2 {
		return v2
	} else {
		return v1
	}
}

func i8MaxS(v1, v2 byte) byte {
	if int8(v1) < int8(v2) {
		return v2
	} else {
		return v1
	}
}

func i8MinS(v1, v2 byte) byte {
	if int8(v1) > int8(v2) {
		return v2
	} else {
		return v1
	}
}

func i16MaxU(v1, v2 uint16) uint16 {
	if v1 < v2 {
		return v2
	} else {
		return v1
	}
}

func i16MinU(v1, v2 uint16) uint16 {
	if v1 > v2 {
		return v2
	} else {
		return v1
	}
}

func i16MaxS(v1, v2 uint16) uint16 {
	if int16(v1) < int16(v2) {
		return v2
	} else {
		return v1
	}
}

func i16MinS(v1, v2 uint16) uint16 {
	if int16(v1) > int16(v2) {
		return v2
	} else {
		return v1
	}
}

func i32MaxU(v1, v2 uint32) uint32 {
	if v1 < v2 {
		return v2
	} else {
		return v1
	}
}

func i32MinU(v1, v2 uint32) uint32 {
	if v1 > v2 {
		return v2
	} else {
		return v1
	}
}

func i32MaxS(v1, v2 uint32) uint32 {
	if int32(v1) < int32(v2) {
		return v2
	} else {
		return v1
	}
}

func i32MinS(v1, v2 uint32) uint32 {
	if int32(v1) > int32(v2) {
		return v2
	} else {
		return v1
	}
}

func i16Abs(v uint16) uint16 {
	if i := int16(v); i < 0 {
		return uint16(-i)
	} else {
		return uint16(i)
	}
}

func i32Abs(v uint32) uint32 {
	if i := int32(v); i < 0 {
		return uint32(-i)
	} else {
		return uint32(i)
	}
}

func (ce *callEngine) callNativeFuncWithListener(ctx context.Context, callCtx *wasm.CallContext, f *function, fnl experimental.FunctionListener) context.Context {
	if f.parent.isHostFunction {
		callCtx = callCtx.WithMemory(ce.callerMemory())
	}
	ctx = fnl.Before(ctx, callCtx, f.source.Definition, ce.peekValues(len(f.source.Type.Params)))
	ce.callNativeFunc(ctx, callCtx, f)
	// TODO: This doesn't get the error due to use of panic to propagate them.
	fnl.After(ctx, callCtx, f.source.Definition, nil, ce.peekValues(len(f.source.Type.Results)))
	return ctx
}

// popMemoryOffset takes a memory offset off the stack for use in load and store instructions.
// As the top of stack value is 64-bit, this ensures it is in range before returning it.
func (ce *callEngine) popMemoryOffset(op *interpreterOp) uint32 {
	// TODO: Document what 'us' is and why we expect to look at value 1.
	offset := op.us[1] + ce.popValue()
	if offset > math.MaxUint32 {
		panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
	}
	return uint32(offset)
}

func (ce *callEngine) callGoFuncWithStack(ctx context.Context, callCtx *wasm.CallContext, f *function) {
	paramLen := f.source.Type.ParamNumInUint64
	resultLen := f.source.Type.ResultNumInUint64
	stackLen := paramLen

	// In the interpreter engine, ce.stack may only have capacity to store
	// parameters. Grow when there are more results than parameters.
	if growLen := resultLen - paramLen; growLen > 0 {
		for i := 0; i < growLen; i++ {
			ce.stack = append(ce.stack, 0)
		}
		stackLen += growLen
	}

	// Pass the stack elements to the go function.
	stack := ce.stack[len(ce.stack)-stackLen:]
	ce.callGoFunc(ctx, callCtx, f, stack)

	// Shrink the stack when there were more parameters than results.
	if shrinkLen := paramLen - resultLen; shrinkLen > 0 {
		ce.stack = ce.stack[0 : len(ce.stack)-shrinkLen]
	}
}
