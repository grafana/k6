package compiler

import (
	"github.com/tetratelabs/wazero/internal/asm"
	"github.com/tetratelabs/wazero/internal/asm/arm64"
	"github.com/tetratelabs/wazero/internal/wazeroir"
)

// compileV128Const implements compiler.compileV128Const for arm64.
func (c *arm64Compiler) compileV128Const(o wazeroir.OperationV128Const) error {
	if err := c.maybeCompileMoveTopConditionalToGeneralPurposeRegister(); err != nil {
		return err
	}

	result, err := c.allocateRegister(registerTypeVector)
	if err != nil {
		return err
	}

	// Moves the lower 64-bits as a scalar float.
	intReg := arm64ReservedRegisterForTemporary
	if o.Lo == 0 {
		intReg = arm64.RegRZR
	} else {
		c.assembler.CompileConstToRegister(arm64.MOVD, int64(o.Lo), arm64ReservedRegisterForTemporary)
	}
	c.assembler.CompileRegisterToRegister(arm64.FMOVD, intReg, result)

	// Then, insert the higher bits with INS(vector,general).
	intReg = arm64ReservedRegisterForTemporary
	if o.Hi == 0 {
		intReg = arm64.RegRZR
	} else {
		c.assembler.CompileConstToRegister(arm64.MOVD, int64(o.Hi), arm64ReservedRegisterForTemporary)
	}
	// "ins Vn.D[1], intReg"
	c.assembler.CompileRegisterToVectorRegister(arm64.INSGEN, intReg, result, arm64.VectorArrangementD, 1)

	c.pushVectorRuntimeValueLocationOnRegister(result)
	return nil
}

// compileV128Add implements compiler.compileV128Add for arm64.
func (c *arm64Compiler) compileV128Add(o wazeroir.OperationV128Add) error {
	x2 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x2); err != nil {
		return err
	}

	x1 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x1); err != nil {
		return err
	}

	x1r, x2r := x1.register, x2.register

	var arr arm64.VectorArrangement
	var inst asm.Instruction
	switch o.Shape {
	case wazeroir.ShapeI8x16:
		inst = arm64.VADD
		arr = arm64.VectorArrangement16B
	case wazeroir.ShapeI16x8:
		inst = arm64.VADD
		arr = arm64.VectorArrangement8H
	case wazeroir.ShapeI32x4:
		inst = arm64.VADD
		arr = arm64.VectorArrangement4S
	case wazeroir.ShapeI64x2:
		inst = arm64.VADD
		arr = arm64.VectorArrangement2D
	case wazeroir.ShapeF32x4:
		inst = arm64.VFADDS
		arr = arm64.VectorArrangement4S
	case wazeroir.ShapeF64x2:
		inst = arm64.VFADDD
		arr = arm64.VectorArrangement2D
	}

	c.assembler.CompileVectorRegisterToVectorRegister(inst, x1r, x2r, arr,
		arm64.VectorIndexNone, arm64.VectorIndexNone)

	c.pushVectorRuntimeValueLocationOnRegister(x2r)
	c.markRegisterUnused(x1r)
	return nil
}

// compileV128Sub implements compiler.compileV128Sub for arm64.
func (c *arm64Compiler) compileV128Sub(o wazeroir.OperationV128Sub) (err error) {
	x2 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x2); err != nil {
		return err
	}

	x1 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x1); err != nil {
		return err
	}

	x1r, x2r := x1.register, x2.register

	var arr arm64.VectorArrangement
	var inst asm.Instruction
	switch o.Shape {
	case wazeroir.ShapeI8x16:
		inst = arm64.VSUB
		arr = arm64.VectorArrangement16B
	case wazeroir.ShapeI16x8:
		inst = arm64.VSUB
		arr = arm64.VectorArrangement8H
	case wazeroir.ShapeI32x4:
		inst = arm64.VSUB
		arr = arm64.VectorArrangement4S
	case wazeroir.ShapeI64x2:
		inst = arm64.VSUB
		arr = arm64.VectorArrangement2D
	case wazeroir.ShapeF32x4:
		inst = arm64.VFSUBS
		arr = arm64.VectorArrangement4S
	case wazeroir.ShapeF64x2:
		inst = arm64.VFSUBD
		arr = arm64.VectorArrangement2D
	}

	c.assembler.CompileVectorRegisterToVectorRegister(inst, x2r, x1r, arr,
		arm64.VectorIndexNone, arm64.VectorIndexNone)

	c.pushVectorRuntimeValueLocationOnRegister(x1r)
	c.markRegisterUnused(x2r)
	return
}

// compileV128Load implements compiler.compileV128Load for arm64.
func (c *arm64Compiler) compileV128Load(o wazeroir.OperationV128Load) (err error) {
	if err := c.maybeCompileMoveTopConditionalToGeneralPurposeRegister(); err != nil {
		return err
	}
	result, err := c.allocateRegister(registerTypeVector)
	if err != nil {
		return err
	}

	switch o.Type {
	case wazeroir.V128LoadType128:
		offset, err := c.compileMemoryAccessOffsetSetup(o.Arg.Offset, 16)
		if err != nil {
			return err
		}
		c.assembler.CompileMemoryWithRegisterOffsetToVectorRegister(arm64.VMOV,
			arm64ReservedRegisterForMemory, offset, result, arm64.VectorArrangementQ,
		)
	case wazeroir.V128LoadType8x8s:
		offset, err := c.compileMemoryAccessOffsetSetup(o.Arg.Offset, 8)
		if err != nil {
			return err
		}
		c.assembler.CompileMemoryWithRegisterOffsetToVectorRegister(arm64.VMOV,
			arm64ReservedRegisterForMemory, offset, result, arm64.VectorArrangementD,
		)
		c.assembler.CompileVectorRegisterToVectorRegister(arm64.SSHLL, result, result,
			arm64.VectorArrangement8B, arm64.VectorIndexNone, arm64.VectorIndexNone)
	case wazeroir.V128LoadType8x8u:
		offset, err := c.compileMemoryAccessOffsetSetup(o.Arg.Offset, 8)
		if err != nil {
			return err
		}
		c.assembler.CompileMemoryWithRegisterOffsetToVectorRegister(arm64.VMOV,
			arm64ReservedRegisterForMemory, offset, result, arm64.VectorArrangementD,
		)
		c.assembler.CompileVectorRegisterToVectorRegister(arm64.USHLL, result, result,
			arm64.VectorArrangement8B, arm64.VectorIndexNone, arm64.VectorIndexNone)
	case wazeroir.V128LoadType16x4s:
		offset, err := c.compileMemoryAccessOffsetSetup(o.Arg.Offset, 8)
		if err != nil {
			return err
		}
		c.assembler.CompileMemoryWithRegisterOffsetToVectorRegister(arm64.VMOV,
			arm64ReservedRegisterForMemory, offset, result, arm64.VectorArrangementD,
		)
		c.assembler.CompileVectorRegisterToVectorRegister(arm64.SSHLL, result, result,
			arm64.VectorArrangement4H, arm64.VectorIndexNone, arm64.VectorIndexNone)
	case wazeroir.V128LoadType16x4u:
		offset, err := c.compileMemoryAccessOffsetSetup(o.Arg.Offset, 8)
		if err != nil {
			return err
		}
		c.assembler.CompileMemoryWithRegisterOffsetToVectorRegister(arm64.VMOV,
			arm64ReservedRegisterForMemory, offset, result, arm64.VectorArrangementD,
		)
		c.assembler.CompileVectorRegisterToVectorRegister(arm64.USHLL, result, result,
			arm64.VectorArrangement4H, arm64.VectorIndexNone, arm64.VectorIndexNone)
	case wazeroir.V128LoadType32x2s:
		offset, err := c.compileMemoryAccessOffsetSetup(o.Arg.Offset, 8)
		if err != nil {
			return err
		}
		c.assembler.CompileMemoryWithRegisterOffsetToVectorRegister(arm64.VMOV,
			arm64ReservedRegisterForMemory, offset, result, arm64.VectorArrangementD,
		)
		c.assembler.CompileVectorRegisterToVectorRegister(arm64.SSHLL, result, result,
			arm64.VectorArrangement2S, arm64.VectorIndexNone, arm64.VectorIndexNone)
	case wazeroir.V128LoadType32x2u:
		offset, err := c.compileMemoryAccessOffsetSetup(o.Arg.Offset, 8)
		if err != nil {
			return err
		}
		c.assembler.CompileMemoryWithRegisterOffsetToVectorRegister(arm64.VMOV,
			arm64ReservedRegisterForMemory, offset, result, arm64.VectorArrangementD,
		)
		c.assembler.CompileVectorRegisterToVectorRegister(arm64.USHLL, result, result,
			arm64.VectorArrangement2S, arm64.VectorIndexNone, arm64.VectorIndexNone)
	case wazeroir.V128LoadType8Splat:
		offset, err := c.compileMemoryAccessOffsetSetup(o.Arg.Offset, 1)
		if err != nil {
			return err
		}
		c.assembler.CompileRegisterToRegister(arm64.ADD, arm64ReservedRegisterForMemory, offset)
		c.assembler.CompileMemoryToVectorRegister(arm64.LD1R, offset, 0, result, arm64.VectorArrangement16B)
	case wazeroir.V128LoadType16Splat:
		offset, err := c.compileMemoryAccessOffsetSetup(o.Arg.Offset, 2)
		if err != nil {
			return err
		}
		c.assembler.CompileRegisterToRegister(arm64.ADD, arm64ReservedRegisterForMemory, offset)
		c.assembler.CompileMemoryToVectorRegister(arm64.LD1R, offset, 0, result, arm64.VectorArrangement8H)
	case wazeroir.V128LoadType32Splat:
		offset, err := c.compileMemoryAccessOffsetSetup(o.Arg.Offset, 4)
		if err != nil {
			return err
		}
		c.assembler.CompileRegisterToRegister(arm64.ADD, arm64ReservedRegisterForMemory, offset)
		c.assembler.CompileMemoryToVectorRegister(arm64.LD1R, offset, 0, result, arm64.VectorArrangement4S)
	case wazeroir.V128LoadType64Splat:
		offset, err := c.compileMemoryAccessOffsetSetup(o.Arg.Offset, 8)
		if err != nil {
			return err
		}
		c.assembler.CompileRegisterToRegister(arm64.ADD, arm64ReservedRegisterForMemory, offset)
		c.assembler.CompileMemoryToVectorRegister(arm64.LD1R, offset, 0, result, arm64.VectorArrangement2D)
	case wazeroir.V128LoadType32zero:
		offset, err := c.compileMemoryAccessOffsetSetup(o.Arg.Offset, 4)
		if err != nil {
			return err
		}
		c.assembler.CompileMemoryWithRegisterOffsetToVectorRegister(arm64.VMOV,
			arm64ReservedRegisterForMemory, offset, result, arm64.VectorArrangementS,
		)
	case wazeroir.V128LoadType64zero:
		offset, err := c.compileMemoryAccessOffsetSetup(o.Arg.Offset, 8)
		if err != nil {
			return err
		}
		c.assembler.CompileMemoryWithRegisterOffsetToVectorRegister(arm64.VMOV,
			arm64ReservedRegisterForMemory, offset, result, arm64.VectorArrangementD,
		)
	}

	c.pushVectorRuntimeValueLocationOnRegister(result)
	return
}

