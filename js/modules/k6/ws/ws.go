package ws

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"
	"github.com/gorilla/websocket"

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
	httpModule "go.k6.io/k6/js/modules/k6/http"
	"go.k6.io/k6/metrics"
)

type (
	// RootModule is the global module instance that will create module
	// instances for each VU.
	RootModule struct{}

	// WS represents a module instance of the WebSocket module.
	WS struct {
		vu  modules.VU
		obj *goja.Object
	}
)

var (
	_ modules.Module   = &RootModule{}
	_ modules.Instance = &WS{}
)

// New returns a pointer to a new RootModule instance.
func New() *RootModule {
	return &RootModule{}
}

// NewModuleInstance implements the modules.Module interface to return
// a new instance for each VU.
func (*RootModule) NewModuleInstance(m modules.VU) modules.Instance {
	rt := m.Runtime()
	mi := &WS{
		vu: m,
	}
	obj := rt.NewObject()
	if err := obj.Set("connect", mi.Connect); err != nil {
		common.Throw(rt, err)
	}

	mi.obj = obj
	return mi
}

// ErrWSInInitContext is returned when websockets are using in the init context
var ErrWSInInitContext = common.NewInitContextError("using websockets in the init context is not supported")

type Socket struct {
	rt            *goja.Runtime
	ctx           context.Context
	conn          *websocket.Conn
	eventHandlers map[string][]goja.Callable
	scheduled     chan goja.Callable
	done          chan struct{}
	shutdownOnce  sync.Once

	pingSendTimestamps map[string]time.Time
	pingSendCounter    int

	sampleTags     *metrics.SampleTags
	samplesOutput  chan<- metrics.SampleContainer
	builtinMetrics *metrics.BuiltinMetrics
}

type WSHTTPResponse struct {
	URL     string            `json:"url"`
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
	Error   string            `json:"error"`
}

type message struct {
	mtype int // message type consts as defined in gorilla/websocket/conn.go
	data  []byte
}

const writeWait = 10 * time.Second

// Exports returns the exports of the ws module.
func (mi *WS) Exports() modules.Exports {
	return modules.Exports{Default: mi.obj}
}

