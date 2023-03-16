package wazeroir

import (
	"fmt"
)

// UnsignedInt represents unsigned 32-bit or 64-bit integers.
type UnsignedInt byte

const (
	UnsignedInt32 UnsignedInt = iota
	UnsignedInt64
)

// String implements fmt.Stringer.
func (s UnsignedInt) String() (ret string) {
	switch s {
	case UnsignedInt32:
		ret = "i32"
	case UnsignedInt64:
		ret = "i64"
	}
	return
}

// SignedInt represents signed or unsigned integers.
type SignedInt byte

const (
	SignedInt32 SignedInt = iota
	SignedInt64
	SignedUint32
	SignedUint64
)

// String implements fmt.Stringer.
func (s SignedInt) String() (ret string) {
	switch s {
	case SignedUint32:
		ret = "u32"
	case SignedUint64:
		ret = "u64"
	case SignedInt32:
		ret = "s32"
	case SignedInt64:
		ret = "s64"
	}
	return
}

// Float represents the scalar double or single precision floating points.
type Float byte

const (
	Float32 Float = iota
	Float64
)

// String implements fmt.Stringer.
func (s Float) String() (ret string) {
	switch s {
	case Float32:
		ret = "f32"
	case Float64:
		ret = "f64"
	}
	return
}

// UnsignedType is the union of UnsignedInt, Float and V128 vector type.
type UnsignedType byte

const (
	UnsignedTypeI32 UnsignedType = iota
	UnsignedTypeI64
	UnsignedTypeF32
	UnsignedTypeF64
	UnsignedTypeV128
	UnsignedTypeUnknown
)

// String implements fmt.Stringer.
func (s UnsignedType) String() (ret string) {
	switch s {
	case UnsignedTypeI32:
		ret = "i32"
	case UnsignedTypeI64:
		ret = "i64"
	case UnsignedTypeF32:
		ret = "f32"
	case UnsignedTypeF64:
		ret = "f64"
	case UnsignedTypeV128:
		ret = "v128"
	case UnsignedTypeUnknown:
		ret = "unknown"
	}
	return
}

// SignedType is the union of SignedInt and Float types.
type SignedType byte

const (
	SignedTypeInt32 SignedType = iota
	SignedTypeUint32
	SignedTypeInt64
	SignedTypeUint64
	SignedTypeFloat32
	SignedTypeFloat64
)

// String implements fmt.Stringer.
func (s SignedType) String() (ret string) {
	switch s {
	case SignedTypeInt32:
		ret = "s32"
	case SignedTypeUint32:
		ret = "u32"
	case SignedTypeInt64:
		ret = "s64"
	case SignedTypeUint64:
		ret = "u64"
	case SignedTypeFloat32:
		ret = "f32"
	case SignedTypeFloat64:
		ret = "f64"
	}
	return
}

// Operation is the interface implemented by each individual operation.
type Operation interface {
	// Kind returns the kind of the implementation.
	Kind() OperationKind
	// TODO String()
}

// OperationKind is the kind of each implementation of Operation interface.
type OperationKind uint16

// String implements fmt.Stringer.
func (o OperationKind) String() (ret string) {
	switch o {
	case OperationKindUnreachable:
		ret = "Unreachable"
	case OperationKindLabel:
		ret = "Label"
	case OperationKindBr:
		ret = "Br"
	case OperationKindBrIf:
		ret = "BrIf"
	case OperationKindBrTable:
		ret = "BrTable"
	case OperationKindCall:
		ret = "Call"
	case OperationKindCallIndirect:
		ret = "CallIndirect"
	case OperationKindDrop:
		ret = "Drop"
	case OperationKindSelect:
		ret = "Select"
	case OperationKindPick:
		ret = "Pick"
	case OperationKindSet:
		ret = "Swap"
	case OperationKindGlobalGet:
		ret = "GlobalGet"
	case OperationKindGlobalSet:
		ret = "GlobalSet"
	case OperationKindLoad:
		ret = "Load"
	case OperationKindLoad8:
		ret = "Load8"
	case OperationKindLoad16:
		ret = "Load16"
	case OperationKindLoad32:
		ret = "Load32"
	case OperationKindStore:
		ret = "Store"
	case OperationKindStore8:
		ret = "Store8"
	case OperationKindStore16:
		ret = "Store16"
	case OperationKindStore32:
		ret = "Store32"
	case OperationKindMemorySize:
		ret = "MemorySize"
	case OperationKindMemoryGrow:
		ret = "MemoryGrow"
	case OperationKindConstI32:
		ret = "ConstI32"
	case OperationKindConstI64:
		ret = "ConstI64"
	case OperationKindConstF32:
		ret = "ConstF32"
	case OperationKindConstF64:
		ret = "ConstF64"
	case OperationKindEq:
		ret = "Eq"
	case OperationKindNe:
		ret = "Ne"
	case OperationKindEqz:
		ret = "Eqz"
	case OperationKindLt:
		ret = "Lt"
	case OperationKindGt:
		ret = "Gt"
	case OperationKindLe:
		ret = "Le"
	case OperationKindGe:
		ret = "Ge"
	case OperationKindAdd:
		ret = "Add"
	case OperationKindSub:
		ret = "Sub"
	case OperationKindMul:
		ret = "Mul"
	case OperationKindClz:
		ret = "Clz"
	case OperationKindCtz:
		ret = "Ctz"
	case OperationKindPopcnt:
		ret = "Popcnt"
	case OperationKindDiv:
		ret = "Div"
	case OperationKindRem:
		ret = "Rem"
	case OperationKindAnd:
		ret = "And"
	case OperationKindOr:
		ret = "Or"
	case OperationKindXor:
		ret = "Xor"
	case OperationKindShl:
		ret = "Shl"
	case OperationKindShr:
		ret = "Shr"
	case OperationKindRotl:
		ret = "Rotl"
	case OperationKindRotr:
		ret = "Rotr"
	case OperationKindAbs:
		ret = "Abs"
	case OperationKindNeg:
		ret = "Neg"
	case OperationKindCeil:
		ret = "Ceil"
	case OperationKindFloor:
		ret = "Floor"
	case OperationKindTrunc:
		ret = "Trunc"
	case OperationKindNearest:
		ret = "Nearest"
	case OperationKindSqrt:
		ret = "Sqrt"
	case OperationKindMin:
		ret = "Min"
	case OperationKindMax:
		ret = "Max"
	case OperationKindCopysign:
		ret = "Copysign"
	case OperationKindI32WrapFromI64:
		ret = "I32WrapFromI64"
	case OperationKindITruncFromF:
		ret = "ITruncFromF"
	case OperationKindFConvertFromI:
		ret = "FConvertFromI"
	case OperationKindF32DemoteFromF64:
		ret = "F32DemoteFromF64"
	case OperationKindF64PromoteFromF32:
		ret = "F64PromoteFromF32"
	case OperationKindI32ReinterpretFromF32:
		ret = "I32ReinterpretFromF32"
	case OperationKindI64ReinterpretFromF64:
		ret = "I64ReinterpretFromF64"
	case OperationKindF32ReinterpretFromI32:
		ret = "F32ReinterpretFromI32"
	case OperationKindF64ReinterpretFromI64:
		ret = "F64ReinterpretFromI64"
	case OperationKindExtend:
		ret = "Extend"
	case OperationKindMemoryInit:
		ret = "MemoryInit"
	case OperationKindDataDrop:
		ret = "DataDrop"
	case OperationKindMemoryCopy:
		ret = "MemoryCopy"
	case OperationKindMemoryFill:
		ret = "MemoryFill"
	case OperationKindTableInit:
		ret = "TableInit"
	case OperationKindElemDrop:
		ret = "ElemDrop"
	case OperationKindTableCopy:
		ret = "TableCopy"
	case OperationKindRefFunc:
		ret = "RefFunc"
	case OperationKindTableGet:
		ret = "TableGet"
	case OperationKindTableSet:
		ret = "TableSet"
	case OperationKindTableSize:
		ret = "TableSize"
	case OperationKindTableGrow:
		ret = "TableGrow"
	case OperationKindTableFill:
		ret = "TableFill"
	case OperationKindV128Const:
		ret = "ConstV128"
	case OperationKindV128Add:
		ret = "V128Add"
	case OperationKindV128Sub:
		ret = "V128Sub"
	case OperationKindV128Load:
		ret = "V128Load"
	case OperationKindV128LoadLane:
		ret = "V128LoadLane"
	case OperationKindV128Store:
		ret = "V128Store"
	case OperationKindV128StoreLane:
		ret = "V128StoreLane"
	case OperationKindV128ExtractLane:
		ret = "V128ExtractLane"
	case OperationKindV128ReplaceLane:
		ret = "V128ReplaceLane"
	case OperationKindV128Splat:
		ret = "V128Splat"
	case OperationKindV128Shuffle:
		ret = "V128Shuffle"
	case OperationKindV128Swizzle:
		ret = "V128Swizzle"
	case OperationKindV128AnyTrue:
		ret = "V128AnyTrue"
	case OperationKindV128AllTrue:
		ret = "V128AllTrue"
	case OperationKindV128And:
		ret = "V128And"
	case OperationKindV128Not:
		ret = "V128Not"
	case OperationKindV128Or:
		ret = "V128Or"
	case OperationKindV128Xor:
		ret = "V128Xor"
	case OperationKindV128Bitselect:
		ret = "V128Bitselect"
	case OperationKindV128AndNot:
		ret = "V128AndNot"
	case OperationKindV128BitMask:
		ret = "V128BitMask"
	case OperationKindV128Shl:
		ret = "V128Shl"
	case OperationKindV128Shr:
		ret = "V128Shr"
	case OperationKindV128Cmp:
		ret = "V128Cmp"
	case OperationKindSignExtend32From8:
		ret = "SignExtend32From8"
	case OperationKindSignExtend32From16:
		ret = "SignExtend32From16"
	case OperationKindSignExtend64From8:
		ret = "SignExtend64From8"
	case OperationKindSignExtend64From16:
		ret = "SignExtend64From16"
	case OperationKindSignExtend64From32:
		ret = "SignExtend64From32"
	case OperationKindV128AddSat:
		ret = "V128AddSat"
	case OperationKindV128SubSat:
		ret = "V128SubSat"
	case OperationKindV128Mul:
		ret = "V128Mul"
	case OperationKindV128Div:
		ret = "V128Div"
	case OperationKindV128Neg:
		ret = "V128Neg"
	case OperationKindV128Sqrt:
		ret = "V128Sqrt"
	case OperationKindV128Abs:
		ret = "V128Abs"
	case OperationKindV128Popcnt:
		ret = "V128Popcnt"
	case OperationKindV128Min:
		ret = "V128Min"
	case OperationKindV128Max:
		ret = "V128Max"
	case OperationKindV128AvgrU:
		ret = "V128AvgrU"
	case OperationKindV128Ceil:
		ret = "V128Ceil"
	case OperationKindV128Floor:
		ret = "V128Floor"
	case OperationKindV128Trunc:
		ret = "V128Trunc"
	case OperationKindV128Nearest:
		ret = "V128Nearest"
	case OperationKindV128Pmin:
		ret = "V128Pmin"
	case OperationKindV128Pmax:
		ret = "V128Pmax"
	case OperationKindV128Extend:
		ret = "V128Extend"
	case OperationKindV128ExtMul:
		ret = "V128ExtMul"
	case OperationKindV128Q15mulrSatS:
		ret = "V128Q15mulrSatS"
	case OperationKindV128ExtAddPairwise:
		ret = "V128ExtAddPairwise"
	case OperationKindV128FloatPromote:
		ret = "V128FloatPromote"
	case OperationKindV128FloatDemote:
		ret = "V128FloatDemote"
	case OperationKindV128FConvertFromI:
		ret = "V128FConvertFromI"
	case OperationKindV128Dot:
		ret = "V128Dot"
	case OperationKindV128Narrow:
		ret = "V128Narrow"
	case OperationKindV128ITruncSatFromF:
		ret = "V128ITruncSatFromF"
	case OperationKindBuiltinFunctionCheckExitCode:
		ret = "BuiltinFunctionCheckExitCode"
	default:
		panic(fmt.Errorf("unknown operation %d", o))
	}
	return
}