// compileV128LoadLane implements compiler.compileV128LoadLane for arm64.
func (c *arm64Compiler) compileV128LoadLane(o wazeroir.OperationV128LoadLane) (err error) {
	targetVector := c.locationStack.popV128()
	if err = c.compileEnsureOnRegister(targetVector); err != nil {
		return
	}

	targetSizeInBytes := int64(o.LaneSize / 8)
	source, err := c.compileMemoryAccessOffsetSetup(o.Arg.Offset, targetSizeInBytes)
	if err != nil {
		return err
	}

	var loadInst asm.Instruction
	var arr arm64.VectorArrangement
	switch o.LaneSize {
	case 8:
		arr = arm64.VectorArrangementB
		loadInst = arm64.LDRB
	case 16:
		arr = arm64.VectorArrangementH
		loadInst = arm64.LDRH
	case 32:
		loadInst = arm64.LDRW
		arr = arm64.VectorArrangementS
	case 64:
		loadInst = arm64.LDRD
		arr = arm64.VectorArrangementD
	}

	c.assembler.CompileMemoryWithRegisterOffsetToRegister(loadInst, arm64ReservedRegisterForMemory, source, source)
	c.assembler.CompileRegisterToVectorRegister(arm64.INSGEN, source, targetVector.register, arr, arm64.VectorIndex(o.LaneIndex))

	c.pushVectorRuntimeValueLocationOnRegister(targetVector.register)
	c.locationStack.markRegisterUnused(source)
	return
}

// compileV128Store implements compiler.compileV128Store for arm64.
func (c *arm64Compiler) compileV128Store(o wazeroir.OperationV128Store) (err error) {
	v := c.locationStack.popV128()
	if err = c.compileEnsureOnRegister(v); err != nil {
		return
	}

	const targetSizeInBytes = 16
	offset, err := c.compileMemoryAccessOffsetSetup(o.Arg.Offset, targetSizeInBytes)
	if err != nil {
		return err
	}

	c.assembler.CompileVectorRegisterToMemoryWithRegisterOffset(arm64.VMOV,
		v.register, arm64ReservedRegisterForMemory, offset, arm64.VectorArrangementQ)

	c.markRegisterUnused(v.register)
	return
}

// compileV128StoreLane implements compiler.compileV128StoreLane for arm64.
func (c *arm64Compiler) compileV128StoreLane(o wazeroir.OperationV128StoreLane) (err error) {
	var arr arm64.VectorArrangement
	var storeInst asm.Instruction
	switch o.LaneSize {
	case 8:
		storeInst = arm64.STRB
		arr = arm64.VectorArrangementB
	case 16:
		storeInst = arm64.STRH
		arr = arm64.VectorArrangementH
	case 32:
		storeInst = arm64.STRW
		arr = arm64.VectorArrangementS
	case 64:
		storeInst = arm64.STRD
		arr = arm64.VectorArrangementD
	}

	v := c.locationStack.popV128()
	if err = c.compileEnsureOnRegister(v); err != nil {
		return
	}

	targetSizeInBytes := int64(o.LaneSize / 8)
	offset, err := c.compileMemoryAccessOffsetSetup(o.Arg.Offset, targetSizeInBytes)
	if err != nil {
		return err
	}

	c.assembler.CompileVectorRegisterToRegister(arm64.UMOV, v.register, arm64ReservedRegisterForTemporary, arr,
		arm64.VectorIndex(o.LaneIndex))

	c.assembler.CompileRegisterToMemoryWithRegisterOffset(storeInst,
		arm64ReservedRegisterForTemporary, arm64ReservedRegisterForMemory, offset)

	c.locationStack.markRegisterUnused(v.register)
	return
}

// compileV128ExtractLane implements compiler.compileV128ExtractLane for arm64.
func (c *arm64Compiler) compileV128ExtractLane(o wazeroir.OperationV128ExtractLane) (err error) {
	v := c.locationStack.popV128()
	if err = c.compileEnsureOnRegister(v); err != nil {
		return
	}

	switch o.Shape {
	case wazeroir.ShapeI8x16:
		result, err := c.allocateRegister(registerTypeGeneralPurpose)
		if err != nil {
			return err
		}
		var inst asm.Instruction
		if o.Signed {
			inst = arm64.SMOV32
		} else {
			inst = arm64.UMOV
		}
		c.assembler.CompileVectorRegisterToRegister(inst, v.register, result,
			arm64.VectorArrangementB, arm64.VectorIndex(o.LaneIndex))

		c.locationStack.markRegisterUnused(v.register)
		c.pushRuntimeValueLocationOnRegister(result, runtimeValueTypeI32)
	case wazeroir.ShapeI16x8:
		result, err := c.allocateRegister(registerTypeGeneralPurpose)
		if err != nil {
			return err
		}
		var inst asm.Instruction
		if o.Signed {
			inst = arm64.SMOV32
		} else {
			inst = arm64.UMOV
		}
		c.assembler.CompileVectorRegisterToRegister(inst, v.register, result,
			arm64.VectorArrangementH, arm64.VectorIndex(o.LaneIndex))

		c.locationStack.markRegisterUnused(v.register)
		c.pushRuntimeValueLocationOnRegister(result, runtimeValueTypeI32)
	case wazeroir.ShapeI32x4:
		result, err := c.allocateRegister(registerTypeGeneralPurpose)
		if err != nil {
			return err
		}
		c.assembler.CompileVectorRegisterToRegister(arm64.UMOV, v.register, result,
			arm64.VectorArrangementS, arm64.VectorIndex(o.LaneIndex))

		c.locationStack.markRegisterUnused(v.register)
		c.pushRuntimeValueLocationOnRegister(result, runtimeValueTypeI32)
	case wazeroir.ShapeI64x2:
		result, err := c.allocateRegister(registerTypeGeneralPurpose)
		if err != nil {
			return err
		}
		c.assembler.CompileVectorRegisterToRegister(arm64.UMOV, v.register, result,
			arm64.VectorArrangementD, arm64.VectorIndex(o.LaneIndex))

		c.locationStack.markRegisterUnused(v.register)
		c.pushRuntimeValueLocationOnRegister(result, runtimeValueTypeI64)
	case wazeroir.ShapeF32x4:
		c.assembler.CompileVectorRegisterToVectorRegister(arm64.INSELEM, v.register, v.register,
			arm64.VectorArrangementS, arm64.VectorIndex(o.LaneIndex), 0)
		c.pushRuntimeValueLocationOnRegister(v.register, runtimeValueTypeF32)
	case wazeroir.ShapeF64x2:
		c.assembler.CompileVectorRegisterToVectorRegister(arm64.INSELEM, v.register, v.register,
			arm64.VectorArrangementD, arm64.VectorIndex(o.LaneIndex), 0)
		c.pushRuntimeValueLocationOnRegister(v.register, runtimeValueTypeF64)
	}
	return
}

