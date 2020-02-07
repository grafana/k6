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

// WebSocketIO represents a JS console implemented as a logrus.Logger.
type WebSocketIO struct{}

type socketIORunner struct {
	runtime          *goja.Runtime
	ctx              *context.Context
	callbackFunction goja.Callable
	socketOptions    goja.Value
	requestHeaders   http.Header
	state            *lib.State
	conn             *websocket.Conn
	dialer           *websocket.Dialer
	url              *url.URL
	response         *WSHTTPResponse
}

type socketIOMetrics struct {
	connectionStart       time.Time
	connectionEnd         time.Time
	msgSentTimestamps     []time.Time
	msgReceivedTimestamps []time.Time

	pingSendTimestamps map[string]time.Time
	pingSendCounter    int
	pingTimestamps     []pingDelta
}

type socketIO struct {
	runner            socketIORunner
	metrics           socketIOMetrics
	eventHandlers     map[string][]goja.Callable
	scheduled         chan goja.Callable
	done              chan struct{}
	shutdownOnce      sync.Once
	tags              map[string]string
	eventLoopDataChan eventLoopDataChannel
}

type eventLoopDataChannel struct {
	pingChan      chan string
	pongChan      chan string
	readDataChan  chan []byte
	readCloseChan chan int
	readErrChan   chan error
}

const (
	openCode          = "0"
	pingCode          = "2"
	pongCode          = "3"
	messageCode       = "4"
	emptyMessageCode  = "40"
	commonMessageCode = "42"
)

// NewSocketIO creates a socketIO API object.
func NewSocketIO() *WebSocketIO {
	return &WebSocketIO{}
}

// Connect makes an WebSocket connect request and returns a corresponding response by taking goja.Values as arguments
func (*WebSocketIO) Connect(ctx context.Context, url string, args ...goja.Value) (*WSHTTPResponse, error) {
	validateParamArguments(ctx, args...)
	socket := newWebSocketIO(ctx, url)
	socket.setUpSocketIO(args...)
	if err := socket.startConnect(); err != nil {
		return nil, err
	}
	defer socket.close()
	if err := invokeSocketCallBackFunc(&socket); err != nil {
		return nil, err
	}
	socket.runner.conn.SetCloseHandler(func(code int, text string) error { return nil })
	socket.runner.conn.SetPingHandler(func(msg string) error {
		socket.eventLoopDataChan.pingChan <- msg
		return nil
	})
	socket.runner.conn.SetPongHandler(func(pingID string) error {
		socket.eventLoopDataChan.pongChan <- pingID
		return nil
	})

	// Wraps a couple of channels around conn.ReadMessage
	go readPump(socket.runner.conn,
		socket.eventLoopDataChan.readDataChan,
		socket.eventLoopDataChan.readErrChan,
		socket.eventLoopDataChan.readCloseChan)
	return socket.eventLoopHandler(ctx, socket.eventLoopDataChan)
}

func invokeSocketCallBackFunc(s *socketIO) (err error) {
	if _, err = (s.runner.callbackFunction)(goja.Undefined(), s.runner.runtime.ToValue(s)); err != nil {
		_ = s.closeConnection(websocket.CloseGoingAway)
		return
	}
	return
}

func (s *socketIO) eventLoopHandler(ctx context.Context,
	eventLoopDataChan eventLoopDataChannel) (*WSHTTPResponse, error) {
	for {
		select {
		case pingData := <-eventLoopDataChan.pingChan:
			s.pingHandler(pingData)

		case pingID := <-eventLoopDataChan.pongChan:
			s.pongHandler(pingID)

		case readData := <-eventLoopDataChan.readDataChan:
			s.receiveDataHandler(string(readData))

		case readErr := <-eventLoopDataChan.readErrChan:
			s.readErrorHandler(readErr)

		case code := <-eventLoopDataChan.readCloseChan:
			s.closeConnectionHandler(code)

		case scheduledFn := <-s.scheduled:
			if err := s.gojaCallbackFuncHandler(scheduledFn); err != nil {
				return nil, err
			}

		case <-ctx.Done():
			// VU is shutting down during an interrupt
			// socket events will not be forwarded to the VU
			s.closeConnectionHandler(websocket.CloseGoingAway)

		case <-s.done:
			return s.doneHandler(ctx)
		}
	}
}

