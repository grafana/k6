package arm64

import (
	"github.com/tetratelabs/wazero/internal/asm"
)

// Assembler is the interface for arm64 specific assembler.
type Assembler interface {
	asm.AssemblerBase

	// CompileMemoryWithRegisterOffsetToRegister adds an instruction where source operand is the memory address
	// specified as `srcBaseReg + srcOffsetReg` and dst is the register `dstReg`.
	CompileMemoryWithRegisterOffsetToRegister(instruction asm.Instruction, srcBaseReg, srcOffsetReg, dstReg asm.Register)

	// CompileRegisterToMemoryWithRegisterOffset adds an instruction where source operand is the register `srcReg`,
	// and the destination is the memory address specified as `dstBaseReg + dstOffsetReg`
	CompileRegisterToMemoryWithRegisterOffset(instruction asm.Instruction, srcReg, dstBaseReg, dstOffsetReg asm.Register)

	// CompileTwoRegistersToRegister adds an instruction where source operands consists of two registers `src1` and `src2`,
	// and the destination is the register `dst`.
	CompileTwoRegistersToRegister(instruction asm.Instruction, src1, src2, dst asm.Register)

	// CompileThreeRegistersToRegister adds an instruction where source operands consist of three registers
	// `src1`, `src2` and `src3`, and destination operands consist of `dst` register.
	CompileThreeRegistersToRegister(instruction asm.Instruction, src1, src2, src3, dst asm.Register)

	// CompileTwoRegistersToNone adds an instruction where source operands consist of two registers `src1` and `src2`,
	// and destination operand is unspecified.
	CompileTwoRegistersToNone(instruction asm.Instruction, src1, src2 asm.Register)

	// CompileRegisterAndConstToNone adds an instruction where source operands consist of one register `src` and
	// constant `srcConst`, and destination operand is unspecified.
	CompileRegisterAndConstToNone(instruction asm.Instruction, src asm.Register, srcConst asm.ConstantValue)

	// CompileRegisterAndConstToRegister adds an instruction where source operands consist of one register `src` and
	// constant `srcConst`, and destination operand is a register `dst`.
	CompileRegisterAndConstToRegister(instruction asm.Instruction, src asm.Register, srcConst asm.ConstantValue, dst asm.Register)

	// CompileLeftShiftedRegisterToRegister adds an instruction where source operand is the "left shifted register"
	// represented as `srcReg << shiftNum` and the destination is the register `dstReg`.
	CompileLeftShiftedRegisterToRegister(
		instruction asm.Instruction,
		shiftedSourceReg asm.Register,
		shiftNum asm.ConstantValue,
		srcReg, dstReg asm.Register,
	)

	// CompileConditionalRegisterSet adds an instruction to set 1 on dstReg if the condition satisfies,
	// otherwise set 0.
	CompileConditionalRegisterSet(cond asm.ConditionalRegisterState, dstReg asm.Register)

	// CompileMemoryToVectorRegister adds an instruction where source operands is the memory address specified by
	// `srcBaseReg+srcOffset` and the destination is `dstReg` vector register.
	CompileMemoryToVectorRegister(instruction asm.Instruction, srcBaseReg asm.Register, srcOffset asm.ConstantValue,
		dstReg asm.Register, arrangement VectorArrangement)

	// CompileMemoryWithRegisterOffsetToVectorRegister is the same as CompileMemoryToVectorRegister except that the
	// offset is specified by the `srcOffsetRegister` register.
	CompileMemoryWithRegisterOffsetToVectorRegister(instruction asm.Instruction, srcBaseReg,
		srcOffsetRegister asm.Register, dstReg asm.Register, arrangement VectorArrangement)

	// CompileVectorRegisterToMemory adds an instruction where source operand is `srcReg` vector register and the
	// destination is the memory address specified by `dstBaseReg+dstOffset`.
	CompileVectorRegisterToMemory(instruction asm.Instruction, srcReg, dstBaseReg asm.Register,
		dstOffset asm.ConstantValue, arrangement VectorArrangement)

	// CompileVectorRegisterToMemoryWithRegisterOffset is the same as CompileVectorRegisterToMemory except that the
	// offset is specified by the `dstOffsetRegister` register.
	CompileVectorRegisterToMemoryWithRegisterOffset(instruction asm.Instruction, srcReg, dstBaseReg,
		dstOffsetRegister asm.Register, arrangement VectorArrangement)

	// CompileRegisterToVectorRegister adds an instruction where source operand is `srcReg` general purpose register and
	// the destination is the `dstReg` vector register. The destination vector's arrangement and index of element can be
	// given by `arrangement` and `index`, but not all the instructions will use them.
	CompileRegisterToVectorRegister(instruction asm.Instruction, srcReg, dstReg asm.Register,
		arrangement VectorArrangement, index VectorIndex)

	// CompileVectorRegisterToRegister adds an instruction where destination operand is `dstReg` general purpose register
	// and the source is the `srcReg` vector register. The source vector's arrangement and index of element can be
	// given by `arrangement` and `index`, but not all the instructions will use them.
	CompileVectorRegisterToRegister(instruction asm.Instruction, srcReg, dstReg asm.Register,
		arrangement VectorArrangement, index VectorIndex)

	// CompileVectorRegisterToVectorRegister adds an instruction where both source and destination operands are vector
	// registers. The vector's arrangement can be specified `arrangement`, and the source and destination element's
	// index are given by `srcIndex` and `dstIndex` respectively, but not all the instructions will use them.
	CompileVectorRegisterToVectorRegister(instruction asm.Instruction, srcReg, dstReg asm.Register,
		arrangement VectorArrangement, srcIndex, dstIndex VectorIndex)

	// CompileVectorRegisterToVectorRegisterWithConst is the same as CompileVectorRegisterToVectorRegister but the
	// additional constant can be provided.
	// For example, the const can be used to specify the shift amount for USHLL instruction.
	CompileVectorRegisterToVectorRegisterWithConst(instruction asm.Instruction, srcReg, dstReg asm.Register,
		arrangement VectorArrangement, c asm.ConstantValue)

	// CompileStaticConstToRegister adds an instruction where the source operand is StaticConstant located in
	// the memory and the destination is the dstReg.
	CompileStaticConstToRegister(instruction asm.Instruction, c *asm.StaticConst, dstReg asm.Register)

	// CompileStaticConstToVectorRegister adds an instruction where the source operand is StaticConstant located in
	// the memory and the destination is the dstReg.
	CompileStaticConstToVectorRegister(instruction asm.Instruction, c *asm.StaticConst, dstReg asm.Register,
		arrangement VectorArrangement)

	// CompileTwoVectorRegistersToVectorRegister adds an instruction where source are two vectors and destination is one
	// vector. The vector's arrangement can be specified `arrangement`.
	CompileTwoVectorRegistersToVectorRegister(instruction asm.Instruction, srcReg, srcReg2, dstReg asm.Register,
		arrangement VectorArrangement)

	// CompileTwoVectorRegistersToVectorRegisterWithConst is the same as CompileTwoVectorRegistersToVectorRegister except
	// that this also accept additional constant.
	// For example EXIT instruction needs the extraction target immediate as const.
	CompileTwoVectorRegistersToVectorRegisterWithConst(instruction asm.Instruction, srcReg, srcReg2, dstReg asm.Register,
		arrangement VectorArrangement, c asm.ConstantValue)
}
