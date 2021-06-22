/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2017 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */
package ws

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/metrics"
	"go.k6.io/k6/lib/testutils/httpmultibin"
	"go.k6.io/k6/stats"
)

func assertSessionMetricsEmitted(t *testing.T, sampleContainers []stats.SampleContainer, subprotocol, url string, status int, group string) {
	seenSessions := false
	seenSessionDuration := false
	seenConnecting := false

	for _, sampleContainer := range sampleContainers {
		for _, sample := range sampleContainer.GetSamples() {
			tags := sample.Tags.CloneTags()
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

func assertMetricEmitted(t *testing.T, metricName string, sampleContainers []stats.SampleContainer, url string) {
	seenMetric := false

	for _, sampleContainer := range sampleContainers {
		for _, sample := range sampleContainer.GetSamples() {
			surl, ok := sample.Tags.Get("url")
			assert.True(t, ok)
			if surl == url {
				if sample.Metric.Name == metricName {
					seenMetric = true
				}
			}
		}
	}
	assert.True(t, seenMetric, "url %s didn't emit %s", url, metricName)
}

func TestSession(t *testing.T) {
	// TODO: split and paralelize tests
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)
	sr := tb.Replacer.Replace

	root, err := lib.NewGroup("", nil)
	assert.NoError(t, err)

	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})
	samples := make(chan stats.SampleContainer, 1000)
	state := &lib.State{
		Group:  root,
		Dialer: tb.Dialer,
		Options: lib.Options{
			SystemTags: stats.NewSystemTagSet(
				stats.TagURL,
				stats.TagProto,
				stats.TagStatus,
				stats.TagSubproto,
			),
		},
		Samples:        samples,
		TLSConfig:      tb.TLSClientConfig,
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(metrics.NewRegistry()),
	}

	ctx := context.Background()
	ctx = lib.WithState(ctx, state)
	ctx = common.WithRuntime(ctx, rt)

	rt.Set("ws", common.Bind(rt, New(), &ctx))

	t.Run("connect_ws", func(t *testing.T) {
		_, err := rt.RunString(sr(`
		var res = ws.connect("WSBIN_URL/ws-echo", function(socket){
			socket.close()
		});
		if (res.status != 101) { throw new Error("connection failed with status: " + res.status); }
		`))
		assert.NoError(t, err)
	})
	assertSessionMetricsEmitted(t, stats.GetBufferedSamples(samples), "", sr("WSBIN_URL/ws-echo"), 101, "")

	t.Run("connect_wss", func(t *testing.T) {
		_, err := rt.RunString(sr(`
		var res = ws.connect("WSSBIN_URL/ws-echo", function(socket){
			socket.close()
		});
		if (res.status != 101) { throw new Error("TLS connection failed with status: " + res.status); }
		`))
		assert.NoError(t, err)
	})
	assertSessionMetricsEmitted(t, stats.GetBufferedSamples(samples), "", sr("WSSBIN_URL/ws-echo"), 101, "")

	t.Run("open", func(t *testing.T) {
		_, err := rt.RunString(sr(`
		var opened = false;
		var res = ws.connect("WSBIN_URL/ws-echo", function(socket){
			socket.on("open", function() {
				opened = true;
				socket.close()
			})
		});
		if (!opened) { throw new Error ("open event not fired"); }
		`))
		assert.NoError(t, err)
	})
	assertSessionMetricsEmitted(t, stats.GetBufferedSamples(samples), "", sr("WSBIN_URL/ws-echo"), 101, "")

	t.Run("send_receive", func(t *testing.T) {
		_, err := rt.RunString(sr(`
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
		assert.NoError(t, err)
	})

	samplesBuf := stats.GetBufferedSamples(samples)
	assertSessionMetricsEmitted(t, samplesBuf, "", sr("WSBIN_URL/ws-echo"), 101, "")
	assertMetricEmitted(t, metrics.WSMessagesSentName, samplesBuf, sr("WSBIN_URL/ws-echo"))
	assertMetricEmitted(t, metrics.WSMessagesReceivedName, samplesBuf, sr("WSBIN_URL/ws-echo"))

	t.Run("interval", func(t *testing.T) {
		_, err := rt.RunString(sr(`
		var counter = 0;
		var res = ws.connect("WSBIN_URL/ws-echo", function(socket){
			socket.setInterval(function () {
				counter += 1;
				if (counter > 2) { socket.close(); }
			}, 100);
		});
		if (counter < 3) {throw new Error ("setInterval should have been called at least 3 times, counter=" + counter);}
		`))
		assert.NoError(t, err)
	})
	assertSessionMetricsEmitted(t, stats.GetBufferedSamples(samples), "", sr("WSBIN_URL/ws-echo"), 101, "")
	t.Run("bad interval", func(t *testing.T) {
		_, err := rt.RunString(sr(`
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
	})

	t.Run("timeout", func(t *testing.T) {
		_, err := rt.RunString(sr(`
		var start = new Date().getTime();
		var ellapsed = new Date().getTime() - start;
		var res = ws.connect("WSBIN_URL/ws-echo", function(socket){
			socket.setTimeout(function () {
				ellapsed = new Date().getTime() - start;
				socket.close();
			}, 500);
		});
		if (ellapsed > 3000 || ellapsed < 500) {
			throw new Error ("setTimeout occurred after " + ellapsed + "ms, expected 500<T<3000");
		}
		`))
		assert.NoError(t, err)
	})

	t.Run("bad timeout", func(t *testing.T) {
		_, err := rt.RunString(sr(`
		var start = new Date().getTime();
		var ellapsed = new Date().getTime() - start;
		var res = ws.connect("WSBIN_URL/ws-echo", function(socket){
			socket.setTimeout(function () {
				ellapsed = new Date().getTime() - start;
				socket.close();
			}, 0);
		});
		`))
		require.Error(t, err)
		require.Contains(t, err.Error(), "setTimeout requires a >0 timeout parameter, received 0.00 ")
	})
	assertSessionMetricsEmitted(t, stats.GetBufferedSamples(samples), "", sr("WSBIN_URL/ws-echo"), 101, "")

	t.Run("ping", func(t *testing.T) {
		_, err := rt.RunString(sr(`
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
		assert.NoError(t, err)
	})

	samplesBuf = stats.GetBufferedSamples(samples)
	assertSessionMetricsEmitted(t, samplesBuf, "", sr("WSBIN_URL/ws-echo"), 101, "")
	assertMetricEmitted(t, metrics.WSPingName, samplesBuf, sr("WSBIN_URL/ws-echo"))

	t.Run("multiple_handlers", func(t *testing.T) {
		_, err := rt.RunString(sr(`
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
		assert.NoError(t, err)
	})

	samplesBuf = stats.GetBufferedSamples(samples)
	assertSessionMetricsEmitted(t, samplesBuf, "", sr("WSBIN_URL/ws-echo"), 101, "")
	assertMetricEmitted(t, metrics.WSPingName, samplesBuf, sr("WSBIN_URL/ws-echo"))

	t.Run("client_close", func(t *testing.T) {
		_, err := rt.RunString(sr(`
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
		assert.NoError(t, err)
	})
	assertSessionMetricsEmitted(t, stats.GetBufferedSamples(samples), "", sr("WSBIN_URL/ws-echo"), 101, "")

	serverCloseTests := []struct {
		name     string
		endpoint string
	}{
		{"server_close_ok", "/ws-echo"},
		// Ensure we correctly handle invalid WS server
		// implementations that close the connection prematurely
		// without sending a close control frame first.
		{"server_close_invalid", "/ws-close-invalid"},
	}

	for _, tc := range serverCloseTests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, err := rt.RunString(sr(fmt.Sprintf(`
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
			assert.NoError(t, err)
		})
	}
}

func TestSocketSendBinary(t *testing.T) { //nolint: tparallel
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)
	sr := tb.Replacer.Replace

	root, err := lib.NewGroup("", nil)
	assert.NoError(t, err)

	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})
	samples := make(chan stats.SampleContainer, 1000)
	state := &lib.State{ //nolint: exhaustivestruct
		Group:  root,
		Dialer: tb.Dialer,
		Options: lib.Options{ //nolint: exhaustivestruct
			SystemTags: stats.NewSystemTagSet(
				stats.TagURL,
				stats.TagProto,
				stats.TagStatus,
				stats.TagSubproto,
			),
		},
		Samples:        samples,
		TLSConfig:      tb.TLSClientConfig,
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(metrics.NewRegistry()),
	}

	ctx := context.Background()
	ctx = lib.WithState(ctx, state)
	ctx = common.WithRuntime(ctx, rt)

	err = rt.Set("ws", common.Bind(rt, New(), &ctx))
	assert.NoError(t, err)

	t.Run("ok", func(t *testing.T) {
		_, err = rt.RunString(sr(`
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
		assert.NoError(t, err)
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
		{"new Uint8Array([1, 2, 3])", "Object"},
		{"Symbol('a')", "Symbol"},
		{"function() {}", "Function"},
	}

	for _, tc := range errTestCases { //nolint: paralleltest
		tc := tc
		t.Run(fmt.Sprintf("err_%s", tc.expErrType), func(t *testing.T) {
			_, err = rt.RunString(fmt.Sprintf(sr(`
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
	tb := httpmultibin.NewHTTPMultiBin(t)
	sr := tb.Replacer.Replace

	root, err := lib.NewGroup("", nil)
	assert.NoError(t, err)

	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})
	samples := make(chan stats.SampleContainer, 1000)
	state := &lib.State{
		Group:  root,
		Dialer: tb.Dialer,
		Options: lib.Options{
			SystemTags: &stats.DefaultSystemTagSet,
		},
		Samples:        samples,
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(metrics.NewRegistry()),
	}

	ctx := context.Background()
	ctx = lib.WithState(ctx, state)
	ctx = common.WithRuntime(ctx, rt)

	rt.Set("ws", common.Bind(rt, New(), &ctx))

	t.Run("invalid_url", func(t *testing.T) {
		_, err := rt.RunString(`
		var res = ws.connect("INVALID", function(socket){
			socket.on("open", function() {
				socket.close();
			});
		});
		`)
		assert.Error(t, err)
	})

	t.Run("invalid_url_message_panic", func(t *testing.T) {
		// Attempting to send a message to a non-existent socket shouldn't panic
		_, err := rt.RunString(`
		var res = ws.connect("INVALID", function(socket){
			socket.send("new message");
		});
		`)
		assert.Error(t, err)
	})

	t.Run("error_in_setup", func(t *testing.T) {
		_, err := rt.RunString(sr(`
		var res = ws.connect("WSBIN_URL/ws-echo-invalid", function(socket){
			throw new Error("error in setup");
		});
		`))
		assert.Error(t, err)
	})

	t.Run("send_after_close", func(t *testing.T) {
		_, err := rt.RunString(sr(`
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
		assert.NoError(t, err)
		assertSessionMetricsEmitted(t, stats.GetBufferedSamples(samples), "", sr("WSBIN_URL/ws-echo-invalid"), 101, "")
	})

	t.Run("error on close", func(t *testing.T) {
		_, err := rt.RunString(sr(`
		var closed = false;
		var res = ws.connect("WSBIN_URL/ws-close", function(socket){
			socket.on('open', function open() {
				socket.setInterval(function timeout() {
				  socket.ping();
				}, 1000);
			});

			socket.on("ping", function() {
				socket.close();
			});

			socket.on("error", function(errorEvent) {
				if (errorEvent == null) {
					throw new Error(JSON.stringify(errorEvent));
				}
				if (!closed) {
					closed = true;
				    socket.close();
				}
			});
		});
		`))
		assert.NoError(t, err)
		assertSessionMetricsEmitted(t, stats.GetBufferedSamples(samples), "", sr("WSBIN_URL/ws-close"), 101, "")
	})
}

func TestSystemTags(t *testing.T) {
	tb := httpmultibin.NewHTTPMultiBin(t)

	sr := tb.Replacer.Replace

	root, err := lib.NewGroup("", nil)
	assert.NoError(t, err)

	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})

	// TODO: test for actual tag values after removing the dependency on the
	// external service demos.kaazing.com (https://github.com/k6io/k6/issues/537)
	testedSystemTags := []string{"group", "status", "subproto", "url", "ip"}

	samples := make(chan stats.SampleContainer, 1000)
	state := &lib.State{
		Group:          root,
		Dialer:         tb.Dialer,
		Options:        lib.Options{SystemTags: stats.ToSystemTagSet(testedSystemTags)},
		Samples:        samples,
		TLSConfig:      tb.TLSClientConfig,
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(metrics.NewRegistry()),
	}

	ctx := context.Background()
	ctx = lib.WithState(ctx, state)
	ctx = common.WithRuntime(ctx, rt)

	rt.Set("ws", common.Bind(rt, New(), &ctx))

	for _, expectedTag := range testedSystemTags {
		expectedTag := expectedTag
		t.Run("only "+expectedTag, func(t *testing.T) {
			state.Options.SystemTags = stats.ToSystemTagSet([]string{expectedTag})
			_, err := rt.RunString(sr(`
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
			assert.NoError(t, err)

			for _, sampleContainer := range stats.GetBufferedSamples(samples) {
				for _, sample := range sampleContainer.GetSamples() {
					for emittedTag := range sample.Tags.CloneTags() {
						assert.Equal(t, expectedTag, emittedTag)
					}
				}
			}
		})
	}
}

func TestTLSConfig(t *testing.T) {
	root, err := lib.NewGroup("", nil)
	assert.NoError(t, err)

	tb := httpmultibin.NewHTTPMultiBin(t)

	sr := tb.Replacer.Replace

	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})
	samples := make(chan stats.SampleContainer, 1000)
	state := &lib.State{
		Group:  root,
		Dialer: tb.Dialer,
		Options: lib.Options{
			SystemTags: stats.NewSystemTagSet(
				stats.TagURL,
				stats.TagProto,
				stats.TagStatus,
				stats.TagSubproto,
				stats.TagIP,
			),
		},
		Samples:        samples,
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(metrics.NewRegistry()),
	}

	ctx := context.Background()
	ctx = lib.WithState(ctx, state)
	ctx = common.WithRuntime(ctx, rt)

	rt.Set("ws", common.Bind(rt, New(), &ctx))

	t.Run("insecure skip verify", func(t *testing.T) {
		state.TLSConfig = &tls.Config{
			InsecureSkipVerify: true,
		}

		_, err := rt.RunString(sr(`
		var res = ws.connect("WSSBIN_URL/ws-close", function(socket){
			socket.close()
		});
		if (res.status != 101) { throw new Error("TLS connection failed with status: " + res.status); }
		`))
		assert.NoError(t, err)
	})
	assertSessionMetricsEmitted(t, stats.GetBufferedSamples(samples), "", sr("WSSBIN_URL/ws-close"), 101, "")

	t.Run("custom certificates", func(t *testing.T) {
		state.TLSConfig = tb.TLSClientConfig

		_, err := rt.RunString(sr(`
			var res = ws.connect("WSSBIN_URL/ws-close", function(socket){
				socket.close()
			});
			if (res.status != 101) {
				throw new Error("TLS connection failed with status: " + res.status);
			}
		`))
		assert.NoError(t, err)
	})
	assertSessionMetricsEmitted(t, stats.GetBufferedSamples(samples), "", sr("WSSBIN_URL/ws-close"), 101, "")
}

func TestReadPump(t *testing.T) {
	var closeCode int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, w.Header())
		assert.NoError(t, err)
		closeMsg := websocket.FormatCloseMessage(closeCode, "")
		_ = conn.WriteControl(websocket.CloseMessage, closeMsg, time.Now().Add(time.Second))
	}))
	defer srv.Close()

	closeCodes := []int{websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseInternalServerErr}

	numAsserts := 0
	srvURL := "ws://" + srv.Listener.Addr().String()

	// Ensure readPump returns the response close code sent by the server
	for _, code := range closeCodes {
		code := code
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			closeCode = code
			conn, resp, err := websocket.DefaultDialer.Dial(srvURL, nil)
			assert.NoError(t, err)
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
		})
	}

	// Ensure all close code asserts passed
	assert.Equal(t, numAsserts, len(closeCodes))
}
