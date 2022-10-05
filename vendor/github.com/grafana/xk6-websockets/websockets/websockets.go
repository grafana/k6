// Package websockets implements to some extend WebSockets API https://websockets.spec.whatwg.org
package websockets

import (
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
	tagsAndMeta    metrics.TagsAndMeta
	tq             *taskqueue.TaskQueue
	builtinMetrics *metrics.BuiltinMetrics
	obj            *goja.Object // the object that is given to js to interact with the WebSocket
	started        time.Time

	done         chan struct{}
	writeQueueCh chan message
	// listeners
	// this return goja.value *and* error in order to return error on exception instead of panic
	// https://pkg.go.dev/github.com/dop251/goja#hdr-Functions
	openListeners    []func(goja.Value) (goja.Value, error)
	messageListeners []func(goja.Value) (goja.Value, error)
	errorListeners   []func(goja.Value) (goja.Value, error)
	closeListeners   []func(goja.Value) (goja.Value, error)

	// fields that should be seen by js only be updated on the event loop
	readyState     ReadyState
	bufferedAmount int
}

func (r *WebSocketsAPI) websocket(c goja.ConstructorCall) *goja.Object {
	urlValue := c.Argument(0)
	rt := r.vu.Runtime()
	if urlValue == nil || goja.IsUndefined(urlValue) {
		common.Throw(rt, errors.New("WebSocket requires a url"))
	}
	urlString := urlValue.String()
	url, err := url.Parse(urlString)
	if err != nil {
		common.Throw(rt, fmt.Errorf("WebSocket requires valid url, but got %q which resulted in %w", urlString, err))
	}
	if url.Scheme != "ws" && url.Scheme != "wss" {
		common.Throw(rt, fmt.Errorf("WebSocket requires url with scheme ws or wss, but got %q", url.Scheme))
	}
	if url.Fragment != "" {
		common.Throw(rt, fmt.Errorf("WebSocket requires no url fragment, but got %q", url.Fragment))
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
		obj:            rt.NewObject(),
	}

	// Maybe have this after the goroutine below ?!?

	must := func(err error) {
		if err != nil {
			common.Throw(rt, err)
		}
	}
	must(w.obj.DefineDataProperty(
		"addEventListener", rt.ToValue(w.addEventListener), goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_TRUE))
	// TODO add onmessage,onclose and so on.
	must(w.obj.DefineDataProperty(
		"send", rt.ToValue(w.send), goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_TRUE))
	must(w.obj.DefineDataProperty(
		"close", rt.ToValue(w.close), goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_TRUE))
	must(w.obj.DefineDataProperty(
		"url", rt.ToValue(urlString), goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_TRUE))
	must(w.obj.DefineAccessorProperty( // this needs to be with an accessor as we change the value
		"readyState", rt.ToValue(func() ReadyState {
			return w.readyState
		}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE))
	must(w.obj.DefineDataProperty(
		"bufferedAmount", rt.ToValue(w.bufferedAmount), goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_TRUE))
	// extensions
	// protocol
	must(w.obj.DefineAccessorProperty(
		"binaryType", rt.ToValue(func() goja.Value {
			return rt.ToValue("ArrayBuffer")
		}), rt.ToValue(func() goja.Value {
			common.Throw(rt, errors.New("binaryType is not settable in k6 as it doesn't support Blob"))
			return nil // it never gets to here
		}), goja.FLAG_FALSE, goja.FLAG_TRUE))

	go w.establishConnection()
	return w.obj
}

type message struct {
	mtype int // message type consts as defined in gorilla/websocket/conn.go
	data  []byte
	t     time.Time
}

// documented https://websockets.spec.whatwg.org/#concept-websocket-establish
func (w *webSocket) establishConnection() {
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
		NetDialContext:  state.Dialer.DialContext,
		Proxy:           http.ProxyFromEnvironment,
		TLSClientConfig: tlsConfig,
		// EnableCompression: enableCompression,
		// Jar:               jar,
	}
	// TODO figure out cookie jar given the specification
	header := make(http.Header)
	header.Set("User-Agent", state.Options.UserAgent.String)
	ctx := w.vu.Context()
	start := time.Now()
	conn, httpResponse, connErr := wsd.DialContext(ctx, w.url.String(), header)
	connectionEnd := time.Now()
	connectionDuration := metrics.D(connectionEnd.Sub(start))
	ctm := state.Tags.GetCurrentValues()
	if state.Options.SystemTags.Has(metrics.TagIP) && conn.RemoteAddr() != nil {
		if ip, _, err := net.SplitHostPort(conn.RemoteAddr().String()); err == nil {
			ctm.SetTag("ip", ip)
		}
	}

	if httpResponse != nil {
		defer func() {
			_ = httpResponse.Body.Close()
		}()
		if state.Options.SystemTags.Has(metrics.TagStatus) {
			ctm.SetTag("status", strconv.Itoa(httpResponse.StatusCode))
		}
		if state.Options.SystemTags.Has(metrics.TagSubproto) {
			ctm.SetTag("subproto", httpResponse.Header.Get("Sec-WebSocket-Protocol"))
		}
	}
	w.conn = conn
	if state.Options.SystemTags.Has(metrics.TagURL) {
		ctm.SetTag("url", w.url.String())
	}
	w.tagsAndMeta = ctm

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
				Value:      connectionDuration,
			},
		},
		Tags: w.tagsAndMeta.Tags,
		Time: start,
	})
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

