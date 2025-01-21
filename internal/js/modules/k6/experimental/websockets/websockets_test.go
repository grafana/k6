package websockets

import (
	"context"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"strconv"
	"strings"
	"sync"
	"testing"

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

// copied from k6/ws
func assertSessionMetricsEmitted(
	t *testing.T,
	sampleContainers []metrics.SampleContainer,
	subprotocol,
	url string,
	status int, //nolint:unparam // TODO: check why it always same in tests
	group string, //nolint:unparam // TODO: check why it always same in tests
) {
	t.Helper()
	seenSessions := false
	seenSessionDuration := false
	seenConnecting := false

	require.NotEmpty(t, sampleContainers)
	for _, sampleContainer := range sampleContainers {
		require.NotEmpty(t, sampleContainer.GetSamples())
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

// also copied from k6/ws
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
	tb      *httpmultibin.HTTPMultiBin
	runtime *modulestest.Runtime
	samples chan metrics.SampleContainer
	t       testing.TB

	callRecorder *callRecorder
	errors       chan error

	module *WebSocketsAPI
}

// callRecorder a helper type that records all calls
type callRecorder struct {
	sync.Mutex
	calls []string
}

// Call records a call
func (r *callRecorder) Call(text string) {
	r.Lock()
	defer r.Unlock()

	r.calls = append(r.calls, text)
}

// Len just returns the length of the calls
func (r *callRecorder) Len() int {
	r.Lock()
	defer r.Unlock()

	return len(r.calls)
}

// Len just returns the length of the calls
func (r *callRecorder) Recorded() []string {
	r.Lock()
	defer r.Unlock()

	result := []string{}
	result = append(result, r.calls...)

	return result
}

func newTestState(t testing.TB) testState {
	runtime := modulestest.NewRuntime(t)
	tb := httpmultibin.NewHTTPMultiBin(t)

	samples := make(chan metrics.SampleContainer, 1000)
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
		},
		Samples:        samples,
		TLSConfig:      tb.TLSClientConfig,
		BuiltinMetrics: runtime.BuiltinMetrics,
		Tags:           lib.NewVUStateTags(runtime.VU.InitEnvField.Registry.RootTagSet()),
	}

	recorder := &callRecorder{
		calls: make([]string, 0),
	}

	m := new(RootModule).NewModuleInstance(runtime.VU)
	require.NoError(t, runtime.VU.RuntimeField.Set("WebSocket", m.Exports().Named["WebSocket"]))
	require.NoError(t, runtime.VU.RuntimeField.Set("Blob", m.Exports().Named["Blob"]))
	require.NoError(t, runtime.VU.RuntimeField.Set("call", recorder.Call))

	runtime.MoveToVUContext(state)
	return testState{
		runtime:      runtime,
		tb:           tb,
		samples:      samples,
		callRecorder: recorder,
		errors:       make(chan error, 50),
		t:            t,
		module:       m.(*WebSocketsAPI),
	}
}

func (ts *testState) addHandler(uri string, upgrader *websocket.Upgrader, message *testMessage) {
	ts.tb.Mux.HandleFunc(uri, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// when upgrader not passed we should use default one
		if upgrader == nil {
			upgrader = &websocket.Upgrader{}
		}

		conn, err := upgrader.Upgrade(w, req, w.Header())
		if err != nil {
			ts.errors <- fmt.Errorf("%s cannot upgrade request: %w", uri, err)
			return
		}

		defer func() {
			err = conn.Close()
			if err != nil {
				ts.t.Logf("error while closing connection in %s: %v", uri, err)
				return
			}
		}()

		if message == nil {
			return
		}

		if err = conn.WriteMessage(message.kind, message.data); err != nil {
			ts.errors <- fmt.Errorf("%s cannot write message: %w", uri, err)
			return
		}
	}))
}

type testMessage struct {
	kind int
	data []byte
}

func TestBasic(t *testing.T) {
	t.Parallel()
	ts := newTestState(t)
	sr := ts.tb.Replacer.Replace
	_, err := ts.runtime.RunOnEventLoop(sr(`
		var ws = new WebSocket("WSBIN_URL/ws-echo")
		ws.addEventListener("open", () => {
			ws.send("something")
			ws.close()
		})
	`))
	require.NoError(t, err)
	samples := metrics.GetBufferedSamples(ts.samples)
	assertSessionMetricsEmitted(t, samples, "", sr("WSBIN_URL/ws-echo"), http.StatusSwitchingProtocols, "")
}

func TestBasicSendBlob(t *testing.T) {
	t.Parallel()
	ts := newTestState(t)
	sr := ts.tb.Replacer.Replace
	_, err := ts.runtime.RunOnEventLoop(sr(`
		var ws = new WebSocket("WSBIN_URL/ws-echo")
		ws.addEventListener("open", () => {
			ws.send(new Blob(["something"]))
			ws.close()
		})
	`))
	require.NoError(t, err)
	samples := metrics.GetBufferedSamples(ts.samples)
	assertSessionMetricsEmitted(t, samples, "", sr("WSBIN_URL/ws-echo"), http.StatusSwitchingProtocols, "")
}

func TestAddUndefinedHandler(t *testing.T) {
	t.Parallel()
	ts := newTestState(t)
	sr := ts.tb.Replacer.Replace
	_, err := ts.runtime.RunOnEventLoop(sr(`
		var ws = new WebSocket("WSBIN_URL/ws-echo")
		ws.addEventListener("open", () => {
			ws.close()
		})
		ws.addEventListener("open", undefined)
	`))
	require.ErrorContains(t, err, "handler for event type \"open\" isn't a callable function")
}

