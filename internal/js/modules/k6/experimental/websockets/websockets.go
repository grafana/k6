// Package websockets implements to some extend WebSockets API https://websockets.spec.whatwg.org
package websockets

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/grafana/sobek"
	"github.com/mstoykov/k6-taskqueue-lib/taskqueue"
	"go.k6.io/k6/internal/js/modules/k6/experimental/websockets/events"

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/metrics"
)

// RootModule is the root module for the websockets API
type RootModule struct{}

// WebSocketsAPI is the k6 extension implementing the websocket API as defined in https://websockets.spec.whatwg.org
type WebSocketsAPI struct { //nolint:revive
	vu              modules.VU
	blobConstructor sobek.Value
}

var _ modules.Module = &RootModule{}

// New websockets root module
func New() *RootModule {
	return &RootModule{}
}

// NewModuleInstance returns a new instance of the module
func (r *RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	return &WebSocketsAPI{
		vu: vu,
	}
}

// Exports implements the modules.Instance interface's Exports
func (r *WebSocketsAPI) Exports() modules.Exports {
	r.blobConstructor = r.vu.Runtime().ToValue(r.blob)
	return modules.Exports{
		Named: map[string]interface{}{
			"WebSocket": r.websocket,
			"Blob":      r.blobConstructor,
		},
	}
}

// ReadyState is websocket specification's readystate
type ReadyState uint8

const (
	// CONNECTING is the state while the web socket is connecting
	CONNECTING ReadyState = iota
	// OPEN is the state after the websocket is established and before it starts closing
	OPEN
	// CLOSING is while the websocket is closing but is *not* closed yet
	CLOSING
	// CLOSED is when the websocket is finally closed
	CLOSED
)

type webSocket struct {
	vu              modules.VU
	blobConstructor sobek.Value

	url            *url.URL
	conn           *websocket.Conn
	tagsAndMeta    *metrics.TagsAndMeta
	tq             *taskqueue.TaskQueue
	builtinMetrics *metrics.BuiltinMetrics
	obj            *sobek.Object // the object that is given to js to interact with the WebSocket
	started        time.Time

	done         chan struct{}
	writeQueueCh chan message

	eventListeners *eventListeners

	sendPings ping

	// fields that should be seen by js only be updated on the event loop
	readyState     ReadyState
	bufferedAmount int
	binaryType     string
	protocol       string
	extensions     []string
}

type ping struct {
	counter    int
	timestamps map[string]time.Time
}

func (r *WebSocketsAPI) websocket(c sobek.ConstructorCall) *sobek.Object {
	rt := r.vu.Runtime()

	url, err := parseURL(c.Argument(0))
	if err != nil {
		common.Throw(rt, err)
	}

	params, err := buildParams(r.vu.State(), rt, c.Argument(2))
	if err != nil {
		common.Throw(rt, err)
	}

	subprocotolsArg := c.Argument(1)
	if !common.IsNullish(subprocotolsArg) {
		subprocotolsObj := subprocotolsArg.ToObject(rt)
		switch {
		case isString(subprocotolsObj, rt):
			params.subprocotols = append(params.subprocotols, subprocotolsObj.String())
		case isArray(subprocotolsObj, rt):
			for _, key := range subprocotolsObj.Keys() {
				params.subprocotols = append(params.subprocotols, subprocotolsObj.Get(key).String())
			}
		}
	}

	w := &webSocket{
		vu:              r.vu,
		blobConstructor: r.blobConstructor,
		url:             url,
		tq:              taskqueue.New(r.vu.RegisterCallback),
		readyState:      CONNECTING,
		builtinMetrics:  r.vu.State().BuiltinMetrics,
		done:            make(chan struct{}),
		writeQueueCh:    make(chan message),
		eventListeners:  newEventListeners(),
		obj:             rt.NewObject(),
		tagsAndMeta:     params.tagsAndMeta,
		sendPings:       ping{timestamps: make(map[string]time.Time)},
		binaryType:      blobBinaryType,
	}

	// Maybe have this after the goroutine below ?!?
	defineWebsocket(rt, w)

	go w.establishConnection(params)
	return w.obj
}

