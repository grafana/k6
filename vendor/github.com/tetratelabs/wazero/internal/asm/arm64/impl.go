package arm64

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/tetratelabs/wazero/internal/asm"
)

type nodeImpl struct {
	instruction asm.Instruction

	offsetInBinaryField asm.NodeOffsetInBinary // Field suffix to dodge conflict with OffsetInBinary

	// jumpTarget holds the target node in the linked for the jump-kind instruction.
	jumpTarget *nodeImpl
	// next holds the next node from this node in the assembled linked list.
	next *nodeImpl

	types                            operandTypes
	srcReg, srcReg2, dstReg, dstReg2 asm.Register
	srcConst, dstConst               asm.ConstantValue

	vectorArrangement              VectorArrangement
	srcVectorIndex, dstVectorIndex VectorIndex

	// readInstructionAddressBeforeTargetInstruction holds the instruction right before the target of
	// read instruction address instruction. See asm.assemblerBase.CompileReadInstructionAddress.
	readInstructionAddressBeforeTargetInstruction asm.Instruction

	staticConst *asm.StaticConst
}

// AssignJumpTarget implements the same method as documented on asm.Node.
func (n *nodeImpl) AssignJumpTarget(target asm.Node) {
	n.jumpTarget = target.(*nodeImpl)
}

// AssignDestinationConstant implements the same method as documented on asm.Node.
func (n *nodeImpl) AssignDestinationConstant(value asm.ConstantValue) {
	n.dstConst = value
}

// AssignSourceConstant implements the same method as documented on asm.Node.
func (n *nodeImpl) AssignSourceConstant(value asm.ConstantValue) {
	n.srcConst = value
}

// OffsetInBinary implements the same method as documented on asm.Node.
func (n *nodeImpl) OffsetInBinary() asm.NodeOffsetInBinary {
	return n.offsetInBinaryField
}

// String implements fmt.Stringer.
//
// This is for debugging purpose, and the format is similar to the AT&T assembly syntax,
// meaning that this should look like "INSTRUCTION ${from}, ${to}" where each operand
// might be embraced by '[]' to represent the memory location, and multiple operands
// are embraced by `()`.
func (n *nodeImpl) String() (ret string) {
	instName := InstructionName(n.instruction)
	switch n.types {
	case operandTypesNoneToNone:
		ret = instName
	case operandTypesNoneToRegister:
		ret = fmt.Sprintf("%s %s", instName, RegisterName(n.dstReg))
	case operandTypesNoneToBranch:
		ret = fmt.Sprintf("%s {%v}", instName, n.jumpTarget)
	case operandTypesRegisterToRegister:
		ret = fmt.Sprintf("%s %s, %s", instName, RegisterName(n.srcReg), RegisterName(n.dstReg))
	case operandTypesLeftShiftedRegisterToRegister:
		ret = fmt.Sprintf("%s (%s, %s << %d), %s", instName, RegisterName(n.srcReg), RegisterName(n.srcReg2), n.srcConst, RegisterName(n.dstReg))
	case operandTypesTwoRegistersToRegister:
		ret = fmt.Sprintf("%s (%s, %s), %s", instName, RegisterName(n.srcReg), RegisterName(n.srcReg2), RegisterName(n.dstReg))
	case operandTypesThreeRegistersToRegister:
		ret = fmt.Sprintf("%s (%s, %s, %s), %s)", instName, RegisterName(n.srcReg), RegisterName(n.srcReg2), RegisterName(n.dstReg), RegisterName(n.dstReg2))
	case operandTypesTwoRegistersToNone:
		ret = fmt.Sprintf("%s (%s, %s)", instName, RegisterName(n.srcReg), RegisterName(n.srcReg2))
	case operandTypesRegisterAndConstToNone:
		ret = fmt.Sprintf("%s (%s, 0x%x)", instName, RegisterName(n.srcReg), n.srcConst)
	case operandTypesRegisterAndConstToRegister:
		ret = fmt.Sprintf("%s (%s, 0x%x), %s", instName, RegisterName(n.srcReg), n.srcConst, RegisterName(n.dstReg))
	case operandTypesRegisterToMemory:
		if n.dstReg2 != asm.NilRegister {
			ret = fmt.Sprintf("%s %s, [%s + %s]", instName, RegisterName(n.srcReg), RegisterName(n.dstReg), RegisterName(n.dstReg2))
		} else {
			ret = fmt.Sprintf("%s %s, [%s + 0x%x]", instName, RegisterName(n.srcReg), RegisterName(n.dstReg), n.dstConst)
		}
	case operandTypesMemoryToRegister:
		if n.srcReg2 != asm.NilRegister {
			ret = fmt.Sprintf("%s [%s + %s], %s", instName, RegisterName(n.srcReg), RegisterName(n.srcReg2), RegisterName(n.dstReg))
		} else {
			ret = fmt.Sprintf("%s [%s + 0x%x], %s", instName, RegisterName(n.srcReg), n.srcConst, RegisterName(n.dstReg))
		}
	case operandTypesConstToRegister:
		ret = fmt.Sprintf("%s 0x%x, %s", instName, n.srcConst, RegisterName(n.dstReg))
	case operandTypesRegisterToVectorRegister:
		ret = fmt.Sprintf("%s %s, %s.%s[%d]", instName, RegisterName(n.srcReg), RegisterName(n.dstReg), n.vectorArrangement, n.dstVectorIndex)
	case operandTypesVectorRegisterToRegister:
		ret = fmt.Sprintf("%s %s.%s[%d], %s", instName, RegisterName(n.srcReg), n.vectorArrangement, n.srcVectorIndex, RegisterName(n.dstReg))
	case operandTypesVectorRegisterToMemory:
		if n.dstReg2 != asm.NilRegister {
			ret = fmt.Sprintf("%s %s.%s, [%s + %s]", instName, RegisterName(n.srcReg), n.vectorArrangement, RegisterName(n.dstReg), RegisterName(n.dstReg2))
		} else {
			ret = fmt.Sprintf("%s %s.%s, [%s + 0x%x]", instName, RegisterName(n.srcReg), n.vectorArrangement, RegisterName(n.dstReg), n.dstConst)
		}
	case operandTypesMemoryToVectorRegister:
		ret = fmt.Sprintf("%s [%s], %s.%s", instName, RegisterName(n.srcReg), RegisterName(n.dstReg), n.vectorArrangement)
	case operandTypesVectorRegisterToVectorRegister:
		ret = fmt.Sprintf("%s %[2]s.%[4]s, %[3]s.%[4]s", instName, RegisterName(n.srcReg), RegisterName(n.dstReg), n.vectorArrangement)
	case operandTypesStaticConstToVectorRegister:
		ret = fmt.Sprintf("%s $%#x %s.%s", instName, n.staticConst.Raw, RegisterName(n.dstReg), n.vectorArrangement)
	case operandTypesTwoVectorRegistersToVectorRegister:
		ret = fmt.Sprintf("%s (%s.%[5]s, %[3]s.%[5]s), %[4]s.%[5]s", instName, RegisterName(n.srcReg), RegisterName(n.srcReg2), RegisterName(n.dstReg), n.vectorArrangement)
	}
	return
}

// operandType represents where an operand is placed for an instruction.
// Note: this is almost the same as obj.AddrType in GO assembler.
type operandType byte

const (
	operandTypeNone operandType = iota
	operandTypeRegister
	operandTypeLeftShiftedRegister
	operandTypeTwoRegisters
	operandTypeThreeRegisters
	operandTypeRegisterAndConst
	operandTypeMemory
	operandTypeConst
	operandTypeBranch
	operandTypeSIMDByte
	operandTypeTwoSIMDBytes
	operandTypeVectorRegister
	operandTypeTwoVectorRegisters
	operandTypeStaticConst
)

// String implements fmt.Stringer.
func (o operandType) String() (ret string) {
	switch o {
	case operandTypeNone:
		ret = "none"
	case operandTypeRegister:
		ret = "register"
	case operandTypeLeftShiftedRegister:
		ret = "left-shifted-register"
	case operandTypeTwoRegisters:
		ret = "two-registers"
	case operandTypeRegisterAndConst:
		ret = "register-and-const"
	case operandTypeMemory:
		ret = "memory"
	case operandTypeConst:
		ret = "const"
	case operandTypeBranch:
		ret = "branch"
	case operandTypeSIMDByte:
		ret = "simd-byte"
	case operandTypeTwoSIMDBytes:
		ret = "two-simd-bytes"
	case operandTypeVectorRegister:
		ret = "vector-register"
	case operandTypeStaticConst:
		ret = "static-const"
	case operandTypeTwoVectorRegisters:
		ret = "two-vector-registers"
	}
	return
}

// operandTypes represents the only combinations of two operandTypes used by wazero
type operandTypes struct{ src, dst operandType }

var (
	operandTypesNoneToNone                         = operandTypes{operandTypeNone, operandTypeNone}
	operandTypesNoneToRegister                     = operandTypes{operandTypeNone, operandTypeRegister}
	operandTypesNoneToBranch                       = operandTypes{operandTypeNone, operandTypeBranch}
	operandTypesRegisterToRegister                 = operandTypes{operandTypeRegister, operandTypeRegister}
	operandTypesLeftShiftedRegisterToRegister      = operandTypes{operandTypeLeftShiftedRegister, operandTypeRegister}
	operandTypesTwoRegistersToRegister             = operandTypes{operandTypeTwoRegisters, operandTypeRegister}
	operandTypesThreeRegistersToRegister           = operandTypes{operandTypeThreeRegisters, operandTypeRegister}
	operandTypesTwoRegistersToNone                 = operandTypes{operandTypeTwoRegisters, operandTypeNone}
	operandTypesRegisterAndConstToNone             = operandTypes{operandTypeRegisterAndConst, operandTypeNone}
	operandTypesRegisterAndConstToRegister         = operandTypes{operandTypeRegisterAndConst, operandTypeRegister}
	operandTypesRegisterToMemory                   = operandTypes{operandTypeRegister, operandTypeMemory}
	operandTypesMemoryToRegister                   = operandTypes{operandTypeMemory, operandTypeRegister}
	operandTypesConstToRegister                    = operandTypes{operandTypeConst, operandTypeRegister}
	operandTypesRegisterToVectorRegister           = operandTypes{operandTypeRegister, operandTypeVectorRegister}
	operandTypesVectorRegisterToRegister           = operandTypes{operandTypeVectorRegister, operandTypeRegister}
	operandTypesMemoryToVectorRegister             = operandTypes{operandTypeMemory, operandTypeVectorRegister}
	operandTypesVectorRegisterToMemory             = operandTypes{operandTypeVectorRegister, operandTypeMemory}
	operandTypesVectorRegisterToVectorRegister     = operandTypes{operandTypeVectorRegister, operandTypeVectorRegister}
	operandTypesTwoVectorRegistersToVectorRegister = operandTypes{operandTypeTwoVectorRegisters, operandTypeVectorRegister}
	operandTypesStaticConstToVectorRegister        = operandTypes{operandTypeStaticConst, operandTypeVectorRegister}
)

// String implements fmt.Stringer
func (o operandTypes) String() string {
	return fmt.Sprintf("from:%s,to:%s", o.src, o.dst)
}

const (
	maxSignedInt26 int64 = 1<<25 - 1
	minSignedInt26 int64 = -(1 << 25)

	maxSignedInt19 int64 = 1<<19 - 1
	minSignedInt19 int64 = -(1 << 19)
)

// AssemblerImpl implements Assembler.
type AssemblerImpl struct {
	nodePool *nodePool
	asm.BaseAssemblerImpl
	root, current     *nodeImpl
	buf               *bytes.Buffer
	temporaryRegister asm.Register
	nodeCount         int
	pool              *asm.StaticConstPool
	// MaxDisplacementForConstantPool is fixed to defaultMaxDisplacementForConstPool
	// but have it as a field here for testability.
	MaxDisplacementForConstantPool         int
	relativeJumpNodes, adrInstructionNodes []*nodeImpl
}

const nodePoolPageSize = 1000

// nodePool is the central allocation pool for nodeImpl used by a single AssemblerImpl.
// This reduces the allocations over compilation by reusing AssemblerImpl.
type nodePool struct {
	pages [][nodePoolPageSize]nodeImpl
	// page is the index on pages to allocate node on.
	page,
	// pos is the index on pages[.page] where the next allocation target exists.
	pos int
}

// allocNode allocates a new nodeImpl for use from the pool.
// This expands the pool if there is no space left for it.
func (n *nodePool) allocNode() (ret *nodeImpl) {
	if n.pos == nodePoolPageSize {
		if len(n.pages)-1 == n.page {
			n.pages = append(n.pages, [nodePoolPageSize]nodeImpl{})
		}
		n.page++
		n.pos = 0
	}
	ret = &n.pages[n.page][n.pos]
	*ret = nodeImpl{}
	n.pos++
	return
}

func NewAssembler(temporaryRegister asm.Register) *AssemblerImpl {
	return &AssemblerImpl{
		nodePool:                       &nodePool{pages: [][nodePoolPageSize]nodeImpl{{}}},
		buf:                            bytes.NewBuffer(nil),
		temporaryRegister:              temporaryRegister,
		pool:                           asm.NewStaticConstPool(),
		MaxDisplacementForConstantPool: defaultMaxDisplacementForConstPool,
	}
}

// Reset implements asm.AssemblerBase.
func (a *AssemblerImpl) Reset() {
	buf, np, tmp := a.buf, a.nodePool, a.temporaryRegister
	*a = AssemblerImpl{
		buf: buf, nodePool: np, pool: asm.NewStaticConstPool(),
		temporaryRegister:   tmp,
		adrInstructionNodes: a.adrInstructionNodes[:0],
		relativeJumpNodes:   a.relativeJumpNodes[:0],
		BaseAssemblerImpl: asm.BaseAssemblerImpl{
			SetBranchTargetOnNextNodes: a.SetBranchTargetOnNextNodes[:0],
			JumpTableEntries:           a.JumpTableEntries[:0],
		},
	}
	a.nodePool.pos, a.nodePool.page = 0, 0
	a.buf.Reset()
}

// newNode creates a new Node and appends it into the linked list.
func (a *AssemblerImpl) newNode(instruction asm.Instruction, types operandTypes) *nodeImpl {
	n := a.nodePool.allocNode()
	n.instruction = instruction
	n.types = types

	a.addNode(n)
	return n
}

// addNode appends the new node into the linked list.
func (a *AssemblerImpl) addNode(node *nodeImpl) {
	a.nodeCount++

	if a.root == nil {
		a.root = node
		a.current = node
	} else {
		parent := a.current
		parent.next = node
		a.current = node
	}

	for _, o := range a.SetBranchTargetOnNextNodes {
		origin := o.(*nodeImpl)
		origin.jumpTarget = node
	}
	a.SetBranchTargetOnNextNodes = nil
}

// Assemble implements asm.AssemblerBase
func (a *AssemblerImpl) Assemble() ([]byte, error) {
	// arm64 has 32-bit fixed length instructions,
	// but note that some nodes are encoded as multiple instructions,
	// so the resulting binary might not be the size of count*8.
	a.buf.Grow(a.nodeCount * 8)

	for n := a.root; n != nil; n = n.next {
		n.offsetInBinaryField = uint64(a.buf.Len())
		if err := a.encodeNode(n); err != nil {
			return nil, err
		}
		a.maybeFlushConstPool(n.next == nil)
	}

	code := a.bytes()

	if err := a.FinalizeJumpTableEntry(code); err != nil {
		return nil, err
	}

	for _, rel := range a.relativeJumpNodes {
		if err := a.relativeBranchFinalize(code, rel); err != nil {
			return nil, err
		}
	}

	for _, adr := range a.adrInstructionNodes {
		if err := a.finalizeADRInstructionNode(code, adr); err != nil {
			return nil, err
		}
	}
	return code, nil
}

const defaultMaxDisplacementForConstPool = (1 << 20) - 1 - 4 // -4 for unconditional branch to skip the constants.

// maybeFlushConstPool flushes the constant pool if endOfBinary or a boundary condition was met.
func (a *AssemblerImpl) maybeFlushConstPool(endOfBinary bool) {
	if a.pool.FirstUseOffsetInBinary == nil {
		return
	}

	// If endOfBinary = true, we no longer need to emit the instructions, therefore
	// flush all the constants.
	if endOfBinary ||
		// Also, if the offset between the first usage of the constant pool and
		// the first constant would exceed 2^20 -1(= 2MiB-1), which is the maximum offset
		// for LDR(literal)/ADR instruction, flush all the constants in the pool.
		(a.buf.Len()+a.pool.PoolSizeInBytes-int(*a.pool.FirstUseOffsetInBinary)) >= a.MaxDisplacementForConstantPool {

		// Before emitting consts, we have to add br instruction to skip the const pool.
		// https://github.com/golang/go/blob/release-branch.go1.15/src/cmd/internal/obj/arm64/asm7.go#L1123-L1129
		skipOffset := a.pool.PoolSizeInBytes/4 + 1
		if a.pool.PoolSizeInBytes%4 != 0 {
			skipOffset++
		}
		if endOfBinary {
			// If this is the end of binary, we never reach this block,
			// so offset can be zero (which is the behavior of Go's assembler).
			skipOffset = 0
		}

		a.buf.Write([]byte{
			byte(skipOffset),
			byte(skipOffset >> 8),
			byte(skipOffset >> 16),
			0x14,
		})

		// Then adding the consts into the binary.
		for _, c := range a.pool.Consts {
			c.SetOffsetInBinary(uint64(a.buf.Len()))
			a.buf.Write(c.Raw)
		}

		// arm64 instructions are 4-byte (32-bit) aligned, so we must pad the zero consts here.
		if pad := a.buf.Len() % 4; pad != 0 {
			a.buf.Write(make([]byte, 4-pad))
		}

		// After the flush, reset the constant pool.
		a.pool = asm.NewStaticConstPool()
	}
}

// bytes returns the encoded binary.
func (a *AssemblerImpl) bytes() []byte {
	// 16 bytes alignment to match our impl with golang-asm.
	// https://github.com/golang/go/blob/release-branch.go1.15/src/cmd/internal/obj/arm64/asm7.go#L62
	//
	// TODO: Delete after golang-asm removal.
	if pad := 16 - a.buf.Len()%16; pad > 0 && pad != 16 {
		a.buf.Write(make([]byte, pad))
	}
	return a.buf.Bytes()
}

// encodeNode encodes the given node into writer.
func (a *AssemblerImpl) encodeNode(n *nodeImpl) (err error) {
	switch n.types {
	case operandTypesNoneToNone:
		err = a.encodeNoneToNone(n)
	case operandTypesNoneToRegister:
		err = a.encodeJumpToRegister(n)
	case operandTypesNoneToBranch:
		err = a.encodeRelativeBranch(n)
	case operandTypesRegisterToRegister:
		err = a.encodeRegisterToRegister(n)
	case operandTypesLeftShiftedRegisterToRegister:
		err = a.encodeLeftShiftedRegisterToRegister(n)
	case operandTypesTwoRegistersToRegister:
		err = a.encodeTwoRegistersToRegister(n)
	case operandTypesThreeRegistersToRegister:
		err = a.encodeThreeRegistersToRegister(n)
	case operandTypesTwoRegistersToNone:
		err = a.encodeTwoRegistersToNone(n)
	case operandTypesRegisterAndConstToNone:
		err = a.encodeRegisterAndConstToNone(n)
	case operandTypesRegisterToMemory:
		err = a.encodeRegisterToMemory(n)
	case operandTypesMemoryToRegister:
		err = a.encodeMemoryToRegister(n)
	case operandTypesRegisterAndConstToRegister, operandTypesConstToRegister:
		err = a.encodeConstToRegister(n)
	case operandTypesRegisterToVectorRegister:
		err = a.encodeRegisterToVectorRegister(n)
	case operandTypesVectorRegisterToRegister:
		err = a.encodeVectorRegisterToRegister(n)
	case operandTypesMemoryToVectorRegister:
		err = a.encodeMemoryToVectorRegister(n)
	case operandTypesVectorRegisterToMemory:
		err = a.encodeVectorRegisterToMemory(n)
	case operandTypesVectorRegisterToVectorRegister:
		err = a.encodeVectorRegisterToVectorRegister(n)
	case operandTypesStaticConstToVectorRegister:
		err = a.encodeStaticConstToVectorRegister(n)
	case operandTypesTwoVectorRegistersToVectorRegister:
		err = a.encodeTwoVectorRegistersToVectorRegister(n)
	default:
		err = fmt.Errorf("encoder undefined for [%s] operand type", n.types)
	}
	if err != nil {
		err = fmt.Errorf("%w: %s", err, n) // Ensure the error is debuggable by including the string value of the node.
	}
	return
}

