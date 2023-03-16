package asm

import (
	"fmt"
	"math"
)

// Register represents architecture-specific registers.
type Register byte

// NilRegister is the only architecture-independent register, and
// can be used to indicate that no register is specified.
const NilRegister Register = 0

// Instruction represents architecture-specific instructions.
type Instruction uint16 // to accommodate the high cardinality of vector ops

// ConditionalRegisterState represents architecture-specific conditional
// register's states.
type ConditionalRegisterState byte

// ConditionalRegisterStateUnset is the only architecture-independent conditional state, and
// can be used to indicate that no conditional state is specified.
const ConditionalRegisterStateUnset ConditionalRegisterState = 0

// Node represents a node in the linked list of assembled operations.
type Node interface {
	fmt.Stringer

	// AssignJumpTarget assigns the given target node as the destination of
	// jump instruction for this Node.
	AssignJumpTarget(target Node)

	// AssignDestinationConstant assigns the given constant as the destination
	// of the instruction for this node.
	AssignDestinationConstant(value ConstantValue)

	// AssignSourceConstant assigns the given constant as the source
	// of the instruction for this node.
	AssignSourceConstant(value ConstantValue)

	// OffsetInBinary returns the offset of this node in the assembled binary.
	OffsetInBinary() NodeOffsetInBinary
}

// NodeOffsetInBinary represents an offset of this node in the final binary.
type NodeOffsetInBinary = uint64

// ConstantValue represents a constant value used in an instruction.
type ConstantValue = int64

// StaticConst represents an arbitrary constant bytes which are pooled and emitted by assembler into the binary.
// These constants can be referenced by instructions.
type StaticConst struct {
	Raw []byte
	// OffsetInBinary is the offset of this static const in the result binary.
	OffsetInBinary uint64
	// offsetFinalizedCallbacks holds callbacks which are called when .OffsetInBinary is finalized by assembler implementation.
	offsetFinalizedCallbacks []func(offsetOfConstInBinary uint64)
}

// NewStaticConst returns the pointer to the new NewStaticConst for given bytes.
func NewStaticConst(raw []byte) *StaticConst {
	return &StaticConst{Raw: raw}
}

// AddOffsetFinalizedCallback adds a callback into offsetFinalizedCallbacks.
func (s *StaticConst) AddOffsetFinalizedCallback(cb func(offsetOfConstInBinary uint64)) {
	s.offsetFinalizedCallbacks = append(s.offsetFinalizedCallbacks, cb)
}

// SetOffsetInBinary finalizes the offset of this StaticConst, and invokes callbacks.
func (s *StaticConst) SetOffsetInBinary(offset uint64) {
	s.OffsetInBinary = offset
	for _, cb := range s.offsetFinalizedCallbacks {
		cb(offset)
	}
}

// StaticConstPool holds a bulk of StaticConst which are yet to be emitted into the binary.
type StaticConstPool struct {
	// FirstUseOffsetInBinary holds the offset of the first instruction which accesses this const pool .
	FirstUseOffsetInBinary *NodeOffsetInBinary
	Consts                 []*StaticConst
	// addedConsts is used to deduplicate the consts to reduce the final size of binary.
	// Note: we can use map on .consts field and remove this field,
	// but we have the separate field for deduplication in order to have deterministic assembling behavior.
	addedConsts map[*StaticConst]struct{}
	// PoolSizeInBytes is the current size of the pool in bytes.
	PoolSizeInBytes int
}

// NewStaticConstPool returns the pointer to a new StaticConstPool.
func NewStaticConstPool() *StaticConstPool {
	return &StaticConstPool{addedConsts: map[*StaticConst]struct{}{}}
}

// AddConst adds a *StaticConst into the pool if it's not already added.
func (p *StaticConstPool) AddConst(c *StaticConst, useOffset NodeOffsetInBinary) {
	if _, ok := p.addedConsts[c]; ok {
		return
	}

	if p.FirstUseOffsetInBinary == nil {
		p.FirstUseOffsetInBinary = &useOffset
	}

	p.Consts = append(p.Consts, c)
	p.PoolSizeInBytes += len(c.Raw)
	p.addedConsts[c] = struct{}{}
}

// AssemblerBase is the common interface for assemblers among multiple architectures.
//
// Note: some of them can be implemented in an arch-independent way, but not all can be
// implemented as such. However, we intentionally put such arch-dependant methods here
// in order to provide the common documentation interface.
type AssemblerBase interface {
	// Reset resets the state of Assembler implementation and mark it ready for
	// the compilation of the new function compilation.
	Reset()

	// Assemble produces the final binary for the assembled operations.
	Assemble() ([]byte, error)

	// SetJumpTargetOnNext instructs the assembler that the next node must be
	// assigned to the given node's jump destination.
	SetJumpTargetOnNext(nodes ...Node)

	// BuildJumpTable calculates the offsets between the first instruction `initialInstructions[0]`
	// and others (e.g. initialInstructions[3]), and wrote the calculated offsets into pre-allocated
	// `table` StaticConst in little endian.
	BuildJumpTable(table *StaticConst, initialInstructions []Node)

	// CompileStandAlone adds an instruction to take no arguments.
	CompileStandAlone(instruction Instruction) Node

	// CompileConstToRegister adds an instruction where source operand is `value` as constant and destination is `destinationReg` register.
	CompileConstToRegister(instruction Instruction, value ConstantValue, destinationReg Register) Node

	// CompileRegisterToRegister adds an instruction where source and destination operands are registers.
	CompileRegisterToRegister(instruction Instruction, from, to Register)

	// CompileMemoryToRegister adds an instruction where source operands is the memory address specified by `sourceBaseReg+sourceOffsetConst`
	// and the destination is `destinationReg` register.
	CompileMemoryToRegister(
		instruction Instruction,
		sourceBaseReg Register,
		sourceOffsetConst ConstantValue,
		destinationReg Register,
	)

	// CompileRegisterToMemory adds an instruction where source operand is `sourceRegister` register and the destination is the
	// memory address specified by `destinationBaseRegister+destinationOffsetConst`.
	CompileRegisterToMemory(
		instruction Instruction,
		sourceRegister Register,
		destinationBaseRegister Register,
		destinationOffsetConst ConstantValue,
	)

	// CompileJump adds jump-type instruction and returns the corresponding Node in the assembled linked list.
	CompileJump(jmpInstruction Instruction) Node

	// CompileJumpToRegister adds jump-type instruction whose destination is the memory address specified by `reg` register.
	CompileJumpToRegister(jmpInstruction Instruction, reg Register)

	// CompileReadInstructionAddress adds an ADR instruction to set the absolute address of "target instruction"
	// into destinationRegister. "target instruction" is specified by beforeTargetInst argument and
	// the target is determined by "the instruction right after beforeTargetInst type".
	//
	// For example, if `beforeTargetInst == RET` and we have the instruction sequence like
	// `ADR -> X -> Y -> ... -> RET -> MOV`, then the `ADR` instruction emitted by this function set the absolute
	// address of `MOV` instruction into the destination register.
	CompileReadInstructionAddress(destinationRegister Register, beforeAcquisitionTargetInstruction Instruction)
}

// JumpTableMaximumOffset represents the limit on the size of jump table in bytes.
// When users try loading an extremely large WebAssembly binary which contains a br_table
// statement with approximately 4294967296 (2^32) targets. Realistically speaking, that kind of binary
// could result in more than ten gigabytes of native compiled code where we have to care about
// huge stacks whose height might exceed 32-bit range, and such huge stack doesn't work with the
// current implementation.
const JumpTableMaximumOffset = math.MaxUint32
