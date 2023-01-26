package tracing

import (
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/js/modules/k6/http"
	"go.k6.io/k6/js/modulestest"
	"go.k6.io/k6/lib"
)

func TestInstrumentHTTP_SucceedsInInitContext(t *testing.T) {
	t.Parallel()

	ts := modulestest.NewRuntime(t)
	m := new(RootModule).NewModuleInstance(ts.VU)

	rt := ts.VU.Runtime()
	require.NoError(t, rt.Set("instrumentHTTP", m.Exports().Named["instrumentHTTP"]))

	require.NoError(t, rt.Set("require", func(module string) *goja.Object {
		require.Equal(t, "k6/http", module)
		export := http.New().NewModuleInstance(ts.VU).Exports().Default

		return rt.ToValue(export).ToObject(rt)
	}))

	// Calling in the init context should succeed
	_, err := ts.VU.Runtime().RunString(`
		instrumentHTTP({propagator: 'w3c'})
	`)

	assert.NoError(t, err)
}

func TestInstrumentHTTP_FailsWhenCalledTwice(t *testing.T) {
	t.Parallel()

	ts := modulestest.NewRuntime(t)
	m := new(RootModule).NewModuleInstance(ts.VU)

	rt := ts.VU.Runtime()
	require.NoError(t, rt.Set("instrumentHTTP", m.Exports().Named["instrumentHTTP"]))

	require.NoError(t, rt.Set("require", func(module string) *goja.Object {
		require.Equal(t, "k6/http", module)
		export := http.New().NewModuleInstance(ts.VU).Exports().Default

		return rt.ToValue(export).ToObject(rt)
	}))

	// Calling it twice in the init context should fail
	_, err := ts.VU.Runtime().RunString(`
		instrumentHTTP({propagator: 'w3c'})
		instrumentHTTP({propagator: 'w3c'})
	`)

	assert.Error(t, err)
}

func TestInstrumentHTTP_FailsInVUContext(t *testing.T) {
	t.Parallel()

	ts := modulestest.NewRuntime(t)
	ts.MoveToVUContext(&lib.State{})
	m := new(RootModule).NewModuleInstance(ts.VU)

	rt := ts.VU.Runtime()
	require.NoError(t, rt.Set("instrumentHTTP", m.Exports().Named["instrumentHTTP"]))

	require.NoError(t, rt.Set("require", func(module string) *goja.Object {
		require.Equal(t, "k6/http", module)
		export := http.New().NewModuleInstance(ts.VU).Exports().Default

		return rt.ToValue(export).ToObject(rt)
	}))

	// Calling in the VU context should fail
	_, err := ts.VU.Runtime().RunString(`
		instrumentHTTP({propagator: 'w3c'})
	`)

	assert.Error(t, err)
}
