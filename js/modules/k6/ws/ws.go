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
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"
	"github.com/gorilla/websocket"
	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/stats"
)

// ErrWSInInitContext is returned when websockets are using in the init context
var ErrWSInInitContext = common.NewInitContextError("using websockets in the init context is not supported")

type WS struct{}

type pingDelta struct {
	ping time.Time
	pong time.Time
}

type Socket struct {
	ctx           context.Context
	conn          *websocket.Conn
	eventHandlers map[string][]goja.Callable
	scheduled     chan goja.Callable
	done          chan struct{}
	shutdownOnce  sync.Once

	msgSentTimestamps     []time.Time
	msgReceivedTimestamps []time.Time

	pingSendTimestamps map[string]time.Time
	pingSendCounter    int
	pingTimestamps     []pingDelta
}

type WSHTTPResponse struct {
	URL     string            `json:"url"`
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
	Error   string            `json:"error"`
}

const writeWait = 10 * time.Second

func New() *WS {
	return &WS{}
}

func (*WS) Connect(ctx context.Context, url string, args ...goja.Value) (*WSHTTPResponse, error) {
	rt := common.GetRuntime(ctx)
	state := lib.GetState(ctx)
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

	// Leave header to nil by default so we can pass it directly to the Dialer
	var header http.Header

	tags := state.Options.RunTags.CloneTags()

	// Parse the optional second argument (params)
	if !goja.IsUndefined(paramsV) && !goja.IsNull(paramsV) {
		params := paramsV.ToObject(rt)
		for _, k := range params.Keys() {
			switch k {
			case "headers":
				header = http.Header{}
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
			}
		}

	}

	if state.Options.SystemTags.Has(stats.TagURL) {
		tags["url"] = url
	}
	if state.Options.SystemTags.Has(stats.TagGroup) {
		tags["group"] = state.Group.Path
	}

	// Pass a custom net.Dial function to websocket.Dialer that will substitute
	// the underlying net.Conn with our own tracked netext.Conn
	netDial := func(network, address string) (net.Conn, error) {
		return state.Dialer.DialContext(ctx, network, address)
	}

	// Overriding the NextProtos to avoid talking http2
	var tlsConfig *tls.Config
	if state.TLSConfig != nil {
		tlsConfig = state.TLSConfig.Clone()
		tlsConfig.NextProtos = []string{"http/1.1"}
	}

	wsd := websocket.Dialer{
		NetDial:         netDial,
		Proxy:           http.ProxyFromEnvironment,
		TLSClientConfig: tlsConfig,
	}

	start := time.Now()
	conn, httpResponse, connErr := wsd.Dial(url, header)
	connectionEnd := time.Now()
	connectionDuration := stats.D(connectionEnd.Sub(start))

	socket := Socket{
		ctx:                ctx,
		conn:               conn,
		eventHandlers:      make(map[string][]goja.Callable),
		pingSendTimestamps: make(map[string]time.Time),
		scheduled:          make(chan goja.Callable),
		done:               make(chan struct{}),
	}

	if state.Options.SystemTags.Has(stats.TagIP) && conn.RemoteAddr() != nil {
		if ip, _, err := net.SplitHostPort(conn.RemoteAddr().String()); err == nil {
			tags["ip"] = ip
		}
	}

	if connErr != nil {
		// Pass the error to the user script before exiting immediately
		socket.handleEvent("error", rt.ToValue(connErr))

		return nil, connErr
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

	if state.Options.SystemTags.Has(stats.TagStatus) {
		tags["status"] = strconv.Itoa(httpResponse.StatusCode)
	}
	if state.Options.SystemTags.Has(stats.TagSubProto) {
		tags["subproto"] = httpResponse.Header.Get("Sec-WebSocket-Protocol")
	}

	// The connection is now open, emit the event
	socket.handleEvent("open")

	// Pass ping/pong events through the main control loop
	pingChan := make(chan string)
	pongChan := make(chan string)
	conn.SetPingHandler(func(msg string) error { pingChan <- msg; return nil })
	conn.SetPongHandler(func(pingID string) error { pongChan <- pingID; return nil })

	readDataChan := make(chan []byte)
	readCloseChan := make(chan int)
	readErrChan := make(chan error)

	// Wraps a couple of channels around conn.ReadMessage
	go readPump(conn, readDataChan, readErrChan, readCloseChan)

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

		case readData := <-readDataChan:
			socket.msgReceivedTimestamps = append(socket.msgReceivedTimestamps, time.Now())
			socket.handleEvent("message", rt.ToValue(string(readData)))

		case readErr := <-readErrChan:
			socket.handleEvent("error", rt.ToValue(readErr))

		case readClose := <-readCloseChan:
			// handle server close
			socket.handleEvent("close", rt.ToValue(readClose))

		case scheduledFn := <-socket.scheduled:
			if _, err := scheduledFn(goja.Undefined()); err != nil {
				return nil, err
			}

		case <-ctx.Done():
			// VU is shutting down during an interrupt
			// socket events will not be forwarded to the VU
			_ = socket.closeConnection(websocket.CloseGoingAway)

		case <-socket.done:
			// This is the final exit point normally triggered by closeConnection
			end := time.Now()
			sessionDuration := stats.D(end.Sub(start))

			sampleTags := stats.IntoSampleTags(&tags)

			stats.PushIfNotCancelled(ctx, state.Samples, stats.ConnectedSamples{
				Samples: []stats.Sample{
					{Metric: metrics.WSSessions, Time: start, Tags: sampleTags, Value: 1},
					{Metric: metrics.WSConnecting, Time: start, Tags: sampleTags, Value: connectionDuration},
					{Metric: metrics.WSSessionDuration, Time: start, Tags: sampleTags, Value: sessionDuration},
				},
				Tags: sampleTags,
				Time: start,
			})

			for _, msgSentTimestamp := range socket.msgSentTimestamps {
				stats.PushIfNotCancelled(ctx, state.Samples, stats.Sample{
					Metric: metrics.WSMessagesSent,
					Time:   msgSentTimestamp,
					Tags:   sampleTags,
					Value:  1,
				})
			}

			for _, msgReceivedTimestamp := range socket.msgReceivedTimestamps {
				stats.PushIfNotCancelled(ctx, state.Samples, stats.Sample{
					Metric: metrics.WSMessagesReceived,
					Time:   msgReceivedTimestamp,
					Tags:   sampleTags,
					Value:  1,
				})
			}

			for _, pingDelta := range socket.pingTimestamps {
				stats.PushIfNotCancelled(ctx, state.Samples, stats.Sample{
					Metric: metrics.WSPing,
					Time:   pingDelta.pong,
					Tags:   sampleTags,
					Value:  stats.D(pingDelta.pong.Sub(pingDelta.ping)),
				})
			}

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
				common.Throw(common.GetRuntime(s.ctx), err)
			}
		}
	}
}

