package ws

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/internal/lib/testutils"
	"go.k6.io/k6/internal/lib/testutils/httpmultibin"
	httpModule "go.k6.io/k6/js/modules/k6/http"
	"go.k6.io/k6/js/modulestest"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
)

const statusProtocolSwitch = 101

func assertSessionMetricsEmitted(t *testing.T, sampleContainers []metrics.SampleContainer, subprotocol, url string, status int, group string) { //nolint:unparam
	seenSessions := false
	seenSessionDuration := false
	seenConnecting := false

	for _, sampleContainer := range sampleContainers {
		for _, sample := range sampleContainer.GetSamples() {
			tags := sample.Tags.Map()
			if tags["url"] == url {
				switch sample.Metric.Name {
				case metrics.WSConnectingName:
					seenConnecting = true
				case metrics.WSSessionDurationName:
					seenSessionDuration = true
				case metrics.WSSessionsName:
					seenSessions = true
				}

				assert.Equal(t, strconv.Itoa(status), tags["status"])
				assert.Equal(t, subprotocol, tags["subproto"])
				assert.Equal(t, group, tags["group"])
			}
		}
	}
	assert.True(t, seenConnecting, "url %s didn't emit Connecting", url)
	assert.True(t, seenSessions, "url %s didn't emit Sessions", url)
	assert.True(t, seenSessionDuration, "url %s didn't emit SessionDuration", url)
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

	registry := metrics.NewRegistry()
	state := &lib.State{
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
	require.NoError(t, testRuntime.VU.RuntimeField.Set("ws", m.Exports().Default))
	testRuntime.MoveToVUContext(state)

	return testState{
		Runtime: testRuntime,
		tb:      tb,
		samples: samples,
	}
}

func TestSessionConnectWs(t *testing.T) {
	// TODO: split and paralelize tests
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)
	sr := tb.Replacer.Replace

	test := newTestState(t)
	_, err := test.VU.Runtime().RunString(sr(`
		var res = ws.connect("WSBIN_URL/ws-echo", function(socket){
			socket.close()
		});
		if (res.status != 101) { throw new Error("connection failed with status: " + res.status); }
		`))
	require.NoError(t, err)
	assertSessionMetricsEmitted(t, metrics.GetBufferedSamples(test.samples), "", sr("WSBIN_URL/ws-echo"), statusProtocolSwitch, "")
}

func TestSessionConnectWss(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)
	sr := tb.Replacer.Replace

	test := newTestState(t)
	_, err := test.VU.Runtime().RunString(sr(`
		var res = ws.connect("WSSBIN_URL/ws-echo", function(socket){
			socket.close()
		});
		if (res.status != 101) { throw new Error("TLS connection failed with status: " + res.status); }
		`))
	require.NoError(t, err)
	assertSessionMetricsEmitted(t, metrics.GetBufferedSamples(test.samples), "", sr("WSSBIN_URL/ws-echo"), statusProtocolSwitch, "")
}

func TestSessionOpen(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)
	sr := tb.Replacer.Replace

	test := newTestState(t)
	_, err := test.VU.Runtime().RunString(sr(`
		var opened = false;
		var res = ws.connect("WSBIN_URL/ws-echo", function(socket){
			socket.on("open", function() {
				opened = true;
				socket.close()
			})
		});
		if (!opened) { throw new Error ("open event not fired"); }
		`))
	require.NoError(t, err)
	assertSessionMetricsEmitted(t, metrics.GetBufferedSamples(test.samples), "", sr("WSBIN_URL/ws-echo"), statusProtocolSwitch, "")
}

func TestSessionSendReceive(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)
	sr := tb.Replacer.Replace

	test := newTestState(t)
	_, err := test.VU.Runtime().RunString(sr(`
		var res = ws.connect("WSBIN_URL/ws-echo", function(socket){
			socket.on("open", function() {
				socket.send("test")
			})
			socket.on("message", function (data) {
				if (!data=="test") {
					throw new Error ("echo'd data doesn't match our message!");
				}
				socket.close()
			});
		});
		`))
	require.NoError(t, err)
	samplesBuf := metrics.GetBufferedSamples(test.samples)
	assertSessionMetricsEmitted(t, samplesBuf, "", sr("WSBIN_URL/ws-echo"), statusProtocolSwitch, "")
	assertMetricEmittedCount(t, metrics.WSMessagesSentName, samplesBuf, sr("WSBIN_URL/ws-echo"), 1)
	assertMetricEmittedCount(t, metrics.WSMessagesReceivedName, samplesBuf, sr("WSBIN_URL/ws-echo"), 1)
}