// parseURL parses the url from the first constructor calls argument or returns an error
func parseURL(urlValue sobek.Value) (*url.URL, error) {
	if urlValue == nil || sobek.IsUndefined(urlValue) {
		return nil, errors.New("WebSocket requires a url")
	}

	// TODO: throw the SyntaxError (https://websockets.spec.whatwg.org/#dom-websocket-websocket)
	urlString := urlValue.String()
	url, err := url.Parse(urlString)
	if err != nil {
		return nil, fmt.Errorf("WebSocket requires valid url, but got %q which resulted in %w", urlString, err)
	}
	if url.Scheme != "ws" && url.Scheme != "wss" {
		return nil, fmt.Errorf("WebSocket requires url with scheme ws or wss, but got %q", url.Scheme)
	}
	if url.Fragment != "" {
		return nil, fmt.Errorf("WebSocket requires no url fragment, but got %q", url.Fragment)
	}

	return url, nil
}

const (
	arraybufferBinaryType = "arraybuffer"
	blobBinaryType        = "blob"
)

// defineWebsocket defines all properties and methods for the WebSocket
func defineWebsocket(rt *sobek.Runtime, w *webSocket) {
	must(rt, w.obj.DefineDataProperty(
		"addEventListener", rt.ToValue(w.addEventListener), sobek.FLAG_FALSE, sobek.FLAG_FALSE, sobek.FLAG_TRUE))
	must(rt, w.obj.DefineDataProperty(
		"send", rt.ToValue(w.send), sobek.FLAG_FALSE, sobek.FLAG_FALSE, sobek.FLAG_TRUE))
	must(rt, w.obj.DefineDataProperty(
		"ping", rt.ToValue(w.ping), sobek.FLAG_FALSE, sobek.FLAG_FALSE, sobek.FLAG_TRUE))
	must(rt, w.obj.DefineDataProperty(
		"close", rt.ToValue(w.close), sobek.FLAG_FALSE, sobek.FLAG_FALSE, sobek.FLAG_TRUE))
	must(rt, w.obj.DefineDataProperty(
		"url", rt.ToValue(w.url.String()), sobek.FLAG_FALSE, sobek.FLAG_FALSE, sobek.FLAG_TRUE))
	must(rt, w.obj.DefineAccessorProperty( // this needs to be with an accessor as we change the value
		"readyState", rt.ToValue(func() sobek.Value {
			return rt.ToValue((uint)(w.readyState))
		}), nil, sobek.FLAG_FALSE, sobek.FLAG_TRUE))
	must(rt, w.obj.DefineAccessorProperty(
		"bufferedAmount", rt.ToValue(func() sobek.Value { return rt.ToValue(w.bufferedAmount) }), nil,
		sobek.FLAG_FALSE, sobek.FLAG_TRUE))
	must(rt, w.obj.DefineAccessorProperty("extensions",
		rt.ToValue(func() sobek.Value { return rt.ToValue(w.extensions) }), nil, sobek.FLAG_FALSE, sobek.FLAG_TRUE))
	must(rt, w.obj.DefineAccessorProperty(
		"protocol", rt.ToValue(func() sobek.Value { return rt.ToValue(w.protocol) }), nil, sobek.FLAG_FALSE, sobek.FLAG_TRUE))
	must(rt, w.obj.DefineAccessorProperty(
		"binaryType", rt.ToValue(func() sobek.Value {
			return rt.ToValue(w.binaryType)
		}), rt.ToValue(func(s string) error {
			switch s {
			case blobBinaryType, arraybufferBinaryType:
				w.binaryType = s
				return nil
			default:
				return fmt.Errorf(`unknown binaryType %s, the supported ones are "blob" and "arraybuffer"`, s)
			}
		}), sobek.FLAG_FALSE, sobek.FLAG_TRUE))

	setOn := func(property string, el *eventListener) {
		if el == nil {
			// this is generally should not happen, but we're being defensive
			common.Throw(rt, fmt.Errorf("not supported on-handler '%s'", property))
		}

		must(rt, w.obj.DefineAccessorProperty(
			property, rt.ToValue(func() sobek.Value {
				return rt.ToValue(el.getOn)
			}), rt.ToValue(func(call sobek.FunctionCall) sobek.Value {
				arg := call.Argument(0)

				// it's possible to unset handlers by setting them to null
				if arg == nil || sobek.IsUndefined(arg) || sobek.IsNull(arg) {
					el.setOn(nil)

					return nil
				}

				fn, isFunc := sobek.AssertFunction(arg)
				if !isFunc {
					common.Throw(rt, fmt.Errorf("a value for '%s' should be callable", property))
				}

				el.setOn(func(v sobek.Value) (sobek.Value, error) { return fn(sobek.Undefined(), v) })

				return nil
			}), sobek.FLAG_FALSE, sobek.FLAG_TRUE))
	}

	setOn("onmessage", w.eventListeners.getType(events.MESSAGE))
	setOn("onerror", w.eventListeners.getType(events.ERROR))
	setOn("onopen", w.eventListeners.getType(events.OPEN))
	setOn("onclose", w.eventListeners.getType(events.CLOSE))
	setOn("onping", w.eventListeners.getType(events.PING))
	setOn("onpong", w.eventListeners.getType(events.PONG))
}