const (
	// OperationKindUnreachable is the kind for OperationUnreachable.
	OperationKindUnreachable OperationKind = iota
	// OperationKindLabel is the kind for OperationLabel.
	OperationKindLabel
	// OperationKindBr is the kind for OperationBr.
	OperationKindBr
	// OperationKindBrIf is the kind for OperationBrIf.
	OperationKindBrIf
	// OperationKindBrTable is the kind for OperationBrTable.
	OperationKindBrTable
	// OperationKindCall is the kind for OperationCall.
	OperationKindCall
	// OperationKindCallIndirect is the kind for OperationCallIndirect.
	OperationKindCallIndirect
	// OperationKindDrop is the kind for OperationDrop.
	OperationKindDrop
	// OperationKindSelect is the kind for OperationSelect.
	OperationKindSelect
	// OperationKindPick is the kind for OperationPick.
	OperationKindPick
	// OperationKindSet is the kind for OperationSet.
	OperationKindSet
	// OperationKindGlobalGet is the kind for OperationGlobalGet.
	OperationKindGlobalGet
	// OperationKindGlobalSet is the kind for OperationGlobalSet.
	OperationKindGlobalSet
	// OperationKindLoad is the kind for OperationLoad.
	OperationKindLoad
	// OperationKindLoad8 is the kind for OperationLoad8.
	OperationKindLoad8
	// OperationKindLoad16 is the kind for OperationLoad16.
	OperationKindLoad16
	// OperationKindLoad32 is the kind for OperationLoad32.
	OperationKindLoad32
	// OperationKindStore is the kind for OperationStore.
	OperationKindStore
	// OperationKindStore8 is the kind for OperationStore8.
	OperationKindStore8
	// OperationKindStore16 is the kind for OperationStore16.
	OperationKindStore16
	// OperationKindStore32 is the kind for OperationStore32.
	OperationKindStore32
	// OperationKindMemorySize is the kind for OperationMemorySize.
	OperationKindMemorySize
	// OperationKindMemoryGrow is the kind for OperationMemoryGrow.
	OperationKindMemoryGrow
	// OperationKindConstI32 is the kind for OperationConstI32.
	OperationKindConstI32
	// OperationKindConstI64 is the kind for OperationConstI64.
	OperationKindConstI64
	// OperationKindConstF32 is the kind for OperationConstF32.
	OperationKindConstF32
	// OperationKindConstF64 is the kind for OperationConstF64.
	OperationKindConstF64
	// OperationKindEq is the kind for OperationEq.
	OperationKindEq
	// OperationKindNe is the kind for OperationNe.
	OperationKindNe
	// OperationKindEqz is the kind for OperationEqz.
	OperationKindEqz
	// OperationKindLt is the kind for OperationLt.
	OperationKindLt
	// OperationKindGt is the kind for OperationGt.
	OperationKindGt
	// OperationKindLe is the kind for OperationLe.
	OperationKindLe
	// OperationKindGe is the kind for OperationGe.
	OperationKindGe
	// OperationKindAdd is the kind for OperationAdd.
	OperationKindAdd
	// OperationKindSub is the kind for OperationSub.
	OperationKindSub
	// OperationKindMul is the kind for OperationMul.
	OperationKindMul
	// OperationKindClz is the kind for OperationClz.
	OperationKindClz
	// OperationKindCtz is the kind for OperationCtz.
	OperationKindCtz
	// OperationKindPopcnt is the kind for OperationPopcnt.
	OperationKindPopcnt
	// OperationKindDiv is the kind for OperationDiv.
	OperationKindDiv
	// OperationKindRem is the kind for OperationRem.
	OperationKindRem
	// OperationKindAnd is the kind for OperationAnd.
	OperationKindAnd
	// OperationKindOr is the kind for OperationOr.
	OperationKindOr
	// OperationKindXor is the kind for OperationXor.
	OperationKindXor
	// OperationKindShl is the kind for OperationShl.
	OperationKindShl
	// OperationKindShr is the kind for OperationShr.
	OperationKindShr
	// OperationKindRotl is the kind for OperationRotl.
	OperationKindRotl
	// OperationKindRotr is the kind for OperationRotr.
	OperationKindRotr
	// OperationKindAbs is the kind for OperationAbs.
	OperationKindAbs
	// OperationKindNeg is the kind for OperationNeg.
	OperationKindNeg
	// OperationKindCeil is the kind for OperationCeil.
	OperationKindCeil
	// OperationKindFloor is the kind for OperationFloor.
	OperationKindFloor
	// OperationKindTrunc is the kind for OperationTrunc.
	OperationKindTrunc
	// OperationKindNearest is the kind for OperationNearest.
	OperationKindNearest
	// OperationKindSqrt is the kind for OperationSqrt.
	OperationKindSqrt
	// OperationKindMin is the kind for OperationMin.
	OperationKindMin
	// OperationKindMax is the kind for OperationMax.
	OperationKindMax
	// OperationKindCopysign is the kind for OperationCopysign.
	OperationKindCopysign
	// OperationKindI32WrapFromI64 is the kind for OperationI32WrapFromI64.
	OperationKindI32WrapFromI64
	// OperationKindITruncFromF is the kind for OperationITruncFromF.
	OperationKindITruncFromF
	// OperationKindFConvertFromI is the kind for OperationFConvertFromI.
	OperationKindFConvertFromI
	// OperationKindF32DemoteFromF64 is the kind for OperationF32DemoteFromF64.
	OperationKindF32DemoteFromF64
	// OperationKindF64PromoteFromF32 is the kind for OperationF64PromoteFromF32.
	OperationKindF64PromoteFromF32
	// OperationKindI32ReinterpretFromF32 is the kind for OperationI32ReinterpretFromF32.
	OperationKindI32ReinterpretFromF32
	// OperationKindI64ReinterpretFromF64 is the kind for OperationI64ReinterpretFromF64.
	OperationKindI64ReinterpretFromF64
	// OperationKindF32ReinterpretFromI32 is the kind for OperationF32ReinterpretFromI32.
	OperationKindF32ReinterpretFromI32
	// OperationKindF64ReinterpretFromI64 is the kind for OperationF64ReinterpretFromI64.
	OperationKindF64ReinterpretFromI64
	// OperationKindExtend is the kind for OperationExtend.
	OperationKindExtend
	// OperationKindSignExtend32From8 is the kind for OperationSignExtend32From8.
	OperationKindSignExtend32From8
	// OperationKindSignExtend32From16 is the kind for OperationSignExtend32From16.
	OperationKindSignExtend32From16
	// OperationKindSignExtend64From8 is the kind for OperationSignExtend64From8.
	OperationKindSignExtend64From8
	// OperationKindSignExtend64From16 is the kind for OperationSignExtend64From16.
	OperationKindSignExtend64From16
	// OperationKindSignExtend64From32 is the kind for OperationSignExtend64From32.
	OperationKindSignExtend64From32
	// OperationKindMemoryInit is the kind for OperationMemoryInit.
	OperationKindMemoryInit
	// OperationKindDataDrop is the kind for OperationDataDrop.
	OperationKindDataDrop
	// OperationKindMemoryCopy is the kind for OperationMemoryCopy.
	OperationKindMemoryCopy
	// OperationKindMemoryFill is the kind for OperationMemoryFill.
	OperationKindMemoryFill
	// OperationKindTableInit is the kind for OperationTableInit.
	OperationKindTableInit
	// OperationKindElemDrop is the kind for OperationElemDrop.
	OperationKindElemDrop
	// OperationKindTableCopy is the kind for OperationTableCopy.
	OperationKindTableCopy
	// OperationKindRefFunc is the kind for OperationRefFunc.
	OperationKindRefFunc
	// OperationKindTableGet is the kind for OperationTableGet.
	OperationKindTableGet
	// OperationKindTableSet is the kind for OperationTableSet.
	OperationKindTableSet
	// OperationKindTableSize is the kind for OperationTableSize.
	OperationKindTableSize
	// OperationKindTableGrow is the kind for OperationTableGrow.
	OperationKindTableGrow
	// OperationKindTableFill is the kind for OperationTableFill.
	OperationKindTableFill

	// Vector value related instructions are prefixed by V128.

	// OperationKindV128Const is the kind for OperationV128Const.
	OperationKindV128Const
	// OperationKindV128Add is the kind for OperationV128Add.
	OperationKindV128Add
	// OperationKindV128Sub is the kind for OperationV128Sub.
	OperationKindV128Sub
	// OperationKindV128Load is the kind for OperationV128Load.
	OperationKindV128Load
	// OperationKindV128LoadLane is the kind for OperationV128LoadLane.
	OperationKindV128LoadLane
	// OperationKindV128Store is the kind for OperationV128Store.
	OperationKindV128Store
	// OperationKindV128StoreLane is the kind for OperationV128StoreLane.
	OperationKindV128StoreLane
	// OperationKindV128ExtractLane is the kind for OperationV128ExtractLane.
	OperationKindV128ExtractLane
	// OperationKindV128ReplaceLane is the kind for OperationV128ReplaceLane.
	OperationKindV128ReplaceLane
	// OperationKindV128Splat is the kind for OperationV128Splat.
	OperationKindV128Splat
	// OperationKindV128Shuffle is the kind for OperationV128Shuffle.
	OperationKindV128Shuffle
	// OperationKindV128Swizzle is the kind for OperationV128Swizzle.
	OperationKindV128Swizzle
	// OperationKindV128AnyTrue is the kind for OperationV128AnyTrue.
	OperationKindV128AnyTrue
	// OperationKindV128AllTrue is the kind for OperationV128AllTrue.
	OperationKindV128AllTrue
	// OperationKindV128BitMask is the kind for OperationV128BitMask.
	OperationKindV128BitMask
	// OperationKindV128And is the kind for OperationV128And.
	OperationKindV128And
	// OperationKindV128Not is the kind for OperationV128Not.
	OperationKindV128Not
	// OperationKindV128Or is the kind for OperationV128Or.
	OperationKindV128Or
	// OperationKindV128Xor is the kind for OperationV128Xor.
	OperationKindV128Xor
	// OperationKindV128Bitselect is the kind for OperationV128Bitselect.
	OperationKindV128Bitselect
	// OperationKindV128AndNot is the kind for OperationV128AndNot.
	OperationKindV128AndNot
	// OperationKindV128Shl is the kind for OperationV128Shl.
	OperationKindV128Shl
	// OperationKindV128Shr is the kind for OperationV128Shr.
	OperationKindV128Shr
	// OperationKindV128Cmp is the kind for OperationV128Cmp.
	OperationKindV128Cmp
	// OperationKindV128AddSat is the kind for OperationV128AddSat.
	OperationKindV128AddSat
	// OperationKindV128SubSat is the kind for OperationV128SubSat.
	OperationKindV128SubSat
	// OperationKindV128Mul is the kind for OperationV128Mul.
	OperationKindV128Mul
	// OperationKindV128Div is the kind for OperationV128Div.
	OperationKindV128Div
	// OperationKindV128Neg is the kind for OperationV128Neg.
	OperationKindV128Neg
	// OperationKindV128Sqrt is the kind for OperationV128Sqrt.
	OperationKindV128Sqrt
	// OperationKindV128Abs is the kind for OperationV128Abs.
	OperationKindV128Abs
	// OperationKindV128Popcnt is the kind for OperationV128Popcnt.
	OperationKindV128Popcnt
	// OperationKindV128Min is the kind for OperationV128Min.
	OperationKindV128Min
	// OperationKindV128Max is the kind for OperationV128Max.
	OperationKindV128Max
	// OperationKindV128AvgrU is the kind for OperationV128AvgrU.
	OperationKindV128AvgrU
	// OperationKindV128Pmin is the kind for OperationV128Pmin.
	OperationKindV128Pmin
	// OperationKindV128Pmax is the kind for OperationV128Pmax.
	OperationKindV128Pmax
	// OperationKindV128Ceil is the kind for OperationV128Ceil.
	OperationKindV128Ceil
	// OperationKindV128Floor is the kind for OperationV128Floor.
	OperationKindV128Floor
	// OperationKindV128Trunc is the kind for OperationV128Trunc.
	OperationKindV128Trunc
	// OperationKindV128Nearest is the kind for OperationV128Nearest.
	OperationKindV128Nearest
	// OperationKindV128Extend is the kind for OperationV128Extend.
	OperationKindV128Extend
	// OperationKindV128ExtMul is the kind for OperationV128ExtMul.
	OperationKindV128ExtMul
	// OperationKindV128Q15mulrSatS is the kind for OperationV128Q15mulrSatS.
	OperationKindV128Q15mulrSatS
	// OperationKindV128ExtAddPairwise is the kind for OperationV128ExtAddPairwise.
	OperationKindV128ExtAddPairwise
	// OperationKindV128FloatPromote is the kind for OperationV128FloatPromote.
	OperationKindV128FloatPromote
	// OperationKindV128FloatDemote is the kind for OperationV128FloatDemote.
	OperationKindV128FloatDemote
	// OperationKindV128FConvertFromI is the kind for OperationV128FConvertFromI.
	OperationKindV128FConvertFromI
	// OperationKindV128Dot is the kind for OperationV128Dot.
	OperationKindV128Dot
	// OperationKindV128Narrow is the kind for OperationV128Narrow.
	OperationKindV128Narrow
	// OperationKindV128ITruncSatFromF is the kind for OperationV128ITruncSatFromF.
	OperationKindV128ITruncSatFromF

	// OperationKindBuiltinFunctionCheckExitCode is the kind for OperationBuiltinFunctionCheckExitCode.
	OperationKindBuiltinFunctionCheckExitCode

	// operationKindEnd is always placed at the bottom of this iota definition to be used in the test.
	operationKindEnd
)

// OperationBuiltinFunctionCheckExitCode implements Operation.
//
// OperationBuiltinFunctionCheckExitCode corresponds to the instruction to check the api.Module is already closed due to
// context.DeadlineExceeded, context.Canceled, or the explicit call of CloseWithExitCode on api.Module.
type OperationBuiltinFunctionCheckExitCode struct{}

// Kind implements Operation.Kind
func (OperationBuiltinFunctionCheckExitCode) Kind() OperationKind {
	return OperationKindBuiltinFunctionCheckExitCode
}