//nolint:funlen,gocognit,cyclop
func (w *webSocket) loop() {
	readDataChan := make(chan *message)
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
					err := w.conn.WriteMessage(msg.mtype, msg.data)
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
		/* FIX this later
		case pingData := <-pingChan:
			// Handle pings received from the server
			// - trigger the `ping` event
			// - reply with pong (needed when `SetPingHandler` is overwritten)
			err := socket.conn.WriteControl(websocket.PongMessage, []byte(pingData), time.Now().Add(writeWait))
			if err != nil {
				socket.handleEvent("error", rt.ToValue(err))
			}
			socket.handleEvent("ping")

		case pingID := <-pongChan:
			// Handle pong responses to our pings
			socket.trackPong(pingID)
			socket.handleEvent("pong")

		*/
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
				ev := w.newEvent("message", msg.t)
				must := func(err error) {
					if err != nil {
						common.Throw(rt, err)
					}
				}
				if msg.mtype == websocket.BinaryMessage {
					// TODO this technically could be BLOB , but we don't support that
					ab := rt.NewArrayBuffer(msg.data)
					must(ev.DefineDataProperty("data", rt.ToValue(ab), goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_TRUE))
				} else {
					must(ev.DefineDataProperty("data", rt.ToValue(string(msg.data)), goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_TRUE))
				}
				must(ev.DefineDataProperty("origin", rt.ToValue(w.url.String()), goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_TRUE))

				// fmt.Println("messagelisteners", len(w.messageListeners))
				for _, messageListener := range w.messageListeners {
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
	if w.readyState != OPEN {
		// TODO figure out if we should give different error while being closed/closed/connecting
		common.Throw(w.vu.Runtime(), errors.New("InvalidStateError"))
	}
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
	return w.callCloseListeners()
}

// newEvent return an event implementing "implements" https://dom.spec.whatwg.org/#event
// needs to be called on the event loop
func (w *webSocket) newEvent(eventType string, t time.Time) *goja.Object {
	rt := w.vu.Runtime()
	o := rt.NewObject()
	must := func(err error) {
		if err != nil {
			common.Throw(rt, err)
		}
	}
	must(o.DefineAccessorProperty("type", rt.ToValue(func() string {
		return eventType
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE))
	must(o.DefineAccessorProperty("target", rt.ToValue(func() interface{} {
		return w.obj
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE))
	// skip srcElement
	// skip currentTarget ??!!
	// skip eventPhase ??!!
	// skip stopPropagation
	// skip cancelBubble
	// skip stopImmediatePropagation
	// skip a bunch more

	must(o.DefineAccessorProperty("timestamp", rt.ToValue(func() float64 {
		return float64(t.UnixNano()) / 1_000_000 // milliseconds as double as per the spec
		// https://w3c.github.io/hr-time/#dom-domhighrestimestamp
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE))

	return o
}

func (w *webSocket) callOpenListeners(timestamp time.Time) error {
	for _, openListener := range w.openListeners {
		if _, err := openListener(w.newEvent("open", timestamp)); err != nil {
			_ = w.conn.Close()                   // TODO log it?
			_ = w.connectionClosedWithError(err) // TODO log it?
			return err
		}
	}
	return nil
}

func (w *webSocket) callErrorListeners(e error) error { // TODO use the error even thought it is not by the spec
	rt := w.vu.Runtime()
	must := func(err error) {
		if err != nil {
			common.Throw(rt, err)
		}
	}
	ev := w.newEvent("error", time.Now())
	must(ev.DefineDataProperty("error",
		rt.ToValue(e),
		goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_TRUE))
	for _, errorListener := range w.errorListeners {
		if _, err := errorListener(ev); err != nil { // TODO fix timestamp
			return err
		}
	}
	return nil
}

func (w *webSocket) callCloseListeners() error {
	for _, closeListener := range w.closeListeners {
		// TODO the event here needs to be different and have an error
		if _, err := closeListener(w.newEvent("close", time.Now())); err != nil { // TODO fix timestamp
			return err
		}
	}
	return nil
}

func (w *webSocket) addEventListener(event string, listener func(goja.Value) (goja.Value, error)) {
	// TODO support options https://developer.mozilla.org/en-US/docs/Web/API/EventTarget/addEventListener#parameters
	// TODO implement `onerror` and co as well
	switch event {
	case "open":
		w.openListeners = append(w.openListeners, listener)
	case "error":
		w.errorListeners = append(w.errorListeners, listener)
	case "message":
		// fmt.Println("!!!!!!!!!!!!! message added!!!!!!")
		w.messageListeners = append(w.messageListeners, listener)
	case "close":
		w.closeListeners = append(w.closeListeners, listener)
	default:
		w.vu.State().Logger.Warnf("Unknown event for websocket %s", event)
	}
}

// TODO add remove listeners