func TestBasicWithOn(t *testing.T) {
	t.Parallel()

	ts := newTestState(t)
	sr := ts.tb.Replacer.Replace
	_, err := ts.runtime.RunOnEventLoop(sr(`
		var ws = new WebSocket("WSBIN_URL/ws-echo")
		ws.onopen = () => {
			ws.send("something")
			ws.close()
		}
	`))
	require.NoError(t, err)
	samples := metrics.GetBufferedSamples(ts.samples)
	assertSessionMetricsEmitted(t, samples, "", sr("WSBIN_URL/ws-echo"), http.StatusSwitchingProtocols, "")
}

func TestReadyState(t *testing.T) {
	t.Parallel()
	ts := newTestState(t)
	_, err := ts.runtime.RunOnEventLoop(ts.tb.Replacer.Replace(`
		var ws = new WebSocket("WSBIN_URL/ws-echo")
		ws.addEventListener("open", () => {
			if (ws.readyState != 1){
				throw new Error("Expected ready state 1 got "+ ws.readyState)
			}
			ws.addEventListener("close", () => {
				if (ws.readyState != 3){
					throw new Error("Expected ready state 3 got "+ ws.readyState)
				}

			})
			ws.send("something")
			ws.close()
		})
		if (ws.readyState != 0){
			throw new Error("Expected ready state 0 got "+ ws.readyState)
		}
	`))
	require.NoError(t, err)
}

func TestBinaryState(t *testing.T) {
	t.Parallel()
	ts := newTestState(t)
	logger, hook := testutils.NewLoggerWithHook(t, logrus.WarnLevel)
	ts.runtime.VU.StateField.Logger = logger
	_, err := ts.runtime.RunOnEventLoop(ts.tb.Replacer.Replace(`
		var ws = new WebSocket("WSBIN_URL/ws-echo-invalid")
		ws.addEventListener("open", () => {
			ws.send(new Uint8Array([164,41]).buffer)
			if (ws.bufferedAmount != 2) {
				throw "Expected 2 bufferedAmount got "+ ws.bufferedAmount
			}
			ws.send("k6")
			if (ws.bufferedAmount != 4) {
				throw "Expected 4 bufferedAmount got "+ ws.bufferedAmount
			}
			ws.onmessage = (e) => {
				if (ws.bufferedAmount != 0 && ws.bufferedAmount != 2) { // it is possible one or both were flushed
					throw "Expected 0 or 2 bufferedAmount, but got "+ ws.bufferedAmount
				}
				ws.close()
				call(JSON.stringify(e))
			}
		})

		if (ws.binaryType != "blob") {
			throw new Error("Wrong binaryType value, expected to be blob got "+ ws.binaryType)
		}

		var thrown = false;
		try {
			ws.binaryType = "something"
		} catch(e) {
			thrown = true
		}
		if (!thrown) {
			throw new Error("Expects ws.binaryType to be writable only with valid values")
		}
	`))
	require.NoError(t, err)
	logs := hook.Drain()
	require.Len(t, logs, 0)
}

func TestBinaryType_Default(t *testing.T) {
	t.Parallel()
	ts := newTestState(t)
	logger, hook := testutils.NewLoggerWithHook(t, logrus.WarnLevel)
	ts.runtime.VU.StateField.Logger = logger
	_, err := ts.runtime.RunOnEventLoop(ts.tb.Replacer.Replace(`
		var ws = new WebSocket("WSBIN_URL/ws-echo-invalid")
		ws.addEventListener("open", () => {
			const sent = new Uint8Array([164,41]).buffer
			ws.send(sent)
			ws.onmessage = async (e) => {
				if (!(e.data instanceof Blob)) {
					throw new Error("Wrong event.data type; expected: Blob, got: "+ typeof e.data)
				}
				const received = await e.data.arrayBuffer();

				if (sent.byteLength !== received.byteLength) {
					throw new Error("The data received " + received.byteLength +" isn't equal to the data sent "+ sent.byteLength)
				}

				ws.close()
			}
		})
	`))
	require.NoError(t, err)
	logs := hook.Drain()
	require.Len(t, logs, 0)
}

func TestBinaryType_Blob(t *testing.T) {
	t.Parallel()
	ts := newTestState(t)
	logger, hook := testutils.NewLoggerWithHook(t, logrus.WarnLevel)
	ts.runtime.VU.StateField.Logger = logger
	_, err := ts.runtime.RunOnEventLoop(ts.tb.Replacer.Replace(`
		var ws = new WebSocket("WSBIN_URL/ws-echo")
		ws.binaryType = "blob"
		ws.addEventListener("open", () => {
			const sent = new Uint8Array([164,41]).buffer
			ws.send(sent)
			ws.onmessage = (e) => {
				if (!(e.data instanceof Blob)) {
					throw new Error("Wrong event.data type; expected: Blob, got: "+ typeof e.data)
				}

				e.data.arrayBuffer().then((ab) => {
					if (sent.byteLength !== ab.byteLength) {
						throw new Error("The data received isn't equal to the data sent")
					}
				})

				ws.close()
			}
		})
	`))
	require.NoError(t, err)
	logs := hook.Drain()
	require.Len(t, logs, 0)
}

func TestBinaryType_ArrayBuffer(t *testing.T) {
	t.Parallel()
	ts := newTestState(t)
	logger, hook := testutils.NewLoggerWithHook(t, logrus.WarnLevel)
	ts.runtime.VU.StateField.Logger = logger
	_, err := ts.runtime.RunOnEventLoop(ts.tb.Replacer.Replace(`
		var ws = new WebSocket("WSBIN_URL/ws-echo")
		ws.binaryType = "arraybuffer"
		ws.addEventListener("open", () => {
			const sent = new Uint8Array([164,41]).buffer
			ws.send(sent)
			ws.onmessage = (e) => {
				if (!(e.data instanceof ArrayBuffer)) {
					throw new Error("Wrong event.data type; expected: ArrayBuffer, got: "+ typeof e.data)
				}

				if (sent.byteLength !== e.data.byteLength) {
					throw new Error("The data received isn't equal to the data sent")
				}

				ws.close()
			}
		})
	`))
	require.NoError(t, err)
	logs := hook.Drain()
	require.Len(t, logs, 0)
}

