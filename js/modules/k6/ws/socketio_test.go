package ws

import (
	"context"
	"testing"

	"github.com/dop251/goja"
	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/lib/testutils/httpmultibin"
	"github.com/loadimpact/k6/stats"
	"github.com/stretchr/testify/assert"
)

// func assertSocketIOSessionMetricsEmitted(t *testing.T, sampleContainers []stats.SampleContainer, subprotocol, url string, status int, group string) {
// 	seenSessions := false
// 	seenSessionDuration := false
// 	seenConnecting := false

// 	for _, sampleContainer := range sampleContainers {
// 		for _, sample := range sampleContainer.GetSamples() {
// 			tags := sample.Tags.CloneTags()
// 			if tags["url"] == url {
// 				switch sample.Metric {
// 				case metrics.WSConnecting:
// 					seenConnecting = true
// 				case metrics.WSSessionDuration:
// 					seenSessionDuration = true
// 				case metrics.WSSessions:
// 					seenSessions = true
// 				}

// 				assert.Equal(t, strconv.Itoa(status), tags["status"])
// 				assert.Equal(t, subprotocol, tags["subproto"])
// 				assert.Equal(t, group, tags["group"])
// 			}
// 		}
// 	}
// 	assert.True(t, seenConnecting, "url %s didn't emit Connecting", url)
// 	assert.True(t, seenSessions, "url %s didn't emit Sessions", url)
// 	assert.True(t, seenSessionDuration, "url %s didn't emit SessionDuration", url)
// }

// func assertSocketIOMetricEmitted(t *testing.T, metric *stats.Metric, sampleContainers []stats.SampleContainer, url string) {
// 	seenMetric := false

// 	for _, sampleContainer := range sampleContainers {
// 		for _, sample := range sampleContainer.GetSamples() {
// 			surl, ok := sample.Tags.Get("url")
// 			assert.True(t, ok)
// 			if surl == url {
// 				if sample.Metric == metric {
// 					seenMetric = true
// 				}
// 			}
// 		}
// 	}
// 	assert.True(t, seenMetric, "url %s didn't emit %s", url, metric.Name)
// }

func TestSocketIOSession(t *testing.T) {
	t.Parallel()
	sr, rt, samples, tb := setUpTest(t)
	defer tb.Cleanup()
	testConnectWSS(t, rt, sr, samples)
	testConnectWS(t, rt, sr, samples)
	testOpenEventHandler(t, rt, sr, samples)
	testSendReciveStringData(t, rt, sr, samples)
	testSendReciveJSONData(t, rt, sr, samples)
	testSendReciveEmptyData(t, rt, sr, samples)
	testIntervalHandler(t, rt, sr, samples)
	testTimeoutHandler(t, rt, sr, samples)
	testPingHandler(t, rt, sr, samples)
	testMultipleHandler(t, rt, sr, samples)
	testClientCloserHandler(t, rt, sr, samples)
	testClientCloserWithoutSendCloseRequestHandler(t, rt, sr, samples)
	testServerClosePrematurely(t, rt, sr, samples)
}

func TestSocketIOErrors(t *testing.T) {

	t.Parallel()
	sr, rt, samples, tb := setUpTest(t)
	defer tb.Cleanup()
	testThrowErrorWithInvalidURL(t, rt, sr, samples)
	testThrowErrorMsgWithInvalidURL(t, rt, sr, samples)
	testThrowErrorWithMissingChannelName(t, rt, sr, samples)
	testThrowErrorInInterval(t, rt, sr, samples)
	testThrowErrorInSetup(t, rt, sr, samples)
	testThrowErrorSendAfterClose(t, rt, sr, samples)
	testThrowErrorOnClose(t, rt, sr, samples)
}

// func TestSocketIOSystemTags(t *testing.T) {
// 	tb := httpmultibin.NewHTTPMultiBin(t)
// 	defer tb.Cleanup()

// 	sr := tb.Replacer.Replace

// 	root, err := lib.NewGroup("", nil)
// 	assert.NoError(t, err)

// 	rt := goja.New()
// 	rt.SetFieldNameMapper(common.FieldNameMapper{})

// 	//TODO: test for actual tag values after removing the dependency on the
// 	// external service demos.kaazing.com (https://github.com/loadimpact/k6/issues/537)
// 	testedSystemTags := []string{"group", "status", "subproto", "url", "ip"}

