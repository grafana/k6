//go:build wpt

package streams

import (
	"testing"

	"github.com/dop251/goja"
	"go.k6.io/k6/js/modules/k6/timers"
	"go.k6.io/k6/js/modulestest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadableStream(t *testing.T) {
	t.Parallel()

	suites := []string{
		"bad-strategies.any.js",
		"bad-underlying-sources.any.js",
		"cancel.any.js",
		"constructor.any.js",
		"count-queuing-strategy-integration.any.js",
		"default-reader.any.js",
		"floating-point-total-queue-size.any.js",
		"general.any.js",
		"reentrant-strategies.any.js",
		"templated.any.js",
	}

	for _, s := range suites {
		s := s
		t.Run(s, func(t *testing.T) {
			t.Parallel()
			ts := newConfiguredRuntime(t)
			gotErr := ts.EventLoop.Start(func() error {
				return executeTestScripts(ts.VU.Runtime(), "tests/wpt/streams/readable-streams", s)
			})
			assert.NoError(t, gotErr)
		})
	}
}

func newConfiguredRuntime(t testing.TB) *modulestest.Runtime {
	rt := modulestest.NewRuntime(t)

	// We want to make the [self] available for Web Platform Tests, as it is used in test harness.
	_, err := rt.VU.Runtime().RunString("var self = this;")
	require.NoError(t, err)

	// We also want to make [timers.Timers] available for Web Platform Tests.
	for k, v := range timers.New().NewModuleInstance(rt.VU).Exports().Named {
		require.NoError(t, rt.VU.RuntimeField.Set(k, v))
	}

	// We also want the streams module exports to be globally available.
	m := new(RootModule).NewModuleInstance(rt.VU)
	for k, v := range m.Exports().Named {
		require.NoError(t, rt.VU.RuntimeField.Set(k, v))
	}

	// Then, we register the Web Platform Tests harness.
	compileAndRun(t, rt, "tests/wpt", "resources/testharness.js")

	// And the Streams-specific test utilities.
	files := []string{
		"resources/rs-test-templates.js",
		"resources/rs-utils.js",
		"resources/test-utils.js",
	}
	for _, file := range files {
		compileAndRun(t, rt, "tests/wpt/streams", file)
	}

	return rt
}

func compileAndRun(t testing.TB, runtime *modulestest.Runtime, base, file string) {
	program, err := modulestest.CompileFile(base, file)
	require.NoError(t, err)

	_, err = runtime.VU.Runtime().RunProgram(program)
	require.NoError(t, err)
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
