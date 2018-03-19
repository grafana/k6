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
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/gorilla/websocket"
	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/lib/netext"
	"github.com/loadimpact/k6/stats"
	"github.com/stretchr/testify/assert"
)

func assertSessionMetricsEmitted(t *testing.T, samples []stats.Sample, subprotocol, url string, status int, group string) {
	seenSessions := false
	seenSessionDuration := false
	seenConnecting := false

	for _, sample := range samples {
		if sample.Tags["url"] == url {
			switch sample.Metric {
			case metrics.WSConnecting:
				seenConnecting = true
			case metrics.WSSessionDuration:
				seenSessionDuration = true
			case metrics.WSSessions:
				seenSessions = true
			}

			assert.Equal(t, strconv.Itoa(status), sample.Tags["status"])
			assert.Equal(t, subprotocol, sample.Tags["subproto"])
			assert.Equal(t, group, sample.Tags["group"])
		}
	}
	assert.True(t, seenConnecting, "url %s didn't emit Connecting", url)
	assert.True(t, seenSessions, "url %s didn't emit Sessions", url)
	assert.True(t, seenSessionDuration, "url %s didn't emit SessionDuration", url)
}

func assertMetricEmitted(t *testing.T, metric *stats.Metric, samples []stats.Sample, url string) {
	seenMetric := false

	for _, sample := range samples {
		if sample.Tags["url"] == url {
			if sample.Metric == metric {
				seenMetric = true
			}
		}
	}
	assert.True(t, seenMetric, "url %s didn't emit %s", url, metric.Name)
}

func TLSServerMock(t *testing.T) (string, *tls.Config, func()) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		conn, err := websocket.Upgrade(w, req, w.Header(), 1024, 1024)
		assert.NoError(t, err)

		if err := conn.Close(); err != nil {
			t.Logf("Close: %v", err)
			return
		}
	})

	srv := httptest.NewTLSServer(mux)

	return srv.URL, srv.TLS, srv.Close
}

func makeWsProto(s string) string {
	return "ws" + strings.TrimPrefix(s, "http")
}