// 	samples := make(chan stats.SampleContainer, 1000)
// 	state := &lib.State{
// 		Group:     root,
// 		Dialer:    tb.Dialer,
// 		Options:   lib.Options{SystemTags: stats.ToSystemTagSet(testedSystemTags)},
// 		Samples:   samples,
// 		TLSConfig: tb.TLSClientConfig,
// 	}

// 	ctx := context.Background()
// 	ctx = lib.WithState(ctx, state)
// 	ctx = common.WithRuntime(ctx, rt)

// 	rt.Set("ws", common.Bind(rt, NewSocketIO(), &ctx))

// 	for _, expectedTag := range testedSystemTags {
// 		expectedTag := expectedTag
// 		t.Run("only "+expectedTag, func(t *testing.T) {
// 			state.Options.SystemTags = stats.ToSystemTagSet([]string{expectedTag})
// 			_, err := common.RunString(rt, sr(`
// 			let res = ws.connect("WSBIN_URL/ws-echo", function(socket){
// 				socket.on("open", function() {
// 					socket.send("test")
// 				})
// 				socket.on("message", function (data){
// 					if (data!=="test") {
// 						throw new Error ("echo'd data doesn't match our message!");
// 					}
// 					socket.close()
// 				});
// 			});
// 			`))
// 			assert.NoError(t, err)

// 			for _, sampleContainer := range stats.GetBufferedSamples(samples) {
// 				for _, sample := range sampleContainer.GetSamples() {
// 					for emittedTag := range sample.Tags.CloneTags() {
// 						assert.Equal(t, expectedTag, emittedTag)
// 					}
// 				}
// 			}
// 		})
// 	}
// }

// func TestSocketIOTLSConfig(t *testing.T) {
// 	root, err := lib.NewGroup("", nil)
// 	assert.NoError(t, err)

// 	tb := httpmultibin.NewHTTPMultiBin(t)
// 	defer tb.Cleanup()

// 	sr := tb.Replacer.Replace

// 	rt := goja.New()
// 	rt.SetFieldNameMapper(common.FieldNameMapper{})
// 	samples := make(chan stats.SampleContainer, 1000)
// 	state := &lib.State{
// 		Group:  root,
// 		Dialer: tb.Dialer,
// 		Options: lib.Options{
// 			SystemTags: stats.NewSystemTagSet(
// 				stats.TagURL,
// 				stats.TagProto,
// 				stats.TagStatus,
// 				stats.TagSubproto,
// 				stats.TagIP,
// 			),
// 		},
// 		Samples: samples,
// 	}

// 	ctx := context.Background()
// 	ctx = lib.WithState(ctx, state)
// 	ctx = common.WithRuntime(ctx, rt)

// 	rt.Set("ws", common.Bind(rt, NewSocketIO(), &ctx))

// 	t.Run("insecure skip verify", func(t *testing.T) {
// 		state.TLSConfig = &tls.Config{
// 			InsecureSkipVerify: true,
// 		}

// 		_, err := common.RunString(rt, sr(`
// 		let res = ws.connect("WSSBIN_URL/ws-close", function(socket){
// 			socket.close()
// 		});
// 		if (res.status != 101) { throw new Error("TLS connection failed with status: " + res.status); }
// 		`))
// 		assert.NoError(t, err)
// 	})
// 	assertSessionMetricsEmitted(t, stats.GetBufferedSamples(samples), "", sr("WSSBIN_URL/ws-close"), 101, "")

// 	t.Run("custom certificates", func(t *testing.T) {
// 		state.TLSConfig = tb.TLSClientConfig

// 		_, err := common.RunString(rt, sr(`
// 			let res = ws.connect("WSSBIN_URL/ws-close", function(socket){
// 				socket.close()
// 			});
// 			if (res.status != 101) {
// 				throw new Error("TLS connection failed with status: " + res.status);
// 			}
// 		`))
// 		assert.NoError(t, err)
// 	})
// 	assertSessionMetricsEmitted(t, stats.GetBufferedSamples(samples), "", sr("WSSBIN_URL/ws-close"), 101, "")
// }

func setUpTest(t *testing.T) (func(string) string, *goja.Runtime, chan stats.SampleContainer, *httpmultibin.HTTPMultiBin) {
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
		Samples:   samples,
		TLSConfig: tb.TLSClientConfig,
	}

	ctx := context.Background()
	ctx = lib.WithState(ctx, state)
	ctx = common.WithRuntime(ctx, rt)

	rt.Set("ws", common.Bind(rt, NewSocketIO(), &ctx))
	return sr, rt, samples, tb
}

