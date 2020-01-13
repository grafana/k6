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
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"reflect"
	"sync"
	"time"

	"github.com/dop251/goja"
	"github.com/gorilla/websocket"
	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/lib"
)

type WSIO struct{}

type SocketIO struct {
	rt                    *goja.Runtime
	ctx                   *context.Context
	callbackFunction      *goja.Callable
	socketOptions         *goja.Value
	requestHeaders        *http.Header
	state                 *lib.State
	conn                  *websocket.Conn
	eventHandlers         map[string][]goja.Callable
	scheduled             chan goja.Callable
	done                  chan struct{}
	shutdownOnce          sync.Once
	tags                  map[string]string
	msgSentTimestamps     []time.Time
	msgReceivedTimestamps []time.Time

	pingSendTimestamps map[string]time.Time
	pingSendCounter    int
	pingTimestamps     []pingDelta
}

const (
	SOCKET_PROTOCOL        = "ws://"
	SOCKET_SECURE_PROTOCOL = "wss://"
	SOCKET_IO_PATH         = "/socket.io/?EIO=3&transport=websocket"
)
const (
	OPEN    = "0"
	CLOSE   = "1"
	PING    = "2"
	PONG    = "3"
	MESSAGE = "4"
	UPGRADE = "5"
	NOOP    = "6"
)

func NewSocketIO() *WSIO {
	return &WSIO{}
}

func (*WSIO) Connect(ctx context.Context, url string, args ...goja.Value) (*WSHTTPResponse, error) {
	validateParamArguments(ctx, args...)
	// rt, state := initConnectState(ctx)
	socket := newWebSocketIO(ctx)
	socket.extractParams(args...)
	socket.configureSocketHeadersAndTags(socket.socketOptions)
	dialer := socket.createSocketIODialer()
	socket.configureSocketCookies(socket.socketOptions, &dialer)
	conn, httpResponse, connErr := dialer.Dial(url, socket.requestHeaders.Clone())
	fmt.Println(conn)
	fmt.Println(httpResponse)
	fmt.Println(connErr)
	(*socket.callbackFunction)(goja.Undefined(), socket.rt.ToValue(&socket))
	return nil, nil
}

func newWebSocketIO(initCtx context.Context) (socket SocketIO) {
	initRuntime, initState := initConnectState(initCtx)
	return SocketIO{
		rt:                 initRuntime,
		requestHeaders:     &http.Header{},
		ctx:                &initCtx,
		state:              initState,
		tags:               initState.Options.RunTags.CloneTags(),
		eventHandlers:      make(map[string][]goja.Callable),
		pingSendTimestamps: make(map[string]time.Time),
		scheduled:          make(chan goja.Callable),
		done:               make(chan struct{}),
	}
}

func initConnectState(ctx context.Context) (rt *goja.Runtime, state *lib.State) {
	rt = common.GetRuntime(ctx)
	state = lib.GetState(ctx)
	if state == nil {
		panic(common.NewInitContextError("using websockets in the init context is not supported"))
	}
	return
}

func (s *SocketIO) extractParams(args ...goja.Value) {
	var callFunc goja.Value
	for _, v := range args {
		argType := v.ExportType()
		switch argType.Kind() {
		case reflect.Map:
			s.socketOptions = &v
			break
		case reflect.Func:
			callFunc = v
			break
		default:
			common.Throw(common.GetRuntime(*s.ctx), errors.New("Invalid argument types. Allowing Map and Function types"))
		}
	}
	s.callbackFunction = validateCallableParam(s.ctx, &callFunc)
}

func validateParamArguments(ctx context.Context, args ...goja.Value) {
	switch len(args) {
	case 1, 2:
		return
	default:
		common.Throw(common.GetRuntime(ctx), errors.New("invalid number of arguments to ws.connect. Method is required 3 params ( url, params, callback )"))
	}
}

func validateCallableParam(ctx *context.Context, callableParam *goja.Value) (setupFn *goja.Callable) {
	callableFunc, isFunc := goja.AssertFunction(*callableParam)
	if !isFunc {
		common.Throw(common.GetRuntime(*ctx), errors.New("last argument to ws.connect must be a function"))
	}
	return &callableFunc
}