func TestExceptionDontPanic(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		script, expectedError string
	}{
		"open": {
			script: `
		var ws = new WebSocket("WSBIN_URL/ws/echo")
		ws.addEventListener("open", () => {
			oops
		})`,
			expectedError: "oops is not defined at <eval>:4:4",
		},
		"onopen": {
			script: `
		var ws = new WebSocket("WSBIN_URL/ws/echo")
		ws.onopen = () => {
			oops
		}`,
			expectedError: "oops is not defined at <eval>:4:4",
		},
		"error": {
			script: `
		var ws = new WebSocket("WSBIN_URL/badurl")
		ws.addEventListener("error", () =>{
			inerroridf
		})
		`,
			expectedError: "inerroridf is not defined at <eval>:4:4",
		},
		"onerror": {
			script: `
		var ws = new WebSocket("WSBIN_URL/badurl")
		ws.onerror = () => {
			inerroridf
		}
		`,
			expectedError: "inerroridf is not defined at <eval>:4:4",
		},
		"close": {
			script: `
		var ws = new WebSocket("WSBIN_URL/ws/echo")
		ws.addEventListener("open", () => {
				ws.close()
		})
		ws.addEventListener("close", ()=>{
			incloseidf
		})`,
			expectedError: "incloseidf is not defined at <eval>:7:4",
		},
		"onclose": {
			script: `
		var ws = new WebSocket("WSBIN_URL/ws/echo")
		ws.onopen = () => {
				ws.close()
		}
		ws.onclose = () =>{
			incloseidf
		}`,
			expectedError: "incloseidf is not defined at <eval>:7:4",
		},
		"message": {
			script: `
		var ws = new WebSocket("WSBIN_URL/ws/echo")
		ws.addEventListener("open", () => {
				ws.send("something")
		})
		ws.addEventListener("message", ()=>{
			inmessageidf
		})`,
			expectedError: "inmessageidf is not defined at <eval>:7:4",
		},
		"onmessage": {
			script: `
		var ws = new WebSocket("WSBIN_URL/ws/echo")
		ws.onopen = () => {
				ws.send("something")
		}
		ws.onmessage = () =>{
			inmessageidf
		}`,
			expectedError: "inmessageidf is not defined at <eval>:7:4",
		},
	}
	for name, testcase := range cases {
		testcase := testcase
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ts := newTestState(t)
			// This is here as the on in k6 echos and closes, which races to whether we will get the message or not. And that seems like the correct thing to happen either way.
			ts.tb.Mux.HandleFunc("/ws/echo", func(w http.ResponseWriter, req *http.Request) {
				conn, err := (&websocket.Upgrader{}).Upgrade(w, req, w.Header())
				if err != nil {
					return
				}
				defer func() {
					_ = conn.Close()
				}()
				for {
					msgt, msg, err := conn.ReadMessage()
					if err != nil {
						return
					}
					err = conn.WriteMessage(msgt, msg)
					if err != nil {
						return
					}
				}
			})

			sr := ts.tb.Replacer.Replace
			_, err := ts.runtime.RunOnEventLoop(sr(testcase.script))
			require.Error(t, err)
			require.ErrorContains(t, err, testcase.expectedError)
		})
	}
}

func TestTwoTalking(t *testing.T) {
	t.Parallel()
	ts := newTestState(t)
	sr := ts.tb.Replacer.Replace

	ch1 := make(chan message)
	ch2 := make(chan message)

	ts.tb.Mux.HandleFunc("/ws/couple/", func(w http.ResponseWriter, req *http.Request) {
		path := strings.TrimPrefix(req.URL.Path, "/ws/couple/")
		var wch chan message
		var rch chan message

		switch path {
		case "1":
			wch = ch1
			rch = ch2
		case "2":
			wch = ch2
			rch = ch1
		default:
			w.WriteHeader(http.StatusTeapot)
		}

		conn, err := (&websocket.Upgrader{}).Upgrade(w, req, w.Header())
		if err != nil {
			return
		}
		defer func() {
			_ = conn.Close()
		}()

		go func() {
			defer close(wch)
			for {
				msgT, msg, err := conn.ReadMessage()
				if err != nil {
					return
				}
				wch <- message{
					data:  msg,
					mtype: msgT,
				}
			}
		}()
		for msg := range rch {
			err := conn.WriteMessage(msg.mtype, msg.data)
			if err != nil {
				return
			}
		}
	})

	_, err := ts.runtime.RunOnEventLoop(sr(`
		var count = 0;
		var ws1 = new WebSocket("WSBIN_URL/ws/couple/1");
		ws1.addEventListener("open", () => {
			ws1.send("I am 1");
		})
		ws1.addEventListener("message", (e)=>{
			if (e.data != "I am 2") {
				throw "oops";
			}
			count++;
			if (count == 2) {
				ws1.close();
			}
		})
		var ws2 = new WebSocket("WSBIN_URL/ws/couple/2");
		ws2.addEventListener("open", () => {
			ws2.send("I am 2");
		})
		ws2.addEventListener("message", (e)=>{
			if (e.data != "I am 1") {
				throw "oops";
			}
			count++;
			if (count == 2) {
				ws2.close();
			}
		})
	`))
	require.NoError(t, err)
	samples := metrics.GetBufferedSamples(ts.samples)
	assertSessionMetricsEmitted(t, samples, "", sr("WSBIN_URL/ws/couple/1"), http.StatusSwitchingProtocols, "")
	assertSessionMetricsEmitted(t, samples, "", sr("WSBIN_URL/ws/couple/2"), http.StatusSwitchingProtocols, "")
}

