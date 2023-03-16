package arm64

import (
	"fmt"

	"github.com/tetratelabs/wazero/internal/asm"
)

// Arm64-specific register states.
//
// Note: Naming conventions intentionally match the Go assembler: https://go.dev/doc/asm
// See https://community.arm.com/arm-community-blogs/b/architectures-and-processors-blog/posts/condition-codes-1-condition-flags-and-codes
const (
	// CondEQ is the eq (equal) condition code
	CondEQ = asm.ConditionalRegisterStateUnset + 1 + iota
	// CondNE is the ne (not equal) condition code
	CondNE
	// CondHS is the hs (unsigned higher or same) condition code
	CondHS
	// CondLO is the lo (unsigned lower) condition code
	CondLO
	// CondMI is the mi (negative) condition code
	CondMI
	// CondPL is the pl (positive or zero) condition code
	CondPL
	// CondVS is the vs (signed overflow) condition code
	CondVS
	// CondVC is the vc (no signed overflow) condition code
	CondVC
	// CondHI is the hi (unsigned higher) condition code
	CondHI
	// CondLS is the ls (unsigned lower or same) condition code
	CondLS
	// CondGE is the ge (signed greater than or equal) condition code
	CondGE
	// CondLT is the lt (signed less than) condition code
	CondLT
	// CondGT is the gt (signed greater than) condition code
	CondGT
	// CondLE is the le (signed less than or equal) condition code
	CondLE
	// CondAL is the al (always executed) condition code
	CondAL
	// CondNV has the same meaning as CondAL
	CondNV
)

// Arm64-specific registers.
//
// Note: Naming conventions intentionally match the Go assembler: https://go.dev/doc/asm
// See https://developer.arm.com/documentation/dui0801/a/Overview-of-AArch64-state/Predeclared-core-register-names-in-AArch64-state
const (
	// Integer registers.

	// RegR0 is the R0 register
	RegR0 asm.Register = asm.NilRegister + 1 + iota
	// RegR1 is the R1 register
	RegR1
	// RegR2 is the R2 register
	RegR2
	// RegR3 is the R3 register
	RegR3
	// RegR4 is the R4 register
	RegR4
	// RegR5 is the R5 register
	RegR5
	// RegR6 is the R6 register
	RegR6
	// RegR7 is the R7 register
	RegR7
	// RegR8 is the R8 register
	RegR8
	// RegR9 is the R9 register
	RegR9
	// RegR10 is the R10 register
	RegR10
	// RegR11 is the R11 register
	RegR11
	// RegR12 is the R12 register
	RegR12
	// RegR13 is the R13 register
	RegR13
	// RegR14 is the R14 register
	RegR14
	// RegR15 is the R15 register
	RegR15
	// RegR16 is the R16 register
	RegR16
	// RegR17 is the R17 register
	RegR17
	// RegR18 is the R18 register
	RegR18
	// RegR19 is the R19 register
	RegR19
	// RegR20 is the R20 register
	RegR20
	// RegR21 is the R21 register
	RegR21
	// RegR22 is the R22 register
	RegR22
	// RegR23 is the R23 register
	RegR23
	// RegR24 is the R24 register
	RegR24
	// RegR25 is the R25 register
	RegR25
	// RegR26 is the R26 register
	RegR26
	// RegR27 is the R27 register
	RegR27
	// RegR28 is the R28 register
	RegR28
	// RegR29 is the R29 register
	RegR29
	// RegR30 is the R30 register
	RegR30
	// RegRZR is the RZR register (read-only, always returning zero)
	RegRZR
	// RegSP is the SP register
	RegSP

	// Scalar floating point registers.

	// RegV0 is the V0 register
	RegV0
	// RegV1 is the V1 register
	RegV1
	// RegV2 is the V2 register
	RegV2
	// RegV3 is the V3 register
	RegV3
	// RegV4 is the V4 register
	RegV4
	// RegV5 is the V5 register
	RegV5
	// RegV6 is the V6 register
	RegV6
	// RegV7 is the V7 register
	RegV7
	// RegV8 is the V8 register
	RegV8
	// RegV9 is the V9 register
	RegV9
	// RegV10 is the V10 register
	RegV10
	// RegV11 is the V11 register
	RegV11
	// RegV12 is the V12 register
	RegV12
	// RegV13 is the V13 register
	RegV13
	// RegV14 is the V14 register
	RegV14
	// RegV15 is the V15 register
	RegV15
	// RegV16 is the V16 register
	RegV16
	// RegV17 is the V17 register
	RegV17
	// RegV18 is the V18 register
	RegV18
	// RegV19 is the V19 register
	RegV19
	// RegV20 is the V20 register
	RegV20
	// RegV21 is the V21 register
	RegV21
	// RegV22 is the V22 register
	RegV22
	// RegV23 is the V23 register
	RegV23
	// RegV24 is the V24 register
	RegV24
	// RegV25 is the V25 register
	RegV25
	// RegV26 is the V26 register
	RegV26
	// RegV27 is the V27 register
	RegV27
	// RegV28 is the V28 register
	RegV28
	// RegV29 is the V29 register
	RegV29
	// RegV30 is the V30 register
	RegV30
	// RegV31 is the V31 register
	RegV31

	// Floating point status register.

	// RegFPSR is the FPSR register
	RegFPSR

	// Assign each conditional register state to the unique register ID.
	// This is to reduce the size of nodeImpl struct without having dedicated field
	// for conditional register state which would not be used by most nodes.
	// This is taking advantage of the fact that conditional operations are always
	// on a single register and condition code, and never two registers.

	// RegCondEQ encodes CondEQ into a field that would otherwise store a register
	RegCondEQ
	// RegCondNE encodes CondNE into a field that would otherwise store a register
	RegCondNE
	// RegCondHS encodes CondHS into a field that would otherwise store a register
	RegCondHS
	// RegCondLO encodes CondLO into a field that would otherwise store a register
	RegCondLO
	// RegCondMI encodes CondMI into a field that would otherwise store a register
	RegCondMI
	// RegCondPL encodes CondPL into a field that would otherwise store a register
	RegCondPL
	// RegCondVS encodes CondVS into a field that would otherwise store a register
	RegCondVS
	// RegCondVC encodes CondVC into a field that would otherwise store a register
	RegCondVC
	// RegCondHI encodes CondHI into a field that would otherwise store a register
	RegCondHI
	// RegCondLS encodes CondLS into a field that would otherwise store a register
	RegCondLS
	// RegCondGE encodes CondGE into a field that would otherwise store a register
	RegCondGE
	// RegCondLT encodes CondLT into a field that would otherwise store a register
	RegCondLT
	// RegCondGT encodes CondGT into a field that would otherwise store a register
	RegCondGT
	// RegCondLE encodes CondLE into a field that would otherwise store a register
	RegCondLE
	// RegCondAL encodes CondAL into a field that would otherwise store a register
	RegCondAL
	// RegCondNV encodes CondNV into a field that would otherwise store a register
	RegCondNV
)

