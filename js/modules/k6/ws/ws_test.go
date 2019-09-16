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
	"strconv"
	"testing"

	"github.com/dop251/goja"
	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/lib/testutils/httpmultibin"
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

func TestSession(t *testing.T) {
	//TODO: split and paralelize tests
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)
	defer tb.Cleanup()
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
			SystemTags: stats.ToSystemTagSet([]string{
				stats.TagURL.String(),
				stats.TagProto.String(),
				stats.TagStatus.String(),
				stats.TagSubProto.String(),
			}),
		},
		Samples:   samples,
		TLSConfig: tb.TLSClientConfig,
	}

	ctx := context.Background()
	ctx = lib.WithState(ctx, state)
	ctx = common.WithRuntime(ctx, rt)

	rt.Set("ws", common.Bind(rt, New(), &ctx))

	t.Run("connect_ws", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
		let res = ws.connect("WSBIN_URL/ws-echo", function(socket){
			socket.close()
		});
		if (res.status != 101) { throw new Error("connection failed with status: " + res.status); }
		`))
		assert.NoError(t, err)
	})
	assertSessionMetricsEmitted(t, stats.GetBufferedSamples(samples), "", sr("WSBIN_URL/ws-echo"), 101, "")

	t.Run("connect_wss", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
		let res = ws.connect("WSSBIN_URL/ws-echo", function(socket){
			socket.close()
		});
		if (res.status != 101) { throw new Error("TLS connection failed with status: " + res.status); }
		`))
		assert.NoError(t, err)
	})
	assertSessionMetricsEmitted(t, stats.GetBufferedSamples(samples), "", sr("WSSBIN_URL/ws-echo"), 101, "")

	t.Run("open", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
		let opened = false;
		let res = ws.connect("WSBIN_URL/ws-echo", function(socket){
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
		_, err := common.RunString(rt, sr(`
		let res = ws.connect("WSBIN_URL/ws-echo", function(socket){
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
	assertMetricEmitted(t, metrics.WSMessagesSent, samplesBuf, sr("WSBIN_URL/ws-echo"))
	assertMetricEmitted(t, metrics.WSMessagesReceived, samplesBuf, sr("WSBIN_URL/ws-echo"))

	t.Run("interval", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
		let counter = 0;
		let res = ws.connect("WSBIN_URL/ws-echo", function(socket){
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

	t.Run("timeout", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
		let start = new Date().getTime();
		let ellapsed = new Date().getTime() - start;
		let res = ws.connect("WSBIN_URL/ws-echo", function(socket){
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
	assertSessionMetricsEmitted(t, stats.GetBufferedSamples(samples), "", sr("WSBIN_URL/ws-echo"), 101, "")

	t.Run("ping", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
		let pongReceived = false;
		let res = ws.connect("WSBIN_URL/ws-echo", function(socket){
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
	assertMetricEmitted(t, metrics.WSPing, samplesBuf, sr("WSBIN_URL/ws-echo"))

	t.Run("multiple_handlers", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
		let pongReceived = false;
		let otherPongReceived = false;

		let res = ws.connect("WSBIN_URL/ws-echo", function(socket){
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
	assertMetricEmitted(t, metrics.WSPing, samplesBuf, sr("WSBIN_URL/ws-echo"))

	t.Run("close", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
		let closed = false;
		let res = ws.connect("WSBIN_URL/ws-echo", function(socket){
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
}

func TestErrors(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)
	defer tb.Cleanup()
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
			SystemTags: stats.ToSystemTagSet(stats.DefaultSystemTagList),
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

	t.Run("invalid_url_message_panic", func(t *testing.T) {
		// Attempting to send a message to a non-existent socket shouldn't panic
		_, err := common.RunString(rt, `
		let res = ws.connect("INVALID", function(socket){
			socket.send("new message");
		});
		`)
		assert.Error(t, err)
	})

	t.Run("error_in_setup", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
		let res = ws.connect("WSBIN_URL/ws-echo", function(socket){
			throw new Error("error in setup");
		});
		`))
		assert.Error(t, err)
	})

	t.Run("send_after_close", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
		let hasError = false;
		let res = ws.connect("WSBIN_URL/ws-echo", function(socket){
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
		assertSessionMetricsEmitted(t, stats.GetBufferedSamples(samples), "", sr("WSBIN_URL/ws-echo"), 101, "")
	})

	t.Run("error on close", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
		var closed = false;
		let res = ws.connect("WSBIN_URL/ws-close", function(socket){
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
	defer tb.Cleanup()

	sr := tb.Replacer.Replace

	root, err := lib.NewGroup("", nil)
	assert.NoError(t, err)

	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})

	//TODO: test for actual tag values after removing the dependency on the
	// external service demos.kaazing.com (https://github.com/loadimpact/k6/issues/537)
	testedSystemTags := []string{"group", "status", "subproto", "url", "ip"}

	samples := make(chan stats.SampleContainer, 1000)
	state := &lib.State{
		Group:     root,
		Dialer:    tb.Dialer,
		Options:   lib.Options{SystemTags: stats.ToSystemTagSet(testedSystemTags)},
		Samples:   samples,
		TLSConfig: tb.TLSClientConfig,
	}

	ctx := context.Background()
	ctx = lib.WithState(ctx, state)
	ctx = common.WithRuntime(ctx, rt)

	rt.Set("ws", common.Bind(rt, New(), &ctx))

	for _, expectedTag := range testedSystemTags {
		expectedTag := expectedTag
		t.Run("only "+expectedTag, func(t *testing.T) {
			state.Options.SystemTags = stats.ToSystemTagSet([]string{expectedTag})
			_, err := common.RunString(rt, sr(`
			let res = ws.connect("WSBIN_URL/ws-echo", function(socket){
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
	defer tb.Cleanup()

	sr := tb.Replacer.Replace

	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})
	samples := make(chan stats.SampleContainer, 1000)
	state := &lib.State{
		Group:  root,
		Dialer: tb.Dialer,
		Options: lib.Options{
			SystemTags: stats.ToSystemTagSet([]string{
				stats.TagURL.String(),
				stats.TagProto.String(),
				stats.TagStatus.String(),
				stats.TagSubProto.String(),
				stats.TagIP.String(),
			}),
		},
		Samples: samples,
	}

	ctx := context.Background()
	ctx = lib.WithState(ctx, state)
	ctx = common.WithRuntime(ctx, rt)

	rt.Set("ws", common.Bind(rt, New(), &ctx))

	t.Run("insecure skip verify", func(t *testing.T) {
		state.TLSConfig = &tls.Config{
			InsecureSkipVerify: true,
		}

		_, err := common.RunString(rt, sr(`
		let res = ws.connect("WSSBIN_URL/ws-close", function(socket){
			socket.close()
		});
		if (res.status != 101) { throw new Error("TLS connection failed with status: " + res.status); }
		`))
		assert.NoError(t, err)
	})
	assertSessionMetricsEmitted(t, stats.GetBufferedSamples(samples), "", sr("WSSBIN_URL/ws-close"), 101, "")

	t.Run("custom certificates", func(t *testing.T) {
		state.TLSConfig = tb.TLSClientConfig

		_, err := common.RunString(rt, sr(`
			let res = ws.connect("WSSBIN_URL/ws-close", function(socket){
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