func TestTwoTalkingUsingOn(t *testing.T) {
	t.Parallel()
	ts := newTestState(t)
	sr := ts.tb.Replacer.Replace

	ch1 := make(chan message)
	ch2 := make(chan message)

	ts.tb.Mux.HandleFunc("/ws/couple/", func(w http.ResponseWriter, req *http.Request) {
		path := strings.TrimPrefix(req.URL.Path, "/ws/couple/")
		var wch chan message
		var rch chan message

		switch path {
		case "1":
			wch = ch1
			rch = ch2
		case "2":
			wch = ch2
			rch = ch1
		default:
			w.WriteHeader(http.StatusTeapot)
		}

		conn, err := (&websocket.Upgrader{}).Upgrade(w, req, w.Header())
		if err != nil {
			return
		}
		defer func() {
			_ = conn.Close()
		}()

		go func() {
			defer close(wch)
			for {
				msgT, msg, err := conn.ReadMessage()
				if err != nil {
					return
				}
				wch <- message{
					data:  msg,
					mtype: msgT,
				}
			}
		}()
		for msg := range rch {
			err := conn.WriteMessage(msg.mtype, msg.data)
			if err != nil {
				return
			}
		}
	})

	_, err := ts.runtime.RunOnEventLoop(sr(`
		var count = 0;
		var ws1 = new WebSocket("WSBIN_URL/ws/couple/1");
		ws1.onopen = () => {
			ws1.send("I am 1");
		}

		ws1.onmessage = (e) => {
			if (e.data != "I am 2") {
				throw "oops";
			}
			count++;
			if (count == 2) {
				ws1.close();
			}
		}

		var ws2 = new WebSocket("WSBIN_URL/ws/couple/2");
		ws2.onopen = () => {
			ws2.send("I am 2");
		}
		ws2.onmessage = (e) => {
			if (e.data != "I am 1") {
				throw "oops";
			}
			count++;
			if (count == 2) {
				ws2.close();
			}
		}
	`))
	require.NoError(t, err)
	samples := metrics.GetBufferedSamples(ts.samples)
	assertSessionMetricsEmitted(t, samples, "", sr("WSBIN_URL/ws/couple/1"), http.StatusSwitchingProtocols, "")
	assertSessionMetricsEmitted(t, samples, "", sr("WSBIN_URL/ws/couple/2"), http.StatusSwitchingProtocols, "")
}

func TestSubProtocols(t *testing.T) {
	t.Parallel()
	ts := newTestState(t)
	sr := ts.tb.Replacer.Replace

	ts.tb.Mux.HandleFunc("/ws/protocols", func(w http.ResponseWriter, req *http.Request) {
		conn, err := (&websocket.Upgrader{Subprotocols: []string{"unsupported", "supported"}}).Upgrade(w, req, w.Header())
		if conn.Subprotocol() != "supported" {
			_ = conn.WriteMessage(websocket.TextMessage, []byte(`bad subprotocol on server `+conn.Subprotocol()))
			return
		}
		ch := make(chan message)
		if err != nil {
			return
		}
		defer func() {
			_ = conn.Close()
		}()

		go func() {
			defer close(ch)
			for {
				msgT, msg, err := conn.ReadMessage()
				if err != nil {
					return
				}
				ch <- message{
					data:  msg,
					mtype: msgT,
				}
			}
		}()
		for msg := range ch {
			err := conn.WriteMessage(msg.mtype, msg.data)
			if err != nil {
				return
			}
		}
	})

	_, err := ts.runtime.RunOnEventLoop(sr(`
		const ws = new WebSocket("WSBIN_URL/ws/protocols", ["one", "supported"]);
		ws.onopen = () => {
			if (ws.protocol != "supported") {
				throw "bad protocol " + ws.protocol;
			}
			ws.send("hello");
		}

		ws.onmessage = (e) => {
			if (e.data != "hello") {
				throw "oops";
			}
			ws.close();
		}
		ws.onerror = (e) => { throw e.error; ws.close();}
	`))
	require.NoError(t, err)
	samples := metrics.GetBufferedSamples(ts.samples)
	assertSessionMetricsEmitted(t, samples, "supported", sr("WSBIN_URL/ws/protocols"), http.StatusSwitchingProtocols, "")
}

func TestDialError(t *testing.T) {
	t.Parallel()
	ts := newTestState(t)
	sr := ts.tb.Replacer.Replace

	// without listeners
	_, err := ts.runtime.RunOnEventLoop(sr(`
		var ws = new WebSocket("ws://127.0.0.2");
	`))
	require.NoError(t, err)

	_, err = ts.runtime.RunOnEventLoop(sr(`
		var ws = new WebSocket("ws://127.0.0.2");
		ws.addEventListener("error", (e) =>{
			ws.close();
			throw new Error("The provided url is an invalid endpoint")
		})
	`))
	assert.Error(t, err)
}

func TestOnError(t *testing.T) {
	t.Parallel()
	ts := newTestState(t)
	sr := ts.tb.Replacer.Replace

	_, err := ts.runtime.RunOnEventLoop(sr(`
		var ws = new WebSocket("ws://127.0.0.2");
		ws.onerror = (e) => {
			ws.close();
			throw new Error("lorem ipsum")
		}
	`))
	assert.Error(t, err)
	assert.Equal(t, "Error: lorem ipsum at <eval>:5:10(7)", err.Error())
}

