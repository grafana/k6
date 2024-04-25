package modulestest

import (
	"os"
	"path"
	"path/filepath"

	"github.com/dop251/goja"
)

// CompileFile compiles a JS file as a [*goja.Program].
//
// The base path is used to resolve the file path. The name is the file name.
//
// This function facilitates evaluating javascript test files in a [goja.Runtime] using
// the [goja.Runtime.RunProgram] method.
func CompileFile(base, name string) (*goja.Program, error) {
	b, err := os.ReadFile(filepath.Clean(path.Join(base, name))) //nolint:forbidigo
	if err != nil {
		return nil, err
	}

	return goja.Compile(name, string(b), false)
}
