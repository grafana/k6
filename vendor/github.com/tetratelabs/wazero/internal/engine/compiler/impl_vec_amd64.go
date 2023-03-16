package compiler

import (
	"errors"

	"github.com/tetratelabs/wazero/internal/asm"
	"github.com/tetratelabs/wazero/internal/asm/amd64"
	"github.com/tetratelabs/wazero/internal/wazeroir"
)

// compileV128Const implements compiler.compileV128Const for amd64 architecture.
func (c *amd64Compiler) compileV128Const(o wazeroir.OperationV128Const) error {
	if err := c.maybeCompileMoveTopConditionalToGeneralPurposeRegister(); err != nil {
		return err
	}

	result, err := c.allocateRegister(registerTypeVector)
	if err != nil {
		return err
	}

	// We cannot directly load the value from memory to float regs,
	// so we move it to int reg temporarily.
	tmpReg, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}

	// Move the lower 64-bits.
	if o.Lo == 0 {
		c.assembler.CompileRegisterToRegister(amd64.XORQ, tmpReg, tmpReg)
	} else {
		c.assembler.CompileConstToRegister(amd64.MOVQ, int64(o.Lo), tmpReg)
	}
	c.assembler.CompileRegisterToRegister(amd64.MOVQ, tmpReg, result)

	if o.Lo != 0 && o.Hi == 0 {
		c.assembler.CompileRegisterToRegister(amd64.XORQ, tmpReg, tmpReg)
	} else if o.Hi != 0 {
		c.assembler.CompileConstToRegister(amd64.MOVQ, int64(o.Hi), tmpReg)
	}
	// Move the higher 64-bits with PINSRQ at the second element of 64x2 vector.
	c.assembler.CompileRegisterToRegisterWithArg(amd64.PINSRQ, tmpReg, result, 1)

	c.pushVectorRuntimeValueLocationOnRegister(result)
	return nil
}

// compileV128Add implements compiler.compileV128Add for amd64 architecture.
func (c *amd64Compiler) compileV128Add(o wazeroir.OperationV128Add) error {
	x2 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x2); err != nil {
		return err
	}

	x1 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x1); err != nil {
		return err
	}
	var inst asm.Instruction
	switch o.Shape {
	case wazeroir.ShapeI8x16:
		inst = amd64.PADDB
	case wazeroir.ShapeI16x8:
		inst = amd64.PADDW
	case wazeroir.ShapeI32x4:
		inst = amd64.PADDD
	case wazeroir.ShapeI64x2:
		inst = amd64.PADDQ
	case wazeroir.ShapeF32x4:
		inst = amd64.ADDPS
	case wazeroir.ShapeF64x2:
		inst = amd64.ADDPD
	}
	c.assembler.CompileRegisterToRegister(inst, x2.register, x1.register)

	c.pushVectorRuntimeValueLocationOnRegister(x1.register)
	c.locationStack.markRegisterUnused(x2.register)
	return nil
}

// compileV128Sub implements compiler.compileV128Sub for amd64 architecture.
func (c *amd64Compiler) compileV128Sub(o wazeroir.OperationV128Sub) error {
	x2 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x2); err != nil {
		return err
	}

	x1 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x1); err != nil {
		return err
	}
	var inst asm.Instruction
	switch o.Shape {
	case wazeroir.ShapeI8x16:
		inst = amd64.PSUBB
	case wazeroir.ShapeI16x8:
		inst = amd64.PSUBW
	case wazeroir.ShapeI32x4:
		inst = amd64.PSUBD
	case wazeroir.ShapeI64x2:
		inst = amd64.PSUBQ
	case wazeroir.ShapeF32x4:
		inst = amd64.SUBPS
	case wazeroir.ShapeF64x2:
		inst = amd64.SUBPD
	}
	c.assembler.CompileRegisterToRegister(inst, x2.register, x1.register)

	c.pushVectorRuntimeValueLocationOnRegister(x1.register)
	c.locationStack.markRegisterUnused(x2.register)
	return nil
}

// compileV128Load implements compiler.compileV128Load for amd64 architecture.
func (c *amd64Compiler) compileV128Load(o wazeroir.OperationV128Load) error {
	result, err := c.allocateRegister(registerTypeVector)
	if err != nil {
		return err
	}

	switch o.Type {
	case wazeroir.V128LoadType128:
		err = c.compileV128LoadImpl(amd64.MOVDQU, o.Arg.Offset, 16, result)
	case wazeroir.V128LoadType8x8s:
		err = c.compileV128LoadImpl(amd64.PMOVSXBW, o.Arg.Offset, 8, result)
	case wazeroir.V128LoadType8x8u:
		err = c.compileV128LoadImpl(amd64.PMOVZXBW, o.Arg.Offset, 8, result)
	case wazeroir.V128LoadType16x4s:
		err = c.compileV128LoadImpl(amd64.PMOVSXWD, o.Arg.Offset, 8, result)
	case wazeroir.V128LoadType16x4u:
		err = c.compileV128LoadImpl(amd64.PMOVZXWD, o.Arg.Offset, 8, result)
	case wazeroir.V128LoadType32x2s:
		err = c.compileV128LoadImpl(amd64.PMOVSXDQ, o.Arg.Offset, 8, result)
	case wazeroir.V128LoadType32x2u:
		err = c.compileV128LoadImpl(amd64.PMOVZXDQ, o.Arg.Offset, 8, result)
	case wazeroir.V128LoadType8Splat:
		reg, err := c.compileMemoryAccessCeilSetup(o.Arg.Offset, 1)
		if err != nil {
			return err
		}
		c.assembler.CompileMemoryWithIndexToRegister(amd64.MOVBQZX, amd64ReservedRegisterForMemory, -1,
			reg, 1, reg)
		// pinsrb   $0, reg, result
		// pxor	    tmpVReg, tmpVReg
		// pshufb   tmpVReg, result
		c.locationStack.markRegisterUsed(result)
		tmpVReg, err := c.allocateRegister(registerTypeVector)
		if err != nil {
			return err
		}
		c.assembler.CompileRegisterToRegisterWithArg(amd64.PINSRB, reg, result, 0)
		c.assembler.CompileRegisterToRegister(amd64.PXOR, tmpVReg, tmpVReg)
		c.assembler.CompileRegisterToRegister(amd64.PSHUFB, tmpVReg, result)
	case wazeroir.V128LoadType16Splat:
		reg, err := c.compileMemoryAccessCeilSetup(o.Arg.Offset, 2)
		if err != nil {
			return err
		}
		c.assembler.CompileMemoryWithIndexToRegister(amd64.MOVWQZX, amd64ReservedRegisterForMemory, -2,
			reg, 1, reg)
		// pinsrw $0, reg, result
		// pinsrw $1, reg, result
		// pshufd $0, result, result (result = result[0,0,0,0])
		c.assembler.CompileRegisterToRegisterWithArg(amd64.PINSRW, reg, result, 0)
		c.assembler.CompileRegisterToRegisterWithArg(amd64.PINSRW, reg, result, 1)
		c.assembler.CompileRegisterToRegisterWithArg(amd64.PSHUFD, result, result, 0)
	case wazeroir.V128LoadType32Splat:
		reg, err := c.compileMemoryAccessCeilSetup(o.Arg.Offset, 4)
		if err != nil {
			return err
		}
		c.assembler.CompileMemoryWithIndexToRegister(amd64.MOVLQZX, amd64ReservedRegisterForMemory, -4,
			reg, 1, reg)
		// pinsrd $0, reg, result
		// pshufd $0, result, result (result = result[0,0,0,0])
		c.assembler.CompileRegisterToRegisterWithArg(amd64.PINSRD, reg, result, 0)
		c.assembler.CompileRegisterToRegisterWithArg(amd64.PSHUFD, result, result, 0)
	case wazeroir.V128LoadType64Splat:
		reg, err := c.compileMemoryAccessCeilSetup(o.Arg.Offset, 8)
		if err != nil {
			return err
		}
		c.assembler.CompileMemoryWithIndexToRegister(amd64.MOVQ, amd64ReservedRegisterForMemory, -8,
			reg, 1, reg)
		// pinsrq $0, reg, result
		// pinsrq $1, reg, result
		c.assembler.CompileRegisterToRegisterWithArg(amd64.PINSRQ, reg, result, 0)
		c.assembler.CompileRegisterToRegisterWithArg(amd64.PINSRQ, reg, result, 1)
	case wazeroir.V128LoadType32zero:
		err = c.compileV128LoadImpl(amd64.MOVL, o.Arg.Offset, 4, result)
	case wazeroir.V128LoadType64zero:
		err = c.compileV128LoadImpl(amd64.MOVQ, o.Arg.Offset, 8, result)
	}

	if err != nil {
		return err
	}

	c.pushVectorRuntimeValueLocationOnRegister(result)
	return nil
}

func (c *amd64Compiler) compileV128LoadImpl(inst asm.Instruction, offset uint32, targetSizeInBytes int64, dst asm.Register) error {
	offsetReg, err := c.compileMemoryAccessCeilSetup(offset, targetSizeInBytes)
	if err != nil {
		return err
	}
	c.assembler.CompileMemoryWithIndexToRegister(inst, amd64ReservedRegisterForMemory, -targetSizeInBytes,
		offsetReg, 1, dst)
	return nil
}

// compileV128LoadLane implements compiler.compileV128LoadLane for amd64.
func (c *amd64Compiler) compileV128LoadLane(o wazeroir.OperationV128LoadLane) error {
	targetVector := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(targetVector); err != nil {
		return err
	}

	var insertInst asm.Instruction
	switch o.LaneSize {
	case 8:
		insertInst = amd64.PINSRB
	case 16:
		insertInst = amd64.PINSRW
	case 32:
		insertInst = amd64.PINSRD
	case 64:
		insertInst = amd64.PINSRQ
	}

	targetSizeInBytes := int64(o.LaneSize / 8)
	offsetReg, err := c.compileMemoryAccessCeilSetup(o.Arg.Offset, targetSizeInBytes)
	if err != nil {
		return err
	}
	c.assembler.CompileMemoryWithIndexAndArgToRegister(insertInst, amd64ReservedRegisterForMemory, -targetSizeInBytes,
		offsetReg, 1, targetVector.register, o.LaneIndex)

	c.pushVectorRuntimeValueLocationOnRegister(targetVector.register)
	return nil
}

// compileV128Store implements compiler.compileV128Store for amd64.
func (c *amd64Compiler) compileV128Store(o wazeroir.OperationV128Store) error {
	val := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(val); err != nil {
		return err
	}

	const targetSizeInBytes = 16
	offsetReg, err := c.compileMemoryAccessCeilSetup(o.Arg.Offset, targetSizeInBytes)
	if err != nil {
		return err
	}

	c.assembler.CompileRegisterToMemoryWithIndex(amd64.MOVDQU, val.register,
		amd64ReservedRegisterForMemory, -targetSizeInBytes, offsetReg, 1)

	c.locationStack.markRegisterUnused(val.register, offsetReg)
	return nil
}

// compileV128StoreLane implements compiler.compileV128StoreLane for amd64.
func (c *amd64Compiler) compileV128StoreLane(o wazeroir.OperationV128StoreLane) error {
	var storeInst asm.Instruction
	switch o.LaneSize {
	case 8:
		storeInst = amd64.PEXTRB
	case 16:
		storeInst = amd64.PEXTRW
	case 32:
		storeInst = amd64.PEXTRD
	case 64:
		storeInst = amd64.PEXTRQ
	}

	val := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(val); err != nil {
		return err
	}

	targetSizeInBytes := int64(o.LaneSize / 8)
	offsetReg, err := c.compileMemoryAccessCeilSetup(o.Arg.Offset, targetSizeInBytes)
	if err != nil {
		return err
	}

	c.assembler.CompileRegisterToMemoryWithIndexAndArg(storeInst, val.register,
		amd64ReservedRegisterForMemory, -targetSizeInBytes, offsetReg, 1, o.LaneIndex)

	c.locationStack.markRegisterUnused(val.register, offsetReg)
	return nil
}

// compileV128ExtractLane implements compiler.compileV128ExtractLane for amd64.
func (c *amd64Compiler) compileV128ExtractLane(o wazeroir.OperationV128ExtractLane) error {
	v := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(v); err != nil {
		return err
	}
	vreg := v.register
	switch o.Shape {
	case wazeroir.ShapeI8x16:
		result, err := c.allocateRegister(registerTypeGeneralPurpose)
		if err != nil {
			return err
		}
		c.assembler.CompileRegisterToRegisterWithArg(amd64.PEXTRB, vreg, result, o.LaneIndex)
		if o.Signed {
			c.assembler.CompileRegisterToRegister(amd64.MOVBLSX, result, result)
		} else {
			c.assembler.CompileRegisterToRegister(amd64.MOVBLZX, result, result)
		}
		c.pushRuntimeValueLocationOnRegister(result, runtimeValueTypeI32)
		c.locationStack.markRegisterUnused(vreg)
	case wazeroir.ShapeI16x8:
		result, err := c.allocateRegister(registerTypeGeneralPurpose)
		if err != nil {
			return err
		}
		c.assembler.CompileRegisterToRegisterWithArg(amd64.PEXTRW, vreg, result, o.LaneIndex)
		if o.Signed {
			c.assembler.CompileRegisterToRegister(amd64.MOVWLSX, result, result)
		} else {
			c.assembler.CompileRegisterToRegister(amd64.MOVWLZX, result, result)
		}
		c.pushRuntimeValueLocationOnRegister(result, runtimeValueTypeI32)
		c.locationStack.markRegisterUnused(vreg)
	case wazeroir.ShapeI32x4:
		result, err := c.allocateRegister(registerTypeGeneralPurpose)
		if err != nil {
			return err
		}
		c.assembler.CompileRegisterToRegisterWithArg(amd64.PEXTRD, vreg, result, o.LaneIndex)
		c.pushRuntimeValueLocationOnRegister(result, runtimeValueTypeI32)
		c.locationStack.markRegisterUnused(vreg)
	case wazeroir.ShapeI64x2:
		result, err := c.allocateRegister(registerTypeGeneralPurpose)
		if err != nil {
			return err
		}
		c.assembler.CompileRegisterToRegisterWithArg(amd64.PEXTRQ, vreg, result, o.LaneIndex)
		c.pushRuntimeValueLocationOnRegister(result, runtimeValueTypeI64)
		c.locationStack.markRegisterUnused(vreg)
	case wazeroir.ShapeF32x4:
		if o.LaneIndex != 0 {
			c.assembler.CompileRegisterToRegisterWithArg(amd64.PSHUFD, vreg, vreg, o.LaneIndex)
		}
		c.pushRuntimeValueLocationOnRegister(vreg, runtimeValueTypeF32)
	case wazeroir.ShapeF64x2:
		if o.LaneIndex != 0 {
			// This case we can assume LaneIndex == 1.
			// We have to modify the val.register as, for example:
			//    0b11 0b10 0b01 0b00
			//     |    |    |    |
			//   [x3,  x2,  x1,  x0] -> [x0,  x0,  x3,  x2]
			// where val.register = [x3, x2, x1, x0] and each xN = 32bits.
			// Then, we interpret the register as float64, therefore, the float64 value is obtained as [x3, x2].
			arg := byte(0b00_00_11_10)
			c.assembler.CompileRegisterToRegisterWithArg(amd64.PSHUFD, vreg, vreg, arg)
		}
		c.pushRuntimeValueLocationOnRegister(vreg, runtimeValueTypeF64)
	}

	return nil
}