// CompileStandAlone implements the same method as documented on asm.AssemblerBase.
func (a *AssemblerImpl) CompileStandAlone(instruction asm.Instruction) asm.Node {
	return a.newNode(instruction, operandTypesNoneToNone)
}

// CompileConstToRegister implements the same method as documented on asm.AssemblerBase.
func (a *AssemblerImpl) CompileConstToRegister(
	instruction asm.Instruction,
	value asm.ConstantValue,
	destinationReg asm.Register,
) (inst asm.Node) {
	n := a.newNode(instruction, operandTypesConstToRegister)
	n.srcConst = value
	n.dstReg = destinationReg
	return n
}

// CompileRegisterToRegister implements the same method as documented on asm.AssemblerBase.
func (a *AssemblerImpl) CompileRegisterToRegister(instruction asm.Instruction, from, to asm.Register) {
	n := a.newNode(instruction, operandTypesRegisterToRegister)
	n.srcReg = from
	n.dstReg = to
}

// CompileMemoryToRegister implements the same method as documented on asm.AssemblerBase.
func (a *AssemblerImpl) CompileMemoryToRegister(
	instruction asm.Instruction,
	sourceBaseReg asm.Register,
	sourceOffsetConst asm.ConstantValue,
	destinationReg asm.Register,
) {
	n := a.newNode(instruction, operandTypesMemoryToRegister)
	n.srcReg = sourceBaseReg
	n.srcConst = sourceOffsetConst
	n.dstReg = destinationReg
}

// CompileRegisterToMemory implements the same method as documented on asm.AssemblerBase.
func (a *AssemblerImpl) CompileRegisterToMemory(
	instruction asm.Instruction,
	sourceRegister, destinationBaseRegister asm.Register,
	destinationOffsetConst asm.ConstantValue,
) {
	n := a.newNode(instruction, operandTypesRegisterToMemory)
	n.srcReg = sourceRegister
	n.dstReg = destinationBaseRegister
	n.dstConst = destinationOffsetConst
}

// CompileJump implements the same method as documented on asm.AssemblerBase.
func (a *AssemblerImpl) CompileJump(jmpInstruction asm.Instruction) asm.Node {
	return a.newNode(jmpInstruction, operandTypesNoneToBranch)
}

// CompileJumpToRegister implements the same method as documented on asm.AssemblerBase.
func (a *AssemblerImpl) CompileJumpToRegister(jmpInstruction asm.Instruction, reg asm.Register) {
	n := a.newNode(jmpInstruction, operandTypesNoneToRegister)
	n.dstReg = reg
}

// CompileReadInstructionAddress implements the same method as documented on asm.AssemblerBase.
func (a *AssemblerImpl) CompileReadInstructionAddress(
	destinationRegister asm.Register,
	beforeAcquisitionTargetInstruction asm.Instruction,
) {
	n := a.newNode(ADR, operandTypesMemoryToRegister)
	n.dstReg = destinationRegister
	n.readInstructionAddressBeforeTargetInstruction = beforeAcquisitionTargetInstruction
}

// CompileMemoryWithRegisterOffsetToRegister implements Assembler.CompileMemoryWithRegisterOffsetToRegister
func (a *AssemblerImpl) CompileMemoryWithRegisterOffsetToRegister(
	instruction asm.Instruction,
	srcBaseReg, srcOffsetReg, dstReg asm.Register,
) {
	n := a.newNode(instruction, operandTypesMemoryToRegister)
	n.dstReg = dstReg
	n.srcReg = srcBaseReg
	n.srcReg2 = srcOffsetReg
}

// CompileRegisterToMemoryWithRegisterOffset implements Assembler.CompileRegisterToMemoryWithRegisterOffset
func (a *AssemblerImpl) CompileRegisterToMemoryWithRegisterOffset(
	instruction asm.Instruction,
	srcReg, dstBaseReg, dstOffsetReg asm.Register,
) {
	n := a.newNode(instruction, operandTypesRegisterToMemory)
	n.srcReg = srcReg
	n.dstReg = dstBaseReg
	n.dstReg2 = dstOffsetReg
}

// CompileTwoRegistersToRegister implements Assembler.CompileTwoRegistersToRegister
func (a *AssemblerImpl) CompileTwoRegistersToRegister(instruction asm.Instruction, src1, src2, dst asm.Register) {
	n := a.newNode(instruction, operandTypesTwoRegistersToRegister)
	n.srcReg = src1
	n.srcReg2 = src2
	n.dstReg = dst
}

// CompileThreeRegistersToRegister implements Assembler.CompileThreeRegistersToRegister
func (a *AssemblerImpl) CompileThreeRegistersToRegister(
	instruction asm.Instruction,
	src1, src2, src3, dst asm.Register,
) {
	n := a.newNode(instruction, operandTypesThreeRegistersToRegister)
	n.srcReg = src1
	n.srcReg2 = src2
	n.dstReg = src3 // To minimize the size of nodeImpl struct, we reuse dstReg for the third source operand.
	n.dstReg2 = dst
}

// CompileTwoRegistersToNone implements Assembler.CompileTwoRegistersToNone
func (a *AssemblerImpl) CompileTwoRegistersToNone(instruction asm.Instruction, src1, src2 asm.Register) {
	n := a.newNode(instruction, operandTypesTwoRegistersToNone)
	n.srcReg = src1
	n.srcReg2 = src2
}

// CompileRegisterAndConstToNone implements Assembler.CompileRegisterAndConstToNone
func (a *AssemblerImpl) CompileRegisterAndConstToNone(
	instruction asm.Instruction,
	src asm.Register,
	srcConst asm.ConstantValue,
) {
	n := a.newNode(instruction, operandTypesRegisterAndConstToNone)
	n.srcReg = src
	n.srcConst = srcConst
}

// CompileRegisterAndConstToRegister implements Assembler.CompileRegisterAndConstToRegister
func (a *AssemblerImpl) CompileRegisterAndConstToRegister(
	instruction asm.Instruction,
	src asm.Register,
	srcConst asm.ConstantValue,
	dst asm.Register,
) {
	n := a.newNode(instruction, operandTypesRegisterAndConstToRegister)
	n.srcReg = src
	n.srcConst = srcConst
	n.dstReg = dst
}

// CompileLeftShiftedRegisterToRegister implements Assembler.CompileLeftShiftedRegisterToRegister
func (a *AssemblerImpl) CompileLeftShiftedRegisterToRegister(
	instruction asm.Instruction,
	shiftedSourceReg asm.Register,
	shiftNum asm.ConstantValue,
	srcReg, dstReg asm.Register,
) {
	n := a.newNode(instruction, operandTypesLeftShiftedRegisterToRegister)
	n.srcReg = srcReg
	n.srcReg2 = shiftedSourceReg
	n.srcConst = shiftNum
	n.dstReg = dstReg
}

// CompileConditionalRegisterSet implements Assembler.CompileConditionalRegisterSet
func (a *AssemblerImpl) CompileConditionalRegisterSet(cond asm.ConditionalRegisterState, dstReg asm.Register) {
	n := a.newNode(CSET, operandTypesRegisterToRegister)
	n.srcReg = conditionalRegisterStateToRegister(cond)
	n.dstReg = dstReg
}

// CompileMemoryToVectorRegister implements Assembler.CompileMemoryToVectorRegister
func (a *AssemblerImpl) CompileMemoryToVectorRegister(
	instruction asm.Instruction, srcBaseReg asm.Register, dstOffset asm.ConstantValue, dstReg asm.Register, arrangement VectorArrangement,
) {
	n := a.newNode(instruction, operandTypesMemoryToVectorRegister)
	n.srcReg = srcBaseReg
	n.srcConst = dstOffset
	n.dstReg = dstReg
	n.vectorArrangement = arrangement
}

// CompileMemoryWithRegisterOffsetToVectorRegister implements Assembler.CompileMemoryWithRegisterOffsetToVectorRegister
func (a *AssemblerImpl) CompileMemoryWithRegisterOffsetToVectorRegister(instruction asm.Instruction,
	srcBaseReg, srcOffsetRegister asm.Register, dstReg asm.Register, arrangement VectorArrangement,
) {
	n := a.newNode(instruction, operandTypesMemoryToVectorRegister)
	n.srcReg = srcBaseReg
	n.srcReg2 = srcOffsetRegister
	n.dstReg = dstReg
	n.vectorArrangement = arrangement
}

// CompileVectorRegisterToMemory implements Assembler.CompileVectorRegisterToMemory
func (a *AssemblerImpl) CompileVectorRegisterToMemory(
	instruction asm.Instruction, srcReg, dstBaseReg asm.Register, dstOffset asm.ConstantValue, arrangement VectorArrangement,
) {
	n := a.newNode(instruction, operandTypesVectorRegisterToMemory)
	n.srcReg = srcReg
	n.dstReg = dstBaseReg
	n.dstConst = dstOffset
	n.vectorArrangement = arrangement
}

// CompileVectorRegisterToMemoryWithRegisterOffset implements Assembler.CompileVectorRegisterToMemoryWithRegisterOffset
func (a *AssemblerImpl) CompileVectorRegisterToMemoryWithRegisterOffset(instruction asm.Instruction,
	srcReg, dstBaseReg, dstOffsetRegister asm.Register, arrangement VectorArrangement,
) {
	n := a.newNode(instruction, operandTypesVectorRegisterToMemory)
	n.srcReg = srcReg
	n.dstReg = dstBaseReg
	n.dstReg2 = dstOffsetRegister
	n.vectorArrangement = arrangement
}

// CompileRegisterToVectorRegister implements Assembler.CompileRegisterToVectorRegister
func (a *AssemblerImpl) CompileRegisterToVectorRegister(
	instruction asm.Instruction, srcReg, dstReg asm.Register, arrangement VectorArrangement, index VectorIndex,
) {
	n := a.newNode(instruction, operandTypesRegisterToVectorRegister)
	n.srcReg = srcReg
	n.dstReg = dstReg
	n.vectorArrangement = arrangement
	n.dstVectorIndex = index
}

// CompileVectorRegisterToRegister implements Assembler.CompileVectorRegisterToRegister
func (a *AssemblerImpl) CompileVectorRegisterToRegister(instruction asm.Instruction, srcReg, dstReg asm.Register,
	arrangement VectorArrangement, index VectorIndex,
) {
	n := a.newNode(instruction, operandTypesVectorRegisterToRegister)
	n.srcReg = srcReg
	n.dstReg = dstReg
	n.vectorArrangement = arrangement
	n.srcVectorIndex = index
}

// CompileVectorRegisterToVectorRegister implements Assembler.CompileVectorRegisterToVectorRegister
func (a *AssemblerImpl) CompileVectorRegisterToVectorRegister(
	instruction asm.Instruction, srcReg, dstReg asm.Register, arrangement VectorArrangement, srcIndex, dstIndex VectorIndex,
) {
	n := a.newNode(instruction, operandTypesVectorRegisterToVectorRegister)
	n.srcReg = srcReg
	n.dstReg = dstReg
	n.vectorArrangement = arrangement
	n.srcVectorIndex = srcIndex
	n.dstVectorIndex = dstIndex
}

// CompileVectorRegisterToVectorRegisterWithConst implements Assembler.CompileVectorRegisterToVectorRegisterWithConst
func (a *AssemblerImpl) CompileVectorRegisterToVectorRegisterWithConst(instruction asm.Instruction,
	srcReg, dstReg asm.Register, arrangement VectorArrangement, c asm.ConstantValue,
) {
	n := a.newNode(instruction, operandTypesVectorRegisterToVectorRegister)
	n.srcReg = srcReg
	n.srcConst = c
	n.dstReg = dstReg
	n.vectorArrangement = arrangement
}

// CompileStaticConstToRegister implements Assembler.CompileStaticConstToVectorRegister
func (a *AssemblerImpl) CompileStaticConstToRegister(instruction asm.Instruction, c *asm.StaticConst, dstReg asm.Register) {
	n := a.newNode(instruction, operandTypesMemoryToRegister)
	n.staticConst = c
	n.dstReg = dstReg
}

// CompileStaticConstToVectorRegister implements Assembler.CompileStaticConstToVectorRegister
func (a *AssemblerImpl) CompileStaticConstToVectorRegister(instruction asm.Instruction,
	c *asm.StaticConst, dstReg asm.Register, arrangement VectorArrangement,
) {
	n := a.newNode(instruction, operandTypesStaticConstToVectorRegister)
	n.staticConst = c
	n.dstReg = dstReg
	n.vectorArrangement = arrangement
}

// CompileTwoVectorRegistersToVectorRegister implements Assembler.CompileTwoVectorRegistersToVectorRegister.
func (a *AssemblerImpl) CompileTwoVectorRegistersToVectorRegister(instruction asm.Instruction, srcReg, srcReg2, dstReg asm.Register,
	arrangement VectorArrangement,
) {
	n := a.newNode(instruction, operandTypesTwoVectorRegistersToVectorRegister)
	n.srcReg = srcReg
	n.srcReg2 = srcReg2
	n.dstReg = dstReg
	n.vectorArrangement = arrangement
}

// CompileTwoVectorRegistersToVectorRegisterWithConst implements Assembler.CompileTwoVectorRegistersToVectorRegisterWithConst.
func (a *AssemblerImpl) CompileTwoVectorRegistersToVectorRegisterWithConst(instruction asm.Instruction,
	srcReg, srcReg2, dstReg asm.Register, arrangement VectorArrangement, c asm.ConstantValue,
) {
	n := a.newNode(instruction, operandTypesTwoVectorRegistersToVectorRegister)
	n.srcReg = srcReg
	n.srcReg2 = srcReg2
	n.srcConst = c
	n.dstReg = dstReg
	n.vectorArrangement = arrangement
}

func errorEncodingUnsupported(n *nodeImpl) error {
	return fmt.Errorf("%s is unsupported for %s type", InstructionName(n.instruction), n.types)
}

func (a *AssemblerImpl) encodeNoneToNone(n *nodeImpl) (err error) {
	switch n.instruction {
	case UDF:
		a.buf.Write([]byte{0, 0, 0, 0})
	case NOP:
	default:
		err = errorEncodingUnsupported(n)
	}
	return
}

func (a *AssemblerImpl) encodeJumpToRegister(n *nodeImpl) (err error) {
	// "Unconditional branch (register)" in https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Branches--Exception-Generating-and-System-instructions
	var opc byte
	switch n.instruction {
	case RET:
		opc = 0b0010
	case B:
		opc = 0b0000
	default:
		return errorEncodingUnsupported(n)
	}

	regBits, err := intRegisterBits(n.dstReg)
	if err != nil {
		return fmt.Errorf("invalid destination register: %w", err)
	}

	a.buf.Write([]byte{
		0x00 | (regBits << 5),
		0x00 | (regBits >> 3),
		0b000_11111 | (opc << 5),
		0b1101011_0 | (opc >> 3),
	})
	return
}

func (a *AssemblerImpl) relativeBranchFinalize(code []byte, n *nodeImpl) error {
	var condBits byte
	const condBitsUnconditional = 0xff // Indicates this is not conditional jump.

	// https://developer.arm.com/documentation/den0024/a/CHDEEABE
	switch n.instruction {
	case B:
		condBits = condBitsUnconditional
	case BCONDEQ:
		condBits = 0b0000
	case BCONDGE:
		condBits = 0b1010
	case BCONDGT:
		condBits = 0b1100
	case BCONDHI:
		condBits = 0b1000
	case BCONDHS:
		condBits = 0b0010
	case BCONDLE:
		condBits = 0b1101
	case BCONDLO:
		condBits = 0b0011
	case BCONDLS:
		condBits = 0b1001
	case BCONDLT:
		condBits = 0b1011
	case BCONDMI:
		condBits = 0b0100
	case BCONDPL:
		condBits = 0b0101
	case BCONDNE:
		condBits = 0b0001
	case BCONDVS:
		condBits = 0b0110
	}

	branchInstOffset := int64(n.OffsetInBinary())
	offset := int64(n.jumpTarget.OffsetInBinary()) - branchInstOffset
	if offset%4 != 0 {
		return errors.New("BUG: relative jump offset must be 4 bytes aligned")
	}

	branchInst := code[branchInstOffset : branchInstOffset+4]
	if condBits == condBitsUnconditional {
		imm26 := offset / 4
		if imm26 < minSignedInt26 || imm26 > maxSignedInt26 {
			// In theory this could happen if a Wasm binary has a huge single label (more than 128MB for a single block),
			// and in that case, we use load the offset into a register and do the register jump, but to avoid the complexity,
			// we impose this limit for now as that would be *unlikely* happen in practice.
			return fmt.Errorf("relative jump offset %d/4 must be within %d and %d", offset, minSignedInt26, maxSignedInt26)
		}
		// https://developer.arm.com/documentation/ddi0596/2021-12/Base-Instructions/B--Branch-?lang=en
		branchInst[0] = byte(imm26)
		branchInst[1] = byte(imm26 >> 8)
		branchInst[2] = byte(imm26 >> 16)
		branchInst[3] = (byte(imm26 >> 24 & 0b000000_11)) | 0b000101_00
	} else {
		imm19 := offset / 4
		if imm19 < minSignedInt19 || imm19 > maxSignedInt19 {
			// This should be a bug in our compiler as the conditional jumps are only used in the small offsets (~a few bytes),
			// and if ever happens, compiler can be fixed.
			return fmt.Errorf("BUG: relative jump offset %d/4(=%d) must be within %d and %d", offset, imm19, minSignedInt19, maxSignedInt19)
		}
		// https://developer.arm.com/documentation/ddi0596/2021-12/Base-Instructions/B-cond--Branch-conditionally-?lang=en
		branchInst[0] = (byte(imm19<<5) & 0b111_0_0000) | condBits
		branchInst[1] = byte(imm19 >> 3)
		branchInst[2] = byte(imm19 >> 11)
		branchInst[3] = 0b01010100
	}
	return nil
}

func (a *AssemblerImpl) encodeRelativeBranch(n *nodeImpl) (err error) {
	switch n.instruction {
	case B, BCONDEQ, BCONDGE, BCONDGT, BCONDHI, BCONDHS, BCONDLE, BCONDLO, BCONDLS, BCONDLT, BCONDMI, BCONDNE, BCONDVS, BCONDPL:
	default:
		return errorEncodingUnsupported(n)
	}

	if n.jumpTarget == nil {
		return fmt.Errorf("branch target must be set for %s", InstructionName(n.instruction))
	}

	// At this point, we don't yet know that target's branch, so emit the placeholder (4 bytes).
	a.buf.Write([]byte{0, 0, 0, 0})
	a.relativeJumpNodes = append(a.relativeJumpNodes, n)
	return
}

func checkRegisterToRegisterType(src, dst asm.Register, requireSrcInt, requireDstInt bool) (err error) {
	isSrcInt, isDstInt := isIntRegister(src), isIntRegister(dst)
	if isSrcInt && !requireSrcInt {
		err = fmt.Errorf("src requires float register but got %s", RegisterName(src))
	} else if !isSrcInt && requireSrcInt {
		err = fmt.Errorf("src requires int register but got %s", RegisterName(src))
	} else if isDstInt && !requireDstInt {
		err = fmt.Errorf("dst requires float register but got %s", RegisterName(dst))
	} else if !isDstInt && requireDstInt {
		err = fmt.Errorf("dst requires int register but got %s", RegisterName(dst))
	}
	return
}

