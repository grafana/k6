package sql

import (
	"context"
	"runtime"
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/js/modulestest"
)

func TestOpen(t *testing.T) { //nolint: paralleltest
	rt := modulestest.NewRuntime(t)

	mod, ok := New().NewModuleInstance(rt.VU).(*module)

	require.True(t, ok)

	driver := RegisterDriver("ramsql")
	require.NotNil(t, driver)

	db, err := mod.Open(driver, "", nil)

	require.NoError(t, err)
	require.NotNil(t, db)

	const expr = `CREATE TABLE address (id BIGSERIAL PRIMARY KEY, street TEXT, street_number INT);`

	if runtime.GOOS != "windows" {
		_, err = db.ExecWithTimeout("1ns", expr)
		require.ErrorIs(t, err, context.DeadlineExceeded)

		_, err = db.QueryWithTimeout("1ns", expr)
		require.ErrorIs(t, err, context.DeadlineExceeded)
	}

	_, err = mod.Open(sobek.New().ToValue("foo"), "testdb", nil) // not a Symbol

	require.Error(t, err)

	_, err = mod.Open(sobek.NewSymbol("ramsql"), "testdb", nil) // not a registered Symbol

	require.Error(t, err)
}
