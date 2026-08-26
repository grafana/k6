package postgres

import (
	"context"
	_ "embed"
	"runtime"
	"testing"

	"github.com/grafana/xk6-sql/sqltest"
	"github.com/stretchr/testify/require"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

//go:embed testdata/script.js
var script string

func TestIntegration(t *testing.T) { //nolint:paralleltest
	if testing.Short() {
		t.Skip()
	}

	if runtime.GOOS != "linux" {
		t.Skip("Works only on Linux (Testcontainers)")
	}

	ctx := context.Background()

	ctr, err := postgres.Run(ctx, "docker.io/postgres:16-alpine",
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2)),
	)

	require.NoError(t, err)
	defer func() { require.NoError(t, ctr.Terminate(ctx)) }()

	conn, err := ctr.ConnectionString(ctx, "sslmode=disable")

	require.NoError(t, err)

	sqltest.RunScript(t, "postgres", conn, script)
}
