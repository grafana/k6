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
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/lib/netext"
	"github.com/loadimpact/k6/lib/testutils"
	"github.com/loadimpact/k6/stats"
	"github.com/stretchr/testify/assert"
)

func assertSessionMetricsEmitted(t *testing.T, sampleContainers []stats.SampleContainer, subprotocol, url string, status int, group string) {
	seenSessions := false
	seenSessionDuration := false
	seenConnecting := false

	for _, sampleContainer := range sampleContainers {
		for _, sample := range sampleContainer.GetSamples() {
			tags := sample.Tags.CloneTags()
			if tags["url"] == url {
				switch sample.Metric {
				case metrics.WSConnecting:
					seenConnecting = true
				case metrics.WSSessionDuration:
					seenSessionDuration = true
				case metrics.WSSessions:
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

func assertMetricEmitted(t *testing.T, metric *stats.Metric, sampleContainers []stats.SampleContainer, url string) {
	seenMetric := false

	for _, sampleContainer := range sampleContainers {
		for _, sample := range sampleContainer.GetSamples() {
			surl, ok := sample.Tags.Get("url")
			assert.True(t, ok)
			if surl == url {
				if sample.Metric == metric {
					seenMetric = true
				}
			}
		}
	}
	assert.True(t, seenMetric, "url %s didn't emit %s", url, metric.Name)
}

func makeWsProto(s string) string {
	return "ws" + strings.TrimPrefix(s, "http")
}

func TestSession(t *testing.T) {
	//TODO: split and paralelize tests

	root, err := lib.NewGroup("", nil)
	assert.NoError(t, err)

	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})
	dialer := netext.NewDialer(net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 60 * time.Second,
		DualStack: true,
	})
	samples := make(chan stats.SampleContainer, 1000)
	state := &lib.State{
		Group:  root,
		Dialer: dialer,
		Options: lib.Options{
			SystemTags: lib.GetTagSet("url", "proto", "status", "subproto"),
		},
		Samples: samples,
	}

	ctx := context.Background()
	ctx = lib.WithState(ctx, state)
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
	assertSessionMetricsEmitted(t, stats.GetBufferedSamples(samples), "", "ws://demos.kaazing.com/echo", 101, "")

	t.Run("connect_wss", func(t *testing.T) {
		_, err := common.RunString(rt, `
		let res = ws.connect("wss://demos.kaazing.com/echo", function(socket){
			socket.close()
		});
		if (res.status != 101) { throw new Error("TLS connection failed with status: " + res.status); }
		`)
		assert.NoError(t, err)
	})
	assertSessionMetricsEmitted(t, stats.GetBufferedSamples(samples), "", "wss://demos.kaazing.com/echo", 101, "")

	t.Run("open", func(t *testing.T) {
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
	assertSessionMetricsEmitted(t, stats.GetBufferedSamples(samples), "", "ws://demos.kaazing.com/echo", 101, "")

	t.Run("send_receive", func(t *testing.T) {
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

	samplesBuf := stats.GetBufferedSamples(samples)
	assertSessionMetricsEmitted(t, samplesBuf, "", "ws://demos.kaazing.com/echo", 101, "")
	assertMetricEmitted(t, metrics.WSMessagesSent, samplesBuf, "ws://demos.kaazing.com/echo")
	assertMetricEmitted(t, metrics.WSMessagesReceived, samplesBuf, "ws://demos.kaazing.com/echo")

	t.Run("interval", func(t *testing.T) {
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
	assertSessionMetricsEmitted(t, stats.GetBufferedSamples(samples), "", "ws://demos.kaazing.com/echo", 101, "")

	t.Run("timeout", func(t *testing.T) {
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
	assertSessionMetricsEmitted(t, stats.GetBufferedSamples(samples), "", "ws://demos.kaazing.com/echo", 101, "")

	t.Run("ping", func(t *testing.T) {
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

	samplesBuf = stats.GetBufferedSamples(samples)
	assertSessionMetricsEmitted(t, samplesBuf, "", "ws://demos.kaazing.com/echo", 101, "")
	assertMetricEmitted(t, metrics.WSPing, samplesBuf, "ws://demos.kaazing.com/echo")

	t.Run("multiple_handlers", func(t *testing.T) {
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

	samplesBuf = stats.GetBufferedSamples(samples)
	assertSessionMetricsEmitted(t, samplesBuf, "", "ws://demos.kaazing.com/echo", 101, "")
	assertMetricEmitted(t, metrics.WSPing, samplesBuf, "ws://demos.kaazing.com/echo")

	t.Run("close", func(t *testing.T) {
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
	assertSessionMetricsEmitted(t, stats.GetBufferedSamples(samples), "", "ws://demos.kaazing.com/echo", 101, "")
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
	samples := make(chan stats.SampleContainer, 1000)
	state := &lib.State{
		Group:  root,
		Dialer: dialer,
		Options: lib.Options{
			SystemTags: lib.GetTagSet(lib.DefaultSystemTagList...),
		},
		Samples: samples,
	}

	ctx := context.Background()
	ctx = lib.WithState(ctx, state)
	ctx = common.WithRuntime(ctx, rt)

	rt.Set("ws", common.Bind(rt, New(), &ctx))

	t.Run("invalid_url", func(t *testing.T) {
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
		assertSessionMetricsEmitted(t, stats.GetBufferedSamples(samples), "", "ws://demos.kaazing.com/echo", 101, "")
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

	//TODO: test for actual tag values after removing the dependency on the
	// external service demos.kaazing.com (https://github.com/loadimpact/k6/issues/537)
	testedSystemTags := []string{"group", "status", "subproto", "url", "ip"}

	samples := make(chan stats.SampleContainer, 1000)
	state := &lib.State{
		Group:   root,
		Dialer:  dialer,
		Options: lib.Options{SystemTags: lib.GetTagSet(testedSystemTags...)},
		Samples: samples,
	}

	ctx := context.Background()
	ctx = lib.WithState(ctx, state)
	ctx = common.WithRuntime(ctx, rt)

	rt.Set("ws", common.Bind(rt, New(), &ctx))

	for _, expectedTag := range testedSystemTags {
		t.Run("only "+expectedTag, func(t *testing.T) {
			state.Options.SystemTags = map[string]bool{
				expectedTag: true,
			}
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

	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})
	dialer := netext.NewDialer(net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 60 * time.Second,
		DualStack: true,
	})
	samples := make(chan stats.SampleContainer, 1000)
	state := &lib.State{
		Group:  root,
		Dialer: dialer,
		Options: lib.Options{
			SystemTags: lib.GetTagSet("url", "proto", "status", "subproto", "ip"),
		},
		Samples: samples,
	}

	ctx := context.Background()
	ctx = lib.WithState(ctx, state)
	ctx = common.WithRuntime(ctx, rt)

	rt.Set("ws", common.Bind(rt, New(), &ctx))

	tb := testutils.NewHTTPMultiBin(t)
	defer tb.Cleanup()

	url := makeWsProto(tb.ServerHTTPS.URL) + "/ws-close"

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
	assertSessionMetricsEmitted(t, stats.GetBufferedSamples(samples), "", url, 101, "")

	t.Run("custom certificates", func(t *testing.T) {
		state.TLSConfig = tb.TLSClientConfig

		_, err := common.RunString(rt, fmt.Sprintf(`
		let res = ws.connect("%s", function(socket){
			socket.close()
		});
		if (res.status != 101) { throw new Error("TLS connection failed with status: " + res.status); }
		`, url))
		assert.NoError(t, err)
	})
	assertSessionMetricsEmitted(t, stats.GetBufferedSamples(samples), "", url, 101, "")
}
