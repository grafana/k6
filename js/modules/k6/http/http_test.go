package http

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/js/modulestest"
	"go.k6.io/k6/lib/netext/httpext"
)

func getTestModuleInstance(t testing.TB) (*modulestest.Runtime, *ModuleInstance) {
	runtime := modulestest.NewRuntime(t)
	mi, ok := New().NewModuleInstance(runtime.VU).(*ModuleInstance)
	require.True(t, ok)

	require.NoError(t, runtime.VU.Runtime().Set("http", mi.Exports().Default))

	return runtime, mi
}

func TestTagURL(t *testing.T) {
	t.Parallel()

	testdata := map[string]struct{ u, n string }{
		`http://localhost/anything/`:               {"http://localhost/anything/", "http://localhost/anything/"},
		`http://localhost/anything/${1+1}`:         {"http://localhost/anything/2", "http://localhost/anything/${}"},
		`http://localhost/anything/${1+1}/`:        {"http://localhost/anything/2/", "http://localhost/anything/${}/"},
		`http://localhost/anything/${1+1}/${1+2}`:  {"http://localhost/anything/2/3", "http://localhost/anything/${}/${}"},
		`http://localhost/anything/${1+1}/${1+2}/`: {"http://localhost/anything/2/3/", "http://localhost/anything/${}/${}/"},
	}
	for expr, data := range testdata {
		expr, data := expr, data
		t.Run("expr="+expr, func(t *testing.T) {
			t.Parallel()
			runtime, _ := getTestModuleInstance(t)
			rt := runtime.VU.RuntimeField
			tag, err := httpext.NewURL(data.u, data.n)
			require.NoError(t, err)
			v, err := rt.RunString("http.url`" + expr + "`")
			require.NoError(t, err)
			assert.Equal(t, tag, v.Export())
		})
	}
}
