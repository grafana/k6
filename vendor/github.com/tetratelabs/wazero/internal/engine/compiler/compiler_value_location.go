package compiler

import (
	"fmt"
	"strings"

	"github.com/tetratelabs/wazero/internal/asm"
	"github.com/tetratelabs/wazero/internal/wasm"
)

var (
	// unreservedGeneralPurposeRegisters contains unreserved general purpose registers of integer type.
	unreservedGeneralPurposeRegisters []asm.Register

	// unreservedVectorRegisters contains unreserved vector registers.
	unreservedVectorRegisters []asm.Register
)

func isGeneralPurposeRegister(r asm.Register) bool {
	return unreservedGeneralPurposeRegisters[0] <= r && r <= unreservedGeneralPurposeRegisters[len(unreservedGeneralPurposeRegisters)-1]
}

func isVectorRegister(r asm.Register) bool {
	return unreservedVectorRegisters[0] <= r && r <= unreservedVectorRegisters[len(unreservedVectorRegisters)-1]
}

// runtimeValueLocation corresponds to each variable pushed onto the wazeroir (virtual) stack,
// and it has the information about where it exists in the physical machine.
// It might exist in registers, or maybe on in the non-virtual physical stack allocated in memory.
type runtimeValueLocation struct {
	valueType runtimeValueType
	// register is set to asm.NilRegister if the value is stored in the memory stack.
	register asm.Register
	// conditionalRegister is set to conditionalRegisterStateUnset if the value is not on the conditional register.
	conditionalRegister asm.ConditionalRegisterState
	// stackPointer is the location of this value in the memory stack at runtime,
	stackPointer uint64
}

func (v *runtimeValueLocation) getRegisterType() (ret registerType) {
	switch v.valueType {
	case runtimeValueTypeI32, runtimeValueTypeI64:
		ret = registerTypeGeneralPurpose
	case runtimeValueTypeF32, runtimeValueTypeF64,
		runtimeValueTypeV128Lo, runtimeValueTypeV128Hi:
		ret = registerTypeVector
	default:
		panic("BUG")
	}
	return
}

type runtimeValueType byte

const (
	runtimeValueTypeNone runtimeValueType = iota
	runtimeValueTypeI32
	runtimeValueTypeI64
	runtimeValueTypeF32
	runtimeValueTypeF64
	runtimeValueTypeV128Lo
	runtimeValueTypeV128Hi
)

func (r runtimeValueType) String() (ret string) {
	switch r {
	case runtimeValueTypeI32:
		ret = "i32"
	case runtimeValueTypeI64:
		ret = "i64"
	case runtimeValueTypeF32:
		ret = "f32"
	case runtimeValueTypeF64:
		ret = "f64"
	case runtimeValueTypeV128Lo:
		ret = "v128.lo"
	case runtimeValueTypeV128Hi:
		ret = "v128.hi"
	}
	return
}

func (v *runtimeValueLocation) setRegister(reg asm.Register) {
	v.register = reg
	v.conditionalRegister = asm.ConditionalRegisterStateUnset
}

func (v *runtimeValueLocation) onRegister() bool {
	return v.register != asm.NilRegister && v.conditionalRegister == asm.ConditionalRegisterStateUnset
}

func (v *runtimeValueLocation) onStack() bool {
	return v.register == asm.NilRegister && v.conditionalRegister == asm.ConditionalRegisterStateUnset
}

func (v *runtimeValueLocation) onConditionalRegister() bool {
	return v.conditionalRegister != asm.ConditionalRegisterStateUnset
}

func (v *runtimeValueLocation) String() string {
	var location string
	if v.onStack() {
		location = fmt.Sprintf("stack(%d)", v.stackPointer)
	} else if v.onConditionalRegister() {
		location = fmt.Sprintf("conditional(%d)", v.conditionalRegister)
	} else if v.onRegister() {
		location = fmt.Sprintf("register(%s)", registerNameFn(v.register))
	}
	return fmt.Sprintf("{type=%s,location=%s}", v.valueType, location)
}