// Connect establishes a WebSocket connection based on the parameters provided.
// TODO: refactor to reduce the method complexity
//nolint:funlen,gocognit,gocyclo
func (mi *WS) Connect(url string, args ...goja.Value) (*WSHTTPResponse, error) {
	ctx := mi.vu.Context()
	rt := mi.vu.Runtime()
	state := mi.vu.State()
	if state == nil {
		return nil, ErrWSInInitContext
	}

	// The params argument is optional
	var callableV, paramsV goja.Value
	switch len(args) {
	case 2:
		paramsV = args[0]
		callableV = args[1]
	case 1:
		paramsV = goja.Undefined()
		callableV = args[0]
	default:
		return nil, errors.New("invalid number of arguments to ws.connect")
	}
	// Get the callable (required)
	setupFn, isFunc := goja.AssertFunction(callableV)
	if !isFunc {
		return nil, errors.New("last argument to ws.connect must be a function")
	}

	header := make(http.Header)
	header.Set("User-Agent", state.Options.UserAgent.String)

	enableCompression := false

	tags := state.CloneTags()
	jar := state.CookieJar

	// Parse the optional second argument (params)
	if !goja.IsUndefined(paramsV) && !goja.IsNull(paramsV) {
		params := paramsV.ToObject(rt)
		for _, k := range params.Keys() {
			switch k {
			case "headers":
				headersV := params.Get(k)
				if goja.IsUndefined(headersV) || goja.IsNull(headersV) {
					continue
				}
				headersObj := headersV.ToObject(rt)
				if headersObj == nil {
					continue
				}
				for _, key := range headersObj.Keys() {
					header.Set(key, headersObj.Get(key).String())
				}
			case "tags":
				tagsV := params.Get(k)
				if goja.IsUndefined(tagsV) || goja.IsNull(tagsV) {
					continue
				}
				tagObj := tagsV.ToObject(rt)
				if tagObj == nil {
					continue
				}
				for _, key := range tagObj.Keys() {
					tags[key] = tagObj.Get(key).String()
				}
			case "jar":
				jarV := params.Get(k)
				if goja.IsUndefined(jarV) || goja.IsNull(jarV) {
					continue
				}
				if v, ok := jarV.Export().(*httpModule.CookieJar); ok {
					jar = v.Jar
				}
			case "compression":
				// deflate compression algorithm is supported - as defined in RFC7692
				// compression here relies on the implementation in gorilla/websocket package, usage is
				// experimental and may result in decreased performance. package supports
				// only "no context takeover" scenario

				algoString := strings.TrimSpace(params.Get(k).ToString().String())
				if algoString == "" {
					continue
				}

				if algoString != "deflate" {
					return nil, fmt.Errorf("unsupported compression algorithm '%s', supported algorithm is 'deflate'", algoString)
				}

				enableCompression = true
			}
		}

	}

	if state.Options.SystemTags.Has(metrics.TagURL) {
		tags["url"] = url
	}

	// Overriding the NextProtos to avoid talking http2
	var tlsConfig *tls.Config
	if state.TLSConfig != nil {
		tlsConfig = state.TLSConfig.Clone()
		tlsConfig.NextProtos = []string{"http/1.1"}
	}

	wsd := websocket.Dialer{
		HandshakeTimeout: time.Second * 60, // TODO configurable
		// Pass a custom net.DialContext function to websocket.Dialer that will substitute
		// the underlying net.Conn with our own tracked netext.Conn
		NetDialContext:    state.Dialer.DialContext,
		Proxy:             http.ProxyFromEnvironment,
		TLSClientConfig:   tlsConfig,
		EnableCompression: enableCompression,
		Jar:               jar,
	}
	if jar == nil { // this is needed because of how interfaces work and that wsd.Jar is http.Cookiejar
		wsd.Jar = nil
	}

	start := time.Now()
	conn, httpResponse, connErr := wsd.DialContext(ctx, url, header)
	connectionEnd := time.Now()
	connectionDuration := metrics.D(connectionEnd.Sub(start))

	if state.Options.SystemTags.Has(metrics.TagIP) && conn.RemoteAddr() != nil {
		if ip, _, err := net.SplitHostPort(conn.RemoteAddr().String()); err == nil {
			tags["ip"] = ip
		}
	}

	if httpResponse != nil {
		if state.Options.SystemTags.Has(metrics.TagStatus) {
			tags["status"] = strconv.Itoa(httpResponse.StatusCode)
		}

		if state.Options.SystemTags.Has(metrics.TagSubproto) {
			tags["subproto"] = httpResponse.Header.Get("Sec-WebSocket-Protocol")
		}
	}

	socket := Socket{
		ctx:                ctx,
		rt:                 rt,
		conn:               conn,
		eventHandlers:      make(map[string][]goja.Callable),
		pingSendTimestamps: make(map[string]time.Time),
		scheduled:          make(chan goja.Callable),
		done:               make(chan struct{}),
		samplesOutput:      state.Samples,
		sampleTags:         metrics.IntoSampleTags(&tags),
		builtinMetrics:     state.BuiltinMetrics,
	}

	metrics.PushIfNotDone(ctx, state.Samples, metrics.ConnectedSamples{
		Samples: []metrics.Sample{
			{Metric: state.BuiltinMetrics.WSSessions, Time: start, Tags: socket.sampleTags, Value: 1},
			{Metric: state.BuiltinMetrics.WSConnecting, Time: start, Tags: socket.sampleTags, Value: connectionDuration},
		},
		Tags: socket.sampleTags,
		Time: start,
	})

	if connErr != nil {
		// Pass the error to the user script before exiting immediately
		socket.handleEvent("error", rt.ToValue(connErr))
		if state.Options.Throw.Bool {
			return nil, connErr
		}
		state.Logger.WithError(connErr).Warn("Attempt to establish a WebSocket connection failed")
		return &WSHTTPResponse{
			Error: connErr.Error(),
		}, nil
	}

	// Run the user-provided set up function
	if _, err := setupFn(goja.Undefined(), rt.ToValue(&socket)); err != nil {
		_ = socket.closeConnection(websocket.CloseGoingAway)
		return nil, err
	}
	wsResponse, wsRespErr := wrapHTTPResponse(httpResponse)
	if wsRespErr != nil {
		return nil, wsRespErr
	}
	wsResponse.URL = url

	defer func() { _ = conn.Close() }()

	// The connection is now open, emit the event
	socket.handleEvent("open")

	// Make the default close handler a noop to avoid duplicate closes,
	// since we use custom closing logic to call user's event
	// handlers and for cleanup. See closeConnection.
	// closeConnection is not set directly as a handler here to
	// avoid race conditions when calling the Goja runtime.
	conn.SetCloseHandler(func(code int, text string) error { return nil })

	// Pass ping/pong events through the main control loop
	pingChan := make(chan string)
	pongChan := make(chan string)
	conn.SetPingHandler(func(msg string) error { pingChan <- msg; return nil })
	conn.SetPongHandler(func(pingID string) error { pongChan <- pingID; return nil })

	readDataChan := make(chan *message)
	readCloseChan := make(chan int)
	readErrChan := make(chan error)

	// Wraps a couple of channels around conn.ReadMessage
	go socket.readPump(readDataChan, readErrChan, readCloseChan)

	// we do it here as below we can panic, which translates to an exception in js code
	defer func() {
		socket.Close() // just in case
		end := time.Now()
		sessionDuration := metrics.D(end.Sub(start))

		metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
			Metric: socket.builtinMetrics.WSSessionDuration,
			Tags:   socket.sampleTags,
			Time:   start,
			Value:  sessionDuration,
		})
	}()

	// This is the main control loop. All JS code (including error handlers)
	// should only be executed by this thread to avoid race conditions
	for {
		select {
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

		case msg := <-readDataChan:
			metrics.PushIfNotDone(ctx, socket.samplesOutput, metrics.Sample{
				Metric: socket.builtinMetrics.WSMessagesReceived,
				Time:   time.Now(),
				Tags:   socket.sampleTags,
				Value:  1,
			})

			if msg.mtype == websocket.BinaryMessage {
				ab := rt.NewArrayBuffer(msg.data)
				socket.handleEvent("binaryMessage", rt.ToValue(&ab))
			} else {
				socket.handleEvent("message", rt.ToValue(string(msg.data)))
			}

		case readErr := <-readErrChan:
			socket.handleEvent("error", rt.ToValue(readErr))

		case code := <-readCloseChan:
			_ = socket.closeConnection(code)

		case scheduledFn := <-socket.scheduled:
			if _, err := scheduledFn(goja.Undefined()); err != nil {
				_ = socket.closeConnection(websocket.CloseGoingAway)
				return nil, err
			}

		case <-ctx.Done():
			// VU is shutting down during an interrupt
			// socket events will not be forwarded to the VU
			_ = socket.closeConnection(websocket.CloseGoingAway)

		case <-socket.done:
			// This is the final exit point normally triggered by closeConnection
			return wsResponse, nil
		}
	}
}

