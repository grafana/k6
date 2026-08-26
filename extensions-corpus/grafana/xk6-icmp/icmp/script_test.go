package icmp

import (
	_ "embed"
	"path/filepath"
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/js/modulestest"
)

func runScriptTest(t *testing.T, filename string) {
	t.Helper()

	runtime := newTestRuntime(t)
	state := newTestVUState(t)

	module := runtime.VU.Runtime().NewObject()
	exports := runtime.VU.Runtime().NewObject()

	require.NoError(t, module.Set("exports", exports))
	require.NoError(t, runtime.VU.Runtime().Set("module", module))

	prog, err := modulestest.CompileFile(filepath.Dir(filename), filepath.Base(filename))
	require.NoError(t, err)

	_, err = runtime.VU.Runtime().RunProgram(prog)
	require.NoError(t, err)

	runtime.MoveToVUContext(state)

	result, err := runtime.VU.Runtime().RunString("module.exports")
	require.NoError(t, err)

	get := result.ToObject(runtime.VU.Runtime()).Get

	if fn, ok := sobek.AssertFunction(get("setup")); ok {
		_, err = fn(sobek.Undefined())

		require.NoError(t, err)
		runtime.EventLoop.WaitOnRegistered()
	}

	fn, ok := sobek.AssertFunction(result)
	require.True(t, ok, "module.exports should be a function")

	_, err = fn(sobek.Undefined())
	require.NoError(t, err)

	runtime.EventLoop.WaitOnRegistered()

	if fn, ok = sobek.AssertFunction(get("teardown")); ok {
		_, err = fn(sobek.Undefined())

		require.NoError(t, err)
		runtime.EventLoop.WaitOnRegistered()
	}
}

func Test_script(t *testing.T) { //nolint:tparallel
	t.Parallel()

	skipOnGitHubLinuxRunner(t)

	files, err := filepath.Glob("testdata/*_test.cjs")

	require.NoError(t, err)
	require.NotEmpty(t, files, "No test scripts found in testdata directory")

	for _, file := range files { //nolint:paralleltest
		t.Run(filepath.ToSlash(file), func(t *testing.T) {
			runScriptTest(t, file)
		})
	}
}