func newRuntimeValueLocationStack() runtimeValueLocationStack {
	return runtimeValueLocationStack{
		stack:                             make([]runtimeValueLocation, 10),
		usedRegisters:                     map[asm.Register]struct{}{},
		unreservedVectorRegisters:         unreservedVectorRegisters,
		unreservedGeneralPurposeRegisters: unreservedGeneralPurposeRegisters,
	}
}

// runtimeValueLocationStack represents the wazeroir virtual stack
// where each item holds the location information about where it exists
// on the physical machine at runtime.
//
// Notably this is only used in the compilation phase, not runtime,
// and we change the state of this struct at every wazeroir operation we compile.
// In this way, we can see where the operands of an operation (for example,
// two variables for wazeroir add operation.) exist and check the necessity for
// moving the variable to registers to perform actual CPU instruction
// to achieve wazeroir's add operation.
type runtimeValueLocationStack struct {
	// stack holds all the variables.
	stack []runtimeValueLocation
	// sp is the current stack pointer.
	sp uint64
	// usedRegisters stores the used registers.
	usedRegisters map[asm.Register]struct{}
	// stackPointerCeil tracks max(.sp) across the lifespan of this struct.
	stackPointerCeil uint64
	// unreservedGeneralPurposeRegisters and unreservedVectorRegisters hold
	// architecture dependent unreserved register list.
	unreservedGeneralPurposeRegisters, unreservedVectorRegisters []asm.Register
}

func (v *runtimeValueLocationStack) initialized() bool {
	return len(v.unreservedGeneralPurposeRegisters) > 0
}

func (v *runtimeValueLocationStack) reset() {
	v.stackPointerCeil, v.sp = 0, 0
	v.stack = v.stack[:0]
	v.usedRegisters = map[asm.Register]struct{}{}
}

func (v *runtimeValueLocationStack) String() string {
	var stackStr []string
	for i := uint64(0); i < v.sp; i++ {
		stackStr = append(stackStr, v.stack[i].String())
	}
	var usedRegisters []string
	for reg := range v.usedRegisters {
		usedRegisters = append(usedRegisters, registerNameFn(reg))
	}
	return fmt.Sprintf("sp=%d, stack=[%s], used_registers=[%s]", v.sp, strings.Join(stackStr, ","), strings.Join(usedRegisters, ","))
}

func (v *runtimeValueLocationStack) clone() runtimeValueLocationStack {
	ret := runtimeValueLocationStack{}
	ret.sp = v.sp
	ret.usedRegisters = make(map[asm.Register]struct{}, len(ret.usedRegisters))
	for r := range v.usedRegisters {
		ret.markRegisterUsed(r)
	}
	ret.stack = make([]runtimeValueLocation, len(v.stack))
	copy(ret.stack, v.stack)
	ret.stackPointerCeil = v.stackPointerCeil
	ret.unreservedGeneralPurposeRegisters = v.unreservedGeneralPurposeRegisters
	ret.unreservedVectorRegisters = v.unreservedVectorRegisters
	return ret
}

// pushRuntimeValueLocationOnRegister creates a new runtimeValueLocation with a given register and pushes onto
// the location stack.
func (v *runtimeValueLocationStack) pushRuntimeValueLocationOnRegister(reg asm.Register, vt runtimeValueType) (loc *runtimeValueLocation) {
	loc = v.push(reg, asm.ConditionalRegisterStateUnset)
	loc.valueType = vt
	return
}

// pushRuntimeValueLocationOnRegister creates a new runtimeValueLocation and pushes onto the location stack.
func (v *runtimeValueLocationStack) pushRuntimeValueLocationOnStack() (loc *runtimeValueLocation) {
	loc = v.push(asm.NilRegister, asm.ConditionalRegisterStateUnset)
	loc.valueType = runtimeValueTypeNone
	return
}

