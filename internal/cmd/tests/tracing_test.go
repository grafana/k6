package tests

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"

	"go.k6.io/k6/v2/errext/exitcodes"
	"go.k6.io/k6/v2/internal/cmd"
)

// otlpCollector is a minimal OTLP/HTTP traces receiver that keeps the received spans.
type otlpCollector struct {
	mu    sync.Mutex
	spans []*tracepb.Span
}

func newOTLPCollector(t *testing.T) (*otlpCollector, string) {
	t.Helper()

	c := &otlpCollector{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if !assert.NoError(t, err) {
			return
		}
		var req coltracepb.ExportTraceServiceRequest
		if !assert.NoError(t, proto.Unmarshal(body, &req)) {
			return
		}

		c.mu.Lock()
		defer c.mu.Unlock()
		for _, rs := range req.GetResourceSpans() {
			for _, ss := range rs.GetScopeSpans() {
				c.spans = append(c.spans, ss.GetSpans()...)
			}
		}
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	return c, srv.URL + "/v1/traces"
}

func (c *otlpCollector) all() []*tracepb.Span {
	c.mu.Lock()
	defer c.mu.Unlock()

	return slices.Clone(c.spans)
}

func (c *otlpCollector) byName(name string) []*tracepb.Span {
	c.mu.Lock()
	defer c.mu.Unlock()

	var out []*tracepb.Span
	for _, s := range c.spans {
		if s.GetName() == name {
			out = append(out, s)
		}
	}
	return out
}

// nativeTracingFlag enables the feature flag gating the k6 execution hierarchy spans.
const nativeTracingFlag = "--features=native-tracing"

const tracingScript = `
	export const options = {
		scenarios: {
			first: { executor: 'per-vu-iterations', vus: 2, iterations: 1 },
			second: { executor: 'shared-iterations', vus: 1, iterations: 1 },
		},
	};
	export default function () {}
`

func TestTracesOutputDoesNotExportRunHierarchyWithoutFeatureFlag(t *testing.T) {
	t.Parallel()

	collector, endpoint := newOTLPCollector(t)
	ts := getSingleFileTestState(t, tracingScript, []string{"--traces-output=otel=" + endpoint}, 0)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	for _, name := range []string{"k6.run", "scenario", "vu", "iteration"} {
		assert.Empty(t, collector.byName(name), "no %q span without the feature flag", name)
	}
}

func TestTracesOutputExportsRunHierarchy(t *testing.T) {
	t.Parallel()

	collector, endpoint := newOTLPCollector(t)
	ts := getSingleFileTestState(t, tracingScript, []string{nativeTracingFlag, "--traces-output=otel=" + endpoint}, 0)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	// Every span, including the run span that ends last, was flushed before k6 exited.
	runs := collector.byName("k6.run")
	require.Len(t, runs, 1)
	run := runs[0]
	assert.Empty(t, run.GetParentSpanId(), "the run span starts a new trace")

	scenarios := collector.byName("scenario")
	vus := collector.byName("vu")
	iterations := collector.byName("iteration")
	require.Len(t, scenarios, 2)
	require.Len(t, vus, 3)
	require.Len(t, iterations, 3)

	ids := map[string]string{} // span id -> name
	ids[string(run.GetSpanId())] = "run"
	for _, group := range [][]*tracepb.Span{scenarios, vus, iterations} {
		for _, s := range group {
			assert.Equal(t, run.GetTraceId(), s.GetTraceId(), "%s must be part of the run trace", s.GetName())
			ids[string(s.GetSpanId())] = s.GetName()
		}
	}
	for _, s := range scenarios {
		assert.Equal(t, "run", ids[string(s.GetParentSpanId())])
	}
	for _, s := range vus {
		assert.Equal(t, "scenario", ids[string(s.GetParentSpanId())])
	}
	for _, s := range iterations {
		assert.Equal(t, "vu", ids[string(s.GetParentSpanId())])
		assert.NotNil(t, attr(s, "k6.scenario.iteration_in_test"), "executors provide the scenario iteration counters")
	}
}

func TestTracesOutputRecordsFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		script   string
		exitCode exitcodes.ExitCode
		// wantRunErr is the error message of the run span, empty when the run must not fail.
		wantRunErr string
		// wantIterationErrs are the error messages of the failed iterations.
		wantIterationErrs []string
		wantIterations    int
	}{
		{
			// A failing iteration doesn't fail the test run.
			name:              "iteration error",
			script:            `export default function () { throw new Error("boom"); }`,
			wantIterationErrs: []string{"boom"},
			wantIterations:    1,
		},
		{
			// Only the iteration calling test.fail() failed, the test goes on and fails at the end.
			name: "test marked as failed",
			script: `
				import exec from 'k6/execution';
				export const options = { iterations: 2 };
				export default function () {
					if (exec.scenario.iterationInTest === 0) {
						exec.test.fail('oops');
					}
				}
			`,
			exitCode:          exitcodes.MarkedAsFailed,
			wantRunErr:        "test run was marked as failed",
			wantIterationErrs: []string{"oops"},
			wantIterations:    2,
		},
		{
			// The iteration aborting the test failed, as did the run.
			name: "test aborted",
			script: `
				import exec from 'k6/execution';
				export default function () { exec.test.abort('stop'); }
			`,
			exitCode:          exitcodes.ScriptAborted,
			wantRunErr:        "stop",
			wantIterationErrs: []string{"stop"},
			wantIterations:    1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			collector, endpoint := newOTLPCollector(t)
			ts := getSingleFileTestState(t, tc.script, []string{nativeTracingFlag, "--traces-output=otel=" + endpoint}, tc.exitCode)
			cmd.ExecuteWithGlobalState(ts.GlobalState)

			runs := collector.byName("k6.run")
			require.Len(t, runs, 1)

			span := runs[0]
			if tc.wantRunErr == "" {
				assert.Equal(t, tracepb.Status_STATUS_CODE_UNSET, span.GetStatus().GetCode())
				return
			}
			assert.Equal(t, tracepb.Status_STATUS_CODE_ERROR, span.GetStatus().GetCode())
			assert.Contains(t, span.GetStatus().GetMessage(), tc.wantRunErr)

			iterations := collector.byName("iteration")
			require.Len(t, iterations, tc.wantIterations)
			var errs []string
			for _, s := range iterations {
				if s.GetStatus().GetCode() == tracepb.Status_STATUS_CODE_ERROR {
					errs = append(errs, s.GetStatus().GetMessage())
				}
				assert.Nil(t, attr(s, "k6.iteration.interrupted"), "a failed iteration isn't interrupted")
			}
			require.Len(t, errs, len(tc.wantIterationErrs))
			for i, want := range tc.wantIterationErrs {
				assert.Contains(t, errs[i], want)
			}
		})
	}
}