// compileV128ReplaceLane implements compiler.compileV128ReplaceLane for amd64.
func (c *amd64Compiler) compileV128ReplaceLane(o wazeroir.OperationV128ReplaceLane) error {
	origin := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(origin); err != nil {
		return err
	}

	vector := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(vector); err != nil {
		return err
	}

	switch o.Shape {
	case wazeroir.ShapeI8x16:
		c.assembler.CompileRegisterToRegisterWithArg(amd64.PINSRB, origin.register, vector.register, o.LaneIndex)
	case wazeroir.ShapeI16x8:
		c.assembler.CompileRegisterToRegisterWithArg(amd64.PINSRW, origin.register, vector.register, o.LaneIndex)
	case wazeroir.ShapeI32x4:
		c.assembler.CompileRegisterToRegisterWithArg(amd64.PINSRD, origin.register, vector.register, o.LaneIndex)
	case wazeroir.ShapeI64x2:
		c.assembler.CompileRegisterToRegisterWithArg(amd64.PINSRQ, origin.register, vector.register, o.LaneIndex)
	case wazeroir.ShapeF32x4:
		c.assembler.CompileRegisterToRegisterWithArg(amd64.INSERTPS, origin.register, vector.register,
			// In INSERTPS instruction, the destination index is encoded at 4 and 5 bits of the argument.
			// See https://www.felixcloutier.com/x86/insertps
			o.LaneIndex<<4,
		)
	case wazeroir.ShapeF64x2:
		if o.LaneIndex == 0 {
			c.assembler.CompileRegisterToRegister(amd64.MOVSD, origin.register, vector.register)
		} else {
			c.assembler.CompileRegisterToRegister(amd64.MOVLHPS, origin.register, vector.register)
		}
	}

	c.pushVectorRuntimeValueLocationOnRegister(vector.register)
	c.locationStack.markRegisterUnused(origin.register)
	return nil
}

// compileV128Splat implements compiler.compileV128Splat for amd64.
func (c *amd64Compiler) compileV128Splat(o wazeroir.OperationV128Splat) (err error) {
	origin := c.locationStack.pop()
	if err = c.compileEnsureOnRegister(origin); err != nil {
		return
	}

	var result asm.Register
	switch o.Shape {
	case wazeroir.ShapeI8x16:
		result, err = c.allocateRegister(registerTypeVector)
		if err != nil {
			return err
		}
		c.locationStack.markRegisterUsed(result)

		tmp, err := c.allocateRegister(registerTypeVector)
		if err != nil {
			return err
		}
		c.assembler.CompileRegisterToRegisterWithArg(amd64.PINSRB, origin.register, result, 0)
		c.assembler.CompileRegisterToRegister(amd64.PXOR, tmp, tmp)
		c.assembler.CompileRegisterToRegister(amd64.PSHUFB, tmp, result)
	case wazeroir.ShapeI16x8:
		result, err = c.allocateRegister(registerTypeVector)
		if err != nil {
			return err
		}
		c.locationStack.markRegisterUsed(result)
		c.assembler.CompileRegisterToRegisterWithArg(amd64.PINSRW, origin.register, result, 0)
		c.assembler.CompileRegisterToRegisterWithArg(amd64.PINSRW, origin.register, result, 1)
		c.assembler.CompileRegisterToRegisterWithArg(amd64.PSHUFD, result, result, 0)
	case wazeroir.ShapeI32x4:
		result, err = c.allocateRegister(registerTypeVector)
		if err != nil {
			return err
		}
		c.locationStack.markRegisterUsed(result)
		c.assembler.CompileRegisterToRegisterWithArg(amd64.PINSRD, origin.register, result, 0)
		c.assembler.CompileRegisterToRegisterWithArg(amd64.PSHUFD, result, result, 0)
	case wazeroir.ShapeI64x2:
		result, err = c.allocateRegister(registerTypeVector)
		if err != nil {
			return err
		}
		c.locationStack.markRegisterUsed(result)
		c.assembler.CompileRegisterToRegisterWithArg(amd64.PINSRQ, origin.register, result, 0)
		c.assembler.CompileRegisterToRegisterWithArg(amd64.PINSRQ, origin.register, result, 1)
	case wazeroir.ShapeF32x4:
		result = origin.register
		c.assembler.CompileRegisterToRegisterWithArg(amd64.INSERTPS, origin.register, result, 0)
		c.assembler.CompileRegisterToRegisterWithArg(amd64.PSHUFD, result, result, 0)
	case wazeroir.ShapeF64x2:
		result = origin.register
		c.assembler.CompileRegisterToRegister(amd64.MOVQ, origin.register, result)
		c.assembler.CompileRegisterToRegister(amd64.MOVLHPS, origin.register, result)
	}

	c.locationStack.markRegisterUnused(origin.register)
	c.pushVectorRuntimeValueLocationOnRegister(result)
	return nil
}

// compileV128Shuffle implements compiler.compileV128Shuffle for amd64.
func (c *amd64Compiler) compileV128Shuffle(o wazeroir.OperationV128Shuffle) error {
	w := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(w); err != nil {
		return err
	}

	v := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(v); err != nil {
		return err
	}

	wr, vr := w.register, v.register

	tmp, err := c.allocateRegister(registerTypeVector)
	if err != nil {
		return err
	}

	consts := [32]byte{}
	for i, lane := range o.Lanes {
		if lane < 16 {
			consts[i+16] = 0x80
			consts[i] = lane
		} else {
			consts[i+16] = lane - 16
			consts[i] = 0x80
		}
	}

	err = c.assembler.CompileStaticConstToRegister(amd64.MOVDQU, asm.NewStaticConst(consts[:16]), tmp)
	if err != nil {
		return err
	}
	c.assembler.CompileRegisterToRegister(amd64.PSHUFB, tmp, vr)
	err = c.assembler.CompileStaticConstToRegister(amd64.MOVDQU, asm.NewStaticConst(consts[16:]), tmp)
	if err != nil {
		return err
	}
	c.assembler.CompileRegisterToRegister(amd64.PSHUFB, tmp, wr)
	c.assembler.CompileRegisterToRegister(amd64.ORPS, vr, wr)

	c.pushVectorRuntimeValueLocationOnRegister(wr)
	c.locationStack.markRegisterUnused(vr)
	return nil
}

var swizzleConst = [16]byte{
	0x70, 0x70, 0x70, 0x70, 0x70, 0x70, 0x70, 0x70,
	0x70, 0x70, 0x70, 0x70, 0x70, 0x70, 0x70, 0x70,
}

// compileV128Swizzle implements compiler.compileV128Swizzle for amd64.
func (c *amd64Compiler) compileV128Swizzle(wazeroir.OperationV128Swizzle) error {
	index := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(index); err != nil {
		return err
	}

	base := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(base); err != nil {
		return err
	}

	idxReg, baseReg := index.register, base.register

	tmp, err := c.allocateRegister(registerTypeVector)
	if err != nil {
		return err
	}

	err = c.assembler.CompileStaticConstToRegister(amd64.MOVDQU, asm.NewStaticConst(swizzleConst[:]), tmp)
	if err != nil {
		return err
	}

	c.assembler.CompileRegisterToRegister(amd64.PADDUSB, tmp, idxReg)
	c.assembler.CompileRegisterToRegister(amd64.PSHUFB, idxReg, baseReg)

	c.pushVectorRuntimeValueLocationOnRegister(baseReg)
	c.locationStack.markRegisterUnused(idxReg)
	return nil
}

// compileV128AnyTrue implements compiler.compileV128AnyTrue for amd64.
func (c *amd64Compiler) compileV128AnyTrue(wazeroir.OperationV128AnyTrue) error {
	v := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(v); err != nil {
		return err
	}
	vreg := v.register

	c.assembler.CompileRegisterToRegister(amd64.PTEST, vreg, vreg)

	c.locationStack.pushRuntimeValueLocationOnConditionalRegister(amd64.ConditionalRegisterStateNE)
	c.locationStack.markRegisterUnused(vreg)
	return nil
}

// compileV128AllTrue implements compiler.compileV128AllTrue for amd64.
func (c *amd64Compiler) compileV128AllTrue(o wazeroir.OperationV128AllTrue) error {
	v := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(v); err != nil {
		return err
	}

	tmp, err := c.allocateRegister(registerTypeVector)
	if err != nil {
		return err
	}

	var cmpInst asm.Instruction
	switch o.Shape {
	case wazeroir.ShapeI8x16:
		cmpInst = amd64.PCMPEQB
	case wazeroir.ShapeI16x8:
		cmpInst = amd64.PCMPEQW
	case wazeroir.ShapeI32x4:
		cmpInst = amd64.PCMPEQD
	case wazeroir.ShapeI64x2:
		cmpInst = amd64.PCMPEQQ
	}

	c.assembler.CompileRegisterToRegister(amd64.PXOR, tmp, tmp)
	c.assembler.CompileRegisterToRegister(cmpInst, v.register, tmp)
	c.assembler.CompileRegisterToRegister(amd64.PTEST, tmp, tmp)
	c.locationStack.markRegisterUnused(v.register, tmp)
	c.locationStack.pushRuntimeValueLocationOnConditionalRegister(amd64.ConditionalRegisterStateE)
	return nil
}

// compileV128BitMask implements compiler.compileV128BitMask for amd64.
func (c *amd64Compiler) compileV128BitMask(o wazeroir.OperationV128BitMask) error {
	v := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(v); err != nil {
		return err
	}

	result, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}

	switch o.Shape {
	case wazeroir.ShapeI8x16:
		c.assembler.CompileRegisterToRegister(amd64.PMOVMSKB, v.register, result)
	case wazeroir.ShapeI16x8:
		// When we have:
		// 	R1 = [R1(w1), R1(w2), R1(w3), R1(w4), R1(w5), R1(w6), R1(w7), R1(v8)]
		// 	R2 = [R2(w1), R2(w2), R2(w3), R2(v4), R2(w5), R2(w6), R2(w7), R2(v8)]
		//	where RX(wn) is n-th signed word (16-bit) of RX register,
		//
		// "PACKSSWB R1, R2" produces
		//  R1 = [
		// 		byte_sat(R1(w1)), byte_sat(R1(w2)), byte_sat(R1(w3)), byte_sat(R1(w4)),
		// 		byte_sat(R1(w5)), byte_sat(R1(w6)), byte_sat(R1(w7)), byte_sat(R1(w8)),
		// 		byte_sat(R2(w1)), byte_sat(R2(w2)), byte_sat(R2(w3)), byte_sat(R2(w4)),
		// 		byte_sat(R2(w5)), byte_sat(R2(w6)), byte_sat(R2(w7)), byte_sat(R2(w8)),
		//  ]
		//  where R1 is the destination register, and
		// 	byte_sat(w) = int8(w) if w fits as signed 8-bit,
		//                0x80 if w is less than 0x80
		//                0x7F if w is greater than 0x7f
		//
		// See https://www.felixcloutier.com/x86/packsswb:packssdw for detail.
		//
		// Therefore, v.register ends up having i-th and (i+8)-th bit set if i-th lane is negative (for i in 0..8).
		c.assembler.CompileRegisterToRegister(amd64.PACKSSWB, v.register, v.register)
		c.assembler.CompileRegisterToRegister(amd64.PMOVMSKB, v.register, result)
		// Clear the higher bits than 8.
		c.assembler.CompileConstToRegister(amd64.SHRQ, 8, result)
	case wazeroir.ShapeI32x4:
		c.assembler.CompileRegisterToRegister(amd64.MOVMSKPS, v.register, result)
	case wazeroir.ShapeI64x2:
		c.assembler.CompileRegisterToRegister(amd64.MOVMSKPD, v.register, result)
	}

	c.locationStack.markRegisterUnused(v.register)
	c.pushRuntimeValueLocationOnRegister(result, runtimeValueTypeI32)
	return nil
}

// compileV128And implements compiler.compileV128And for amd64.
func (c *amd64Compiler) compileV128And(wazeroir.OperationV128And) error {
	x2 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x2); err != nil {
		return err
	}

	x1 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x1); err != nil {
		return err
	}

	c.assembler.CompileRegisterToRegister(amd64.PAND, x2.register, x1.register)

	c.locationStack.markRegisterUnused(x2.register)
	c.pushVectorRuntimeValueLocationOnRegister(x1.register)
	return nil
}

// compileV128Not implements compiler.compileV128Not for amd64.
func (c *amd64Compiler) compileV128Not(wazeroir.OperationV128Not) error {
	v := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(v); err != nil {
		return err
	}

	tmp, err := c.allocateRegister(registerTypeVector)
	if err != nil {
		return err
	}

	// Set all bits on tmp register.
	c.assembler.CompileRegisterToRegister(amd64.PCMPEQD, tmp, tmp)
	// Then XOR with tmp to reverse all bits on v.register.
	c.assembler.CompileRegisterToRegister(amd64.PXOR, tmp, v.register)
	c.pushVectorRuntimeValueLocationOnRegister(v.register)
	return nil
}

// compileV128Or implements compiler.compileV128Or for amd64.
func (c *amd64Compiler) compileV128Or(wazeroir.OperationV128Or) error {
	x2 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x2); err != nil {
		return err
	}

	x1 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x1); err != nil {
		return err
	}

	c.assembler.CompileRegisterToRegister(amd64.POR, x2.register, x1.register)

	c.locationStack.markRegisterUnused(x2.register)
	c.pushVectorRuntimeValueLocationOnRegister(x1.register)
	return nil
}

// compileV128Xor implements compiler.compileV128Xor for amd64.
func (c *amd64Compiler) compileV128Xor(wazeroir.OperationV128Xor) error {
	x2 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x2); err != nil {
		return err
	}

	x1 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x1); err != nil {
		return err
	}

	c.assembler.CompileRegisterToRegister(amd64.PXOR, x2.register, x1.register)

	c.locationStack.markRegisterUnused(x2.register)
	c.pushVectorRuntimeValueLocationOnRegister(x1.register)
	return nil
}

