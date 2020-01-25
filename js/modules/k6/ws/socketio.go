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
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"
	"github.com/gorilla/websocket"
	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/lib/netext/httpext"
	"github.com/loadimpact/k6/stats"
)

type WSIO struct{}

type SocketIORunner struct {
	runtime          *goja.Runtime
	ctx              *context.Context
	callbackFunction goja.Callable
	socketOptions    goja.Value
	requestHeaders   *http.Header
	state            *lib.State
	conn             *websocket.Conn
	dialer           *websocket.Dialer
	url              *url.URL
	response         *WSHTTPResponse
}

type SocketIOMetrics struct {
	connectionStart       time.Time
	connectionEnd         time.Time
	msgSentTimestamps     []time.Time
	msgReceivedTimestamps []time.Time

	pingSendTimestamps map[string]time.Time
	pingSendCounter    int
	pingTimestamps     []pingDelta
}

type SocketIO struct {
	runner        SocketIORunner
	metrics       SocketIOMetrics
	eventHandlers map[string][]goja.Callable
	scheduled     chan goja.Callable
	done          chan struct{}
	shutdownOnce  sync.Once
	tags          map[string]string
}

type EventLoopDataChannel struct {
	pingChan      chan string
	pongChan      chan string
	readDataChan  chan []byte
	readCloseChan chan int
	readErrChan   chan error
}

const (
	SOCKET_PROTOCOL        = "ws://"
	SOCKET_SECURE_PROTOCOL = "wss://"
	SOCKET_IO_PATH         = "/socket.io/?EIO=3&transport=websocket"
)
const (
	OPEN           = "0"
	CLOSE          = "1"
	PING           = "2"
	PONG           = "3"
	MESSAGE        = "4"
	EMPTY_MESSAGE  = "40"
	COMMON_MESSAGE = "42"
	UPGRADE        = "5"
	NOOP           = "6"
)

func NewSocketIO() *WSIO {
	return &WSIO{}
}

func (*WSIO) Connect(ctx context.Context, url string, args ...goja.Value) (*WSHTTPResponse, error) {
	validateParamArguments(ctx, args...)
	socket := newWebSocketIO(ctx, url)
	socket.setUpSocketIO(args...)
	if err := socket.startConnect(); err != nil {
		return nil, err
	}
	defer socket.close()
	(socket.runner.callbackFunction)(goja.Undefined(), socket.runner.runtime.ToValue(&socket))
	socket.runner.conn.SetCloseHandler(func(code int, text string) error { return nil })
	eventLoopDataChan := newEventLoopData()
	socket.runner.conn.SetPingHandler(func(msg string) error { eventLoopDataChan.pingChan <- msg; return nil })
	socket.runner.conn.SetPongHandler(func(pingID string) error { eventLoopDataChan.pongChan <- pingID; return nil })

	// Wraps a couple of channels around conn.ReadMessage
	go readPump(socket.runner.conn, eventLoopDataChan.readDataChan, eventLoopDataChan.readErrChan, eventLoopDataChan.readCloseChan)
	return socket.eventLoopHandler(ctx, eventLoopDataChan)
}