// compileV128ReplaceLane implements compiler.compileV128ReplaceLane for arm64.
func (c *arm64Compiler) compileV128ReplaceLane(o wazeroir.OperationV128ReplaceLane) (err error) {
	origin := c.locationStack.pop()
	if err = c.compileEnsureOnRegister(origin); err != nil {
		return
	}

	vector := c.locationStack.popV128()
	if err = c.compileEnsureOnRegister(vector); err != nil {
		return
	}

	switch o.Shape {
	case wazeroir.ShapeI8x16:
		c.assembler.CompileRegisterToVectorRegister(arm64.INSGEN, origin.register, vector.register,
			arm64.VectorArrangementB, arm64.VectorIndex(o.LaneIndex))
	case wazeroir.ShapeI16x8:
		c.assembler.CompileRegisterToVectorRegister(arm64.INSGEN, origin.register, vector.register,
			arm64.VectorArrangementH, arm64.VectorIndex(o.LaneIndex))
	case wazeroir.ShapeI32x4:
		c.assembler.CompileRegisterToVectorRegister(arm64.INSGEN, origin.register, vector.register,
			arm64.VectorArrangementS, arm64.VectorIndex(o.LaneIndex))
	case wazeroir.ShapeI64x2:
		c.assembler.CompileRegisterToVectorRegister(arm64.INSGEN, origin.register, vector.register,
			arm64.VectorArrangementD, arm64.VectorIndex(o.LaneIndex))
	case wazeroir.ShapeF32x4:
		c.assembler.CompileVectorRegisterToVectorRegister(arm64.INSELEM, origin.register, vector.register,
			arm64.VectorArrangementS, 0, arm64.VectorIndex(o.LaneIndex))
	case wazeroir.ShapeF64x2:
		c.assembler.CompileVectorRegisterToVectorRegister(arm64.INSELEM, origin.register, vector.register,
			arm64.VectorArrangementD, 0, arm64.VectorIndex(o.LaneIndex))
	}

	c.locationStack.markRegisterUnused(origin.register)
	c.pushVectorRuntimeValueLocationOnRegister(vector.register)
	return
}

// compileV128Splat implements compiler.compileV128Splat for arm64.
func (c *arm64Compiler) compileV128Splat(o wazeroir.OperationV128Splat) (err error) {
	origin := c.locationStack.pop()
	if err = c.compileEnsureOnRegister(origin); err != nil {
		return
	}

	var result asm.Register
	switch o.Shape {
	case wazeroir.ShapeI8x16:
		result, err = c.allocateRegister(registerTypeVector)
		if err != nil {
			return
		}
		c.assembler.CompileRegisterToVectorRegister(arm64.DUPGEN, origin.register, result,
			arm64.VectorArrangement16B, arm64.VectorIndexNone)
	case wazeroir.ShapeI16x8:
		result, err = c.allocateRegister(registerTypeVector)
		if err != nil {
			return
		}
		c.assembler.CompileRegisterToVectorRegister(arm64.DUPGEN, origin.register, result,
			arm64.VectorArrangement8H, arm64.VectorIndexNone)
	case wazeroir.ShapeI32x4:
		result, err = c.allocateRegister(registerTypeVector)
		if err != nil {
			return
		}
		c.assembler.CompileRegisterToVectorRegister(arm64.DUPGEN, origin.register, result,
			arm64.VectorArrangement4S, arm64.VectorIndexNone)
	case wazeroir.ShapeI64x2:
		result, err = c.allocateRegister(registerTypeVector)
		if err != nil {
			return
		}
		c.assembler.CompileRegisterToVectorRegister(arm64.DUPGEN, origin.register, result,
			arm64.VectorArrangement2D, arm64.VectorIndexNone)
	case wazeroir.ShapeF32x4:
		result = origin.register
		c.assembler.CompileVectorRegisterToVectorRegister(arm64.DUPELEM, origin.register, result,
			arm64.VectorArrangementS, 0, arm64.VectorIndexNone)
	case wazeroir.ShapeF64x2:
		result = origin.register
		c.assembler.CompileVectorRegisterToVectorRegister(arm64.DUPELEM, origin.register, result,
			arm64.VectorArrangementD, 0, arm64.VectorIndexNone)
	}

	c.locationStack.markRegisterUnused(origin.register)
	c.pushVectorRuntimeValueLocationOnRegister(result)
	return
}

func (c *arm64Compiler) onValueReleaseRegisterToStack(reg asm.Register) {
	for i := uint64(0); i < c.locationStack.sp; i++ {
		prevValue := &c.locationStack.stack[i]
		if prevValue.register == reg {
			c.compileReleaseRegisterToStack(prevValue)
			break
		}
	}
}

// compileV128Shuffle implements compiler.compileV128Shuffle for arm64.
func (c *arm64Compiler) compileV128Shuffle(o wazeroir.OperationV128Shuffle) (err error) {
	// Shuffle needs two operands (v, w) must be next to each other.
	// For simplicity, we use V29 for v and V30 for w values respectively.
	const vReg, wReg = arm64.RegV29, arm64.RegV30

	// Ensures that w value is placed on wReg.
	w := c.locationStack.popV128()
	if w.register != wReg {
		// If wReg is already in use, save the value onto the stack.
		c.onValueReleaseRegisterToStack(wReg)

		if w.onRegister() {
			c.assembler.CompileTwoVectorRegistersToVectorRegister(arm64.VORR,
				w.register, w.register, wReg, arm64.VectorArrangement16B)
			// We no longer use the old register.
			c.markRegisterUnused(w.register)
		} else { // on stack
			w.setRegister(wReg)
			c.compileLoadValueOnStackToRegister(w)
		}
	}

	// Ensures that v value is placed on wReg.
	v := c.locationStack.popV128()
	if v.register != vReg {
		// If vReg is already in use, save the value onto the stack.
		c.onValueReleaseRegisterToStack(vReg)

		if v.onRegister() {
			c.assembler.CompileTwoVectorRegistersToVectorRegister(arm64.VORR,
				v.register, v.register, vReg, arm64.VectorArrangement16B)
			// We no longer use the old register.
			c.markRegisterUnused(v.register)
		} else { // on stack
			v.setRegister(vReg)
			c.compileLoadValueOnStackToRegister(v)
		}
	}

	c.locationStack.markRegisterUsed(vReg, wReg)
	result, err := c.allocateRegister(registerTypeVector)
	if err != nil {
		return err
	}

	c.assembler.CompileStaticConstToVectorRegister(arm64.VMOV, asm.NewStaticConst(o.Lanes[:]), result, arm64.VectorArrangementQ)
	c.assembler.CompileVectorRegisterToVectorRegister(arm64.TBL2, vReg, result, arm64.VectorArrangement16B,
		arm64.VectorIndexNone, arm64.VectorIndexNone)

	c.locationStack.markRegisterUnused(vReg, wReg)
	c.pushVectorRuntimeValueLocationOnRegister(result)
	return
}

// compileV128Swizzle implements compiler.compileV128Swizzle for arm64.
func (c *arm64Compiler) compileV128Swizzle(wazeroir.OperationV128Swizzle) (err error) {
	indexVec := c.locationStack.popV128()
	if err = c.compileEnsureOnRegister(indexVec); err != nil {
		return
	}
	baseVec := c.locationStack.popV128()
	if err = c.compileEnsureOnRegister(baseVec); err != nil {
		return
	}

	c.assembler.CompileVectorRegisterToVectorRegister(arm64.TBL1, baseVec.register, indexVec.register,
		arm64.VectorArrangement16B, arm64.VectorIndexNone, arm64.VectorIndexNone)

	c.markRegisterUnused(baseVec.register)
	c.pushVectorRuntimeValueLocationOnRegister(indexVec.register)
	return
}