// compileV128Bitselect implements compiler.compileV128Bitselect for amd64.
func (c *amd64Compiler) compileV128Bitselect(wazeroir.OperationV128Bitselect) error {
	selector := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(selector); err != nil {
		return err
	}

	x2 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x2); err != nil {
		return err
	}

	x1 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x1); err != nil {
		return err
	}

	// The following logic is equivalent to v128.or(v128.and(v1, selector), v128.and(v2, v128.not(selector)))
	// See https://github.com/WebAssembly/spec/blob/wg-2.0.draft1/proposals/simd/SIMD.md#bitwise-select
	c.assembler.CompileRegisterToRegister(amd64.PAND, selector.register, x1.register)
	c.assembler.CompileRegisterToRegister(amd64.PANDN, x2.register, selector.register)
	c.assembler.CompileRegisterToRegister(amd64.POR, selector.register, x1.register)

	c.locationStack.markRegisterUnused(x2.register, selector.register)
	c.pushVectorRuntimeValueLocationOnRegister(x1.register)
	return nil
}

// compileV128AndNot implements compiler.compileV128AndNot for amd64.
func (c *amd64Compiler) compileV128AndNot(wazeroir.OperationV128AndNot) error {
	x2 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x2); err != nil {
		return err
	}

	x1 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x1); err != nil {
		return err
	}

	c.assembler.CompileRegisterToRegister(amd64.PANDN, x1.register, x2.register)

	c.locationStack.markRegisterUnused(x1.register)
	c.pushVectorRuntimeValueLocationOnRegister(x2.register)
	return nil
}

// compileV128Shr implements compiler.compileV128Shr for amd64.
func (c *amd64Compiler) compileV128Shr(o wazeroir.OperationV128Shr) error {
	// https://stackoverflow.com/questions/35002937/sse-simd-shift-with-one-byte-element-size-granularity
	if o.Shape == wazeroir.ShapeI8x16 {
		return c.compileV128ShrI8x16Impl(o.Signed)
	} else if o.Shape == wazeroir.ShapeI64x2 && o.Signed {
		return c.compileV128ShrI64x2SignedImpl()
	} else {
		return c.compileV128ShrImpl(o)
	}
}

// compileV128ShrImpl implements shift right instructions except for i8x16 (logical/arithmetic) and i64x2 (arithmetic).
func (c *amd64Compiler) compileV128ShrImpl(o wazeroir.OperationV128Shr) error {
	s := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(s); err != nil {
		return err
	}

	x1 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x1); err != nil {
		return err
	}

	vecTmp, err := c.allocateRegister(registerTypeVector)
	if err != nil {
		return err
	}

	var moduleConst int64
	var shift asm.Instruction
	switch o.Shape {
	case wazeroir.ShapeI16x8:
		moduleConst = 0xf // modulo 16.
		if o.Signed {
			shift = amd64.PSRAW
		} else {
			shift = amd64.PSRLW
		}
	case wazeroir.ShapeI32x4:
		moduleConst = 0x1f // modulo 32.
		if o.Signed {
			shift = amd64.PSRAD
		} else {
			shift = amd64.PSRLD
		}
	case wazeroir.ShapeI64x2:
		moduleConst = 0x3f // modulo 64.
		shift = amd64.PSRLQ
	}

	gpShiftAmount := s.register
	c.assembler.CompileConstToRegister(amd64.ANDQ, moduleConst, gpShiftAmount)
	c.assembler.CompileRegisterToRegister(amd64.MOVL, gpShiftAmount, vecTmp)
	c.assembler.CompileRegisterToRegister(shift, vecTmp, x1.register)

	c.locationStack.markRegisterUnused(gpShiftAmount)
	c.pushVectorRuntimeValueLocationOnRegister(x1.register)
	return nil
}

// compileV128ShrI64x2SignedImpl implements compiler.compileV128Shr for i64x2 signed (arithmetic) shift.
// PSRAQ instruction requires AVX, so we emulate it without AVX instructions. https://www.felixcloutier.com/x86/psraw:psrad:psraq
func (c *amd64Compiler) compileV128ShrI64x2SignedImpl() error {
	const shiftCountRegister = amd64.RegCX

	s := c.locationStack.pop()
	if s.register != shiftCountRegister {
		// If another value lives on the CX register, we release it to the stack.
		c.onValueReleaseRegisterToStack(shiftCountRegister)
		if s.onStack() {
			s.setRegister(shiftCountRegister)
			c.compileLoadValueOnStackToRegister(s)
		} else if s.onConditionalRegister() {
			c.compileMoveConditionalToGeneralPurposeRegister(s, shiftCountRegister)
		} else { // already on register.
			old := s.register
			c.assembler.CompileRegisterToRegister(amd64.MOVL, old, shiftCountRegister)
			s.setRegister(shiftCountRegister)
			c.locationStack.markRegisterUnused(old)
		}
	}

	c.locationStack.markRegisterUsed(shiftCountRegister)
	tmp, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}

	x1 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x1); err != nil {
		return err
	}

	// Extract each lane into tmp, execute SHR on tmp, and write it back to the lane.
	c.assembler.CompileRegisterToRegisterWithArg(amd64.PEXTRQ, x1.register, tmp, 0)
	c.assembler.CompileRegisterToRegister(amd64.SARQ, shiftCountRegister, tmp)
	c.assembler.CompileRegisterToRegisterWithArg(amd64.PINSRQ, tmp, x1.register, 0)
	c.assembler.CompileRegisterToRegisterWithArg(amd64.PEXTRQ, x1.register, tmp, 1)
	c.assembler.CompileRegisterToRegister(amd64.SARQ, shiftCountRegister, tmp)
	c.assembler.CompileRegisterToRegisterWithArg(amd64.PINSRQ, tmp, x1.register, 1)

	c.locationStack.markRegisterUnused(shiftCountRegister)
	c.pushVectorRuntimeValueLocationOnRegister(x1.register)
	return nil
}

// i8x16LogicalSHRMaskTable is necessary for emulating non-existent packed bytes logical right shifts on amd64.
// The mask is applied after performing packed word shifts on the value to clear out the unnecessary bits.
var i8x16LogicalSHRMaskTable = [8 * 16]byte{ // (the number of possible shift amount 0, 1, ..., 7.) * 16 bytes.
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, // for 0 shift
	0x7f, 0x7f, 0x7f, 0x7f, 0x7f, 0x7f, 0x7f, 0x7f, 0x7f, 0x7f, 0x7f, 0x7f, 0x7f, 0x7f, 0x7f, 0x7f, // for 1 shift
	0x3f, 0x3f, 0x3f, 0x3f, 0x3f, 0x3f, 0x3f, 0x3f, 0x3f, 0x3f, 0x3f, 0x3f, 0x3f, 0x3f, 0x3f, 0x3f, // for 2 shift
	0x1f, 0x1f, 0x1f, 0x1f, 0x1f, 0x1f, 0x1f, 0x1f, 0x1f, 0x1f, 0x1f, 0x1f, 0x1f, 0x1f, 0x1f, 0x1f, // for 3 shift
	0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, // for 4 shift
	0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07, // for 5 shift
	0x03, 0x03, 0x03, 0x03, 0x03, 0x03, 0x03, 0x03, 0x03, 0x03, 0x03, 0x03, 0x03, 0x03, 0x03, 0x03, // for 6 shift
	0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, // for 7 shift
}

// compileV128ShrI64x2SignedImpl implements compiler.compileV128Shr for i8x16 signed logical/arithmetic shifts.
// amd64 doesn't have packed byte shifts, so we need this special casing.
// See https://stackoverflow.com/questions/35002937/sse-simd-shift-with-one-byte-element-size-granularity
func (c *amd64Compiler) compileV128ShrI8x16Impl(signed bool) error {
	s := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(s); err != nil {
		return err
	}

	v := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(v); err != nil {
		return err
	}

	vecTmp, err := c.allocateRegister(registerTypeVector)
	if err != nil {
		return err
	}

	gpShiftAmount := s.register
	c.assembler.CompileConstToRegister(amd64.ANDQ, 0x7, gpShiftAmount) // mod 8.

	if signed {
		c.locationStack.markRegisterUsed(vecTmp)
		vecTmp2, err := c.allocateRegister(registerTypeVector)
		if err != nil {
			return err
		}

		vreg := v.register

		// Copy the value from v.register to vecTmp.
		c.assembler.CompileRegisterToRegister(amd64.MOVDQA, vreg, vecTmp)

		// Assuming that we have
		//  vreg   = [b1, ..., b16]
		//  vecTmp = [b1, ..., b16]
		// at this point, then we use PUNPCKLBW and PUNPCKHBW to produce:
		//  vreg   = [b1, b1, b2, b2, ..., b8, b8]
		//  vecTmp = [b9, b9, b10, b10, ..., b16, b16]
		c.assembler.CompileRegisterToRegister(amd64.PUNPCKLBW, vreg, vreg)
		c.assembler.CompileRegisterToRegister(amd64.PUNPCKHBW, vecTmp, vecTmp)

		// Adding 8 to the shift amount, and then move the amount to vecTmp2.
		c.assembler.CompileConstToRegister(amd64.ADDQ, 0x8, gpShiftAmount)
		c.assembler.CompileRegisterToRegister(amd64.MOVL, gpShiftAmount, vecTmp2)

		// Perform the word packed arithmetic right shifts on vreg and vecTmp.
		// This changes these two registers as:
		//  vreg   = [xxx, b1 >> s, xxx, b2 >> s, ..., xxx, b8 >> s]
		//  vecTmp = [xxx, b9 >> s, xxx, b10 >> s, ..., xxx, b16 >> s]
		// where xxx is 1 or 0 depending on each byte's sign, and ">>" is the arithmetic shift on a byte.
		c.assembler.CompileRegisterToRegister(amd64.PSRAW, vecTmp2, vreg)
		c.assembler.CompileRegisterToRegister(amd64.PSRAW, vecTmp2, vecTmp)

		// Finally, we can get the result by packing these two word vectors.
		c.assembler.CompileRegisterToRegister(amd64.PACKSSWB, vecTmp, vreg)

		c.locationStack.markRegisterUnused(gpShiftAmount, vecTmp)
		c.pushVectorRuntimeValueLocationOnRegister(vreg)
	} else {
		c.assembler.CompileRegisterToRegister(amd64.MOVL, gpShiftAmount, vecTmp)
		// amd64 doesn't have packed byte shifts, so we packed word shift here, and then mark-out
		// the unnecessary bits below.
		c.assembler.CompileRegisterToRegister(amd64.PSRLW, vecTmp, v.register)

		gpTmp, err := c.allocateRegister(registerTypeGeneralPurpose)
		if err != nil {
			return err
		}

		// Read the initial address of the mask table into gpTmp register.
		err = c.assembler.CompileStaticConstToRegister(amd64.LEAQ, asm.NewStaticConst(i8x16LogicalSHRMaskTable[:]), gpTmp)
		if err != nil {
			return err
		}

		// We have to get the mask according to the shift amount, so we first have to do
		// gpShiftAmount << 4 = gpShiftAmount*16 to get the initial offset of the mask (16 is the size of each mask in bytes).
		c.assembler.CompileConstToRegister(amd64.SHLQ, 4, gpShiftAmount)

		// Now ready to read the content of the mask into the vecTmp.
		c.assembler.CompileMemoryWithIndexToRegister(amd64.MOVDQU,
			gpTmp, 0, gpShiftAmount, 1,
			vecTmp,
		)

		// Finally, clear out the unnecessary
		c.assembler.CompileRegisterToRegister(amd64.PAND, vecTmp, v.register)

		c.locationStack.markRegisterUnused(gpShiftAmount)
		c.pushVectorRuntimeValueLocationOnRegister(v.register)
	}
	return nil
}

// i8x16SHLMaskTable is necessary for emulating non-existent packed bytes left shifts on amd64.
// The mask is applied after performing packed word shifts on the value to clear out the unnecessary bits.
var i8x16SHLMaskTable = [8 * 16]byte{ // (the number of possible shift amount 0, 1, ..., 7.) * 16 bytes.
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, // for 0 shift
	0xfe, 0xfe, 0xfe, 0xfe, 0xfe, 0xfe, 0xfe, 0xfe, 0xfe, 0xfe, 0xfe, 0xfe, 0xfe, 0xfe, 0xfe, 0xfe, // for 1 shift
	0xfc, 0xfc, 0xfc, 0xfc, 0xfc, 0xfc, 0xfc, 0xfc, 0xfc, 0xfc, 0xfc, 0xfc, 0xfc, 0xfc, 0xfc, 0xfc, // for 2 shift
	0xf8, 0xf8, 0xf8, 0xf8, 0xf8, 0xf8, 0xf8, 0xf8, 0xf8, 0xf8, 0xf8, 0xf8, 0xf8, 0xf8, 0xf8, 0xf8, // for 3 shift
	0xf0, 0xf0, 0xf0, 0xf0, 0xf0, 0xf0, 0xf0, 0xf0, 0xf0, 0xf0, 0xf0, 0xf0, 0xf0, 0xf0, 0xf0, 0xf0, // for 4 shift
	0xe0, 0xe0, 0xe0, 0xe0, 0xe0, 0xe0, 0xe0, 0xe0, 0xe0, 0xe0, 0xe0, 0xe0, 0xe0, 0xe0, 0xe0, 0xe0, // for 5 shift
	0xc0, 0xc0, 0xc0, 0xc0, 0xc0, 0xc0, 0xc0, 0xc0, 0xc0, 0xc0, 0xc0, 0xc0, 0xc0, 0xc0, 0xc0, 0xc0, // for 6 shift
	0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, // for 7 shift
}

// compileV128Shl implements compiler.compileV128Shl for amd64.
func (c *amd64Compiler) compileV128Shl(o wazeroir.OperationV128Shl) error {
	s := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(s); err != nil {
		return err
	}

	x1 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x1); err != nil {
		return err
	}

	vecTmp, err := c.allocateRegister(registerTypeVector)
	if err != nil {
		return err
	}

	var modulo int64
	var shift asm.Instruction
	switch o.Shape {
	case wazeroir.ShapeI8x16:
		modulo = 0x7 // modulo 8.
		// x86 doesn't have packed bytes shift, so we use PSLLW and mask-out the redundant bits.
		// See https://stackoverflow.com/questions/35002937/sse-simd-shift-with-one-byte-element-size-granularity
		shift = amd64.PSLLW
	case wazeroir.ShapeI16x8:
		modulo = 0xf // modulo 16.
		shift = amd64.PSLLW
	case wazeroir.ShapeI32x4:
		modulo = 0x1f // modulo 32.
		shift = amd64.PSLLD
	case wazeroir.ShapeI64x2:
		modulo = 0x3f // modulo 64.
		shift = amd64.PSLLQ
	}

	gpShiftAmount := s.register
	c.assembler.CompileConstToRegister(amd64.ANDQ, modulo, gpShiftAmount)
	c.assembler.CompileRegisterToRegister(amd64.MOVL, gpShiftAmount, vecTmp)
	c.assembler.CompileRegisterToRegister(shift, vecTmp, x1.register)

	if o.Shape == wazeroir.ShapeI8x16 {
		gpTmp, err := c.allocateRegister(registerTypeGeneralPurpose)
		if err != nil {
			return err
		}

		// Read the initial address of the mask table into gpTmp register.
		err = c.assembler.CompileStaticConstToRegister(amd64.LEAQ, asm.NewStaticConst(i8x16SHLMaskTable[:]), gpTmp)
		if err != nil {
			return err
		}

		// We have to get the mask according to the shift amount, so we first have to do
		// gpShiftAmount << 4 = gpShiftAmount*16 to get the initial offset of the mask (16 is the size of each mask in bytes).
		c.assembler.CompileConstToRegister(amd64.SHLQ, 4, gpShiftAmount)

		// Now ready to read the content of the mask into the vecTmp.
		c.assembler.CompileMemoryWithIndexToRegister(amd64.MOVDQU,
			gpTmp, 0, gpShiftAmount, 1,
			vecTmp,
		)

		// Finally, clear out the unnecessary
		c.assembler.CompileRegisterToRegister(amd64.PAND, vecTmp, x1.register)
	}

	c.locationStack.markRegisterUnused(gpShiftAmount)
	c.pushVectorRuntimeValueLocationOnRegister(x1.register)
	return nil
}

