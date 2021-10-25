/*
 *
 * xk6-browser - a browser automation extension for k6
 * Copyright (C) 2021 Load Impact
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

package common

import (
	"crypto/tls"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chromedp/cdproto"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/target"
	"github.com/dop251/goja"
	"github.com/gorilla/websocket"
	"github.com/mailru/easyjson"
	"github.com/mailru/easyjson/jlexer"
	"github.com/mailru/easyjson/jwriter"
	"golang.org/x/net/context"
)

const wsWriteBufferSize = 1 << 20

// Ensure Connection implements the EventEmitter and Executor interfaces
var _ EventEmitter = &Connection{}
var _ cdp.Executor = &Connection{}

// Action is the general interface of an CDP action.
type Action interface {
	Do(context.Context) error
}

// ActionFunc is an adapter to allow regular functions to be used as an Action.
type ActionFunc func(context.Context) error

// Do executes the func f using the provided context.
func (f ActionFunc) Do(ctx context.Context) error {
	return f(ctx)
}

/*
	Connection represents a WebSocket connection and the root "Browser Session".

	                                      ┌───────────────────────────────────────────────────────────────────┐
                                          │                                                                   │
                                          │                          Browser Process                          │
                                          │                                                                   │
                                          └───────────────────────────────────────────────────────────────────┘
┌───────────────────────────┐                                           │      ▲
│Reads JSON-RPC CDP messages│                                           │      │
│from WS connection and puts│                                           ▼      │
│ them on incoming queue of │             ┌───────────────────────────────────────────────────────────────────┐
│    target session, as     ├─────────────■                                                                   │
│   identified by message   │             │                       WebSocket Connection                        │
│   session ID. Messages    │             │                                                                   │
│ without a session ID are  │             └───────────────────────────────────────────────────────────────────┘
│considered to belong to the│                    │      ▲                                       │      ▲
│  root "Browser Session".  │                    │      │                                       │      │
└───────────────────────────┘                    ▼      │                                       ▼      │
┌───────────────────────────┐             ┌────────────────────┐                         ┌────────────────────┐
│  Handles CDP messages on  ├─────────────■                    │                         │                    │
│incoming queue and puts CDP│             │      Session       │      *  *  *  *  *      │      Session       │
│   messages on outgoing    │             │                    │                         │                    │
│ channel of WS connection. │             └────────────────────┘                         └────────────────────┘
└───────────────────────────┘                    │      ▲                                       │      ▲
                                                 │      │                                       │      │
                                                 ▼      │                                       ▼      │
┌───────────────────────────┐             ┌────────────────────┐                         ┌────────────────────┐
│Registers with session as a├─────────────■                    │                         │                    │
│handler for a specific CDP │             │   Event Listener   │      *  *  *  *  *      │   Event Listener   │
│       Domain event.       │             │                    │                         │                    │
└───────────────────────────┘             └────────────────────┘                         └────────────────────┘
*/
type Connection struct {
	BaseEventEmitter

	ctx          context.Context
	wsURL        string
	logger       *Logger
	conn         *websocket.Conn
	sendCh       chan *cdproto.Message
	recvCh       chan *cdproto.Message
	closeCh      chan int
	errorCh      chan error
	done         chan struct{}
	shutdownOnce sync.Once
	msgID        int64

	sessionsMu sync.RWMutex
	sessions   map[target.SessionID]*Session

	// Reuse the easyjson structs to avoid allocs per Read/Write.
	decoder jlexer.Lexer
	encoder jwriter.Writer
}

