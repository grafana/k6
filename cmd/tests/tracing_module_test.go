package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/cmd"
	"go.k6.io/k6/js/modules/k6/experimental/tracing"
	"go.k6.io/k6/lib/testutils/httpmultibin"
)

func TestTracingModuleClient(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)

	gotRequests := 0

	tb.Mux.HandleFunc("/tracing", func(w http.ResponseWriter, r *http.Request) {
		gotRequests++
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

	assert.Equal(t, 8, gotRequests)

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

	gotRequests := 0
	gotInstrumentedRequests := 0

	tb.Mux.HandleFunc("/tracing", func(w http.ResponseWriter, r *http.Request) {
		gotRequests++

		if r.Header.Get("traceparent") != "" {
			gotInstrumentedRequests++
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

	assert.Equal(t, 3, gotRequests)
	assert.Equal(t, 2, gotInstrumentedRequests)
}

func TestTracingClient_Sampling(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)

	gotRequests := 0
	gotSampledRequests := 0
	gotNonsampledRequests := 0

	tb.Mux.HandleFunc("/tracing", func(w http.ResponseWriter, r *http.Request) {
		gotRequests++

		if r.Header.Get("traceparent") != "" {
			assert.Len(t, r.Header.Get("traceparent"), 55)

			// Check if the sampled flag is set
			if r.Header.Get("traceparent")[54] == '1' {
				gotSampledRequests++
			} else {
				gotNonsampledRequests++
			}
		}
	})

	// The tracing client supports probabilistic sampling.
	// Because of its very nature, it is hard (impossible) to test
	// this in a satisfyingly deterministic way.
	// Thus we will just test that the sampling flag is set according
	// to sampling rates set to 0%, or 100%, and that the traceparent
	// header is of the correct length.
	script := tb.Replacer.Replace(`
		import http from "k6/http";
		import { check } from "k6";
		import tracing from "k6/experimental/tracing";

		const sampledHTTP = new tracing.Client({
			propagator: "w3c",
			sampling: 100,
		})

		const nonSampledHTTP = new tracing.Client({
			propagator: "w3c",
			sampling: 0,
		})

		export default function () {
			sampledHTTP.get("HTTPBIN_IP_URL/tracing");
			nonSampledHTTP.head("HTTPBIN_IP_URL/tracing");
		};
	`)

	ts := getSingleFileTestState(t, script, []string{"--out", "json=results.json"}, 0)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	assert.Equal(t, 2, gotRequests)
	assert.Equal(t, 1, gotSampledRequests)
	assert.Equal(t, 1, gotNonsampledRequests)

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

func TestTracingInstrumentHTTP_W3C(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)

	gotRequests := 0

	tb.Mux.HandleFunc("/tracing", func(w http.ResponseWriter, r *http.Request) {
		gotRequests++
		assert.NotEmpty(t, r.Header.Get("traceparent"))
		assert.Len(t, r.Header.Get("traceparent"), 55)
	})

	script := tb.Replacer.Replace(`
		import http from "k6/http";
		import { check } from "k6";
		import tracing from "k6/experimental/tracing";

		tracing.instrumentHTTP({
			propagator: "w3c",
		})

		export default function () {
			http.del("HTTPBIN_IP_URL/tracing");
			http.get("HTTPBIN_IP_URL/tracing");
			http.head("HTTPBIN_IP_URL/tracing");
			http.options("HTTPBIN_IP_URL/tracing");
			http.patch("HTTPBIN_IP_URL/tracing");
			http.post("HTTPBIN_IP_URL/tracing");
			http.put("HTTPBIN_IP_URL/tracing");
			http.request("GET", "HTTPBIN_IP_URL/tracing");
		};
	`)

	ts := getSingleFileTestState(t, script, []string{"--out", "json=results.json"}, 0)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	assert.Equal(t, 8, gotRequests)

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

func TestTracingInstrumentHTTP_Jaeger(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)

	gotRequests := 0

	tb.Mux.HandleFunc("/tracing", func(w http.ResponseWriter, r *http.Request) {
		gotRequests++
		assert.NotEmpty(t, r.Header.Get(tracing.JaegerHeaderName))
		assert.Len(t, r.Header.Get(tracing.JaegerHeaderName), 45)
	})

	script := tb.Replacer.Replace(`
		import http from "k6/http";
		import { check } from "k6";
		import tracing from "k6/experimental/tracing";

		tracing.instrumentHTTP({
			propagator: "jaeger",
		})

		export default function () {
			http.del("HTTPBIN_IP_URL/tracing");
			http.get("HTTPBIN_IP_URL/tracing");
			http.head("HTTPBIN_IP_URL/tracing");
			http.options("HTTPBIN_IP_URL/tracing");
			http.patch("HTTPBIN_IP_URL/tracing");
			http.post("HTTPBIN_IP_URL/tracing");
			http.put("HTTPBIN_IP_URL/tracing");
			http.request("GET", "HTTPBIN_IP_URL/tracing");
		};
	`)

	ts := getSingleFileTestState(t, script, []string{"--out", "json=results.json"}, 0)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	assert.Equal(t, 8, gotRequests)

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

func TestTracingInstrumentHTTP_FillsParams(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)

	gotRequests := 0

	tb.Mux.HandleFunc("/tracing", func(w http.ResponseWriter, r *http.Request) {
		gotRequests++

		assert.NotEmpty(t, r.Header.Get("traceparent"))
		assert.Len(t, r.Header.Get("traceparent"), 55)

		assert.NotEmpty(t, r.Header.Get("X-Test-Header"))
		assert.Equal(t, "test", r.Header.Get("X-Test-Header"))
	})

	script := tb.Replacer.Replace(`
		import http from "k6/http";
		import { check } from "k6";
		import tracing from "k6/experimental/tracing";

		tracing.instrumentHTTP({
			propagator: "w3c",
		})

		const testHeaders = {
			"X-Test-Header": "test",
		}

		export default function () {
			http.del("HTTPBIN_IP_URL/tracing", null, { headers: testHeaders });
			http.get("HTTPBIN_IP_URL/tracing", { headers: testHeaders });
			http.head("HTTPBIN_IP_URL/tracing", { headers: testHeaders });
			http.options("HTTPBIN_IP_URL/tracing", null, { headers: testHeaders });
			http.patch("HTTPBIN_IP_URL/tracing", null, { headers: testHeaders });
			http.post("HTTPBIN_IP_URL/tracing", null, { headers: testHeaders });
			http.put("HTTPBIN_IP_URL/tracing", null, { headers: testHeaders });
			http.request("GET", "HTTPBIN_IP_URL/tracing", null, { headers: testHeaders });
		};
	`)

	ts := getSingleFileTestState(t, script, []string{"--out", "json=results.json"}, 0)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	assert.Equal(t, 8, gotRequests)

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