// compileV128Cmp implements compiler.compileV128Cmp for amd64.
func (c *amd64Compiler) compileV128Cmp(o wazeroir.OperationV128Cmp) error {
	x2 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x2); err != nil {
		return err
	}

	x1 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x1); err != nil {
		return err
	}

	const (
		// See https://www.felixcloutier.com/x86/cmppd and https://www.felixcloutier.com/x86/cmpps
		floatEqualArg           = 0
		floatLessThanArg        = 1
		floatLessThanOrEqualArg = 2
		floatNotEqualARg        = 4
	)

	x1Reg, x2Reg, result := x1.register, x2.register, asm.NilRegister
	switch o.Type {
	case wazeroir.V128CmpTypeF32x4Eq:
		c.assembler.CompileRegisterToRegisterWithArg(amd64.CMPPS, x2Reg, x1Reg, floatEqualArg)
		result = x1Reg
	case wazeroir.V128CmpTypeF32x4Ne:
		c.assembler.CompileRegisterToRegisterWithArg(amd64.CMPPS, x2Reg, x1Reg, floatNotEqualARg)
		result = x1Reg
	case wazeroir.V128CmpTypeF32x4Lt:
		c.assembler.CompileRegisterToRegisterWithArg(amd64.CMPPS, x2Reg, x1Reg, floatLessThanArg)
		result = x1Reg
	case wazeroir.V128CmpTypeF32x4Gt:
		// Without AVX, there's no float Gt instruction, so we swap the register and use Lt instead.
		c.assembler.CompileRegisterToRegisterWithArg(amd64.CMPPS, x1Reg, x2Reg, floatLessThanArg)
		result = x2Reg
	case wazeroir.V128CmpTypeF32x4Le:
		c.assembler.CompileRegisterToRegisterWithArg(amd64.CMPPS, x2Reg, x1Reg, floatLessThanOrEqualArg)
		result = x1Reg
	case wazeroir.V128CmpTypeF32x4Ge:
		// Without AVX, there's no float Ge instruction, so we swap the register and use Le instead.
		c.assembler.CompileRegisterToRegisterWithArg(amd64.CMPPS, x1Reg, x2Reg, floatLessThanOrEqualArg)
		result = x2Reg
	case wazeroir.V128CmpTypeF64x2Eq:
		c.assembler.CompileRegisterToRegisterWithArg(amd64.CMPPD, x2Reg, x1Reg, floatEqualArg)
		result = x1Reg
	case wazeroir.V128CmpTypeF64x2Ne:
		c.assembler.CompileRegisterToRegisterWithArg(amd64.CMPPD, x2Reg, x1Reg, floatNotEqualARg)
		result = x1Reg
	case wazeroir.V128CmpTypeF64x2Lt:
		c.assembler.CompileRegisterToRegisterWithArg(amd64.CMPPD, x2Reg, x1Reg, floatLessThanArg)
		result = x1Reg
	case wazeroir.V128CmpTypeF64x2Gt:
		// Without AVX, there's no float Gt instruction, so we swap the register and use Lt instead.
		c.assembler.CompileRegisterToRegisterWithArg(amd64.CMPPD, x1Reg, x2Reg, floatLessThanArg)
		result = x2Reg
	case wazeroir.V128CmpTypeF64x2Le:
		c.assembler.CompileRegisterToRegisterWithArg(amd64.CMPPD, x2Reg, x1Reg, floatLessThanOrEqualArg)
		result = x1Reg
	case wazeroir.V128CmpTypeF64x2Ge:
		// Without AVX, there's no float Ge instruction, so we swap the register and use Le instead.
		c.assembler.CompileRegisterToRegisterWithArg(amd64.CMPPD, x1Reg, x2Reg, floatLessThanOrEqualArg)
		result = x2Reg
	case wazeroir.V128CmpTypeI8x16Eq:
		c.assembler.CompileRegisterToRegister(amd64.PCMPEQB, x2Reg, x1Reg)
		result = x1Reg
	case wazeroir.V128CmpTypeI8x16Ne:
		c.assembler.CompileRegisterToRegister(amd64.PCMPEQB, x2Reg, x1Reg)
		// Set all bits on x2Reg register.
		c.assembler.CompileRegisterToRegister(amd64.PCMPEQD, x2Reg, x2Reg)
		// Swap the bits on x1Reg register.
		c.assembler.CompileRegisterToRegister(amd64.PXOR, x2Reg, x1Reg)
		result = x1Reg
	case wazeroir.V128CmpTypeI8x16LtS:
		c.assembler.CompileRegisterToRegister(amd64.PCMPGTB, x1Reg, x2Reg)
		result = x2Reg
	case wazeroir.V128CmpTypeI8x16LtU, wazeroir.V128CmpTypeI8x16GtU:
		// Take the unsigned min/max values on each byte on x1 and x2 onto x1Reg.
		if o.Type == wazeroir.V128CmpTypeI8x16LtU {
			c.assembler.CompileRegisterToRegister(amd64.PMINUB, x2Reg, x1Reg)
		} else {
			c.assembler.CompileRegisterToRegister(amd64.PMAXUB, x2Reg, x1Reg)
		}
		c.assembler.CompileRegisterToRegister(amd64.PCMPEQB, x2Reg, x1Reg)
		// Set all bits on x2Reg register.
		c.assembler.CompileRegisterToRegister(amd64.PCMPEQD, x2Reg, x2Reg)
		// Swap the bits on x2Reg register.
		c.assembler.CompileRegisterToRegister(amd64.PXOR, x2Reg, x1Reg)
		result = x1Reg
	case wazeroir.V128CmpTypeI8x16GtS:
		c.assembler.CompileRegisterToRegister(amd64.PCMPGTB, x2Reg, x1Reg)
		result = x1Reg
	case wazeroir.V128CmpTypeI8x16LeS, wazeroir.V128CmpTypeI8x16LeU:
		tmp, err := c.allocateRegister(registerTypeVector)
		if err != nil {
			return err
		}
		// Copy the value on the src to tmp.
		c.assembler.CompileRegisterToRegister(amd64.MOVDQA, x1Reg, tmp)
		if o.Type == wazeroir.V128CmpTypeI8x16LeS {
			c.assembler.CompileRegisterToRegister(amd64.PMINSB, x2Reg, tmp)
		} else {
			c.assembler.CompileRegisterToRegister(amd64.PMINUB, x2Reg, tmp)
		}
		c.assembler.CompileRegisterToRegister(amd64.PCMPEQB, tmp, x1Reg)
		result = x1Reg
	case wazeroir.V128CmpTypeI8x16GeS, wazeroir.V128CmpTypeI8x16GeU:
		tmp, err := c.allocateRegister(registerTypeVector)
		if err != nil {
			return err
		}
		c.assembler.CompileRegisterToRegister(amd64.MOVDQA, x1Reg, tmp)
		if o.Type == wazeroir.V128CmpTypeI8x16GeS {
			c.assembler.CompileRegisterToRegister(amd64.PMAXSB, x2Reg, tmp)
		} else {
			c.assembler.CompileRegisterToRegister(amd64.PMAXUB, x2Reg, tmp)
		}
		c.assembler.CompileRegisterToRegister(amd64.PCMPEQB, tmp, x1Reg)
		result = x1Reg
	case wazeroir.V128CmpTypeI16x8Eq:
		c.assembler.CompileRegisterToRegister(amd64.PCMPEQW, x2Reg, x1Reg)
		result = x1Reg
	case wazeroir.V128CmpTypeI16x8Ne:
		c.assembler.CompileRegisterToRegister(amd64.PCMPEQW, x2Reg, x1Reg)
		// Set all bits on x2Reg register.
		c.assembler.CompileRegisterToRegister(amd64.PCMPEQD, x2Reg, x2Reg)
		// Swap the bits on x1Reg register.
		c.assembler.CompileRegisterToRegister(amd64.PXOR, x2Reg, x1Reg)
		result = x1Reg
	case wazeroir.V128CmpTypeI16x8LtS:
		c.assembler.CompileRegisterToRegister(amd64.PCMPGTW, x1Reg, x2Reg)
		result = x2Reg
	case wazeroir.V128CmpTypeI16x8LtU, wazeroir.V128CmpTypeI16x8GtU:
		// Take the unsigned min/max values on each byte on x1 and x2 onto x1Reg.
		if o.Type == wazeroir.V128CmpTypeI16x8LtU {
			c.assembler.CompileRegisterToRegister(amd64.PMINUW, x2Reg, x1Reg)
		} else {
			c.assembler.CompileRegisterToRegister(amd64.PMAXUW, x2Reg, x1Reg)
		}
		c.assembler.CompileRegisterToRegister(amd64.PCMPEQW, x2Reg, x1Reg)
		// Set all bits on x2Reg register.
		c.assembler.CompileRegisterToRegister(amd64.PCMPEQD, x2Reg, x2Reg)
		// Swap the bits on x2Reg register.
		c.assembler.CompileRegisterToRegister(amd64.PXOR, x2Reg, x1Reg)
		result = x1Reg
	case wazeroir.V128CmpTypeI16x8GtS:
		c.assembler.CompileRegisterToRegister(amd64.PCMPGTW, x2Reg, x1Reg)
		result = x1Reg
	case wazeroir.V128CmpTypeI16x8LeS, wazeroir.V128CmpTypeI16x8LeU:
		tmp, err := c.allocateRegister(registerTypeVector)
		if err != nil {
			return err
		}
		// Copy the value on the src to tmp.
		c.assembler.CompileRegisterToRegister(amd64.MOVDQA, x1Reg, tmp)
		if o.Type == wazeroir.V128CmpTypeI16x8LeS {
			c.assembler.CompileRegisterToRegister(amd64.PMINSW, x2Reg, tmp)
		} else {
			c.assembler.CompileRegisterToRegister(amd64.PMINUW, x2Reg, tmp)
		}
		c.assembler.CompileRegisterToRegister(amd64.PCMPEQW, tmp, x1Reg)
		result = x1Reg
	case wazeroir.V128CmpTypeI16x8GeS, wazeroir.V128CmpTypeI16x8GeU:
		tmp, err := c.allocateRegister(registerTypeVector)
		if err != nil {
			return err
		}
		c.assembler.CompileRegisterToRegister(amd64.MOVDQA, x1Reg, tmp)
		if o.Type == wazeroir.V128CmpTypeI16x8GeS {
			c.assembler.CompileRegisterToRegister(amd64.PMAXSW, x2Reg, tmp)
		} else {
			c.assembler.CompileRegisterToRegister(amd64.PMAXUW, x2Reg, tmp)
		}
		c.assembler.CompileRegisterToRegister(amd64.PCMPEQW, tmp, x1Reg)
		result = x1Reg
	case wazeroir.V128CmpTypeI32x4Eq:
		c.assembler.CompileRegisterToRegister(amd64.PCMPEQD, x2Reg, x1Reg)
		result = x1Reg
	case wazeroir.V128CmpTypeI32x4Ne:
		c.assembler.CompileRegisterToRegister(amd64.PCMPEQD, x2Reg, x1Reg)
		// Set all bits on x2Reg register.
		c.assembler.CompileRegisterToRegister(amd64.PCMPEQD, x2Reg, x2Reg)
		// Swap the bits on x1Reg register.
		c.assembler.CompileRegisterToRegister(amd64.PXOR, x2Reg, x1Reg)
		result = x1Reg
	case wazeroir.V128CmpTypeI32x4LtS:
		c.assembler.CompileRegisterToRegister(amd64.PCMPGTD, x1Reg, x2Reg)
		result = x2Reg
	case wazeroir.V128CmpTypeI32x4LtU, wazeroir.V128CmpTypeI32x4GtU:
		// Take the unsigned min/max values on each byte on x1 and x2 onto x1Reg.
		if o.Type == wazeroir.V128CmpTypeI32x4LtU {
			c.assembler.CompileRegisterToRegister(amd64.PMINUD, x2Reg, x1Reg)
		} else {
			c.assembler.CompileRegisterToRegister(amd64.PMAXUD, x2Reg, x1Reg)
		}
		c.assembler.CompileRegisterToRegister(amd64.PCMPEQD, x2Reg, x1Reg)
		// Set all bits on x2Reg register.
		c.assembler.CompileRegisterToRegister(amd64.PCMPEQD, x2Reg, x2Reg)
		// Swap the bits on x2Reg register.
		c.assembler.CompileRegisterToRegister(amd64.PXOR, x2Reg, x1Reg)
		result = x1Reg
	case wazeroir.V128CmpTypeI32x4GtS:
		c.assembler.CompileRegisterToRegister(amd64.PCMPGTD, x2Reg, x1Reg)
		result = x1Reg
	case wazeroir.V128CmpTypeI32x4LeS, wazeroir.V128CmpTypeI32x4LeU:
		tmp, err := c.allocateRegister(registerTypeVector)
		if err != nil {
			return err
		}
		// Copy the value on the src to tmp.
		c.assembler.CompileRegisterToRegister(amd64.MOVDQA, x1Reg, tmp)
		if o.Type == wazeroir.V128CmpTypeI32x4LeS {
			c.assembler.CompileRegisterToRegister(amd64.PMINSD, x2Reg, tmp)
		} else {
			c.assembler.CompileRegisterToRegister(amd64.PMINUD, x2Reg, tmp)
		}
		c.assembler.CompileRegisterToRegister(amd64.PCMPEQD, tmp, x1Reg)
		result = x1Reg
	case wazeroir.V128CmpTypeI32x4GeS, wazeroir.V128CmpTypeI32x4GeU:
		tmp, err := c.allocateRegister(registerTypeVector)
		if err != nil {
			return err
		}
		c.assembler.CompileRegisterToRegister(amd64.MOVDQA, x1Reg, tmp)
		if o.Type == wazeroir.V128CmpTypeI32x4GeS {
			c.assembler.CompileRegisterToRegister(amd64.PMAXSD, x2Reg, tmp)
		} else {
			c.assembler.CompileRegisterToRegister(amd64.PMAXUD, x2Reg, tmp)
		}
		c.assembler.CompileRegisterToRegister(amd64.PCMPEQD, tmp, x1Reg)
		result = x1Reg
	case wazeroir.V128CmpTypeI64x2Eq:
		c.assembler.CompileRegisterToRegister(amd64.PCMPEQQ, x2Reg, x1Reg)
		result = x1Reg
	case wazeroir.V128CmpTypeI64x2Ne:
		c.assembler.CompileRegisterToRegister(amd64.PCMPEQQ, x2Reg, x1Reg)
		// Set all bits on x2Reg register.
		c.assembler.CompileRegisterToRegister(amd64.PCMPEQD, x2Reg, x2Reg)
		// Swap the bits on x1Reg register.
		c.assembler.CompileRegisterToRegister(amd64.PXOR, x2Reg, x1Reg)
		result = x1Reg
	case wazeroir.V128CmpTypeI64x2LtS:
		c.assembler.CompileRegisterToRegister(amd64.PCMPGTQ, x1Reg, x2Reg)
		result = x2Reg
	case wazeroir.V128CmpTypeI64x2GtS:
		c.assembler.CompileRegisterToRegister(amd64.PCMPGTQ, x2Reg, x1Reg)
		result = x1Reg
	case wazeroir.V128CmpTypeI64x2LeS:
		c.assembler.CompileRegisterToRegister(amd64.PCMPGTQ, x2Reg, x1Reg)
		// Set all bits on x2Reg register.
		c.assembler.CompileRegisterToRegister(amd64.PCMPEQD, x2Reg, x2Reg)
		// Swap the bits on x1Reg register.
		c.assembler.CompileRegisterToRegister(amd64.PXOR, x2Reg, x1Reg)
		result = x1Reg
	case wazeroir.V128CmpTypeI64x2GeS:
		c.assembler.CompileRegisterToRegister(amd64.PCMPGTQ, x1Reg, x2Reg)
		// Set all bits on x1Reg register.
		c.assembler.CompileRegisterToRegister(amd64.PCMPEQD, x1Reg, x1Reg)
		// Swap the bits on x2Reg register.
		c.assembler.CompileRegisterToRegister(amd64.PXOR, x1Reg, x2Reg)
		result = x2Reg
	}

	c.locationStack.markRegisterUnused(x1Reg, x2Reg)
	c.pushVectorRuntimeValueLocationOnRegister(result)
	return nil
}