func (s *SocketIO) configureSocketHeadersAndTags(params *goja.Value) {
	if params == nil {
		return
	}
	paramsObject := (*params).ToObject(s.rt)
	for _, key := range paramsObject.Keys() {
		switch key {
		case "headers":
			s.setSocketHeaders(paramsObject, s.rt)
			break
		case "tags":
			s.setSocketTags(paramsObject, s.rt)
			break
		default:
			break
		}
	}
}

func (s *SocketIO) configureSocketCookies(params *goja.Value, dialer *websocket.Dialer) {
	if params == nil {
		return
	}
	paramsObject := (*params).ToObject(s.rt)
	for _, key := range paramsObject.Keys() {
		switch key {
		case "cookies":
			s.setSocketCookies(paramsObject, s.rt, dialer)
			break
		default:
			break
		}
	}
}

func (s *SocketIO) setSocketHeaders(paramsObject *goja.Object, rt *goja.Runtime) {
	headersV := paramsObject.Get("headers")
	if goja.IsUndefined(headersV) || goja.IsNull(headersV) {
		return
	}
	headersObj := headersV.ToObject(rt)
	for _, key := range headersObj.Keys() {
		s.requestHeaders.Set(key, headersObj.Get(key).String())
	}
}

func (s *SocketIO) setSocketCookies(paramsObject *goja.Object, rt *goja.Runtime, dialer *websocket.Dialer) {
	cookiesV := paramsObject.Get("cookies")
	fmt.Println(cookiesV)
}

func (s *SocketIO) setSocketTags(paramsObject *goja.Object, rt *goja.Runtime) {
	tagsV := paramsObject.Get("tags")
	if goja.IsUndefined(tagsV) || goja.IsNull(tagsV) {
		return
	}
	tagsObj := tagsV.ToObject(rt)
	for _, key := range tagsObj.Keys() {
		s.tags[key] = tagsObj.Get(key).String()
	}
}

func (s *SocketIO) createSocketIODialer() (dialer websocket.Dialer) {
	return websocket.Dialer{
		NetDial:         s.createDialer,
		Proxy:           http.ProxyFromEnvironment,
		TLSClientConfig: s.createTlsConfig(),
	}
}

func (s *SocketIO) createDialer(network, address string) (net.Conn, error) {
	return s.state.Dialer.DialContext(*s.ctx, network, address)
}

func (s *SocketIO) createTlsConfig() (tlsConfig *tls.Config) {
	if s.state.TLSConfig != nil {
		tlsConfig = s.state.TLSConfig.Clone()
		tlsConfig.NextProtos = []string{"http/1.1"}
	}
	return
}

// package ws

// import (
// 	"context"
// 	"crypto/tls"
// 	"encoding/json"
// 	"errors"
// 	"fmt"
// 	"io/ioutil"
// 	"net"
// 	"net/http"
// 	"strconv"
// 	"strings"
// 	"sync"
// 	"time"

// 	"github.com/dop251/goja"
// 	"github.com/gorilla/websocket"
// 	"github.com/loadimpact/k6/js/common"
// 	"github.com/loadimpact/k6/lib"
// 	"github.com/loadimpact/k6/lib/metrics"
// 	"github.com/loadimpact/k6/stats"
// )

// // ErrWSInInitContext is returned when websockets are using in the init context
// var ErrWSInInitContext = common.NewInitContextError("using websockets in the init context is not supported")

// type WS struct{}

// type pingDelta struct {
// 	ping time.Time
// 	pong time.Time
// }

// const (
// 	SOCKET_IO_EVENT    = "io"
// 	SOCKET_EVENT       = "message"
// 	ERROR_WRONG_PACKET = "Wrong packet"
// 	COMMON_MSG         = "42"
// )

// type Socket struct {
// 	ctx             context.Context
// 	conn            *websocket.Conn
// 	eventHandlers   map[string][]goja.Callable
// 	ioEventHandlers map[string][]goja.Callable
// 	scheduled       chan goja.Callable
// 	done            chan struct{}
// 	shutdownOnce    sync.Once

