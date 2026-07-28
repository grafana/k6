package tests

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/cmd/state"
	"go.k6.io/k6/v2/internal/cmd"
	"go.k6.io/k6/v2/lib/fsext"
)

// TestConfigFileOptOutSuppressesReports covers the config-file opt-out
// (noUsageReport: true) with K6_NO_USAGE_REPORT unset: it must suppress the
// usage report on every reporting path, matching the documented opt-out.
func TestConfigFileOptOutSuppressesReports(t *testing.T) {
	t.Parallel()

	registerReportTestSubcommand(t)

	testSubModule := "go.k6.io/k6/v2/internal/cmd/tests"

	tt := []struct {
		name  string
		args  []string
		stdin string
	}{
		{
			name:  "config file opt-out suppresses the run report",
			args:  []string{"run", "-"},
			stdin: `export default function() {};`,
		},
		{
			name: "config file opt-out suppresses the subcommand report",
			args: []string{"x", "testsub"},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var reportCount atomic.Int32
			reportServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					reportCount.Add(1)
				}
				w.WriteHeader(http.StatusOK)
			}))
			t.Cleanup(reportServer.Close)

			catalogServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"testsub": {"subcommands":["testsub"],"module":"` + testSubModule + `"}}`))
			}))
			t.Cleanup(catalogServer.Close)

			ts := NewGlobalTestState(t)
			// The env opt-out must stay unset so the config file is the only gate.
			delete(ts.Env, "K6_NO_USAGE_REPORT")
			ts.Env[state.UsageReportURL] = reportServer.URL
			ts.Env[state.ProvisionCatalogURL] = catalogServer.URL

			require.NoError(t, ts.FS.MkdirAll(filepath.Dir(ts.Flags.ConfigFilePath), 0o755))
			require.NoError(t, fsext.WriteFile(ts.FS, ts.Flags.ConfigFilePath,
				[]byte(`{"noUsageReport": true}`), 0o600))

			ts.CmdArgs = append([]string{"k6"}, tc.args...)
			if tc.stdin != "" {
				ts.Stdin = bytes.NewBufferString(tc.stdin)
			}
			ts.ReparseFlags()

			cmd.ExecuteWithGlobalState(ts.GlobalState)

			require.Equal(t, int32(0), reportCount.Load(),
				"expected the config-file opt-out to suppress the usage report")
		})
	}
}
