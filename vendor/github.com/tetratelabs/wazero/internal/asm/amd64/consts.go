package amd64

import (
	"fmt"

	"github.com/tetratelabs/wazero/internal/asm"
)

// AMD64-specific conditional register states.
//
// See https://www.lri.fr/~filliatr/ens/compil/x86-64.pdf
// See https://www.intel.com/content/dam/www/public/us/en/documents/manuals/64-ia-32-architectures-software-developer-instruction-set-reference-manual-325383.pdf
const (
	// ConditionalRegisterStateE is the e (equal to zero) condition code
	ConditionalRegisterStateE = asm.ConditionalRegisterStateUnset + 1 + iota // ZF equal to zero
	// ConditionalRegisterStateNE is the ne (not equal to zero) condition code
	ConditionalRegisterStateNE // ˜ZF not equal to zero
	// ConditionalRegisterStateS is the s (negative) condition code
	ConditionalRegisterStateS // SF negative
	// ConditionalRegisterStateNS is the ns (non-negative) condition code
	ConditionalRegisterStateNS // ˜SF non-negative
	// ConditionalRegisterStateG is the g (greater) condition code
	ConditionalRegisterStateG // ˜(SF xor OF) & ˜ ZF greater (signed >)
	// ConditionalRegisterStateGE is the ge (greater or equal) condition code
	ConditionalRegisterStateGE // ˜(SF xor OF) greater or equal (signed >=)
	// ConditionalRegisterStateL is the l (less) condition code
	ConditionalRegisterStateL // SF xor OF less (signed <)
	// ConditionalRegisterStateLE is the le (less or equal) condition code
	ConditionalRegisterStateLE // (SF xor OF) | ZF less or equal (signed <=)
	// ConditionalRegisterStateA is the a (above) condition code
	ConditionalRegisterStateA // ˜CF & ˜ZF above (unsigned >)
	// ConditionalRegisterStateAE is the ae (above or equal) condition code
	ConditionalRegisterStateAE // ˜CF above or equal (unsigned >=)
	// ConditionalRegisterStateB is the b (below) condition code
	ConditionalRegisterStateB // CF below (unsigned <)
	// ConditionalRegisterStateBE is the be (below or equal) condition code
	ConditionalRegisterStateBE // CF | ZF below or equal (unsigned <=)
)

