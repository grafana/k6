package tests

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/cmd/state"
	"go.k6.io/k6/v2/ext"
	"go.k6.io/k6/v2/internal/cmd"
	"go.k6.io/k6/v2/internal/cmd/tests/testimport2"
	"go.k6.io/k6/v2/js/modules"
)

// testImportModule is the resolved module path of the in-tree k6/x/testimport
// extension registered by registerRunReportTestExtensions. It is whatever
// ext.Get(ext.JSExtension)["k6/x/testimport"].Path returns for an in-tree
// registration, i.e. this test package's own Go package path.
const testImportModule = "go.k6.io/k6/v2/internal/cmd/tests"

// testImportModule2 is the resolved module path of the in-tree k6/x/testimport2
// extension. Its module type lives in its own Go package, so it resolves to a
// distinct path from testImportModule.
const testImportModule2 = "go.k6.io/k6/v2/internal/cmd/tests/testimport2"

type runReportTestModule struct{}

func (*runReportTestModule) NewModuleInstance(_ modules.VU) modules.Instance {
	return &runReportTestModuleInstance{}
}

type runReportTestModuleInstance struct{}

func (*runReportTestModuleInstance) Exports() modules.Exports {
	return modules.Exports{Named: map[string]any{}}
}

// registerRunReportTestExtensionsOnce guards process-global ext.Register, which
// panics on a duplicate name/type.
var registerRunReportTestExtensionsOnce sync.Once //nolint:gochecknoglobals

func registerRunReportTestExtensions(t *testing.T) {
	t.Helper()

	registerRunReportTestExtensionsOnce.Do(func() {
		modules.Register("k6/x/testimport", &runReportTestModule{})
		modules.Register("k6/x/testimport2", &testimport2.Module{})
	})
}

func TestRunReportsExtensions(t *testing.T) {
	t.Parallel()

	registerRunReportTestExtensions(t)

	tt := []struct {
		name           string
		args           []string
		script         string
		catalog        string
		wantExtensions []map[string]any
	}{
		{
			name:   "sends the usage report to the configured endpoint",
			script: `export default function() {};`,
		},
		{
			name:    "used public import is reported",
			script:  `import "k6/x/testimport"; export default function() {};`,
			catalog: `{"k6/x/testimport": {"module":"` + testImportModule + `"}}`,
			wantExtensions: []map[string]any{
				{
					"module":  testImportModule,
					"version": ext.Get(ext.JSExtension)["k6/x/testimport"].Version,
					"kind":    "js",
				},
			},
		},
		{
			name:    "module imported by many VUs is reported once",
			args:    []string{"--vus", "5", "--iterations", "5"},
			script:  `import "k6/x/testimport"; export default function() {};`,
			catalog: `{"k6/x/testimport": {"module":"` + testImportModule + `"}}`,
			wantExtensions: []map[string]any{
				{
					"module":  testImportModule,
					"version": ext.Get(ext.JSExtension)["k6/x/testimport"].Version,
					"kind":    "js",
				},
			},
		},
		{
			name:    "distinct imported extensions are each reported",
			script:  `import "k6/x/testimport"; import "k6/x/testimport2"; export default function() {};`,
			catalog: `{"k6/x/testimport": {"module":"` + testImportModule + `"},"k6/x/testimport2": {"module":"` + testImportModule2 + `"}}`,
			wantExtensions: []map[string]any{
				{
					"module":  testImportModule,
					"version": ext.Get(ext.JSExtension)["k6/x/testimport"].Version,
					"kind":    "js",
				},
				{
					"module":  testImportModule2,
					"version": ext.Get(ext.JSExtension)["k6/x/testimport2"].Version,
					"kind":    "js",
				},
			},
		},
		{
			name:    "extension not in the catalog is filtered out",
			script:  `import "k6/x/testimport"; import "k6/x/testimport2"; export default function() {};`,
			catalog: `{"k6/x/testimport2": {"module":"` + testImportModule2 + `"}}`,
			wantExtensions: []map[string]any{
				{
					"module":  testImportModule2,
					"version": ext.Get(ext.JSExtension)["k6/x/testimport2"].Version,
					"kind":    "js",
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var reported atomic.Bool
			var gotBody atomic.Value
			reportServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					body, _ := io.ReadAll(r.Body)
					gotBody.Store(body)
					reported.Store(true)
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
			ts.CmdArgs = append([]string{"k6", "run"}, tc.args...)
			ts.CmdArgs = append(ts.CmdArgs, "-")
			ts.Stdin = bytes.NewBufferString(tc.script)

			cmd.ExecuteWithGlobalState(ts.GlobalState)

			require.True(t, reported.Load(), "expected the usage report to reach the configured endpoint")

			if tc.wantExtensions == nil {
				return
			}

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