// compileV128AnyTrue implements compiler.compileV128AnyTrue for arm64.
func (c *arm64Compiler) compileV128AnyTrue(wazeroir.OperationV128AnyTrue) (err error) {
	vector := c.locationStack.popV128()
	if err = c.compileEnsureOnRegister(vector); err != nil {
		return
	}

	v := vector.register
	c.assembler.CompileVectorRegisterToVectorRegister(arm64.UMAXP, v, v,
		arm64.VectorArrangement16B, arm64.VectorIndexNone, arm64.VectorIndexNone)
	c.assembler.CompileVectorRegisterToRegister(arm64.UMOV, v, arm64ReservedRegisterForTemporary,
		arm64.VectorArrangementD, 0)
	c.assembler.CompileTwoRegistersToNone(arm64.CMP, arm64.RegRZR, arm64ReservedRegisterForTemporary)
	c.locationStack.pushRuntimeValueLocationOnConditionalRegister(arm64.CondNE)

	c.locationStack.markRegisterUnused(v)
	return
}

// compileV128AllTrue implements compiler.compileV128AllTrue for arm64.
func (c *arm64Compiler) compileV128AllTrue(o wazeroir.OperationV128AllTrue) (err error) {
	vector := c.locationStack.popV128()
	if err = c.compileEnsureOnRegister(vector); err != nil {
		return
	}

	v := vector.register
	if o.Shape == wazeroir.ShapeI64x2 {
		c.assembler.CompileVectorRegisterToVectorRegister(arm64.CMEQZERO, arm64.RegRZR, v,
			arm64.VectorArrangement2D, arm64.VectorIndexNone, arm64.VectorIndexNone)
		c.assembler.CompileVectorRegisterToVectorRegister(arm64.ADDP, v, v,
			arm64.VectorArrangement2D, arm64.VectorIndexNone, arm64.VectorIndexNone)
		c.assembler.CompileTwoRegistersToNone(arm64.FCMPD, v, v)
		c.locationStack.pushRuntimeValueLocationOnConditionalRegister(arm64.CondEQ)
	} else {
		var arr arm64.VectorArrangement
		switch o.Shape {
		case wazeroir.ShapeI8x16:
			arr = arm64.VectorArrangement16B
		case wazeroir.ShapeI16x8:
			arr = arm64.VectorArrangement8H
		case wazeroir.ShapeI32x4:
			arr = arm64.VectorArrangement4S
		}

		c.assembler.CompileVectorRegisterToVectorRegister(arm64.UMINV, v, v,
			arr, arm64.VectorIndexNone, arm64.VectorIndexNone)
		c.assembler.CompileVectorRegisterToRegister(arm64.UMOV, v, arm64ReservedRegisterForTemporary,
			arm64.VectorArrangementD, 0)
		c.assembler.CompileTwoRegistersToNone(arm64.CMP, arm64.RegRZR, arm64ReservedRegisterForTemporary)
		c.locationStack.pushRuntimeValueLocationOnConditionalRegister(arm64.CondNE)
	}
	c.markRegisterUnused(v)
	return
}

var (
	i8x16BitmaskConst = [16]byte{
		0x01, 0x02, 0x04, 0x08, 0x10, 0x20, 0x40, 0x80,
		0x01, 0x02, 0x04, 0x08, 0x10, 0x20, 0x40, 0x80,
	}
	i16x8BitmaskConst = [16]byte{
		0x01, 0x00, 0x02, 0x00, 0x04, 0x00, 0x08, 0x00,
		0x10, 0x00, 0x20, 0x00, 0x40, 0x00, 0x80, 0x00,
	}
	i32x4BitmaskConst = [16]byte{
		0x01, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00,
		0x04, 0x00, 0x00, 0x00, 0x08, 0x00, 0x00, 0x00,
	}
)

// compileV128BitMask implements compiler.compileV128BitMask for arm64.
func (c *arm64Compiler) compileV128BitMask(o wazeroir.OperationV128BitMask) (err error) {
	vector := c.locationStack.popV128()
	if err = c.compileEnsureOnRegister(vector); err != nil {
		return
	}

	v := vector.register

	result, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}

	switch o.Shape {
	case wazeroir.ShapeI8x16:
		vecTmp, err := c.allocateRegister(registerTypeVector)
		if err != nil {
			return err
		}
		// Right arithmetic shift on the original vector and store the result into vecTmp. So we have:
		// v[i] = 0xff if vi<0, 0 otherwise.
		c.assembler.CompileVectorRegisterToVectorRegisterWithConst(arm64.SSHR, v, v, arm64.VectorArrangement16B, 7)

		// Load the bit mask into vecTmp.
		c.assembler.CompileStaticConstToVectorRegister(arm64.VMOV, asm.NewStaticConst(i8x16BitmaskConst[:]), vecTmp, arm64.VectorArrangementQ)

		// Lane-wise logical AND with i8x16BitmaskConst, meaning that we have
		// v[i] = (1 << i) if vi<0, 0 otherwise.
		//
		// Below, we use the following notation:
		// wi := (1 << i) if vi<0, 0 otherwise.
		c.assembler.CompileVectorRegisterToVectorRegister(arm64.VAND, vecTmp, v, arm64.VectorArrangement16B,
			arm64.VectorIndexNone, arm64.VectorIndexNone)

		// Swap the lower and higher 8 byte elements, and write it into vecTmp, meaning that we have
		// vecTmp[i] = w(i+8) if i < 8, w(i-8) otherwise.
		//
		c.assembler.CompileTwoVectorRegistersToVectorRegisterWithConst(arm64.EXT, v, v, vecTmp, arm64.VectorArrangement16B, 0x8)

		// v = [w0, w8, ..., w7, w15]
		c.assembler.CompileTwoVectorRegistersToVectorRegister(arm64.ZIP1, vecTmp, v, v, arm64.VectorArrangement16B)

		// v.h[0] = w0 + ... + w15
		c.assembler.CompileVectorRegisterToVectorRegister(arm64.ADDV, v, v,
			arm64.VectorArrangement8H, arm64.VectorIndexNone, arm64.VectorIndexNone)

		// Extract the v.h[0] as the result.
		c.assembler.CompileVectorRegisterToRegister(arm64.UMOV, v, result, arm64.VectorArrangementH, 0)
	case wazeroir.ShapeI16x8:
		vecTmp, err := c.allocateRegister(registerTypeVector)
		if err != nil {
			return err
		}
		// Right arithmetic shift on the original vector and store the result into vecTmp. So we have:
		// v[i] = 0xffff if vi<0, 0 otherwise.
		c.assembler.CompileVectorRegisterToVectorRegisterWithConst(arm64.SSHR, v, v, arm64.VectorArrangement8H, 15)

		// Load the bit mask into vecTmp.
		c.assembler.CompileStaticConstToVectorRegister(arm64.VMOV, asm.NewStaticConst(i16x8BitmaskConst[:]), vecTmp, arm64.VectorArrangementQ)

		// Lane-wise logical AND with i16x8BitmaskConst, meaning that we have
		// v[i] = (1 << i)     if vi<0, 0 otherwise for i=0..3
		//      = (1 << (i+4)) if vi<0, 0 otherwise for i=3..7
		c.assembler.CompileVectorRegisterToVectorRegister(arm64.VAND, vecTmp, v, arm64.VectorArrangement16B,
			arm64.VectorIndexNone, arm64.VectorIndexNone)

		c.assembler.CompileVectorRegisterToVectorRegister(arm64.ADDV, v, v,
			arm64.VectorArrangement8H, arm64.VectorIndexNone, arm64.VectorIndexNone)

		c.assembler.CompileVectorRegisterToRegister(arm64.UMOV, v, result, arm64.VectorArrangementH, 0)
	case wazeroir.ShapeI32x4:
		vecTmp, err := c.allocateRegister(registerTypeVector)
		if err != nil {
			return err
		}

		// Right arithmetic shift on the original vector and store the result into vecTmp. So we have:
		// v[i] = 0xffffffff if vi<0, 0 otherwise.
		c.assembler.CompileVectorRegisterToVectorRegisterWithConst(arm64.SSHR, v, v, arm64.VectorArrangement4S, 32)

		// Load the bit mask into vecTmp.
		c.assembler.CompileStaticConstToVectorRegister(arm64.VMOV,
			asm.NewStaticConst(i32x4BitmaskConst[:]), vecTmp, arm64.VectorArrangementQ)

		// Lane-wise logical AND with i16x8BitmaskConst, meaning that we have
		// v[i] = (1 << i)     if vi<0, 0 otherwise for i in [0, 1]
		//      = (1 << (i+4)) if vi<0, 0 otherwise for i in [2, 3]
		c.assembler.CompileVectorRegisterToVectorRegister(arm64.VAND, vecTmp, v, arm64.VectorArrangement16B,
			arm64.VectorIndexNone, arm64.VectorIndexNone)

		c.assembler.CompileVectorRegisterToVectorRegister(arm64.ADDV, v, v,
			arm64.VectorArrangement4S, arm64.VectorIndexNone, arm64.VectorIndexNone)

		c.assembler.CompileVectorRegisterToRegister(arm64.UMOV, v, result, arm64.VectorArrangementS, 0)
	case wazeroir.ShapeI64x2:
		// Move the lower 64-bit int into result,
		c.assembler.CompileVectorRegisterToRegister(arm64.UMOV, v, result,
			arm64.VectorArrangementD, 0)
		// Move the higher 64-bit int into arm64ReservedRegisterForTemporary.
		c.assembler.CompileVectorRegisterToRegister(arm64.UMOV, v, arm64ReservedRegisterForTemporary,
			arm64.VectorArrangementD, 1)

		// Move the sign bit into the least significant bit.
		c.assembler.CompileConstToRegister(arm64.LSR, 63, result)
		c.assembler.CompileConstToRegister(arm64.LSR, 63, arm64ReservedRegisterForTemporary)

		// result = (arm64ReservedRegisterForTemporary<<1) | result
		c.assembler.CompileLeftShiftedRegisterToRegister(arm64.ADD,
			arm64ReservedRegisterForTemporary, 1, result, result)
	}

	c.markRegisterUnused(v)
	c.pushRuntimeValueLocationOnRegister(result, runtimeValueTypeI32)
	return
}

