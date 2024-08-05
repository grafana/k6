//go:build go1.21
// +build go1.21

package pprof

import (
	"runtime"
	_ "unsafe"
)

// runtime_FrameStartLine is defined in runtime/symtab.go.
//
//go:noescape
//go:linkname runtime_FrameStartLine runtime/pprof.runtime_FrameStartLine
func runtime_FrameStartLine(f *runtime.Frame) int

// runtime_FrameSymbolName is defined in runtime/symtab.go.
//
//go:noescape
//go:linkname runtime_FrameSymbolName runtime/pprof.runtime_FrameSymbolName
func runtime_FrameSymbolName(f *runtime.Frame) string