func TestSessionInterval(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)
	sr := tb.Replacer.Replace

	test := newTestState(t)
	_, err := test.VU.Runtime().RunString(sr(`
		var counter = 0;
		var res = ws.connect("WSBIN_URL/ws-echo", function(socket){
			socket.setInterval(function () {
				counter += 1;
				if (counter > 2) { socket.close(); }
			}, 100);
		});
		if (counter < 3) {throw new Error ("setInterval should have been called at least 3 times, counter=" + counter);}
		`))
	require.NoError(t, err)
	assertSessionMetricsEmitted(t, metrics.GetBufferedSamples(test.samples), "", sr("WSBIN_URL/ws-echo"), statusProtocolSwitch, "")
}

func TestSessionNegativeInterval(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)
	sr := tb.Replacer.Replace

	test := newTestState(t)
	_, err := test.VU.Runtime().RunString(sr(`
		var counter = 0;
		var res = ws.connect("WSBIN_URL/ws-echo", function(socket){
			socket.setInterval(function () {
				counter += 1;
				if (counter > 2) { socket.close(); }
			}, -1.23);
		});
		`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "setInterval requires a >0 timeout parameter, received -1.23 ")
}

func TestSessionIntervalSub1(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)
	sr := tb.Replacer.Replace

	test := newTestState(t)
	_, err := test.VU.Runtime().RunString(sr(`
		var counter = 0;
		var res = ws.connect("WSBIN_URL/ws-echo", function(socket){
			socket.setInterval(function () {
				counter += 1;
				if (counter > 2) { socket.close(); }
			}, 0.3);
		});
		`))
	require.NoError(t, err)
}

func TestSessionTimeout(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)
	sr := tb.Replacer.Replace

	test := newTestState(t)
	_, err := test.VU.Runtime().RunString(sr(`
		var start = new Date().getTime();
		var elapsed = new Date().getTime() - start;
		var res = ws.connect("WSBIN_URL/ws-echo", function(socket){
			socket.setTimeout(function () {
				elapsed = new Date().getTime() - start;
				socket.close();
			}, 500);
		});
		if (elapsed > 3000 || elapsed < 500) {
			throw new Error ("setTimeout occurred after " + elapsed + "ms, expected 500<T<3000")
		}`))
	require.NoError(t, err)
}

func TestSessionBadTimeout(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)
	sr := tb.Replacer.Replace

	test := newTestState(t)
	_, err := test.VU.Runtime().RunString(sr(`
		var start = new Date().getTime();
		var ellapsed = new Date().getTime() - start;
		var res = ws.connect("WSBIN_URL/ws-echo", function(socket){
			socket.setTimeout(function () {
				ellapsed = new Date().getTime() - start;
				socket.close();
			}, 0);
		});
		`))
	require.ErrorContains(t, err, "setTimeout requires a >0 timeout parameter, received 0.00 ")
	assertSessionMetricsEmitted(t, metrics.GetBufferedSamples(test.samples), "", sr("WSBIN_URL/ws-echo"), statusProtocolSwitch, "")
}

func TestSessionPing(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)
	sr := tb.Replacer.Replace

	test := newTestState(t)
	_, err := test.VU.Runtime().RunString(sr(`
		var pongReceived = false;
		var res = ws.connect("WSBIN_URL/ws-echo", function(socket){
			socket.on("open", function(data) {
				socket.ping();
			});
			socket.on("pong", function() {
				pongReceived = true;
				socket.close();
			});
			socket.setTimeout(function (){socket.close();}, 3000);
		});
		if (!pongReceived) {
			throw new Error ("sent ping but didn't get pong back");
		}
		`))
	require.NoError(t, err)
	samplesBuf := metrics.GetBufferedSamples(test.samples)
	assertSessionMetricsEmitted(t, samplesBuf, "", sr("WSBIN_URL/ws-echo"), statusProtocolSwitch, "")
	assertMetricEmittedCount(t, metrics.WSPingName, samplesBuf, sr("WSBIN_URL/ws-echo"), 1)
}

func TestSessionMultipleHandlers(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)
	sr := tb.Replacer.Replace

	test := newTestState(t)
	_, err := test.VU.Runtime().RunString(sr(`
		var pongReceived = false;
		var otherPongReceived = false;

		var res = ws.connect("WSBIN_URL/ws-echo", function(socket){
			socket.on("open", function(data) {
				socket.ping();
			});
			socket.on("pong", function() {
				pongReceived = true;
				if (otherPongReceived) {
					socket.close();
				}
			});
			socket.on("pong", function() {
				otherPongReceived = true;
				if (pongReceived) {
					socket.close();
				}
			});
			socket.setTimeout(function (){socket.close();}, 3000);
		});
		if (!pongReceived || !otherPongReceived) {
			throw new Error ("sent ping but didn't get pong back");
		}
		`))
	require.NoError(t, err)
	samplesBuf := metrics.GetBufferedSamples(test.samples)
	assertSessionMetricsEmitted(t, samplesBuf, "", sr("WSBIN_URL/ws-echo"), statusProtocolSwitch, "")
	assertMetricEmittedCount(t, metrics.WSPingName, samplesBuf, sr("WSBIN_URL/ws-echo"), 1)
}

