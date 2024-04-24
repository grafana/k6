package modulestest

import (
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/dop251/goja"
)

// CompileFile compiles a JS file as a [*goja.Program].
func CompileFile(base, name string) (*goja.Program, error) {
	filename := path.Join(base, name)

	//nolint:forbidigo // Allow os.Open in tests
	f, err := os.Open(filepath.Clean(filename))
	if err != nil {
		return nil, err
	}
	defer func() {
		err = f.Close()
		if err != nil {
			panic(err)
		}
	}()

	b, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	return compile(name, b)
}

func compile(name string, b []byte) (*goja.Program, error) {
	program, err := goja.Compile(name, string(b), false)
	if err != nil {
		return nil, err
	}

	return program, nil
}
