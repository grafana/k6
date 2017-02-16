package compiler

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