// Label is the label of each block in wazeroir where "block" consists of multiple operations,
// and must end with branching operations (e.g. OperationBr or OperationBrIf).
type Label struct {
	FrameID uint32
	Kind    LabelKind
}

// LabelID is the unique identifiers for blocks in a single function.
type LabelID uint64

// ID returns the LabelID for this Label.
func (l Label) ID() (id LabelID) {
	id = LabelID(l.Kind) | LabelID(l.FrameID)<<32
	return
}

// String implements fmt.Stringer.
func (l Label) String() (ret string) {
	switch l.Kind {
	case LabelKindHeader:
		ret = fmt.Sprintf(".L%d", l.FrameID)
	case LabelKindElse:
		ret = fmt.Sprintf(".L%d_else", l.FrameID)
	case LabelKindContinuation:
		ret = fmt.Sprintf(".L%d_cont", l.FrameID)
	case LabelKindReturn:
		return ".return"
	}
	return
}

func (l Label) IsReturnTarget() bool {
	return l.Kind == LabelKindReturn
}

// LabelKind is the kind of the label.
type LabelKind = byte

const (
	// LabelKindHeader is the header for various blocks. For example, the "then" block of
	// wasm.OpcodeIfName in Wasm has the label of this kind.
	LabelKindHeader LabelKind = iota
	// LabelKindElse is the kind of label for "else" block of wasm.OpcodeIfName in Wasm.
	LabelKindElse
	// LabelKindContinuation is the kind of label which is the continuation of blocks.
	// For example, for wasm text like
	// (func
	//   ....
	//   (if (local.get 0) (then (nop)) (else (nop)))
	//   return
	// )
	// we have the continuation block (of if-block) corresponding to "return" opcode.
	LabelKindContinuation
	LabelKindReturn
)

func (l Label) asBranchTargetDrop() BranchTargetDrop {
	return BranchTargetDrop{Target: l}
}

// BranchTargetDrop represents the branch target and the drop range which must be dropped
// before give the control over to the target label.
type BranchTargetDrop struct {
	Target Label
	ToDrop *InclusiveRange
}

// String implements fmt.Stringer.
func (b BranchTargetDrop) String() (ret string) {
	if b.ToDrop != nil {
		ret = fmt.Sprintf("%s(drop %d..%d)", b.Target, b.ToDrop.Start, b.ToDrop.End)
	} else {
		ret = b.Target.String()
	}
	return
}

// OperationUnreachable implements Operation.
//
// This corresponds to wasm.OpcodeUnreachable.
//
// The engines are expected to exit the execution with wasmruntime.ErrRuntimeUnreachable error.
type OperationUnreachable struct{}

// Kind implements Operation.Kind
func (OperationUnreachable) Kind() OperationKind {
	return OperationKindUnreachable
}

// OperationLabel implements Operation.
//
// This is used to inform the engines of the beginning of a label.
type OperationLabel struct {
	Label Label
}

// Kind implements Operation.Kind
func (OperationLabel) Kind() OperationKind {
	return OperationKindLabel
}

// OperationBr implements Operation.
//
// The engines are expected to branch into OperationBr.Target label.
type OperationBr struct {
	Target Label
}

// Kind implements Operation.Kind
func (OperationBr) Kind() OperationKind {
	return OperationKindBr
}

// OperationBrIf implements Operation.
//
// The engines are expected to pop a value and branch into OperationBrIf.Then label if the value equals 1.
// Otherwise, the code branches into OperationBrIf.Else label.
type OperationBrIf struct {
	Then, Else BranchTargetDrop
}

// Kind implements Operation.Kind
func (OperationBrIf) Kind() OperationKind {
	return OperationKindBrIf
}

// OperationBrTable implements Operation.
//
// This corresponds to wasm.OpcodeBrTableName except that the label
// here means the wazeroir level, not the ones of Wasm.
//
// The engines are expected to do the br_table operation base on the
// OperationBrTable.Default and OperationBrTable.Targets. More precisely,
// this pops a value from the stack (called "index") and decide which branch we go into next
// based on the value.
//
// For example, assume we have operations like {default: L_DEFAULT, targets: [L0, L1, L2]}.
// If "index" >= len(defaults), then branch into the L_DEFAULT label.
// Otherwise, we enter label of targets[index].
type OperationBrTable struct {
	Targets []*BranchTargetDrop
	Default *BranchTargetDrop
}

// Kind implements Operation.Kind
func (OperationBrTable) Kind() OperationKind {
	return OperationKindBrTable
}

// OperationCall implements Operation.
//
// This corresponds to wasm.OpcodeCallName, and engines are expected to
// enter into a function whose index equals OperationCall.FunctionIndex.
type OperationCall struct {
	FunctionIndex uint32
}

// Kind implements Operation.Kind
func (OperationCall) Kind() OperationKind {
	return OperationKindCall
}

// OperationCallIndirect implements Operation.
//
// This corresponds to wasm.OpcodeCallIndirectName, and engines are expected to
// consume the one value from the top of stack (called "offset"),
// and make a function call against the function whose function address equals
// Tables[OperationCallIndirect.TableIndex][offset].
//
// Note: This is called indirect function call in the sense that the target function is indirectly
// determined by the current state (top value) of the stack.
// Therefore, two checks are performed at runtime before entering the target function:
// 1) whether "offset" exceeds the length of table Tables[OperationCallIndirect.TableIndex].
// 2) whether the type of the function table[offset] matches the function type specified by OperationCallIndirect.TypeIndex.
type OperationCallIndirect struct {
	TypeIndex, TableIndex uint32
}

// Kind implements Operation.Kind
func (OperationCallIndirect) Kind() OperationKind {
	return OperationKindCallIndirect
}

// InclusiveRange is the range which spans across the value stack starting from the top to the bottom, and
// both boundary are included in the range.
type InclusiveRange struct {
	Start, End int
}

// OperationDrop implements Operation.
//
// The engines are expected to discard the values selected by OperationDrop.Depth which
// starts from the top of the stack to the bottom.
type OperationDrop struct {
	// Depths spans across the uint64 value stack at runtime to be dropped by this operation.
	Depth *InclusiveRange
}

// Kind implements Operation.Kind
func (OperationDrop) Kind() OperationKind {
	return OperationKindDrop
}

// OperationSelect implements Operation.
//
// This corresponds to wasm.OpcodeSelect.
//
// The engines are expected to pop three values, say [..., x2, x1, c], then if the value "c" equals zero,
// "x1" is pushed back onto the stack and, otherwise "x2" is pushed back.
type OperationSelect struct {
	// IsTargetVector true if the selection target value's type is wasm.ValueTypeV128.
	IsTargetVector bool
}

// Kind implements Operation.Kind
func (OperationSelect) Kind() OperationKind {
	return OperationKindSelect
}

// OperationPick implements Operation.
//
// The engines are expected to copy a value pointed by OperationPick.Depth, and push the
// copied value onto the top of the stack.
type OperationPick struct {
	// Depth is the location of the pick target in the uint64 value stack at runtime.
	// If IsTargetVector=true, this points to the location of the lower 64-bits of the vector.
	Depth          int
	IsTargetVector bool
}

// Kind implements Operation.Kind
func (OperationPick) Kind() OperationKind {
	return OperationKindPick
}

// OperationSet implements Operation.
//
// The engines are expected to set the top value of the stack to the location specified by
// OperationSet.Depth.
type OperationSet struct {
	// Depth is the location of the set target in the uint64 value stack at runtime.
	// If IsTargetVector=true, this points the location of the lower 64-bits of the vector.
	Depth          int
	IsTargetVector bool
}

// Kind implements Operation.Kind
func (OperationSet) Kind() OperationKind {
	return OperationKindSet
}

// OperationGlobalGet implements Operation.
//
// The engines are expected to read the global value specified by OperationGlobalGet.Index,
// and push the copy of the value onto the stack.
//
// See wasm.OpcodeGlobalGet.
type OperationGlobalGet struct{ Index uint32 }

// Kind implements Operation.Kind
func (OperationGlobalGet) Kind() OperationKind {
	return OperationKindGlobalGet
}

// OperationGlobalSet implements Operation.
//
// The engines are expected to consume the value from the top of the stack,
// and write the value into the global specified by OperationGlobalSet.Index.
//
// See wasm.OpcodeGlobalSet.
type OperationGlobalSet struct{ Index uint32 }

// Kind implements Operation.Kind
func (OperationGlobalSet) Kind() OperationKind {
	return OperationKindGlobalSet
}

// MemoryArg is the "memarg" to all memory instructions.
//
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#memory-instructions%E2%91%A0
type MemoryArg struct {
	// Alignment the expected alignment (expressed as the exponent of a power of 2). Default to the natural alignment.
	//
	// "Natural alignment" is defined here as the smallest power of two that can hold the size of the value type. Ex
	// wasm.ValueTypeI64 is encoded in 8 little-endian bytes. 2^3 = 8, so the natural alignment is three.
	Alignment uint32

	// Offset is the address offset added to the instruction's dynamic address operand, yielding a 33-bit effective
	// address that is the zero-based index at which the memory is accessed. Default to zero.
	Offset uint32
}

// OperationLoad implements Operation.
//
// This corresponds to wasm.OpcodeI32LoadName wasm.OpcodeI64LoadName wasm.OpcodeF32LoadName and wasm.OpcodeF64LoadName.
//
// The engines are expected to check the boundary of memory length, and exit the execution if this exceeds the boundary,
// otherwise load the corresponding value following the semantics of the corresponding WebAssembly instruction.
type OperationLoad struct {
	Type UnsignedType
	Arg  MemoryArg
}

// Kind implements Operation.Kind
func (OperationLoad) Kind() OperationKind {
	return OperationKindLoad
}

// OperationLoad8 implements Operation.
//
// This corresponds to wasm.OpcodeI32Load8SName wasm.OpcodeI32Load8UName wasm.OpcodeI64Load8SName wasm.OpcodeI64Load8UName.
//
// The engines are expected to check the boundary of memory length, and exit the execution if this exceeds the boundary,
// otherwise load the corresponding value following the semantics of the corresponding WebAssembly instruction.
type OperationLoad8 struct {
	Type SignedInt
	Arg  MemoryArg
}

// Kind implements Operation.Kind
func (OperationLoad8) Kind() OperationKind {
	return OperationKindLoad8
}

// OperationLoad16 implements Operation.
//
// This corresponds to wasm.OpcodeI32Load16SName wasm.OpcodeI32Load16UName wasm.OpcodeI64Load16SName wasm.OpcodeI64Load16UName.
//
// The engines are expected to check the boundary of memory length, and exit the execution if this exceeds the boundary,
// otherwise load the corresponding value following the semantics of the corresponding WebAssembly instruction.
type OperationLoad16 struct {
	Type SignedInt
	Arg  MemoryArg
}

// Kind implements Operation.Kind
func (OperationLoad16) Kind() OperationKind {
	return OperationKindLoad16
}

// OperationLoad32 implements Operation.
//
// This corresponds to wasm.OpcodeI64Load32SName wasm.OpcodeI64Load32UName.
//
// The engines are expected to check the boundary of memory length, and exit the execution if this exceeds the boundary,
// otherwise load the corresponding value following the semantics of the corresponding WebAssembly instruction.
type OperationLoad32 struct {
	Signed bool
	Arg    MemoryArg
}

// Kind implements Operation.Kind
func (OperationLoad32) Kind() OperationKind {
	return OperationKindLoad32
}

// OperationStore implements Operation.
//
// # This corresponds to wasm.OpcodeI32StoreName wasm.OpcodeI64StoreName wasm.OpcodeF32StoreName wasm.OpcodeF64StoreName
//
// The engines are expected to check the boundary of memory length, and exit the execution if this exceeds the boundary,
// otherwise store the corresponding value following the semantics of the corresponding WebAssembly instruction.
type OperationStore struct {
	Type UnsignedType
	Arg  MemoryArg
}

// Kind implements Operation.Kind
func (OperationStore) Kind() OperationKind {
	return OperationKindStore
}

// OperationStore8 implements Operation.
//
// # This corresponds to wasm.OpcodeI32Store8Name wasm.OpcodeI64Store8Name
//
// The engines are expected to check the boundary of memory length, and exit the execution if this exceeds the boundary,
// otherwise store the corresponding value following the semantics of the corresponding WebAssembly instruction.
type OperationStore8 struct {
	Arg MemoryArg
}

// Kind implements Operation.Kind
func (OperationStore8) Kind() OperationKind {
	return OperationKindStore8
}

// OperationStore16 implements Operation.
//
// # This corresponds to wasm.OpcodeI32Store16Name wasm.OpcodeI64Store16Name
//
// The engines are expected to check the boundary of memory length, and exit the execution if this exceeds the boundary,
// otherwise store the corresponding value following the semantics of the corresponding WebAssembly instruction.
type OperationStore16 struct {
	Arg MemoryArg
}

// Kind implements Operation.Kind
func (OperationStore16) Kind() OperationKind {
	return OperationKindStore16
}

