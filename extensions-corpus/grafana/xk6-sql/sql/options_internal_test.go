package sql

import (
	"database/sql"
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/require"
)

func Test_options_apply(t *testing.T) {
	t.Parallel()

	db, err := sql.Open("ramsql", "foo")

	require.NoError(t, err)

	rt := sobek.New()

	require.NoError(t, (&options{}).apply(db))
	require.NoError(t, (&options{ConnMaxIdleTime: rt.ToValue("5s")}).apply(db))
	require.NoError(t, (&options{ConnMaxLifetime: rt.ToValue("10m")}).apply(db))

	require.NoError(t, (&options{MaxIdleConns: rt.ToValue(5)}).apply(db))
	require.NoError(t, (&options{MaxOpenConns: rt.ToValue(10)}).apply(db))

	require.Error(t, (&options{ConnMaxIdleTime: rt.ToValue("5g")}).apply(db))
	require.Error(t, (&options{ConnMaxLifetime: rt.ToValue("10e")}).apply(db))
}
