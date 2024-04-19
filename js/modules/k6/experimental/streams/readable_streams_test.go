package streams

import (
	"testing"

	"github.com/dop251/goja"
	"go.k6.io/k6/js/compiler"
	"go.k6.io/k6/js/modulestest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadableStream(t *testing.T) {
	suites := []string{
		"bad-strategies.js",
		"bad-underlying-sources.js",
		"cancel.js",
		"constructor.js",
		"count-queuing-strategy-integration.js",
		"default-reader.js",
		"floating-point-total-queue-size.js",
		"general.js",
		"reentrant-strategies.js",
		"templated.js",
	}

	for _, s := range suites {
		t.Run(s, func(t *testing.T) {
			t.Parallel()
			ts := newConfiguredRuntime(t)
			gotErr := ts.EventLoop.Start(func() error {
				return executeTestScripts(ts.VU.Runtime(), "./tests/readable-streams", s)
			})
			assert.NoError(t, gotErr)
		})
	}
}

func newConfiguredRuntime(t testing.TB) *modulestest.Runtime {
	// We want a runtime with the Web Platform Tests harness available.
	runtime := modulestest.NewRuntimeForWPT(t)
	require.NoError(t, runtime.SetupModuleSystem(nil, nil, compiler.New(runtime.VU.InitEnv().Logger)))

	// We also want the streams module exports to be globally available.
	m := new(RootModule).NewModuleInstance(runtime.VU)
	for k, v := range m.Exports().Named {
		require.NoError(t, runtime.VU.RuntimeField.Set(k, v))
	}

	return runtime
}

func executeTestScripts(rt *goja.Runtime, base string, scripts ...string) error {
	for _, script := range scripts {
		program, err := modulestest.CompileFile(base, script)
		if err != nil {
			return err
		}

		if _, err = rt.RunProgram(program); err != nil {
			return err
		}
	}

	return nil
}