// OperationStore32 implements Operation.
//
// # This corresponds to wasm.OpcodeI64Store32Name
//
// The engines are expected to check the boundary of memory length, and exit the execution if this exceeds the boundary,
// otherwise store the corresponding value following the semantics of the corresponding WebAssembly instruction.
type OperationStore32 struct {
	Arg MemoryArg
}

// Kind implements Operation.Kind.
func (OperationStore32) Kind() OperationKind {
	return OperationKindStore32
}

// OperationMemorySize implements Operation.
//
// This corresponds to wasm.OpcodeMemorySize.
//
// The engines are expected to push the current page size of the memory onto the stack.
type OperationMemorySize struct{}

// Kind implements Operation.Kind.
func (OperationMemorySize) Kind() OperationKind {
	return OperationKindMemorySize
}

// OperationMemoryGrow implements Operation.
type OperationMemoryGrow struct{ Alignment uint64 }

// Kind implements Operation.Kind.
//
// This corresponds to wasm.OpcodeMemoryGrow.
//
// The engines are expected to pop one value from the top of the stack, then
// execute wasm.MemoryInstance Grow with the value, and push the previous
// page size of the memory onto the stack.
func (OperationMemoryGrow) Kind() OperationKind {
	return OperationKindMemoryGrow
}

// OperationConstI32 implements Operation.
//
// This corresponds to wasm.OpcodeI32Const.
type OperationConstI32 struct{ Value uint32 }

// Kind implements Operation.Kind.
func (OperationConstI32) Kind() OperationKind {
	return OperationKindConstI32
}

// OperationConstI64 implements Operation.
//
// This corresponds to wasm.OpcodeI64Const.
type OperationConstI64 struct{ Value uint64 }

// Kind implements Operation.Kind.
func (OperationConstI64) Kind() OperationKind {
	return OperationKindConstI64
}

// OperationConstF32 implements Operation.
//
// This corresponds to wasm.OpcodeF32Const.
type OperationConstF32 struct{ Value float32 }

// Kind implements Operation.Kind.
func (OperationConstF32) Kind() OperationKind {
	return OperationKindConstF32
}

// OperationConstF64 implements Operation.
//
// This corresponds to wasm.OpcodeF64Const.
type OperationConstF64 struct{ Value float64 }

// Kind implements Operation.Kind.
func (OperationConstF64) Kind() OperationKind {
	return OperationKindConstF64
}

// OperationEq implements Operation.
//
// This corresponds to wasm.OpcodeI32EqName wasm.OpcodeI64EqName wasm.OpcodeF32EqName wasm.OpcodeF64EqName
type OperationEq struct{ Type UnsignedType }

// Kind implements Operation.Kind.
func (OperationEq) Kind() OperationKind {
	return OperationKindEq
}

// OperationNe implements Operation.
//
// This corresponds to wasm.OpcodeI32NeName wasm.OpcodeI64NeName wasm.OpcodeF32NeName wasm.OpcodeF64NeName
type OperationNe struct{ Type UnsignedType }

// Kind implements Operation.Kind.
func (OperationNe) Kind() OperationKind {
	return OperationKindNe
}

// OperationEqz implements Operation.
//
// This corresponds to wasm.OpcodeI32EqzName wasm.OpcodeI64EqzName
type OperationEqz struct{ Type UnsignedInt }

// Kind implements Operation.Kind.
func (OperationEqz) Kind() OperationKind {
	return OperationKindEqz
}

// OperationLt implements Operation.
//
// This corresponds to wasm.OpcodeI32LtS wasm.OpcodeI32LtU wasm.OpcodeI64LtS wasm.OpcodeI64LtU wasm.OpcodeF32Lt wasm.OpcodeF64Lt
type OperationLt struct{ Type SignedType }

// Kind implements Operation.Kind.
func (OperationLt) Kind() OperationKind {
	return OperationKindLt
}

// OperationGt implements Operation.
//
// This corresponds to wasm.OpcodeI32GtS wasm.OpcodeI32GtU wasm.OpcodeI64GtS wasm.OpcodeI64GtU wasm.OpcodeF32Gt wasm.OpcodeF64Gt
type OperationGt struct{ Type SignedType }

// Kind implements Operation.Kind.
func (OperationGt) Kind() OperationKind {
	return OperationKindGt
}

// OperationLe implements Operation.
//
// This corresponds to wasm.OpcodeI32LeS wasm.OpcodeI32LeU wasm.OpcodeI64LeS wasm.OpcodeI64LeU wasm.OpcodeF32Le wasm.OpcodeF64Le
type OperationLe struct{ Type SignedType }

// Kind implements Operation.Kind.
func (OperationLe) Kind() OperationKind {
	return OperationKindLe
}

// OperationGe implements Operation.
//
// This corresponds to wasm.OpcodeI32GeS wasm.OpcodeI32GeU wasm.OpcodeI64GeS wasm.OpcodeI64GeU wasm.OpcodeF32Ge wasm.OpcodeF64Ge
type OperationGe struct{ Type SignedType }

// Kind implements Operation.Kind.
func (OperationGe) Kind() OperationKind {
	return OperationKindGe
}

// OperationAdd implements Operation.
//
// This corresponds to wasm.OpcodeI32AddName wasm.OpcodeI64AddName wasm.OpcodeF32AddName wasm.OpcodeF64AddName.
type OperationAdd struct{ Type UnsignedType }

// Kind implements Operation.Kind.
func (OperationAdd) Kind() OperationKind {
	return OperationKindAdd
}

// OperationSub implements Operation.
//
// This corresponds to wasm.OpcodeI32SubName wasm.OpcodeI64SubName wasm.OpcodeF32SubName wasm.OpcodeF64SubName.
type OperationSub struct{ Type UnsignedType }

// Kind implements Operation.Kind.
func (OperationSub) Kind() OperationKind {
	return OperationKindSub
}

// OperationMul implements Operation.
//
// This corresponds to wasm.OpcodeI32MulName wasm.OpcodeI64MulName wasm.OpcodeF32MulName wasm.OpcodeF64MulName.
type OperationMul struct{ Type UnsignedType }

// Kind implements Operation.Kind.
func (OperationMul) Kind() OperationKind {
	return OperationKindMul
}

// OperationClz implements Operation.
//
// This corresponds to wasm.OpcodeI32ClzName wasm.OpcodeI64ClzName.
//
// The engines are expected to count up the leading zeros in the
// current top of the stack, and push the count result.
// For example, stack of [..., 0x00_ff_ff_ff] results in [..., 8].
// See wasm.OpcodeI32Clz wasm.OpcodeI64Clz
type OperationClz struct{ Type UnsignedInt }

// Kind implements Operation.Kind.
func (OperationClz) Kind() OperationKind {
	return OperationKindClz
}

// OperationCtz implements Operation.
//
// This corresponds to wasm.OpcodeI32CtzName wasm.OpcodeI64CtzName.
//
// The engines are expected to count up the trailing zeros in the
// current top of the stack, and push the count result.
// For example, stack of [..., 0xff_ff_ff_00] results in [..., 8].
type OperationCtz struct{ Type UnsignedInt }

// Kind implements Operation.Kind.
func (OperationCtz) Kind() OperationKind {
	return OperationKindCtz
}

// OperationPopcnt implements Operation.
//
// This corresponds to wasm.OpcodeI32PopcntName wasm.OpcodeI64PopcntName.
//
// The engines are expected to count up the number of set bits in the
// current top of the stack, and push the count result.
// For example, stack of [..., 0b00_00_00_11] results in [..., 2].
type OperationPopcnt struct{ Type UnsignedInt }

// Kind implements Operation.Kind.
func (OperationPopcnt) Kind() OperationKind {
	return OperationKindPopcnt
}

// OperationDiv implements Operation.
//
// This corresponds to wasm.OpcodeI32DivS wasm.OpcodeI32DivU wasm.OpcodeI64DivS
//
//	wasm.OpcodeI64DivU wasm.OpcodeF32Div wasm.OpcodeF64Div.
type OperationDiv struct{ Type SignedType }

// Kind implements Operation.Kind.
func (OperationDiv) Kind() OperationKind {
	return OperationKindDiv
}

// OperationRem implements Operation.
//
// This corresponds to wasm.OpcodeI32RemS wasm.OpcodeI32RemU wasm.OpcodeI64RemS wasm.OpcodeI64RemU.
//
// The engines are expected to perform division on the top
// two values of integer type on the stack and puts the remainder of the result
// onto the stack. For example, stack [..., 10, 3] results in [..., 1] where
// the quotient is discarded.
type OperationRem struct{ Type SignedInt }

// Kind implements Operation.Kind.
func (OperationRem) Kind() OperationKind {
	return OperationKindRem
}

// OperationAnd implements Operation.
//
// # This corresponds to wasm.OpcodeI32AndName wasm.OpcodeI64AndName
//
// The engines are expected to perform "And" operation on
// top two values on the stack, and pushes the result.
type OperationAnd struct{ Type UnsignedInt }

// Kind implements Operation.Kind.
func (OperationAnd) Kind() OperationKind {
	return OperationKindAnd
}

// OperationOr implements Operation.
//
// # This corresponds to wasm.OpcodeI32OrName wasm.OpcodeI64OrName
//
// The engines are expected to perform "Or" operation on
// top two values on the stack, and pushes the result.
type OperationOr struct{ Type UnsignedInt }

// Kind implements Operation.Kind.
func (OperationOr) Kind() OperationKind {
	return OperationKindOr
}

// OperationXor implements Operation.
//
// # This corresponds to wasm.OpcodeI32XorName wasm.OpcodeI64XorName
//
// The engines are expected to perform "Xor" operation on
// top two values on the stack, and pushes the result.
type OperationXor struct{ Type UnsignedInt }

// Kind implements Operation.Kind.
func (OperationXor) Kind() OperationKind {
	return OperationKindXor
}

// OperationShl implements Operation.
//
// # This corresponds to wasm.OpcodeI32ShlName wasm.OpcodeI64ShlName
//
// The engines are expected to perform "Shl" operation on
// top two values on the stack, and pushes the result.
type OperationShl struct{ Type UnsignedInt }

// Kind implements Operation.Kind.
func (OperationShl) Kind() OperationKind {
	return OperationKindShl
}

// OperationShr implements Operation.
//
// # This corresponds to wasm.OpcodeI32ShrSName wasm.OpcodeI32ShrUName wasm.OpcodeI64ShrSName wasm.OpcodeI64ShrUName
//
// If OperationShr.Type is signed integer, then, the engines are expected to perform arithmetic right shift on the two
// top values on the stack, otherwise do the logical right shift.
type OperationShr struct{ Type SignedInt }

// Kind implements Operation.Kind.
func (OperationShr) Kind() OperationKind {
	return OperationKindShr
}

// OperationRotl implements Operation.
//
// # This corresponds to wasm.OpcodeI32RotlName wasm.OpcodeI64RotlName
//
// The engines are expected to perform "Rotl" operation on
// top two values on the stack, and pushes the result.
type OperationRotl struct{ Type UnsignedInt }

// Kind implements Operation.Kind.
func (OperationRotl) Kind() OperationKind {
	return OperationKindRotl
}

// OperationRotr implements Operation.
//
// # This corresponds to wasm.OpcodeI32RotrName wasm.OpcodeI64RotrName
//
// The engines are expected to perform "Rotr" operation on
// top two values on the stack, and pushes the result.
type OperationRotr struct{ Type UnsignedInt }

// Kind implements Operation.Kind.
func (OperationRotr) Kind() OperationKind {
	return OperationKindRotr
}

// OperationAbs implements Operation.
//
// This corresponds to wasm.OpcodeF32Abs wasm.OpcodeF64Abs
type OperationAbs struct{ Type Float }

// Kind implements Operation.Kind.
func (OperationAbs) Kind() OperationKind {
	return OperationKindAbs
}

// OperationNeg implements Operation.
//
// This corresponds to wasm.OpcodeF32Neg wasm.OpcodeF64Neg
type OperationNeg struct{ Type Float }

// Kind implements Operation.Kind.
func (OperationNeg) Kind() OperationKind {
	return OperationKindNeg
}

// OperationCeil implements Operation.
//
// This corresponds to wasm.OpcodeF32CeilName wasm.OpcodeF64CeilName
type OperationCeil struct{ Type Float }

// Kind implements Operation.Kind.
func (OperationCeil) Kind() OperationKind {
	return OperationKindCeil
}

// OperationFloor implements Operation.
//
// This corresponds to wasm.OpcodeF32FloorName wasm.OpcodeF64FloorName
type OperationFloor struct{ Type Float }

// Kind implements Operation.Kind.
func (OperationFloor) Kind() OperationKind {
	return OperationKindFloor
}

// OperationTrunc implements Operation.
//
// This corresponds to wasm.OpcodeF32TruncName wasm.OpcodeF64TruncName
type OperationTrunc struct{ Type Float }

// Kind implements Operation.Kind.
func (OperationTrunc) Kind() OperationKind {
	return OperationKindTrunc
}

// OperationNearest implements Operation.
//
// # This corresponds to wasm.OpcodeF32NearestName wasm.OpcodeF64NearestName
//
// Note: this is *not* equivalent to math.Round and instead has the same
// the semantics of LLVM's rint intrinsic. See https://llvm.org/docs/LangRef.html#llvm-rint-intrinsic.
// For example, math.Round(-4.5) produces -5 while we want to produce -4.
type OperationNearest struct{ Type Float }