// AMD64-specific instructions.
//
// Note: This only defines amd64 instructions used by wazero's compiler.
// Note: Naming conventions intentionally match the Go assembler: https://go.dev/doc/asm
// See https://www.felixcloutier.com/x86/index.html
const (
	// NONE is not a real instruction but represents the lack of an instruction
	NONE asm.Instruction = iota
	// ADDL is the ADD instruction in 32-bit mode. https://www.felixcloutier.com/x86/add
	ADDL
	// ADDQ is the ADD instruction in 64-bit mode. https://www.felixcloutier.com/x86/add
	ADDQ
	// ADDSD is the ADDSD instruction. https://www.felixcloutier.com/x86/addsd
	ADDSD
	// ADDSS is the ADDSS instruction. https://www.felixcloutier.com/x86/addss
	ADDSS
	// ANDL is the AND instruction in 32-bit mode. https://www.felixcloutier.com/x86/and
	ANDL
	// ANDPD is the ANDPD instruction. https://www.felixcloutier.com/x86/andpd
	ANDPD
	// ANDPS is the ANDPS instruction. https://www.felixcloutier.com/x86/andps
	ANDPS
	// ANDQ is the AND instruction in 64-bit mode. https://www.felixcloutier.com/x86/and
	ANDQ
	// BSRL is the BSR instruction in 32-bit mode. https://www.felixcloutier.com/x86/bsr
	BSRL
	// BSRQ is the BSR instruction in 64-bit mode. https://www.felixcloutier.com/x86/bsr
	BSRQ
	// CDQ is the CDQ instruction. https://www.felixcloutier.com/x86/cwd:cdq:cqo
	CDQ
	// CLD is the CLD instruction. https://www.felixcloutier.com/x86/cld
	CLD
	// CMOVQCS is the CMOVC (move if carry) instruction in 64-bit mode. https://www.felixcloutier.com/x86/cmovcc
	CMOVQCS
	// CMPL is the CMP instruction in 32-bit mode. https://www.felixcloutier.com/x86/cmp
	CMPL
	// CMPQ is the CMP instruction in 64-bit mode. https://www.felixcloutier.com/x86/cmp
	CMPQ
	// COMISD is the COMISD instruction. https://www.felixcloutier.com/x86/comisd
	COMISD
	// COMISS is the COMISS instruction. https://www.felixcloutier.com/x86/comiss
	COMISS
	// CQO is the CQO instruction. https://www.felixcloutier.com/x86/cwd:cdq:cqo
	CQO
	// CVTSD2SS is the CVTSD2SS instruction. https://www.felixcloutier.com/x86/cvtsd2ss
	CVTSD2SS
	// CVTSL2SD is the CVTSI2SD instruction in 32-bit mode. https://www.felixcloutier.com/x86/cvtsi2sd
	CVTSL2SD
	// CVTSL2SS is the CVTSI2SS instruction in 32-bit mode. https://www.felixcloutier.com/x86/cvtsi2ss
	CVTSL2SS
	// CVTSQ2SD is the CVTSI2SD instruction in 64-bit mode. https://www.felixcloutier.com/x86/cvtsi2sd
	CVTSQ2SD
	// CVTSQ2SS is the CVTSI2SS instruction in 64-bit mode. https://www.felixcloutier.com/x86/cvtsi2ss
	CVTSQ2SS
	// CVTSS2SD is the CVTSS2SD instruction. https://www.felixcloutier.com/x86/cvtss2sd
	CVTSS2SD
	// CVTTSD2SL is the CVTTSD2SI instruction in 32-bit mode. https://www.felixcloutier.com/x86/cvttsd2si
	CVTTSD2SL
	// CVTTSD2SQ is the CVTTSD2SI instruction in 64-bit mode. https://www.felixcloutier.com/x86/cvttsd2si
	CVTTSD2SQ
	// CVTTSS2SL is the CVTTSS2SI instruction in 32-bit mode. https://www.felixcloutier.com/x86/cvttss2si
	CVTTSS2SL
	// CVTTSS2SQ is the CVTTSS2SI instruction in 64-bit mode. https://www.felixcloutier.com/x86/cvttss2si
	CVTTSS2SQ
	// DECQ is the DEC instruction in 64-bit mode. https://www.felixcloutier.com/x86/dec
	DECQ
	// DIVL is the DIV instruction in 32-bit mode. https://www.felixcloutier.com/x86/div
	DIVL
	// DIVQ is the DIV instruction in 64-bit mode. https://www.felixcloutier.com/x86/div
	DIVQ
	// DIVSD is the DIVSD instruction. https://www.felixcloutier.com/x86/divsd
	DIVSD
	// DIVSS is the DIVSS instruction. https://www.felixcloutier.com/x86/divss
	DIVSS
	// IDIVL is the IDIV instruction in 32-bit mode. https://www.felixcloutier.com/x86/idiv
	IDIVL
	// IDIVQ is the IDIV instruction in 64-bit mode. https://www.felixcloutier.com/x86/idiv
	IDIVQ
	// INCQ is the INC instruction in 64-bit mode. https://www.felixcloutier.com/x86/inc
	INCQ
	// JCC is the JAE (jump if above or equal) instruction. https://www.felixcloutier.com/x86/jcc
	JCC
	// JCS is the JB (jump if below) instruction. https://www.felixcloutier.com/x86/jcc
	JCS
	// JEQ is the JE (jump if equal) instruction. https://www.felixcloutier.com/x86/jcc
	JEQ
	// JGE is the JGE (jump if greater or equal) instruction. https://www.felixcloutier.com/x86/jcc
	JGE
	// JGT is the JG (jump if greater) instruction. https://www.felixcloutier.com/x86/jcc
	JGT
	// JHI is the JNBE (jump if not below or equal) instruction. https://www.felixcloutier.com/x86/jcc
	JHI
	// JLE is the JLE (jump if less or equal) instruction. https://www.felixcloutier.com/x86/jcc
	JLE
	// JLS is the JNA (jump if not above) instruction. https://www.felixcloutier.com/x86/jcc
	JLS
	// JLT is the JL (jump if less) instruction. https://www.felixcloutier.com/x86/jcc
	JLT
	// JMI is the JS (jump if sign) instruction. https://www.felixcloutier.com/x86/jcc
	JMI
	// JNE is the JNE (jump if not equal) instruction. https://www.felixcloutier.com/x86/jcc
	JNE
	// JPC is the JPO (jump if parity odd) instruction. https://www.felixcloutier.com/x86/jcc
	JPC
	// JPL is the JNS (jump if not sign) instruction. https://www.felixcloutier.com/x86/jcc
	JPL
	// JPS is the JPE (jump if parity even) instruction. https://www.felixcloutier.com/x86/jcc
	JPS
	// LEAQ is the LEA instruction in 64-bit mode. https://www.felixcloutier.com/x86/lea
	LEAQ
	// LZCNTL is the LZCNT instruction in 32-bit mode. https://www.felixcloutier.com/x86/lzcnt
	LZCNTL
	// LZCNTQ is the LZCNT instruction in 64-bit mode. https://www.felixcloutier.com/x86/lzcnt
	LZCNTQ
	// MAXSD is the MAXSD instruction. https://www.felixcloutier.com/x86/maxsd
	MAXSD
	// MAXSS is the MAXSS instruction. https://www.felixcloutier.com/x86/maxss
	MAXSS
	// MINSD is the MINSD instruction. https://www.felixcloutier.com/x86/minsd
	MINSD
	// MINSS is the MINSS instruction. https://www.felixcloutier.com/x86/minss
	MINSS
	// MOVB is the MOV instruction for a single byte. https://www.felixcloutier.com/x86/mov
	MOVB
	// MOVBLSX is the MOVSX instruction for single byte in 32-bit mode. https://www.felixcloutier.com/x86/movsx:movsxd
	MOVBLSX
	// MOVBLZX is the MOVZX instruction for single-byte in 32-bit mode. https://www.felixcloutier.com/x86/movzx
	MOVBLZX
	// MOVBQSX is the MOVSX instruction for single byte in 64-bit mode. https://www.felixcloutier.com/x86/movsx:movsxd
	MOVBQSX
	// MOVBQZX is the MOVZX instruction for single-byte in 64-bit mode. https://www.felixcloutier.com/x86/movzx
	MOVBQZX
	// MOVL is the MOV instruction for a double word.
	MOVL
	// MOVLQSX is the MOVSXD instruction. https://www.felixcloutier.com/x86/movsx:movsxd
	MOVLQSX
	// MOVLQZX is the MOVZX instruction for a word to a doubleword. https://www.felixcloutier.com/x86/movzx
	MOVLQZX
	// MOVQ is the MOV instruction for a doubleword. https://www.felixcloutier.com/x86/mov
	MOVQ
	// MOVW is the MOV instruction for a word. https://www.felixcloutier.com/x86/mov
	MOVW
	// MOVWLSX is the MOVSX instruction for a word in 32-bit mode. https://www.felixcloutier.com/x86/movsx:movsxd
	MOVWLSX
	// MOVWLZX is the MOVZX instruction for a word in 32-bit mode. https://www.felixcloutier.com/x86/movzx
	MOVWLZX
	// MOVWQSX is the MOVSX instruction for a word in 64-bit mode. https://www.felixcloutier.com/x86/movsx:movsxd
	MOVWQSX
	// MOVWQZX is the MOVZX instruction for a word in 64-bit mode. https://www.felixcloutier.com/x86/movzx
	MOVWQZX
	// MULL is the MUL instruction in 32-bit mode. https://www.felixcloutier.com/x86/mul
	MULL
	// MULQ is the MUL instruction in 64-bit mode. https://www.felixcloutier.com/x86/mul
	MULQ
	// IMULQ is the IMUL instruction in 64-bit mode. https://www.felixcloutier.com/x86/imul
	IMULQ
	// MULSD is the MULSD instruction. https://www.felixcloutier.com/x86/mulsd
	MULSD
	// MULSS is the MULSS instruction. https://www.felixcloutier.com/x86/mulss
	MULSS
	// NEGQ is the NEG instruction in 64-bit mode. https://www.felixcloutier.com/x86/neg
	NEGQ
	// ORL is the OR instruction in 32-bit mode. https://www.felixcloutier.com/x86/or
	ORL
	// ORPD is the ORPD instruction. https://www.felixcloutier.com/x86/orpd
	ORPD
	// ORPS is the ORPS instruction. https://www.felixcloutier.com/x86/orps
	ORPS
	// ORQ is the OR instruction in 64-bit mode. https://www.felixcloutier.com/x86/or
	ORQ
	// POPCNTL is the POPCNT instruction in 32-bit mode. https://www.felixcloutier.com/x86/popcnt
	POPCNTL
	// POPCNTQ is the POPCNT instruction in 64-bit mode. https://www.felixcloutier.com/x86/popcnt
	POPCNTQ
	// PSLLD is the PSLLD instruction. https://www.felixcloutier.com/x86/psllw:pslld:psllq
	PSLLD
	// PSLLQ is the PSLLQ instruction. https://www.felixcloutier.com/x86/psllw:pslld:psllq
	PSLLQ
	// PSRLD is the PSRLD instruction. https://www.felixcloutier.com/x86/psrlw:psrld:psrlq
	PSRLD
	// PSRLQ is the PSRLQ instruction. https://www.felixcloutier.com/x86/psrlw:psrld:psrlq
	PSRLQ
	// REPMOVSQ is the REP MOVSQ instruction in 64-bit mode. https://www.felixcloutier.com/x86/movs:movsb:movsw:movsd:movsq https://www.felixcloutier.com/x86/rep:repe:repz:repne:repnz
	REPMOVSQ
	// REPSTOSQ is the REP STOSQ instruction in 64-bit mode. https://www.felixcloutier.com/x86/stos:stosb:stosw:stosd:stosq https://www.felixcloutier.com/x86/rep:repe:repz:repne:repnz
	REPSTOSQ
	// ROLL is the ROL instruction in 32-bit mode. https://www.felixcloutier.com/x86/rcl:rcr:rol:ror
	ROLL
	// ROLQ is the ROL instruction in 64-bit mode. https://www.felixcloutier.com/x86/rcl:rcr:rol:ror
	ROLQ
	// RORL is the ROR instruction in 32-bit mode. https://www.felixcloutier.com/x86/rcl:rcr:rol:ror
	RORL
	// RORQ is the ROR instruction in 64-bit mode. https://www.felixcloutier.com/x86/rcl:rcr:rol:ror
	RORQ
	// ROUNDSD is the ROUNDSD instruction. https://www.felixcloutier.com/x86/roundsd
	ROUNDSD
	// ROUNDSS is the ROUNDSS instruction. https://www.felixcloutier.com/x86/roundss
	ROUNDSS
	// SARL is the SAR instruction in 32-bit mode. https://www.felixcloutier.com/x86/sal:sar:shl:shr
	SARL
	// SARQ is the SAR instruction in 64-bit mode. https://www.felixcloutier.com/x86/sal:sar:shl:shr
	SARQ
	// SETCC is the SETAE (set if above or equal) instruction. https://www.felixcloutier.com/x86/setcc
	SETCC
	// SETCS is the SETB (set if below) instruction. https://www.felixcloutier.com/x86/setcc
	SETCS
	// SETEQ is the SETE (set if equal) instruction. https://www.felixcloutier.com/x86/setcc
	SETEQ
	// SETGE is the SETGE (set if greater or equal) instruction. https://www.felixcloutier.com/x86/setcc
	SETGE
	// SETGT is the SETG (set if greater) instruction. https://www.felixcloutier.com/x86/setcc
	SETGT
	// SETHI is the SETNBE (set if not below or equal) instruction. https://www.felixcloutier.com/x86/setcc
	SETHI
	// SETLE is the SETLE (set if less or equal) instruction. https://www.felixcloutier.com/x86/setcc
	SETLE
	// SETLS is the SETNA (set if not above) instruction. https://www.felixcloutier.com/x86/setcc
	SETLS
	// SETLT is the SETL (set if less) instruction. https://www.felixcloutier.com/x86/setcc
	SETLT
	// SETMI is the SETS (set if sign) instruction. https://www.felixcloutier.com/x86/setcc
	SETMI
	// SETNE is the SETNE (set if not equal) instruction. https://www.felixcloutier.com/x86/setcc
	SETNE
	// SETPC is the SETNP (set if not parity) instruction. https://www.felixcloutier.com/x86/setcc
	SETPC
	// SETPL is the SETNS (set if not sign) instruction. https://www.felixcloutier.com/x86/setcc
	SETPL
	// SETPS is the SETP (set if parity) instruction. https://www.felixcloutier.com/x86/setcc
	SETPS
	// SHLL is the SHL instruction in 32-bit mode. https://www.felixcloutier.com/x86/sal:sar:shl:shr
	SHLL
	// SHLQ is the SHL instruction in 64-bit mode. https://www.felixcloutier.com/x86/sal:sar:shl:shr
	SHLQ
	// SHRL is the SHR instruction in 32-bit mode. https://www.felixcloutier.com/x86/sal:sar:shl:shr
	SHRL
	// SHRQ is the SHR instruction in 64-bit mode. https://www.felixcloutier.com/x86/sal:sar:shl:shr
	SHRQ
	// SQRTSD is the SQRTSD instruction. https://www.felixcloutier.com/x86/sqrtsd
	SQRTSD
	// SQRTSS is the SQRTSS instruction. https://www.felixcloutier.com/x86/sqrtss
	SQRTSS
	// STD is the STD instruction. https://www.felixcloutier.com/x86/std
	STD
	// SUBL is the SUB instruction in 32-bit mode. https://www.felixcloutier.com/x86/sub
	SUBL
	// SUBQ is the SUB instruction in 64-bit mode. https://www.felixcloutier.com/x86/sub
	SUBQ
	// SUBSD is the SUBSD instruction. https://www.felixcloutier.com/x86/subsd
	SUBSD
	// SUBSS is the SUBSS instruction. https://www.felixcloutier.com/x86/subss
	SUBSS
	// TESTL is the TEST instruction in 32-bit mode. https://www.felixcloutier.com/x86/test
	TESTL
	// TESTQ is the TEST instruction in 64-bit mode. https://www.felixcloutier.com/x86/test
	TESTQ
	// TZCNTL is the TZCNT instruction in 32-bit mode. https://www.felixcloutier.com/x86/tzcnt
	TZCNTL
	// TZCNTQ is the TZCNT instruction in 64-bit mode. https://www.felixcloutier.com/x86/tzcnt
	TZCNTQ
	// UCOMISD is the UCOMISD instruction. https://www.felixcloutier.com/x86/ucomisd
	UCOMISD
	// UCOMISS is the UCOMISS instruction. https://www.felixcloutier.com/x86/ucomisd
	UCOMISS
	// XORL is the XOR instruction in 32-bit mode. https://www.felixcloutier.com/x86/xor
	XORL
	// XORPD is the XORPD instruction. https://www.felixcloutier.com/x86/xorpd
	XORPD
	// XORPS is the XORPS instruction. https://www.felixcloutier.com/x86/xorps
	XORPS
	// XORQ is the XOR instruction in 64-bit mode. https://www.felixcloutier.com/x86/xor
	XORQ
	// XCHGQ is the XCHG instruction in 64-bit mode. https://www.felixcloutier.com/x86/xchg
	XCHGQ
	// RET is the RET instruction. https://www.felixcloutier.com/x86/ret
	RET
	// JMP is the JMP instruction. https://www.felixcloutier.com/x86/jmp
	JMP
	// NOP is the NOP instruction. https://www.felixcloutier.com/x86/nop
	NOP
	// UD2 is the UD2 instruction. https://www.felixcloutier.com/x86/ud
	UD2
	// MOVDQU is the MOVDQU instruction in 64-bit mode. https://www.felixcloutier.com/x86/movdqu:vmovdqu8:vmovdqu16:vmovdqu32:vmovdqu64
	MOVDQU
	// MOVDQA is the MOVDQA instruction in 64-bit mode. https://www.felixcloutier.com/x86/movdqa:vmovdqa32:vmovdqa64
	MOVDQA
	// PINSRB is the PINSRB instruction. https://www.felixcloutier.com/x86/pinsrb:pinsrd:pinsrq
	PINSRB
	// PINSRW is the PINSRW instruction. https://www.felixcloutier.com/x86/pinsrw
	PINSRW
	// PINSRD is the PINSRD instruction. https://www.felixcloutier.com/x86/pinsrb:pinsrd:pinsrq
	PINSRD
	// PINSRQ is the PINSRQ instruction. https://www.felixcloutier.com/x86/pinsrb:pinsrd:pinsrq
	PINSRQ
	// PADDB is the PADDB instruction. https://www.felixcloutier.com/x86/paddb:paddw:paddd:paddq
	PADDB
	// PADDW is the PADDW instruction. https://www.felixcloutier.com/x86/paddb:paddw:paddd:paddq
	PADDW
	// PADDD is the PADDD instruction. https://www.felixcloutier.com/x86/paddb:paddw:paddd:paddq
	PADDD
	// PADDQ is the PADDQ instruction. https://www.felixcloutier.com/x86/paddb:paddw:paddd:paddq
	PADDQ
	// PSUBB is the PSUBB instruction. https://www.felixcloutier.com/x86/psubb:psubw:psubd
	PSUBB
	// PSUBW is the PSUBW instruction. https://www.felixcloutier.com/x86/psubb:psubw:psubd
	PSUBW
	// PSUBD is the PSUBD instruction. https://www.felixcloutier.com/x86/psubb:psubw:psubd
	PSUBD
	// PSUBQ is the PSUBQ instruction. https://www.felixcloutier.com/x86/psubq
	PSUBQ
	// ADDPS is the ADDPS instruction. https://www.felixcloutier.com/x86/addps
	ADDPS
	// ADDPD is the ADDPD instruction. https://www.felixcloutier.com/x86/addpd
	ADDPD
	// SUBPS is the SUBPS instruction. https://www.felixcloutier.com/x86/subps
	SUBPS
	// SUBPD is the SUBPD instruction. https://www.felixcloutier.com/x86/subpd
	SUBPD
	// PMOVSXBW is the PMOVSXBW instruction https://www.felixcloutier.com/x86/pmovsx
	PMOVSXBW
	// PMOVSXWD is the PMOVSXWD instruction https://www.felixcloutier.com/x86/pmovsx
	PMOVSXWD
	// PMOVSXDQ is the PMOVSXDQ instruction https://www.felixcloutier.com/x86/pmovsx
	PMOVSXDQ
	// PMOVZXBW is the PMOVZXBW instruction https://www.felixcloutier.com/x86/pmovzx
	PMOVZXBW
	// PMOVZXWD is the PMOVZXWD instruction https://www.felixcloutier.com/x86/pmovzx
	PMOVZXWD
	// PMOVZXDQ is the PMOVZXDQ instruction https://www.felixcloutier.com/x86/pmovzx
	PMOVZXDQ
	// PSHUFB is the PSHUFB instruction https://www.felixcloutier.com/x86/pshufb
	PSHUFB
	// PSHUFD is the PSHUFD instruction https://www.felixcloutier.com/x86/pshufd
	PSHUFD
	// PXOR is the PXOR instruction https://www.felixcloutier.com/x86/pxor
	PXOR
	// PEXTRB is the PEXTRB instruction https://www.felixcloutier.com/x86/pextrb:pextrd:pextrq
	PEXTRB
	// PEXTRW is the PEXTRW instruction https://www.felixcloutier.com/x86/pextrw
	PEXTRW
	// PEXTRD is the PEXTRD instruction https://www.felixcloutier.com/x86/pextrb:pextrd:pextrq
	PEXTRD
	// PEXTRQ is the PEXTRQ instruction https://www.felixcloutier.com/x86/pextrb:pextrd:pextrq
	PEXTRQ
	// MOVLHPS is the MOVLHPS instruction https://www.felixcloutier.com/x86/movlhps
	MOVLHPS
	// INSERTPS is the INSERTPS instruction https://www.felixcloutier.com/x86/insertps
	INSERTPS
	// PTEST is the PTEST instruction https://www.felixcloutier.com/x86/ptest
	PTEST
	// PCMPEQB is the PCMPEQB instruction https://www.felixcloutier.com/x86/pcmpeqb:pcmpeqw:pcmpeqd
	PCMPEQB
	// PCMPEQW is the PCMPEQW instruction https://www.felixcloutier.com/x86/pcmpeqb:pcmpeqw:pcmpeqd
	PCMPEQW
	// PCMPEQD is the PCMPEQD instruction https://www.felixcloutier.com/x86/pcmpeqb:pcmpeqw:pcmpeqd
	PCMPEQD
	// PCMPEQQ is the PCMPEQQ instruction https://www.felixcloutier.com/x86/pcmpeqq
	PCMPEQQ
	// PADDUSB is the PADDUSB instruction https://www.felixcloutier.com/x86/paddusb:paddusw
	PADDUSB
	// MOVSD is the MOVSD instruction https://www.felixcloutier.com/x86/movsd
	MOVSD
	// PACKSSWB is the PACKSSWB instruction https://www.felixcloutier.com/x86/packsswb:packssdw
	PACKSSWB
	// PMOVMSKB is the PMOVMSKB instruction https://www.felixcloutier.com/x86/pmovmskb
	PMOVMSKB
	// MOVMSKPS is the MOVMSKPS instruction https://www.felixcloutier.com/x86/movmskps
	MOVMSKPS
	// MOVMSKPD is the MOVMSKPD instruction https://www.felixcloutier.com/x86/movmskpd
	MOVMSKPD
	// PAND is the PAND instruction https://www.felixcloutier.com/x86/pand
	PAND
	// POR is the POR instruction https://www.felixcloutier.com/x86/por
	POR
	// PANDN is the PANDN instruction https://www.felixcloutier.com/x86/pandn
	PANDN
	// PSRAD is the PSRAD instruction https://www.felixcloutier.com/x86/psraw:psrad:psraq
	PSRAD
	// PSRAW is the PSRAW instruction https://www.felixcloutier.com/x86/psraw:psrad:psraq
	PSRAW
	// PSRLW is the PSRLW instruction https://www.felixcloutier.com/x86/psrlw:psrld:psrlq
	PSRLW
	// PSLLW is the PSLLW instruction https://www.felixcloutier.com/x86/psllw:pslld:psllq
	PSLLW
	// PUNPCKLBW is the PUNPCKLBW instruction https://www.felixcloutier.com/x86/punpcklbw:punpcklwd:punpckldq:punpcklqdq
	PUNPCKLBW
	// PUNPCKHBW is the PUNPCKHBW instruction https://www.felixcloutier.com/x86/punpckhbw:punpckhwd:punpckhdq:punpckhqdq
	PUNPCKHBW
	// CMPPS is the CMPPS instruction https://www.felixcloutier.com/x86/cmpps
	CMPPS
	// CMPPD is the https://www.felixcloutier.com/x86/cmppd
	CMPPD
	// PCMPGTQ is the PCMPGTQ instruction https://www.felixcloutier.com/x86/pcmpgtq
	PCMPGTQ
	// PCMPGTD is the PCMPGTD instruction https://www.felixcloutier.com/x86/pcmpgtb:pcmpgtw:pcmpgtd
	PCMPGTD
	// PCMPGTW is the PCMPGTW instruction https://www.felixcloutier.com/x86/pcmpgtb:pcmpgtw:pcmpgtd
	PCMPGTW
	// PCMPGTB is the PCMPGTB instruction https://www.felixcloutier.com/x86/pcmpgtb:pcmpgtw:pcmpgtd
	PCMPGTB
	// PMINSD is the PMINSD instruction https://www.felixcloutier.com/x86/pminsd:pminsq
	PMINSD
	// PMINSW is the PMINSW instruction https://www.felixcloutier.com/x86/pminsb:pminsw
	PMINSW
	// PMINSB is the PMINSB instruction https://www.felixcloutier.com/x86/pminsb:pminsw
	PMINSB
	// PMAXSD is the PMAXSD instruction https://www.felixcloutier.com/x86/pmaxsb:pmaxsw:pmaxsd:pmaxsq
	PMAXSD
	// PMAXSW is the PMAXSW instruction https://www.felixcloutier.com/x86/pmaxsb:pmaxsw:pmaxsd:pmaxsq
	PMAXSW
	// PMAXSB is the PMAXSB instruction https://www.felixcloutier.com/x86/pmaxsb:pmaxsw:pmaxsd:pmaxsq
	PMAXSB
	// PMINUD is the PMINUD instruction https://www.felixcloutier.com/x86/pminud:pminuq
	PMINUD
	// PMINUW is the PMINUW instruction https://www.felixcloutier.com/x86/pminub:pminuw
	PMINUW
	// PMINUB is the PMINUB instruction https://www.felixcloutier.com/x86/pminub:pminuw
	PMINUB
	// PMAXUD is the PMAXUD instruction https://www.felixcloutier.com/x86/pmaxud:pmaxuq
	PMAXUD
	// PMAXUW is the PMAXUW instruction https://www.felixcloutier.com/x86/pmaxub:pmaxuw
	PMAXUW
	// PMAXUB is the PMAXUB instruction https://www.felixcloutier.com/x86/pmaxub:pmaxuw
	PMAXUB
	// PMULLW is the PMULLW instruction https://www.felixcloutier.com/x86/pmullw
	PMULLW
	// PMULLD is the PMULLD instruction https://www.felixcloutier.com/x86/pmulld:pmullq
	PMULLD
	// PMULUDQ is the PMULUDQ instruction https://www.felixcloutier.com/x86/pmuludq
	PMULUDQ
	// PSUBSB is the PSUBSB instruction https://www.felixcloutier.com/x86/psubsb:psubsw
	PSUBSB
	// PSUBSW is the PSUBSW instruction https://www.felixcloutier.com/x86/psubsb:psubsw
	PSUBSW
	// PSUBUSB is the PSUBUSB instruction https://www.felixcloutier.com/x86/psubusb:psubusw
	PSUBUSB
	// PSUBUSW is the PSUBUSW instruction https://www.felixcloutier.com/x86/psubusb:psubusw
	PSUBUSW
	// PADDSW is the PADDSW instruction https://www.felixcloutier.com/x86/paddsb:paddsw
	PADDSW
	// PADDSB is the PADDSB instruction https://www.felixcloutier.com/x86/paddsb:paddsw
	PADDSB
	// PADDUSW is the PADDUSW instruction https://www.felixcloutier.com/x86/paddusb:paddusw
	PADDUSW
	// PAVGB is the PAVGB instruction https://www.felixcloutier.com/x86/pavgb:pavgw
	PAVGB
	// PAVGW is the PAVGW instruction https://www.felixcloutier.com/x86/pavgb:pavgw
	PAVGW
	// PABSB is the PABSB instruction https://www.felixcloutier.com/x86/pabsb:pabsw:pabsd:pabsq
	PABSB
	// PABSW is the PABSW instruction https://www.felixcloutier.com/x86/pabsb:pabsw:pabsd:pabsq
	PABSW
	// PABSD is the PABSD instruction https://www.felixcloutier.com/x86/pabsb:pabsw:pabsd:pabsq
	PABSD
	// BLENDVPD is the BLENDVPD instruction https://www.felixcloutier.com/x86/blendvpd
	BLENDVPD
	// MAXPD is the MAXPD instruction https://www.felixcloutier.com/x86/maxpd
	MAXPD
	// MAXPS is the MAXPS instruction https://www.felixcloutier.com/x86/maxps
	MAXPS
	// MINPD is the MINPD instruction https://www.felixcloutier.com/x86/minpd
	MINPD
	// MINPS is the MINPS instruction https://www.felixcloutier.com/x86/minps
	MINPS
	// ANDNPD is the ANDNPD instruction https://www.felixcloutier.com/x86/andnpd
	ANDNPD
	// ANDNPS is the ANDNPS instruction https://www.felixcloutier.com/x86/andnps
	ANDNPS
	// MULPS is the MULPS instruction https://www.felixcloutier.com/x86/mulps
	MULPS
	// MULPD is the MULPD instruction https://www.felixcloutier.com/x86/mulpd
	MULPD
	// DIVPS is the DIVPS instruction https://www.felixcloutier.com/x86/divps
	DIVPS
	// DIVPD is the DIVPD instruction https://www.felixcloutier.com/x86/divpd
	DIVPD
	// SQRTPS is the SQRTPS instruction https://www.felixcloutier.com/x86/sqrtps
	SQRTPS
	// SQRTPD is the SQRTPD instruction https://www.felixcloutier.com/x86/sqrtpd
	SQRTPD
	// ROUNDPS is the ROUNDPS instruction https://www.felixcloutier.com/x86/roundps
	ROUNDPS
	// ROUNDPD is the ROUNDPD instruction https://www.felixcloutier.com/x86/roundpd
	ROUNDPD
	// PALIGNR is the PALIGNR instruction https://www.felixcloutier.com/x86/palignr
	PALIGNR
	// PUNPCKLWD is the PUNPCKLWD instruction https://www.felixcloutier.com/x86/punpcklbw:punpcklwd:punpckldq:punpcklqdq
	PUNPCKLWD
	// PUNPCKHWD is the PUNPCKHWD instruction https://www.felixcloutier.com/x86/punpckhbw:punpckhwd:punpckhdq:punpckhqdq
	PUNPCKHWD
	// PMULHUW is the PMULHUW instruction https://www.felixcloutier.com/x86/pmulhuw
	PMULHUW
	// PMULDQ is the PMULDQ instruction https://www.felixcloutier.com/x86/pmuldq
	PMULDQ
	// PMULHRSW is the PMULHRSW instruction https://www.felixcloutier.com/x86/pmulhrsw
	PMULHRSW
	// PMULHW is the PMULHW instruction https://www.felixcloutier.com/x86/pmulhw
	PMULHW
	// CMPEQPS is the CMPEQPS instruction https://www.felixcloutier.com/x86/cmpps
	CMPEQPS
	// CMPEQPD is the CMPEQPD instruction https://www.felixcloutier.com/x86/cmppd
	CMPEQPD
	// CVTTPS2DQ is the CVTTPS2DQ instruction https://www.felixcloutier.com/x86/cvttps2dq
	CVTTPS2DQ
	// CVTDQ2PS is the CVTDQ2PS instruction https://www.felixcloutier.com/x86/cvtdq2ps
	CVTDQ2PS
	// MOVUPD is the MOVUPD instruction https://www.felixcloutier.com/x86/movupd
	MOVUPD
	// SHUFPS is the SHUFPS instruction https://www.felixcloutier.com/x86/shufps
	SHUFPS
	// PMADDWD is the PMADDWD instruction https://www.felixcloutier.com/x86/pmaddwd
	PMADDWD
	// CVTDQ2PD is the CVTDQ2PD instruction https://www.felixcloutier.com/x86/cvtdq2pd
	CVTDQ2PD
	// UNPCKLPS is the UNPCKLPS instruction https://www.felixcloutier.com/x86/unpcklps
	UNPCKLPS
	// PACKUSWB is the PACKUSWB instruction https://www.felixcloutier.com/x86/packuswb
	PACKUSWB
	// PACKSSDW is the PACKSSDW instruction https://www.felixcloutier.com/x86/packsswb:packssdw
	PACKSSDW
	// PACKUSDW is the PACKUSDW instruction https://www.felixcloutier.com/x86/packusdw
	PACKUSDW
	// CVTPS2PD is the CVTPS2PD instruction https://www.felixcloutier.com/x86/cvtps2pd
	CVTPS2PD
	// CVTPD2PS is the CVTPD2PS instruction https://www.felixcloutier.com/x86/cvtpd2ps
	CVTPD2PS
	// PMADDUBSW is the PMADDUBSW instruction https://www.felixcloutier.com/x86/pmaddubsw
	PMADDUBSW
	// CVTTPD2DQ is the CVTTPD2DQ instruction https://www.felixcloutier.com/x86/cvttpd2dq
	CVTTPD2DQ

	// instructionEnd is always placed at the bottom of this iota definition to be used in the test.
	instructionEnd
)