// conditionalRegisterStateToRegister cast a conditional register to its unique register ID.
// See the comment on RegCondEQ above.
func conditionalRegisterStateToRegister(c asm.ConditionalRegisterState) asm.Register {
	switch c {
	case CondEQ:
		return RegCondEQ
	case CondNE:
		return RegCondNE
	case CondHS:
		return RegCondHS
	case CondLO:
		return RegCondLO
	case CondMI:
		return RegCondMI
	case CondPL:
		return RegCondPL
	case CondVS:
		return RegCondVS
	case CondVC:
		return RegCondVC
	case CondHI:
		return RegCondHI
	case CondLS:
		return RegCondLS
	case CondGE:
		return RegCondGE
	case CondLT:
		return RegCondLT
	case CondGT:
		return RegCondGT
	case CondLE:
		return RegCondLE
	case CondAL:
		return RegCondAL
	case CondNV:
		return RegCondNV
	}
	return asm.NilRegister
}

// RegisterName returns the name of a given register
func RegisterName(r asm.Register) string {
	switch r {
	case asm.NilRegister:
		return "nil"
	case RegR0:
		return "R0"
	case RegR1:
		return "R1"
	case RegR2:
		return "R2"
	case RegR3:
		return "R3"
	case RegR4:
		return "R4"
	case RegR5:
		return "R5"
	case RegR6:
		return "R6"
	case RegR7:
		return "R7"
	case RegR8:
		return "R8"
	case RegR9:
		return "R9"
	case RegR10:
		return "R10"
	case RegR11:
		return "R11"
	case RegR12:
		return "R12"
	case RegR13:
		return "R13"
	case RegR14:
		return "R14"
	case RegR15:
		return "R15"
	case RegR16:
		return "R16"
	case RegR17:
		return "R17"
	case RegR18:
		return "R18"
	case RegR19:
		return "R19"
	case RegR20:
		return "R20"
	case RegR21:
		return "R21"
	case RegR22:
		return "R22"
	case RegR23:
		return "R23"
	case RegR24:
		return "R24"
	case RegR25:
		return "R25"
	case RegR26:
		return "R26"
	case RegR27:
		return "R27"
	case RegR28:
		return "R28"
	case RegR29:
		return "R29"
	case RegR30:
		return "R30"
	case RegRZR:
		return "RZR"
	case RegSP:
		return "SP"
	case RegV0:
		return "V0"
	case RegV1:
		return "V1"
	case RegV2:
		return "V2"
	case RegV3:
		return "V3"
	case RegV4:
		return "V4"
	case RegV5:
		return "V5"
	case RegV6:
		return "V6"
	case RegV7:
		return "V7"
	case RegV8:
		return "V8"
	case RegV9:
		return "V9"
	case RegV10:
		return "V10"
	case RegV11:
		return "V11"
	case RegV12:
		return "V12"
	case RegV13:
		return "V13"
	case RegV14:
		return "V14"
	case RegV15:
		return "V15"
	case RegV16:
		return "V16"
	case RegV17:
		return "V17"
	case RegV18:
		return "V18"
	case RegV19:
		return "V19"
	case RegV20:
		return "V20"
	case RegV21:
		return "V21"
	case RegV22:
		return "V22"
	case RegV23:
		return "V23"
	case RegV24:
		return "V24"
	case RegV25:
		return "V25"
	case RegV26:
		return "V26"
	case RegV27:
		return "V27"
	case RegV28:
		return "V28"
	case RegV29:
		return "V29"
	case RegV30:
		return "V30"
	case RegV31:
		return "V31"
	case RegFPSR:
		return "FPSR"
	case RegCondEQ:
		return "COND_EQ"
	case RegCondNE:
		return "COND_NE"
	case RegCondHS:
		return "COND_HS"
	case RegCondLO:
		return "COND_LO"
	case RegCondMI:
		return "COND_MI"
	case RegCondPL:
		return "COND_PL"
	case RegCondVS:
		return "COND_VS"
	case RegCondVC:
		return "COND_VC"
	case RegCondHI:
		return "COND_HI"
	case RegCondLS:
		return "COND_LS"
	case RegCondGE:
		return "COND_GE"
	case RegCondLT:
		return "COND_LT"
	case RegCondGT:
		return "COND_GT"
	case RegCondLE:
		return "COND_LE"
	case RegCondAL:
		return "COND_AL"
	case RegCondNV:
		return "COND_NV"
	}
	return "UNKNOWN"
}

