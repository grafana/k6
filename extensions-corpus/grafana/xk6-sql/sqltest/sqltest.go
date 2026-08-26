// Package sqltest contains helper functions for driver integration tests of the xk6-sql extension.
package sqltest

import (
	"testing"

	"github.com/grafana/sobek"
	"github.com/grafana/xk6-sql/sql"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/ext"
	"go.k6.io/k6/v2/js/modules"
	"go.k6.io/k6/v2/js/modulestest"
)

// RunScript executes JavaScript code in a specially initialized interpreter.
// The "sql" variable contains the xk6-sql module,
// the "driver" variable contains the Symbol identifying the driver,
// and the "connection" variable contains the database connection string.
func RunScript(t *testing.T, driver string, connection string, script string) sobek.Value {
	t.Helper()

	runtime := modulestest.NewRuntime(t)
	vu := runtime.VU

	root := sql.New()
	m := root.NewModuleInstance(runtime.VU)

	require.NoError(t, vu.RuntimeField.Set("sql", m.Exports().Default))

	jsext, found := ext.Get(ext.JSExtension)["k6/x/sql/driver/"+driver]

	require.True(t, found, "Driver extension found: "+driver)

	jsmod, ok := jsext.Module.(modules.Module)

	require.True(t, ok, "Driver extension module is JavaScript module")

	m = jsmod.NewModuleInstance(vu)

	require.NoError(t, vu.RuntimeField.Set("driver", m.Exports().Default))
	require.NoError(t, vu.RuntimeField.Set("connection", connection))

	value, err := runtime.VU.RuntimeField.RunString(script)

	require.NoError(t, err)

	return value
}