// 	msgSentTimestamps     []time.Time
// 	msgReceivedTimestamps []time.Time

// 	pingSendTimestamps map[string]time.Time
// 	pingSendCounter    int
// 	pingTimestamps     []pingDelta
// }

// type WSHTTPResponse struct {
// 	URL     string            `json:"url"`
// 	Status  int               `json:"status"`
// 	Headers map[string]string `json:"headers"`
// 	Body    string            `json:"body"`
// 	Error   string            `json:"error"`
// }

// const writeWait = 10 * time.Second

// func New() *WS {
// 	return &WS{}
// }

// func (*WS) Connect(ctx context.Context, url string, args ...goja.Value) (*WSHTTPResponse, error) {
// 	rt := common.GetRuntime(ctx)
// 	state := lib.GetState(ctx)
// 	if state == nil {
// 		return nil, ErrWSInInitContext
// 	}

// 	// The params argument is optional
// 	var callableV, paramsV goja.Value
// 	switch len(args) {
// 	case 2:
// 		paramsV = args[0]
// 		callableV = args[1]
// 	case 1:
// 		paramsV = goja.Undefined()
// 		callableV = args[0]
// 	default:
// 		return nil, errors.New("invalid number of arguments to ws.connect")
// 	}

// 	// Get the callable (required)
// 	setupFn, isFunc := goja.AssertFunction(callableV)
// 	if !isFunc {
// 		return nil, errors.New("last argument to ws.connect must be a function")
// 	}

// 	// Leave header to nil by default so we can pass it directly to the Dialer
// 	var header http.Header

// 	tags := state.Options.RunTags.CloneTags()

// 	// Parse the optional second argument (params)
// 	if !goja.IsUndefined(paramsV) && !goja.IsNull(paramsV) {
// 		params := paramsV.ToObject(rt)
// 		for _, k := range params.Keys() {
// 			switch k {
// 			case "headers":
// 				header = http.Header{}
// 				headersV := params.Get(k)
// 				if goja.IsUndefined(headersV) || goja.IsNull(headersV) {
// 					continue
// 				}
// 				headersObj := headersV.ToObject(rt)
// 				if headersObj == nil {
// 					continue
// 				}
// 				for _, key := range headersObj.Keys() {
// 					header.Set(key, headersObj.Get(key).String())
// 				}
// 			case "tags":
// 				tagsV := params.Get(k)
// 				if goja.IsUndefined(tagsV) || goja.IsNull(tagsV) {
// 					continue
// 				}
// 				tagObj := tagsV.ToObject(rt)
// 				if tagObj == nil {
// 					continue
// 				}
// 				for _, key := range tagObj.Keys() {
// 					tags[key] = tagObj.Get(key).String()
// 				}
// 			}
// 		}

// 	}

// 	if state.Options.SystemTags.Has(stats.TagURL) {
// 		tags["url"] = url
// 	}
// 	if state.Options.SystemTags.Has(stats.TagGroup) {
// 		tags["group"] = state.Group.Path
// 	}

// 	// Pass a custom net.Dial function to websocket.Dialer that will substitute
// 	// the underlying net.Conn with our own tracked netext.Conn
// 	netDial := func(network, address string) (net.Conn, error) {
// 		return state.Dialer.DialContext(ctx, network, address)
// 	}

// 	// Overriding the NextProtos to avoid talking http2
// 	var tlsConfig *tls.Config
// 	if state.TLSConfig != nil {
// 		tlsConfig = state.TLSConfig.Clone()
// 		tlsConfig.NextProtos = []string{"http/1.1"}
// 	}

// 	wsd := websocket.Dialer{
// 		NetDial:         netDial,
// 		Proxy:           http.ProxyFromEnvironment,
// 		TLSClientConfig: tlsConfig,
// 	}

// 	start := time.Now()
// 	conn, httpResponse, connErr := wsd.Dial(url, header)
// 	connectionEnd := time.Now()
// 	connectionDuration := stats.D(connectionEnd.Sub(start))

