package encoding

import (
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/js/modulestest"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
)

// testScript is a helper struct holding the base path
// and the path of a test script.
type testScript struct {
	base string
	path string
}

// testSetup is a helper struct holding components
// necessary to test the redis client, in the context
// of the execution of a k6 script.
type testSetup struct {
	rt      *modulestest.Runtime
	state   *lib.State
	samples chan metrics.SampleContainer
}

// newTestSetup initializes a new test setup.
// It prepares a test setup with a mocked redis server and a sobek runtime,
// and event loop, ready to execute scripts as if being executed in the
// main context of k6.
func newTestSetup(t testing.TB) *testSetup {
	rt := modulestest.NewRuntime(t)
	rt.VU.RuntimeField.SetFieldNameMapper(sobek.TagFieldNameMapper("json", true))

	samples := make(chan metrics.SampleContainer, 1000)
	m := new(RootModule).NewModuleInstance(rt.VU)
	err := rt.VU.RuntimeField.Set("TextDecoder", m.Exports().Named["TextDecoder"])
	if err != nil {
		t.Fatalf("failed to set test setup's TextDecoder: %v", err)
	}
	err = rt.VU.RuntimeField.Set("TextEncoder", m.Exports().Named["TextEncoder"])
	if err != nil {
		t.Fatalf("failed to set test setup's TextEncoder: %v", err)
	}
	ts := &testSetup{
		rt:      rt,
		state:   rt.VU.State(),
		samples: samples,
	}
	err = testExecuteTestScripts(ts)
	require.NoError(t, err)
	return ts
}

func testExecuteTestScripts(ts *testSetup) error {
	scripts := []testScript{
		{base: "./tests/utils", path: "assert.js"},
		{base: "./tests/resources", path: "encodings.js"},
	}

	return executeTestScripts(ts, scripts)
}

func executeTestScripts(ts *testSetup, scripts []testScript) error {
	for _, script := range scripts {
		program, err := modulestest.CompileFile(script.base, script.path)
		if err != nil {
			return err
		}

		_, err = ts.rt.VU.RuntimeField.RunProgram(program)
		if err != nil {
			return err
		}
	}

	return nil
}