// compileV128AddSat implements compiler.compileV128AddSat for amd64.
func (c *amd64Compiler) compileV128AddSat(o wazeroir.OperationV128AddSat) error {
	var inst asm.Instruction
	switch o.Shape {
	case wazeroir.ShapeI8x16:
		if o.Signed {
			inst = amd64.PADDSB
		} else {
			inst = amd64.PADDUSB
		}
	case wazeroir.ShapeI16x8:
		if o.Signed {
			inst = amd64.PADDSW
		} else {
			inst = amd64.PADDUSW
		}
	}

	x2 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x2); err != nil {
		return err
	}

	x1 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x1); err != nil {
		return err
	}

	c.assembler.CompileRegisterToRegister(inst, x2.register, x1.register)

	c.locationStack.markRegisterUnused(x2.register)
	c.pushVectorRuntimeValueLocationOnRegister(x1.register)
	return nil
}

// compileV128SubSat implements compiler.compileV128SubSat for amd64.
func (c *amd64Compiler) compileV128SubSat(o wazeroir.OperationV128SubSat) error {
	var inst asm.Instruction
	switch o.Shape {
	case wazeroir.ShapeI8x16:
		if o.Signed {
			inst = amd64.PSUBSB
		} else {
			inst = amd64.PSUBUSB
		}
	case wazeroir.ShapeI16x8:
		if o.Signed {
			inst = amd64.PSUBSW
		} else {
			inst = amd64.PSUBUSW
		}
	}

	x2 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x2); err != nil {
		return err
	}

	x1 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x1); err != nil {
		return err
	}

	c.assembler.CompileRegisterToRegister(inst, x2.register, x1.register)

	c.locationStack.markRegisterUnused(x2.register)
	c.pushVectorRuntimeValueLocationOnRegister(x1.register)
	return nil
}

// compileV128Mul implements compiler.compileV128Mul for amd64.
func (c *amd64Compiler) compileV128Mul(o wazeroir.OperationV128Mul) error {
	var inst asm.Instruction
	switch o.Shape {
	case wazeroir.ShapeI16x8:
		inst = amd64.PMULLW
	case wazeroir.ShapeI32x4:
		inst = amd64.PMULLD
	case wazeroir.ShapeI64x2:
		return c.compileV128MulI64x2()
	case wazeroir.ShapeF32x4:
		inst = amd64.MULPS
	case wazeroir.ShapeF64x2:
		inst = amd64.MULPD
	}

	x2 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x2); err != nil {
		return err
	}

	x1 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x1); err != nil {
		return err
	}

	c.assembler.CompileRegisterToRegister(inst, x2.register, x1.register)

	c.locationStack.markRegisterUnused(x2.register)
	c.pushVectorRuntimeValueLocationOnRegister(x1.register)
	return nil
}

// compileV128MulI64x2 implements V128Mul for i64x2.
func (c *amd64Compiler) compileV128MulI64x2() error {
	x2 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x2); err != nil {
		return err
	}

	x1 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x1); err != nil {
		return err
	}

	x1r, x2r := x1.register, x2.register

	tmp1, err := c.allocateRegister(registerTypeVector)
	if err != nil {
		return err
	}

	c.locationStack.markRegisterUsed(tmp1)

	tmp2, err := c.allocateRegister(registerTypeVector)
	if err != nil {
		return err
	}

	// Assuming that we have
	//	x1r = [p1, p2] = [p1_lo, p1_hi, p2_lo, p2_high]
	//  x2r = [q1, q2] = [q1_lo, q1_hi, q2_lo, q2_high]
	// where pN and qN are 64-bit (quad word) lane, and pN_lo, pN_hi, qN_lo and qN_hi are 32-bit (double word) lane.

	// Copy x1's value into tmp1.
	c.assembler.CompileRegisterToRegister(amd64.MOVDQA, x1r, tmp1)
	// And do the logical right shift by 32-bit on tmp1, which makes tmp1 = [0, p1_high, 0, p2_high]
	c.assembler.CompileConstToRegister(amd64.PSRLQ, 32, tmp1)

	// Execute "pmuludq x2r,tmp1", which makes tmp1 = [p1_high*q1_lo, p2_high*q2_lo] where each lane is 64-bit.
	c.assembler.CompileRegisterToRegister(amd64.PMULUDQ, x2r, tmp1)

	// Copy x2's value into tmp2.
	c.assembler.CompileRegisterToRegister(amd64.MOVDQA, x2r, tmp2)
	// And do the logical right shift by 32-bit on tmp2, which makes tmp2 = [0, q1_high, 0, q2_high]
	c.assembler.CompileConstToRegister(amd64.PSRLQ, 32, tmp2)

	// Execute "pmuludq x1r,tmp2", which makes tmp2 = [p1_lo*q1_high, p2_lo*q2_high] where each lane is 64-bit.
	c.assembler.CompileRegisterToRegister(amd64.PMULUDQ, x1r, tmp2)

	// Adds tmp1 and tmp2 and do the logical left shift by 32-bit,
	// which makes tmp1 = [(p1_lo*q1_high+p1_high*q1_lo)<<32, (p2_lo*q2_high+p2_high*q2_lo)<<32]
	c.assembler.CompileRegisterToRegister(amd64.PADDQ, tmp2, tmp1)
	c.assembler.CompileConstToRegister(amd64.PSLLQ, 32, tmp1)

	// Execute "pmuludq x2r,x1r", which makes x1r = [p1_lo*q1_lo, p2_lo*q2_lo] where each lane is 64-bit.
	c.assembler.CompileRegisterToRegister(amd64.PMULUDQ, x2r, x1r)

	// Finally, we get the result by adding x1r and tmp1,
	// which makes x1r = [(p1_lo*q1_high+p1_high*q1_lo)<<32+p1_lo*q1_lo, (p2_lo*q2_high+p2_high*q2_lo)<<32+p2_lo*q2_lo]
	c.assembler.CompileRegisterToRegister(amd64.PADDQ, tmp1, x1r)

	c.locationStack.markRegisterUnused(x2r, tmp1)
	c.pushVectorRuntimeValueLocationOnRegister(x1r)
	return nil
}

// compileV128Div implements compiler.compileV128Div for amd64.
func (c *amd64Compiler) compileV128Div(o wazeroir.OperationV128Div) error {
	x2 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x2); err != nil {
		return err
	}

	x1 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x1); err != nil {
		return err
	}

	var inst asm.Instruction
	switch o.Shape {
	case wazeroir.ShapeF32x4:
		inst = amd64.DIVPS
	case wazeroir.ShapeF64x2:
		inst = amd64.DIVPD
	}

	c.assembler.CompileRegisterToRegister(inst, x2.register, x1.register)

	c.locationStack.markRegisterUnused(x2.register)
	c.pushVectorRuntimeValueLocationOnRegister(x1.register)
	return nil
}

// compileV128Neg implements compiler.compileV128Neg for amd64.
func (c *amd64Compiler) compileV128Neg(o wazeroir.OperationV128Neg) error {
	if o.Shape <= wazeroir.ShapeI64x2 {
		return c.compileV128NegInt(o.Shape)
	} else {
		return c.compileV128NegFloat(o.Shape)
	}
}

// compileV128NegInt implements compiler.compileV128Neg for integer lanes.
func (c *amd64Compiler) compileV128NegInt(s wazeroir.Shape) error {
	v := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(v); err != nil {
		return err
	}

	result, err := c.allocateRegister(registerTypeVector)
	if err != nil {
		return err
	}

	var subInst asm.Instruction
	switch s {
	case wazeroir.ShapeI8x16:
		subInst = amd64.PSUBB
	case wazeroir.ShapeI16x8:
		subInst = amd64.PSUBW
	case wazeroir.ShapeI32x4:
		subInst = amd64.PSUBD
	case wazeroir.ShapeI64x2:
		subInst = amd64.PSUBQ
	}

	c.assembler.CompileRegisterToRegister(amd64.PXOR, result, result)
	c.assembler.CompileRegisterToRegister(subInst, v.register, result)

	c.locationStack.markRegisterUnused(v.register)
	c.pushVectorRuntimeValueLocationOnRegister(result)
	return nil
}

// compileV128NegInt implements compiler.compileV128Neg for float lanes.
func (c *amd64Compiler) compileV128NegFloat(s wazeroir.Shape) error {
	v := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(v); err != nil {
		return err
	}

	tmp, err := c.allocateRegister(registerTypeVector)
	if err != nil {
		return err
	}

	var leftShiftInst, xorInst asm.Instruction
	var leftShiftAmount asm.ConstantValue
	if s == wazeroir.ShapeF32x4 {
		leftShiftInst, leftShiftAmount, xorInst = amd64.PSLLD, 31, amd64.XORPS
	} else {
		leftShiftInst, leftShiftAmount, xorInst = amd64.PSLLQ, 63, amd64.XORPD
	}

	// Clear all bits on tmp.
	c.assembler.CompileRegisterToRegister(amd64.XORPS, tmp, tmp)
	// Set all bits on tmp by CMPPD with arg=0 (== pseudo CMPEQPD instruction).
	// See https://www.felixcloutier.com/x86/cmpps
	//
	// Note: if we do not clear all the bits ^ with XORPS, this might end up not setting ones on some lane
	// if the lane is NaN.
	c.assembler.CompileRegisterToRegisterWithArg(amd64.CMPPD, tmp, tmp, 0x8)
	// Do the left shift on each lane to set only the most significant bit in each.
	c.assembler.CompileConstToRegister(leftShiftInst, leftShiftAmount, tmp)
	// Get the negated result by XOR on each lane with tmp.
	c.assembler.CompileRegisterToRegister(xorInst, tmp, v.register)

	c.pushVectorRuntimeValueLocationOnRegister(v.register)
	return nil
}

// compileV128Sqrt implements compiler.compileV128Sqrt for amd64.
func (c *amd64Compiler) compileV128Sqrt(o wazeroir.OperationV128Sqrt) error {
	v := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(v); err != nil {
		return err
	}

	var inst asm.Instruction
	switch o.Shape {
	case wazeroir.ShapeF64x2:
		inst = amd64.SQRTPD
	case wazeroir.ShapeF32x4:
		inst = amd64.SQRTPS
	}

	c.assembler.CompileRegisterToRegister(inst, v.register, v.register)
	c.pushVectorRuntimeValueLocationOnRegister(v.register)
	return nil
}

// compileV128Abs implements compiler.compileV128Abs for amd64.
func (c *amd64Compiler) compileV128Abs(o wazeroir.OperationV128Abs) error {
	if o.Shape == wazeroir.ShapeI64x2 {
		return c.compileV128AbsI64x2()
	}

	v := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(v); err != nil {
		return err
	}

	result := v.register
	switch o.Shape {
	case wazeroir.ShapeI8x16:
		c.assembler.CompileRegisterToRegister(amd64.PABSB, result, result)
	case wazeroir.ShapeI16x8:
		c.assembler.CompileRegisterToRegister(amd64.PABSW, result, result)
	case wazeroir.ShapeI32x4:
		c.assembler.CompileRegisterToRegister(amd64.PABSD, result, result)
	case wazeroir.ShapeF32x4:
		tmp, err := c.allocateRegister(registerTypeVector)
		if err != nil {
			return err
		}
		// Set all bits on tmp.
		c.assembler.CompileRegisterToRegister(amd64.PCMPEQD, tmp, tmp)
		// Shift right packed single floats by 1 to clear the sign bits.
		c.assembler.CompileConstToRegister(amd64.PSRLD, 1, tmp)
		// Clear the sign bit of vr.
		c.assembler.CompileRegisterToRegister(amd64.ANDPS, tmp, result)
	case wazeroir.ShapeF64x2:
		tmp, err := c.allocateRegister(registerTypeVector)
		if err != nil {
			return err
		}
		// Set all bits on tmp.
		c.assembler.CompileRegisterToRegister(amd64.PCMPEQD, tmp, tmp)
		// Shift right packed single floats by 1 to clear the sign bits.
		c.assembler.CompileConstToRegister(amd64.PSRLQ, 1, tmp)
		// Clear the sign bit of vr.
		c.assembler.CompileRegisterToRegister(amd64.ANDPD, tmp, result)
	}

	c.pushVectorRuntimeValueLocationOnRegister(result)
	return nil
}

