package wazeroir

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/leb128"
	"github.com/tetratelabs/wazero/internal/wasm"
)

type controlFrameKind byte

const (
	controlFrameKindBlockWithContinuationLabel controlFrameKind = iota
	controlFrameKindBlockWithoutContinuationLabel
	controlFrameKindFunction
	controlFrameKindLoop
	controlFrameKindIfWithElse
	controlFrameKindIfWithoutElse
)

type (
	controlFrame struct {
		frameID uint32
		// originalStackLen holds the number of values on the stack
		// when start executing this control frame minus params for the block.
		originalStackLenWithoutParam int
		blockType                    *wasm.FunctionType
		kind                         controlFrameKind
	}
	controlFrames struct{ frames []controlFrame }
)

func (c *controlFrame) ensureContinuation() {
	// Make sure that if the frame is block and doesn't have continuation,
	// change the kind so we can emit the continuation block
	// later when we reach the end instruction of this frame.
	if c.kind == controlFrameKindBlockWithoutContinuationLabel {
		c.kind = controlFrameKindBlockWithContinuationLabel
	}
}

func (c *controlFrame) asLabel() Label {
	switch c.kind {
	case controlFrameKindBlockWithContinuationLabel,
		controlFrameKindBlockWithoutContinuationLabel:
		return Label{FrameID: c.frameID, Kind: LabelKindContinuation}
	case controlFrameKindLoop:
		return Label{FrameID: c.frameID, Kind: LabelKindHeader}
	case controlFrameKindFunction:
		return Label{Kind: LabelKindReturn}
	case controlFrameKindIfWithElse,
		controlFrameKindIfWithoutElse:
		return Label{FrameID: c.frameID, Kind: LabelKindContinuation}
	}
	panic(fmt.Sprintf("unreachable: a bug in wazeroir implementation: %v", c.kind))
}

func (c *controlFrames) functionFrame() *controlFrame {
	// No need to check stack bound
	// as we can assume that all the operations
	// are valid thanks to validateFunction
	// at module validation phase.
	return &c.frames[0]
}

func (c *controlFrames) get(n int) *controlFrame {
	// No need to check stack bound
	// as we can assume that all the operations
	// are valid thanks to validateFunction
	// at module validation phase.
	return &c.frames[len(c.frames)-n-1]
}

func (c *controlFrames) top() *controlFrame {
	// No need to check stack bound
	// as we can assume that all the operations
	// are valid thanks to validateFunction
	// at module validation phase.
	return &c.frames[len(c.frames)-1]
}

func (c *controlFrames) empty() bool {
	return len(c.frames) == 0
}

func (c *controlFrames) pop() (frame *controlFrame) {
	// No need to check stack bound
	// as we can assume that all the operations
	// are valid thanks to validateFunction
	// at module validation phase.
	frame = c.top()
	c.frames = c.frames[:len(c.frames)-1]
	return
}

func (c *controlFrames) push(frame controlFrame) {
	c.frames = append(c.frames, frame)
}

func (c *compiler) initializeStack() {
	c.localIndexToStackHeightInUint64 = make(map[uint32]int, len(c.sig.Params)+len(c.localTypes))
	var current int
	for index, lt := range c.sig.Params {
		c.localIndexToStackHeightInUint64[wasm.Index(index)] = current
		if lt == wasm.ValueTypeV128 {
			current++
		}
		current++
	}

	if c.callFrameStackSizeInUint64 > 0 {
		// We reserve the stack slots for result values below the return call frame slots.
		if diff := c.sig.ResultNumInUint64 - c.sig.ParamNumInUint64; diff > 0 {
			current += diff
		}
	}

	// Non-func param locals start after the return call frame.
	current += c.callFrameStackSizeInUint64

	for index, lt := range c.localTypes {
		index += len(c.sig.Params)
		c.localIndexToStackHeightInUint64[wasm.Index(index)] = current
		if lt == wasm.ValueTypeV128 {
			current++
		}
		current++
	}

	// Push function arguments.
	for _, t := range c.sig.Params {
		c.stackPush(wasmValueTypeToUnsignedType(t))
	}

	if c.callFrameStackSizeInUint64 > 0 {
		// Reserve the stack slots for results.
		for i := 0; i < c.sig.ResultNumInUint64-c.sig.ParamNumInUint64; i++ {
			c.stackPush(UnsignedTypeI64)
		}

		// Reserve the stack slots for call frame.
		for i := 0; i < c.callFrameStackSizeInUint64; i++ {
			c.stackPush(UnsignedTypeI64)
		}
	}
}

type compiler struct {
	enabledFeatures            api.CoreFeatures
	callFrameStackSizeInUint64 int
	stack                      []UnsignedType
	currentID                  uint32
	controlFrames              *controlFrames
	unreachableState           struct {
		on    bool
		depth int
	}
	pc, currentOpPC uint64
	result          CompilationResult

	// body holds the code for the function's body where Wasm instructions are stored.
	body []byte
	// sig is the function type of the target function.
	sig *wasm.FunctionType
	// localTypes holds the target function locals' value types except function params.
	localTypes []wasm.ValueType
	// localIndexToStackHeightInUint64 maps the local index (starting with function params) to the stack height
	// where the local is places. This is the necessary mapping for functions who contain vector type locals.
	localIndexToStackHeightInUint64 map[wasm.Index]int

	// types hold all the function types in the module where the targe function exists.
	types []*wasm.FunctionType
	// funcs holds the type indexes for all declard functions in the module where the targe function exists.
	funcs []uint32
	// globals holds the global types for all declard globas in the module where the targe function exists.
	globals []*wasm.GlobalType

	// needSourceOffset is true if this module requires DWARF based stack trace.
	needSourceOffset bool
	// bodyOffsetInCodeSection is the offset of the body of this function in the original Wasm binary's code section.
	bodyOffsetInCodeSection uint64

	ensureTermination bool
	// Pre-allocated bytes.Reader to be used in varous places.
	br             *bytes.Reader
	funcTypeToSigs *funcTypeToIRSignatures
}

//lint:ignore U1000 for debugging only.
func (c *compiler) stackDump() string {
	strs := make([]string, 0, len(c.stack))
	for _, s := range c.stack {
		strs = append(strs, s.String())
	}
	return "[" + strings.Join(strs, ", ") + "]"
}

func (c *compiler) markUnreachable() {
	c.unreachableState.on = true
}

func (c *compiler) resetUnreachable() {
	c.unreachableState.on = false
}

type CompilationResult struct {
	// IsHostFunction is the data returned by the same field documented on
	// wasm.Code.
	IsHostFunction bool

	// GoFunc is the data returned by the same field documented on wasm.Code.
	// In this case, IsHostFunction is true and other fields can be ignored.
	GoFunc interface{}

	// Operations holds wazeroir operations compiled from Wasm instructions in a Wasm function.
	Operations []Operation

	// IROperationSourceOffsetsInWasmBinary is index-correlated with Operation and maps each operation to the corresponding source instruction's
	// offset in the original WebAssembly binary.
	// Non nil only when the given Wasm module has the DWARF section.
	IROperationSourceOffsetsInWasmBinary []uint64

	// LabelCallers maps Label.String() to the number of callers to that label.
	// Here "callers" means that the call-sites which jumps to the label with br, br_if or br_table
	// instructions.
	//
	// Note: zero possible and allowed in wasm. e.g.
	//
	//	(block
	//	  (br 0)
	//	  (block i32.const 1111)
	//	)
	//
	// This example the label corresponding to `(block i32.const 1111)` is never be reached at runtime because `br 0` exits the function before we reach there
	LabelCallers map[LabelID]uint32

	// Signature is the function type of the compilation target function.
	Signature *wasm.FunctionType
	// Globals holds all the declarations of globals in the module from which this function is compiled.
	Globals []*wasm.GlobalType
	// Functions holds all the declarations of function in the module from which this function is compiled, including itself.
	Functions []wasm.Index
	// Types holds all the types in the module from which this function is compiled.
	Types []*wasm.FunctionType
	// TableTypes holds all the reference types of all tables declared in the module.
	TableTypes []wasm.ValueType
	// HasMemory is true if the module from which this function is compiled has memory declaration.
	HasMemory bool
	// UsesMemory is true if this function might use memory.
	UsesMemory bool
	// HasTable is true if the module from which this function is compiled has table declaration.
	HasTable bool
	// HasDataInstances is true if the module has data instances which might be used by memory.init or data.drop instructions.
	HasDataInstances bool
	// HasDataInstances is true if the module has element instances which might be used by table.init or elem.drop instructions.
	HasElementInstances bool
	EnsureTermination   bool
}

func CompileFunctions(enabledFeatures api.CoreFeatures, callFrameStackSizeInUint64 int, module *wasm.Module, ensureTermination bool) ([]*CompilationResult, error) {
	functions, globals, mem, tables, err := module.AllDeclarations()
	if err != nil {
		return nil, err
	}

	hasMemory, hasTable, hasDataInstances, hasElementInstances := mem != nil, len(tables) > 0,
		len(module.DataSection) > 0, len(module.ElementSection) > 0

	tableTypes := make([]wasm.ValueType, len(tables))
	for i := range tableTypes {
		tableTypes[i] = tables[i].Type
	}

	types := module.TypeSection

	funcTypeToSigs := &funcTypeToIRSignatures{
		indirectCalls: make([]*signature, len(types)),
		directCalls:   make([]*signature, len(types)),
		wasmTypes:     types,
	}

	controlFramesStack := &controlFrames{}
	var ret []*CompilationResult
	for funcIndex := range module.FunctionSection {
		typeID := module.FunctionSection[funcIndex]
		sig := types[typeID]
		code := module.CodeSection[funcIndex]
		if code.GoFunc != nil {
			// Assume the function might use memory if it has a parameter for the api.Module
			_, usesMemory := code.GoFunc.(api.GoModuleFunction)

			ret = append(ret, &CompilationResult{
				IsHostFunction: true,
				UsesMemory:     usesMemory,
				GoFunc:         code.GoFunc,
				Signature:      sig,
			})
			continue
		}
		r, err := compile(enabledFeatures, callFrameStackSizeInUint64, sig, code.Body,
			code.LocalTypes, types, functions, globals, code.BodyOffsetInCodeSection,
			module.DWARFLines != nil, ensureTermination, funcTypeToSigs, controlFramesStack)
		if err != nil {
			def := module.FunctionDefinitionSection[uint32(funcIndex)+module.ImportFuncCount()]
			return nil, fmt.Errorf("failed to lower func[%s] to wazeroir: %w", def.DebugName(), err)
		}
		r.IsHostFunction = code.IsHostFunction
		r.Globals = globals
		r.Functions = functions
		r.Types = types
		r.HasMemory = hasMemory
		r.HasTable = hasTable
		r.HasDataInstances = hasDataInstances
		r.HasElementInstances = hasElementInstances
		r.Signature = sig
		r.TableTypes = tableTypes
		r.EnsureTermination = ensureTermination
		ret = append(ret, r)

		// We reuse the stack to reduce allocations, so reset the length here.
		controlFramesStack.frames = controlFramesStack.frames[:0]
	}
	return ret, nil
}

// Compile lowers given function instance into wazeroir operations
// so that the resulting operations can be consumed by the interpreter
// or the Compiler compilation engine.
func compile(enabledFeatures api.CoreFeatures,
	callFrameStackSizeInUint64 int,
	sig *wasm.FunctionType,
	body []byte,
	localTypes []wasm.ValueType,
	types []*wasm.FunctionType,
	functions []uint32, globals []*wasm.GlobalType,
	bodyOffsetInCodeSection uint64,
	needSourceOffset bool,
	ensureTermination bool,
	funcTypeToSigs *funcTypeToIRSignatures,
	controlFramesStack *controlFrames,
) (*CompilationResult, error) {
	c := compiler{
		enabledFeatures:            enabledFeatures,
		controlFrames:              controlFramesStack,
		callFrameStackSizeInUint64: callFrameStackSizeInUint64,
		result:                     CompilationResult{LabelCallers: map[LabelID]uint32{}},
		body:                       body,
		localTypes:                 localTypes,
		sig:                        sig,
		globals:                    globals,
		funcs:                      functions,
		types:                      types,
		needSourceOffset:           needSourceOffset,
		bodyOffsetInCodeSection:    bodyOffsetInCodeSection,
		ensureTermination:          ensureTermination,
		br:                         bytes.NewReader(nil),
		funcTypeToSigs:             funcTypeToSigs,
	}

	c.initializeStack()

	// Emit const expressions for locals.
	// Note that here we don't take function arguments
	// into account, meaning that callers must push
	// arguments before entering into the function body.
	for _, t := range localTypes {
		c.emitDefaultValue(t)
	}

	// Insert the function control frame.
	c.controlFrames.push(controlFrame{
		frameID:   c.nextID(),
		blockType: c.sig,
		kind:      controlFrameKindFunction,
	})

	if c.ensureTermination {
		c.emit(OperationBuiltinFunctionCheckExitCode{})
	}

	// Now, enter the function body.
	for !c.controlFrames.empty() && c.pc < uint64(len(c.body)) {
		if err := c.handleInstruction(); err != nil {
			return nil, fmt.Errorf("handling instruction: %w", err)
		}
	}
	return &c.result, nil
}