func (s *Socket) On(event string, handler goja.Value) {
	if handler, ok := goja.AssertFunction(handler); ok {
		s.eventHandlers[event] = append(s.eventHandlers[event], handler)
	}
}

func (s *Socket) handleEvent(event string, args ...goja.Value) {
	if handlers, ok := s.eventHandlers[event]; ok {
		for _, handler := range handlers {
			if _, err := handler(goja.Undefined(), args...); err != nil {
				common.Throw(s.rt, err)
			}
		}
	}
}

// Send writes the given string message to the connection.
func (s *Socket) Send(message string) {
	if err := s.conn.WriteMessage(websocket.TextMessage, []byte(message)); err != nil {
		s.handleEvent("error", s.rt.ToValue(err))
	}

	metrics.PushIfNotDone(s.ctx, s.samplesOutput, metrics.Sample{
		Metric: s.builtinMetrics.WSMessagesSent,
		Time:   time.Now(),
		Tags:   s.sampleTags,
		Value:  1,
	})
}

// SendBinary writes the given ArrayBuffer message to the connection.
func (s *Socket) SendBinary(message goja.Value) {
	if message == nil {
		common.Throw(s.rt, errors.New("missing argument, expected ArrayBuffer"))
	}

	msg := message.Export()
	if ab, ok := msg.(goja.ArrayBuffer); ok {
		if err := s.conn.WriteMessage(websocket.BinaryMessage, ab.Bytes()); err != nil {
			s.handleEvent("error", s.rt.ToValue(err))
		}
	} else {
		var jsType string
		switch {
		case goja.IsNull(message), goja.IsUndefined(message):
			jsType = message.String()
		default:
			jsType = message.ToObject(s.rt).ClassName()
		}
		common.Throw(s.rt, fmt.Errorf("expected ArrayBuffer as argument, received: %s", jsType))
	}

	metrics.PushIfNotDone(s.ctx, s.samplesOutput, metrics.Sample{
		Metric: s.builtinMetrics.WSMessagesSent,
		Time:   time.Now(),
		Tags:   s.sampleTags,
		Value:  1,
	})
}

func (s *Socket) Ping() {
	deadline := time.Now().Add(writeWait)
	pingID := strconv.Itoa(s.pingSendCounter)
	data := []byte(pingID)

	err := s.conn.WriteControl(websocket.PingMessage, data, deadline)
	if err != nil {
		s.handleEvent("error", s.rt.ToValue(err))
		return
	}

	s.pingSendTimestamps[pingID] = time.Now()
	s.pingSendCounter++
}