func TestSession(t *testing.T) {
	root, err := lib.NewGroup("", nil)
	assert.NoError(t, err)

	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})
	dialer := netext.NewDialer(net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 60 * time.Second,
		DualStack: true,
	})
	state := &common.State{
		Group:  root,
		Dialer: dialer,
		Options: lib.Options{
			SystemTags: lib.GetTagSet("url", "proto", "status", "subproto"),
		},
	}

	ctx := context.Background()
	ctx = common.WithState(ctx, state)
	ctx = common.WithRuntime(ctx, rt)

	rt.Set("ws", common.Bind(rt, New(), &ctx))

	t.Run("connect_ws", func(t *testing.T) {
		_, err := common.RunString(rt, `
		let res = ws.connect("ws://demos.kaazing.com/echo", function(socket){
			socket.close()
		});
		if (res.status != 101) { throw new Error("connection failed with status: " + res.status); }
		`)
		assert.NoError(t, err)
	})
	assertSessionMetricsEmitted(t, state.Samples, "", "ws://demos.kaazing.com/echo", 101, "")

	t.Run("connect_wss", func(t *testing.T) {
		_, err := common.RunString(rt, `
		let res = ws.connect("wss://demos.kaazing.com/echo", function(socket){
			socket.close()
		});
		if (res.status != 101) { throw new Error("TLS connection failed with status: " + res.status); }
		`)
		assert.NoError(t, err)
	})
	assertSessionMetricsEmitted(t, state.Samples, "", "wss://demos.kaazing.com/echo", 101, "")

	t.Run("open", func(t *testing.T) {
		state.Samples = nil
		_, err := common.RunString(rt, `
		let opened = false;
		let res = ws.connect("ws://demos.kaazing.com/echo", function(socket){
			socket.on("open", function() {
				opened = true;
				socket.close()
			})
		});
		if (!opened) { throw new Error ("open event not fired"); }
		`)
		assert.NoError(t, err)
	})
	assertSessionMetricsEmitted(t, state.Samples, "", "ws://demos.kaazing.com/echo", 101, "")

	t.Run("send_receive", func(t *testing.T) {
		state.Samples = nil
		_, err := common.RunString(rt, `
		let res = ws.connect("ws://demos.kaazing.com/echo", function(socket){
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
		`)
		assert.NoError(t, err)
	})
	assertSessionMetricsEmitted(t, state.Samples, "", "ws://demos.kaazing.com/echo", 101, "")
	assertMetricEmitted(t, metrics.WSMessagesSent, state.Samples, "ws://demos.kaazing.com/echo")
	assertMetricEmitted(t, metrics.WSMessagesReceived, state.Samples, "ws://demos.kaazing.com/echo")

	t.Run("interval", func(t *testing.T) {
		state.Samples = nil
		_, err := common.RunString(rt, `
		let counter = 0;
		let res = ws.connect("ws://demos.kaazing.com/echo", function(socket){
			socket.setInterval(function () {
				counter += 1;
				if (counter > 2) { socket.close(); }
			}, 100);
		});
		if (counter < 3) {throw new Error ("setInterval should have been called at least 3 times, counter=" + counter);}
		`)
		assert.NoError(t, err)
	})
	assertSessionMetricsEmitted(t, state.Samples, "", "ws://demos.kaazing.com/echo", 101, "")

	t.Run("timeout", func(t *testing.T) {
		state.Samples = nil
		_, err := common.RunString(rt, `
		let start = new Date().getTime();
		let ellapsed = new Date().getTime() - start;
		let res = ws.connect("ws://demos.kaazing.com/echo", function(socket){
			socket.setTimeout(function () {
				ellapsed = new Date().getTime() - start;
				socket.close();
			}, 500);
		});
		if (ellapsed > 2000 || ellapsed < 500) {
			throw new Error ("setTimeout occurred after " + ellapsed + "ms, expected 500<T<2000");
		}
		`)
		assert.NoError(t, err)
	})
	assertSessionMetricsEmitted(t, state.Samples, "", "ws://demos.kaazing.com/echo", 101, "")

	t.Run("ping", func(t *testing.T) {
		state.Samples = nil
		_, err := common.RunString(rt, `
		let pongReceived = false;
		let res = ws.connect("ws://demos.kaazing.com/echo", function(socket){
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
		`)
		assert.NoError(t, err)
	})
	assertSessionMetricsEmitted(t, state.Samples, "", "ws://demos.kaazing.com/echo", 101, "")
	assertMetricEmitted(t, metrics.WSPing, state.Samples, "ws://demos.kaazing.com/echo")

	t.Run("multiple_handlers", func(t *testing.T) {
		state.Samples = nil
		_, err := common.RunString(rt, `
		let pongReceived = false;
		let otherPongReceived = false;

		let res = ws.connect("ws://demos.kaazing.com/echo", function(socket){
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
		`)
		assert.NoError(t, err)
	})
	assertSessionMetricsEmitted(t, state.Samples, "", "ws://demos.kaazing.com/echo", 101, "")
	assertMetricEmitted(t, metrics.WSPing, state.Samples, "ws://demos.kaazing.com/echo")

	t.Run("close", func(t *testing.T) {
		state.Samples = nil
		_, err := common.RunString(rt, `
		let closed = false;
		let res = ws.connect("ws://demos.kaazing.com/echo", function(socket){
			socket.on("open", function() {
							socket.close()
			})
			socket.on("close", function() {
							closed = true;
			})
		});
		if (!closed) { throw new Error ("close event not fired"); }
		`)
		assert.NoError(t, err)
	})
	assertSessionMetricsEmitted(t, state.Samples, "", "ws://demos.kaazing.com/echo", 101, "")
}

