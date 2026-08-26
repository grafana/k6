package sse

import (
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	httpModule "go.k6.io/k6/v2/js/modules/k6/http"
	"go.k6.io/k6/v2/js/modulestest"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/lib/netext"
	"go.k6.io/k6/v2/lib/types"
	"go.k6.io/k6/v2/metrics"
	"gopkg.in/guregu/null.v3"
)

type httpbin struct {
	Mux             *http.ServeMux
	Dialer          lib.DialContexter
	TLSClientConfig *tls.Config
	Replacer        *strings.Replacer
}

func newHTTPBin(tb testing.TB) *httpbin {
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	replacer := strings.NewReplacer("HTTPBIN_IP_URL", srv.URL)
	dialer := netext.NewDialer(net.Dialer{
		Timeout:   2 * time.Second,
		KeepAlive: 10 * time.Second,
	}, netext.NewResolver(net.LookupIP, 0, types.DNSfirst, types.DNSpreferIPv4))

	var err error
	httpURL, err := url.Parse(srv.URL)
	require.NoError(tb, err)
	httpIP := net.ParseIP(httpURL.Hostname())
	httpDomainValue, err := types.NewHost(httpIP, "")
	require.NoError(tb, err)
	dialer.Hosts, err = types.NewHosts(map[string]types.Host{
		"httpbin.local": *httpDomainValue,
	})
	require.NoError(tb, err)

	return &httpbin{
		Mux:             mux,
		Replacer:        replacer,
		TLSClientConfig: srv.TLS,
		Dialer:          dialer,
	}
}

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
				case MetricEventName:
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

func assertSseCount(t *testing.T, sampleContainers []metrics.SampleContainer, url string, count int) {
	assertMetricEmittedCount(t, MetricEventName, sampleContainers, url, count)
}

type testState struct {
	*modulestest.Runtime
	tb      *httpbin
	samples chan metrics.SampleContainer
}

func newTestState(tb testing.TB) testState {
	httpBin := newHTTPBin(tb)
	httpBin.Mux.Handle("/sse", sseHandler(tb, false))
	httpBin.Mux.Handle("/sse-invalid", sseHandler(tb, true))
	httpBin.Mux.Handle("/sse-slow", sseSlowHandler(tb))

	testRuntime := modulestest.NewRuntime(tb)
	registry := metrics.NewRegistry()
	samples := make(chan metrics.SampleContainer, 1000)

	state := &lib.State{
		Dialer: httpBin.Dialer,
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
		TLSConfig:      httpBin.TLSClientConfig,
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(registry),
		Tags:           lib.NewVUStateTags(registry.RootTagSet()),
	}

	m := New().NewModuleInstance(testRuntime.VU)
	require.NoError(tb, testRuntime.VU.RuntimeField.Set("sse", m.Exports().Default))
	testRuntime.MoveToVUContext(state)

	return testState{
		Runtime: testRuntime,
		tb:      httpBin,
		samples: samples,
	}
}

