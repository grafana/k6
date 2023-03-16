package compiler

// This file implements the compiler for amd64/x86_64 target.
// Please refer to https://www.felixcloutier.com/x86/index.html
// if unfamiliar with amd64 instructions used here.

import (
	"bytes"
	"fmt"
	"math"

	"github.com/tetratelabs/wazero/internal/asm"
	"github.com/tetratelabs/wazero/internal/asm/amd64"
	"github.com/tetratelabs/wazero/internal/platform"
	"github.com/tetratelabs/wazero/internal/u32"
	"github.com/tetratelabs/wazero/internal/u64"
	"github.com/tetratelabs/wazero/internal/wasm"
	"github.com/tetratelabs/wazero/internal/wazeroir"
)

var (
	minimum32BitSignedInt                  int32  = math.MinInt32
	maximum32BitSignedInt                  int32  = math.MaxInt32
	maximum32BitUnsignedInt                uint32 = math.MaxUint32
	minimum64BitSignedInt                  int64  = math.MinInt64
	maximum64BitSignedInt                  int64  = math.MaxInt64
	maximum64BitUnsignedInt                uint64 = math.MaxUint64
	float32SignBitMask                     uint32 = 1 << 31
	float32RestBitMask                            = ^float32SignBitMask
	float64SignBitMask                     uint64 = 1 << 63
	float64RestBitMask                            = ^float64SignBitMask
	float32ForMinimumSigned32bitInteger           = uint32(0xCF00_0000)
	float64ForMinimumSigned32bitInteger           = uint64(0xC1E0_0000_0020_0000)
	float32ForMinimumSigned64bitInteger           = uint32(0xDF00_0000)
	float64ForMinimumSigned64bitInteger           = uint64(0xC3E0_0000_0000_0000)
	float32ForMaximumSigned32bitIntPlusOne        = uint32(0x4F00_0000)
	float64ForMaximumSigned32bitIntPlusOne        = uint64(0x41E0_0000_0000_0000)
	float32ForMaximumSigned64bitIntPlusOne        = uint32(0x5F00_0000)
	float64ForMaximumSigned64bitIntPlusOne        = uint64(0x43E0_0000_0000_0000)
)

var (
	// amd64ReservedRegisterForCallEngine: pointer to callEngine (i.e. *callEngine as uintptr)
	amd64ReservedRegisterForCallEngine = amd64.RegR13
	// amd64ReservedRegisterForStackBasePointerAddress: stack base pointer's address (callEngine.stackBasePointer) in the current function call.
	amd64ReservedRegisterForStackBasePointerAddress = amd64.RegR14
	// amd64ReservedRegisterForMemory: pointer to the memory slice's data (i.e. &memory.Buffer[0] as uintptr).
	amd64ReservedRegisterForMemory = amd64.RegR15
)

var (
	amd64UnreservedVectorRegisters = []asm.Register{ //nolint
		amd64.RegX0, amd64.RegX1, amd64.RegX2, amd64.RegX3,
		amd64.RegX4, amd64.RegX5, amd64.RegX6, amd64.RegX7,
		amd64.RegX8, amd64.RegX9, amd64.RegX10, amd64.RegX11,
		amd64.RegX12, amd64.RegX13, amd64.RegX14, amd64.RegX15,
	}
	// Note that we never invoke "call" instruction,
	// so we don't need to care about the calling convention.
	// TODO: Maybe it is safe just save rbp, rsp somewhere
	// in Go-allocated variables, and reuse these registers
	// in compiled functions and write them back before returns.
	amd64UnreservedGeneralPurposeRegisters = []asm.Register{ //nolint
		amd64.RegAX, amd64.RegCX, amd64.RegDX, amd64.RegBX,
		amd64.RegSI, amd64.RegDI, amd64.RegR8, amd64.RegR9,
		amd64.RegR10, amd64.RegR11, amd64.RegR12,
	}
)

// amd64CallingConventionDestinationFunctionModuleInstanceAddressRegister holds *wasm.ModuleInstance of the
// next executing function instance. The value is set and used when making function calls
// or function returns in the ModuleContextInitialization. See compileModuleContextInitialization.
var amd64CallingConventionDestinationFunctionModuleInstanceAddressRegister = amd64.RegR12

func (c *amd64Compiler) String() string {
	return c.locationStack.String()
}

// compileNOP implements compiler.compileNOP for the amd64 architecture.
func (c *amd64Compiler) compileNOP() asm.Node {
	return c.assembler.CompileStandAlone(amd64.NOP)
}

type amd64Compiler struct {
	assembler   amd64.Assembler
	ir          *wazeroir.CompilationResult
	cpuFeatures platform.CpuFeatureFlags
	// locationStack holds the state of wazeroir virtual stack.
	// and each item is either placed in register or the actual memory stack.
	locationStack runtimeValueLocationStack
	// labels hold per wazeroir label specific information in this function.
	labels map[wazeroir.LabelID]*amd64LabelInfo
	// stackPointerCeil is the greatest stack pointer value (from runtimeValueLocationStack) seen during compilation.
	stackPointerCeil uint64
	// onStackPointerCeilDeterminedCallBack hold a callback which are called when the max stack pointer is determined BEFORE generating native code.
	onStackPointerCeilDeterminedCallBack func(stackPointerCeil uint64)
	withListener                         bool
}

func newAmd64Compiler() compiler {
	c := &amd64Compiler{
		assembler:     amd64.NewAssembler(),
		locationStack: newRuntimeValueLocationStack(),
		cpuFeatures:   platform.CpuFeatures,
	}
	return c
}

func (c *amd64Compiler) Init(ir *wazeroir.CompilationResult, withListener bool) {
	assembler, vstack := c.assembler, c.locationStack
	assembler.Reset()
	vstack.reset()
	*c = amd64Compiler{
		labels:       map[wazeroir.LabelID]*amd64LabelInfo{},
		ir:           ir,
		cpuFeatures:  c.cpuFeatures,
		withListener: withListener,
	}
	c.assembler, c.locationStack = assembler, vstack
}

// runtimeValueLocationStack implements compilerImpl.runtimeValueLocationStack for the amd64 architecture.
func (c *amd64Compiler) runtimeValueLocationStack() *runtimeValueLocationStack {
	return &c.locationStack
}

// setLocationStack sets the given runtimeValueLocationStack to .locationStack field,
// while allowing us to track runtimeValueLocationStack.stackPointerCeil across multiple stacks.
// This is called when we branch into different block.
func (c *amd64Compiler) setLocationStack(newStack runtimeValueLocationStack) {
	if c.stackPointerCeil < c.locationStack.stackPointerCeil {
		c.stackPointerCeil = c.locationStack.stackPointerCeil
	}
	c.locationStack = newStack
}

// pushRuntimeValueLocationOnRegister implements compiler.pushRuntimeValueLocationOnRegister for amd64.
func (c *amd64Compiler) pushRuntimeValueLocationOnRegister(reg asm.Register, vt runtimeValueType) (ret *runtimeValueLocation) {
	ret = c.locationStack.pushRuntimeValueLocationOnRegister(reg, vt)
	c.locationStack.markRegisterUsed(reg)
	return
}

// pushVectorRuntimeValueLocationOnRegister implements compiler.pushVectorRuntimeValueLocationOnRegister for amd64.
func (c *amd64Compiler) pushVectorRuntimeValueLocationOnRegister(reg asm.Register) (lowerBitsLocation *runtimeValueLocation) {
	lowerBitsLocation = c.locationStack.pushRuntimeValueLocationOnRegister(reg, runtimeValueTypeV128Lo)
	c.locationStack.pushRuntimeValueLocationOnRegister(reg, runtimeValueTypeV128Hi)
	c.locationStack.markRegisterUsed(reg)
	return
}

type amd64LabelInfo struct {
	// initialInstruction is the initial instruction for this label so other block can jump into it.
	initialInstruction asm.Node
	// initialStack is the initial value location stack from which we start compiling this label.
	initialStack runtimeValueLocationStack
	// labelBeginningCallbacks holds callbacks should to be called with initialInstruction
	labelBeginningCallbacks []func(asm.Node)
}

func (c *amd64Compiler) label(labelID wazeroir.LabelID) *amd64LabelInfo {
	ret, ok := c.labels[labelID]
	if ok {
		return ret
	}
	c.labels[labelID] = &amd64LabelInfo{}
	return c.labels[labelID]
}

// compileBuiltinFunctionCheckExitCode implements compiler.compileBuiltinFunctionCheckExitCode for the amd64 architecture.
func (c *amd64Compiler) compileBuiltinFunctionCheckExitCode() error {
	if err := c.compileCallBuiltinFunction(builtinFunctionIndexCheckExitCode); err != nil {
		return err
	}

	// After the function call, we have to initialize the stack base pointer and memory reserved registers.
	c.compileReservedStackBasePointerInitialization()
	c.compileReservedMemoryPointerInitialization()
	return nil
}

// compileGoDefinedHostFunction constructs the entire code to enter the host function implementation,
// and return to the caller.
func (c *amd64Compiler) compileGoDefinedHostFunction() error {
	// First we must update the location stack to reflect the number of host function inputs.
	c.locationStack.init(c.ir.Signature)

	if c.withListener {
		if err := c.compileCallBuiltinFunction(builtinFunctionIndexFunctionListenerBefore); err != nil {
			return err
		}
	}

	if err := c.compileCallGoHostFunction(); err != nil {
		return err
	}

	// Initializes the reserved stack base pointer which is used to retrieve the call frame stack.
	c.compileReservedStackBasePointerInitialization()

	// Go function can change the module state in arbitrary way, so we have to force
	// the callEngine.moduleContext initialization on the function return. To do so,
	// we zero-out callEngine.moduleInstanceAddress.
	c.assembler.CompileConstToMemory(amd64.MOVQ,
		0, amd64ReservedRegisterForCallEngine, callEngineModuleContextModuleInstanceAddressOffset)
	return c.compileReturnFunction()
}

// compile implements compiler.compile for the amd64 architecture.
func (c *amd64Compiler) compile() (code []byte, stackPointerCeil uint64, err error) {
	// c.stackPointerCeil tracks the stack pointer ceiling (max seen) value across all runtimeValueLocationStack(s)
	// used for all labels (via setLocationStack), excluding the current one.
	// Hence, we check here if the final block's max one exceeds the current c.stackPointerCeil.
	stackPointerCeil = c.stackPointerCeil
	if stackPointerCeil < c.locationStack.stackPointerCeil {
		stackPointerCeil = c.locationStack.stackPointerCeil
	}

	// Now that the max stack pointer is determined, we are invoking the callback.
	// Note this MUST be called before Assemble() below.
	if c.onStackPointerCeilDeterminedCallBack != nil {
		c.onStackPointerCeilDeterminedCallBack(stackPointerCeil)
		c.onStackPointerCeilDeterminedCallBack = nil
	}

	code, err = c.assembler.Assemble()
	if err != nil {
		return
	}

	code, err = platform.MmapCodeSegment(bytes.NewReader(code), len(code))
	return
}

// compileUnreachable implements compiler.compileUnreachable for the amd64 architecture.
func (c *amd64Compiler) compileUnreachable() error {
	c.compileExitFromNativeCode(nativeCallStatusCodeUnreachable)
	return nil
}

// compileSet implements compiler.compileSet for the amd64 architecture.
func (c *amd64Compiler) compileSet(o wazeroir.OperationSet) error {
	setTargetIndex := int(c.locationStack.sp) - 1 - o.Depth

	if o.IsTargetVector {
		_ = c.locationStack.pop() // ignore the higher 64-bits.
	}
	v := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(v); err != nil {
		return err
	}

	targetLocation := &c.locationStack.stack[setTargetIndex]
	if targetLocation.onRegister() {
		// We no longer need the register previously used by the target location.
		c.locationStack.markRegisterUnused(targetLocation.register)
	}

	reg := v.register
	targetLocation.setRegister(reg)
	targetLocation.valueType = v.valueType
	if o.IsTargetVector {
		hi := &c.locationStack.stack[setTargetIndex+1]
		hi.setRegister(reg)
	}
	return nil
}

// compileGlobalGet implements compiler.compileGlobalGet for the amd64 architecture.
func (c *amd64Compiler) compileGlobalGet(o wazeroir.OperationGlobalGet) error {
	if err := c.maybeCompileMoveTopConditionalToGeneralPurposeRegister(); err != nil {
		return err
	}

	intReg, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}

	// First, move the pointer to the global slice into the allocated register.
	c.assembler.CompileMemoryToRegister(amd64.MOVQ, amd64ReservedRegisterForCallEngine, callEngineModuleContextGlobalElement0AddressOffset, intReg)

	// Now, move the location of the global instance into the register.
	c.assembler.CompileMemoryToRegister(amd64.MOVQ, intReg, 8*int64(o.Index), intReg)

	// When an integer, reuse the pointer register for the value. Otherwise, allocate a float register for it.
	valueReg := intReg
	var vt runtimeValueType
	var inst asm.Instruction
	switch c.ir.Globals[o.Index].ValType {
	case wasm.ValueTypeI32:
		inst = amd64.MOVL
		vt = runtimeValueTypeI32
	case wasm.ValueTypeI64, wasm.ValueTypeExternref, wasm.ValueTypeFuncref:
		inst = amd64.MOVQ
		vt = runtimeValueTypeI64
	case wasm.ValueTypeF32:
		inst = amd64.MOVL
		vt = runtimeValueTypeF32
		valueReg, err = c.allocateRegister(registerTypeVector)
		if err != nil {
			return err
		}
	case wasm.ValueTypeF64:
		inst = amd64.MOVQ
		vt = runtimeValueTypeF64
		valueReg, err = c.allocateRegister(registerTypeVector)
		if err != nil {
			return err
		}
	case wasm.ValueTypeV128:
		inst = amd64.MOVDQU
		vt = runtimeValueTypeV128Lo
		valueReg, err = c.allocateRegister(registerTypeVector)
		if err != nil {
			return err
		}
	default:
		panic("BUG: unknown runtime value type")
	}

	// Using the register holding the pointer to the target instance, move its value into a register.
	c.assembler.CompileMemoryToRegister(inst, intReg, globalInstanceValueOffset, valueReg)

	// Record that the retrieved global value on the top of the stack is now in a register.
	if vt == runtimeValueTypeV128Lo {
		c.pushVectorRuntimeValueLocationOnRegister(valueReg)
	} else {
		c.pushRuntimeValueLocationOnRegister(valueReg, vt)
	}
	return nil
}

// compileGlobalSet implements compiler.compileGlobalSet for the amd64 architecture.
func (c *amd64Compiler) compileGlobalSet(o wazeroir.OperationGlobalSet) error {
	wasmValueType := c.ir.Globals[o.Index].ValType
	isV128 := wasmValueType == wasm.ValueTypeV128

	// First, move the value to set into a temporary register.
	val := c.locationStack.pop()
	if isV128 {
		// The previous val is higher 64-bits, and have to use lower 64-bit's runtimeValueLocation for allocation, etc.
		val = c.locationStack.pop()
	}
	if err := c.compileEnsureOnRegister(val); err != nil {
		return err
	}

	// Allocate a register to hold the memory location of the target global instance.
	intReg, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}

	// First, move the pointer to the global slice into the allocated register.
	c.assembler.CompileMemoryToRegister(amd64.MOVQ, amd64ReservedRegisterForCallEngine, callEngineModuleContextGlobalElement0AddressOffset, intReg)

	// Now, move the location of the global instance into the register.
	c.assembler.CompileMemoryToRegister(amd64.MOVQ, intReg, 8*int64(o.Index), intReg)

	// Now ready to write the value to the global instance location.
	var inst asm.Instruction
	if isV128 {
		inst = amd64.MOVDQU
	} else if wasmValueType == wasm.ValueTypeI32 || wasmValueType == wasm.ValueTypeF32 {
		inst = amd64.MOVL
	} else {
		inst = amd64.MOVQ
	}
	c.assembler.CompileRegisterToMemory(inst, val.register, intReg, globalInstanceValueOffset)

	// Since the value is now written to memory, release the value register.
	c.locationStack.releaseRegister(val)
	return nil
}

// compileBr implements compiler.compileBr for the amd64 architecture.
func (c *amd64Compiler) compileBr(o wazeroir.OperationBr) error {
	if err := c.maybeCompileMoveTopConditionalToGeneralPurposeRegister(); err != nil {
		return err
	}
	return c.branchInto(o.Target)
}

// branchInto adds instruction necessary to jump into the given branch target.
func (c *amd64Compiler) branchInto(target wazeroir.Label) error {
	if target.IsReturnTarget() {
		return c.compileReturnFunction()
	} else {
		labelID := target.ID()
		if c.ir.LabelCallers[labelID] > 1 {
			// We can only re-use register state if when there's a single call-site.
			// Release existing values on registers to the stack if there's multiple ones to have
			// the consistent value location state at the beginning of label.
			if err := c.compileReleaseAllRegistersToStack(); err != nil {
				return err
			}
		}
		// Set the initial stack of the target label, so we can start compiling the label
		// with the appropriate value locations. Note we clone the stack here as we maybe
		// manipulate the stack before compiler reaches the label.
		targetLabel := c.label(labelID)
		if !targetLabel.initialStack.initialized() {
			// It seems unnecessary to clone as branchInto is always the tail of the current block.
			// TODO: verify ^^.
			targetLabel.initialStack = c.locationStack.clone()
		}
		jmp := c.assembler.CompileJump(amd64.JMP)
		c.assignJumpTarget(labelID, jmp)
	}
	return nil
}

// compileBrIf implements compiler.compileBrIf for the amd64 architecture.
func (c *amd64Compiler) compileBrIf(o wazeroir.OperationBrIf) error {
	cond := c.locationStack.pop()
	var jmpWithCond asm.Node
	if cond.onConditionalRegister() {
		var inst asm.Instruction
		switch cond.conditionalRegister {
		case amd64.ConditionalRegisterStateE:
			inst = amd64.JEQ
		case amd64.ConditionalRegisterStateNE:
			inst = amd64.JNE
		case amd64.ConditionalRegisterStateS:
			inst = amd64.JMI
		case amd64.ConditionalRegisterStateNS:
			inst = amd64.JPL
		case amd64.ConditionalRegisterStateG:
			inst = amd64.JGT
		case amd64.ConditionalRegisterStateGE:
			inst = amd64.JGE
		case amd64.ConditionalRegisterStateL:
			inst = amd64.JLT
		case amd64.ConditionalRegisterStateLE:
			inst = amd64.JLE
		case amd64.ConditionalRegisterStateA:
			inst = amd64.JHI
		case amd64.ConditionalRegisterStateAE:
			inst = amd64.JCC
		case amd64.ConditionalRegisterStateB:
			inst = amd64.JCS
		case amd64.ConditionalRegisterStateBE:
			inst = amd64.JLS
		}
		jmpWithCond = c.assembler.CompileJump(inst)
	} else {
		// Usually the comparison operand for br_if is on the conditional register,
		// but in some cases, they are on the stack or register.
		// For example, the following code
		// 		i64.const 1
		//      local.get 1
		//      i64.add
		//      br_if ....
		// will try to use the result of i64.add, which resides on the (virtual) stack,
		// as the operand for br_if instruction.
		if err := c.compileEnsureOnRegister(cond); err != nil {
			return err
		}
		// Check if the value not equals zero.
		c.assembler.CompileRegisterToConst(amd64.CMPQ, cond.register, 0)

		// Emit jump instruction which jumps when the value does not equals zero.
		jmpWithCond = c.assembler.CompileJump(amd64.JNE)
		c.locationStack.markRegisterUnused(cond.register)
	}

	// Make sure that the next coming label is the else jump target.
	thenTarget, elseTarget := o.Then, o.Else

	// Here's the diagram of how we organize the instructions necessarily for brif operation.
	//
	// jmp_with_cond -> jmp (.Else) -> Then operations...
	//    |---------(satisfied)------------^^^
	//
	// Note that .Else branch doesn't have ToDrop as .Else is in reality
	// corresponding to either If's Else block or Br_if's else block in Wasm.

	// Emit for else branches
	saved := c.locationStack
	c.setLocationStack(saved.clone())
	if elseTarget.Target.IsReturnTarget() {
		if err := c.compileReturnFunction(); err != nil {
			return err
		}
	} else {
		elseLabelID := elseTarget.Target.ID()
		if c.ir.LabelCallers[elseLabelID] > 1 {
			// We can only re-use register state if when there's a single call-site.
			// Release existing values on registers to the stack if there's multiple ones to have
			// the consistent value location state at the beginning of label.
			if err := c.compileReleaseAllRegistersToStack(); err != nil {
				return err
			}
		}
		// Set the initial stack of the target label, so we can start compiling the label
		// with the appropriate value locations. Note we clone the stack here as we maybe
		// manipulate the stack before compiler reaches the label.
		labelInfo := c.label(elseLabelID)
		if !labelInfo.initialStack.initialized() {
			labelInfo.initialStack = c.locationStack
		}

		elseJmp := c.assembler.CompileJump(amd64.JMP)
		c.assignJumpTarget(elseLabelID, elseJmp)
	}

	// Handle then branch.
	c.assembler.SetJumpTargetOnNext(jmpWithCond)
	c.setLocationStack(saved)
	if err := compileDropRange(c, thenTarget.ToDrop); err != nil {
		return err
	}
	if thenTarget.Target.IsReturnTarget() {
		return c.compileReturnFunction()
	} else {
		thenLabelID := thenTarget.Target.ID()
		if c.ir.LabelCallers[thenLabelID] > 1 {
			// We can only re-use register state if when there's a single call-site.
			// Release existing values on registers to the stack if there's multiple ones to have
			// the consistent value location state at the beginning of label.
			if err := c.compileReleaseAllRegistersToStack(); err != nil {
				return err
			}
		}
		// Set the initial stack of the target label, so we can start compiling the label
		// with the appropriate value locations. Note we clone the stack here as we maybe
		// manipulate the stack before compiler reaches the label.
		labelInfo := c.label(thenLabelID)
		if !labelInfo.initialStack.initialized() {
			labelInfo.initialStack = c.locationStack
		}
		thenJmp := c.assembler.CompileJump(amd64.JMP)
		c.assignJumpTarget(thenLabelID, thenJmp)
		return nil
	}
}

// compileBrTable implements compiler.compileBrTable for the amd64 architecture.
func (c *amd64Compiler) compileBrTable(o wazeroir.OperationBrTable) error {
	index := c.locationStack.pop()

	// If the operation only consists of the default target, we branch into it and return early.
	if len(o.Targets) == 0 {
		c.locationStack.releaseRegister(index)
		if err := compileDropRange(c, o.Default.ToDrop); err != nil {
			return err
		}
		return c.branchInto(o.Default.Target)
	}

	// Otherwise, we jump into the selected branch.
	if err := c.compileEnsureOnRegister(index); err != nil {
		return err
	}

	tmp, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}

	// First, we move the length of target list into the tmp register.
	c.assembler.CompileConstToRegister(amd64.MOVQ, int64(len(o.Targets)), tmp)

	// Then, we compare the value with the length of targets.
	c.assembler.CompileRegisterToRegister(amd64.CMPL, tmp, index.register)

	// If the value is larger than the length,
	// we round the index to the length as the spec states that
	// if the index is larger than or equal the length of list,
	// branch into the default branch.
	c.assembler.CompileRegisterToRegister(amd64.CMOVQCS, tmp, index.register)

	// We prepare the static data which holds the offset of
	// each target's first instruction (incl. default)
	// relative to the beginning of label tables.
	//
	// For example, if we have targets=[L0, L1] and default=L_DEFAULT,
	// we emit the the code like this at [Emit the code for each targets and default branch] below.
	//
	// L0:
	//  0x123001: XXXX, ...
	//  .....
	// L1:
	//  0x123005: YYY, ...
	//  .....
	// L_DEFAULT:
	//  0x123009: ZZZ, ...
	//
	// then offsetData becomes like [0x0, 0x5, 0x8].
	// By using this offset list, we could jump into the label for the index by
	// "jmp offsetData[index]+0x123001" and "0x123001" can be acquired by "LEA"
	// instruction.
	//
	// Note: We store each offset of 32-bite unsigned integer as 4 consecutive bytes. So more precisely,
	// the above example's offsetData would be [0x0, 0x0, 0x0, 0x0, 0x5, 0x0, 0x0, 0x0, 0x8, 0x0, 0x0, 0x0].
	//
	// Note: this is similar to how GCC implements Switch statements in C.
	offsetData := asm.NewStaticConst(make([]byte, 4*(len(o.Targets)+1)))

	// Load the offsetData's address into tmp.
	if err = c.assembler.CompileStaticConstToRegister(amd64.LEAQ, offsetData, tmp); err != nil {
		return err
	}

	// Now we have the address of first byte of offsetData in tmp register.
	// So the target offset's first byte is at tmp+index*4 as we store
	// the offset as 4 bytes for a 32-byte integer.
	// Here, we store the offset into the index.register.
	c.assembler.CompileMemoryWithIndexToRegister(amd64.MOVL, tmp, 0, index.register, 4, index.register)

	// Now we read the address of the beginning of the jump table.
	// In the above example, this corresponds to reading the address of 0x123001.
	c.assembler.CompileReadInstructionAddress(tmp, amd64.JMP)

	// Now we have the address of L0 in tmp register, and the offset to the target label in the index.register.
	// So we could achieve the br_table jump by adding them and jump into the resulting address.
	c.assembler.CompileRegisterToRegister(amd64.ADDQ, index.register, tmp)

	c.assembler.CompileJumpToRegister(amd64.JMP, tmp)

	// We no longer need the index's register, so mark it unused.
	c.locationStack.markRegisterUnused(index.register)

	// [Emit the code for each targets and default branch]
	labelInitialInstructions := make([]asm.Node, len(o.Targets)+1)
	saved := c.locationStack
	for i := range labelInitialInstructions {
		// Emit the initial instruction of each target.
		// We use NOP as we don't yet know the next instruction in each label.
		// Assembler would optimize out this NOP during code generation, so this is harmless.
		labelInitialInstructions[i] = c.assembler.CompileStandAlone(amd64.NOP)

		var locationStack runtimeValueLocationStack
		var target *wazeroir.BranchTargetDrop
		if i < len(o.Targets) {
			target = o.Targets[i]
			// Clone the location stack so the branch-specific code doesn't
			// affect others.
			locationStack = saved.clone()
		} else {
			target = o.Default
			// If this is the default branch, we use the original one
			// as this is the last code in this block.
			locationStack = saved
		}
		c.setLocationStack(locationStack)
		if err := compileDropRange(c, target.ToDrop); err != nil {
			return err
		}
		if err := c.branchInto(target.Target); err != nil {
			return err
		}
	}

	c.assembler.BuildJumpTable(offsetData, labelInitialInstructions)
	return nil
}

func (c *amd64Compiler) assignJumpTarget(labelID wazeroir.LabelID, jmpInstruction asm.Node) {
	jmpTargetLabel := c.label(labelID)
	if jmpTargetLabel.initialInstruction != nil {
		jmpInstruction.AssignJumpTarget(jmpTargetLabel.initialInstruction)
	} else {
		jmpTargetLabel.labelBeginningCallbacks = append(jmpTargetLabel.labelBeginningCallbacks, func(labelInitialInstruction asm.Node) {
			jmpInstruction.AssignJumpTarget(labelInitialInstruction)
		})
	}
}