type message struct {
	mtype int // message type consts as defined in gorilla/websocket/conn.go
	data  []byte
	t     time.Time
}

// documented https://websockets.spec.whatwg.org/#concept-websocket-establish
func (w *webSocket) establishConnection(params *wsParams) {
	state := w.vu.State()
	w.started = time.Now()
	var tlsConfig *tls.Config
	if state.TLSConfig != nil {
		tlsConfig = state.TLSConfig.Clone()
		tlsConfig.NextProtos = []string{"http/1.1"}
	}
	// technically we have to do a fetch request here, so ... uh do normal one ;)
	wsd := websocket.Dialer{
		HandshakeTimeout: time.Second * 60, // TODO configurable
		// Pass a custom net.DialContext function to websocket.Dialer that will substitute
		// the underlying net.Conn with our own tracked netext.Conn
		NetDialContext:    state.Dialer.DialContext,
		Proxy:             http.ProxyFromEnvironment,
		TLSClientConfig:   tlsConfig,
		EnableCompression: params.enableCompression,
		Subprotocols:      params.subprocotols,
	}

	// this is needed because of how interfaces work and that wsd.Jar is http.Cookiejar
	if params.cookieJar != nil {
		wsd.Jar = params.cookieJar
	}

	ctx := w.vu.Context()
	start := time.Now()
	conn, httpResponse, connErr := wsd.DialContext(ctx, w.url.String(), params.headers)
	connectionEnd := time.Now()
	connectionDuration := metrics.D(connectionEnd.Sub(start))

	systemTags := state.Options.SystemTags

	if conn != nil && conn.RemoteAddr() != nil {
		if ip, _, err := net.SplitHostPort(conn.RemoteAddr().String()); err == nil {
			w.tagsAndMeta.SetSystemTagOrMetaIfEnabled(systemTags, metrics.TagIP, ip)
		}
	}

	if httpResponse != nil {
		defer func() {
			_ = httpResponse.Body.Close()
		}()

		w.tagsAndMeta.SetSystemTagOrMetaIfEnabled(systemTags, metrics.TagStatus, strconv.Itoa(httpResponse.StatusCode))
		if conn != nil {
			w.protocol = conn.Subprotocol()
		}
		w.extensions = httpResponse.Header.Values("Sec-WebSocket-Extensions")
		w.tagsAndMeta.SetSystemTagOrMetaIfEnabled(systemTags, metrics.TagSubproto, w.protocol)
	}
	w.conn = conn

	nameTagValue, nameTagManuallySet := params.tagsAndMeta.Tags.Get(metrics.TagName.String())
	// After k6 v0.41.0, the `name` and `url` tags have the exact same values:
	if nameTagManuallySet {
		w.tagsAndMeta.SetSystemTagOrMetaIfEnabled(systemTags, metrics.TagURL, nameTagValue)
	} else {
		w.tagsAndMeta.SetSystemTagOrMetaIfEnabled(systemTags, metrics.TagURL, w.url.String())
		w.tagsAndMeta.SetSystemTagOrMetaIfEnabled(systemTags, metrics.TagName, w.url.String())
	}

	w.emitConnectionMetrics(ctx, start, connectionDuration)
	if connErr != nil {
		// Pass the error to the user script before exiting immediately
		w.tq.Queue(func() error {
			return w.connectionClosedWithError(connErr)
		})
		w.tq.Close()
		return
	}
	go w.loop()
	w.tq.Queue(func() error {
		return w.connectionConnected()
	})
}