func (a *AssemblerImpl) encodeRegisterToRegister(n *nodeImpl) (err error) {
	switch inst := n.instruction; inst {
	case ADD, ADDW, SUB:
		if err = checkRegisterToRegisterType(n.srcReg, n.dstReg, true, true); err != nil {
			return
		}

		// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Register?lang=en#addsub_shift
		var sfops byte
		switch inst {
		case ADD:
			sfops = 0b100
		case ADDW:
		case SUB:
			sfops = 0b110
		}

		srcRegBits, dstRegBits := registerBits(n.srcReg), registerBits(n.dstReg)
		a.buf.Write([]byte{
			(dstRegBits << 5) | dstRegBits,
			dstRegBits >> 3,
			srcRegBits,
			(sfops << 5) | 0b01011,
		})
	case CLZ, CLZW, RBIT, RBITW:
		if err = checkRegisterToRegisterType(n.srcReg, n.dstReg, true, true); err != nil {
			return
		}

		var sf, opcode byte
		switch inst {
		case CLZ:
			// https://developer.arm.com/documentation/ddi0596/2021-12/Base-Instructions/CLZ--Count-Leading-Zeros-?lang=en
			sf, opcode = 0b1, 0b000_100
		case CLZW:
			// https://developer.arm.com/documentation/ddi0596/2021-12/Base-Instructions/CLZ--Count-Leading-Zeros-?lang=en
			sf, opcode = 0b0, 0b000_100
		case RBIT:
			// https://developer.arm.com/documentation/ddi0596/2021-12/Base-Instructions/RBIT--Reverse-Bits-?lang=en
			sf, opcode = 0b1, 0b000_000
		case RBITW:
			// https://developer.arm.com/documentation/ddi0596/2021-12/Base-Instructions/RBIT--Reverse-Bits-?lang=en
			sf, opcode = 0b0, 0b000_000
		}
		if inst == CLZ {
			sf = 1
		}

		srcRegBits, dstRegBits := registerBits(n.srcReg), registerBits(n.dstReg)
		a.buf.Write([]byte{
			(srcRegBits << 5) | dstRegBits,
			opcode<<2 | (srcRegBits >> 3),
			0b110_00000,
			(sf << 7) | 0b0_1011010,
		})
	case CSET:
		if !isConditionalRegister(n.srcReg) {
			return fmt.Errorf("CSET requires conditional register but got %s", RegisterName(n.srcReg))
		}

		dstRegBits, err := intRegisterBits(n.dstReg)
		if err != nil {
			return err
		}

		// CSET encodes the conditional bits with its least significant bit inverted.
		// https://developer.arm.com/documentation/ddi0596/2021-12/Base-Instructions/CSET--Conditional-Set--an-alias-of-CSINC-?lang=en
		//
		// https://developer.arm.com/documentation/den0024/a/CHDEEABE
		var conditionalBits byte
		switch n.srcReg {
		case RegCondEQ:
			conditionalBits = 0b0001
		case RegCondNE:
			conditionalBits = 0b0000
		case RegCondHS:
			conditionalBits = 0b0011
		case RegCondLO:
			conditionalBits = 0b0010
		case RegCondMI:
			conditionalBits = 0b0101
		case RegCondPL:
			conditionalBits = 0b0100
		case RegCondVS:
			conditionalBits = 0b0111
		case RegCondVC:
			conditionalBits = 0b0110
		case RegCondHI:
			conditionalBits = 0b1001
		case RegCondLS:
			conditionalBits = 0b1000
		case RegCondGE:
			conditionalBits = 0b1011
		case RegCondLT:
			conditionalBits = 0b1010
		case RegCondGT:
			conditionalBits = 0b1101
		case RegCondLE:
			conditionalBits = 0b1100
		case RegCondAL:
			conditionalBits = 0b1111
		case RegCondNV:
			conditionalBits = 0b1110
		}

		// https://developer.arm.com/documentation/ddi0596/2021-12/Base-Instructions/CSET--Conditional-Set--an-alias-of-CSINC-?lang=en
		a.buf.Write([]byte{
			0b111_00000 | dstRegBits,
			(conditionalBits << 4) | 0b0000_0111,
			0b100_11111,
			0b10011010,
		})

	case FABSD, FABSS, FNEGD, FNEGS, FSQRTD, FSQRTS, FCVTSD, FCVTDS, FRINTMD, FRINTMS,
		FRINTND, FRINTNS, FRINTPD, FRINTPS, FRINTZD, FRINTZS:
		if err = checkRegisterToRegisterType(n.srcReg, n.dstReg, false, false); err != nil {
			return
		}

		srcRegBits, dstRegBits := registerBits(n.srcReg), registerBits(n.dstReg)

		// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en#floatdp1
		var tp, opcode byte
		switch inst {
		case FABSD:
			opcode, tp = 0b000001, 0b01
		case FABSS:
			opcode, tp = 0b000001, 0b00
		case FNEGD:
			opcode, tp = 0b000010, 0b01
		case FNEGS:
			opcode, tp = 0b000010, 0b00
		case FSQRTD:
			opcode, tp = 0b000011, 0b01
		case FSQRTS:
			opcode, tp = 0b000011, 0b00
		case FCVTSD:
			opcode, tp = 0b000101, 0b00
		case FCVTDS:
			opcode, tp = 0b000100, 0b01
		case FRINTMD:
			opcode, tp = 0b001010, 0b01
		case FRINTMS:
			opcode, tp = 0b001010, 0b00
		case FRINTND:
			opcode, tp = 0b001000, 0b01
		case FRINTNS:
			opcode, tp = 0b001000, 0b00
		case FRINTPD:
			opcode, tp = 0b001001, 0b01
		case FRINTPS:
			opcode, tp = 0b001001, 0b00
		case FRINTZD:
			opcode, tp = 0b001011, 0b01
		case FRINTZS:
			opcode, tp = 0b001011, 0b00
		}
		a.buf.Write([]byte{
			(srcRegBits << 5) | dstRegBits,
			(opcode << 7) | 0b0_10000_00 | (srcRegBits >> 3),
			tp<<6 | 0b00_1_00000 | opcode>>1,
			0b0_00_11110,
		})

	case FADDD, FADDS, FDIVS, FDIVD, FMAXD, FMAXS, FMIND, FMINS, FMULS, FMULD:
		if err = checkRegisterToRegisterType(n.srcReg, n.dstReg, false, false); err != nil {
			return
		}

		srcRegBits, dstRegBits := registerBits(n.srcReg), registerBits(n.dstReg)

		// "Floating-point data-processing (2 source)" in
		// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en#floatdp1
		var tp, opcode byte
		switch inst {
		case FADDD:
			opcode, tp = 0b0010, 0b01
		case FADDS:
			opcode, tp = 0b0010, 0b00
		case FDIVD:
			opcode, tp = 0b0001, 0b01
		case FDIVS:
			opcode, tp = 0b0001, 0b00
		case FMAXD:
			opcode, tp = 0b0100, 0b01
		case FMAXS:
			opcode, tp = 0b0100, 0b00
		case FMIND:
			opcode, tp = 0b0101, 0b01
		case FMINS:
			opcode, tp = 0b0101, 0b00
		case FMULS:
			opcode, tp = 0b0000, 0b00
		case FMULD:
			opcode, tp = 0b0000, 0b01
		}

		a.buf.Write([]byte{
			(dstRegBits << 5) | dstRegBits,
			opcode<<4 | 0b0000_10_00 | (dstRegBits >> 3),
			tp<<6 | 0b00_1_00000 | srcRegBits,
			0b0001_1110,
		})

	case FCVTZSD, FCVTZSDW, FCVTZSS, FCVTZSSW, FCVTZUD, FCVTZUDW, FCVTZUS, FCVTZUSW:
		if err = checkRegisterToRegisterType(n.srcReg, n.dstReg, false, true); err != nil {
			return
		}

		srcRegBits, dstRegBits := registerBits(n.srcReg), registerBits(n.dstReg)

		// "Conversion between floating-point and integer" in
		// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en#floatdp1
		var sf, tp, opcode byte
		switch inst {
		case FCVTZSD: // Double to signed 64-bit
			sf, tp, opcode = 0b1, 0b01, 0b000
		case FCVTZSDW: // Double to signed 32-bit.
			sf, tp, opcode = 0b0, 0b01, 0b000
		case FCVTZSS: // Single to signed 64-bit.
			sf, tp, opcode = 0b1, 0b00, 0b000
		case FCVTZSSW: // Single to signed 32-bit.
			sf, tp, opcode = 0b0, 0b00, 0b000
		case FCVTZUD: // Double to unsigned 64-bit.
			sf, tp, opcode = 0b1, 0b01, 0b001
		case FCVTZUDW: // Double to unsigned 32-bit.
			sf, tp, opcode = 0b0, 0b01, 0b001
		case FCVTZUS: // Single to unsigned 64-bit.
			sf, tp, opcode = 0b1, 0b00, 0b001
		case FCVTZUSW: // Single to unsigned 32-bit.
			sf, tp, opcode = 0b0, 0b00, 0b001
		}

		a.buf.Write([]byte{
			(srcRegBits << 5) | dstRegBits,
			0 | (srcRegBits >> 3),
			tp<<6 | 0b00_1_11_000 | opcode,
			sf<<7 | 0b0_0_0_11110,
		})

	case FMOVD, FMOVS:
		isSrcInt, isDstInt := isIntRegister(n.srcReg), isIntRegister(n.dstReg)
		if isSrcInt && isDstInt {
			return errors.New("FMOV needs at least one of operands to be integer")
		}

		srcRegBits, dstRegBits := registerBits(n.srcReg), registerBits(n.dstReg)
		// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FMOV--register---Floating-point-Move-register-without-conversion-?lang=en
		if !isSrcInt && !isDstInt { // Float to float.
			var tp byte
			if inst == FMOVD {
				tp = 0b01
			}
			a.buf.Write([]byte{
				(srcRegBits << 5) | dstRegBits,
				0b0_10000_00 | (srcRegBits >> 3),
				tp<<6 | 0b00_1_00000,
				0b000_11110,
			})
		} else if isSrcInt && !isDstInt { // Int to float.
			var tp, sf byte
			if inst == FMOVD {
				tp, sf = 0b01, 0b1
			}
			a.buf.Write([]byte{
				(srcRegBits << 5) | dstRegBits,
				srcRegBits >> 3,
				tp<<6 | 0b00_1_00_111,
				sf<<7 | 0b0_00_11110,
			})
		} else { // Float to int.
			var tp, sf byte
			if inst == FMOVD {
				tp, sf = 0b01, 0b1
			}
			a.buf.Write([]byte{
				(srcRegBits << 5) | dstRegBits,
				srcRegBits >> 3,
				tp<<6 | 0b00_1_00_110,
				sf<<7 | 0b0_00_11110,
			})
		}

	case MOVD, MOVW:
		if err = checkRegisterToRegisterType(n.srcReg, n.dstReg, true, true); err != nil {
			return
		}
		srcRegBits, dstRegBits := registerBits(n.srcReg), registerBits(n.dstReg)

		if n.srcReg == RegSP || n.dstReg == RegSP {
			// Moving between stack pointers.
			// https://developer.arm.com/documentation/ddi0602/2021-12/Base-Instructions/MOV--to-from-SP---Move-between-register-and-stack-pointer--an-alias-of-ADD--immediate--
			a.buf.Write([]byte{
				(srcRegBits << 5) | dstRegBits,
				srcRegBits >> 3,
				0x0,
				0b1001_0001,
			})
			return
		}

		if n.srcReg == RegRZR && inst == MOVD {
			// If this is 64-bit mov from zero register, then we encode this as MOVK.
			// See "Move wide (immediate)" in
			// https://developer.arm.com/documentation/ddi0602/2021-06/Index-by-Encoding/Data-Processing----Immediate
			a.buf.Write([]byte{
				dstRegBits,
				0x0,
				0b1000_0000,
				0b1_10_10010,
			})
		} else {
			// MOV can be encoded as ORR (shifted register): "ORR Wd, WZR, Wm".
			// https://developer.arm.com/documentation/100069/0609/A64-General-Instructions/MOV--register-
			var sf byte
			if inst == MOVD {
				sf = 0b1
			}
			a.buf.Write([]byte{
				(zeroRegisterBits << 5) | dstRegBits,
				zeroRegisterBits >> 3,
				0b000_00000 | srcRegBits,
				sf<<7 | 0b0_01_01010,
			})
		}

	case MRS:
		if n.srcReg != RegFPSR {
			return fmt.Errorf("MRS has only support for FPSR register as a src but got %s", RegisterName(n.srcReg))
		}

		// For how to specify FPSR register, see "Accessing FPSR" in:
		// https://developer.arm.com/documentation/ddi0595/2021-12/AArch64-Registers/FPSR--Floating-point-Status-Register?lang=en
		dstRegBits := registerBits(n.dstReg)
		a.buf.Write([]byte{
			0b001<<5 | dstRegBits,
			0b0100<<4 | 0b0100,
			0b0011_0000 | 0b11<<3 | 0b011,
			0b1101_0101,
		})

	case MSR:
		if n.dstReg != RegFPSR {
			return fmt.Errorf("MSR has only support for FPSR register as a dst but got %s", RegisterName(n.srcReg))
		}

		// For how to specify FPSR register, see "Accessing FPSR" in:
		// https://developer.arm.com/documentation/ddi0595/2021-12/AArch64-Registers/FPSR--Floating-point-Status-Register?lang=en
		srcRegBits := registerBits(n.srcReg)
		a.buf.Write([]byte{
			0b001<<5 | srcRegBits,
			0b0100<<4 | 0b0100,
			0b0001_0000 | 0b11<<3 | 0b011,
			0b1101_0101,
		})

	case MUL, MULW:
		// Multiplications are encoded as MADD (zero register, src, dst), dst = zero + (src * dst) = src * dst.
		// See "Data-processing (3 source)" in
		// https://developer.arm.com/documentation/ddi0602/2021-06/Index-by-Encoding/Data-Processing----Register?lang=en
		if err = checkRegisterToRegisterType(n.srcReg, n.dstReg, true, true); err != nil {
			return
		}

		var sf byte
		if inst == MUL {
			sf = 0b1
		}

		srcRegBits, dstRegBits := registerBits(n.srcReg), registerBits(n.dstReg)

		a.buf.Write([]byte{
			dstRegBits<<5 | dstRegBits,
			zeroRegisterBits<<2 | dstRegBits>>3,
			srcRegBits,
			sf<<7 | 0b11011,
		})

	case NEG, NEGW:
		srcRegBits, dstRegBits := registerBits(n.srcReg), registerBits(n.dstReg)

		if err = checkRegisterToRegisterType(n.srcReg, n.dstReg, true, true); err != nil {
			return
		}

		// NEG is encoded as "SUB dst, XZR, src" = "dst = 0 - src"
		// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Register?lang=en#addsub_shift
		var sf byte
		if inst == NEG {
			sf = 0b1
		}

		a.buf.Write([]byte{
			(zeroRegisterBits << 5) | dstRegBits,
			zeroRegisterBits >> 3,
			srcRegBits,
			sf<<7 | 0b0_10_00000 | 0b0_00_01011,
		})

	case SDIV, SDIVW, UDIV, UDIVW:
		srcRegBits, dstRegBits := registerBits(n.srcReg), registerBits(n.dstReg)

		if err = checkRegisterToRegisterType(n.srcReg, n.dstReg, true, true); err != nil {
			return
		}

		// See "Data-processing (2 source)" in
		// https://developer.arm.com/documentation/ddi0602/2021-06/Index-by-Encoding/Data-Processing----Register?lang=en
		var sf, opcode byte
		switch inst {
		case SDIV:
			sf, opcode = 0b1, 0b000011
		case SDIVW:
			sf, opcode = 0b0, 0b000011
		case UDIV:
			sf, opcode = 0b1, 0b000010
		case UDIVW:
			sf, opcode = 0b0, 0b000010
		}

		a.buf.Write([]byte{
			(dstRegBits << 5) | dstRegBits,
			opcode<<2 | (dstRegBits >> 3),
			0b110_00000 | srcRegBits,
			sf<<7 | 0b0_00_11010,
		})

	case SCVTFD, SCVTFWD, SCVTFS, SCVTFWS, UCVTFD, UCVTFS, UCVTFWD, UCVTFWS:
		srcRegBits, dstRegBits := registerBits(n.srcReg), registerBits(n.dstReg)

		if err = checkRegisterToRegisterType(n.srcReg, n.dstReg, true, false); err != nil {
			return
		}

		// "Conversion between floating-point and integer" in
		// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en#floatdp1
		var sf, tp, opcode byte
		switch inst {
		case SCVTFD: // 64-bit integer to double
			sf, tp, opcode = 0b1, 0b01, 0b010
		case SCVTFWD: // 32-bit integer to double
			sf, tp, opcode = 0b0, 0b01, 0b010
		case SCVTFS: // 64-bit integer to single
			sf, tp, opcode = 0b1, 0b00, 0b010
		case SCVTFWS: // 32-bit integer to single
			sf, tp, opcode = 0b0, 0b00, 0b010
		case UCVTFD: // 64-bit to double
			sf, tp, opcode = 0b1, 0b01, 0b011
		case UCVTFWD: // 32-bit to double
			sf, tp, opcode = 0b0, 0b01, 0b011
		case UCVTFS: // 64-bit to single
			sf, tp, opcode = 0b1, 0b00, 0b011
		case UCVTFWS: // 32-bit to single
			sf, tp, opcode = 0b0, 0b00, 0b011
		}

		a.buf.Write([]byte{
			(srcRegBits << 5) | dstRegBits,
			srcRegBits >> 3,
			tp<<6 | 0b00_1_00_000 | opcode,
			sf<<7 | 0b0_0_0_11110,
		})

	case SXTB, SXTBW, SXTH, SXTHW, SXTW:
		if err = checkRegisterToRegisterType(n.srcReg, n.dstReg, true, true); err != nil {
			return
		}

		srcRegBits, dstRegBits := registerBits(n.srcReg), registerBits(n.dstReg)
		if n.srcReg == RegRZR {
			// If the source is zero register, we encode as MOV dst, zero.
			var sf byte
			if inst == MOVD {
				sf = 0b1
			}
			a.buf.Write([]byte{
				(zeroRegisterBits << 5) | dstRegBits,
				zeroRegisterBits >> 3,
				0b000_00000 | srcRegBits,
				sf<<7 | 0b0_01_01010,
			})
			return
		}

		// SXTB is encoded as "SBFM Wd, Wn, #0, #7"
		// https://developer.arm.com/documentation/dui0801/g/A64-General-Instructions/SXTB
		// SXTH is encoded as "SBFM Wd, Wn, #0, #15"
		// https://developer.arm.com/documentation/dui0801/g/A64-General-Instructions/SXTH
		// SXTW is encoded as "SBFM Xd, Xn, #0, #31"
		// https://developer.arm.com/documentation/dui0802/b/A64-General-Instructions/SXTW

		var n, sf, imms, opc byte
		switch inst {
		case SXTB:
			n, sf, imms = 0b1, 0b1, 0x7
		case SXTBW:
			n, sf, imms = 0b0, 0b0, 0x7
		case SXTH:
			n, sf, imms = 0b1, 0b1, 0xf
		case SXTHW:
			n, sf, imms = 0b0, 0b0, 0xf
		case SXTW:
			n, sf, imms = 0b1, 0b1, 0x1f
		}

		a.buf.Write([]byte{
			(srcRegBits << 5) | dstRegBits,
			imms<<2 | (srcRegBits >> 3),
			n << 6,
			sf<<7 | opc<<5 | 0b10011,
		})
	default:
		return errorEncodingUnsupported(n)
	}
	return
}

func (a *AssemblerImpl) encodeLeftShiftedRegisterToRegister(n *nodeImpl) (err error) {
	baseRegBits, err := intRegisterBits(n.srcReg)
	if err != nil {
		return err
	}
	shiftTargetRegBits, err := intRegisterBits(n.srcReg2)
	if err != nil {
		return err
	}
	dstRegBits, err := intRegisterBits(n.dstReg)
	if err != nil {
		return err
	}

	switch n.instruction {
	case ADD:
		// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Register?lang=en#addsub_shift
		const logicalLeftShiftBits = 0b00
		if n.srcConst < 0 || n.srcConst > 64 {
			return fmt.Errorf("shift amount must fit in unsigned 6-bit integer (0-64) but got %d", n.srcConst)
		}
		shiftByte := byte(n.srcConst)
		a.buf.Write([]byte{
			(baseRegBits << 5) | dstRegBits,
			(shiftByte << 2) | (baseRegBits >> 3),
			(logicalLeftShiftBits << 6) | shiftTargetRegBits,
			0b1000_1011,
		})
	default:
		return errorEncodingUnsupported(n)
	}
	return
}