// compileV128AbsI64x2 implements compileV128Abs for i64x2 lanes.
func (c *amd64Compiler) compileV128AbsI64x2() error {
	// See https://www.felixcloutier.com/x86/blendvpd
	const blendMaskReg = amd64.RegX0
	c.onValueReleaseRegisterToStack(blendMaskReg)
	c.locationStack.markRegisterUsed(blendMaskReg)

	v := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(v); err != nil {
		return err
	}
	vr := v.register

	if vr == blendMaskReg {
		return errors.New("BUG: X0 must not be used")
	}

	tmp, err := c.allocateRegister(registerTypeVector)
	if err != nil {
		return err
	}
	c.locationStack.markRegisterUsed(tmp)

	// Copy the value to tmp.
	c.assembler.CompileRegisterToRegister(amd64.MOVDQA, vr, tmp)

	// Clear all bits on blendMaskReg.
	c.assembler.CompileRegisterToRegister(amd64.PXOR, blendMaskReg, blendMaskReg)
	// Subtract vr from blendMaskReg.
	c.assembler.CompileRegisterToRegister(amd64.PSUBQ, vr, blendMaskReg)
	// Copy the subtracted value ^^ back into vr.
	c.assembler.CompileRegisterToRegister(amd64.MOVDQA, blendMaskReg, vr)

	c.assembler.CompileRegisterToRegister(amd64.BLENDVPD, tmp, vr)

	c.locationStack.markRegisterUnused(blendMaskReg, tmp)
	c.pushVectorRuntimeValueLocationOnRegister(vr)
	return nil
}

var (
	popcntMask = [16]byte{
		0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f,
		0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f,
	}
	// popcntTable holds each index's Popcnt, for example popcntTable[5] holds popcnt(0x05).
	popcntTable = [16]byte{
		0x00, 0x01, 0x01, 0x02, 0x01, 0x02, 0x02, 0x03,
		0x01, 0x02, 0x02, 0x03, 0x02, 0x03, 0x03, 0x04,
	}
)

// compileV128Popcnt implements compiler.compileV128Popcnt for amd64.
func (c *amd64Compiler) compileV128Popcnt(wazeroir.OperationV128Popcnt) error {
	v := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(v); err != nil {
		return err
	}
	vr := v.register

	tmp1, err := c.allocateRegister(registerTypeVector)
	if err != nil {
		return err
	}

	c.locationStack.markRegisterUsed(tmp1)

	tmp2, err := c.allocateRegister(registerTypeVector)
	if err != nil {
		return err
	}

	c.locationStack.markRegisterUsed(tmp2)

	tmp3, err := c.allocateRegister(registerTypeVector)
	if err != nil {
		return err
	}

	// Read the popcntMask into tmp1, and we have
	//  tmp1 = [0xf, ..., 0xf]
	if err := c.assembler.CompileStaticConstToRegister(amd64.MOVDQU, asm.NewStaticConst(popcntMask[:]), tmp1); err != nil {
		return err
	}

	// Copy the original value into tmp2.
	c.assembler.CompileRegisterToRegister(amd64.MOVDQA, vr, tmp2)

	// Given that we have:
	//  v = [b1, ..., b16] where bn = hn:ln and hn and ln are higher and lower 4-bits of bn.
	//
	// Take PAND on tmp1 and tmp2, and we have
	//  tmp2 = [l1, ..., l16].
	c.assembler.CompileRegisterToRegister(amd64.PAND, tmp1, tmp2)

	// Do logical (packed word) right shift by 4 on vr and PAND with vr and tmp1, meaning that we have
	//  vr = [h1, ...., h16].
	c.assembler.CompileConstToRegister(amd64.PSRLW, 4, vr)
	c.assembler.CompileRegisterToRegister(amd64.PAND, tmp1, vr)

	// Read the popcntTable into tmp1, and we have
	//  tmp1 = [0x00, 0x01, 0x01, 0x02, 0x01, 0x02, 0x02, 0x03, 0x01, 0x02, 0x02, 0x03, 0x02, 0x03, 0x03, 0x04]
	if err := c.assembler.CompileStaticConstToRegister(amd64.MOVDQU, asm.NewStaticConst(popcntTable[:]), tmp1); err != nil {
		return err
	}

	// Copy the tmp1 into tmp3, and we have
	//  tmp3 = [0x00, 0x01, 0x01, 0x02, 0x01, 0x02, 0x02, 0x03, 0x01, 0x02, 0x02, 0x03, 0x02, 0x03, 0x03, 0x04]
	c.assembler.CompileRegisterToRegister(amd64.MOVDQU, tmp1, tmp3)

	//  tmp3 = [popcnt(l1), ..., popcnt(l16)].
	c.assembler.CompileRegisterToRegister(amd64.PSHUFB, tmp2, tmp3)

	//  tmp1 = [popcnt(h1), ..., popcnt(h16)].
	c.assembler.CompileRegisterToRegister(amd64.PSHUFB, vr, tmp1)

	// vr = tmp1 = [popcnt(h1), ..., popcnt(h16)].
	c.assembler.CompileRegisterToRegister(amd64.MOVDQA, tmp1, vr)

	// vr += tmp3 = [popcnt(h1)+popcnt(l1), ..., popcnt(h16)+popcnt(l16)] = [popcnt(b1), ..., popcnt(b16)].
	c.assembler.CompileRegisterToRegister(amd64.PADDB, tmp3, vr)

	c.locationStack.markRegisterUnused(tmp1, tmp2)
	c.pushVectorRuntimeValueLocationOnRegister(vr)
	return nil
}

// compileV128Min implements compiler.compileV128Min for amd64.
func (c *amd64Compiler) compileV128Min(o wazeroir.OperationV128Min) error {
	x2 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x2); err != nil {
		return err
	}

	x1 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x1); err != nil {
		return err
	}

	if o.Shape >= wazeroir.ShapeF32x4 {
		return c.compileV128FloatMinImpl(o.Shape == wazeroir.ShapeF32x4, x1.register, x2.register)
	}

	var inst asm.Instruction
	switch o.Shape {
	case wazeroir.ShapeI8x16:
		if o.Signed {
			inst = amd64.PMINSB
		} else {
			inst = amd64.PMINUB
		}
	case wazeroir.ShapeI16x8:
		if o.Signed {
			inst = amd64.PMINSW
		} else {
			inst = amd64.PMINUW
		}
	case wazeroir.ShapeI32x4:
		if o.Signed {
			inst = amd64.PMINSD
		} else {
			inst = amd64.PMINUD
		}
	}

	c.assembler.CompileRegisterToRegister(inst, x2.register, x1.register)

	c.locationStack.markRegisterUnused(x2.register)
	c.pushVectorRuntimeValueLocationOnRegister(x1.register)
	return nil
}

// compileV128FloatMinImpl implements compiler.compileV128Min for float lanes.
func (c *amd64Compiler) compileV128FloatMinImpl(is32bit bool, x1r, x2r asm.Register) error {
	tmp, err := c.allocateRegister(registerTypeVector)
	if err != nil {
		return err
	}

	var min, cmp, andn, or, srl /* shit right logical */ asm.Instruction
	var shiftNumToInverseNaN asm.ConstantValue
	if is32bit {
		min, cmp, andn, or, srl, shiftNumToInverseNaN = amd64.MINPS, amd64.CMPPS, amd64.ANDNPS, amd64.ORPS, amd64.PSRLD, 0xa
	} else {
		min, cmp, andn, or, srl, shiftNumToInverseNaN = amd64.MINPD, amd64.CMPPD, amd64.ANDNPD, amd64.ORPD, amd64.PSRLQ, 0xd
	}

	// Let v1 and v2 be the operand values on x1r and x2r at this point.

	// Copy the value into tmp: tmp=v1
	c.assembler.CompileRegisterToRegister(amd64.MOVDQA, x1r, tmp)
	// tmp=min(v1, v2)
	c.assembler.CompileRegisterToRegister(min, x2r, tmp)
	// x2r=min(v2, v1)
	c.assembler.CompileRegisterToRegister(min, x1r, x2r)
	// x1r=min(v2, v1)
	c.assembler.CompileRegisterToRegister(amd64.MOVDQA, x2r, x1r)

	// x2r = -0          if (v1 == -0 || x2 == -0) && v1 != NaN && v2 !=NaN
	//       NaN         if v1 == NaN || v2 == NaN
	//       min(v1, v2) otherwise
	c.assembler.CompileRegisterToRegister(or, tmp, x2r)
	// x1r = 0^ (set all bits) if v1 == NaN || v2 == NaN
	//       0 otherwise
	c.assembler.CompileRegisterToRegisterWithArg(cmp, tmp, x1r, 3)
	// x2r = -0          if (v1 == -0 || x2 == -0) && v1 != NaN && v2 !=NaN
	//       ^0          if v1 == NaN || v2 == NaN
	//       min(v1, v2) otherwise
	c.assembler.CompileRegisterToRegister(or, x1r, x2r)
	// x1r = set all bits on the mantissa bits
	//       0 otherwise
	c.assembler.CompileConstToRegister(srl, shiftNumToInverseNaN, x1r)
	// x1r = x2r and !x1r
	//     = -0                                                   if (v1 == -0 || x2 == -0) && v1 != NaN && v2 !=NaN
	//       set all bits on exponential and sign bit (== NaN)    if v1 == NaN || v2 == NaN
	//       min(v1, v2)                                          otherwise
	c.assembler.CompileRegisterToRegister(andn, x2r, x1r)

	c.locationStack.markRegisterUnused(x2r)
	c.pushVectorRuntimeValueLocationOnRegister(x1r)
	return nil
}

// compileV128Max implements compiler.compileV128Max for amd64.
func (c *amd64Compiler) compileV128Max(o wazeroir.OperationV128Max) error {
	x2 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x2); err != nil {
		return err
	}

	x1 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x1); err != nil {
		return err
	}

	if o.Shape >= wazeroir.ShapeF32x4 {
		return c.compileV128FloatMaxImpl(o.Shape == wazeroir.ShapeF32x4, x1.register, x2.register)
	}

	var inst asm.Instruction
	switch o.Shape {
	case wazeroir.ShapeI8x16:
		if o.Signed {
			inst = amd64.PMAXSB
		} else {
			inst = amd64.PMAXUB
		}
	case wazeroir.ShapeI16x8:
		if o.Signed {
			inst = amd64.PMAXSW
		} else {
			inst = amd64.PMAXUW
		}
	case wazeroir.ShapeI32x4:
		if o.Signed {
			inst = amd64.PMAXSD
		} else {
			inst = amd64.PMAXUD
		}
	}

	c.assembler.CompileRegisterToRegister(inst, x2.register, x1.register)

	c.locationStack.markRegisterUnused(x2.register)
	c.pushVectorRuntimeValueLocationOnRegister(x1.register)
	return nil
}

// compileV128FloatMaxImpl implements compiler.compileV128Max for float lanes.
func (c *amd64Compiler) compileV128FloatMaxImpl(is32bit bool, x1r, x2r asm.Register) error {
	tmp, err := c.allocateRegister(registerTypeVector)
	if err != nil {
		return err
	}

	var max, cmp, andn, or, xor, sub, srl /* shit right logical */ asm.Instruction
	var shiftNumToInverseNaN asm.ConstantValue
	if is32bit {
		max, cmp, andn, or, xor, sub, srl, shiftNumToInverseNaN = amd64.MAXPS, amd64.CMPPS, amd64.ANDNPS, amd64.ORPS, amd64.XORPS, amd64.SUBPS, amd64.PSRLD, 0xa
	} else {
		max, cmp, andn, or, xor, sub, srl, shiftNumToInverseNaN = amd64.MAXPD, amd64.CMPPD, amd64.ANDNPD, amd64.ORPD, amd64.XORPD, amd64.SUBPD, amd64.PSRLQ, 0xd
	}

	// Let v1 and v2 be the operand values on x1r and x2r at this point.

	// Copy the value into tmp: tmp=v2
	c.assembler.CompileRegisterToRegister(amd64.MOVDQA, x2r, tmp)
	// tmp=max(v2, v1)
	c.assembler.CompileRegisterToRegister(max, x1r, tmp)
	// x1r=max(v1, v2)
	c.assembler.CompileRegisterToRegister(max, x2r, x1r)
	// x2r=max(v1, v2)
	c.assembler.CompileRegisterToRegister(amd64.MOVDQA, x1r, x2r)

	// x2r = -0      if (v1 == -0 && v2 == 0) || (v1 == 0 && v2 == -0)
	//       0       if (v1 == 0 && v2 ==  0)
	//       -0       if (v1 == -0 && v2 == -0)
	//       v1^v2   if v1 == NaN || v2 == NaN
	//       0       otherwise
	c.assembler.CompileRegisterToRegister(xor, tmp, x2r)
	// x1r = -0           if (v1 == -0 && v2 == 0) || (v1 == 0 && v2 == -0)
	//       0            if (v1 == 0 && v2 ==  0)
	//       -0           if (v1 == -0 && v2 == -0)
	//       NaN          if v1 == NaN || v2 == NaN
	//       max(v1, v2)  otherwise
	c.assembler.CompileRegisterToRegister(or, x2r, x1r)
	// Copy x1r into tmp.
	c.assembler.CompileRegisterToRegister(amd64.MOVDQA, x1r, tmp)
	// tmp = 0            if (v1 == -0 && v2 == 0) || (v1 == 0 && v2 == -0) || (v1 == 0 && v2 ==  0)
	//       -0           if (v1 == -0 && v2 == -0)
	//       NaN          if v1 == NaN || v2 == NaN
	//       max(v1, v2)  otherwise
	//
	// Note: -0 - (-0) = 0 (!= -0) in floating point operation.
	c.assembler.CompileRegisterToRegister(sub, x2r, tmp)
	// x1r = 0^ if v1 == NaN || v2 == NaN
	c.assembler.CompileRegisterToRegisterWithArg(cmp, x1r, x1r, 3)
	// x1r = set all bits on the mantissa bits
	//       0 otherwise
	c.assembler.CompileConstToRegister(srl, shiftNumToInverseNaN, x1r)
	c.assembler.CompileRegisterToRegister(andn, tmp, x1r)

	c.locationStack.markRegisterUnused(x2r)
	c.pushVectorRuntimeValueLocationOnRegister(x1r)
	return nil
}

// compileV128AvgrU implements compiler.compileV128AvgrU for amd64.
func (c *amd64Compiler) compileV128AvgrU(o wazeroir.OperationV128AvgrU) error {
	x2 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x2); err != nil {
		return err
	}

	x1 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x1); err != nil {
		return err
	}

	var inst asm.Instruction
	switch o.Shape {
	case wazeroir.ShapeI8x16:
		inst = amd64.PAVGB
	case wazeroir.ShapeI16x8:
		inst = amd64.PAVGW
	}

	c.assembler.CompileRegisterToRegister(inst, x2.register, x1.register)

	c.locationStack.markRegisterUnused(x2.register)
	c.pushVectorRuntimeValueLocationOnRegister(x1.register)
	return nil
}

