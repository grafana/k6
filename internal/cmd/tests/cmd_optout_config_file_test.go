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

// TestConfigFileOptOut covers the config-file opt-out (noUsageReport: true)
// on every reporting path: alone it must suppress the usage report,
// K6_NO_USAGE_REPORT=false must win over it and re-enable reporting, and an
// unreadable config file must fail closed by skipping the report.
func TestConfigFileOptOut(t *testing.T) {
	t.Parallel()

	registerReportTestSubcommand(t)

	testSubModule := "go.k6.io/k6/v2/internal/cmd/tests"

	tt := []struct {
		name        string
		args        []string
		stdin       string
		config      string // config file content; empty means {"noUsageReport": true}
		envOptOut   string // K6_NO_USAGE_REPORT; empty leaves it unset
		wantReports int32
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
		{
			name:        "env re-enables the run report over the config file",
			args:        []string{"run", "-"},
			stdin:       `export default function() {};`,
			envOptOut:   "false",
			wantReports: 1,
		},
		{
			name:        "env re-enables the subcommand report over the config file",
			args:        []string{"x", "testsub"},
			envOptOut:   "false",
			wantReports: 1,
		},
		{
			// An unreadable config file must fail closed: no report, while
			// the subcommand still runs (the zero exit code is asserted by
			// the test state).
			name:      "unreadable config file skips the subcommand report",
			args:      []string{"x", "testsub"},
			config:    `{not json`,
			envOptOut: "false",
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
			// The default env opt-out would mask the config file gate.
			delete(ts.Env, "K6_NO_USAGE_REPORT")
			if tc.envOptOut != "" {
				ts.Env["K6_NO_USAGE_REPORT"] = tc.envOptOut
			}
			ts.Env[state.UsageReportURL] = reportServer.URL
			ts.Env[state.ProvisionCatalogURL] = catalogServer.URL

			config := tc.config
			if config == "" {
				config = `{"noUsageReport": true}`
			}
			require.NoError(t, ts.FS.MkdirAll(filepath.Dir(ts.Flags.ConfigFilePath), 0o755))
			require.NoError(t, fsext.WriteFile(ts.FS, ts.Flags.ConfigFilePath, []byte(config), 0o600))

			ts.CmdArgs = append([]string{"k6"}, tc.args...)
			if tc.stdin != "" {
				ts.Stdin = bytes.NewBufferString(tc.stdin)
			}
			ts.ReparseFlags()

			cmd.ExecuteWithGlobalState(ts.GlobalState)

			require.Equal(t, tc.wantReports, reportCount.Load(),
				"unexpected usage report count with noUsageReport set in the config file")
		})
	}
}
