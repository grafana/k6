package compiler

import (
	"github.com/dop251/goja"
)

func init() {
	c, err := New()
	if err != nil {
		panic(err)
	}
	DefaultCompiler = c
}

var DefaultCompiler *Compiler

func Transform(src, filename string) (code string, srcmap SourceMap, err error) {
	return DefaultCompiler.Transform(src, filename)
}

func Compile(src, filename string, pre, post string, strict bool) (*goja.Program, string, error) {
	return DefaultCompiler.Compile(src, filename, pre, post, strict)
}