func (s *socketIO) startConnect() error {
	s.metrics.connectionStart = time.Now()
	err := s.connect()
	s.metrics.connectionEnd = time.Now()
	return err
}

func (s *socketIO) connect() error {
	conn, response, err := s.runner.dialer.Dial(s.runner.url.String(), s.runner.requestHeaders)
	if err != nil {
		errMsg := fmt.Sprintf("%s with url: %s", err.Error(), s.runner.url.String())
		s.handleEvent("error", s.runner.runtime.ToValue(err))
		return errors.New(errMsg)
	}
	s.pushSessionMetrics(response)
	wsResponse, wsRespErr := wrapHTTPResponse(response)
	if wsRespErr != nil {
		return wsRespErr
	}
	s.runner.conn = conn
	s.runner.response = wsResponse
	s.runner.response.URL = s.runner.url.String()
	return nil
}

func newWebSocketIO(initCtx context.Context, url string) socketIO {
	initRunner := newWebSocketIORunner(initCtx, url)
	initMetrics := newWebSocketIOMetrics()
	return socketIO{
		runner:            initRunner,
		metrics:           initMetrics,
		tags:              initRunner.state.Options.RunTags.CloneTags(),
		eventHandlers:     make(map[string][]goja.Callable),
		scheduled:         make(chan goja.Callable),
		done:              make(chan struct{}),
		eventLoopDataChan: newEventLoopData(),
	}
}

func newWebSocketIORunner(initCtx context.Context, rawURL string) socketIORunner {
	initRuntime, initState := initConnectState(initCtx)
	u, _ := url.Parse(rawURL)
	return socketIORunner{
		runtime:        initRuntime,
		requestHeaders: make(http.Header),
		ctx:            &initCtx,
		state:          initState,
		url:            u,
	}
}

func newWebSocketIOMetrics() socketIOMetrics {
	return socketIOMetrics{
		pingSendTimestamps: make(map[string]time.Time),
	}
}

func newEventLoopData() eventLoopDataChannel {
	return eventLoopDataChannel{
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

func (s *socketIO) setUpSocketIO(args ...goja.Value) {
	s.extractSocketOptions(args...)
	s.configureSocketOptions(s.runner.socketOptions)
	s.createSocketIODialer()
	s.initDefaultTags()
}

func (s *socketIO) extractSocketOptions(args ...goja.Value) {
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
			common.Throw(common.GetRuntime(*s.runner.ctx), errors.New("invalid argument types, allowing Map and Function types"))
			continue
		}
	}
	s.runner.callbackFunction = validateCallableParam(s.runner.ctx, callFunc)
}

func validateParamArguments(ctx context.Context, args ...goja.Value) {
	const (
		singleArg  = 1
		doubleArgs = 2
	)
	switch len(args) {
	case singleArg, doubleArgs:
		return
	default:
		common.Throw(common.GetRuntime(ctx),
			errors.New("invalid number of arguments to ws.connect, method is required 3 params ( url, params, callback )"))
	}
}

func validateCallableParam(ctx *context.Context, callableParam goja.Value) (setupFn goja.Callable) {
	callableFunc, isFunc := goja.AssertFunction(callableParam)
	if !isFunc {
		common.Throw(common.GetRuntime(*ctx), errors.New("last argument to ws.connect must be a function"))
	}
	return callableFunc
}

func (s *socketIO) configureSocketOptions(params goja.Value) {
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
			continue
		default:
			continue
		}
	}
}

