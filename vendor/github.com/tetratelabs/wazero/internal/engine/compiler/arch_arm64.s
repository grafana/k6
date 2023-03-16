#include "funcdata.h"
#include "textflag.h"

// nativecall(codeSegment, ce, moduleInstanceAddress)
TEXT ·nativecall(SB), NOSPLIT|NOFRAME, $0-24
	// Load the address of *callEngine into arm64ReservedRegisterForCallEngine.
	MOVD ce+8(FP), R0

	// In arm64, return address is stored in R30 after jumping into the code.
	// We save the return address value into archContext.compilerReturnAddress in Engine.
	// Note that the const 136 drifts after editting Engine or archContext struct. See TestArchContextOffsetInEngine.
	MOVD R30, 136(R0)

	// Load the address of *wasm.ModuleInstance into arm64CallingConventionModuleInstanceAddressRegister.
	MOVD moduleInstanceAddress+16(FP), R29

	// Load the address of native code.
	MOVD codeSegment+0(FP), R1

	// Jump to native code.
	JMP (R1)
