package amd64

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/tetratelabs/wazero/internal/asm"
)

// defaultMaxDisplacementForConstantPool is the maximum displacement allowed for literal move instructions which access
// the constant pool. This is set as 2 ^30 conservatively while the actual limit is 2^31 since we actually allow this
// limit plus max(length(c) for c in the pool) so we must ensure that limit is less than 2^31.
const defaultMaxDisplacementForConstantPool = 1 << 30

func (a *AssemblerImpl) maybeFlushConstants(isEndOfFunction bool) {
	if a.pool.FirstUseOffsetInBinary == nil {
		return
	}

	if isEndOfFunction ||
		// If the distance between (the first use in binary) and (end of constant pool) can be larger
		// than MaxDisplacementForConstantPool, we have to emit the constant pool now, otherwise
		// a const might be unreachable by a literal move whose maximum offset is +- 2^31.
		((a.pool.PoolSizeInBytes+a.buf.Len())-int(*a.pool.FirstUseOffsetInBinary)) >= a.MaxDisplacementForConstantPool {
		if !isEndOfFunction {
			// Adds the jump instruction to skip the constants if this is not the end of function.
			//
			// TODO: consider NOP padding for this jump, though this rarely happens as most functions should be
			// small enough to fit all consts after the end of function.
			if a.pool.PoolSizeInBytes >= math.MaxInt8-2 {
				// long (near-relative) jump: https://www.felixcloutier.com/x86/jmp
				a.buf.WriteByte(0xe9)
				a.WriteConst(int64(a.pool.PoolSizeInBytes), 32)
			} else {
				// short jump: https://www.felixcloutier.com/x86/jmp
				a.buf.WriteByte(0xeb)
				a.WriteConst(int64(a.pool.PoolSizeInBytes), 8)
			}
		}

		for _, c := range a.pool.Consts {
			c.SetOffsetInBinary(uint64(a.buf.Len()))
			a.buf.Write(c.Raw)
		}

		a.pool = asm.NewStaticConstPool() // reset
	}
}

type staticConstOpcode struct {
	opcode          []byte
	mandatoryPrefix byte
	rex             RexPrefix
}

var registerToStaticConstOpcodes = map[asm.Instruction]staticConstOpcode{
	// https://www.felixcloutier.com/x86/cmp
	CMPL: {opcode: []byte{0x3b}},
	CMPQ: {opcode: []byte{0x3b}, rex: RexPrefixW},
}

func (a *AssemblerImpl) encodeRegisterToStaticConst(n *nodeImpl) (err error) {
	opc, ok := registerToStaticConstOpcodes[n.instruction]
	if !ok {
		return errorEncodingUnsupported(n)
	}
	return a.encodeStaticConstImpl(n, opc.opcode, opc.rex, opc.mandatoryPrefix)
}

var staticConstToRegisterOpcodes = map[asm.Instruction]struct {
	opcode          []byte
	mandatoryPrefix byte
	rex             RexPrefix
}{
	// https://www.felixcloutier.com/x86/movdqu:vmovdqu8:vmovdqu16:vmovdqu32:vmovdqu64
	MOVDQU: {mandatoryPrefix: 0xf3, opcode: []byte{0x0f, 0x6f}},
	// https://www.felixcloutier.com/x86/lea
	LEAQ: {opcode: []byte{0x8d}, rex: RexPrefixW},
	// https://www.felixcloutier.com/x86/movupd
	MOVUPD: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x10}},
	// https://www.felixcloutier.com/x86/mov
	MOVL: {opcode: []byte{0x8b}},
	MOVQ: {opcode: []byte{0x8b}, rex: RexPrefixW},
	// https://www.felixcloutier.com/x86/ucomisd
	UCOMISD: {opcode: []byte{0x0f, 0x2e}, mandatoryPrefix: 0x66},
	// https://www.felixcloutier.com/x86/ucomiss
	UCOMISS: {opcode: []byte{0x0f, 0x2e}},
	// https://www.felixcloutier.com/x86/subss
	SUBSS: {opcode: []byte{0x0f, 0x5c}, mandatoryPrefix: 0xf3},
	// https://www.felixcloutier.com/x86/subsd
	SUBSD: {opcode: []byte{0x0f, 0x5c}, mandatoryPrefix: 0xf2},
	// https://www.felixcloutier.com/x86/cmp
	CMPL: {opcode: []byte{0x39}},
	CMPQ: {opcode: []byte{0x39}, rex: RexPrefixW},
	// https://www.felixcloutier.com/x86/add
	ADDL: {opcode: []byte{0x03}},
	ADDQ: {opcode: []byte{0x03}, rex: RexPrefixW},
}