// Kind implements Operation.Kind.
func (OperationNearest) Kind() OperationKind {
	return OperationKindNearest
}

// OperationSqrt implements Operation.
//
// This corresponds to wasm.OpcodeF32SqrtName wasm.OpcodeF64SqrtName
type OperationSqrt struct{ Type Float }

// Kind implements Operation.Kind.
func (OperationSqrt) Kind() OperationKind {
	return OperationKindSqrt
}

// OperationMin implements Operation.
//
// # This corresponds to wasm.OpcodeF32MinName wasm.OpcodeF64MinName
//
// The engines are expected to pop two values from the stack, and push back the maximum of
// these two values onto the stack. For example, stack [..., 100.1, 1.9] results in [..., 1.9].
//
// Note: WebAssembly specifies that min/max must always return NaN if one of values is NaN,
// which is a different behavior different from math.Min.
type OperationMin struct{ Type Float }

// Kind implements Operation.Kind.
func (OperationMin) Kind() OperationKind {
	return OperationKindMin
}

// OperationMax implements Operation.
//
// # This corresponds to wasm.OpcodeF32MaxName wasm.OpcodeF64MaxName
//
// The engines are expected to pop two values from the stack, and push back the maximum of
// these two values onto the stack. For example, stack [..., 100.1, 1.9] results in [..., 100.1].
//
// Note: WebAssembly specifies that min/max must always return NaN if one of values is NaN,
// which is a different behavior different from math.Max.
type OperationMax struct{ Type Float }

// Kind implements Operation.Kind.
func (OperationMax) Kind() OperationKind {
	return OperationKindMax
}

// OperationCopysign implements Operation.
//
// # This corresponds to wasm.OpcodeF32CopysignName wasm.OpcodeF64CopysignName
//
// The engines are expected to pop two float values from the stack, and copy the signbit of
// the first-popped value to the last one.
// For example, stack [..., 1.213, -5.0] results in [..., -1.213].
type OperationCopysign struct{ Type Float }

// Kind implements Operation.Kind.
func (OperationCopysign) Kind() OperationKind {
	return OperationKindCopysign
}

// OperationI32WrapFromI64 implements Operation.
//
// This corresponds to wasm.OpcodeI32WrapI64 and equivalent to uint64(uint32(v)) in Go.
//
// The engines are expected to replace the 64-bit int on top of the stack
// with the corresponding 32-bit integer.
type OperationI32WrapFromI64 struct{}

// Kind implements Operation.Kind.
func (OperationI32WrapFromI64) Kind() OperationKind {
	return OperationKindI32WrapFromI64
}

// OperationITruncFromF implements Operation.
//
// This corresponds to
//
//	wasm.OpcodeI32TruncF32SName wasm.OpcodeI32TruncF32UName wasm.OpcodeI32TruncF64SName
//	wasm.OpcodeI32TruncF64UName wasm.OpcodeI64TruncF32SName wasm.OpcodeI64TruncF32UName wasm.OpcodeI64TruncF64SName
//	wasm.OpcodeI64TruncF64UName. wasm.OpcodeI32TruncSatF32SName wasm.OpcodeI32TruncSatF32UName
//	wasm.OpcodeI32TruncSatF64SName wasm.OpcodeI32TruncSatF64UName wasm.OpcodeI64TruncSatF32SName
//	wasm.OpcodeI64TruncSatF32UName wasm.OpcodeI64TruncSatF64SName wasm.OpcodeI64TruncSatF64UName
//
// See [1] and [2] for when we encounter undefined behavior in the WebAssembly specification if OperationITruncFromF.NonTrapping == false.
// To summarize, if the source float value is NaN or doesn't fit in the destination range of integers (incl. +=Inf),
// then the runtime behavior is undefined. In wazero, the engines are expected to exit the execution in these undefined cases with
// wasmruntime.ErrRuntimeInvalidConversionToInteger error.
//
// [1] https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#-hrefop-trunc-umathrmtruncmathsfu_m-n-z for unsigned integers.
// [2] https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#-hrefop-trunc-smathrmtruncmathsfs_m-n-z for signed integers.
type OperationITruncFromF struct {
	InputType  Float
	OutputType SignedInt
	// NonTrapping true if this conversion is "nontrapping" in the sense of the
	// https://github.com/WebAssembly/spec/blob/ce4b6c4d47eb06098cc7ab2e81f24748da822f20/proposals/nontrapping-float-to-int-conversion/Overview.md
	NonTrapping bool
}

// Kind implements Operation.Kind.
func (OperationITruncFromF) Kind() OperationKind {
	return OperationKindITruncFromF
}

// OperationFConvertFromI implements Operation.
//
// This corresponds to
//
//	wasm.OpcodeF32ConvertI32SName wasm.OpcodeF32ConvertI32UName wasm.OpcodeF32ConvertI64SName wasm.OpcodeF32ConvertI64UName
//	wasm.OpcodeF64ConvertI32SName wasm.OpcodeF64ConvertI32UName wasm.OpcodeF64ConvertI64SName wasm.OpcodeF64ConvertI64UName
//
// and equivalent to float32(uint32(x)), float32(int32(x)), etc in Go.
type OperationFConvertFromI struct {
	InputType  SignedInt
	OutputType Float
}

// Kind implements Operation.Kind.
func (OperationFConvertFromI) Kind() OperationKind {
	return OperationKindFConvertFromI
}

// OperationF32DemoteFromF64 implements Operation.
//
// This corresponds to wasm.OpcodeF32DemoteF64 and is equivalent float32(float64(v)).
type OperationF32DemoteFromF64 struct{}

// Kind implements Operation.Kind.
func (OperationF32DemoteFromF64) Kind() OperationKind {
	return OperationKindF32DemoteFromF64
}

// OperationF64PromoteFromF32 implements Operation.
//
// This corresponds to wasm.OpcodeF64PromoteF32 and is equivalent float64(float32(v)).
type OperationF64PromoteFromF32 struct{}

// Kind implements Operation.Kind.
func (OperationF64PromoteFromF32) Kind() OperationKind {
	return OperationKindF64PromoteFromF32
}

// OperationI32ReinterpretFromF32 implements Operation.
//
// This corresponds to wasm.OpcodeI32ReinterpretF32Name.
type OperationI32ReinterpretFromF32 struct{}

// Kind implements Operation.Kind.
func (OperationI32ReinterpretFromF32) Kind() OperationKind {
	return OperationKindI32ReinterpretFromF32
}

// OperationI64ReinterpretFromF64 implements Operation.
//
// This corresponds to wasm.OpcodeI64ReinterpretF64Name.
type OperationI64ReinterpretFromF64 struct{}

// Kind implements Operation.Kind.
func (OperationI64ReinterpretFromF64) Kind() OperationKind {
	return OperationKindI64ReinterpretFromF64
}

// OperationF32ReinterpretFromI32 implements Operation.
//
// This corresponds to wasm.OpcodeF32ReinterpretI32Name.
type OperationF32ReinterpretFromI32 struct{}

// Kind implements Operation.Kind.
func (OperationF32ReinterpretFromI32) Kind() OperationKind {
	return OperationKindF32ReinterpretFromI32
}

// OperationF64ReinterpretFromI64 implements Operation.
//
// This corresponds to wasm.OpcodeF64ReinterpretI64Name.
type OperationF64ReinterpretFromI64 struct{}

// Kind implements Operation.Kind.
func (OperationF64ReinterpretFromI64) Kind() OperationKind {
	return OperationKindF64ReinterpretFromI64
}

// OperationExtend implements Operation.
//
// # This corresponds to wasm.OpcodeI64ExtendI32SName wasm.OpcodeI64ExtendI32UName
//
// The engines are expected to extend the 32-bit signed or unsigned int on top of the stack
// as a 64-bit integer of corresponding signedness. For unsigned case, this is just reinterpreting the
// underlying bit pattern as 64-bit integer. For signed case, this is sign-extension which preserves the
// original integer's sign.
type OperationExtend struct{ Signed bool }

// Kind implements Operation.Kind.
func (OperationExtend) Kind() OperationKind {
	return OperationKindExtend
}

// OperationSignExtend32From8 implements Operation.
//
// This corresponds to wasm.OpcodeI32Extend8SName.
//
// The engines are expected to sign-extend the first 8-bits of 32-bit in as signed 32-bit int.
type OperationSignExtend32From8 struct{}

// Kind implements Operation.Kind.
func (OperationSignExtend32From8) Kind() OperationKind {
	return OperationKindSignExtend32From8
}

// OperationSignExtend32From16 implements Operation.
//
// This corresponds to wasm.OpcodeI32Extend16SName.
//
// The engines are expected to sign-extend the first 16-bits of 32-bit in as signed 32-bit int.
type OperationSignExtend32From16 struct{}

// Kind implements Operation.Kind.
func (OperationSignExtend32From16) Kind() OperationKind {
	return OperationKindSignExtend32From16
}

// OperationSignExtend64From8 implements Operation.
//
// This corresponds to wasm.OpcodeI64Extend8SName.
//
// The engines are expected to sign-extend the first 8-bits of 64-bit in as signed 32-bit int.
type OperationSignExtend64From8 struct{}

// Kind implements Operation.Kind.
func (OperationSignExtend64From8) Kind() OperationKind {
	return OperationKindSignExtend64From8
}

// OperationSignExtend64From16 implements Operation.
//
// This corresponds to wasm.OpcodeI64Extend16SName.
//
// The engines are expected to sign-extend the first 16-bits of 64-bit in as signed 32-bit int.
type OperationSignExtend64From16 struct{}

// Kind implements Operation.Kind.
func (OperationSignExtend64From16) Kind() OperationKind {
	return OperationKindSignExtend64From16
}

// OperationSignExtend64From32 implements Operation.
//
// This corresponds to wasm.OpcodeI64Extend32SName.
//
// The engines are expected to sign-extend the first 32-bits of 64-bit in as signed 32-bit int.
type OperationSignExtend64From32 struct{}

// Kind implements Operation.Kind.
func (OperationSignExtend64From32) Kind() OperationKind {
	return OperationKindSignExtend64From32
}

// OperationMemoryInit implements Operation.
//
// This corresponds to wasm.OpcodeMemoryInitName.
type OperationMemoryInit struct {
	// DataIndex is the index of the data instance in ModuleInstance.DataInstances
	// by which this operation instantiates a part of the memory.
	DataIndex uint32
}

// Kind implements Operation.Kind.
func (OperationMemoryInit) Kind() OperationKind {
	return OperationKindMemoryInit
}

// OperationDataDrop implements Operation.
//
// This corresponds to wasm.OpcodeDataDropName.
type OperationDataDrop struct {
	// DataIndex is the index of the data instance in ModuleInstance.DataInstances
	// which this operation drops.
	DataIndex uint32
}

// Kind implements Operation.Kind.
func (OperationDataDrop) Kind() OperationKind {
	return OperationKindDataDrop
}

// OperationMemoryCopy implements Operation.
//
// This corresponds to wasm.OpcodeMemoryCopyName.
type OperationMemoryCopy struct{}

// Kind implements Operation.Kind.
func (OperationMemoryCopy) Kind() OperationKind {
	return OperationKindMemoryCopy
}

// OperationMemoryFill implements Operation.
//
// This corresponds to wasm.OpcodeMemoryFillName.
type OperationMemoryFill struct{}

// Kind implements Operation.Kind.
func (OperationMemoryFill) Kind() OperationKind {
	return OperationKindMemoryFill
}

// OperationTableInit implements Operation.
//
// This corresponds to wasm.OpcodeTableInitName.
type OperationTableInit struct {
	// ElemIndex is the index of the element by which this operation initializes a part of the table.
	ElemIndex uint32
	// TableIndex is the index of the table on which this operation initialize by the target element.
	TableIndex uint32
}

// Kind implements Operation.Kind.
func (OperationTableInit) Kind() OperationKind {
	return OperationKindTableInit
}

// OperationElemDrop implements Operation.
//
// This corresponds to wasm.OpcodeElemDropName.
type OperationElemDrop struct {
	// ElemIndex is the index of the element which this operation drops.
	ElemIndex uint32
}

// Kind implements Operation.Kind.
func (OperationElemDrop) Kind() OperationKind {
	return OperationKindElemDrop
}

// OperationTableCopy implements Operation.
//
// This corresponds to wasm.OpcodeTableCopyName.
type OperationTableCopy struct {
	SrcTableIndex, DstTableIndex uint32
}

// Kind implements Operation.Kind.
func (OperationTableCopy) Kind() OperationKind {
	return OperationKindTableCopy
}

// OperationRefFunc implements Operation.
//
// This corresponds to wasm.OpcodeRefFuncName, and engines are expected to
// push the opaque pointer value of engine specific func for the given FunctionIndex.
//
// Note: in wazero, we express any reference types (funcref or externref) as opaque pointers which is uint64.
// Therefore, the engine implementations emit instructions to push the address of *function onto the stack.
type OperationRefFunc struct {
	FunctionIndex uint32
}

// Kind implements Operation.Kind.
func (OperationRefFunc) Kind() OperationKind {
	return OperationKindRefFunc
}

