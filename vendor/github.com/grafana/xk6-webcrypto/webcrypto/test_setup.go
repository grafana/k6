package webcrypto

import (
	"io"
	"os"
	"path"
	"path/filepath"
	"testing"

	"go.k6.io/k6/js/compiler"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/require"
	k6encoding "go.k6.io/k6/js/modules/k6/encoding"
	"go.k6.io/k6/js/modulestest"
)

const initGlobals = `
	globalThis.CryptoKey = require("k6/x/webcrypto").CryptoKey;
`

// newConfiguredRuntime initializes a new test setup.
// It prepares a test setup with a mocked redis server and a goja runtime,
// and event loop, ready to execute scripts as if being executed in the
// main context of k6.
func newConfiguredRuntime(t testing.TB) *modulestest.Runtime {
	var err error
	runtime := modulestest.NewRuntime(t)

	err = runtime.SetupModuleSystem(
		map[string]interface{}{"k6/x/webcrypto": New()},
		nil,
		compiler.New(runtime.VU.InitEnv().Logger),
	)
	require.NoError(t, err)

	// We compile the Web Platform testharness script into a sobek.Program
	harnessProgram, err := CompileFile("./tests/util", "testharness.js")
	require.NoError(t, err)

	// We execute the harness script in the goja runtime
	// in order to make the Web Platform assertion functions available
	// to the tests.
	_, err = runtime.VU.Runtime().RunProgram(harnessProgram)
	require.NoError(t, err)

	// We compile the Web Platform helpers script into a sobek.Program
	helpersProgram, err := CompileFile("./tests/util", "helpers.js")
	require.NoError(t, err)

	// We execute the helpers script in the goja runtime
	// in order to make the Web Platform helpers available
	// to the tests.
	_, err = runtime.VU.Runtime().RunProgram(helpersProgram)
	require.NoError(t, err)

	m := new(RootModule).NewModuleInstance(runtime.VU)

	err = runtime.VU.Runtime().Set("crypto", m.Exports().Named["crypto"])
	require.NoError(t, err)

	// we define the btoa function in the goja runtime
	// so that the Web Platform tests can use it.
	encodingModule := k6encoding.New().NewModuleInstance(runtime.VU)
	err = runtime.VU.Runtime().Set("btoa", encodingModule.Exports().Named["b64encode"])
	require.NoError(t, err)

	_, err = runtime.VU.Runtime().RunString(initGlobals)
	require.NoError(t, err)

	return runtime
}

// CompileFile compiles a javascript file as a sobek.Program.
func CompileFile(base, name string) (*sobek.Program, error) {
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

	str := string(b)
	program, err := sobek.Compile(name, str, false)
	if err != nil {
		return nil, err
	}

	return program, nil
}