func testConnectWS(tt *testing.T, rt *goja.Runtime, sr func(string) string, samples chan stats.SampleContainer) {
	tt.Run("connect_ws", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
		let res = ws.connect("WSBIN_URL/wsio-echo", function(socket){
			socket.close()
		});
		if (res.status != 101) { throw new Error("connection failed with status: " + res.status); }
		`))
		assert.NoError(t, err)
	})
	assertSessionMetricsEmitted(tt, stats.GetBufferedSamples(samples), "", sr("WSBIN_URL/wsio-echo"), 101, "")
}

func testConnectWSS(tt *testing.T, rt *goja.Runtime, sr func(string) string, samples chan stats.SampleContainer) {
	tt.Run("connect_wss", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
		let res = ws.connect("WSSBIN_URL/wsio-ssl", function(socket){
			socket.close()
		});
		if (res.status != 101) { throw new Error("TLS connection failed with status: " + res.status); }
		`))
		assert.NoError(t, err)
	})
	assertSessionMetricsEmitted(tt, stats.GetBufferedSamples(samples), "", sr("WSSBIN_URL/wsio-ssl"), 101, "")
}

func testOpenEventHandler(tt *testing.T, rt *goja.Runtime, sr func(string) string, samples chan stats.SampleContainer) {
	tt.Run("open", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
		let opened = false;
		let res = ws.connect("WSBIN_URL/wsio-open", function(socket){
			socket.on("open", function() {
				opened = true;
				socket.close()
			})
		});
		if (!opened) { throw new Error ("open event not fired"); }
		`))
		assert.NoError(t, err)
	})
	assertSessionMetricsEmitted(tt, stats.GetBufferedSamples(samples), "", sr("WSBIN_URL/wsio-open"), 101, "")
}

func testSendReciveStringData(tt *testing.T, rt *goja.Runtime, sr func(string) string, samples chan stats.SampleContainer) {
	tt.Run("send_receive_string", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
		let res = ws.connect("WSBIN_URL/wsio-echo-data", function(socket){
			socket.on("open", function (){
				socket.send("stringChannel", "test" )
			});
			socket.on("stringChannel", function (data){
				if (data!=="test") {
					throw new Error ("echo data doesn't match our message in testChannel! " + data);
				}
			});
			socket.on("message", function (data){
					throw new Error ("echo data doesn't match our channel event! " + data);
			});
		});
		`))
		assert.NoError(t, err)
	})
	samplesBuf := stats.GetBufferedSamples(samples)
	assertSessionMetricsEmitted(tt, samplesBuf, "", sr("WSBIN_URL/wsio-echo-data"), 101, "")
	assertMetricEmitted(tt, metrics.WSMessagesSent, samplesBuf, sr("WSBIN_URL/wsio-echo-data"))
	assertMetricEmitted(tt, metrics.WSMessagesReceived, samplesBuf, sr("WSBIN_URL/wsio-echo-data"))
}

func testSendReciveJSONData(tt *testing.T, rt *goja.Runtime, sr func(string) string, samples chan stats.SampleContainer) {
	tt.Run("send_receive_json", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
		let res = ws.connect("WSBIN_URL/wsio-echo-data", function(socket){
			socket.on("open", function (){
				socket.send("jsonChannel", {sample: "sample"} )
			});
			socket.on("jsonChannel", function (data){
				const dataObj = JSON.parse(data)
				if (dataObj.sample !== "sample"){
					throw new Error ("echo data doesn't match our message in jsonChannel! " + data);
				}
			});
			socket.on("message", function (data){
					throw new Error ("echo data doesn't match our channel event! " + data);
			});
		});
		`))
		assert.NoError(t, err)
	})
	samplesBuf := stats.GetBufferedSamples(samples)
	assertSessionMetricsEmitted(tt, samplesBuf, "", sr("WSBIN_URL/wsio-echo-data"), 101, "")
	assertMetricEmitted(tt, metrics.WSMessagesSent, samplesBuf, sr("WSBIN_URL/wsio-echo-data"))
	assertMetricEmitted(tt, metrics.WSMessagesReceived, samplesBuf, sr("WSBIN_URL/wsio-echo-data"))
}

func testSendReciveEmptyData(tt *testing.T, rt *goja.Runtime, sr func(string) string, samples chan stats.SampleContainer) {
	tt.Run("send_receive_empty", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
		var reciveData = false;
		let res = ws.connect("WSBIN_URL/wsio-echo-empty-data", function(socket){
			socket.on("emptyMessage", function (data){
				reciveData = true;
			});
			socket.on("message", function (data){
					throw new Error ("echo data doesn't match our channel event! " + data);
			});
		});
		if (!reciveData) throw new Error ("Empty data doesn't recieve!");
		`))
		assert.NoError(t, err)
	})
	samplesBuf := stats.GetBufferedSamples(samples)
	assertSessionMetricsEmitted(tt, samplesBuf, "", sr("WSBIN_URL/wsio-echo-empty-data"), 101, "")
}