// compileV128Pmin implements compiler.compileV128Pmin for amd64.
func (c *amd64Compiler) compileV128Pmin(o wazeroir.OperationV128Pmin) error {
	x2 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x2); err != nil {
		return err
	}

	x1 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x1); err != nil {
		return err
	}

	var min asm.Instruction
	if o.Shape == wazeroir.ShapeF32x4 {
		min = amd64.MINPS
	} else {
		min = amd64.MINPD
	}

	x1r, v2r := x1.register, x2.register

	c.assembler.CompileRegisterToRegister(min, x1r, v2r)

	c.locationStack.markRegisterUnused(x1r)
	c.pushVectorRuntimeValueLocationOnRegister(v2r)
	return nil
}

// compileV128Pmax implements compiler.compileV128Pmax for amd64.
func (c *amd64Compiler) compileV128Pmax(o wazeroir.OperationV128Pmax) error {
	x2 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x2); err != nil {
		return err
	}

	x1 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x1); err != nil {
		return err
	}

	var min asm.Instruction
	if o.Shape == wazeroir.ShapeF32x4 {
		min = amd64.MAXPS
	} else {
		min = amd64.MAXPD
	}

	x1r, v2r := x1.register, x2.register

	c.assembler.CompileRegisterToRegister(min, x1r, v2r)

	c.locationStack.markRegisterUnused(x1r)
	c.pushVectorRuntimeValueLocationOnRegister(v2r)
	return nil
}

// compileV128Ceil implements compiler.compileV128Ceil for amd64.
func (c *amd64Compiler) compileV128Ceil(o wazeroir.OperationV128Ceil) error {
	// See https://www.felixcloutier.com/x86/roundpd
	const roundModeCeil = 0x2
	return c.compileV128RoundImpl(o.Shape == wazeroir.ShapeF32x4, roundModeCeil)
}

// compileV128Floor implements compiler.compileV128Floor for amd64.
func (c *amd64Compiler) compileV128Floor(o wazeroir.OperationV128Floor) error {
	// See https://www.felixcloutier.com/x86/roundpd
	const roundModeFloor = 0x1
	return c.compileV128RoundImpl(o.Shape == wazeroir.ShapeF32x4, roundModeFloor)
}

// compileV128Trunc implements compiler.compileV128Trunc for amd64.
func (c *amd64Compiler) compileV128Trunc(o wazeroir.OperationV128Trunc) error {
	// See https://www.felixcloutier.com/x86/roundpd
	const roundModeTrunc = 0x3
	return c.compileV128RoundImpl(o.Shape == wazeroir.ShapeF32x4, roundModeTrunc)
}

// compileV128Nearest implements compiler.compileV128Nearest for amd64.
func (c *amd64Compiler) compileV128Nearest(o wazeroir.OperationV128Nearest) error {
	// See https://www.felixcloutier.com/x86/roundpd
	const roundModeNearest = 0x0
	return c.compileV128RoundImpl(o.Shape == wazeroir.ShapeF32x4, roundModeNearest)
}

// compileV128RoundImpl implements compileV128Nearest compileV128Trunc compileV128Floor and compileV128Ceil
// with ROUNDPS (32-bit lane) and ROUNDPD (64-bit lane).
func (c *amd64Compiler) compileV128RoundImpl(is32bit bool, mode byte) error {
	v := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(v); err != nil {
		return err
	}
	vr := v.register

	var round asm.Instruction
	if is32bit {
		round = amd64.ROUNDPS
	} else {
		round = amd64.ROUNDPD
	}

	c.assembler.CompileRegisterToRegisterWithArg(round, vr, vr, mode)
	c.pushVectorRuntimeValueLocationOnRegister(vr)
	return nil
}

// compileV128Extend implements compiler.compileV128Extend for amd64.
func (c *amd64Compiler) compileV128Extend(o wazeroir.OperationV128Extend) error {
	v := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(v); err != nil {
		return err
	}
	vr := v.register

	if !o.UseLow {
		// We have to shift the higher 64-bits into the lower ones before the actual extending instruction.
		// Shifting right by 0x8 * 8 = 64bits and concatenate itself.
		// See https://www.felixcloutier.com/x86/palignr
		c.assembler.CompileRegisterToRegisterWithArg(amd64.PALIGNR, v.register, v.register, 0x8)
	}

	var extend asm.Instruction
	switch o.OriginShape {
	case wazeroir.ShapeI8x16:
		if o.Signed {
			extend = amd64.PMOVSXBW
		} else {
			extend = amd64.PMOVZXBW
		}
	case wazeroir.ShapeI16x8:
		if o.Signed {
			extend = amd64.PMOVSXWD
		} else {
			extend = amd64.PMOVZXWD
		}
	case wazeroir.ShapeI32x4:
		if o.Signed {
			extend = amd64.PMOVSXDQ
		} else {
			extend = amd64.PMOVZXDQ
		}
	}

	c.assembler.CompileRegisterToRegister(extend, vr, vr)
	c.pushVectorRuntimeValueLocationOnRegister(vr)
	return nil
}

// compileV128ExtMul implements compiler.compileV128ExtMul for amd64.
func (c *amd64Compiler) compileV128ExtMul(o wazeroir.OperationV128ExtMul) error {
	x2 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x2); err != nil {
		return err
	}

	x1 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x1); err != nil {
		return err
	}

	x1r, x2r := x1.register, x2.register

	switch o.OriginShape {
	case wazeroir.ShapeI8x16:
		if !o.UseLow {
			// We have to shift the higher 64-bits into the lower ones before the actual extending instruction.
			// Shifting right by 0x8 * 8 = 64bits and concatenate itself.
			// See https://www.felixcloutier.com/x86/palignr
			c.assembler.CompileRegisterToRegisterWithArg(amd64.PALIGNR, x1r, x1r, 0x8)
			c.assembler.CompileRegisterToRegisterWithArg(amd64.PALIGNR, x2r, x2r, 0x8)
		}

		var ext asm.Instruction
		if o.Signed {
			ext = amd64.PMOVSXBW
		} else {
			ext = amd64.PMOVZXBW
		}

		// Signed or Zero extend lower half packed bytes to packed words.
		c.assembler.CompileRegisterToRegister(ext, x1r, x1r)
		c.assembler.CompileRegisterToRegister(ext, x2r, x2r)

		c.assembler.CompileRegisterToRegister(amd64.PMULLW, x2r, x1r)
	case wazeroir.ShapeI16x8:
		tmp, err := c.allocateRegister(registerTypeVector)
		if err != nil {
			return err
		}

		// Copy the value on x1r to tmp.
		c.assembler.CompileRegisterToRegister(amd64.MOVDQA, x1r, tmp)

		// Multiply the values and store the lower 16-bits into x1r.
		c.assembler.CompileRegisterToRegister(amd64.PMULLW, x2r, x1r)
		if o.Signed {
			// Signed multiply the values and store the higher 16-bits into tmp.
			c.assembler.CompileRegisterToRegister(amd64.PMULHW, x2r, tmp)
		} else {
			// Unsigned multiply the values and store the higher 16-bits into tmp.
			c.assembler.CompileRegisterToRegister(amd64.PMULHUW, x2r, tmp)
		}

		// Unpack lower or higher half of vectors (tmp and x1r) and concatenate them.
		if o.UseLow {
			c.assembler.CompileRegisterToRegister(amd64.PUNPCKLWD, tmp, x1r)
		} else {
			c.assembler.CompileRegisterToRegister(amd64.PUNPCKHWD, tmp, x1r)
		}
	case wazeroir.ShapeI32x4:
		var shuffleOrder byte
		// Given that the original state of the register is as [v1, v2, v3, v4] where vN = a word,
		if o.UseLow {
			// This makes the register as [v1, v1, v2, v2]
			shuffleOrder = 0b01010000
		} else {
			// This makes the register as [v3, v3, v4, v4]
			shuffleOrder = 0b11111010
		}
		// See https://www.felixcloutier.com/x86/pshufd
		c.assembler.CompileRegisterToRegisterWithArg(amd64.PSHUFD, x1r, x1r, shuffleOrder)
		c.assembler.CompileRegisterToRegisterWithArg(amd64.PSHUFD, x2r, x2r, shuffleOrder)

		var mul asm.Instruction
		if o.Signed {
			mul = amd64.PMULDQ
		} else {
			mul = amd64.PMULUDQ
		}
		c.assembler.CompileRegisterToRegister(mul, x2r, x1r)
	}

	c.locationStack.markRegisterUnused(x2r)
	c.pushVectorRuntimeValueLocationOnRegister(x1r)
	return nil
}

var q15mulrSatSMask = [16]byte{
	0x00, 0x80, 0x00, 0x80, 0x00, 0x80, 0x00, 0x80,
	0x00, 0x80, 0x00, 0x80, 0x00, 0x80, 0x00, 0x80,
}

// compileV128Q15mulrSatS implements compiler.compileV128Q15mulrSatS for amd64.
func (c *amd64Compiler) compileV128Q15mulrSatS(wazeroir.OperationV128Q15mulrSatS) error {
	x2 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x2); err != nil {
		return err
	}

	x1 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x1); err != nil {
		return err
	}

	tmp, err := c.allocateRegister(registerTypeVector)
	if err != nil {
		return err
	}

	x1r, x2r := x1.register, x2.register

	// See https://github.com/WebAssembly/simd/pull/365 for the following logic.
	if err := c.assembler.CompileStaticConstToRegister(amd64.MOVDQU, asm.NewStaticConst(q15mulrSatSMask[:]), tmp); err != nil {
		return err
	}

	c.assembler.CompileRegisterToRegister(amd64.PMULHRSW, x2r, x1r)
	c.assembler.CompileRegisterToRegister(amd64.PCMPEQW, x1r, tmp)
	c.assembler.CompileRegisterToRegister(amd64.PXOR, tmp, x1r)

	c.locationStack.markRegisterUnused(x2r)
	c.pushVectorRuntimeValueLocationOnRegister(x1r)
	return nil
}

var (
	allOnesI8x16 = [16]byte{0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1}
	allOnesI16x8 = [16]byte{0x1, 0x0, 0x1, 0x0, 0x1, 0x0, 0x1, 0x0, 0x1, 0x0, 0x1, 0x0, 0x1, 0x0, 0x1, 0x0}

	extAddPairwiseI16x8uMask = [16 * 2]byte{
		0x00, 0x80, 0x00, 0x80, 0x00, 0x80, 0x00, 0x80, 0x00, 0x80, 0x00, 0x80, 0x00, 0x80, 0x00, 0x80,
		0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x00,
	}
)

// compileV128ExtAddPairwise implements compiler.compileV128ExtAddPairwise for amd64.
func (c *amd64Compiler) compileV128ExtAddPairwise(o wazeroir.OperationV128ExtAddPairwise) error {
	v := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(v); err != nil {
		return err
	}
	vr := v.register

	switch o.OriginShape {
	case wazeroir.ShapeI8x16:
		allOnesReg, err := c.allocateRegister(registerTypeVector)
		if err != nil {
			return err
		}

		if err = c.assembler.CompileStaticConstToRegister(amd64.MOVDQU,
			asm.NewStaticConst(allOnesI8x16[:]), allOnesReg); err != nil {
			return err
		}

		var result asm.Register
		// See https://www.felixcloutier.com/x86/pmaddubsw for detail.
		if o.Signed {
			// Interpret vr's value as signed byte and multiply with one and add pairwise, which results in pairwise
			// signed extadd.
			c.assembler.CompileRegisterToRegister(amd64.PMADDUBSW, vr, allOnesReg)
			result = allOnesReg
		} else {
			// Interpreter tmp (all ones) as signed byte meaning that all the multiply-add is unsigned.
			c.assembler.CompileRegisterToRegister(amd64.PMADDUBSW, allOnesReg, vr)
			result = vr
		}

		if result != vr {
			c.locationStack.markRegisterUnused(vr)
		}
		c.pushVectorRuntimeValueLocationOnRegister(result)
	case wazeroir.ShapeI16x8:
		tmp, err := c.allocateRegister(registerTypeVector)
		if err != nil {
			return err
		}

		if o.Signed {
			// See https://www.felixcloutier.com/x86/pmaddwd
			if err = c.assembler.CompileStaticConstToRegister(amd64.MOVDQU,
				asm.NewStaticConst(allOnesI16x8[:]), tmp); err != nil {
				return err
			}

			c.assembler.CompileRegisterToRegister(amd64.PMADDWD, tmp, vr)
			c.pushVectorRuntimeValueLocationOnRegister(vr)
		} else {

			if err = c.assembler.CompileStaticConstToRegister(amd64.MOVDQU,
				asm.NewStaticConst(extAddPairwiseI16x8uMask[:16]), tmp); err != nil {
				return err
			}

			// Flip the sign bits on vr.
			//
			// Assuming that vr = [w1, ..., w8], now we have,
			// 	vr[i] = int8(-w1) for i = 0...8
			c.assembler.CompileRegisterToRegister(amd64.PXOR, tmp, vr)

			if err = c.assembler.CompileStaticConstToRegister(amd64.MOVDQU,
				asm.NewStaticConst(allOnesI16x8[:]), tmp); err != nil {
				return err
			}

			// For i = 0,..4 (as this results in i32x4 lanes), now we have
			// vr[i] = int32(-wn + -w(n+1)) = int32(-(wn + w(n+1)))
			c.assembler.CompileRegisterToRegister(amd64.PMADDWD, tmp, vr)

			// tmp[i] = [0, 0, 1, 0] = int32(math.MaxInt16+1)
			if err = c.assembler.CompileStaticConstToRegister(amd64.MOVDQU,
				asm.NewStaticConst(extAddPairwiseI16x8uMask[16:]), tmp); err != nil {
				return err
			}

			// vr[i] = int32(-(wn + w(n+1))) + int32(math.MaxInt16+1) = int32((wn + w(n+1))) = uint32(wn + w(n+1)).
			c.assembler.CompileRegisterToRegister(amd64.PADDD, tmp, vr)
			c.pushVectorRuntimeValueLocationOnRegister(vr)
		}
	}
	return nil
}

// compileV128FloatPromote implements compiler.compileV128FloatPromote for amd64.
func (c *amd64Compiler) compileV128FloatPromote(wazeroir.OperationV128FloatPromote) error {
	v := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(v); err != nil {
		return err
	}
	vr := v.register

	c.assembler.CompileRegisterToRegister(amd64.CVTPS2PD, vr, vr)
	c.pushVectorRuntimeValueLocationOnRegister(vr)
	return nil
}

// compileV128FloatDemote implements compiler.compileV128FloatDemote for amd64.
func (c *amd64Compiler) compileV128FloatDemote(wazeroir.OperationV128FloatDemote) error {
	v := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(v); err != nil {
		return err
	}
	vr := v.register

	c.assembler.CompileRegisterToRegister(amd64.CVTPD2PS, vr, vr)
	c.pushVectorRuntimeValueLocationOnRegister(vr)
	return nil
}

