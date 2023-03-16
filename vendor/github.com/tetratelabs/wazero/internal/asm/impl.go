package asm

import (
	"encoding/binary"
	"fmt"
)

// BaseAssemblerImpl includes code common to all architectures.
//
// Note: When possible, add code here instead of in architecture-specific files to reduce drift:
// As this is internal, exporting symbols only to reduce duplication is ok.
type BaseAssemblerImpl struct {
	// SetBranchTargetOnNextNodes holds branch kind instructions (BR, conditional BR, etc.)
	// where we want to set the next coming instruction as the destination of these BR instructions.
	SetBranchTargetOnNextNodes []Node

	// JumpTableEntries holds the information to build jump tables.
	JumpTableEntries []JumpTableEntry
}

// JumpTableEntry is the necessary data to build a jump table.
// This is exported for testing purpose.
type JumpTableEntry struct {
	t                        *StaticConst
	labelInitialInstructions []Node
}

// SetJumpTargetOnNext implements AssemblerBase.SetJumpTargetOnNext
func (a *BaseAssemblerImpl) SetJumpTargetOnNext(nodes ...Node) {
	a.SetBranchTargetOnNextNodes = append(a.SetBranchTargetOnNextNodes, nodes...)
}

// BuildJumpTable implements AssemblerBase.BuildJumpTable
func (a *BaseAssemblerImpl) BuildJumpTable(table *StaticConst, labelInitialInstructions []Node) {
	a.JumpTableEntries = append(a.JumpTableEntries, JumpTableEntry{
		t:                        table,
		labelInitialInstructions: labelInitialInstructions,
	})
}

// FinalizeJumpTableEntry finalizes the build tables inside the given code.
func (a *BaseAssemblerImpl) FinalizeJumpTableEntry(code []byte) (err error) {
	for i := range a.JumpTableEntries {
		ent := &a.JumpTableEntries[i]
		labelInitialInstructions := ent.labelInitialInstructions
		table := ent.t
		// Compile the offset table for each target.
		base := labelInitialInstructions[0].OffsetInBinary()
		for i, nop := range labelInitialInstructions {
			if nop.OffsetInBinary()-base >= JumpTableMaximumOffset {
				return fmt.Errorf("too large br_table")
			}
			// We store the offset from the beginning of the L0's initial instruction.
			binary.LittleEndian.PutUint32(code[table.OffsetInBinary+uint64(i*4):table.OffsetInBinary+uint64((i+1)*4)],
				uint32(nop.OffsetInBinary())-uint32(base))
		}
	}
	return
}