// Arm64-specific instructions.
//
// Note: This only defines arm64 instructions used by wazero's compiler.
// Note: Naming conventions partially match the Go assembler: https://go.dev/doc/asm
const (
	// NOP is the NOP instruction. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/NOP
	NOP asm.Instruction = iota
	// RET is the RET instruction. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/RET
	RET
	// ADD is the ADD instruction. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/ADD--shifted-register-
	ADD
	// ADDS is the ADDS instruction. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/ADDS--shifted-register-
	ADDS
	// ADDW is the ADD instruction, in 64-bit mode. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/ADD--shifted-register-
	ADDW
	// ADR is the ADR instruction. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/ADR
	ADR
	// AND is the AND instruction. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/AND--shifted-register-
	AND
	// ANDIMM32 is the AND(immediate) instruction in 32-bit mode https://developer.arm.com/documentation/ddi0596/2021-12/Base-Instructions/AND--immediate---Bitwise-AND--immediate--?lang=en
	ANDIMM32
	// ANDIMM64 is the AND(immediate) instruction in 64-bit mode https://developer.arm.com/documentation/ddi0596/2021-12/Base-Instructions/AND--immediate---Bitwise-AND--immediate--?lang=en
	ANDIMM64
	// ANDW is the AND instruction, in 64-bit mode. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/AND--register-
	ANDW
	// ASR is the ASR instruction. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/ASR--register-
	ASR
	// ASRW is the ASR instruction, in 64-bit mode. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/ASR--register-
	ASRW
	// B is the B instruction. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/B
	B

	// Below are B.cond instructions.
	// 	* https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/B-cond
	// 	* https://developer.arm.com/documentation/dui0802/a/A32-and-T32-Instructions/Condition-codes

	// BCONDEQ is the B.cond instruction with CondEQ.
	BCONDEQ
	// BCONDGE is the B.cond instruction with CondGE.
	BCONDGE
	// BCONDGT is the B.cond instruction with CondGT.
	BCONDGT
	// BCONDHI is the B.cond instruction with CondHI.
	BCONDHI
	// BCONDHS is the B.cond instruction with CondHS.
	BCONDHS
	// BCONDLE is the B.cond instruction with CondLE.
	BCONDLE
	// BCONDLO is the B.cond instruction with CondLO.
	BCONDLO
	// BCONDLS is the B.cond instruction with CondLS.
	BCONDLS
	// BCONDLT is the B.cond instruction with CondLT.
	BCONDLT
	// BCONDMI is the B.cond instruction with CondMI.
	BCONDMI
	// BCONDPL is the B.cond instruction with CondPL.
	BCONDPL
	// BCONDNE is the B.cond instruction with CondNE.
	BCONDNE
	// BCONDVS is the B.cond instruction with CondVS.
	BCONDVS

	// CLZ is the CLZ instruction. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/CLZ
	CLZ
	// CLZW is the CLZ instruction, in 64-bit mode. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/CLZ
	CLZW
	// CMP is the CMP instruction. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/CMP--shifted-register-
	CMP
	// CMPW is the CMP instruction, in 64-bit mode. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/CMP--shifted-register-
	CMPW
	// CSET is the CSET instruction. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/CSET
	CSET
	// EOR is the EOR instruction. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/EOR--shifted-register-
	EOR
	// EORW is the EOR instruction, in 64-bit mode. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/EOR--shifted-register-
	EORW
	// FABSD is the FABS instruction, for double-precision. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/FABS--scalar-
	FABSD
	// FABSS is the FABS instruction, for single-precision. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/FABS--scalar-
	FABSS
	// FADDD is the FADD instruction, for double-precision. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/FADD--scalar-
	FADDD
	// FADDS is the FADD instruction, for single-precision. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/FADD--scalar-
	FADDS
	// FCMPD is the FCMP instruction, for double-precision. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/FCMP
	FCMPD
	// FCMPS is the FCMP instruction, for single-precision. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/FCMP
	FCMPS
	// FCVTDS is the FCVT instruction, for single to double precision. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/FCVT
	FCVTDS
	// FCVTSD is the FCVT instruction, for double to single precision. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/FCVT
	FCVTSD
	// FCVTZSD is the FCVTZS instruction, for double precision. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/FCVTZS--scalar--integer-
	FCVTZSD
	// FCVTZSDW is the FCVTZS instruction, for double precision in 64-bit mode. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/FCVTZS--scalar--integer-
	FCVTZSDW
	// FCVTZSS is the FCVTZS instruction, for single precision. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/FCVTZS--scalar--integer-
	FCVTZSS
	// FCVTZSSW is the FCVTZS instruction, for single precision in 64-bit mode. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/FCVTZS--scalar--integer-
	FCVTZSSW
	// FCVTZUD is the FCVTZU instruction, for double precision. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/FCVTZU--scalar--integer-
	FCVTZUD
	// FCVTZUDW is the FCVTZU instruction, for double precision in 64-bit mode. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/FCVTZU--scalar--integer-
	FCVTZUDW
	// FCVTZUS is the FCVTZU instruction, for single precision. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/FCVTZU--scalar--integer-
	FCVTZUS
	// FCVTZUSW is the FCVTZU instruction, for single precision in 64-bit mode. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/FCVTZU--scalar--integer-
	FCVTZUSW
	// FDIVD is the FDIV instruction, for double precision. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/FDIV--scalar-
	FDIVD
	// FDIVS is the FDIV instruction, for single precision. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/FDIV--scalar-
	FDIVS
	// FMAXD is the FMAX instruction, for double precision. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/FMAX--scalar-
	FMAXD
	// FMAXS is the FMAX instruction, for single precision. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/FMAX--scalar-
	FMAXS
	// FMIND is the FMIN instruction, for double precision. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/FMIN--scalar-
	FMIND
	// FMINS is the FMIN instruction, for single precision. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/FMIN--scalar-
	FMINS
	// FMOVD is the FMOV instruction, for double precision. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/FMOV--register-
	FMOVD
	// FMOVS is the FMOV instruction, for single precision. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/FMOV--register-
	FMOVS
	// FMULD is the FMUL instruction, for double precision. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/FMUL--scalar-
	FMULD
	// FMULS is the FMUL instruction, for single precision. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/FMUL--scalar-
	FMULS
	// FNEGD is the FNEG instruction, for double precision. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/FNEG--scalar-
	FNEGD
	// FNEGS is the FNEG instruction, for single precision. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/FNEG--scalar-
	FNEGS
	// FRINTMD is the FRINTM instruction, for double precision. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/FRINTM--scalar-
	FRINTMD
	// FRINTMS is the FRINTM instruction, for single precision. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/FRINTM--scalar-
	FRINTMS
	// FRINTND is the FRINTN instruction, for double precision. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/FRINTN--scalar-
	FRINTND
	// FRINTNS is the FRINTN instruction, for single precision. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/FRINTN--scalar-
	FRINTNS
	// FRINTPD is the FRINTP instruction, for double precision. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/FRINTP--scalar-
	FRINTPD
	// FRINTPS is the FRINTP instruction, for single precision. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/FRINTP--scalar-
	FRINTPS
	// FRINTZD is the FRINTZ instruction, for double precision. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/FRINTZ--scalar-
	FRINTZD
	// FRINTZS is the FRINTZ instruction, for single precision. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/FRINTZ--scalar-
	FRINTZS
	// FSQRTD is the FSQRT instruction, for double precision. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/FSQRT--scalar-
	FSQRTD
	// FSQRTS is the FSQRT instruction, for single precision. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/FSQRT--scalar-
	FSQRTS
	// FSUBD is the FSUB instruction, for double precision. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/FSUB--scalar-
	FSUBD
	// FSUBS is the FSUB instruction, for single precision. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/FSUB--scalar-
	FSUBS
	// LSL is the LSL instruction. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/LSL--register-
	LSL
	// LSLW is the LSL instruction, in 64-bit mode. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/LSL--register-
	LSLW
	// LSR is the LSR instruction. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/LSR--register-
	LSR
	// LSRW is the LSR instruction, in 64-bit mode. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/LSR--register-
	LSRW
	// FLDRD is the LDR (SIMD&FP) instruction for double precisions. https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/LDR--register--SIMD-FP---Load-SIMD-FP-Register--register-offset--?lang=en
	FLDRD
	// FLDRS is the LDR (SIMD&FP) instruction for single precisions. https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/LDR--register--SIMD-FP---Load-SIMD-FP-Register--register-offset--?lang=en
	FLDRS
	// LDRD is the LDR instruction in 64-bit mode. https://developer.arm.com/documentation/ddi0596/2021-12/Base-Instructions/LDR--register---Load-Register--register--?lang=en
	LDRD
	// LDRW is the LDR instruction in 32-bit mode. https://developer.arm.com/documentation/ddi0596/2021-12/Base-Instructions/LDR--register---Load-Register--register--?lang=en
	LDRW
	// LDRSBD is the LDRSB instruction in 64-bit mode. https://developer.arm.com/documentation/dui0802/a/A64-Data-Transfer-Instructions/LDRSB--register-
	LDRSBD
	// LDRSBW is the LDRSB instruction in 32-bit mode. https://developer.arm.com/documentation/dui0802/a/A64-Data-Transfer-Instructions/LDRSB--register-
	LDRSBW
	// LDRB is the LDRB instruction. https://developer.arm.com/documentation/dui0802/a/A64-Data-Transfer-Instructions/LDRB--register-
	LDRB
	// LDRSHD is the LDRSHW instruction in 64-bit mode. https://developer.arm.com/documentation/dui0802/a/A64-Data-Transfer-Instructions/LDRSH--register-
	LDRSHD
	// LDRSHW is the LDRSHW instruction in 32-bit mode. https://developer.arm.com/documentation/dui0802/a/A64-Data-Transfer-Instructions/LDRSH--register-
	LDRSHW
	// LDRH is the LDRH instruction. https://developer.arm.com/documentation/dui0802/a/A64-Data-Transfer-Instructions/LDRH--register-
	LDRH
	// LDRSW is the LDRSW instruction https://developer.arm.com/documentation/dui0802/a/A64-Data-Transfer-Instructions/LDRSW--register-
	LDRSW
	// FSTRD is the STR (SIMD&FP) instruction for double precisions. https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/STR--immediate--SIMD-FP---Store-SIMD-FP-register--immediate-offset--?lang=en
	FSTRD
	// FSTRS is the STR (SIMD&FP) instruction for single precisions. https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/STR--immediate--SIMD-FP---Store-SIMD-FP-register--immediate-offset--?lang=en
	FSTRS
	// STRD is the STR instruction in 64-bit mode. https://developer.arm.com/documentation/ddi0596/2021-12/Base-Instructions/STR--register---Store-Register--register--?lang=en
	STRD
	// STRW is the STR instruction in 32-bit mode. https://developer.arm.com/documentation/ddi0596/2021-12/Base-Instructions/STR--register---Store-Register--register--?lang=en
	STRW
	// STRH is the STRH instruction. https://developer.arm.com/documentation/ddi0596/2021-12/Base-Instructions/STRH--register---Store-Register-Halfword--register--?lang=en
	STRH
	// STRB is the STRB instruction. https://developer.arm.com/documentation/ddi0596/2021-12/Base-Instructions/STRB--register---Store-Register-Byte--register--?lang=en
	STRB
	// MOVD moves a double word from register to register, or const to register.
	MOVD
	// MOVW moves a word from register to register, or const to register.
	MOVW
	// MRS is the MRS instruction. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/MRS
	MRS
	// MSR is the MSR instruction. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/MSR--register-
	MSR
	// MSUB is the MSUB instruction. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/MSUB
	MSUB
	// MSUBW is the MSUB instruction, in 64-bit mode. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/MSUB
	MSUBW
	// MUL is the MUL instruction. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/MUL
	MUL
	// MULW is the MUL instruction, in 32-bit mode. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/MUL
	MULW
	// NEG is the NEG instruction. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/NEG
	NEG
	// NEGW is the NEG instruction, in 32-bit mode. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/NEG
	NEGW
	// ORR is the ORR instruction. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/ORR--shifted-register-
	ORR
	// ORRW is the ORR instruction, in 64-bit mode. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/ORR--shifted-register-
	ORRW
	// RBIT is the RBIT instruction. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/RBIT
	RBIT
	// RBITW is the RBIT instruction, in 64-bit mode. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/RBIT
	RBITW
	// ROR is the ROR instruction. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/ROR--register-
	ROR
	// RORW is the RORW instruction, in 64-bit mode. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/ROR--register-
	RORW
	// SCVTFD is the SCVTF instruction, for double precision. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/SCVTF--scalar--integer-
	SCVTFD
	// SCVTFS is the SCVTF instruction, for single precision. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/SCVTF--scalar--integer-
	SCVTFS
	// SCVTFWD is the SCVTF instruction, for double precision in 64-bit mode. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/SCVTF--scalar--integer-
	SCVTFWD
	// SCVTFWS is the SCVTF instruction, for single precision in 64-bit mode. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/SCVTF--scalar--integer-
	SCVTFWS
	// SDIV is the SDIV instruction. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/SDIV
	SDIV
	// SDIVW is the SDIV instruction. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/SDIV
	SDIVW
	// SUB is the SUB instruction. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/SUB--shifted-register-
	SUB
	// SUBS is the SUBS instruction. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/SUBS--shifted-register-
	SUBS
	// SUBW is the SUB instruction, in 64-bit mode. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/SUB--shifted-register-
	SUBW
	// SXTB is the SXTB instruction. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/SXTB
	SXTB
	// SXTBW is the SXTB instruction, in 64-bit mode. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/SXTB
	SXTBW
	// SXTH is the SXTH instruction. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/SXTH
	SXTH
	// SXTHW is the SXTH instruction, in 64-bit mode. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/SXTH
	SXTHW
	// SXTW is the SXTW instruction. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/SXTW
	SXTW
	// UCVTFD is the UCVTF instruction, for double precision. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/UCVTF--scalar--integer-
	UCVTFD
	// UCVTFS is the UCVTF instruction, for single precision. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/UCVTF--scalar--integer-
	UCVTFS
	// UCVTFWD is the UCVTF instruction, for double precision in 64-bit mode. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/UCVTF--scalar--integer-
	UCVTFWD
	// UCVTFWS is the UCVTF instruction, for single precision in 64-bit mode. https://developer.arm.com/documentation/dui0802/a/A64-Floating-point-Instructions/UCVTF--scalar--integer-
	UCVTFWS
	// UDIV is the UDIV instruction. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/UDIV
	UDIV
	// UDIVW is the UDIV instruction, in 64-bit mode. https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/UDIV
	UDIVW
	// VBIT is the BIT instruction. https://developer.arm.com/documentation/dui0802/a/A64-Advanced-SIMD-Vector-Instructions/BIT--vector-
	VBIT
	// VCNT is the CNT instruction. https://developer.arm.com/documentation/dui0802/a/A64-Advanced-SIMD-Vector-Instructions/CNT--vector-
	VCNT
	// VMOV has different semantics depending on the types of operands:
	//   - LDR(SIMD&FP) if the src is memory and dst is a vector: https://developer.arm.com/documentation/ddi0596/2020-12/SIMD-FP-Instructions/LDR--immediate--SIMD-FP---Load-SIMD-FP-Register--immediate-offset--
	//   - LDR(literal, SIMD&FP) if the src is static const and dst is a vector: https://developer.arm.com/documentation/dui0801/h/A64-Floating-point-Instructions/LDR--literal--SIMD-and-FP-
	//   - STR(SIMD&FP) if the dst is memory and src is a vector: https://developer.arm.com/documentation/ddi0596/2020-12/SIMD-FP-Instructions/STR--immediate--SIMD-FP---Store-SIMD-FP-register--immediate-offset--
	VMOV
	// UMOV is the UMOV instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/UMOV--Unsigned-Move-vector-element-to-general-purpose-register-?lang=en
	UMOV
	// INSGEN is the INS(general) instruction https://developer.arm.com/documentation/ddi0596/2020-12/SIMD-FP-Instructions/INS--general---Insert-vector-element-from-general-purpose-register-?lang=en
	INSGEN
	// INSELEM is the INS(element) instruction https://developer.arm.com/documentation/ddi0596/2020-12/SIMD-FP-Instructions/INS--element---Insert-vector-element-from-another-vector-element-?lang=en
	INSELEM
	// UADDLV is the UADDLV(vector) instruction. https://developer.arm.com/documentation/dui0802/a/A64-Advanced-SIMD-Vector-Instructions/UADDLV--vector-
	UADDLV
	// VADD is the ADD(vector) instruction. https://developer.arm.com/documentation/dui0802/a/A64-Advanced-SIMD-Vector-Instructions/ADD--vector-
	VADD
	// VFADDS is the FADD(vector) instruction, for single precision. https://developer.arm.com/documentation/dui0802/a/A64-Advanced-SIMD-Vector-Instructions/FADD--vector-
	VFADDS
	// VFADDD is the FADD(vector) instruction, for double precision. https://developer.arm.com/documentation/dui0802/a/A64-Advanced-SIMD-Vector-Instructions/FADD--vector-
	VFADDD
	// VSUB is the SUB(vector) instruction.  https://developer.arm.com/documentation/dui0802/a/A64-Advanced-SIMD-Vector-Instructions/SUB--vector-
	VSUB
	// VFSUBS is the FSUB(vector) instruction, for single precision. https://developer.arm.com/documentation/dui0802/a/A64-Advanced-SIMD-Vector-Instructions/FSUB--vector-
	VFSUBS
	// VFSUBD is the FSUB(vector) instruction, for double precision. https://developer.arm.com/documentation/dui0802/a/A64-Advanced-SIMD-Vector-Instructions/FSUB--vector-
	VFSUBD
	// SSHL is the SSHL(vector,register) instruction. https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SSHL--Signed-Shift-Left--register--?lang=en
	SSHL
	// SSHLL is the SSHLL(vector,immediate) instruction. https://developer.arm.com/documentation/dui0801/h/A64-SIMD-Vector-Instructions/SSHLL--SSHLL2--vector-
	SSHLL
	// USHL is the USHL(vector,register) instruction. https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SSHL--Signed-Shift-Left--register--?lang=en
	USHL
	// USHLL is the USHLL(vector,immediate) instruction. https://developer.arm.com/documentation/dui0801/h/A64-SIMD-Vector-Instructions/SSHLL--SSHLL2--vector-
	USHLL
	// LD1R is the LD1R instruction. https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/LD1R--Load-one-single-element-structure-and-Replicate-to-all-lanes--of-one-register--
	LD1R
	// SMOV32 is the 32-bit variant of SMOV(vector) instruction. https://developer.arm.com/documentation/100069/0610/A64-SIMD-Vector-Instructions/SMOV--vector-
	SMOV32
	// DUPGEN is the DUP(general) instruction. https://developer.arm.com/documentation/ddi0596/2020-12/SIMD-FP-Instructions/DUP--general---Duplicate-general-purpose-register-to-vector-
	DUPGEN
	// DUPELEM is the DUP(element) instruction. https://developer.arm.com/documentation/ddi0596/2020-12/SIMD-FP-Instructions/DUP--element---Duplicate-vector-element-to-vector-or-scalar-
	DUPELEM
	// UMAXP is the UMAXP(vector) instruction. https://developer.arm.com/documentation/dui0801/g/A64-SIMD-Vector-Instructions/UMAXP--vector-
	UMAXP
	// UMINV is the UMINV(vector) instruction. https://developer.arm.com/documentation/100069/0610/A64-SIMD-Vector-Instructions/UMINV--vector-
	UMINV
	// CMEQ is the CMEQ(vector, register) instruction. https://developer.arm.com/documentation/dui0801/g/A64-SIMD-Vector-Instructions/CMEQ--vector--register-
	CMEQ
	// CMEQZERO is the CMEP(zero) instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/CMEQ--zero---Compare-bitwise-Equal-to-zero--vector--?lang=en
	CMEQZERO
	// ADDP is the ADDP(scalar) instruction. https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/ADDP--scalar---Add-Pair-of-elements--scalar--?lang=en
	ADDP
	// VADDP is the ADDP(vector) instruction. https://developer.arm.com/documentation/dui0801/g/A64-SIMD-Vector-Instructions/ADDP--vector-
	// Note: prefixed by V to distinguish from the non-vector variant of ADDP(scalar).
	VADDP
	// TBL1 is the TBL instruction whose source is one vector. https://developer.arm.com/documentation/ddi0596/2020-12/SIMD-FP-Instructions/TBL--Table-vector-Lookup-
	TBL1
	// TBL2 is the TBL instruction whose source is two vectors. https://developer.arm.com/documentation/ddi0596/2020-12/SIMD-FP-Instructions/TBL--Table-vector-Lookup-
	TBL2
	// NOT is the NOT(vector) instruction https://developer.arm.com/documentation/ddi0596/2020-12/SIMD-FP-Instructions/NOT--Bitwise-NOT--vector--?lang=en
	NOT
	// VAND is the AND(vector) instruction https://developer.arm.com/documentation/ddi0596/2020-12/SIMD-FP-Instructions/AND--vector---Bitwise-AND--vector--
	// Note: prefixed by V to distinguish from the non-vector variant of AND.
	VAND
	// VORR is the ORR(vector) instruction https://developer.arm.com/documentation/ddi0596/2020-12/SIMD-FP-Instructions/ORR--vector--register---Bitwise-inclusive-OR--vector--register--
	// Note: prefixed by V to distinguish from the non-vector variant of ORR.
	VORR
	// BSL https://developer.arm.com/documentation/ddi0596/2020-12/SIMD-FP-Instructions/BSL--Bitwise-Select-
	BSL
	// BIC is the BIC(vector) instruction https://developer.arm.com/documentation/ddi0596/2020-12/SIMD-FP-Instructions/BIC--vector--register---Bitwise-bit-Clear--vector--register--
	BIC
	// VFNEG is the FNEG(vector) instruction https://developer.arm.com/documentation/ddi0596/2020-12/SIMD-FP-Instructions/FNEG--vector---Floating-point-Negate--vector--
	// Note: prefixed by V to distinguish from the non-vector variant of FNEG.
	VFNEG
	// ADDV is the ADDV instruction https://developer.arm.com/documentation/ddi0596/2020-12/SIMD-FP-Instructions/ADDV--Add-across-Vector-
	ADDV
	// ZIP1 is the ZIP1 instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/ZIP1--Zip-vectors--primary--?lang=en
	ZIP1
	// SSHR is the SSHR(immediate,vector) instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SSHR--Signed-Shift-Right--immediate--?lang=en
	SSHR
	// EXT is the EXT instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/EXT--Extract-vector-from-pair-of-vectors-?lang=en
	EXT
	// CMGT is the CMGT(register) instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/CMGT--register---Compare-signed-Greater-than--vector--?lang=en
	CMGT
	// CMHI is the CMHI(register) instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/CMHI--register---Compare-unsigned-Higher--vector--?lang=en
	CMHI
	// CMGE is the CMGE(register) instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/CMGE--register---Compare-signed-Greater-than-or-Equal--vector--?lang=en
	CMGE
	// CMHS is the CMHS(register) instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/CMHS--register---Compare-unsigned-Higher-or-Same--vector--?lang=en
	CMHS
	// FCMEQ is the FCMEQ(register) instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FCMEQ--register---Floating-point-Compare-Equal--vector--?lang=en
	FCMEQ
	// FCMGT is the FCMGT(register) instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FCMGT--register---Floating-point-Compare-Greater-than--vector--?lang=en
	FCMGT
	// FCMGE is the FCMGE(register) instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FCMGE--register---Floating-point-Compare-Greater-than-or-Equal--vector--?lang=en
	FCMGE
	// VFMUL is the FMUL(vector) instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FMUL--vector---Floating-point-Multiply--vector--?lang=en
	// Note: prefixed by V to distinguish from the non-vector variant.
	VFMUL
	// VFDIV is the FDIV(vector) instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FDIV--vector---Floating-point-Divide--vector--?lang=en
	// Note: prefixed by V to distinguish from the non-vector variant.
	VFDIV
	// VFSQRT is the FSQRT(vector) instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FSQRT--vector---Floating-point-Square-Root--vector--?lang=en
	// Note: prefixed by V to distinguish from the non-vector variant.
	VFSQRT
	// VFMIN is the FMIN(vector) instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FMIN--vector---Floating-point-minimum--vector--?lang=en
	// Note: prefixed by V to distinguish from the non-vector variant.
	VFMIN
	// VFMAX is the FMAX(vector) instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FMAX--vector---Floating-point-Maximum--vector--?lang=en
	// Note: prefixed by V to distinguish from the non-vector variant.
	VFMAX
	// VFABS is the FABS(vector) instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FABS--vector---Floating-point-Absolute-value--vector--?lang=en
	// Note: prefixed by V to distinguish from the non-vector variant.
	VFABS
	// VFRINTP is the FRINTP(vector) instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FRINTP--vector---Floating-point-Round-to-Integral--toward-Plus-infinity--vector--?lang=en
	// Note: prefixed by V to distinguish from the non-vector variant.
	VFRINTP
	// VFRINTM is the FRINTM(vector) instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FRINTM--vector---Floating-point-Round-to-Integral--toward-Minus-infinity--vector--?lang=en
	// Note: prefixed by V to distinguish from the non-vector variant.
	VFRINTM
	// VFRINTZ is the FRINTZ(vector) instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FRINTZ--vector---Floating-point-Round-to-Integral--toward-Zero--vector--?lang=en
	// Note: prefixed by V to distinguish from the non-vector variant.
	VFRINTZ
	// VFRINTN is the FRINTN(vector) instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FRINTN--vector---Floating-point-Round-to-Integral--to-nearest-with-ties-to-even--vector--?lang=en
	// Note: prefixed by V to distinguish from the non-vector variant.
	VFRINTN
	// VMUL is the MUL(vector) instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/MUL--vector---Multiply--vector--?lang=en
	// Note: prefixed by V to distinguish from the non-vector variant.
	VMUL
	// VNEG is the NEG(vector) instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/NEG--vector---Negate--vector--?lang=en
	// Note: prefixed by V to distinguish from the non-vector variant.
	VNEG
	// VABS is the ABS(vector) instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/ABS--Absolute-value--vector--?lang=en
	// Note: prefixed by V to distinguish from the non-vector variant.
	VABS
	// VSQADD is the SQADD(vector) instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SQADD--Signed-saturating-Add-?lang=en
	// Note: prefixed by V to distinguish from the non-vector variant.
	VSQADD
	// VUQADD is the UQADD(vector) instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/UQADD--Unsigned-saturating-Add-?lang=en
	// Note: prefixed by V to distinguish from the non-vector variant.
	VUQADD
	// VSQSUB is the SQSUB(vector) instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SQSUB--Signed-saturating-Subtract-?lang=en
	// Note: prefixed by V to distinguish from the non-vector variant.
	VSQSUB
	// VUQSUB is the UQSUB(vector) instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/UQSUB--Unsigned-saturating-Subtract-?lang=en
	// Note: prefixed by V to distinguish from the non-vector variant.
	VUQSUB
	// SMIN is the SMIN instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SMIN--Signed-Minimum--vector--?lang=en
	SMIN
	// SMAX is the SMAX instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SMAX--Signed-Maximum--vector--?lang=en
	SMAX
	// UMIN is the UMIN instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/UMIN--Unsigned-Minimum--vector--?lang=en
	UMIN
	// UMAX is the UMAX instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/UMAX--Unsigned-Maximum--vector--?lang=en
	UMAX
	// URHADD is the URHADD instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/URHADD--Unsigned-Rounding-Halving-Add-?lang=en
	URHADD
	// REV64 is the REV64 instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/REV64--Reverse-elements-in-64-bit-doublewords--vector--?lang=en
	REV64
	// XTN is the XTN instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/XTN--XTN2--Extract-Narrow-?lang=en
	XTN
	// VUMLAL is the UMLAL(vector) instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/UMLAL--UMLAL2--vector---Unsigned-Multiply-Add-Long--vector--?lang=en
	// Note: prefixed by V to distinguish from the non-vector variant.
	VUMLAL
	// SHLL is the SHLL instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SHLL--SHLL2--Shift-Left-Long--by-element-size--?lang=en
	SHLL
	// SADDLP is the SADDLP instruction. https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SADDLP--Signed-Add-Long-Pairwise-?lang=en
	SADDLP
	// UADDLP is the UADDLP instruction. https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/UADDLP--Unsigned-Add-Long-Pairwise-?lang=en
	UADDLP
	// SSHLL2 is the SSHLL2(vector,immediate) instruction. https://developer.arm.com/documentation/dui0801/h/A64-SIMD-Vector-Instructions/SSHLL--SSHLL2--vector-
	SSHLL2
	// USHLL2 is the USHLL2(vector,immediate) instruction. https://developer.arm.com/documentation/dui0801/h/A64-SIMD-Vector-Instructions/SSHLL--SSHLL2--vector-
	USHLL2
	// SQRDMULH is the SQRDMULH(vector) instruction. https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SQRDMULH--vector---Signed-saturating-Rounding-Doubling-Multiply-returning-High-half-?lang=en
	SQRDMULH
	// SMULL is the SMULL(vector) instruction. https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SMULL--SMULL2--vector---Signed-Multiply-Long--vector--?lang=en
	SMULL
	// SMULL2 is the SMULL2(vector) instruction. https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SMULL--SMULL2--vector---Signed-Multiply-Long--vector--?lang=en
	SMULL2
	// UMULL is the UMULL instruction. https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en
	UMULL
	// UMULL2 is the UMULL2 instruction. https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en
	UMULL2
	// VFCVTZS is the FCVTZS(vector,integer) instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FCVTZS--vector--integer---Floating-point-Convert-to-Signed-integer--rounding-toward-Zero--vector--?lang=en
	// Note: prefixed by V to distinguish from the non-vector variant.
	VFCVTZS
	// VFCVTZU is the FCVTZU(vector,integer) instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FCVTZU--vector--integer---Floating-point-Convert-to-Unsigned-integer--rounding-toward-Zero--vector--?lang=en
	// Note: prefixed by V to distinguish from the non-vector variant.
	VFCVTZU
	// SQXTN is the SQXTN instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SQXTN--SQXTN2--Signed-saturating-extract-Narrow-?lang=en
	SQXTN
	// UQXTN is the UQXTN instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/UQXTN--UQXTN2--Unsigned-saturating-extract-Narrow-?lang=en
	UQXTN
	// SQXTN2 is the SQXTN2 instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SQXTN--SQXTN2--Signed-saturating-extract-Narrow-?lang=en
	SQXTN2
	// SQXTUN is the SQXTUN instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SQXTUN--SQXTUN2--Signed-saturating-extract-Unsigned-Narrow-?lang=en
	SQXTUN
	// SQXTUN2 is the SQXTUN2 instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SQXTUN--SQXTUN2--Signed-saturating-extract-Unsigned-Narrow-?lang=en
	SQXTUN2
	// VSCVTF is the SCVTF(vector, integer) instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SCVTF--vector--integer---Signed-integer-Convert-to-Floating-point--vector--?lang=en
	// Note: prefixed by V to distinguish from the non-vector variant.
	VSCVTF
	// VUCVTF is the UCVTF(vector, integer) instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/UCVTF--vector--integer---Unsigned-integer-Convert-to-Floating-point--vector--?lang=en
	// Note: prefixed by V to distinguish from the non-vector variant.
	VUCVTF
	// FCVTL is the FCVTL instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FCVTL--FCVTL2--Floating-point-Convert-to-higher-precision-Long--vector--?lang=en
	FCVTL
	// FCVTN is the FCVTN instruction https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FCVTN--FCVTN2--Floating-point-Convert-to-lower-precision-Narrow--vector--?lang=en
	FCVTN

	// UDF is the UDF instruction https://developer.arm.com/documentation/ddi0596/2021-12/Base-Instructions/UDF--Permanently-Undefined-?lang=en
	UDF

	// instructionEnd is always placed at the bottom of this iota definition to be used in the test.
	instructionEnd
)