func TestOnClose(t *testing.T) {
	t.Parallel()
	ts := newTestState(t)
	sr := ts.tb.Replacer.Replace

	_, err := ts.runtime.RunOnEventLoop(sr(`
		var ws = new WebSocket("WSBIN_URL/ws/echo")
		ws.onopen = () => {
			ws.close()
		}
		ws.onclose = () =>{
			call("from close")
		}
	`))
	assert.NoError(t, err)
	assert.Equal(t, []string{"from close"}, ts.callRecorder.Recorded())
}

func TestMixingOnAndAddHandlers(t *testing.T) {
	t.Parallel()
	ts := newTestState(t)
	sr := ts.tb.Replacer.Replace

	_, err := ts.runtime.RunOnEventLoop(sr(`
		var ws = new WebSocket("WSBIN_URL/ws/echo")
		ws.onopen = () => {
			ws.close()
		}
		ws.addEventListener("close", () => {
			call("from addEventListener")
		})
		ws.onclose = () =>{
			call("from onclose")
		}
	`))
	assert.NoError(t, err)
	assert.Equal(t, 2, ts.callRecorder.Len())
	assert.Contains(t, ts.callRecorder.Recorded(), "from addEventListener")
	assert.Contains(t, ts.callRecorder.Recorded(), "from onclose")
}

func TestOncloseRedefineListener(t *testing.T) {
	t.Parallel()

	ts := newTestState(t)
	sr := ts.tb.Replacer.Replace

	_, err := ts.runtime.RunOnEventLoop(sr(`
		var ws = new WebSocket("WSBIN_URL/ws/echo")
		ws.onopen = () => {
			ws.close()
		}
		ws.onclose = () =>{
			call("from onclose")
		}
		ws.onclose = () =>{
			call("from onclose 2")
		}
	`))
	assert.NoError(t, err)
	assert.Equal(t, []string{"from onclose 2"}, ts.callRecorder.Recorded())
}

func TestOncloseRedefineWithNull(t *testing.T) {
	t.Parallel()

	ts := newTestState(t)
	sr := ts.tb.Replacer.Replace

	_, err := ts.runtime.RunOnEventLoop(sr(`
		var ws = new WebSocket("WSBIN_URL/ws/echo")
		ws.onopen = () => {
			ws.close()
		}
		ws.onclose = () =>{
			call("from onclose")
		}
		ws.onclose = null
	`))
	assert.NoError(t, err)
	assert.Equal(t, 0, ts.callRecorder.Len())
}

func TestOncloseDefineWithInvalidValue(t *testing.T) {
	t.Parallel()

	ts := newTestState(t)
	sr := ts.tb.Replacer.Replace

	_, err := ts.runtime.RunOnEventLoop(sr(`
		var ws = new WebSocket("WSBIN_URL/ws/echo")
		ws.onclose = 1
	`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "a value for 'onclose' should be callable")
}

func TestCustomHeaders(t *testing.T) {
	t.Parallel()
	ts := newTestState(t)
	sr := ts.tb.Replacer.Replace

	mu := &sync.Mutex{}
	collected := make(http.Header)

	ts.tb.Mux.HandleFunc("/ws-echo-someheader", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		responseHeaders := w.Header().Clone()
		conn, err := (&websocket.Upgrader{}).Upgrade(w, req, responseHeaders)
		if err != nil {
			ts.errors <- fmt.Errorf("/ws-echo-someheader cannot upgrade request: %w", err)
			return
		}

		mu.Lock()
		collected = req.Header.Clone()
		mu.Unlock()

		err = conn.Close()
		if err != nil {
			t.Logf("error while closing connection in /ws-echo-someheader: %v", err)
		}
	}))

	_, err := ts.runtime.RunOnEventLoop(sr(`
		var ws = new WebSocket("WSBIN_URL/ws-echo-someheader", null, {headers: {"x-lorem": "ipsum"}})
		ws.onopen = () => {
			ws.close()
		}
	`))
	assert.NoError(t, err)

	samples := metrics.GetBufferedSamples(ts.samples)
	assertSessionMetricsEmitted(t, samples, "", sr("WSBIN_URL/ws-echo-someheader"), http.StatusSwitchingProtocols, "")

	mu.Lock()
	assert.True(t, len(collected) > 0)
	assert.Equal(t, "ipsum", collected.Get("x-lorem"))
	assert.Equal(t, "TestUserAgent", collected.Get("User-Agent"))
	mu.Unlock()
	assert.Len(t, ts.errors, 0)
}

func TestCookies(t *testing.T) {
	t.Parallel()
	ts := newTestState(t)
	sr := ts.tb.Replacer.Replace

	mu := &sync.Mutex{}
	collected := make(map[string]string)

	ts.tb.Mux.HandleFunc("/ws-echo-someheader", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		responseHeaders := w.Header().Clone()
		conn, err := (&websocket.Upgrader{}).Upgrade(w, req, responseHeaders)
		if err != nil {
			ts.errors <- fmt.Errorf("/ws-echo-someheader cannot upgrade request: %w", err)
			return
		}

		mu.Lock()
		for _, v := range req.Cookies() {
			collected[v.Name] = v.Value
		}
		mu.Unlock()

		err = conn.Close()
		if err != nil {
			t.Logf("error while closing connection in /ws-echo-someheader: %v", err)
		}
	}))

	err := ts.runtime.VU.RuntimeField.Set("http", httpModule.New().NewModuleInstance(ts.runtime.VU).Exports().Default)
	require.NoError(t, err)

	ts.runtime.VU.StateField.CookieJar, _ = cookiejar.New(nil)
	_, err = ts.runtime.RunOnEventLoop(sr(`
		var jar = new http.CookieJar();
		jar.set("HTTPBIN_URL/ws-echo-someheader", "someheader", "customjar")

		var ws = new WebSocket("WSBIN_URL/ws-echo-someheader", null, {jar: jar})
		ws.onopen = () => {
			ws.close()
		}
	`))
	assert.NoError(t, err)

	samples := metrics.GetBufferedSamples(ts.samples)
	assertSessionMetricsEmitted(t, samples, "", sr("WSBIN_URL/ws-echo-someheader"), http.StatusSwitchingProtocols, "")

	mu.Lock()
	assert.True(t, len(collected) > 0)
	assert.Equal(t, map[string]string{"someheader": "customjar"}, collected)
	mu.Unlock()

	assert.Len(t, ts.errors, 0)
}

