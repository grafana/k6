// Package ws implements a k6/ws for k6. It provides basic functionality to communicate over websockets
// that *blocks* the event loop while the connection is opened.
package ws

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/grafana/sobek"

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
	httpModule "go.k6.io/k6/js/modules/k6/http"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
)

type (
	// RootModule is the global module instance that will create module
	// instances for each VU.
	RootModule struct{}

	// WS represents a module instance of the WebSocket module.
	WS struct {
		vu  modules.VU
		obj *sobek.Object
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

// Socket is the representation of the websocket returned to the js.
type Socket struct {
	rt            *sobek.Runtime
	ctx           context.Context //nolint:containedctx
	conn          *websocket.Conn
	eventHandlers map[string][]sobek.Callable
	scheduled     chan sobek.Callable
	done          chan struct{}
	shutdownOnce  sync.Once

	pingSendTimestamps map[string]time.Time
	pingSendCounter    int

	tagsAndMeta    *metrics.TagsAndMeta
	samplesOutput  chan<- metrics.SampleContainer
	builtinMetrics *metrics.BuiltinMetrics
}

// HTTPResponse is the http response returned by ws.connect.
type HTTPResponse struct {
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

type wsConnectArgs struct {
	setupFn           sobek.Callable
	headers           http.Header
	enableCompression bool
	cookieJar         *cookiejar.Jar
	tagsAndMeta       *metrics.TagsAndMeta
}

const writeWait = 10 * time.Second

// Exports returns the exports of the ws module.
func (mi *WS) Exports() modules.Exports {
	return modules.Exports{Default: mi.obj}
}

// Connect establishes a WebSocket connection based on the parameters provided.
//
//nolint:funlen
func (mi *WS) Connect(url string, args ...sobek.Value) (*HTTPResponse, error) {
	ctx := mi.vu.Context()
	rt := mi.vu.Runtime()
	state := mi.vu.State()
	if state == nil {
		return nil, ErrWSInInitContext
	}

	parsedArgs, err := parseConnectArgs(state, rt, args...)
	if err != nil {
		return nil, err
	}

	parsedArgs.tagsAndMeta.SetSystemTagOrMetaIfEnabled(state.Options.SystemTags, metrics.TagURL, url)

	socket, httpResponse, connEndHook, err := mi.dial(ctx, state, rt, url, parsedArgs)
	defer connEndHook()
	if err != nil {
		// Pass the error to the user script before exiting immediately
		socket.handleEvent("error", rt.ToValue(err))
		if state.Options.Throw.Bool {
			return nil, err
		}
		if httpResponse != nil {
			return wrapHTTPResponse(httpResponse)
		}
		return &HTTPResponse{Error: err.Error()}, nil
	}

	defer socket.Close()

	// Run the user-provided set up function
	if _, err := parsedArgs.setupFn(sobek.Undefined(), rt.ToValue(&socket)); err != nil {
		_ = socket.closeConnection(websocket.CloseGoingAway)
		return nil, err
	}
	wsResponse, wsRespErr := wrapHTTPResponse(httpResponse)
	if wsRespErr != nil {
		return nil, wsRespErr
	}
	wsResponse.URL = url

	// The connection is now open, emit the event
	socket.handleEvent("open")

	// Make the default close handler a noop to avoid duplicate closes,
	// since we use custom closing logic to call user's event
	// handlers and for cleanup. See closeConnection.
	// closeConnection is not set directly as a handler here to
	// avoid race conditions when calling the Sobek runtime.
	socket.conn.SetCloseHandler(func(_ int, _ string) error { return nil })

	// Pass ping/pong events through the main control loop
	pingChan := make(chan string)
	pongChan := make(chan string)
	socket.conn.SetPingHandler(func(msg string) error { pingChan <- msg; return nil })
	socket.conn.SetPongHandler(func(pingID string) error { pongChan <- pingID; return nil })

	readDataChan := make(chan *message)
	readCloseChan := make(chan int)
	readErrChan := make(chan error)

	// Wraps a couple of channels around conn.ReadMessage
	go socket.readPump(readDataChan, readErrChan, readCloseChan)

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
				TimeSeries: metrics.TimeSeries{
					Metric: socket.builtinMetrics.WSMessagesReceived,
					Tags:   socket.tagsAndMeta.Tags,
				},
				Time:     time.Now(),
				Metadata: socket.tagsAndMeta.Metadata,
				Value:    1,
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
			if _, err := scheduledFn(sobek.Undefined()); err != nil {
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

func (mi *WS) dial(
	ctx context.Context, state *lib.State, rt *sobek.Runtime, url string,
	args *wsConnectArgs,
) (*Socket, *http.Response, func(), error) {
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
		EnableCompression: args.enableCompression,
	}
	// this is needed because of how interfaces work and that wsd.Jar is http.Cookiejar
	if args.cookieJar != nil {
		wsd.Jar = args.cookieJar
	}

	connStart := time.Now()
	conn, httpResponse, dialErr := wsd.DialContext(ctx, url, args.headers)
	connEnd := time.Now()

	if state.Options.SystemTags.Has(metrics.TagIP) && conn.RemoteAddr() != nil {
		if ip, _, err := net.SplitHostPort(conn.RemoteAddr().String()); err == nil {
			args.tagsAndMeta.SetSystemTagOrMeta(metrics.TagIP, ip)
		}
	}

	if httpResponse != nil {
		if state.Options.SystemTags.Has(metrics.TagStatus) {
			args.tagsAndMeta.SetSystemTagOrMeta(
				metrics.TagStatus, strconv.Itoa(httpResponse.StatusCode))
		}

		if state.Options.SystemTags.Has(metrics.TagSubproto) {
			args.tagsAndMeta.SetSystemTagOrMeta(
				metrics.TagSubproto, httpResponse.Header.Get("Sec-WebSocket-Protocol"))
		}
	}

	socket := Socket{
		ctx:                ctx,
		rt:                 rt,
		conn:               conn,
		eventHandlers:      make(map[string][]sobek.Callable),
		pingSendTimestamps: make(map[string]time.Time),
		scheduled:          make(chan sobek.Callable),
		done:               make(chan struct{}),
		samplesOutput:      state.Samples,
		tagsAndMeta:        args.tagsAndMeta,
		builtinMetrics:     state.BuiltinMetrics,
	}

	connEndHook := socket.pushSessionMetrics(connStart, connEnd)

	return &socket, httpResponse, connEndHook, dialErr
}

// On is used to configure what the websocket should do on each event.
func (s *Socket) On(event string, handler sobek.Value) {
	if handler, ok := sobek.AssertFunction(handler); ok {
		s.eventHandlers[event] = append(s.eventHandlers[event], handler)
	}
}

func (s *Socket) handleEvent(event string, args ...sobek.Value) {
	if handlers, ok := s.eventHandlers[event]; ok {
		for _, handler := range handlers {
			if _, err := handler(sobek.Undefined(), args...); err != nil {
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
		TimeSeries: metrics.TimeSeries{
			Metric: s.builtinMetrics.WSMessagesSent,
			Tags:   s.tagsAndMeta.Tags,
		},
		Time:     time.Now(),
		Metadata: s.tagsAndMeta.Metadata,
		Value:    1,
	})
}

// SendBinary writes the given ArrayBuffer message to the connection.
func (s *Socket) SendBinary(message sobek.Value) {
	if message == nil {
		common.Throw(s.rt, errors.New("missing argument, expected ArrayBuffer"))
	}

	msg := message.Export()
	if ab, ok := msg.(sobek.ArrayBuffer); ok {
		if err := s.conn.WriteMessage(websocket.BinaryMessage, ab.Bytes()); err != nil {
			s.handleEvent("error", s.rt.ToValue(err))
		}
	} else {
		var jsType string
		switch {
		case sobek.IsNull(message), sobek.IsUndefined(message):
			jsType = message.String()
		default:
			jsType = message.ToObject(s.rt).Get("constructor").ToObject(s.rt).Get("name").String()
		}
		common.Throw(s.rt, fmt.Errorf("expected ArrayBuffer as argument, received: %s", jsType))
	}

	metrics.PushIfNotDone(s.ctx, s.samplesOutput, metrics.Sample{
		TimeSeries: metrics.TimeSeries{
			Metric: s.builtinMetrics.WSMessagesSent,
			Tags:   s.tagsAndMeta.Tags,
		},
		Time:     time.Now(),
		Metadata: s.tagsAndMeta.Metadata,
		Value:    1,
	})
}

// Ping sends a ping message over the websocket.
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
		TimeSeries: metrics.TimeSeries{
			Metric: s.builtinMetrics.WSPing,
			Tags:   s.tagsAndMeta.Tags,
		},
		Time:     pongTimestamp,
		Metadata: s.tagsAndMeta.Metadata,
		Value:    metrics.D(pongTimestamp.Sub(pingTimestamp)),
	})
}

// SetTimeout executes the provided function inside the socket's event loop after at least the provided
// timeout, which is in ms, has elapsed
func (s *Socket) SetTimeout(fn sobek.Callable, timeoutMs float64) error {
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
func (s *Socket) SetInterval(fn sobek.Callable, intervalMs float64) error {
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
		ticker := time.NewTicker(d)
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

// Close closes the webscocket. If providede the first argument will be used as the code for the close message.
func (s *Socket) Close(args ...sobek.Value) {
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

func (s *Socket) pushSessionMetrics(connStart, connEnd time.Time) func() {
	connDuration := metrics.D(connEnd.Sub(connStart))

	metrics.PushIfNotDone(s.ctx, s.samplesOutput, metrics.ConnectedSamples{
		Samples: []metrics.Sample{
			{
				TimeSeries: metrics.TimeSeries{
					Metric: s.builtinMetrics.WSSessions,
					Tags:   s.tagsAndMeta.Tags,
				},
				Time:     connStart,
				Metadata: s.tagsAndMeta.Metadata,
				Value:    1,
			},
			{
				TimeSeries: metrics.TimeSeries{
					Metric: s.builtinMetrics.WSConnecting,
					Tags:   s.tagsAndMeta.Tags,
				},
				Time:     connStart,
				Metadata: s.tagsAndMeta.Metadata,
				Value:    connDuration,
			},
		},
		Tags: s.tagsAndMeta.Tags,
		Time: connStart,
	})

	return func() {
		end := time.Now()
		sessionDuration := metrics.D(end.Sub(connStart))

		metrics.PushIfNotDone(s.ctx, s.samplesOutput, metrics.Sample{
			TimeSeries: metrics.TimeSeries{
				Metric: s.builtinMetrics.WSSessionDuration,
				Tags:   s.tagsAndMeta.Tags,
			},
			Time:     connStart,
			Metadata: s.tagsAndMeta.Metadata,
			Value:    sessionDuration,
		})
	}
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
			e := new(websocket.CloseError)

			if errors.As(err, &e) {
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
func wrapHTTPResponse(httpResponse *http.Response) (*HTTPResponse, error) {
	wsResponse := HTTPResponse{
		Status: httpResponse.StatusCode,
	}

	body, err := io.ReadAll(httpResponse.Body)
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

//nolint:gocognit
func parseConnectArgs(state *lib.State, rt *sobek.Runtime, args ...sobek.Value) (*wsConnectArgs, error) {
	// The params argument is optional
	var callableV, paramsV sobek.Value
	switch len(args) {
	case 2:
		paramsV = args[0]
		callableV = args[1]
	case 1:
		paramsV = sobek.Undefined()
		callableV = args[0]
	default:
		return nil, errors.New("invalid number of arguments to ws.connect")
	}
	// Get the callable (required)
	setupFn, isFunc := sobek.AssertFunction(callableV)
	if !isFunc {
		return nil, errors.New("last argument to ws.connect must be a function")
	}

	headers := make(http.Header)
	headers.Set("User-Agent", state.Options.UserAgent.String)
	tagsAndMeta := state.Tags.GetCurrentValues()
	parsedArgs := &wsConnectArgs{
		setupFn:     setupFn,
		headers:     headers,
		cookieJar:   state.CookieJar,
		tagsAndMeta: &tagsAndMeta,
	}

	if sobek.IsUndefined(paramsV) || sobek.IsNull(paramsV) {
		return parsedArgs, nil
	}

	// Parse the optional second argument (params)
	params := paramsV.ToObject(rt)
	for _, k := range params.Keys() {
		switch k {
		case "headers":
			headersV := params.Get(k)
			if sobek.IsUndefined(headersV) || sobek.IsNull(headersV) {
				continue
			}
			headersObj := headersV.ToObject(rt)
			if headersObj == nil {
				continue
			}
			for _, key := range headersObj.Keys() {
				parsedArgs.headers.Set(key, headersObj.Get(key).String())
			}
		case "tags":
			if err := common.ApplyCustomUserTags(rt, parsedArgs.tagsAndMeta, params.Get(k)); err != nil {
				return nil, fmt.Errorf("invalid ws.connect() metric tags: %w", err)
			}
		case "jar":
			jarV := params.Get(k)
			if sobek.IsUndefined(jarV) || sobek.IsNull(jarV) {
				continue
			}
			if v, ok := jarV.Export().(*httpModule.CookieJar); ok {
				parsedArgs.cookieJar = v.Jar
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

			parsedArgs.enableCompression = true
		}
	}

	return parsedArgs, nil
}
