package compiler

import (
	"math"

	"github.com/tetratelabs/wazero/internal/asm/arm64"
)

// init initializes variables for the arm64 architecture
func init() {
	newArchContext = newArchContextImpl
	registerNameFn = arm64.RegisterName
	unreservedGeneralPurposeRegisters = arm64UnreservedGeneralPurposeRegisters
	unreservedVectorRegisters = arm64UnreservedVectorRegisters
}

// archContext is embedded in callEngine in order to store architecture-specific data.
type archContext struct {
	// compilerCallReturnAddress holds the absolute return address for nativecall.
	// The value is set whenever nativecall is executed and done in compiler_arm64.s
	// Native code can return to the ce.execWasmFunction's main loop back by
	// executing "ret" instruction with this value. See arm64Compiler.exit.
	// Note: this is only used by Compiler code so mark this as nolint.
	compilerCallReturnAddress uint64 //nolint

	// Loading large constants in arm64 is a bit costly, so we place the following
	// consts on callEngine struct so that we can quickly access them during various operations.

	// minimum32BitSignedInt is used for overflow check for 32-bit signed division.
	// Note: this can be obtained by moving $1 and doing left-shift with 31, but it is
	// slower than directly loading from this location.
	minimum32BitSignedInt int32
	// Note: this can be obtained by moving $1 and doing left-shift with 63, but it is
	// slower than directly loading from this location.
	// minimum64BitSignedInt is used for overflow check for 64-bit signed division.
	minimum64BitSignedInt int64
}

// newArchContextImpl implements newArchContext for amd64 architecture.
func newArchContextImpl() archContext {
	return archContext{
		minimum32BitSignedInt: math.MinInt32,
		minimum64BitSignedInt: math.MinInt64,
	}
}

// newCompiler returns a new compiler interface which can be used to compile the given function instance.
// Note: ir param can be nil for host functions.
func newCompiler() compiler {
	return newArm64Compiler()
}