// compileV128And implements compiler.compileV128And for arm64.
func (c *arm64Compiler) compileV128And(wazeroir.OperationV128And) error {
	return c.compileV128x2BinOp(arm64.VAND, arm64.VectorArrangement16B)
}

// compileV128Not implements compiler.compileV128Not for arm64.
func (c *arm64Compiler) compileV128Not(wazeroir.OperationV128Not) error {
	return c.compileV128UniOp(arm64.NOT, arm64.VectorArrangement16B)
}

// compileV128Or implements compiler.compileV128Or for arm64.
func (c *arm64Compiler) compileV128Or(wazeroir.OperationV128Or) error {
	return c.compileV128x2BinOp(arm64.VORR, arm64.VectorArrangement16B)
}

// compileV128Xor implements compiler.compileV128Xor for arm64.
func (c *arm64Compiler) compileV128Xor(wazeroir.OperationV128Xor) error {
	return c.compileV128x2BinOp(arm64.EOR, arm64.VectorArrangement16B)
}

// compileV128Bitselect implements compiler.compileV128Bitselect for arm64.
func (c *arm64Compiler) compileV128Bitselect(wazeroir.OperationV128Bitselect) error {
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

	c.assembler.CompileTwoVectorRegistersToVectorRegister(arm64.BSL,
		x2.register, x1.register, selector.register, arm64.VectorArrangement16B)

	c.markRegisterUnused(x1.register, x2.register)
	c.pushVectorRuntimeValueLocationOnRegister(selector.register)
	return nil
}

// compileV128AndNot implements compiler.compileV128AndNot for arm64.
func (c *arm64Compiler) compileV128AndNot(wazeroir.OperationV128AndNot) error {
	return c.compileV128x2BinOp(arm64.BIC, arm64.VectorArrangement16B)
}

func (c *arm64Compiler) compileV128UniOp(inst asm.Instruction, arr arm64.VectorArrangement) error {
	v := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(v); err != nil {
		return err
	}

	c.assembler.CompileVectorRegisterToVectorRegister(inst, v.register, v.register, arr, arm64.VectorIndexNone, arm64.VectorIndexNone)

	c.pushVectorRuntimeValueLocationOnRegister(v.register)
	return nil
}

func (c *arm64Compiler) compileV128x2BinOp(inst asm.Instruction, arr arm64.VectorArrangement) error {
	x2 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x2); err != nil {
		return err
	}

	x1 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x1); err != nil {
		return err
	}

	c.assembler.CompileVectorRegisterToVectorRegister(inst, x2.register, x1.register, arr, arm64.VectorIndexNone, arm64.VectorIndexNone)

	c.markRegisterUnused(x2.register)
	c.pushVectorRuntimeValueLocationOnRegister(x1.register)
	return nil
}

// compileV128Shr implements compiler.compileV128Shr for arm64.
func (c *arm64Compiler) compileV128Shr(o wazeroir.OperationV128Shr) error {
	var inst asm.Instruction
	if o.Signed {
		inst = arm64.SSHL
	} else {
		inst = arm64.USHL
	}
	return c.compileV128ShiftImpl(o.Shape, inst, true)
}

// compileV128Shl implements compiler.compileV128Shl for arm64.
func (c *arm64Compiler) compileV128Shl(o wazeroir.OperationV128Shl) error {
	return c.compileV128ShiftImpl(o.Shape, arm64.SSHL, false)
}

func (c *arm64Compiler) compileV128ShiftImpl(shape wazeroir.Shape, ins asm.Instruction, rightShift bool) error {
	s := c.locationStack.pop()
	if s.register == arm64.RegRZR {
		// If the shift amount is zero register, nothing to do here.
		return nil
	}

	var modulo asm.ConstantValue
	var arr arm64.VectorArrangement
	switch shape {
	case wazeroir.ShapeI8x16:
		modulo = 0x7 // modulo 8.
		arr = arm64.VectorArrangement16B
	case wazeroir.ShapeI16x8:
		modulo = 0xf // modulo 16.
		arr = arm64.VectorArrangement8H
	case wazeroir.ShapeI32x4:
		modulo = 0x1f // modulo 32.
		arr = arm64.VectorArrangement4S
	case wazeroir.ShapeI64x2:
		modulo = 0x3f // modulo 64.
		arr = arm64.VectorArrangement2D
	}

	if err := c.compileEnsureOnRegister(s); err != nil {
		return err
	}

	v := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(v); err != nil {
		return err
	}

	tmp, err := c.allocateRegister(registerTypeVector)
	if err != nil {
		return err
	}

	c.assembler.CompileConstToRegister(arm64.ANDIMM32, modulo, s.register)

	if rightShift {
		// Negate the amount to make this as right shift.
		c.assembler.CompileRegisterToRegister(arm64.NEG, s.register, s.register)
	}

	// Copy the shift amount into a vector register as SSHL requires it to be there.
	c.assembler.CompileRegisterToVectorRegister(arm64.DUPGEN, s.register, tmp,
		arr, arm64.VectorIndexNone)

	c.assembler.CompileVectorRegisterToVectorRegister(ins, tmp, v.register, arr,
		arm64.VectorIndexNone, arm64.VectorIndexNone)

	c.markRegisterUnused(s.register)
	c.pushVectorRuntimeValueLocationOnRegister(v.register)
	return nil
}