// compileLabel implements compiler.compileLabel for the amd64 architecture.
func (c *amd64Compiler) compileLabel(o wazeroir.OperationLabel) (skipLabel bool) {
	labelID := o.Label.ID()
	labelInfo := c.label(labelID)

	// If initialStack is not set, that means this label has never been reached.
	if !labelInfo.initialStack.initialized() {
		skipLabel = true
		return
	}

	// We use NOP as a beginning of instructions in a label.
	labelBegin := c.assembler.CompileStandAlone(amd64.NOP)

	// Save the instructions so that backward branching
	// instructions can jump to this label.
	labelInfo.initialInstruction = labelBegin

	// Set the initial stack.
	c.setLocationStack(labelInfo.initialStack)

	// Invoke callbacks to notify the forward branching
	// instructions can properly jump to this label.
	for _, cb := range labelInfo.labelBeginningCallbacks {
		cb(labelBegin)
	}

	// Clear for debugging purpose. See the comment in "len(amd64LabelInfo.labelBeginningCallbacks) > 0" block above.
	labelInfo.labelBeginningCallbacks = nil
	return
}

// compileCall implements compiler.compileCall for the amd64 architecture.
func (c *amd64Compiler) compileCall(o wazeroir.OperationCall) error {
	if err := c.maybeCompileMoveTopConditionalToGeneralPurposeRegister(); err != nil {
		return err
	}

	target := c.ir.Functions[o.FunctionIndex]
	targetType := c.ir.Types[target]

	targetAddressRegister, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}

	// First, push the index to the callEngine.functionsElement0Address into the target register.
	c.assembler.CompileConstToRegister(amd64.MOVQ, int64(o.FunctionIndex)*functionSize, targetAddressRegister)

	// Next, we add the address of the first item of callEngine.functions slice (= &callEngine.functions[0])
	// to the target register.
	c.assembler.CompileMemoryToRegister(amd64.ADDQ, amd64ReservedRegisterForCallEngine,
		callEngineModuleContextFunctionsElement0AddressOffset, targetAddressRegister)

	if err := c.compileCallFunctionImpl(targetAddressRegister, targetType); err != nil {
		return err
	}
	return nil
}

// compileCallIndirect implements compiler.compileCallIndirect for the amd64 architecture.
func (c *amd64Compiler) compileCallIndirect(o wazeroir.OperationCallIndirect) error {
	offset := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(offset); err != nil {
		return nil
	}

	tmp, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}
	c.locationStack.markRegisterUsed(tmp)

	tmp2, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}
	c.locationStack.markRegisterUsed(tmp2)

	// Load the address of the target table: tmp = &module.Tables[0]
	c.assembler.CompileMemoryToRegister(amd64.MOVQ, amd64ReservedRegisterForCallEngine, callEngineModuleContextTablesElement0AddressOffset, tmp)
	// tmp = &module.Tables[0] + Index*8 = &module.Tables[0] + sizeOf(*TableInstance)*index = module.Tables[o.TableIndex].
	c.assembler.CompileMemoryToRegister(amd64.MOVQ, tmp, int64(o.TableIndex*8), tmp)

	// Then, we need to check if the offset doesn't exceed the length of table.
	c.assembler.CompileMemoryToRegister(amd64.CMPQ, tmp, tableInstanceTableLenOffset, offset.register)
	notLengthExceedJump := c.assembler.CompileJump(amd64.JHI)

	// If it exceeds, we return the function with nativeCallStatusCodeInvalidTableAccess.
	c.compileExitFromNativeCode(nativeCallStatusCodeInvalidTableAccess)
	c.assembler.SetJumpTargetOnNext(notLengthExceedJump)

	// next we check if the target's type matches the operation's one.
	// In order to get the type instance's address, we have to multiply the offset
	// by 8 as the offset is the "length" of table in Go's "[]uintptr{}",
	// and size of uintptr equals 8 bytes == (2^3).
	c.assembler.CompileConstToRegister(amd64.SHLQ, pointerSizeLog2, offset.register)

	// Adds the address of wasm.Table[0] stored as callEngine.tableElement0Address to the offset.
	c.assembler.CompileMemoryToRegister(amd64.ADDQ,
		tmp, tableInstanceTableOffset, offset.register)

	// "offset = (*offset) (== table[offset]  == *code type)"
	c.assembler.CompileMemoryToRegister(amd64.MOVQ, offset.register, 0, offset.register)

	// At this point offset.register holds the address of *code (as uintptr) at wasm.Table[offset].
	//
	// Check if the value of table[offset] equals zero, meaning that the target is uninitialized.
	c.assembler.CompileRegisterToConst(amd64.CMPQ, offset.register, 0)

	// Jump if the target is initialized element.
	jumpIfInitialized := c.assembler.CompileJump(amd64.JNE)

	// If not initialized, we return the function with nativeCallStatusCodeInvalidTableAccess.
	c.compileExitFromNativeCode(nativeCallStatusCodeInvalidTableAccess)

	c.assembler.SetJumpTargetOnNext(jumpIfInitialized)

	// next we need to check the type matches, i.e. table[offset].source.TypeID == targetFunctionType's typeID.
	//
	// "tmp = table[offset].source ( == *FunctionInstance type)"
	c.assembler.CompileMemoryToRegister(amd64.MOVQ, offset.register, functionSourceOffset, tmp)

	// "tmp2 = [&moduleInstance.TypeIDs[0] + index * 4] (== moduleInstance.TypeIDs[index])"
	c.assembler.CompileMemoryToRegister(amd64.MOVQ,
		amd64ReservedRegisterForCallEngine, callEngineModuleContextTypeIDsElement0AddressOffset,
		tmp2)
	c.assembler.CompileMemoryToRegister(amd64.MOVL, tmp2, int64(o.TypeIndex)*4, tmp2)

	// Jump if the type matches.
	c.assembler.CompileMemoryToRegister(amd64.CMPL, tmp, functionInstanceTypeIDOffset, tmp2)
	jumpIfTypeMatch := c.assembler.CompileJump(amd64.JEQ)

	// Otherwise, exit with type mismatch status.
	c.compileExitFromNativeCode(nativeCallStatusCodeTypeMismatchOnIndirectCall)

	c.assembler.SetJumpTargetOnNext(jumpIfTypeMatch)
	targetFunctionType := c.ir.Types[o.TypeIndex]
	if err = c.compileCallFunctionImpl(offset.register, targetFunctionType); err != nil {
		return nil
	}

	// The offset register should be marked as un-used as we consumed in the function call.
	c.locationStack.markRegisterUnused(offset.register, tmp, tmp2)
	return nil
}

// compileDrop implements compiler.compileDrop for the amd64 architecture.
func (c *amd64Compiler) compileDrop(o wazeroir.OperationDrop) error {
	return compileDropRange(c, o.Depth)
}

// compileSelectV128Impl implements compileSelect for vector values.
func (c *amd64Compiler) compileSelectV128Impl(selectorReg asm.Register) error {
	x2 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x2); err != nil {
		return err
	}

	x1 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x1); err != nil {
		return err
	}

	// Compare the conditional value with zero.
	c.assembler.CompileRegisterToConst(amd64.CMPQ, selectorReg, 0)

	// Set the jump if the top value is not zero.
	jmpIfNotZero := c.assembler.CompileJump(amd64.JNE)

	// In this branch, we select the value of x2, so we move the value into x1.register so that
	// we can have the result in x1.register regardless of the selection.
	c.assembler.CompileRegisterToRegister(amd64.MOVDQU, x2.register, x1.register)

	// Else, we don't need to adjust value, just need to jump to the next instruction.
	c.assembler.SetJumpTargetOnNext(jmpIfNotZero)

	// As noted, the result exists in x1.register regardless of the selector.
	c.pushVectorRuntimeValueLocationOnRegister(x1.register)
	// Plus, x2.register is no longer used.
	c.locationStack.markRegisterUnused(x2.register)
	c.locationStack.markRegisterUnused(selectorReg)
	return nil
}

// compileSelect implements compiler.compileSelect for the amd64 architecture.
//
// The emitted native code depends on whether the values are on
// the physical registers or memory stack, or maybe conditional register.
func (c *amd64Compiler) compileSelect(o wazeroir.OperationSelect) error {
	cv := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(cv); err != nil {
		return err
	}

	if o.IsTargetVector {
		return c.compileSelectV128Impl(cv.register)
	}

	x2 := c.locationStack.pop()
	// We do not consume x1 here, but modify the value according to
	// the conditional value "c" above.
	peekedX1 := c.locationStack.peek()

	// Compare the conditional value with zero.
	c.assembler.CompileRegisterToConst(amd64.CMPQ, cv.register, 0)

	// Now we can use c.register as temporary location.
	// We alias it here for readability.
	tmpRegister := cv.register

	// Set the jump if the top value is not zero.
	jmpIfNotZero := c.assembler.CompileJump(amd64.JNE)

	// If the value is zero, we must place the value of x2 onto the stack position of x1.

	// First we copy the value of x2 to the temporary register if x2 is not currently on a register.
	if x2.onStack() {
		x2.register = tmpRegister
		c.compileLoadValueOnStackToRegister(x2)
	}

	//
	// At this point x2's value is always on a register.
	//

	// Then release the value in the x2's register to the x1's stack position.
	if peekedX1.onRegister() {
		c.assembler.CompileRegisterToRegister(amd64.MOVQ, x2.register, peekedX1.register)
	} else {
		peekedX1.register = x2.register
		c.compileReleaseRegisterToStack(peekedX1) // Note inside we mark the register unused!
	}

	// Else, we don't need to adjust value, just need to jump to the next instruction.
	c.assembler.SetJumpTargetOnNext(jmpIfNotZero)

	// In any case, we don't need x2 and c anymore!
	c.locationStack.releaseRegister(x2)
	c.locationStack.releaseRegister(cv)
	return nil
}

// compilePick implements compiler.compilePick for the amd64 architecture.
func (c *amd64Compiler) compilePick(o wazeroir.OperationPick) error {
	if err := c.maybeCompileMoveTopConditionalToGeneralPurposeRegister(); err != nil {
		return err
	}

	// TODO: if we track the type of values on the stack,
	// we could optimize the instruction according to the bit size of the value.
	// For now, we just move the entire register i.e. as a quad word (8 bytes).
	pickTarget := &c.locationStack.stack[c.locationStack.sp-1-uint64(o.Depth)]
	reg, err := c.allocateRegister(pickTarget.getRegisterType())
	if err != nil {
		return err
	}

	if pickTarget.onRegister() {
		var inst asm.Instruction
		if o.IsTargetVector {
			inst = amd64.MOVDQU
		} else if pickTarget.valueType == runtimeValueTypeI32 { // amd64 cannot copy single-precisions between registers.
			inst = amd64.MOVL
		} else {
			inst = amd64.MOVQ
		}
		c.assembler.CompileRegisterToRegister(inst, pickTarget.register, reg)
	} else if pickTarget.onStack() {
		// Copy the value from the stack.
		var inst asm.Instruction
		if o.IsTargetVector {
			inst = amd64.MOVDQU
		} else if pickTarget.valueType == runtimeValueTypeI32 || pickTarget.valueType == runtimeValueTypeF32 {
			inst = amd64.MOVL
		} else {
			inst = amd64.MOVQ
		}
		// Note: stack pointers are ensured not to exceed 2^27 so this offset never exceeds 32-bit range.
		c.assembler.CompileMemoryToRegister(inst, amd64ReservedRegisterForStackBasePointerAddress,
			int64(pickTarget.stackPointer)*8, reg)
	}
	// Now we already placed the picked value on the register,
	// so push the location onto the stack.
	if o.IsTargetVector {
		c.pushVectorRuntimeValueLocationOnRegister(reg)
	} else {
		c.pushRuntimeValueLocationOnRegister(reg, pickTarget.valueType)
	}
	return nil
}

// compileAdd implements compiler.compileAdd for the amd64 architecture.
func (c *amd64Compiler) compileAdd(o wazeroir.OperationAdd) error {
	// TODO: if the previous instruction is const, then
	// this can be optimized. Same goes for other arithmetic instructions.

	var instruction asm.Instruction
	switch o.Type {
	case wazeroir.UnsignedTypeI32:
		instruction = amd64.ADDL
	case wazeroir.UnsignedTypeI64:
		instruction = amd64.ADDQ
	case wazeroir.UnsignedTypeF32:
		instruction = amd64.ADDSS
	case wazeroir.UnsignedTypeF64:
		instruction = amd64.ADDSD
	}

	x2 := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(x2); err != nil {
		return err
	}

	x1 := c.locationStack.peek() // Note this is peek, pop!
	if err := c.compileEnsureOnRegister(x1); err != nil {
		return err
	}

	// x1 += x2.
	c.assembler.CompileRegisterToRegister(instruction, x2.register, x1.register)

	// We no longer need x2 register after ADD operation here,
	// so we release it.
	c.locationStack.releaseRegister(x2)
	return nil
}

// compileSub implements compiler.compileSub for the amd64 architecture.
func (c *amd64Compiler) compileSub(o wazeroir.OperationSub) error {
	// TODO: if the previous instruction is const, then
	// this can be optimized. Same goes for other arithmetic instructions.

	var instruction asm.Instruction
	switch o.Type {
	case wazeroir.UnsignedTypeI32:
		instruction = amd64.SUBL
	case wazeroir.UnsignedTypeI64:
		instruction = amd64.SUBQ
	case wazeroir.UnsignedTypeF32:
		instruction = amd64.SUBSS
	case wazeroir.UnsignedTypeF64:
		instruction = amd64.SUBSD
	}

	x2 := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(x2); err != nil {
		return err
	}

	x1 := c.locationStack.peek() // Note this is peek, pop!
	if err := c.compileEnsureOnRegister(x1); err != nil {
		return err
	}

	// x1 -= x2.
	c.assembler.CompileRegisterToRegister(instruction, x2.register, x1.register)

	// We no longer need x2 register after ADD operation here,
	// so we release it.
	c.locationStack.releaseRegister(x2)
	return nil
}

// compileMul implements compiler.compileMul for the amd64 architecture.
func (c *amd64Compiler) compileMul(o wazeroir.OperationMul) (err error) {
	switch o.Type {
	case wazeroir.UnsignedTypeI32:
		err = c.compileMulForInts(true, amd64.MULL)
	case wazeroir.UnsignedTypeI64:
		err = c.compileMulForInts(false, amd64.MULQ)
	case wazeroir.UnsignedTypeF32:
		err = c.compileMulForFloats(amd64.MULSS)
	case wazeroir.UnsignedTypeF64:
		err = c.compileMulForFloats(amd64.MULSD)
	}
	return
}

// compileMulForInts emits instructions to perform integer multiplication for
// top two values on the stack. If unfamiliar with the convention for integer
// multiplication on x86, see https://www.felixcloutier.com/x86/mul.
//
// In summary, one of the values must be on the AX register,
// and the mul instruction stores the overflow info in DX register which we don't use.
// Here, we mean "the overflow info" by 65 bit or higher part of the result for 64 bit case.
//
// So, we have to ensure that
//  1. Previously located value on DX must be saved to memory stack. That is because
//     the existing value will be overridden after the mul execution.
//  2. One of the operands (x1 or x2) must be on AX register.
//
// See https://www.felixcloutier.com/x86/mul#description for detail semantics.
func (c *amd64Compiler) compileMulForInts(is32Bit bool, mulInstruction asm.Instruction) error {
	const (
		resultRegister   = amd64.RegAX
		reservedRegister = amd64.RegDX
	)

	x2 := c.locationStack.pop()
	x1 := c.locationStack.pop()

	var valueOnAX *runtimeValueLocation
	if x1.register == resultRegister {
		valueOnAX = x1
	} else if x2.register == resultRegister {
		valueOnAX = x2
	} else {
		valueOnAX = x2
		// This case we  move x2 to AX register.
		c.onValueReleaseRegisterToStack(resultRegister)
		if x2.onConditionalRegister() {
			c.compileMoveConditionalToGeneralPurposeRegister(x2, resultRegister)
		} else if x2.onStack() {
			x2.setRegister(resultRegister)
			c.compileLoadValueOnStackToRegister(x2)
			c.locationStack.markRegisterUsed(resultRegister)
		} else {
			var inst asm.Instruction
			if is32Bit {
				inst = amd64.MOVL
			} else {
				inst = amd64.MOVQ
			}
			c.assembler.CompileRegisterToRegister(inst, x2.register, resultRegister)

			// We no longer uses the prev register of x2.
			c.locationStack.releaseRegister(x2)
			x2.setRegister(resultRegister)
			c.locationStack.markRegisterUsed(resultRegister)
		}
	}

	// We have to make sure that at this point the operands must be on registers.
	if err := c.compileEnsureOnRegister(x2); err != nil {
		return err
	}
	if err := c.compileEnsureOnRegister(x1); err != nil {
		return err
	}

	// We have to save the existing value on DX.
	// If the DX register is used by either x1 or x2, we don't need to
	// save the value because it is consumed by mul anyway.
	if x1.register != reservedRegister && x2.register != reservedRegister {
		c.onValueReleaseRegisterToStack(reservedRegister)
	}

	// Now ready to emit the mul instruction.
	if x1 == valueOnAX {
		c.assembler.CompileRegisterToNone(mulInstruction, x2.register)
	} else {
		c.assembler.CompileRegisterToNone(mulInstruction, x1.register)
	}

	c.locationStack.markRegisterUnused(x2.register)
	c.locationStack.markRegisterUnused(x1.register)

	// Now we have the result in the AX register,
	// so we record it.
	c.pushRuntimeValueLocationOnRegister(resultRegister, x1.valueType)
	return nil
}

func (c *amd64Compiler) compileMulForFloats(instruction asm.Instruction) error {
	x2 := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(x2); err != nil {
		return err
	}

	x1 := c.locationStack.peek() // Note this is peek!
	if err := c.compileEnsureOnRegister(x1); err != nil {
		return err
	}

	// x1 *= x2.
	c.assembler.CompileRegisterToRegister(instruction, x2.register, x1.register)

	// We no longer need x2 register after MUL operation here,
	// so we release it.
	c.locationStack.releaseRegister(x2)
	return nil
}

// compileClz implements compiler.compileClz for the amd64 architecture.
func (c *amd64Compiler) compileClz(o wazeroir.OperationClz) error {
	target := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(target); err != nil {
		return err
	}

	if c.cpuFeatures.HasExtra(platform.CpuExtraFeatureABM) {
		if o.Type == wazeroir.UnsignedInt32 {
			c.assembler.CompileRegisterToRegister(amd64.LZCNTL, target.register, target.register)
		} else {
			c.assembler.CompileRegisterToRegister(amd64.LZCNTQ, target.register, target.register)
		}
	} else {
		// On x86 mac, we cannot use LZCNT as it always results in zero.
		// Instead we combine BSR (calculating most significant set bit)
		// with XOR. This logic is described in
		// "Replace Raw Assembly Code with Builtin Intrinsics" section in:
		// https://developer.apple.com/documentation/apple-silicon/addressing-architectural-differences-in-your-macos-code.

		// First, we have to check if the target is non-zero as BSR is undefined
		// on zero. See https://www.felixcloutier.com/x86/bsr.
		c.assembler.CompileRegisterToConst(amd64.CMPQ, target.register, 0)
		jmpIfNonZero := c.assembler.CompileJump(amd64.JNE)

		// If the value is zero, we just push the const value.
		if o.Type == wazeroir.UnsignedInt32 {
			c.assembler.CompileConstToRegister(amd64.MOVL, int64(32), target.register)
		} else {
			c.assembler.CompileConstToRegister(amd64.MOVL, int64(64), target.register)
		}

		// Emit the jmp instruction to jump to the position right after
		// the non-zero case.
		jmpAtEndOfZero := c.assembler.CompileJump(amd64.JMP)

		// Start emitting non-zero case.
		c.assembler.SetJumpTargetOnNext(jmpIfNonZero)
		// First, we calculate the most significant set bit.
		if o.Type == wazeroir.UnsignedInt32 {
			c.assembler.CompileRegisterToRegister(amd64.BSRL, target.register, target.register)
		} else {
			c.assembler.CompileRegisterToRegister(amd64.BSRQ, target.register, target.register)
		}

		// Now we XOR the value with the bit length minus one.
		if o.Type == wazeroir.UnsignedInt32 {
			c.assembler.CompileConstToRegister(amd64.XORL, 31, target.register)
		} else {
			c.assembler.CompileConstToRegister(amd64.XORQ, 63, target.register)
		}

		// Finally the end jump instruction of zero case must target towards
		// the next instruction.
		c.assembler.SetJumpTargetOnNext(jmpAtEndOfZero)
	}

	// We reused the same register of target for the result.
	c.locationStack.markRegisterUnused(target.register)
	c.pushRuntimeValueLocationOnRegister(target.register, target.valueType)
	return nil
}

// compileCtz implements compiler.compileCtz for the amd64 architecture.
func (c *amd64Compiler) compileCtz(o wazeroir.OperationCtz) error {
	target := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(target); err != nil {
		return err
	}

	if c.cpuFeatures.HasExtra(platform.CpuExtraFeatureABM) {
		if o.Type == wazeroir.UnsignedInt32 {
			c.assembler.CompileRegisterToRegister(amd64.TZCNTL, target.register, target.register)
		} else {
			c.assembler.CompileRegisterToRegister(amd64.TZCNTQ, target.register, target.register)
		}
	} else {
		// Somehow, if the target value is zero, TZCNT always returns zero: this is wrong.
		// Meanwhile, we need branches for non-zero and zero cases on macos.
		// TODO: find the reference to this behavior and put the link here.

		// First we compare the target with zero.
		c.assembler.CompileRegisterToConst(amd64.CMPQ, target.register, 0)
		jmpIfNonZero := c.assembler.CompileJump(amd64.JNE)

		// If the value is zero, we just push the const value.
		if o.Type == wazeroir.UnsignedInt32 {
			c.assembler.CompileConstToRegister(amd64.MOVL, int64(32), target.register)
		} else {
			c.assembler.CompileConstToRegister(amd64.MOVL, int64(64), target.register)
		}

		// Emit the jmp instruction to jump to the position right after
		// the non-zero case.
		jmpAtEndOfZero := c.assembler.CompileJump(amd64.JMP)

		// Otherwise, emit the TZCNT.
		c.assembler.SetJumpTargetOnNext(jmpIfNonZero)
		if o.Type == wazeroir.UnsignedInt32 {
			c.assembler.CompileRegisterToRegister(amd64.TZCNTL, target.register, target.register)
		} else {
			c.assembler.CompileRegisterToRegister(amd64.TZCNTQ, target.register, target.register)
		}

		// Finally the end jump instruction of zero case must target towards
		// the next instruction.
		c.assembler.SetJumpTargetOnNext(jmpAtEndOfZero)
	}

	// We reused the same register of target for the result.
	c.locationStack.markRegisterUnused(target.register)
	c.pushRuntimeValueLocationOnRegister(target.register, target.valueType)
	return nil
}

// compilePopcnt implements compiler.compilePopcnt for the amd64 architecture.
func (c *amd64Compiler) compilePopcnt(o wazeroir.OperationPopcnt) error {
	target := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(target); err != nil {
		return err
	}

	if o.Type == wazeroir.UnsignedInt32 {
		c.assembler.CompileRegisterToRegister(amd64.POPCNTL, target.register, target.register)
	} else {
		c.assembler.CompileRegisterToRegister(amd64.POPCNTQ, target.register, target.register)
	}

	// We reused the same register of target for the result.
	c.locationStack.markRegisterUnused(target.register)
	c.pushRuntimeValueLocationOnRegister(target.register, target.valueType)
	return nil
}

// compileDiv implements compiler.compileDiv for the amd64 architecture.
func (c *amd64Compiler) compileDiv(o wazeroir.OperationDiv) (err error) {
	switch o.Type {
	case wazeroir.SignedTypeUint32:
		err = c.compileDivForInts(true, false)
	case wazeroir.SignedTypeUint64:
		err = c.compileDivForInts(false, false)
	case wazeroir.SignedTypeInt32:
		err = c.compileDivForInts(true, true)
	case wazeroir.SignedTypeInt64:
		err = c.compileDivForInts(false, true)
	case wazeroir.SignedTypeFloat32:
		err = c.compileDivForFloats(true)
	case wazeroir.SignedTypeFloat64:
		err = c.compileDivForFloats(false)
	}
	return
}

// compileDivForInts emits the instructions to perform division on the top
// two values of integer type on the stack and puts the quotient of the result
// onto the stack. For example, stack [..., 10, 3] results in [..., 3] where
// the remainder is discarded.
func (c *amd64Compiler) compileDivForInts(is32Bit bool, signed bool) error {
	if err := c.performDivisionOnInts(false, is32Bit, signed); err != nil {
		return err
	}
	// Now we have the quotient of the division result in the AX register,
	// so we record it.
	if is32Bit {
		c.pushRuntimeValueLocationOnRegister(amd64.RegAX, runtimeValueTypeI32)
	} else {
		c.pushRuntimeValueLocationOnRegister(amd64.RegAX, runtimeValueTypeI64)
	}
	return nil
}

// compileRem implements compiler.compileRem for the amd64 architecture.
func (c *amd64Compiler) compileRem(o wazeroir.OperationRem) (err error) {
	var vt runtimeValueType
	switch o.Type {
	case wazeroir.SignedInt32:
		err = c.performDivisionOnInts(true, true, true)
		vt = runtimeValueTypeI32
	case wazeroir.SignedInt64:
		err = c.performDivisionOnInts(true, false, true)
		vt = runtimeValueTypeI64
	case wazeroir.SignedUint32:
		err = c.performDivisionOnInts(true, true, false)
		vt = runtimeValueTypeI32
	case wazeroir.SignedUint64:
		err = c.performDivisionOnInts(true, false, false)
		vt = runtimeValueTypeI64
	}
	if err != nil {
		return err
	}

	// Now we have the remainder of the division result in the DX register,
	// so we record it.
	c.pushRuntimeValueLocationOnRegister(amd64.RegDX, vt)
	return
}

