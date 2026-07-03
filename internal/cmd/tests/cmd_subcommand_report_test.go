package tests

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/cmd/state"
	"go.k6.io/k6/v2/ext"
	"go.k6.io/k6/v2/internal/cmd"
	"go.k6.io/k6/v2/subcommand"
)

// registerReportTestSubcommandOnce guards process-global ext.Register, which
// panics on a duplicate name/type.
var registerReportTestSubcommandOnce sync.Once //nolint:gochecknoglobals

func registerReportTestSubcommand(t *testing.T) {
	t.Helper()

	registerReportTestSubcommandOnce.Do(func() {
		subcommand.RegisterExtension("testsub", func(*state.GlobalState) *cobra.Command {
			return &cobra.Command{
				Use: "testsub",
				Run: func(*cobra.Command, []string) {},
			}
		})
	})
}

func TestSubcommandReportsUsage(t *testing.T) {
	t.Parallel()

	registerReportTestSubcommand(t)

	// The subcommand constructor is an in-tree func, so its resolved module path
	// is the constructor's runtime name in this test package.
	testSubModule := ext.Get(ext.SubcommandExtension)["testsub"].Path

	tt := []struct {
		name           string
		args           []string
		catalog        string
		wantExtensions []map[string]any
	}{
		{
			name:    "catalogued subcommand run is reported",
			args:    []string{"x", "testsub"},
			catalog: `{"testsub": {"subcommands":["testsub"],"module":"` + testSubModule + `"}}`,
			wantExtensions: []map[string]any{
				{
					"module":  testSubModule,
					"version": ext.Get(ext.SubcommandExtension)["testsub"].Version,
					"kind":    "subcommand",
				},
			},
		},
		{
			name:           "subcommand absent from catalog is dropped",
			args:           []string{"x", "testsub"},
			catalog:        `{"other": {"subcommands":["other"],"module":"example.com/x-other"}}`,
			wantExtensions: nil,
		},
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

			ts := NewGlobalTestState(t)
			ts.Env["K6_NO_USAGE_REPORT"] = "false"
			ts.Env[state.UsageReportURL] = reportServer.URL
			if tc.catalog != "" {
				catalogServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(tc.catalog))
				}))
				t.Cleanup(catalogServer.Close)
				ts.Env[state.ProvisionCatalogURL] = catalogServer.URL
			}
			ts.CmdArgs = append([]string{"k6"}, tc.args...)
			ts.ReparseFlags()

			cmd.ExecuteWithGlobalState(ts.GlobalState)

			require.Equal(t, int32(1), reportCount.Load(), "expected exactly one usage report")

			raw, ok := gotBody.Load().([]byte)
			require.True(t, ok, "expected a report body")
			var report struct {
				Extensions []map[string]any `json:"extensions"`
			}
			require.NoError(t, json.Unmarshal(raw, &report))
			require.ElementsMatch(t, tc.wantExtensions, report.Extensions)
		})
	}
}