// emitConnectionMetrics emits the metrics for a websocket connection.
func (w *webSocket) emitConnectionMetrics(ctx context.Context, start time.Time, duration float64) {
	state := w.vu.State()

	metrics.PushIfNotDone(ctx, state.Samples, metrics.ConnectedSamples{
		Samples: []metrics.Sample{
			{
				TimeSeries: metrics.TimeSeries{Metric: state.BuiltinMetrics.WSSessions, Tags: w.tagsAndMeta.Tags},
				Time:       start,
				Metadata:   w.tagsAndMeta.Metadata,
				Value:      1,
			},
			{
				TimeSeries: metrics.TimeSeries{Metric: state.BuiltinMetrics.WSConnecting, Tags: w.tagsAndMeta.Tags},
				Time:       start,
				Metadata:   w.tagsAndMeta.Metadata,
				Value:      duration,
			},
		},
		Tags: w.tagsAndMeta.Tags,
		Time: start,
	})
}

const writeWait = 10 * time.Second

func (w *webSocket) loop() {
	// Pass ping/pong events through the main control loop
	pingChan := make(chan string)
	pongChan := make(chan string)
	w.conn.SetPingHandler(func(msg string) error { pingChan <- msg; return nil })
	w.conn.SetPongHandler(func(pingID string) error { pongChan <- pingID; return nil })

	ctx := w.vu.Context()
	wg := new(sync.WaitGroup)

	defer func() {
		metrics.PushIfNotDone(ctx, w.vu.State().Samples, metrics.Sample{
			TimeSeries: metrics.TimeSeries{
				Metric: w.builtinMetrics.WSSessionDuration,
				Tags:   w.tagsAndMeta.Tags,
			},
			Time:     time.Now(),
			Metadata: w.tagsAndMeta.Metadata,
			Value:    metrics.D(time.Since(w.started)),
		})
		_ = w.conn.Close()
		wg.Wait()
		w.tq.Close()
	}()
	wg.Add(2)
	go w.readPump(wg)
	go w.writePump(wg)

	ctxDone := ctx.Done()
	for {
		select {
		case <-ctxDone:
			// VU is shutting down during an interrupt
			// socket events will not be forwarded to the VU
			w.queueClose()
			ctxDone = nil // this is to block this branch and get through w.done
		case <-w.done:
			return
		case pingData := <-pingChan:

			// Handle pings received from the server
			// - trigger the `ping` event
			// - reply with pong (needed when `SetPingHandler` is overwritten)
			// WriteControl is okay to be concurrent so we don't need to gsend this over writeChannel
			err := w.conn.WriteControl(websocket.PongMessage, []byte(pingData), time.Now().Add(writeWait))
			w.tq.Queue(func() error {
				if err != nil {
					return w.callErrorListeners(err)
				}

				return w.callEventListeners(events.PING)
			})

		case pingID := <-pongChan:
			w.tq.Queue(func() error {
				// Handle pong responses to our pings
				w.trackPong(pingID)

				return w.callEventListeners(events.PONG)
			})
		}
	}
}