// Translate the current Wasm instruction to wazeroir's operations,
// and emit the results into c.results.
func (c *compiler) handleInstruction() error {
	op := c.body[c.pc]
	c.currentOpPC = c.pc
	if false {
		var instName string
		if op == wasm.OpcodeVecPrefix {
			instName = wasm.VectorInstructionName(c.body[c.pc+1])
		} else if op == wasm.OpcodeMiscPrefix {
			instName = wasm.MiscInstructionName(c.body[c.pc+1])
		} else {
			instName = wasm.InstructionName(op)
		}
		fmt.Printf("handling %s, unreachable_state(on=%v,depth=%d), stack=%v\n",
			instName, c.unreachableState.on, c.unreachableState.depth, c.stack,
		)
	}

	var peekValueType UnsignedType
	if len(c.stack) > 0 {
		peekValueType = c.stackPeek()
	}

	// Modify the stack according the current instruction.
	// Note that some instructions will read "index" in
	// applyToStack and advance c.pc inside the function.
	index, err := c.applyToStack(op)
	if err != nil {
		return fmt.Errorf("apply stack failed for %s: %w", wasm.InstructionName(op), err)
	}
	// Now we handle each instruction, and
	// emit the corresponding wazeroir operations to the results.
operatorSwitch:
	switch op {
	case wasm.OpcodeUnreachable:
		c.emit(OperationUnreachable{})
		c.markUnreachable()
	case wasm.OpcodeNop:
		// Nop is noop!
	case wasm.OpcodeBlock:
		c.br.Reset(c.body[c.pc+1:])
		bt, num, err := wasm.DecodeBlockType(c.types, c.br, c.enabledFeatures)
		if err != nil {
			return fmt.Errorf("reading block type for block instruction: %w", err)
		}
		c.pc += num

		if c.unreachableState.on {
			// If it is currently in unreachable,
			// just remove the entire block.
			c.unreachableState.depth++
			break operatorSwitch
		}

		// Create a new frame -- entering this block.
		frame := controlFrame{
			frameID:                      c.nextID(),
			originalStackLenWithoutParam: len(c.stack) - len(bt.Params),
			kind:                         controlFrameKindBlockWithoutContinuationLabel,
			blockType:                    bt,
		}
		c.controlFrames.push(frame)

	case wasm.OpcodeLoop:
		c.br.Reset(c.body[c.pc+1:])
		bt, num, err := wasm.DecodeBlockType(c.types, c.br, c.enabledFeatures)
		if err != nil {
			return fmt.Errorf("reading block type for loop instruction: %w", err)
		}
		c.pc += num

		if c.unreachableState.on {
			// If it is currently in unreachable,
			// just remove the entire block.
			c.unreachableState.depth++
			break operatorSwitch
		}

		// Create a new frame -- entering loop.
		frame := controlFrame{
			frameID:                      c.nextID(),
			originalStackLenWithoutParam: len(c.stack) - len(bt.Params),
			kind:                         controlFrameKindLoop,
			blockType:                    bt,
		}
		c.controlFrames.push(frame)

		// Prep labels for inside and the continuation of this loop.
		loopLabel := Label{FrameID: frame.frameID, Kind: LabelKindHeader}
		c.result.LabelCallers[loopLabel.ID()]++

		// Emit the branch operation to enter inside the loop.
		c.emit(
			OperationBr{
				Target: loopLabel,
			},
			OperationLabel{Label: loopLabel},
		)

		// Insert the exit code check on the loop header, which is the only necessary point in the function body
		// to prevent infinite loop.
		//
		// Note that this is a little aggressive: this checks the exit code regardless the loop header is actually
		// the loop. In other words, this checks even when no br/br_if/br_table instructions jumping to this loop
		// exist. However, in reality, that shouldn't be an issue since such "noop" loop header will highly likely be
		// optimized out by almost all guest language compilers which have the control flow optimization passes.
		if c.ensureTermination {
			c.emit(OperationBuiltinFunctionCheckExitCode{})
		}
	case wasm.OpcodeIf:
		c.br.Reset(c.body[c.pc+1:])
		bt, num, err := wasm.DecodeBlockType(c.types, c.br, c.enabledFeatures)
		if err != nil {
			return fmt.Errorf("reading block type for if instruction: %w", err)
		}
		c.pc += num

		if c.unreachableState.on {
			// If it is currently in unreachable,
			// just remove the entire block.
			c.unreachableState.depth++
			break operatorSwitch
		}

		// Create a new frame -- entering if.
		frame := controlFrame{
			frameID:                      c.nextID(),
			originalStackLenWithoutParam: len(c.stack) - len(bt.Params),
			// Note this will be set to controlFrameKindIfWithElse
			// when else opcode found later.
			kind:      controlFrameKindIfWithoutElse,
			blockType: bt,
		}
		c.controlFrames.push(frame)

		// Prep labels for if and else of this if.
		thenLabel := Label{Kind: LabelKindHeader, FrameID: frame.frameID}
		elseLabel := Label{Kind: LabelKindElse, FrameID: frame.frameID}
		c.result.LabelCallers[thenLabel.ID()]++
		c.result.LabelCallers[elseLabel.ID()]++

		// Emit the branch operation to enter the then block.
		c.emit(
			OperationBrIf{
				Then: thenLabel.asBranchTargetDrop(),
				Else: elseLabel.asBranchTargetDrop(),
			},
			OperationLabel{
				Label: thenLabel,
			},
		)
	case wasm.OpcodeElse:
		frame := c.controlFrames.top()
		if c.unreachableState.on && c.unreachableState.depth > 0 {
			// If it is currently in unreachable, and the nested if,
			// just remove the entire else block.
			break operatorSwitch
		} else if c.unreachableState.on {
			// If it is currently in unreachable, and the non-nested if,
			// reset the stack so we can correctly handle the else block.
			top := c.controlFrames.top()
			c.stack = c.stack[:top.originalStackLenWithoutParam]
			top.kind = controlFrameKindIfWithElse

			// Re-push the parameters to the if block so that else block can use them.
			for _, t := range frame.blockType.Params {
				c.stackPush(wasmValueTypeToUnsignedType(t))
			}

			// We are no longer unreachable in else frame,
			// so emit the correct label, and reset the unreachable state.
			elseLabel := Label{FrameID: frame.frameID, Kind: LabelKindElse}
			c.resetUnreachable()
			c.emit(
				OperationLabel{Label: elseLabel},
			)
			break operatorSwitch
		}

		// Change the kind of this If block, indicating that
		// the if has else block.
		frame.kind = controlFrameKindIfWithElse

		// We need to reset the stack so that
		// the values pushed inside the then block
		// do not affect the else block.
		dropOp := OperationDrop{Depth: c.getFrameDropRange(frame, false)}

		// Reset the stack manipulated by the then block, and re-push the block param types to the stack.

		c.stack = c.stack[:frame.originalStackLenWithoutParam]
		for _, t := range frame.blockType.Params {
			c.stackPush(wasmValueTypeToUnsignedType(t))
		}

		// Prep labels for else and the continuation of this if block.
		elseLabel := Label{FrameID: frame.frameID, Kind: LabelKindElse}
		continuationLabel := Label{FrameID: frame.frameID, Kind: LabelKindContinuation}
		c.result.LabelCallers[continuationLabel.ID()]++

		// Emit the instructions for exiting the if loop,
		// and then the initiation of else block.
		c.emit(
			dropOp,
			// Jump to the continuation of this block.
			OperationBr{Target: continuationLabel},
			// Initiate the else block.
			OperationLabel{Label: elseLabel},
		)
	case wasm.OpcodeEnd:
		if c.unreachableState.on && c.unreachableState.depth > 0 {
			c.unreachableState.depth--
			break operatorSwitch
		} else if c.unreachableState.on {
			c.resetUnreachable()

			frame := c.controlFrames.pop()
			if c.controlFrames.empty() {
				return nil
			}

			c.stack = c.stack[:frame.originalStackLenWithoutParam]
			for _, t := range frame.blockType.Results {
				c.stackPush(wasmValueTypeToUnsignedType(t))
			}

			continuationLabel := Label{FrameID: frame.frameID, Kind: LabelKindContinuation}
			if frame.kind == controlFrameKindIfWithoutElse {
				// Emit the else label.
				elseLabel := Label{Kind: LabelKindElse, FrameID: frame.frameID}
				c.result.LabelCallers[continuationLabel.ID()]++
				c.emit(
					OperationLabel{Label: elseLabel},
					OperationBr{Target: continuationLabel},
					OperationLabel{Label: continuationLabel},
				)
			} else {
				c.emit(
					OperationLabel{Label: continuationLabel},
				)
			}

			break operatorSwitch
		}

		frame := c.controlFrames.pop()

		// We need to reset the stack so that
		// the values pushed inside the block.
		dropOp := OperationDrop{Depth: c.getFrameDropRange(frame, true)}
		c.stack = c.stack[:frame.originalStackLenWithoutParam]

		// Push the result types onto the stack.
		for _, t := range frame.blockType.Results {
			c.stackPush(wasmValueTypeToUnsignedType(t))
		}

		// Emit the instructions according to the kind of the current control frame.
		switch frame.kind {
		case controlFrameKindFunction:
			if !c.controlFrames.empty() {
				// Should never happen. If so, there's a bug in the translation.
				panic("bug: found more function control frames")
			}
			// Return from function.
			c.emit(
				dropOp,
				OperationBr{Target: Label{Kind: LabelKindReturn}},
			)
		case controlFrameKindIfWithoutElse:
			// This case we have to emit "empty" else label.
			elseLabel := Label{Kind: LabelKindElse, FrameID: frame.frameID}
			continuationLabel := Label{Kind: LabelKindContinuation, FrameID: frame.frameID}
			c.result.LabelCallers[continuationLabel.ID()] += 2
			c.emit(
				dropOp,
				OperationBr{Target: continuationLabel},
				// Emit the else which soon branches into the continuation.
				OperationLabel{Label: elseLabel},
				OperationBr{Target: continuationLabel},
				// Initiate the continuation.
				OperationLabel{Label: continuationLabel},
			)
		case controlFrameKindBlockWithContinuationLabel,
			controlFrameKindIfWithElse:
			continuationLabel := Label{Kind: LabelKindContinuation, FrameID: frame.frameID}
			c.result.LabelCallers[continuationLabel.ID()]++
			c.emit(
				dropOp,
				OperationBr{Target: continuationLabel},
				OperationLabel{Label: continuationLabel},
			)
		case controlFrameKindLoop, controlFrameKindBlockWithoutContinuationLabel:
			c.emit(
				dropOp,
			)
		default:
			// Should never happen. If so, there's a bug in the translation.
			panic(fmt.Errorf("bug: invalid control frame kind: 0x%x", frame.kind))
		}

	case wasm.OpcodeBr:
		targetIndex, n, err := leb128.LoadUint32(c.body[c.pc+1:])
		if err != nil {
			return fmt.Errorf("read the target for br_if: %w", err)
		}
		c.pc += n

		if c.unreachableState.on {
			// If it is currently in unreachable, br is no-op.
			break operatorSwitch
		}

		targetFrame := c.controlFrames.get(int(targetIndex))
		targetFrame.ensureContinuation()
		dropOp := OperationDrop{Depth: c.getFrameDropRange(targetFrame, false)}
		target := targetFrame.asLabel()
		c.result.LabelCallers[target.ID()]++
		c.emit(
			dropOp,
			OperationBr{Target: target},
		)
		// Br operation is stack-polymorphic, and mark the state as unreachable.
		// That means subsequent instructions in the current control frame are "unreachable"
		// and can be safely removed.
		c.markUnreachable()
	case wasm.OpcodeBrIf:
		targetIndex, n, err := leb128.LoadUint32(c.body[c.pc+1:])
		if err != nil {
			return fmt.Errorf("read the target for br_if: %w", err)
		}
		c.pc += n

		if c.unreachableState.on {
			// If it is currently in unreachable, br-if is no-op.
			break operatorSwitch
		}

		targetFrame := c.controlFrames.get(int(targetIndex))
		targetFrame.ensureContinuation()
		drop := c.getFrameDropRange(targetFrame, false)
		target := targetFrame.asLabel()
		c.result.LabelCallers[target.ID()]++

		continuationLabel := Label{FrameID: c.nextID(), Kind: LabelKindHeader}
		c.result.LabelCallers[continuationLabel.ID()]++
		c.emit(
			OperationBrIf{
				Then: BranchTargetDrop{ToDrop: drop, Target: target},
				Else: continuationLabel.asBranchTargetDrop(),
			},
			// Start emitting else block operations.
			OperationLabel{
				Label: continuationLabel,
			},
		)
	case wasm.OpcodeBrTable:
		c.br.Reset(c.body[c.pc+1:])
		r := c.br
		numTargets, n, err := leb128.DecodeUint32(r)
		if err != nil {
			return fmt.Errorf("error reading number of targets in br_table: %w", err)
		}
		c.pc += n

		if c.unreachableState.on {
			// If it is currently in unreachable, br_table is no-op.
			// But before proceeding to the next instruction, we must advance the pc
			// according to the number of br_table targets.
			for i := uint32(0); i <= numTargets; i++ { // inclusive as we also need to read the index of default target.
				_, n, err := leb128.DecodeUint32(r)
				if err != nil {
					return fmt.Errorf("error reading target %d in br_table: %w", i, err)
				}
				c.pc += n
			}
			break operatorSwitch
		}

		// Read the branch targets.
		targets := make([]*BranchTargetDrop, numTargets)
		for i := range targets {
			l, n, err := leb128.DecodeUint32(r)
			if err != nil {
				return fmt.Errorf("error reading target %d in br_table: %w", i, err)
			}
			c.pc += n
			targetFrame := c.controlFrames.get(int(l))
			targetFrame.ensureContinuation()
			drop := c.getFrameDropRange(targetFrame, false)
			target := &BranchTargetDrop{ToDrop: drop, Target: targetFrame.asLabel()}
			targets[i] = target
			c.result.LabelCallers[target.Target.ID()]++
		}

		// Prep default target control frame.
		l, n, err := leb128.DecodeUint32(r)
		if err != nil {
			return fmt.Errorf("error reading default target of br_table: %w", err)
		}
		c.pc += n
		defaultTargetFrame := c.controlFrames.get(int(l))
		defaultTargetFrame.ensureContinuation()
		defaultTargetDrop := c.getFrameDropRange(defaultTargetFrame, false)
		defaultTarget := defaultTargetFrame.asLabel()
		c.result.LabelCallers[defaultTarget.ID()]++

		c.emit(
			OperationBrTable{
				Targets: targets,
				Default: &BranchTargetDrop{
					ToDrop: defaultTargetDrop, Target: defaultTarget,
				},
			},
		)
		// Br operation is stack-polymorphic, and mark the state as unreachable.
		// That means subsequent instructions in the current control frame are "unreachable"
		// and can be safely removed.
		c.markUnreachable()
	case wasm.OpcodeReturn:
		functionFrame := c.controlFrames.functionFrame()
		dropOp := OperationDrop{Depth: c.getFrameDropRange(functionFrame, false)}

		// Cleanup the stack and then jmp to function frame's continuation (meaning return).
		c.emit(
			dropOp,
			OperationBr{Target: functionFrame.asLabel()},
		)

		// Return operation is stack-polymorphic, and mark the state as unreachable.
		// That means subsequent instructions in the current control frame are "unreachable"
		// and can be safely removed.
		c.markUnreachable()
	case wasm.OpcodeCall:
		c.emit(
			OperationCall{FunctionIndex: index},
		)
	case wasm.OpcodeCallIndirect:
		tableIndex, n, err := leb128.LoadUint32(c.body[c.pc+1:])
		if err != nil {
			return fmt.Errorf("read target for br_table: %w", err)
		}
		c.pc += n
		c.emit(
			OperationCallIndirect{TypeIndex: index, TableIndex: tableIndex},
		)
	case wasm.OpcodeDrop:
		r := &InclusiveRange{Start: 0, End: 0}
		if peekValueType == UnsignedTypeV128 {
			// InclusiveRange is the range in uint64 representation, so dropping a vector value on top
			// should be translated as drop [0..1] inclusively.
			r.End++
		}
		c.emit(
			OperationDrop{Depth: r},
		)
	case wasm.OpcodeSelect:
		// If it is on the unreachable state, ignore the instruction.
		if c.unreachableState.on {
			break operatorSwitch
		}
		c.emit(
			OperationSelect{IsTargetVector: c.stackPeek() == UnsignedTypeV128},
		)
	case wasm.OpcodeTypedSelect:
		// Skips two bytes: vector size fixed to 1, and the value type for select.
		c.pc += 2
		// If it is on the unreachable state, ignore the instruction.
		if c.unreachableState.on {
			break operatorSwitch
		}
		// Typed select is semantically equivalent to select at runtime.
		c.emit(
			OperationSelect{IsTargetVector: c.stackPeek() == UnsignedTypeV128},
		)
	case wasm.OpcodeLocalGet:
		depth := c.localDepth(index)
		if isVector := c.localType(index) == wasm.ValueTypeV128; !isVector {
			c.emit(
				// -1 because we already manipulated the stack before
				// called localDepth ^^.
				OperationPick{Depth: depth - 1, IsTargetVector: isVector},
			)
		} else {
			c.emit(
				// -2 because we already manipulated the stack before
				// called localDepth ^^.
				OperationPick{Depth: depth - 2, IsTargetVector: isVector},
			)
		}
	case wasm.OpcodeLocalSet:
		depth := c.localDepth(index)

		isVector := c.localType(index) == wasm.ValueTypeV128
		if isVector {
			c.emit(
				// +2 because we already popped the operands for this operation from the c.stack before
				// called localDepth ^^,
				OperationSet{Depth: depth + 2, IsTargetVector: isVector},
			)
		} else {
			c.emit(
				// +1 because we already popped the operands for this operation from the c.stack before
				// called localDepth ^^,
				OperationSet{Depth: depth + 1, IsTargetVector: isVector},
			)
		}
	case wasm.OpcodeLocalTee:
		depth := c.localDepth(index)
		isVector := c.localType(index) == wasm.ValueTypeV128
		if isVector {
			c.emit(
				OperationPick{Depth: 1, IsTargetVector: isVector},
				OperationSet{Depth: depth + 2, IsTargetVector: isVector},
			)
		} else {
			c.emit(
				OperationPick{Depth: 0, IsTargetVector: isVector},
				OperationSet{Depth: depth + 1, IsTargetVector: isVector},
			)
		}
	case wasm.OpcodeGlobalGet:
		c.emit(
			OperationGlobalGet{Index: index},
		)
	case wasm.OpcodeGlobalSet:
		c.emit(
			OperationGlobalSet{Index: index},
		)
	case wasm.OpcodeI32Load:
		imm, err := c.readMemoryArg(wasm.OpcodeI32LoadName)
		if err != nil {
			return err
		}
		c.emit(OperationLoad{Type: UnsignedTypeI32, Arg: imm})
	case wasm.OpcodeI64Load:
		imm, err := c.readMemoryArg(wasm.OpcodeI64LoadName)
		if err != nil {
			return err
		}
		c.emit(OperationLoad{Type: UnsignedTypeI64, Arg: imm})
	case wasm.OpcodeF32Load:
		imm, err := c.readMemoryArg(wasm.OpcodeF32LoadName)
		if err != nil {
			return err
		}
		c.emit(OperationLoad{Type: UnsignedTypeF32, Arg: imm})
	case wasm.OpcodeF64Load:
		imm, err := c.readMemoryArg(wasm.OpcodeF64LoadName)
		if err != nil {
			return err
		}
		c.emit(OperationLoad{Type: UnsignedTypeF64, Arg: imm})
	case wasm.OpcodeI32Load8S:
		imm, err := c.readMemoryArg(wasm.OpcodeI32Load8SName)
		if err != nil {
			return err
		}
		c.emit(OperationLoad8{Type: SignedInt32, Arg: imm})
	case wasm.OpcodeI32Load8U:
		imm, err := c.readMemoryArg(wasm.OpcodeI32Load8UName)
		if err != nil {
			return err
		}
		c.emit(OperationLoad8{Type: SignedUint32, Arg: imm})
	case wasm.OpcodeI32Load16S:
		imm, err := c.readMemoryArg(wasm.OpcodeI32Load16SName)
		if err != nil {
			return err
		}
		c.emit(OperationLoad16{Type: SignedInt32, Arg: imm})
	case wasm.OpcodeI32Load16U:
		imm, err := c.readMemoryArg(wasm.OpcodeI32Load16UName)
		if err != nil {
			return err
		}
		c.emit(
			OperationLoad16{Type: SignedUint32, Arg: imm},
		)
	case wasm.OpcodeI64Load8S:
		imm, err := c.readMemoryArg(wasm.OpcodeI64Load8SName)
		if err != nil {
			return err
		}
		c.emit(
			OperationLoad8{Type: SignedInt64, Arg: imm},
		)
	case wasm.OpcodeI64Load8U:
		imm, err := c.readMemoryArg(wasm.OpcodeI64Load8UName)
		if err != nil {
			return err
		}
		c.emit(
			OperationLoad8{Type: SignedUint64, Arg: imm},
		)
	case wasm.OpcodeI64Load16S:
		imm, err := c.readMemoryArg(wasm.OpcodeI64Load16SName)
		if err != nil {
			return err
		}
		c.emit(
			OperationLoad16{Type: SignedInt64, Arg: imm},
		)
	case wasm.OpcodeI64Load16U:
		imm, err := c.readMemoryArg(wasm.OpcodeI64Load16UName)
		if err != nil {
			return err
		}
		c.emit(
			OperationLoad16{Type: SignedUint64, Arg: imm},
		)
	case wasm.OpcodeI64Load32S:
		imm, err := c.readMemoryArg(wasm.OpcodeI64Load32SName)
		if err != nil {
			return err
		}
		c.emit(
			OperationLoad32{Signed: true, Arg: imm},
		)
	case wasm.OpcodeI64Load32U:
		imm, err := c.readMemoryArg(wasm.OpcodeI64Load32UName)
		if err != nil {
			return err
		}
		c.emit(
			OperationLoad32{Signed: false, Arg: imm},
		)
	case wasm.OpcodeI32Store:
		imm, err := c.readMemoryArg(wasm.OpcodeI32StoreName)
		if err != nil {
			return err
		}
		c.emit(
			OperationStore{Type: UnsignedTypeI32, Arg: imm},
		)
	case wasm.OpcodeI64Store:
		imm, err := c.readMemoryArg(wasm.OpcodeI64StoreName)
		if err != nil {
			return err
		}
		c.emit(
			OperationStore{Type: UnsignedTypeI64, Arg: imm},
		)
	case wasm.OpcodeF32Store:
		imm, err := c.readMemoryArg(wasm.OpcodeF32StoreName)
		if err != nil {
			return err
		}
		c.emit(
			OperationStore{Type: UnsignedTypeF32, Arg: imm},
		)
	case wasm.OpcodeF64Store:
		imm, err := c.readMemoryArg(wasm.OpcodeF64StoreName)
		if err != nil {
			return err
		}
		c.emit(
			OperationStore{Type: UnsignedTypeF64, Arg: imm},
		)
	case wasm.OpcodeI32Store8:
		imm, err := c.readMemoryArg(wasm.OpcodeI32Store8Name)
		if err != nil {
			return err
		}
		c.emit(
			OperationStore8{Arg: imm},
		)
	case wasm.OpcodeI32Store16:
		imm, err := c.readMemoryArg(wasm.OpcodeI32Store16Name)
		if err != nil {
			return err
		}
		c.emit(
			OperationStore16{Arg: imm},
		)
	case wasm.OpcodeI64Store8:
		imm, err := c.readMemoryArg(wasm.OpcodeI64Store8Name)
		if err != nil {
			return err
		}
		c.emit(
			OperationStore8{Arg: imm},
		)
	case wasm.OpcodeI64Store16:
		imm, err := c.readMemoryArg(wasm.OpcodeI64Store16Name)
		if err != nil {
			return err
		}
		c.emit(
			OperationStore16{Arg: imm},
		)
	case wasm.OpcodeI64Store32:
		imm, err := c.readMemoryArg(wasm.OpcodeI64Store32Name)
		if err != nil {
			return err
		}
		c.emit(
			OperationStore32{Arg: imm},
		)
	case wasm.OpcodeMemorySize:
		c.result.UsesMemory = true
		c.pc++ // Skip the reserved one byte.
		c.emit(
			OperationMemorySize{},
		)
	case wasm.OpcodeMemoryGrow:
		c.result.UsesMemory = true
		c.pc++ // Skip the reserved one byte.
		c.emit(
			OperationMemoryGrow{},
		)
	case wasm.OpcodeI32Const:
		val, num, err := leb128.LoadInt32(c.body[c.pc+1:])
		if err != nil {
			return fmt.Errorf("reading i32.const value: %v", err)
		}
		c.pc += num
		c.emit(
			OperationConstI32{Value: uint32(val)},
		)
	case wasm.OpcodeI64Const:
		val, num, err := leb128.LoadInt64(c.body[c.pc+1:])
		if err != nil {
			return fmt.Errorf("reading i64.const value: %v", err)
		}
		c.pc += num
		c.emit(
			OperationConstI64{Value: uint64(val)},
		)
	case wasm.OpcodeF32Const:
		v := math.Float32frombits(binary.LittleEndian.Uint32(c.body[c.pc+1:]))
		c.pc += 4
		c.emit(
			OperationConstF32{Value: v},
		)
	case wasm.OpcodeF64Const:
		v := math.Float64frombits(binary.LittleEndian.Uint64(c.body[c.pc+1:]))
		c.pc += 8
		c.emit(
			OperationConstF64{Value: v},
		)
	case wasm.OpcodeI32Eqz:
		c.emit(
			OperationEqz{Type: UnsignedInt32},
		)
	case wasm.OpcodeI32Eq:
		c.emit(
			OperationEq{Type: UnsignedTypeI32},
		)
	case wasm.OpcodeI32Ne:
		c.emit(
			OperationNe{Type: UnsignedTypeI32},
		)
	case wasm.OpcodeI32LtS:
		c.emit(
			OperationLt{Type: SignedTypeInt32},
		)
	case wasm.OpcodeI32LtU:
		c.emit(
			OperationLt{Type: SignedTypeUint32},
		)
	case wasm.OpcodeI32GtS:
		c.emit(
			OperationGt{Type: SignedTypeInt32},
		)
	case wasm.OpcodeI32GtU:
		c.emit(
			OperationGt{Type: SignedTypeUint32},
		)
	case wasm.OpcodeI32LeS:
		c.emit(
			OperationLe{Type: SignedTypeInt32},
		)
	case wasm.OpcodeI32LeU:
		c.emit(
			OperationLe{Type: SignedTypeUint32},
		)
	case wasm.OpcodeI32GeS:
		c.emit(
			OperationGe{Type: SignedTypeInt32},
		)
	case wasm.OpcodeI32GeU:
		c.emit(
			OperationGe{Type: SignedTypeUint32},
		)
	case wasm.OpcodeI64Eqz:
		c.emit(
			OperationEqz{Type: UnsignedInt64},
		)
	case wasm.OpcodeI64Eq:
		c.emit(
			OperationEq{Type: UnsignedTypeI64},
		)
	case wasm.OpcodeI64Ne:
		c.emit(
			OperationNe{Type: UnsignedTypeI64},
		)
	case wasm.OpcodeI64LtS:
		c.emit(
			OperationLt{Type: SignedTypeInt64},
		)
	case wasm.OpcodeI64LtU:
		c.emit(
			OperationLt{Type: SignedTypeUint64},
		)
	case wasm.OpcodeI64GtS:
		c.emit(
			OperationGt{Type: SignedTypeInt64},
		)
	case wasm.OpcodeI64GtU:
		c.emit(
			OperationGt{Type: SignedTypeUint64},
		)
	case wasm.OpcodeI64LeS:
		c.emit(
			OperationLe{Type: SignedTypeInt64},
		)
	case wasm.OpcodeI64LeU:
		c.emit(
			OperationLe{Type: SignedTypeUint64},
		)
	case wasm.OpcodeI64GeS:
		c.emit(
			OperationGe{Type: SignedTypeInt64},
		)
	case wasm.OpcodeI64GeU:
		c.emit(
			OperationGe{Type: SignedTypeUint64},
		)
	case wasm.OpcodeF32Eq:
		c.emit(
			OperationEq{Type: UnsignedTypeF32},
		)
	case wasm.OpcodeF32Ne:
		c.emit(
			OperationNe{Type: UnsignedTypeF32},
		)
	case wasm.OpcodeF32Lt:
		c.emit(
			OperationLt{Type: SignedTypeFloat32},
		)
	case wasm.OpcodeF32Gt:
		c.emit(
			OperationGt{Type: SignedTypeFloat32},
		)
	case wasm.OpcodeF32Le:
		c.emit(
			OperationLe{Type: SignedTypeFloat32},
		)
	case wasm.OpcodeF32Ge:
		c.emit(
			OperationGe{Type: SignedTypeFloat32},
		)
	case wasm.OpcodeF64Eq:
		c.emit(
			OperationEq{Type: UnsignedTypeF64},
		)
	case wasm.OpcodeF64Ne:
		c.emit(
			OperationNe{Type: UnsignedTypeF64},
		)
	case wasm.OpcodeF64Lt:
		c.emit(
			OperationLt{Type: SignedTypeFloat64},
		)
	case wasm.OpcodeF64Gt:
		c.emit(
			OperationGt{Type: SignedTypeFloat64},
		)
	case wasm.OpcodeF64Le:
		c.emit(
			OperationLe{Type: SignedTypeFloat64},
		)
	case wasm.OpcodeF64Ge:
		c.emit(
			OperationGe{Type: SignedTypeFloat64},
		)
	case wasm.OpcodeI32Clz:
		c.emit(
			OperationClz{Type: UnsignedInt32},
		)
	case wasm.OpcodeI32Ctz:
		c.emit(
			OperationCtz{Type: UnsignedInt32},
		)
	case wasm.OpcodeI32Popcnt:
		c.emit(
			OperationPopcnt{Type: UnsignedInt32},
		)
	case wasm.OpcodeI32Add:
		c.emit(
			OperationAdd{Type: UnsignedTypeI32},
		)
	case wasm.OpcodeI32Sub:
		c.emit(
			OperationSub{Type: UnsignedTypeI32},
		)
	case wasm.OpcodeI32Mul:
		c.emit(
			OperationMul{Type: UnsignedTypeI32},
		)
	case wasm.OpcodeI32DivS:
		c.emit(
			OperationDiv{Type: SignedTypeInt32},
		)
	case wasm.OpcodeI32DivU:
		c.emit(
			OperationDiv{Type: SignedTypeUint32},
		)
	case wasm.OpcodeI32RemS:
		c.emit(
			OperationRem{Type: SignedInt32},
		)
	case wasm.OpcodeI32RemU:
		c.emit(
			OperationRem{Type: SignedUint32},
		)
	case wasm.OpcodeI32And:
		c.emit(
			OperationAnd{Type: UnsignedInt32},
		)
	case wasm.OpcodeI32Or:
		c.emit(
			OperationOr{Type: UnsignedInt32},
		)
	case wasm.OpcodeI32Xor:
		c.emit(
			OperationXor{Type: UnsignedInt64},
		)
	case wasm.OpcodeI32Shl:
		c.emit(
			OperationShl{Type: UnsignedInt32},
		)
	case wasm.OpcodeI32ShrS:
		c.emit(
			OperationShr{Type: SignedInt32},
		)
	case wasm.OpcodeI32ShrU:
		c.emit(
			OperationShr{Type: SignedUint32},
		)
	case wasm.OpcodeI32Rotl:
		c.emit(
			OperationRotl{Type: UnsignedInt32},
		)
	case wasm.OpcodeI32Rotr:
		c.emit(
			OperationRotr{Type: UnsignedInt32},
		)
	case wasm.OpcodeI64Clz:
		c.emit(
			OperationClz{Type: UnsignedInt64},
		)
	case wasm.OpcodeI64Ctz:
		c.emit(
			OperationCtz{Type: UnsignedInt64},
		)
	case wasm.OpcodeI64Popcnt:
		c.emit(
			OperationPopcnt{Type: UnsignedInt64},
		)
	case wasm.OpcodeI64Add:
		c.emit(
			OperationAdd{Type: UnsignedTypeI64},
		)
	case wasm.OpcodeI64Sub:
		c.emit(
			OperationSub{Type: UnsignedTypeI64},
		)
	case wasm.OpcodeI64Mul:
		c.emit(
			OperationMul{Type: UnsignedTypeI64},
		)
	case wasm.OpcodeI64DivS:
		c.emit(
			OperationDiv{Type: SignedTypeInt64},
		)
	case wasm.OpcodeI64DivU:
		c.emit(
			OperationDiv{Type: SignedTypeUint64},
		)
	case wasm.OpcodeI64RemS:
		c.emit(
			OperationRem{Type: SignedInt64},
		)
	case wasm.OpcodeI64RemU:
		c.emit(
			OperationRem{Type: SignedUint64},
		)
	case wasm.OpcodeI64And:
		c.emit(
			OperationAnd{Type: UnsignedInt64},
		)
	case wasm.OpcodeI64Or:
		c.emit(
			OperationOr{Type: UnsignedInt64},
		)
	case wasm.OpcodeI64Xor:
		c.emit(
			OperationXor{Type: UnsignedInt64},
		)
	case wasm.OpcodeI64Shl:
		c.emit(
			OperationShl{Type: UnsignedInt64},
		)
	case wasm.OpcodeI64ShrS:
		c.emit(
			OperationShr{Type: SignedInt64},
		)
	case wasm.OpcodeI64ShrU:
		c.emit(
			OperationShr{Type: SignedUint64},
		)
	case wasm.OpcodeI64Rotl:
		c.emit(
			OperationRotl{Type: UnsignedInt64},
		)
	case wasm.OpcodeI64Rotr:
		c.emit(
			OperationRotr{Type: UnsignedInt64},
		)
	case wasm.OpcodeF32Abs:
		c.emit(
			OperationAbs{Type: Float32},
		)
	case wasm.OpcodeF32Neg:
		c.emit(
			OperationNeg{Type: Float32},
		)
	case wasm.OpcodeF32Ceil:
		c.emit(
			OperationCeil{Type: Float32},
		)
	case wasm.OpcodeF32Floor:
		c.emit(
			OperationFloor{Type: Float32},
		)
	case wasm.OpcodeF32Trunc:
		c.emit(
			OperationTrunc{Type: Float32},
		)
	case wasm.OpcodeF32Nearest:
		c.emit(
			OperationNearest{Type: Float32},
		)
	case wasm.OpcodeF32Sqrt:
		c.emit(
			OperationSqrt{Type: Float32},
		)
	case wasm.OpcodeF32Add:
		c.emit(
			OperationAdd{Type: UnsignedTypeF32},
		)
	case wasm.OpcodeF32Sub:
		c.emit(
			OperationSub{Type: UnsignedTypeF32},
		)
	case wasm.OpcodeF32Mul:
		c.emit(
			OperationMul{Type: UnsignedTypeF32},
		)
	case wasm.OpcodeF32Div:
		c.emit(
			OperationDiv{Type: SignedTypeFloat32},
		)
	case wasm.OpcodeF32Min:
		c.emit(
			OperationMin{Type: Float32},
		)
	case wasm.OpcodeF32Max:
		c.emit(
			OperationMax{Type: Float32},
		)
	case wasm.OpcodeF32Copysign:
		c.emit(
			OperationCopysign{Type: Float32},
		)
	case wasm.OpcodeF64Abs:
		c.emit(
			OperationAbs{Type: Float64},
		)
	case wasm.OpcodeF64Neg:
		c.emit(
			OperationNeg{Type: Float64},
		)
	case wasm.OpcodeF64Ceil:
		c.emit(
			OperationCeil{Type: Float64},
		)
	case wasm.OpcodeF64Floor:
		c.emit(
			OperationFloor{Type: Float64},
		)
	case wasm.OpcodeF64Trunc:
		c.emit(
			OperationTrunc{Type: Float64},
		)
	case wasm.OpcodeF64Nearest:
		c.emit(
			OperationNearest{Type: Float64},
		)
	case wasm.OpcodeF64Sqrt:
		c.emit(
			OperationSqrt{Type: Float64},
		)
	case wasm.OpcodeF64Add:
		c.emit(
			OperationAdd{Type: UnsignedTypeF64},
		)
	case wasm.OpcodeF64Sub:
		c.emit(
			OperationSub{Type: UnsignedTypeF64},
		)
	case wasm.OpcodeF64Mul:
		c.emit(
			OperationMul{Type: UnsignedTypeF64},
		)
	case wasm.OpcodeF64Div:
		c.emit(
			OperationDiv{Type: SignedTypeFloat64},
		)
	case wasm.OpcodeF64Min:
		c.emit(
			OperationMin{Type: Float64},
		)
	case wasm.OpcodeF64Max:
		c.emit(
			OperationMax{Type: Float64},
		)
	case wasm.OpcodeF64Copysign:
		c.emit(
			OperationCopysign{Type: Float64},
		)
	case wasm.OpcodeI32WrapI64:
		c.emit(
			OperationI32WrapFromI64{},
		)
	case wasm.OpcodeI32TruncF32S:
		c.emit(
			OperationITruncFromF{InputType: Float32, OutputType: SignedInt32},
		)
	case wasm.OpcodeI32TruncF32U:
		c.emit(
			OperationITruncFromF{InputType: Float32, OutputType: SignedUint32},
		)
	case wasm.OpcodeI32TruncF64S:
		c.emit(
			OperationITruncFromF{InputType: Float64, OutputType: SignedInt32},
		)
	case wasm.OpcodeI32TruncF64U:
		c.emit(
			OperationITruncFromF{InputType: Float64, OutputType: SignedUint32},
		)
	case wasm.OpcodeI64ExtendI32S:
		c.emit(
			OperationExtend{Signed: true},
		)
	case wasm.OpcodeI64ExtendI32U:
		c.emit(
			OperationExtend{Signed: false},
		)
	case wasm.OpcodeI64TruncF32S:
		c.emit(
			OperationITruncFromF{InputType: Float32, OutputType: SignedInt64},
		)
	case wasm.OpcodeI64TruncF32U:
		c.emit(
			OperationITruncFromF{InputType: Float32, OutputType: SignedUint64},
		)
	case wasm.OpcodeI64TruncF64S:
		c.emit(
			OperationITruncFromF{InputType: Float64, OutputType: SignedInt64},
		)
	case wasm.OpcodeI64TruncF64U:
		c.emit(
			OperationITruncFromF{InputType: Float64, OutputType: SignedUint64},
		)
	case wasm.OpcodeF32ConvertI32S:
		c.emit(
			OperationFConvertFromI{InputType: SignedInt32, OutputType: Float32},
		)
	case wasm.OpcodeF32ConvertI32U:
		c.emit(
			OperationFConvertFromI{InputType: SignedUint32, OutputType: Float32},
		)
	case wasm.OpcodeF32ConvertI64S:
		c.emit(
			OperationFConvertFromI{InputType: SignedInt64, OutputType: Float32},
		)
	case wasm.OpcodeF32ConvertI64U:
		c.emit(
			OperationFConvertFromI{InputType: SignedUint64, OutputType: Float32},
		)
	case wasm.OpcodeF32DemoteF64:
		c.emit(
			OperationF32DemoteFromF64{},
		)
	case wasm.OpcodeF64ConvertI32S:
		c.emit(
			OperationFConvertFromI{InputType: SignedInt32, OutputType: Float64},
		)
	case wasm.OpcodeF64ConvertI32U:
		c.emit(
			OperationFConvertFromI{InputType: SignedUint32, OutputType: Float64},
		)
	case wasm.OpcodeF64ConvertI64S:
		c.emit(
			OperationFConvertFromI{InputType: SignedInt64, OutputType: Float64},
		)
	case wasm.OpcodeF64ConvertI64U:
		c.emit(
			OperationFConvertFromI{InputType: SignedUint64, OutputType: Float64},
		)
	case wasm.OpcodeF64PromoteF32:
		c.emit(
			OperationF64PromoteFromF32{},
		)
	case wasm.OpcodeI32ReinterpretF32:
		c.emit(
			OperationI32ReinterpretFromF32{},
		)
	case wasm.OpcodeI64ReinterpretF64:
		c.emit(
			OperationI64ReinterpretFromF64{},
		)
	case wasm.OpcodeF32ReinterpretI32:
		c.emit(
			OperationF32ReinterpretFromI32{},
		)
	case wasm.OpcodeF64ReinterpretI64:
		c.emit(
			OperationF64ReinterpretFromI64{},
		)
	case wasm.OpcodeI32Extend8S:
		c.emit(
			OperationSignExtend32From8{},
		)
	case wasm.OpcodeI32Extend16S:
		c.emit(
			OperationSignExtend32From16{},
		)
	case wasm.OpcodeI64Extend8S:
		c.emit(
			OperationSignExtend64From8{},
		)
	case wasm.OpcodeI64Extend16S:
		c.emit(
			OperationSignExtend64From16{},
		)
	case wasm.OpcodeI64Extend32S:
		c.emit(
			OperationSignExtend64From32{},
		)
	case wasm.OpcodeRefFunc:
		c.pc++
		index, num, err := leb128.LoadUint32(c.body[c.pc:])
		if err != nil {
			return fmt.Errorf("failed to read function index for ref.func: %v", err)
		}
		c.pc += num - 1
		c.emit(
			OperationRefFunc{FunctionIndex: index},
		)
	case wasm.OpcodeRefNull:
		c.pc++ // Skip the type of reftype as every ref value is opaque pointer.
		c.emit(
			OperationConstI64{Value: 0},
		)
	case wasm.OpcodeRefIsNull:
		// Simply compare the opaque pointer (i64) with zero.
		c.emit(
			OperationEqz{Type: UnsignedInt64},
		)
	case wasm.OpcodeTableGet:
		c.pc++
		tableIndex, num, err := leb128.LoadUint32(c.body[c.pc:])
		if err != nil {
			return fmt.Errorf("failed to read function index for table.get: %v", err)
		}
		c.pc += num - 1
		c.emit(
			OperationTableGet{TableIndex: tableIndex},
		)
	case wasm.OpcodeTableSet:
		c.pc++
		tableIndex, num, err := leb128.LoadUint32(c.body[c.pc:])
		if err != nil {
			return fmt.Errorf("failed to read function index for table.set: %v", err)
		}
		c.pc += num - 1
		c.emit(
			OperationTableSet{TableIndex: tableIndex},
		)
	case wasm.OpcodeMiscPrefix:
		c.pc++
		// A misc opcode is encoded as an unsigned variable 32-bit integer.
		miscOp, num, err := leb128.LoadUint32(c.body[c.pc:])
		if err != nil {
			return fmt.Errorf("failed to read misc opcode: %v", err)
		}
		c.pc += num - 1
		switch byte(miscOp) {
		case wasm.OpcodeMiscI32TruncSatF32S:
			c.emit(
				OperationITruncFromF{InputType: Float32, OutputType: SignedInt32, NonTrapping: true},
			)
		case wasm.OpcodeMiscI32TruncSatF32U:
			c.emit(
				OperationITruncFromF{InputType: Float32, OutputType: SignedUint32, NonTrapping: true},
			)
		case wasm.OpcodeMiscI32TruncSatF64S:
			c.emit(
				OperationITruncFromF{InputType: Float64, OutputType: SignedInt32, NonTrapping: true},
			)
		case wasm.OpcodeMiscI32TruncSatF64U:
			c.emit(
				OperationITruncFromF{InputType: Float64, OutputType: SignedUint32, NonTrapping: true},
			)
		case wasm.OpcodeMiscI64TruncSatF32S:
			c.emit(
				OperationITruncFromF{InputType: Float32, OutputType: SignedInt64, NonTrapping: true},
			)
		case wasm.OpcodeMiscI64TruncSatF32U:
			c.emit(
				OperationITruncFromF{InputType: Float32, OutputType: SignedUint64, NonTrapping: true},
			)
		case wasm.OpcodeMiscI64TruncSatF64S:
			c.emit(
				OperationITruncFromF{InputType: Float64, OutputType: SignedInt64, NonTrapping: true},
			)
		case wasm.OpcodeMiscI64TruncSatF64U:
			c.emit(
				OperationITruncFromF{InputType: Float64, OutputType: SignedUint64, NonTrapping: true},
			)
		case wasm.OpcodeMiscMemoryInit:
			c.result.UsesMemory = true
			dataIndex, num, err := leb128.LoadUint32(c.body[c.pc+1:])
			if err != nil {
				return fmt.Errorf("reading i32.const value: %v", err)
			}
			c.pc += num + 1 // +1 to skip the memory index which is fixed to zero.
			c.emit(
				OperationMemoryInit{DataIndex: dataIndex},
			)
		case wasm.OpcodeMiscDataDrop:
			dataIndex, num, err := leb128.LoadUint32(c.body[c.pc+1:])
			if err != nil {
				return fmt.Errorf("reading i32.const value: %v", err)
			}
			c.pc += num
			c.emit(
				OperationDataDrop{DataIndex: dataIndex},
			)
		case wasm.OpcodeMiscMemoryCopy:
			c.result.UsesMemory = true
			c.pc += 2 // +2 to skip two memory indexes which are fixed to zero.
			c.emit(
				OperationMemoryCopy{},
			)
		case wasm.OpcodeMiscMemoryFill:
			c.result.UsesMemory = true
			c.pc += 1 // +1 to skip the memory index which is fixed to zero.
			c.emit(
				OperationMemoryFill{},
			)
		case wasm.OpcodeMiscTableInit:
			elemIndex, num, err := leb128.LoadUint32(c.body[c.pc+1:])
			if err != nil {
				return fmt.Errorf("reading i32.const value: %v", err)
			}
			c.pc += num
			// Read table index which is fixed to zero currently.
			tableIndex, num, err := leb128.LoadUint32(c.body[c.pc+1:])
			if err != nil {
				return fmt.Errorf("reading i32.const value: %v", err)
			}
			c.pc += num
			c.emit(
				OperationTableInit{ElemIndex: elemIndex, TableIndex: tableIndex},
			)
		case wasm.OpcodeMiscElemDrop:
			elemIndex, num, err := leb128.LoadUint32(c.body[c.pc+1:])
			if err != nil {
				return fmt.Errorf("reading i32.const value: %v", err)
			}
			c.pc += num
			c.emit(
				OperationElemDrop{ElemIndex: elemIndex},
			)
		case wasm.OpcodeMiscTableCopy:
			// Read the source table inde.g.
			dst, num, err := leb128.LoadUint32(c.body[c.pc+1:])
			if err != nil {
				return fmt.Errorf("reading i32.const value: %v", err)
			}
			c.pc += num
			// Read the destination table inde.g.
			src, num, err := leb128.LoadUint32(c.body[c.pc+1:])
			if err != nil {
				return fmt.Errorf("reading i32.const value: %v", err)
			}
			c.pc += num
			c.emit(
				OperationTableCopy{SrcTableIndex: src, DstTableIndex: dst},
			)
		case wasm.OpcodeMiscTableGrow:
			// Read the source table inde.g.
			tableIndex, num, err := leb128.LoadUint32(c.body[c.pc+1:])
			if err != nil {
				return fmt.Errorf("reading i32.const value: %v", err)
			}
			c.pc += num
			c.emit(
				OperationTableGrow{TableIndex: tableIndex},
			)
		case wasm.OpcodeMiscTableSize:
			// Read the source table inde.g.
			tableIndex, num, err := leb128.LoadUint32(c.body[c.pc+1:])
			if err != nil {
				return fmt.Errorf("reading i32.const value: %v", err)
			}
			c.pc += num
			c.emit(
				OperationTableSize{TableIndex: tableIndex},
			)
		case wasm.OpcodeMiscTableFill:
			// Read the source table index.
			tableIndex, num, err := leb128.LoadUint32(c.body[c.pc+1:])
			if err != nil {
				return fmt.Errorf("reading i32.const value: %v", err)
			}
			c.pc += num
			c.emit(
				OperationTableFill{TableIndex: tableIndex},
			)
		default:
			return fmt.Errorf("unsupported misc instruction in wazeroir: 0x%x", op)
		}
	case wasm.OpcodeVecPrefix:
		c.pc++
		switch vecOp := c.body[c.pc]; vecOp {
		case wasm.OpcodeVecV128Const:
			c.pc++
			lo := binary.LittleEndian.Uint64(c.body[c.pc : c.pc+8])
			c.pc += 8
			hi := binary.LittleEndian.Uint64(c.body[c.pc : c.pc+8])
			c.emit(
				OperationV128Const{Lo: lo, Hi: hi},
			)
			c.pc += 7
		case wasm.OpcodeVecV128Load:
			arg, err := c.readMemoryArg(wasm.OpcodeI32LoadName)
			if err != nil {
				return err
			}
			c.emit(
				OperationV128Load{Type: V128LoadType128, Arg: arg},
			)
		case wasm.OpcodeVecV128Load8x8s:
			arg, err := c.readMemoryArg(wasm.OpcodeVecV128Load8x8SName)
			if err != nil {
				return err
			}
			c.emit(
				OperationV128Load{Type: V128LoadType8x8s, Arg: arg},
			)
		case wasm.OpcodeVecV128Load8x8u:
			arg, err := c.readMemoryArg(wasm.OpcodeVecV128Load8x8UName)
			if err != nil {
				return err
			}
			c.emit(
				OperationV128Load{Type: V128LoadType8x8u, Arg: arg},
			)
		case wasm.OpcodeVecV128Load16x4s:
			arg, err := c.readMemoryArg(wasm.OpcodeVecV128Load16x4SName)
			if err != nil {
				return err
			}
			c.emit(
				OperationV128Load{Type: V128LoadType16x4s, Arg: arg},
			)
		case wasm.OpcodeVecV128Load16x4u:
			arg, err := c.readMemoryArg(wasm.OpcodeVecV128Load16x4UName)
			if err != nil {
				return err
			}
			c.emit(
				OperationV128Load{Type: V128LoadType16x4u, Arg: arg},
			)
		case wasm.OpcodeVecV128Load32x2s:
			arg, err := c.readMemoryArg(wasm.OpcodeVecV128Load32x2SName)
			if err != nil {
				return err
			}
			c.emit(
				OperationV128Load{Type: V128LoadType32x2s, Arg: arg},
			)
		case wasm.OpcodeVecV128Load32x2u:
			arg, err := c.readMemoryArg(wasm.OpcodeVecV128Load32x2UName)
			if err != nil {
				return err
			}
			c.emit(
				OperationV128Load{Type: V128LoadType32x2u, Arg: arg},
			)
		case wasm.OpcodeVecV128Load8Splat:
			arg, err := c.readMemoryArg(wasm.OpcodeVecV128Load8SplatName)
			if err != nil {
				return err
			}
			c.emit(
				OperationV128Load{Type: V128LoadType8Splat, Arg: arg},
			)
		case wasm.OpcodeVecV128Load16Splat:
			arg, err := c.readMemoryArg(wasm.OpcodeVecV128Load16SplatName)
			if err != nil {
				return err
			}
			c.emit(
				OperationV128Load{Type: V128LoadType16Splat, Arg: arg},
			)
		case wasm.OpcodeVecV128Load32Splat:
			arg, err := c.readMemoryArg(wasm.OpcodeVecV128Load32SplatName)
			if err != nil {
				return err
			}
			c.emit(
				OperationV128Load{Type: V128LoadType32Splat, Arg: arg},
			)
		case wasm.OpcodeVecV128Load64Splat:
			arg, err := c.readMemoryArg(wasm.OpcodeVecV128Load64SplatName)
			if err != nil {
				return err
			}
			c.emit(
				OperationV128Load{Type: V128LoadType64Splat, Arg: arg},
			)
		case wasm.OpcodeVecV128Load32zero:
			arg, err := c.readMemoryArg(wasm.OpcodeVecV128Load32zeroName)
			if err != nil {
				return err
			}
			c.emit(
				OperationV128Load{Type: V128LoadType32zero, Arg: arg},
			)
		case wasm.OpcodeVecV128Load64zero:
			arg, err := c.readMemoryArg(wasm.OpcodeVecV128Load64zeroName)
			if err != nil {
				return err
			}
			c.emit(
				OperationV128Load{Type: V128LoadType64zero, Arg: arg},
			)
		case wasm.OpcodeVecV128Load8Lane:
			arg, err := c.readMemoryArg(wasm.OpcodeVecV128Load8LaneName)
			if err != nil {
				return err
			}
			c.pc++
			laneIndex := c.body[c.pc]
			c.emit(
				OperationV128LoadLane{LaneIndex: laneIndex, LaneSize: 8, Arg: arg},
			)
		case wasm.OpcodeVecV128Load16Lane:
			arg, err := c.readMemoryArg(wasm.OpcodeVecV128Load16LaneName)
			if err != nil {
				return err
			}
			c.pc++
			laneIndex := c.body[c.pc]
			c.emit(
				OperationV128LoadLane{LaneIndex: laneIndex, LaneSize: 16, Arg: arg},
			)
		case wasm.OpcodeVecV128Load32Lane:
			arg, err := c.readMemoryArg(wasm.OpcodeVecV128Load32LaneName)
			if err != nil {
				return err
			}
			c.pc++
			laneIndex := c.body[c.pc]
			c.emit(
				OperationV128LoadLane{LaneIndex: laneIndex, LaneSize: 32, Arg: arg},
			)
		case wasm.OpcodeVecV128Load64Lane:
			arg, err := c.readMemoryArg(wasm.OpcodeVecV128Load64LaneName)
			if err != nil {
				return err
			}
			c.pc++
			laneIndex := c.body[c.pc]
			c.emit(
				OperationV128LoadLane{LaneIndex: laneIndex, LaneSize: 64, Arg: arg},
			)
		case wasm.OpcodeVecV128Store:
			arg, err := c.readMemoryArg(wasm.OpcodeVecV128StoreName)
			if err != nil {
				return err
			}
			c.emit(
				OperationV128Store{Arg: arg},
			)
		case wasm.OpcodeVecV128Store8Lane:
			arg, err := c.readMemoryArg(wasm.OpcodeVecV128Store8LaneName)
			if err != nil {
				return err
			}
			c.pc++
			laneIndex := c.body[c.pc]
			c.emit(
				OperationV128StoreLane{LaneIndex: laneIndex, LaneSize: 8, Arg: arg},
			)
		case wasm.OpcodeVecV128Store16Lane:
			arg, err := c.readMemoryArg(wasm.OpcodeVecV128Store16LaneName)
			if err != nil {
				return err
			}
			c.pc++
			laneIndex := c.body[c.pc]
			c.emit(
				OperationV128StoreLane{LaneIndex: laneIndex, LaneSize: 16, Arg: arg},
			)
		case wasm.OpcodeVecV128Store32Lane:
			arg, err := c.readMemoryArg(wasm.OpcodeVecV128Store32LaneName)
			if err != nil {
				return err
			}
			c.pc++
			laneIndex := c.body[c.pc]
			c.emit(
				OperationV128StoreLane{LaneIndex: laneIndex, LaneSize: 32, Arg: arg},
			)
		case wasm.OpcodeVecV128Store64Lane:
			arg, err := c.readMemoryArg(wasm.OpcodeVecV128Store64LaneName)
			if err != nil {
				return err
			}
			c.pc++
			laneIndex := c.body[c.pc]
			c.emit(
				OperationV128StoreLane{LaneIndex: laneIndex, LaneSize: 64, Arg: arg},
			)
		case wasm.OpcodeVecI8x16ExtractLaneS:
			c.pc++
			laneIndex := c.body[c.pc]
			c.emit(
				OperationV128ExtractLane{LaneIndex: laneIndex, Shape: ShapeI8x16, Signed: true},
			)
		case wasm.OpcodeVecI8x16ExtractLaneU:
			c.pc++
			laneIndex := c.body[c.pc]
			c.emit(
				OperationV128ExtractLane{LaneIndex: laneIndex, Shape: ShapeI8x16, Signed: false},
			)
		case wasm.OpcodeVecI16x8ExtractLaneS:
			c.pc++
			laneIndex := c.body[c.pc]
			c.emit(
				OperationV128ExtractLane{LaneIndex: laneIndex, Shape: ShapeI16x8, Signed: true},
			)
		case wasm.OpcodeVecI16x8ExtractLaneU:
			c.pc++
			laneIndex := c.body[c.pc]
			c.emit(
				OperationV128ExtractLane{LaneIndex: laneIndex, Shape: ShapeI16x8, Signed: false},
			)
		case wasm.OpcodeVecI32x4ExtractLane:
			c.pc++
			laneIndex := c.body[c.pc]
			c.emit(
				OperationV128ExtractLane{LaneIndex: laneIndex, Shape: ShapeI32x4},
			)
		case wasm.OpcodeVecI64x2ExtractLane:
			c.pc++
			laneIndex := c.body[c.pc]
			c.emit(
				OperationV128ExtractLane{LaneIndex: laneIndex, Shape: ShapeI64x2},
			)
		case wasm.OpcodeVecF32x4ExtractLane:
			c.pc++
			laneIndex := c.body[c.pc]
			c.emit(
				OperationV128ExtractLane{LaneIndex: laneIndex, Shape: ShapeF32x4},
			)
		case wasm.OpcodeVecF64x2ExtractLane:
			c.pc++
			laneIndex := c.body[c.pc]
			c.emit(
				OperationV128ExtractLane{LaneIndex: laneIndex, Shape: ShapeF64x2},
			)
		case wasm.OpcodeVecI8x16ReplaceLane:
			c.pc++
			laneIndex := c.body[c.pc]
			c.emit(
				OperationV128ReplaceLane{LaneIndex: laneIndex, Shape: ShapeI8x16},
			)
		case wasm.OpcodeVecI16x8ReplaceLane:
			c.pc++
			laneIndex := c.body[c.pc]
			c.emit(
				OperationV128ReplaceLane{LaneIndex: laneIndex, Shape: ShapeI16x8},
			)
		case wasm.OpcodeVecI32x4ReplaceLane:
			c.pc++
			laneIndex := c.body[c.pc]
			c.emit(
				OperationV128ReplaceLane{LaneIndex: laneIndex, Shape: ShapeI32x4},
			)
		case wasm.OpcodeVecI64x2ReplaceLane:
			c.pc++
			laneIndex := c.body[c.pc]
			c.emit(
				OperationV128ReplaceLane{LaneIndex: laneIndex, Shape: ShapeI64x2},
			)
		case wasm.OpcodeVecF32x4ReplaceLane:
			c.pc++
			laneIndex := c.body[c.pc]
			c.emit(
				OperationV128ReplaceLane{LaneIndex: laneIndex, Shape: ShapeF32x4},
			)
		case wasm.OpcodeVecF64x2ReplaceLane:
			c.pc++
			laneIndex := c.body[c.pc]
			c.emit(
				OperationV128ReplaceLane{LaneIndex: laneIndex, Shape: ShapeF64x2},
			)
		case wasm.OpcodeVecI8x16Splat:
			c.emit(
				OperationV128Splat{Shape: ShapeI8x16},
			)
		case wasm.OpcodeVecI16x8Splat:
			c.emit(
				OperationV128Splat{Shape: ShapeI16x8},
			)
		case wasm.OpcodeVecI32x4Splat:
			c.emit(
				OperationV128Splat{Shape: ShapeI32x4},
			)
		case wasm.OpcodeVecI64x2Splat:
			c.emit(
				OperationV128Splat{Shape: ShapeI64x2},
			)
		case wasm.OpcodeVecF32x4Splat:
			c.emit(
				OperationV128Splat{Shape: ShapeF32x4},
			)
		case wasm.OpcodeVecF64x2Splat:
			c.emit(
				OperationV128Splat{Shape: ShapeF64x2},
			)
		case wasm.OpcodeVecI8x16Swizzle:
			c.emit(
				OperationV128Swizzle{},
			)
		case wasm.OpcodeVecV128i8x16Shuffle:
			c.pc++
			op := OperationV128Shuffle{}
			copy(op.Lanes[:], c.body[c.pc:c.pc+16])
			c.emit(op)
			c.pc += 15
		case wasm.OpcodeVecV128AnyTrue:
			c.emit(
				OperationV128AnyTrue{},
			)
		case wasm.OpcodeVecI8x16AllTrue:
			c.emit(
				OperationV128AllTrue{Shape: ShapeI8x16},
			)
		case wasm.OpcodeVecI16x8AllTrue:
			c.emit(
				OperationV128AllTrue{Shape: ShapeI16x8},
			)
		case wasm.OpcodeVecI32x4AllTrue:
			c.emit(
				OperationV128AllTrue{Shape: ShapeI32x4},
			)
		case wasm.OpcodeVecI64x2AllTrue:
			c.emit(
				OperationV128AllTrue{Shape: ShapeI64x2},
			)
		case wasm.OpcodeVecI8x16BitMask:
			c.emit(
				OperationV128BitMask{Shape: ShapeI8x16},
			)
		case wasm.OpcodeVecI16x8BitMask:
			c.emit(
				OperationV128BitMask{Shape: ShapeI16x8},
			)
		case wasm.OpcodeVecI32x4BitMask:
			c.emit(
				OperationV128BitMask{Shape: ShapeI32x4},
			)
		case wasm.OpcodeVecI64x2BitMask:
			c.emit(
				OperationV128BitMask{Shape: ShapeI64x2},
			)
		case wasm.OpcodeVecV128And:
			c.emit(
				OperationV128And{},
			)
		case wasm.OpcodeVecV128Not:
			c.emit(
				OperationV128Not{},
			)
		case wasm.OpcodeVecV128Or:
			c.emit(
				OperationV128Or{},
			)
		case wasm.OpcodeVecV128Xor:
			c.emit(
				OperationV128Xor{},
			)
		case wasm.OpcodeVecV128Bitselect:
			c.emit(
				OperationV128Bitselect{},
			)
		case wasm.OpcodeVecV128AndNot:
			c.emit(
				OperationV128AndNot{},
			)
		case wasm.OpcodeVecI8x16Shl:
			c.emit(
				OperationV128Shl{Shape: ShapeI8x16},
			)
		case wasm.OpcodeVecI8x16ShrS:
			c.emit(
				OperationV128Shr{Shape: ShapeI8x16, Signed: true},
			)
		case wasm.OpcodeVecI8x16ShrU:
			c.emit(
				OperationV128Shr{Shape: ShapeI8x16, Signed: false},
			)
		case wasm.OpcodeVecI16x8Shl:
			c.emit(
				OperationV128Shl{Shape: ShapeI16x8},
			)
		case wasm.OpcodeVecI16x8ShrS:
			c.emit(
				OperationV128Shr{Shape: ShapeI16x8, Signed: true},
			)
		case wasm.OpcodeVecI16x8ShrU:
			c.emit(
				OperationV128Shr{Shape: ShapeI16x8, Signed: false},
			)
		case wasm.OpcodeVecI32x4Shl:
			c.emit(
				OperationV128Shl{Shape: ShapeI32x4},
			)
		case wasm.OpcodeVecI32x4ShrS:
			c.emit(
				OperationV128Shr{Shape: ShapeI32x4, Signed: true},
			)
		case wasm.OpcodeVecI32x4ShrU:
			c.emit(
				OperationV128Shr{Shape: ShapeI32x4, Signed: false},
			)
		case wasm.OpcodeVecI64x2Shl:
			c.emit(
				OperationV128Shl{Shape: ShapeI64x2},
			)
		case wasm.OpcodeVecI64x2ShrS:
			c.emit(
				OperationV128Shr{Shape: ShapeI64x2, Signed: true},
			)
		case wasm.OpcodeVecI64x2ShrU:
			c.emit(
				OperationV128Shr{Shape: ShapeI64x2, Signed: false},
			)
		case wasm.OpcodeVecI8x16Eq:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeI8x16Eq},
			)
		case wasm.OpcodeVecI8x16Ne:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeI8x16Ne},
			)
		case wasm.OpcodeVecI8x16LtS:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeI8x16LtS},
			)
		case wasm.OpcodeVecI8x16LtU:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeI8x16LtU},
			)
		case wasm.OpcodeVecI8x16GtS:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeI8x16GtS},
			)
		case wasm.OpcodeVecI8x16GtU:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeI8x16GtU},
			)
		case wasm.OpcodeVecI8x16LeS:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeI8x16LeS},
			)
		case wasm.OpcodeVecI8x16LeU:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeI8x16LeU},
			)
		case wasm.OpcodeVecI8x16GeS:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeI8x16GeS},
			)
		case wasm.OpcodeVecI8x16GeU:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeI8x16GeU},
			)
		case wasm.OpcodeVecI16x8Eq:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeI16x8Eq},
			)
		case wasm.OpcodeVecI16x8Ne:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeI16x8Ne},
			)
		case wasm.OpcodeVecI16x8LtS:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeI16x8LtS},
			)
		case wasm.OpcodeVecI16x8LtU:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeI16x8LtU},
			)
		case wasm.OpcodeVecI16x8GtS:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeI16x8GtS},
			)
		case wasm.OpcodeVecI16x8GtU:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeI16x8GtU},
			)
		case wasm.OpcodeVecI16x8LeS:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeI16x8LeS},
			)
		case wasm.OpcodeVecI16x8LeU:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeI16x8LeU},
			)
		case wasm.OpcodeVecI16x8GeS:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeI16x8GeS},
			)
		case wasm.OpcodeVecI16x8GeU:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeI16x8GeU},
			)
		case wasm.OpcodeVecI32x4Eq:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeI32x4Eq},
			)
		case wasm.OpcodeVecI32x4Ne:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeI32x4Ne},
			)
		case wasm.OpcodeVecI32x4LtS:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeI32x4LtS},
			)
		case wasm.OpcodeVecI32x4LtU:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeI32x4LtU},
			)
		case wasm.OpcodeVecI32x4GtS:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeI32x4GtS},
			)
		case wasm.OpcodeVecI32x4GtU:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeI32x4GtU},
			)
		case wasm.OpcodeVecI32x4LeS:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeI32x4LeS},
			)
		case wasm.OpcodeVecI32x4LeU:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeI32x4LeU},
			)
		case wasm.OpcodeVecI32x4GeS:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeI32x4GeS},
			)
		case wasm.OpcodeVecI32x4GeU:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeI32x4GeU},
			)
		case wasm.OpcodeVecI64x2Eq:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeI64x2Eq},
			)
		case wasm.OpcodeVecI64x2Ne:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeI64x2Ne},
			)
		case wasm.OpcodeVecI64x2LtS:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeI64x2LtS},
			)
		case wasm.OpcodeVecI64x2GtS:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeI64x2GtS},
			)
		case wasm.OpcodeVecI64x2LeS:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeI64x2LeS},
			)
		case wasm.OpcodeVecI64x2GeS:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeI64x2GeS},
			)
		case wasm.OpcodeVecF32x4Eq:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeF32x4Eq},
			)
		case wasm.OpcodeVecF32x4Ne:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeF32x4Ne},
			)
		case wasm.OpcodeVecF32x4Lt:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeF32x4Lt},
			)
		case wasm.OpcodeVecF32x4Gt:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeF32x4Gt},
			)
		case wasm.OpcodeVecF32x4Le:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeF32x4Le},
			)
		case wasm.OpcodeVecF32x4Ge:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeF32x4Ge},
			)
		case wasm.OpcodeVecF64x2Eq:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeF64x2Eq},
			)
		case wasm.OpcodeVecF64x2Ne:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeF64x2Ne},
			)
		case wasm.OpcodeVecF64x2Lt:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeF64x2Lt},
			)
		case wasm.OpcodeVecF64x2Gt:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeF64x2Gt},
			)
		case wasm.OpcodeVecF64x2Le:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeF64x2Le},
			)
		case wasm.OpcodeVecF64x2Ge:
			c.emit(
				OperationV128Cmp{Type: V128CmpTypeF64x2Ge},
			)
		case wasm.OpcodeVecI8x16Neg:
			c.emit(
				OperationV128Neg{Shape: ShapeI8x16},
			)
		case wasm.OpcodeVecI16x8Neg:
			c.emit(
				OperationV128Neg{Shape: ShapeI16x8},
			)
		case wasm.OpcodeVecI32x4Neg:
			c.emit(
				OperationV128Neg{Shape: ShapeI32x4},
			)
		case wasm.OpcodeVecI64x2Neg:
			c.emit(
				OperationV128Neg{Shape: ShapeI64x2},
			)
		case wasm.OpcodeVecF32x4Neg:
			c.emit(
				OperationV128Neg{Shape: ShapeF32x4},
			)
		case wasm.OpcodeVecF64x2Neg:
			c.emit(
				OperationV128Neg{Shape: ShapeF64x2},
			)
		case wasm.OpcodeVecI8x16Add:
			c.emit(
				OperationV128Add{Shape: ShapeI8x16},
			)
		case wasm.OpcodeVecI16x8Add:
			c.emit(
				OperationV128Add{Shape: ShapeI16x8},
			)
		case wasm.OpcodeVecI32x4Add:
			c.emit(
				OperationV128Add{Shape: ShapeI32x4},
			)
		case wasm.OpcodeVecI64x2Add:
			c.emit(
				OperationV128Add{Shape: ShapeI64x2},
			)
		case wasm.OpcodeVecF32x4Add:
			c.emit(
				OperationV128Add{Shape: ShapeF32x4},
			)
		case wasm.OpcodeVecF64x2Add:
			c.emit(
				OperationV128Add{Shape: ShapeF64x2},
			)
		case wasm.OpcodeVecI8x16Sub:
			c.emit(
				OperationV128Sub{Shape: ShapeI8x16},
			)
		case wasm.OpcodeVecI16x8Sub:
			c.emit(
				OperationV128Sub{Shape: ShapeI16x8},
			)
		case wasm.OpcodeVecI32x4Sub:
			c.emit(
				OperationV128Sub{Shape: ShapeI32x4},
			)
		case wasm.OpcodeVecI64x2Sub:
			c.emit(
				OperationV128Sub{Shape: ShapeI64x2},
			)
		case wasm.OpcodeVecF32x4Sub:
			c.emit(
				OperationV128Sub{Shape: ShapeF32x4},
			)
		case wasm.OpcodeVecF64x2Sub:
			c.emit(
				OperationV128Sub{Shape: ShapeF64x2},
			)
		case wasm.OpcodeVecI8x16AddSatS:
			c.emit(
				OperationV128AddSat{Shape: ShapeI8x16, Signed: true},
			)
		case wasm.OpcodeVecI8x16AddSatU:
			c.emit(
				OperationV128AddSat{Shape: ShapeI8x16, Signed: false},
			)
		case wasm.OpcodeVecI16x8AddSatS:
			c.emit(
				OperationV128AddSat{Shape: ShapeI16x8, Signed: true},
			)
		case wasm.OpcodeVecI16x8AddSatU:
			c.emit(
				OperationV128AddSat{Shape: ShapeI16x8, Signed: false},
			)
		case wasm.OpcodeVecI8x16SubSatS:
			c.emit(
				OperationV128SubSat{Shape: ShapeI8x16, Signed: true},
			)
		case wasm.OpcodeVecI8x16SubSatU:
			c.emit(
				OperationV128SubSat{Shape: ShapeI8x16, Signed: false},
			)
		case wasm.OpcodeVecI16x8SubSatS:
			c.emit(
				OperationV128SubSat{Shape: ShapeI16x8, Signed: true},
			)
		case wasm.OpcodeVecI16x8SubSatU:
			c.emit(
				OperationV128SubSat{Shape: ShapeI16x8, Signed: false},
			)
		case wasm.OpcodeVecI16x8Mul:
			c.emit(
				OperationV128Mul{Shape: ShapeI16x8},
			)
		case wasm.OpcodeVecI32x4Mul:
			c.emit(
				OperationV128Mul{Shape: ShapeI32x4},
			)
		case wasm.OpcodeVecI64x2Mul:
			c.emit(
				OperationV128Mul{Shape: ShapeI64x2},
			)
		case wasm.OpcodeVecF32x4Mul:
			c.emit(
				OperationV128Mul{Shape: ShapeF32x4},
			)
		case wasm.OpcodeVecF64x2Mul:
			c.emit(
				OperationV128Mul{Shape: ShapeF64x2},
			)
		case wasm.OpcodeVecF32x4Sqrt:
			c.emit(
				OperationV128Sqrt{Shape: ShapeF32x4},
			)
		case wasm.OpcodeVecF64x2Sqrt:
			c.emit(
				OperationV128Sqrt{Shape: ShapeF64x2},
			)
		case wasm.OpcodeVecF32x4Div:
			c.emit(
				OperationV128Div{Shape: ShapeF32x4},
			)
		case wasm.OpcodeVecF64x2Div:
			c.emit(
				OperationV128Div{Shape: ShapeF64x2},
			)
		case wasm.OpcodeVecI8x16Abs:
			c.emit(
				OperationV128Abs{Shape: ShapeI8x16},
			)
		case wasm.OpcodeVecI8x16Popcnt:
			c.emit(
				OperationV128Popcnt{},
			)
		case wasm.OpcodeVecI16x8Abs:
			c.emit(
				OperationV128Abs{Shape: ShapeI16x8},
			)
		case wasm.OpcodeVecI32x4Abs:
			c.emit(
				OperationV128Abs{Shape: ShapeI32x4},
			)
		case wasm.OpcodeVecI64x2Abs:
			c.emit(
				OperationV128Abs{Shape: ShapeI64x2},
			)
		case wasm.OpcodeVecF32x4Abs:
			c.emit(
				OperationV128Abs{Shape: ShapeF32x4},
			)
		case wasm.OpcodeVecF64x2Abs:
			c.emit(
				OperationV128Abs{Shape: ShapeF64x2},
			)
		case wasm.OpcodeVecI8x16MinS:
			c.emit(
				OperationV128Min{Signed: true, Shape: ShapeI8x16},
			)
		case wasm.OpcodeVecI8x16MinU:
			c.emit(
				OperationV128Min{Shape: ShapeI8x16},
			)
		case wasm.OpcodeVecI8x16MaxS:
			c.emit(
				OperationV128Max{Shape: ShapeI8x16, Signed: true},
			)
		case wasm.OpcodeVecI8x16MaxU:
			c.emit(
				OperationV128Max{Shape: ShapeI8x16},
			)
		case wasm.OpcodeVecI8x16AvgrU:
			c.emit(
				OperationV128AvgrU{Shape: ShapeI8x16},
			)
		case wasm.OpcodeVecI16x8MinS:
			c.emit(
				OperationV128Min{Signed: true, Shape: ShapeI16x8},
			)
		case wasm.OpcodeVecI16x8MinU:
			c.emit(
				OperationV128Min{Shape: ShapeI16x8},
			)
		case wasm.OpcodeVecI16x8MaxS:
			c.emit(
				OperationV128Max{Shape: ShapeI16x8, Signed: true},
			)
		case wasm.OpcodeVecI16x8MaxU:
			c.emit(
				OperationV128Max{Shape: ShapeI16x8},
			)
		case wasm.OpcodeVecI16x8AvgrU:
			c.emit(
				OperationV128AvgrU{Shape: ShapeI16x8},
			)
		case wasm.OpcodeVecI32x4MinS:
			c.emit(
				OperationV128Min{Signed: true, Shape: ShapeI32x4},
			)
		case wasm.OpcodeVecI32x4MinU:
			c.emit(
				OperationV128Min{Shape: ShapeI32x4},
			)
		case wasm.OpcodeVecI32x4MaxS:
			c.emit(
				OperationV128Max{Shape: ShapeI32x4, Signed: true},
			)
		case wasm.OpcodeVecI32x4MaxU:
			c.emit(
				OperationV128Max{Shape: ShapeI32x4},
			)
		case wasm.OpcodeVecF32x4Min:
			c.emit(
				OperationV128Min{Shape: ShapeF32x4},
			)
		case wasm.OpcodeVecF32x4Max:
			c.emit(
				OperationV128Max{Shape: ShapeF32x4},
			)
		case wasm.OpcodeVecF64x2Min:
			c.emit(
				OperationV128Min{Shape: ShapeF64x2},
			)
		case wasm.OpcodeVecF64x2Max:
			c.emit(
				OperationV128Max{Shape: ShapeF64x2},
			)
		case wasm.OpcodeVecF32x4Pmin:
			c.emit(
				OperationV128Pmin{Shape: ShapeF32x4},
			)
		case wasm.OpcodeVecF32x4Pmax:
			c.emit(
				OperationV128Pmax{Shape: ShapeF32x4},
			)
		case wasm.OpcodeVecF64x2Pmin:
			c.emit(
				OperationV128Pmin{Shape: ShapeF64x2},
			)
		case wasm.OpcodeVecF64x2Pmax:
			c.emit(
				OperationV128Pmax{Shape: ShapeF64x2},
			)
		case wasm.OpcodeVecF32x4Ceil:
			c.emit(
				OperationV128Ceil{Shape: ShapeF32x4},
			)
		case wasm.OpcodeVecF32x4Floor:
			c.emit(
				OperationV128Floor{Shape: ShapeF32x4},
			)
		case wasm.OpcodeVecF32x4Trunc:
			c.emit(
				OperationV128Trunc{Shape: ShapeF32x4},
			)
		case wasm.OpcodeVecF32x4Nearest:
			c.emit(
				OperationV128Nearest{Shape: ShapeF32x4},
			)
		case wasm.OpcodeVecF64x2Ceil:
			c.emit(
				OperationV128Ceil{Shape: ShapeF64x2},
			)
		case wasm.OpcodeVecF64x2Floor:
			c.emit(
				OperationV128Floor{Shape: ShapeF64x2},
			)
		case wasm.OpcodeVecF64x2Trunc:
			c.emit(
				OperationV128Trunc{Shape: ShapeF64x2},
			)
		case wasm.OpcodeVecF64x2Nearest:
			c.emit(
				OperationV128Nearest{Shape: ShapeF64x2},
			)
		case wasm.OpcodeVecI16x8ExtendLowI8x16S:
			c.emit(
				OperationV128Extend{OriginShape: ShapeI8x16, Signed: true, UseLow: true},
			)
		case wasm.OpcodeVecI16x8ExtendHighI8x16S:
			c.emit(
				OperationV128Extend{OriginShape: ShapeI8x16, Signed: true, UseLow: false},
			)
		case wasm.OpcodeVecI16x8ExtendLowI8x16U:
			c.emit(
				OperationV128Extend{OriginShape: ShapeI8x16, Signed: false, UseLow: true},
			)
		case wasm.OpcodeVecI16x8ExtendHighI8x16U:
			c.emit(
				OperationV128Extend{OriginShape: ShapeI8x16, Signed: false, UseLow: false},
			)
		case wasm.OpcodeVecI32x4ExtendLowI16x8S:
			c.emit(
				OperationV128Extend{OriginShape: ShapeI16x8, Signed: true, UseLow: true},
			)
		case wasm.OpcodeVecI32x4ExtendHighI16x8S:
			c.emit(
				OperationV128Extend{OriginShape: ShapeI16x8, Signed: true, UseLow: false},
			)
		case wasm.OpcodeVecI32x4ExtendLowI16x8U:
			c.emit(
				OperationV128Extend{OriginShape: ShapeI16x8, Signed: false, UseLow: true},
			)
		case wasm.OpcodeVecI32x4ExtendHighI16x8U:
			c.emit(
				OperationV128Extend{OriginShape: ShapeI16x8, Signed: false, UseLow: false},
			)
		case wasm.OpcodeVecI64x2ExtendLowI32x4S:
			c.emit(
				OperationV128Extend{OriginShape: ShapeI32x4, Signed: true, UseLow: true},
			)
		case wasm.OpcodeVecI64x2ExtendHighI32x4S:
			c.emit(
				OperationV128Extend{OriginShape: ShapeI32x4, Signed: true, UseLow: false},
			)
		case wasm.OpcodeVecI64x2ExtendLowI32x4U:
			c.emit(
				OperationV128Extend{OriginShape: ShapeI32x4, Signed: false, UseLow: true},
			)
		case wasm.OpcodeVecI64x2ExtendHighI32x4U:
			c.emit(
				OperationV128Extend{OriginShape: ShapeI32x4, Signed: false, UseLow: false},
			)
		case wasm.OpcodeVecI16x8Q15mulrSatS:
			c.emit(
				OperationV128Q15mulrSatS{},
			)
		case wasm.OpcodeVecI16x8ExtMulLowI8x16S:
			c.emit(
				OperationV128ExtMul{OriginShape: ShapeI8x16, Signed: true, UseLow: true},
			)
		case wasm.OpcodeVecI16x8ExtMulHighI8x16S:
			c.emit(
				OperationV128ExtMul{OriginShape: ShapeI8x16, Signed: true, UseLow: false},
			)
		case wasm.OpcodeVecI16x8ExtMulLowI8x16U:
			c.emit(
				OperationV128ExtMul{OriginShape: ShapeI8x16, Signed: false, UseLow: true},
			)
		case wasm.OpcodeVecI16x8ExtMulHighI8x16U:
			c.emit(
				OperationV128ExtMul{OriginShape: ShapeI8x16, Signed: false, UseLow: false},
			)
		case wasm.OpcodeVecI32x4ExtMulLowI16x8S:
			c.emit(
				OperationV128ExtMul{OriginShape: ShapeI16x8, Signed: true, UseLow: true},
			)
		case wasm.OpcodeVecI32x4ExtMulHighI16x8S:
			c.emit(
				OperationV128ExtMul{OriginShape: ShapeI16x8, Signed: true, UseLow: false},
			)
		case wasm.OpcodeVecI32x4ExtMulLowI16x8U:
			c.emit(
				OperationV128ExtMul{OriginShape: ShapeI16x8, Signed: false, UseLow: true},
			)
		case wasm.OpcodeVecI32x4ExtMulHighI16x8U:
			c.emit(
				OperationV128ExtMul{OriginShape: ShapeI16x8, Signed: false, UseLow: false},
			)
		case wasm.OpcodeVecI64x2ExtMulLowI32x4S:
			c.emit(
				OperationV128ExtMul{OriginShape: ShapeI32x4, Signed: true, UseLow: true},
			)
		case wasm.OpcodeVecI64x2ExtMulHighI32x4S:
			c.emit(
				OperationV128ExtMul{OriginShape: ShapeI32x4, Signed: true, UseLow: false},
			)
		case wasm.OpcodeVecI64x2ExtMulLowI32x4U:
			c.emit(
				OperationV128ExtMul{OriginShape: ShapeI32x4, Signed: false, UseLow: true},
			)
		case wasm.OpcodeVecI64x2ExtMulHighI32x4U:
			c.emit(
				OperationV128ExtMul{OriginShape: ShapeI32x4, Signed: false, UseLow: false},
			)
		case wasm.OpcodeVecI16x8ExtaddPairwiseI8x16S:
			c.emit(
				OperationV128ExtAddPairwise{OriginShape: ShapeI8x16, Signed: true},
			)
		case wasm.OpcodeVecI16x8ExtaddPairwiseI8x16U:
			c.emit(
				OperationV128ExtAddPairwise{OriginShape: ShapeI8x16, Signed: false},
			)
		case wasm.OpcodeVecI32x4ExtaddPairwiseI16x8S:
			c.emit(
				OperationV128ExtAddPairwise{OriginShape: ShapeI16x8, Signed: true},
			)
		case wasm.OpcodeVecI32x4ExtaddPairwiseI16x8U:
			c.emit(
				OperationV128ExtAddPairwise{OriginShape: ShapeI16x8, Signed: false},
			)
		case wasm.OpcodeVecF64x2PromoteLowF32x4Zero:
			c.emit(
				OperationV128FloatPromote{},
			)
		case wasm.OpcodeVecF32x4DemoteF64x2Zero:
			c.emit(
				OperationV128FloatDemote{},
			)
		case wasm.OpcodeVecF32x4ConvertI32x4S:
			c.emit(
				OperationV128FConvertFromI{DestinationShape: ShapeF32x4, Signed: true},
			)
		case wasm.OpcodeVecF32x4ConvertI32x4U:
			c.emit(
				OperationV128FConvertFromI{DestinationShape: ShapeF32x4, Signed: false},
			)
		case wasm.OpcodeVecF64x2ConvertLowI32x4S:
			c.emit(
				OperationV128FConvertFromI{DestinationShape: ShapeF64x2, Signed: true},
			)
		case wasm.OpcodeVecF64x2ConvertLowI32x4U:
			c.emit(
				OperationV128FConvertFromI{DestinationShape: ShapeF64x2, Signed: false},
			)
		case wasm.OpcodeVecI32x4DotI16x8S:
			c.emit(
				OperationV128Dot{},
			)
		case wasm.OpcodeVecI8x16NarrowI16x8S:
			c.emit(
				OperationV128Narrow{OriginShape: ShapeI16x8, Signed: true},
			)
		case wasm.OpcodeVecI8x16NarrowI16x8U:
			c.emit(
				OperationV128Narrow{OriginShape: ShapeI16x8, Signed: false},
			)
		case wasm.OpcodeVecI16x8NarrowI32x4S:
			c.emit(
				OperationV128Narrow{OriginShape: ShapeI32x4, Signed: true},
			)
		case wasm.OpcodeVecI16x8NarrowI32x4U:
			c.emit(
				OperationV128Narrow{OriginShape: ShapeI32x4, Signed: false},
			)
		case wasm.OpcodeVecI32x4TruncSatF32x4S:
			c.emit(
				OperationV128ITruncSatFromF{OriginShape: ShapeF32x4, Signed: true},
			)
		case wasm.OpcodeVecI32x4TruncSatF32x4U:
			c.emit(
				OperationV128ITruncSatFromF{OriginShape: ShapeF32x4, Signed: false},
			)
		case wasm.OpcodeVecI32x4TruncSatF64x2SZero:
			c.emit(
				OperationV128ITruncSatFromF{OriginShape: ShapeF64x2, Signed: true},
			)
		case wasm.OpcodeVecI32x4TruncSatF64x2UZero:
			c.emit(
				OperationV128ITruncSatFromF{OriginShape: ShapeF64x2, Signed: false},
			)
		default:
			return fmt.Errorf("unsupported vector instruction in wazeroir: %s", wasm.VectorInstructionName(vecOp))
		}
	default:
		return fmt.Errorf("unsupported instruction in wazeroir: 0x%x", op)
	}

	// Move the program counter to point to the next instruction.
	c.pc++
	return nil
}

