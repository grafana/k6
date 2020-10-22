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
	"github.com/loadimpact/k6/js/internal/modules"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/stats"
)

func init() {
	modules.Register("k6/ws", New())
}

// ErrWSInInitContext is returned when websockets are using in the init context
var ErrWSInInitContext = common.NewInitContextError("using websockets in the init context is not supported")

type WS struct{}

type Socket struct {
	ctx           context.Context
	conn          *websocket.Conn
	eventHandlers map[string][]goja.Callable
	scheduled     chan goja.Callable
	done          chan struct{}
	shutdownOnce  sync.Once

	pingSendTimestamps map[string]time.Time
	pingSendCounter    int

	sampleTags    *stats.SampleTags
	samplesOutput chan<- stats.SampleContainer
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

	tags := state.CloneTags()

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
		NetDialContext:  state.Dialer.DialContext,
		Proxy:           http.ProxyFromEnvironment,
		TLSClientConfig: tlsConfig,
	}

	start := time.Now()
	conn, httpResponse, connErr := wsd.DialContext(ctx, url, header)
	connectionEnd := time.Now()
	connectionDuration := stats.D(connectionEnd.Sub(start))

	if state.Options.SystemTags.Has(stats.TagIP) && conn.RemoteAddr() != nil {
		if ip, _, err := net.SplitHostPort(conn.RemoteAddr().String()); err == nil {
			tags["ip"] = ip
		}
	}

	if httpResponse != nil {
		if state.Options.SystemTags.Has(stats.TagStatus) {
			tags["status"] = strconv.Itoa(httpResponse.StatusCode)
		}

		if state.Options.SystemTags.Has(stats.TagSubproto) {
			tags["subproto"] = httpResponse.Header.Get("Sec-WebSocket-Protocol")
		}
	}

	socket := Socket{
		ctx:                ctx,
		conn:               conn,
		eventHandlers:      make(map[string][]goja.Callable),
		pingSendTimestamps: make(map[string]time.Time),
		scheduled:          make(chan goja.Callable),
		done:               make(chan struct{}),
		samplesOutput:      state.Samples,
		sampleTags:         stats.IntoSampleTags(&tags),
	}

	stats.PushIfNotDone(ctx, state.Samples, stats.ConnectedSamples{
		Samples: []stats.Sample{
			{Metric: metrics.WSSessions, Time: start, Tags: socket.sampleTags, Value: 1},
			{Metric: metrics.WSConnecting, Time: start, Tags: socket.sampleTags, Value: connectionDuration},
		},
		Tags: socket.sampleTags,
		Time: start,
	})

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

	readDataChan := make(chan []byte)
	readCloseChan := make(chan int)
	readErrChan := make(chan error)

	// Wraps a couple of channels around conn.ReadMessage
	go socket.readPump(readDataChan, readErrChan, readCloseChan)

	// we do it here as below we can panic, which translates to an exception in js code
	defer func() {
		socket.Close() // just in case
		end := time.Now()
		sessionDuration := stats.D(end.Sub(start))

		stats.PushIfNotDone(ctx, state.Samples, stats.Sample{
			Metric: metrics.WSSessionDuration,
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

		case readData := <-readDataChan:
			stats.PushIfNotDone(ctx, socket.samplesOutput, stats.Sample{
				Metric: metrics.WSMessagesReceived,
				Time:   time.Now(),
				Tags:   socket.sampleTags,
				Value:  1,
			})
			socket.handleEvent("message", rt.ToValue(string(readData)))

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

	stats.PushIfNotDone(s.ctx, s.samplesOutput, stats.Sample{
		Metric: metrics.WSMessagesSent,
		Time:   time.Now(),
		Tags:   s.sampleTags,
		Value:  1,
	})
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

	stats.PushIfNotDone(s.ctx, s.samplesOutput, stats.Sample{
		Metric: metrics.WSPing,
		Time:   pongTimestamp,
		Tags:   s.sampleTags,
		Value:  stats.D(pongTimestamp.Sub(pingTimestamp)),
	})
}

// SetTimeout executes the provided function inside the socket's event loop after at least the provided
// timeout, which is in ms, has elapsed
func (s *Socket) SetTimeout(fn goja.Callable, timeoutMs float64) error {
	// Starts a goroutine, blocks once on the timeout and pushes the callable
	// back to the main loop through the scheduled channel
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
	// back to the main loop through the scheduled channel
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
		rt := common.GetRuntime(s.ctx)

		err = s.conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(code, ""),
			time.Now().Add(writeWait),
		)
		if err != nil {
			// Call the user-defined error handler
			s.handleEvent("error", rt.ToValue(err))
		}

		// Call the user-defined close handler
		s.handleEvent("close", rt.ToValue(code))
	})

	return err
}

// Wraps conn.ReadMessage in a channel
func (s *Socket) readPump(readChan chan []byte, errorChan chan error, closeChan chan int) {
	for {
		_, message, err := s.conn.ReadMessage()
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
		case readChan <- message:
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
