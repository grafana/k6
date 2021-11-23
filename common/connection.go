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
	"context"
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
		BaseEventEmitter: NewBaseEventEmitter(ctx),
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

func (c *Connection) closeSession(sid target.SessionID, tid target.ID) {
	c.logger.Errorf("Connection:closeSession", "sid:%v tid:%v wsURL:%v", sid, tid, c.wsURL)
	c.sessionsMu.Lock()
	if session, ok := c.sessions[sid]; ok {
		session.close()
	}
	delete(c.sessions, sid)
	c.sessionsMu.Unlock()
}

func (c *Connection) createSession(info *target.Info) (*Session, error) {
	c.logger.Errorf("Connection:createSession", "")

	var sessionID target.SessionID
	var err error
	action := target.AttachToTarget(info.TargetID).WithFlatten(true)
	if sessionID, err = action.Do(cdp.WithExecutor(c.ctx, c)); err != nil {
		c.logger.Errorf("Connection:createSession", "err:%v", err)
		return nil, err
	}
	c.logger.Errorf("Connection:createSession", "sid:%v", sessionID)
	return c.getSession(sessionID), nil
}

func (c *Connection) handleIOError(err error) {
	c.logger.Errorf("Connection:handleIOError", "err:%v", err)

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
		c.logger.Errorf("Connection:handleIOError:c.closeCh <-", "code:%d", code)
	case <-c.done:
		c.logger.Errorf("Connection:handleIOError:<-c.done", "")
	}
}

func (c *Connection) getSession(id target.SessionID) *Session {
	c.sessionsMu.RLock()
	defer c.sessionsMu.RUnlock()
	return c.sessions[id]
}

func (c *Connection) findSessionTargetID(id target.SessionID) target.ID {
	c.sessionsMu.RLock()
	defer c.sessionsMu.RUnlock()
	s, ok := c.sessions[id]
	if !ok {
		return ""
	}
	return s.targetID
}