func (a *AssemblerImpl) encodeTwoRegistersToRegister(n *nodeImpl) (err error) {
	switch inst := n.instruction; inst {
	case AND, ANDW, ORR, ORRW, EOR, EORW:
		// See "Logical (shifted register)" in
		// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Register?lang=en
		srcRegBits, srcReg2Bits, dstRegBits := registerBits(n.srcReg), registerBits(n.srcReg2), registerBits(n.dstReg)
		var sf, opc byte
		switch inst {
		case AND:
			sf, opc = 0b1, 0b00
		case ANDW:
			sf, opc = 0b0, 0b00
		case ORR:
			sf, opc = 0b1, 0b01
		case ORRW:
			sf, opc = 0b0, 0b01
		case EOR:
			sf, opc = 0b1, 0b10
		case EORW:
			sf, opc = 0b0, 0b10
		}
		a.buf.Write([]byte{
			(srcReg2Bits << 5) | dstRegBits,
			srcReg2Bits >> 3,
			srcRegBits,
			sf<<7 | opc<<5 | 0b01010,
		})
	case ASR, ASRW, LSL, LSLW, LSR, LSRW, ROR, RORW:
		// See "Data-processing (2 source)" in
		// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Register?lang=en
		srcRegBits, srcReg2Bits, dstRegBits := registerBits(n.srcReg), registerBits(n.srcReg2), registerBits(n.dstReg)

		var sf, opcode byte
		switch inst {
		case ASR:
			sf, opcode = 0b1, 0b001010
		case ASRW:
			sf, opcode = 0b0, 0b001010
		case LSL:
			sf, opcode = 0b1, 0b001000
		case LSLW:
			sf, opcode = 0b0, 0b001000
		case LSR:
			sf, opcode = 0b1, 0b001001
		case LSRW:
			sf, opcode = 0b0, 0b001001
		case ROR:
			sf, opcode = 0b1, 0b001011
		case RORW:
			sf, opcode = 0b0, 0b001011
		}
		a.buf.Write([]byte{
			(srcReg2Bits << 5) | dstRegBits,
			opcode<<2 | (srcReg2Bits >> 3),
			0b110_00000 | srcRegBits,
			sf<<7 | 0b0_00_11010,
		})
	case SDIV, SDIVW, UDIV, UDIVW:
		srcRegBits, srcReg2Bits, dstRegBits := registerBits(n.srcReg), registerBits(n.srcReg2), registerBits(n.dstReg)

		// See "Data-processing (2 source)" in
		// https://developer.arm.com/documentation/ddi0602/2021-06/Index-by-Encoding/Data-Processing----Register?lang=en
		var sf, opcode byte
		switch inst {
		case SDIV:
			sf, opcode = 0b1, 0b000011
		case SDIVW:
			sf, opcode = 0b0, 0b000011
		case UDIV:
			sf, opcode = 0b1, 0b000010
		case UDIVW:
			sf, opcode = 0b0, 0b000010
		}

		a.buf.Write([]byte{
			(srcReg2Bits << 5) | dstRegBits,
			opcode<<2 | (srcReg2Bits >> 3),
			0b110_00000 | srcRegBits,
			sf<<7 | 0b0_00_11010,
		})
	case SUB, SUBW:
		srcRegBits, srcReg2Bits, dstRegBits := registerBits(n.srcReg), registerBits(n.srcReg2), registerBits(n.dstReg)

		// See "Add/subtract (shifted register)" in
		// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Register?lang=en
		var sf byte
		if inst == SUB {
			sf = 0b1
		}

		a.buf.Write([]byte{
			(srcReg2Bits << 5) | dstRegBits,
			srcReg2Bits >> 3,
			srcRegBits,
			sf<<7 | 0b0_10_01011,
		})
	case FSUBD, FSUBS:
		srcRegBits, srcReg2Bits, dstRegBits := registerBits(n.srcReg), registerBits(n.srcReg2), registerBits(n.dstReg)

		// See "Floating-point data-processing (2 source)" in
		// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en
		var tp byte
		if inst == FSUBD {
			tp = 0b01
		}
		a.buf.Write([]byte{
			(srcReg2Bits << 5) | dstRegBits,
			0b0011_10_00 | (srcReg2Bits >> 3),
			tp<<6 | 0b00_1_00000 | srcRegBits,
			0b0_00_11110,
		})
	default:
		return errorEncodingUnsupported(n)
	}
	return
}

func (a *AssemblerImpl) encodeThreeRegistersToRegister(n *nodeImpl) (err error) {
	switch n.instruction {
	case MSUB, MSUBW:
		// Dst = Src2 - (Src1 * Src3)
		// "Data-processing (3 source)" in:
		// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Register?lang=en
		src1RegBits, err := intRegisterBits(n.srcReg)
		if err != nil {
			return err
		}
		src2RegBits, err := intRegisterBits(n.srcReg2)
		if err != nil {
			return err
		}
		src3RegBits, err := intRegisterBits(n.dstReg)
		if err != nil {
			return err
		}
		dstRegBits, err := intRegisterBits(n.dstReg2)
		if err != nil {
			return err
		}

		var sf byte // is zero for MSUBW (32-bit MSUB).
		if n.instruction == MSUB {
			sf = 0b1
		}

		a.buf.Write([]byte{
			(src3RegBits << 5) | dstRegBits,
			0b1_0000000 | (src2RegBits << 2) | (src3RegBits >> 3),
			src1RegBits,
			sf<<7 | 0b00_11011,
		})
	default:
		return errorEncodingUnsupported(n)
	}
	return
}

func (a *AssemblerImpl) encodeTwoRegistersToNone(n *nodeImpl) (err error) {
	switch n.instruction {
	case CMPW, CMP:
		// Compare on two registers is an alias for "SUBS (src1, src2) ZERO"
		// which can be encoded as SUBS (shifted registers) with zero shifting.
		// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Register?lang=en#addsub_shift
		src1RegBits, err := intRegisterBits(n.srcReg)
		if err != nil {
			return err
		}
		src2RegBits, err := intRegisterBits(n.srcReg2)
		if err != nil {
			return err
		}

		var op byte
		if n.instruction == CMP {
			op = 0b111
		} else {
			op = 0b011
		}

		a.buf.Write([]byte{
			(src2RegBits << 5) | zeroRegisterBits,
			src2RegBits >> 3,
			src1RegBits,
			0b01011 | (op << 5),
		})
	case FCMPS, FCMPD:
		// "Floating-point compare" section in:
		// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en
		src1RegBits, err := vectorRegisterBits(n.srcReg)
		if err != nil {
			return err
		}
		src2RegBits, err := vectorRegisterBits(n.srcReg2)
		if err != nil {
			return err
		}

		var ftype byte // is zero for FCMPS (single precision float compare).
		if n.instruction == FCMPD {
			ftype = 0b01
		}
		a.buf.Write([]byte{
			src2RegBits << 5,
			0b001000_00 | (src2RegBits >> 3),
			ftype<<6 | 0b1_00000 | src1RegBits,
			0b000_11110,
		})
	default:
		return errorEncodingUnsupported(n)
	}
	return
}

func (a *AssemblerImpl) encodeRegisterAndConstToNone(n *nodeImpl) (err error) {
	if n.instruction != CMP {
		return errorEncodingUnsupported(n)
	}

	// https://developer.arm.com/documentation/ddi0596/2021-12/Base-Instructions/CMP--immediate---Compare--immediate---an-alias-of-SUBS--immediate--?lang=en
	if n.srcConst < 0 || n.srcConst > 4095 {
		return fmt.Errorf("immediate for CMP must fit in 0 to 4095 but got %d", n.srcConst)
	} else if n.srcReg == RegRZR {
		return errors.New("zero register is not supported for CMP (immediate)")
	}

	srcRegBits, err := intRegisterBits(n.srcReg)
	if err != nil {
		return err
	}

	a.buf.Write([]byte{
		(srcRegBits << 5) | zeroRegisterBits,
		(byte(n.srcConst) << 2) | (srcRegBits >> 3),
		byte(n.srcConst >> 6),
		0b111_10001,
	})
	return
}

func fitInSigned9Bits(v int64) bool {
	return v >= -256 && v <= 255
}

func (a *AssemblerImpl) encodeLoadOrStoreWithRegisterOffset(
	baseRegBits, offsetRegBits, targetRegBits byte, opcode, size, v byte,
) {
	// See "Load/store register (register offset)".
	// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Loads-and-Stores?lang=en#ldst_regoff
	a.buf.Write([]byte{
		(baseRegBits << 5) | targetRegBits,
		0b011_010_00 | (baseRegBits >> 3),
		opcode<<6 | 0b00_1_00000 | offsetRegBits,
		size<<6 | v<<2 | 0b00_111_0_00,
	})
}

// validateMemoryOffset validates the memory offset if the given offset can be encoded in the assembler.
// In theory, offset can be any, but for simplicity of our homemade assembler, we limit the offset range
// that can be encoded enough for supporting compiler.
func validateMemoryOffset(offset int64) (err error) {
	if offset > 255 && offset%4 != 0 {
		// This is because we only have large offsets for load/store with Wasm value stack or reading type IDs, and its offset
		// is always multiplied by 4 or 8 (== the size of uint32 or uint64 == the type of wasm.FunctionTypeID or value stack in Go)
		err = fmt.Errorf("large memory offset (>255) must be a multiple of 4 but got %d", offset)
	} else if offset < -256 { // 9-bit signed integer's minimum = 2^8.
		err = fmt.Errorf("negative memory offset must be larget than or equal -256 but got %d", offset)
	} else if offset > 1<<31-1 {
		return fmt.Errorf("large memory offset must be less than %d but got %d", 1<<31-1, offset)
	}
	return
}

// encodeLoadOrStoreWithConstOffset encodes load/store instructions with the constant offset.
//
// Note: Encoding strategy intentionally matches the Go assembler: https://go.dev/doc/asm
func (a *AssemblerImpl) encodeLoadOrStoreWithConstOffset(
	baseRegBits, targetRegBits byte,
	offset int64,
	opcode, size, v byte,
	datasize, datasizeLog2 int64,
) (err error) {
	if err = validateMemoryOffset(offset); err != nil {
		return
	}

	if fitInSigned9Bits(offset) {
		// See "LDAPR/STLR (unscaled immediate)"
		// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Loads-and-Stores?lang=en#ldapstl_unscaled
		if offset < 0 || offset%datasize != 0 {
			// This case is encoded as one "unscaled signed store".
			a.buf.Write([]byte{
				(baseRegBits << 5) | targetRegBits,
				byte(offset<<4) | (baseRegBits >> 3),
				opcode<<6 | (0b00_00_11111 & byte(offset>>4)),
				size<<6 | v<<2 | 0b00_1_11_0_00,
			})
			return
		}
	}

	// At this point we have the assumption that offset is positive.
	// Plus if it is a multiple of datasize, then it can be encoded as a single "unsigned immediate".
	if offset%datasize == 0 &&
		offset < (1<<12)<<datasizeLog2 {
		m := offset / datasize
		a.buf.Write([]byte{
			(baseRegBits << 5) | targetRegBits,
			(byte(m << 2)) | (baseRegBits >> 3),
			opcode<<6 | 0b00_111111&byte(m>>6),
			size<<6 | v<<2 | 0b00_1_11_0_01,
		})
		return
	}

	// Otherwise, we need multiple instructions.
	tmpRegBits := registerBits(a.temporaryRegister)
	offset32 := int32(offset)

	// Go's assembler adds a const into the const pool at this point,
	// regardless of its usage; e.g. if we enter the then block of the following if statement,
	// the const is not used but it is added into the const pool.
	c := asm.NewStaticConst(make([]byte, 4))
	binary.LittleEndian.PutUint32(c.Raw, uint32(offset))
	a.pool.AddConst(c, uint64(a.buf.Len()))

	// https://github.com/golang/go/blob/release-branch.go1.15/src/cmd/internal/obj/arm64/asm7.go#L3529-L3532
	// If the offset is within 24-bits, we can load it with two ADD instructions.
	hi := offset32 - (offset32 & (0xfff << uint(datasizeLog2)))
	if hi&^0xfff000 == 0 {
		var sfops byte = 0b100
		m := ((offset32 - hi) >> datasizeLog2) & 0xfff
		hi >>= 12

		// https://github.com/golang/go/blob/release-branch.go1.15/src/cmd/internal/obj/arm64/asm7.go#L3534-L3535
		a.buf.Write([]byte{
			(baseRegBits << 5) | tmpRegBits,
			(byte(hi) << 2) | (baseRegBits >> 3),
			0b01<<6 /* shift by 12 */ | byte(hi>>6),
			sfops<<5 | 0b10001,
		})

		a.buf.Write([]byte{
			(tmpRegBits << 5) | targetRegBits,
			(byte(m << 2)) | (tmpRegBits >> 3),
			opcode<<6 | 0b00_111111&byte(m>>6),
			size<<6 | v<<2 | 0b00_1_11_0_01,
		})
	} else {
		// This case we load the const via ldr(literal) into tem register,
		// and the target const is placed after this instruction below.
		loadLiteralOffsetInBinary := uint64(a.buf.Len())

		// First we emit the ldr(literal) with offset zero as we don't yet know the const's placement in the binary.
		// https://developer.arm.com/documentation/ddi0596/2020-12/Base-Instructions/LDR--literal---Load-Register--literal--
		a.buf.Write([]byte{tmpRegBits, 0x0, 0x0, 0b00_011_0_00})

		// Set the callback for the constant, and we set properly the offset in the callback.

		c.AddOffsetFinalizedCallback(func(offsetOfConst uint64) {
			// ldr(literal) encodes offset divided by 4.
			offset := (int(offsetOfConst) - int(loadLiteralOffsetInBinary)) / 4
			bin := a.buf.Bytes()
			bin[loadLiteralOffsetInBinary] |= byte(offset << 5)
			bin[loadLiteralOffsetInBinary+1] |= byte(offset >> 3)
			bin[loadLiteralOffsetInBinary+2] |= byte(offset >> 11)
		})

		// Then, load the constant with the register offset.
		// https://developer.arm.com/documentation/ddi0596/2020-12/Base-Instructions/LDR--register---Load-Register--register--
		a.buf.Write([]byte{
			(baseRegBits << 5) | targetRegBits,
			0b011_010_00 | (baseRegBits >> 3),
			opcode<<6 | 0b00_1_00000 | tmpRegBits,
			size<<6 | v<<2 | 0b00_111_0_00,
		})
	}
	return
}

// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Loads-and-Stores?lang=en#ldst_regoff
var storeInstructionTable = map[asm.Instruction]struct {
	size, v                byte
	datasize, datasizeLog2 int64
	isTargetFloat          bool
}{
	STRD:  {size: 0b11, v: 0x0, datasize: 8, datasizeLog2: 3},
	STRW:  {size: 0b10, v: 0x0, datasize: 4, datasizeLog2: 2},
	STRH:  {size: 0b01, v: 0x0, datasize: 2, datasizeLog2: 1},
	STRB:  {size: 0b00, v: 0x0, datasize: 1, datasizeLog2: 0},
	FSTRD: {size: 0b11, v: 0x1, datasize: 8, datasizeLog2: 3, isTargetFloat: true},
	FSTRS: {size: 0b10, v: 0x1, datasize: 4, datasizeLog2: 2, isTargetFloat: true},
}

func (a *AssemblerImpl) encodeRegisterToMemory(n *nodeImpl) (err error) {
	inst, ok := storeInstructionTable[n.instruction]
	if !ok {
		return errorEncodingUnsupported(n)
	}

	var srcRegBits byte
	if inst.isTargetFloat {
		srcRegBits, err = vectorRegisterBits(n.srcReg)
	} else {
		srcRegBits, err = intRegisterBits(n.srcReg)
	}
	if err != nil {
		return
	}

	baseRegBits, err := intRegisterBits(n.dstReg)
	if err != nil {
		return err
	}

	const opcode = 0x00 // opcode for store instructions.
	if n.dstReg2 != asm.NilRegister {
		offsetRegBits, err := intRegisterBits(n.dstReg2)
		if err != nil {
			return err
		}
		a.encodeLoadOrStoreWithRegisterOffset(baseRegBits, offsetRegBits, srcRegBits, opcode, inst.size, inst.v)
	} else {
		err = a.encodeLoadOrStoreWithConstOffset(baseRegBits, srcRegBits, n.dstConst, opcode, inst.size, inst.v, inst.datasize, inst.datasizeLog2)
	}
	return
}

func (a *AssemblerImpl) encodeADR(n *nodeImpl) (err error) {
	dstRegBits, err := intRegisterBits(n.dstReg)
	if err != nil {
		return err
	}

	adrInstructionOffsetInBinary := uint64(a.buf.Len())

	// At this point, we don't yet know the target offset to read from,
	// so we emit the ADR instruction with 0 offset, and replace later in the callback.
	// https://developer.arm.com/documentation/ddi0596/2021-12/Base-Instructions/ADR--Form-PC-relative-address-?lang=en
	a.buf.Write([]byte{dstRegBits, 0x0, 0x0, 0b10000})

	// This case, the ADR's target offset is for the staticConst's initial address.
	if sc := n.staticConst; sc != nil {
		a.pool.AddConst(sc, adrInstructionOffsetInBinary)
		sc.AddOffsetFinalizedCallback(func(offsetOfConst uint64) {
			adrInstructionBytes := a.buf.Bytes()[adrInstructionOffsetInBinary : adrInstructionOffsetInBinary+4]
			offset := int(offsetOfConst) - int(adrInstructionOffsetInBinary)

			// See https://developer.arm.com/documentation/ddi0596/2021-12/Base-Instructions/ADR--Form-PC-relative-address-?lang=en
			adrInstructionBytes[3] |= byte(offset & 0b00000011 << 5)
			offset >>= 2
			adrInstructionBytes[0] |= byte(offset << 5)
			offset >>= 3
			adrInstructionBytes[1] |= byte(offset)
			offset >>= 8
			adrInstructionBytes[2] |= byte(offset)
		})
		return
	} else {
		a.adrInstructionNodes = append(a.adrInstructionNodes, n)
	}
	return
}

func (a *AssemblerImpl) finalizeADRInstructionNode(code []byte, n *nodeImpl) (err error) {
	// Find the target instruction node.
	targetNode := n
	for ; targetNode != nil; targetNode = targetNode.next {
		if targetNode.instruction == n.readInstructionAddressBeforeTargetInstruction {
			targetNode = targetNode.next
			break
		}
	}

	if targetNode == nil {
		return fmt.Errorf("BUG: target instruction %s not found for ADR", InstructionName(n.readInstructionAddressBeforeTargetInstruction))
	}

	offset := targetNode.OffsetInBinary() - n.OffsetInBinary()
	if i64 := int64(offset); i64 >= 1<<20 || i64 < -1<<20 {
		// We could support offset over 20-bit range by special casing them here,
		// but 20-bit range should be enough for our impl. If the necessity comes up,
		// we could add the special casing here to support arbitrary large offset.
		return fmt.Errorf("BUG: too large offset for ADR: %#x", offset)
	}

	adrInstructionBytes := code[n.OffsetInBinary() : n.OffsetInBinary()+4]
	// According to the binary format of ADR instruction:
	// https://developer.arm.com/documentation/ddi0596/2021-12/Base-Instructions/ADR--Form-PC-relative-address-?lang=en
	adrInstructionBytes[3] |= byte(offset & 0b00000011 << 5)
	offset >>= 2
	adrInstructionBytes[0] |= byte(offset << 5)
	offset >>= 3
	adrInstructionBytes[1] |= byte(offset)
	offset >>= 8
	adrInstructionBytes[2] |= byte(offset)
	return nil
}

// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Loads-and-Stores?lang=en#ldst_regoff
var loadInstructionTable = map[asm.Instruction]struct {
	size, v, opcode        byte
	datasize, datasizeLog2 int64
	isTargetFloat          bool
}{
	FLDRD:  {size: 0b11, v: 0x1, datasize: 8, datasizeLog2: 3, isTargetFloat: true, opcode: 0b01},
	FLDRS:  {size: 0b10, v: 0x1, datasize: 4, datasizeLog2: 2, isTargetFloat: true, opcode: 0b01},
	LDRD:   {size: 0b11, v: 0x0, datasize: 8, datasizeLog2: 3, opcode: 0b01},
	LDRW:   {size: 0b10, v: 0x0, datasize: 4, datasizeLog2: 2, opcode: 0b01},
	LDRSHD: {size: 0b01, v: 0x0, datasize: 2, datasizeLog2: 1, opcode: 0b10},
	LDRSHW: {size: 0b01, v: 0x0, datasize: 2, datasizeLog2: 1, opcode: 0b11},
	LDRH:   {size: 0b01, v: 0x0, datasize: 2, datasizeLog2: 1, opcode: 0b01},
	LDRSBD: {size: 0b00, v: 0x0, datasize: 1, datasizeLog2: 0, opcode: 0b10},
	LDRSBW: {size: 0b00, v: 0x0, datasize: 1, datasizeLog2: 0, opcode: 0b11},
	LDRB:   {size: 0b00, v: 0x0, datasize: 1, datasizeLog2: 0, opcode: 0b01},
	LDRSW:  {size: 0b10, v: 0x0, datasize: 4, datasizeLog2: 2, opcode: 0b10},
}

func (a *AssemblerImpl) encodeMemoryToRegister(n *nodeImpl) (err error) {
	if n.instruction == ADR {
		return a.encodeADR(n)
	}

	inst, ok := loadInstructionTable[n.instruction]
	if !ok {
		return errorEncodingUnsupported(n)
	}

	var dstRegBits byte
	if inst.isTargetFloat {
		dstRegBits, err = vectorRegisterBits(n.dstReg)
	} else {
		dstRegBits, err = intRegisterBits(n.dstReg)
	}
	if err != nil {
		return
	}
	baseRegBits, err := intRegisterBits(n.srcReg)
	if err != nil {
		return err
	}

	if n.srcReg2 != asm.NilRegister {
		offsetRegBits, err := intRegisterBits(n.srcReg2)
		if err != nil {
			return err
		}
		a.encodeLoadOrStoreWithRegisterOffset(baseRegBits, offsetRegBits, dstRegBits, inst.opcode,
			inst.size, inst.v)
	} else {
		err = a.encodeLoadOrStoreWithConstOffset(baseRegBits, dstRegBits, n.srcConst, inst.opcode,
			inst.size, inst.v, inst.datasize, inst.datasizeLog2)
	}
	return
}

// const16bitAligned check if the value is on the 16-bit alignment.
// If so, returns the shift num divided by 16, and otherwise -1.
func const16bitAligned(v int64) (ret int) {
	ret = -1
	for s := 0; s < 64; s += 16 {
		if (uint64(v) &^ (uint64(0xffff) << uint(s))) == 0 {
			ret = s / 16
			break
		}
	}
	return
}

// isBitMaskImmediate determines if the value can be encoded as "bitmask immediate".
//
//	Such an immediate is a 32-bit or 64-bit pattern viewed as a vector of identical elements of size e = 2, 4, 8, 16, 32, or 64 bits.
//	Each element contains the same sub-pattern: a single run of 1 to e-1 non-zero bits, rotated by 0 to e-1 bits.
//
// See https://developer.arm.com/documentation/dui0802/b/A64-General-Instructions/MOV--bitmask-immediate-
func isBitMaskImmediate(x uint64) bool {
	// All zeros and ones are not "bitmask immediate" by defainition.
	if x == 0 || x == 0xffff_ffff_ffff_ffff {
		return false
	}

	switch {
	case x != x>>32|x<<32:
		// e = 64
	case x != x>>16|x<<48:
		// e = 32 (x == x>>32|x<<32).
		// e.g. 0x00ff_ff00_00ff_ff00
		x = uint64(int32(x))
	case x != x>>8|x<<56:
		// e = 16 (x == x>>16|x<<48).
		// e.g. 0x00ff_00ff_00ff_00ff
		x = uint64(int16(x))
	case x != x>>4|x<<60:
		// e = 8 (x == x>>8|x<<56).
		// e.g. 0x0f0f_0f0f_0f0f_0f0f
		x = uint64(int8(x))
	default:
		// e = 4 or 2.
		return true
	}
	return sequenceOfSetbits(x) || sequenceOfSetbits(^x)
}

// sequenceOfSetbits returns true if the number's binary representation is the sequence set bit (1).
// For example: 0b1110 -> true, 0b1010 -> false
func sequenceOfSetbits(x uint64) bool {
	y := getLowestBit(x)
	// If x is a sequence of set bit, this should results in the number
	// with only one set bit (i.e. power of two).
	y += x
	return (y-1)&y == 0
}

func getLowestBit(x uint64) uint64 {
	// See https://stackoverflow.com/questions/12247186/find-the-lowest-set-bit
	return x & (^x + 1)
}

func (a *AssemblerImpl) addOrSub64BitRegisters(sfops byte, sp bool, dstRegBits, src1RegBits, src2RegBits byte) {
	// src1Reg = src1Reg +/- src2Reg

	if sp {
		// https://developer.arm.com/documentation/ddi0596/2021-12/Base-Instructions/ADD--extended-register---Add--extended-register--?lang=en
		a.buf.Write([]byte{
			(src1RegBits << 5) | dstRegBits,
			0b011<<5 | src1RegBits>>3,
			1<<5 | src2RegBits,
			sfops<<5 | 0b01011,
		})
	} else {
		a.buf.Write([]byte{
			(src1RegBits << 5) | dstRegBits,
			src1RegBits >> 3,
			src2RegBits,
			sfops<<5 | 0b01011,
		})
	}
}

// See "Logical (immediate)" in
// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Immediate
var logicalImmediate = map[asm.Instruction]struct {
	sf, opc  byte
	resolver func(imm asm.ConstantValue) (imms, immr, N byte, err error)
}{
	ANDIMM32: {sf: 0b0, opc: 0b00, resolver: func(imm asm.ConstantValue) (imms, immr, N byte, err error) {
		if !isBitMaskImmediate(uint64(imm)) {
			err = fmt.Errorf("const %d must be valid bitmask immediate for %s", imm, InstructionName(ANDIMM64))
			return
		}
		immr, imms, N = bitmaskImmediate(uint64(imm), false)
		return
	}},
	ANDIMM64: {sf: 0b1, opc: 0b00, resolver: func(imm asm.ConstantValue) (imms, immr, N byte, err error) {
		if !isBitMaskImmediate(uint64(imm)) {
			err = fmt.Errorf("const %d must be valid bitmask immediate for %s", imm, InstructionName(ANDIMM64))
			return
		}
		immr, imms, N = bitmaskImmediate(uint64(imm), true)
		return
	}},
}

func bitmaskImmediate(c uint64, is64bit bool) (immr, imms, N byte) {
	var size uint32
	switch {
	case c != c>>32|c<<32:
		size = 64
	case c != c>>16|c<<48:
		size = 32
		c = uint64(int32(c))
	case c != c>>8|c<<56:
		size = 16
		c = uint64(int16(c))
	case c != c>>4|c<<60:
		size = 8
		c = uint64(int8(c))
	case c != c>>2|c<<62:
		size = 4
		c = uint64(int64(c<<60) >> 60)
	default:
		size = 2
		c = uint64(int64(c<<62) >> 62)
	}

	neg := false
	if int64(c) < 0 {
		c = ^c
		neg = true
	}

	onesSize, nonZeroPos := getOnesSequenceSize(c)
	if neg {
		nonZeroPos = onesSize + nonZeroPos
		onesSize = size - onesSize
	}

	var mode byte = 32
	if is64bit {
		N, mode = 0b1, 64
	}

	immr = byte((size - nonZeroPos) & (size - 1) & uint32(mode-1))
	imms = byte((onesSize - 1) | 63&^(size<<1-1))
	return
}

func (a *AssemblerImpl) encodeConstToRegister(n *nodeImpl) (err error) {
	// Alias for readability.
	c := n.srcConst

	dstRegBits, err := intRegisterBits(n.dstReg)
	if err != nil {
		return err
	}

	if log, ok := logicalImmediate[n.instruction]; ok {
		// See "Logical (immediate)" in
		// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Immediate
		imms, immr, N, err := log.resolver(c)
		if err != nil {
			return err
		}

		a.buf.Write([]byte{
			(dstRegBits << 5) | dstRegBits,
			imms<<2 | dstRegBits>>3,
			N<<6 | immr,
			log.sf<<7 | log.opc<<5 | 0b10010,
		})
		return nil
	}

	// TODO: refactor and generalize the following like ^ logicalImmediate, etc.
	switch inst := n.instruction; inst {
	case ADD, ADDS, SUB, SUBS:
		srcRegBits := dstRegBits
		if n.srcReg != asm.NilRegister {
			srcRegBits, err = intRegisterBits(n.srcReg)
			if err != nil {
				return err
			}
		}

		var sfops byte
		if inst == ADD {
			sfops = 0b100
		} else if inst == ADDS {
			sfops = 0b101
		} else if inst == SUB {
			sfops = 0b110
		} else if inst == SUBS {
			sfops = 0b111
		}

		isSP := n.srcReg == RegSP || n.dstReg == RegSP
		if c == 0 {
			// If the constant equals zero, we encode it as ADD (register) with zero register.
			a.addOrSub64BitRegisters(sfops, isSP, dstRegBits, srcRegBits, zeroRegisterBits)
			return
		}

		if c >= 0 && (c <= 0xfff || (c&0xfff) == 0 && (uint64(c>>12) <= 0xfff)) {
			// If the const can be represented as "imm12" or "imm12 << 12": one instruction
			// https://github.com/golang/go/blob/release-branch.go1.15/src/cmd/internal/obj/arm64/asm7.go#L2992

			if c <= 0xfff {
				a.buf.Write([]byte{
					(srcRegBits << 5) | dstRegBits,
					(byte(c) << 2) | (srcRegBits >> 3),
					byte(c >> 6),
					sfops<<5 | 0b10001,
				})
			} else {
				c >>= 12
				a.buf.Write([]byte{
					(srcRegBits << 5) | dstRegBits,
					(byte(c) << 2) | (srcRegBits >> 3),
					0b01<<6 /* shift by 12 */ | byte(c>>6),
					sfops<<5 | 0b10001,
				})
			}
			return
		}

		if t := const16bitAligned(c); t >= 0 {
			// If the const can fit within 16-bit alignment, for example, 0xffff, 0xffff_0000 or 0xffff_0000_0000_0000
			// We could load it into temporary with movk.
			// https://github.com/golang/go/blob/release-branch.go1.15/src/cmd/internal/obj/arm64/asm7.go#L4029
			tmpRegBits := registerBits(a.temporaryRegister)

			// MOVZ $c, tmpReg with shifting.
			a.load16bitAlignedConst(c>>(16*t), byte(t), tmpRegBits, false, true)

			// ADD/SUB tmpReg, dstReg
			a.addOrSub64BitRegisters(sfops, isSP, dstRegBits, srcRegBits, tmpRegBits)
			return
		} else if t := const16bitAligned(^c); t >= 0 {
			// Also if the reverse of the const can fit within 16-bit range, do the same ^^.
			// https://github.com/golang/go/blob/release-branch.go1.15/src/cmd/internal/obj/arm64/asm7.go#L4029
			tmpRegBits := registerBits(a.temporaryRegister)

			// MOVN $c, tmpReg with shifting.
			a.load16bitAlignedConst(^c>>(16*t), byte(t), tmpRegBits, true, true)

			// ADD/SUB tmpReg, dstReg
			a.addOrSub64BitRegisters(sfops, isSP, dstRegBits, srcRegBits, tmpRegBits)
			return
		}

		if uc := uint64(c); isBitMaskImmediate(uc) {
			// If the const can be represented as "bitmask immediate", we load it via ORR into temp register.
			// https://github.com/golang/go/blob/release-branch.go1.15/src/cmd/internal/obj/arm64/asm7.go#L6570-L6583
			tmpRegBits := registerBits(a.temporaryRegister)
			// OOR $c, tmpReg
			a.loadConstViaBitMaskImmediate(uc, tmpRegBits, true)

			// ADD/SUB tmpReg, dstReg
			a.addOrSub64BitRegisters(sfops, isSP, dstRegBits, srcRegBits, tmpRegBits)
			return
		}

		// If the value fits within 24-bit, then we emit two add instructions
		if 0 <= c && c <= 0xffffff && inst != SUBS && inst != ADDS {
			// https://github.com/golang/go/blob/release-branch.go1.15/src/cmd/internal/obj/arm64/asm7.go#L3849-L3862
			a.buf.Write([]byte{
				(dstRegBits << 5) | dstRegBits,
				(byte(c) << 2) | (dstRegBits >> 3),
				byte(c & 0xfff >> 6),
				sfops<<5 | 0b10001,
			})
			c = c >> 12
			a.buf.Write([]byte{
				(dstRegBits << 5) | dstRegBits,
				(byte(c) << 2) | (dstRegBits >> 3),
				0b01_000000 /* shift by 12 */ | byte(c>>6),
				sfops<<5 | 0b10001,
			})
			return
		}

		// https://github.com/golang/go/blob/release-branch.go1.15/src/cmd/internal/obj/arm64/asm7.go#L3163-L3203
		// Otherwise we use MOVZ and MOVNs for loading const into tmpRegister.
		tmpRegBits := registerBits(a.temporaryRegister)
		a.load64bitConst(c, tmpRegBits)
		a.addOrSub64BitRegisters(sfops, isSP, dstRegBits, srcRegBits, tmpRegBits)
	case MOVW:
		if c == 0 {
			a.buf.Write([]byte{
				(zeroRegisterBits << 5) | dstRegBits,
				zeroRegisterBits >> 3,
				0b000_00000 | zeroRegisterBits,
				0b0_01_01010,
			})
			return
		}

		// Following the logic here:
		// https://github.com/golang/go/blob/release-branch.go1.15/src/cmd/internal/obj/arm64/asm7.go#L1637
		c32 := uint32(c)
		ic := int64(c32)
		if ic >= 0 && (ic <= 0xfff || (ic&0xfff) == 0 && (uint64(ic>>12) <= 0xfff)) {
			if isBitMaskImmediate(uint64(c)) {
				a.loadConstViaBitMaskImmediate(uint64(c), dstRegBits, false)
				return
			}
		}

		if t := const16bitAligned(int64(c32)); t >= 0 {
			// If the const can fit within 16-bit alignment, for example, 0xffff, 0xffff_0000 or 0xffff_0000_0000_0000
			// We could load it into temporary with movk.
			a.load16bitAlignedConst(int64(c32)>>(16*t), byte(t), dstRegBits, false, false)
		} else if t := const16bitAligned(int64(^c32)); t >= 0 {
			// Also, if the reverse of the const can fit within 16-bit range, do the same ^^.
			a.load16bitAlignedConst(int64(^c32)>>(16*t), byte(t), dstRegBits, true, false)
		} else if isBitMaskImmediate(uint64(c)) {
			a.loadConstViaBitMaskImmediate(uint64(c), dstRegBits, false)
		} else {
			// Otherwise, we use MOVZ and MOVK to load it.
			// https://github.com/golang/go/blob/release-branch.go1.15/src/cmd/internal/obj/arm64/asm7.go#L6623-L6630
			c16 := uint16(c32)
			// MOVZ: https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/MOVZ
			a.buf.Write([]byte{
				(byte(c16) << 5) | dstRegBits,
				byte(c16 >> 3),
				1<<7 | byte(c16>>11),
				0b0_10_10010,
			})
			// MOVK: https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/MOVK
			c16 = uint16(c32 >> 16)
			if c16 != 0 {
				a.buf.Write([]byte{
					(byte(c16) << 5) | dstRegBits,
					byte(c16 >> 3),
					1<<7 | 0b0_01_00000 /* shift by 16 */ | byte(c16>>11),
					0b0_11_10010,
				})
			}
		}
	case MOVD:
		// Following the logic here:
		// https://github.com/golang/go/blob/release-branch.go1.15/src/cmd/internal/obj/arm64/asm7.go#L1798-L1852
		if c >= 0 && (c <= 0xfff || (c&0xfff) == 0 && (uint64(c>>12) <= 0xfff)) {
			if isBitMaskImmediate(uint64(c)) {
				a.loadConstViaBitMaskImmediate(uint64(c), dstRegBits, true)
				return
			}
		}

		if t := const16bitAligned(c); t >= 0 {
			// If the const can fit within 16-bit alignment, for example, 0xffff, 0xffff_0000 or 0xffff_0000_0000_0000
			// We could load it into temporary with movk.
			a.load16bitAlignedConst(c>>(16*t), byte(t), dstRegBits, false, true)
		} else if t := const16bitAligned(^c); t >= 0 {
			// Also, if the reverse of the const can fit within 16-bit range, do the same ^^.
			a.load16bitAlignedConst((^c)>>(16*t), byte(t), dstRegBits, true, true)
		} else if isBitMaskImmediate(uint64(c)) {
			a.loadConstViaBitMaskImmediate(uint64(c), dstRegBits, true)
		} else {
			a.load64bitConst(c, dstRegBits)
		}
	case LSR:
		if c == 0 {
			err = errors.New("LSR with zero constant should be optimized out")
			return
		} else if c < 0 || c > 63 {
			err = fmt.Errorf("LSR requires immediate to be within 0 to 63, but got %d", c)
			return
		}

		// LSR(immediate) is an alias of UBFM
		// https://developer.arm.com/documentation/ddi0596/2021-12/Base-Instructions/LSR--immediate---Logical-Shift-Right--immediate---an-alias-of-UBFM-?lang=en
		a.buf.Write([]byte{
			(dstRegBits << 5) | dstRegBits,
			0b111111_00 | dstRegBits>>3,
			0b01_000000 | byte(c),
			0b110_10011,
		})
	case LSL:
		if c == 0 {
			err = errors.New("LSL with zero constant should be optimized out")
			return
		} else if c < 0 || c > 63 {
			err = fmt.Errorf("LSL requires immediate to be within 0 to 63, but got %d", c)
			return
		}

		// LSL(immediate) is an alias of UBFM
		// https://developer.arm.com/documentation/ddi0596/2021-12/Base-Instructions/LSL--immediate---Logical-Shift-Left--immediate---an-alias-of-UBFM-
		cb := byte(c)
		a.buf.Write([]byte{
			(dstRegBits << 5) | dstRegBits,
			(0b111111-cb)<<2 | dstRegBits>>3,
			0b01_000000 | (64 - cb),
			0b110_10011,
		})

	default:
		return errorEncodingUnsupported(n)
	}
	return
}

func (a *AssemblerImpl) movk(v uint64, shfitNum int, dstRegBits byte) {
	// https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/MOVK
	a.buf.Write([]byte{
		(byte(v) << 5) | dstRegBits,
		byte(v >> 3),
		1<<7 | byte(shfitNum)<<5 | (0b000_11111 & byte(v>>11)),
		0b1_11_10010,
	})
}

func (a *AssemblerImpl) movz(v uint64, shfitNum int, dstRegBits byte) {
	// https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/MOVZ
	a.buf.Write([]byte{
		(byte(v) << 5) | dstRegBits,
		byte(v >> 3),
		1<<7 | byte(shfitNum)<<5 | (0b000_11111 & byte(v>>11)),
		0b1_10_10010,
	})
}

func (a *AssemblerImpl) movn(v uint64, shfitNum int, dstRegBits byte) {
	// https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/MOVZ
	a.buf.Write([]byte{
		(byte(v) << 5) | dstRegBits,
		byte(v >> 3),
		1<<7 | byte(shfitNum)<<5 | (0b000_11111 & byte(v>>11)),
		0b1_00_10010,
	})
}

