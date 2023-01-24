package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/cmd"
	"go.k6.io/k6/lib/testutils/httpmultibin"
)

func TestTracingModuleClient(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)

	var gotRequests int64

	tb.Mux.HandleFunc("/tracing", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&gotRequests, 1)
		assert.NotEmpty(t, r.Header.Get("traceparent"))
		assert.Len(t, r.Header.Get("traceparent"), 55)
	})

	script := tb.Replacer.Replace(`
		import http from "k6/http";
		import { check } from "k6";
		import tracing from "k6/experimental/tracing";

		const instrumentedHTTP = new tracing.Client({
			propagator: "w3c",
		})

		export default function () {
			instrumentedHTTP.del("HTTPBIN_IP_URL/tracing");
			instrumentedHTTP.get("HTTPBIN_IP_URL/tracing");
			instrumentedHTTP.head("HTTPBIN_IP_URL/tracing");
			instrumentedHTTP.options("HTTPBIN_IP_URL/tracing");
			instrumentedHTTP.patch("HTTPBIN_IP_URL/tracing");
			instrumentedHTTP.post("HTTPBIN_IP_URL/tracing");
			instrumentedHTTP.put("HTTPBIN_IP_URL/tracing");
			instrumentedHTTP.request("GET", "HTTPBIN_IP_URL/tracing");
		};
	`)

	ts := getSingleFileTestState(t, script, []string{"--out", "json=results.json"}, 0)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	assert.Equal(t, int64(8), atomic.LoadInt64(&gotRequests))

	jsonResults, err := afero.ReadFile(ts.FS, "results.json")
	require.NoError(t, err)

	gotHTTPDataPoints := false

	for _, jsonLine := range bytes.Split(jsonResults, []byte("\n")) {
		if len(jsonLine) == 0 {
			continue
		}

		var line sampleEnvelope
		require.NoError(t, json.Unmarshal(jsonLine, &line))

		if line.Type != "Point" {
			continue
		}

		// Filter metric samples which are not related to http
		if !strings.HasPrefix(line.Metric, "http_") {
			continue
		}

		gotHTTPDataPoints = true

		anyTraceID, hasTraceID := line.Data.Metadata["trace_id"]
		require.True(t, hasTraceID)

		traceID, gotTraceID := anyTraceID.(string)
		require.True(t, gotTraceID)

		assert.Len(t, traceID, 32)
	}

	assert.True(t, gotHTTPDataPoints)
}

func TestTracingClient_DoesNotInterfereWithHTTPModule(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)

	var gotRequests int64
	var gotInstrumentedRequests int64

	tb.Mux.HandleFunc("/tracing", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&gotRequests, 1)

		if r.Header.Get("traceparent") != "" {
			atomic.AddInt64(&gotInstrumentedRequests, 1)
			assert.Len(t, r.Header.Get("traceparent"), 55)
		}
	})

	script := tb.Replacer.Replace(`
		import http from "k6/http";
		import { check } from "k6";
		import tracing from "k6/experimental/tracing";

		const instrumentedHTTP = new tracing.Client({
			propagator: "w3c",
		})

		export default function () {
			instrumentedHTTP.get("HTTPBIN_IP_URL/tracing");
			http.get("HTTPBIN_IP_URL/tracing");
			instrumentedHTTP.head("HTTPBIN_IP_URL/tracing");
		};
	`)

	ts := getSingleFileTestState(t, script, []string{"--out", "json=results.json"}, 0)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	assert.Equal(t, int64(3), atomic.LoadInt64(&gotRequests))
	assert.Equal(t, int64(2), atomic.LoadInt64(&gotInstrumentedRequests))
}

// sampleEnvelope is a trimmed version of the struct found
// in output/json/wrapper.go
// TODO: use the json output's wrapper struct instead if it's ever exported
type sampleEnvelope struct {
	Metric string `json:"metric"`
	Type   string `json:"type"`
	Data   struct {
		Value    float64                `json:"value"`
		Metadata map[string]interface{} `json:"metadata"`
	} `json:"data"`
}
