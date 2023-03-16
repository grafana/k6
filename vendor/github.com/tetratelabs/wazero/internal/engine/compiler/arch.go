package compiler

import "github.com/tetratelabs/wazero/internal/asm"

// newArchContext returns a new archContext which is architecture-specific type to be embedded in callEngine.
// This must be initialized in init() function in architecture-specific arch_*.go file which is guarded by build tag.
var newArchContext func() archContext

// nativecall is used by callEngine.execWasmFunction and the entrypoint to enter the compiled native code.
// codeSegment is the pointer to the initial instruction of the compiled native code.
// ce is "*callEngine" as uintptr.
//
// Note: this is implemented in per-arch Go assembler file. For example, arch_amd64.s implements this for amd64.
func nativecall(codeSegment, ce uintptr, moduleInstanceAddress uintptr)

// registerNameFn is used for debugging purpose to have register symbols in the string of runtimeValueLocation.
var registerNameFn func(register asm.Register) string
