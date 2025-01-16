//go:build wpt

package streams

import (
	"testing"

	"go.k6.io/k6/js/modules"
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

	for _, suite := range suites {
		suite := suite
		t.Run(suite, func(t *testing.T) {
			t.Parallel()
			ts := newConfiguredRuntime(t)
			gotErr := ts.EventLoop.Start(func() error {
				return executeTestScript(ts.VU, "tests/wpt/streams/readable-streams", suite)
			})
			assert.NoError(t, gotErr)
		})
	}
}

func newConfiguredRuntime(t testing.TB) *modulestest.Runtime {
	rt := modulestest.NewRuntime(t)

	require.NoError(t, rt.SetupModuleSystem(nil, nil, nil))

	// We want to make the [self] available for Web Platform Tests, as it is used in test harness.
	_, err := rt.VU.Runtime().RunString("var self = this;")
	require.NoError(t, err)

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

func executeTestScript(vu modules.VU, base string, script string) error {
	program, err := modulestest.CompileFile(base, script)
	if err != nil {
		return err
	}

	if _, err = vu.Runtime().RunProgram(program); err != nil {
		return err
	}

	// After having executed the tests suite file,
	// we use a callback to make sure we wait until all
	// the promise-based tests have finished.
	// Also, as a mechanism to capture deadlocks caused
	// by those promises not resolved during normal execution.
	callback := vu.RegisterCallback()
	if err := vu.Runtime().Set("wait", func() {
		callback(func() error { return nil })
	}); err != nil {
		return err
	}

	waitForPromiseTests := `
if (this.tests && this.tests.promise_tests && typeof this.tests.promise_tests.then === 'function') {
	this.tests.promise_tests.then(() => wait());
} else {
	wait();
}
`
	if _, err = vu.Runtime().RunString(waitForPromiseTests); err != nil {
		return err
	}

	return nil
}