func (c *Connection) recvLoop() {
	c.logger.Infof("Connection:recvLoop", "wsURL:%q", c.wsURL)
	for {
		_, buf, err := c.conn.ReadMessage()
		if err != nil {
			c.handleIOError(err)
			c.logger.Infof("Connection:recvLoop", "wsURL:%q ioErr:%v", c.wsURL, err)
			return
		}

		c.logger.Debugf("cdp:recv", "<- %s", buf)

		var msg cdproto.Message
		c.decoder = jlexer.Lexer{Data: buf}
		msg.UnmarshalEasyJSON(&c.decoder)
		if err := c.decoder.Error(); err != nil {
			select {
			case c.errorCh <- err:
				c.logger.Infof("Connection:recvLoop:<-err", "wsURL:%q err:%v", c.wsURL, err)
			case <-c.done:
				c.logger.Infof("Connection:recvLoop:<-c.done", "wsURL:%q", c.wsURL)
				return
			}
		}

		// Handle attachment and detachment from targets,
		// creating and deleting sessions as necessary.
		if msg.Method == cdproto.EventTargetAttachedToTarget {
			ev, err := cdproto.UnmarshalMessage(&msg)
			if err != nil {
				c.logger.Errorf("cdp", "%s", err)
				continue
			}
			eva := ev.(*target.EventAttachedToTarget)
			sid, tid := eva.SessionID, eva.TargetInfo.TargetID
			c.sessionsMu.Lock()
			session := NewSession(c.ctx, c, sid, tid)
			c.logger.Infof("Connection:recvLoop:EventAttachedToTarget", "sid:%v tid:%v wsURL:%q, NewSession", sid, tid, c.wsURL)
			c.sessions[sid] = session
			c.sessionsMu.Unlock()
		} else if msg.Method == cdproto.EventTargetDetachedFromTarget {
			ev, err := cdproto.UnmarshalMessage(&msg)
			if err != nil {
				c.logger.Errorf("cdp", "%s", err)
				continue
			}
			evt := ev.(*target.EventDetachedFromTarget)
			sid := evt.SessionID
			tid := c.findSessionTargetID(sid)
			c.logger.Errorf("Connection:recvLoop:EventDetachedFromTarget", "sid:%v tid:%v wsURL:%q, closeSession", sid, tid, c.wsURL)
			c.closeSession(sid, tid)
		}

		switch {
		case msg.SessionID != "" && (msg.Method != "" || msg.ID != 0):
			// TODO: possible data race - session can get removed after getting it here
			session := c.getSession(msg.SessionID)
			if session == nil {
				continue
			}
			if msg.Error != nil && msg.Error.Message == "No session with given id" {
				c.logger.Errorf("Connection:recvLoop", "sid:%v tid:%v wsURL:%q, closeSession #2", session.id, session.targetID, c.wsURL)
				c.closeSession(session.id, session.targetID)
				continue
			}

			select {
			case session.readCh <- &msg:
				// c.logger.Errorf("Connection:recvLoop:session.readCh<-", "sid=%v wsURL=%v crashed:%t", session.id, c.wsURL, session.crashed)
			case code := <-c.closeCh:
				c.logger.Errorf("Connection:recvLoop:<-c.closeCh", "sid:%v tid:%v wsURL:%v crashed:%t", session.id, session.targetID, c.wsURL, session.crashed)
				_ = c.closeConnection(code)
			case <-c.done:
				c.logger.Errorf("Connection:recvLoop:<-c.done", "sid:%v tid:%v wsURL:%v crashed:%t", session.id, session.targetID, c.wsURL, session.crashed)
				return
			}

		case msg.Method != "":
			c.logger.Errorf("Connection:recvLoop:msg.Method:emit", "method=%q", msg.Method)
			ev, err := cdproto.UnmarshalMessage(&msg)
			if err != nil {
				c.logger.Errorf("cdp", "%s", err)
				continue
			}
			c.emit(string(msg.Method), ev)

		case msg.ID != 0:
			c.logger.Errorf("Connection:recvLoop:msg.ID:emit", "method=%q", msg.ID)
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
		c.logger.Errorf("Connection:send:<-c.errorCh", "wsURL:%q sid:%v, err:%v", c.wsURL, msg.SessionID, err)
		return err
	case code := <-c.closeCh:
		c.logger.Errorf("Connection:send:<-c.closeCh", "wsURL:%q sid:%v, websocket code:%v", c.wsURL, msg.SessionID, code)
		_ = c.closeConnection(code)
		return &websocket.CloseError{Code: code}
	case <-c.done:
		c.logger.Errorf("Connection:send:<-c.done", "wsURL:%q sid:%v", c.wsURL, msg.SessionID)
		return nil
	}

	// Block waiting for response.
	if recvCh == nil {
		return nil
	}
	select {
	case msg := <-recvCh:
		var (
			sid target.SessionID
			tid target.ID
		)
		if msg != nil {
			sid = msg.SessionID
			tid = c.findSessionTargetID(sid)
		}
		switch {
		case msg == nil:
			c.logger.Errorf("Connection:send", "wsURL:%q, err:ErrChannelClosed", c.wsURL)
			return ErrChannelClosed
		case msg.Error != nil:
			c.logger.Errorf("Connection:send", "sid:%v tid:%v wsURL:%q, msg err:%v", sid, tid, c.wsURL, msg.Error)
			return msg.Error
		case res != nil:
			return easyjson.Unmarshal(msg.Result, res)
		}
	case err := <-c.errorCh:
		tid := c.findSessionTargetID(msg.SessionID)
		c.logger.Errorf("Connection:send:<-c.errorCh #2", "sid:%v tid:%v wsURL:%q, err:%v", msg.SessionID, tid, c.wsURL, err)
		return err
	case code := <-c.closeCh:
		tid := c.findSessionTargetID(msg.SessionID)
		c.logger.Errorf("Connection:send:<-c.closeCh #2", "sid:%v tid:%v wsURL:%q, websocket code:%v", msg.SessionID, tid, c.wsURL, code)
		_ = c.closeConnection(code)
		return &websocket.CloseError{Code: code}
	case <-c.done:
		tid := c.findSessionTargetID(msg.SessionID)
		c.logger.Errorf("Connection:send:<-c.done #2", "sid:%v tid:%v wsURL:%q", msg.SessionID, tid, c.wsURL)
	case <-c.ctx.Done():
		tid := c.findSessionTargetID(msg.SessionID)
		c.logger.Errorf("Connection:send:<-c.ctx.Done()", "sid:%v tid:%v wsURL:%q err:%v", msg.SessionID, tid, c.wsURL, c.ctx.Err())
		// TODO: this brings many bugs to the surface
		return c.ctx.Err()
		// TODO: add a timeout?
		// case <-timeout:
		// 	return
	}
	return nil
}

func (c *Connection) sendLoop() {
	c.logger.Errorf("Connection:sendLoop", "wsURL:%q, starts", c.wsURL)
	for {
		select {
		case msg := <-c.sendCh:
			c.encoder = jwriter.Writer{}
			msg.MarshalEasyJSON(&c.encoder)
			if err := c.encoder.Error; err != nil {
				sid := msg.SessionID
				tid := c.findSessionTargetID(sid)
				select {
				case c.errorCh <- err:
					c.logger.Errorf("Connection:sendLoop:c.errorCh <- err", "sid:%v tid:%v wsURL:%q err:%v", sid, tid, c.wsURL, err)
				case <-c.done:
					c.logger.Errorf("Connection:sendLoop:<-c.done", "sid:%v tid:%v wsURL:%q", sid, tid, c.wsURL)
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
			c.logger.Errorf("Connection:sendLoop:<-c.closeCh", "wsURL:%q code:%d", c.wsURL, code)
			_ = c.closeConnection(code)
		case <-c.done:
			c.logger.Errorf("Connection:sendLoop:<-c.done#2", "wsURL:%q", c.wsURL)
		case <-c.ctx.Done():
			c.logger.Errorf("connection:sendLoop", "returning, ctx.Err: %q", c.ctx.Err())
			return
			// case <-time.After(time.Second * 10):
			// 	c.logger.Errorf("connection:sendLoop", "returning, timeout")
			// 	c.Close()
			// 	return
		}
	}
}

func (c *Connection) Close(args ...goja.Value) {
	code := websocket.CloseGoingAway
	if len(args) > 0 {
		code = int(args[0].ToInteger())
	}
	c.logger.Errorf("connection:Close", "wsURL:%q code:%d", c.wsURL, code)
	_ = c.closeConnection(code)
}

// Execute implements cdproto.Executor and performs a synchronous send and receive
func (c *Connection) Execute(ctx context.Context, method string, params easyjson.Marshaler, res easyjson.Unmarshaler) error {
	c.logger.Errorf("connection:Execute", "wsURL:%q method:%q", c.wsURL, method)
	id := atomic.AddInt64(&c.msgID, 1)

	// Setup event handler used to block for response to message being sent.
	ch := make(chan *cdproto.Message, 1)
	evCancelCtx, evCancelFn := context.WithCancel(ctx)
	chEvHandler := make(chan Event)
	go func() {
		for {
			select {
			case <-evCancelCtx.Done():
				c.logger.Errorf("connection:Execute:<-evCancelCtx.Done()", "wsURL:%q err:%v", c.wsURL, evCancelCtx.Err())
				return
			case ev := <-chEvHandler:
				msg, ok := ev.data.(*cdproto.Message)
				if ok && msg.ID == id {
					select {
					case <-evCancelCtx.Done():
						c.logger.Errorf("connection:Execute:<-evCancelCtx.Done()#2", "wsURL:%q err:%v", c.wsURL, evCancelCtx.Err())
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
