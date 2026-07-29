package tests

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/cmd/state"
	"go.k6.io/k6/v2/ext"
	"go.k6.io/k6/v2/internal/cmd"
)

// TestHelpInvocationReportsUsage covers help invocations of a subcommand
// extension: both `k6 x <name> --help` and `k6 help x <name>` display the
// extension's help, so both count as usage. `k6 help x` describes the x
// namespace itself, and `k6 x help <name>` also falls back to the namespace
// help, so neither reports.
func TestHelpInvocationReportsUsage(t *testing.T) {
	t.Parallel()

	registerReportTestSubcommand(t)

	testSubModule := ext.Get(ext.SubcommandExtension)["testsub"].Path
	wantExtensions := []map[string]any{
		{
			"module":  testSubModule,
			"version": ext.Get(ext.SubcommandExtension)["testsub"].Version,
			"kind":    "subcommand",
		},
	}

	tt := []struct {
		name        string
		args        []string
		wantReports int32
	}{
		{name: "flag help on the extension is reported", args: []string{"x", "testsub", "--help"}, wantReports: 1},
		{name: "help command on the extension is reported", args: []string{"help", "x", "testsub"}, wantReports: 1},
		{name: "help of the x namespace is not reported", args: []string{"help", "x"}},
		{name: "namespace help under x is not reported", args: []string{"x", "help", "testsub"}},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var reportCount atomic.Int32
			var gotBody atomic.Value
			reportServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					body, _ := io.ReadAll(r.Body)
					gotBody.Store(body)
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
			ts.Env["K6_NO_USAGE_REPORT"] = "false"
			ts.Env[state.UsageReportURL] = reportServer.URL
			ts.Env[state.ProvisionCatalogURL] = catalogServer.URL
			ts.CmdArgs = append([]string{"k6"}, tc.args...)
			ts.ReparseFlags()

			cmd.ExecuteWithGlobalState(ts.GlobalState)

			require.Equal(t, tc.wantReports, reportCount.Load(), "unexpected usage report count for a help invocation")

			if tc.wantReports == 0 {
				return
			}
			raw, ok := gotBody.Load().([]byte)
			require.True(t, ok, "expected a report body")
			var report struct {
				Extensions []map[string]any `json:"extensions"`
			}
			require.NoError(t, json.Unmarshal(raw, &report))
			require.ElementsMatch(t, wantExtensions, report.Extensions)
		})
	}
}