// performDivisionOnInts emits the instructions to do divisions on top two integers on the stack
// via DIV (unsigned div) and IDIV (signed div) instructions.
// See the following explanation of these instructions' semantics from https://www.lri.fr/~filliatr/ens/compil/x86-64.pdf
//
// >> Division requires special arrangements: idiv (signed) and div (unsigned) operate on a 2n-byte dividend and
// >> an n-byte divisor to produce an n-byte quotient and n-byte remainder. The dividend always lives in a fixed pair of
// >> registers (%edx and %eax for the 32-bit case; %rdx and %rax for the 64-bit case); the divisor is specified as the
// >> source operand in the instruction. The quotient goes in %eax (resp. %rax); the remainder in %edx (resp. %rdx). For
// >> signed division, the cltd (resp. ctqo) instruction is used to prepare %edx (resp. %rdx) with the sign extension of
// >> %eax (resp. %rax). For example, if a,b, c are memory locations holding quad words, then we could set c = a/b
// >> using the sequence: movq a(%rip), %rax; ctqo; idivq b(%rip); movq %rax, c(%rip).
//
// tl;dr is that the division result is placed in AX and DX registers after instructions emitted by this function
// where AX holds the quotient while DX the remainder of the division result.
func (c *amd64Compiler) performDivisionOnInts(isRem, is32Bit, signed bool) error {
	const (
		quotientRegister  = amd64.RegAX
		remainderRegister = amd64.RegDX
	)

	if err := c.maybeCompileMoveTopConditionalToGeneralPurposeRegister(); err != nil {
		return err
	}

	// Ensures that previous values on these registers are saved to memory.
	c.onValueReleaseRegisterToStack(quotientRegister)
	c.onValueReleaseRegisterToStack(remainderRegister)

	// In order to ensure x2 is placed on a temporary register for x2 value other than AX and DX,
	// we mark them as used here.
	c.locationStack.markRegisterUsed(quotientRegister)
	c.locationStack.markRegisterUsed(remainderRegister)

	// Ensure that x2 is placed on a register which is not either AX or DX.
	x2 := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(x2); err != nil {
		return err
	}

	// Now we successfully place x2 on a temp register, so we no longer need to
	// mark these registers used.
	c.locationStack.markRegisterUnused(quotientRegister)
	c.locationStack.markRegisterUnused(remainderRegister)

	// Check if the x2 equals zero.
	if is32Bit {
		c.assembler.CompileRegisterToConst(amd64.CMPL, x2.register, 0)
	} else {
		c.assembler.CompileRegisterToConst(amd64.CMPQ, x2.register, 0)
	}

	// Jump if the divisor is not zero.
	jmpIfNotZero := c.assembler.CompileJump(amd64.JNE)

	// Otherwise, we return with nativeCallStatusIntegerDivisionByZero status.
	c.compileExitFromNativeCode(nativeCallStatusIntegerDivisionByZero)

	c.assembler.SetJumpTargetOnNext(jmpIfNotZero)

	// next, we ensure that x1 is placed on AX.
	x1 := c.locationStack.pop()
	if x1.onRegister() && x1.register != quotientRegister {
		// Move x1 to quotientRegister.
		if is32Bit {
			c.assembler.CompileRegisterToRegister(amd64.MOVL, x1.register, quotientRegister)
		} else {
			c.assembler.CompileRegisterToRegister(amd64.MOVQ, x1.register, quotientRegister)
		}
		c.locationStack.markRegisterUnused(x1.register)
		x1.setRegister(quotientRegister)
	} else if x1.onStack() {
		x1.setRegister(quotientRegister)
		c.compileLoadValueOnStackToRegister(x1)
	}

	// Note: at this point, x1 is placed on AX, x2 is on a register which is not AX or DX.

	isSignedRem := isRem && signed
	isSignedDiv := !isRem && signed
	var signedRemMinusOneDivisorJmp asm.Node
	if isSignedRem {
		// If this is for getting remainder of signed division,
		// we have to treat the special case where the divisor equals -1.
		// For example, if this is 32-bit case, the result of (-2^31) / -1 equals (quotient=2^31, remainder=0)
		// where quotient doesn't fit in the 32-bit range whose maximum is 2^31-1.
		// x86 in this case cause floating point exception, but according to the Wasm spec
		// if the divisor equals -1, the result must be zero (not undefined!) as opposed to be "undefined"
		// for divisions on (-2^31) / -1 where we do not need to emit the special branches.
		// For detail, please refer to https://stackoverflow.com/questions/56303282/why-idiv-with-1-causes-floating-point-exception

		// First we compare the division with -1.
		if is32Bit {
			c.assembler.CompileRegisterToConst(amd64.CMPL, x2.register, -1)
		} else {
			c.assembler.CompileRegisterToConst(amd64.CMPQ, x2.register, -1)
		}

		// If it doesn't equal minus one, we jump to the normal case.
		okJmp := c.assembler.CompileJump(amd64.JNE)

		// Otherwise, we store zero into the remainder result register (DX).
		if is32Bit {
			c.assembler.CompileRegisterToRegister(amd64.XORL, remainderRegister, remainderRegister)
		} else {
			c.assembler.CompileRegisterToRegister(amd64.XORQ, remainderRegister, remainderRegister)
		}

		// Emit the exit jump instruction for the divisor -1 case so
		// we skips the normal case.
		signedRemMinusOneDivisorJmp = c.assembler.CompileJump(amd64.JMP)

		// Set the normal case's jump target.
		c.assembler.SetJumpTargetOnNext(okJmp)
	} else if isSignedDiv {
		// For signed division, we have to have branches for "math.MinInt{32,64} / -1"
		// case which results in the floating point exception via division error as
		// the resulting value exceeds the maximum of signed int.

		// First we compare the division with -1.
		if is32Bit {
			c.assembler.CompileRegisterToConst(amd64.CMPL, x2.register, -1)
		} else {
			c.assembler.CompileRegisterToConst(amd64.CMPQ, x2.register, -1)
		}

		// If it doesn't equal minus one, we jump to the normal case.
		nonMinusOneDivisorJmp := c.assembler.CompileJump(amd64.JNE)

		// next we check if the quotient is the most negative value for the signed integer.
		// That means whether or not we try to do (math.MaxInt32 / -1) or (math.Math.Int64 / -1) respectively.
		if is32Bit {
			if err := c.assembler.CompileRegisterToStaticConst(amd64.CMPL, x1.register,
				asm.NewStaticConst(u32.LeBytes(uint32(minimum32BitSignedInt)))); err != nil {
				return err
			}
		} else {
			if err := c.assembler.CompileRegisterToStaticConst(amd64.CMPQ, x1.register,
				asm.NewStaticConst(u64.LeBytes(uint64(minimum64BitSignedInt)))); err != nil {
				return err
			}
		}

		// If it doesn't equal, we jump to the normal case.
		jmpOK := c.assembler.CompileJump(amd64.JNE)

		// Otherwise, we are trying to do (math.MaxInt32 / -1) or (math.Math.Int64 / -1),
		// and that is the overflow in division as the result becomes 2^31 which is larger than
		// the maximum of signed 32-bit int (2^31-1).
		c.compileExitFromNativeCode(nativeCallStatusIntegerOverflow)

		// Set the normal case's jump target.
		c.assembler.SetJumpTargetOnNext(nonMinusOneDivisorJmp, jmpOK)
	}

	// Now ready to emit the div instruction.
	// Since the div instructions takes 2n byte dividend placed in DX:AX registers...
	// * signed case - we need to sign-extend the dividend into DX register via CDQ (32 bit) or CQO (64 bit).
	// * unsigned case - we need to zero DX register via "XOR DX DX"
	if is32Bit && signed {
		// Emit sign-extension to have 64 bit dividend over DX and AX registers.
		c.assembler.CompileStandAlone(amd64.CDQ)
		c.assembler.CompileRegisterToNone(amd64.IDIVL, x2.register)
	} else if is32Bit && !signed {
		// Zeros DX register to have 64 bit dividend over DX and AX registers.
		c.assembler.CompileRegisterToRegister(amd64.XORQ, amd64.RegDX, amd64.RegDX)
		c.assembler.CompileRegisterToNone(amd64.DIVL, x2.register)
	} else if !is32Bit && signed {
		// Emits sign-extension to have 128 bit dividend over DX and AX registers.
		c.assembler.CompileStandAlone(amd64.CQO)
		c.assembler.CompileRegisterToNone(amd64.IDIVQ, x2.register)
	} else if !is32Bit && !signed {
		// Zeros DX register to have 128 bit dividend over DX and AX registers.
		c.assembler.CompileRegisterToRegister(amd64.XORQ, amd64.RegDX, amd64.RegDX)
		c.assembler.CompileRegisterToNone(amd64.DIVQ, x2.register)
	}

	// If this is signed rem instruction, we must set the jump target of
	// the exit jump from division -1 case towards the next instruction.
	if signedRemMinusOneDivisorJmp != nil {
		c.assembler.SetJumpTargetOnNext(signedRemMinusOneDivisorJmp)
	}

	// We mark them as unused so that we can push one of them onto the location stack at call sites.
	c.locationStack.markRegisterUnused(remainderRegister)
	c.locationStack.markRegisterUnused(quotientRegister)
	c.locationStack.markRegisterUnused(x2.register)
	return nil
}

// compileDivForFloats emits the instructions to perform division
// on the top two values of float type on the stack, placing the result back onto the stack.
// For example, stack [..., 1.0, 4.0] results in [..., 0.25].
func (c *amd64Compiler) compileDivForFloats(is32Bit bool) error {
	if is32Bit {
		return c.compileSimpleBinaryOp(amd64.DIVSS)
	} else {
		return c.compileSimpleBinaryOp(amd64.DIVSD)
	}
}

// compileAnd implements compiler.compileAnd for the amd64 architecture.
func (c *amd64Compiler) compileAnd(o wazeroir.OperationAnd) (err error) {
	switch o.Type {
	case wazeroir.UnsignedInt32:
		err = c.compileSimpleBinaryOp(amd64.ANDL)
	case wazeroir.UnsignedInt64:
		err = c.compileSimpleBinaryOp(amd64.ANDQ)
	}
	return
}

// compileOr implements compiler.compileOr for the amd64 architecture.
func (c *amd64Compiler) compileOr(o wazeroir.OperationOr) (err error) {
	switch o.Type {
	case wazeroir.UnsignedInt32:
		err = c.compileSimpleBinaryOp(amd64.ORL)
	case wazeroir.UnsignedInt64:
		err = c.compileSimpleBinaryOp(amd64.ORQ)
	}
	return
}

// compileXor implements compiler.compileXor for the amd64 architecture.
func (c *amd64Compiler) compileXor(o wazeroir.OperationXor) (err error) {
	switch o.Type {
	case wazeroir.UnsignedInt32:
		err = c.compileSimpleBinaryOp(amd64.XORL)
	case wazeroir.UnsignedInt64:
		err = c.compileSimpleBinaryOp(amd64.XORQ)
	}
	return
}

// compileSimpleBinaryOp emits instructions to pop two values from the stack
// and perform the given instruction on these two values and push the result
// onto the stack.
func (c *amd64Compiler) compileSimpleBinaryOp(instruction asm.Instruction) error {
	x2 := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(x2); err != nil {
		return err
	}

	x1 := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(x1); err != nil {
		return err
	}

	c.assembler.CompileRegisterToRegister(instruction, x2.register, x1.register)

	// We consumed x2 register after the operation here,
	// so we release it.
	c.locationStack.releaseRegister(x2)

	// We already stored the result in the register used by x1
	// so we record it.
	c.locationStack.markRegisterUnused(x1.register)
	c.pushRuntimeValueLocationOnRegister(x1.register, x1.valueType)
	return nil
}

// compileShl implements compiler.compileShl for the amd64 architecture.
func (c *amd64Compiler) compileShl(o wazeroir.OperationShl) (err error) {
	switch o.Type {
	case wazeroir.UnsignedInt32:
		err = c.compileShiftOp(amd64.SHLL, false)
	case wazeroir.UnsignedInt64:
		err = c.compileShiftOp(amd64.SHLQ, true)
	}
	return
}

// compileShr implements compiler.compileShr for the amd64 architecture.
func (c *amd64Compiler) compileShr(o wazeroir.OperationShr) (err error) {
	switch o.Type {
	case wazeroir.SignedInt32:
		err = c.compileShiftOp(amd64.SARL, true)
	case wazeroir.SignedInt64:
		err = c.compileShiftOp(amd64.SARQ, false)
	case wazeroir.SignedUint32:
		err = c.compileShiftOp(amd64.SHRL, true)
	case wazeroir.SignedUint64:
		err = c.compileShiftOp(amd64.SHRQ, false)
	}
	return
}

// compileRotl implements compiler.compileRotl for the amd64 architecture.
func (c *amd64Compiler) compileRotl(o wazeroir.OperationRotl) (err error) {
	switch o.Type {
	case wazeroir.UnsignedInt32:
		err = c.compileShiftOp(amd64.ROLL, true)
	case wazeroir.UnsignedInt64:
		err = c.compileShiftOp(amd64.ROLQ, false)
	}
	return
}

// compileRotr implements compiler.compileRotr for the amd64 architecture.
func (c *amd64Compiler) compileRotr(o wazeroir.OperationRotr) (err error) {
	switch o.Type {
	case wazeroir.UnsignedInt32:
		err = c.compileShiftOp(amd64.RORL, true)
	case wazeroir.UnsignedInt64:
		err = c.compileShiftOp(amd64.RORQ, false)
	}
	return
}

// compileShiftOp adds instructions for shift operations (SHR, SHL, ROTR, ROTL)
// where we have to place the second value (shift counts) on the CX register.
func (c *amd64Compiler) compileShiftOp(instruction asm.Instruction, is32Bit bool) error {
	if err := c.maybeCompileMoveTopConditionalToGeneralPurposeRegister(); err != nil {
		return err
	}

	x2 := c.locationStack.pop()

	// Ensures that x2 (holding shift counts) is placed on the CX register.
	const shiftCountRegister = amd64.RegCX
	if (x2.onRegister() && x2.register != shiftCountRegister) || x2.onStack() {
		// If another value lives on the CX register, we release it to the stack.
		c.onValueReleaseRegisterToStack(shiftCountRegister)

		if x2.onRegister() {
			x2r := x2.register
			// If x2 lives on a register, we move the value to CX.
			if is32Bit {
				c.assembler.CompileRegisterToRegister(amd64.MOVL, x2r, shiftCountRegister)
			} else {
				c.assembler.CompileRegisterToRegister(amd64.MOVQ, x2r, shiftCountRegister)
			}
			// We no longer place any value on the original register, so we record it.
			c.locationStack.markRegisterUnused(x2r)
		} else {
			// If it is on stack, we just move the memory allocated value to the CX register.
			x2.setRegister(shiftCountRegister)
			c.compileLoadValueOnStackToRegister(x2)
		}
		c.locationStack.markRegisterUsed(shiftCountRegister)
	}

	x1 := c.locationStack.peek() // Note this is peek!
	x1r := x1.register

	if x1.onRegister() {
		c.assembler.CompileRegisterToRegister(instruction, shiftCountRegister, x1r)
	} else {
		// Shift target can be placed on a memory location.
		// Note: stack pointers are ensured not to exceed 2^27 so this offset never exceeds 32-bit range.
		c.assembler.CompileRegisterToMemory(instruction, shiftCountRegister, amd64ReservedRegisterForStackBasePointerAddress, int64(x1.stackPointer)*8)
	}

	// We consumed x2 register after the operation here,
	// so we release it.
	c.locationStack.markRegisterUnused(shiftCountRegister)
	return nil
}

// compileAbs implements compiler.compileAbs for the amd64 architecture.
//
// See the following discussions for how we could take the abs of floats on x86 assembly.
// https://stackoverflow.com/questions/32408665/fastest-way-to-compute-absolute-value-using-sse/32422471#32422471
// https://stackoverflow.com/questions/44630015/how-would-fabsdouble-be-implemented-on-x86-is-it-an-expensive-operation
func (c *amd64Compiler) compileAbs(o wazeroir.OperationAbs) (err error) {
	target := c.locationStack.peek() // Note this is peek!
	if err = c.compileEnsureOnRegister(target); err != nil {
		return err
	}

	// First shift left by one to clear the sign bit, and then shift right by one.
	if o.Type == wazeroir.Float32 {
		c.assembler.CompileConstToRegister(amd64.PSLLD, 1, target.register)
		c.assembler.CompileConstToRegister(amd64.PSRLD, 1, target.register)
	} else {
		c.assembler.CompileConstToRegister(amd64.PSLLQ, 1, target.register)
		c.assembler.CompileConstToRegister(amd64.PSRLQ, 1, target.register)
	}
	return nil
}

// compileNeg implements compiler.compileNeg for the amd64 architecture.
func (c *amd64Compiler) compileNeg(o wazeroir.OperationNeg) (err error) {
	target := c.locationStack.peek() // Note this is peek!
	if err := c.compileEnsureOnRegister(target); err != nil {
		return err
	}

	tmpReg, err := c.allocateRegister(registerTypeVector)
	if err != nil {
		return err
	}

	// First we move the sign-bit mask (placed in memory) to the tmp register,
	// since we cannot take XOR directly with float reg and const.
	// And then negate the value by XOR it with the sign-bit mask.
	if o.Type == wazeroir.Float32 {
		err = c.assembler.CompileStaticConstToRegister(amd64.MOVL, asm.NewStaticConst(u32.LeBytes(float32SignBitMask)), tmpReg)
		if err != nil {
			return err
		}
		c.assembler.CompileRegisterToRegister(amd64.XORPS, tmpReg, target.register)
	} else {
		err = c.assembler.CompileStaticConstToRegister(amd64.MOVQ, asm.NewStaticConst(u64.LeBytes(float64SignBitMask)), tmpReg)
		if err != nil {
			return err
		}
		c.assembler.CompileRegisterToRegister(amd64.XORPD, tmpReg, target.register)
	}
	return nil
}

// compileCeil implements compiler.compileCeil for the amd64 architecture.
func (c *amd64Compiler) compileCeil(o wazeroir.OperationCeil) (err error) {
	// Internally, ceil can be performed via ROUND instruction with 0x02 mode.
	// See https://android.googlesource.com/platform/bionic/+/882b8af/libm/x86_64/ceilf.S for example.
	return c.compileRoundInstruction(o.Type == wazeroir.Float32, 0x02)
}

// compileFloor implements compiler.compileFloor for the amd64 architecture.
func (c *amd64Compiler) compileFloor(o wazeroir.OperationFloor) (err error) {
	// Internally, floor can be performed via ROUND instruction with 0x01 mode.
	// See https://android.googlesource.com/platform/bionic/+/882b8af/libm/x86_64/floorf.S for example.
	return c.compileRoundInstruction(o.Type == wazeroir.Float32, 0x01)
}

// compileTrunc implements compiler.compileTrunc for the amd64 architecture.
func (c *amd64Compiler) compileTrunc(o wazeroir.OperationTrunc) error {
	// Internally, trunc can be performed via ROUND instruction with 0x03 mode.
	// See https://android.googlesource.com/platform/bionic/+/882b8af/libm/x86_64/truncf.S for example.
	return c.compileRoundInstruction(o.Type == wazeroir.Float32, 0x03)
}

// compileNearest implements compiler.compileNearest for the amd64 architecture.
func (c *amd64Compiler) compileNearest(o wazeroir.OperationNearest) error {
	// Nearest can be performed via ROUND instruction with 0x00 mode.
	return c.compileRoundInstruction(o.Type == wazeroir.Float32, 0x00)
}

func (c *amd64Compiler) compileRoundInstruction(is32Bit bool, mode int64) error {
	target := c.locationStack.peek() // Note this is peek!
	if err := c.compileEnsureOnRegister(target); err != nil {
		return err
	}

	if is32Bit {
		c.assembler.CompileRegisterToRegisterWithArg(amd64.ROUNDSS, target.register, target.register, byte(mode))
	} else {
		c.assembler.CompileRegisterToRegisterWithArg(amd64.ROUNDSD, target.register, target.register, byte(mode))
	}
	return nil
}

// compileMin implements compiler.compileMin for the amd64 architecture.
func (c *amd64Compiler) compileMin(o wazeroir.OperationMin) error {
	is32Bit := o.Type == wazeroir.Float32
	if is32Bit {
		return c.compileMinOrMax(is32Bit, true, amd64.MINSS)
	} else {
		return c.compileMinOrMax(is32Bit, true, amd64.MINSD)
	}
}

// compileMax implements compiler.compileMax for the amd64 architecture.
func (c *amd64Compiler) compileMax(o wazeroir.OperationMax) error {
	is32Bit := o.Type == wazeroir.Float32
	if is32Bit {
		return c.compileMinOrMax(is32Bit, false, amd64.MAXSS)
	} else {
		return c.compileMinOrMax(is32Bit, false, amd64.MAXSD)
	}
}

// emitMinOrMax adds instructions to pop two values from the stack, and push back either minimum or
// minimum of these two values onto the stack according to the minOrMaxInstruction argument.
// minOrMaxInstruction must be one of MAXSS, MAXSD, MINSS or MINSD.
// Note: These native min/max instructions are almost compatible with min/max in the Wasm specification,
// but it is slightly different with respect to the NaN handling.
// Native min/max instructions return non-NaN value if exactly one of target values
// is NaN. For example native_{min,max}(5.0, NaN) returns always 5.0, not NaN.
// However, WebAssembly specifies that min/max must always return NaN if one of values is NaN.
// Therefore, in this function, we have to add conditional jumps to check if one of values is NaN before
// the native min/max, which is why we cannot simply emit a native min/max instruction here.
//
// For the semantics, see wazeroir.Min and wazeroir.Max for detail.
func (c *amd64Compiler) compileMinOrMax(is32Bit, isMin bool, minOrMaxInstruction asm.Instruction) error {
	x2 := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(x2); err != nil {
		return err
	}
	x1 := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(x1); err != nil {
		return err
	}

	// Check if this is (either x1 or x2 is NaN) or (x1 equals x2) case
	if is32Bit {
		c.assembler.CompileRegisterToRegister(amd64.UCOMISS, x2.register, x1.register)
	} else {
		c.assembler.CompileRegisterToRegister(amd64.UCOMISD, x2.register, x1.register)
	}

	// At this point, we have the three cases of conditional flags below
	// (See https://www.felixcloutier.com/x86/ucomiss#operation for detail.)
	//
	// 1) Two values are NaN-free and different: All flags are cleared.
	// 2) Two values are NaN-free and equal: Only ZF flags is set.
	// 3) One of Two values is NaN: ZF, PF and CF flags are set.

	// Jump instruction to handle 1) case by checking the ZF flag
	// as ZF is only set for 2) and 3) cases.
	nanFreeOrDiffJump := c.assembler.CompileJump(amd64.JNE)

	// Start handling 2) and 3).

	// Jump if one of two values is NaN by checking the parity flag (PF).
	includeNaNJmp := c.assembler.CompileJump(amd64.JPS)

	// Start handling 2).

	// Before we exit this case, we have to ensure that positive zero (or negative zero for min instruction) is
	// returned if two values are positive and negative zeros.
	var inst asm.Instruction
	switch {
	case is32Bit && isMin:
		inst = amd64.ORPS
	case !is32Bit && isMin:
		inst = amd64.ORPD
	case is32Bit && !isMin:
		inst = amd64.ANDPS
	case !is32Bit && !isMin:
		inst = amd64.ANDPD
	}
	c.assembler.CompileRegisterToRegister(inst, x2.register, x1.register)

	sameExitJmp := c.assembler.CompileJump(amd64.JMP)

	// Start handling 3).
	c.assembler.SetJumpTargetOnNext(includeNaNJmp)

	// We emit the ADD instruction to produce the NaN in x1.
	if is32Bit {
		c.assembler.CompileRegisterToRegister(amd64.ADDSS, x2.register, x1.register)
	} else {
		c.assembler.CompileRegisterToRegister(amd64.ADDSD, x2.register, x1.register)
	}

	// Exit from the NaN case branch.
	nanExitJmp := c.assembler.CompileJump(amd64.JMP)

	// Start handling 1).
	c.assembler.SetJumpTargetOnNext(nanFreeOrDiffJump)

	// Now handle the NaN-free and different values case.
	c.assembler.CompileRegisterToRegister(minOrMaxInstruction, x2.register, x1.register)

	// Set the jump target of 1) and 2) cases to the next instruction after 3) case.
	c.assembler.SetJumpTargetOnNext(nanExitJmp, sameExitJmp)

	// Record that we consumed the x2 and placed the minOrMax result in the x1's register.
	c.locationStack.markRegisterUnused(x2.register)
	c.locationStack.markRegisterUnused(x1.register)
	c.pushRuntimeValueLocationOnRegister(x1.register, x1.valueType)
	return nil
}

// compileCopysign implements compiler.compileCopysign for the amd64 architecture.
func (c *amd64Compiler) compileCopysign(o wazeroir.OperationCopysign) error {
	is32Bit := o.Type == wazeroir.Float32

	x2 := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(x2); err != nil {
		return err
	}
	x1 := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(x1); err != nil {
		return err
	}
	tmpReg, err := c.allocateRegister(registerTypeVector)
	if err != nil {
		return err
	}

	// Move the rest bit mask to the temp register.
	if is32Bit {
		err = c.assembler.CompileStaticConstToRegister(amd64.MOVL, asm.NewStaticConst(u32.LeBytes(float32RestBitMask)), tmpReg)
	} else {
		err = c.assembler.CompileStaticConstToRegister(amd64.MOVQ, asm.NewStaticConst(u64.LeBytes(float64RestBitMask)), tmpReg)
	}
	if err != nil {
		return err
	}

	// Clear the sign bit of x1 via AND with the mask.
	if is32Bit {
		c.assembler.CompileRegisterToRegister(amd64.ANDPS, tmpReg, x1.register)
	} else {
		c.assembler.CompileRegisterToRegister(amd64.ANDPD, tmpReg, x1.register)
	}

	// Move the sign bit mask to the temp register.
	if is32Bit {
		err = c.assembler.CompileStaticConstToRegister(amd64.MOVL, asm.NewStaticConst(u32.LeBytes(float32SignBitMask)), tmpReg)
	} else {
		err = c.assembler.CompileStaticConstToRegister(amd64.MOVQ, asm.NewStaticConst(u64.LeBytes(float64SignBitMask)), tmpReg)
	}
	if err != nil {
		return err
	}

	// Clear the non-sign bits of x2 via AND with the mask.
	if is32Bit {
		c.assembler.CompileRegisterToRegister(amd64.ANDPS, tmpReg, x2.register)
	} else {
		c.assembler.CompileRegisterToRegister(amd64.ANDPD, tmpReg, x2.register)
	}

	// Finally, copy the sign bit of x2 to x1.
	if is32Bit {
		c.assembler.CompileRegisterToRegister(amd64.ORPS, x2.register, x1.register)
	} else {
		c.assembler.CompileRegisterToRegister(amd64.ORPD, x2.register, x1.register)
	}

	// Record that we consumed the x2 and placed the copysign result in the x1's register.
	c.locationStack.markRegisterUnused(x2.register)
	c.locationStack.markRegisterUnused(x1.register)
	c.pushRuntimeValueLocationOnRegister(x1.register, x1.valueType)
	return nil
}

// compileSqrt implements compiler.compileSqrt for the amd64 architecture.
func (c *amd64Compiler) compileSqrt(o wazeroir.OperationSqrt) error {
	target := c.locationStack.peek() // Note this is peek!
	if err := c.compileEnsureOnRegister(target); err != nil {
		return err
	}
	if o.Type == wazeroir.Float32 {
		c.assembler.CompileRegisterToRegister(amd64.SQRTSS, target.register, target.register)
	} else {
		c.assembler.CompileRegisterToRegister(amd64.SQRTSD, target.register, target.register)
	}
	return nil
}

// compileI32WrapFromI64 implements compiler.compileI32WrapFromI64 for the amd64 architecture.
func (c *amd64Compiler) compileI32WrapFromI64() error {
	target := c.locationStack.peek() // Note this is peek!
	if err := c.compileEnsureOnRegister(target); err != nil {
		return err
	}
	c.assembler.CompileRegisterToRegister(amd64.MOVL, target.register, target.register)
	target.valueType = runtimeValueTypeI32
	return nil
}