func (w *webSocket) queueMessage(msg *message) {
	w.tq.Queue(func() error {
		if w.readyState != OPEN {
			return nil // TODO maybe still emit
		}
		// TODO maybe emit after all the listeners have fired and skip it if defaultPrevent was called?!?
		metrics.PushIfNotDone(w.vu.Context(), w.vu.State().Samples, metrics.Sample{
			TimeSeries: metrics.TimeSeries{
				Metric: w.builtinMetrics.WSMessagesReceived,
				Tags:   w.tagsAndMeta.Tags,
			},
			Time:     msg.t,
			Metadata: w.tagsAndMeta.Metadata,
			Value:    1,
		})

		rt := w.vu.Runtime()
		ev := w.newEvent(events.MESSAGE, msg.t)

		if msg.mtype == websocket.BinaryMessage {
			var data any
			switch w.binaryType {
			case blobBinaryType:
				var err error
				data, err = rt.New(w.blobConstructor, rt.ToValue([]interface{}{msg.data}))
				if err != nil {
					return fmt.Errorf("failed to create Blob: %w", err)
				}
			case arraybufferBinaryType:
				data = rt.NewArrayBuffer(msg.data)
			default:
				return fmt.Errorf(`unknown binaryType %q, the supported ones are "blob" and "arraybuffer"`, w.binaryType)
			}
			must(rt, ev.DefineDataProperty("data", rt.ToValue(data), sobek.FLAG_FALSE, sobek.FLAG_FALSE, sobek.FLAG_TRUE))
		} else {
			must(
				rt,
				ev.DefineDataProperty("data", rt.ToValue(string(msg.data)), sobek.FLAG_FALSE, sobek.FLAG_FALSE, sobek.FLAG_TRUE),
			)
		}
		must(
			rt,
			ev.DefineDataProperty("origin", rt.ToValue(w.url.String()), sobek.FLAG_FALSE, sobek.FLAG_FALSE, sobek.FLAG_TRUE),
		)

		for _, messageListener := range w.eventListeners.all(events.MESSAGE) {
			if _, err := messageListener(ev); err != nil {
				_ = w.conn.Close()                   // TODO log it?
				_ = w.connectionClosedWithError(err) // TODO log it?
				return err
			}
		}
		return nil
	})
}

func (w *webSocket) readPump(wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		messageType, data, err := w.conn.ReadMessage()
		if err == nil {
			w.queueMessage(&message{
				mtype: messageType,
				data:  data,
				t:     time.Now(),
			})

			continue
		}

		if !websocket.IsUnexpectedCloseError(
			err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
			// maybe still log it with debug level?
			err = nil
		}

		if err != nil {
			w.tq.Queue(func() error {
				_ = w.conn.Close() // TODO fix this
				return nil
			})
		}

		w.tq.Queue(func() error {
			return w.connectionClosedWithError(err)
		})

		return
	}
}

func (w *webSocket) writePump(wg *sync.WaitGroup) {
	defer wg.Done()
	wg.Add(1)
	samplesOutput := w.vu.State().Samples
	ctx := w.vu.Context()
	writeChannel := make(chan message)
	go func() {
		defer wg.Done()
		for {
			select {
			case msg, ok := <-writeChannel:
				if !ok {
					return
				}
				size := len(msg.data)

				err := func() error {
					if msg.mtype != websocket.PingMessage {
						return w.conn.WriteMessage(msg.mtype, msg.data)
					}

					// WriteControl is concurrently okay
					return w.conn.WriteControl(msg.mtype, msg.data, msg.t.Add(writeWait))
				}()
				if err != nil {
					w.tq.Queue(func() error {
						_ = w.conn.Close() // TODO fix
						closeErr := w.connectionClosedWithError(err)
						return closeErr
					})
					return
				}
				// This from the specification needs to happen like that instead of with
				// atomics or locks outside of the event loop
				w.tq.Queue(func() error {
					w.bufferedAmount -= size
					return nil
				})

				metrics.PushIfNotDone(ctx, samplesOutput, metrics.Sample{
					TimeSeries: metrics.TimeSeries{
						Metric: w.builtinMetrics.WSMessagesSent,
						Tags:   w.tagsAndMeta.Tags,
					},
					Time:     time.Now(),
					Metadata: w.tagsAndMeta.Metadata,
					Value:    1,
				})
			case <-w.done:
				return
			}
		}
	}()
	{
		defer close(writeChannel)
		queue := make([]message, 0)
		var wch chan message
		var msg message
		for {
			wch = nil // this way if nothing to read it will just block
			if len(queue) > 0 {
				msg = queue[0]
				wch = writeChannel
			}
			select {
			case msg = <-w.writeQueueCh:
				queue = append(queue, msg)
			case wch <- msg:
				queue = queue[:copy(queue, queue[1:])]
			case <-w.done:
				return
			}
		}
	}
}