// InstructionName returns the name for an instruction
func InstructionName(instruction asm.Instruction) string {
	switch instruction {
	case ADDL:
		return "ADDL"
	case ADDQ:
		return "ADDQ"
	case ADDSD:
		return "ADDSD"
	case ADDSS:
		return "ADDSS"
	case ANDL:
		return "ANDL"
	case ANDPD:
		return "ANDPD"
	case ANDPS:
		return "ANDPS"
	case ANDQ:
		return "ANDQ"
	case BSRL:
		return "BSRL"
	case BSRQ:
		return "BSRQ"
	case CDQ:
		return "CDQ"
	case CLD:
		return "CLD"
	case CMOVQCS:
		return "CMOVQCS"
	case CMPL:
		return "CMPL"
	case CMPQ:
		return "CMPQ"
	case COMISD:
		return "COMISD"
	case COMISS:
		return "COMISS"
	case CQO:
		return "CQO"
	case CVTSD2SS:
		return "CVTSD2SS"
	case CVTSL2SD:
		return "CVTSL2SD"
	case CVTSL2SS:
		return "CVTSL2SS"
	case CVTSQ2SD:
		return "CVTSQ2SD"
	case CVTSQ2SS:
		return "CVTSQ2SS"
	case CVTSS2SD:
		return "CVTSS2SD"
	case CVTTSD2SL:
		return "CVTTSD2SL"
	case CVTTSD2SQ:
		return "CVTTSD2SQ"
	case CVTTSS2SL:
		return "CVTTSS2SL"
	case CVTTSS2SQ:
		return "CVTTSS2SQ"
	case DECQ:
		return "DECQ"
	case DIVL:
		return "DIVL"
	case DIVQ:
		return "DIVQ"
	case DIVSD:
		return "DIVSD"
	case DIVSS:
		return "DIVSS"
	case IDIVL:
		return "IDIVL"
	case IDIVQ:
		return "IDIVQ"
	case INCQ:
		return "INCQ"
	case JCC:
		return "JCC"
	case JCS:
		return "JCS"
	case JEQ:
		return "JEQ"
	case JGE:
		return "JGE"
	case JGT:
		return "JGT"
	case JHI:
		return "JHI"
	case JLE:
		return "JLE"
	case JLS:
		return "JLS"
	case JLT:
		return "JLT"
	case JMI:
		return "JMI"
	case JNE:
		return "JNE"
	case JPC:
		return "JPC"
	case JPL:
		return "JPL"
	case JPS:
		return "JPS"
	case LEAQ:
		return "LEAQ"
	case LZCNTL:
		return "LZCNTL"
	case LZCNTQ:
		return "LZCNTQ"
	case MAXSD:
		return "MAXSD"
	case MAXSS:
		return "MAXSS"
	case MINSD:
		return "MINSD"
	case MINSS:
		return "MINSS"
	case MOVB:
		return "MOVB"
	case MOVBLSX:
		return "MOVBLSX"
	case MOVBLZX:
		return "MOVBLZX"
	case MOVBQSX:
		return "MOVBQSX"
	case MOVBQZX:
		return "MOVBQZX"
	case MOVL:
		return "MOVL"
	case MOVLQSX:
		return "MOVLQSX"
	case MOVLQZX:
		return "MOVLQZX"
	case MOVQ:
		return "MOVQ"
	case MOVW:
		return "MOVW"
	case MOVWLSX:
		return "MOVWLSX"
	case MOVWLZX:
		return "MOVWLZX"
	case MOVWQSX:
		return "MOVWQSX"
	case MOVWQZX:
		return "MOVWQZX"
	case MULL:
		return "MULL"
	case MULQ:
		return "MULQ"
	case IMULQ:
		return "IMULQ"
	case MULSD:
		return "MULSD"
	case MULSS:
		return "MULSS"
	case ORL:
		return "ORL"
	case ORPD:
		return "ORPD"
	case ORPS:
		return "ORPS"
	case ORQ:
		return "ORQ"
	case POPCNTL:
		return "POPCNTL"
	case POPCNTQ:
		return "POPCNTQ"
	case PSLLD:
		return "PSLLD"
	case PSLLQ:
		return "PSLLQ"
	case PSRLD:
		return "PSRLD"
	case PSRLQ:
		return "PSRLQ"
	case REPMOVSQ:
		return "REP MOVSQ"
	case REPSTOSQ:
		return "REP STOSQ"
	case ROLL:
		return "ROLL"
	case ROLQ:
		return "ROLQ"
	case RORL:
		return "RORL"
	case RORQ:
		return "RORQ"
	case ROUNDSD:
		return "ROUNDSD"
	case ROUNDSS:
		return "ROUNDSS"
	case SARL:
		return "SARL"
	case SARQ:
		return "SARQ"
	case SETCC:
		return "SETCC"
	case SETCS:
		return "SETCS"
	case SETEQ:
		return "SETEQ"
	case SETGE:
		return "SETGE"
	case SETGT:
		return "SETGT"
	case SETHI:
		return "SETHI"
	case SETLE:
		return "SETLE"
	case SETLS:
		return "SETLS"
	case SETLT:
		return "SETLT"
	case SETMI:
		return "SETMI"
	case SETNE:
		return "SETNE"
	case SETPC:
		return "SETPC"
	case SETPL:
		return "SETPL"
	case SETPS:
		return "SETPS"
	case SHLL:
		return "SHLL"
	case SHLQ:
		return "SHLQ"
	case SHRL:
		return "SHRL"
	case SHRQ:
		return "SHRQ"
	case SQRTSD:
		return "SQRTSD"
	case SQRTSS:
		return "SQRTSS"
	case STD:
		return "STD"
	case SUBL:
		return "SUBL"
	case SUBQ:
		return "SUBQ"
	case SUBSD:
		return "SUBSD"
	case SUBSS:
		return "SUBSS"
	case TESTL:
		return "TESTL"
	case TESTQ:
		return "TESTQ"
	case TZCNTL:
		return "TZCNTL"
	case TZCNTQ:
		return "TZCNTQ"
	case UCOMISD:
		return "UCOMISD"
	case UCOMISS:
		return "UCOMISS"
	case XORL:
		return "XORL"
	case XORPD:
		return "XORPD"
	case XORPS:
		return "XORPS"
	case XORQ:
		return "XORQ"
	case XCHGQ:
		return "XCHGQ"
	case RET:
		return "RET"
	case JMP:
		return "JMP"
	case NOP:
		return "NOP"
	case UD2:
		return "UD2"
	case MOVDQU:
		return "MOVDQU"
	case PINSRB:
		return "PINSRB"
	case PINSRW:
		return "PINSRW"
	case PINSRD:
		return "PINSRD"
	case PINSRQ:
		return "PINSRQ"
	case PADDB:
		return "PADDB"
	case PADDW:
		return "PADDW"
	case PADDD:
		return "PADDD"
	case PADDQ:
		return "PADDQ"
	case ADDPS:
		return "ADDPS"
	case ADDPD:
		return "ADDPD"
	case PSUBB:
		return "PSUBB"
	case PSUBW:
		return "PSUBW"
	case PSUBD:
		return "PSUBD"
	case PSUBQ:
		return "PSUBQ"
	case SUBPS:
		return "SUBPS"
	case SUBPD:
		return "SUBPD"
	case PMOVSXBW:
		return "PMOVSXBW"
	case PMOVSXWD:
		return "PMOVSXWD"
	case PMOVSXDQ:
		return "PMOVSXDQ"
	case PMOVZXBW:
		return "PMOVZXBW"
	case PMOVZXWD:
		return "PMOVZXWD"
	case PMOVZXDQ:
		return "PMOVZXDQ"
	case PSHUFB:
		return "PSHUFB"
	case PSHUFD:
		return "PSHUFD"
	case PXOR:
		return "PXOR"
	case PEXTRB:
		return "PEXTRB"
	case PEXTRW:
		return "PEXTRW"
	case PEXTRD:
		return "PEXTRD"
	case PEXTRQ:
		return "PEXTRQ"
	case INSERTPS:
		return "INSERTPS"
	case MOVLHPS:
		return "MOVLHPS"
	case PTEST:
		return "PTEST"
	case PCMPEQB:
		return "PCMPEQB"
	case PCMPEQW:
		return "PCMPEQW"
	case PCMPEQD:
		return "PCMPEQD"
	case PCMPEQQ:
		return "PCMPEQQ"
	case PADDUSB:
		return "PADDUSB"
	case MOVDQA:
		return "MOVDQA"
	case MOVSD:
		return "MOVSD"
	case PACKSSWB:
		return "PACKSSWB"
	case PMOVMSKB:
		return "PMOVMSKB"
	case MOVMSKPS:
		return "MOVMSKPS"
	case MOVMSKPD:
		return "MOVMSKPD"
	case PAND:
		return "PAND"
	case POR:
		return "POR"
	case PANDN:
		return "PANDN"
	case PSRAD:
		return "PSRAD"
	case PSRAW:
		return "PSRAW"
	case PSRLW:
		return "PSRLW"
	case PSLLW:
		return "PSLLW"
	case PUNPCKLBW:
		return "PUNPCKLBW"
	case PUNPCKHBW:
		return "PUNPCKHBW"
	case NEGQ:
		return "NEGQ"
	case NONE:
		return "NONE"
	case CMPPS:
		return "CMPPS"
	case CMPPD:
		return "CMPPD"
	case PCMPGTQ:
		return "PCMPGTQ"
	case PCMPGTD:
		return "PCMPGTD"
	case PMINSD:
		return "PMINSD"
	case PMAXSD:
		return "PMAXSD"
	case PMINSW:
		return "PMINSW"
	case PCMPGTB:
		return "PCMPGTB"
	case PMINSB:
		return "PMINSB"
	case PMINUD:
		return "PMINUD"
	case PMINUW:
		return "PMINUW"
	case PMINUB:
		return "PMINUB"
	case PMAXUD:
		return "PMAXUD"
	case PMAXUW:
		return "PMAXUW"
	case PMAXUB:
		return "PMAXUB"
	case PCMPGTW:
		return "PCMPGTW"
	case PMAXSW:
		return "PMAXSW"
	case PMAXSB:
		return "PMAXSB"
	case PMULLW:
		return "PMULLW"
	case PMULLD:
		return "PMULLD"
	case PMULUDQ:
		return "PMULUDQ"
	case PSUBSB:
		return "PSUBSB"
	case PSUBUSB:
		return "PSUBUSB"
	case PADDSW:
		return "PADDSW"
	case PADDSB:
		return "PADDSB"
	case PADDUSW:
		return "PADDUSW"
	case PSUBSW:
		return "PSUBSW"
	case PSUBUSW:
		return "PSUBUSW"
	case PAVGB:
		return "PAVGB"
	case PAVGW:
		return "PAVGW"
	case PABSB:
		return "PABSB"
	case PABSW:
		return "PABSW"
	case PABSD:
		return "PABSD"
	case BLENDVPD:
		return "BLENDVPD"
	case MAXPD:
		return "MAXPD"
	case MAXPS:
		return "MAXPS"
	case MINPD:
		return "MINPD"
	case MINPS:
		return "MINPS"
	case ANDNPD:
		return "ANDNPD"
	case ANDNPS:
		return "ANDNPS"
	case MULPS:
		return "MULPS"
	case MULPD:
		return "MULPD"
	case DIVPS:
		return "DIVPS"
	case DIVPD:
		return "DIVPD"
	case SQRTPS:
		return "SQRTPS"
	case SQRTPD:
		return "SQRTPD"
	case ROUNDPS:
		return "ROUNDPS"
	case ROUNDPD:
		return "ROUNDPD"
	case PALIGNR:
		return "PALIGNR"
	case PUNPCKLWD:
		return "PUNPCKLWD"
	case PUNPCKHWD:
		return "PUNPCKHWD"
	case PMULHUW:
		return "PMULHUW"
	case PMULDQ:
		return "PMULDQ"
	case PMULHRSW:
		return "PMULHRSW"
	case PMULHW:
		return "PMULHW"
	case CMPEQPS:
		return "CMPEQPS"
	case CMPEQPD:
		return "CMPEQPD"
	case CVTTPS2DQ:
		return "CVTTPS2DQ"
	case CVTDQ2PS:
		return "CVTDQ2PS"
	case MOVUPD:
		return "MOVUPD"
	case SHUFPS:
		return "SHUFPS"
	case PMADDWD:
		return "PMADDWD"
	case CVTDQ2PD:
		return "CVTDQ2PD"
	case UNPCKLPS:
		return "UNPCKLPS"
	case PACKUSWB:
		return "PACKUSWB"
	case PACKSSDW:
		return "PACKSSDW"
	case PACKUSDW:
		return "PACKUSDW"
	case CVTPS2PD:
		return "CVTPS2PD"
	case CVTPD2PS:
		return "CVTPD2PS"
	case PMADDUBSW:
		return "PMADDUBSW"
	case CVTTPD2DQ:
		return "CVTTPD2DQ"
	}
	panic(fmt.Errorf("unknown instruction %d", instruction))
}

