package webcrypto

import (
	"go.k6.io/k6/js/compiler"
	"io"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/dop251/goja"
	"go.k6.io/k6/js/modulestest"
)

const initGlobals = `
	globalThis.CryptoKey = require("k6/x/webcrypto").CryptoKey;
`

// newConfiguredRuntime initializes a new test setup.
// It prepares a test setup with a mocked redis server and a goja runtime,
// and event loop, ready to execute scripts as if being executed in the
// main context of k6.
func newConfiguredRuntime(t testing.TB) (*modulestest.Runtime, error) {
	var err error
	runtime := modulestest.NewRuntime(t)

	err = runtime.SetupModuleSystem(
		map[string]interface{}{"k6/x/webcrypto": New()},
		nil,
		compiler.New(runtime.VU.InitEnv().Logger),
	)
	if err != nil {
		return nil, err
	}

	// We compile the Web Platform testharness script into a goja.Program
	harnessProgram, err := CompileFile("./tests/util", "testharness.js")
	if err != nil {
		return nil, err
	}

	// We execute the harness script in the goja runtime
	// in order to make the Web Platform assertion functions available
	// to the tests.
	_, err = runtime.VU.Runtime().RunProgram(harnessProgram)
	if err != nil {
		return nil, err
	}

	// We compile the Web Platform helpers script into a goja.Program
	helpersProgram, err := CompileFile("./tests/util", "helpers.js")
	if err != nil {
		return nil, err
	}

	// We execute the helpers script in the goja runtime
	// in order to make the Web Platform helpers available
	// to the tests.
	_, err = runtime.VU.Runtime().RunProgram(helpersProgram)
	if err != nil {
		return nil, err
	}

	m := new(RootModule).NewModuleInstance(runtime.VU)

	if err = runtime.VU.Runtime().Set("crypto", m.Exports().Named["crypto"]); err != nil {
		return nil, err
	}

	_, err = runtime.VU.Runtime().RunString(initGlobals)
	if err != nil {
		return nil, err
	}

	return runtime, nil
}

// CompileFile compiles a javascript file as a goja.Program.
func CompileFile(base, name string) (*goja.Program, error) {
	fname := path.Join(base, name)

	//nolint:forbidigo // Allow os.Open in tests
	f, err := os.Open(filepath.Clean(fname))
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

	str := string(b)
	program, err := goja.Compile(name, str, false)
	if err != nil {
		return nil, err
	}

	return program, nil
}