func TestSessionClientClose(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)
	sr := tb.Replacer.Replace

	test := newTestState(t)
	_, err := test.VU.Runtime().RunString(sr(`
		var closed = false;
		var res = ws.connect("WSBIN_URL/ws-echo", function(socket){
			socket.on("open", function() {
							socket.close()
			})
			socket.on("close", function() {
							closed = true;
			})
		});
		if (!closed) { throw new Error ("close event not fired"); }
		`))
	require.NoError(t, err)
	assertSessionMetricsEmitted(t, metrics.GetBufferedSamples(test.samples), "", sr("WSBIN_URL/ws-echo"), statusProtocolSwitch, "")
}

func TestSessionClose(t *testing.T) {
	t.Parallel()
	serverCloseTests := []struct {
		name     string
		endpoint string
	}{
		{"server_close_ok", "/ws-close"},
		// Ensure we correctly handle invalid WS server
		// implementations that close the connection prematurely
		// without sending a close control frame first.
		{"server_close_invalid", "/ws-close-invalid"},
	}

	for _, tc := range serverCloseTests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tb := httpmultibin.NewHTTPMultiBin(t)
			sr := tb.Replacer.Replace

			test := newTestState(t)
			_, err := test.VU.Runtime().RunString(sr(fmt.Sprintf(`
			var closed = false;
			var res = ws.connect("WSBIN_URL%s", function(socket){
				socket.on("open", function() {
					socket.send("test");
				})
				socket.on("close", function() {
					closed = true;
				})
			});
			if (!closed) { throw new Error ("close event not fired"); }
			`, tc.endpoint)))
			require.NoError(t, err)
		})
	}
}

func TestMultiMessage(t *testing.T) {
	t.Parallel()

	registerMultiMessage := func(tb *httpmultibin.HTTPMultiBin) {
		tb.Mux.HandleFunc("/ws-echo-multi", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			conn, err := (&websocket.Upgrader{}).Upgrade(w, req, w.Header())
			if err != nil {
				return
			}

			for {
				messageType, r, e := conn.NextReader()
				if e != nil {
					return
				}
				var wc io.WriteCloser
				wc, err = conn.NextWriter(messageType)
				if err != nil {
					return
				}
				if _, err = io.Copy(wc, r); err != nil {
					return
				}
				if err = wc.Close(); err != nil {
					return
				}
			}
		}))
	}

	t.Run("send_receive_multiple_ws", func(t *testing.T) {
		t.Parallel()
		tb := httpmultibin.NewHTTPMultiBin(t)
		registerMultiMessage(tb)
		sr := tb.Replacer.Replace

		test := newTestState(t)
		_, err := test.VU.Runtime().RunString(sr(`
			var msg1 = "test1"
			var msg2 = "test2"
			var msg3 = "test3"
			var allMsgsRecvd = false
			var res = ws.connect("WSBIN_URL/ws-echo-multi", (socket) => {
				socket.on("open", () => {
					socket.send(msg1)
				})
				socket.on("message", (data) => {
					if (data == msg1){
						socket.send(msg2)
					}
					if (data == msg2){
						socket.send(msg3)
					}
					if (data == msg3){
						allMsgsRecvd = true
						socket.close()
					}
				});
			});

			if (!allMsgsRecvd) {
				throw new Error ("messages 1,2,3 in sequence, was not received from server");
			}
			`))
		require.NoError(t, err)
		samplesBuf := metrics.GetBufferedSamples(test.samples)
		assertSessionMetricsEmitted(t, samplesBuf, "", sr("WSBIN_URL/ws-echo-multi"), statusProtocolSwitch, "")
		assertMetricEmittedCount(t, metrics.WSMessagesSentName, samplesBuf, sr("WSBIN_URL/ws-echo-multi"), 3)
		assertMetricEmittedCount(t, metrics.WSMessagesReceivedName, samplesBuf, sr("WSBIN_URL/ws-echo-multi"), 3)
	})

	t.Run("send_receive_multiple_wss", func(t *testing.T) {
		t.Parallel()
		tb := httpmultibin.NewHTTPMultiBin(t)
		registerMultiMessage(tb)
		sr := tb.Replacer.Replace

		test := newTestState(t)
		_, err := test.VU.Runtime().RunString(sr(`
			var msg1 = "test1"
			var msg2 = "test2"
			var secondMsgReceived = false
			var res = ws.connect("WSSBIN_URL/ws-echo-multi", (socket) => {
				socket.on("open", () => {
					socket.send(msg1)
				})
				socket.on("message", (data) => {
					if (data == msg1){
						socket.send(msg2)
					}
					if (data == msg2){
						secondMsgReceived = true
						socket.close()
					}
				});
			});

			if (!secondMsgReceived) {
				throw new Error ("second test message was not received from server!");
			}
			`))
		require.NoError(t, err)
		samplesBuf := metrics.GetBufferedSamples(test.samples)
		assertSessionMetricsEmitted(t, samplesBuf, "", sr("WSSBIN_URL/ws-echo-multi"), statusProtocolSwitch, "")
		assertMetricEmittedCount(t, metrics.WSMessagesSentName, samplesBuf, sr("WSSBIN_URL/ws-echo-multi"), 2)
		assertMetricEmittedCount(t, metrics.WSMessagesReceivedName, samplesBuf, sr("WSSBIN_URL/ws-echo-multi"), 2)
	})

	t.Run("send_receive_text_binary", func(t *testing.T) {
		t.Parallel()
		tb := httpmultibin.NewHTTPMultiBin(t)
		registerMultiMessage(tb)
		sr := tb.Replacer.Replace

		test := newTestState(t)
		_, err := test.VU.Runtime().RunString(sr(`
			var msg1 = "test1"
			var msg2 = new Uint8Array([116, 101, 115, 116, 50]); // 'test2'
			var secondMsgReceived = false
			var res = ws.connect("WSBIN_URL/ws-echo-multi", (socket) => {
				socket.on("open", () => {
					socket.send(msg1)
				})
				socket.on("message", (data) => {
					if (data == msg1){
						socket.sendBinary(msg2.buffer)
					}
				});
				socket.on("binaryMessage", (data) => {
					let data2 = new Uint8Array(data)
					if(JSON.stringify(msg2) == JSON.stringify(data2)){
						secondMsgReceived = true
					}
					socket.close()
				})
			});

			if (!secondMsgReceived) {
				throw new Error ("second test message was not received from server!");
			}
			`))
		require.NoError(t, err)
		samplesBuf := metrics.GetBufferedSamples(test.samples)
		assertSessionMetricsEmitted(t, samplesBuf, "", sr("WSBIN_URL/ws-echo-multi"), statusProtocolSwitch, "")
		assertMetricEmittedCount(t, metrics.WSMessagesSentName, samplesBuf, sr("WSBIN_URL/ws-echo-multi"), 2)
		assertMetricEmittedCount(t, metrics.WSMessagesReceivedName, samplesBuf, sr("WSBIN_URL/ws-echo-multi"), 2)
	})
}