func (socket *SocketIO) eventLoopHandler(ctx context.Context, eventLoopDataChan EventLoopDataChannel) (*WSHTTPResponse, error) {
	for {
		select {
		case pingData := <-eventLoopDataChan.pingChan:
			socket.pingHandler(pingData)

		case pingID := <-eventLoopDataChan.pongChan:
			socket.pongHandler(pingID)

		case readData := <-eventLoopDataChan.readDataChan:
			socket.receiveDataHandler(string(readData))

		case readErr := <-eventLoopDataChan.readErrChan:
			socket.readErrorHandler(readErr)

		case code := <-eventLoopDataChan.readCloseChan:
			// _ = socket.closeConnection(code)
			socket.closeConnectionHandler(code)

		case scheduledFn := <-socket.scheduled:
			if err := socket.gojaCallbackFuncHandler(scheduledFn); err != nil {
				return nil, err
			}

		case <-ctx.Done():
			// VU is shutting down during an interrupt
			// socket events will not be forwarded to the VU
			socket.closeConnectionHandler(websocket.CloseGoingAway)

		case <-socket.done:
			return socket.doneHandler(ctx)
		}
	}
}
func (socket *SocketIO) pingHandler(pingData string) {
	// Handle pings received from the server
	// - trigger the `ping` event
	// - reply with pong (needed when `SetPingHandler` is overwritten)
	err := socket.runner.conn.WriteControl(websocket.PongMessage, []byte(pingData), time.Now().Add(writeWait))
	if err != nil {
		socket.handleEvent("error", socket.runner.runtime.ToValue(err))
	}
	socket.handleEvent("ping")
}

func (socket *SocketIO) pongHandler(pingID string) {
	socket.trackPong(pingID)
	socket.handleEvent("pong")
}

func (socket *SocketIO) receiveDataHandler(readData string) {
	socket.metrics.msgReceivedTimestamps = append(socket.metrics.msgReceivedTimestamps, time.Now())
	event, _, _ := socket.handleSocketIOResponse(string(readData))
	socket.handleEvent(event, socket.runner.runtime.ToValue(string(readData)))

}

func (socket *SocketIO) readErrorHandler(readErr error) {
	socket.handleEvent("error", socket.runner.runtime.ToValue(readErr))
}

func (socket *SocketIO) gojaCallbackFuncHandler(scheduledFn goja.Callable) (err error) {
	if _, err = scheduledFn(goja.Undefined()); err != nil {
		return err
	}
	return
}

func (socket *SocketIO) closeConnectionHandler(code int) {
	switch code {
	case websocket.CloseGoingAway:
		_ = socket.closeConnection(code)
	default:
		_ = socket.closeConnection(code)
	}
}

func (socket *SocketIO) doneHandler(ctx context.Context) (*WSHTTPResponse, error) {
	// This is the final exit point normally triggered by closeConnection
	end := time.Now()
	sessionDuration := stats.D(end.Sub(socket.metrics.connectionStart))

	sampleTags := stats.IntoSampleTags(&socket.tags)
	socket.pushOverviewMetrics(ctx, sampleTags, sessionDuration)
	socket.pushSentMetrics(ctx, sampleTags)
	socket.pushReceivedMetrics(ctx, sampleTags)
	socket.pushPingMetrics(ctx, sampleTags)
	return socket.runner.response, nil
}

func (socket *SocketIO) pushOverviewMetrics(ctx context.Context, sampleTags *stats.SampleTags, sessionDuration float64) {
	stats.PushIfNotDone(ctx, socket.runner.state.Samples, stats.ConnectedSamples{
		Samples: []stats.Sample{
			{Metric: metrics.WSSessions, Time: socket.metrics.connectionStart, Tags: sampleTags, Value: 1},
			{Metric: metrics.WSConnecting, Time: socket.metrics.connectionStart, Tags: sampleTags, Value: stats.D(socket.metrics.connectionEnd.Sub(socket.metrics.connectionStart))},
			{Metric: metrics.WSSessionDuration, Time: socket.metrics.connectionStart, Tags: sampleTags, Value: sessionDuration},
		},
		Tags: sampleTags,
		Time: socket.metrics.connectionStart,
	})
}

func (socket *SocketIO) pushSentMetrics(ctx context.Context, sampleTags *stats.SampleTags) {
	for _, msgSentTimestamp := range socket.metrics.msgSentTimestamps {
		stats.PushIfNotDone(ctx, socket.runner.state.Samples, stats.Sample{
			Metric: metrics.WSMessagesSent,
			Time:   msgSentTimestamp,
			Tags:   sampleTags,
			Value:  1,
		})
	}
}