func TestOpen(t *testing.T) {
	t.Parallel()

	t.Run("nominal get", func(t *testing.T) {
		t.Parallel()
		test := newTestState(t)
		sr := test.tb.Replacer.Replace

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
		assertSseCount(t, samplesBuf, sr("HTTPBIN_IP_URL/sse"), 2)
	})

	t.Run("post method", func(t *testing.T) {
		t.Parallel()

		test := newTestState(t)
		sr := test.tb.Replacer.Replace
		_, err := test.VU.Runtime().RunString(sr(`
		var events = [];
		var res = sse.open("HTTPBIN_IP_URL/sse", {method: 'POST', body: '{"ping": true}', headers: {"content-type": "application/json", "Authorization": "Bearer XXXX"}}, function(client){
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
		assertSseCount(t, samplesBuf, sr("HTTPBIN_IP_URL/sse"), 1)
	})
}

func TestClose(t *testing.T) {
	t.Parallel()

	t.Run("nominal get close", func(t *testing.T) {
		t.Parallel()
		test := newTestState(t)
		sr := test.tb.Replacer.Replace

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
				client.close()
				events.push(event);
			});
		});
		if (!open) {
			throw new Error("opened is not called");
		}
		if (error) {
			throw new Error("error raised");
		}
		if (events.length != 1) {
			throw new Error("unexpected number of events");
		}
`))
		require.NoError(t, err)
		samplesBuf := metrics.GetBufferedSamples(test.samples)
		assertSseCount(t, samplesBuf, sr("HTTPBIN_IP_URL/sse"), 1)
	})

	t.Run("post method", func(t *testing.T) {
		t.Parallel()

		test := newTestState(t)
		sr := test.tb.Replacer.Replace
		_, err := test.VU.Runtime().RunString(sr(`
		var events = [];
		var res = sse.open("HTTPBIN_IP_URL/sse", {method: 'POST', body: '{"ping": true}', headers: {"content-type": "application/json", "Authorization": "Bearer XXXX"}}, function(client){
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
		assertSseCount(t, samplesBuf, sr("HTTPBIN_IP_URL/sse"), 1)
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
		test := newTestState(t)
		sr := test.tb.Replacer.Replace
		_, err := test.VU.Runtime().RunString(sr(`
		var res = sse.open("HTTPBIN_URL/sse-echo", function(client){
			throw new Error("error in setup");
		});
		`))
		assert.Error(t, err)
	})

	t.Run("error_in_response", func(t *testing.T) {
		t.Parallel()
		test := newTestState(t)
		sr := test.tb.Replacer.Replace
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
	sr := test.tb.Replacer.Replace
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
	test := newTestState(t)
	sr := test.tb.Replacer.Replace

	test.tb.Mux.HandleFunc("/sse-echo-useragent", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// Echo back User-Agent header if it exists
		responseHeaders := w.Header()
		if ua := req.Header.Get("User-Agent"); ua != "" {
			responseHeaders.Add("X-Echo-User-Agent", req.Header.Get("User-Agent"))
		}
		_, err := w.Write([]byte(`data: {"ping": "pong"}` + "\n\n"))
		require.NoError(t, err)
	}))

	// client handler should echo back User-Agent as Echo-User-Agent for this test to work
	_, err := test.VU.Runtime().RunString(sr(`
		var res = sse.open("HTTPBIN_IP_URL/sse-echo-useragent", function(client){})
		var userAgent = res.headers["X-Echo-User-Agent"];
		if (userAgent == undefined) {
			throw new Error("user agent is not echoed back by test server");
		}
		if (userAgent != "TestUserAgent") {
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

		test := newTestState(t)
		sr := test.tb.Replacer.Replace
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
		test := newTestState(t)
		sr := test.tb.Replacer.Replace
		test.VU.StateField.TLSConfig = test.tb.TLSClientConfig

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

func TestLineEnding(t *testing.T) {
	t.Parallel()
	test := newTestState(t)
	sr := test.tb.Replacer.Replace

	// Register the line endings handler
	test.tb.Mux.Handle("/sse-line-endings", sseLineEndingsHandler(t))

	_, err := test.VU.Runtime().RunString(sr(`
	var events = [];
	var res = sse.open("HTTPBIN_IP_URL/sse-line-endings", function(client){
		client.on("event", function(event) {
			events.push(event);
		});
	});
	
	// Wait a bit to ensure all events are received
	if (events.length !== 2) {
		throw new Error("Expected 2 events, got " + events.length);
	}
	
	// Check CRLF-terminated event data
	if (events[0].data !== "CRLF line ending") {
		throw new Error("CRLF event data incorrect: '" + events[0].data + "'");
	}
	
	// Check LF-terminated event data
	if (events[1].data !== "LF line ending") {
		throw new Error("LF event data incorrect: '" + events[1].data + "'");
	}
	`))

	require.NoError(t, err)
	samplesBuf := metrics.GetBufferedSamples(test.samples)
	assertSseCount(t, samplesBuf, sr("HTTPBIN_IP_URL/sse-line-endings"), 2)
}

func TestTimeout(t *testing.T) {
	t.Parallel()

	t.Run("request timeout", func(t *testing.T) {
		t.Parallel()
		test := newTestState(t)
		sr := test.tb.Replacer.Replace
		test.VU.StateField.Options.Throw = null.BoolFrom(true)

		_, err := test.VU.Runtime().RunString(sr(`
		var error = false;
		var errorMessage = "";
		var res = sse.open("HTTPBIN_IP_URL/sse-slow", {timeout: "50ms"}, function(client){
			client.on("error", function(err) {
				error = true;
				errorMessage = err.toString();
			});
		});
		if (error) {
			throw new Error("unexpected error event raised: " + errorMessage);
		}
		`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Client.Timeout exceeded")
	})

	t.Run("invalid timeout", func(t *testing.T) {
		t.Parallel()
		test := newTestState(t)
		sr := test.tb.Replacer.Replace

		_, err := test.VU.Runtime().RunString(sr(`
		var res = sse.open("HTTPBIN_IP_URL/sse", {timeout: "invalid"}, function(client){});
		`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid sse.open() timeout")
	})
}

// sseSlowHandler simulates a slow SSE server for timeout testing
func sseSlowHandler(t testing.TB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write([]byte("data: first response\n\n"))
		require.NoError(t, err)

		for i := 0; i < 10; i++ {
			time.Sleep(10 * time.Millisecond)
			_, err = w.Write([]byte("data: delayed response\n\n"))
			require.NoError(t, err)
		}
	})
}

// sseLineEndingsHandler sends events with different line endings to test the parser
func sseLineEndingsHandler(t testing.TB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// 1. Event with CRLF line endings
		_, err := w.Write([]byte("data: CRLF line ending\n\r\n"))
		require.NoError(t, err)

		// 2. Event with LF line endings (not preceded by CR)
		_, err = w.Write([]byte("data: LF line ending\n\n"))
		require.NoError(t, err)
	})
}

// sseHandler handles sse requests and generates some events.
// If generateErrors is true then it generates junk
// without respecting the protocol.
func sseHandler(t testing.TB, generateErrors bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if generateErrors {
			_, _ = w.Write([]byte("junk\n"))
		} else {
			if req.Method == http.MethodPost {
				assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
				assert.Equal(t, "Bearer XXXX", req.Header.Get("Authorization"))

				body, err := io.ReadAll(req.Body)
				require.NoError(t, err)
				if `{"ping": true}` != string(body) {
					t.Fail()
				}

				_, err = w.Write([]byte("id: pong\n")) // event id
				require.NoError(t, err)
				_, err = w.Write([]byte(`data: {"ping": "pong"}` + "\n\n")) // event data
				require.NoError(t, err)
			} else {
				_, err := w.Write([]byte("retry: 10000\n")) // retry
				require.NoError(t, err)

				_, err = w.Write([]byte(": hello\n")) // comment
				require.NoError(t, err)

				_, err = w.Write([]byte("id: ABCD\n")) // id
				require.NoError(t, err)

				_, err = w.Write([]byte(`data: {"ping": "pong"}` + "\n")) // event 1 data 1
				require.NoError(t, err)
				_, err = w.Write([]byte(`data: {"hello": "sse"}` + "\n\n")) // event 1 data 2
				require.NoError(t, err)

				_, err = w.Write([]byte("event: EFGH\n")) // event name
				require.NoError(t, err)
				_, err = w.Write([]byte(`data: {"hello": "sse"}` + "\n\n")) // event 2 data
				require.NoError(t, err)
			}
		}
	})
}