// compileV128Cmp implements compiler.compileV128Cmp for arm64.
func (c *arm64Compiler) compileV128Cmp(o wazeroir.OperationV128Cmp) error {
	x2 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x2); err != nil {
		return err
	}

	x1 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x1); err != nil {
		return err
	}

	var arr arm64.VectorArrangement
	if o.Type <= wazeroir.V128CmpTypeI8x16GeU {
		arr = arm64.VectorArrangement16B
	} else if o.Type <= wazeroir.V128CmpTypeI16x8GeU {
		arr = arm64.VectorArrangement8H
	} else if o.Type <= wazeroir.V128CmpTypeI32x4GeU {
		arr = arm64.VectorArrangement4S
	} else if o.Type <= wazeroir.V128CmpTypeI64x2GeS {
		arr = arm64.VectorArrangement2D
	} else if o.Type <= wazeroir.V128CmpTypeF32x4Ge {
		arr = arm64.VectorArrangement4S
	} else { // f64x2
		arr = arm64.VectorArrangement2D
	}

	result := x1.register
	switch o.Type {
	case wazeroir.V128CmpTypeI8x16Eq, wazeroir.V128CmpTypeI16x8Eq, wazeroir.V128CmpTypeI32x4Eq, wazeroir.V128CmpTypeI64x2Eq:
		c.assembler.CompileTwoVectorRegistersToVectorRegister(arm64.CMEQ, x1.register, x2.register, result, arr)
	case wazeroir.V128CmpTypeI8x16Ne, wazeroir.V128CmpTypeI16x8Ne, wazeroir.V128CmpTypeI32x4Ne, wazeroir.V128CmpTypeI64x2Ne:
		c.assembler.CompileTwoVectorRegistersToVectorRegister(arm64.CMEQ, x1.register, x2.register, result, arr)
		// Reverse the condition by flipping all bits.
		c.assembler.CompileVectorRegisterToVectorRegister(arm64.NOT, result, result,
			arm64.VectorArrangement16B, arm64.VectorIndexNone, arm64.VectorIndexNone)
	case wazeroir.V128CmpTypeI8x16LtS, wazeroir.V128CmpTypeI16x8LtS, wazeroir.V128CmpTypeI32x4LtS, wazeroir.V128CmpTypeI64x2LtS:
		c.assembler.CompileTwoVectorRegistersToVectorRegister(arm64.CMGT, x1.register, x2.register, result, arr)
	case wazeroir.V128CmpTypeI8x16LtU, wazeroir.V128CmpTypeI16x8LtU, wazeroir.V128CmpTypeI32x4LtU:
		c.assembler.CompileTwoVectorRegistersToVectorRegister(arm64.CMHI, x1.register, x2.register, result, arr)
	case wazeroir.V128CmpTypeI8x16GtS, wazeroir.V128CmpTypeI16x8GtS, wazeroir.V128CmpTypeI32x4GtS, wazeroir.V128CmpTypeI64x2GtS:
		c.assembler.CompileTwoVectorRegistersToVectorRegister(arm64.CMGT, x2.register, x1.register, result, arr)
	case wazeroir.V128CmpTypeI8x16GtU, wazeroir.V128CmpTypeI16x8GtU, wazeroir.V128CmpTypeI32x4GtU:
		c.assembler.CompileTwoVectorRegistersToVectorRegister(arm64.CMHI, x2.register, x1.register, result, arr)
	case wazeroir.V128CmpTypeI8x16LeS, wazeroir.V128CmpTypeI16x8LeS, wazeroir.V128CmpTypeI32x4LeS, wazeroir.V128CmpTypeI64x2LeS:
		c.assembler.CompileTwoVectorRegistersToVectorRegister(arm64.CMGE, x1.register, x2.register, result, arr)
	case wazeroir.V128CmpTypeI8x16LeU, wazeroir.V128CmpTypeI16x8LeU, wazeroir.V128CmpTypeI32x4LeU:
		c.assembler.CompileTwoVectorRegistersToVectorRegister(arm64.CMHS, x1.register, x2.register, result, arr)
	case wazeroir.V128CmpTypeI8x16GeS, wazeroir.V128CmpTypeI16x8GeS, wazeroir.V128CmpTypeI32x4GeS, wazeroir.V128CmpTypeI64x2GeS:
		c.assembler.CompileTwoVectorRegistersToVectorRegister(arm64.CMGE, x2.register, x1.register, result, arr)
	case wazeroir.V128CmpTypeI8x16GeU, wazeroir.V128CmpTypeI16x8GeU, wazeroir.V128CmpTypeI32x4GeU:
		c.assembler.CompileTwoVectorRegistersToVectorRegister(arm64.CMHS, x2.register, x1.register, result, arr)
	case wazeroir.V128CmpTypeF32x4Eq, wazeroir.V128CmpTypeF64x2Eq:
		c.assembler.CompileTwoVectorRegistersToVectorRegister(arm64.FCMEQ, x2.register, x1.register, result, arr)
	case wazeroir.V128CmpTypeF32x4Ne, wazeroir.V128CmpTypeF64x2Ne:
		c.assembler.CompileTwoVectorRegistersToVectorRegister(arm64.FCMEQ, x2.register, x1.register, result, arr)
		// Reverse the condition by flipping all bits.
		c.assembler.CompileVectorRegisterToVectorRegister(arm64.NOT, result, result,
			arm64.VectorArrangement16B, arm64.VectorIndexNone, arm64.VectorIndexNone)
	case wazeroir.V128CmpTypeF32x4Lt, wazeroir.V128CmpTypeF64x2Lt:
		c.assembler.CompileTwoVectorRegistersToVectorRegister(arm64.FCMGT, x1.register, x2.register, result, arr)
	case wazeroir.V128CmpTypeF32x4Le, wazeroir.V128CmpTypeF64x2Le:
		c.assembler.CompileTwoVectorRegistersToVectorRegister(arm64.FCMGE, x1.register, x2.register, result, arr)
	case wazeroir.V128CmpTypeF32x4Gt, wazeroir.V128CmpTypeF64x2Gt:
		c.assembler.CompileTwoVectorRegistersToVectorRegister(arm64.FCMGT, x2.register, x1.register, result, arr)
	case wazeroir.V128CmpTypeF32x4Ge, wazeroir.V128CmpTypeF64x2Ge:
		c.assembler.CompileTwoVectorRegistersToVectorRegister(arm64.FCMGE, x2.register, x1.register, result, arr)
	}

	c.markRegisterUnused(x2.register)
	c.pushVectorRuntimeValueLocationOnRegister(result)
	return nil
}

// compileV128AddSat implements compiler.compileV128AddSat for arm64.
func (c *arm64Compiler) compileV128AddSat(o wazeroir.OperationV128AddSat) error {
	var inst asm.Instruction
	if o.Signed {
		inst = arm64.VSQADD
	} else {
		inst = arm64.VUQADD
	}
	return c.compileV128x2BinOp(inst, defaultArrangementForShape(o.Shape))
}

// compileV128SubSat implements compiler.compileV128SubSat for arm64.
func (c *arm64Compiler) compileV128SubSat(o wazeroir.OperationV128SubSat) error {
	var inst asm.Instruction
	if o.Signed {
		inst = arm64.VSQSUB
	} else {
		inst = arm64.VUQSUB
	}
	return c.compileV128x2BinOp(inst, defaultArrangementForShape(o.Shape))
}

// compileV128Mul implements compiler.compileV128Mul for arm64.
func (c *arm64Compiler) compileV128Mul(o wazeroir.OperationV128Mul) (err error) {
	switch o.Shape {
	case wazeroir.ShapeI8x16, wazeroir.ShapeI16x8, wazeroir.ShapeI32x4:
		err = c.compileV128x2BinOp(arm64.VMUL, defaultArrangementForShape(o.Shape))
	case wazeroir.ShapeF32x4, wazeroir.ShapeF64x2:
		err = c.compileV128x2BinOp(arm64.VFMUL, defaultArrangementForShape(o.Shape))
	case wazeroir.ShapeI64x2:
		x2 := c.locationStack.popV128()
		if err = c.compileEnsureOnRegister(x2); err != nil {
			return
		}

		x1 := c.locationStack.popV128()
		if err = c.compileEnsureOnRegister(x1); err != nil {
			return
		}

		src1, src2 := x1.register, x2.register

		tmp1, err := c.allocateRegister(registerTypeVector)
		if err != nil {
			return err
		}
		c.markRegisterUsed(tmp1)

		tmp2, err := c.allocateRegister(registerTypeVector)
		if err != nil {
			return err
		}

		c.markRegisterUsed(tmp2)

		tmp3, err := c.allocateRegister(registerTypeVector)
		if err != nil {
			return err
		}

		// Following the algorithm in https://chromium-review.googlesource.com/c/v8/v8/+/1781696
		c.assembler.CompileVectorRegisterToVectorRegister(arm64.REV64, src2, tmp2,
			arm64.VectorArrangement4S, arm64.VectorIndexNone, arm64.VectorIndexNone)
		c.assembler.CompileTwoVectorRegistersToVectorRegister(arm64.VMUL, src1, tmp2, tmp2, arm64.VectorArrangement4S)

		c.assembler.CompileVectorRegisterToVectorRegister(arm64.XTN, src1, tmp1,
			arm64.VectorArrangement2D, arm64.VectorIndexNone, arm64.VectorIndexNone)

		c.assembler.CompileVectorRegisterToVectorRegister(arm64.VADDP, tmp2, tmp2, arm64.VectorArrangement4S,
			arm64.VectorIndexNone, arm64.VectorIndexNone,
		)

		c.assembler.CompileVectorRegisterToVectorRegister(arm64.XTN, src2, tmp3,
			arm64.VectorArrangement2D, arm64.VectorIndexNone, arm64.VectorIndexNone)

		c.assembler.CompileVectorRegisterToVectorRegister(arm64.SHLL, tmp2, src1,
			arm64.VectorArrangement2S, arm64.VectorIndexNone, arm64.VectorIndexNone)

		c.assembler.CompileTwoVectorRegistersToVectorRegister(arm64.VUMLAL, tmp3, tmp1, src1, arm64.VectorArrangement2S)

		c.markRegisterUnused(src2, tmp1, tmp2)
		c.pushVectorRuntimeValueLocationOnRegister(src1)
	}
	return
}