// OperationTableGet implements Operation.
//
// This corresponds to wasm.OpcodeTableGetName.
type OperationTableGet struct {
	TableIndex uint32
}

// Kind implements Operation.Kind.
func (OperationTableGet) Kind() OperationKind {
	return OperationKindTableGet
}

// OperationTableSet implements Operation.
//
// This corresponds to wasm.OpcodeTableSetName.
type OperationTableSet struct {
	TableIndex uint32
}

// Kind implements Operation.Kind.
func (OperationTableSet) Kind() OperationKind {
	return OperationKindTableSet
}

// OperationTableSize implements Operation.
//
// This corresponds to wasm.OpcodeTableSizeName.
type OperationTableSize struct {
	TableIndex uint32
}

// Kind implements Operation.Kind.
func (OperationTableSize) Kind() OperationKind {
	return OperationKindTableSize
}

// OperationTableGrow implements Operation.
//
// This corresponds to wasm.OpcodeTableGrowName.
type OperationTableGrow struct {
	TableIndex uint32
}

// Kind implements Operation.Kind.
func (OperationTableGrow) Kind() OperationKind {
	return OperationKindTableGrow
}

// OperationTableFill implements Operation.
//
// This corresponds to wasm.OpcodeTableFillName.
type OperationTableFill struct {
	TableIndex uint32
}

// Kind implements Operation.Kind.
func (OperationTableFill) Kind() OperationKind {
	return OperationKindTableFill
}

// OperationV128Const implements Operation.
type OperationV128Const struct {
	Lo, Hi uint64
}

// Kind implements Operation.Kind.
//
// This corresponds to wasm.OpcodeVecV128Const.
func (OperationV128Const) Kind() OperationKind {
	return OperationKindV128Const
}

// Shape corresponds to a shape of v128 values.
// https://webassembly.github.io/spec/core/syntax/instructions.html#syntax-shape
type Shape = byte

const (
	ShapeI8x16 Shape = iota
	ShapeI16x8
	ShapeI32x4
	ShapeI64x2
	ShapeF32x4
	ShapeF64x2
)

func shapeName(s Shape) (ret string) {
	switch s {
	case ShapeI8x16:
		ret = "I8x16"
	case ShapeI16x8:
		ret = "I16x8"
	case ShapeI32x4:
		ret = "I32x4"
	case ShapeI64x2:
		ret = "I64x2"
	case ShapeF32x4:
		ret = "F32x4"
	case ShapeF64x2:
		ret = "F64x2"
	}
	return
}

// OperationV128Add implements Operation.
//
// This corresponds to wasm.OpcodeVecI8x16AddName wasm.OpcodeVecI16x8AddName wasm.OpcodeVecI32x4AddName
//
//	wasm.OpcodeVecI64x2AddName wasm.OpcodeVecF32x4AddName wasm.OpcodeVecF64x2AddName
type OperationV128Add struct {
	Shape Shape
}

// Kind implements Operation.Kind.
func (OperationV128Add) Kind() OperationKind {
	return OperationKindV128Add
}

// OperationV128Sub implements Operation.
//
// This corresponds to wasm.OpcodeVecI8x16SubName wasm.OpcodeVecI16x8SubName wasm.OpcodeVecI32x4SubName
//
//	wasm.OpcodeVecI64x2SubName wasm.OpcodeVecF32x4SubName wasm.OpcodeVecF64x2SubName
type OperationV128Sub struct {
	Shape Shape
}

// Kind implements Operation.Kind.
func (OperationV128Sub) Kind() OperationKind {
	return OperationKindV128Sub
}

// V128LoadType represents a type of wasm.OpcodeVecV128Load* instructions.
type V128LoadType = byte

const (
	// V128LoadType128 corresponds to wasm.OpcodeVecV128LoadName.
	V128LoadType128 V128LoadType = iota
	// V128LoadType8x8s corresponds to wasm.OpcodeVecV128Load8x8SName.
	V128LoadType8x8s
	// V128LoadType8x8u corresponds to wasm.OpcodeVecV128Load8x8UName.
	V128LoadType8x8u
	// V128LoadType16x4s corresponds to wasm.OpcodeVecV128Load16x4SName
	V128LoadType16x4s
	// V128LoadType16x4u corresponds to wasm.OpcodeVecV128Load16x4UName
	V128LoadType16x4u
	// V128LoadType32x2s corresponds to wasm.OpcodeVecV128Load32x2SName
	V128LoadType32x2s
	// V128LoadType32x2u corresponds to wasm.OpcodeVecV128Load32x2UName
	V128LoadType32x2u
	// V128LoadType8Splat corresponds to wasm.OpcodeVecV128Load8SplatName
	V128LoadType8Splat
	// V128LoadType16Splat corresponds to wasm.OpcodeVecV128Load16SplatName
	V128LoadType16Splat
	// V128LoadType32Splat corresponds to wasm.OpcodeVecV128Load32SplatName
	V128LoadType32Splat
	// V128LoadType64Splat corresponds to wasm.OpcodeVecV128Load64SplatName
	V128LoadType64Splat
	// V128LoadType32zero corresponds to wasm.OpcodeVecV128Load32zeroName
	V128LoadType32zero
	// V128LoadType64zero corresponds to wasm.OpcodeVecV128Load64zeroName
	V128LoadType64zero
)

// OperationV128Load implements Operation.
//
// This corresponds to
//
//	wasm.OpcodeVecV128LoadName wasm.OpcodeVecV128Load8x8SName wasm.OpcodeVecV128Load8x8UName
//	wasm.OpcodeVecV128Load16x4SName wasm.OpcodeVecV128Load16x4UName wasm.OpcodeVecV128Load32x2SName
//	wasm.OpcodeVecV128Load32x2UName wasm.OpcodeVecV128Load8SplatName wasm.OpcodeVecV128Load16SplatName
//	wasm.OpcodeVecV128Load32SplatName wasm.OpcodeVecV128Load64SplatName wasm.OpcodeVecV128Load32zeroName
//	wasm.OpcodeVecV128Load64zeroName
type OperationV128Load struct {
	Type V128LoadType
	Arg  MemoryArg
}

// Kind implements Operation.Kind.
func (OperationV128Load) Kind() OperationKind {
	return OperationKindV128Load
}

// OperationV128LoadLane implements Operation.
//
// This corresponds to wasm.OpcodeVecV128Load8LaneName wasm.OpcodeVecV128Load16LaneName
//
//	wasm.OpcodeVecV128Load32LaneName wasm.OpcodeVecV128Load64LaneName.
type OperationV128LoadLane struct {
	// LaneIndex is >=0 && <(128/LaneSize).
	LaneIndex byte
	// LaneSize is either 8, 16, 32, or 64.
	LaneSize byte
	Arg      MemoryArg
}

// Kind implements Operation.Kind.
func (OperationV128LoadLane) Kind() OperationKind {
	return OperationKindV128LoadLane
}

// OperationV128Store implements Operation.
//
// This corresponds to wasm.OpcodeVecV128Load8LaneName wasm.OpcodeVecV128Load16LaneName
//
//	wasm.OpcodeVecV128Load32LaneName wasm.OpcodeVecV128Load64LaneName.
type OperationV128Store struct {
	Arg MemoryArg
}

// Kind implements Operation.Kind.
func (OperationV128Store) Kind() OperationKind {
	return OperationKindV128Store
}

// OperationV128StoreLane implements Operation.
//
// This corresponds to wasm.OpcodeVecV128Load8LaneName wasm.OpcodeVecV128Load16LaneName
//
//	wasm.OpcodeVecV128Load32LaneName wasm.OpcodeVecV128Load64LaneName.
type OperationV128StoreLane struct {
	// LaneIndex is >=0 && <(128/LaneSize).
	LaneIndex byte
	// LaneSize is either 8, 16, 32, or 64.
	LaneSize byte
	Arg      MemoryArg
}

// Kind implements Operation.Kind.
func (OperationV128StoreLane) Kind() OperationKind {
	return OperationKindV128StoreLane
}

// OperationV128ExtractLane implements Operation.
//
// This corresponds to
//
//	wasm.OpcodeVecI8x16ExtractLaneSName wasm.OpcodeVecI8x16ExtractLaneUName
//	wasm.OpcodeVecI16x8ExtractLaneSName wasm.OpcodeVecI16x8ExtractLaneUName
//	wasm.OpcodeVecI32x4ExtractLaneName wasm.OpcodeVecI64x2ExtractLaneName
//	wasm.OpcodeVecF32x4ExtractLaneName wasm.OpcodeVecF64x2ExtractLaneName.
type OperationV128ExtractLane struct {
	// LaneIndex is >=0 && <M where shape = NxM.
	LaneIndex byte
	// Signed is used when shape is either i8x16 or i16x2 to specify whether to sign-extend or not.
	Signed bool
	Shape  Shape
}

// Kind implements Operation.Kind.
func (OperationV128ExtractLane) Kind() OperationKind {
	return OperationKindV128ExtractLane
}

// OperationV128ReplaceLane implements Operation.
//
// This corresponds to
//
//	wasm.OpcodeVecI8x16ReplaceLaneName wasm.OpcodeVecI16x8ReplaceLaneName
//	wasm.OpcodeVecI32x4ReplaceLaneName wasm.OpcodeVecI64x2ReplaceLaneName
//	wasm.OpcodeVecF32x4ReplaceLaneName wasm.OpcodeVecF64x2ReplaceLaneName.
type OperationV128ReplaceLane struct {
	// LaneIndex is >=0 && <M where shape = NxM.
	LaneIndex byte
	Shape     Shape
}

// Kind implements Operation.Kind.
func (OperationV128ReplaceLane) Kind() OperationKind {
	return OperationKindV128ReplaceLane
}

// OperationV128Splat implements Operation.
//
// This corresponds to
//
//	wasm.OpcodeVecI8x16SplatName wasm.OpcodeVecI16x8SplatName
//	wasm.OpcodeVecI32x4SplatName wasm.OpcodeVecI64x2SplatName
//	wasm.OpcodeVecF32x4SplatName wasm.OpcodeVecF64x2SplatName.
type OperationV128Splat struct {
	Shape Shape
}

// Kind implements Operation.Kind.
func (OperationV128Splat) Kind() OperationKind {
	return OperationKindV128Splat
}

// OperationV128Shuffle implements Operation.
type OperationV128Shuffle struct {
	Lanes [16]byte
}

// Kind implements Operation.Kind.
//
// This corresponds to wasm.OpcodeVecV128i8x16ShuffleName.
func (OperationV128Shuffle) Kind() OperationKind {
	return OperationKindV128Shuffle
}

// OperationV128Swizzle implements Operation.
type OperationV128Swizzle struct{}

// Kind implements Operation.Kind.
//
// This corresponds to wasm.OpcodeVecI8x16SwizzleName.
func (OperationV128Swizzle) Kind() OperationKind {
	return OperationKindV128Swizzle
}

// OperationV128AnyTrue implements Operation.
//
// This corresponds to wasm.OpcodeVecV128AnyTrueName.
type OperationV128AnyTrue struct{}

// Kind implements Operation.Kind.
func (OperationV128AnyTrue) Kind() OperationKind {
	return OperationKindV128AnyTrue
}

// OperationV128AllTrue implements Operation.
//
// This corresponds to
//
//	wasm.OpcodeVecI8x16AllTrueName wasm.OpcodeVecI16x8AllTrueName
//	wasm.OpcodeVecI32x4AllTrueName wasm.OpcodeVecI64x2AllTrueName.
type OperationV128AllTrue struct {
	Shape Shape
}

// Kind implements Operation.Kind.
func (OperationV128AllTrue) Kind() OperationKind {
	return OperationKindV128AllTrue
}

// OperationV128BitMask implements Operation.
//
// This corresponds to
//
//	wasm.OpcodeVecI8x16BitMaskName wasm.OpcodeVecI16x8BitMaskName
//	wasm.OpcodeVecI32x4BitMaskName wasm.OpcodeVecI64x2BitMaskName.
type OperationV128BitMask struct {
	Shape Shape
}

// Kind implements Operation.Kind.
func (OperationV128BitMask) Kind() OperationKind {
	return OperationKindV128BitMask
}

// OperationV128And implements Operation.
//
// This corresponds to wasm.OpcodeVecV128And.
type OperationV128And struct{}

// Kind implements Operation.Kind.
func (OperationV128And) Kind() OperationKind {
	return OperationKindV128And
}

// OperationV128Not implements Operation.
//
// This corresponds to wasm.OpcodeVecV128Not.
type OperationV128Not struct{}

// Kind implements Operation.Kind.
func (OperationV128Not) Kind() OperationKind {
	return OperationKindV128Not
}

// OperationV128Or implements Operation.
//
// This corresponds to wasm.OpcodeVecV128Or.
type OperationV128Or struct{}

// Kind implements Operation.Kind.
func (OperationV128Or) Kind() OperationKind {
	return OperationKindV128Or
}

// OperationV128Xor implements Operation.
//
// This corresponds to wasm.OpcodeVecV128Xor.
type OperationV128Xor struct{}

// Kind implements Operation.Kind.
func (OperationV128Xor) Kind() OperationKind {
	return OperationKindV128Xor
}

