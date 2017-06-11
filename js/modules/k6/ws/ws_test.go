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
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/dop251/goja"
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
	seenMessagesReceived := false
	seenMessagesSent := false
	seenPing := false
	seenDataSent := false
	seenDataReceived := false

	for _, sample := range samples {
		if sample.Tags["url"] == url {
			switch sample.Metric {
			case metrics.WSConnecting:
				seenConnecting = true
			case metrics.WSMessagesReceived:
				seenMessagesReceived = true
			case metrics.WSMessagesSent:
				seenMessagesSent = true
			case metrics.WSPing:
				seenPing = true
			case metrics.WSSessionDuration:
				seenSessionDuration = true
			case metrics.WSSessions:
				seenSessions = true
			case metrics.DataReceived:
				seenDataReceived = true
			case metrics.DataSent:
				seenDataSent = true
			}

			assert.Equal(t, strconv.Itoa(status), sample.Tags["status"])
			assert.Equal(t, subprotocol, sample.Tags["subprotocol"])
			assert.Equal(t, group, sample.Tags["group"])
		}
	}
	assert.True(t, seenConnecting, "url %s didn't emit Connecting", url)
	assert.True(t, seenMessagesReceived, "url %s didn't emit MessagesReceived", url)
	assert.True(t, seenMessagesSent, "url %s didn't emit MessagesSent", url)
	assert.True(t, seenPing, "url %s didn't emit Ping", url)
	assert.True(t, seenSessions, "url %s didn't emit Sessions", url)
	assert.True(t, seenSessionDuration, "url %s didn't emit SessionDuration", url)
	assert.True(t, seenDataSent, "url %s didn't emit DataSent", url)
	assert.True(t, seenDataReceived, "url %s didn't emit DataReceived", url)
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
	state := &common.State{Group: root, Dialer: dialer}

	ctx := context.Background()
	ctx = common.WithState(ctx, state)
	ctx = common.WithRuntime(ctx, rt)

	rt.Set("ws", common.Bind(rt, &WS{}, &ctx))

	t.Run("connect_ws", func(t *testing.T) {
		_, err := common.RunString(rt, `
		let res = ws.connect("ws://echo.websocket.org", function(socket){
			socket.close()
		});
		if (res.status_code != 101) { throw new Error("connection failed with status: " + res.status); }
		`)
		assert.NoError(t, err)
	})
	assertSessionMetricsEmitted(t, state.Samples, "", "ws://echo.websocket.org", 101, "")

	t.Run("connect_wss", func(t *testing.T) {
		_, err := common.RunString(rt, `
		let res = ws.connect("wss://echo.websocket.org", function(socket){
			socket.close()
		});
		if (res.status_code != 101) { throw new Error("TLS connection failed with status: " + res.status); }
		`)
		assert.NoError(t, err)
	})
	assertSessionMetricsEmitted(t, state.Samples, "", "wss://echo.websocket.org", 101, "")

	t.Run("open", func(t *testing.T) {
		state.Samples = nil
		_, err := common.RunString(rt, `
		let opened = false;
		let res = ws.connect("ws://echo.websocket.org", function(socket){
			socket.on("open", function() {
				opened = true;
				socket.close()
			})
		});
		if (!opened) { throw new Error ("open event not fired"); }
		`)
		assert.NoError(t, err)
	})
	assertSessionMetricsEmitted(t, state.Samples, "", "ws://echo.websocket.org", 101, "")

	t.Run("send_receive", func(t *testing.T) {
		state.Samples = nil
		_, err := common.RunString(rt, `
		let res = ws.connect("ws://echo.websocket.org", function(socket){
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
	assertSessionMetricsEmitted(t, state.Samples, "", "ws://echo.websocket.org", 101, "")

	t.Run("interval", func(t *testing.T) {
		state.Samples = nil
		_, err := common.RunString(rt, `
		let counter = 0;
		let res = ws.connect("ws://echo.websocket.org", function(socket){
			socket.setInterval(function () {
				counter += 1;
				if (counter > 2) { socket.close(); }
			}, 100);
		});
		if (counter < 3) {throw new Error ("setInterval should have been called at least 3 times, counter=" + counter);}
		`)
		assert.NoError(t, err)
	})
	assertSessionMetricsEmitted(t, state.Samples, "", "ws://echo.websocket.org", 101, "")

	t.Run("timeout", func(t *testing.T) {
		state.Samples = nil
		_, err := common.RunString(rt, `
		let start = new Date().getTime();
		let ellapsed = new Date().getTime() - start;
		let res = ws.connect("ws://echo.websocket.org", function(socket){
			socket.setTimeout(function () {
				ellapsed = new Date().getTime() - start;
				socket.close();
			}, 500);
		});
		if (ellapsed > 2000 || ellapsed < 500) {
			throw new Error ("setTimeout occured after " + ellapsed + "ms, expected 500<T<2000");
		}
		`)
		assert.NoError(t, err)
	})
	assertSessionMetricsEmitted(t, state.Samples, "", "ws://echo.websocket.org", 101, "")

	t.Run("ping", func(t *testing.T) {
		state.Samples = nil
		_, err := common.RunString(rt, `
		let pongReceived = false;
		let res = ws.connect("ws://echo.websocket.org", function(socket){
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
	assertSessionMetricsEmitted(t, state.Samples, "", "ws://echo.websocket.org", 101, "")

	t.Run("multiple_handlers", func(t *testing.T) {
		state.Samples = nil
		_, err := common.RunString(rt, `
		let pongReceived = false;
		let otherPongReceived = false;

		let res = ws.connect("ws://echo.websocket.org", function(socket){
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
	assertSessionMetricsEmitted(t, state.Samples, "", "ws://echo.websocket.org", 101, "")
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
	state := &common.State{Group: root, Dialer: dialer}

	ctx := context.Background()
	ctx = common.WithState(ctx, state)
	ctx = common.WithRuntime(ctx, rt)

	rt.Set("ws", common.Bind(rt, &WS{}, &ctx))

	t.Run("invalid_url", func(t *testing.T) {
		state.Samples = nil
		_, err := common.RunString(rt, `
		let hasError = false;
		let res = ws.connect("INVALID", function(socket){
			socket.on("open", function() {
				socket.close();
			});

			socket.on("error", function(errorEvent) {
				hasError = true;
			});
		});
		if (!hasError) {
			throw new Error ("no error emitted for invalid url");
		}
		`)
		assert.NoError(t, err)
	})

	t.Run("send_after_close", func(t *testing.T) {
		state.Samples = nil
		_, err := common.RunString(rt, `
		let hasError = false;
		let res = ws.connect("ws://echo.websocket.org", function(socket){
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
		assertSessionMetricsEmitted(t, state.Samples, "", "ws://echo.websocket.org", 101, "")
	})
}