// compileV128Div implements compiler.compileV128Div for arm64.
func (c *arm64Compiler) compileV128Div(o wazeroir.OperationV128Div) error {
	var arr arm64.VectorArrangement
	var inst asm.Instruction
	switch o.Shape {
	case wazeroir.ShapeF32x4:
		arr = arm64.VectorArrangement4S
		inst = arm64.VFDIV
	case wazeroir.ShapeF64x2:
		arr = arm64.VectorArrangement2D
		inst = arm64.VFDIV
	}
	return c.compileV128x2BinOp(inst, arr)
}

// compileV128Neg implements compiler.compileV128Neg for arm64.
func (c *arm64Compiler) compileV128Neg(o wazeroir.OperationV128Neg) error {
	var inst asm.Instruction
	if o.Shape <= wazeroir.ShapeI64x2 { // Integer lanes
		inst = arm64.VNEG
	} else { // Floating point lanes
		inst = arm64.VFNEG
	}
	return c.compileV128UniOp(inst, defaultArrangementForShape(o.Shape))
}

// compileV128Sqrt implements compiler.compileV128Sqrt for arm64.
func (c *arm64Compiler) compileV128Sqrt(o wazeroir.OperationV128Sqrt) error {
	var arr arm64.VectorArrangement
	switch o.Shape {
	case wazeroir.ShapeF32x4:
		arr = arm64.VectorArrangement4S
	case wazeroir.ShapeF64x2:
		arr = arm64.VectorArrangement2D
	}
	return c.compileV128UniOp(arm64.VFSQRT, arr)
}

// compileV128Abs implements compiler.compileV128Abs for arm64.
func (c *arm64Compiler) compileV128Abs(o wazeroir.OperationV128Abs) error {
	var inst asm.Instruction
	if o.Shape <= wazeroir.ShapeI64x2 { // Integer lanes
		inst = arm64.VABS
	} else { // Floating point lanes
		inst = arm64.VFABS
	}
	return c.compileV128UniOp(inst, defaultArrangementForShape(o.Shape))
}

// compileV128Popcnt implements compiler.compileV128Popcnt for arm64.
func (c *arm64Compiler) compileV128Popcnt(o wazeroir.OperationV128Popcnt) error {
	return c.compileV128UniOp(arm64.VCNT, defaultArrangementForShape(o.Shape))
}

// compileV128Min implements compiler.compileV128Min for arm64.
func (c *arm64Compiler) compileV128Min(o wazeroir.OperationV128Min) error {
	var inst asm.Instruction
	if o.Shape <= wazeroir.ShapeI64x2 { // Integer lanes
		if o.Signed {
			inst = arm64.SMIN
		} else {
			inst = arm64.UMIN
		}
	} else { // Floating point lanes
		inst = arm64.VFMIN
	}
	return c.compileV128x2BinOp(inst, defaultArrangementForShape(o.Shape))
}

func defaultArrangementForShape(s wazeroir.Shape) (arr arm64.VectorArrangement) {
	switch s {
	case wazeroir.ShapeI8x16:
		arr = arm64.VectorArrangement16B
	case wazeroir.ShapeI16x8:
		arr = arm64.VectorArrangement8H
	case wazeroir.ShapeI32x4:
		arr = arm64.VectorArrangement4S
	case wazeroir.ShapeI64x2:
		arr = arm64.VectorArrangement2D
	case wazeroir.ShapeF32x4:
		arr = arm64.VectorArrangement4S
	case wazeroir.ShapeF64x2:
		arr = arm64.VectorArrangement2D
	}
	return
}

// compileV128Max implements compiler.compileV128Max for arm64.
func (c *arm64Compiler) compileV128Max(o wazeroir.OperationV128Max) error {
	var inst asm.Instruction
	if o.Shape <= wazeroir.ShapeI64x2 { // Integer lanes
		if o.Signed {
			inst = arm64.SMAX
		} else {
			inst = arm64.UMAX
		}
	} else { // Floating point lanes
		inst = arm64.VFMAX
	}
	return c.compileV128x2BinOp(inst, defaultArrangementForShape(o.Shape))
}

// compileV128AvgrU implements compiler.compileV128AvgrU for arm64.
func (c *arm64Compiler) compileV128AvgrU(o wazeroir.OperationV128AvgrU) error {
	return c.compileV128x2BinOp(arm64.URHADD, defaultArrangementForShape(o.Shape))
}

// compileV128Pmin implements compiler.compileV128Pmin for arm64.
func (c *arm64Compiler) compileV128Pmin(o wazeroir.OperationV128Pmin) error {
	return c.compileV128PseudoMinOrMax(defaultArrangementForShape(o.Shape), false)
}

// compileV128Pmax implements compiler.compileV128Pmax for arm64.
func (c *arm64Compiler) compileV128Pmax(o wazeroir.OperationV128Pmax) error {
	return c.compileV128PseudoMinOrMax(defaultArrangementForShape(o.Shape), true)
}

// compileV128PseudoMinOrMax implements compileV128Pmax and compileV128Pmin.
func (c *arm64Compiler) compileV128PseudoMinOrMax(arr arm64.VectorArrangement, max bool) error {
	x2 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x2); err != nil {
		return err
	}

	x1 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x1); err != nil {
		return err
	}

	result, err := c.allocateRegister(registerTypeVector)
	if err != nil {
		return err
	}

	x1r, x2r := x1.register, x2.register

	// Sets all bits on each lane if x1r's lane satisfies the condition (min or max), zeros otherwise.
	if max {
		c.assembler.CompileTwoVectorRegistersToVectorRegister(arm64.FCMGT, x1r, x2r, result, arr)
	} else {
		c.assembler.CompileTwoVectorRegistersToVectorRegister(arm64.FCMGT, x2r, x1r, result, arr)
	}
	// Select each bit based on the result bits ^.
	c.assembler.CompileTwoVectorRegistersToVectorRegister(arm64.BSL, x1r, x2r, result, arm64.VectorArrangement16B)

	c.markRegisterUnused(x1r, x2r)
	c.pushVectorRuntimeValueLocationOnRegister(result)
	return nil
}

// compileV128Ceil implements compiler.compileV128Ceil for arm64.
func (c *arm64Compiler) compileV128Ceil(o wazeroir.OperationV128Ceil) error {
	var arr arm64.VectorArrangement
	switch o.Shape {
	case wazeroir.ShapeF32x4:
		arr = arm64.VectorArrangement4S
	case wazeroir.ShapeF64x2:
		arr = arm64.VectorArrangement2D
	}
	return c.compileV128UniOp(arm64.VFRINTP, arr)
}

// compileV128Floor implements compiler.compileV128Floor for arm64.
func (c *arm64Compiler) compileV128Floor(o wazeroir.OperationV128Floor) error {
	var arr arm64.VectorArrangement
	switch o.Shape {
	case wazeroir.ShapeF32x4:
		arr = arm64.VectorArrangement4S
	case wazeroir.ShapeF64x2:
		arr = arm64.VectorArrangement2D
	}
	return c.compileV128UniOp(arm64.VFRINTM, arr)
}

// compileV128Trunc implements compiler.compileV128Trunc for arm64.
func (c *arm64Compiler) compileV128Trunc(o wazeroir.OperationV128Trunc) error {
	var arr arm64.VectorArrangement
	switch o.Shape {
	case wazeroir.ShapeF32x4:
		arr = arm64.VectorArrangement4S
	case wazeroir.ShapeF64x2:
		arr = arm64.VectorArrangement2D
	}
	return c.compileV128UniOp(arm64.VFRINTZ, arr)
}

// compileV128Nearest implements compiler.compileV128Nearest for arm64.
func (c *arm64Compiler) compileV128Nearest(o wazeroir.OperationV128Nearest) error {
	var arr arm64.VectorArrangement
	switch o.Shape {
	case wazeroir.ShapeF32x4:
		arr = arm64.VectorArrangement4S
	case wazeroir.ShapeF64x2:
		arr = arm64.VectorArrangement2D
	}
	return c.compileV128UniOp(arm64.VFRINTN, arr)
}

