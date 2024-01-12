package webcrypto

import (
	"io"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/js/modulestest"
)

// newTestSetup initializes a new test setup.
// It prepares a test setup with a mocked redis server and a goja runtime,
// and event loop, ready to execute scripts as if being executed in the
// main context of k6.
func newTestSetup(t testing.TB) *modulestest.Runtime {
	ts := modulestest.NewRuntime(t)

	// We compile the Web Platform testharness script into a goja.Program
	harnessProgram, err := CompileFile("./tests/util", "testharness.js")
	require.NoError(t, err)

	// We execute the harness script in the goja runtime
	// in order to make the Web Platform assertion functions available
	// to the tests.
	_, err = ts.VU.Runtime().RunProgram(harnessProgram)
	require.NoError(t, err)

	// We compile the Web Platform helpers script into a goja.Program
	helpersProgram, err := CompileFile("./tests/util", "helpers.js")
	require.NoError(t, err)
	// We execute the helpers script in the goja runtime
	// in order to make the Web Platform helpers available
	// to the tests.
	_, err = ts.VU.Runtime().RunProgram(helpersProgram)
	require.NoError(t, err)

	m := new(RootModule).NewModuleInstance(ts.VU)
	require.NoError(t, ts.VU.Runtime().Set("crypto", m.Exports().Named["crypto"]))
	require.NoError(t, ts.VU.Runtime().GlobalObject().Set("CryptoKey", CryptoKey{}))

	return ts
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
