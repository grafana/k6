package ws

import (
	"context"
	"crypto/tls"
	"testing"

	"github.com/dop251/goja"
	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/lib/testutils/httpmultibin"
	"github.com/loadimpact/k6/stats"
	"github.com/stretchr/testify/assert"
)

func TestSocketIOSession(t *testing.T) {
	t.Parallel()
	sr, rt, samples, tb, _ := setUpTest(t)
	defer tb.Cleanup()

	testConnectWSS(t, rt, sr, samples)
	testConnectWS(t, rt, sr, samples)
	testOpenEventHandler(t, rt, sr, samples)
	testSendReciveStringData(t, rt, sr, samples)
	testSendReciveJSONData(t, rt, sr, samples)
	testSendReceiveEmptyData(t, rt, sr, samples)
	testIntervalHandler(t, rt, sr, samples)
	testTimeoutHandler(t, rt, sr, samples)
	testPingHandler(t, rt, sr, samples)
	testMultipleHandler(t, rt, sr, samples)
	testClientCloserHandler(t, rt, sr, samples)
	testClientCloserWithoutSendCloseRequestHandler(t, rt, sr)
	testServerClosePrematurely(t, rt, sr)
}

func TestSocketIOErrors(t *testing.T) {
	t.Parallel()
	sr, rt, samples, tb, _ := setUpTest(t)
	defer tb.Cleanup()

	testThrowErrorWithInvalidURL(t, rt)
	testThrowErrorMsgWithInvalidURL(t, rt)
	testThrowErrorWithMissingChannelName(t, rt, sr)
	testThrowErrorInInterval(t, rt, sr)
	testThrowErrorInSetup(t, rt, sr)
	testThrowErrorSendAfterClose(t, rt, sr, samples)
	testThrowErrorOnClose(t, rt, sr, samples)
}

func TestSocketIOTLSConfig(t *testing.T) {
	t.Parallel()
	sr, rt, samples, tb, state := setUpTest(t)
	defer tb.Cleanup()

	testInsecureSkipVerify(t, rt, sr, samples, state)
	testCustomCertificates(t, rt, sr, samples, state, tb)
}

func TestSocketIOHeaders(t *testing.T) {
	t.Parallel()
	sr, rt, _, tb, _ := setUpTest(t)
	defer tb.Cleanup()

	testSetCookies(t, rt, sr)
	testSetUndefinedCookies(t, rt, sr)
	testSetHeaders(t, rt, sr)
	testSetUndefinedHeaders(t, rt, sr)
}

func setUpTest(t *testing.T) (func(string) string, *goja.Runtime, chan stats.SampleContainer, *httpmultibin.HTTPMultiBin, *lib.State) {
	tb := httpmultibin.NewHTTPMultiBin(t)

	sr := tb.Replacer.Replace

	root, err := lib.NewGroup("", nil)
	assert.NoError(t, err)

	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})
	const chanLen = 1000
	samples := make(chan stats.SampleContainer, chanLen)
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
	return sr, rt, samples, tb, state
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