// load64bitConst loads a 64-bit constant into the register, following the same logic to decide how to load large 64-bit
// consts as in the Go assembler.
//
// See https://github.com/golang/go/blob/release-branch.go1.15/src/cmd/internal/obj/arm64/asm7.go#L6632-L6759
func (a *AssemblerImpl) load64bitConst(c int64, dstRegBits byte) {
	var bits [4]uint64
	var zeros, negs int
	for i := 0; i < 4; i++ {
		bits[i] = uint64((c >> uint(i*16)) & 0xffff)
		if v := bits[i]; v == 0 {
			zeros++
		} else if v == 0xffff {
			negs++
		}
	}

	if zeros == 3 {
		// one MOVZ instruction.
		for i, v := range bits {
			if v != 0 {
				a.movz(v, i, dstRegBits)
			}
		}
	} else if negs == 3 {
		// one MOVN instruction.
		for i, v := range bits {
			if v != 0xffff {
				v = ^v
				a.movn(v, i, dstRegBits)
			}
		}
	} else if zeros == 2 {
		// one MOVZ then one OVK.
		var movz bool
		for i, v := range bits {
			if !movz && v != 0 { // MOVZ.
				// https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/MOVZ
				a.movz(v, i, dstRegBits)
				movz = true
			} else if v != 0 {
				a.movk(v, i, dstRegBits)
			}
		}

	} else if negs == 2 {
		// one MOVN then one or two MOVK.
		var movn bool
		for i, v := range bits { // Emit MOVN.
			if !movn && v != 0xffff {
				v = ^v
				// https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/MOVN
				a.movn(v, i, dstRegBits)
				movn = true
			} else if v != 0xffff {
				a.movk(v, i, dstRegBits)
			}
		}

	} else if zeros == 1 {
		// one MOVZ then two MOVK.
		var movz bool
		for i, v := range bits {
			if !movz && v != 0 { // MOVZ.
				// https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/MOVZ
				a.movz(v, i, dstRegBits)
				movz = true
			} else if v != 0 {
				a.movk(v, i, dstRegBits)
			}
		}

	} else if negs == 1 {
		// one MOVN then two MOVK.
		var movn bool
		for i, v := range bits { // Emit MOVN.
			if !movn && v != 0xffff {
				v = ^v
				// https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/MOVN
				a.movn(v, i, dstRegBits)
				movn = true
			} else if v != 0xffff {
				a.movk(v, i, dstRegBits)
			}
		}

	} else {
		// one MOVZ then tree MOVK.
		var movz bool
		for i, v := range bits {
			if !movz && v != 0 { // MOVZ.
				// https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/MOVZ
				a.movz(v, i, dstRegBits)
				movz = true
			} else if v != 0 {
				a.movk(v, i, dstRegBits)
			}
		}

	}
}

func (a *AssemblerImpl) load16bitAlignedConst(c int64, shiftNum byte, regBits byte, reverse bool, dst64bit bool) {
	var lastByte byte
	if reverse {
		// MOVN: https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/MOVZ
		lastByte = 0b0_00_10010
	} else {
		// MOVZ: https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/MOVN
		lastByte = 0b0_10_10010
	}
	if dst64bit {
		lastByte |= 0b1 << 7
	}
	a.buf.Write([]byte{
		(byte(c) << 5) | regBits,
		byte(c >> 3),
		1<<7 | (shiftNum << 5) | byte(c>>11),
		lastByte,
	})
}

// loadConstViaBitMaskImmediate loads the constant with ORR (bitmask immediate).
// https://developer.arm.com/documentation/ddi0596/2021-12/Base-Instructions/ORR--immediate---Bitwise-OR--immediate--?lang=en
func (a *AssemblerImpl) loadConstViaBitMaskImmediate(c uint64, regBits byte, dst64bit bool) {
	var size uint32
	switch {
	case c != c>>32|c<<32:
		size = 64
	case c != c>>16|c<<48:
		size = 32
		c = uint64(int32(c))
	case c != c>>8|c<<56:
		size = 16
		c = uint64(int16(c))
	case c != c>>4|c<<60:
		size = 8
		c = uint64(int8(c))
	case c != c>>2|c<<62:
		size = 4
		c = uint64(int64(c<<60) >> 60)
	default:
		size = 2
		c = uint64(int64(c<<62) >> 62)
	}

	neg := false
	if int64(c) < 0 {
		c = ^c
		neg = true
	}

	onesSize, nonZeroPos := getOnesSequenceSize(c)
	if neg {
		nonZeroPos = onesSize + nonZeroPos
		onesSize = size - onesSize
	}

	// See the following article for understanding the encoding.
	// https://dinfuehr.github.io/blog/encoding-of-immediate-values-on-aarch64/
	var n byte
	mode := 32
	if dst64bit && size == 64 {
		n = 0b1
		mode = 64
	}

	r := byte((size - nonZeroPos) & (size - 1) & uint32(mode-1))
	s := byte((onesSize - 1) | 63&^(size<<1-1))

	var sf byte
	if dst64bit {
		sf = 0b1
	}
	a.buf.Write([]byte{
		(zeroRegisterBits << 5) | regBits,
		s<<2 | (zeroRegisterBits >> 3),
		n<<6 | r,
		sf<<7 | 0b0_01_10010,
	})
}

func getOnesSequenceSize(x uint64) (size, nonZeroPos uint32) {
	// Take 0b00111000 for example:
	y := getLowestBit(x)               // = 0b0000100
	nonZeroPos = setBitPos(y)          // = 2
	size = setBitPos(x+y) - nonZeroPos // = setBitPos(0b0100000) - 2 = 5 - 2 = 3
	return
}

func setBitPos(x uint64) (ret uint32) {
	for ; ; ret++ {
		if x == 0b1 {
			break
		}
		x = x >> 1
	}
	return
}

func checkArrangementIndexPair(arr VectorArrangement, index VectorIndex) (err error) {
	if arr == VectorArrangementNone {
		return nil
	}
	var valid bool
	switch arr {
	case VectorArrangement8B:
		valid = index < 8
	case VectorArrangement16B:
		valid = index < 16
	case VectorArrangement4H:
		valid = index < 4
	case VectorArrangement8H:
		valid = index < 8
	case VectorArrangement2S:
		valid = index < 2
	case VectorArrangement4S:
		valid = index < 4
	case VectorArrangement1D:
		valid = index < 1
	case VectorArrangement2D:
		valid = index < 2
	case VectorArrangementB:
		valid = index < 16
	case VectorArrangementH:
		valid = index < 8
	case VectorArrangementS:
		valid = index < 4
	case VectorArrangementD:
		valid = index < 2
	}
	if !valid {
		err = fmt.Errorf("invalid arrangement and index pair: %s[%d]", arr, index)
	}
	return
}

func (a *AssemblerImpl) encodeMemoryToVectorRegister(n *nodeImpl) (err error) {
	srcBaseRegBits, err := intRegisterBits(n.srcReg)
	if err != nil {
		return err
	}

	dstVectorRegBits, err := vectorRegisterBits(n.dstReg)
	if err != nil {
		return err
	}

	switch n.instruction {
	case VMOV: // translated as LDR(immediate,SIMD&FP)
		// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/LDR--immediate--SIMD-FP---Load-SIMD-FP-Register--immediate-offset--?lang=en
		var size, opcode byte
		var dataSize, dataSizeLog2 int64
		switch n.vectorArrangement {
		case VectorArrangementB:
			size, opcode, dataSize, dataSizeLog2 = 0b00, 0b01, 1, 0
		case VectorArrangementH:
			size, opcode, dataSize, dataSizeLog2 = 0b01, 0b01, 2, 1
		case VectorArrangementS:
			size, opcode, dataSize, dataSizeLog2 = 0b10, 0b01, 4, 2
		case VectorArrangementD:
			size, opcode, dataSize, dataSizeLog2 = 0b11, 0b01, 8, 3
		case VectorArrangementQ:
			size, opcode, dataSize, dataSizeLog2 = 0b00, 0b11, 16, 4
		}
		const v = 1 // v as in https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Loads-and-Stores?lang=en#ldst_pos
		if n.srcReg2 != asm.NilRegister {
			offsetRegBits, err := intRegisterBits(n.srcReg2)
			if err != nil {
				return err
			}
			a.encodeLoadOrStoreWithRegisterOffset(srcBaseRegBits, offsetRegBits, dstVectorRegBits, opcode, size, v)
		} else {
			err = a.encodeLoadOrStoreWithConstOffset(srcBaseRegBits, dstVectorRegBits,
				n.srcConst, opcode, size, v, dataSize, dataSizeLog2)
		}
	case LD1R:
		if n.srcReg2 != asm.NilRegister || n.srcConst != 0 {
			return fmt.Errorf("offset for %s is not implemented", InstructionName(LD1R))
		}

		var size, q byte
		switch n.vectorArrangement {
		case VectorArrangement8B:
			size, q = 0b00, 0b0
		case VectorArrangement16B:
			size, q = 0b00, 0b1
		case VectorArrangement4H:
			size, q = 0b01, 0b0
		case VectorArrangement8H:
			size, q = 0b01, 0b1
		case VectorArrangement2S:
			size, q = 0b10, 0b0
		case VectorArrangement4S:
			size, q = 0b10, 0b1
		case VectorArrangement1D:
			size, q = 0b11, 0b0
		case VectorArrangement2D:
			size, q = 0b11, 0b1
		}

		// No offset encoding.
		// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/LD1R--Load-one-single-element-structure-and-Replicate-to-all-lanes--of-one-register--?lang=en#iclass_as_post_index
		a.buf.Write([]byte{
			(srcBaseRegBits << 5) | dstVectorRegBits,
			0b11_000000 | size<<2 | srcBaseRegBits>>3,
			0b01_000000,
			q<<6 | 0b1101,
		})
	default:
		return errorEncodingUnsupported(n)
	}
	return
}

func arrangementSizeQ(arr VectorArrangement) (size, q byte) {
	switch arr {
	case VectorArrangement8B:
		size, q = 0b00, 0
	case VectorArrangement16B:
		size, q = 0b00, 1
	case VectorArrangement4H:
		size, q = 0b01, 0
	case VectorArrangement8H:
		size, q = 0b01, 1
	case VectorArrangement2S:
		size, q = 0b10, 0
	case VectorArrangement4S:
		size, q = 0b10, 1
	case VectorArrangement1D:
		size, q = 0b11, 0
	case VectorArrangement2D:
		size, q = 0b11, 1
	}
	return
}

func (a *AssemblerImpl) encodeVectorRegisterToMemory(n *nodeImpl) (err error) {
	srcVectorRegBits, err := vectorRegisterBits(n.srcReg)
	if err != nil {
		return err
	}

	dstBaseRegBits, err := intRegisterBits(n.dstReg)
	if err != nil {
		return err
	}

	switch n.instruction {
	case VMOV: // translated as STR(immediate,SIMD&FP)
		// https://developer.arm.com/documentation/ddi0596/2020-12/SIMD-FP-Instructions/STR--immediate--SIMD-FP---Store-SIMD-FP-register--immediate-offset--
		var size, opcode byte
		var dataSize, dataSizeLog2 int64
		switch n.vectorArrangement {
		case VectorArrangementB:
			size, opcode, dataSize, dataSizeLog2 = 0b00, 0b00, 1, 0
		case VectorArrangementH:
			size, opcode, dataSize, dataSizeLog2 = 0b01, 0b00, 2, 1
		case VectorArrangementS:
			size, opcode, dataSize, dataSizeLog2 = 0b10, 0b00, 4, 2
		case VectorArrangementD:
			size, opcode, dataSize, dataSizeLog2 = 0b11, 0b00, 8, 3
		case VectorArrangementQ:
			size, opcode, dataSize, dataSizeLog2 = 0b00, 0b10, 16, 4
		}
		const v = 1 // v as in https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Loads-and-Stores?lang=en#ldst_pos

		if n.dstReg2 != asm.NilRegister {
			offsetRegBits, err := intRegisterBits(n.dstReg2)
			if err != nil {
				return err
			}
			a.encodeLoadOrStoreWithRegisterOffset(dstBaseRegBits, offsetRegBits, srcVectorRegBits, opcode, size, v)
		} else {
			err = a.encodeLoadOrStoreWithConstOffset(dstBaseRegBits, srcVectorRegBits,
				n.dstConst, opcode, size, v, dataSize, dataSizeLog2)
		}
	default:
		return errorEncodingUnsupported(n)
	}
	return
}

func (a *AssemblerImpl) encodeStaticConstToVectorRegister(n *nodeImpl) (err error) {
	if n.instruction != VMOV {
		return errorEncodingUnsupported(n)
	}

	dstRegBits, err := vectorRegisterBits(n.dstReg)
	if err != nil {
		return err
	}

	// LDR (literal, SIMD&FP)
	// https://developer.arm.com/documentation/ddi0596/2020-12/SIMD-FP-Instructions/LDR--literal--SIMD-FP---Load-SIMD-FP-Register--PC-relative-literal--
	var opc byte
	var constLength int
	switch n.vectorArrangement {
	case VectorArrangementS:
		opc, constLength = 0b00, 4
	case VectorArrangementD:
		opc, constLength = 0b01, 8
	case VectorArrangementQ:
		opc, constLength = 0b10, 16
	}

	loadLiteralOffsetInBinary := uint64(a.buf.Len())
	a.pool.AddConst(n.staticConst, loadLiteralOffsetInBinary)

	if len(n.staticConst.Raw) != constLength {
		return fmt.Errorf("invalid const length for %s: want %d but was %d",
			n.vectorArrangement, constLength, len(n.staticConst.Raw))
	}

	a.buf.Write([]byte{dstRegBits, 0x0, 0x0, opc<<6 | 0b11100})
	n.staticConst.AddOffsetFinalizedCallback(func(offsetOfConst uint64) {
		// LDR (literal, SIMD&FP) encodes offset divided by 4.
		offset := (int(offsetOfConst) - int(loadLiteralOffsetInBinary)) / 4
		bin := a.buf.Bytes()
		bin[loadLiteralOffsetInBinary] |= byte(offset << 5)
		bin[loadLiteralOffsetInBinary+1] |= byte(offset >> 3)
		bin[loadLiteralOffsetInBinary+2] |= byte(offset >> 11)
	})
	return
}