// 	socket := Socket{
// 		ctx:                ctx,
// 		conn:               conn,
// 		eventHandlers:      make(map[string][]goja.Callable),
// 		ioEventHandlers:    make(map[string][]goja.Callable),
// 		pingSendTimestamps: make(map[string]time.Time),
// 		scheduled:          make(chan goja.Callable),
// 		done:               make(chan struct{}),
// 	}
// 	if state.Options.SystemTags.Has(stats.TagIP) && conn.RemoteAddr() != nil {
// 		if ip, _, err := net.SplitHostPort(conn.RemoteAddr().String()); err == nil {
// 			tags["ip"] = ip
// 		}
// 	}

// 	if connErr != nil {
// 		// Pass the error to the user script before exiting immediately
// 		socket.handleEvent("error", rt.ToValue(connErr))

// 		return nil, connErr
// 	}

// 	// Run the user-provided set up function
// 	if _, err := setupFn(goja.Undefined(), rt.ToValue(&socket)); err != nil {
// 		_ = socket.closeConnection(websocket.CloseGoingAway)
// 		return nil, err
// 	}

// 	wsResponse, wsRespErr := wrapHTTPResponse(httpResponse)
// 	if wsRespErr != nil {
// 		return nil, wsRespErr
// 	}
// 	wsResponse.URL = url

// 	defer func() { _ = conn.Close() }()

// 	if state.Options.SystemTags.Has(stats.TagStatus) {
// 		tags["status"] = strconv.Itoa(httpResponse.StatusCode)
// 	}
// 	if state.Options.SystemTags.Has(stats.TagSubproto) {
// 		tags["subproto"] = httpResponse.Header.Get("Sec-WebSocket-Protocol")
// 	}

// 	// The connection is now open, emit the event
// 	socket.handleEvent("open",rt.ToValue("4{status: \"connected\"}"))

// 	// Make the default close handler a noop to avoid duplicate closes,
// 	// since we use custom closing logic to call user's event
// 	// handlers and for cleanup. See closeConnection.
// 	// closeConnection is not set directly as a handler here to
// 	// avoid race conditions when calling the Goja runtime.
// 	conn.SetCloseHandler(func(code int, text string) error { return nil })

// 	// Pass ping/pong events through the main control loop
// 	pingChan := make(chan string)
// 	pongChan := make(chan string)
// 	conn.SetPingHandler(func(msg string) error { pingChan <- msg; return nil })
// 	conn.SetPongHandler(func(pingID string) error { pongChan <- pingID; return nil })

// 	readDataChan := make(chan []byte)
// 	readCloseChan := make(chan int)
// 	readErrChan := make(chan error)

// 	// Wraps a couple of channels around conn.ReadMessage
// 	go readPump(conn, readDataChan, readErrChan, readCloseChan)

// 	// This is the main control loop. All JS code (including error handlers)
// 	// should only be executed by this thread to avoid race conditions
// 	for {
// 		select {
// 		case pingData := <-pingChan:
// 			// Handle pings received from the server
// 			// - trigger the `ping` event
// 			// - reply with pong (needed when `SetPingHandler` is overwritten)
// 			err := socket.conn.WriteControl(websocket.PongMessage, []byte(pingData), time.Now().Add(writeWait))
// 			if err != nil {
// 				socket.handleEvent("error", rt.ToValue(err))
// 			}
// 			socket.handleEvent("ping",rt.ToValue("ping"))

// 		case pingID := <-pongChan:
// 			// Handle pong responses to our pings
// 			socket.trackPong(pingID)
// 			socket.handleEvent("pong", rt.ToValue("pong"))

// 		case readData := <-readDataChan:
// 			socket.msgReceivedTimestamps = append(socket.msgReceivedTimestamps, time.Now())
// 			socket.handleEvent(SOCKET_EVENT, rt.ToValue(string(readData)))

// 		case readErr := <-readErrChan:
// 			socket.handleEvent("error", rt.ToValue(readErr))

// 		case code := <-readCloseChan:
// 			_ = socket.closeConnection(code)