// compileITruncFromF implements compiler.compileITruncFromF for the amd64 architecture.
//
// Note: in the following implementation, we use CVTSS2SI and CVTSD2SI to convert floats to signed integers.
// According to the Intel manual ([1],[2]), if the source float value is either +-Inf or NaN, or it exceeds representative ranges
// of target signed integer, then the instruction returns "masked" response float32SignBitMask (or float64SignBitMask for 64 bit case).
// [1] Chapter 11.5.2, SIMD Floating-Point Exception Conditions in "Vol 1, Intel® 64 and IA-32 Architectures Manual"
//
//	https://www.intel.com/content/www/us/en/architecture-and-technology/64-ia-32-architectures-software-developer-vol-1-manual.html
//
// [2] https://xem.github.io/minix86/manual/intel-x86-and-64-manual-vol1/o_7281d5ea06a5b67a-268.html
func (c *amd64Compiler) compileITruncFromF(o wazeroir.OperationITruncFromF) (err error) {
	if o.InputType == wazeroir.Float32 && o.OutputType == wazeroir.SignedInt32 {
		err = c.emitSignedI32TruncFromFloat(true, o.NonTrapping)
	} else if o.InputType == wazeroir.Float32 && o.OutputType == wazeroir.SignedInt64 {
		err = c.emitSignedI64TruncFromFloat(true, o.NonTrapping)
	} else if o.InputType == wazeroir.Float64 && o.OutputType == wazeroir.SignedInt32 {
		err = c.emitSignedI32TruncFromFloat(false, o.NonTrapping)
	} else if o.InputType == wazeroir.Float64 && o.OutputType == wazeroir.SignedInt64 {
		err = c.emitSignedI64TruncFromFloat(false, o.NonTrapping)
	} else if o.InputType == wazeroir.Float32 && o.OutputType == wazeroir.SignedUint32 {
		err = c.emitUnsignedI32TruncFromFloat(true, o.NonTrapping)
	} else if o.InputType == wazeroir.Float32 && o.OutputType == wazeroir.SignedUint64 {
		err = c.emitUnsignedI64TruncFromFloat(true, o.NonTrapping)
	} else if o.InputType == wazeroir.Float64 && o.OutputType == wazeroir.SignedUint32 {
		err = c.emitUnsignedI32TruncFromFloat(false, o.NonTrapping)
	} else if o.InputType == wazeroir.Float64 && o.OutputType == wazeroir.SignedUint64 {
		err = c.emitUnsignedI64TruncFromFloat(false, o.NonTrapping)
	}
	return
}

// emitUnsignedI32TruncFromFloat implements compileITruncFromF when the destination type is a 32-bit unsigned integer.
func (c *amd64Compiler) emitUnsignedI32TruncFromFloat(isFloat32Bit, nonTrapping bool) error {
	source := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(source); err != nil {
		return err
	}

	result, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}

	// First, we check the source float value is above or equal math.MaxInt32+1.
	if isFloat32Bit {
		err = c.assembler.CompileStaticConstToRegister(amd64.UCOMISS,
			asm.NewStaticConst(u32.LeBytes(float32ForMaximumSigned32bitIntPlusOne)), source.register)
	} else {
		err = c.assembler.CompileStaticConstToRegister(amd64.UCOMISD,
			asm.NewStaticConst(u64.LeBytes(float64ForMaximumSigned32bitIntPlusOne)), source.register)
	}
	if err != nil {
		return err
	}

	// Check the parity flag (set when the value is NaN), and if it is set, we should raise an exception.
	jmpIfNotNaN := c.assembler.CompileJump(amd64.JPC) // jump if parity is not set.

	var nonTrappingNaNJump asm.Node
	if !nonTrapping {
		c.compileExitFromNativeCode(nativeCallStatusCodeInvalidFloatToIntConversion)
	} else {
		// In non trapping case, NaN is casted as zero.
		// Zero out the result register by XOR itsself.
		c.assembler.CompileRegisterToRegister(amd64.XORL, result, result)
		nonTrappingNaNJump = c.assembler.CompileJump(amd64.JMP)
	}

	c.assembler.SetJumpTargetOnNext(jmpIfNotNaN)

	// Jump if the source float value is above or equal math.MaxInt32+1.
	jmpAboveOrEqualMaxIn32PlusOne := c.assembler.CompileJump(amd64.JCC)

	// next we convert the value as a signed integer.
	if isFloat32Bit {
		c.assembler.CompileRegisterToRegister(amd64.CVTTSS2SL, source.register, result)
	} else {
		c.assembler.CompileRegisterToRegister(amd64.CVTTSD2SL, source.register, result)
	}

	// Then if the result is minus, it is invalid conversion from minus float (incl. -Inf).
	c.assembler.CompileRegisterToRegister(amd64.TESTL, result, result)
	jmpIfNotMinusOrMinusInf := c.assembler.CompileJump(amd64.JPL)

	var nonTrappingMinusJump asm.Node
	if !nonTrapping {
		c.compileExitFromNativeCode(nativeCallStatusIntegerOverflow)
	} else {
		// In non trapping case, the minus value is casted as zero.
		// Zero out the result register by XOR itsself.
		c.assembler.CompileRegisterToRegister(amd64.XORL, result, result)
		nonTrappingMinusJump = c.assembler.CompileJump(amd64.JMP)
	}

	c.assembler.SetJumpTargetOnNext(jmpIfNotMinusOrMinusInf)

	// Otherwise, the values is valid.
	okJmpForLessThanMaxInt32PlusOne := c.assembler.CompileJump(amd64.JMP)

	// Now, start handling the case where the original float value is above or equal math.MaxInt32+1.
	//
	// First, we subtract the math.MaxInt32+1 from the original value so it can fit in signed 32-bit integer.
	c.assembler.SetJumpTargetOnNext(jmpAboveOrEqualMaxIn32PlusOne)
	if isFloat32Bit {
		err = c.assembler.CompileStaticConstToRegister(amd64.SUBSS,
			asm.NewStaticConst(u32.LeBytes(float32ForMaximumSigned32bitIntPlusOne)), source.register)
	} else {
		err = c.assembler.CompileStaticConstToRegister(amd64.SUBSD,
			asm.NewStaticConst(u64.LeBytes(float64ForMaximumSigned32bitIntPlusOne)), source.register)
	}
	if err != nil {
		return err
	}

	// Then, convert the subtracted value as a signed 32-bit integer.
	if isFloat32Bit {
		c.assembler.CompileRegisterToRegister(amd64.CVTTSS2SL, source.register, result)
	} else {
		c.assembler.CompileRegisterToRegister(amd64.CVTTSD2SL, source.register, result)
	}

	// next, we have to check if the value is from NaN, +Inf.
	// NaN or +Inf cases result in 0x8000_0000 according to the semantics of conversion,
	// This means we check if the result int value is minus or not.
	c.assembler.CompileRegisterToRegister(amd64.TESTL, result, result)

	// If the result is minus, the conversion is invalid (from NaN or +Inf)
	jmpIfPlusInf := c.assembler.CompileJump(amd64.JMI)

	// Otherwise, we successfully converted the source float minus (math.MaxInt32+1) to int.
	// So, we retrieve the original source float value by adding the sign mask.
	if err = c.assembler.CompileStaticConstToRegister(amd64.ADDL,
		asm.NewStaticConst(u32.LeBytes(float32SignBitMask)), result); err != nil {
		return err
	}

	okJmpForAboveOrEqualMaxInt32PlusOne := c.assembler.CompileJump(amd64.JMP)

	c.assembler.SetJumpTargetOnNext(jmpIfPlusInf)
	if !nonTrapping {
		c.compileExitFromNativeCode(nativeCallStatusIntegerOverflow)
	} else {
		err = c.assembler.CompileStaticConstToRegister(amd64.MOVL,
			asm.NewStaticConst(u32.LeBytes(maximum32BitUnsignedInt)), result)
		if err != nil {
			return err
		}
	}

	// We jump to the next instructions for valid cases.
	c.assembler.SetJumpTargetOnNext(okJmpForLessThanMaxInt32PlusOne, okJmpForAboveOrEqualMaxInt32PlusOne)
	if nonTrapping {
		c.assembler.SetJumpTargetOnNext(nonTrappingMinusJump, nonTrappingNaNJump)
	}

	// We consumed the source's register and placed the conversion result
	// in the result register.
	c.locationStack.markRegisterUnused(source.register)
	c.pushRuntimeValueLocationOnRegister(result, runtimeValueTypeI32)
	return nil
}

// emitUnsignedI32TruncFromFloat implements compileITruncFromF when the destination type is a 64-bit unsigned integer.
func (c *amd64Compiler) emitUnsignedI64TruncFromFloat(isFloat32Bit, nonTrapping bool) error {
	source := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(source); err != nil {
		return err
	}

	result, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}

	// First, we check the source float value is above or equal math.MaxInt64+1.
	if isFloat32Bit {
		err = c.assembler.CompileStaticConstToRegister(amd64.UCOMISS,
			asm.NewStaticConst(u32.LeBytes(float32ForMaximumSigned64bitIntPlusOne)), source.register)
	} else {
		err = c.assembler.CompileStaticConstToRegister(amd64.UCOMISD,
			asm.NewStaticConst(u64.LeBytes(float64ForMaximumSigned64bitIntPlusOne)), source.register)
	}
	if err != nil {
		return err
	}

	// Check the parity flag (set when the value is NaN), and if it is set, we should raise an exception.
	jmpIfNotNaN := c.assembler.CompileJump(amd64.JPC) // jump if parity is not set.

	var nonTrappingNaNJump asm.Node
	if !nonTrapping {
		c.compileExitFromNativeCode(nativeCallStatusCodeInvalidFloatToIntConversion)
	} else {
		// In non trapping case, NaN is casted as zero.
		// Zero out the result register by XOR itsself.
		c.assembler.CompileRegisterToRegister(amd64.XORQ, result, result)
		nonTrappingNaNJump = c.assembler.CompileJump(amd64.JMP)
	}

	c.assembler.SetJumpTargetOnNext(jmpIfNotNaN)

	// Jump if the source float values is above or equal math.MaxInt64+1.
	jmpAboveOrEqualMaxIn32PlusOne := c.assembler.CompileJump(amd64.JCC)

	// next we convert the value as a signed integer.
	if isFloat32Bit {
		c.assembler.CompileRegisterToRegister(amd64.CVTTSS2SQ, source.register, result)
	} else {
		c.assembler.CompileRegisterToRegister(amd64.CVTTSD2SQ, source.register, result)
	}

	// Then if the result is minus, it is invalid conversion from minus float (incl. -Inf).
	c.assembler.CompileRegisterToRegister(amd64.TESTQ, result, result)
	jmpIfNotMinusOrMinusInf := c.assembler.CompileJump(amd64.JPL)

	var nonTrappingMinusJump asm.Node
	if !nonTrapping {
		c.compileExitFromNativeCode(nativeCallStatusIntegerOverflow)
	} else {
		// In non trapping case, the minus value is casted as zero.
		// Zero out the result register by XOR itsself.
		c.assembler.CompileRegisterToRegister(amd64.XORQ, result, result)
		nonTrappingMinusJump = c.assembler.CompileJump(amd64.JMP)
	}

	c.assembler.SetJumpTargetOnNext(jmpIfNotMinusOrMinusInf)

	// Otherwise, the values is valid.
	okJmpForLessThanMaxInt64PlusOne := c.assembler.CompileJump(amd64.JMP)

	// Now, start handling the case where the original float value is above or equal math.MaxInt64+1.
	//
	// First, we subtract the math.MaxInt64+1 from the original value so it can fit in signed 64-bit integer.
	c.assembler.SetJumpTargetOnNext(jmpAboveOrEqualMaxIn32PlusOne)
	if isFloat32Bit {
		err = c.assembler.CompileStaticConstToRegister(amd64.SUBSS,
			asm.NewStaticConst(u32.LeBytes(float32ForMaximumSigned64bitIntPlusOne)), source.register)
	} else {
		err = c.assembler.CompileStaticConstToRegister(amd64.SUBSD,
			asm.NewStaticConst(u64.LeBytes(float64ForMaximumSigned64bitIntPlusOne)), source.register)
	}
	if err != nil {
		return err
	}

	// Then, convert the subtracted value as a signed 64-bit integer.
	if isFloat32Bit {
		c.assembler.CompileRegisterToRegister(amd64.CVTTSS2SQ, source.register, result)
	} else {
		c.assembler.CompileRegisterToRegister(amd64.CVTTSD2SQ, source.register, result)
	}

	// next, we have to check if the value is from NaN, +Inf.
	// NaN or +Inf cases result in 0x8000_0000 according to the semantics of conversion,
	// This means we check if the result int value is minus or not.
	c.assembler.CompileRegisterToRegister(amd64.TESTQ, result, result)

	// If the result is minus, the conversion is invalid (from NaN or +Inf)
	jmpIfPlusInf := c.assembler.CompileJump(amd64.JMI)

	// Otherwise, we successfully converted the the source float minus (math.MaxInt64+1) to int.
	// So, we retrieve the original source float value by adding the sign mask.
	if err = c.assembler.CompileStaticConstToRegister(amd64.ADDQ,
		asm.NewStaticConst(u64.LeBytes(float64SignBitMask)), result); err != nil {
		return err
	}

	okJmpForAboveOrEqualMaxInt64PlusOne := c.assembler.CompileJump(amd64.JMP)

	c.assembler.SetJumpTargetOnNext(jmpIfPlusInf)
	if !nonTrapping {
		c.compileExitFromNativeCode(nativeCallStatusIntegerOverflow)
	} else {
		err = c.assembler.CompileStaticConstToRegister(amd64.MOVQ,
			asm.NewStaticConst(u64.LeBytes(maximum64BitUnsignedInt)), result)
		if err != nil {
			return err
		}
	}

	// We jump to the next instructions for valid cases.
	c.assembler.SetJumpTargetOnNext(okJmpForLessThanMaxInt64PlusOne, okJmpForAboveOrEqualMaxInt64PlusOne)
	if nonTrapping {
		c.assembler.SetJumpTargetOnNext(nonTrappingMinusJump, nonTrappingNaNJump)
	}

	// We consumed the source's register and placed the conversion result
	// in the result register.
	c.locationStack.markRegisterUnused(source.register)
	c.pushRuntimeValueLocationOnRegister(result, runtimeValueTypeI64)
	return nil
}

// emitSignedI32TruncFromFloat implements compileITruncFromF when the destination type is a 32-bit signed integer.
func (c *amd64Compiler) emitSignedI32TruncFromFloat(isFloat32Bit, nonTrapping bool) error {
	source := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(source); err != nil {
		return err
	}

	result, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}

	// First we unconditionally convert source to integer via CVTTSS2SI (CVTTSD2SI for 64bit float).
	if isFloat32Bit {
		c.assembler.CompileRegisterToRegister(amd64.CVTTSS2SL, source.register, result)
	} else {
		c.assembler.CompileRegisterToRegister(amd64.CVTTSD2SL, source.register, result)
	}

	// We compare the conversion result with the sign bit mask to check if it is either
	// 1) the source float value is either +-Inf or NaN, or it exceeds representative ranges of 32bit signed integer, or
	// 2) the source equals the minimum signed 32-bit (=-2147483648.000000) whose bit pattern is float32ForMinimumSigned32bitIntegerAddress for 32 bit float
	// 	  or float64ForMinimumSigned32bitIntegerAddress for 64bit float.
	err = c.assembler.CompileStaticConstToRegister(amd64.CMPL, asm.NewStaticConst(u32.LeBytes(float32SignBitMask)), result)
	if err != nil {
		return err
	}

	// Otherwise, jump to exit as the result is valid.
	okJmp := c.assembler.CompileJump(amd64.JNE)

	// Start handling the case of 1) and 2).
	// First, check if the value is NaN.
	if isFloat32Bit {
		c.assembler.CompileRegisterToRegister(amd64.UCOMISS, source.register, source.register)
	} else {
		c.assembler.CompileRegisterToRegister(amd64.UCOMISD, source.register, source.register)
	}

	// Check the parity flag (set when the value is NaN), and if it is set, we should raise an exception.
	jmpIfNotNaN := c.assembler.CompileJump(amd64.JPC) // jump if parity is not set.

	var nontrappingNanJump asm.Node
	if !nonTrapping {
		// If the value is NaN, we return the function with nativeCallStatusCodeInvalidFloatToIntConversion.
		c.compileExitFromNativeCode(nativeCallStatusCodeInvalidFloatToIntConversion)
	} else {
		// In non trapping case, NaN is casted as zero.
		// Zero out the result register by XOR itsself.
		c.assembler.CompileRegisterToRegister(amd64.XORL, result, result)
		nontrappingNanJump = c.assembler.CompileJump(amd64.JMP)
	}

	// Check if the value is larger than or equal the minimum 32-bit integer value,
	// meaning that the value exceeds the lower bound of 32-bit signed integer range.
	c.assembler.SetJumpTargetOnNext(jmpIfNotNaN)
	if isFloat32Bit {
		err = c.assembler.CompileStaticConstToRegister(amd64.UCOMISS,
			asm.NewStaticConst(u32.LeBytes(float32ForMinimumSigned32bitInteger)), source.register)
	} else {
		err = c.assembler.CompileStaticConstToRegister(amd64.UCOMISD,
			asm.NewStaticConst(u64.LeBytes(float64ForMinimumSigned32bitInteger)), source.register)
	}
	if err != nil {
		return err
	}

	if !nonTrapping {
		// Jump if the value exceeds the lower bound.
		var jmpIfExceedsLowerBound asm.Node
		if isFloat32Bit {
			jmpIfExceedsLowerBound = c.assembler.CompileJump(amd64.JCS)
		} else {
			jmpIfExceedsLowerBound = c.assembler.CompileJump(amd64.JLS)
		}

		// At this point, the value is the minimum signed 32-bit int (=-2147483648.000000) or larger than 32-bit maximum.
		// So, check if the value equals the minimum signed 32-bit int.
		if isFloat32Bit {
			err = c.assembler.CompileStaticConstToRegister(amd64.UCOMISS,
				asm.NewStaticConst([]byte{0, 0, 0, 0}), source.register)
		} else {
			err = c.assembler.CompileStaticConstToRegister(amd64.UCOMISD,
				asm.NewStaticConst([]byte{0, 0, 0, 0, 0, 0, 0, 0}), source.register)
		}
		if err != nil {
			return err
		}

		jmpIfMinimumSignedInt := c.assembler.CompileJump(amd64.JCS) // jump if the value is minus (= the minimum signed 32-bit int).

		c.assembler.SetJumpTargetOnNext(jmpIfExceedsLowerBound)
		c.compileExitFromNativeCode(nativeCallStatusIntegerOverflow)

		// We jump to the next instructions for valid cases.
		c.assembler.SetJumpTargetOnNext(okJmp, jmpIfMinimumSignedInt)
	} else {
		// Jump if the value does not exceed the lower bound.
		var jmpIfNotExceedsLowerBound asm.Node
		if isFloat32Bit {
			jmpIfNotExceedsLowerBound = c.assembler.CompileJump(amd64.JCC)
		} else {
			jmpIfNotExceedsLowerBound = c.assembler.CompileJump(amd64.JHI)
		}

		// If the value exceeds the lower bound, we "saturate" it to the minimum.
		if err = c.assembler.CompileStaticConstToRegister(amd64.MOVL,
			asm.NewStaticConst(u32.LeBytes(uint32(minimum32BitSignedInt))), result); err != nil {
			return err
		}
		nonTrappingSaturatedMinimumJump := c.assembler.CompileJump(amd64.JMP)

		// Otherwise, the value is the minimum signed 32-bit int (=-2147483648.000000) or larger than 32-bit maximum.
		c.assembler.SetJumpTargetOnNext(jmpIfNotExceedsLowerBound)
		if isFloat32Bit {
			err = c.assembler.CompileStaticConstToRegister(amd64.UCOMISS,
				asm.NewStaticConst([]byte{0, 0, 0, 0}), source.register)
		} else {
			err = c.assembler.CompileStaticConstToRegister(amd64.UCOMISD,
				asm.NewStaticConst([]byte{0, 0, 0, 0, 0, 0, 0, 0}), source.register)
		}
		if err != nil {
			return err
		}
		jmpIfMinimumSignedInt := c.assembler.CompileJump(amd64.JCS) // jump if the value is minus (= the minimum signed 32-bit int).

		// If the value exceeds signed 32-bit maximum, we saturate it to the maximum.
		if err = c.assembler.CompileStaticConstToRegister(amd64.MOVL,
			asm.NewStaticConst(u32.LeBytes(uint32(maximum32BitSignedInt))), result); err != nil {
			return err
		}

		c.assembler.SetJumpTargetOnNext(okJmp, nontrappingNanJump, nonTrappingSaturatedMinimumJump, jmpIfMinimumSignedInt)
	}

	// We consumed the source's register and placed the conversion result
	// in the result register.
	c.locationStack.markRegisterUnused(source.register)
	c.pushRuntimeValueLocationOnRegister(result, runtimeValueTypeI32)
	return nil
}

// emitSignedI64TruncFromFloat implements compileITruncFromF when the destination type is a 64-bit signed integer.
func (c *amd64Compiler) emitSignedI64TruncFromFloat(isFloat32Bit, nonTrapping bool) error {
	source := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(source); err != nil {
		return err
	}

	result, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}

	// First we unconditionally convert source to integer via CVTTSS2SI (CVTTSD2SI for 64bit float).
	if isFloat32Bit {
		c.assembler.CompileRegisterToRegister(amd64.CVTTSS2SQ, source.register, result)
	} else {
		c.assembler.CompileRegisterToRegister(amd64.CVTTSD2SQ, source.register, result)
	}

	// We compare the conversion result with the sign bit mask to check if it is either
	// 1) the source float value is either +-Inf or NaN, or it exceeds representative ranges of 32bit signed integer, or
	// 2) the source equals the minimum signed 32-bit (=-9223372036854775808.0) whose bit pattern is float32ForMinimumSigned64bitIntegerAddress for 32 bit float
	// 	  or float64ForMinimumSigned64bitIntegerAddress for 64bit float.
	err = c.assembler.CompileStaticConstToRegister(amd64.CMPQ,
		asm.NewStaticConst(u64.LeBytes(float64SignBitMask)), result)
	if err != nil {
		return err
	}

	// Otherwise, we simply jump to exit as the result is valid.
	okJmp := c.assembler.CompileJump(amd64.JNE)

	// Start handling the case of 1) and 2).
	// First, check if the value is NaN.
	if isFloat32Bit {
		c.assembler.CompileRegisterToRegister(amd64.UCOMISS, source.register, source.register)
	} else {
		c.assembler.CompileRegisterToRegister(amd64.UCOMISD, source.register, source.register)
	}

	// Check the parity flag (set when the value is NaN), and if it is set, we should raise an exception.
	jmpIfNotNaN := c.assembler.CompileJump(amd64.JPC) // jump if parity is not set.

	var nontrappingNanJump asm.Node
	if !nonTrapping {
		c.compileExitFromNativeCode(nativeCallStatusCodeInvalidFloatToIntConversion)
	} else {
		// In non trapping case, NaN is casted as zero.
		// Zero out the result register by XOR itsself.
		c.assembler.CompileRegisterToRegister(amd64.XORQ, result, result)
		nontrappingNanJump = c.assembler.CompileJump(amd64.JMP)
	}

	// Check if the value is larger than or equal the minimum 64-bit integer value,
	// meaning that the value exceeds the lower bound of 64-bit signed integer range.
	c.assembler.SetJumpTargetOnNext(jmpIfNotNaN)
	if isFloat32Bit {
		err = c.assembler.CompileStaticConstToRegister(amd64.UCOMISS,
			asm.NewStaticConst(u32.LeBytes(float32ForMinimumSigned64bitInteger)), source.register)
	} else {
		err = c.assembler.CompileStaticConstToRegister(amd64.UCOMISD,
			asm.NewStaticConst(u64.LeBytes(float64ForMinimumSigned64bitInteger)), source.register)
	}
	if err != nil {
		return err
	}

	if !nonTrapping {
		// Jump if the value is -Inf.
		jmpIfExceedsLowerBound := c.assembler.CompileJump(amd64.JCS)

		// At this point, the value is the minimum signed 64-bit int (=-9223372036854775808.0) or larger than 64-bit maximum.
		// So, check if the value equals the minimum signed 64-bit int.
		if isFloat32Bit {
			err = c.assembler.CompileStaticConstToRegister(amd64.UCOMISS,
				asm.NewStaticConst([]byte{0, 0, 0, 0}), source.register)
		} else {
			err = c.assembler.CompileStaticConstToRegister(amd64.UCOMISD,
				asm.NewStaticConst([]byte{0, 0, 0, 0, 0, 0, 0, 0}), source.register)
		}
		if err != nil {
			return err
		}

		jmpIfMinimumSignedInt := c.assembler.CompileJump(amd64.JCS) // jump if the value is minus (= the minimum signed 64-bit int).

		c.assembler.SetJumpTargetOnNext(jmpIfExceedsLowerBound)
		c.compileExitFromNativeCode(nativeCallStatusIntegerOverflow)

		// We jump to the next instructions for valid cases.
		c.assembler.SetJumpTargetOnNext(okJmp, jmpIfMinimumSignedInt)
	} else {
		// Jump if the value is not -Inf.
		jmpIfNotExceedsLowerBound := c.assembler.CompileJump(amd64.JCC)

		// If the value exceeds the lower bound, we "saturate" it to the minimum.
		err = c.assembler.CompileStaticConstToRegister(amd64.MOVQ,
			asm.NewStaticConst(u64.LeBytes(uint64(minimum64BitSignedInt))), result)
		if err != nil {
			return err
		}

		nonTrappingSaturatedMinimumJump := c.assembler.CompileJump(amd64.JMP)

		// Otherwise, the value is the minimum signed 64-bit int (=-9223372036854775808.0) or larger than 64-bit maximum.
		// So, check if the value equals the minimum signed 64-bit int.
		c.assembler.SetJumpTargetOnNext(jmpIfNotExceedsLowerBound)
		if isFloat32Bit {
			err = c.assembler.CompileStaticConstToRegister(amd64.UCOMISS, asm.NewStaticConst([]byte{0, 0, 0, 0}), source.register)
		} else {
			err = c.assembler.CompileStaticConstToRegister(amd64.UCOMISD, asm.NewStaticConst([]byte{0, 0, 0, 0, 0, 0, 0, 0}), source.register)
		}
		if err != nil {
			return err
		}

		jmpIfMinimumSignedInt := c.assembler.CompileJump(amd64.JCS) // jump if the value is minus (= the minimum signed 64-bit int).

		// If the value exceeds signed 64-bit maximum, we saturate it to the maximum.
		if err = c.assembler.CompileStaticConstToRegister(amd64.MOVQ, asm.NewStaticConst(u64.LeBytes(uint64(maximum64BitSignedInt))), result); err != nil {
			return err
		}

		c.assembler.SetJumpTargetOnNext(okJmp, jmpIfMinimumSignedInt, nonTrappingSaturatedMinimumJump, nontrappingNanJump)
	}

	// We consumed the source's register and placed the conversion result
	// in the result register.
	c.locationStack.markRegisterUnused(source.register)
	c.pushRuntimeValueLocationOnRegister(result, runtimeValueTypeI64)
	return nil
}

// compileFConvertFromI implements compiler.compileFConvertFromI for the amd64 architecture.
func (c *amd64Compiler) compileFConvertFromI(o wazeroir.OperationFConvertFromI) (err error) {
	if o.OutputType == wazeroir.Float32 && o.InputType == wazeroir.SignedInt32 {
		err = c.compileSimpleConversion(amd64.CVTSL2SS, registerTypeVector, runtimeValueTypeF32) // = CVTSI2SS for 32bit int
	} else if o.OutputType == wazeroir.Float32 && o.InputType == wazeroir.SignedInt64 {
		err = c.compileSimpleConversion(amd64.CVTSQ2SS, registerTypeVector, runtimeValueTypeF32) // = CVTSI2SS for 64bit int
	} else if o.OutputType == wazeroir.Float64 && o.InputType == wazeroir.SignedInt32 {
		err = c.compileSimpleConversion(amd64.CVTSL2SD, registerTypeVector, runtimeValueTypeF64) // = CVTSI2SD for 32bit int
	} else if o.OutputType == wazeroir.Float64 && o.InputType == wazeroir.SignedInt64 {
		err = c.compileSimpleConversion(amd64.CVTSQ2SD, registerTypeVector, runtimeValueTypeF64) // = CVTSI2SD for 64bit int
	} else if o.OutputType == wazeroir.Float32 && o.InputType == wazeroir.SignedUint32 {
		// See the following link for why we use 64bit conversion for unsigned 32bit integer sources:
		// https://stackoverflow.com/questions/41495498/fpu-operations-generated-by-gcc-during-casting-integer-to-float.
		//
		// Here's the summary:
		// >> CVTSI2SS is indeed designed for converting a signed integer to a scalar single-precision float,
		// >> not an unsigned integer like you have here. So what gives? Well, a 64-bit processor has 64-bit wide
		// >> registers available, so the unsigned 32-bit input values can be stored as signed 64-bit intermediate values,
		// >> which allows CVTSI2SS to be used after all.
		err = c.compileSimpleConversion(amd64.CVTSQ2SS, registerTypeVector, runtimeValueTypeF32) // = CVTSI2SS for 64bit int.
	} else if o.OutputType == wazeroir.Float64 && o.InputType == wazeroir.SignedUint32 {
		// For the same reason above, we use 64bit conversion for unsigned 32bit.
		err = c.compileSimpleConversion(amd64.CVTSQ2SD, registerTypeVector, runtimeValueTypeF64) // = CVTSI2SD for 64bit int.
	} else if o.OutputType == wazeroir.Float32 && o.InputType == wazeroir.SignedUint64 {
		err = c.emitUnsignedInt64ToFloatConversion(true)
	} else if o.OutputType == wazeroir.Float64 && o.InputType == wazeroir.SignedUint64 {
		err = c.emitUnsignedInt64ToFloatConversion(false)
	}
	return
}