func (s *Socket) trackPong(pingID string) {
	pongTimestamp := time.Now()

	if _, ok := s.pingSendTimestamps[pingID]; !ok {
		// We received a pong for a ping we didn't send; ignore
		// (this shouldn't happen with a compliant server)
		return
	}
	pingTimestamp := s.pingSendTimestamps[pingID]

	metrics.PushIfNotDone(s.ctx, s.samplesOutput, metrics.Sample{
		Metric: s.builtinMetrics.WSPing,
		Time:   pongTimestamp,
		Tags:   s.sampleTags,
		Value:  metrics.D(pongTimestamp.Sub(pingTimestamp)),
	})
}

// SetTimeout executes the provided function inside the socket's event loop after at least the provided
// timeout, which is in ms, has elapsed
func (s *Socket) SetTimeout(fn goja.Callable, timeoutMs float64) error {
	// Starts a goroutine, blocks once on the timeout and pushes the callable
	// back to the main loop through the scheduled channel.
	//
	// Intentionally not using the generic GetDurationValue() helper, since this
	// API is meant to use ms, similar to the original SetTimeout() JS API.
	d := time.Duration(timeoutMs * float64(time.Millisecond))
	if d <= 0 {
		return fmt.Errorf("setTimeout requires a >0 timeout parameter, received %.2f", timeoutMs)
	}
	go func() {
		select {
		case <-time.After(d):
			select {
			case s.scheduled <- fn:
			case <-s.done:
				return
			}

		case <-s.done:
			return
		}
	}()

	return nil
}

// SetInterval executes the provided function inside the socket's event loop each interval time, which is
// in ms
func (s *Socket) SetInterval(fn goja.Callable, intervalMs float64) error {
	// Starts a goroutine, blocks forever on the ticker and pushes the callable
	// back to the main loop through the scheduled channel.
	//
	// Intentionally not using the generic GetDurationValue() helper, since this
	// API is meant to use ms, similar to the original SetInterval() JS API.
	d := time.Duration(intervalMs * float64(time.Millisecond))
	if d <= 0 {
		return fmt.Errorf("setInterval requires a >0 timeout parameter, received %.2f", intervalMs)
	}
	go func() {
		ticker := time.NewTicker(time.Duration(intervalMs) * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				select {
				case s.scheduled <- fn:
				case <-s.done:
					return
				}

			case <-s.done:
				return
			}
		}
	}()

	return nil
}

func (s *Socket) Close(args ...goja.Value) {
	code := websocket.CloseGoingAway
	if len(args) > 0 {
		code = int(args[0].ToInteger())
	}

	_ = s.closeConnection(code)
}

// closeConnection cleanly closes the WebSocket connection.
// Returns an error if sending the close control frame fails.
func (s *Socket) closeConnection(code int) error {
	var err error

	s.shutdownOnce.Do(func() {
		// this is because handleEvent can panic ... on purpose so we just make sure we
		// close the connection and the channel
		defer func() {
			_ = s.conn.Close()

			// Stop the main control loop
			close(s.done)
		}()
		err = s.conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(code, ""),
			time.Now().Add(writeWait),
		)
		if err != nil {
			// Call the user-defined error handler
			s.handleEvent("error", s.rt.ToValue(err))
		}

		// Call the user-defined close handler
		s.handleEvent("close", s.rt.ToValue(code))
	})

	return err
}

// Wraps conn.ReadMessage in a channel
func (s *Socket) readPump(readChan chan *message, errorChan chan error, closeChan chan int) {
	for {
		messageType, data, err := s.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(
				err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				// Report an unexpected closure
				select {
				case errorChan <- err:
				case <-s.done:
					return
				}
			}
			code := websocket.CloseGoingAway
			if e, ok := err.(*websocket.CloseError); ok {
				code = e.Code
			}
			select {
			case closeChan <- code:
			case <-s.done:
			}
			return
		}

		select {
		case readChan <- &message{messageType, data}:
		case <-s.done:
			return
		}
	}
}

// Wrap the raw HTTPResponse we received to a WSHTTPResponse we can pass to the user
func wrapHTTPResponse(httpResponse *http.Response) (*WSHTTPResponse, error) {
	wsResponse := WSHTTPResponse{
		Status: httpResponse.StatusCode,
	}

	body, err := ioutil.ReadAll(httpResponse.Body)
	if err != nil {
		return nil, err
	}

	err = httpResponse.Body.Close()
	if err != nil {
		return nil, err
	}

	wsResponse.Body = string(body)

	wsResponse.Headers = make(map[string]string, len(httpResponse.Header))
	for k, vs := range httpResponse.Header {
		wsResponse.Headers[k] = strings.Join(vs, ", ")
	}

	return &wsResponse, nil
}
