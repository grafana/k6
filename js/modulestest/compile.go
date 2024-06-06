package modulestest

import (
	"os"
	"path"
	"path/filepath"

	"github.com/grafana/sobek"
)

// CompileFile compiles a JS file as a [*sobek.Program].
//
// The base path is used to resolve the file path. The name is the file name.
//
// This function facilitates evaluating javascript test files in a [sobek.Runtime] using
// the [sobek.Runtime.RunProgram] method.
func CompileFile(base, name string) (*sobek.Program, error) {
	b, err := os.ReadFile(filepath.Clean(path.Join(base, name))) //nolint:forbidigo
	if err != nil {
		return nil, err
	}

	return sobek.Compile(name, string(b), false)
}