func testSendReceiveEmptyData(tt *testing.T, rt *goja.Runtime, sr func(string) string, samples chan stats.SampleContainer) {
	tt.Run("send_receive_empty", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
		var receiveData = false;
		let res = ws.connect("WSBIN_URL/wsio-echo-empty-data", function(socket){
			socket.on("emptyMessage", function (data){
				receiveData = true;
			});
			socket.on("message", function (data){
					throw new Error ("echo data doesn't match our channel event! " + data);
			});
		});
		if (!receiveData) throw new Error ("Empty data doesn't receive!");
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

func testClientCloserWithoutSendCloseRequestHandler(tt *testing.T, rt *goja.Runtime, sr func(string) string) {
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

func testServerClosePrematurely(tt *testing.T, rt *goja.Runtime, sr func(string) string) {
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

func testThrowErrorWithInvalidURL(tt *testing.T, rt *goja.Runtime) {
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

func testThrowErrorMsgWithInvalidURL(tt *testing.T, rt *goja.Runtime) {
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

func testThrowErrorWithMissingChannelName(tt *testing.T, rt *goja.Runtime, sr func(string) string) {
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

func testThrowErrorInSetup(tt *testing.T, rt *goja.Runtime, sr func(string) string) {
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

func testThrowErrorInInterval(tt *testing.T, rt *goja.Runtime, sr func(string) string) {
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

func testCustomCertificates(tt *testing.T, rt *goja.Runtime, sr func(string) string, samples chan stats.SampleContainer, state *lib.State, tb *httpmultibin.HTTPMultiBin) {
	tt.Run("custom_certificates", func(t *testing.T) {
		state.TLSConfig = tb.TLSClientConfig

		_, err := common.RunString(rt, sr(`
				let res = ws.connect("WSSBIN_URL/wsio-open", function(socket){
				socket.close()
			});
			if (res.status != 101) {
				throw new Error("TLS connection failed with status: " + res.status);
			}
		`))
		assert.NoError(t, err)
	})
	assertSessionMetricsEmitted(tt, stats.GetBufferedSamples(samples), "", sr("WSSBIN_URL/wsio-open"), 101, "")
}

func testInsecureSkipVerify(tt *testing.T, rt *goja.Runtime, sr func(string) string, samples chan stats.SampleContainer, state *lib.State) {
	tt.Run("insecure skip verify", func(t *testing.T) {
		state.TLSConfig = &tls.Config{
			InsecureSkipVerify: true, //nolint:gosec
		}
		_, err := common.RunString(rt, sr(`
		let res = ws.connect("WSSBIN_URL/wsio-open", function(socket){
			socket.close()
		});
		if (res.status != 101) { throw new Error("TLS connection failed with status: " + res.status); }
		`))
		assert.NoError(t, err)
	})
	assertSessionMetricsEmitted(tt, stats.GetBufferedSamples(samples), "", sr("WSSBIN_URL/wsio-open"), 101, "")
}

func testSetCookies(tt *testing.T, rt *goja.Runtime, sr func(string) string) {
	tt.Run("set_cookies", func(t *testing.T) {
		value, err := common.RunString(rt, sr(`
		function check(){
		let res = ws.connect("WSBIN_URL/wsio-echo", {cookies:{sampleA: { value: "a", replace: true },sampleB: { value: "b", replace: false },sampleC: "c", sampleD: undefined}},function(socket){
				socket.close()
		});
		return res
		}
		check()
		`))
		cookieValue := value.ToObject(rt).Get("headers").ToObject(rt).Get("Cookie").ToObject(rt).String()
		assert.NoError(t, err)
		assert.Contains(t, cookieValue, "sampleA=a")
		assert.Contains(t, cookieValue, "sampleB=b")
		assert.Contains(t, cookieValue, "sampleC=c")
	})
}

func testSetUndefinedCookies(tt *testing.T, rt *goja.Runtime, sr func(string) string) {
	tt.Run("set_undefined_cookies", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
		function check(){
		let res = ws.connect("WSBIN_URL/wsio-echo", {cookies:undefined},function(socket){
				socket.close()
		});
		return res
		}
		check()
		`))
		assert.NoError(t, err)
	})
}

func testSetHeaders(tt *testing.T, rt *goja.Runtime, sr func(string) string) {
	tt.Run("set_headers", func(t *testing.T) {
		value, err := common.RunString(rt, sr(`
		function check(){
		let res = ws.connect("WSBIN_URL/wsio-echo", {headers:{key1: ["1", "2", "3"],key2: ["4", "5", "6"],key3: undefined}},function(socket){
				socket.close()
		});
		return res
		}
		check()
		`))
		headerKey1 := value.ToObject(rt).Get("headers").ToObject(rt).Get("Key1").ToObject(rt).String()
		headerKey2 := value.ToObject(rt).Get("headers").ToObject(rt).Get("Key2").ToObject(rt).String()
		assert.NoError(t, err)
		assert.Equal(t, "1,2,3", headerKey1)
		assert.Equal(t, "4,5,6", headerKey2)
	})
}

func testSetUndefinedHeaders(tt *testing.T, rt *goja.Runtime, sr func(string) string) {
	tt.Run("set_undefined_headers", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
		function check(){
		let res = ws.connect("WSBIN_URL/wsio-echo", {headers:undefined},function(socket){
				socket.close()
		});
		return res
		}
		check()
		`))
		assert.NoError(t, err)
	})
}
