//go:build wpt

package tests

import (
	"testing"

	k6encoding "go.k6.io/k6/internal/js/modules/k6/encoding"
	"go.k6.io/k6/internal/js/modules/k6/webcrypto"
	"go.k6.io/k6/js/modulestest"

	"github.com/stretchr/testify/require"
)

const initGlobals = `
	globalThis.CryptoKey = crypto.CryptoKey;
`

// newConfiguredRuntime initializes a new test setup.
// It prepares a test setup with a mocked redis server and a goja runtime,
// and event loop, ready to execute scripts as if being executed in the
// main context of k6.
func newConfiguredRuntime(t testing.TB) *modulestest.Runtime {
	var err error
	rt := modulestest.NewRuntime(t)

	// We want to make the [self] available for Web Platform Tests, as it is used in test harness.
	_, err = rt.VU.Runtime().RunString("var self = this;")
	require.NoError(t, err)

	require.NoError(t, webcrypto.SetupGlobally(rt.VU))

	// We compile the Web Platform testharness script into a sobek.Program
	compileAndRun(t, rt, "./wpt/resources", "testharness.js")

	// We compile the Web Platform helpers script into a sobek.Program
	// TODO: check if we need to compile the helpers.js script each time
	// or it can be just yet another test
	compileAndRun(t, rt, "./util", "helpers.js")

	m := new(webcrypto.RootModule).NewModuleInstance(rt.VU)

	err = rt.VU.Runtime().Set("crypto", m.Exports().Named["crypto"])
	require.NoError(t, err)

	// we define the btoa function in the goja runtime
	// so that the Web Platform tests can use it.
	encodingModule := k6encoding.New().NewModuleInstance(rt.VU)
	err = rt.VU.Runtime().Set("btoa", encodingModule.Exports().Named["b64encode"])
	require.NoError(t, err)

	_, err = rt.VU.Runtime().RunString(initGlobals)
	require.NoError(t, err)

	return rt
}

func compileAndRun(t testing.TB, runtime *modulestest.Runtime, base, file string) {
	program, err := modulestest.CompileFile(base, file)
	require.NoError(t, err)

	_, err = runtime.VU.Runtime().RunProgram(program)
	require.NoError(t, err)
}