func (w *webSocket) send(msg sobek.Value) {
	w.assertStateOpen()

	switch o := msg.Export().(type) {
	case string:
		w.bufferedAmount += len(o)
		w.writeQueueCh <- message{
			mtype: websocket.TextMessage,
			data:  []byte(o),
			t:     time.Now(),
		}
	case *sobek.ArrayBuffer:
		w.sendArrayBuffer(*o)
	case sobek.ArrayBuffer:
		w.sendArrayBuffer(o)
	case map[string]interface{}:
		rt := w.vu.Runtime()
		obj := msg.ToObject(rt)
		if !isBlob(obj, w.blobConstructor) {
			common.Throw(rt, fmt.Errorf("unsupported send type %T", o))
		}

		b := extractBytes(obj, rt)
		w.bufferedAmount += len(b)
		w.writeQueueCh <- message{
			mtype: websocket.BinaryMessage,
			data:  b,
			t:     time.Now(),
		}
	default:
		rt := w.vu.Runtime()
		isView, err := isArrayBufferView(rt, msg)
		if err != nil {
			common.Throw(rt,
				fmt.Errorf("got error while trying to check if argument is ArrayBufferView: %w", err))
		}
		if !isView {
			common.Throw(rt, fmt.Errorf("unsupported send type %T", o))
		}

		buffer := msg.ToObject(rt).Get("buffer")
		ab, ok := buffer.Export().(sobek.ArrayBuffer)
		if !ok {
			common.Throw(rt,
				fmt.Errorf("buffer of an ArrayBufferView was not an ArrayBuffer but %T", buffer.Export()))
		}
		w.sendArrayBuffer(ab)
	}
}

func (w *webSocket) sendArrayBuffer(o sobek.ArrayBuffer) {
	b := o.Bytes()
	w.bufferedAmount += len(b)
	w.writeQueueCh <- message{
		mtype: websocket.BinaryMessage,
		data:  b,
		t:     time.Now(),
	}
}

func isArrayBufferView(rt *sobek.Runtime, v sobek.Value) (bool, error) {
	var isView sobek.Callable
	var ok bool
	exc := rt.Try(func() {
		isView, ok = sobek.AssertFunction(
			rt.Get("ArrayBuffer").ToObject(rt).Get("isView"))
	})
	if exc != nil {
		return false, exc
	}

	if !ok {
		return false, fmt.Errorf("couldn't get ArrayBuffer.isView as it isn't a function")
	}

	boolValue, err := isView(nil, v)
	if err != nil {
		return false, err
	}
	return boolValue.ToBoolean(), nil
}

// Ping sends a ping message over the websocket.
func (w *webSocket) ping() {
	w.assertStateOpen()

	pingID := strconv.Itoa(w.sendPings.counter)

	w.writeQueueCh <- message{
		mtype: websocket.PingMessage,
		data:  []byte(pingID),
		t:     time.Now(),
	}

	w.sendPings.timestamps[pingID] = time.Now()
	w.sendPings.counter++
}

func (w *webSocket) trackPong(pingID string) {
	pongTimestamp := time.Now()

	pingTimestamp, ok := w.sendPings.timestamps[pingID]
	if !ok {
		// We received a pong for a ping we didn't send; ignore
		// (this shouldn't happen with a compliant server)
		w.vu.State().Logger.Warnf("received pong for unknown ping ID %s", pingID)

		return
	}

	metrics.PushIfNotDone(w.vu.Context(), w.vu.State().Samples, metrics.Sample{
		TimeSeries: metrics.TimeSeries{
			Metric: w.builtinMetrics.WSPing,
			Tags:   w.tagsAndMeta.Tags,
		},
		Time:     pongTimestamp,
		Metadata: w.tagsAndMeta.Metadata,
		Value:    metrics.D(pongTimestamp.Sub(pingTimestamp)),
	})
}

// assertStateOpen checks if the websocket is in the OPEN state
// otherwise it throws an error (panic)
func (w *webSocket) assertStateOpen() {
	if w.readyState == OPEN {
		return
	}

	// TODO figure out if we should give different error while being closed/closed/connecting
	common.Throw(w.vu.Runtime(), errors.New("InvalidStateError"))
}

// TODO support code and reason
func (w *webSocket) close(code int, reason string) {
	if w.readyState == CLOSED || w.readyState == CLOSING {
		return
	}
	w.readyState = CLOSING
	if code == 0 {
		code = websocket.CloseNormalClosure
	}
	w.writeQueueCh <- message{
		mtype: websocket.CloseMessage,
		data:  websocket.FormatCloseMessage(code, reason),
		t:     time.Now(),
	}
}

