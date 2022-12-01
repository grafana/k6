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
	"time"

	"github.com/dop251/goja"
	"github.com/gorilla/websocket"
	"github.com/grafana/xk6-websockets/websockets/events"
	"github.com/mstoykov/k6-taskqueue-lib/taskqueue"

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/metrics"
)

// RootModule is the root module for the websockets API
type RootModule struct{}

// WebSocketsAPI is the k6 extension implementing the websocket API as defined in https://websockets.spec.whatwg.org
type WebSocketsAPI struct { //nolint:revive
	vu modules.VU
}

var _ modules.Module = &RootModule{}

// NewModuleInstance returns a new instance of the module
func (r *RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	return &WebSocketsAPI{
		vu: vu,
	}
}

// Exports implements the modules.Instance interface's Exports
func (r *WebSocketsAPI) Exports() modules.Exports {
	return modules.Exports{
		Named: map[string]interface{}{
			"WebSocket": r.websocket,
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
	vu             modules.VU
	url            *url.URL
	conn           *websocket.Conn
	tagsAndMeta    *metrics.TagsAndMeta
	tq             *taskqueue.TaskQueue
	builtinMetrics *metrics.BuiltinMetrics
	obj            *goja.Object // the object that is given to js to interact with the WebSocket
	started        time.Time

	done         chan struct{}
	writeQueueCh chan message

	eventListeners *eventListeners

	sendPings ping

	// fields that should be seen by js only be updated on the event loop
	readyState     ReadyState
	bufferedAmount int
}

type ping struct {
	counter    int
	timestamps map[string]time.Time
}

func (r *WebSocketsAPI) websocket(c goja.ConstructorCall) *goja.Object {
	rt := r.vu.Runtime()

	url, err := parseURL(c.Argument(0))
	if err != nil {
		common.Throw(rt, err)
	}

	params, err := buildParams(r.vu.State(), rt, c.Argument(2))
	if err != nil {
		common.Throw(rt, err)
	}

	// TODO implement protocols
	registerCallback := func() func(func() error) {
		// fmt.Println("RegisterCallback called")
		callback := r.vu.RegisterCallback()
		return func(f func() error) {
			// fmt.Println("callback called")
			callback(f)
			// fmt.Println("callback ended")
		}
	}

	w := &webSocket{
		vu:             r.vu,
		url:            url,
		tq:             taskqueue.New(registerCallback),
		readyState:     CONNECTING,
		builtinMetrics: r.vu.State().BuiltinMetrics,
		done:           make(chan struct{}),
		writeQueueCh:   make(chan message, 10),
		eventListeners: newEventListeners(),
		obj:            rt.NewObject(),
		tagsAndMeta:    params.tagsAndMeta,
		sendPings: ping{
			timestamps: make(map[string]time.Time),
			counter:    0,
		},
	}

	// Maybe have this after the goroutine below ?!?
	defineWebsocket(rt, w)

	go w.establishConnection(params)
	return w.obj
}

// parseURL parses the url from the first constructor calls argument or returns an error
func parseURL(urlValue goja.Value) (*url.URL, error) {
	if urlValue == nil || goja.IsUndefined(urlValue) {
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

// defineWebsocket defines all properties and methods for the WebSocket
func defineWebsocket(rt *goja.Runtime, w *webSocket) {
	must(rt, w.obj.DefineDataProperty(
		"addEventListener", rt.ToValue(w.addEventListener), goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_TRUE))
	must(rt, w.obj.DefineDataProperty(
		"send", rt.ToValue(w.send), goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_TRUE))
	must(rt, w.obj.DefineDataProperty(
		"ping", rt.ToValue(w.ping), goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_TRUE))
	must(rt, w.obj.DefineDataProperty(
		"close", rt.ToValue(w.close), goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_TRUE))
	must(rt, w.obj.DefineDataProperty(
		"url", rt.ToValue(w.url.String()), goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_TRUE))
	must(rt, w.obj.DefineAccessorProperty( // this needs to be with an accessor as we change the value
		"readyState", rt.ToValue(func() ReadyState {
			return w.readyState
		}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE))
	must(rt, w.obj.DefineDataProperty(
		"bufferedAmount", rt.ToValue(w.bufferedAmount), goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_TRUE))
	// extensions
	// protocol
	must(rt, w.obj.DefineAccessorProperty(
		"binaryType", rt.ToValue(func() goja.Value {
			return rt.ToValue("ArrayBuffer")
		}), rt.ToValue(func() goja.Value {
			common.Throw(rt, errors.New("binaryType is not settable in k6 as it doesn't support Blob"))
			return nil // it never gets to here
		}), goja.FLAG_FALSE, goja.FLAG_TRUE))

	setOn := func(property string, el *eventListener) {
		if el == nil {
			// this is generally should not happen, but we're being defensive
			common.Throw(rt, fmt.Errorf("not supported on-handler '%s'", property))
		}

		must(rt, w.obj.DefineAccessorProperty(
			property, rt.ToValue(func() goja.Value {
				return rt.ToValue(el.getOn)
			}), rt.ToValue(func(call goja.FunctionCall) goja.Value {
				arg := call.Argument(0)

				// it's possible to unset handlers by setting them to null
				if arg == nil || goja.IsUndefined(arg) || goja.IsNull(arg) {
					el.setOn(nil)

					return nil
				}

				fn, isFunc := goja.AssertFunction(arg)
				if !isFunc {
					common.Throw(rt, fmt.Errorf("a value for '%s' should be callable", property))
				}

				el.setOn(func(v goja.Value) (goja.Value, error) { return fn(goja.Undefined(), v) })

				return nil
			}), goja.FLAG_FALSE, goja.FLAG_TRUE))
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
		subProtocol := httpResponse.Header.Get("Sec-WebSocket-Protocol")
		w.tagsAndMeta.SetSystemTagOrMetaIfEnabled(systemTags, metrics.TagSubproto, subProtocol)
	}
	w.conn = conn

	w.tagsAndMeta.SetSystemTagOrMetaIfEnabled(systemTags, metrics.TagURL, w.url.String())

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

//nolint:funlen,gocognit,cyclop
func (w *webSocket) loop() {
	readDataChan := make(chan *message)

	// Pass ping/pong events through the main control loop
	pingChan := make(chan string)
	pongChan := make(chan string)
	w.conn.SetPingHandler(func(msg string) error { pingChan <- msg; return nil })
	w.conn.SetPongHandler(func(pingID string) error { pongChan <- pingID; return nil })

	// readCloseChan := make(chan int)
	// readErrChan := make(chan error)
	samplesOutput := w.vu.State().Samples
	ctx := w.vu.Context()

	defer func() {
		now := time.Now()
		duration := metrics.D(time.Since(w.started))

		metrics.PushIfNotDone(ctx, w.vu.State().Samples, metrics.Sample{
			TimeSeries: metrics.TimeSeries{
				Metric: w.builtinMetrics.WSSessionDuration,
				Tags:   w.tagsAndMeta.Tags,
			},
			Time:     now,
			Metadata: w.tagsAndMeta.Metadata,
			Value:    duration,
		})
		ch := make(chan struct{})
		w.tq.Queue(func() error {
			defer close(ch)
			return w.connectionClosedWithError(nil)
		})
		select {
		case <-ch:
		case <-ctx.Done():
			// unfortunately it is really possible that k6 has been winding down the VU and the above code
			// to close the connection will just never be called as the event loop no longer executes the callbacks.
			// This ultimately needs a separate signal for when the eventloop will not execute anything after this.
			// To try to prevent leaking goroutines here we will try to figure out if we need to close the `done`
			// channel and wait a bit and close it if it isn't. This might be better off with more mutexes
			timer := time.NewTimer(time.Millisecond * 250)
			select {
			case <-w.done:
				// everything is fine
			case <-timer.C:
				close(w.done) // hopefully this means we won't double close
			}
			timer.Stop()
		}
		_ = w.conn.Close()
		w.tq.Close()
	}()
	// Wraps a couple of channels around conn.ReadMessage
	go func() { // copied from k6/ws
		defer close(readDataChan)
		for {
			messageType, data, err := w.conn.ReadMessage()
			if err != nil {
				if !websocket.IsUnexpectedCloseError(
					err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					return
				}
				/*
					code := websocket.CloseGoingAway
					if e, ok := err.(*websocket.CloseError); ok {
						code = e.Code
					}
					select {
					case readCloseChan <- code:
					case <-w.done:
					}
				*/
				w.tq.Queue(func() error {
					_ = w.conn.Close() // TODO fix this
					// fmt.Println("read message", err)
					err = w.connectionClosedWithError(err)
					return err
				})
				return
			}

			select {
			case readDataChan <- &message{
				mtype: messageType,
				data:  data,
				t:     time.Now(),
			}:
			case <-w.done:
				return
			}
		}
	}()

	go func() {
		writeChannel := make(chan message)
		go func() {
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
							// fmt.Println("write channel", err)
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
					select {
					case writeChannel <- msg:
					default:
						queue = append(queue, msg)
					}
				case wch <- msg:
					queue = queue[:copy(queue, queue[1:])]

				case <-w.done:
					return
				}
			}
		}
	}()
	ctxDone := ctx.Done()
	for {
		select {
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

		case msg, ok := <-readDataChan:
			if !ok {
				return
			}
			// fmt.Println("got message")
			w.tq.Queue(func() error {
				// fmt.Println("message being processed in state", w.readyState)
				if w.readyState != OPEN {
					return nil // TODO maybe still emit
				}
				// TODO maybe emit after all the listeners have fired and skip it if defaultPrevent was called?!?
				metrics.PushIfNotDone(ctx, samplesOutput, metrics.Sample{
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
					// TODO this technically could be BLOB , but we don't support that
					ab := rt.NewArrayBuffer(msg.data)
					must(rt, ev.DefineDataProperty("data", rt.ToValue(ab), goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_TRUE))
				} else {
					must(
						rt,
						ev.DefineDataProperty("data", rt.ToValue(string(msg.data)), goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_TRUE),
					)
				}
				must(
					rt,
					ev.DefineDataProperty("origin", rt.ToValue(w.url.String()), goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_TRUE),
				)

				for _, messageListener := range w.eventListeners.all(events.MESSAGE) {
					if _, err := messageListener(ev); err != nil {
						// fmt.Println("message listener err", err)
						_ = w.conn.Close()                   // TODO log it?
						_ = w.connectionClosedWithError(err) // TODO log it?
						return err
					}
				}
				return nil
			})

			/* TODO
			case readErr := <-readErrChan:
				 socket.handleEvent("error", rt.ToValue(readErr))

			case code := <-readCloseChan:
				_ = socket.closeConnection(code)

			case scheduledFn := <-socket.scheduled:
				if _, err := scheduledFn(goja.Undefined()); err != nil {
					_ = socket.closeConnection(websocket.CloseGoingAway)
					return nil, err
				}
			*/
		case <-ctxDone:
			// VU is shutting down during an interrupt
			// socket events will not be forwarded to the VU
			w.queueClose()
			ctxDone = nil // this is to block this branch and get through w.done

		case <-w.done:
			return
		}
	}
}

func (w *webSocket) send(msg goja.Value) {
	w.assertStateOpen()

	switch o := msg.Export().(type) {
	case string:
		w.bufferedAmount += len(o)
		w.writeQueueCh <- message{
			mtype: websocket.TextMessage,
			data:  []byte(o),
			t:     time.Now(),
		}
	case *goja.ArrayBuffer:
		b := o.Bytes()
		w.bufferedAmount += len(b)
		w.writeQueueCh <- message{
			mtype: websocket.BinaryMessage,
			data:  b,
			t:     time.Now(),
		}
	case goja.ArrayBuffer:
		b := o.Bytes()
		w.bufferedAmount += len(b)
		w.writeQueueCh <- message{
			mtype: websocket.BinaryMessage,
			data:  b,
			t:     time.Now(),
		}
	default:
		common.Throw(w.vu.Runtime(), fmt.Errorf("unsupported send type %T", o))
	}
}

// Ping sends a ping message over the websocket.
func (w *webSocket) ping() {
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
	// fmt.Println("in Close")
	w.writeQueueCh <- message{
		mtype: websocket.CloseMessage,
		data:  websocket.FormatCloseMessage(code, reason),
		t:     time.Now(),
	}
}

func (w *webSocket) queueClose() {
	w.tq.Queue(func() error {
		// fmt.Println("in close")
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
	// fmt.Println(w.url, "closing")
	w.readyState = CLOSED
	// fmt.Println("closing w.done")
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
func (w *webSocket) newEvent(eventType string, t time.Time) *goja.Object {
	rt := w.vu.Runtime()
	o := rt.NewObject()

	must(rt, o.DefineAccessorProperty("type", rt.ToValue(func() string {
		return eventType
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE))
	must(rt, o.DefineAccessorProperty("target", rt.ToValue(func() interface{} {
		return w.obj
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE))
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
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE))

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
		goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_TRUE))
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

func (w *webSocket) addEventListener(event string, listener func(goja.Value) (goja.Value, error)) {
	// TODO support options https://developer.mozilla.org/en-US/docs/Web/API/EventTarget/addEventListener#parameters
	if err := w.eventListeners.add(event, listener); err != nil {
		w.vu.State().Logger.Warnf("can't add event listener: %s", err)
	}
}

// TODO add remove listeners