func testIntervalHandler(tt *testing.T, rt *goja.Runtime, sr func(string) string, samples chan stats.SampleContainer) {
	tt.Run("interval", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
		let counter = 0;
		let res = ws.connect("WSBIN_URL/wsio-echo", function(socket){
			socket.setInterval(function () {
				counter += 1;
				if (counter > 2) { socket.close(); }
			}, 100);
		});
		if (counter < 3) {throw new Error ("setInterval should have been called at least 3 times, counter=" + counter);}
		`))
		assert.NoError(t, err)
	})
	assertSessionMetricsEmitted(tt, stats.GetBufferedSamples(samples), "", sr("WSBIN_URL/wsio-echo"), 101, "")
}

func testTimeoutHandler(tt *testing.T, rt *goja.Runtime, sr func(string) string, samples chan stats.SampleContainer) {
	tt.Run("timeout", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
		let start = new Date().getTime();
		let ellapsed = new Date().getTime() - start;
		let res = ws.connect("WSBIN_URL/wsio-echo", function(socket){
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
	assertSessionMetricsEmitted(tt, stats.GetBufferedSamples(samples), "", sr("WSBIN_URL/wsio-echo"), 101, "")
}

func testPingHandler(tt *testing.T, rt *goja.Runtime, sr func(string) string, samples chan stats.SampleContainer) {
	tt.Run("ping", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
		let pongReceived = false;
		let res = ws.connect("WSBIN_URL/wsio-ping", function(socket){
			socket.on("open", function(data) {
				socket.ping();
			});
			socket.on("pong", function() {
				pongReceived = true;
			});
			socket.setTimeout(function (){socket.close();}, 300);
		});
		if (!pongReceived) {
			throw new Error ("sent ping but didn't get pong back");
		}
		`))
		assert.NoError(t, err)
	})
	samplesBuf := stats.GetBufferedSamples(samples)
	assertSessionMetricsEmitted(tt, samplesBuf, "", sr("WSBIN_URL/wsio-ping"), 101, "")
	assertMetricEmitted(tt, metrics.WSPing, samplesBuf, sr("WSBIN_URL/wsio-ping"))
}

func testMultipleHandler(tt *testing.T, rt *goja.Runtime, sr func(string) string, samples chan stats.SampleContainer) {
	tt.Run("multiple_handlers", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
		let pongReceived = false;
		let otherPongReceived = false;

		let res = ws.connect("WSBIN_URL/wsio-echo", function(socket){
			socket.on("open", function(data) {
				socket.ping();
			});
			socket.on("pong", function() {
				pongReceived = true;
			});
			socket.on("pong", function() {
				otherPongReceived = true;
			});
			socket.setTimeout(function (){socket.close();}, 300);
		});
		if (!pongReceived || !otherPongReceived) {
			throw new Error ("sent ping but didn't get pong back");
		}
		`))
		assert.NoError(t, err)
	})

	samplesBuf := stats.GetBufferedSamples(samples)
	assertSessionMetricsEmitted(tt, samplesBuf, "", sr("WSBIN_URL/wsio-echo"), 101, "")
	assertMetricEmitted(tt, metrics.WSPing, samplesBuf, sr("WSBIN_URL/wsio-echo"))
}

func testClientCloserHandler(tt *testing.T, rt *goja.Runtime, sr func(string) string, samples chan stats.SampleContainer) {
	tt.Run("client_close", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
		let closed = false;
		let res = ws.connect("WSBIN_URL/wsio-echo", function(socket){
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
	assertSessionMetricsEmitted(tt, stats.GetBufferedSamples(samples), "", sr("WSBIN_URL/wsio-echo"), 101, "")
}

func testClientCloserWithoutSendCloseRequestHandler(tt *testing.T, rt *goja.Runtime, sr func(string) string, samples chan stats.SampleContainer) {
	tt.Run("server_close_ok", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
			let closed = false;
			let res = ws.connect("WSBIN_URL/wsio-echo", function(socket){
				socket.on("open", function() {
					socket.send("message","test");
				})
				socket.on("close", function() {
					closed = true;
				})
			});
			if (!closed) { throw new Error ("close event not fired"); }
			`))
		assert.NoError(t, err)
	})
}

