package tests

import (
	"net/http"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/cmd"
	"go.k6.io/k6/v2/internal/lib/testutils"
)

func TestStatsCommandWithoutAddressFails(t *testing.T) {
	t.Parallel()

	ts := NewGlobalTestState(t)
	ts.CmdArgs = []string{"k6", "stats"}
	ts.ExpectedExitCode = -1

	cmd.ExecuteWithGlobalState(ts.GlobalState)

	logEntries := ts.LoggerHook.Drain()
	require.True(t, testutils.LogContains(logEntries, logrus.ErrorLevel, "REST API server is disabled"))
	assert.Empty(t, ts.Stdout.String())
}

func TestStatsCommand(t *testing.T) {
	t.Parallel()

	srv := getTestServer(t, map[string]http.Handler{
		`GET ^/v1/metrics$`: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"type":"metrics","id":"http_reqs","attributes":{"type":"counter","contains":"default","tainted":null,"sample":{"count":1,"rate":0.5}}}]}`))
		}),
	})
	defer srv.Close()

	ts := NewGlobalTestState(t)
	ts.CmdArgs = []string{"k6", "--address", srv.Listener.Addr().String(), "stats"}

	cmd.ExecuteWithGlobalState(ts.GlobalState)

	assert.Contains(t, ts.Stdout.String(), "http_reqs")
	assert.Contains(t, ts.Stdout.String(), "counter")
}