// NewConnection creates a new browser
func NewConnection(ctx context.Context, wsURL string, logger *Logger) (*Connection, error) {
	var header http.Header
	var tlsConfig *tls.Config
	wsd := websocket.Dialer{
		HandshakeTimeout: time.Second * 60,
		Proxy:            http.ProxyFromEnvironment, // TODO(fix): use proxy settings from launch options
		TLSClientConfig:  tlsConfig,
		WriteBufferSize:  wsWriteBufferSize,
	}

	conn, _, connErr := wsd.DialContext(ctx, wsURL, header)
	if connErr != nil {
		return nil, connErr
	}

	c := Connection{
		BaseEventEmitter: NewBaseEventEmitter(),
		ctx:              ctx,
		wsURL:            wsURL,
		logger:           logger,
		conn:             conn,
		sendCh:           make(chan *cdproto.Message, 32), // Avoid blocking in Execute
		recvCh:           make(chan *cdproto.Message),
		closeCh:          make(chan int),
		errorCh:          make(chan error),
		done:             make(chan struct{}),
		msgID:            0,
		sessions:         make(map[target.SessionID]*Session),
	}

	go c.recvLoop()
	go c.sendLoop()

	return &c, nil
}

// closeConnection cleanly closes the WebSocket connection.
// Returns an error if sending the close control frame fails.
func (c *Connection) closeConnection(code int) error {
	var err error

	c.shutdownOnce.Do(func() {
		defer func() {
			_ = c.conn.Close()

			// Stop the main control loop
			close(c.done)
		}()

		err = c.conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(code, ""),
			time.Now().Add(10*time.Second),
		)

		c.sessionsMu.Lock()
		for _, s := range c.sessions {
			s.close()
			delete(c.sessions, s.id)
		}
		c.sessionsMu.Unlock()

		c.emit(EventConnectionClose, nil)
	})

	return err
}

func (c *Connection) closeSession(sessionID target.SessionID) {
	c.sessionsMu.Lock()
	if session, ok := c.sessions[sessionID]; ok {
		session.close()
	}
	delete(c.sessions, sessionID)
	c.sessionsMu.Unlock()
}

func (c *Connection) createSession(info *target.Info) (*Session, error) {
	var sessionID target.SessionID
	var err error
	action := target.AttachToTarget(info.TargetID).WithFlatten(true)
	if sessionID, err = action.Do(cdp.WithExecutor(c.ctx, c)); err != nil {
		return nil, err
	}
	return c.getSession(sessionID), nil
}

func (c *Connection) handleIOError(err error) {
	if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
		// Report an unexpected closure
		select {
		case c.errorCh <- err:
		case <-c.done:
			return
		}
	}
	code := websocket.CloseGoingAway
	if e, ok := err.(*websocket.CloseError); ok {
		code = e.Code
	}
	select {
	case c.closeCh <- code:
	case <-c.done:
	}
}

func (c *Connection) getSession(id target.SessionID) *Session {
	c.sessionsMu.RLock()
	defer c.sessionsMu.RUnlock()
	return c.sessions[id]
}

func (c *Connection) recvLoop() {
	for {
		_, buf, err := c.conn.ReadMessage()
		if err != nil {
			c.handleIOError(err)
			return
		}

		c.logger.Debugf("cdp:recv", "<- %s", buf)

		var msg cdproto.Message
		c.decoder = jlexer.Lexer{Data: buf}
		msg.UnmarshalEasyJSON(&c.decoder)
		if err := c.decoder.Error(); err != nil {
			select {
			case c.errorCh <- err:
			case <-c.done:
				return
			}
		}

		if msg.Method != "" {
			// Handle attachment and detachment from targets,
			// creating and deleting sessions as necessary.
			if msg.Method == cdproto.EventTargetAttachedToTarget {
				ev, err := cdproto.UnmarshalMessage(&msg)
				if err != nil {
					c.logger.Errorf("cdp", "%s", err)
					continue
				}
				sessionID := ev.(*target.EventAttachedToTarget).SessionID
				c.sessionsMu.Lock()
				session := NewSession(c.ctx, c, sessionID)
				c.sessions[sessionID] = session
				c.sessionsMu.Unlock()
			} else if msg.Method == cdproto.EventTargetDetachedFromTarget {
				ev, err := cdproto.UnmarshalMessage(&msg)
				if err != nil {
					c.logger.Errorf("cdp", "%s", err)
					continue
				}
				sessionID := ev.(*target.EventDetachedFromTarget).SessionID
				c.closeSession(sessionID)
			}
		}

		switch {
		case msg.SessionID != "" && (msg.Method != "" || msg.ID != 0):
			if session, ok := c.sessions[msg.SessionID]; ok {
				if msg.Error != nil && msg.Error.Message == "No session with given id" {
					c.closeSession(session.id)
					continue
				}

				select {
				case session.readCh <- &msg:
				case code := <-c.closeCh:
					_ = c.closeConnection(code)
				case <-c.done:
					return
				}
			}

		case msg.Method != "":
			ev, err := cdproto.UnmarshalMessage(&msg)
			if err != nil {
				c.logger.Errorf("cdp", "%s", err)
				continue
			}
			c.emit(string(msg.Method), ev)

		case msg.ID != 0:
			c.emit("", &msg)

		default:
			c.logger.Errorf("cdp", "ignoring malformed incoming message (missing id or method): %#v (message: %s)", msg, msg.Error.Message)
		}
	}
}