// 		case scheduledFn := <-socket.scheduled:
// 			if _, err := scheduledFn(goja.Undefined()); err != nil {
// 				return nil, err
// 			}

// 		case <-ctx.Done():
// 			// VU is shutting down during an interrupt
// 			// socket events will not be forwarded to the VU
// 			_ = socket.closeConnection(websocket.CloseGoingAway)

// 		case <-socket.done:
// 			// This is the final exit point normally triggered by closeConnection
// 			end := time.Now()
// 			sessionDuration := stats.D(end.Sub(start))

// 			sampleTags := stats.IntoSampleTags(&tags)

// 			stats.PushIfNotCancelled(ctx, state.Samples, stats.ConnectedSamples{
// 				Samples: []stats.Sample{
// 					{Metric: metrics.WSSessions, Time: start, Tags: sampleTags, Value: 1},
// 					{Metric: metrics.WSConnecting, Time: start, Tags: sampleTags, Value: connectionDuration},
// 					{Metric: metrics.WSSessionDuration, Time: start, Tags: sampleTags, Value: sessionDuration},
// 				},
// 				Tags: sampleTags,
// 				Time: start,
// 			})

// 			for _, msgSentTimestamp := range socket.msgSentTimestamps {
// 				stats.PushIfNotCancelled(ctx, state.Samples, stats.Sample{
// 					Metric: metrics.WSMessagesSent,
// 					Time:   msgSentTimestamp,
// 					Tags:   sampleTags,
// 					Value:  1,
// 				})
// 			}

// 			for _, msgReceivedTimestamp := range socket.msgReceivedTimestamps {
// 				stats.PushIfNotCancelled(ctx, state.Samples, stats.Sample{
// 					Metric: metrics.WSMessagesReceived,
// 					Time:   msgReceivedTimestamp,
// 					Tags:   sampleTags,
// 					Value:  1,
// 				})
// 			}

// 			for _, pingDelta := range socket.pingTimestamps {
// 				stats.PushIfNotCancelled(ctx, state.Samples, stats.Sample{
// 					Metric: metrics.WSPing,
// 					Time:   pingDelta.pong,
// 					Tags:   sampleTags,
// 					Value:  stats.D(pingDelta.pong.Sub(pingDelta.ping)),
// 				})
// 			}

// 			return wsResponse, nil
// 		}
// 	}
// }

// func (s *Socket) On(event string, handler goja.Value) {
// 	if handler, ok := goja.AssertFunction(handler); ok {
// 		s.eventHandlers[event] = append(s.eventHandlers[event], handler)
// 	}
// }

// func (s *Socket) OnSocketIO(event string, handler goja.Value) {
// 	if handler, ok := goja.AssertFunction(handler); ok {
// 		s.ioEventHandlers[event] = append(s.ioEventHandlers[event], handler)
// 	}
// }

// func (s *Socket) handleEvent(event string, args ...goja.Value) {
// 	s.eventHandlersProcess(s.eventHandlers, event, args)
// 	s.eventHandlersProcess(s.ioEventHandlers, SOCKET_IO_EVENT, args)
// }

// func (s *Socket) eventHandlersProcess(evenHandlers map[string][]goja.Callable, event string, args []goja.Value) {
// 	if event == SOCKET_IO_EVENT {
// 		s.eventSocketIOHandlersProcess(evenHandlers, args)
// 	} else {
// 		s.eventWebSocketHandlersProcess(evenHandlers, event, args)
// 	}
// }

// func (s *Socket) eventSocketIOHandlersProcess(evenHandlers map[string][]goja.Callable, args []goja.Value) {
// 	event, message, err := s.getSocketIOData(args)
// 	if err != nil {
// 		common.Throw(common.GetRuntime(s.ctx), err)
// 	} else {
// 		s.handlersProcess(evenHandlers, event, message)
// 	}
// }

// func (s *Socket) eventWebSocketHandlersProcess(evenHandlers map[string][]goja.Callable, event string, args []goja.Value) {
// 	s.handlersProcess(evenHandlers, event, args)
// }

