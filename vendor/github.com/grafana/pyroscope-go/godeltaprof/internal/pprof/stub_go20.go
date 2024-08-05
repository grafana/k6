//go:build go1.16 && !go1.21
// +build go1.16,!go1.21

package pprof

import "runtime"

// runtime_FrameStartLine is defined in runtime/symtab.go.
func runtime_FrameStartLine(f *runtime.Frame) int {
	return 0
}

// runtime_FrameSymbolName is defined in runtime/symtab.go.
func runtime_FrameSymbolName(f *runtime.Frame) string {
	return f.Function
}