func (c *Connection) send(msg *cdproto.Message, recvCh chan *cdproto.Message, res easyjson.Unmarshaler) error {
	select {
	case c.sendCh <- msg:
	case err := <-c.errorCh:
		return err
	case code := <-c.closeCh:
		_ = c.closeConnection(code)
		return &websocket.CloseError{Code: code}
	case <-c.done:
	}

	if recvCh != nil {
		// Block waiting for response.
		select {
		case msg := <-recvCh:
			switch {
			case msg == nil:
				return ErrChannelClosed
			case msg.Error != nil:
				return msg.Error
			case res != nil:
				return easyjson.Unmarshal(msg.Result, res)
			}
		case err := <-c.errorCh:
			return err
		case code := <-c.closeCh:
			_ = c.closeConnection(code)
			return &websocket.CloseError{Code: code}
		case <-c.done:
		}
	}

	return nil
}

func (c *Connection) sendLoop() {
	for {
		select {
		case msg := <-c.sendCh:
			c.encoder = jwriter.Writer{}
			msg.MarshalEasyJSON(&c.encoder)
			if err := c.encoder.Error; err != nil {
				select {
				case c.errorCh <- err:
				case <-c.done:
					return
				}
			}

			buf, _ := c.encoder.BuildBytes()
			c.logger.Debugf("cdp:send", "-> %s", buf)
			writer, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				c.handleIOError(err)
				return
			}
			if _, err := writer.Write(buf); err != nil {
				c.handleIOError(err)
				return
			}
			if err := writer.Close(); err != nil {
				c.handleIOError(err)
				return
			}
		case code := <-c.closeCh:
			_ = c.closeConnection(code)
		case <-c.done:
		}
	}
}

func (c *Connection) Close(args ...goja.Value) {
	code := websocket.CloseGoingAway
	if len(args) > 0 {
		code = int(args[0].ToInteger())
	}
	_ = c.closeConnection(code)
}

// Execute implements cdproto.Executor and performs a synchronous send and receive
func (c *Connection) Execute(ctx context.Context, method string, params easyjson.Marshaler, res easyjson.Unmarshaler) error {
	id := atomic.AddInt64(&c.msgID, 1)

	// Setup event handler used to block for response to message being sent.
	ch := make(chan *cdproto.Message, 1)
	evCancelCtx, evCancelFn := context.WithCancel(ctx)
	chEvHandler := make(chan Event)
	go func() {
		for {
			select {
			case <-evCancelCtx.Done():
				return
			case ev := <-chEvHandler:
				msg, ok := ev.data.(*cdproto.Message)
				if ok && msg.ID == id {
					select {
					case <-evCancelCtx.Done():
					case ch <- msg:
						// We expect only one response with the matching message ID,
						// then remove event handler by cancelling context and stopping goroutine.
						evCancelFn()
						return
					}
				}
			}
		}
	}()
	c.onAll(evCancelCtx, chEvHandler)
	defer evCancelFn() // Remove event handler

	// Send the message
	var buf []byte
	if params != nil {
		var err error
		buf, err = easyjson.Marshal(params)
		if err != nil {
			return err
		}
	}
	msg := &cdproto.Message{
		ID:     id,
		Method: cdproto.MethodType(method),
		Params: buf,
	}
	return c.send(msg, ch, res)
}