// pushRuntimeValueLocationOnRegister creates a new runtimeValueLocation with a given conditional register state
// and pushes onto the location stack.
func (v *runtimeValueLocationStack) pushRuntimeValueLocationOnConditionalRegister(state asm.ConditionalRegisterState) (loc *runtimeValueLocation) {
	loc = v.push(asm.NilRegister, state)
	loc.valueType = runtimeValueTypeI32
	return
}

// push a runtimeValueLocation onto the stack.
func (v *runtimeValueLocationStack) push(reg asm.Register, conditionalRegister asm.ConditionalRegisterState) (ret *runtimeValueLocation) {
	if v.sp >= uint64(len(v.stack)) {
		// This case we need to grow the stack capacity by appending the item,
		// rather than indexing.
		v.stack = append(v.stack, runtimeValueLocation{})
	}
	ret = &v.stack[v.sp]
	ret.register, ret.conditionalRegister, ret.stackPointer = reg, conditionalRegister, v.sp
	v.sp++
	// stackPointerCeil must be set after sp is incremented since
	// we skip the stack grow if len(stack) >= basePointer+stackPointerCeil.
	if v.sp > v.stackPointerCeil {
		v.stackPointerCeil = v.sp
	}
	return
}

func (v *runtimeValueLocationStack) pop() (loc *runtimeValueLocation) {
	v.sp--
	loc = &v.stack[v.sp]
	return
}

func (v *runtimeValueLocationStack) popV128() (loc *runtimeValueLocation) {
	v.sp -= 2
	loc = &v.stack[v.sp]
	return
}

func (v *runtimeValueLocationStack) peek() (loc *runtimeValueLocation) {
	loc = &v.stack[v.sp-1]
	return
}

func (v *runtimeValueLocationStack) releaseRegister(loc *runtimeValueLocation) {
	v.markRegisterUnused(loc.register)
	loc.register = asm.NilRegister
	loc.conditionalRegister = asm.ConditionalRegisterStateUnset
}

func (v *runtimeValueLocationStack) markRegisterUnused(regs ...asm.Register) {
	for _, reg := range regs {
		delete(v.usedRegisters, reg)
	}
}

func (v *runtimeValueLocationStack) markRegisterUsed(regs ...asm.Register) {
	for _, reg := range regs {
		v.usedRegisters[reg] = struct{}{}
	}
}

type registerType byte

const (
	registerTypeGeneralPurpose registerType = iota
	// registerTypeVector represents a vector register which can be used for either scalar float
	// operation or SIMD vector operation depending on the instruction by which the register is used.
	//
	// Note: In normal assembly language, scalar float and vector register have different notations as
	// Vn is for vectors and Qn is for scalar floats on arm64 for example. But on physical hardware,
	// they are placed on the same locations. (Qn means the lower 64-bit of Vn vector register on arm64).
	//
	// In wazero, for the sake of simplicity in the register allocation, we intentionally conflate these two types
	// and delegate the decision to the assembler which is aware of the instruction types for which these registers are used.
	registerTypeVector
)

func (tp registerType) String() (ret string) {
	switch tp {
	case registerTypeGeneralPurpose:
		ret = "int"
	case registerTypeVector:
		ret = "vector"
	}
	return
}

// takeFreeRegister searches for unused registers. Any found are marked used and returned.
func (v *runtimeValueLocationStack) takeFreeRegister(tp registerType) (reg asm.Register, found bool) {
	var targetRegs []asm.Register
	switch tp {
	case registerTypeVector:
		targetRegs = v.unreservedVectorRegisters
	case registerTypeGeneralPurpose:
		targetRegs = v.unreservedGeneralPurposeRegisters
	}
	for _, candidate := range targetRegs {
		if _, ok := v.usedRegisters[candidate]; ok {
			continue
		}
		return candidate, true
	}
	return 0, false
}