// compileV128Dot implements compiler.compileV128Dot for amd64.
func (c *amd64Compiler) compileV128Dot(wazeroir.OperationV128Dot) error {
	x2 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x2); err != nil {
		return err
	}

	x1 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x1); err != nil {
		return err
	}

	c.assembler.CompileRegisterToRegister(amd64.PMADDWD, x2.register, x1.register)

	c.locationStack.markRegisterUnused(x2.register)
	c.pushVectorRuntimeValueLocationOnRegister(x1.register)
	return nil
}

var fConvertFromIMask = [16]byte{
	0x00, 0x00, 0x30, 0x43, 0x00, 0x00, 0x30, 0x43, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
}

// compileV128FConvertFromI implements compiler.compileV128FConvertFromI for amd64.
func (c *amd64Compiler) compileV128FConvertFromI(o wazeroir.OperationV128FConvertFromI) error {
	v := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(v); err != nil {
		return err
	}
	vr := v.register

	switch o.DestinationShape {
	case wazeroir.ShapeF32x4:
		if o.Signed {
			c.assembler.CompileRegisterToRegister(amd64.CVTDQ2PS, vr, vr)
		} else {
			tmp, err := c.allocateRegister(registerTypeVector)
			if err != nil {
				return err
			}

			// Copy the value into tmp.
			c.assembler.CompileRegisterToRegister(amd64.MOVDQA, vr, tmp)

			// Clear the higher 16-bits of tmp.
			c.assembler.CompileConstToRegister(amd64.PSLLD, 0xa, tmp)
			c.assembler.CompileConstToRegister(amd64.PSRLD, 0xa, tmp)

			// Subtract the higher 16-bits from vr == clear the lower 16-bits of vr.
			c.assembler.CompileRegisterToRegister(amd64.PSUBD, tmp, vr)

			// Convert the lower 16-bits in tmp.
			c.assembler.CompileRegisterToRegister(amd64.CVTDQ2PS, tmp, tmp)

			// Left shift by one and convert vr, meaning that halved conversion result of higher 16-bits in vr.
			c.assembler.CompileConstToRegister(amd64.PSRLD, 1, vr)
			c.assembler.CompileRegisterToRegister(amd64.CVTDQ2PS, vr, vr)

			// Double the converted halved higher 16bits.
			c.assembler.CompileRegisterToRegister(amd64.ADDPS, vr, vr)

			// Get the conversion result by add tmp (holding lower 16-bit conversion) into vr.
			c.assembler.CompileRegisterToRegister(amd64.ADDPS, tmp, vr)
		}
	case wazeroir.ShapeF64x2:
		if o.Signed {
			c.assembler.CompileRegisterToRegister(amd64.CVTDQ2PD, vr, vr)
		} else {
			tmp, err := c.allocateRegister(registerTypeVector)
			if err != nil {
				return err
			}

			// tmp = [0x00, 0x00, 0x30, 0x43, 0x00, 0x00, 0x30, 0x43, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00]
			if err = c.assembler.CompileStaticConstToRegister(amd64.MOVDQU, asm.NewStaticConst(fConvertFromIMask[:16]), tmp); err != nil {
				return err
			}

			// Given that we have vr = [d1, d2, d3, d4], this results in
			//	vr = [d1, [0x00, 0x00, 0x30, 0x43], d2, [0x00, 0x00, 0x30, 0x43]]
			//     = [float64(uint32(d1)) + 0x1.0p52, float64(uint32(d2)) + 0x1.0p52]
			//     ^See https://stackoverflow.com/questions/13269523/can-all-32-bit-ints-be-exactly-represented-as-a-double
			c.assembler.CompileRegisterToRegister(amd64.UNPCKLPS, tmp, vr)

			// tmp = [float64(0x1.0p52), float64(0x1.0p52)]
			if err = c.assembler.CompileStaticConstToRegister(amd64.MOVDQU,
				asm.NewStaticConst(twop52[:]), tmp); err != nil {
				return err
			}

			// Now, we get the result as
			// 	vr = [float64(uint32(d1)), float64(uint32(d2))]
			// because the following equality always satisfies:
			//  float64(0x1.0p52 + float64(uint32(x))) - float64(0x1.0p52 + float64(uint32(y))) = float64(uint32(x)) - float64(uint32(y))
			c.assembler.CompileRegisterToRegister(amd64.SUBPD, tmp, vr)
		}
	}

	c.pushVectorRuntimeValueLocationOnRegister(vr)
	return nil
}

// compileV128Narrow implements compiler.compileV128Narrow for amd64.
func (c *amd64Compiler) compileV128Narrow(o wazeroir.OperationV128Narrow) error {
	x2 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x2); err != nil {
		return err
	}

	x1 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x1); err != nil {
		return err
	}

	var narrow asm.Instruction
	switch o.OriginShape {
	case wazeroir.ShapeI16x8:
		if o.Signed {
			narrow = amd64.PACKSSWB
		} else {
			narrow = amd64.PACKUSWB
		}
	case wazeroir.ShapeI32x4:
		if o.Signed {
			narrow = amd64.PACKSSDW
		} else {
			narrow = amd64.PACKUSDW
		}
	}
	c.assembler.CompileRegisterToRegister(narrow, x2.register, x1.register)

	c.locationStack.markRegisterUnused(x2.register)
	c.pushVectorRuntimeValueLocationOnRegister(x1.register)
	return nil
}

var (
	// i32sMaxOnF64x2 holds math.MaxInt32(=2147483647.0) on two f64 lanes.
	i32sMaxOnF64x2 = [16]byte{
		0x00, 0x00, 0xc0, 0xff, 0xff, 0xff, 0xdf, 0x41, // float64(2147483647.0)
		0x00, 0x00, 0xc0, 0xff, 0xff, 0xff, 0xdf, 0x41, // float64(2147483647.0)
	}

	// i32sMaxOnF64x2 holds math.MaxUint32(=4294967295.0) on two f64 lanes.
	i32uMaxOnF64x2 = [16]byte{
		0x00, 0x00, 0xe0, 0xff, 0xff, 0xff, 0xef, 0x41, // float64(4294967295.0)
		0x00, 0x00, 0xe0, 0xff, 0xff, 0xff, 0xef, 0x41, // float64(4294967295.0)
	}

	// twop52 holds two float64(0x1.0p52) on two f64 lanes. 0x1.0p52 is special in the sense that
	// with this exponent, the mantissa represents a corresponding uint32 number, and arithmetics,
	// like addition or subtraction, the resulted floating point holds exactly the same
	// bit representations in 32-bit integer on its mantissa.
	//
	// Note: the name twop52 is common across various compiler ecosystem.
	// 	E.g. https://github.com/llvm/llvm-project/blob/92ab024f81e5b64e258b7c3baaf213c7c26fcf40/compiler-rt/lib/builtins/floatdidf.c#L28
	// 	E.g. https://opensource.apple.com/source/clang/clang-425.0.24/src/projects/compiler-rt/lib/floatdidf.c.auto.html
	twop52 = [16]byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x30, 0x43, // float64(0x1.0p52)
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x30, 0x43, // float64(0x1.0p52)
	}
)

// compileV128ITruncSatFromF implements compiler.compileV128ITruncSatFromF for amd64.
func (c *amd64Compiler) compileV128ITruncSatFromF(o wazeroir.OperationV128ITruncSatFromF) error {
	v := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(v); err != nil {
		return err
	}
	vr := v.register

	tmp, err := c.allocateRegister(registerTypeVector)
	if err != nil {
		return err
	}

	c.locationStack.markRegisterUsed(tmp)

	switch o.OriginShape {
	case wazeroir.ShapeF32x4:
		if o.Signed {
			// Copy the value into tmp.
			c.assembler.CompileRegisterToRegister(amd64.MOVDQA, vr, tmp)

			// Assuming we have vr = [v1, v2, v3, v4].
			//
			// Set all bits if lane is not NaN on tmp.
			// tmp[i] = 0xffffffff  if vi != NaN
			//        = 0           if vi == NaN
			c.assembler.CompileRegisterToRegister(amd64.CMPEQPS, tmp, tmp)

			// Clear NaN lanes on vr, meaning that
			// 	vr[i] = vi  if vi != NaN
			//	        0   if vi == NaN
			c.assembler.CompileRegisterToRegister(amd64.ANDPS, tmp, vr)

			// tmp[i] = ^vi         if vi != NaN
			//        = 0xffffffff  if vi == NaN
			// which means that tmp[i] & 0x80000000 != 0 if and only if vi is negative.
			c.assembler.CompileRegisterToRegister(amd64.PXOR, vr, tmp)

			// vr[i] = int32(vi)   if vi != NaN and vr is not overflowing.
			//       = 0x80000000  if vi != NaN and vr is overflowing (See https://www.felixcloutier.com/x86/cvttps2dq)
			//       = 0           if vi == NaN
			c.assembler.CompileRegisterToRegister(amd64.CVTTPS2DQ, vr, vr)

			// Below, we have to convert 0x80000000 into 0x7FFFFFFF for positive overflowing lane.
			//
			// tmp[i] = 0x80000000                         if vi is positive
			//        = any satisfying any&0x80000000 = 0  if vi is negative or zero.
			c.assembler.CompileRegisterToRegister(amd64.PAND, vr, tmp)

			// Arithmetic right shifting tmp by 31, meaning that we have
			// tmp[i] = 0xffffffff if vi is positive, 0 otherwise.
			c.assembler.CompileConstToRegister(amd64.PSRAD, 0x1f, tmp)

			// Flipping 0x80000000 if vi is positive, otherwise keep intact.
			c.assembler.CompileRegisterToRegister(amd64.PXOR, tmp, vr)
		} else {
			tmp2, err := c.allocateRegister(registerTypeVector)
			if err != nil {
				return err
			}

			// See https://github.com/bytecodealliance/wasmtime/pull/2440
			// Note: even v8 doesn't seem to have support for this i32x4.tranc_sat_f32x4_u.
			c.assembler.CompileRegisterToRegister(amd64.PXOR, tmp, tmp)
			c.assembler.CompileRegisterToRegister(amd64.MAXPS, tmp, vr)
			c.assembler.CompileRegisterToRegister(amd64.PCMPEQD, tmp, tmp)
			c.assembler.CompileConstToRegister(amd64.PSRLD, 0x1, tmp)
			c.assembler.CompileRegisterToRegister(amd64.CVTDQ2PS, tmp, tmp)
			c.assembler.CompileRegisterToRegister(amd64.MOVDQA, vr, tmp2)
			c.assembler.CompileRegisterToRegister(amd64.CVTTPS2DQ, vr, vr)
			c.assembler.CompileRegisterToRegister(amd64.SUBPS, tmp, tmp2)
			c.assembler.CompileRegisterToRegisterWithArg(amd64.CMPPS, tmp2, tmp, 0x2) // == CMPLEPS
			c.assembler.CompileRegisterToRegister(amd64.CVTTPS2DQ, tmp2, tmp2)
			c.assembler.CompileRegisterToRegister(amd64.PXOR, tmp, tmp2)
			c.assembler.CompileRegisterToRegister(amd64.PXOR, tmp, tmp)
			c.assembler.CompileRegisterToRegister(amd64.PMAXSD, tmp, tmp2)
			c.assembler.CompileRegisterToRegister(amd64.PADDD, tmp2, vr)
		}
	case wazeroir.ShapeF64x2:
		tmp2, err := c.allocateRegister(registerTypeVector)
		if err != nil {
			return err
		}

		if o.Signed {
			// Copy the value into tmp.
			c.assembler.CompileRegisterToRegister(amd64.MOVDQA, vr, tmp)

			// Set all bits for non-NaN lanes, zeros otherwise.
			// I.e. tmp[i] = 0xffffffff_ffffffff if vi != NaN, 0 otherwise.
			c.assembler.CompileRegisterToRegister(amd64.CMPEQPD, tmp, tmp)

			// Load the 2147483647 into tmp2's each lane.
			if err = c.assembler.CompileStaticConstToRegister(amd64.MOVUPD, asm.NewStaticConst(i32sMaxOnF64x2[:]), tmp2); err != nil {
				return err
			}

			// tmp[i] = 2147483647 if vi != NaN, 0 otherwise.
			c.assembler.CompileRegisterToRegister(amd64.ANDPS, tmp2, tmp)

			// MINPD returns the source register's value as-is, so we have
			//  vr[i] = vi   if vi != NaN
			//        = 0    if vi == NaN
			c.assembler.CompileRegisterToRegister(amd64.MINPD, tmp, vr)

			c.assembler.CompileRegisterToRegister(amd64.CVTTPD2DQ, vr, vr)
		} else {
			// Clears all bits on tmp.
			c.assembler.CompileRegisterToRegister(amd64.PXOR, tmp, tmp)

			//  vr[i] = vi   if vi != NaN && vi > 0
			//        = 0    if vi == NaN || vi <= 0
			c.assembler.CompileRegisterToRegister(amd64.MAXPD, tmp, vr)

			// tmp2[i] = float64(math.MaxUint32) = math.MaxUint32
			if err = c.assembler.CompileStaticConstToRegister(amd64.MOVUPD, asm.NewStaticConst(i32uMaxOnF64x2[:]), tmp2); err != nil {
				return err
			}

			// vr[i] = vi   if vi != NaN && vi > 0 && vi <= math.MaxUint32
			//       = 0    otherwise
			c.assembler.CompileRegisterToRegister(amd64.MINPD, tmp2, vr)

			// Round the floating points into integer.
			c.assembler.CompileRegisterToRegisterWithArg(amd64.ROUNDPD, vr, vr, 0x3)

			// tmp2[i] = float64(0x1.0p52)
			if err = c.assembler.CompileStaticConstToRegister(amd64.MOVUPD, asm.NewStaticConst(twop52[:]), tmp2); err != nil {
				return err
			}

			// vr[i] = float64(0x1.0p52) + float64(uint32(vi)) if vi != NaN && vi > 0 && vi <= math.MaxUint32
			//       = 0                                       otherwise
			//
			// This means that vr[i] holds exactly the same bit of uint32(vi) in its lower 32-bits.
			c.assembler.CompileRegisterToRegister(amd64.ADDPD, tmp2, vr)

			// At this point, we have
			// 	vr  = [uint32(v0), float64(0x1.0p52), uint32(v1), float64(0x1.0p52)]
			//  tmp = [0, 0, 0, 0]
			// as 32x4 lanes. Therefore, SHUFPS with 0b00_00_10_00 results in
			//	vr = [vr[00], vr[10], tmp[00], tmp[00]] = [vr[00], vr[10], 0, 0]
			// meaning that for i = 0 and 1, we have
			//  vr[i] = uint32(vi) if vi != NaN && vi > 0 && vi <= math.MaxUint32
			//        = 0          otherwise.
			c.assembler.CompileRegisterToRegisterWithArg(amd64.SHUFPS, tmp, vr, 0b00_00_10_00)
		}
	}

	c.locationStack.markRegisterUnused(tmp)
	c.pushVectorRuntimeValueLocationOnRegister(vr)
	return nil
}
