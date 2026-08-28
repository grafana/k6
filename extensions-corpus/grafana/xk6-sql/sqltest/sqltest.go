// Package sqltest contains helper functions for driver integration tests of the xk6-sql extension.
package sqltest

import (
	"testing"

	"github.com/grafana/sobek"
	"github.com/grafana/xk6-sql/sql"
	"github.com/stretchr/testify/require"
	extensionapi "go.k6.io/k6-extension-api"
	extensionapitest "go.k6.io/k6-extension-api/test"
)

// RunScript executes JavaScript code in a specially initialized interpreter.
// The "sql" variable contains the xk6-sql module,
// the "driver" variable contains the Symbol identifying the driver,
// and the "connection" variable contains the database connection string.
func RunScript(t *testing.T, driver string, connection string, script string) sobek.Value {
	t.Helper()

	runtime := extensionapitest.NewRuntime()
	vu := runtime.VU

	sqlModule := sql.New().NewModuleInstance(vu)

	require.NoError(t, vu.Runtime().Set("sql", sqlModule.Exports().Default))

	jsext, found := extensionapi.Registered()["k6/x/sql/driver/"+driver]

	require.True(t, found, "Driver extension found: "+driver)

	jsmod, ok := jsext.(extensionapi.Module)

	require.True(t, ok, "Driver extension module is JavaScript module")

	driverModule := jsmod.NewModuleInstance(vu)

	require.NoError(t, vu.Runtime().Set("driver", driverModule.Exports().Default))
	require.NoError(t, vu.Runtime().Set("connection", connection))

	value, err := vu.Runtime().RunString(script)

	require.NoError(t, err)

	return value
}