func (v *runtimeValueLocationStack) takeFreeRegisters(tp registerType, num int) (regs []asm.Register, found bool) {
	var targetRegs []asm.Register
	switch tp {
	case registerTypeVector:
		targetRegs = v.unreservedVectorRegisters
	case registerTypeGeneralPurpose:
		targetRegs = v.unreservedGeneralPurposeRegisters
	}

	regs = make([]asm.Register, 0, num)
	for _, candidate := range targetRegs {
		if _, ok := v.usedRegisters[candidate]; ok {
			continue
		}
		regs = append(regs, candidate)
		if len(regs) == num {
			found = true
			break
		}
	}
	return
}

// Search through the stack, and steal the register from the last used
// variable on the stack.
func (v *runtimeValueLocationStack) takeStealTargetFromUsedRegister(tp registerType) (*runtimeValueLocation, bool) {
	for i := uint64(0); i < v.sp; i++ {
		loc := &v.stack[i]
		if loc.onRegister() {
			switch tp {
			case registerTypeVector:
				if loc.valueType == runtimeValueTypeV128Hi {
					panic("BUG: V128Hi must be above the corresponding V128Lo")
				}
				if isVectorRegister(loc.register) {
					return loc, true
				}
			case registerTypeGeneralPurpose:
				if isGeneralPurposeRegister(loc.register) {
					return loc, true
				}
			}
		}
	}
	return nil, false
}

// init sets up the runtimeValueLocationStack which reflects the state of
// the stack at the beginning of the function.
//
// See the diagram in callEngine.stack.
func (v *runtimeValueLocationStack) init(sig *wasm.FunctionType) {
	for _, t := range sig.Params {
		loc := v.pushRuntimeValueLocationOnStack()
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
			hi := v.pushRuntimeValueLocationOnStack()
			hi.valueType = runtimeValueTypeV128Hi
		default:
			panic("BUG")
		}
	}

	// If the len(results) > len(args), the slots for all results are reserved after
	// arguments, so we reflect that into the location stack.
	for i := 0; i < sig.ResultNumInUint64-sig.ParamNumInUint64; i++ {
		_ = v.pushRuntimeValueLocationOnStack()
	}

	// Then push the control frame fields.
	for i := 0; i < callFrameDataSizeInUint64; i++ {
		loc := v.pushRuntimeValueLocationOnStack()
		loc.valueType = runtimeValueTypeI64
	}
}

// getCallFrameLocations returns each field of callFrame's runtime location.
//
// See the diagram in callEngine.stack.
func (v *runtimeValueLocationStack) getCallFrameLocations(sig *wasm.FunctionType) (
	returnAddress, callerStackBasePointerInBytes, callerFunction *runtimeValueLocation,
) {
	offset := callFrameOffset(sig)
	return &v.stack[offset], &v.stack[offset+1], &v.stack[offset+2]
}

// pushCallFrame pushes a call frame's runtime locations onto the stack assuming that
// the function call parameters are already pushed there.
//
// See the diagram in callEngine.stack.
func (v *runtimeValueLocationStack) pushCallFrame(callTargetFunctionType *wasm.FunctionType) (
	returnAddress, callerStackBasePointerInBytes, callerFunction *runtimeValueLocation,
) {
	// If len(results) > len(args), we reserve the slots for the results below the call frame.
	reservedSlotsBeforeCallFrame := callTargetFunctionType.ResultNumInUint64 - callTargetFunctionType.ParamNumInUint64
	for i := 0; i < reservedSlotsBeforeCallFrame; i++ {
		v.pushRuntimeValueLocationOnStack()
	}

	// Push the runtime location for each field of callFrame struct. Note that each of them has
	// uint64 type, and therefore must be treated as runtimeValueTypeI64.

	// callFrame.returnAddress
	returnAddress = v.pushRuntimeValueLocationOnStack()
	returnAddress.valueType = runtimeValueTypeI64
	// callFrame.returnStackBasePointerInBytes
	callerStackBasePointerInBytes = v.pushRuntimeValueLocationOnStack()
	callerStackBasePointerInBytes.valueType = runtimeValueTypeI64
	// callFrame.function
	callerFunction = v.pushRuntimeValueLocationOnStack()
	callerFunction.valueType = runtimeValueTypeI64
	return
}
