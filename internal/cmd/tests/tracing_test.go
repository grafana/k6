package tests

import (
	"encoding/hex"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
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

func (c *otlpCollector) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.spans)
}

const tracingScript = `
	export const options = {
		scenarios: {
			first: { executor: 'per-vu-iterations', vus: 2, iterations: 1 },
			second: { executor: 'shared-iterations', vus: 1, iterations: 1 },
		},
	};
	export default function () {}
`

func TestTracesOutputExportsRunHierarchy(t *testing.T) {
	t.Parallel()

	collector, endpoint := newOTLPCollector(t)
	ts := getSingleFileTestState(t, tracingScript, []string{"--traces-output=otel=" + endpoint}, 0)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	// Every span, including the run span that ends last, was flushed before k6 exited.
	runs := collector.byName("run")
	require.Len(t, runs, 1)
	run := runs[0]
	assert.Empty(t, run.GetParentSpanId(), "a run without incoming context starts a new trace")

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
	}
}

func TestTracesOutputRecordsIterationErrors(t *testing.T) {
	t.Parallel()

	collector, endpoint := newOTLPCollector(t)
	script := `export default function () { throw new Error("boom"); }`
	ts := getSingleFileTestState(t, script, []string{"--traces-output=otel=" + endpoint}, 0)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	// A failing iteration doesn't fail the test run.
	runs := collector.byName("run")
	require.Len(t, runs, 1)
	assert.Equal(t, tracepb.Status_STATUS_CODE_UNSET, runs[0].GetStatus().GetCode())

	iterations := collector.byName("iteration")
	require.Len(t, iterations, 1)
	assert.Equal(t, tracepb.Status_STATUS_CODE_ERROR, iterations[0].GetStatus().GetCode())
	assert.Contains(t, iterations[0].GetStatus().GetMessage(), "boom")
}

func TestTracesOutputRecordsRunFailure(t *testing.T) {
	t.Parallel()

	collector, endpoint := newOTLPCollector(t)
	script := `
		import exec from 'k6/execution';
		export default function () { exec.test.abort('stop'); }
	`
	ts := getSingleFileTestState(t, script, []string{"--traces-output=otel=" + endpoint}, exitcodes.ScriptAborted)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	runs := collector.byName("run")
	require.Len(t, runs, 1)
	assert.Equal(t, tracepb.Status_STATUS_CODE_ERROR, runs[0].GetStatus().GetCode())
	assert.Contains(t, runs[0].GetStatus().GetMessage(), "stop")

	// The aborted iteration is interrupted, the error belongs to the run.
	iterations := collector.byName("iteration")
	require.Len(t, iterations, 1)
	assert.Equal(t, tracepb.Status_STATUS_CODE_UNSET, iterations[0].GetStatus().GetCode())
	assert.True(t, hasBoolAttribute(iterations[0], "k6.iteration.interrupted"))
}

func TestNoTraceLogWithoutTracesOutput(t *testing.T) {
	t.Parallel()

	ts := getSingleFileTestState(t, tracingScript, nil, 0)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	for _, entry := range ts.LoggerHook.Drain() {
		assert.NotContains(t, entry.Message, "Tracing the test run")
	}
}

func TestTraceparentEnvIgnoredWithoutTracesOutput(t *testing.T) {
	t.Parallel()

	// TRACEPARENT may be set by a parent process for another purpose. Without a traces output,
	// it must neither fail the run when invalid, nor make k6 claim it is tracing when valid.
	for name, parent := range map[string]string{
		"valid":   "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
		"invalid": "garbage",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ts := getSingleFileTestState(t, tracingScript, nil, 0)
			ts.Env["TRACEPARENT"] = parent
			cmd.ExecuteWithGlobalState(ts.GlobalState)

			for _, entry := range ts.LoggerHook.Drain() {
				assert.NotContains(t, entry.Message, "Tracing the test run")
				assert.NotContains(t, entry.Message, "invalid trace parent")
			}
		})
	}
}