func (c *compiler) nextID() (id uint32) {
	id = c.currentID + 1
	c.currentID++
	return
}

func (c *compiler) applyToStack(opcode wasm.Opcode) (index uint32, err error) {
	switch opcode {
	case
		// These are the opcodes that is coupled with "index"　immediate
		// and it DOES affect the signature of opcode.
		wasm.OpcodeCall,
		wasm.OpcodeCallIndirect,
		wasm.OpcodeLocalGet,
		wasm.OpcodeLocalSet,
		wasm.OpcodeLocalTee,
		wasm.OpcodeGlobalGet,
		wasm.OpcodeGlobalSet:
		// Assumes that we are at the opcode now so skip it before read immediates.
		v, num, err := leb128.LoadUint32(c.body[c.pc+1:])
		if err != nil {
			return 0, fmt.Errorf("reading immediates: %w", err)
		}
		c.pc += num
		index = v
	default:
		// Note that other opcodes are free of index
		// as it doesn't affect the signature of opt code.
		// In other words, the "index" argument of wasmOpcodeSignature
		// is ignored there.
	}

	if c.unreachableState.on {
		return 0, nil
	}

	// Retrieve the signature of the opcode.
	s, err := c.wasmOpcodeSignature(opcode, index)
	if err != nil {
		return 0, err
	}

	// Manipulate the stack according to the signature.
	// Note that the following algorithm assumes that
	// the unknown type is unique in the signature,
	// and is determined by the actual type on the stack.
	// The determined type is stored in this typeParam.
	var typeParam UnsignedType
	var typeParamFound bool
	for i := range s.in {
		want := s.in[len(s.in)-1-i]
		actual := c.stackPop()
		if want == UnsignedTypeUnknown && typeParamFound {
			want = typeParam
		} else if want == UnsignedTypeUnknown {
			want = actual
			typeParam = want
			typeParamFound = true
		}
		if want != actual {
			return 0, fmt.Errorf("input signature mismatch: want %s but have %s", want, actual)
		}
	}

	for _, target := range s.out {
		if target == UnsignedTypeUnknown && !typeParamFound {
			return 0, fmt.Errorf("cannot determine type of unknown result")
		} else if target == UnsignedTypeUnknown {
			c.stackPush(typeParam)
		} else {
			c.stackPush(target)
		}
	}

	return index, nil
}

