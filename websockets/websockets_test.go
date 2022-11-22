package websockets

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/dop251/goja"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/eventloop"
	"go.k6.io/k6/js/modulestest"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/testutils/httpmultibin"
	"go.k6.io/k6/metrics"
)

// copied from k6/ws
func assertSessionMetricsEmitted(
	t *testing.T,
	sampleContainers []metrics.SampleContainer,
	subprotocol, //nolint:unparam // TODO: check why it always same in tests
	url string,
	status int, //nolint:unparam // TODO: check why it always same in tests
	group string, //nolint:unparam // TODO: check why it always same in tests
) {
	t.Helper()
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

type testState struct {
	rt      *goja.Runtime
	tb      *httpmultibin.HTTPMultiBin
	vu      *modulestest.VU
	state   *lib.State
	samples chan metrics.SampleContainer
	ev      *eventloop.EventLoop

	callRecorder *callRecorder
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
	tb := httpmultibin.NewHTTPMultiBin(t)

	root, err := lib.NewGroup("", nil)
	require.NoError(t, err)

	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})

	samples := make(chan metrics.SampleContainer, 1000)
	registry := metrics.NewRegistry()
	state := &lib.State{
		Group:  root,
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
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(registry),
		Tags:           lib.NewVUStateTags(registry.RootTagSet()),
	}

	recorder := &callRecorder{
		calls: make([]string, 0),
	}

	vu := &modulestest.VU{
		CtxField:     tb.Context,
		RuntimeField: rt,
		StateField:   state,
	}
	m := new(RootModule).NewModuleInstance(vu)
	require.NoError(t, rt.Set("WebSocket", m.Exports().Named["WebSocket"]))
	require.NoError(t, rt.Set("call", recorder.Call))

	ev := eventloop.New(vu)
	vu.RegisterCallbackField = ev.RegisterCallback

	return testState{
		rt:           rt,
		tb:           tb,
		vu:           vu,
		state:        state,
		samples:      samples,
		ev:           ev,
		callRecorder: recorder,
	}
}

func TestBasic(t *testing.T) {
	t.Parallel()
	ts := newTestState(t)
	sr := ts.tb.Replacer.Replace
	err := ts.ev.Start(func() error {
		_, err := ts.rt.RunString(sr(`
    var ws = new WebSocket("WSBIN_URL/ws-echo")
    ws.addEventListener("open", () => {
      ws.send("something")
      ws.close()
    })
	`))
		return err
	})
	require.NoError(t, err)
	samples := metrics.GetBufferedSamples(ts.samples)
	assertSessionMetricsEmitted(t, samples, "", sr("WSBIN_URL/ws-echo"), http.StatusSwitchingProtocols, "")
}

func TestBasicWithOn(t *testing.T) {
	t.Parallel()

	ts := newTestState(t)
	sr := ts.tb.Replacer.Replace
	err := ts.ev.Start(func() error {
		_, err := ts.rt.RunString(sr(`
    var ws = new WebSocket("WSBIN_URL/ws-echo")
    ws.onopen = () => {
      ws.send("something")
      ws.close()
    }
	`))
		return err
	})
	require.NoError(t, err)
	samples := metrics.GetBufferedSamples(ts.samples)
	assertSessionMetricsEmitted(t, samples, "", sr("WSBIN_URL/ws-echo"), http.StatusSwitchingProtocols, "")
}