func (w *webSocket) queueClose() {
	w.tq.Queue(func() error {
		w.close(websocket.CloseNormalClosure, "")
		return nil
	})
}

// to be run only on the eventloop
// from https://websockets.spec.whatwg.org/#feedback-from-the-protocol
func (w *webSocket) connectionConnected() error {
	if w.readyState != CONNECTING {
		return nil
	}
	w.readyState = OPEN
	return w.callOpenListeners(time.Now()) // TODO fix time
}

// to be run only on the eventloop
func (w *webSocket) connectionClosedWithError(err error) error {
	if w.readyState == CLOSED {
		return nil
	}
	w.readyState = CLOSED
	close(w.done)

	if err != nil {
		if errList := w.callErrorListeners(err); errList != nil {
			return errList // TODO ... still call the close listeners ?!?
		}
	}
	return w.callEventListeners(events.CLOSE)
}

// newEvent return an event implementing "implements" https://dom.spec.whatwg.org/#event
// needs to be called on the event loop
// TODO: move to events
func (w *webSocket) newEvent(eventType string, t time.Time) *sobek.Object {
	rt := w.vu.Runtime()
	o := rt.NewObject()

	must(rt, o.DefineAccessorProperty("type", rt.ToValue(func() string {
		return eventType
	}), nil, sobek.FLAG_FALSE, sobek.FLAG_TRUE))
	must(rt, o.DefineAccessorProperty("target", rt.ToValue(func() interface{} {
		return w.obj
	}), nil, sobek.FLAG_FALSE, sobek.FLAG_TRUE))
	// skip srcElement
	// skip currentTarget ??!!
	// skip eventPhase ??!!
	// skip stopPropagation
	// skip cancelBubble
	// skip stopImmediatePropagation
	// skip a bunch more

	must(rt, o.DefineAccessorProperty("timestamp", rt.ToValue(func() float64 {
		return float64(t.UnixNano()) / 1_000_000 // milliseconds as double as per the spec
		// https://w3c.github.io/hr-time/#dom-domhighrestimestamp
	}), nil, sobek.FLAG_FALSE, sobek.FLAG_TRUE))

	return o
}

func (w *webSocket) callOpenListeners(timestamp time.Time) error {
	for _, openListener := range w.eventListeners.all(events.OPEN) {
		if _, err := openListener(w.newEvent(events.OPEN, timestamp)); err != nil {
			_ = w.conn.Close()                   // TODO log it?
			_ = w.connectionClosedWithError(err) // TODO log it?
			return err
		}
	}
	return nil
}

func (w *webSocket) callErrorListeners(e error) error { // TODO use the error even thought it is not by the spec
	rt := w.vu.Runtime()

	ev := w.newEvent(events.ERROR, time.Now())
	must(rt, ev.DefineDataProperty("error",
		rt.ToValue(e.Error()),
		sobek.FLAG_FALSE, sobek.FLAG_FALSE, sobek.FLAG_TRUE))
	for _, errorListener := range w.eventListeners.all(events.ERROR) {
		if _, err := errorListener(ev); err != nil { // TODO fix timestamp
			return err
		}
	}
	return nil
}

func (w *webSocket) callEventListeners(eventType string) error {
	for _, listener := range w.eventListeners.all(eventType) {
		// TODO the event here needs to be different and have an error (figure out it was for the close listeners)
		if _, err := listener(w.newEvent(eventType, time.Now())); err != nil { // TODO fix timestamp
			return err
		}
	}
	return nil
}

func (w *webSocket) addEventListener(event string, handler func(sobek.Value) (sobek.Value, error)) {
	// TODO support options https://developer.mozilla.org/en-US/docs/Web/API/EventTarget/addEventListener#parameters

	if handler == nil {
		common.Throw(w.vu.Runtime(), fmt.Errorf("handler for event type %q isn't a callable function", event))
	}

	if err := w.eventListeners.add(event, handler); err != nil {
		w.vu.State().Logger.Warnf("can't add event handler: %s", err)
	}
}

// TODO add remove listeners