func TestCookiesDefaultJar(t *testing.T) {
	t.Parallel()
	ts := newTestState(t)
	sr := ts.tb.Replacer.Replace

	mu := &sync.Mutex{}
	collected := make(map[string]string)

	ts.tb.Mux.HandleFunc("/ws-echo-someheader", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		responseHeaders := w.Header().Clone()
		conn, err := (&websocket.Upgrader{}).Upgrade(w, req, responseHeaders)
		if err != nil {
			ts.errors <- fmt.Errorf("/ws-echo-someheader cannot upgrade request: %w", err)
			return
		}

		mu.Lock()
		for _, v := range req.Cookies() {
			collected[v.Name] = v.Value
		}
		mu.Unlock()

		err = conn.Close()
		if err != nil {
			t.Logf("error while closing connection in /ws-echo-someheader: %v", err)
		}
	}))

	err := ts.runtime.VU.RuntimeField.Set("http", httpModule.New().NewModuleInstance(ts.runtime.VU).Exports().Default)
	require.NoError(t, err)

	ts.runtime.VU.StateField.CookieJar, _ = cookiejar.New(nil)
	_, err = ts.runtime.RunOnEventLoop(sr(`
		http.cookieJar().set("HTTPBIN_URL/ws-echo-someheader", "someheader", "defaultjar")		

		var ws = new WebSocket("WSBIN_URL/ws-echo-someheader", null)
		ws.onopen = () => {
			ws.close()
		}
	`))
	assert.NoError(t, err)

	samples := metrics.GetBufferedSamples(ts.samples)
	assertSessionMetricsEmitted(t, samples, "", sr("WSBIN_URL/ws-echo-someheader"), http.StatusSwitchingProtocols, "")

	mu.Lock()
	assert.True(t, len(collected) > 0)
	assert.Equal(t, map[string]string{"someheader": "defaultjar"}, collected)
	mu.Unlock()

	assert.Len(t, ts.errors, 0)
}

func TestManualNameTag(t *testing.T) {
	t.Parallel()
	ts := newTestState(t)
	sr := ts.tb.Replacer.Replace

	ts.runtime.VU.StateField.Options.SystemTags = metrics.ToSystemTagSet([]string{"url", "name"})

	_, err := ts.runtime.RunOnEventLoop(sr(`
				var ws = new WebSocket("WSBIN_URL/ws-echo", null, { tags: { name: "custom" } } )
				ws.onopen = () => {
					ws.send("test")
				}
				ws.onmessage = (event) => {
					if (event.data != "test") {
						throw new Error ("echo'd data doesn't match our message!");
					}
					ws.close()
				}
				ws.onerror = (e) => { throw JSON.stringify(e) }
			`))
	require.NoError(t, err)

	containers := metrics.GetBufferedSamples(ts.samples)
	require.NotEmpty(t, containers)

	for _, sampleContainer := range containers {
		require.NotEmpty(t, sampleContainer.GetSamples())
		for _, sample := range sampleContainer.GetSamples() {
			dataToCheck := sample.Tags.Map()
			require.NotEmpty(t, dataToCheck)

			assert.Equal(t, "custom", dataToCheck["url"])
			assert.Equal(t, "custom", dataToCheck["name"])
		}
	}
}

