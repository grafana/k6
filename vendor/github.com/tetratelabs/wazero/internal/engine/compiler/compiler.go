package compiler

import (
	"github.com/tetratelabs/wazero/internal/asm"
	"github.com/tetratelabs/wazero/internal/wazeroir"
)

// compiler is the interface of architecture-specific native code compiler,
// and this is responsible for compiling native code for all wazeroir operations.
type compiler interface {
	Init(ir *wazeroir.CompilationResult, withListener bool)

	// String is for debugging purpose.
	String() string
	// compilePreamble is called before compiling any wazeroir operation.
	// This is used, for example, to initialize the reserved registers, etc.
	compilePreamble() error
	// compile generates the byte slice of native code.
	// stackPointerCeil is the max stack pointer that the target function would reach.
	compile() (code []byte, stackPointerCeil uint64, err error)
	// compileGoHostFunction adds the trampoline code from which native code can jump into the Go-defined host function.
	// TODO: maybe we wouldn't need to have trampoline for host functions.
	compileGoDefinedHostFunction() error
	// compileLabel notify compilers of the beginning of a label.
	// Return true if the compiler decided to skip the entire label.
	// See wazeroir.OperationLabel
	compileLabel(o wazeroir.OperationLabel) (skipThisLabel bool)
	// compileUnreachable adds instruction to perform wazeroir.OperationUnreachable.
	compileUnreachable() error
	// compileSet adds instruction to perform wazeroir.OperationSet.
	compileSet(o wazeroir.OperationSet) error
	// compileGlobalGet adds instructions to perform wazeroir.OperationGlobalGet.
	compileGlobalGet(o wazeroir.OperationGlobalGet) error
	// compileGlobalSet adds instructions to perform wazeroir.OperationGlobalSet.
	compileGlobalSet(o wazeroir.OperationGlobalSet) error
	// compileBr adds instructions to perform wazeroir.OperationBr.
	compileBr(o wazeroir.OperationBr) error
	// compileBrIf adds instructions to perform wazeroir.OperationBrIf.
	compileBrIf(o wazeroir.OperationBrIf) error
	// compileBrTable adds instructions to perform wazeroir.OperationBrTable.
	compileBrTable(o wazeroir.OperationBrTable) error
	// compileCall adds instructions to perform wazeroir.OperationCall.
	compileCall(o wazeroir.OperationCall) error
	// compileCallIndirect adds instructions to perform wazeroir.OperationCallIndirect.
	compileCallIndirect(o wazeroir.OperationCallIndirect) error
	// compileDrop adds instructions to perform wazeroir.OperationDrop.
	compileDrop(o wazeroir.OperationDrop) error
	// compileSelect adds instructions to perform wazeroir.OperationSelect.
	compileSelect(o wazeroir.OperationSelect) error
	// compilePick adds instructions to perform wazeroir.OperationPick.
	compilePick(o wazeroir.OperationPick) error
	// compileAdd adds instructions to perform wazeroir.OperationAdd.
	compileAdd(o wazeroir.OperationAdd) error
	// compileSub adds instructions to perform wazeroir.OperationSub.
	compileSub(o wazeroir.OperationSub) error
	// compileMul adds instructions to perform wazeroir.OperationMul.
	compileMul(o wazeroir.OperationMul) error
	// compileClz adds instructions to perform wazeroir.OperationClz.
	compileClz(o wazeroir.OperationClz) error
	// compileCtz adds instructions to perform wazeroir.OperationCtz.
	compileCtz(o wazeroir.OperationCtz) error
	// compilePopcnt adds instructions to perform wazeroir.OperationPopcnt.
	compilePopcnt(o wazeroir.OperationPopcnt) error
	// compileDiv adds instructions to perform wazeroir.OperationDiv.
	compileDiv(o wazeroir.OperationDiv) error
	// compileRem adds instructions to perform wazeroir.OperationRem.
	compileRem(o wazeroir.OperationRem) error
	// compileAnd adds instructions to perform wazeroir.OperationAnd.
	compileAnd(o wazeroir.OperationAnd) error
	// compileOr adds instructions to perform wazeroir.OperationOr.
	compileOr(o wazeroir.OperationOr) error
	// compileXor adds instructions to perform wazeroir.OperationXor.
	compileXor(o wazeroir.OperationXor) error
	// compileShl adds instructions to perform wazeroir.OperationShl.
	compileShl(o wazeroir.OperationShl) error
	// compileShr adds instructions to perform wazeroir.OperationShr.
	compileShr(o wazeroir.OperationShr) error
	// compileRotl adds instructions to perform wazeroir.OperationRotl.
	compileRotl(o wazeroir.OperationRotl) error
	// compileRotr adds instructions to perform wazeroir.OperationRotr.
	compileRotr(o wazeroir.OperationRotr) error
	// compileNeg adds instructions to perform wazeroir.OperationAbs.
	compileAbs(o wazeroir.OperationAbs) error
	// compileNeg adds instructions to perform wazeroir.OperationNeg.
	compileNeg(o wazeroir.OperationNeg) error
	// compileCeil adds instructions to perform wazeroir.OperationCeil.
	compileCeil(o wazeroir.OperationCeil) error
	// compileFloor adds instructions to perform wazeroir.OperationFloor.
	compileFloor(o wazeroir.OperationFloor) error
	// compileTrunc adds instructions to perform wazeroir.OperationTrunc.
	compileTrunc(o wazeroir.OperationTrunc) error
	// compileNearest adds instructions to perform wazeroir.OperationNearest.
	compileNearest(o wazeroir.OperationNearest) error
	// compileSqrt adds instructions perform wazeroir.OperationSqrt.
	compileSqrt(o wazeroir.OperationSqrt) error
	// compileMin adds instructions perform wazeroir.OperationMin.
	compileMin(o wazeroir.OperationMin) error
	// compileMax adds instructions perform wazeroir.OperationMax.
	compileMax(o wazeroir.OperationMax) error
	// compileCopysign adds instructions to perform wazeroir.OperationCopysign.
	compileCopysign(o wazeroir.OperationCopysign) error
	// compileI32WrapFromI64 adds instructions to perform wazeroir.OperationI32WrapFromI64.
	compileI32WrapFromI64() error
	// compileITruncFromF adds instructions to perform wazeroir.OperationITruncFromF.
	compileITruncFromF(o wazeroir.OperationITruncFromF) error
	// compileFConvertFromI adds instructions to perform wazeroir.OperationFConvertFromI.
	compileFConvertFromI(o wazeroir.OperationFConvertFromI) error
	// compileF32DemoteFromF64 adds instructions to perform wazeroir.OperationF32DemoteFromF64.
	compileF32DemoteFromF64() error
	// compileF64PromoteFromF32 adds instructions to perform wazeroir.OperationF64PromoteFromF32.
	compileF64PromoteFromF32() error
	// compileI32ReinterpretFromF32 adds instructions to perform wazeroir.OperationI32ReinterpretFromF32.
	compileI32ReinterpretFromF32() error
	// compileI64ReinterpretFromF64 adds instructions to perform wazeroir.OperationI64ReinterpretFromF64.
	compileI64ReinterpretFromF64() error
	// compileF32ReinterpretFromI32 adds instructions to perform wazeroir.OperationF32ReinterpretFromI32.
	compileF32ReinterpretFromI32() error
	// compileF64ReinterpretFromI64 adds instructions to perform wazeroir.OperationF64ReinterpretFromI64.
	compileF64ReinterpretFromI64() error
	// compileExtend adds instructions to perform wazeroir.OperationExtend.
	compileExtend(o wazeroir.OperationExtend) error
	// compileEq adds instructions to perform wazeroir.OperationEq.
	compileEq(o wazeroir.OperationEq) error
	// compileEq adds instructions to perform wazeroir.OperationNe.
	compileNe(o wazeroir.OperationNe) error
	// compileEq adds instructions to perform wazeroir.OperationEqz.
	compileEqz(o wazeroir.OperationEqz) error
	// compileLt adds instructions to perform wazeroir.OperationLt.
	compileLt(o wazeroir.OperationLt) error
	// compileGt adds instructions to perform wazeroir.OperationGt.
	compileGt(o wazeroir.OperationGt) error
	// compileLe adds instructions to perform wazeroir.OperationLe.
	compileLe(o wazeroir.OperationLe) error
	// compileLe adds instructions to perform wazeroir.OperationGe.
	compileGe(o wazeroir.OperationGe) error
	// compileLoad adds instructions to perform wazeroir.OperationLoad.
	compileLoad(o wazeroir.OperationLoad) error
	// compileLoad8 adds instructions to perform wazeroir.OperationLoad8.
	compileLoad8(o wazeroir.OperationLoad8) error
	// compileLoad16 adds instructions to perform wazeroir.OperationLoad16.
	compileLoad16(o wazeroir.OperationLoad16) error
	// compileLoad32 adds instructions to perform wazeroir.OperationLoad32.
	compileLoad32(o wazeroir.OperationLoad32) error
	// compileStore adds instructions to perform wazeroir.OperationStore.
	compileStore(o wazeroir.OperationStore) error
	// compileStore8 adds instructions to perform wazeroir.OperationStore8.
	compileStore8(o wazeroir.OperationStore8) error
	// compileStore16 adds instructions to perform wazeroir.OperationStore16.
	compileStore16(o wazeroir.OperationStore16) error
	// compileStore32 adds instructions to perform wazeroir.OperationStore32.
	compileStore32(o wazeroir.OperationStore32) error
	// compileMemorySize adds instruction to perform wazeroir.OperationMemoryGrow.
	compileMemoryGrow() error
	// compileMemorySize adds instruction to perform wazeroir.OperationMemorySize.
	compileMemorySize() error
	// compileConstI32 adds instruction to perform wazeroir.OperationConstI32.
	compileConstI32(o wazeroir.OperationConstI32) error
	// compileConstI64 adds instruction to perform wazeroir.OperationConstI64.
	compileConstI64(o wazeroir.OperationConstI64) error
	// compileConstF32 adds instruction to perform wazeroir.OperationConstF32.
	compileConstF32(o wazeroir.OperationConstF32) error
	// compileConstF64 adds instruction to perform wazeroir.OperationConstF64.
	compileConstF64(o wazeroir.OperationConstF64) error
	// compileSignExtend32From8 adds instructions to perform wazeroir.OperationSignExtend32From8.
	compileSignExtend32From8() error
	// compileSignExtend32From16 adds instructions to perform wazeroir.OperationSignExtend32From16.
	compileSignExtend32From16() error
	// compileSignExtend64From8 adds instructions to perform wazeroir.OperationSignExtend64From8.
	compileSignExtend64From8() error
	// compileSignExtend64From16 adds instructions to perform wazeroir.OperationSignExtend64From16.
	compileSignExtend64From16() error
	// compileSignExtend64From32 adds instructions to perform wazeroir.OperationSignExtend64From32.
	compileSignExtend64From32() error
	// compileMemoryInit adds instructions to perform wazeroir.OperationMemoryInit.
	compileMemoryInit(wazeroir.OperationMemoryInit) error
	// compileDataDrop adds instructions to perform wazeroir.OperationDataDrop.
	compileDataDrop(wazeroir.OperationDataDrop) error
	// compileMemoryCopy adds instructions to perform wazeroir.OperationMemoryCopy.
	compileMemoryCopy() error
	// compileMemoryFill adds instructions to perform wazeroir.OperationMemoryFill.
	compileMemoryFill() error
	// compileTableInit adds instructions to perform wazeroir.OperationTableInit.
	compileTableInit(wazeroir.OperationTableInit) error
	// compileTableCopy adds instructions to perform wazeroir.OperationTableCopy.
	compileTableCopy(wazeroir.OperationTableCopy) error
	// compileElemDrop adds instructions to perform wazeroir.OperationElemDrop.
	compileElemDrop(wazeroir.OperationElemDrop) error
	// compileRefFunc adds instructions to perform wazeroir.OperationRefFunc.
	compileRefFunc(wazeroir.OperationRefFunc) error
	// compileTableGet adds instructions to perform wazeroir.OperationTableGet.
	compileTableGet(wazeroir.OperationTableGet) error
	// compileTableSet adds instructions to perform wazeroir.OperationTableSet.
	compileTableSet(wazeroir.OperationTableSet) error
	// compileTableGrow adds instructions to perform wazeroir.OperationTableGrow.
	compileTableGrow(wazeroir.OperationTableGrow) error
	// compileTableSize adds instructions to perform wazeroir.OperationTableSize.
	compileTableSize(wazeroir.OperationTableSize) error
	// compileTableFill adds instructions to perform wazeroir.OperationTableFill.
	compileTableFill(wazeroir.OperationTableFill) error
	// compileV128Const adds instructions to perform wazeroir.OperationV128Const.
	compileV128Const(wazeroir.OperationV128Const) error
	// compileV128Add adds instructions to perform wazeroir.OperationV128Add.
	compileV128Add(o wazeroir.OperationV128Add) error
	// compileV128Sub adds instructions to perform wazeroir.OperationV128Sub.
	compileV128Sub(o wazeroir.OperationV128Sub) error
	// compileV128Load adds instructions to perform wazeroir.OperationV128Load.
	compileV128Load(o wazeroir.OperationV128Load) error
	// compileV128LoadLane adds instructions to perform wazeroir.OperationV128LoadLane.
	compileV128LoadLane(o wazeroir.OperationV128LoadLane) error
	// compileV128Store adds instructions to perform wazeroir.OperationV128Store.
	compileV128Store(o wazeroir.OperationV128Store) error
	// compileV128StoreLane adds instructions to perform wazeroir.OperationV128StoreLane.
	compileV128StoreLane(o wazeroir.OperationV128StoreLane) error
	// compileV128ExtractLane adds instructions to perform wazeroir.OperationV128ExtractLane.
	compileV128ExtractLane(o wazeroir.OperationV128ExtractLane) error
	// compileV128ReplaceLane adds instructions to perform wazeroir.OperationV128ReplaceLane.
	compileV128ReplaceLane(o wazeroir.OperationV128ReplaceLane) error
	// compileV128Splat adds instructions to perform wazeroir.OperationV128Splat.
	compileV128Splat(o wazeroir.OperationV128Splat) error
	// compileV128Shuffle adds instructions to perform wazeroir.OperationV128Shuffle.
	compileV128Shuffle(o wazeroir.OperationV128Shuffle) error
	// compileV128Swizzle adds instructions to perform wazeroir.OperationV128Swizzle.
	compileV128Swizzle(o wazeroir.OperationV128Swizzle) error
	// compileV128AnyTrue adds instructions to perform wazeroir.OperationV128AnyTrue.
	compileV128AnyTrue(o wazeroir.OperationV128AnyTrue) error
	// compileV128AllTrue adds instructions to perform wazeroir.OperationV128AllTrue.
	compileV128AllTrue(o wazeroir.OperationV128AllTrue) error
	// compileV128BitMask adds instructions to perform wazeroir.OperationV128BitMask.
	compileV128BitMask(wazeroir.OperationV128BitMask) error
	// compileV128And adds instructions to perform wazeroir.OperationV128And.
	compileV128And(wazeroir.OperationV128And) error
	// compileV128Not adds instructions to perform wazeroir.OperationV128Not.
	compileV128Not(wazeroir.OperationV128Not) error
	// compileV128Or adds instructions to perform wazeroir.OperationV128Or.
	compileV128Or(wazeroir.OperationV128Or) error
	// compileV128Xor adds instructions to perform wazeroir.OperationV128Xor.
	compileV128Xor(wazeroir.OperationV128Xor) error
	// compileV128Bitselect adds instructions to perform wazeroir.OperationV128Bitselect.
	compileV128Bitselect(wazeroir.OperationV128Bitselect) error
	// compileV128AndNot adds instructions to perform wazeroir.OperationV128AndNot.
	compileV128AndNot(wazeroir.OperationV128AndNot) error
	// compileV128Shr adds instructions to perform wazeroir.OperationV128Shr.
	compileV128Shr(wazeroir.OperationV128Shr) error
	// compileV128Shl adds instructions to perform wazeroir.OperationV128Shl.
	compileV128Shl(wazeroir.OperationV128Shl) error
	// compileV128Cmp adds instructions to perform wazeroir.OperationV128Cmp.
	compileV128Cmp(wazeroir.OperationV128Cmp) error
	// compileV128AddSat adds instructions to perform wazeroir.OperationV128AddSat.
	compileV128AddSat(wazeroir.OperationV128AddSat) error
	// compileV128SubSat adds instructions to perform wazeroir.OperationV128SubSat.
	compileV128SubSat(wazeroir.OperationV128SubSat) error
	// compileV128Mul adds instructions to perform wazeroir.OperationV128Mul.
	compileV128Mul(wazeroir.OperationV128Mul) error
	// compileV128Div adds instructions to perform wazeroir.OperationV128Div.
	compileV128Div(wazeroir.OperationV128Div) error
	// compileV128Neg adds instructions to perform wazeroir.OperationV128Neg.
	compileV128Neg(wazeroir.OperationV128Neg) error
	// compileV128Sqrt adds instructions to perform wazeroir.OperationV128Sqrt.
	compileV128Sqrt(wazeroir.OperationV128Sqrt) error
	// compileV128Abs adds instructions to perform wazeroir.OperationV128Abs.
	compileV128Abs(wazeroir.OperationV128Abs) error
	// compileV128Popcnt adds instructions to perform wazeroir.OperationV128Popcnt.
	compileV128Popcnt(wazeroir.OperationV128Popcnt) error
	// compileV128Min adds instructions to perform wazeroir.OperationV128Min.
	compileV128Min(wazeroir.OperationV128Min) error
	// compileV128Max adds instructions to perform wazeroir.OperationV128Max.
	compileV128Max(wazeroir.OperationV128Max) error
	// compileV128AvgrU adds instructions to perform wazeroir.OperationV128AvgrU.
	compileV128AvgrU(wazeroir.OperationV128AvgrU) error
	// compileV128Pmin adds instructions to perform wazeroir.OperationV128Pmin.
	compileV128Pmin(wazeroir.OperationV128Pmin) error
	// compileV128Pmax adds instructions to perform wazeroir.OperationV128Pmax.
	compileV128Pmax(wazeroir.OperationV128Pmax) error
	// compileV128Ceil adds instructions to perform wazeroir.OperationV128Ceil.
	compileV128Ceil(wazeroir.OperationV128Ceil) error
	// compileV128Floor adds instructions to perform wazeroir.OperationV128Floor.
	compileV128Floor(wazeroir.OperationV128Floor) error
	// compileV128Trunc adds instructions to perform wazeroir.OperationV128Trunc.
	compileV128Trunc(wazeroir.OperationV128Trunc) error
	// compileV128Nearest adds instructions to perform wazeroir.OperationV128Nearest.
	compileV128Nearest(wazeroir.OperationV128Nearest) error
	// compileV128Extend adds instructions to perform wazeroir.OperationV128Extend.
	compileV128Extend(wazeroir.OperationV128Extend) error
	// compileV128ExtMul adds instructions to perform wazeroir.OperationV128ExtMul.
	compileV128ExtMul(wazeroir.OperationV128ExtMul) error
	// compileV128Q15mulrSatS adds instructions to perform wazeroir.OperationV128Q15mulrSatS.
	compileV128Q15mulrSatS(wazeroir.OperationV128Q15mulrSatS) error
	// compileV128ExtAddPairwise adds instructions to perform wazeroir.OperationV128ExtAddPairwise.
	compileV128ExtAddPairwise(o wazeroir.OperationV128ExtAddPairwise) error
	// compileV128FloatPromote adds instructions to perform wazeroir.OperationV128FloatPromote.
	compileV128FloatPromote(o wazeroir.OperationV128FloatPromote) error
	// compileV128FloatDemote adds instructions to perform wazeroir.OperationV128FloatDemote.
	compileV128FloatDemote(o wazeroir.OperationV128FloatDemote) error
	// compileV128FConvertFromI adds instructions to perform wazeroir.OperationV128FConvertFromI.
	compileV128FConvertFromI(o wazeroir.OperationV128FConvertFromI) error
	// compileV128Dot adds instructions to perform wazeroir.OperationV128Dot.
	compileV128Dot(o wazeroir.OperationV128Dot) error
	// compileV128Narrow adds instructions to perform wazeroir.OperationV128Narrow.
	compileV128Narrow(o wazeroir.OperationV128Narrow) error
	// compileV128ITruncSatFromF adds instructions to perform wazeroir.OperationV128ITruncSatFromF.
	compileV128ITruncSatFromF(o wazeroir.OperationV128ITruncSatFromF) error

	// compileBuiltinFunctionCheckExitCode adds instructions to perform wazeroir.OperationBuiltinFunctionCheckExitCode.
	compileBuiltinFunctionCheckExitCode() error

	// compileReleaseRegisterToStack adds instructions to write the value on a register back to memory stack region.
	compileReleaseRegisterToStack(loc *runtimeValueLocation)
	// compileLoadValueOnStackToRegister adds instructions to load the value located on the stack to the assigned register.
	compileLoadValueOnStackToRegister(loc *runtimeValueLocation)

	// maybeCompileMoveTopConditionalToGeneralPurposeRegister moves the top value on the stack
	// if the value is located on a conditional register.
	//
	// This is usually called at the beginning of methods on compiler interface where we possibly
	// compile instructions without saving the conditional register value.
	// The compileXXX functions without calling this function is saving the conditional
	// value to the stack or register by invoking compileEnsureOnRegister for the top.
	maybeCompileMoveTopConditionalToGeneralPurposeRegister() error
	// allocateRegister returns an unused register of the given type. The register will be taken
	// either from the free register pool or by stealing a used register.
	//
	// Note: resulting registers will not be marked as used so the call site should
	// mark it used if necessary.
	allocateRegister(t registerType) (reg asm.Register, err error)
	// runtimeValueLocationStack returns the current runtimeValueLocationStack of the compiler implementation.
	runtimeValueLocationStack() *runtimeValueLocationStack
	// pushRuntimeValueLocationOnRegister pushes a new runtimeValueLocation on a register `reg` and of the type `vt`.
	pushRuntimeValueLocationOnRegister(reg asm.Register, vt runtimeValueType) (ret *runtimeValueLocation)
	// pushRuntimeValueLocationOnRegister pushes a new vector value's runtimeValueLocation on a register `reg`.
	pushVectorRuntimeValueLocationOnRegister(reg asm.Register) (lowerBitsLocation *runtimeValueLocation)
	// compileNOP compiles NOP instruction and returns the corresponding asm.Node in the assembled native code.
	// This is used to emit DWARF based stack traces.
	compileNOP() asm.Node
}
