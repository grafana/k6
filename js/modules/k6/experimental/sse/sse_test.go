package sse

import (
	"crypto/tls"
	"net/http"
	"net/http/cookiejar"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	httpModule "go.k6.io/k6/js/modules/k6/http"
	"go.k6.io/k6/js/modulestest"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/testutils/httpmultibin"
	"go.k6.io/k6/metrics"
	"gopkg.in/guregu/null.v3"
)

func assertSSEMetricsEmitted(t *testing.T, sampleContainers []metrics.SampleContainer, subprotocol, url string, status int, group string) { //nolint:unparam
	seenEvents := false
	seenRequestDuration := false
	seenHTTPReq := false

	for _, sampleContainer := range sampleContainers {
		for _, sample := range sampleContainer.GetSamples() {
			tags := sample.Tags.Map()
			if tags["url"] == url {
				switch sample.Metric.Name {
				case metrics.HTTPReqsName:
					seenHTTPReq = true
				case metrics.HTTPReqDurationName:
					seenRequestDuration = true
				case metrics.SSEName:
					seenEvents = true
				}

				assert.Equal(t, strconv.Itoa(status), tags["status"])
				assert.Equal(t, subprotocol, tags["subproto"])
				assert.Equal(t, group, tags["group"])
			}
		}
	}
	assert.True(t, seenEvents, "url %s didn't emit SSE events", url)
	assert.True(t, seenRequestDuration, "url %s didn't emit seenRequestDuration", url)
	assert.True(t, seenHTTPReq, "url %s didn't emit seenHTTPReq", url)
}

func assertMetricEmittedCount(t *testing.T, metricName string, sampleContainers []metrics.SampleContainer, url string, count int) {
	t.Helper()
	actualCount := 0

	for _, sampleContainer := range sampleContainers {
		for _, sample := range sampleContainer.GetSamples() {
			surl, ok := sample.Tags.Get("url")
			assert.True(t, ok)
			if surl == url && sample.Metric.Name == metricName {
				actualCount++
			}
		}
	}
	assert.Equal(t, count, actualCount, "url %s emitted %s %d times, expected was %d times", url, metricName, actualCount, count)
}

type testState struct {
	*modulestest.Runtime
	tb      *httpmultibin.HTTPMultiBin
	samples chan metrics.SampleContainer
}

func newTestState(t testing.TB) testState {
	tb := httpmultibin.NewHTTPMultiBin(t)

	testRuntime := modulestest.NewRuntime(t)
	samples := make(chan metrics.SampleContainer, 1000)

	root, err := lib.NewGroup("", nil)
	require.NoError(t, err)
	registry := metrics.NewRegistry()
	state := &lib.State{
		Group:  root,
		Dialer: tb.Dialer,
		Options: lib.Options{
			SystemTags: metrics.NewSystemTagSet(
				metrics.TagURL,
				metrics.TagProto,
				metrics.TagStatus,
				metrics.TagSubproto,
			),
			UserAgent: null.StringFrom("TestUserAgent"),
			Throw:     null.BoolFrom(true),
		},
		Samples:        samples,
		TLSConfig:      tb.TLSClientConfig,
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(registry),
		Tags:           lib.NewVUStateTags(registry.RootTagSet()),
	}

	m := New().NewModuleInstance(testRuntime.VU)
	require.NoError(t, testRuntime.VU.RuntimeField.Set("sse", m.Exports().Default))
	testRuntime.MoveToVUContext(state)

	return testState{
		Runtime: testRuntime,
		tb:      tb,
		samples: samples,
	}
}