func TestSocketSendBinary(t *testing.T) { //nolint:tparallel
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)
	sr := tb.Replacer.Replace

	test := newTestState(t)

	t.Run("ok", func(t *testing.T) {
		_, err := test.VU.Runtime().RunString(sr(`
		var gotMsg = false;
		var res = ws.connect('WSBIN_URL/ws-echo', function(socket){
			var data = new Uint8Array([104, 101, 108, 108, 111]); // 'hello'

			socket.on('open', function() {
				socket.sendBinary(data.buffer);
			})
			socket.on('binaryMessage', function(msg) {
				gotMsg = true;
				let decText = String.fromCharCode.apply(null, new Uint8Array(msg));
				decText = decodeURIComponent(escape(decText));
				if (decText !== 'hello') {
					throw new Error('received unexpected binary message: ' + decText);
				}
				socket.close()
			});
		});
		if (!gotMsg) {
			throw new Error("the 'binaryMessage' handler wasn't called")
		}
		`))
		require.NoError(t, err)
	})

	errTestCases := []struct {
		in, expErrType string
	}{
		{"", ""},
		{"undefined", "undefined"},
		{"null", "null"},
		{"true", "Boolean"},
		{"1", "Number"},
		{"3.14", "Number"},
		{"'str'", "String"},
		{"[1, 2, 3]", "Array"},
		{"new Uint8Array([1, 2, 3])", "Uint8Array"},
		{"Symbol('a')", "Symbol"},
		{"function() {}", "Function"},
	}

	for _, tc := range errTestCases { //nolint:paralleltest
		tc := tc
		t.Run(fmt.Sprintf("err_%s", tc.expErrType), func(t *testing.T) {
			_, err := test.VU.Runtime().RunString(fmt.Sprintf(sr(`
			var res = ws.connect('WSBIN_URL/ws-echo', function(socket){
				socket.on('open', function() {
					socket.sendBinary(%s);
				})
			});
		`), tc.in))
			require.Error(t, err)
			if tc.in == "" {
				assert.Contains(t, err.Error(), "missing argument, expected ArrayBuffer")
			} else {
				assert.Contains(t, err.Error(), fmt.Sprintf("expected ArrayBuffer as argument, received: %s", tc.expErrType))
			}
		})
	}
}

