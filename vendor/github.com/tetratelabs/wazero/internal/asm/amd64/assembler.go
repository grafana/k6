package amd64

import (
	"github.com/tetratelabs/wazero/internal/asm"
)

// Assembler is the interface used by amd64 compiler.
type Assembler interface {
	asm.AssemblerBase

	// CompileJumpToMemory adds jump-type instruction whose destination is stored in the memory address specified by `baseReg+offset`,
	// and returns the corresponding Node in the assembled linked list.
	CompileJumpToMemory(jmpInstruction asm.Instruction, baseReg asm.Register, offset asm.ConstantValue)

	// CompileRegisterToRegisterWithArg adds an instruction where source and destination
	// are `from` and `to` registers.
	CompileRegisterToRegisterWithArg(instruction asm.Instruction, from, to asm.Register, arg byte)

	// CompileMemoryWithIndexToRegister adds an instruction where source operand is the memory address
	// specified as `srcBaseReg + srcOffsetConst + srcIndex*srcScale` and destination is the register `dstReg`.
	// Note: sourceScale must be one of 1, 2, 4, 8.
	CompileMemoryWithIndexToRegister(
		instruction asm.Instruction,
		srcBaseReg asm.Register,
		srcOffsetConst int64,
		srcIndex asm.Register,
		srcScale int16,
		dstReg asm.Register,
	)

	// CompileMemoryWithIndexAndArgToRegister is the same as CompileMemoryWithIndexToRegister except that this
	// also accepts one argument.
	CompileMemoryWithIndexAndArgToRegister(
		instruction asm.Instruction,
		srcBaseReg asm.Register,
		srcOffsetConst int64,
		srcIndex asm.Register,
		srcScale int16,
		dstReg asm.Register,
		arg byte,
	)

	// CompileRegisterToMemoryWithIndex adds an instruction where source operand is the register `srcReg`,
	// and the destination is the memory address specified as `dstBaseReg + dstOffsetConst + dstIndex*dstScale`
	// Note: dstScale must be one of 1, 2, 4, 8.
	CompileRegisterToMemoryWithIndex(
		instruction asm.Instruction,
		srcReg asm.Register,
		dstBaseReg asm.Register,
		dstOffsetConst int64,
		dstIndex asm.Register,
		dstScale int16,
	)

	// CompileRegisterToMemoryWithIndexAndArg is the same as CompileRegisterToMemoryWithIndex except that this
	// also accepts one argument.
	CompileRegisterToMemoryWithIndexAndArg(
		instruction asm.Instruction,
		srcReg asm.Register,
		dstBaseReg asm.Register,
		dstOffsetConst int64,
		dstIndex asm.Register,
		dstScale int16,
		arg byte,
	)

	// CompileRegisterToConst adds an instruction where source operand is the register `srcRegister`,
	// and the destination is the const `value`.
	CompileRegisterToConst(instruction asm.Instruction, srcRegister asm.Register, value int64) asm.Node

	// CompileRegisterToNone adds an instruction where source operand is the register `register`,
	// and there's no destination operand.
	CompileRegisterToNone(instruction asm.Instruction, register asm.Register)

	// CompileNoneToRegister adds an instruction where destination operand is the register `register`,
	// and there's no source operand.
	CompileNoneToRegister(instruction asm.Instruction, register asm.Register)

	// CompileNoneToMemory adds an instruction where destination operand is the memory address specified
	// as `baseReg+offset`. and there's no source operand.
	CompileNoneToMemory(instruction asm.Instruction, baseReg asm.Register, offset int64)

	// CompileConstToMemory adds an instruction where source operand is the constant `value` and
	// the destination is the memory address specified as `dstBaseReg+dstOffset`.
	CompileConstToMemory(instruction asm.Instruction, value int64, dstBaseReg asm.Register, dstOffset int64) asm.Node

	// CompileMemoryToConst adds an instruction where source operand is the memory address, and
	// the destination is the constant `value`.
	CompileMemoryToConst(instruction asm.Instruction, srcBaseReg asm.Register, srcOffset int64, value int64) asm.Node

	// CompileStaticConstToRegister adds an instruction where the source operand is asm.StaticConst located in the
	// memory and the destination is the dstReg.
	CompileStaticConstToRegister(instruction asm.Instruction, c *asm.StaticConst, dstReg asm.Register) error

	// CompileRegisterToStaticConst adds an instruction where the destination operand is asm.StaticConst located in the
	// memory and the source is the srcReg.
	CompileRegisterToStaticConst(instruction asm.Instruction, srcReg asm.Register, c *asm.StaticConst) error
}