// OperationV128Bitselect implements Operation.
//
// This corresponds to wasm.OpcodeVecV128Bitselect.
type OperationV128Bitselect struct{}

// Kind implements Operation.Kind.
func (OperationV128Bitselect) Kind() OperationKind {
	return OperationKindV128Bitselect
}

// OperationV128AndNot implements Operation.
//
// This corresponds to wasm.OpcodeVecV128AndNot.
type OperationV128AndNot struct{}

// Kind implements Operation.Kind.
func (OperationV128AndNot) Kind() OperationKind {
	return OperationKindV128AndNot
}

// OperationV128Shl implements Operation.
//
// This corresponds to
//
//	wasm.OpcodeVecI8x16ShlName wasm.OpcodeVecI16x8ShlName
//	wasm.OpcodeVecI32x4ShlName wasm.OpcodeVecI64x2ShlName
type OperationV128Shl struct {
	Shape Shape
}

// Kind implements Operation.Kind.
func (OperationV128Shl) Kind() OperationKind {
	return OperationKindV128Shl
}

// OperationV128Shr implements Operation.
//
// This corresponds to
//
//	wasm.OpcodeVecI8x16ShrSName wasm.OpcodeVecI8x16ShrUName wasm.OpcodeVecI16x8ShrSName
//	wasm.OpcodeVecI16x8ShrUName wasm.OpcodeVecI32x4ShrSName wasm.OpcodeVecI32x4ShrUName.
//	wasm.OpcodeVecI64x2ShrSName wasm.OpcodeVecI64x2ShrUName.
type OperationV128Shr struct {
	Shape  Shape
	Signed bool
}

// Kind implements Operation.Kind.
func (OperationV128Shr) Kind() OperationKind {
	return OperationKindV128Shr
}

// OperationV128Cmp implements Operation.
//
// This corresponds to
//
//	wasm.OpcodeVecI8x16EqName, wasm.OpcodeVecI8x16NeName, wasm.OpcodeVecI8x16LtSName, wasm.OpcodeVecI8x16LtUName, wasm.OpcodeVecI8x16GtSName,
//	wasm.OpcodeVecI8x16GtUName, wasm.OpcodeVecI8x16LeSName, wasm.OpcodeVecI8x16LeUName, wasm.OpcodeVecI8x16GeSName, wasm.OpcodeVecI8x16GeUName,
//	wasm.OpcodeVecI16x8EqName, wasm.OpcodeVecI16x8NeName, wasm.OpcodeVecI16x8LtSName, wasm.OpcodeVecI16x8LtUName, wasm.OpcodeVecI16x8GtSName,
//	wasm.OpcodeVecI16x8GtUName, wasm.OpcodeVecI16x8LeSName, wasm.OpcodeVecI16x8LeUName, wasm.OpcodeVecI16x8GeSName, wasm.OpcodeVecI16x8GeUName,
//	wasm.OpcodeVecI32x4EqName, wasm.OpcodeVecI32x4NeName, wasm.OpcodeVecI32x4LtSName, wasm.OpcodeVecI32x4LtUName, wasm.OpcodeVecI32x4GtSName,
//	wasm.OpcodeVecI32x4GtUName, wasm.OpcodeVecI32x4LeSName, wasm.OpcodeVecI32x4LeUName, wasm.OpcodeVecI32x4GeSName, wasm.OpcodeVecI32x4GeUName,
//	wasm.OpcodeVecI64x2EqName, wasm.OpcodeVecI64x2NeName, wasm.OpcodeVecI64x2LtSName, wasm.OpcodeVecI64x2GtSName, wasm.OpcodeVecI64x2LeSName,
//	wasm.OpcodeVecI64x2GeSName, wasm.OpcodeVecF32x4EqName, wasm.OpcodeVecF32x4NeName, wasm.OpcodeVecF32x4LtName, wasm.OpcodeVecF32x4GtName,
//	wasm.OpcodeVecF32x4LeName, wasm.OpcodeVecF32x4GeName, wasm.OpcodeVecF64x2EqName, wasm.OpcodeVecF64x2NeName, wasm.OpcodeVecF64x2LtName,
//	wasm.OpcodeVecF64x2GtName, wasm.OpcodeVecF64x2LeName, wasm.OpcodeVecF64x2GeName
type OperationV128Cmp struct {
	Type V128CmpType
}

// V128CmpType represents a type of vector comparison operation.
type V128CmpType = byte

const (
	// V128CmpTypeI8x16Eq corresponds to wasm.OpcodeVecI8x16EqName.
	V128CmpTypeI8x16Eq V128CmpType = iota
	// V128CmpTypeI8x16Ne corresponds to wasm.OpcodeVecI8x16NeName.
	V128CmpTypeI8x16Ne
	// V128CmpTypeI8x16LtS corresponds to wasm.OpcodeVecI8x16LtSName.
	V128CmpTypeI8x16LtS
	// V128CmpTypeI8x16LtU corresponds to wasm.OpcodeVecI8x16LtUName.
	V128CmpTypeI8x16LtU
	// V128CmpTypeI8x16GtS corresponds to wasm.OpcodeVecI8x16GtSName.
	V128CmpTypeI8x16GtS
	// V128CmpTypeI8x16GtU corresponds to wasm.OpcodeVecI8x16GtUName.
	V128CmpTypeI8x16GtU
	// V128CmpTypeI8x16LeS corresponds to wasm.OpcodeVecI8x16LeSName.
	V128CmpTypeI8x16LeS
	// V128CmpTypeI8x16LeU corresponds to wasm.OpcodeVecI8x16LeUName.
	V128CmpTypeI8x16LeU
	// V128CmpTypeI8x16GeS corresponds to wasm.OpcodeVecI8x16GeSName.
	V128CmpTypeI8x16GeS
	// V128CmpTypeI8x16GeU corresponds to wasm.OpcodeVecI8x16GeUName.
	V128CmpTypeI8x16GeU
	// V128CmpTypeI16x8Eq corresponds to wasm.OpcodeVecI16x8EqName.
	V128CmpTypeI16x8Eq
	// V128CmpTypeI16x8Ne corresponds to wasm.OpcodeVecI16x8NeName.
	V128CmpTypeI16x8Ne
	// V128CmpTypeI16x8LtS corresponds to wasm.OpcodeVecI16x8LtSName.
	V128CmpTypeI16x8LtS
	// V128CmpTypeI16x8LtU corresponds to wasm.OpcodeVecI16x8LtUName.
	V128CmpTypeI16x8LtU
	// V128CmpTypeI16x8GtS corresponds to wasm.OpcodeVecI16x8GtSName.
	V128CmpTypeI16x8GtS
	// V128CmpTypeI16x8GtU corresponds to wasm.OpcodeVecI16x8GtUName.
	V128CmpTypeI16x8GtU
	// V128CmpTypeI16x8LeS corresponds to wasm.OpcodeVecI16x8LeSName.
	V128CmpTypeI16x8LeS
	// V128CmpTypeI16x8LeU corresponds to wasm.OpcodeVecI16x8LeUName.
	V128CmpTypeI16x8LeU
	// V128CmpTypeI16x8GeS corresponds to wasm.OpcodeVecI16x8GeSName.
	V128CmpTypeI16x8GeS
	// V128CmpTypeI16x8GeU corresponds to wasm.OpcodeVecI16x8GeUName.
	V128CmpTypeI16x8GeU
	// V128CmpTypeI32x4Eq corresponds to wasm.OpcodeVecI32x4EqName.
	V128CmpTypeI32x4Eq
	// V128CmpTypeI32x4Ne corresponds to wasm.OpcodeVecI32x4NeName.
	V128CmpTypeI32x4Ne
	// V128CmpTypeI32x4LtS corresponds to wasm.OpcodeVecI32x4LtSName.
	V128CmpTypeI32x4LtS
	// V128CmpTypeI32x4LtU corresponds to wasm.OpcodeVecI32x4LtUName.
	V128CmpTypeI32x4LtU
	// V128CmpTypeI32x4GtS corresponds to wasm.OpcodeVecI32x4GtSName.
	V128CmpTypeI32x4GtS
	// V128CmpTypeI32x4GtU corresponds to wasm.OpcodeVecI32x4GtUName.
	V128CmpTypeI32x4GtU
	// V128CmpTypeI32x4LeS corresponds to wasm.OpcodeVecI32x4LeSName.
	V128CmpTypeI32x4LeS
	// V128CmpTypeI32x4LeU corresponds to wasm.OpcodeVecI32x4LeUName.
	V128CmpTypeI32x4LeU
	// V128CmpTypeI32x4GeS corresponds to wasm.OpcodeVecI32x4GeSName.
	V128CmpTypeI32x4GeS
	// V128CmpTypeI32x4GeU corresponds to wasm.OpcodeVecI32x4GeUName.
	V128CmpTypeI32x4GeU
	// V128CmpTypeI64x2Eq corresponds to wasm.OpcodeVecI64x2EqName.
	V128CmpTypeI64x2Eq
	// V128CmpTypeI64x2Ne corresponds to wasm.OpcodeVecI64x2NeName.
	V128CmpTypeI64x2Ne
	// V128CmpTypeI64x2LtS corresponds to wasm.OpcodeVecI64x2LtSName.
	V128CmpTypeI64x2LtS
	// V128CmpTypeI64x2GtS corresponds to wasm.OpcodeVecI64x2GtSName.
	V128CmpTypeI64x2GtS
	// V128CmpTypeI64x2LeS corresponds to wasm.OpcodeVecI64x2LeSName.
	V128CmpTypeI64x2LeS
	// V128CmpTypeI64x2GeS corresponds to wasm.OpcodeVecI64x2GeSName.
	V128CmpTypeI64x2GeS
	// V128CmpTypeF32x4Eq corresponds to wasm.OpcodeVecF32x4EqName.
	V128CmpTypeF32x4Eq
	// V128CmpTypeF32x4Ne corresponds to wasm.OpcodeVecF32x4NeName.
	V128CmpTypeF32x4Ne
	// V128CmpTypeF32x4Lt corresponds to wasm.OpcodeVecF32x4LtName.
	V128CmpTypeF32x4Lt
	// V128CmpTypeF32x4Gt corresponds to wasm.OpcodeVecF32x4GtName.
	V128CmpTypeF32x4Gt
	// V128CmpTypeF32x4Le corresponds to wasm.OpcodeVecF32x4LeName.
	V128CmpTypeF32x4Le
	// V128CmpTypeF32x4Ge corresponds to wasm.OpcodeVecF32x4GeName.
	V128CmpTypeF32x4Ge
	// V128CmpTypeF64x2Eq corresponds to wasm.OpcodeVecF64x2EqName.
	V128CmpTypeF64x2Eq
	// V128CmpTypeF64x2Ne corresponds to wasm.OpcodeVecF64x2NeName.
	V128CmpTypeF64x2Ne
	// V128CmpTypeF64x2Lt corresponds to wasm.OpcodeVecF64x2LtName.
	V128CmpTypeF64x2Lt
	// V128CmpTypeF64x2Gt corresponds to wasm.OpcodeVecF64x2GtName.
	V128CmpTypeF64x2Gt
	// V128CmpTypeF64x2Le corresponds to wasm.OpcodeVecF64x2LeName.
	V128CmpTypeF64x2Le
	// V128CmpTypeF64x2Ge corresponds to wasm.OpcodeVecF64x2GeName.
	V128CmpTypeF64x2Ge
)

// Kind implements Operation.Kind.
func (OperationV128Cmp) Kind() OperationKind {
	return OperationKindV128Cmp
}

// OperationV128AddSat implements Operation.
//
// This corresponds to wasm.OpcodeVecI8x16AddSatUName wasm.OpcodeVecI8x16AddSatSName
//
//	wasm.OpcodeVecI16x8AddSatUName wasm.OpcodeVecI16x8AddSatSName
type OperationV128AddSat struct {
	// Shape is either ShapeI8x16 or ShapeI16x8.
	Shape  Shape
	Signed bool
}

// Kind implements Operation.Kind.
func (OperationV128AddSat) Kind() OperationKind {
	return OperationKindV128AddSat
}

// OperationV128SubSat implements Operation.
//
// This corresponds to wasm.OpcodeVecI8x16SubSatUName wasm.OpcodeVecI8x16SubSatSName
//
//	wasm.OpcodeVecI16x8SubSatUName wasm.OpcodeVecI16x8SubSatSName
type OperationV128SubSat struct {
	// Shape is either ShapeI8x16 or ShapeI16x8.
	Shape  Shape
	Signed bool
}

// Kind implements Operation.Kind.
func (OperationV128SubSat) Kind() OperationKind {
	return OperationKindV128SubSat
}

// OperationV128Mul implements Operation.
//
// This corresponds to wasm.OpcodeVecF32x4MulName wasm.OpcodeVecF64x2MulName
//
//	wasm.OpcodeVecI16x8MulName wasm.OpcodeVecI32x4MulName wasm.OpcodeVecI64x2MulName.
type OperationV128Mul struct {
	// Shape is either ShapeI16x8, ShapeI32x4, ShapeI64x2, ShapeF32x4 or ShapeF64x2.
	Shape Shape
}

// Kind implements Operation.Kind.
func (OperationV128Mul) Kind() OperationKind {
	return OperationKindV128Mul
}