// emitUnsignedInt64ToFloatConversion is handling the case of unsigned 64-bit integer
// in compileFConvertFromI.
func (c *amd64Compiler) emitUnsignedInt64ToFloatConversion(isFloat32bit bool) error {
	// The logic here is exactly the same as GCC emits for the following code:
	//
	// float convert(int num) {
	//     float foo;
	//     uint64_t ptr1 = 100;
	//     foo = (float)(ptr1);
	//     return foo;
	// }
	//
	// which is compiled by GCC as
	//
	// convert:
	// 	   push    rbp
	// 	   mov     rbp, rsp
	// 	   mov     DWORD PTR [rbp-20], edi
	// 	   mov     DWORD PTR [rbp-4], 100
	// 	   mov     eax, DWORD PTR [rbp-4]
	// 	   test    rax, rax
	// 	   js      .handle_sign_bit_case
	// 	   cvtsi2ss        xmm0, rax
	// 	   jmp     .exit
	// .handle_sign_bit_case:
	// 	   mov     rdx, rax
	// 	   shr     rdx
	// 	   and     eax, 1
	// 	   or      rdx, rax
	// 	   cvtsi2ss        xmm0, rdx
	// 	   addsd   xmm0, xmm0
	// .exit: ...
	//
	// tl;dr is that we have a branch depending on whether or not sign bit is set.

	origin := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(origin); err != nil {
		return err
	}

	dest, err := c.allocateRegister(registerTypeVector)
	if err != nil {
		return err
	}

	c.locationStack.markRegisterUsed(dest)

	tmpReg, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}

	// Check if the most significant bit (sign bit) is set.
	c.assembler.CompileRegisterToRegister(amd64.TESTQ, origin.register, origin.register)

	// Jump if the sign bit is set.
	jmpIfSignbitSet := c.assembler.CompileJump(amd64.JMI)

	// Otherwise, we could fit the unsigned int into float32.
	// So, we convert it to float32 and emit jump instruction to exit from this branch.
	if isFloat32bit {
		c.assembler.CompileRegisterToRegister(amd64.CVTSQ2SS, origin.register, dest)
	} else {
		c.assembler.CompileRegisterToRegister(amd64.CVTSQ2SD, origin.register, dest)
	}
	exitFromSignbitUnSet := c.assembler.CompileJump(amd64.JMP)

	// Now handling the case where sign-bit is set.
	// We emit the following sequences:
	// 	   mov     tmpReg, origin
	// 	   shr     tmpReg, 1
	// 	   and     origin, 1
	// 	   or      tmpReg, origin
	// 	   cvtsi2ss        xmm0, tmpReg
	// 	   addsd   xmm0, xmm0

	c.assembler.SetJumpTargetOnNext(jmpIfSignbitSet)
	c.assembler.CompileRegisterToRegister(amd64.MOVQ, origin.register, tmpReg)
	c.assembler.CompileConstToRegister(amd64.SHRQ, 1, tmpReg)
	c.assembler.CompileConstToRegister(amd64.ANDQ, 1, origin.register)
	c.assembler.CompileRegisterToRegister(amd64.ORQ, origin.register, tmpReg)
	if isFloat32bit {
		c.assembler.CompileRegisterToRegister(amd64.CVTSQ2SS, tmpReg, dest)
	} else {
		c.assembler.CompileRegisterToRegister(amd64.CVTSQ2SD, tmpReg, dest)
	}
	if isFloat32bit {
		c.assembler.CompileRegisterToRegister(amd64.ADDSS, dest, dest)
	} else {
		c.assembler.CompileRegisterToRegister(amd64.ADDSD, dest, dest)
	}

	// Now, we finished the sign-bit set branch.
	// We have to make the exit jump target of sign-bit unset branch
	// towards the next instruction.
	c.assembler.SetJumpTargetOnNext(exitFromSignbitUnSet)

	// We consumed the origin's register and placed the conversion result
	// in the dest register.
	c.locationStack.markRegisterUnused(origin.register)
	if isFloat32bit {
		c.pushRuntimeValueLocationOnRegister(dest, runtimeValueTypeF32)
	} else {
		c.pushRuntimeValueLocationOnRegister(dest, runtimeValueTypeF64)
	}
	return nil
}

// compileSimpleConversion pops a value type from the stack, and applies the
// given instruction on it, and push the result onto a register of the given type.
func (c *amd64Compiler) compileSimpleConversion(convInstruction asm.Instruction,
	destinationRegisterType registerType, destinationValueType runtimeValueType,
) error {
	origin := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(origin); err != nil {
		return err
	}

	dest, err := c.allocateRegister(destinationRegisterType)
	if err != nil {
		return err
	}

	c.assembler.CompileRegisterToRegister(convInstruction, origin.register, dest)

	c.locationStack.markRegisterUnused(origin.register)
	c.pushRuntimeValueLocationOnRegister(dest, destinationValueType)
	return nil
}

// compileF32DemoteFromF64 implements compiler.compileF32DemoteFromF64 for the amd64 architecture.
func (c *amd64Compiler) compileF32DemoteFromF64() error {
	target := c.locationStack.peek() // Note this is peek!
	if err := c.compileEnsureOnRegister(target); err != nil {
		return err
	}

	c.assembler.CompileRegisterToRegister(amd64.CVTSD2SS, target.register, target.register)
	target.valueType = runtimeValueTypeF32
	return nil
}

// compileF64PromoteFromF32 implements compiler.compileF64PromoteFromF32 for the amd64 architecture.
func (c *amd64Compiler) compileF64PromoteFromF32() error {
	target := c.locationStack.peek() // Note this is peek!
	if err := c.compileEnsureOnRegister(target); err != nil {
		return err
	}

	c.assembler.CompileRegisterToRegister(amd64.CVTSS2SD, target.register, target.register)
	target.valueType = runtimeValueTypeF64
	return nil
}

// compileI32ReinterpretFromF32 implements compiler.compileI32ReinterpretFromF32 for the amd64 architecture.
func (c *amd64Compiler) compileI32ReinterpretFromF32() error {
	if peek := c.locationStack.peek(); peek.onStack() {
		// If the value is on the stack, this is no-op as there is nothing to do for converting type.
		peek.valueType = runtimeValueTypeI32
		return nil
	}
	return c.compileSimpleConversion(amd64.MOVL, registerTypeGeneralPurpose, runtimeValueTypeI32)
}

// compileI64ReinterpretFromF64 implements compiler.compileI64ReinterpretFromF64 for the amd64 architecture.
func (c *amd64Compiler) compileI64ReinterpretFromF64() error {
	if peek := c.locationStack.peek(); peek.onStack() {
		// If the value is on the stack, this is no-op as there is nothing to do for converting type.
		peek.valueType = runtimeValueTypeI64
		return nil
	}
	return c.compileSimpleConversion(amd64.MOVQ, registerTypeGeneralPurpose, runtimeValueTypeI64)
}

// compileF32ReinterpretFromI32 implements compiler.compileF32ReinterpretFromI32 for the amd64 architecture.
func (c *amd64Compiler) compileF32ReinterpretFromI32() error {
	if peek := c.locationStack.peek(); peek.onStack() {
		// If the value is on the stack, this is no-op as there is nothing to do for converting type.
		peek.valueType = runtimeValueTypeF32
		return nil
	}
	return c.compileSimpleConversion(amd64.MOVL, registerTypeVector, runtimeValueTypeF32)
}

// compileF64ReinterpretFromI64 implements compiler.compileF64ReinterpretFromI64 for the amd64 architecture.
func (c *amd64Compiler) compileF64ReinterpretFromI64() error {
	if peek := c.locationStack.peek(); peek.onStack() {
		// If the value is on the stack, this is no-op as there is nothing to do for converting type.
		peek.valueType = runtimeValueTypeF64
		return nil
	}
	return c.compileSimpleConversion(amd64.MOVQ, registerTypeVector, runtimeValueTypeF64)
}

// compileExtend implements compiler.compileExtend for the amd64 architecture.
func (c *amd64Compiler) compileExtend(o wazeroir.OperationExtend) error {
	var inst asm.Instruction
	if o.Signed {
		inst = amd64.MOVLQSX // = MOVSXD https://www.felixcloutier.com/x86/movsx:movsxd
	} else {
		inst = amd64.MOVL
	}
	return c.compileExtendImpl(inst, runtimeValueTypeI64)
}

// compileSignExtend32From8 implements compiler.compileSignExtend32From8 for the amd64 architecture.
func (c *amd64Compiler) compileSignExtend32From8() error {
	return c.compileExtendImpl(amd64.MOVBLSX, runtimeValueTypeI32)
}

// compileSignExtend32From16 implements compiler.compileSignExtend32From16 for the amd64 architecture.
func (c *amd64Compiler) compileSignExtend32From16() error {
	return c.compileExtendImpl(amd64.MOVWLSX, runtimeValueTypeI32)
}

// compileSignExtend64From8 implements compiler.compileSignExtend64From8 for the amd64 architecture.
func (c *amd64Compiler) compileSignExtend64From8() error {
	return c.compileExtendImpl(amd64.MOVBQSX, runtimeValueTypeI64)
}

// compileSignExtend64From16 implements compiler.compileSignExtend64From16 for the amd64 architecture.
func (c *amd64Compiler) compileSignExtend64From16() error {
	return c.compileExtendImpl(amd64.MOVWQSX, runtimeValueTypeI64)
}

// compileSignExtend64From32 implements compiler.compileSignExtend64From32 for the amd64 architecture.
func (c *amd64Compiler) compileSignExtend64From32() error {
	return c.compileExtendImpl(amd64.MOVLQSX, runtimeValueTypeI64)
}

func (c *amd64Compiler) compileExtendImpl(inst asm.Instruction, destinationType runtimeValueType) error {
	target := c.locationStack.peek() // Note this is peek!
	if err := c.compileEnsureOnRegister(target); err != nil {
		return err
	}

	c.assembler.CompileRegisterToRegister(inst, target.register, target.register)
	target.valueType = destinationType
	return nil
}

// compileEq implements compiler.compileEq for the amd64 architecture.
func (c *amd64Compiler) compileEq(o wazeroir.OperationEq) error {
	return c.compileEqOrNe(o.Type, true)
}

// compileNe implements compiler.compileNe for the amd64 architecture.
func (c *amd64Compiler) compileNe(o wazeroir.OperationNe) error {
	return c.compileEqOrNe(o.Type, false)
}

func (c *amd64Compiler) compileEqOrNe(t wazeroir.UnsignedType, shouldEqual bool) (err error) {
	x2 := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(x2); err != nil {
		return err
	}

	x1 := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(x1); err != nil {
		return err
	}

	x1r, x2r := x1.register, x2.register

	// x1 and x2 are temporary registers only used for the cmp operation. Release them.
	c.locationStack.releaseRegister(x1)
	c.locationStack.releaseRegister(x2)

	switch t {
	case wazeroir.UnsignedTypeI32:
		err = c.compileEqOrNeForInts(x1r, x2r, amd64.CMPL, shouldEqual)
	case wazeroir.UnsignedTypeI64:
		err = c.compileEqOrNeForInts(x1r, x2r, amd64.CMPQ, shouldEqual)
	case wazeroir.UnsignedTypeF32:
		err = c.compileEqOrNeForFloats(x1r, x2r, amd64.UCOMISS, shouldEqual)
	case wazeroir.UnsignedTypeF64:
		err = c.compileEqOrNeForFloats(x1r, x2r, amd64.UCOMISD, shouldEqual)
	}
	if err != nil {
		return
	}
	return
}

func (c *amd64Compiler) compileEqOrNeForInts(x1Reg, x2Reg asm.Register, cmpInstruction asm.Instruction,
	shouldEqual bool,
) error {
	c.assembler.CompileRegisterToRegister(cmpInstruction, x2Reg, x1Reg)

	// Record that the result is on the conditional register.
	var condReg asm.ConditionalRegisterState
	if shouldEqual {
		condReg = amd64.ConditionalRegisterStateE
	} else {
		condReg = amd64.ConditionalRegisterStateNE
	}
	loc := c.locationStack.pushRuntimeValueLocationOnConditionalRegister(condReg)
	loc.valueType = runtimeValueTypeI32
	return nil
}

// For float EQ and NE, we have to take NaN values into account.
// Notably, Wasm specification states that if one of targets is NaN,
// the result must be zero for EQ or one for NE.
func (c *amd64Compiler) compileEqOrNeForFloats(x1Reg, x2Reg asm.Register, cmpInstruction asm.Instruction, shouldEqual bool) error {
	// Before we allocate the result, we have to reserve two int registers.
	nanFragReg, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}
	c.locationStack.markRegisterUsed(nanFragReg)
	cmpResultReg, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}

	// Then, execute the comparison.
	c.assembler.CompileRegisterToRegister(cmpInstruction, x2Reg, x1Reg)

	// First, we get the parity flag which indicates whether one of values was NaN.
	if shouldEqual {
		// Set 1 if two values are NOT NaN.
		c.assembler.CompileNoneToRegister(amd64.SETPC, nanFragReg)
	} else {
		// Set 1 if one of values is NaN.
		c.assembler.CompileNoneToRegister(amd64.SETPS, nanFragReg)
	}

	// next, we get the usual comparison flag.
	if shouldEqual {
		// Set 1 if equal.
		c.assembler.CompileNoneToRegister(amd64.SETEQ, cmpResultReg)
	} else {
		// Set 1 if not equal.
		c.assembler.CompileNoneToRegister(amd64.SETNE, cmpResultReg)
	}

	// Do "and" or "or" operations on these two flags to get the actual result.
	if shouldEqual {
		c.assembler.CompileRegisterToRegister(amd64.ANDL, nanFragReg, cmpResultReg)
	} else {
		c.assembler.CompileRegisterToRegister(amd64.ORL, nanFragReg, cmpResultReg)
	}

	// Clear the unnecessary bits by zero extending the first byte.
	// This is necessary the upper bits (5 to 32 bits) of SET* instruction result is undefined.
	c.assembler.CompileRegisterToRegister(amd64.MOVBLZX, cmpResultReg, cmpResultReg)

	// Now we have the result in cmpResultReg register, so we record it.
	c.pushRuntimeValueLocationOnRegister(cmpResultReg, runtimeValueTypeI32)
	// Also, we no longer need nanFragRegister.
	c.locationStack.markRegisterUnused(nanFragReg)
	return nil
}

// compileEqz implements compiler.compileEqz for the amd64 architecture.
func (c *amd64Compiler) compileEqz(o wazeroir.OperationEqz) (err error) {
	v := c.locationStack.pop()
	if err = c.compileEnsureOnRegister(v); err != nil {
		return err
	}

	switch o.Type {
	case wazeroir.UnsignedInt32:
		err = c.assembler.CompileStaticConstToRegister(amd64.CMPL, asm.NewStaticConst([]byte{0, 0, 0, 0}), v.register)
	case wazeroir.UnsignedInt64:
		err = c.assembler.CompileStaticConstToRegister(amd64.CMPQ, asm.NewStaticConst([]byte{0, 0, 0, 0, 0, 0, 0, 0}), v.register)
	}
	if err != nil {
		return err
	}

	// v is consumed by the cmp operation so release it.
	c.locationStack.releaseRegister(v)

	// Finally, record that the result is on the conditional register.
	loc := c.locationStack.pushRuntimeValueLocationOnConditionalRegister(amd64.ConditionalRegisterStateE)
	loc.valueType = runtimeValueTypeI32
	return nil
}

// compileLt implements compiler.compileLt for the amd64 architecture.
func (c *amd64Compiler) compileLt(o wazeroir.OperationLt) error {
	x2 := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(x2); err != nil {
		return err
	}

	x1 := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(x1); err != nil {
		return err
	}

	// Emit the compare instruction.
	var resultConditionState asm.ConditionalRegisterState
	var inst asm.Instruction
	switch o.Type {
	case wazeroir.SignedTypeInt32:
		resultConditionState = amd64.ConditionalRegisterStateL
		inst = amd64.CMPL
	case wazeroir.SignedTypeUint32:
		resultConditionState = amd64.ConditionalRegisterStateB
		inst = amd64.CMPL
	case wazeroir.SignedTypeInt64:
		inst = amd64.CMPQ
		resultConditionState = amd64.ConditionalRegisterStateL
	case wazeroir.SignedTypeUint64:
		resultConditionState = amd64.ConditionalRegisterStateB
		inst = amd64.CMPQ
	case wazeroir.SignedTypeFloat32:
		resultConditionState = amd64.ConditionalRegisterStateA
		inst = amd64.COMISS
	case wazeroir.SignedTypeFloat64:
		resultConditionState = amd64.ConditionalRegisterStateA
		inst = amd64.COMISD
	}
	c.assembler.CompileRegisterToRegister(inst, x1.register, x2.register)

	// x1 and x2 are temporary registers only used for the cmp operation. Release them.
	c.locationStack.releaseRegister(x1)
	c.locationStack.releaseRegister(x2)

	// Finally, record that the result is on the conditional register.
	loc := c.locationStack.pushRuntimeValueLocationOnConditionalRegister(resultConditionState)
	loc.valueType = runtimeValueTypeI32
	return nil
}

// compileGt implements compiler.compileGt for the amd64 architecture.
func (c *amd64Compiler) compileGt(o wazeroir.OperationGt) error {
	x2 := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(x2); err != nil {
		return err
	}

	x1 := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(x1); err != nil {
		return err
	}

	// Emit the compare instruction.
	var resultConditionState asm.ConditionalRegisterState
	switch o.Type {
	case wazeroir.SignedTypeInt32:
		resultConditionState = amd64.ConditionalRegisterStateG
		c.assembler.CompileRegisterToRegister(amd64.CMPL, x1.register, x2.register)
	case wazeroir.SignedTypeUint32:
		c.assembler.CompileRegisterToRegister(amd64.CMPL, x1.register, x2.register)
		resultConditionState = amd64.ConditionalRegisterStateA
	case wazeroir.SignedTypeInt64:
		c.assembler.CompileRegisterToRegister(amd64.CMPQ, x1.register, x2.register)
		resultConditionState = amd64.ConditionalRegisterStateG
	case wazeroir.SignedTypeUint64:
		c.assembler.CompileRegisterToRegister(amd64.CMPQ, x1.register, x2.register)
		resultConditionState = amd64.ConditionalRegisterStateA
	case wazeroir.SignedTypeFloat32:
		c.assembler.CompileRegisterToRegister(amd64.UCOMISS, x2.register, x1.register)
		resultConditionState = amd64.ConditionalRegisterStateA
	case wazeroir.SignedTypeFloat64:
		c.assembler.CompileRegisterToRegister(amd64.UCOMISD, x2.register, x1.register)
		resultConditionState = amd64.ConditionalRegisterStateA
	}

	// x1 and x2 are temporary registers only used for the cmp operation. Release them.
	c.locationStack.releaseRegister(x1)
	c.locationStack.releaseRegister(x2)

	// Finally, record that the result is on the conditional register.
	loc := c.locationStack.pushRuntimeValueLocationOnConditionalRegister(resultConditionState)
	loc.valueType = runtimeValueTypeI32
	return nil
}

// compileLe implements compiler.compileLe for the amd64 architecture.
func (c *amd64Compiler) compileLe(o wazeroir.OperationLe) error {
	x2 := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(x2); err != nil {
		return err
	}

	x1 := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(x1); err != nil {
		return err
	}

	// Emit the compare instruction.
	var inst asm.Instruction
	var resultConditionState asm.ConditionalRegisterState
	switch o.Type {
	case wazeroir.SignedTypeInt32:
		resultConditionState = amd64.ConditionalRegisterStateLE
		inst = amd64.CMPL
	case wazeroir.SignedTypeUint32:
		resultConditionState = amd64.ConditionalRegisterStateBE
		inst = amd64.CMPL
	case wazeroir.SignedTypeInt64:
		resultConditionState = amd64.ConditionalRegisterStateLE
		inst = amd64.CMPQ
	case wazeroir.SignedTypeUint64:
		resultConditionState = amd64.ConditionalRegisterStateBE
		inst = amd64.CMPQ
	case wazeroir.SignedTypeFloat32:
		resultConditionState = amd64.ConditionalRegisterStateAE
		inst = amd64.UCOMISS
	case wazeroir.SignedTypeFloat64:
		resultConditionState = amd64.ConditionalRegisterStateAE
		inst = amd64.UCOMISD
	}
	c.assembler.CompileRegisterToRegister(inst, x1.register, x2.register)

	// x1 and x2 are temporary registers only used for the cmp operation. Release them.
	c.locationStack.releaseRegister(x1)
	c.locationStack.releaseRegister(x2)

	// Finally, record that the result is on the conditional register.
	loc := c.locationStack.pushRuntimeValueLocationOnConditionalRegister(resultConditionState)
	loc.valueType = runtimeValueTypeI32
	return nil
}

// compileGe implements compiler.compileGe for the amd64 architecture.
func (c *amd64Compiler) compileGe(o wazeroir.OperationGe) error {
	x2 := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(x2); err != nil {
		return err
	}

	x1 := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(x1); err != nil {
		return err
	}

	// Emit the compare instruction.
	var resultConditionState asm.ConditionalRegisterState
	switch o.Type {
	case wazeroir.SignedTypeInt32:
		c.assembler.CompileRegisterToRegister(amd64.CMPL, x1.register, x2.register)
		resultConditionState = amd64.ConditionalRegisterStateGE
	case wazeroir.SignedTypeUint32:
		c.assembler.CompileRegisterToRegister(amd64.CMPL, x1.register, x2.register)
		resultConditionState = amd64.ConditionalRegisterStateAE
	case wazeroir.SignedTypeInt64:
		c.assembler.CompileRegisterToRegister(amd64.CMPQ, x1.register, x2.register)
		resultConditionState = amd64.ConditionalRegisterStateGE
	case wazeroir.SignedTypeUint64:
		c.assembler.CompileRegisterToRegister(amd64.CMPQ, x1.register, x2.register)
		resultConditionState = amd64.ConditionalRegisterStateAE
	case wazeroir.SignedTypeFloat32:
		c.assembler.CompileRegisterToRegister(amd64.COMISS, x2.register, x1.register)
		resultConditionState = amd64.ConditionalRegisterStateAE
	case wazeroir.SignedTypeFloat64:
		c.assembler.CompileRegisterToRegister(amd64.COMISD, x2.register, x1.register)
		resultConditionState = amd64.ConditionalRegisterStateAE
	}

	// x1 and x2 are temporary registers only used for the cmp operation. Release them.
	c.locationStack.releaseRegister(x1)
	c.locationStack.releaseRegister(x2)

	// Finally, record that the result is on the conditional register.
	loc := c.locationStack.pushRuntimeValueLocationOnConditionalRegister(resultConditionState)
	loc.valueType = runtimeValueTypeI32
	return nil
}

// compileLoad implements compiler.compileLoad for the amd64 architecture.
func (c *amd64Compiler) compileLoad(o wazeroir.OperationLoad) error {
	var (
		isIntType         bool
		movInst           asm.Instruction
		targetSizeInBytes int64
		vt                runtimeValueType
	)
	switch o.Type {
	case wazeroir.UnsignedTypeI32:
		isIntType = true
		movInst = amd64.MOVL
		targetSizeInBytes = 32 / 8
		vt = runtimeValueTypeI32
	case wazeroir.UnsignedTypeI64:
		isIntType = true
		movInst = amd64.MOVQ
		targetSizeInBytes = 64 / 8
		vt = runtimeValueTypeI64
	case wazeroir.UnsignedTypeF32:
		isIntType = false
		movInst = amd64.MOVL
		targetSizeInBytes = 32 / 8
		vt = runtimeValueTypeF32
	case wazeroir.UnsignedTypeF64:
		isIntType = false
		movInst = amd64.MOVQ
		targetSizeInBytes = 64 / 8
		vt = runtimeValueTypeF64
	}

	reg, err := c.compileMemoryAccessCeilSetup(o.Arg.Offset, targetSizeInBytes)
	if err != nil {
		return err
	}

	if isIntType {
		// For integer types, read the corresponding bytes from the offset to the memory
		// and store the value to the int register.
		c.assembler.CompileMemoryWithIndexToRegister(movInst,
			// we access memory as memory.Buffer[ceil-targetSizeInBytes: ceil].
			amd64ReservedRegisterForMemory, -targetSizeInBytes, reg, 1,
			reg)
		c.pushRuntimeValueLocationOnRegister(reg, vt)
	} else {
		// For float types, we read the value to the float register.
		floatReg, err := c.allocateRegister(registerTypeVector)
		if err != nil {
			return err
		}
		c.assembler.CompileMemoryWithIndexToRegister(movInst,
			// we access memory as memory.Buffer[ceil-targetSizeInBytes: ceil].
			amd64ReservedRegisterForMemory, -targetSizeInBytes, reg, 1,
			floatReg)
		c.pushRuntimeValueLocationOnRegister(floatReg, vt)
		// We no longer need the int register so mark it unused.
		c.locationStack.markRegisterUnused(reg)
	}
	return nil
}

// compileLoad8 implements compiler.compileLoad8 for the amd64 architecture.
func (c *amd64Compiler) compileLoad8(o wazeroir.OperationLoad8) error {
	const targetSizeInBytes = 1
	reg, err := c.compileMemoryAccessCeilSetup(o.Arg.Offset, targetSizeInBytes)
	if err != nil {
		return err
	}

	// Then move a byte at the offset to the register.
	// Note that Load8 is only for integer types.
	var inst asm.Instruction
	var vt runtimeValueType
	switch o.Type {
	case wazeroir.SignedInt32:
		inst = amd64.MOVBLSX
		vt = runtimeValueTypeI32
	case wazeroir.SignedUint32:
		inst = amd64.MOVBLZX
		vt = runtimeValueTypeI32
	case wazeroir.SignedInt64:
		inst = amd64.MOVBQSX
		vt = runtimeValueTypeI64
	case wazeroir.SignedUint64:
		inst = amd64.MOVBQZX
		vt = runtimeValueTypeI64
	}

	c.assembler.CompileMemoryWithIndexToRegister(inst,
		// we access memory as memory.Buffer[ceil-targetSizeInBytes: ceil].
		amd64ReservedRegisterForMemory, -targetSizeInBytes, reg, 1,
		reg)

	c.pushRuntimeValueLocationOnRegister(reg, vt)
	return nil
}