func TestTracesOutputLogsTraceID(t *testing.T) {
	t.Parallel()

	_, endpoint := newOTLPCollector(t)
	ts := getSingleFileTestState(t, tracingScript, []string{"--traces-output=otel=" + endpoint}, 0)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	var found bool
	for _, entry := range ts.LoggerHook.Drain() {
		if entry.Message == "Tracing the test run" {
			found = true
			assert.NotEmpty(t, entry.Data["trace_id"])
		}
	}
	assert.True(t, found, "the trace ID of the run must be logged")
}

func TestTraceParentContinuesTheTrace(t *testing.T) {
	t.Parallel()

	const (
		traceID  = "4bf92f3577b34da6a3ce929d0e0e4736"
		parentID = "00f067aa0ba902b7"
		parent   = "00-" + traceID + "-" + parentID + "-01"
	)

	for name, flags := range map[string]func(endpoint string) ([]string, map[string]string){
		"flag": func(endpoint string) ([]string, map[string]string) {
			return []string{"--traces-output=otel=" + endpoint, "--traceparent=" + parent}, nil
		},
		"env": func(endpoint string) ([]string, map[string]string) {
			return []string{"--traces-output=otel=" + endpoint}, map[string]string{"TRACEPARENT": parent}
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			collector, endpoint := newOTLPCollector(t)
			cliFlags, env := flags(endpoint)
			ts := getSingleFileTestState(t, tracingScript, cliFlags, 0)
			maps.Copy(ts.Env, env)
			cmd.ExecuteWithGlobalState(ts.GlobalState)

			runs := collector.byName("run")
			require.Len(t, runs, 1)
			assert.Equal(t, traceID, hex.EncodeToString(runs[0].GetTraceId()))
			assert.Equal(t, parentID, hex.EncodeToString(runs[0].GetParentSpanId()))
			for _, name := range []string{"scenario", "vu", "iteration"} {
				spans := collector.byName(name)
				require.NotEmpty(t, spans, name)
				for _, s := range spans {
					assert.Equal(t, traceID, hex.EncodeToString(s.GetTraceId()), name)
				}
			}
		})
	}
}

func TestTraceParentFlagWinsOverEnv(t *testing.T) {
	t.Parallel()

	const (
		flagTraceID = "4bf92f3577b34da6a3ce929d0e0e4736"
		envTraceID  = "0af7651916cd43dd8448eb211c80319c"
	)

	collector, endpoint := newOTLPCollector(t)
	ts := getSingleFileTestState(t, tracingScript, []string{
		"--traces-output=otel=" + endpoint, "--traceparent=00-" + flagTraceID + "-00f067aa0ba902b7-01",
	}, 0)
	ts.Env["TRACEPARENT"] = "00-" + envTraceID + "-b7ad6b7169203331-01"
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	runs := collector.byName("run")
	require.Len(t, runs, 1)
	assert.Equal(t, flagTraceID, hex.EncodeToString(runs[0].GetTraceId()))
}

func TestInvalidTraceParentFailsTheRun(t *testing.T) {
	t.Parallel()

	const parent = "not-a-traceparent"
	for name, tc := range map[string]struct {
		cliFlags []string
		env      map[string]string
	}{
		"flag": {cliFlags: []string{"--traceparent=" + parent}},
		"env":  {env: map[string]string{"TRACEPARENT": parent}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			collector, endpoint := newOTLPCollector(t)
			cliFlags := append([]string{"--traces-output=otel=" + endpoint}, tc.cliFlags...)
			ts := getSingleFileTestState(t, tracingScript, cliFlags, exitcodes.InvalidConfig)
			maps.Copy(ts.Env, tc.env)
			cmd.ExecuteWithGlobalState(ts.GlobalState)

			assert.Zero(t, collector.count())
			assert.Contains(t, ts.Stderr.String(), "invalid trace parent")
		})
	}
}

func hasBoolAttribute(span *tracepb.Span, key string) bool {
	for _, kv := range span.GetAttributes() {
		if kv.GetKey() == key {
			return kv.GetValue().GetBoolValue()
		}
	}
	return false
}