// OperationV128Div implements Operation.
//
// This corresponds to wasm.OpcodeVecF32x4DivName wasm.OpcodeVecF64x2DivName.
type OperationV128Div struct {
	// Shape is either ShapeF32x4 or ShapeF64x2.
	Shape Shape
}

// Kind implements Operation.Kind.
func (OperationV128Div) Kind() OperationKind {
	return OperationKindV128Div
}

// OperationV128Neg implements Operation.
//
// This corresponds to wasm.OpcodeVecI8x16NegName wasm.OpcodeVecI16x8NegName wasm.OpcodeVecI32x4NegName
//
//	wasm.OpcodeVecI64x2NegName wasm.OpcodeVecF32x4NegName wasm.OpcodeVecF64x2NegName.
type OperationV128Neg struct {
	Shape Shape
}

// Kind implements Operation.Kind.
func (OperationV128Neg) Kind() OperationKind {
	return OperationKindV128Neg
}

// OperationV128Sqrt implements Operation.
//
// This corresponds to wasm.OpcodeVecF32x4SqrtName wasm.OpcodeVecF64x2SqrtName.
type OperationV128Sqrt struct {
	// Shape is either ShapeF32x4 or ShapeF64x2.
	Shape Shape
}

// Kind implements Operation.Kind.
func (OperationV128Sqrt) Kind() OperationKind {
	return OperationKindV128Sqrt
}

// OperationV128Abs implements Operation.
//
// This corresponds to wasm.OpcodeVecI8x16AbsName wasm.OpcodeVecI16x8AbsName wasm.OpcodeVecI32x4AbsName
//
//	wasm.OpcodeVecI64x2AbsName wasm.OpcodeVecF32x4AbsName wasm.OpcodeVecF64x2AbsName.
type OperationV128Abs struct {
	Shape Shape
}

// Kind implements Operation.Kind.
func (OperationV128Abs) Kind() OperationKind {
	return OperationKindV128Abs
}

// OperationV128Popcnt implements Operation.
//
// This corresponds to wasm.OpcodeVecI8x16PopcntName.
type OperationV128Popcnt struct {
	Shape Shape
}

// Kind implements Operation.Kind.
func (OperationV128Popcnt) Kind() OperationKind {
	return OperationKindV128Popcnt
}

// OperationV128Min implements Operation.
//
// This corresponds to
//
//	wasm.OpcodeVecI8x16MinSName wasm.OpcodeVecI8x16MinUName　wasm.OpcodeVecI16x8MinSName wasm.OpcodeVecI16x8MinUName
//	wasm.OpcodeVecI32x4MinSName wasm.OpcodeVecI32x4MinUName　wasm.OpcodeVecI16x8MinSName wasm.OpcodeVecI16x8MinUName
//	wasm.OpcodeVecF32x4MinName wasm.OpcodeVecF64x2MinName
type OperationV128Min struct {
	Shape  Shape
	Signed bool
}

// Kind implements Operation.Kind.
func (OperationV128Min) Kind() OperationKind {
	return OperationKindV128Min
}

// OperationV128Max implements Operation.
//
// This corresponds to
//
//	wasm.OpcodeVecI8x16MaxSName wasm.OpcodeVecI8x16MaxUName　wasm.OpcodeVecI16x8MaxSName wasm.OpcodeVecI16x8MaxUName
//	wasm.OpcodeVecI32x4MaxSName wasm.OpcodeVecI32x4MaxUName　wasm.OpcodeVecI16x8MaxSName wasm.OpcodeVecI16x8MaxUName
//	wasm.OpcodeVecF32x4MaxName wasm.OpcodeVecF64x2MaxName.
type OperationV128Max struct {
	Shape  Shape
	Signed bool
}

// Kind implements Operation.Kind.
func (OperationV128Max) Kind() OperationKind {
	return OperationKindV128Max
}

// OperationV128AvgrU implements Operation.
//
// This corresponds to wasm.OpcodeVecI8x16AvgrUName.
type OperationV128AvgrU struct {
	Shape Shape
}

// Kind implements Operation.Kind.
func (OperationV128AvgrU) Kind() OperationKind {
	return OperationKindV128AvgrU
}

// OperationV128Pmin implements Operation.
//
// This corresponds to wasm.OpcodeVecF32x4PminName wasm.OpcodeVecF64x2PminName.
type OperationV128Pmin struct{ Shape Shape }

// Kind implements Operation.Kind
func (OperationV128Pmin) Kind() OperationKind {
	return OperationKindV128Pmin
}

// OperationV128Pmax implements Operation.
//
// This corresponds to wasm.OpcodeVecF32x4PmaxName wasm.OpcodeVecF64x2PmaxName.
type OperationV128Pmax struct{ Shape Shape }

// Kind implements Operation.Kind
func (OperationV128Pmax) Kind() OperationKind {
	return OperationKindV128Pmax
}

// OperationV128Ceil implements Operation.
//
// This corresponds to wasm.OpcodeVecF32x4CeilName wasm.OpcodeVecF64x2CeilName
type OperationV128Ceil struct{ Shape Shape }

// Kind implements Operation.Kind
func (OperationV128Ceil) Kind() OperationKind {
	return OperationKindV128Ceil
}

// OperationV128Floor implements Operation.
//
// This corresponds to wasm.OpcodeVecF32x4FloorName wasm.OpcodeVecF64x2FloorName
type OperationV128Floor struct{ Shape Shape }

// Kind implements Operation.Kind
func (OperationV128Floor) Kind() OperationKind {
	return OperationKindV128Floor
}

// OperationV128Trunc implements Operation.
//
// This corresponds to wasm.OpcodeVecF32x4TruncName wasm.OpcodeVecF64x2TruncName
type OperationV128Trunc struct{ Shape Shape }

// Kind implements Operation.Kind
func (OperationV128Trunc) Kind() OperationKind {
	return OperationKindV128Trunc
}

// OperationV128Nearest implements Operation.
//
// This corresponds to wasm.OpcodeVecF32x4NearestName wasm.OpcodeVecF64x2NearestName
type OperationV128Nearest struct{ Shape Shape }

// Kind implements Operation.Kind
func (OperationV128Nearest) Kind() OperationKind {
	return OperationKindV128Nearest
}

// OperationV128Extend implements Operation
//
// This corresponds to
//
//	wasm.OpcodeVecI16x8ExtendLowI8x16SName wasm.OpcodeVecI16x8ExtendHighI8x16SName
//	wasm.OpcodeVecI16x8ExtendLowI8x16UName wasm.OpcodeVecI16x8ExtendHighI8x16UName
//	wasm.OpcodeVecI32x4ExtendLowI16x8SName wasm.OpcodeVecI32x4ExtendHighI16x8SName
//	wasm.OpcodeVecI32x4ExtendLowI16x8UName wasm.OpcodeVecI32x4ExtendHighI16x8UName
//	wasm.OpcodeVecI64x2ExtendLowI32x4SName wasm.OpcodeVecI64x2ExtendHighI32x4SName
//	wasm.OpcodeVecI64x2ExtendLowI32x4UName wasm.OpcodeVecI64x2ExtendHighI32x4UName
type OperationV128Extend struct {
	// OriginShape is the shape of the original lanes for extension which is
	// either ShapeI8x16, ShapeI16x8, or ShapeI32x4.
	OriginShape Shape
	Signed      bool
	// UseLow true if it uses the lower half of vector for extension.
	UseLow bool
}

// Kind implements Operation.Kind
func (OperationV128Extend) Kind() OperationKind {
	return OperationKindV128Extend
}

// OperationV128ExtMul implements Operation
//
// This corresponds to
//
//		wasm.OpcodeVecI16x8ExtMulLowI8x16SName wasm.OpcodeVecI16x8ExtMulLowI8x16UName
//		wasm.OpcodeVecI16x8ExtMulHighI8x16SName wasm.OpcodeVecI16x8ExtMulHighI8x16UName
//	 wasm.OpcodeVecI32x4ExtMulLowI16x8SName wasm.OpcodeVecI32x4ExtMulLowI16x8UName
//		wasm.OpcodeVecI32x4ExtMulHighI16x8SName wasm.OpcodeVecI32x4ExtMulHighI16x8UName
//	 wasm.OpcodeVecI64x2ExtMulLowI32x4SName wasm.OpcodeVecI64x2ExtMulLowI32x4UName
//		wasm.OpcodeVecI64x2ExtMulHighI32x4SName wasm.OpcodeVecI64x2ExtMulHighI32x4UName.
type OperationV128ExtMul struct {
	// OriginShape is the shape of the original lanes for extension which is
	// either ShapeI8x16, ShapeI16x8, or ShapeI32x4.
	OriginShape Shape
	Signed      bool
	// UseLow true if it uses the lower half of vector for extension.
	UseLow bool
}

// Kind implements Operation.Kind
func (OperationV128ExtMul) Kind() OperationKind {
	return OperationKindV128ExtMul
}

// OperationV128Q15mulrSatS implements Operation
//
// This corresponds to wasm.OpcodeVecI16x8Q15mulrSatSName
type OperationV128Q15mulrSatS struct{}

// Kind implements Operation.Kind
func (OperationV128Q15mulrSatS) Kind() OperationKind {
	return OperationKindV128Q15mulrSatS
}

// OperationV128ExtAddPairwise implements Operation.
//
// This corresponds to
//
//	wasm.OpcodeVecI16x8ExtaddPairwiseI8x16SName wasm.OpcodeVecI16x8ExtaddPairwiseI8x16UName
//	wasm.OpcodeVecI32x4ExtaddPairwiseI16x8SName wasm.OpcodeVecI32x4ExtaddPairwiseI16x8UName.
type OperationV128ExtAddPairwise struct {
	// OriginShape is the shape of the original lanes for extension which is
	// either ShapeI8x16, or ShapeI16x8.
	OriginShape Shape
	Signed      bool
}

// Kind implements Operation.Kind.
func (OperationV128ExtAddPairwise) Kind() OperationKind {
	return OperationKindV128ExtAddPairwise
}

// OperationV128FloatPromote implements Operation.
//
// This corresponds to wasm.OpcodeVecF64x2PromoteLowF32x4ZeroName
// This discards the higher 64-bit of a vector, and promotes two
// 32-bit floats in the lower 64-bit as two 64-bit floats.
type OperationV128FloatPromote struct{}

// Kind implements Operation.Kind.
func (OperationV128FloatPromote) Kind() OperationKind {
	return OperationKindV128FloatPromote
}

// OperationV128FloatDemote implements Operation.
//
// This corresponds to wasm.OpcodeVecF32x4DemoteF64x2ZeroName.
type OperationV128FloatDemote struct{}

// Kind implements Operation.Kind.
func (OperationV128FloatDemote) Kind() OperationKind {
	return OperationKindV128FloatDemote
}

// OperationV128FConvertFromI implements Operation.
//
// This corresponds to
//
//	wasm.OpcodeVecF32x4ConvertI32x4SName wasm.OpcodeVecF32x4ConvertI32x4UName
//	wasm.OpcodeVecF64x2ConvertLowI32x4SName wasm.OpcodeVecF64x2ConvertLowI32x4UName.
type OperationV128FConvertFromI struct {
	// DestinationShape is the shape of the destination lanes for conversion which is
	// either ShapeF32x4, or ShapeF64x2.
	DestinationShape Shape
	Signed           bool
}

// Kind implements Operation.Kind.
func (OperationV128FConvertFromI) Kind() OperationKind {
	return OperationKindV128FConvertFromI
}

// OperationV128Dot implements Operation.
//
// This corresponds to wasm.OpcodeVecI32x4DotI16x8SName
type OperationV128Dot struct{}

// Kind implements Operation.Kind.
func (OperationV128Dot) Kind() OperationKind {
	return OperationKindV128Dot
}

// OperationV128Narrow implements Operation.
//
// This corresponds to
//
//	wasm.OpcodeVecI8x16NarrowI16x8SName wasm.OpcodeVecI8x16NarrowI16x8UName
//	wasm.OpcodeVecI16x8NarrowI32x4SName wasm.OpcodeVecI16x8NarrowI32x4UName.
type OperationV128Narrow struct {
	// OriginShape is the shape of the original lanes for narrowing which is
	// either ShapeI16x8, or ShapeI32x4.
	OriginShape Shape
	Signed      bool
}

// Kind implements Operation.Kind.
func (OperationV128Narrow) Kind() OperationKind {
	return OperationKindV128Narrow
}

// OperationV128ITruncSatFromF implements Operation.
//
// This corresponds to
//
//	wasm.OpcodeVecI32x4TruncSatF64x2UZeroName wasm.OpcodeVecI32x4TruncSatF64x2SZeroName
//	wasm.OpcodeVecI32x4TruncSatF32x4UName wasm.OpcodeVecI32x4TruncSatF32x4SName.
type OperationV128ITruncSatFromF struct {
	// OriginShape is the shape of the original lanes for truncation which is
	// either ShapeF32x4, or ShapeF64x2.
	OriginShape Shape
	Signed      bool
}

// Kind implements Operation.Kind.
func (OperationV128ITruncSatFromF) Kind() OperationKind {
	return OperationKindV128ITruncSatFromF
}
