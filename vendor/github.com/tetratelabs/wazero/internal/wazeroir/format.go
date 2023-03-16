package wazeroir

import (
	"bytes"
	"fmt"
	"io"
	"strings"
)

const EntrypointLabel = ".entrypoint"

func Format(ops []Operation) string {
	buf := bytes.NewBuffer(nil)

	_, _ = buf.WriteString(EntrypointLabel + "\n")
	for _, op := range ops {
		formatOperation(buf, op)
	}

	return buf.String()
}

func formatOperation(w io.StringWriter, b Operation) {
	var str string
	var isLabel bool
	switch o := b.(type) {
	case OperationUnreachable:
		str = "unreachable"
	case OperationLabel:
		isLabel = true
		str = fmt.Sprintf("%s:", o.Label)
	case OperationBr:
		str = fmt.Sprintf("br %s", o.Target.String())
	case OperationBrIf:
		str = fmt.Sprintf("br_if %s, %s", o.Then, o.Else)
	case OperationBrTable:
		targets := make([]string, len(o.Targets))
		for i, t := range o.Targets {
			targets[i] = t.String()
		}
		str = fmt.Sprintf("br_table [%s] %s", strings.Join(targets, ","), o.Default)
	case OperationCall:
		str = fmt.Sprintf("call %d", o.FunctionIndex)
	case OperationCallIndirect:
		str = fmt.Sprintf("call_indirect: type=%d, table=%d", o.TypeIndex, o.TableIndex)
	case OperationDrop:
		str = fmt.Sprintf("drop %d..%d", o.Depth.Start, o.Depth.End)
	case OperationSelect:
		str = "select"
	case OperationPick:
		str = fmt.Sprintf("pick %d (is_vector=%v)", o.Depth, o.IsTargetVector)
	case OperationSet:
		str = fmt.Sprintf("swap %d (is_vector=%v)", o.Depth, o.IsTargetVector)
	case OperationGlobalGet:
		str = fmt.Sprintf("global.get %d", o.Index)
	case OperationGlobalSet:
		str = fmt.Sprintf("global.set %d", o.Index)
	case OperationLoad:
		str = fmt.Sprintf("%s.load (align=%d, offset=%d)", o.Type, o.Arg.Alignment, o.Arg.Offset)
	case OperationLoad8:
		str = fmt.Sprintf("%s.load8 (align=%d, offset=%d)", o.Type, o.Arg.Alignment, o.Arg.Offset)
	case OperationLoad16:
		str = fmt.Sprintf("%s.load16 (align=%d, offset=%d)", o.Type, o.Arg.Alignment, o.Arg.Offset)
	case OperationLoad32:
		var t string
		if o.Signed {
			t = "i64"
		} else {
			t = "u64"
		}
		str = fmt.Sprintf("%s.load32 (align=%d, offset=%d)", t, o.Arg.Alignment, o.Arg.Offset)
	case OperationStore:
		str = fmt.Sprintf("%s.store (align=%d, offset=%d)", o.Type, o.Arg.Alignment, o.Arg.Offset)
	case OperationStore8:
		str = fmt.Sprintf("store8 (align=%d, offset=%d)", o.Arg.Alignment, o.Arg.Offset)
	case OperationStore16:
		str = fmt.Sprintf("store16 (align=%d, offset=%d)", o.Arg.Alignment, o.Arg.Offset)
	case OperationStore32:
		str = fmt.Sprintf("i64.store32 (align=%d, offset=%d)", o.Arg.Alignment, o.Arg.Offset)
	case OperationMemorySize:
		str = "memory.size"
	case OperationMemoryGrow:
		str = "memory.grow"
	case OperationConstI32:
		str = fmt.Sprintf("i32.const %d", o.Value)
	case OperationConstI64:
		str = fmt.Sprintf("i64.const %d", o.Value)
	case OperationConstF32:
		str = fmt.Sprintf("f32.const %f", o.Value)
	case OperationConstF64:
		str = fmt.Sprintf("f64.const %f", o.Value)
	case OperationEq:
		str = fmt.Sprintf("%s.eq", o.Type)
	case OperationNe:
		str = fmt.Sprintf("%s.ne", o.Type)
	case OperationEqz:
		str = fmt.Sprintf("%s.eqz", o.Type)
	case OperationLt:
		str = fmt.Sprintf("%s.lt", o.Type)
	case OperationGt:
		str = fmt.Sprintf("%s.gt", o.Type)
	case OperationLe:
		str = fmt.Sprintf("%s.le", o.Type)
	case OperationGe:
		str = fmt.Sprintf("%s.ge", o.Type)
	case OperationAdd:
		str = fmt.Sprintf("%s.add", o.Type)
	case OperationSub:
		str = fmt.Sprintf("%s.sub", o.Type)
	case OperationMul:
		str = fmt.Sprintf("%s.mul", o.Type)
	case OperationClz:
		str = fmt.Sprintf("%s.clz", o.Type)
	case OperationCtz:
		str = fmt.Sprintf("%s.ctz", o.Type)
	case OperationPopcnt:
		str = fmt.Sprintf("%s.popcnt", o.Type)
	case OperationDiv:
		str = fmt.Sprintf("%s.div", o.Type)
	case OperationRem:
		str = fmt.Sprintf("%s.rem", o.Type)
	case OperationAnd:
		str = fmt.Sprintf("%s.and", o.Type)
	case OperationOr:
		str = fmt.Sprintf("%s.or", o.Type)
	case OperationXor:
		str = fmt.Sprintf("%s.xor", o.Type)
	case OperationShl:
		str = fmt.Sprintf("%s.shl", o.Type)
	case OperationShr:
		str = fmt.Sprintf("%s.shr", o.Type)
	case OperationRotl:
		str = fmt.Sprintf("%s.rotl", o.Type)
	case OperationRotr:
		str = fmt.Sprintf("%s.rotr", o.Type)
	case OperationAbs:
		str = fmt.Sprintf("%s.abs", o.Type)
	case OperationNeg:
		str = fmt.Sprintf("%s.neg", o.Type)
	case OperationCeil:
		str = fmt.Sprintf("%s.ceil", o.Type)
	case OperationFloor:
		str = fmt.Sprintf("%s.floor", o.Type)
	case OperationTrunc:
		str = fmt.Sprintf("%s.trunc", o.Type)
	case OperationNearest:
		str = fmt.Sprintf("%s.nearest", o.Type)
	case OperationSqrt:
		str = fmt.Sprintf("%s.sqrt", o.Type)
	case OperationMin:
		str = fmt.Sprintf("%s.min", o.Type)
	case OperationMax:
		str = fmt.Sprintf("%s.max", o.Type)
	case OperationCopysign:
		str = fmt.Sprintf("%s.copysign", o.Type)
	case OperationI32WrapFromI64:
		str = "i32.wrap_from.i64"
	case OperationITruncFromF:
		str = fmt.Sprintf("%s.truncate_from.%s", o.OutputType, o.InputType)
	case OperationFConvertFromI:
		str = fmt.Sprintf("%s.convert_from.%s", o.OutputType, o.InputType)
	case OperationF32DemoteFromF64:
		str = "f32.demote_from.f64"
	case OperationF64PromoteFromF32:
		str = "f64.promote_from.f32"
	case OperationI32ReinterpretFromF32:
		str = "i32.reinterpret_from.f32"
	case OperationI64ReinterpretFromF64:
		str = "i64.reinterpret_from.f64"
	case OperationF32ReinterpretFromI32:
		str = "f32.reinterpret_from.i32"
	case OperationF64ReinterpretFromI64:
		str = "f64.reinterpret_from.i64"
	case OperationExtend:
		var in, out string
		if o.Signed {
			in = "i32"
			out = "i64"
		} else {
			in = "u32"
			out = "u64"
		}
		str = fmt.Sprintf("%s.extend_from.%s", out, in)
	case OperationV128Const:
		str = fmt.Sprintf("v128.const [%#x, %#x]", o.Lo, o.Hi)
	case OperationV128Add:
		str = fmt.Sprintf("v128.add (shape=%s)", shapeName(o.Shape))
	case OperationV128ITruncSatFromF:
		if o.Signed {
			str = fmt.Sprintf("v128.ITruncSatFrom%sS", shapeName(o.OriginShape))
		} else {
			str = fmt.Sprintf("v128.ITruncSatFrom%sU", shapeName(o.OriginShape))
		}
	case OperationBuiltinFunctionCheckExitCode:
		str = "builtin_function.check_closed"
	default:
		panic("unreachable: a bug in wazeroir implementation")
	}

	if !isLabel {
		const indent = "\t"
		str = indent + str
	}

	_, _ = w.WriteString(str + "\n")
}