// compileLoad16 implements compiler.compileLoad16 for the amd64 architecture.
func (c *amd64Compiler) compileLoad16(o wazeroir.OperationLoad16) error {
	const targetSizeInBytes = 16 / 8
	reg, err := c.compileMemoryAccessCeilSetup(o.Arg.Offset, targetSizeInBytes)
	if err != nil {
		return err
	}

	// Then move 2 bytes at the offset to the register.
	// Note that Load16 is only for integer types.
	var inst asm.Instruction
	var vt runtimeValueType
	switch o.Type {
	case wazeroir.SignedInt32:
		inst = amd64.MOVWLSX
		vt = runtimeValueTypeI32
	case wazeroir.SignedInt64:
		inst = amd64.MOVWQSX
		vt = runtimeValueTypeI64
	case wazeroir.SignedUint32:
		inst = amd64.MOVWLZX
		vt = runtimeValueTypeI32
	case wazeroir.SignedUint64:
		inst = amd64.MOVWQZX
		vt = runtimeValueTypeI64
	}

	c.assembler.CompileMemoryWithIndexToRegister(inst,
		// we access memory as memory.Buffer[ceil-targetSizeInBytes: ceil].
		amd64ReservedRegisterForMemory, -targetSizeInBytes, reg, 1,
		reg)

	c.pushRuntimeValueLocationOnRegister(reg, vt)
	return nil
}

// compileLoad32 implements compiler.compileLoad32 for the amd64 architecture.
func (c *amd64Compiler) compileLoad32(o wazeroir.OperationLoad32) error {
	const targetSizeInBytes = 32 / 8
	reg, err := c.compileMemoryAccessCeilSetup(o.Arg.Offset, targetSizeInBytes)
	if err != nil {
		return err
	}

	// Then move 4 bytes at the offset to the register.
	var inst asm.Instruction
	if o.Signed {
		inst = amd64.MOVLQSX
	} else {
		inst = amd64.MOVLQZX
	}
	c.assembler.CompileMemoryWithIndexToRegister(inst,
		// We access memory as memory.Buffer[ceil-targetSizeInBytes: ceil].
		amd64ReservedRegisterForMemory, -targetSizeInBytes, reg, 1,
		reg)
	c.pushRuntimeValueLocationOnRegister(reg, runtimeValueTypeI64)
	return nil
}

// compileMemoryAccessCeilSetup pops the top value from the stack (called "base"), stores "base + offsetArg + targetSizeInBytes"
// into a register, and returns the stored register. We call the result "ceil" because we access the memory
// as memory.Buffer[ceil-targetSizeInBytes: ceil].
//
// Note: this also emits the instructions to check the out-of-bounds memory access.
// In other words, if the ceil exceeds the memory size, the code exits with nativeCallStatusCodeMemoryOutOfBounds status.
func (c *amd64Compiler) compileMemoryAccessCeilSetup(offsetArg uint32, targetSizeInBytes int64) (asm.Register, error) {
	base := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(base); err != nil {
		return asm.NilRegister, err
	}

	result := base.register
	if offsetConst := int64(offsetArg) + targetSizeInBytes; offsetConst <= math.MaxInt32 {
		c.assembler.CompileConstToRegister(amd64.ADDQ, offsetConst, result)
	} else if offsetConst <= math.MaxUint32 {
		// Note: in practice, this branch rarely happens as in this case, the wasm binary know that
		// memory has more than 1 GBi or at least tries to access above 1 GBi memory region.
		//
		// This case, we cannot directly add the offset to a register by ADDQ(const) instruction.
		// That is because the imm32 const is sign-extended to 64-bit in ADDQ(const), and we end up
		// making offsetConst as the negative number, which is wrong.
		tmp, err := c.allocateRegister(registerTypeGeneralPurpose)
		if err != nil {
			return asm.NilRegister, err
		}
		c.assembler.CompileConstToRegister(amd64.MOVL, int64(uint32(offsetConst)), tmp)
		c.assembler.CompileRegisterToRegister(amd64.ADDQ, tmp, result)
	} else {
		// If the offset const is too large, we exit with nativeCallStatusCodeMemoryOutOfBounds.
		c.compileExitFromNativeCode(nativeCallStatusCodeMemoryOutOfBounds)
		return result, nil
	}

	// Now we compare the value with the memory length which is held by callEngine.
	c.assembler.CompileMemoryToRegister(amd64.CMPQ,
		amd64ReservedRegisterForCallEngine, callEngineModuleContextMemorySliceLenOffset, result)

	// Jump if the value is within the memory length.
	okJmp := c.assembler.CompileJump(amd64.JCC)

	// Otherwise, we exit the function with out-of-bounds status code.
	c.compileExitFromNativeCode(nativeCallStatusCodeMemoryOutOfBounds)

	c.assembler.SetJumpTargetOnNext(okJmp)

	c.locationStack.markRegisterUnused(result)
	return result, nil
}

// compileStore implements compiler.compileStore for the amd64 architecture.
func (c *amd64Compiler) compileStore(o wazeroir.OperationStore) error {
	var movInst asm.Instruction
	var targetSizeInByte int64
	switch o.Type {
	case wazeroir.UnsignedTypeI32, wazeroir.UnsignedTypeF32:
		movInst = amd64.MOVL
		targetSizeInByte = 32 / 8
	case wazeroir.UnsignedTypeI64, wazeroir.UnsignedTypeF64:
		movInst = amd64.MOVQ
		targetSizeInByte = 64 / 8
	}
	return c.compileStoreImpl(o.Arg.Offset, movInst, targetSizeInByte)
}

// compileStore8 implements compiler.compileStore8 for the amd64 architecture.
func (c *amd64Compiler) compileStore8(o wazeroir.OperationStore8) error {
	return c.compileStoreImpl(o.Arg.Offset, amd64.MOVB, 1)
}

// compileStore32 implements compiler.compileStore32 for the amd64 architecture.
func (c *amd64Compiler) compileStore16(o wazeroir.OperationStore16) error {
	return c.compileStoreImpl(o.Arg.Offset, amd64.MOVW, 16/8)
}

// compileStore32 implements compiler.compileStore32 for the amd64 architecture.
func (c *amd64Compiler) compileStore32(o wazeroir.OperationStore32) error {
	return c.compileStoreImpl(o.Arg.Offset, amd64.MOVL, 32/8)
}

func (c *amd64Compiler) compileStoreImpl(offsetConst uint32, inst asm.Instruction, targetSizeInBytes int64) error {
	val := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(val); err != nil {
		return err
	}

	reg, err := c.compileMemoryAccessCeilSetup(offsetConst, targetSizeInBytes)
	if err != nil {
		return nil
	}

	c.assembler.CompileRegisterToMemoryWithIndex(
		inst, val.register,
		amd64ReservedRegisterForMemory, -targetSizeInBytes, reg, 1,
	)

	// We no longer need both the value and base registers.
	c.locationStack.releaseRegister(val)
	c.locationStack.markRegisterUnused(reg)
	return nil
}

// compileMemoryGrow implements compiler.compileMemoryGrow for the amd64 architecture.
func (c *amd64Compiler) compileMemoryGrow() error {
	if err := c.maybeCompileMoveTopConditionalToGeneralPurposeRegister(); err != nil {
		return err
	}

	if err := c.compileCallBuiltinFunction(builtinFunctionIndexMemoryGrow); err != nil {
		return err
	}

	// After the function call, we have to initialize the stack base pointer and memory reserved registers.
	c.compileReservedStackBasePointerInitialization()
	c.compileReservedMemoryPointerInitialization()
	return nil
}

// compileMemorySize implements compiler.compileMemorySize for the amd64 architecture.
func (c *amd64Compiler) compileMemorySize() error {
	if err := c.maybeCompileMoveTopConditionalToGeneralPurposeRegister(); err != nil {
		return err
	}

	reg, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}
	loc := c.pushRuntimeValueLocationOnRegister(reg, runtimeValueTypeI32)

	c.assembler.CompileMemoryToRegister(amd64.MOVQ, amd64ReservedRegisterForCallEngine, callEngineModuleContextMemorySliceLenOffset, loc.register)

	// WebAssembly's memory.size returns the page size (65536) of memory region.
	// That is equivalent to divide the len of memory slice by 65536 and
	// that can be calculated as SHR by 16 bits as 65536 = 2^16.
	c.assembler.CompileConstToRegister(amd64.SHRQ, wasm.MemoryPageSizeInBits, loc.register)
	return nil
}

// compileMemoryInit implements compiler.compileMemoryInit for the amd64 architecture.
func (c *amd64Compiler) compileMemoryInit(o wazeroir.OperationMemoryInit) error {
	return c.compileInitImpl(false, o.DataIndex, 0)
}

// compileInitImpl implements compileTableInit and compileMemoryInit.
//
// TODO: the compiled code in this function should be reused and compile at once as
// the code is independent of any module.
func (c *amd64Compiler) compileInitImpl(isTable bool, index, tableIndex uint32) error {
	outOfBoundsErrorStatus := nativeCallStatusCodeMemoryOutOfBounds
	if isTable {
		outOfBoundsErrorStatus = nativeCallStatusCodeInvalidTableAccess
	}

	copySize := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(copySize); err != nil {
		return err
	}

	sourceOffset := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(sourceOffset); err != nil {
		return err
	}

	destinationOffset := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(destinationOffset); err != nil {
		return err
	}

	instanceAddr, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}
	c.locationStack.markRegisterUsed(instanceAddr)
	if isTable {
		c.compileLoadElemInstanceAddress(index, instanceAddr)
	} else {
		c.compileLoadDataInstanceAddress(index, instanceAddr)
	}

	tmp, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}
	c.locationStack.markRegisterUsed(tmp)

	// sourceOffset += size.
	c.assembler.CompileRegisterToRegister(amd64.ADDQ, copySize.register, sourceOffset.register)
	// destinationOffset += size.
	c.assembler.CompileRegisterToRegister(amd64.ADDQ, copySize.register, destinationOffset.register)

	// Check instance bounds and if exceeds the length, exit with out of bounds error.
	c.assembler.CompileMemoryToRegister(amd64.CMPQ,
		instanceAddr, 8, // DataInstance and Element instance holds the length is stored at offset 8.
		sourceOffset.register)
	sourceBoundOKJump := c.assembler.CompileJump(amd64.JCC)
	c.compileExitFromNativeCode(outOfBoundsErrorStatus)
	c.assembler.SetJumpTargetOnNext(sourceBoundOKJump)

	// Check destination bounds and if exceeds the length, exit with out of bounds error.
	if isTable {
		// Load the target table's address.
		c.assembler.CompileMemoryToRegister(amd64.MOVQ, amd64ReservedRegisterForCallEngine, callEngineModuleContextTablesElement0AddressOffset, tmp)
		c.assembler.CompileMemoryToRegister(amd64.MOVQ, tmp, int64(tableIndex*8), tmp)
		// Compare length.
		c.assembler.CompileMemoryToRegister(amd64.CMPQ, tmp, tableInstanceTableLenOffset, destinationOffset.register)
	} else {
		c.assembler.CompileMemoryToRegister(amd64.CMPQ,
			amd64ReservedRegisterForCallEngine, callEngineModuleContextMemorySliceLenOffset,
			destinationOffset.register)
	}

	destinationBoundOKJump := c.assembler.CompileJump(amd64.JCC)
	c.compileExitFromNativeCode(outOfBoundsErrorStatus)
	c.assembler.SetJumpTargetOnNext(destinationBoundOKJump)

	// Otherwise, ready to copy the value from source to destination.
	//
	// If the copy size equal zero, we skip the entire instructions below.
	c.assembler.CompileRegisterToConst(amd64.CMPQ, copySize.register, 0)
	skipJump := c.assembler.CompileJump(amd64.JEQ)

	var scale int16
	var memToReg, regToMem asm.Instruction
	if isTable {
		// Each element is of type uintptr; 2^3 = 1 << pointerSizeLog2.
		c.assembler.CompileConstToRegister(amd64.SHLQ, pointerSizeLog2, sourceOffset.register)
		c.assembler.CompileConstToRegister(amd64.SHLQ, pointerSizeLog2, destinationOffset.register)
		// destinationOffset += table buffer's absolute address.
		c.assembler.CompileMemoryToRegister(amd64.ADDQ,
			tmp, tableInstanceTableOffset, destinationOffset.register)
		// sourceOffset += data buffer's absolute address.
		c.assembler.CompileMemoryToRegister(amd64.ADDQ,
			instanceAddr, 0, sourceOffset.register)

		// For tables, we move 8 bytes at once.
		memToReg = amd64.MOVQ
		regToMem = memToReg
		scale = 8
	} else {
		// destinationOffset += memory buffer's absolute address.
		c.assembler.CompileRegisterToRegister(amd64.ADDQ, amd64ReservedRegisterForMemory, destinationOffset.register)

		// sourceOffset += data buffer's absolute address.
		c.assembler.CompileMemoryToRegister(amd64.ADDQ, instanceAddr, 0, sourceOffset.register)

		// Move one byte at once.
		memToReg = amd64.MOVBQZX
		regToMem = amd64.MOVB
		scale = 1
	}

	// Negate the counter.
	c.assembler.CompileNoneToRegister(amd64.NEGQ, copySize.register)

	beginCopyLoop := c.assembler.CompileStandAlone(amd64.NOP)

	c.assembler.CompileMemoryWithIndexToRegister(memToReg,
		sourceOffset.register, 0, copySize.register, scale,
		tmp)
	// [destinationOffset + (size.register)] = tmp.
	c.assembler.CompileRegisterToMemoryWithIndex(regToMem,
		tmp,
		destinationOffset.register, 0, copySize.register, scale,
	)

	// size += 1
	c.assembler.CompileNoneToRegister(amd64.INCQ, copySize.register)
	c.assembler.CompileJump(amd64.JMI).AssignJumpTarget(beginCopyLoop)

	c.locationStack.markRegisterUnused(copySize.register, sourceOffset.register,
		destinationOffset.register, instanceAddr, tmp)
	c.assembler.SetJumpTargetOnNext(skipJump)
	return nil
}

// compileDataDrop implements compiler.compileDataDrop for the amd64 architecture.
func (c *amd64Compiler) compileDataDrop(o wazeroir.OperationDataDrop) error {
	if err := c.maybeCompileMoveTopConditionalToGeneralPurposeRegister(); err != nil {
		return err
	}

	tmp, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}

	c.compileLoadDataInstanceAddress(o.DataIndex, tmp)

	// Clears the content of DataInstance[o.DataIndex] (== []byte type).
	c.assembler.CompileConstToMemory(amd64.MOVQ, 0, tmp, 0)
	c.assembler.CompileConstToMemory(amd64.MOVQ, 0, tmp, 8)
	c.assembler.CompileConstToMemory(amd64.MOVQ, 0, tmp, 16)
	return nil
}

func (c *amd64Compiler) compileLoadDataInstanceAddress(dataIndex uint32, dst asm.Register) {
	// dst = dataIndex * dataInstanceStructSize.
	c.assembler.CompileConstToRegister(amd64.MOVQ, int64(dataIndex)*dataInstanceStructSize, dst)

	// dst = &moduleInstance.DataInstances[0] + dst
	//     = &moduleInstance.DataInstances[0] + dataIndex*dataInstanceStructSize
	//     = &moduleInstance.DataInstances[dataIndex]
	c.assembler.CompileMemoryToRegister(amd64.ADDQ,
		amd64ReservedRegisterForCallEngine, callEngineModuleContextDataInstancesElement0AddressOffset,
		dst,
	)
}

// compileCopyLoopImpl implements a REP MOVSQ memory copy for the given range with support for both directions.
func (c *amd64Compiler) compileCopyLoopImpl(destinationOffset, sourceOffset, copySize *runtimeValueLocation, backwards bool, bwOffset uint8) {
	// skip if nothing to copy
	c.assembler.CompileRegisterToConst(amd64.CMPQ, copySize.register, 0)
	emptyEightGroupsJump := c.assembler.CompileJump(amd64.JEQ)

	// Prepare registers for swaps. There will never be more than 3 XCHGs in total.
	restoreCrossing := c.compilePreventCrossedTargetRegisters(
		[]*runtimeValueLocation{destinationOffset, sourceOffset, copySize},
		[]asm.Register{amd64.RegDI, amd64.RegSI, amd64.RegCX})

	// Prepare registers for REP MOVSQ: copy from rsi to rdi, rcx times.
	c.compileMaybeSwapRegisters(destinationOffset.register, amd64.RegDI)
	c.compileMaybeSwapRegisters(sourceOffset.register, amd64.RegSI)
	c.compileMaybeSwapRegisters(copySize.register, amd64.RegCX)

	// Point on first byte of first quadword to copy.
	if backwards {
		c.assembler.CompileConstToRegister(amd64.ADDQ, -int64(bwOffset), amd64.RegDI)
		c.assembler.CompileConstToRegister(amd64.ADDQ, -int64(bwOffset), amd64.RegSI)
		// Set REP prefix direction backwards.
		c.assembler.CompileStandAlone(amd64.STD)
	}

	c.assembler.CompileStandAlone(amd64.REPMOVSQ)

	if backwards {
		// Reset direction.
		c.assembler.CompileStandAlone(amd64.CLD)
	}

	// Restore registers.
	c.compileMaybeSwapRegisters(destinationOffset.register, amd64.RegDI)
	c.compileMaybeSwapRegisters(sourceOffset.register, amd64.RegSI)
	c.compileMaybeSwapRegisters(copySize.register, amd64.RegCX)
	restoreCrossing()

	c.assembler.SetJumpTargetOnNext(emptyEightGroupsJump)
	c.assembler.CompileStandAlone(amd64.NOP)
}

// compileMemoryCopyLoopImpl is used for directly copying after bounds/direction check.
func (c *amd64Compiler) compileMemoryCopyLoopImpl(destinationOffset, sourceOffset, copySize *runtimeValueLocation, tmp asm.Register, backwards bool) {
	// Point on first byte to be copied depending on direction.
	if backwards {
		c.assembler.CompileNoneToRegister(amd64.DECQ, sourceOffset.register)
		c.assembler.CompileNoneToRegister(amd64.DECQ, destinationOffset.register)
	} else {
		c.assembler.CompileRegisterToRegister(amd64.SUBQ, copySize.register, sourceOffset.register)
		c.assembler.CompileRegisterToRegister(amd64.SUBQ, copySize.register, destinationOffset.register)
	}

	// destinationOffset += memory buffer's absolute address.
	c.assembler.CompileRegisterToRegister(amd64.ADDQ, amd64ReservedRegisterForMemory, destinationOffset.register)
	// sourceOffset += memory buffer's absolute address.
	c.assembler.CompileRegisterToRegister(amd64.ADDQ, amd64ReservedRegisterForMemory, sourceOffset.register)

	// Copy copySize % 8 bytes in loop to allow copying in 8 byte groups afterward.
	beginLoop := c.assembler.CompileStandAlone(amd64.NOP)

	// Check copySize % 8 == 0.
	c.assembler.CompileConstToRegister(amd64.TESTQ, 7, copySize.register)
	breakLoop := c.assembler.CompileJump(amd64.JEQ)

	c.assembler.CompileMemoryToRegister(amd64.MOVBQZX, sourceOffset.register, 0, tmp)
	c.assembler.CompileRegisterToMemory(amd64.MOVB, tmp, destinationOffset.register, 0)

	if backwards {
		c.assembler.CompileNoneToRegister(amd64.DECQ, sourceOffset.register)
		c.assembler.CompileNoneToRegister(amd64.DECQ, destinationOffset.register)
	} else {
		c.assembler.CompileNoneToRegister(amd64.INCQ, sourceOffset.register)
		c.assembler.CompileNoneToRegister(amd64.INCQ, destinationOffset.register)
	}

	c.assembler.CompileNoneToRegister(amd64.DECQ, copySize.register)
	c.assembler.CompileJump(amd64.JMP).AssignJumpTarget(beginLoop)
	c.assembler.SetJumpTargetOnNext(breakLoop)

	// compileCopyLoopImpl counts in groups of 8 bytes, so we have to divide the copySize by 8.
	c.assembler.CompileConstToRegister(amd64.SHRQ, 3, copySize.register)

	c.compileCopyLoopImpl(destinationOffset, sourceOffset, copySize, backwards, 7)
}

// compileMemoryCopy implements compiler.compileMemoryCopy for the amd64 architecture.
//
// This uses efficient `REP MOVSQ` instructions to copy in quadword (8 bytes) batches. The remaining bytes
// are copied with a simple `MOV` loop. It uses backward copying for overlapped segments.
func (c *amd64Compiler) compileMemoryCopy() error {
	copySize := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(copySize); err != nil {
		return err
	}

	sourceOffset := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(sourceOffset); err != nil {
		return err
	}

	destinationOffset := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(destinationOffset); err != nil {
		return err
	}

	tmp, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}
	c.locationStack.markRegisterUsed(tmp)

	// sourceOffset += size.
	c.assembler.CompileRegisterToRegister(amd64.ADDQ, copySize.register, sourceOffset.register)
	// destinationOffset += size.
	c.assembler.CompileRegisterToRegister(amd64.ADDQ, copySize.register, destinationOffset.register)

	// Check source bounds and if exceeds the length, exit with out of bounds error.
	c.assembler.CompileMemoryToRegister(amd64.CMPQ,
		amd64ReservedRegisterForCallEngine, callEngineModuleContextMemorySliceLenOffset, sourceOffset.register)
	sourceBoundOKJump := c.assembler.CompileJump(amd64.JCC)
	c.compileExitFromNativeCode(nativeCallStatusCodeMemoryOutOfBounds)
	c.assembler.SetJumpTargetOnNext(sourceBoundOKJump)

	// Check destination bounds and if exceeds the length, exit with out of bounds error.
	c.assembler.CompileMemoryToRegister(amd64.CMPQ,
		amd64ReservedRegisterForCallEngine, callEngineModuleContextMemorySliceLenOffset, destinationOffset.register)
	destinationBoundOKJump := c.assembler.CompileJump(amd64.JCC)
	c.compileExitFromNativeCode(nativeCallStatusCodeMemoryOutOfBounds)
	c.assembler.SetJumpTargetOnNext(destinationBoundOKJump)

	// Skip zero size.
	c.assembler.CompileRegisterToConst(amd64.CMPQ, copySize.register, 0)
	skipJump := c.assembler.CompileJump(amd64.JEQ)

	// If dest < source, we can copy forwards
	c.assembler.CompileRegisterToRegister(amd64.CMPQ, destinationOffset.register, sourceOffset.register)
	destLowerThanSourceJump := c.assembler.CompileJump(amd64.JLS)

	// If source + size < dest, we can copy forwards
	c.assembler.CompileRegisterToRegister(amd64.MOVQ, destinationOffset.register, tmp)
	c.assembler.CompileRegisterToRegister(amd64.SUBQ, copySize.register, tmp)
	c.assembler.CompileRegisterToRegister(amd64.CMPQ, sourceOffset.register, tmp)
	sourceBoundLowerThanDestJump := c.assembler.CompileJump(amd64.JLS)

	// Copy backwards.
	c.compileMemoryCopyLoopImpl(destinationOffset, sourceOffset, copySize, tmp, true)
	endJump := c.assembler.CompileJump(amd64.JMP)

	// Copy forwards.
	c.assembler.SetJumpTargetOnNext(destLowerThanSourceJump, sourceBoundLowerThanDestJump)
	c.compileMemoryCopyLoopImpl(destinationOffset, sourceOffset, copySize, tmp, false)

	c.locationStack.markRegisterUnused(copySize.register, sourceOffset.register,
		destinationOffset.register, tmp)
	c.assembler.SetJumpTargetOnNext(skipJump, endJump)

	return nil
}

// compileFillLoopImpl implements a REP STOSQ fill loop.
func (c *amd64Compiler) compileFillLoopImpl(destinationOffset, value, fillSize *runtimeValueLocation, tmp asm.Register, replicateByte bool) {
	// Skip if nothing to fill.
	c.assembler.CompileRegisterToConst(amd64.CMPQ, fillSize.register, 0)
	emptyEightGroupsJump := c.assembler.CompileJump(amd64.JEQ)

	if replicateByte {
		// Truncate value.register to a single byte
		c.assembler.CompileConstToRegister(amd64.ANDQ, 0xff, value.register)
		// Replicate single byte onto full 8-byte register.
		c.assembler.CompileConstToRegister(amd64.MOVQ, 0x0101010101010101, tmp)
		c.assembler.CompileRegisterToRegister(amd64.IMULQ, tmp, value.register)
	}

	// Prepare registers for swaps. There will never be more than 3 XCHGs in total.
	restoreCrossing := c.compilePreventCrossedTargetRegisters(
		[]*runtimeValueLocation{destinationOffset, value, fillSize},
		[]asm.Register{amd64.RegDI, amd64.RegAX, amd64.RegCX})

	// Prepare registers for REP STOSQ: fill at [rdi] with rax, rcx times.
	c.compileMaybeSwapRegisters(destinationOffset.register, amd64.RegDI)
	c.compileMaybeSwapRegisters(value.register, amd64.RegAX)
	c.compileMaybeSwapRegisters(fillSize.register, amd64.RegCX)

	c.assembler.CompileStandAlone(amd64.REPSTOSQ)

	// Restore registers.
	c.compileMaybeSwapRegisters(destinationOffset.register, amd64.RegDI)
	c.compileMaybeSwapRegisters(value.register, amd64.RegAX)
	c.compileMaybeSwapRegisters(fillSize.register, amd64.RegCX)
	restoreCrossing()

	c.assembler.SetJumpTargetOnNext(emptyEightGroupsJump)
}