// VectorArrangement is the arrangement of data within a vector register.
type VectorArrangement byte

const (
	// VectorArrangementNone is an arrangement indicating no data is stored.
	VectorArrangementNone VectorArrangement = iota
	// VectorArrangement8B is an arrangement of 8 bytes (64-bit vector)
	VectorArrangement8B
	// VectorArrangement16B is an arrangement of 16 bytes (128-bit vector)
	VectorArrangement16B
	// VectorArrangement4H is an arrangement of 4 half precisions (64-bit vector)
	VectorArrangement4H
	// VectorArrangement8H is an arrangement of 8 half precisions (128-bit vector)
	VectorArrangement8H
	// VectorArrangement2S is an arrangement of 2 single precisions (64-bit vector)
	VectorArrangement2S
	// VectorArrangement4S is an arrangement of 4 single precisions (128-bit vector)
	VectorArrangement4S
	// VectorArrangement1D is an arrangement of 1 double precision (64-bit vector)
	VectorArrangement1D
	// VectorArrangement2D is an arrangement of 2 double precisions (128-bit vector)
	VectorArrangement2D

	// Assign each vector size specifier to a vector arrangement ID.
	// Instructions can only have an arrangement or a size specifier, but not both, so it
	// simplifies the internal representation of vector instructions by being able to
	// store either into the same field.

	// VectorArrangementB is a size specifier of byte
	VectorArrangementB
	// VectorArrangementH is a size specifier of word (16-bit)
	VectorArrangementH
	// VectorArrangementS is a size specifier of double word (32-bit)
	VectorArrangementS
	// VectorArrangementD is a size specifier of quad word (64-bit)
	VectorArrangementD
	// VectorArrangementQ is a size specifier of the entire vector (128-bit)
	VectorArrangementQ
)