func (c *compiler) stackPeek() (ret UnsignedType) {
	ret = c.stack[len(c.stack)-1]
	return
}

func (c *compiler) stackPop() (ret UnsignedType) {
	// No need to check stack bound
	// as we can assume that all the operations
	// are valid thanks to validateFunction
	// at module validation phase.
	ret = c.stack[len(c.stack)-1]
	c.stack = c.stack[:len(c.stack)-1]
	return
}

func (c *compiler) stackPush(ts UnsignedType) {
	c.stack = append(c.stack, ts)
}

// emit adds the operations into the result.
func (c *compiler) emit(ops ...Operation) {
	if !c.unreachableState.on {
		for _, op := range ops {
			switch o := op.(type) {
			case OperationDrop:
				// If the drop range is nil,
				// we could remove such operations.
				// That happens when drop operation is unnecessary.
				// i.e. when there's no need to adjust stack before jmp.
				if o.Depth == nil {
					continue
				}
			}
			c.result.Operations = append(c.result.Operations, op)
			if c.needSourceOffset {
				c.result.IROperationSourceOffsetsInWasmBinary = append(c.result.IROperationSourceOffsetsInWasmBinary,
					c.currentOpPC+c.bodyOffsetInCodeSection)
			}
			if false {
				fmt.Printf("emitting ")
				formatOperation(os.Stdout, op)
			}
		}
	}
}

