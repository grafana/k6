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

// registerNestedTestSubcommandOnce guards process-global ext.Register, which
// panics on a duplicate name/type.
var registerNestedTestSubcommandOnce sync.Once //nolint:gochecknoglobals

func registerNestedTestSubcommand(t *testing.T) {
	t.Helper()

	registerNestedTestSubcommandOnce.Do(func() {
		subcommand.RegisterExtension("testnest", func(*state.GlobalState) *cobra.Command {
			root := &cobra.Command{
				Use: "testnest",
				Run: func(*cobra.Command, []string) {},
			}
			child := &cobra.Command{
				Use: "child",
				Run: func(*cobra.Command, []string) {},
			}
			child.AddCommand(&cobra.Command{
				Use: "grandchild",
				Run: func(*cobra.Command, []string) {},
			})
			// An x-named nested subcommand exercises the walk's root-level
			// check: the walk must climb past it to the extension under the
			// real x.
			nestedX := &cobra.Command{
				Use: "x",
				Run: func(*cobra.Command, []string) {},
			}
			nestedX.AddCommand(&cobra.Command{
				Use: "leaf",
				Run: func(*cobra.Command, []string) {},
			})
			root.AddCommand(child, nestedX)
			return root
		})
	})
}

// registerBuiltinNamedTestSubcommandOnce guards process-global ext.Register,
// which panics on a duplicate name/type.
var registerBuiltinNamedTestSubcommandOnce sync.Once //nolint:gochecknoglobals

func registerBuiltinNamedTestSubcommand(t *testing.T) {
	t.Helper()

	registerBuiltinNamedTestSubcommandOnce.Do(func() {
		subcommand.RegisterExtension("version", func(*state.GlobalState) *cobra.Command {
			return &cobra.Command{
				Use: "version",
				Run: func(*cobra.Command, []string) {},
			}
		})
	})
}

// TestBuiltinNamedSubcommandExtensionIsNotReported covers a subcommand
// extension sharing a builtin command's name: running the builtin must not
// report the extension, even when the catalog lists it.
func TestBuiltinNamedSubcommandExtensionIsNotReported(t *testing.T) {
	t.Parallel()

	registerBuiltinNamedTestSubcommand(t)

	versionModule := ext.Get(ext.SubcommandExtension)["version"].Path

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
		_, _ = w.Write([]byte(`{"version": {"subcommands":["version"],"module":"` + versionModule + `"}}`))
	}))
	t.Cleanup(catalogServer.Close)

	ts := NewGlobalTestState(t)
	ts.Env["K6_NO_USAGE_REPORT"] = "false"
	ts.Env[state.UsageReportURL] = reportServer.URL
	ts.Env[state.ProvisionCatalogURL] = catalogServer.URL
	ts.CmdArgs = []string{"k6", "version"}
	ts.ReparseFlags()

	cmd.ExecuteWithGlobalState(ts.GlobalState)

	require.Equal(t, int32(0), reportCount.Load(),
		"expected the builtin run to send no report despite an extension sharing its name")
}

func TestNestedSubcommandReportsUsage(t *testing.T) {
	t.Parallel()

	registerNestedTestSubcommand(t)

	testNestModule := ext.Get(ext.SubcommandExtension)["testnest"].Path
	wantExtensions := []map[string]any{
		{
			"module":  testNestModule,
			"version": ext.Get(ext.SubcommandExtension)["testnest"].Version,
			"kind":    "subcommand",
		},
	}

	tt := []struct {
		name string
		args []string
	}{
		{name: "root invocation is reported", args: []string{"x", "testnest"}},
		{name: "child invocation is reported under the extension", args: []string{"x", "testnest", "child"}},
		{name: "grandchild invocation is reported under the extension", args: []string{"x", "testnest", "child", "grandchild"}},
		{name: "x-named nested invocation is reported under the extension", args: []string{"x", "testnest", "x"}},
		{name: "invocation under an x-named nested subcommand is reported", args: []string{"x", "testnest", "x", "leaf"}},
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
				_, _ = w.Write([]byte(`{"testnest": {"subcommands":["testnest"],"module":"` + testNestModule + `"}}`))
			}))
			t.Cleanup(catalogServer.Close)

			ts := NewGlobalTestState(t)
			ts.Env["K6_NO_USAGE_REPORT"] = "false"
			ts.Env[state.UsageReportURL] = reportServer.URL
			ts.Env[state.ProvisionCatalogURL] = catalogServer.URL
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
			require.ElementsMatch(t, wantExtensions, report.Extensions)
		})
	}
}