func TestReadyState(t *testing.T) {
	t.Parallel()
	ts := newTestState(t)
	err := ts.ev.Start(func() error {
		_, err := ts.rt.RunString(ts.tb.Replacer.Replace(`
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
		return err
	})
	require.NoError(t, err)
}

func TestBinaryState(t *testing.T) {
	t.Parallel()
	ts := newTestState(t)
	err := ts.ev.Start(func() error {
		_, err := ts.rt.RunString(ts.tb.Replacer.Replace(`
    var ws = new WebSocket("WSBIN_URL/ws-echo")
    ws.addEventListener("open", () => ws.close())

    if (ws.binaryType != "ArrayBuffer") {
      throw new Error("Wrong binaryType value, expected ArrayBuffer got "+ ws.binaryType)
    }

    var thrown = false;
    try {
      ws.binaryType = "something"
    } catch(e) {
      thrown = true
    }
    if (!thrown) {
      throw new Error("Expects ws.binaryType to not be writable")
    }
	`))
		return err
	})
	require.NoError(t, err)
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
			expectedError: "oops is not defined at <eval>:4:7",
		},
		"onopen": {
			script: `
    var ws = new WebSocket("WSBIN_URL/ws/echo")
    ws.onopen = () => {
      oops
    }`,
			expectedError: "oops is not defined at <eval>:4:7",
		},
		"error": {
			script: `
    var ws = new WebSocket("WSBIN_URL/badurl")
    ws.addEventListener("error", () =>{
      inerroridf
    })
    `,
			expectedError: "inerroridf is not defined at <eval>:4:7",
		},
		"onerror": {
			script: `
    var ws = new WebSocket("WSBIN_URL/badurl")
    ws.onerror = () => {
      inerroridf
    }
    `,
			expectedError: "inerroridf is not defined at <eval>:4:7",
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
			expectedError: "incloseidf is not defined at <eval>:7:7",
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
			expectedError: "incloseidf is not defined at <eval>:7:7",
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
			expectedError: "inmessageidf is not defined at <eval>:7:7",
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
			expectedError: "inmessageidf is not defined at <eval>:7:7",
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
			err := ts.ev.Start(func() error {
				_, err := ts.rt.RunString(sr(testcase.script))
				return err
			})
			require.Error(t, err)
			require.Contains(t, err.Error(), testcase.expectedError)
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
		// fmt.Println(path, "ending")
	})

	err := ts.ev.Start(func() error {
		_, err := ts.rt.RunString(sr(`
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
		return err
	})
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

	err := ts.ev.Start(func() error {
		_, err := ts.rt.RunString(sr(`
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
		return err
	})
	require.NoError(t, err)
	samples := metrics.GetBufferedSamples(ts.samples)
	assertSessionMetricsEmitted(t, samples, "", sr("WSBIN_URL/ws/couple/1"), http.StatusSwitchingProtocols, "")
	assertSessionMetricsEmitted(t, samples, "", sr("WSBIN_URL/ws/couple/2"), http.StatusSwitchingProtocols, "")
}

func TestDialError(t *testing.T) {
	t.Parallel()
	ts := newTestState(t)
	sr := ts.tb.Replacer.Replace

	// without listeners
	err := ts.ev.Start(func() error {
		_, runErr := ts.rt.RunString(sr(`
		var ws = new WebSocket("ws://127.0.0.2");
	`))
		return runErr
	})
	require.NoError(t, err)

	// with the error listener
	ts.ev.WaitOnRegistered()
	err = ts.ev.Start(func() error {
		_, runErr := ts.rt.RunString(sr(`
		var ws = new WebSocket("ws://127.0.0.2");
		ws.addEventListener("error", (e) =>{
			ws.close();
			throw new Error("The provided url is an invalid endpoint")
		})
	`))
		return runErr
	})
	assert.Error(t, err)
}

func TestOnError(t *testing.T) {
	t.Parallel()
	ts := newTestState(t)
	sr := ts.tb.Replacer.Replace

	ts.ev.WaitOnRegistered()
	err := ts.ev.Start(func() error {
		_, runErr := ts.rt.RunString(sr(`
		var ws = new WebSocket("ws://127.0.0.2");
		ws.onerror = (e) => {
			ws.close();
			throw new Error("lorem ipsum")
		}
	`))
		return runErr
	})
	assert.Error(t, err)
	assert.Equal(t, "Error: lorem ipsum at <eval>:5:10(7)", err.Error())
}

func TestOnClose(t *testing.T) {
	t.Parallel()
	ts := newTestState(t)
	sr := ts.tb.Replacer.Replace

	ts.ev.WaitOnRegistered()
	err := ts.ev.Start(func() error {
		_, runErr := ts.rt.RunString(sr(`
		var ws = new WebSocket("WSBIN_URL/ws/echo")
		ws.onopen = () => {
			ws.close()
		}
		ws.onclose = () =>{
			call("from close")
		}
	`))
		return runErr
	})
	assert.NoError(t, err)
	assert.Equal(t, []string{"from close"}, ts.callRecorder.Recorded())
}

func TestMixingOnAndAddHandlers(t *testing.T) {
	t.Parallel()
	ts := newTestState(t)
	sr := ts.tb.Replacer.Replace

	ts.ev.WaitOnRegistered()
	err := ts.ev.Start(func() error {
		_, runErr := ts.rt.RunString(sr(`
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
		return runErr
	})
	assert.NoError(t, err)
	assert.Equal(t, 2, ts.callRecorder.Len())
	assert.Contains(t, ts.callRecorder.Recorded(), "from addEventListener")
	assert.Contains(t, ts.callRecorder.Recorded(), "from onclose")
}

func TestOncloseRedefineListener(t *testing.T) {
	t.Parallel()

	ts := newTestState(t)
	sr := ts.tb.Replacer.Replace

	ts.ev.WaitOnRegistered()
	err := ts.ev.Start(func() error {
		_, runErr := ts.rt.RunString(sr(`
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
		return runErr
	})
	assert.NoError(t, err)
	assert.Equal(t, []string{"from onclose 2"}, ts.callRecorder.Recorded())
}

func TestOncloseRedefineWithNull(t *testing.T) {
	t.Parallel()

	ts := newTestState(t)
	sr := ts.tb.Replacer.Replace

	ts.ev.WaitOnRegistered()
	err := ts.ev.Start(func() error {
		_, runErr := ts.rt.RunString(sr(`
		var ws = new WebSocket("WSBIN_URL/ws/echo")
		ws.onopen = () => {
			ws.close()
		}
		ws.onclose = () =>{
			call("from onclose")
		}
		ws.onclose = null
	`))
		return runErr
	})
	assert.NoError(t, err)
	assert.Equal(t, 0, ts.callRecorder.Len())
}

func TestOncloseDefineWithInvalidValue(t *testing.T) {
	t.Parallel()

	ts := newTestState(t)
	sr := ts.tb.Replacer.Replace

	ts.ev.WaitOnRegistered()
	err := ts.ev.Start(func() error {
		_, runErr := ts.rt.RunString(sr(`
		var ws = new WebSocket("WSBIN_URL/ws/echo")
		ws.onclose = 1
	`))
		return runErr
	})
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
			t.Fatalf("/ws-echo-someheader cannot upgrade request: %v", err)
		}

		mu.Lock()
		collected = req.Header.Clone()
		mu.Unlock()

		err = conn.Close()
		if err != nil {
			t.Logf("error while closing connection in /ws-echo-someheader: %v", err)
		}
	}))

	err := ts.ev.Start(func() error {
		_, runErr := ts.rt.RunString(sr(`
		var ws = new WebSocket("WSBIN_URL/ws-echo-someheader", null, {headers: {"x-lorem": "ipsum"}})
		ws.onopen = () => {
			ws.close()
		}
	`))
		return runErr
	})
	assert.NoError(t, err)

	samples := metrics.GetBufferedSamples(ts.samples)
	assertSessionMetricsEmitted(t, samples, "", sr("WSBIN_URL/ws-echo-someheader"), http.StatusSwitchingProtocols, "")

	mu.Lock()
	assert.True(t, len(collected) > 0)
	assert.Equal(t, "ipsum", collected.Get("x-lorem"))
	assert.Equal(t, "TestUserAgent", collected.Get("User-Agent"))
	mu.Unlock()
}