func (socket *SocketIO) pushReceivedMetrics(ctx context.Context, sampleTags *stats.SampleTags) {
	for _, msgReceivedTimestamp := range socket.metrics.msgReceivedTimestamps {
		stats.PushIfNotDone(ctx, socket.runner.state.Samples, stats.Sample{
			Metric: metrics.WSMessagesReceived,
			Time:   msgReceivedTimestamp,
			Tags:   sampleTags,
			Value:  1,
		})
	}
}

func (socket *SocketIO) pushPingMetrics(ctx context.Context, sampleTags *stats.SampleTags) {
	for _, pingDelta := range socket.metrics.pingTimestamps {
		stats.PushIfNotDone(ctx, socket.runner.state.Samples, stats.Sample{
			Metric: metrics.WSPing,
			Time:   pingDelta.pong,
			Tags:   sampleTags,
			Value:  stats.D(pingDelta.pong.Sub(pingDelta.ping)),
		})
	}
}

func (s *SocketIO) startConnect() error {
	s.metrics.connectionStart = time.Now()
	err := s.connect()
	s.metrics.connectionEnd = time.Now()
	return err
}

func (s *SocketIO) connect() error {
	conn, response, err := s.runner.dialer.Dial(s.runner.url.String(), s.runner.requestHeaders.Clone())
	if err != nil {
		return err
	}
	wsResponse, wsRespErr := wrapHTTPResponse(response)
	if wsRespErr != nil {
		return wsRespErr
	}
	s.runner.conn = conn
	s.runner.response = wsResponse
	s.runner.response.URL = s.runner.url.String()
	return nil
}

func newWebSocketIO(initCtx context.Context, url string) SocketIO {
	initRunner := newWebSocketIORunner(initCtx, url)
	initMetrics := newWebSocketIOMetrics()
	return SocketIO{
		runner:        initRunner,
		metrics:       initMetrics,
		tags:          initRunner.state.Options.RunTags.CloneTags(),
		eventHandlers: make(map[string][]goja.Callable),
		scheduled:     make(chan goja.Callable),
		done:          make(chan struct{}),
	}
}

func newWebSocketIORunner(initCtx context.Context, rawUrl string) SocketIORunner {
	initRuntime, initState := initConnectState(initCtx)
	u, err := url.Parse(rawUrl)
	isInvalidUrl := len(strings.TrimSpace(u.Hostname())) == 0
	if err != nil || isInvalidUrl {
		msg := fmt.Sprintf("URL [%s] is invalid", rawUrl)
		panic(common.NewInitContextError(msg))
	}
	return SocketIORunner{
		runtime:        initRuntime,
		requestHeaders: &http.Header{},
		ctx:            &initCtx,
		state:          initState,
		url:            u,
	}
}

func newWebSocketIOMetrics() SocketIOMetrics {
	return SocketIOMetrics{
		pingSendTimestamps: make(map[string]time.Time),
	}
}