func TestOpen(t *testing.T) {
	t.Parallel()

	t.Run("nominal get", func(t *testing.T) {
		t.Parallel()
		tb := httpmultibin.NewHTTPMultiBin(t)
		sr := tb.Replacer.Replace

		test := newTestState(t)
		_, err := test.VU.Runtime().RunString(sr(`
		var open = false;
		var error = false;
		var events = [];
		var res = sse.open("HTTPBIN_IP_URL/sse", function(client){
			client.on("error", function(err) {
				error = true
			});
			client.on("open", function(err) {
				open = true
			});
			client.on("error", function(err) {
				error = true
			});
			client.on("event", function(event) {
				events.push(event);
			});
		});
		if (!open) {
			throw new Error("opened is not called");
		}
		if (error) {
			throw new Error("error raised");
		}
		for (let i = 0; i < events.length; i++) {
			let event = events[i];
			switch(i) {
				case 0:
					if (event.id !== "ABCD") {
						throw new Error("unexpected event id: " + event.id);
					}
					if (event.comment !== 'hello') {
						throw new Error("unexpected event comment: " + event.comment);
					}
					if (event.data !== '{"ping": "pong"}\n{"hello": "sse"}') {
						throw new Error("unexpected event data: " + event.data);
					}
					break;
				case 1:
					if (event.id !== "") {
						throw new Error("unexpected event id: " + event.id);
					}
					if (event.name !== "EFGH") {
						throw new Error("unexpected event name: " + event.name);
					}
					if (event.data !== '{"hello": "sse"}') {
						throw new Error("unexpected event data: " + event.data);
					}
					break;
				default:
					throw new Error("unexpected event");
			}
		}
		`))
		require.NoError(t, err)
		samplesBuf := metrics.GetBufferedSamples(test.samples)
		assertMetricEmittedCount(t, metrics.SSEName, samplesBuf, sr("HTTPBIN_IP_URL/sse"), 2)
	})

	t.Run("post method", func(t *testing.T) {
		t.Parallel()
		tb := httpmultibin.NewHTTPMultiBin(t)
		sr := tb.Replacer.Replace

		test := newTestState(t)
		_, err := test.VU.Runtime().RunString(sr(`
		var events = [];
		var res = sse.open("HTTPBIN_IP_URL/sse", {method: 'POST', body: '{"ping": true}'}, function(client){
			client.on("event", function(event) {
				events.push(event);
			});
		});
		for (let i = 0; i < events.length; i++) {
			let event = events[i];
			switch(i) {
				case 0:
					if (event.id !== "pong") {
						throw new Error("unexpected event id: " + event.id);
					}
					if (event.data !== '{"ping": "pong"}') {
						throw new Error("unexpected event data: " + event.data);
					}
					break;
				default:
					throw new Error("unexpected event");
			}
		}
		`))
		require.NoError(t, err)
		samplesBuf := metrics.GetBufferedSamples(test.samples)
		assertMetricEmittedCount(t, metrics.SSEName, samplesBuf, sr("HTTPBIN_IP_URL/sse"), 1)
	})
}

func TestErrors(t *testing.T) {
	t.Parallel()

	t.Run("invalid_url", func(t *testing.T) {
		t.Parallel()

		test := newTestState(t)
		_, err := test.VU.Runtime().RunString(`
		var res = sse.open("INVALID", function(client){
			client.on("open", function(client){});
		});
		`)
		assert.Error(t, err)
	})

	t.Run("error_in_setup", func(t *testing.T) {
		t.Parallel()
		tb := httpmultibin.NewHTTPMultiBin(t)
		sr := tb.Replacer.Replace

		test := newTestState(t)
		_, err := test.VU.Runtime().RunString(sr(`
		var res = sse.open("HTTPBIN_URL/sse-echo", function(client){
			throw new Error("error in setup");
		});
		`))
		assert.Error(t, err)
	})

	t.Run("error_in_response", func(t *testing.T) {
		t.Parallel()
		tb := httpmultibin.NewHTTPMultiBin(t)
		sr := tb.Replacer.Replace

		test := newTestState(t)
		_, err := test.VU.Runtime().RunString(sr(`
		var error = false;
		var res = sse.open("HTTPBIN_IP_URL/sse-invalid", function(client){
			client.on("error", function(err) {
				error = true
			});
		});
		if (!error) {
			throw new Error("no error raised");
		}
		`))
		require.NoError(t, err)
	})
}

func TestOpenWrongStatusCode(t *testing.T) {
	t.Parallel()
	test := newTestState(t)
	tb := httpmultibin.NewHTTPMultiBin(t)
	sr := tb.Replacer.Replace
	test.VU.StateField.Options.Throw = null.BoolFrom(false)
	_, err := test.VU.Runtime().RunString(sr(`
	var res = sse.open("HTTPBIN_IP_URL/status/404", function(client){});
	if (res.status != 404) {
		throw new Error ("no status code set for invalid response");
	}
	`))
	assert.NoError(t, err)
}