// compileMemoryFill implements compiler.compileMemoryFill for the amd64 architecture.
//
// This function uses efficient `REP STOSQ` instructions to copy in quadword (8 bytes) batches
// if the size if above 15 bytes. For smaller sizes, a simple MOVB copy loop is the best
// option.
//
// TODO: the compiled code in this function should be reused and compile at once as
// the code is independent of any module.
func (c *amd64Compiler) compileFillImpl(isTable bool, tableIndex uint32) error {
	copySize := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(copySize); err != nil {
		return err
	}

	value := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(value); err != nil {
		return err
	}

	destinationOffset := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(destinationOffset); err != nil {
		return err
	}

	tmp, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}
	c.locationStack.markRegisterUsed(tmp)

	// destinationOffset += size.
	c.assembler.CompileRegisterToRegister(amd64.ADDQ, copySize.register, destinationOffset.register)

	// Check destination bounds and if exceeds the length, exit with out of bounds error.
	if isTable {
		// tmp = &tables[0]
		c.assembler.CompileMemoryToRegister(amd64.MOVQ,
			amd64ReservedRegisterForCallEngine, callEngineModuleContextTablesElement0AddressOffset,
			tmp)

		// tmp = [tmp + TableIndex*8]
		//     = [&tables[0] + TableIndex*sizeOf(*tableInstance)]
		//     = [&tables[TableIndex]] = tables[TableIndex].
		c.assembler.CompileMemoryToRegister(amd64.MOVQ, tmp, int64(tableIndex)*8, tmp)

		c.assembler.CompileMemoryToRegister(amd64.CMPQ,
			tmp, tableInstanceTableLenOffset,
			destinationOffset.register)
	} else {
		c.assembler.CompileMemoryToRegister(amd64.CMPQ,
			amd64ReservedRegisterForCallEngine, callEngineModuleContextMemorySliceLenOffset,
			destinationOffset.register)
	}
	destinationBoundOKJump := c.assembler.CompileJump(amd64.JCC)
	if isTable {
		c.compileExitFromNativeCode(nativeCallStatusCodeInvalidTableAccess)
	} else {
		c.compileExitFromNativeCode(nativeCallStatusCodeMemoryOutOfBounds)
	}
	c.assembler.SetJumpTargetOnNext(destinationBoundOKJump)

	// Otherwise, ready to copy the value from source to destination.
	//
	// If the copy size equal zero, we skip the entire instructions below.
	c.assembler.CompileRegisterToConst(amd64.CMPQ, copySize.register, 0)
	skipJump := c.assembler.CompileJump(amd64.JEQ)

	// destinationOffset -= size.
	c.assembler.CompileRegisterToRegister(amd64.SUBQ, copySize.register, destinationOffset.register)

	if isTable {
		// Each element is of type uintptr; 2^3 = 1 << pointerSizeLog2.
		c.assembler.CompileConstToRegister(amd64.SHLQ, pointerSizeLog2, destinationOffset.register)
		// destinationOffset += table buffer's absolute address.
		c.assembler.CompileMemoryToRegister(amd64.ADDQ, tmp, tableInstanceTableOffset, destinationOffset.register)

	} else {
		// destinationOffset += memory buffer's absolute address.
		c.assembler.CompileRegisterToRegister(amd64.ADDQ, amd64ReservedRegisterForMemory, destinationOffset.register)

		// Copy first %15 bytes with simple MOVB instruction.
		beginCopyLoop := c.assembler.CompileStandAlone(amd64.NOP)
		c.assembler.CompileConstToRegister(amd64.TESTQ, 15, copySize.register)
		breakLoop := c.assembler.CompileJump(amd64.JEQ)

		c.assembler.CompileRegisterToMemory(amd64.MOVB, value.register, destinationOffset.register, 0)

		c.assembler.CompileNoneToRegister(amd64.INCQ, destinationOffset.register)
		c.assembler.CompileNoneToRegister(amd64.DECQ, copySize.register)
		c.assembler.CompileJump(amd64.JMP).AssignJumpTarget(beginCopyLoop)

		c.assembler.SetJumpTargetOnNext(breakLoop)
		// compileFillLoopImpl counts in groups of 8 bytes, so we have to divide the copySize by 8.
		c.assembler.CompileConstToRegister(amd64.SHRQ, 3, copySize.register)
	}

	c.compileFillLoopImpl(destinationOffset, value, copySize, tmp, !isTable)

	c.locationStack.markRegisterUnused(copySize.register, value.register,
		destinationOffset.register, tmp)
	c.assembler.SetJumpTargetOnNext(skipJump)
	return nil
}

// compileMemoryFill implements compiler.compileMemoryFill for the amd64 architecture.
//
// TODO: the compiled code in this function should be reused and compile at once as
// the code is independent of any module.
func (c *amd64Compiler) compileMemoryFill() error {
	return c.compileFillImpl(false, 0)
}

// compileTableInit implements compiler.compileTableInit for the amd64 architecture.
func (c *amd64Compiler) compileTableInit(o wazeroir.OperationTableInit) error {
	return c.compileInitImpl(true, o.ElemIndex, o.TableIndex)
}

// compileTableCopyLoopImpl is used for directly copying after bounds/direction check.
func (c *amd64Compiler) compileTableCopyLoopImpl(o wazeroir.OperationTableCopy, destinationOffset, sourceOffset, copySize *runtimeValueLocation, tmp asm.Register, backwards bool) {
	// Point on first byte to be copied.
	if !backwards {
		c.assembler.CompileRegisterToRegister(amd64.SUBQ, copySize.register, sourceOffset.register)
		c.assembler.CompileRegisterToRegister(amd64.SUBQ, copySize.register, destinationOffset.register)
	}

	// Each element is of type uintptr; 2^3 = 1 << pointerSizeLog2.
	c.assembler.CompileConstToRegister(amd64.SHLQ, pointerSizeLog2, sourceOffset.register)
	c.assembler.CompileConstToRegister(amd64.SHLQ, pointerSizeLog2, destinationOffset.register)
	// destinationOffset += table buffer's absolute address.
	c.assembler.CompileMemoryToRegister(amd64.MOVQ, amd64ReservedRegisterForCallEngine, callEngineModuleContextTablesElement0AddressOffset, tmp)
	c.assembler.CompileMemoryToRegister(amd64.MOVQ, tmp, int64(o.DstTableIndex*8), tmp)
	c.assembler.CompileMemoryToRegister(amd64.ADDQ, tmp, tableInstanceTableOffset, destinationOffset.register)
	// sourceOffset += table buffer's absolute address.
	c.assembler.CompileMemoryToRegister(amd64.MOVQ, amd64ReservedRegisterForCallEngine, callEngineModuleContextTablesElement0AddressOffset, tmp)
	c.assembler.CompileMemoryToRegister(amd64.MOVQ, tmp, int64(o.SrcTableIndex*8), tmp)
	c.assembler.CompileMemoryToRegister(amd64.ADDQ, tmp, tableInstanceTableOffset, sourceOffset.register)

	c.compileCopyLoopImpl(destinationOffset, sourceOffset, copySize, backwards, 8)
}

// compileTableCopy implements compiler.compileTableCopy for the amd64 architecture.
//
// It uses efficient `REP MOVSB` instructions for optimized copying. It uses backward copying for
// overlapped segments.
func (c *amd64Compiler) compileTableCopy(o wazeroir.OperationTableCopy) error {
	copySize := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(copySize); err != nil {
		return err
	}

	sourceOffset := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(sourceOffset); err != nil {
		return err
	}

	destinationOffset := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(destinationOffset); err != nil {
		return err
	}

	tmp, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}

	// sourceOffset += size.
	c.assembler.CompileRegisterToRegister(amd64.ADDQ, copySize.register, sourceOffset.register)
	// destinationOffset += size.
	c.assembler.CompileRegisterToRegister(amd64.ADDQ, copySize.register, destinationOffset.register)

	// Check source bounds and if exceeds the length, exit with out of bounds error.
	c.assembler.CompileMemoryToRegister(amd64.MOVQ, amd64ReservedRegisterForCallEngine, callEngineModuleContextTablesElement0AddressOffset, tmp)
	c.assembler.CompileMemoryToRegister(amd64.MOVQ, tmp, int64(o.SrcTableIndex*8), tmp)
	c.assembler.CompileMemoryToRegister(amd64.CMPQ, tmp, tableInstanceTableLenOffset, sourceOffset.register)
	sourceBoundOKJump := c.assembler.CompileJump(amd64.JCC)
	c.compileExitFromNativeCode(nativeCallStatusCodeInvalidTableAccess)
	c.assembler.SetJumpTargetOnNext(sourceBoundOKJump)

	// Check destination bounds and if exceeds the length, exit with out of bounds error.
	c.assembler.CompileMemoryToRegister(amd64.MOVQ, amd64ReservedRegisterForCallEngine, callEngineModuleContextTablesElement0AddressOffset, tmp)
	c.assembler.CompileMemoryToRegister(amd64.MOVQ, tmp, int64(o.DstTableIndex*8), tmp)
	c.assembler.CompileMemoryToRegister(amd64.CMPQ, tmp, tableInstanceTableLenOffset, destinationOffset.register)
	destinationBoundOKJump := c.assembler.CompileJump(amd64.JCC)
	c.compileExitFromNativeCode(nativeCallStatusCodeInvalidTableAccess)
	c.assembler.SetJumpTargetOnNext(destinationBoundOKJump)

	// Skip zero size.
	c.assembler.CompileRegisterToConst(amd64.CMPQ, copySize.register, 0)
	skipJump := c.assembler.CompileJump(amd64.JEQ)

	// If dest < source, we can copy forwards.
	c.assembler.CompileRegisterToRegister(amd64.CMPQ, destinationOffset.register, sourceOffset.register)
	destLowerThanSourceJump := c.assembler.CompileJump(amd64.JLS)

	// If source + size < dest, we can copy forwards.
	c.assembler.CompileRegisterToRegister(amd64.MOVQ, destinationOffset.register, tmp)
	c.assembler.CompileRegisterToRegister(amd64.SUBQ, copySize.register, tmp)
	c.assembler.CompileRegisterToRegister(amd64.CMPQ, sourceOffset.register, tmp)
	sourceBoundLowerThanDestJump := c.assembler.CompileJump(amd64.JLS)

	// Copy backwards.
	c.compileTableCopyLoopImpl(o, destinationOffset, sourceOffset, copySize, tmp, true)
	endJump := c.assembler.CompileJump(amd64.JMP)

	// Copy forwards.
	c.assembler.SetJumpTargetOnNext(destLowerThanSourceJump, sourceBoundLowerThanDestJump)
	c.compileTableCopyLoopImpl(o, destinationOffset, sourceOffset, copySize, tmp, false)

	c.locationStack.markRegisterUnused(copySize.register, sourceOffset.register,
		destinationOffset.register, tmp)
	c.assembler.SetJumpTargetOnNext(skipJump, endJump)
	return nil
}

// compileElemDrop implements compiler.compileElemDrop for the amd64 architecture.
func (c *amd64Compiler) compileElemDrop(o wazeroir.OperationElemDrop) error {
	if err := c.maybeCompileMoveTopConditionalToGeneralPurposeRegister(); err != nil {
		return err
	}

	tmp, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}

	c.compileLoadElemInstanceAddress(o.ElemIndex, tmp)

	// Clears the content of ElementInstances[o.ElemIndex].References (== []uintptr{} type).
	c.assembler.CompileConstToMemory(amd64.MOVQ, 0, tmp, 0)
	c.assembler.CompileConstToMemory(amd64.MOVQ, 0, tmp, 8)
	c.assembler.CompileConstToMemory(amd64.MOVQ, 0, tmp, 16)
	return nil
}

func (c *amd64Compiler) compileLoadElemInstanceAddress(elemIndex uint32, dst asm.Register) {
	// dst = elemIndex * elementInstanceStructSize
	c.assembler.CompileConstToRegister(amd64.MOVQ, int64(elemIndex)*elementInstanceStructSize, dst)

	// dst = &moduleInstance.ElementInstances[0] + dst
	//     = &moduleInstance.ElementInstances[0] + elemIndex*elementInstanceStructSize
	//     = &moduleInstance.ElementInstances[elemIndex]
	c.assembler.CompileMemoryToRegister(amd64.ADDQ,
		amd64ReservedRegisterForCallEngine, callEngineModuleContextElementInstancesElement0AddressOffset,
		dst,
	)
}

// compileTableGet implements compiler.compileTableGet for the amd64 architecture.
func (c *amd64Compiler) compileTableGet(o wazeroir.OperationTableGet) error {
	ref, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}

	c.locationStack.markRegisterUsed(ref)

	offset := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(offset); err != nil {
		return err
	}

	// ref = &tables[0]
	c.assembler.CompileMemoryToRegister(amd64.MOVQ,
		amd64ReservedRegisterForCallEngine, callEngineModuleContextTablesElement0AddressOffset,
		ref)

	// ref = [ref + TableIndex*8]
	//     = [&tables[0] + TableIndex*sizeOf(*tableInstance)]
	//     = [&tables[TableIndex]] = tables[TableIndex].
	c.assembler.CompileMemoryToRegister(amd64.MOVQ, ref, int64(o.TableIndex)*8, ref)

	// Out of bounds check.
	c.assembler.CompileMemoryToRegister(amd64.CMPQ, ref, tableInstanceTableLenOffset, offset.register)
	boundOKJmp := c.assembler.CompileJump(amd64.JHI)
	c.compileExitFromNativeCode(nativeCallStatusCodeInvalidTableAccess)
	c.assembler.SetJumpTargetOnNext(boundOKJmp)

	// ref = [&tables[TableIndex] + tableInstanceTableOffset] = &tables[TableIndex].References[0]
	c.assembler.CompileMemoryToRegister(amd64.MOVQ, ref, tableInstanceTableOffset, ref)

	// ref = [ref + 0 + offset.register * 8]
	//     = [&tables[TableIndex].References[0] + sizeOf(uintptr) * offset]
	//     = [&tables[TableIndex].References[offset]]
	//     = tables[TableIndex].References[offset]
	c.assembler.CompileMemoryWithIndexToRegister(amd64.MOVQ, ref,
		0, offset.register, 8, ref,
	)

	c.locationStack.markRegisterUnused(offset.register)
	c.pushRuntimeValueLocationOnRegister(ref, runtimeValueTypeI64) // table elements are opaque 64-bit at runtime.
	return nil
}

// compileTableSet implements compiler.compileTableSet for the amd64 architecture.
func (c *amd64Compiler) compileTableSet(o wazeroir.OperationTableSet) error {
	ref := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(ref); err != nil {
		return err
	}

	offset := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(offset); err != nil {
		return err
	}

	tmp, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}

	// tmp = &tables[0]
	c.assembler.CompileMemoryToRegister(amd64.MOVQ,
		amd64ReservedRegisterForCallEngine, callEngineModuleContextTablesElement0AddressOffset,
		tmp)

	// ref = [ref + TableIndex*8]
	//     = [&tables[0] + TableIndex*sizeOf(*tableInstance)]
	//     = [&tables[TableIndex]] = tables[TableIndex].
	c.assembler.CompileMemoryToRegister(amd64.MOVQ, tmp, int64(o.TableIndex)*8, tmp)

	// Out of bounds check.
	c.assembler.CompileMemoryToRegister(amd64.CMPQ, tmp, tableInstanceTableLenOffset, offset.register)
	boundOKJmp := c.assembler.CompileJump(amd64.JHI)
	c.compileExitFromNativeCode(nativeCallStatusCodeInvalidTableAccess)
	c.assembler.SetJumpTargetOnNext(boundOKJmp)

	// tmp = [&tables[TableIndex] + tableInstanceTableOffset] = &tables[TableIndex].References[0]
	c.assembler.CompileMemoryToRegister(amd64.MOVQ, tmp, tableInstanceTableOffset, tmp)

	// [tmp + 0 + offset.register * 8] = ref
	// [&tables[TableIndex].References[0] + sizeOf(uintptr) * offset] = ref
	// [&tables[TableIndex].References[offset]] = ref
	// tables[TableIndex].References[offset] = ref
	c.assembler.CompileRegisterToMemoryWithIndex(amd64.MOVQ,
		ref.register,
		tmp, 0, offset.register, 8)

	c.locationStack.markRegisterUnused(offset.register, ref.register)
	return nil
}

// compileTableGrow implements compiler.compileTableGrow for the amd64 architecture.
func (c *amd64Compiler) compileTableGrow(o wazeroir.OperationTableGrow) error {
	if err := c.maybeCompileMoveTopConditionalToGeneralPurposeRegister(); err != nil {
		return err
	}

	// Pushes the table index.
	if err := c.compileConstI32(wazeroir.OperationConstI32{Value: o.TableIndex}); err != nil {
		return err
	}

	// Table grow cannot be done in assembly just like memory grow as it involves with allocation in Go.
	// Therefore, call out to the built function for this purpose.
	if err := c.compileCallBuiltinFunction(builtinFunctionIndexTableGrow); err != nil {
		return err
	}

	// TableGrow consumes three values (table index, number of items, initial value).
	for i := 0; i < 3; i++ {
		c.locationStack.pop()
	}

	// Then, the previous length was pushed as the result.
	loc := c.locationStack.pushRuntimeValueLocationOnStack()
	loc.valueType = runtimeValueTypeI32

	// After return, we re-initialize reserved registers just like preamble of functions.
	c.compileReservedStackBasePointerInitialization()
	c.compileReservedMemoryPointerInitialization()
	return nil
}

// compileTableSize implements compiler.compileTableSize for the amd64 architecture.
func (c *amd64Compiler) compileTableSize(o wazeroir.OperationTableSize) error {
	if err := c.maybeCompileMoveTopConditionalToGeneralPurposeRegister(); err != nil {
		return err
	}

	result, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}

	// result = &tables[0]
	c.assembler.CompileMemoryToRegister(amd64.MOVQ,
		amd64ReservedRegisterForCallEngine, callEngineModuleContextTablesElement0AddressOffset,
		result)

	// result = [result + TableIndex*8]
	//        = [&tables[0] + TableIndex*sizeOf(*tableInstance)]
	//        = [&tables[TableIndex]] = tables[TableIndex].
	c.assembler.CompileMemoryToRegister(amd64.MOVQ, result, int64(o.TableIndex)*8, result)

	// result = [result + tableInstanceTableLenOffset]
	//        = [tables[TableIndex] + tableInstanceTableLenOffset]
	//        = len(tables[TableIndex])
	c.assembler.CompileMemoryToRegister(amd64.MOVQ, result, tableInstanceTableLenOffset, result)

	c.pushRuntimeValueLocationOnRegister(result, runtimeValueTypeI32)
	return nil
}

// compileTableFill implements compiler.compileTableFill for the amd64 architecture.
func (c *amd64Compiler) compileTableFill(o wazeroir.OperationTableFill) error {
	return c.compileFillImpl(true, o.TableIndex)
}

// compileRefFunc implements compiler.compileRefFunc for the amd64 architecture.
func (c *amd64Compiler) compileRefFunc(o wazeroir.OperationRefFunc) error {
	if err := c.maybeCompileMoveTopConditionalToGeneralPurposeRegister(); err != nil {
		return err
	}

	ref, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}

	c.assembler.CompileConstToRegister(amd64.MOVQ, int64(o.FunctionIndex)*functionSize, ref)

	// ref = [amd64ReservedRegisterForCallEngine + callEngineModuleContextFunctionsElement0AddressOffset + int64(o.FunctionIndex)*functionSize]
	//     = &moduleEngine.functions[index]
	c.assembler.CompileMemoryToRegister(
		amd64.ADDQ, amd64ReservedRegisterForCallEngine, callEngineModuleContextFunctionsElement0AddressOffset,
		ref,
	)

	c.pushRuntimeValueLocationOnRegister(ref, runtimeValueTypeI64)
	return nil
}

// compileConstI32 implements compiler.compileConstI32 for the amd64 architecture.
func (c *amd64Compiler) compileConstI32(o wazeroir.OperationConstI32) error {
	if err := c.maybeCompileMoveTopConditionalToGeneralPurposeRegister(); err != nil {
		return err
	}

	reg, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}
	c.pushRuntimeValueLocationOnRegister(reg, runtimeValueTypeI32)
	c.assembler.CompileConstToRegister(amd64.MOVL, int64(o.Value), reg)
	return nil
}

// compileConstI64 implements compiler.compileConstI64 for the amd64 architecture.
func (c *amd64Compiler) compileConstI64(o wazeroir.OperationConstI64) error {
	if err := c.maybeCompileMoveTopConditionalToGeneralPurposeRegister(); err != nil {
		return err
	}

	reg, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}
	c.pushRuntimeValueLocationOnRegister(reg, runtimeValueTypeI64)

	c.assembler.CompileConstToRegister(amd64.MOVQ, int64(o.Value), reg)
	return nil
}

// compileConstF32 implements compiler.compileConstF32 for the amd64 architecture.
func (c *amd64Compiler) compileConstF32(o wazeroir.OperationConstF32) error {
	if err := c.maybeCompileMoveTopConditionalToGeneralPurposeRegister(); err != nil {
		return err
	}

	reg, err := c.allocateRegister(registerTypeVector)
	if err != nil {
		return err
	}
	c.pushRuntimeValueLocationOnRegister(reg, runtimeValueTypeF32)

	// We cannot directly load the value from memory to float regs,
	// so we move it to int reg temporarily.
	tmpReg, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}

	c.assembler.CompileConstToRegister(amd64.MOVL, int64(math.Float32bits(o.Value)), tmpReg)
	c.assembler.CompileRegisterToRegister(amd64.MOVL, tmpReg, reg)
	return nil
}

// compileConstF64 implements compiler.compileConstF64 for the amd64 architecture.
func (c *amd64Compiler) compileConstF64(o wazeroir.OperationConstF64) error {
	if err := c.maybeCompileMoveTopConditionalToGeneralPurposeRegister(); err != nil {
		return err
	}

	reg, err := c.allocateRegister(registerTypeVector)
	if err != nil {
		return err
	}
	c.pushRuntimeValueLocationOnRegister(reg, runtimeValueTypeF64)

	// We cannot directly load the value from memory to float regs,
	// so we move it to int reg temporarily.
	tmpReg, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}

	c.assembler.CompileConstToRegister(amd64.MOVQ, int64(math.Float64bits(o.Value)), tmpReg)
	c.assembler.CompileRegisterToRegister(amd64.MOVQ, tmpReg, reg)
	return nil
}

// compileLoadValueOnStackToRegister implements compiler.compileLoadValueOnStackToRegister for amd64.
func (c *amd64Compiler) compileLoadValueOnStackToRegister(loc *runtimeValueLocation) {
	var inst asm.Instruction
	switch loc.valueType {
	case runtimeValueTypeV128Lo:
		inst = amd64.MOVDQU
	case runtimeValueTypeV128Hi:
		panic("BUG: V128Hi must be be loaded to a register along with V128Lo")
	case runtimeValueTypeI32, runtimeValueTypeF32:
		inst = amd64.MOVL
	case runtimeValueTypeI64, runtimeValueTypeF64:
		inst = amd64.MOVQ
	default:
		panic("BUG: unknown runtime value type")
	}

	// Copy the value from the stack.
	c.assembler.CompileMemoryToRegister(inst,
		// Note: stack pointers are ensured not to exceed 2^27 so this offset never exceeds 32-bit range.
		amd64ReservedRegisterForStackBasePointerAddress, int64(loc.stackPointer)*8,
		loc.register)

	if loc.valueType == runtimeValueTypeV128Lo {
		// Higher 64-bits are loaded as well ^^.
		hi := &c.locationStack.stack[loc.stackPointer+1]
		hi.setRegister(loc.register)
	}
}

// maybeCompileMoveTopConditionalToGeneralPurposeRegister moves the top value on the stack
// if the value is located on a conditional register.
//
// This is usually called at the beginning of methods on compiler interface where we possibly
// compile instructions without saving the conditional register value.
// The compileXXX functions without calling this function is saving the conditional
// value to the stack or register by invoking compileEnsureOnRegister for the top.
func (c *amd64Compiler) maybeCompileMoveTopConditionalToGeneralPurposeRegister() (err error) {
	if c.locationStack.sp > 0 {
		if loc := c.locationStack.peek(); loc.onConditionalRegister() {
			if err = c.compileLoadConditionalRegisterToGeneralPurposeRegister(loc); err != nil {
				return err
			}
		}
	}
	return
}

// loadConditionalRegisterToGeneralPurposeRegister saves the conditional register value
// to a general purpose register.
func (c *amd64Compiler) compileLoadConditionalRegisterToGeneralPurposeRegister(loc *runtimeValueLocation) error {
	reg, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}
	c.compileMoveConditionalToGeneralPurposeRegister(loc, reg)
	return nil
}

func (c *amd64Compiler) compileMoveConditionalToGeneralPurposeRegister(loc *runtimeValueLocation, reg asm.Register) {
	// Set the flag bit to the destination. See
	// - https://c9x.me/x86/html/file_module_x86_id_288.html
	// - https://github.com/golang/go/blob/master/src/cmd/internal/obj/x86/asm6.go#L1453-L1468
	// to translate conditionalRegisterState* to amd64.SET*
	var inst asm.Instruction
	switch loc.conditionalRegister {
	case amd64.ConditionalRegisterStateE:
		inst = amd64.SETEQ
	case amd64.ConditionalRegisterStateNE:
		inst = amd64.SETNE
	case amd64.ConditionalRegisterStateS:
		inst = amd64.SETMI
	case amd64.ConditionalRegisterStateNS:
		inst = amd64.SETPL
	case amd64.ConditionalRegisterStateG:
		inst = amd64.SETGT
	case amd64.ConditionalRegisterStateGE:
		inst = amd64.SETGE
	case amd64.ConditionalRegisterStateL:
		inst = amd64.SETLT
	case amd64.ConditionalRegisterStateLE:
		inst = amd64.SETLE
	case amd64.ConditionalRegisterStateA:
		inst = amd64.SETHI
	case amd64.ConditionalRegisterStateAE:
		inst = amd64.SETCC
	case amd64.ConditionalRegisterStateB:
		inst = amd64.SETCS
	case amd64.ConditionalRegisterStateBE:
		inst = amd64.SETLS
	}

	c.assembler.CompileNoneToRegister(inst, reg)

	// Then we reset the unnecessary bit.
	c.assembler.CompileConstToRegister(amd64.ANDQ, 0x1, reg)

	// Mark it uses the register.
	loc.setRegister(reg)
	c.locationStack.markRegisterUsed(reg)
}

// allocateRegister implements compiler.allocateRegister for amd64.
func (c *amd64Compiler) allocateRegister(t registerType) (reg asm.Register, err error) {
	var ok bool
	// Try to get the unused register.
	reg, ok = c.locationStack.takeFreeRegister(t)
	if ok {
		return
	}

	// If not found, we have to steal the register.
	stealTarget, ok := c.locationStack.takeStealTargetFromUsedRegister(t)
	if !ok {
		err = fmt.Errorf("cannot steal register")
		return
	}

	// Release the steal target register value onto stack location.
	reg = stealTarget.register
	c.compileReleaseRegisterToStack(stealTarget)
	return
}