func (s *socketIO) setSocketHeaders(paramsObject *goja.Object) {
	headersV := paramsObject.Get("headers")
	if goja.IsUndefined(headersV) || goja.IsNull(headersV) {
		return
	}
	headersObj := headersV.ToObject(s.runner.runtime)
	for _, key := range headersObj.Keys() {
		s.runner.requestHeaders.Set(key, headersObj.Get(key).String())
	}
}

func (s *socketIO) setSocketCookies(paramsObject *goja.Object) {
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

func (s *socketIO) setSocketTags(paramsObject *goja.Object) {
	tagsV := paramsObject.Get("tags")
	if goja.IsUndefined(tagsV) || goja.IsNull(tagsV) {
		return
	}
	tagsObj := tagsV.ToObject(s.runner.runtime)
	for _, key := range tagsObj.Keys() {
		s.tags[key] = tagsObj.Get(key).String()
	}
}

func (s *socketIO) createSocketIODialer() {
	jar, _ := cookiejar.New(nil)
	// Create NetDial with k6 context for using in case stub domain in unittest
	dialer := func(network, address string) (net.Conn, error) {
		return s.runner.state.Dialer.DialContext(*s.runner.ctx, network, address)
	}
	s.runner.dialer = &websocket.Dialer{
		Proxy:           http.ProxyFromEnvironment,
		TLSClientConfig: s.createTLSConfig(),
		Jar:             jar,
		NetDial:         dialer,
	}
}

func (s *socketIO) createTLSConfig() (tlsConfig *tls.Config) {
	if s.runner.state.TLSConfig != nil {
		tlsConfig = s.runner.state.TLSConfig.Clone()
		tlsConfig.NextProtos = []string{"http/1.1"}
	}
	return
}

func (s *socketIO) extractCookiesValues(cookiesObject *goja.Object) (
	requestCookies map[string]*httpext.HTTPRequestCookie) {
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

func (s *socketIO) initDefaultTags() {
	if s.runner.state.Options.SystemTags.Has(stats.TagURL) {
		s.tags["url"] = s.runner.url.String()
	}
	if s.runner.state.Options.SystemTags.Has(stats.TagGroup) {
		s.tags["group"] = s.runner.state.Group.Path
	}
}

func (s *socketIO) pingHandler(pingData string) {
	// Handle pings received from the server
	// - trigger the `ping` event
	// - reply with pong (needed when `SetPingHandler` is overwritten)
	err := s.runner.conn.WriteControl(websocket.PongMessage, []byte(pingData), time.Now().Add(writeWait))
	if err != nil {
		s.handleEvent("error", s.runner.runtime.ToValue(err))
	}
	s.handleEvent("ping")
}

func (s *socketIO) pongHandler(pingID string) {
	s.trackPong(pingID)
	s.handleEvent("pong")
}

func (s *socketIO) receiveDataHandler(readData string) {
	s.metrics.msgReceivedTimestamps = append(s.metrics.msgReceivedTimestamps, time.Now())
	event, responseData := parseResponse(readData, s.runner.runtime)
	s.handleEvent(event, responseData...)
}

func (s *socketIO) readErrorHandler(readErr error) {
	s.handleEvent("error", s.runner.runtime.ToValue(readErr))
}

func (s *socketIO) gojaCallbackFuncHandler(scheduledFn goja.Callable) (err error) {
	if _, err = scheduledFn(goja.Undefined()); err != nil {
		return err
	}
	return
}

func (s *socketIO) closeConnectionHandler(code int) {
	_ = s.closeConnection(code)
}

func (s *socketIO) doneHandler(ctx context.Context) (*WSHTTPResponse, error) {
	// This is the final exit point normally triggered by closeConnection
	end := time.Now()
	sessionDuration := stats.D(end.Sub(s.metrics.connectionStart))

	sampleTags := stats.IntoSampleTags(&s.tags)
	s.pushOverviewMetrics(ctx, sampleTags, sessionDuration)
	s.pushSentMetrics(ctx, sampleTags)
	s.pushReceivedMetrics(ctx, sampleTags)
	s.pushPingMetrics(ctx, sampleTags)
	return s.runner.response, nil
}

func (s *socketIO) pushSessionMetrics(response *http.Response) {
	if s.runner.state.Options.SystemTags.Has(stats.TagStatus) {
		s.tags["status"] = strconv.Itoa(response.StatusCode)
	}
	if s.runner.state.Options.SystemTags.Has(stats.TagSubproto) {
		s.tags["subproto"] = response.Header.Get("Sec-WebSocket-Protocol")
	}
}

func (s *socketIO) pushOverviewMetrics(ctx context.Context, sampleTags *stats.SampleTags, sessionDuration float64) {
	stats.PushIfNotDone(ctx, s.runner.state.Samples, stats.ConnectedSamples{
		Samples: []stats.Sample{
			{Metric: metrics.WSSessions, Time: s.metrics.connectionStart, Tags: sampleTags, Value: 1},
			{Metric: metrics.WSConnecting, Time: s.metrics.connectionStart, Tags: sampleTags,
				Value: stats.D(s.metrics.connectionEnd.Sub(s.metrics.connectionStart))},
			{Metric: metrics.WSSessionDuration, Time: s.metrics.connectionStart, Tags: sampleTags, Value: sessionDuration},
		},
		Tags: sampleTags,
		Time: s.metrics.connectionStart,
	})
}

func (s *socketIO) pushSentMetrics(ctx context.Context, sampleTags *stats.SampleTags) {
	for _, msgSentTimestamp := range s.metrics.msgSentTimestamps {
		stats.PushIfNotDone(ctx, s.runner.state.Samples, stats.Sample{
			Metric: metrics.WSMessagesSent,
			Time:   msgSentTimestamp,
			Tags:   sampleTags,
			Value:  1,
		})
	}
}

func (s *socketIO) pushReceivedMetrics(ctx context.Context, sampleTags *stats.SampleTags) {
	for _, msgReceivedTimestamp := range s.metrics.msgReceivedTimestamps {
		stats.PushIfNotDone(ctx, s.runner.state.Samples, stats.Sample{
			Metric: metrics.WSMessagesReceived,
			Time:   msgReceivedTimestamp,
			Tags:   sampleTags,
			Value:  1,
		})
	}
}

func (s *socketIO) pushPingMetrics(ctx context.Context, sampleTags *stats.SampleTags) {
	for _, pingDelta := range s.metrics.pingTimestamps {
		stats.PushIfNotDone(ctx, s.runner.state.Samples, stats.Sample{
			Metric: metrics.WSPing,
			Time:   pingDelta.pong,
			Tags:   sampleTags,
			Value:  stats.D(pingDelta.pong.Sub(pingDelta.ping)),
		})
	}
}

func (s *socketIO) close() {
	_ = s.runner.conn.Close()
}

func (s *socketIO) On(event string, handler goja.Value) {
	if handler, ok := goja.AssertFunction(handler); ok {
		s.eventHandlers[event] = append(s.eventHandlers[event], handler)
	}
}

func (s *socketIO) handleEvent(event string, args ...goja.Value) {
	s.eventHandlersProcess(event, args)
}

func (s *socketIO) eventHandlersProcess(event string, args []goja.Value) {
	s.handlersProcess(event, args)
}

func (s *socketIO) handlersProcess(event string, args []goja.Value) {
	if handlers, ok := s.eventHandlers[event]; ok {
		for _, handler := range handlers {
			if _, err := handler(goja.Undefined(), args...); err != nil {
				common.Throw(common.GetRuntime(*s.runner.ctx), err)
			}
		}
	}
}

func (s *socketIO) Send(event string, message goja.Value) {
	// NOTE: No binary message support for the time being since goja doesn't
	// support typed arrays.
	if missingArguments := len(event) == 0 || message == nil; missingArguments {
		common.Throw(common.GetRuntime(*s.runner.ctx),
			errors.New("invalid number of arguments to ws.send. Method is required 2 params ( channelName, message )"))
	}
	rt := common.GetRuntime(*s.runner.ctx)
	messageObject := message.ToObject(rt)
	var writeData []byte
	jsonByte, err := messageObject.MarshalJSON()
	if err != nil {
		s.handleEvent("error", rt.ToValue(err))
	}
	writeData = []byte(fmt.Sprintf("%s[\"%s\",%s]", commonMessageCode, event, string(jsonByte)))
	if err := s.runner.conn.WriteMessage(websocket.TextMessage, writeData); err != nil {
		s.handleEvent("error", rt.ToValue(err))
	}

	s.metrics.msgSentTimestamps = append(s.metrics.msgSentTimestamps, time.Now())
}

func (s *socketIO) trackPong(pingID string) {
	pongTimestamp := time.Now()

	if _, ok := s.metrics.pingSendTimestamps[pingID]; !ok {
		// We received a pong for a ping we didn't send; ignore
		// (this shouldn't happen with a compliant server)
		return
	}
	pingTimestamp := s.metrics.pingSendTimestamps[pingID]

	s.metrics.pingTimestamps = append(s.metrics.pingTimestamps, pingDelta{pingTimestamp, pongTimestamp})
}

func (s *socketIO) closeConnection(code int) error {
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

func (s *socketIO) SetTimeout(fn goja.Callable, timeoutMs int) {
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

func (s *socketIO) SetInterval(fn goja.Callable, intervalMs int) {
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

func (s *socketIO) Close(args ...goja.Value) {
	code := websocket.CloseGoingAway
	if len(args) > 0 {
		code = int(args[0].ToInteger())
	}

	_ = s.closeConnection(code)
}

func (s *socketIO) Ping() {
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

func parseResponse(rawResponse string, rt *goja.Runtime) (eventName string, messageResponse []goja.Value) {
	eventCode := getEventCode(rawResponse)
	eventName, responseData, err := getEventData(eventCode, rawResponse)
	if err != nil {
		common.Throw(rt, err)
	}
	messageResponse = []goja.Value{rt.ToValue(responseData)}
	return
}

func getEventCode(rawResponse string) (eventCode string) {
	eventCode = rawResponse[0:1]
	switch eventCode {
	case messageCode:
		return rawResponse[0:2]
	default:
		return
	}
}

func getEventData(eventCode, rawResponse string) (eventName, restText string, err error) {
	var start, end, rest int
	switch eventCode {
	case openCode:
		return "open", rawResponse, nil
	case emptyMessageCode:
		return "emptyMessage", rawResponse, nil
	case pongCode:
		return "pong", rawResponse, nil
	case pingCode:
		return "ping", rawResponse, nil
	default:
		start, end, rest, err = decodeData(rawResponse)
		invalidPacket := err != nil || (end < start) || (rest >= len(rawResponse))
		if invalidPacket {
			return "", "", errors.New("wrong packet")
		}
	}
	// Index -2 from the string to check if the character is double quote or not
	isStringResponse := strings.HasPrefix(string(rawResponse[rest]), "\"") &&
		strings.HasSuffix(string(rawResponse[len(rawResponse)-2]), "\"")
	if isStringResponse {
		return rawResponse[start:end], rawResponse[rest+1 : len(rawResponse)-2], nil
	}
	return rawResponse[start:end], rawResponse[rest : len(rawResponse)-1], nil
}

func decodeData(rawResponse string) (start, end, rest int, err error) {
	var countQuote int
	const (
		startQuote   = 0
		endQuote     = 1
		maximumQuote = 2
	)

	for i, c := range rawResponse {
		if c == '"' {
			switch countQuote {
			case startQuote:
				start = i + 1
			case endQuote:
				end = i
				rest = i + 1
			default:
				return 0, 0, 0, errors.New("wrong packet")
			}
			countQuote++
		}
		if c == ',' {
			if countQuote < maximumQuote {
				continue
			}
			rest = i + 1
			break
		}
	}
	return
}