// /**
//  * Parsing the raw message from server to get event name
//  * TODO: The current method assumes the webserver return socketIO message data in the first item in the array
//  *
//  *
// **/
// func (s *Socket) getSocketIOData(args []goja.Value) (event string, message []goja.Value, err error) {
// 	if len(args) == 0 {
// 		event = "message"
// 		message = args
// 	} else {
// 		event, message, err = s.handleSocketIOResponse(args[0].String())
// 	}
// 	return
// }

// func (s *Socket) handleSocketIOResponse(rawResponse string) (eventName string, messageResponse []goja.Value, err error) {
// 	var start int
// 	socketIOCode := rawResponse[0:2]
// 	var v interface{}
// 	for i, c := range rawResponse {
// 		if c == '[' {
// 			start = i
// 			break
// 		}
// 	}
// 	rt := common.GetRuntime(s.ctx)
// 	switch socketIOCode {
// 	case COMMON_MSG:
// 		fmt.Println("COMMON_MSG: ",rawResponse)
// 		responseData := rawResponse[start:len(rawResponse)]
// 		json.Unmarshal([]byte(responseData), &v)
// 		eventName = (v.([]interface{})[0]).(string)
// 		responseMap := (v.([]interface{})[1])
// 		response, error := json.MarshalIndent(responseMap, "", "  ")
// 		if error != nil {
// 			err = error
// 		}
// 		messageResponse = []goja.Value{rt.ToValue(string(response))}
// 		return
// 	default:
// 		fmt.Println("ANOTHER_MSG: ",rawResponse)
// 		return "handshake", []goja.Value{rt.ToValue(string(rawResponse))}, nil
// 	}
// 	return "error", []goja.Value{rt.ToValue(string("Error"))}, errors.New("Can't parse response data")
// }

// func (s *Socket) handlersProcess(evenHandlers map[string][]goja.Callable, event string, args []goja.Value) {
// 	if handlers, ok := evenHandlers[event]; ok {
// 		for _, handler := range handlers {
// 			if _, err := handler(goja.Undefined(), args...); err != nil {
// 				common.Throw(common.GetRuntime(s.ctx), err)
// 			}
// 		}
// 	}
// }

// /**
//  * Allow k6.io send message to socket.io protocol
//  * Ex:
//  export default function() {
//   var url = 'ws://localhost:3000/socket.io/?EIO=3&transport=websocket';
//   var params = { tags: { my_tag: 'hello' } };
//   ws.connect(url, params, function(socket) {
//     socket.on('open', function open() {
//       console.log('connected');
//       socket.sendSocketIO('message', 'Hello! websocket test' + __VU);
//     });

//     socket.setTimeout(() => {
//       console.log('End socket test after 2s');
//       socket.close();
//     }, 10000);
//   });
// }
//  *
// **/
// func (s *Socket) SendSocketIO(event, message string) {
// 	const commonMessage = 42
// 	msg := fmt.Sprintf("%d[\"%s\",%s]", commonMessage, event, message)
// 	s.Send(msg)
// }

// func (s *Socket) Send(message string) {
// 	// NOTE: No binary message support for the time being since goja doesn't
// 	// support typed arrays.
// 	rt := common.GetRuntime(s.ctx)

// 	writeData := []byte(message)
// 	if err := s.conn.WriteMessage(websocket.TextMessage, writeData); err != nil {
// 		s.handleEvent("error", rt.ToValue(err))
// 	}

// 	s.msgSentTimestamps = append(s.msgSentTimestamps, time.Now())
// }

// func (s *Socket) Ping() {
// 	rt := common.GetRuntime(s.ctx)
// 	deadline := time.Now().Add(writeWait)
// 	pingID := strconv.Itoa(s.pingSendCounter)
// 	data := []byte(pingID)

// 	err := s.conn.WriteControl(websocket.PingMessage, data, deadline)
// 	if err != nil {
// 		s.handleEvent("error", rt.ToValue(err))
// 		return
// 	}

// 	s.pingSendTimestamps[pingID] = time.Now()
// 	s.pingSendCounter++
// }