// Amd64-specific registers.
//
// Note: naming convention intentionally matches the Go assembler: https://go.dev/doc/asm
// See https://www.lri.fr/~filliatr/ens/compil/x86-64.pdf
// See https://cs.brown.edu/courses/cs033/docs/guides/x64_cheatsheet.pdf
const (
	// RegAX is the ax register
	RegAX = asm.NilRegister + 1 + iota
	// RegCX is the cx register
	RegCX
	// RegDX is the dx register
	RegDX
	// RegBX is the bx register
	RegBX
	// RegSP is the sp register
	RegSP
	// RegBP is the bp register
	RegBP
	// RegSI is the si register
	RegSI
	// RegDI is the di register
	RegDI
	// RegR8 is the r8 register
	RegR8
	// RegR9 is the r9 register
	RegR9
	// RegR10 is the r10 register
	RegR10
	// RegR11 is the r11 register
	RegR11
	// RegR12 is the r12 register
	RegR12
	// RegR13 is the r13 register
	RegR13
	// RegR14 is the r14 register
	RegR14
	// RegR15 is the r15 register
	RegR15
	// RegX0 is the x0 register
	RegX0
	// RegX1 is the x1 register
	RegX1
	// RegX2 is the x2 register
	RegX2
	// RegX3 is the x3 register
	RegX3
	// RegX4 is the x4 register
	RegX4
	// RegX5 is the x5 register
	RegX5
	// RegX6 is the x6 register
	RegX6
	// RegX7 is the x7 register
	RegX7
	// RegX8 is the x8 register
	RegX8
	// RegX9 is the x9 register
	RegX9
	// RegX10 is the x10 register
	RegX10
	// RegX11 is the x11 register
	RegX11
	// RegX12 is the x12 register
	RegX12
	// RegX13 is the x13 register
	RegX13
	// RegX14 is the x14 register
	RegX14
	// Regx15 is the x15 register
	RegX15
)

// RegisterName returns the name for a register
func RegisterName(reg asm.Register) string {
	switch reg {
	case RegAX:
		return "AX"
	case RegCX:
		return "CX"
	case RegDX:
		return "DX"
	case RegBX:
		return "BX"
	case RegSP:
		return "SP"
	case RegBP:
		return "BP"
	case RegSI:
		return "SI"
	case RegDI:
		return "DI"
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
	case RegX0:
		return "X0"
	case RegX1:
		return "X1"
	case RegX2:
		return "X2"
	case RegX3:
		return "X3"
	case RegX4:
		return "X4"
	case RegX5:
		return "X5"
	case RegX6:
		return "X6"
	case RegX7:
		return "X7"
	case RegX8:
		return "X8"
	case RegX9:
		return "X9"
	case RegX10:
		return "X10"
	case RegX11:
		return "X11"
	case RegX12:
		return "X12"
	case RegX13:
		return "X13"
	case RegX14:
		return "X14"
	case RegX15:
		return "X15"
	default:
		return "nil"
	}
}