// attr returns the value of the attribute of span with the given key, or nil when it has none.
func attr(span *tracepb.Span, key string) *commonpb.AnyValue {
	for _, kv := range span.GetAttributes() {
		if kv.GetKey() == key {
			return kv.GetValue()
		}
	}
	return nil
}

// TestTracesOutputBrowserIterationSpans runs a real browser, so it is skipped when Chromium
// isn't available. Without the native-tracing feature flag, the browser module traces each
// iteration with a root span of its own, as it did before the k6 execution hierarchy spans.
// With it, the browser spans are part of the k6 run trace.
func TestTracesOutputBrowserIterationSpans(t *testing.T) {
	t.Parallel()

	const script = `
		import { browser } from 'k6/browser';
		export const options = {
			scenarios: {
				ui: {
					executor: 'shared-iterations',
					iterations: 2,
					options: { browser: { type: 'chromium' } },
				},
			},
		};
		export default async function () {
			const page = await browser.newPage();
			await page.close();
		}
	`

	browserPath, ok := chromiumPath()
	if !ok {
		t.Skip("Chromium isn't installed")
	}

	for _, nativeTracing := range []bool{false, true} {
		t.Run(fmt.Sprintf("native_tracing=%t", nativeTracing), func(t *testing.T) {
			t.Parallel()

			collector, endpoint := newOTLPCollector(t)
			args := []string{"--traces-output=otel=" + endpoint}
			if nativeTracing {
				args = append(args, nativeTracingFlag)
			}
			ts := getSingleFileTestState(t, script, args, 0)
			ts.Env["K6_BROWSER_HEADLESS"] = "true"
			ts.Env["K6_BROWSER_EXECUTABLE_PATH"] = browserPath
			cmd.ExecuteWithGlobalState(ts.GlobalState)

			iterations := collector.byName("iteration")
			newPages := collector.byName("browser.newPage")
			require.Len(t, iterations, 2)
			require.Len(t, newPages, 2)

			byID := make(map[string]*tracepb.Span)
			for _, s := range collector.all() {
				byID[string(s.GetSpanId())] = s
			}
			for _, it := range iterations {
				parent := byID[string(it.GetParentSpanId())]
				if nativeTracing {
					require.NotNil(t, parent, "the iteration span must have a parent")
					assert.Equal(t, "vu", parent.GetName())
				} else {
					assert.Empty(t, it.GetParentSpanId(), "the iteration span must be a root span")
				}
			}
			traces := make(map[string]bool)
			for _, np := range newPages {
				parent := byID[string(np.GetParentSpanId())]
				require.NotNil(t, parent, "browser.newPage must have a parent")
				assert.Equal(t, "iteration", parent.GetName())
				assert.Equal(t, parent.GetTraceId(), np.GetTraceId())
				traces[string(np.GetTraceId())] = true
			}
			if nativeTracing {
				assert.Len(t, traces, 1, "the iterations must be part of the run trace")
				assert.Len(t, collector.byName("k6.run"), 1)
			} else {
				assert.Len(t, traces, 2, "each iteration must be its own trace")
				for _, name := range []string{"k6.run", "scenario", "vu"} {
					assert.Empty(t, collector.byName(name), "no %q span without the feature flag", name)
				}
			}
		})
	}
}

// chromiumPath returns the path of the first of the Chromium executables the browser module
// looks for by default that is in the PATH.
func chromiumPath() (string, bool) {
	for _, name := range []string{
		"headless_shell", "headless-shell", "chromium", "chromium-browser",
		"google-chrome", "google-chrome-stable", "google-chrome-beta", "google-chrome-unstable",
	} {
		if path, err := exec.LookPath(name); err == nil {
			return path, true
		}
	}
	return "", false
}
