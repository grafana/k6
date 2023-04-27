package tracing

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/js/compiler"
	"go.k6.io/k6/js/modules/k6/http"
	"go.k6.io/k6/js/modulestest"
	"go.k6.io/k6/lib"
)

func TestInstrumentHTTP_SucceedsInInitContext(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)

	// Calling in the init context should succeed
	_, err := ts.TestRuntime.VU.Runtime().RunString(`
		instrumentHTTP({propagator: 'w3c'})
	`)

	assert.NoError(t, err)
}

func TestInstrumentHTTP_FailsWhenCalledTwice(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)

	// Calling it twice in the init context should fail
	_, err := ts.TestRuntime.VU.Runtime().RunString(`
		instrumentHTTP({propagator: 'w3c'})
		instrumentHTTP({propagator: 'w3c'})
	`)

	assert.Error(t, err)
}

func TestInstrumentHTTP_FailsInVUContext(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)
	ts.TestRuntime.MoveToVUContext(&lib.State{})

	// Calling in the VU context should fail
	_, err := ts.TestRuntime.VU.Runtime().RunString(`
		instrumentHTTP({propagator: 'w3c'})
	`)

	assert.Error(t, err)
}

type testSetup struct {
	t           *testing.T
	TestRuntime *modulestest.Runtime
}

func newTestSetup(t *testing.T) testSetup {
	ts := modulestest.NewRuntime(t)
	err := ts.SetupModuleSystem(map[string]interface{}{
		"k6/http":                 http.New(),
		"k6/experimental/tracing": new(RootModule),
	}, nil, compiler.New(ts.VU.InitEnvField.Logger))
	require.NoError(t, err)

	_, err = ts.VU.Runtime().RunString("var instrumentHTTP = require('k6/experimental/tracing').instrumentHTTP")
	require.NoError(t, err)

	return testSetup{
		t:           t,
		TestRuntime: ts,
	}
}