func TestUserAgent(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)
	sr := tb.Replacer.Replace

	tb.Mux.HandleFunc("/sse-echo-useragent", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// Echo back User-Agent header if it exists
		responseHeaders := w.Header()
		if ua := req.Header.Get("User-Agent"); ua != "" {
			responseHeaders.Add("X-Echo-User-Agent", req.Header.Get("User-Agent"))
		}
		_, err := w.Write([]byte(`data: {"ping": "pong"}` + "\n\n"))
		require.NoError(t, err)
	}))

	test := newTestState(t)

	// client handler should echo back User-Agent as Echo-User-Agent for this test to work
	_, err := test.VU.Runtime().RunString(sr(`
		var res = sse.open("HTTPBIN_IP_URL/sse-echo-useragent", function(client){})
		var userAgent = res.headers["X-Echo-User-Agent"];
		if (userAgent == undefined) {
			throw new Error("user agent is not echoed back by test server");
		}
		if (userAgent != "Go-http-client/1.1") {
			throw new Error("incorrect user agent: " + userAgent);
		}
		`))
	require.NoError(t, err)

	assertSSEMetricsEmitted(t, metrics.GetBufferedSamples(test.samples), "", sr("HTTPBIN_IP_URL/sse-echo-useragent"), http.StatusOK, "")
}

func TestCookieJar(t *testing.T) {
	t.Parallel()
	ts := newTestState(t)
	sr := ts.tb.Replacer.Replace

	ts.tb.Mux.HandleFunc("/sse-echo-someheader", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		responseHeaders := w.Header()
		if sh, err := req.Cookie("someheader"); err == nil {
			responseHeaders.Add("Echo-Someheader", sh.Value)
		}
		_, err := w.Write([]byte(`data: {"ping": "pong"}` + "\n\n"))
		require.NoError(t, err)
	}))

	err := ts.VU.Runtime().Set("http", httpModule.New().NewModuleInstance(ts.VU).Exports().Default)
	require.NoError(t, err)
	ts.VU.State().CookieJar, _ = cookiejar.New(nil)

	_, err = ts.VU.Runtime().RunString(sr(`
		var res = sse.open("HTTPBIN_IP_URL/sse-echo-someheader", function(client){})
		var someheader = res.headers["Echo-Someheader"];
		if (someheader !== undefined) {
			throw new Error("someheader is echoed back by test server even though it doesn't exist");
		}

		http.cookieJar().set("HTTPBIN_IP_URL/sse-echo-someheader", "someheader", "defaultjar")
		res = sse.open("HTTPBIN_IP_URL/sse-echo-someheader", function(client){})
		someheader = res.headers["Echo-Someheader"];
		if (someheader != "defaultjar") {
			throw new Error("someheader has wrong value "+ someheader + " instead of defaultjar");
		}

		var jar = new http.CookieJar();
		jar.set("HTTPBIN_IP_URL/sse-echo-someheader", "someheader", "customjar")
		res = sse.open("HTTPBIN_IP_URL/sse-echo-someheader", {jar: jar}, function(client){})
		someheader = res.headers["Echo-Someheader"];
		if (someheader != "customjar") {
			throw new Error("someheader has wrong value "+ someheader + " instead of customjar");
		}
		`))
	require.NoError(t, err)

	assertSSEMetricsEmitted(t, metrics.GetBufferedSamples(ts.samples), "", sr("HTTPBIN_IP_URL/sse-echo-someheader"), http.StatusOK, "")
}

func TestTLSConfig(t *testing.T) {
	t.Parallel()
	t.Run("insecure skip verify", func(t *testing.T) {
		t.Parallel()
		tb := httpmultibin.NewHTTPMultiBin(t)
		sr := tb.Replacer.Replace

		test := newTestState(t)
		test.VU.StateField.TLSConfig = &tls.Config{
			InsecureSkipVerify: true, //nolint:gosec
		}

		_, err := test.VU.Runtime().RunString(sr(`
		var res = sse.open("HTTPBIN_IP_URL/sse", function(client){});
		if (res.status != 200) { throw new Error("TLS connection failed with status: " + res.status); }
		`))
		require.NoError(t, err)
		assertSSEMetricsEmitted(t, metrics.GetBufferedSamples(test.samples), "", sr("HTTPBIN_IP_URL/sse"), http.StatusOK, "")
	})

	t.Run("custom certificates", func(t *testing.T) {
		t.Parallel()
		tb := httpmultibin.NewHTTPMultiBin(t)
		sr := tb.Replacer.Replace

		test := newTestState(t)
		test.VU.StateField.TLSConfig = tb.TLSClientConfig

		_, err := test.VU.Runtime().RunString(sr(`
			var res = sse.open("HTTPBIN_IP_URL/sse", function(client){});
			if (res.status != 200) {
				throw new Error("TLS connection failed with status: " + res.status);
			}
		`))
		require.NoError(t, err)
		assertSSEMetricsEmitted(t, metrics.GetBufferedSamples(test.samples), "", sr("HTTPBIN_IP_URL/sse"), http.StatusOK, "")
	})
}