func TestSystemTags(t *testing.T) {
	t.Parallel()

	testedSystemTags := []string{"status", "subproto", "url", "ip"}
	for _, expectedTagStr := range testedSystemTags {
		expectedTagStr := expectedTagStr
		t.Run("only "+expectedTagStr, func(t *testing.T) {
			t.Parallel()
			expectedTag, err := metrics.SystemTagString(expectedTagStr)
			require.NoError(t, err)

			ts := newTestState(t)
			sr := ts.tb.Replacer.Replace
			ts.runtime.VU.StateField.Options.SystemTags = metrics.ToSystemTagSet([]string{expectedTagStr})

			_, err = ts.runtime.RunOnEventLoop(sr(`
				var ws = new WebSocket("WSBIN_URL/ws-echo")
				ws.onopen = () => {
					ws.send("test")
				}
				ws.onmessage = (event) => {
					if (event.data != "test") {
						throw new Error ("echo'd data doesn't match our message!");
					}
					ws.close()
				}
				ws.onerror = (e) => { throw JSON.stringify(e) }
			`))
			require.NoError(t, err)

			containers := metrics.GetBufferedSamples(ts.samples)
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

func TestCustomTags(t *testing.T) {
	t.Parallel()

	ts := newTestState(t)
	sr := ts.tb.Replacer.Replace
	_, err := ts.runtime.RunOnEventLoop(sr(`
	var ws = new WebSocket("WSBIN_URL/ws-echo", null, {tags: {lorem: "ipsum", version: 13}})
	ws.onopen = () => {
		ws.send("something")
		ws.close()
	}
	ws.onerror = (e) => { throw JSON.stringify(e) }
	`))
	require.NoError(t, err)
	samples := metrics.GetBufferedSamples(ts.samples)
	assertSessionMetricsEmitted(t, samples, "", sr("WSBIN_URL/ws-echo"), http.StatusSwitchingProtocols, "")

	for _, sampleContainer := range samples {
		require.NotEmpty(t, sampleContainer.GetSamples())
		for _, sample := range sampleContainer.GetSamples() {
			dataToCheck := sample.Tags.Map()

			require.NotEmpty(t, dataToCheck)

			assert.Equal(t, "ipsum", dataToCheck["lorem"])
			assert.Equal(t, "13", dataToCheck["version"])
			assert.NotEmpty(t, dataToCheck["url"])
		}
	}
}

func TestCompressionSession(t *testing.T) {
	t.Parallel()
	const text string = `Lorem ipsum dolor sit amet, consectetur adipiscing elit. Maecenas sed pharetra sapien. Nunc laoreet molestie ante ac gravida. Etiam interdum dui viverra posuere egestas. Pellentesque at dolor tristique, mattis turpis eget, commodo purus. Nunc orci aliquam.`

	ts := newTestState(t)
	sr := ts.tb.Replacer.Replace

	ts.addHandler("/ws-compression", &websocket.Upgrader{
		EnableCompression: true,
		ReadBufferSize:    1024,
		WriteBufferSize:   1024,
	}, &testMessage{websocket.TextMessage, []byte(text)})

	_, err := ts.runtime.RunOnEventLoop(sr(`
		var params = {
			"compression": "deflate"
		}
		var ws = new WebSocket("WSBIN_URL/ws-compression", null, params)

		ws.onmessage = (event) => {
			if (event.data != "` + text + `"){
				throw new Error("wrong message received from server: ", event.data)
			}

			const expectedExtension = "permessage-deflate; server_no_context_takeover; client_no_context_takeover"
			if (!(ws.extensions.includes(expectedExtension))) {
				throw "expected value '" + expectedExtension + "' missing in " + JSON.stringify(ws.extensions);
			}

			ws.close()
		}

		`))

	require.NoError(t, err)

	samples := metrics.GetBufferedSamples(ts.samples)
	url := sr("WSBIN_URL/ws-compression")
	assertSessionMetricsEmitted(t, samples, "", url, http.StatusSwitchingProtocols, "")
	assertMetricEmittedCount(t, metrics.WSMessagesReceivedName, samples, url, 1)

	assert.Len(t, ts.errors, 0)
}

func TestServerWithoutCompression(t *testing.T) {
	t.Parallel()
	const text string = `Lorem ipsum dolor sit amet, consectetur adipiscing elit. Maecenas sed pharetra sapien. Nunc laoreet molestie ante ac gravida. Etiam interdum dui viverra posuere egestas. Pellentesque at dolor tristique, mattis turpis eget, commodo purus. Nunc orci aliquam.`

	ts := newTestState(t)
	sr := ts.tb.Replacer.Replace

	ts.addHandler("/ws-compression", &websocket.Upgrader{}, &testMessage{websocket.TextMessage, []byte(text)})

	_, err := ts.runtime.RunOnEventLoop(sr(`
		var params = {
			"compression": "deflate"
		}
		var ws = new WebSocket("WSBIN_URL/ws-compression", null, params)
		ws.onmessage = (event) => {
			if (event.data != "` + text + `"){
				throw new Error("wrong message received from server: ", event.data)
			}

			ws.close()
		}
		`))

	require.NoError(t, err)

	samples := metrics.GetBufferedSamples(ts.samples)
	url := sr("WSBIN_URL/ws-compression")
	assertSessionMetricsEmitted(t, samples, "", url, http.StatusSwitchingProtocols, "")
	assertMetricEmittedCount(t, metrics.WSMessagesReceivedName, samples, url, 1)

	assert.Len(t, ts.errors, 0)
}

func TestCompressionParams(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		compression   string
		expectedError string
	}{
		{
			compression:   `""`,
			expectedError: `unsupported compression algorithm '', supported algorithm is 'deflate'`,
		},
		{
			compression:   `null`,
			expectedError: `unsupported compression algorithm 'null', supported algorithm is 'deflate'`,
		},
		{
			compression:   `undefined`,
			expectedError: `unsupported compression algorithm 'undefined', supported algorithm is 'deflate'`,
		},
		{
			compression:   `" "`,
			expectedError: `unsupported compression algorithm '', supported algorithm is 'deflate'`,
		},
		{compression: `"deflate"`},
		{compression: `"deflate "`},
		{
			compression:   `"gzip"`,
			expectedError: `unsupported compression algorithm 'gzip', supported algorithm is 'deflate'`,
		},
		{
			compression:   `"deflate, gzip"`,
			expectedError: `unsupported compression algorithm 'deflate, gzip', supported algorithm is 'deflate'`,
		},
		{
			compression:   `"deflate, deflate"`,
			expectedError: `unsupported compression algorithm 'deflate, deflate', supported algorithm is 'deflate'`,
		},
		{
			compression:   `"deflate, "`,
			expectedError: `unsupported compression algorithm 'deflate,', supported algorithm is 'deflate'`,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.compression, func(t *testing.T) {
			t.Parallel()
			ts := newTestState(t)
			sr := ts.tb.Replacer.Replace

			ts.addHandler("/ws-compression-param", &websocket.Upgrader{
				EnableCompression: true,
				ReadBufferSize:    1024,
				WriteBufferSize:   1024,
			}, nil)

			_, err := ts.runtime.RunOnEventLoop(sr(`
					var ws = new WebSocket("WSBIN_URL/ws-compression-param", null, {"compression":` + testCase.compression + `})
					ws.onopen = () => {
						ws.close()
					}
					`))

			if testCase.expectedError == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), testCase.expectedError)
			}
		})
	}
}

