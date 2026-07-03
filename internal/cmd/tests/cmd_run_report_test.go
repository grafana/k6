package tests

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/cmd/state"
	"go.k6.io/k6/v2/internal/cmd"
)

func TestRunReportsExtensions(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name string
	}{
		{name: "sends the usage report to the configured endpoint"},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var reported atomic.Bool
			reportServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					reported.Store(true)
				}
				w.WriteHeader(http.StatusOK)
			}))
			t.Cleanup(reportServer.Close)

			ts := NewGlobalTestState(t)
			ts.Env["K6_NO_USAGE_REPORT"] = "false"
			ts.Env[state.UsageReportURL] = reportServer.URL
			ts.CmdArgs = []string{"k6", "run", "-"}
			ts.Stdin = bytes.NewBufferString(`export default function() {};`)

			cmd.ExecuteWithGlobalState(ts.GlobalState)

			require.True(t, reported.Load(), "expected the usage report to reach the configured endpoint")
		})
	}
}