func TestErrors(t *testing.T) {
	root, err := lib.NewGroup("", nil)
	assert.NoError(t, err)

	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})
	dialer := netext.NewDialer(net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 60 * time.Second,
		DualStack: true,
	})
	state := &common.State{
		Group:  root,
		Dialer: dialer,
		Options: lib.Options{
			SystemTags: lib.GetTagSet(lib.DefaultSystemTagList...),
		},
	}

	ctx := context.Background()
	ctx = common.WithState(ctx, state)
	ctx = common.WithRuntime(ctx, rt)

	rt.Set("ws", common.Bind(rt, New(), &ctx))

	t.Run("invalid_url", func(t *testing.T) {
		state.Samples = nil
		_, err := common.RunString(rt, `
		let res = ws.connect("INVALID", function(socket){
			socket.on("open", function() {
				socket.close();
			});
		});
		`)
		assert.Error(t, err)
	})

	t.Run("send_after_close", func(t *testing.T) {
		state.Samples = nil
		_, err := common.RunString(rt, `
		let hasError = false;
		let res = ws.connect("ws://demos.kaazing.com/echo", function(socket){
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
		`)
		assert.NoError(t, err)
		assertSessionMetricsEmitted(t, state.Samples, "", "ws://demos.kaazing.com/echo", 101, "")
	})
}

func TestSystemTags(t *testing.T) {
	root, err := lib.NewGroup("", nil)
	assert.NoError(t, err)

	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})
	dialer := netext.NewDialer(net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 60 * time.Second,
		DualStack: true,
	})

	testedSystemTags := []string{"group", "status", "subproto", "url"}
	state := &common.State{
		Group:   root,
		Dialer:  dialer,
		Options: lib.Options{SystemTags: lib.GetTagSet(testedSystemTags...)},
	}

	ctx := context.Background()
	ctx = common.WithState(ctx, state)
	ctx = common.WithRuntime(ctx, rt)

	rt.Set("ws", common.Bind(rt, New(), &ctx))

	for _, expectedTag := range testedSystemTags {
		t.Run("only "+expectedTag, func(t *testing.T) {
			state.Options.SystemTags = map[string]bool{
				expectedTag: true,
			}
			state.Samples = nil
			_, err := common.RunString(rt, `
			let res = ws.connect("ws://demos.kaazing.com/echo", function(socket){
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
			`)
			assert.NoError(t, err)
			for _, sample := range state.Samples {
				for emittedTag := range sample.Tags {
					assert.Equal(t, expectedTag, emittedTag)
				}
			}
		})
	}
}

func TestTLSConfig(t *testing.T) {
	root, err := lib.NewGroup("", nil)
	assert.NoError(t, err)

	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})
	dialer := netext.NewDialer(net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 60 * time.Second,
		DualStack: true,
	})
	state := &common.State{
		Group:  root,
		Dialer: dialer,
		Options: lib.Options{
			SystemTags: lib.GetTagSet("url", "proto", "status", "subproto"),
		},
	}

	ctx := context.Background()
	ctx = common.WithState(ctx, state)
	ctx = common.WithRuntime(ctx, rt)

	rt.Set("ws", common.Bind(rt, New(), &ctx))

	baseURL, tlsConfig, teardown := TLSServerMock(t)
	defer teardown()

	url := makeWsProto(baseURL)

	t.Run("insecure skip verify", func(t *testing.T) {
		state.TLSConfig = &tls.Config{
			InsecureSkipVerify: true,
		}

		_, err := common.RunString(rt, fmt.Sprintf(`
		let res = ws.connect("%s", function(socket){
			socket.close()
		});
		if (res.status != 101) { throw new Error("TLS connection failed with status: " + res.status); }
		`, url))
		assert.NoError(t, err)
	})
	assertSessionMetricsEmitted(t, state.Samples, "", url, 101, "")

	t.Run("custom certificates", func(t *testing.T) {
		certs := x509.NewCertPool()
		for _, c := range tlsConfig.Certificates {
			roots, err := x509.ParseCertificates(c.Certificate[len(c.Certificate)-1])
			if err != nil {
				t.Fatalf("error parsing server's root cert: %v", err)
			}
			for _, root := range roots {
				certs.AddCert(root)
			}
		}

		state.TLSConfig = &tls.Config{
			RootCAs:            certs,
			InsecureSkipVerify: false,
		}

		_, err := common.RunString(rt, fmt.Sprintf(`
		let res = ws.connect("%s", function(socket){
			socket.close()
		});
		if (res.status != 101) { throw new Error("TLS connection failed with status: " + res.status); }
		`, url))
		assert.NoError(t, err)
	})
	assertSessionMetricsEmitted(t, state.Samples, "", url, 101, "")
}