func TestSessionPing(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)
	sr := tb.Replacer.Replace

	ts := newTestState(t)

	_, err := ts.runtime.RunOnEventLoop(sr(`
			var ws = new WebSocket("WSBIN_URL/ws-echo")
			ws.onopen = () => {
				ws.ping()
			}

			ws.onpong = () => {
				call("from onpong")
				ws.close()
			}
			ws.onerror = (e) => { throw JSON.stringify(e) }
		`))

	require.NoError(t, err)

	samplesBuf := metrics.GetBufferedSamples(ts.samples)
	assertSessionMetricsEmitted(t, samplesBuf, "", sr("WSBIN_URL/ws-echo"), http.StatusSwitchingProtocols, "")
	assert.Equal(t, []string{"from onpong"}, ts.callRecorder.Recorded())
}

func TestSessionPingAdd(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)
	sr := tb.Replacer.Replace

	ts := newTestState(t)

	_, err := ts.runtime.RunOnEventLoop(sr(`
			var ws = new WebSocket("WSBIN_URL/ws-echo")			
			ws.addEventListener("open", () => {
				ws.ping()
			})

			ws.onerror = (e) => { throw JSON.stringify(e) }
			ws.addEventListener("pong", () => {
				call("from onpong")
				ws.close()
			})
		`))

	require.NoError(t, err)

	samplesBuf := metrics.GetBufferedSamples(ts.samples)
	assertSessionMetricsEmitted(t, samplesBuf, "", sr("WSBIN_URL/ws-echo"), http.StatusSwitchingProtocols, "")
	assert.Equal(t, []string{"from onpong"}, ts.callRecorder.Recorded())
}

func TestLockingUpWithAThrow(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sr := tb.Replacer.Replace

	ts := newTestState(t)
	go destroySamples(ctx, ts.samples)
	ts.runtime.VU.CtxField = ctx
	err := ts.runtime.EventLoop.Start(func() error {
		_, runErr := ts.runtime.VU.Runtime().RunString(sr(`
		let a = 0;
		const connections = 200;
		async function s() {
			let ws = new WebSocket("WSBIN_URL/ws-echo")
			ws.addEventListener("open", () => {
				ws.ping()
				a++
			})

			ws.addEventListener("pong", () => {
				ws.ping()
				if (a == connections){
					a++
					ws.close()
					throw "s";
				}
			})
		}
		[...Array(connections)].forEach(_ => s())
		`))
		return runErr
	})

	cancel()
	assert.ErrorContains(t, err, "s at <eval>")
	ts.runtime.EventLoop.WaitOnRegistered()
}

func TestLockingUpWithAJustGeneralCancel(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sr := tb.Replacer.Replace

	ts := newTestState(t)
	defer func() {
		close(ts.samples)
	}()
	go destroySamples(ctx, ts.samples)
	ts.runtime.VU.CtxField = ctx
	require.NoError(t, ts.runtime.VU.RuntimeField.Set("cancel", cancel))
	_, err := ts.runtime.RunOnEventLoop(sr(`
		let a = 0;
		const connections = 1000;
		async function s() {
			var ws = new WebSocket("WSBIN_URL/ws-echo")
			ws.addEventListener("open", () => {
				ws.ping()
			})

			ws.addEventListener("pong", () => {
				try{
					ws.ping() // this will
				} catch(e) {}
				a++
				if (a == connections){
					cancel()
				}
			})
		}
		[...Array(connections)].forEach(_ => s())
		`))

	cancel()
	assert.NoError(t, err)
	ts.runtime.EventLoop.WaitOnRegistered()
}

func destroySamples(ctx context.Context, c <-chan metrics.SampleContainer) {
	for {
		select {
		case <-c:
		case <-ctx.Done():
			return
		}
	}
}

func TestArrayBufferViewSupport(t *testing.T) {
	t.Parallel()
	for _, name := range []string{ // Commented ones aren't support by Sobek
		"Int8Array", "Int16Array", "Int32Array",
		"Uint8Array", "Uint16Array", "Uint32Array", "Uint8ClampedArray",
		// "BigInt64Array", "BigUint64Arrays",
		/*"Float16Array", */ "Float32Array", "Float64Array",
	} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			testArrayBufferViewSupport(t, name)
		})
	}
}

func testArrayBufferViewSupport(t *testing.T, viewName string) {
	t.Helper()
	ts := newTestState(t)
	logger, hook := testutils.NewLoggerWithHook(t, logrus.WarnLevel)
	ts.runtime.VU.StateField.Logger = logger
	_, err := ts.runtime.RunOnEventLoop(ts.tb.Replacer.Replace(fmt.Sprintf(`
		var ws = new WebSocket("WSBIN_URL/ws-echo")
		ws.addEventListener("open", () => {
			const sent = new %[1]s([164, 41])
			ws.send(sent)
			ws.onmessage = async (e) => {
				const received = new %[1]s(await e.data.arrayBuffer());
				for (let i = 0; i < sent.length; i++) {
					if (sent.at(i) != received.at(i)) {
						throw "Values at " + i + " were different " + sent.at(i) + " vs " + received.at(i);
					}
				}
				ws.close()
			}
		})
	`, viewName)))
	require.NoError(t, err)
	logs := hook.Drain()
	require.Len(t, logs, 0)
}

func TestReadyStateSwitch(t *testing.T) {
	t.Parallel()
	ts := newTestState(t)
	logger, hook := testutils.NewLoggerWithHook(t, logrus.WarnLevel)
	ts.runtime.VU.StateField.Logger = logger
	_, err := ts.runtime.RunOnEventLoop(ts.tb.Replacer.Replace(`
		var ws = new WebSocket("WSBIN_URL/ws-echo")
		try {
			switch (ws.readyState) {
				case 0:
					break;
				default:
					throw "ws.readyState doesn't get correct value in switch"
			}
		} finally {
			ws.close()
		}
	`))
	require.NoError(t, err)
	logs := hook.Drain()
	require.Len(t, logs, 0)
}