func TestErrors(t *testing.T) {
	t.Parallel()

	t.Run("invalid_url", func(t *testing.T) {
		t.Parallel()

		test := newTestState(t)
		_, err := test.VU.Runtime().RunString(`
		var res = ws.connect("INVALID", function(socket){
			socket.on("open", function() {
				socket.close();
			});
		});
		`)
		assert.Error(t, err)
	})

	t.Run("invalid_url_message_panic", func(t *testing.T) {
		t.Parallel()

		test := newTestState(t)
		// Attempting to send a message to a non-existent socket shouldn't panic
		_, err := test.VU.Runtime().RunString(`
		var res = ws.connect("INVALID", function(socket){
			socket.send("new message");
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
		var res = ws.connect("WSBIN_URL/ws-echo-invalid", function(socket){
			throw new Error("error in setup");
		});
		`))
		assert.Error(t, err)
	})

	t.Run("send_after_close", func(t *testing.T) {
		t.Parallel()
		tb := httpmultibin.NewHTTPMultiBin(t)
		sr := tb.Replacer.Replace

		test := newTestState(t)
		_, err := test.VU.Runtime().RunString(sr(`
		var hasError = false;
		var res = ws.connect("WSBIN_URL/ws-echo-invalid", function(socket){
			socket.on("open", function() {
				socket.close();
				socket.send("test");
			});

			socket.on("error", function(errorEvent) {
				hasError = true;
			});
		});
		if (!hasError) {
			throw new Error ("no error emitted for send after close");
		}
		`))
		require.NoError(t, err)
		assertSessionMetricsEmitted(t, metrics.GetBufferedSamples(test.samples), "", sr("WSBIN_URL/ws-echo-invalid"), statusProtocolSwitch, "")
	})
}

func TestConnectWrongStatusCode(t *testing.T) {
	t.Parallel()
	test := newTestState(t)
	tb := httpmultibin.NewHTTPMultiBin(t)
	sr := tb.Replacer.Replace
	test.VU.StateField.Options.Throw = null.BoolFrom(false)
	_, err := test.VU.Runtime().RunString(sr(`
	var res = ws.connect("WSBIN_URL/status/404", function(socket){});
	if (res.status != 404) {
		throw new Error ("no status code set for invalid response");
	}
	`))
	assert.NoError(t, err)
}