func (s *Socket) Send(message string) {
	// NOTE: No binary message support for the time being since goja doesn't
	// support typed arrays.
	rt := common.GetRuntime(s.ctx)

	writeData := []byte(message)
	if err := s.conn.WriteMessage(websocket.TextMessage, writeData); err != nil {
		s.handleEvent("error", rt.ToValue(err))
	}

	s.msgSentTimestamps = append(s.msgSentTimestamps, time.Now())
}

func (s *Socket) Ping() {
	rt := common.GetRuntime(s.ctx)
	deadline := time.Now().Add(writeWait)
	pingID := strconv.Itoa(s.pingSendCounter)
	data := []byte(pingID)

	err := s.conn.WriteControl(websocket.PingMessage, data, deadline)
	if err != nil {
		s.handleEvent("error", rt.ToValue(err))
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

	s.pingTimestamps = append(s.pingTimestamps, pingDelta{pingTimestamp, pongTimestamp})
}

func (s *Socket) SetTimeout(fn goja.Callable, timeoutMs int) {
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

func (s *Socket) SetInterval(fn goja.Callable, intervalMs int) {
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

func (s *Socket) Close(args ...goja.Value) {
	code := websocket.CloseGoingAway
	if len(args) > 0 {
		code = int(args[0].ToInteger())
	}

	_ = s.closeConnection(code)
}

// Attempts to close the websocket gracefully
func (s *Socket) closeConnection(code int) error {
	var err error

	s.shutdownOnce.Do(func() {
		rt := common.GetRuntime(s.ctx)

		err = s.conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(code, ""),
			time.Now().Add(writeWait),
		)
		if err != nil {
			// Just call the handler, we'll try to close the connection anyway
			s.handleEvent("error", rt.ToValue(err))
		}

		// trigger `close` event when the client closes the connection
		s.handleEvent("close", rt.ToValue(code))
		_ = s.conn.Close()

		// Stops the main control loop
		close(s.done)
	})

	return err
}

// Wraps conn.ReadMessage in a channel
func readPump(conn *websocket.Conn, readChan chan []byte, errorChan chan error, closeChan chan int) {
	defer func() { _ = conn.Close() }()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {

			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				closeChan <- err.(*websocket.CloseError).Code
			} else if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				// Emit the error if it is not CloseNormalClosure
				// and the error is not  originated from closing the socket ourselves with `CloseGoingAway`
				errorChan <- err
			}

			//CloseGoingAway errors are ignored
			return
		}

		readChan <- message
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