// Emit const expression with default values of the given type.
func (c *compiler) emitDefaultValue(t wasm.ValueType) {
	switch t {
	case wasm.ValueTypeI32:
		c.stackPush(UnsignedTypeI32)
		c.emit(OperationConstI32{Value: 0})
	case wasm.ValueTypeI64, wasm.ValueTypeExternref, wasm.ValueTypeFuncref:
		c.stackPush(UnsignedTypeI64)
		c.emit(OperationConstI64{Value: 0})
	case wasm.ValueTypeF32:
		c.stackPush(UnsignedTypeF32)
		c.emit(OperationConstF32{Value: 0})
	case wasm.ValueTypeF64:
		c.stackPush(UnsignedTypeF64)
		c.emit(OperationConstF64{Value: 0})
	case wasm.ValueTypeV128:
		c.stackPush(UnsignedTypeV128)
		c.emit(OperationV128Const{Hi: 0, Lo: 0})
	}
}

// Returns the "depth" (starting from top of the stack)
// of the n-th local.
func (c *compiler) localDepth(index wasm.Index) int {
	height, ok := c.localIndexToStackHeightInUint64[index]
	if !ok {
		panic("BUG")
	}
	return c.stackLenInUint64(len(c.stack)) - 1 - int(height)
}

func (c *compiler) localType(index wasm.Index) (t wasm.ValueType) {
	if params := uint32(len(c.sig.Params)); index < params {
		t = c.sig.Params[index]
	} else {
		t = c.localTypes[index-params]
	}
	return
}