func newEventLoopData() EventLoopDataChannel {
	return EventLoopDataChannel{
		pingChan:      make(chan string),
		pongChan:      make(chan string),
		readDataChan:  make(chan []byte),
		readCloseChan: make(chan int),
		readErrChan:   make(chan error),
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

func (s *SocketIO) setUpSocketIO(args ...goja.Value) {
	s.extractSocketOptions(args...)
	s.configureSocketOptions(s.runner.socketOptions)
	s.createSocketIODialer()
	s.initDefaultTags()
}

func (s *SocketIO) extractSocketOptions(args ...goja.Value) {
	var callFunc goja.Value
	for _, v := range args {
		argType := v.ExportType()
		switch argType.Kind() {
		case reflect.Map:
			s.runner.socketOptions = v
			continue
		case reflect.Func:
			callFunc = v
			continue
		default:
			common.Throw(common.GetRuntime(*s.runner.ctx), errors.New("Invalid argument types. Allowing Map and Function types"))
			continue
		}
	}
	s.runner.callbackFunction = validateCallableParam(s.runner.ctx, callFunc)
}

func validateParamArguments(ctx context.Context, args ...goja.Value) {
	switch len(args) {
	case 1, 2:
		return
	default:
		common.Throw(common.GetRuntime(ctx), errors.New("invalid number of arguments to ws.connect. Method is required 3 params ( url, params, callback )"))
	}
}

func validateCallableParam(ctx *context.Context, callableParam goja.Value) (setupFn goja.Callable) {
	callableFunc, isFunc := goja.AssertFunction(callableParam)
	if !isFunc {
		common.Throw(common.GetRuntime(*ctx), errors.New("last argument to ws.connect must be a function"))
	}
	return callableFunc
}

func (s *SocketIO) configureSocketOptions(params goja.Value) {
	if params == nil {
		return
	}
	paramsObject := params.ToObject(s.runner.runtime)
	for _, key := range paramsObject.Keys() {
		switch key {
		case "headers":
			s.setSocketHeaders(paramsObject)
			continue
		case "tags":
			s.setSocketTags(paramsObject)
			continue
		case "cookies":
			s.setSocketCookies(paramsObject)
			break
		default:
			continue
		}
	}
}

func (s *SocketIO) setSocketHeaders(paramsObject *goja.Object) {
	headersV := paramsObject.Get("headers")
	if goja.IsUndefined(headersV) || goja.IsNull(headersV) {
		return
	}
	headersObj := headersV.ToObject(s.runner.runtime)
	for _, key := range headersObj.Keys() {
		s.runner.requestHeaders.Set(key, headersObj.Get(key).String())
	}
}

func (s *SocketIO) setSocketCookies(paramsObject *goja.Object) {
	cookiesV := paramsObject.Get("cookies")
	if goja.IsUndefined(cookiesV) || goja.IsNull(cookiesV) {
		return
	}
	cookiesObject := cookiesV.ToObject(s.runner.runtime)
	if cookiesObject == nil {
		return
	}
	requestCookies := s.extractCookiesValues(cookiesObject)
	cookies := []string{}
	for _, value := range requestCookies {
		cookies = append(cookies, fmt.Sprintf("%s=%s", value.Name, value.Value))
	}
	s.runner.requestHeaders.Set("cookie", strings.Join(cookies, ";"))
}

func (s *SocketIO) setSocketTags(paramsObject *goja.Object) {
	tagsV := paramsObject.Get("tags")
	if goja.IsUndefined(tagsV) || goja.IsNull(tagsV) {
		return
	}
	tagsObj := tagsV.ToObject(s.runner.runtime)
	for _, key := range tagsObj.Keys() {
		s.tags[key] = tagsObj.Get(key).String()
	}
}

func (s *SocketIO) createSocketIODialer() {
	jar, _ := cookiejar.New(nil)
	s.runner.dialer = &websocket.Dialer{
		NetDial:         s.createDialer,
		Proxy:           http.ProxyFromEnvironment,
		TLSClientConfig: s.createTlsConfig(),
		Jar:             jar,
	}
}

func (s *SocketIO) createDialer(network, address string) (net.Conn, error) {
	return s.runner.state.Dialer.DialContext(*s.runner.ctx, network, address)
}

func (s *SocketIO) createTlsConfig() (tlsConfig *tls.Config) {
	if s.runner.state.TLSConfig != nil {
		tlsConfig = s.runner.state.TLSConfig.Clone()
		tlsConfig.NextProtos = []string{"http/1.1"}
	}
	return
}

func (s *SocketIO) extractCookiesValues(cookiesObject *goja.Object) (requestCookies map[string]*httpext.HTTPRequestCookie) {
	requestCookies = make(map[string]*httpext.HTTPRequestCookie)
	for _, key := range cookiesObject.Keys() {
		cookieV := cookiesObject.Get(key)
		if goja.IsUndefined(cookieV) || goja.IsNull(cookieV) {
			continue
		}
		switch cookieV.ExportType() {
		case reflect.TypeOf(map[string]interface{}{}):
			requestCookies[key] = &httpext.HTTPRequestCookie{Name: key, Value: "", Replace: false}
			cookie := cookieV.ToObject(s.runner.runtime)
			for _, attr := range cookie.Keys() {
				switch strings.ToLower(attr) {
				case "replace":
					requestCookies[key].Replace = cookie.Get(attr).ToBoolean()
				case "value":
					requestCookies[key].Value = cookie.Get(attr).String()
				}
			}
		default:
			requestCookies[key] = &httpext.HTTPRequestCookie{Name: key, Value: cookieV.String(), Replace: false}
		}
	}
	return
}

func (s *SocketIO) initDefaultTags() {
	if s.runner.state.Options.SystemTags.Has(stats.TagURL) {
		s.tags["url"] = s.runner.url.String()
	}
	if s.runner.state.Options.SystemTags.Has(stats.TagGroup) {
		s.tags["group"] = s.runner.state.Group.Path
	}
}

func (s *SocketIO) close() {
	s.runner.conn.Close()
}

func (s *SocketIO) On(event string, handler goja.Value) {
	if handler, ok := goja.AssertFunction(handler); ok {
		s.eventHandlers[event] = append(s.eventHandlers[event], handler)
	}
}

func (s *SocketIO) handleEvent(event string, args ...goja.Value) {
	s.eventHandlersProcess(event, args)
}

func (s *SocketIO) eventHandlersProcess(event string, args []goja.Value) {
	s.eventSocketIOHandlersProcess(event, args)
}

func (s *SocketIO) eventSocketIOHandlersProcess(event string, args []goja.Value) {
	event, message, err := s.getSocketIOData(event, args)
	if err != nil {
		common.Throw(common.GetRuntime(*s.runner.ctx), err)
	} else {
		s.handlersProcess(event, message)
	}
}

/**
 * Parsing the raw message from server to get event name
 * TODO: The current method assumes the webserver return socketIO message data in the first item in the array
 *
 *
**/
func (s *SocketIO) getSocketIOData(event string, args []goja.Value) (channelName string, message []goja.Value, err error) {
	if len(args) == 0 {
		channelName = event
		message = args
	} else {
		channelName, message, err = s.handleSocketIOResponse(args[0].String())
	}
	return
}

func (s *SocketIO) handleSocketIOResponse(rawResponse string) (eventName string, messageResponse []goja.Value, err error) {
	var startRawResponseCharIndex int
	socketIOCode := rawResponse[0:1]
	for i, c := range rawResponse {
		if c == '[' {
			startRawResponseCharIndex = i
			break
		}
	}
	return s.handleSocketIOResponseProcess(socketIOCode, rawResponse, startRawResponseCharIndex)
}

func (s *SocketIO) handleSocketIOResponseProcess(socketIOCode, rawResponse string, startRawResponseCharIndex int) (eventName string, messageResponse []goja.Value, err error) {
	rt := common.GetRuntime(*s.runner.ctx)
	switch socketIOCode {
	case MESSAGE:
		responseData := rawResponse[startRawResponseCharIndex:len(rawResponse)]
		return s.commonMessageResponseProcess(rawResponse, responseData)
	case OPEN:
		return "open", []goja.Value{rt.ToValue(string(rawResponse))}, nil
	default:
		return "handshake", []goja.Value{rt.ToValue(string(rawResponse))}, nil
	}
}

func (s *SocketIO) commonMessageResponseProcess(rawResponse, responseData string) (eventName string, messageResponse []goja.Value, err error) {
	var v interface{}
	rt := common.GetRuntime(*s.runner.ctx)
	switch responseData {
	case EMPTY_MESSAGE:
		return "message", []goja.Value{rt.ToValue(string(rawResponse))}, nil
	default:
		json.Unmarshal([]byte(responseData), &v)
		eventName = (v.([]interface{})[0]).(string)
		responseMap := (v.([]interface{})[1])
		response, error := json.MarshalIndent(responseMap, "", "  ")
		if error != nil {
			err = error
		}
		messageResponse = []goja.Value{rt.ToValue(string(response))}
		return
	}
}
func (s *SocketIO) handlersProcess(event string, args []goja.Value) {
	if handlers, ok := s.eventHandlers[event]; ok {
		for _, handler := range handlers {
			if _, err := handler(goja.Undefined(), args...); err != nil {
				common.Throw(common.GetRuntime(*s.runner.ctx), err)
			}
		}
	}
}

func (s *SocketIO) Send(event, message string) {
	// NOTE: No binary message support for the time being since goja doesn't
	// support typed arrays.
	rt := common.GetRuntime(*s.runner.ctx)

	writeData := []byte(fmt.Sprintf("%s[\"%s\",%s]", COMMON_MESSAGE, event, message))
	if err := s.runner.conn.WriteMessage(websocket.TextMessage, writeData); err != nil {
		s.handleEvent("error", rt.ToValue(err))
	}

	s.metrics.msgSentTimestamps = append(s.metrics.msgSentTimestamps, time.Now())
}

func (s *SocketIO) trackPong(pingID string) {
	pongTimestamp := time.Now()

	if _, ok := s.metrics.pingSendTimestamps[pingID]; !ok {
		// We received a pong for a ping we didn't send; ignore
		// (this shouldn't happen with a compliant server)
		return
	}
	pingTimestamp := s.metrics.pingSendTimestamps[pingID]

	s.metrics.pingTimestamps = append(s.metrics.pingTimestamps, pingDelta{pingTimestamp, pongTimestamp})
}

func (s *SocketIO) closeConnection(code int) error {
	var err error

	s.shutdownOnce.Do(func() {
		rt := common.GetRuntime(*s.runner.ctx)

		err = s.runner.conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(code, ""),
			time.Now().Add(writeWait),
		)
		if err != nil {
			// Call the user-defined error handler
			s.handleEvent("error", rt.ToValue(err))
		}

		// Call the user-defined close handler
		s.handleEvent("close", rt.ToValue(code))

		_ = s.runner.conn.Close()

		// Stop the main control loop
		close(s.done)
	})

	return err
}

func (s *SocketIO) SetTimeout(fn goja.Callable, timeoutMs int) {
	// Starts a goroutine, blocks once on the timeout and pushes the callable
	// back to the main loop through the scheduled channel
	go func() {
		select {
		case <-time.After(time.Duration(timeoutMs) * time.Millisecond):
			s.scheduled <- fn

		case <-s.done:
			return
		}
	}()
}

func (s *SocketIO) SetInterval(fn goja.Callable, intervalMs int) {
	// Starts a goroutine, blocks forever on the ticker and pushes the callable
	// back to the main loop through the scheduled channel
	go func() {
		ticker := time.NewTicker(time.Duration(intervalMs) * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.scheduled <- fn

			case <-s.done:
				return
			}
		}
	}()
}

func (s *SocketIO) Close(args ...goja.Value) {
	code := websocket.CloseGoingAway
	if len(args) > 0 {
		code = int(args[0].ToInteger())
	}

	_ = s.closeConnection(code)
}

func (s *SocketIO) Ping() {
	rt := common.GetRuntime(*s.runner.ctx)
	deadline := time.Now().Add(writeWait)
	pingID := strconv.Itoa(s.metrics.pingSendCounter)
	data := []byte(pingID)

	err := s.runner.conn.WriteControl(websocket.PingMessage, data, deadline)
	if err != nil {
		s.handleEvent("error", rt.ToValue(err))
		return
	}

	s.metrics.pingSendTimestamps[pingID] = time.Now()
	s.metrics.pingSendCounter++
}