func (v VectorArrangement) String() (ret string) {
	switch v {
	case VectorArrangement8B:
		ret = "8B"
	case VectorArrangement16B:
		ret = "16B"
	case VectorArrangement4H:
		ret = "4H"
	case VectorArrangement8H:
		ret = "8H"
	case VectorArrangement2S:
		ret = "2S"
	case VectorArrangement4S:
		ret = "4S"
	case VectorArrangement1D:
		ret = "1D"
	case VectorArrangement2D:
		ret = "2D"
	case VectorArrangementB:
		ret = "B"
	case VectorArrangementH:
		ret = "H"
	case VectorArrangementS:
		ret = "S"
	case VectorArrangementD:
		ret = "D"
	case VectorArrangementQ:
		ret = "Q"
	case VectorArrangementNone:
		ret = "none"
	default:
		panic(v)
	}
	return
}

// VectorIndex is the index of an element of a vector register
type VectorIndex byte

// VectorIndexNone indicates no vector index specified.
const VectorIndexNone = ^VectorIndex(0)

// InstructionName returns the name of the given instruction
func InstructionName(i asm.Instruction) string {
	switch i {
	case NOP:
		return "NOP"
	case RET:
		return "RET"
	case ADD:
		return "ADD"
	case ADDS:
		return "ADDS"
	case ADDW:
		return "ADDW"
	case ADR:
		return "ADR"
	case AND:
		return "AND"
	case ANDIMM32:
		return "ANDIMM32"
	case ANDIMM64:
		return "ANDIMM64"
	case ANDW:
		return "ANDW"
	case ASR:
		return "ASR"
	case ASRW:
		return "ASRW"
	case B:
		return "B"
	case BCONDEQ:
		return "BCONDEQ"
	case BCONDGE:
		return "BCONDGE"
	case BCONDGT:
		return "BCONDGT"
	case BCONDHI:
		return "BCONDHI"
	case BCONDHS:
		return "BCONDHS"
	case BCONDLE:
		return "BCONDLE"
	case BCONDLO:
		return "BCONDLO"
	case BCONDLS:
		return "BCONDLS"
	case BCONDLT:
		return "BCONDLT"
	case BCONDMI:
		return "BCONDMI"
	case BCONDPL:
		return "BCONDPL"
	case BCONDNE:
		return "BCONDNE"
	case BCONDVS:
		return "BCONDVS"
	case CLZ:
		return "CLZ"
	case CLZW:
		return "CLZW"
	case CMP:
		return "CMP"
	case CMPW:
		return "CMPW"
	case CSET:
		return "CSET"
	case EOR:
		return "EOR"
	case EORW:
		return "EORW"
	case FABSD:
		return "FABSD"
	case FABSS:
		return "FABSS"
	case FADDD:
		return "FADDD"
	case FADDS:
		return "FADDS"
	case FCMPD:
		return "FCMPD"
	case FCMPS:
		return "FCMPS"
	case FCVTDS:
		return "FCVTDS"
	case FCVTSD:
		return "FCVTSD"
	case FCVTZSD:
		return "FCVTZSD"
	case FCVTZSDW:
		return "FCVTZSDW"
	case FCVTZSS:
		return "FCVTZSS"
	case FCVTZSSW:
		return "FCVTZSSW"
	case FCVTZUD:
		return "FCVTZUD"
	case FCVTZUDW:
		return "FCVTZUDW"
	case FCVTZUS:
		return "FCVTZUS"
	case FCVTZUSW:
		return "FCVTZUSW"
	case FDIVD:
		return "FDIVD"
	case FDIVS:
		return "FDIVS"
	case FMAXD:
		return "FMAXD"
	case FMAXS:
		return "FMAXS"
	case FMIND:
		return "FMIND"
	case FMINS:
		return "FMINS"
	case FMOVD:
		return "FMOVD"
	case FMOVS:
		return "FMOVS"
	case FMULD:
		return "FMULD"
	case FMULS:
		return "FMULS"
	case FNEGD:
		return "FNEGD"
	case FNEGS:
		return "FNEGS"
	case FRINTMD:
		return "FRINTMD"
	case FRINTMS:
		return "FRINTMS"
	case FRINTND:
		return "FRINTND"
	case FRINTNS:
		return "FRINTNS"
	case FRINTPD:
		return "FRINTPD"
	case FRINTPS:
		return "FRINTPS"
	case FRINTZD:
		return "FRINTZD"
	case FRINTZS:
		return "FRINTZS"
	case FSQRTD:
		return "FSQRTD"
	case FSQRTS:
		return "FSQRTS"
	case FSUBD:
		return "FSUBD"
	case FSUBS:
		return "FSUBS"
	case LSL:
		return "LSL"
	case LSLW:
		return "LSLW"
	case LSR:
		return "LSR"
	case LSRW:
		return "LSRW"
	case LDRSBD:
		return "LDRSBD"
	case LDRSBW:
		return "LDRSBW"
	case LDRB:
		return "LDRB"
	case MOVD:
		return "MOVD"
	case LDRSHD:
		return "LDRSHD"
	case LDRSHW:
		return "LDRSHW"
	case LDRH:
		return "LDRH"
	case LDRSW:
		return "LDRSW"
	case STRD:
		return "STRD"
	case STRW:
		return "STRW"
	case STRH:
		return "STRH"
	case STRB:
		return "STRB"
	case MOVW:
		return "MOVW"
	case MRS:
		return "MRS"
	case MSR:
		return "MSR"
	case MSUB:
		return "MSUB"
	case MSUBW:
		return "MSUBW"
	case MUL:
		return "MUL"
	case MULW:
		return "MULW"
	case NEG:
		return "NEG"
	case NEGW:
		return "NEGW"
	case ORR:
		return "ORR"
	case ORRW:
		return "ORRW"
	case RBIT:
		return "RBIT"
	case RBITW:
		return "RBITW"
	case ROR:
		return "ROR"
	case RORW:
		return "RORW"
	case SCVTFD:
		return "SCVTFD"
	case SCVTFS:
		return "SCVTFS"
	case SCVTFWD:
		return "SCVTFWD"
	case SCVTFWS:
		return "SCVTFWS"
	case SDIV:
		return "SDIV"
	case SDIVW:
		return "SDIVW"
	case SUB:
		return "SUB"
	case SUBS:
		return "SUBS"
	case SUBW:
		return "SUBW"
	case SXTB:
		return "SXTB"
	case SXTBW:
		return "SXTBW"
	case SXTH:
		return "SXTH"
	case SXTHW:
		return "SXTHW"
	case SXTW:
		return "SXTW"
	case UCVTFD:
		return "UCVTFD"
	case UCVTFS:
		return "UCVTFS"
	case UCVTFWD:
		return "UCVTFWD"
	case UCVTFWS:
		return "UCVTFWS"
	case UDIV:
		return "UDIV"
	case UDIVW:
		return "UDIVW"
	case VBIT:
		return "VBIT"
	case VCNT:
		return "VCNT"
	case UADDLV:
		return "UADDLV"
	case VMOV:
		return "VMOV"
	case INSELEM:
		return "INSELEM"
	case UMOV:
		return "UMOV"
	case INSGEN:
		return "INSGEN"
	case VADD:
		return "VADD"
	case VFADDS:
		return "VFADDS"
	case VFADDD:
		return "VFADDD"
	case VSUB:
		return "VSUB"
	case VFSUBS:
		return "VFSUBS"
	case VFSUBD:
		return "VFSUBD"
	case SSHL:
		return "SSHL"
	case USHL:
		return "USHL"
	case SSHLL:
		return "SSHLL"
	case USHLL:
		return "USHLL"
	case LD1R:
		return "LD1R"
	case SMOV32:
		return "SMOV32"
	case DUPGEN:
		return "DUPGEN"
	case DUPELEM:
		return "DUPELEM"
	case UMAXP:
		return "UMAXP"
	case UMINV:
		return "UMINV"
	case CMEQ:
		return "CMEQ"
	case ADDP:
		return "ADDP"
	case VADDP:
		return "VADDP"
	case TBL1:
		return "TBL1"
	case TBL2:
		return "TBL2"
	case NOT:
		return "NOT"
	case VAND:
		return "VAND"
	case VORR:
		return "VORR"
	case BSL:
		return "BSL"
	case BIC:
		return "BIC"
	case VFNEG:
		return "VFNEG"
	case ADDV:
		return "ADDV"
	case CMEQZERO:
		return "CMEQZERO"
	case ZIP1:
		return "ZIP1"
	case SSHR:
		return "SSHR"
	case EXT:
		return "EXT"
	case CMGT:
		return "CMGT"
	case CMHI:
		return "CMHI"
	case CMGE:
		return "CMGE"
	case CMHS:
		return "CMHS"
	case FCMEQ:
		return "FCMEQ"
	case FCMGT:
		return "FCMGT"
	case FCMGE:
		return "FCMGE"
	case VFMUL:
		return "VFMUL"
	case VFDIV:
		return "VFDIV"
	case VFSQRT:
		return "VFSQRT"
	case VFMIN:
		return "VFMIN"
	case VFMAX:
		return "VFMAX"
	case VFABS:
		return "VFABS"
	case VFRINTP:
		return "VFRINTP"
	case VFRINTM:
		return "VFRINTM"
	case VFRINTZ:
		return "VFRINTZ"
	case VFRINTN:
		return "VFRINTN"
	case VMUL:
		return "VMUL"
	case VNEG:
		return "VNEG"
	case VABS:
		return "VABS"
	case VSQADD:
		return "VSQADD"
	case VUQADD:
		return "VUQADD"
	case SMIN:
		return "SMIN"
	case SMAX:
		return "SMAX"
	case UMIN:
		return "UMIN"
	case UMAX:
		return "UMAX"
	case URHADD:
		return "URHADD"
	case VSQSUB:
		return "VSQSUB"
	case VUQSUB:
		return "VUQSUB"
	case REV64:
		return "REV64"
	case XTN:
		return "XTN"
	case VUMLAL:
		return "VUMLAL"
	case SHLL:
		return "SHLL"
	case SSHLL2:
		return "SSHLL2"
	case USHLL2:
		return "USHLL2"
	case SQRDMULH:
		return "SQRDMULH"
	case SADDLP:
		return "SADDLP"
	case UADDLP:
		return "UADDLP"
	case SMULL:
		return "SMULL"
	case SMULL2:
		return "SMULL2"
	case UMULL:
		return "UMULL"
	case UMULL2:
		return "UMULL2"
	case VFCVTZS:
		return "VFCVTZS"
	case VFCVTZU:
		return "VFCVTZU"
	case SQXTN:
		return "SQXTN"
	case UQXTN:
		return "UQXTN"
	case SQXTN2:
		return "SQXTN2"
	case SQXTUN:
		return "SQXTUN"
	case SQXTUN2:
		return "SQXTUN2"
	case VSCVTF:
		return "VSCVTF"
	case VUCVTF:
		return "VUCVTF"
	case FCVTL:
		return "FCVTL"
	case FCVTN:
		return "FCVTN"
	case FSTRD:
		return "FSTRD"
	case FSTRS:
		return "FSTRS"
	case LDRD:
		return "LDRD"
	case LDRW:
		return "LDRW"
	case FLDRD:
		return "FLDRD"
	case FLDRS:
		return "FLDRS"
	case UDF:
		return "UDF"
	}
	panic(fmt.Errorf("unknown instruction %d", i))
}