var staticConstToVectorRegisterOpcodes = map[asm.Instruction]staticConstOpcode{
	// https://www.felixcloutier.com/x86/mov
	MOVL: {opcode: []byte{0x0f, 0x6e}, mandatoryPrefix: 0x66},
	MOVQ: {opcode: []byte{0x0f, 0x7e}, mandatoryPrefix: 0xf3},
}

func (a *AssemblerImpl) encodeStaticConstToRegister(n *nodeImpl) (err error) {
	var opc staticConstOpcode
	var ok bool
	if IsVectorRegister(n.dstReg) && (n.instruction == MOVL || n.instruction == MOVQ) {
		opc, ok = staticConstToVectorRegisterOpcodes[n.instruction]
	} else {
		opc, ok = staticConstToRegisterOpcodes[n.instruction]
	}
	if !ok {
		return errorEncodingUnsupported(n)
	}
	return a.encodeStaticConstImpl(n, opc.opcode, opc.rex, opc.mandatoryPrefix)
}

// encodeStaticConstImpl encodes an instruction where mod:r/m points to the memory location of the static constant n.staticConst,
// and the other operand is the register given at n.srcReg or n.dstReg.
func (a *AssemblerImpl) encodeStaticConstImpl(n *nodeImpl, opcode []byte, rex RexPrefix, mandatoryPrefix byte) (err error) {
	a.pool.AddConst(n.staticConst, uint64(a.buf.Len()))

	var reg asm.Register
	if n.dstReg != asm.NilRegister {
		reg = n.dstReg
	} else {
		reg = n.srcReg
	}

	reg3Bits, rexPrefix, err := register3bits(reg, registerSpecifierPositionModRMFieldReg)
	if err != nil {
		return err
	}

	rexPrefix |= rex

	var inst []byte
	n.staticConst.AddOffsetFinalizedCallback(func(offsetOfConstInBinary uint64) {
		bin := a.buf.Bytes()
		displacement := int(offsetOfConstInBinary) - int(n.OffsetInBinary()) - len(inst)
		displacementOffsetInInstruction := n.OffsetInBinary() + uint64(len(inst)-4)
		binary.LittleEndian.PutUint32(bin[displacementOffsetInInstruction:], uint32(int32(displacement)))
	})

	// https://wiki.osdev.org/X86-64_Instruction_Encoding#32.2F64-bit_addressing
	modRM := 0b00_000_101 | // Indicate "[RIP + 32bit displacement]" encoding.
		(reg3Bits << 3) // Place the reg on ModRM:reg.

	if mandatoryPrefix != 0 {
		inst = append(inst, mandatoryPrefix)
	}

	if rexPrefix != RexPrefixNone {
		inst = append(inst, rexPrefix)
	}

	inst = append(inst, opcode...)
	inst = append(inst, modRM,
		0x0, 0x0, 0x0, 0x0, // Preserve 4 bytes for displacement.
	)

	a.buf.Write(inst)
	return
}

// CompileStaticConstToRegister implements Assembler.CompileStaticConstToRegister.
func (a *AssemblerImpl) CompileStaticConstToRegister(instruction asm.Instruction, c *asm.StaticConst, dstReg asm.Register) (err error) {
	if len(c.Raw)%2 != 0 {
		err = fmt.Errorf("the length of a static constant must be even but was %d", len(c.Raw))
		return
	}

	n := a.newNode(instruction, operandTypesStaticConstToRegister)
	n.dstReg = dstReg
	n.staticConst = c
	return
}

// CompileRegisterToStaticConst implements Assembler.CompileRegisterToStaticConst.
func (a *AssemblerImpl) CompileRegisterToStaticConst(instruction asm.Instruction, srcReg asm.Register, c *asm.StaticConst) (err error) {
	if len(c.Raw)%2 != 0 {
		err = fmt.Errorf("the length of a static constant must be even but was %d", len(c.Raw))
		return
	}

	n := a.newNode(instruction, operandTypesRegisterToStaticConst)
	n.srcReg = srcReg
	n.staticConst = c
	return
}