// compileV128Extend implements compiler.compileV128Extend for arm64.
func (c *arm64Compiler) compileV128Extend(o wazeroir.OperationV128Extend) error {
	var inst asm.Instruction
	var arr arm64.VectorArrangement
	if o.UseLow {
		if o.Signed {
			inst = arm64.SSHLL
		} else {
			inst = arm64.USHLL
		}

		switch o.OriginShape {
		case wazeroir.ShapeI8x16:
			arr = arm64.VectorArrangement8B
		case wazeroir.ShapeI16x8:
			arr = arm64.VectorArrangement4H
		case wazeroir.ShapeI32x4:
			arr = arm64.VectorArrangement2S
		}
	} else {
		if o.Signed {
			inst = arm64.SSHLL2
		} else {
			inst = arm64.USHLL2
		}
		arr = defaultArrangementForShape(o.OriginShape)
	}

	return c.compileV128UniOp(inst, arr)
}

// compileV128ExtMul implements compiler.compileV128ExtMul for arm64.
func (c *arm64Compiler) compileV128ExtMul(o wazeroir.OperationV128ExtMul) error {
	var inst asm.Instruction
	var arr arm64.VectorArrangement
	if o.UseLow {
		if o.Signed {
			inst = arm64.SMULL
		} else {
			inst = arm64.UMULL
		}

		switch o.OriginShape {
		case wazeroir.ShapeI8x16:
			arr = arm64.VectorArrangement8B
		case wazeroir.ShapeI16x8:
			arr = arm64.VectorArrangement4H
		case wazeroir.ShapeI32x4:
			arr = arm64.VectorArrangement2S
		}
	} else {
		if o.Signed {
			inst = arm64.SMULL2
		} else {
			inst = arm64.UMULL2
		}
		arr = defaultArrangementForShape(o.OriginShape)
	}

	return c.compileV128x2BinOp(inst, arr)
}

// compileV128Q15mulrSatS implements compiler.compileV128Q15mulrSatS for arm64.
func (c *arm64Compiler) compileV128Q15mulrSatS(wazeroir.OperationV128Q15mulrSatS) error {
	return c.compileV128x2BinOp(arm64.SQRDMULH, arm64.VectorArrangement8H)
}

// compileV128ExtAddPairwise implements compiler.compileV128ExtAddPairwise for arm64.
func (c *arm64Compiler) compileV128ExtAddPairwise(o wazeroir.OperationV128ExtAddPairwise) error {
	var inst asm.Instruction
	if o.Signed {
		inst = arm64.SADDLP
	} else {
		inst = arm64.UADDLP
	}
	return c.compileV128UniOp(inst, defaultArrangementForShape(o.OriginShape))
}

// compileV128FloatPromote implements compiler.compileV128FloatPromote for arm64.
func (c *arm64Compiler) compileV128FloatPromote(wazeroir.OperationV128FloatPromote) error {
	return c.compileV128UniOp(arm64.FCVTL, arm64.VectorArrangement2S)
}

// compileV128FloatDemote implements compiler.compileV128FloatDemote for arm64.
func (c *arm64Compiler) compileV128FloatDemote(wazeroir.OperationV128FloatDemote) error {
	return c.compileV128UniOp(arm64.FCVTN, arm64.VectorArrangement2S)
}

// compileV128FConvertFromI implements compiler.compileV128FConvertFromI for arm64.
func (c *arm64Compiler) compileV128FConvertFromI(o wazeroir.OperationV128FConvertFromI) (err error) {
	if o.DestinationShape == wazeroir.ShapeF32x4 {
		if o.Signed {
			err = c.compileV128UniOp(arm64.VSCVTF, defaultArrangementForShape(o.DestinationShape))
		} else {
			err = c.compileV128UniOp(arm64.VUCVTF, defaultArrangementForShape(o.DestinationShape))
		}
		return
	} else { // f64x2
		v := c.locationStack.popV128()
		if err = c.compileEnsureOnRegister(v); err != nil {
			return
		}
		vr := v.register

		var expand, convert asm.Instruction
		if o.Signed {
			expand, convert = arm64.SSHLL, arm64.VSCVTF
		} else {
			expand, convert = arm64.USHLL, arm64.VUCVTF
		}

		// Expand lower two 32-bit lanes as two 64-bit lanes.
		c.assembler.CompileVectorRegisterToVectorRegisterWithConst(expand, vr, vr, arm64.VectorArrangement2S, 0)
		// Convert these two 64-bit (integer) values on each lane as double precision values.
		c.assembler.CompileVectorRegisterToVectorRegister(convert, vr, vr, arm64.VectorArrangement2D,
			arm64.VectorIndexNone, arm64.VectorIndexNone)
		c.pushVectorRuntimeValueLocationOnRegister(vr)
	}
	return
}

// compileV128Dot implements compiler.compileV128Dot for arm64.
func (c *arm64Compiler) compileV128Dot(wazeroir.OperationV128Dot) error {
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

	// Multiply lower integers and get the 32-bit results into tmp.
	c.assembler.CompileTwoVectorRegistersToVectorRegister(arm64.SMULL, x1r, x2r, tmp, arm64.VectorArrangement4H)
	// Multiply higher integers and get the 32-bit results into x1r.
	c.assembler.CompileTwoVectorRegistersToVectorRegister(arm64.SMULL2, x1r, x2r, x1r, arm64.VectorArrangement8H)
	// Adds these two results into x1r.
	c.assembler.CompileTwoVectorRegistersToVectorRegister(arm64.VADDP, x1r, tmp, x1r, arm64.VectorArrangement4S)

	c.markRegisterUnused(x2r)
	c.pushVectorRuntimeValueLocationOnRegister(x1r)

	return nil
}

// compileV128Narrow implements compiler.compileV128Narrow for arm64.
func (c *arm64Compiler) compileV128Narrow(o wazeroir.OperationV128Narrow) error {
	x2 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x2); err != nil {
		return err
	}

	x1 := c.locationStack.popV128()
	if err := c.compileEnsureOnRegister(x1); err != nil {
		return err
	}

	x1r, x2r := x1.register, x2.register

	var arr, arr2 arm64.VectorArrangement
	switch o.OriginShape {
	case wazeroir.ShapeI16x8:
		arr = arm64.VectorArrangement8B
		arr2 = arm64.VectorArrangement16B
	case wazeroir.ShapeI32x4:
		arr = arm64.VectorArrangement4H
		arr2 = arm64.VectorArrangement8H
	}

	var lo, hi asm.Instruction
	if o.Signed {
		lo, hi = arm64.SQXTN, arm64.SQXTN2
	} else {
		lo, hi = arm64.SQXTUN, arm64.SQXTUN2
	}

	// Narrow lanes on x1r and write them into lower-half of x1r.
	c.assembler.CompileVectorRegisterToVectorRegister(lo, x1r, x1r, arr, arm64.VectorIndexNone, arm64.VectorIndexNone)
	// Narrow lanes on x2r and write them into higher-half of x1r.
	c.assembler.CompileVectorRegisterToVectorRegister(hi, x2r, x1r, arr2, arm64.VectorIndexNone, arm64.VectorIndexNone)

	c.markRegisterUnused(x2r)
	c.pushVectorRuntimeValueLocationOnRegister(x1r)
	return nil
}

// compileV128ITruncSatFromF implements compiler.compileV128ITruncSatFromF for arm64.
func (c *arm64Compiler) compileV128ITruncSatFromF(o wazeroir.OperationV128ITruncSatFromF) (err error) {
	v := c.locationStack.popV128()
	if err = c.compileEnsureOnRegister(v); err != nil {
		return err
	}

	var cvt asm.Instruction
	if o.Signed {
		cvt = arm64.VFCVTZS
	} else {
		cvt = arm64.VFCVTZU
	}

	c.assembler.CompileVectorRegisterToVectorRegister(cvt, v.register, v.register,
		defaultArrangementForShape(o.OriginShape), arm64.VectorIndexNone, arm64.VectorIndexNone,
	)

	if o.OriginShape == wazeroir.ShapeF64x2 {
		var narrow asm.Instruction
		if o.Signed {
			narrow = arm64.SQXTN
		} else {
			narrow = arm64.UQXTN
		}
		c.assembler.CompileVectorRegisterToVectorRegister(narrow, v.register, v.register,
			arm64.VectorArrangement2S, arm64.VectorIndexNone, arm64.VectorIndexNone,
		)
	}

	c.pushVectorRuntimeValueLocationOnRegister(v.register)
	return
}