func TestSystemTags(t *testing.T) {
	t.Parallel()
	testedSystemTags := []string{"status", "subproto", "url", "ip"}
	for _, expectedTagStr := range testedSystemTags {
		expectedTagStr := expectedTagStr
		t.Run("only "+expectedTagStr, func(t *testing.T) {
			t.Parallel()
			test := newTestState(t)
			expectedTag, err := metrics.SystemTagString(expectedTagStr)
			require.NoError(t, err)
			tb := httpmultibin.NewHTTPMultiBin(t)
			test.VU.StateField.Options.SystemTags = metrics.ToSystemTagSet([]string{expectedTagStr})
			_, err = test.VU.Runtime().RunString(tb.Replacer.Replace(`
			var res = ws.connect("WSBIN_URL/ws-echo", function(socket){
				socket.on("open", function() {
					socket.send("test")
				})
				socket.on("message", function (data){
					if (!data=="test") {
						throw new Error ("echo'd data doesn't match our message!");
					}
					socket.close()
				});
			});
			`))
			require.NoError(t, err)
			containers := metrics.GetBufferedSamples(test.samples)
			require.NotEmpty(t, containers)
			for _, sampleContainer := range containers {
				require.NotEmpty(t, sampleContainer.GetSamples())
				for _, sample := range sampleContainer.GetSamples() {
					var dataToCheck map[string]string
					if metrics.NonIndexableSystemTags.Has(expectedTag) {
						dataToCheck = sample.Metadata
					} else {
						dataToCheck = sample.Tags.Map()
					}

					require.NotEmpty(t, dataToCheck)
					for emittedTag := range dataToCheck {
						assert.Equal(t, expectedTagStr, emittedTag)
					}
				}
			}
		})
	}
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
		var res = ws.connect("WSSBIN_URL/ws-echo", function(socket){
			socket.close()
		});
		if (res.status != 101) { throw new Error("TLS connection failed with status: " + res.status); }
		`))
		require.NoError(t, err)
		assertSessionMetricsEmitted(t, metrics.GetBufferedSamples(test.samples), "", sr("WSSBIN_URL/ws-echo"), statusProtocolSwitch, "")
	})

	t.Run("custom certificates", func(t *testing.T) {
		t.Parallel()
		tb := httpmultibin.NewHTTPMultiBin(t)
		sr := tb.Replacer.Replace

		test := newTestState(t)
		test.VU.StateField.TLSConfig = tb.TLSClientConfig

		_, err := test.VU.Runtime().RunString(sr(`
			var res = ws.connect("WSSBIN_URL/ws-echo", function(socket){
				socket.close()
			});
			if (res.status != 101) {
				throw new Error("TLS connection failed with status: " + res.status);
			}
		`))
		require.NoError(t, err)
		assertSessionMetricsEmitted(t, metrics.GetBufferedSamples(test.samples), "", sr("WSSBIN_URL/ws-echo"), statusProtocolSwitch, "")
	})
}

func TestReadPump(t *testing.T) {
	t.Parallel()

	closeCodes := []int{websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseInternalServerErr}

	// Ensure readPump returns the response close code sent by the server
	for _, code := range closeCodes {
		code := code
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			t.Parallel()
			closeCode := code
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := (&websocket.Upgrader{}).Upgrade(w, r, w.Header())
				require.NoError(t, err)
				closeMsg := websocket.FormatCloseMessage(closeCode, "")
				_ = conn.WriteControl(websocket.CloseMessage, closeMsg, time.Now().Add(time.Second))
			}))
			numAsserts := 0

			t.Cleanup(srv.Close)
			srvURL := "ws://" + srv.Listener.Addr().String()
			conn, resp, err := websocket.DefaultDialer.Dial(srvURL, nil)
			require.NoError(t, err)
			defer func() {
				_ = resp.Body.Close()
				_ = conn.Close()
			}()

			msgChan := make(chan *message)
			errChan := make(chan error)
			closeChan := make(chan int)
			s := &Socket{conn: conn}
			go s.readPump(msgChan, errChan, closeChan)

		readChans:
			for {
				select {
				case responseCode := <-closeChan:
					assert.Equal(t, code, responseCode)
					numAsserts++
					break readChans
				case <-errChan:
					continue
				case <-time.After(time.Second):
					t.Errorf("Read timed out")
					break readChans
				}
			}
			assert.Equal(t, numAsserts, 1)
		})
	}

	// Ensure all close code asserts passed
}

func TestUserAgent(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)
	sr := tb.Replacer.Replace

	tb.Mux.HandleFunc("/ws-echo-useragent", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// Echo back User-Agent header if it exists
		responseHeaders := w.Header().Clone()
		if ua := req.Header.Get("User-Agent"); ua != "" {
			responseHeaders.Add("Echo-User-Agent", req.Header.Get("User-Agent"))
		}

		conn, err := (&websocket.Upgrader{}).Upgrade(w, req, responseHeaders)
		if err != nil {
			t.Fatalf("/ws-echo-useragent cannot upgrade request: %v", err)
			return
		}

		err = conn.Close()
		if err != nil {
			t.Logf("error while closing connection in /ws-echo-useragent: %v", err)
			return
		}
	}))

	test := newTestState(t)

	// websocket handler should echo back User-Agent as Echo-User-Agent for this test to work
	_, err := test.VU.Runtime().RunString(sr(`
		var res = ws.connect("WSBIN_URL/ws-echo-useragent", function(socket){
			socket.close()
		})
		var userAgent = res.headers["Echo-User-Agent"];
		if (userAgent == undefined) {
			throw new Error("user agent is not echoed back by test server");
		}
		if (userAgent != "TestUserAgent") {
			throw new Error("incorrect user agent: " + userAgent);
		}
		`))
	require.NoError(t, err)

	assertSessionMetricsEmitted(t, metrics.GetBufferedSamples(test.samples), "", sr("WSBIN_URL/ws-echo-useragent"), statusProtocolSwitch, "")
}

func TestCompression(t *testing.T) {
	t.Parallel()

	t.Run("session", func(t *testing.T) {
		t.Parallel()
		const text string = `Lorem ipsum dolor sit amet, consectetur adipiscing elit. Maecenas sed pharetra sapien. Nunc laoreet molestie ante ac gravida. Etiam interdum dui viverra posuere egestas. Pellentesque at dolor tristique, mattis turpis eget, commodo purus. Nunc orci aliquam.`

		ts := newTestState(t)
		sr := ts.tb.Replacer.Replace
		ts.tb.Mux.HandleFunc("/ws-compression", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			upgrader := websocket.Upgrader{
				EnableCompression: true,
				ReadBufferSize:    1024,
				WriteBufferSize:   1024,
			}

			conn, e := upgrader.Upgrade(w, req, w.Header())
			if e != nil {
				t.Fatalf("/ws-compression cannot upgrade request: %v", e)
				return
			}

			// send a message and exit
			if e = conn.WriteMessage(websocket.TextMessage, []byte(text)); e != nil {
				t.Logf("error while sending message in /ws-compression: %v", e)
				return
			}

			e = conn.Close()
			if e != nil {
				t.Logf("error while closing connection in /ws-compression: %v", e)
				return
			}
		}))

		_, err := ts.VU.Runtime().RunString(sr(`
		// if client supports compression, it has to send the header
		// 'Sec-Websocket-Extensions:permessage-deflate; server_no_context_takeover; client_no_context_takeover' to server.
		// if compression is negotiated successfully, server will reply with header
		// 'Sec-Websocket-Extensions:permessage-deflate; server_no_context_takeover; client_no_context_takeover'

		var params = {
			"compression": "deflate"
		}
		var res = ws.connect("WSBIN_URL/ws-compression", params, function(socket){
			socket.on('message', (data) => {
				if(data != "` + text + `"){
					throw new Error("wrong message received from server: ", data)
				}
				socket.close()
			})
		});

		var wsExtensions = res.headers["Sec-Websocket-Extensions"].split(';').map(e => e.trim())
		if (!(wsExtensions.includes("permessage-deflate") && wsExtensions.includes("server_no_context_takeover") && wsExtensions.includes("client_no_context_takeover"))){
			throw new Error("websocket compression negotiation failed");
		}
		`))

		require.NoError(t, err)
		assertSessionMetricsEmitted(t, metrics.GetBufferedSamples(ts.samples), "", sr("WSBIN_URL/ws-compression"), statusProtocolSwitch, "")
	})

	t.Run("params", func(t *testing.T) {
		t.Parallel()
		testCases := []struct {
			compression   string
			expectedError string
		}{
			{compression: ""},
			{compression: "  "},
			{compression: "deflate"},
			{compression: "deflate "},
			{
				compression:   "gzip",
				expectedError: `unsupported compression algorithm 'gzip', supported algorithm is 'deflate'`,
			},
			{
				compression:   "deflate, gzip",
				expectedError: `unsupported compression algorithm 'deflate, gzip', supported algorithm is 'deflate'`,
			},
			{
				compression:   "deflate, deflate",
				expectedError: `unsupported compression algorithm 'deflate, deflate', supported algorithm is 'deflate'`,
			},
			{
				compression:   "deflate, ",
				expectedError: `unsupported compression algorithm 'deflate,', supported algorithm is 'deflate'`,
			},
		}

		for _, testCase := range testCases {
			testCase := testCase
			t.Run(testCase.compression, func(t *testing.T) {
				t.Parallel()
				ts := newTestState(t)
				sr := ts.tb.Replacer.Replace
				ts.tb.Mux.HandleFunc("/ws-compression-param", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					upgrader := websocket.Upgrader{
						EnableCompression: true,
						ReadBufferSize:    1024,
						WriteBufferSize:   1024,
					}

					conn, e := upgrader.Upgrade(w, req, w.Header())
					if e != nil {
						t.Fatalf("/ws-compression-param cannot upgrade request: %v", e)
						return
					}

					e = conn.Close()
					if e != nil {
						t.Logf("error while closing connection in /ws-compression-param: %v", e)
						return
					}
				}))

				_, err := ts.VU.Runtime().RunString(sr(`
					var res = ws.connect("WSBIN_URL/ws-compression-param", {"compression":"` + testCase.compression + `"}, function(socket){
						socket.close()
					});
				`))

				if testCase.expectedError == "" {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
					require.Contains(t, err.Error(), testCase.expectedError)
				}
			})
		}
	})
}

func clearSamples(tb *httpmultibin.HTTPMultiBin, samples chan metrics.SampleContainer) {
	ctxDone := tb.Context.Done()
	for {
		select {
		case <-samples:
		case <-ctxDone:
			return
		}
	}
}

func BenchmarkCompression(b *testing.B) {
	const textMessage = 1
	ts := newTestState(b)
	sr := ts.tb.Replacer.Replace
	go clearSamples(ts.tb, ts.samples)

	testCodes := []string{
		sr(`
		var res = ws.connect("WSBIN_URL/ws-compression", {"compression":"deflate"}, (socket) => {
			socket.on('message', (data) => {
				socket.close()
			})
		});
		`),
		sr(`
		var res = ws.connect("WSBIN_URL/ws-compression", {}, (socket) => {
			socket.on('message', (data) => {
				socket.close()
			})
		});
		`),
	}

	ts.tb.Mux.HandleFunc("/ws-compression", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		kbData := bytes.Repeat([]byte("0123456789"), 100)

		// upgrade connection, send the first (long) message, disconnect
		upgrader := websocket.Upgrader{
			EnableCompression: true,
			ReadBufferSize:    1024,
			WriteBufferSize:   1024,
		}

		conn, e := upgrader.Upgrade(w, req, w.Header())

		if e != nil {
			b.Fatalf("/ws-compression cannot upgrade request: %v", e)
			return
		}

		if e = conn.WriteMessage(textMessage, kbData); e != nil {
			b.Fatalf("/ws-compression cannot write message: %v", e)
			return
		}

		e = conn.Close()
		if e != nil {
			b.Logf("error while closing connection in /ws-compression: %v", e)
			return
		}
	}))

	b.ResetTimer()
	b.Run("compression-enabled", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if _, err := ts.VU.Runtime().RunString(testCodes[0]); err != nil {
				b.Error(err)
			}
		}
	})
	b.Run("compression-disabled", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if _, err := ts.VU.Runtime().RunString(testCodes[1]); err != nil {
				b.Error(err)
			}
		}
	})
}

func TestCookieJar(t *testing.T) {
	t.Parallel()
	ts := newTestState(t)
	sr := ts.tb.Replacer.Replace

	ts.tb.Mux.HandleFunc("/ws-echo-someheader", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		responseHeaders := w.Header().Clone()
		if sh, err := req.Cookie("someheader"); err == nil {
			responseHeaders.Add("Echo-Someheader", sh.Value)
		}

		conn, err := (&websocket.Upgrader{}).Upgrade(w, req, responseHeaders)
		if err != nil {
			t.Fatalf("/ws-echo-someheader cannot upgrade request: %v", err)
		}

		err = conn.Close()
		if err != nil {
			t.Logf("error while closing connection in /ws-echo-someheader: %v", err)
		}
	}))

	err := ts.VU.Runtime().Set("http", httpModule.New().NewModuleInstance(ts.VU).Exports().Default)
	require.NoError(t, err)
	ts.VU.State().CookieJar, _ = cookiejar.New(nil)

	_, err = ts.VU.Runtime().RunString(sr(`
		var res = ws.connect("WSBIN_URL/ws-echo-someheader", function(socket){
			socket.close()
		})
		var someheader = res.headers["Echo-Someheader"];
		if (someheader !== undefined) {
			throw new Error("someheader is echoed back by test server even though it doesn't exist");
		}

		http.cookieJar().set("HTTPBIN_URL/ws-echo-someheader", "someheader", "defaultjar")
		res = ws.connect("WSBIN_URL/ws-echo-someheader", function(socket){
			socket.close()
		})
		someheader = res.headers["Echo-Someheader"];
		if (someheader != "defaultjar") {
			throw new Error("someheader has wrong value "+ someheader + " instead of defaultjar");
		}

		var jar = new http.CookieJar();
		jar.set("HTTPBIN_URL/ws-echo-someheader", "someheader", "customjar")
		res = ws.connect("WSBIN_URL/ws-echo-someheader", {jar: jar}, function(socket){
			socket.close()
		})
		someheader = res.headers["Echo-Someheader"];
		if (someheader != "customjar") {
			throw new Error("someheader has wrong value "+ someheader + " instead of customjar");
		}
		`))
	require.NoError(t, err)

	assertSessionMetricsEmitted(t, metrics.GetBufferedSamples(ts.samples), "", sr("WSBIN_URL/ws-echo-someheader"), statusProtocolSwitch, "")
}

func TestWSConnectEnableThrowErrorOption(t *testing.T) {
	t.Parallel()
	logHook := testutils.NewLogHook(logrus.WarnLevel)
	testLog := logrus.New()
	testLog.AddHook(logHook)
	testLog.SetOutput(io.Discard)
	ts := newTestState(t)
	ts.VU.StateField.Logger = testLog
	_, err := ts.VU.Runtime().RunString(`
		var res = ws.connect("INVALID", function(socket){
			socket.on("open", function() {
				socket.close();
			});
		});
		`)
	entries := logHook.Drain()
	require.Len(t, entries, 0)
	assert.Error(t, err)
}

func TestWSConnectDisableThrowErrorOption(t *testing.T) {
	t.Parallel()
	logHook := testutils.NewLogHook(logrus.WarnLevel)
	testLog := logrus.New()
	testLog.AddHook(logHook)
	testLog.SetOutput(io.Discard)

	ts := newTestState(t)
	ts.VU.StateField.Logger = testLog
	ts.VU.StateField.Options.Throw = null.BoolFrom(false)
	_, err := ts.VU.Runtime().RunString(`
		var res = ws.connect("INVALID", function(socket){
			socket.on("open", function() {
				socket.close();
			});
		});
		if (res == null && res.error == null) {
			throw new Error("res.error is expected to be not null");
		}

		`)
	require.NoError(t, err)
	entries := logHook.Drain()
	assert.Empty(t, entries)
}