// func (s *Socket) trackPong(pingID string) {
// 	pongTimestamp := time.Now()

// 	if _, ok := s.pingSendTimestamps[pingID]; !ok {
// 		// We received a pong for a ping we didn't send; ignore
// 		// (this shouldn't happen with a compliant server)
// 		return
// 	}
// 	pingTimestamp := s.pingSendTimestamps[pingID]

// 	s.pingTimestamps = append(s.pingTimestamps, pingDelta{pingTimestamp, pongTimestamp})
// }

// func (s *Socket) SetTimeout(fn goja.Callable, timeoutMs int) {
// 	// Starts a goroutine, blocks once on the timeout and pushes the callable
// 	// back to the main loop through the scheduled channel
// 	go func() {
// 		select {
// 		case <-time.After(time.Duration(timeoutMs) * time.Millisecond):
// 			s.scheduled <- fn

// 		case <-s.done:
// 			return
// 		}
// 	}()
// }

// func (s *Socket) SetInterval(fn goja.Callable, intervalMs int) {
// 	// Starts a goroutine, blocks forever on the ticker and pushes the callable
// 	// back to the main loop through the scheduled channel
// 	go func() {
// 		ticker := time.NewTicker(time.Duration(intervalMs) * time.Millisecond)
// 		defer ticker.Stop()

// 		for {
// 			select {
// 			case <-ticker.C:
// 				s.scheduled <- fn

// 			case <-s.done:
// 				return
// 			}
// 		}
// 	}()
// }

// func (s *Socket) Close(args ...goja.Value) {
// 	code := websocket.CloseGoingAway
// 	if len(args) > 0 {
// 		code = int(args[0].ToInteger())
// 	}

// 	_ = s.closeConnection(code)
// }

// // closeConnection cleanly closes the WebSocket connection.
// // Returns an error if sending the close control frame fails.
// func (s *Socket) closeConnection(code int) error {
// 	var err error

// 	s.shutdownOnce.Do(func() {
// 		rt := common.GetRuntime(s.ctx)

// 		err = s.conn.WriteControl(websocket.CloseMessage,
// 			websocket.FormatCloseMessage(code, ""),
// 			time.Now().Add(writeWait),
// 		)
// 		if err != nil {
// 			// Call the user-defined error handler
// 			s.handleEvent("error", rt.ToValue(err))
// 		}

// 		// Call the user-defined close handler
// 		s.handleEvent("close", rt.ToValue(code))

// 		_ = s.conn.Close()

// 		// Stop the main control loop
// 		close(s.done)
// 	})

// 	return err
// }

// // Wraps conn.ReadMessage in a channel
// func readPump(conn *websocket.Conn, readChan chan []byte, errorChan chan error, closeChan chan int) {
// 	for {
// 		_, message, err := conn.ReadMessage()
// 		if err != nil {
// 			if websocket.IsUnexpectedCloseError(
// 				err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
// 				// Report an unexpected closure
// 				errorChan <- err
// 			}
// 			code := websocket.CloseGoingAway
// 			if e, ok := err.(*websocket.CloseError); ok {
// 				code = e.Code
// 			}
// 			closeChan <- code
// 			return
// 		}

// 		readChan <- message
// 	}
// }

// // Wrap the raw HTTPResponse we received to a WSHTTPResponse we can pass to the user
// func wrapHTTPResponse(httpResponse *http.Response) (*WSHTTPResponse, error) {
// 	wsResponse := WSHTTPResponse{
// 		Status: httpResponse.StatusCode,
// 	}

// 	body, err := ioutil.ReadAll(httpResponse.Body)
// 	if err != nil {
// 		return nil, err
// 	}

// 	err = httpResponse.Body.Close()
// 	if err != nil {
// 		return nil, err
// 	}

// 	wsResponse.Body = string(body)

// 	wsResponse.Headers = make(map[string]string, len(httpResponse.Header))
// 	for k, vs := range httpResponse.Header {
// 		wsResponse.Headers[k] = strings.Join(vs, ", ")
// 	}

// 	return &wsResponse, nil
// }