// advancedSIMDTwoRegisterMisc holds information to encode instructions as "Advanced SIMD two-register miscellaneous" in
// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en
var advancedSIMDTwoRegisterMisc = map[asm.Instruction]struct {
	u, opcode byte
	qAndSize  map[VectorArrangement]qAndSize
}{
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/NOT--Bitwise-NOT--vector--?lang=en
	NOT: {
		u: 0b1, opcode: 0b00101,
		qAndSize: map[VectorArrangement]qAndSize{
			VectorArrangement16B: {size: 0b00, q: 0b1},
			VectorArrangement8B:  {size: 0b00, q: 0b0},
		},
	},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FNEG--vector---Floating-point-Negate--vector--?lang=en
	VFNEG: {
		u: 0b1, opcode: 0b01111,
		qAndSize: map[VectorArrangement]qAndSize{
			VectorArrangement4S: {size: 0b10, q: 0b1},
			VectorArrangement2S: {size: 0b10, q: 0b0},
			VectorArrangement2D: {size: 0b11, q: 0b1},
		},
	},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FABS--vector---Floating-point-Absolute-value--vector--?lang=en
	VFABS: {u: 0, opcode: 0b01111, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement2D: {size: 0b11, q: 0b1},
		VectorArrangement4S: {size: 0b10, q: 0b1},
		VectorArrangement2S: {size: 0b10, q: 0b0},
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FSQRT--vector---Floating-point-Square-Root--vector--?lang=en
	VFSQRT: {u: 1, opcode: 0b11111, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement2D: {size: 0b11, q: 0b1},
		VectorArrangement4S: {size: 0b10, q: 0b1},
		VectorArrangement2S: {size: 0b10, q: 0b0},
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FRINTM--vector---Floating-point-Round-to-Integral--toward-Minus-infinity--vector--?lang=en
	VFRINTM: {u: 0, opcode: 0b11001, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement2D: {size: 0b01, q: 0b1},
		VectorArrangement4S: {size: 0b00, q: 0b1},
		VectorArrangement2S: {size: 0b00, q: 0b0},
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FRINTN--vector---Floating-point-Round-to-Integral--to-nearest-with-ties-to-even--vector--?lang=en
	VFRINTN: {u: 0, opcode: 0b11000, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement2D: {size: 0b01, q: 0b1},
		VectorArrangement4S: {size: 0b00, q: 0b1},
		VectorArrangement2S: {size: 0b00, q: 0b0},
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FRINTP--vector---Floating-point-Round-to-Integral--toward-Plus-infinity--vector--?lang=en
	VFRINTP: {u: 0, opcode: 0b11000, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement2D: {size: 0b11, q: 0b1},
		VectorArrangement4S: {size: 0b10, q: 0b1},
		VectorArrangement2S: {size: 0b10, q: 0b0},
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FRINTZ--vector---Floating-point-Round-to-Integral--toward-Zero--vector--?lang=en
	VFRINTZ: {u: 0, opcode: 0b11001, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement2D: {size: 0b11, q: 0b1},
		VectorArrangement4S: {size: 0b10, q: 0b1},
		VectorArrangement2S: {size: 0b10, q: 0b0},
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/CNT--Population-Count-per-byte-?lang=en
	VCNT: {u: 0b0, opcode: 0b00101, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement8B:  {size: 0b00, q: 0b0},
		VectorArrangement16B: {size: 0b00, q: 0b1},
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/NEG--vector---Negate--vector--?lang=en
	VNEG: {u: 0b1, opcode: 0b01011, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/ABS--Absolute-value--vector--?lang=en
	VABS: {u: 0b0, opcode: 0b01011, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/REV64--Reverse-elements-in-64-bit-doublewords--vector--?lang=en
	REV64: {u: 0b0, opcode: 0b00000, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/XTN--XTN2--Extract-Narrow-?lang=en
	XTN: {u: 0b0, opcode: 0b10010, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement2D: {q: 0, size: 0b10},
		VectorArrangement4S: {q: 0, size: 0b01},
		VectorArrangement8H: {q: 0, size: 0b00},
	}},
	SHLL: {u: 0b1, opcode: 0b10011, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement8B: {q: 0b00, size: 0b00},
		VectorArrangement4H: {q: 0b00, size: 0b01},
		VectorArrangement2S: {q: 0b00, size: 0b10},
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/CMEQ--zero---Compare-bitwise-Equal-to-zero--vector--?lang=en
	CMEQZERO: {u: 0b0, opcode: 0b01001, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SADDLP--Signed-Add-Long-Pairwise-?lang=en
	SADDLP: {u: 0b0, opcode: 0b00010, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/UADDLP--Unsigned-Add-Long-Pairwise-?lang=en
	UADDLP: {u: 0b1, opcode: 0b00010, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FCVTZS--vector--integer---Floating-point-Convert-to-Signed-integer--rounding-toward-Zero--vector--?lang=en
	VFCVTZS: {u: 0b0, opcode: 0b11011, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement4S: {size: 0b10, q: 0b1},
		VectorArrangement2S: {size: 0b10, q: 0b0},
		VectorArrangement2D: {size: 0b11, q: 0b1},
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FCVTZU--vector--integer---Floating-point-Convert-to-Unsigned-integer--rounding-toward-Zero--vector--?lang=en
	VFCVTZU: {u: 0b1, opcode: 0b11011, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement4S: {size: 0b10, q: 0b1},
		VectorArrangement2S: {size: 0b10, q: 0b0},
		VectorArrangement2D: {size: 0b11, q: 0b1},
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SQXTN--SQXTN2--Signed-saturating-extract-Narrow-?lang=en
	SQXTN: {u: 0b0, opcode: 0b10100, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement8B: {q: 0b0, size: 0b00},
		VectorArrangement4H: {q: 0b0, size: 0b01},
		VectorArrangement2S: {q: 0b0, size: 0b10},
	}},

	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SQXTN--SQXTN2--Signed-saturating-extract-Narrow-?lang=en
	SQXTN2: {u: 0b0, opcode: 0b10100, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement16B: {q: 0b1, size: 0b00},
		VectorArrangement8H:  {q: 0b1, size: 0b01},
		VectorArrangement4S:  {q: 0b1, size: 0b10},
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/UQXTN--UQXTN2--Unsigned-saturating-extract-Narrow-?lang=en
	UQXTN: {u: 0b1, opcode: 0b10100, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SQXTUN--SQXTUN2--Signed-saturating-extract-Unsigned-Narrow-?lang=en
	SQXTUN: {u: 0b1, opcode: 0b10010, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement8B: {q: 0b0, size: 0b00},
		VectorArrangement4H: {q: 0b0, size: 0b01},
		VectorArrangement2S: {q: 0b0, size: 0b10},
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SQXTUN--SQXTUN2--Signed-saturating-extract-Unsigned-Narrow-?lang=en
	SQXTUN2: {u: 0b1, opcode: 0b10010, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement16B: {q: 0b1, size: 0b00},
		VectorArrangement8H:  {q: 0b1, size: 0b01},
		VectorArrangement4S:  {q: 0b1, size: 0b10},
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SCVTF--vector--integer---Signed-integer-Convert-to-Floating-point--vector--?lang=en
	VSCVTF: {u: 0b0, opcode: 0b11101, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement2D: {q: 0b1, size: 0b01},
		VectorArrangement4S: {q: 0b1, size: 0b00},
		VectorArrangement2S: {q: 0b0, size: 0b00},
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/UCVTF--vector--integer---Unsigned-integer-Convert-to-Floating-point--vector--?lang=en
	VUCVTF: {u: 0b1, opcode: 0b11101, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement2D: {q: 0b1, size: 0b01},
		VectorArrangement4S: {q: 0b1, size: 0b00},
		VectorArrangement2S: {q: 0b0, size: 0b00},
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FCVTL--FCVTL2--Floating-point-Convert-to-higher-precision-Long--vector--?lang=en
	FCVTL: {u: 0b0, opcode: 0b10111, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement2S: {size: 0b01, q: 0b0},
		VectorArrangement4H: {size: 0b00, q: 0b0},
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FCVTN--FCVTN2--Floating-point-Convert-to-lower-precision-Narrow--vector--?lang=en
	FCVTN: {u: 0b0, opcode: 0b10110, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement2S: {size: 0b01, q: 0b0},
		VectorArrangement4H: {size: 0b00, q: 0b0},
	}},
}

// advancedSIMDThreeDifferent holds information to encode instructions as "Advanced SIMD three different" in
// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en
var advancedSIMDThreeDifferent = map[asm.Instruction]struct {
	u, opcode byte
	qAndSize  map[VectorArrangement]qAndSize
}{
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/UMLAL--UMLAL2--vector---Unsigned-Multiply-Add-Long--vector--?lang=en
	VUMLAL: {u: 0b1, opcode: 0b1000, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement2S: {q: 0b0, size: 0b10},
		VectorArrangement4H: {q: 0b0, size: 0b01},
		VectorArrangement8B: {q: 0b0, size: 0b00},
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SMULL--SMULL2--vector---Signed-Multiply-Long--vector--?lang=en
	SMULL: {u: 0b0, opcode: 0b1100, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement8B: {q: 0b0, size: 0b00},
		VectorArrangement4H: {q: 0b0, size: 0b01},
		VectorArrangement2S: {q: 0b0, size: 0b10},
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SMULL--SMULL2--vector---Signed-Multiply-Long--vector--?lang=en
	SMULL2: {u: 0b0, opcode: 0b1100, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement16B: {q: 0b1, size: 0b00},
		VectorArrangement8H:  {q: 0b1, size: 0b01},
		VectorArrangement4S:  {q: 0b1, size: 0b10},
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en
	UMULL: {u: 0b1, opcode: 0b1100, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement8B: {q: 0b0, size: 0b00},
		VectorArrangement4H: {q: 0b0, size: 0b01},
		VectorArrangement2S: {q: 0b0, size: 0b10},
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en
	UMULL2: {u: 0b1, opcode: 0b1100, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement16B: {q: 0b1, size: 0b00},
		VectorArrangement8H:  {q: 0b1, size: 0b01},
		VectorArrangement4S:  {q: 0b1, size: 0b10},
	}},
}

// advancedSIMDThreeSame holds information to encode instructions as "Advanced SIMD three same" in
// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en
var advancedSIMDThreeSame = map[asm.Instruction]struct {
	u, opcode byte
	qAndSize  map[VectorArrangement]qAndSize
}{
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/AND--vector---Bitwise-AND--vector--?lang=en
	VAND: {
		u: 0b0, opcode: 0b00011,
		qAndSize: map[VectorArrangement]qAndSize{
			VectorArrangement16B: {size: 0b00, q: 0b1},
			VectorArrangement8B:  {size: 0b00, q: 0b0},
		},
	},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/BSL--Bitwise-Select-?lang=en
	BSL: {
		u: 0b1, opcode: 0b00011,
		qAndSize: map[VectorArrangement]qAndSize{
			VectorArrangement16B: {size: 0b01, q: 0b1},
			VectorArrangement8B:  {size: 0b01, q: 0b0},
		},
	},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/EOR--vector---Bitwise-Exclusive-OR--vector--?lang=en
	EOR: {
		u: 0b1, opcode: 0b00011,
		qAndSize: map[VectorArrangement]qAndSize{
			VectorArrangement16B: {size: 0b00, q: 0b1},
			VectorArrangement8B:  {size: 0b00, q: 0b0},
		},
	},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/ORR--vector--register---Bitwise-inclusive-OR--vector--register--?lang=en
	VORR: {
		u: 0b0, opcode: 0b00011,
		qAndSize: map[VectorArrangement]qAndSize{
			VectorArrangement16B: {size: 0b10, q: 0b1},
			VectorArrangement8B:  {size: 0b10, q: 0b0},
		},
	},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/BIC--vector--register---Bitwise-bit-Clear--vector--register--?lang=en
	BIC: {
		u: 0b0, opcode: 0b00011,
		qAndSize: map[VectorArrangement]qAndSize{
			VectorArrangement16B: {size: 0b01, q: 0b1},
			VectorArrangement8B:  {size: 0b01, q: 0b0},
		},
	},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FADD--vector---Floating-point-Add--vector--?lang=en
	VFADDS: {
		u: 0b0, opcode: 0b11010,
		qAndSize: map[VectorArrangement]qAndSize{
			VectorArrangement4S: {size: 0b00, q: 0b1},
			VectorArrangement2S: {size: 0b00, q: 0b0},
		},
	},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FADD--vector---Floating-point-Add--vector--?lang=en
	VFADDD: {
		u: 0b0, opcode: 0b11010,
		qAndSize: map[VectorArrangement]qAndSize{
			VectorArrangement2D: {size: 0b01, q: 0b1},
		},
	},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FSUB--vector---Floating-point-Subtract--vector--?lang=en
	VFSUBS: {
		u: 0b0, opcode: 0b11010,
		qAndSize: map[VectorArrangement]qAndSize{
			VectorArrangement4S: {size: 0b10, q: 0b1},
			VectorArrangement2S: {size: 0b10, q: 0b0},
		},
	},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FSUB--vector---Floating-point-Subtract--vector--?lang=en
	VFSUBD: {
		u: 0b0, opcode: 0b11010,
		qAndSize: map[VectorArrangement]qAndSize{
			VectorArrangement2D: {size: 0b11, q: 0b1},
		},
	},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/UMAXP--Unsigned-Maximum-Pairwise-?lang=en
	UMAXP: {u: 0b1, opcode: 0b10100, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/CMEQ--register---Compare-bitwise-Equal--vector--?lang=en
	CMEQ: {u: 0b1, opcode: 0b10001, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/dui0801/g/A64-SIMD-Vector-Instructions/ADDP--vector-
	VADDP: {u: 0b0, opcode: 0b10111, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/ADD--vector---Add--vector--?lang=en
	VADD: {u: 0, opcode: 0b10000, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SUB--vector---Subtract--vector--?lang=en
	VSUB: {u: 1, opcode: 0b10000, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SSHL--Signed-Shift-Left--register--?lang=en
	SSHL: {u: 0, opcode: 0b01000, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SSHL--Signed-Shift-Left--register--?lang=en
	USHL: {u: 0b1, opcode: 0b01000, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/CMGT--register---Compare-signed-Greater-than--vector--?lang=en
	CMGT: {u: 0b0, opcode: 0b00110, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/CMHI--register---Compare-unsigned-Higher--vector--?lang=en
	CMHI: {u: 0b1, opcode: 0b00110, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/CMGE--register---Compare-signed-Greater-than-or-Equal--vector--?lang=en
	CMGE: {u: 0b0, opcode: 0b00111, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/CMHS--register---Compare-unsigned-Higher-or-Same--vector--?lang=en
	CMHS: {u: 0b1, opcode: 0b00111, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FCMEQ--register---Floating-point-Compare-Equal--vector--?lang=en
	FCMEQ: {
		u: 0b0, opcode: 0b11100,
		qAndSize: map[VectorArrangement]qAndSize{
			VectorArrangement4S: {size: 0b00, q: 0b1},
			VectorArrangement2S: {size: 0b00, q: 0b0},
			VectorArrangement2D: {size: 0b01, q: 0b1},
		},
	},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FCMGT--register---Floating-point-Compare-Greater-than--vector--?lang=en
	FCMGT: {
		u: 0b1, opcode: 0b11100,
		qAndSize: map[VectorArrangement]qAndSize{
			VectorArrangement4S: {size: 0b10, q: 0b1},
			VectorArrangement2S: {size: 0b10, q: 0b0},
			VectorArrangement2D: {size: 0b11, q: 0b1},
		},
	},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FCMGE--register---Floating-point-Compare-Greater-than-or-Equal--vector--?lang=en
	FCMGE: {
		u: 0b1, opcode: 0b11100,
		qAndSize: map[VectorArrangement]qAndSize{
			VectorArrangement4S: {size: 0b00, q: 0b1},
			VectorArrangement2S: {size: 0b00, q: 0b0},
			VectorArrangement2D: {size: 0b01, q: 0b1},
		},
	},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FMIN--vector---Floating-point-minimum--vector--?lang=en
	VFMIN: {
		u: 0b0, opcode: 0b11110,
		qAndSize: map[VectorArrangement]qAndSize{
			VectorArrangement4S: {size: 0b10, q: 0b1},
			VectorArrangement2S: {size: 0b10, q: 0b0},
			VectorArrangement2D: {size: 0b11, q: 0b1},
		},
	},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FMAX--vector---Floating-point-Maximum--vector--?lang=en
	VFMAX: {
		u: 0b0, opcode: 0b11110,
		qAndSize: map[VectorArrangement]qAndSize{
			VectorArrangement4S: {size: 0b00, q: 0b1},
			VectorArrangement2S: {size: 0b00, q: 0b0},
			VectorArrangement2D: {size: 0b01, q: 0b1},
		},
	},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FMUL--vector---Floating-point-Multiply--vector--?lang=en
	VFMUL: {
		u: 0b1, opcode: 0b11011,
		qAndSize: map[VectorArrangement]qAndSize{
			VectorArrangement4S: {size: 0b00, q: 0b1},
			VectorArrangement2S: {size: 0b00, q: 0b0},
			VectorArrangement2D: {size: 0b01, q: 0b1},
		},
	},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FDIV--vector---Floating-point-Divide--vector--?lang=en
	VFDIV: {
		u: 0b1, opcode: 0b11111,
		qAndSize: map[VectorArrangement]qAndSize{
			VectorArrangement4S: {size: 0b00, q: 0b1},
			VectorArrangement2S: {size: 0b00, q: 0b0},
			VectorArrangement2D: {size: 0b01, q: 0b1},
		},
	},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/MUL--vector---Multiply--vector--?lang=en
	VMUL: {u: 0b0, opcode: 0b10011, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SQADD--Signed-saturating-Add-?lang=en
	VSQADD: {u: 0b0, opcode: 0b00001, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/UQADD--Unsigned-saturating-Add-?lang=en
	VUQADD: {u: 0b1, opcode: 0b00001, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SMIN--Signed-Minimum--vector--?lang=en
	SMIN: {u: 0b0, opcode: 0b01101, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SMAX--Signed-Maximum--vector--?lang=en
	SMAX: {u: 0b0, opcode: 0b01100, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/UMIN--Unsigned-Minimum--vector--?lang=en
	UMIN: {u: 0b1, opcode: 0b01101, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/UMAX--Unsigned-Maximum--vector--?lang=en
	UMAX: {u: 0b1, opcode: 0b01100, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/URHADD--Unsigned-Rounding-Halving-Add-?lang=en
	URHADD: {u: 0b1, opcode: 0b00010, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SQSUB--Signed-saturating-Subtract-?lang=en
	VSQSUB: {u: 0b0, opcode: 0b00101, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/UQSUB--Unsigned-saturating-Subtract-?lang=en
	VUQSUB: {u: 0b1, opcode: 0b00101, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/BIT--Bitwise-Insert-if-True-?lang=en
	VBIT: {u: 0b1, opcode: 0b00011, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement8B:  {q: 0b0, size: 0b10},
		VectorArrangement16B: {q: 0b1, size: 0b10},
	}},
	SQRDMULH: {u: 0b1, opcode: 0b10110, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement4H: {q: 0b0, size: 0b01},
		VectorArrangement8H: {q: 0b1, size: 0b01},
		VectorArrangement2S: {q: 0b0, size: 0b10},
		VectorArrangement4S: {q: 0b1, size: 0b10},
	}},
}

// aAndSize is a pair of "Q" and "size" that appear in https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en
type qAndSize struct{ q, size byte }

// defaultQAndSize maps a vector arrangement to the default qAndSize which is encoded by many instructions.
var defaultQAndSize = map[VectorArrangement]qAndSize{
	VectorArrangement8B:  {size: 0b00, q: 0b0},
	VectorArrangement16B: {size: 0b00, q: 0b1},
	VectorArrangement4H:  {size: 0b01, q: 0b0},
	VectorArrangement8H:  {size: 0b01, q: 0b1},
	VectorArrangement2S:  {size: 0b10, q: 0b0},
	VectorArrangement4S:  {size: 0b10, q: 0b1},
	VectorArrangement1D:  {size: 0b11, q: 0b0},
	VectorArrangement2D:  {size: 0b11, q: 0b1},
}

// advancedSIMDAcrossLanes holds information to encode instructions as "Advanced SIMD across lanes" in
// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en
var advancedSIMDAcrossLanes = map[asm.Instruction]struct {
	u, opcode byte
	qAndSize  map[VectorArrangement]qAndSize
}{
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/ADDV--Add-across-Vector-?lang=en
	ADDV: {
		u: 0b0, opcode: 0b11011,
		qAndSize: map[VectorArrangement]qAndSize{
			VectorArrangement16B: {size: 0b00, q: 0b1},
			VectorArrangement8B:  {size: 0b00, q: 0b0},
			VectorArrangement8H:  {size: 0b01, q: 0b1},
			VectorArrangement4H:  {size: 0b01, q: 0b0},
			VectorArrangement4S:  {size: 0b10, q: 0b1},
		},
	},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/UMINV--Unsigned-Minimum-across-Vector-?lang=en
	UMINV: {
		u: 0b1, opcode: 0b11010,
		qAndSize: map[VectorArrangement]qAndSize{
			VectorArrangement16B: {size: 0b00, q: 0b1},
			VectorArrangement8B:  {size: 0b00, q: 0b0},
			VectorArrangement8H:  {size: 0b01, q: 0b1},
			VectorArrangement4H:  {size: 0b01, q: 0b0},
			VectorArrangement4S:  {size: 0b10, q: 0b1},
		},
	},
	UADDLV: {u: 0b1, opcode: 0b00011, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement16B: {size: 0b00, q: 0b1},
		VectorArrangement8B:  {size: 0b00, q: 0b0},
		VectorArrangement8H:  {size: 0b01, q: 0b1},
		VectorArrangement4H:  {size: 0b01, q: 0b0},
		VectorArrangement4S:  {size: 0b10, q: 0b1},
	}},
}

// advancedSIMDScalarPairwise holds information to encode instructions as "Advanced SIMD scalar pairwise" in
// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en
var advancedSIMDScalarPairwise = map[asm.Instruction]struct {
	u, opcode byte
	size      map[VectorArrangement]byte
}{
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/ADDP--scalar---Add-Pair-of-elements--scalar--?lang=en
	ADDP: {u: 0b0, opcode: 0b11011, size: map[VectorArrangement]byte{VectorArrangement2D: 0b11}},
}

// advancedSIMDCopy holds information to encode instructions as "Advanced SIMD copy" in
// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en
var advancedSIMDCopy = map[asm.Instruction]struct {
	op byte
	// TODO: extract common implementation of resolver.
	resolver func(srcIndex, dstIndex VectorIndex, arr VectorArrangement) (imm5, imm4, q byte, err error)
}{
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/DUP--element---Duplicate-vector-element-to-vector-or-scalar-?lang=en
	DUPELEM: {op: 0, resolver: func(srcIndex, dstIndex VectorIndex, arr VectorArrangement) (imm5, imm4, q byte, err error) {
		imm4 = 0b0000
		q = 0b1

		switch arr {
		case VectorArrangementB:
			imm5 |= 0b1
			imm5 |= byte(srcIndex) << 1
		case VectorArrangementH:
			imm5 |= 0b10
			imm5 |= byte(srcIndex) << 2
		case VectorArrangementS:
			imm5 |= 0b100
			imm5 |= byte(srcIndex) << 3
		case VectorArrangementD:
			imm5 |= 0b1000
			imm5 |= byte(srcIndex) << 4
		default:
			err = fmt.Errorf("unsupported arrangement for DUPELEM: %d", arr)
		}

		return
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/DUP--general---Duplicate-general-purpose-register-to-vector-?lang=en
	DUPGEN: {op: 0b0, resolver: func(srcIndex, dstIndex VectorIndex, arr VectorArrangement) (imm5, imm4, q byte, err error) {
		imm4 = 0b0001
		switch arr {
		case VectorArrangement8B:
			imm5 = 0b1
		case VectorArrangement16B:
			imm5 = 0b1
			q = 0b1
		case VectorArrangement4H:
			imm5 = 0b10
		case VectorArrangement8H:
			imm5 = 0b10
			q = 0b1
		case VectorArrangement2S:
			imm5 = 0b100
		case VectorArrangement4S:
			imm5 = 0b100
			q = 0b1
		case VectorArrangement2D:
			imm5 = 0b1000
			q = 0b1
		default:
			err = fmt.Errorf("unsupported arrangement for DUPGEN: %s", arr)
		}
		return
	}},
	// https://developer.arm.com/documentation/ddi0596/2020-12/SIMD-FP-Instructions/INS--general---Insert-vector-element-from-general-purpose-register-?lang=en
	INSGEN: {op: 0b0, resolver: func(srcIndex, dstIndex VectorIndex, arr VectorArrangement) (imm5, imm4, q byte, err error) {
		imm4, q = 0b0011, 0b1
		switch arr {
		case VectorArrangementB:
			imm5 |= 0b1
			imm5 |= byte(dstIndex) << 1
		case VectorArrangementH:
			imm5 |= 0b10
			imm5 |= byte(dstIndex) << 2
		case VectorArrangementS:
			imm5 |= 0b100
			imm5 |= byte(dstIndex) << 3
		case VectorArrangementD:
			imm5 |= 0b1000
			imm5 |= byte(dstIndex) << 4
		default:
			err = fmt.Errorf("unsupported arrangement for INSGEN: %s", arr)
		}
		return
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/UMOV--Unsigned-Move-vector-element-to-general-purpose-register-?lang=en
	UMOV: {op: 0b0, resolver: func(srcIndex, dstIndex VectorIndex, arr VectorArrangement) (imm5, imm4, q byte, err error) {
		imm4 = 0b0111
		switch arr {
		case VectorArrangementB:
			imm5 |= 0b1
			imm5 |= byte(srcIndex) << 1
		case VectorArrangementH:
			imm5 |= 0b10
			imm5 |= byte(srcIndex) << 2
		case VectorArrangementS:
			imm5 |= 0b100
			imm5 |= byte(srcIndex) << 3
		case VectorArrangementD:
			imm5 |= 0b1000
			imm5 |= byte(srcIndex) << 4
			q = 0b1
		default:
			err = fmt.Errorf("unsupported arrangement for UMOV: %s", arr)
		}
		return
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SMOV--Signed-Move-vector-element-to-general-purpose-register-?lang=en
	SMOV32: {op: 0b0, resolver: func(srcIndex, dstIndex VectorIndex, arr VectorArrangement) (imm5, imm4, q byte, err error) {
		imm4 = 0b0101
		switch arr {
		case VectorArrangementB:
			imm5 |= 0b1
			imm5 |= byte(srcIndex) << 1
		case VectorArrangementH:
			imm5 |= 0b10
			imm5 |= byte(srcIndex) << 2
		default:
			err = fmt.Errorf("unsupported arrangement for SMOV32: %s", arr)
		}
		return
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/INS--element---Insert-vector-element-from-another-vector-element-?lang=en
	INSELEM: {op: 0b1, resolver: func(srcIndex, dstIndex VectorIndex, arr VectorArrangement) (imm5, imm4, q byte, err error) {
		q = 0b1
		switch arr {
		case VectorArrangementB:
			imm5 |= 0b1
			imm5 |= byte(dstIndex) << 1
			imm4 = byte(srcIndex)
		case VectorArrangementH:
			imm5 |= 0b10
			imm5 |= byte(dstIndex) << 2
			imm4 = byte(srcIndex) << 1
		case VectorArrangementS:
			imm5 |= 0b100
			imm5 |= byte(dstIndex) << 3
			imm4 = byte(srcIndex) << 2
		case VectorArrangementD:
			imm5 |= 0b1000
			imm5 |= byte(dstIndex) << 4
			imm4 = byte(srcIndex) << 3
		default:
			err = fmt.Errorf("unsupported arrangement for INSELEM: %d", arr)
		}
		return
	}},
}

// advancedSIMDTableLookup holds information to encode instructions as "Advanced SIMD table lookup" in
// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en
var advancedSIMDTableLookup = map[asm.Instruction]struct {
	op, op2, Len byte
	q            map[VectorArrangement]byte
}{
	TBL1: {op: 0, op2: 0, Len: 0b00, q: map[VectorArrangement]byte{VectorArrangement16B: 0b1, VectorArrangement8B: 0b0}},
	TBL2: {op: 0, op2: 0, Len: 0b01, q: map[VectorArrangement]byte{VectorArrangement16B: 0b1, VectorArrangement8B: 0b0}},
}

// advancedSIMDShiftByImmediate holds information to encode instructions as "Advanced SIMD shift by immediate" in
// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en
var advancedSIMDShiftByImmediate = map[asm.Instruction]struct {
	U, opcode   byte
	q           map[VectorArrangement]byte
	immResolver func(shiftAmount int64, arr VectorArrangement) (immh, immb byte, err error)
}{
	// https://developer.arm.com/documentation/ddi0596/2020-12/SIMD-FP-Instructions/SSHLL--SSHLL2--Signed-Shift-Left-Long--immediate--
	SSHLL: {
		U: 0b0, opcode: 0b10100,
		q:           map[VectorArrangement]byte{VectorArrangement8B: 0b0, VectorArrangement4H: 0b0, VectorArrangement2S: 0b0},
		immResolver: immResolverForSIMDSiftLeftByImmediate,
	},
	// https://developer.arm.com/documentation/ddi0596/2020-12/SIMD-FP-Instructions/SSHLL--SSHLL2--Signed-Shift-Left-Long--immediate--
	SSHLL2: {
		U: 0b0, opcode: 0b10100,
		q:           map[VectorArrangement]byte{VectorArrangement16B: 0b1, VectorArrangement8H: 0b1, VectorArrangement4S: 0b1},
		immResolver: immResolverForSIMDSiftLeftByImmediate,
	},
	// https://developer.arm.com/documentation/ddi0596/2020-12/SIMD-FP-Instructions/USHLL--USHLL2--Unsigned-Shift-Left-Long--immediate--
	USHLL: {
		U: 0b1, opcode: 0b10100,
		q:           map[VectorArrangement]byte{VectorArrangement8B: 0b0, VectorArrangement4H: 0b0, VectorArrangement2S: 0b0},
		immResolver: immResolverForSIMDSiftLeftByImmediate,
	},
	// https://developer.arm.com/documentation/ddi0596/2020-12/SIMD-FP-Instructions/USHLL--USHLL2--Unsigned-Shift-Left-Long--immediate--
	USHLL2: {
		U: 0b1, opcode: 0b10100,
		q:           map[VectorArrangement]byte{VectorArrangement16B: 0b1, VectorArrangement8H: 0b1, VectorArrangement4S: 0b1},
		immResolver: immResolverForSIMDSiftLeftByImmediate,
	},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SSHR--Signed-Shift-Right--immediate--?lang=en
	SSHR: {
		U: 0b0, opcode: 0b00000,
		q: map[VectorArrangement]byte{
			VectorArrangement16B: 0b1, VectorArrangement8H: 0b1, VectorArrangement4S: 0b1, VectorArrangement2D: 0b1,
			VectorArrangement8B: 0b0, VectorArrangement4H: 0b0, VectorArrangement2S: 0b0,
		},
		immResolver: func(shiftAmount int64, arr VectorArrangement) (immh, immb byte, err error) {
			switch arr {
			case VectorArrangement16B, VectorArrangement8B:
				immh = 0b0001
				immb = 8 - byte(shiftAmount&0b111)
			case VectorArrangement8H, VectorArrangement4H:
				v := 16 - byte(shiftAmount&0b1111)
				immb = v & 0b111
				immh = 0b0010 | (v >> 3)
			case VectorArrangement4S, VectorArrangement2S:
				v := 32 - byte(shiftAmount&0b11111)
				immb = v & 0b111
				immh = 0b0100 | (v >> 3)
			case VectorArrangement2D:
				v := 64 - byte(shiftAmount&0b111111)
				immb = v & 0b111
				immh = 0b1000 | (v >> 3)
			default:
				err = fmt.Errorf("unsupported arrangement %s", arr)
			}
			return
		},
	},
}

// advancedSIMDPermute holds information to encode instructions as "Advanced SIMD permute" in
// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en
var advancedSIMDPermute = map[asm.Instruction]struct {
	opcode byte
}{
	ZIP1: {opcode: 0b011},
}

func immResolverForSIMDSiftLeftByImmediate(shiftAmount int64, arr VectorArrangement) (immh, immb byte, err error) {
	switch arr {
	case VectorArrangement16B, VectorArrangement8B:
		immb = byte(shiftAmount)
		immh = 0b0001
	case VectorArrangement8H, VectorArrangement4H:
		immb = byte(shiftAmount) & 0b111
		immh = 0b0010 | byte(shiftAmount>>3)
	case VectorArrangement4S, VectorArrangement2S:
		immb = byte(shiftAmount) & 0b111
		immh = 0b0100 | byte(shiftAmount>>3)
	default:
		err = fmt.Errorf("unsupported arrangement %s", arr)
	}
	return
}

// encodeAdvancedSIMDCopy encodes instruction as "Advanced SIMD copy" in
// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en
func (a *AssemblerImpl) encodeAdvancedSIMDCopy(srcRegBits, dstRegBits, op, imm5, imm4, q byte) {
	a.buf.Write([]byte{
		(srcRegBits << 5) | dstRegBits,
		imm4<<3 | 0b1<<2 | srcRegBits>>3,
		imm5,
		q<<6 | op<<5 | 0b1110,
	})
}

// encodeAdvancedSIMDThreeSame encodes instruction as  "Advanced SIMD three same" in
// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en
func (a *AssemblerImpl) encodeAdvancedSIMDThreeSame(src1, src2, dst, opcode, size, q, u byte) {
	a.buf.Write([]byte{
		(src2 << 5) | dst,
		opcode<<3 | 1<<2 | src2>>3,
		size<<6 | 0b1<<5 | src1,
		q<<6 | u<<5 | 0b1110,
	})
}

// encodeAdvancedSIMDThreeDifferent encodes instruction as  "Advanced SIMD three different" in
// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en
func (a *AssemblerImpl) encodeAdvancedSIMDThreeDifferent(src1, src2, dst, opcode, size, q, u byte) {
	a.buf.Write([]byte{
		(src2 << 5) | dst,
		opcode<<4 | src2>>3,
		size<<6 | 0b1<<5 | src1,
		q<<6 | u<<5 | 0b1110,
	})
}

// encodeAdvancedSIMDPermute encodes instruction as  "Advanced SIMD permute" in
// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en
func (a *AssemblerImpl) encodeAdvancedSIMDPermute(src1, src2, dst, opcode, size, q byte) {
	a.buf.Write([]byte{
		(src2 << 5) | dst,
		opcode<<4 | 0b1<<3 | src2>>3,
		size<<6 | src1,
		q<<6 | 0b1110,
	})
}

func (a *AssemblerImpl) encodeVectorRegisterToVectorRegister(n *nodeImpl) (err error) {
	var srcVectorRegBits byte
	if n.srcReg != RegRZR {
		srcVectorRegBits, err = vectorRegisterBits(n.srcReg)
	} else if n.instruction == CMEQZERO {
		// CMEQZERO has RegRZR as the src, and we apply the instruction to the same register as the destination.
		srcVectorRegBits, err = vectorRegisterBits(n.dstReg)
	}

	if err != nil {
		return err
	}

	dstVectorRegBits, err := vectorRegisterBits(n.dstReg)
	if err != nil {
		return err
	}

	if simdCopy, ok := advancedSIMDCopy[n.instruction]; ok {
		imm5, imm4, q, err := simdCopy.resolver(n.srcVectorIndex, n.dstVectorIndex, n.vectorArrangement)
		if err != nil {
			return err
		}
		a.encodeAdvancedSIMDCopy(srcVectorRegBits, dstVectorRegBits, simdCopy.op, imm5, imm4, q)
		return nil
	}

	if scalarPairwise, ok := advancedSIMDScalarPairwise[n.instruction]; ok {
		// See "Advanced SIMD scalar pairwise" in
		// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en
		size, ok := scalarPairwise.size[n.vectorArrangement]
		if !ok {
			return fmt.Errorf("unsupported vector arrangement %s for %s", n.vectorArrangement, InstructionName(n.instruction))
		}
		a.buf.Write([]byte{
			(srcVectorRegBits << 5) | dstVectorRegBits,
			scalarPairwise.opcode<<4 | 1<<3 | srcVectorRegBits>>3,
			size<<6 | 0b11<<4 | scalarPairwise.opcode>>4,
			0b1<<6 | scalarPairwise.u<<5 | 0b11110,
		})
		return
	}

	if twoRegMisc, ok := advancedSIMDTwoRegisterMisc[n.instruction]; ok {
		// See "Advanced SIMD two-register miscellaneous" in
		// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en
		qs, ok := twoRegMisc.qAndSize[n.vectorArrangement]
		if !ok {
			return fmt.Errorf("unsupported vector arrangement %s for %s", n.vectorArrangement, InstructionName(n.instruction))
		}
		a.buf.Write([]byte{
			(srcVectorRegBits << 5) | dstVectorRegBits,
			twoRegMisc.opcode<<4 | 0b1<<3 | srcVectorRegBits>>3,
			qs.size<<6 | 0b1<<5 | twoRegMisc.opcode>>4,
			qs.q<<6 | twoRegMisc.u<<5 | 0b01110,
		})
		return nil
	}

	if threeSame, ok := advancedSIMDThreeSame[n.instruction]; ok {
		qs, ok := threeSame.qAndSize[n.vectorArrangement]
		if !ok {
			return fmt.Errorf("unsupported vector arrangement %s for %s", n.vectorArrangement, InstructionName(n.instruction))
		}
		a.encodeAdvancedSIMDThreeSame(srcVectorRegBits, dstVectorRegBits, dstVectorRegBits, threeSame.opcode, qs.size, qs.q, threeSame.u)
		return nil
	}

	if threeDifferent, ok := advancedSIMDThreeDifferent[n.instruction]; ok {
		qs, ok := threeDifferent.qAndSize[n.vectorArrangement]
		if !ok {
			return fmt.Errorf("unsupported vector arrangement %s for %s", n.vectorArrangement, InstructionName(n.instruction))
		}
		a.encodeAdvancedSIMDThreeDifferent(srcVectorRegBits, dstVectorRegBits, dstVectorRegBits, threeDifferent.opcode, qs.size, qs.q, threeDifferent.u)
		return nil
	}

	if acrossLanes, ok := advancedSIMDAcrossLanes[n.instruction]; ok {
		// See "Advanced SIMD across lanes" in
		// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en
		qs, ok := acrossLanes.qAndSize[n.vectorArrangement]
		if !ok {
			return fmt.Errorf("unsupported vector arrangement %s for %s", n.vectorArrangement, InstructionName(n.instruction))
		}
		a.buf.Write([]byte{
			(srcVectorRegBits << 5) | dstVectorRegBits,
			acrossLanes.opcode<<4 | 0b1<<3 | srcVectorRegBits>>3,
			qs.size<<6 | 0b11000<<1 | acrossLanes.opcode>>4,
			qs.q<<6 | acrossLanes.u<<5 | 0b01110,
		})
		return nil
	}

	if lookup, ok := advancedSIMDTableLookup[n.instruction]; ok {
		q, ok := lookup.q[n.vectorArrangement]
		if !ok {
			return fmt.Errorf("unsupported vector arrangement %s for %s", n.vectorArrangement, InstructionName(n.instruction))
		}
		a.buf.Write([]byte{
			(srcVectorRegBits << 5) | dstVectorRegBits,
			lookup.Len<<5 | lookup.op<<4 | srcVectorRegBits>>3,
			lookup.op2<<6 | dstVectorRegBits,
			q<<6 | 0b1110,
		})
		return
	}

	if shiftByImmediate, ok := advancedSIMDShiftByImmediate[n.instruction]; ok {
		immh, immb, err := shiftByImmediate.immResolver(n.srcConst, n.vectorArrangement)
		if err != nil {
			return err
		}

		q, ok := shiftByImmediate.q[n.vectorArrangement]
		if !ok {
			return fmt.Errorf("unsupported vector arrangement %s for %s", n.vectorArrangement, InstructionName(n.instruction))
		}

		a.buf.Write([]byte{
			(srcVectorRegBits << 5) | dstVectorRegBits,
			shiftByImmediate.opcode<<3 | 0b1<<2 | srcVectorRegBits>>3,
			immh<<3 | immb,
			q<<6 | shiftByImmediate.U<<5 | 0b1111,
		})
		return nil
	}

	if permute, ok := advancedSIMDPermute[n.instruction]; ok {
		size, q := arrangementSizeQ(n.vectorArrangement)
		a.encodeAdvancedSIMDPermute(srcVectorRegBits, dstVectorRegBits, dstVectorRegBits, permute.opcode, size, q)
		return
	}
	return errorEncodingUnsupported(n)
}

func (a *AssemblerImpl) encodeTwoVectorRegistersToVectorRegister(n *nodeImpl) (err error) {
	var srcRegBits, srcRegBits2, dstRegBits byte
	srcRegBits, err = vectorRegisterBits(n.srcReg)
	if err != nil {
		return err
	}

	srcRegBits2, err = vectorRegisterBits(n.srcReg2)
	if err != nil {
		return err
	}

	dstRegBits, err = vectorRegisterBits(n.dstReg)
	if err != nil {
		return err
	}

	if threeSame, ok := advancedSIMDThreeSame[n.instruction]; ok {
		qs, ok := threeSame.qAndSize[n.vectorArrangement]
		if !ok {
			return fmt.Errorf("unsupported vector arrangement %s for %s", n.vectorArrangement, InstructionName(n.instruction))
		}
		a.encodeAdvancedSIMDThreeSame(srcRegBits, srcRegBits2, dstRegBits, threeSame.opcode, qs.size, qs.q, threeSame.u)
		return nil
	}

	if threeDifferent, ok := advancedSIMDThreeDifferent[n.instruction]; ok {
		qs, ok := threeDifferent.qAndSize[n.vectorArrangement]
		if !ok {
			return fmt.Errorf("unsupported vector arrangement %s for %s", n.vectorArrangement, InstructionName(n.instruction))
		}
		a.encodeAdvancedSIMDThreeDifferent(srcRegBits, srcRegBits2, dstRegBits, threeDifferent.opcode, qs.size, qs.q, threeDifferent.u)
		return nil
	}

	if permute, ok := advancedSIMDPermute[n.instruction]; ok {
		size, q := arrangementSizeQ(n.vectorArrangement)
		a.encodeAdvancedSIMDPermute(srcRegBits, srcRegBits2, dstRegBits, permute.opcode, size, q)
		return
	}

	if n.instruction == EXT {
		// EXT is the only instruction in "Advanced SIMD extract", so inline the encoding here.
		// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/EXT--Extract-vector-from-pair-of-vectors-?lang=en
		var q, imm4 byte
		switch n.vectorArrangement {
		case VectorArrangement16B:
			imm4 = 0b1111 & byte(n.srcConst)
			q = 0b1
		case VectorArrangement8B:
			imm4 = 0b111 & byte(n.srcConst)
		default:
			return fmt.Errorf("invalid arrangement %s for EXT", n.vectorArrangement)
		}
		a.buf.Write([]byte{
			(srcRegBits2 << 5) | dstRegBits,
			imm4<<3 | srcRegBits2>>3,
			srcRegBits,
			q<<6 | 0b101110,
		})
		return
	}
	return
}

func (a *AssemblerImpl) encodeVectorRegisterToRegister(n *nodeImpl) (err error) {
	if err = checkArrangementIndexPair(n.vectorArrangement, n.srcVectorIndex); err != nil {
		return
	}

	srcVecRegBits, err := vectorRegisterBits(n.srcReg)
	if err != nil {
		return err
	}

	dstRegBits, err := intRegisterBits(n.dstReg)
	if err != nil {
		return err
	}

	if simdCopy, ok := advancedSIMDCopy[n.instruction]; ok {
		imm5, imm4, q, err := simdCopy.resolver(n.srcVectorIndex, n.dstVectorIndex, n.vectorArrangement)
		if err != nil {
			return err
		}
		a.encodeAdvancedSIMDCopy(srcVecRegBits, dstRegBits, simdCopy.op, imm5, imm4, q)
		return nil
	}
	return errorEncodingUnsupported(n)
}

func (a *AssemblerImpl) encodeRegisterToVectorRegister(n *nodeImpl) (err error) {
	srcRegBits, err := intRegisterBits(n.srcReg)
	if err != nil {
		return err
	}

	dstVectorRegBits, err := vectorRegisterBits(n.dstReg)
	if err != nil {
		return err
	}

	if simdCopy, ok := advancedSIMDCopy[n.instruction]; ok {
		imm5, imm4, q, err := simdCopy.resolver(n.srcVectorIndex, n.dstVectorIndex, n.vectorArrangement)
		if err != nil {
			return err
		}
		a.encodeAdvancedSIMDCopy(srcRegBits, dstVectorRegBits, simdCopy.op, imm5, imm4, q)
		return nil
	}
	return errorEncodingUnsupported(n)
}

var zeroRegisterBits byte = 0b11111

func isIntRegister(r asm.Register) bool {
	return RegR0 <= r && r <= RegSP
}

func isVectorRegister(r asm.Register) bool {
	return RegV0 <= r && r <= RegV31
}

func isConditionalRegister(r asm.Register) bool {
	return RegCondEQ <= r && r <= RegCondNV
}

func intRegisterBits(r asm.Register) (ret byte, err error) {
	if !isIntRegister(r) {
		err = fmt.Errorf("%s is not integer", RegisterName(r))
	} else if r == RegSP {
		// SP has the same bit representations as RegRZR.
		r = RegRZR
	}
	ret = byte(r - RegR0)
	return
}

func vectorRegisterBits(r asm.Register) (ret byte, err error) {
	if !isVectorRegister(r) {
		err = fmt.Errorf("%s is not vector", RegisterName(r))
	} else {
		ret = byte(r - RegV0)
	}
	return
}

func registerBits(r asm.Register) (ret byte) {
	if isIntRegister(r) {
		if r == RegSP {
			// SP has the same bit representations as RegRZR.
			r = RegRZR
		}
		ret = byte(r - RegR0)
	} else {
		ret = byte(r - RegV0)
	}
	return
}