func testServerClosePrematurely(tt *testing.T, rt *goja.Runtime, sr func(string) string, samples chan stats.SampleContainer) {
	tt.Run("server_close_invalid", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
			let closed = false;
			let res = ws.connect("WSBIN_URL/wsio-close-invalid", function(socket){
				socket.on("open", function() {
					socket.send("message","test");
				})
				socket.on("close", function() {
					closed = true;
				})
			});
			if (!closed) { throw new Error ("close event not fired"); }
			`))
		assert.NoError(t, err)
	})
}

func testThrowErrorWithInvalidURL(tt *testing.T, rt *goja.Runtime, sr func(string) string, samples chan stats.SampleContainer) {
	tt.Run("invalid_url", func(t *testing.T) {
		_, err := common.RunString(rt, `
		let res = ws.connect("INVALID", function(socket){
			socket.on("open", function() {
				socket.close();
			});
		});
		`)
		assert.Error(t, err)
		assert.Equal(t, "GoError: malformed ws or wss URL with url: INVALID", err.Error())
	})
}

func testThrowErrorMsgWithInvalidURL(tt *testing.T, rt *goja.Runtime, sr func(string) string, samples chan stats.SampleContainer) {
	tt.Run("invalid_url_with_send_message", func(t *testing.T) {
		// Attempting to send a message to a non-existent socket shouldn't panic
		_, err := common.RunString(rt, `
		let res = ws.connect("INVALID", function(socket){
			socket.send("sample channel","new message");
		});
		`)
		assert.Error(t, err)
		assert.Equal(t, "GoError: malformed ws or wss URL with url: INVALID", err.Error())
	})
}

func testThrowErrorWithMissingChannelName(tt *testing.T, rt *goja.Runtime, sr func(string) string, samples chan stats.SampleContainer) {
	tt.Run("send_receive", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
		let res = ws.connect("WSBIN_URL/wsio-echo-data", function(socket){
			socket.on("open", function (){
				socket.send("test")
			});
		});
		`))
		assert.Error(t, err)
		assert.Equal(t, "GoError: invalid number of arguments to ws.send. Method is required 2 params ( channelName, message )", err.Error())
	})
}

func testThrowErrorInSetup(tt *testing.T, rt *goja.Runtime, sr func(string) string, samples chan stats.SampleContainer) {
	tt.Run("error_in_setup", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
		let res = ws.connect("WSBIN_URL/wsio-echo-data", function(socket){
			throw new Error("error in setup socket callback func");
		});
		`))
		assert.Error(t, err)
		assert.Equal(t, "Error: error in setup socket callback func at <eval>:3:8(4)", err.Error())
	})
}

func testThrowErrorSendAfterClose(tt *testing.T, rt *goja.Runtime, sr func(string) string, samples chan stats.SampleContainer) {
	tt.Run("send_after_close", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
		let hasError = false;
		let res = ws.connect("WSBIN_URL/wsio-echo-invalid", function(socket){
			socket.on("open", function() {
				socket.close();
				socket.send("message", "test");
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
		assertSessionMetricsEmitted(t, stats.GetBufferedSamples(samples), "", sr("WSBIN_URL/wsio-echo-invalid"), 101, "")
	})
}

func testThrowErrorInInterval(tt *testing.T, rt *goja.Runtime, sr func(string) string, samples chan stats.SampleContainer) {
	tt.Run("interval_error", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
		let counter = 0;
		let hasError = false;
		let res = ws.connect("WSBIN_URL/wsio-echo", function(socket){
			socket.setInterval(function () {
				counter += 1;
				if (counter > 2) { 
					throw new Error ("Throw error in interval function");
				}
			}, 100);
		});
		`))
		assert.Error(t, err)
	})
}

func testThrowErrorOnClose(tt *testing.T, rt *goja.Runtime, sr func(string) string, samples chan stats.SampleContainer) {
	tt.Run("error_on_close", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
		var closed = false;
		let res = ws.connect("WSBIN_URL/wsio-echo", function(socket){
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
		assertSessionMetricsEmitted(t, stats.GetBufferedSamples(samples), "", sr("WSBIN_URL/wsio-echo"), 101, "")
	})
}
