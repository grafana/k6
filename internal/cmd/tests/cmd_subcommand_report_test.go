package tests

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/cmd/state"
	"go.k6.io/k6/v2/errext/exitcodes"
	"go.k6.io/k6/v2/ext"
	"go.k6.io/k6/v2/internal/cmd"
	"go.k6.io/k6/v2/internal/lib/testutils"
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
		name               string
		args               []string
		catalog            string
		unreachableCatalog bool
		slowReport         bool
		wantNoReport       bool
		wantExitCode       int
		wantExtensions     []map[string]any
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
		{
			// An unregistered name cannot be provisioned from an unreachable
			// catalog, so the host builds no command for it and the wrapped hook
			// never runs. Neither the report endpoint nor the catalog is consulted.
			name:               "unknown subcommand is not reported",
			args:               []string{"x", "not-a-catalog-name"},
			unreachableCatalog: true,
			wantNoReport:       true,
			wantExitCode:       int(exitcodes.ScriptException),
		},
		{
			// A slow report endpoint must not delay or fail the command: the send
			// is abandoned within the bounded timeout, the command still exits 0,
			// and only a debug-level log records the failure.
			name:       "slow endpoint does not delay or fail the command",
			args:       []string{"x", "testsub", "-v"},
			catalog:    `{"testsub": {"subcommands":["testsub"],"module":"` + testSubModule + `"}}`,
			slowReport: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var reportCount atomic.Int32
			var gotBody atomic.Value
			released := make(chan struct{})
			reportServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.slowReport {
					// Hang past the bounded timeout so the client abandons the send;
					// release only at cleanup to avoid leaking the handler goroutine.
					select {
					case <-r.Context().Done():
					case <-released:
					}
					return
				}
				if r.Method == http.MethodPost {
					body, _ := io.ReadAll(r.Body)
					gotBody.Store(body)
					reportCount.Add(1)
				}
				w.WriteHeader(http.StatusOK)
			}))
			// Release the hung handler before Close waits on it (cleanups run LIFO).
			t.Cleanup(reportServer.Close)
			t.Cleanup(func() { close(released) })

			var catalogHit atomic.Bool

			ts := NewGlobalTestState(t)
			ts.Env["K6_NO_USAGE_REPORT"] = "false"
			ts.Env[state.UsageReportURL] = reportServer.URL
			if tc.catalog != "" {
				catalogServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					catalogHit.Store(true)
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(tc.catalog))
				}))
				t.Cleanup(catalogServer.Close)
				ts.Env[state.ProvisionCatalogURL] = catalogServer.URL
			}
			if tc.unreachableCatalog {
				// Close the server immediately so its URL refuses connections.
				catalogServer := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
					catalogHit.Store(true)
				}))
				catalogServer.Close()
				ts.Env[state.ProvisionCatalogURL] = catalogServer.URL
				// Keep the provisioning attempt off the real build service.
				ts.Env["K6_BUILD_SERVICE_URL"] = "http://127.0.0.1:1/unreachable"
			}
			ts.ExpectedExitCode = tc.wantExitCode
			ts.CmdArgs = append([]string{"k6"}, tc.args...)
			ts.ReparseFlags()

			start := time.Now()
			cmd.ExecuteWithGlobalState(ts.GlobalState)
			elapsed := time.Since(start)

			if tc.slowReport {
				// Exit 0 (default, asserted by the harness) with the send abandoned
				// within the bounded run-report timeout, surfacing the failure only
				// at debug level.
				require.Equal(t, int32(0), reportCount.Load(), "expected the slow send to be abandoned")
				require.Less(t, elapsed, 10*time.Second, "expected the slow send to be abandoned within the bounded timeout")

				entries := ts.LoggerHook.Drain()
				require.True(t,
					testutils.LogContains(entries, logrus.DebugLevel, "Error sending usage report"),
					"expected a debug-level log for the failed usage report",
				)
				for _, e := range entries {
					if e.Level < logrus.DebugLevel {
						require.NotContains(t, e.Message, "usage report",
							"expected the report failure to surface only at debug level")
					}
				}
				return
			}

			if tc.wantNoReport {
				require.Equal(t, int32(0), reportCount.Load(), "expected no usage report for an unknown subcommand")
				require.False(t, catalogHit.Load(), "expected no catalog request for an unknown subcommand")
				return
			}

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