// callFunction adds instructions to call a function whose address equals either addr parameter or the value on indexReg.
//
// Note: this is the counterpart for returnFunction, and see the comments there as well
// to understand how the function calls are achieved.
func (c *amd64Compiler) compileCallFunctionImpl(functionAddressRegister asm.Register, functype *wasm.FunctionType) error {
	// Release all the registers as our calling convention requires the caller-save.
	if err := c.compileReleaseAllRegistersToStack(); err != nil {
		return err
	}

	c.locationStack.markRegisterUsed(functionAddressRegister)

	// Obtain a temporary register to be used in the followings.
	tmpRegister, found := c.locationStack.takeFreeRegister(registerTypeGeneralPurpose)
	if !found {
		// This in theory never happen as all the registers must be free except codeAddressRegister.
		return fmt.Errorf("could not find enough free registers")
	}

	// The stack should look like:
	//
	//               reserved slots for results (if len(results) > len(args))
	//                      |     |
	//    ,arg0, ..., argN, ..., _, .returnAddress, .returnStackBasePointerInBytes, .function, ....
	//      |                       |                                                        |
	//      |             callFrame{^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^}
	//      |
	// nextStackBasePointerOffset
	//
	// where callFrame is used to return to this currently executed function.

	nextStackBasePointerOffset := int64(c.locationStack.sp) - int64(functype.ParamNumInUint64)

	callFrameReturnAddressLoc, callFrameStackBasePointerInBytesLoc, callFrameFunctionLoc := c.locationStack.pushCallFrame(functype)

	// Save the current stack base pointer at callFrameStackBasePointerInBytesLoc.
	c.assembler.CompileMemoryToRegister(amd64.MOVQ,
		amd64ReservedRegisterForCallEngine, callEngineStackContextStackBasePointerInBytesOffset,
		tmpRegister)
	callFrameStackBasePointerInBytesLoc.setRegister(tmpRegister)
	c.compileReleaseRegisterToStack(callFrameStackBasePointerInBytesLoc)

	// Set callEngine.stackContext.stackBasePointer for the next function.
	c.assembler.CompileConstToRegister(amd64.ADDQ, nextStackBasePointerOffset<<3, tmpRegister)

	// Write the calculated value to callEngine.stackContext.stackBasePointer.
	c.assembler.CompileRegisterToMemory(amd64.MOVQ, tmpRegister,
		amd64ReservedRegisterForCallEngine, callEngineStackContextStackBasePointerInBytesOffset)

	// Save the currently executed *function (placed at callEngine.moduleContext.fn) into callFrameFunctionLoc.
	c.assembler.CompileMemoryToRegister(amd64.MOVQ,
		amd64ReservedRegisterForCallEngine, callEngineModuleContextFnOffset,
		tmpRegister)
	callFrameFunctionLoc.setRegister(tmpRegister)
	c.compileReleaseRegisterToStack(callFrameFunctionLoc)

	// Set callEngine.moduleContext.fn to the next *function.
	c.assembler.CompileRegisterToMemory(amd64.MOVQ, functionAddressRegister,
		amd64ReservedRegisterForCallEngine, callEngineModuleContextFnOffset)

	// Write the return address into callFrameReturnAddressLoc.
	c.assembler.CompileReadInstructionAddress(tmpRegister, amd64.JMP)
	callFrameReturnAddressLoc.setRegister(tmpRegister)
	c.compileReleaseRegisterToStack(callFrameReturnAddressLoc)

	if amd64CallingConventionDestinationFunctionModuleInstanceAddressRegister == functionAddressRegister {
		// This case we must move the value on targetFunctionAddressRegister to another register, otherwise
		// the address (jump target below) will be modified and result in segfault.
		// See #526.
		c.assembler.CompileRegisterToRegister(amd64.MOVQ, functionAddressRegister, tmpRegister)
		functionAddressRegister = tmpRegister
	}

	// Also, we have to put the target function's *wasm.ModuleInstance into amd64CallingConventionDestinationFunctionModuleInstanceAddressRegister.
	c.assembler.CompileMemoryToRegister(amd64.MOVQ, functionAddressRegister, functionModuleInstanceAddressOffset,
		amd64CallingConventionDestinationFunctionModuleInstanceAddressRegister)

	// And jump into the initial address of the target function.
	c.assembler.CompileJumpToMemory(amd64.JMP, functionAddressRegister, functionCodeInitialAddressOffset)

	// All the registers used are temporary, so we mark them unused.
	c.locationStack.markRegisterUnused(tmpRegister, functionAddressRegister)

	// On the function return, we have to initialize the state.
	if err := c.compileModuleContextInitialization(); err != nil {
		return err
	}

	// Due to the change to callEngine.stackContext.stackBasePointer.
	c.compileReservedStackBasePointerInitialization()

	// Due to the change to callEngine.moduleContext.moduleInstanceAddress as that might result in
	// the memory instance manipulation.
	c.compileReservedMemoryPointerInitialization()

	// We consumed the function parameters, the call frame stack and reserved slots during the call.
	c.locationStack.sp = uint64(nextStackBasePointerOffset)

	// Now the function results are pushed by the call.
	for _, t := range functype.Results {
		loc := c.locationStack.pushRuntimeValueLocationOnStack()
		switch t {
		case wasm.ValueTypeI32:
			loc.valueType = runtimeValueTypeI32
		case wasm.ValueTypeI64, wasm.ValueTypeFuncref, wasm.ValueTypeExternref:
			loc.valueType = runtimeValueTypeI64
		case wasm.ValueTypeF32:
			loc.valueType = runtimeValueTypeF32
		case wasm.ValueTypeF64:
			loc.valueType = runtimeValueTypeF64
		case wasm.ValueTypeV128:
			loc.valueType = runtimeValueTypeV128Lo
			hi := c.locationStack.pushRuntimeValueLocationOnStack()
			hi.valueType = runtimeValueTypeV128Hi
		default:
			panic("BUG: invalid type: " + wasm.ValueTypeName(t))
		}
	}
	return nil
}

// returnFunction adds instructions to return from the current callframe back to the caller's frame.
// If this is the current one is the origin, we return to the callEngine.execWasmFunction with the Returned status.
// Otherwise, we jump into the callers' return address stored in callFrame.returnAddress while setting
// up all the necessary change on the callEngine's state.
//
// Note: this is the counterpart for callFunction, and see the comments there as well
// to understand how the function calls are achieved.
func (c *amd64Compiler) compileReturnFunction() error {
	// Release all the registers as our calling convention requires the caller-save.
	if err := c.compileReleaseAllRegistersToStack(); err != nil {
		return err
	}

	if c.withListener {
		if err := c.compileCallBuiltinFunction(builtinFunctionIndexFunctionListenerAfter); err != nil {
			return err
		}
		// After return, we re-initialize the stack base pointer as that is used to return to the caller below.
		c.compileReservedStackBasePointerInitialization()
	}

	// amd64CallingConventionDestinationFunctionModuleInstanceAddressRegister holds the module instance's address
	// so mark it used so that it won't be used as a free register.
	c.locationStack.markRegisterUsed(amd64CallingConventionDestinationFunctionModuleInstanceAddressRegister)
	defer c.locationStack.markRegisterUnused(amd64CallingConventionDestinationFunctionModuleInstanceAddressRegister)

	// Obtain a temporary register to be used in the following.
	returnAddressRegister, found := c.locationStack.takeFreeRegister(registerTypeGeneralPurpose)
	if !found {
		panic("BUG: all the registers should be free at this point: " + c.locationStack.String())
	}

	returnAddress, callerStackBasePointerInBytes, callerFunction := c.locationStack.getCallFrameLocations(c.ir.Signature)

	// A zero return address means return from the execution.
	c.assembler.CompileMemoryToRegister(amd64.MOVQ,
		amd64ReservedRegisterForStackBasePointerAddress, int64(returnAddress.stackPointer)*8,
		returnAddressRegister,
	)
	c.assembler.CompileRegisterToRegister(amd64.TESTQ, returnAddressRegister, returnAddressRegister)

	jmpIfNotReturn := c.assembler.CompileJump(amd64.JNE)
	c.compileExitFromNativeCode(nativeCallStatusCodeReturned)

	// Otherwise, we return to the caller.
	c.assembler.SetJumpTargetOnNext(jmpIfNotReturn)

	// Alias for readability.
	tmpRegister := amd64CallingConventionDestinationFunctionModuleInstanceAddressRegister

	// First, restore the stackContext.stackBasePointerInBytesOffset from callerStackBasePointerInBytes.
	callerStackBasePointerInBytes.setRegister(tmpRegister)
	c.compileLoadValueOnStackToRegister(callerStackBasePointerInBytes)
	c.assembler.CompileRegisterToMemory(amd64.MOVQ,
		tmpRegister, amd64ReservedRegisterForCallEngine, callEngineStackContextStackBasePointerInBytesOffset)

	// Next, restore moduleContext.fn from callerFunction.
	callerFunction.setRegister(tmpRegister)
	c.compileLoadValueOnStackToRegister(callerFunction)
	c.assembler.CompileRegisterToMemory(amd64.MOVQ,
		tmpRegister, amd64ReservedRegisterForCallEngine, callEngineModuleContextFnOffset)

	// Also, we have to put the target function's *wasm.ModuleInstance into amd64CallingConventionDestinationFunctionModuleInstanceAddressRegister.
	c.assembler.CompileMemoryToRegister(amd64.MOVQ,
		tmpRegister, functionModuleInstanceAddressOffset,
		amd64CallingConventionDestinationFunctionModuleInstanceAddressRegister)

	// Then, jump into the return address!
	c.assembler.CompileJumpToRegister(amd64.JMP, returnAddressRegister)
	return nil
}

func (c *amd64Compiler) compileCallGoHostFunction() error {
	return c.compileCallGoFunction(nativeCallStatusCodeCallGoHostFunction)
}

func (c *amd64Compiler) compileCallBuiltinFunction(index wasm.Index) error {
	// Set the functionAddress to the callEngine.exitContext functionCallAddress.
	c.assembler.CompileConstToMemory(amd64.MOVL, int64(index), amd64ReservedRegisterForCallEngine, callEngineExitContextBuiltinFunctionCallIndexOffset)
	return c.compileCallGoFunction(nativeCallStatusCodeCallBuiltInFunction)
}

func (c *amd64Compiler) compileCallGoFunction(compilerStatus nativeCallStatusCode) error {
	// Release all the registers as our calling convention requires the caller-save.
	if err := c.compileReleaseAllRegistersToStack(); err != nil {
		return err
	}

	c.compileExitFromNativeCode(compilerStatus)
	return nil
}

// compileReleaseAllRegistersToStack add the instructions to release all the LIVE value
// in the value location stack at this point into the stack memory location.
func (c *amd64Compiler) compileReleaseAllRegistersToStack() (err error) {
	for i := uint64(0); i < c.locationStack.sp; i++ {
		if loc := &c.locationStack.stack[i]; loc.onRegister() {
			c.compileReleaseRegisterToStack(loc)
		} else if loc.onConditionalRegister() {
			if err = c.compileLoadConditionalRegisterToGeneralPurposeRegister(loc); err != nil {
				return
			}
			c.compileReleaseRegisterToStack(loc)
		}
	}
	return
}

func (c *amd64Compiler) onValueReleaseRegisterToStack(reg asm.Register) {
	for i := uint64(0); i < c.locationStack.sp; i++ {
		prevValue := &c.locationStack.stack[i]
		if prevValue.register == reg {
			c.compileReleaseRegisterToStack(prevValue)
			break
		}
	}
}

// compileReleaseRegisterToStack implements compiler.compileReleaseRegisterToStack for amd64.
func (c *amd64Compiler) compileReleaseRegisterToStack(loc *runtimeValueLocation) {
	var inst asm.Instruction
	switch loc.valueType {
	case runtimeValueTypeV128Lo:
		inst = amd64.MOVDQU
	case runtimeValueTypeV128Hi:
		panic("BUG: V128Hi must be released to the stack along with V128Lo")
	case runtimeValueTypeI32, runtimeValueTypeF32:
		inst = amd64.MOVL
	case runtimeValueTypeI64, runtimeValueTypeF64:
		inst = amd64.MOVQ
	default:
		panic("BUG: unknown runtime value type")
	}

	c.assembler.CompileRegisterToMemory(inst, loc.register,
		// Note: stack pointers are ensured not to exceed 2^27 so this offset never exceeds 32-bit range.
		amd64ReservedRegisterForStackBasePointerAddress, int64(loc.stackPointer)*8)

	// Mark the register is free.
	c.locationStack.releaseRegister(loc)

	if loc.valueType == runtimeValueTypeV128Lo {
		// Higher 64-bits are released as well ^^.
		hi := &c.locationStack.stack[loc.stackPointer+1]
		c.locationStack.releaseRegister(hi)
	}
}

func (c *amd64Compiler) compileExitFromNativeCode(status nativeCallStatusCode) {
	c.assembler.CompileConstToMemory(amd64.MOVB, int64(status),
		amd64ReservedRegisterForCallEngine, callEngineExitContextNativeCallStatusCodeOffset)

	// Write back the cached SP to the actual eng.stackPointer.
	c.assembler.CompileConstToMemory(amd64.MOVQ, int64(c.locationStack.sp),
		amd64ReservedRegisterForCallEngine, callEngineStackContextStackPointerOffset)

	switch status {
	case nativeCallStatusCodeReturned:
	case nativeCallStatusCodeCallGoHostFunction, nativeCallStatusCodeCallBuiltInFunction:
		// Read the return address, and write it to callEngine.exitContext.returnAddress.
		returnAddressReg, ok := c.locationStack.takeFreeRegister(registerTypeGeneralPurpose)
		if !ok {
			panic("BUG: cannot take free register")
		}
		c.assembler.CompileReadInstructionAddress(returnAddressReg, amd64.RET)
		c.assembler.CompileRegisterToMemory(amd64.MOVQ,
			returnAddressReg, amd64ReservedRegisterForCallEngine, callEngineExitContextReturnAddressOffset)
	default:
		// This case, the execution traps, so take tmpReg and store the instruction address onto callEngine.returnAddress
		// so that the stack trace can contain the top frame's source position.
		tmpReg := amd64.RegR15
		c.assembler.CompileReadInstructionAddress(tmpReg, amd64.MOVQ)
		c.assembler.CompileRegisterToMemory(amd64.MOVQ,
			tmpReg, amd64ReservedRegisterForCallEngine, callEngineExitContextReturnAddressOffset)
	}

	c.assembler.CompileStandAlone(amd64.RET)
}

func (c *amd64Compiler) compilePreamble() (err error) {
	// We assume all function parameters are already pushed onto the stack by
	// the caller.
	c.locationStack.init(c.ir.Signature)

	if err := c.compileModuleContextInitialization(); err != nil {
		return err
	}

	// Check if it's necessary to grow the value stack by using max stack pointer.
	if err = c.compileMaybeGrowStack(); err != nil {
		return err
	}

	if c.withListener {
		if err = c.compileCallBuiltinFunction(builtinFunctionIndexFunctionListenerBefore); err != nil {
			return err
		}
	}

	c.compileReservedStackBasePointerInitialization()

	// Finally, we initialize the reserved memory register based on the module context.
	c.compileReservedMemoryPointerInitialization()
	return
}

func (c *amd64Compiler) compileReservedStackBasePointerInitialization() {
	// First, make reservedRegisterForStackBasePointer point to the beginning of the slice backing array.
	c.assembler.CompileMemoryToRegister(amd64.MOVQ,
		amd64ReservedRegisterForCallEngine, callEngineStackContextStackElement0AddressOffset,
		amd64ReservedRegisterForStackBasePointerAddress)

	// next we move the base pointer (callEngine.stackBasePointer) to the tmp register.
	c.assembler.CompileMemoryToRegister(amd64.ADDQ,
		amd64ReservedRegisterForCallEngine, callEngineStackContextStackBasePointerInBytesOffset,
		amd64ReservedRegisterForStackBasePointerAddress,
	)
}

func (c *amd64Compiler) compileReservedMemoryPointerInitialization() {
	if c.ir.HasMemory || c.ir.UsesMemory {
		c.assembler.CompileMemoryToRegister(amd64.MOVQ,
			amd64ReservedRegisterForCallEngine, callEngineModuleContextMemoryElement0AddressOffset,
			amd64ReservedRegisterForMemory,
		)
	}
}

// compileMaybeGrowStack adds instructions to check the necessity to grow the value stack,
// and if so, make the builtin function call to do so. These instructions are called in the function's
// preamble.
func (c *amd64Compiler) compileMaybeGrowStack() error {
	tmpRegister, ok := c.locationStack.takeFreeRegister(registerTypeGeneralPurpose)
	if !ok {
		panic("BUG: cannot take free register")
	}

	c.assembler.CompileMemoryToRegister(amd64.MOVQ,
		amd64ReservedRegisterForCallEngine, callEngineStackContextStackLenInBytesOffset, tmpRegister)
	c.assembler.CompileMemoryToRegister(amd64.SUBQ,
		amd64ReservedRegisterForCallEngine, callEngineStackContextStackBasePointerInBytesOffset, tmpRegister)

	// If stack base pointer + max stack pointer > stackLen, we need to grow the stack.
	cmpWithStackPointerCeil := c.assembler.CompileRegisterToConst(amd64.CMPQ, tmpRegister, 0)
	c.onStackPointerCeilDeterminedCallBack = func(stackPointerCeil uint64) {
		cmpWithStackPointerCeil.AssignDestinationConstant(int64(stackPointerCeil) << 3)
	}

	// Jump if we have no need to grow.
	jmpIfNoNeedToGrowStack := c.assembler.CompileJump(amd64.JCC)

	// Otherwise, we have to make the builtin function call to grow the call stack.
	if err := c.compileCallBuiltinFunction(builtinFunctionIndexGrowStack); err != nil {
		return err
	}

	c.assembler.SetJumpTargetOnNext(jmpIfNoNeedToGrowStack)
	return nil
}

// compileModuleContextInitialization adds instructions to initialize callEngine.ModuleContext's fields based on
// callEngine.ModuleContext.ModuleInstanceAddress.
// This is called in two cases: in function preamble, and on the return from (non-Go) function calls.
func (c *amd64Compiler) compileModuleContextInitialization() error {
	// amd64CallingConventionDestinationFunctionModuleInstanceAddressRegister holds the module instance's address
	// so mark it used so that it won't be used as a free register until the module context initialization finishes.
	c.locationStack.markRegisterUsed(amd64CallingConventionDestinationFunctionModuleInstanceAddressRegister)
	defer c.locationStack.markRegisterUnused(amd64CallingConventionDestinationFunctionModuleInstanceAddressRegister)

	// Obtain the temporary registers to be used in the followings.
	regs, found := c.locationStack.takeFreeRegisters(registerTypeGeneralPurpose, 2)
	if !found {
		// This in theory never happen as all the registers must be free except indexReg.
		return fmt.Errorf("could not find enough free registers")
	}
	c.locationStack.markRegisterUsed(regs...)

	// Alias these free tmp registers for readability.
	tmpRegister, tmpRegister2 := regs[0], regs[1]

	// If the module instance address stays the same, we could skip the entire code below.
	// The rationale/idea for this is that, in almost all use cases, users instantiate a single
	// Wasm binary and run the functions from it, rather than doing import/export on multiple
	// binaries. As a result, this cmp and jmp instruction sequence below must be easy for
	// x64 CPU to do branch prediction since almost 100% jump happens across function calls.
	c.assembler.CompileMemoryToRegister(amd64.CMPQ,
		amd64ReservedRegisterForCallEngine, callEngineModuleContextModuleInstanceAddressOffset, amd64CallingConventionDestinationFunctionModuleInstanceAddressRegister)
	jmpIfModuleNotChange := c.assembler.CompileJump(amd64.JEQ)

	// If engine.CallContext.ModuleInstanceAddress is not equal the value on amd64CallingConventionDestinationFunctionModuleInstanceAddressRegister,
	// we have to put the new value there.
	c.assembler.CompileRegisterToMemory(amd64.MOVQ, amd64CallingConventionDestinationFunctionModuleInstanceAddressRegister,
		amd64ReservedRegisterForCallEngine, callEngineModuleContextModuleInstanceAddressOffset)

	// Also, we have to update the following fields:
	// * callEngine.moduleContext.globalElement0Address
	// * callEngine.moduleContext.tableElement0Address
	// * callEngine.moduleContext.memoryInstance
	// * callEngine.moduleContext.memoryElement0Address
	// * callEngine.moduleContext.memorySliceLen
	// * callEngine.moduleContext.codesElement0Address
	// * callEngine.moduleContext.typeIDsElement0Address
	// * callEngine.moduleContext.dataInstancesElement0Address
	// * callEngine.moduleContext.elementInstancesElement0Address

	// Update globalElement0Address.
	//
	// Note: if there's global.get or set instruction in the function, the existence of the globals
	// is ensured by function validation at module instantiation phase, and that's why it is ok to
	// skip the initialization if the module's globals slice is empty.
	if len(c.ir.Globals) > 0 {
		// Since ModuleInstance.Globals is []*globalInstance, internally
		// the address of the first item in the underlying array lies exactly on the globals offset.
		// See https://go.dev/blog/slices-intro if unfamiliar.
		c.assembler.CompileMemoryToRegister(amd64.MOVQ, amd64CallingConventionDestinationFunctionModuleInstanceAddressRegister, moduleInstanceGlobalsOffset, tmpRegister)

		c.assembler.CompileRegisterToMemory(amd64.MOVQ, tmpRegister, amd64ReservedRegisterForCallEngine, callEngineModuleContextGlobalElement0AddressOffset)
	}

	// Update tableElement0Address.
	//
	// Note: if there's table instruction in the function, the existence of the table
	// is ensured by function validation at module instantiation phase, and that's
	// why it is ok to skip the initialization if the module's table doesn't exist.
	if c.ir.HasTable {
		// First, we need to read the *wasm.Table.
		c.assembler.CompileMemoryToRegister(amd64.MOVQ, amd64CallingConventionDestinationFunctionModuleInstanceAddressRegister, moduleInstanceTablesOffset, tmpRegister)

		// At this point, tmpRegister holds the address of ModuleInstance.Table.
		// So we are ready to read and put the first item's address stored in Table.Table.
		// Here we read the value into tmpRegister2.
		c.assembler.CompileRegisterToMemory(amd64.MOVQ, tmpRegister,
			amd64ReservedRegisterForCallEngine, callEngineModuleContextTablesElement0AddressOffset)

		// Finally, we put &ModuleInstance.TypeIDs[0] into moduleContext.typeIDsElement0Address.
		c.assembler.CompileMemoryToRegister(amd64.MOVQ,
			amd64CallingConventionDestinationFunctionModuleInstanceAddressRegister, moduleInstanceTypeIDsOffset, tmpRegister)
		c.assembler.CompileRegisterToMemory(amd64.MOVQ,
			tmpRegister, amd64ReservedRegisterForCallEngine, callEngineModuleContextTypeIDsElement0AddressOffset)
	}

	// Update memoryElement0Address and memorySliceLen.
	//
	// Note: if there's memory instruction in the function, memory instance must be non-nil.
	// That is ensured by function validation at module instantiation phase, and that's
	// why it is ok to skip the initialization if the module's memory instance is nil.
	if c.ir.HasMemory {
		c.assembler.CompileMemoryToRegister(amd64.MOVQ,
			amd64CallingConventionDestinationFunctionModuleInstanceAddressRegister, moduleInstanceMemoryOffset,
			tmpRegister)

		// Set memory instance.
		c.assembler.CompileRegisterToMemory(amd64.MOVQ, tmpRegister,
			amd64ReservedRegisterForCallEngine, callEngineModuleContextMemoryInstanceOffset)

		// Set length.
		c.assembler.CompileMemoryToRegister(amd64.MOVQ, tmpRegister, memoryInstanceBufferLenOffset, tmpRegister2)
		c.assembler.CompileRegisterToMemory(amd64.MOVQ, tmpRegister2,
			amd64ReservedRegisterForCallEngine, callEngineModuleContextMemorySliceLenOffset)

		// Set element zero address.
		c.assembler.CompileMemoryToRegister(amd64.MOVQ, tmpRegister, memoryInstanceBufferOffset, tmpRegister2)
		c.assembler.CompileRegisterToMemory(amd64.MOVQ, tmpRegister2,
			amd64ReservedRegisterForCallEngine, callEngineModuleContextMemoryElement0AddressOffset)
	}

	// Update moduleContext.codesElement0Address
	{
		// "tmpRegister = [moduleInstanceAddressRegister + moduleInstanceEngineOffset + interfaceDataOffset] (== *moduleEngine)"
		//
		// Go's interface is laid out on memory as two quad words as struct {tab, data uintptr}
		// where tab points to the interface table, and the latter points to the actual
		// implementation of interface. This case, we extract "data" pointer as *moduleEngine.
		// See the following references for detail:
		// * https://research.swtch.com/interfaces
		// * https://github.com/golang/go/blob/release-branch.go1.20/src/runtime/runtime2.go#L207-L210
		c.assembler.CompileMemoryToRegister(amd64.MOVQ, amd64CallingConventionDestinationFunctionModuleInstanceAddressRegister, moduleInstanceEngineOffset+interfaceDataOffset, tmpRegister)

		// "tmpRegister = [tmpRegister + moduleEnginecodesOffset] (== &moduleEngine.codes[0])"
		c.assembler.CompileMemoryToRegister(amd64.MOVQ, tmpRegister, moduleEngineFunctionsOffset, tmpRegister)

		// "callEngine.moduleContext.functionsElement0Address = tmpRegister".
		c.assembler.CompileRegisterToMemory(amd64.MOVQ, tmpRegister, amd64ReservedRegisterForCallEngine,
			callEngineModuleContextFunctionsElement0AddressOffset)
	}

	// Update dataInstancesElement0Address.
	if c.ir.HasDataInstances {
		// "tmpRegister = &moduleInstance.DataInstances[0]"
		c.assembler.CompileMemoryToRegister(
			amd64.MOVQ,
			amd64CallingConventionDestinationFunctionModuleInstanceAddressRegister, moduleInstanceDataInstancesOffset,
			tmpRegister,
		)
		// "callEngine.moduleContext.dataInstancesElement0Address = tmpRegister".
		c.assembler.CompileRegisterToMemory(
			amd64.MOVQ,
			tmpRegister,
			amd64ReservedRegisterForCallEngine, callEngineModuleContextDataInstancesElement0AddressOffset,
		)
	}

	// Update callEngine.moduleContext.elementInstancesElement0Address
	if c.ir.HasElementInstances {
		// "tmpRegister = &moduleInstance.ElementInstnaces[0]"
		c.assembler.CompileMemoryToRegister(
			amd64.MOVQ,
			amd64CallingConventionDestinationFunctionModuleInstanceAddressRegister, moduleInstanceElementInstancesOffset,
			tmpRegister,
		)
		// "callEngine.moduleContext.dataInstancesElement0Address = tmpRegister".
		c.assembler.CompileRegisterToMemory(
			amd64.MOVQ,
			tmpRegister,
			amd64ReservedRegisterForCallEngine, callEngineModuleContextElementInstancesElement0AddressOffset,
		)
	}

	c.locationStack.markRegisterUnused(regs...)

	// Set the jump target towards the next instruction for the case where module instance address hasn't changed.
	c.assembler.SetJumpTargetOnNext(jmpIfModuleNotChange)
	return nil
}

// compileEnsureOnRegister ensures that the given value is located on a
// general purpose register of an appropriate type.
func (c *amd64Compiler) compileEnsureOnRegister(loc *runtimeValueLocation) (err error) {
	if loc.onStack() {
		// Allocate the register.
		reg, err := c.allocateRegister(loc.getRegisterType())
		if err != nil {
			return err
		}

		// Mark it uses the register.
		loc.setRegister(reg)
		c.locationStack.markRegisterUsed(reg)

		c.compileLoadValueOnStackToRegister(loc)
	} else if loc.onConditionalRegister() {
		err = c.compileLoadConditionalRegisterToGeneralPurposeRegister(loc)
	}
	return
}

// compileMaybeSwapRegisters swaps two registers if they're not equal.
func (c *amd64Compiler) compileMaybeSwapRegisters(reg1, reg2 asm.Register) {
	if reg1 != reg2 {
		c.assembler.CompileRegisterToRegister(amd64.XCHGQ, reg1, reg2)
	}
}

// compilePreventCrossedTargetRegisters swaps registers in such a way, that for neither runtimeValueLocation from locs its
// corresponding register with the same index from targets is occupied by some other runtimeValueLocation from locs. It returns a
// closure to restore the original register placement.
//
// This function makes it possible to safely exchange one set of registers with another, where a register might be in both sets.
// Each register will correspond either to itself or another register not present in its own set.
//
// For example, if we have locs = [AX, BX, CX], targets = [BX, SI, AX], then it'll do two swaps
// to make locs = [BX, CX, AX].
func (c *amd64Compiler) compilePreventCrossedTargetRegisters(locs []*runtimeValueLocation, targets []asm.Register) (restore func()) {
	type swap struct{ srcIndex, dstIndex int }
	var swaps []swap
	for i := range locs {
		targetLocation := -1 // -1 means not found.
		for j := range locs {
			if locs[j].register == targets[i] {
				targetLocation = j
				break
			}
		}
		if targetLocation != -1 && targetLocation != i {
			c.compileMaybeSwapRegisters(locs[i].register, locs[targetLocation].register)
			locs[i].register, locs[targetLocation].register = locs[targetLocation].register, locs[i].register
			swaps = append(swaps, swap{i, targetLocation})
		}
	}
	return func() {
		// Restore in reverse order because a register can be moved multiple times.
		for i := len(swaps) - 1; i >= 0; i -= 1 {
			r1, r2 := swaps[i].srcIndex, swaps[i].dstIndex
			c.compileMaybeSwapRegisters(locs[r1].register, locs[r2].register)
			locs[r1].register, locs[r2].register = locs[r2].register, locs[r1].register
		}
	}
}
