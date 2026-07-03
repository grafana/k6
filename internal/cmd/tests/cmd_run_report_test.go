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
	"go.k6.io/k6/v2/internal/cmd/tests/testfork"
	"go.k6.io/k6/v2/internal/cmd/tests/testimport2"
	"go.k6.io/k6/v2/js/modules"
	"go.k6.io/k6/v2/metrics"
	"go.k6.io/k6/v2/output"
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

// newReportTestOutput is the output extension constructor registered under the
// "testoutput" name. Its resolved module path is the runtime name of this
// function, read back via ext.Get(ext.OutputExtension)["testoutput"].Path.
func newReportTestOutput(output.Params) (output.Output, error) {
	return reportTestOutput{}, nil
}

type reportTestOutput struct{}

func (reportTestOutput) Description() string                        { return "testoutput" }
func (reportTestOutput) Start() error                               { return nil }
func (reportTestOutput) AddMetricSamples([]metrics.SampleContainer) {}
func (reportTestOutput) Stop() error                                { return nil }

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
		modules.Register("k6/x/testfork", &testfork.Module{})
		output.RegisterExtension("testoutput", newReportTestOutput)
	})
}

func TestRunReportsExtensions(t *testing.T) {
	t.Parallel()

	registerRunReportTestExtensions(t)

	tt := []struct {
		name                string
		args                []string
		script              string
		catalog             string
		unreachableCatalog  bool
		optOut              bool
		wantExtensions      []map[string]any
		wantNoExtensionsKey bool
	}{
		{
			name:   "sends the usage report to the configured endpoint",
			script: `export default function() {};`,
		},
		{
			name:                "compiled but unused extension omits the extensions key",
			script:              `export default function() {};`,
			catalog:             `{"k6/x/testimport": {"module":"` + testImportModule + `"}}`,
			wantNoExtensionsKey: true,
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
			name:    "selected output extension is reported",
			args:    []string{"--out", "testoutput"},
			script:  `export default function() {};`,
			catalog: `{"testoutput": {"module":"` + ext.Get(ext.OutputExtension)["testoutput"].Path + `"}}`,
			wantExtensions: []map[string]any{
				{
					"module":  ext.Get(ext.OutputExtension)["testoutput"].Path,
					"version": ext.Get(ext.OutputExtension)["testoutput"].Version,
					"kind":    "output",
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
		{
			// The catalog lists the real extension's module path for this import
			// name, but the imported fork resolves to its own path, so matching
			// by module path drops it.
			name:                "private fork reusing a public import name is dropped",
			script:              `import "k6/x/testfork"; export default function() {};`,
			catalog:             `{"k6/x/testfork": {"module":"` + testImportModule + `"}}`,
			wantNoExtensionsKey: true,
		},
		{
			// An unreachable catalog with no cache must fail closed: report no
			// extensions rather than leak the unfiltered import, and never fail
			// the run.
			name:                "unreachable catalog reports nothing",
			script:              `import "k6/x/testimport"; export default function() {};`,
			unreachableCatalog:  true,
			wantNoExtensionsKey: true,
		},
		{
			// The existing opt-out gates the whole report, so neither the report
			// endpoint nor the catalog is consulted.
			name:    "opt-out suppresses the report and catalog fetch",
			script:  `import "k6/x/testimport"; export default function() {};`,
			catalog: `{"k6/x/testimport": {"module":"` + testImportModule + `"}}`,
			optOut:  true,
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

			var catalogHit atomic.Bool

			ts := NewGlobalTestState(t)
			if tc.optOut {
				ts.Env["K6_NO_USAGE_REPORT"] = "true"
			} else {
				ts.Env["K6_NO_USAGE_REPORT"] = "false"
			}
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
				catalogServer := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
				catalogServer.Close()
				ts.Env[state.ProvisionCatalogURL] = catalogServer.URL
			}
			ts.CmdArgs = append([]string{"k6", "run"}, tc.args...)
			ts.CmdArgs = append(ts.CmdArgs, "-")
			ts.Stdin = bytes.NewBufferString(tc.script)

			cmd.ExecuteWithGlobalState(ts.GlobalState)

			if tc.optOut {
				require.False(t, reported.Load(), "expected the opt-out to suppress the usage report")
				require.False(t, catalogHit.Load(), "expected the opt-out to suppress the catalog fetch")
				return
			}

			require.True(t, reported.Load(), "expected the usage report to reach the configured endpoint")

			if tc.wantNoExtensionsKey {
				raw, ok := gotBody.Load().([]byte)
				require.True(t, ok, "expected a report body")
				var report map[string]json.RawMessage
				require.NoError(t, json.Unmarshal(raw, &report))
				_, present := report["extensions"]
				require.False(t, present, "expected no extensions key when nothing catalogued was used")
				return
			}

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