// getFrameDropRange returns the range (starting from top of the stack) that spans across the (uint64) stack. The range is
// supposed to be dropped from the stack when the given frame exists or branch into it.
//
// * frame is the control frame which the call-site is trying to branch into or exit.
// * isEnd true if the call-site is handling wasm.OpcodeEnd.
func (c *compiler) getFrameDropRange(frame *controlFrame, isEnd bool) *InclusiveRange {
	var start int
	if !isEnd && frame.kind == controlFrameKindLoop {
		// If this is not End and the call-site is trying to branch into the Loop control frame,
		// we have to start executing from the beginning of the loop block.
		// Therefore, we have to pass the inputs to the frame.
		start = frame.blockType.ParamNumInUint64
	} else {
		start = frame.blockType.ResultNumInUint64
	}
	var end int
	if frame.kind == controlFrameKindFunction {
		// On the function return, we eliminate all the contents on the stack
		// including locals (existing below of frame.originalStackLen)
		end = c.stackLenInUint64(len(c.stack)) - 1
	} else {
		end = c.stackLenInUint64(len(c.stack)) - 1 - c.stackLenInUint64(frame.originalStackLenWithoutParam)
	}
	if start <= end {
		return &InclusiveRange{Start: start, End: end}
	}
	return nil
}

func (c *compiler) stackLenInUint64(ceil int) (ret int) {
	for i := 0; i < ceil; i++ {
		if c.stack[i] == UnsignedTypeV128 {
			ret += 2
		} else {
			ret++
		}
	}
	return
}

func (c *compiler) readMemoryArg(tag string) (MemoryArg, error) {
	c.result.UsesMemory = true
	alignment, num, err := leb128.LoadUint32(c.body[c.pc+1:])
	if err != nil {
		return MemoryArg{}, fmt.Errorf("reading alignment for %s: %w", tag, err)
	}
	c.pc += num
	offset, num, err := leb128.LoadUint32(c.body[c.pc+1:])
	if err != nil {
		return MemoryArg{}, fmt.Errorf("reading offset for %s: %w", tag, err)
	}
	c.pc += num
	return MemoryArg{Offset: offset, Alignment: alignment}, nil
}
