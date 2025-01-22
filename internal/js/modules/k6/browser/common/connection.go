package common

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"go.k6.io/k6/internal/js/modules/k6/browser/log"

	"github.com/chromedp/cdproto"
	"github.com/chromedp/cdproto/cdp"
	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
	"github.com/gorilla/websocket"
	"github.com/mailru/easyjson"
	"github.com/mailru/easyjson/jlexer"
	"github.com/mailru/easyjson/jwriter"
)

const wsWriteBufferSize = 1 << 20

// Each connection needs its own msgID. A msgID will be used by the
// connection and associated sessions. When a CDP request is made to
// chrome, it's best to work with unique ids to avoid the Execute
// handlers working with the wrong response, or handlers deadlocking
// when their response is rerouted to the wrong handler.
//
// Use the msgIDGenerator interface to abstract `id` away.
type msgID struct {
	id int64
}

func (m *msgID) newID() int64 {
	return atomic.AddInt64(&m.id, 1)
}

type msgIDGenerator interface {
	newID() int64
}

type executorEmitter interface {
	cdp.Executor
	EventEmitter
}

type connection interface {
	executorEmitter
	Close()
	IgnoreIOErrors()
	getSession(target.SessionID) *Session
}

type session interface {
	cdp.Executor
	executorEmitter
	ExecuteWithoutExpectationOnReply(context.Context, string, easyjson.Marshaler, easyjson.Unmarshaler) error
	ID() target.SessionID
	TargetID() target.ID
	Done() <-chan struct{}
}

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

// Connection represents a WebSocket connection and the root "Browser Session".
//
//	                                          ┌───────────────────────────────────────────────────────────────────┐
//	                                          │                                                                   │
//	                                          │                          Browser Process                          │
//	                                          │                                                                   │
//	                                          └───────────────────────────────────────────────────────────────────┘
//	┌───────────────────────────┐                                           │      ▲
//	│Reads JSON-RPC CDP messages│                                           │      │
//	│from WS connection and puts│                                           ▼      │
//	│ them on incoming queue of │             ┌───────────────────────────────────────────────────────────────────┐
//	│    target session, as     ├─────────────■                                                                   │
//	│   identified by message   │             │                       WebSocket Connection                        │
//	│   session ID. Messages    │             │                                                                   │
//	│ without a session ID are  │             └───────────────────────────────────────────────────────────────────┘
//	│considered to belong to the│                    │      ▲                                       │      ▲
//	│  root "Browser Session".  │                    │      │                                       │      │
//	└───────────────────────────┘                    ▼      │                                       ▼      │
//	┌───────────────────────────┐             ┌────────────────────┐                         ┌────────────────────┐
//	│  Handles CDP messages on  ├─────────────■                    │                         │                    │
//	│incoming queue and puts CDP│             │      Session       │      *  *  *  *  *      │      Session       │
//	│   messages on outgoing    │             │                    │                         │                    │
//	│ channel of WS connection. │             └────────────────────┘                         └────────────────────┘
//	└───────────────────────────┘                    │      ▲                                       │      ▲
//	  │      │                                       │      │                                       │      │
//	  ▼      │                                       ▼      │                                       ▼      │
//
//	┌───────────────────────────┐             ┌────────────────────┐                         ┌────────────────────┐
//	│Registers with session as a├─────────────■                    │                         │                    │
//	│handler for a specific CDP │             │   Event Listener   │      *  *  *  *  *      │   Event Listener   │
//	│       Domain event.       │             │                    │                         │                    │
//	└───────────────────────────┘             └────────────────────┘                         └────────────────────┘
type Connection struct {
	BaseEventEmitter

	ctx          context.Context
	cancelCtx    context.CancelFunc
	wsURL        string
	logger       *log.Logger
	conn         *websocket.Conn
	sendCh       chan *cdproto.Message
	recvCh       chan *cdproto.Message
	closeCh      chan int
	errorCh      chan error
	done         chan struct{}
	closing      chan struct{}
	shutdownOnce sync.Once
	msgIDGen     msgIDGenerator

	sessionsMu sync.RWMutex
	sessions   map[target.SessionID]*Session

	// Reuse the easyjson structs to avoid allocs per Read/Write.
	decoder jlexer.Lexer
	encoder jwriter.Writer

	// onTargetAttachedToTarget is called when a new target is attached to the browser.
	// Returning false will prevent the session from being created.
	// If onTargetAttachedToTarget is nil, the session will be created.
	onTargetAttachedToTarget func(*target.EventAttachedToTarget) bool
}

// NewConnection creates a new browser.
func NewConnection(
	ctx context.Context,
	wsURL string,
	logger *log.Logger,
	onTargetAttachedToTarget func(*target.EventAttachedToTarget) bool,
) (*Connection, error) {
	var header http.Header
	var tlsConfig *tls.Config
	wsd := websocket.Dialer{
		HandshakeTimeout: time.Second * 60,
		Proxy:            http.ProxyFromEnvironment, // TODO(fix): use proxy settings from launch options
		TLSClientConfig:  tlsConfig,
		WriteBufferSize:  wsWriteBufferSize,
	}

	ctx, cancelCtx := context.WithCancel(ctx)

	conn, response, connErr := wsd.DialContext(ctx, wsURL, header)
	if response != nil {
		defer func() {
			_ = response.Body.Close()
		}()
	}
	if connErr != nil {
		cancelCtx()
		return nil, connErr
	}

	c := Connection{
		BaseEventEmitter:         NewBaseEventEmitter(ctx),
		ctx:                      ctx,
		cancelCtx:                cancelCtx,
		wsURL:                    wsURL,
		logger:                   logger,
		conn:                     conn,
		sendCh:                   make(chan *cdproto.Message, 32), // Avoid blocking in Execute
		recvCh:                   make(chan *cdproto.Message),
		closeCh:                  make(chan int),
		errorCh:                  make(chan error),
		done:                     make(chan struct{}),
		closing:                  make(chan struct{}),
		msgIDGen:                 &msgID{},
		sessions:                 make(map[target.SessionID]*Session),
		onTargetAttachedToTarget: onTargetAttachedToTarget,
	}

	go c.recvLoop()
	go c.sendLoop()

	return &c, nil
}

func (c *Connection) close(code int) error {
	c.logger.Debugf("Connection:close", "code:%d", code)

	defer c.cancelCtx()

	var err error
	c.shutdownOnce.Do(func() {
		defer func() {
			// Stop the main control loop
			close(c.done)
			_ = c.conn.Close()
		}()

		c.closeAllSessions()

		err = c.conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(code, ""),
			time.Now().Add(time.Second),
		)

		// According to the WS RFC[1], we might want to wait for a response
		// Control frame back from the browser here (possibly for the above
		// timeout duration), but Chrom{e,ium} never sends one, even when
		// the browser process exits normally after the Browser.close CDP
		// command. So we don't bother waiting, since it would just needlessly
		// delay the k6 iteration.
		// [1]: https://www.rfc-editor.org/rfc/rfc6455#section-1.4

		c.emit(EventConnectionClose, nil)
	})

	return err
}

// closeSession closes the session with the given session ID.
// It returns true if the session was found and closed, false otherwise.
func (c *Connection) closeSession(sid target.SessionID, tid target.ID) bool {
	c.logger.Debugf("Connection:closeSession", "sid:%v tid:%v wsURL:%v", sid, tid, c.wsURL)

	c.sessionsMu.Lock()
	defer c.sessionsMu.Unlock()

	session, ok := c.sessions[sid]
	if !ok {
		return false
	}
	session.close()
	delete(c.sessions, sid)

	return true
}

func (c *Connection) closeAllSessions() {
	c.logger.Debugf("Connection:closeAllSessions", "wsURL:%v", c.wsURL)

	c.sessionsMu.Lock()
	for _, s := range c.sessions {
		s.close()
		delete(c.sessions, s.id)
	}
	c.sessionsMu.Unlock()
}

func (c *Connection) createSession(info *target.Info) (*Session, error) {
	c.logger.Debugf("Connection:createSession", "tid:%v bctxid:%v type:%s",
		info.TargetID, info.BrowserContextID, info.Type)

	var sessionID target.SessionID
	var err error
	action := target.AttachToTarget(info.TargetID).WithFlatten(true)
	if sessionID, err = action.Do(cdp.WithExecutor(c.ctx, c)); err != nil {
		c.logger.Debugf("Connection:createSession", "tid:%v bctxid:%v type:%s err:%v",
			info.TargetID, info.BrowserContextID, info.Type, err)
		return nil, err
	}
	sess := c.getSession(sessionID)
	if sess == nil {
		c.logger.Warnf("Connection:createSession", "tid:%v bctxid:%v type:%s sid:%v, session is nil",
			info.TargetID, info.BrowserContextID, info.Type, sessionID)
	}
	return sess, nil
}

func (c *Connection) handleIOError(err error) {
	if closing := c.isClosing(); websocket.IsCloseError(
		err, websocket.CloseNormalClosure, websocket.CloseGoingAway,
	) || closing {
		c.logger.Debugf("cdp", "received IO error: %v, connection is closing: %v", err, closing)
		return
	}

	// Report an unexpected closure
	c.logger.Errorf("cdp", "communicating with browser: %v", err)
	select {
	case c.errorCh <- err:
	case <-c.done:
		return
	}
	var (
		cerr *websocket.CloseError
		code = websocket.CloseGoingAway
	)
	if errors.As(err, &cerr) {
		code = cerr.Code
	}
	select {
	case c.closeCh <- code:
		c.logger.Debugf("cdp", "ending browser communication with code %d", code)
	case <-c.done:
		c.logger.Debugf("cdp", "ending browser communication")
	}
}

func (c *Connection) getSession(id target.SessionID) *Session {
	c.sessionsMu.RLock()
	defer c.sessionsMu.RUnlock()

	return c.sessions[id]
}

// findTragetIDForLog should only be used for logging purposes.
// It will return an empty string if logger.DebugMode is false.
func (c *Connection) findTargetIDForLog(id target.SessionID) target.ID {
	if !c.logger.DebugMode() {
		return ""
	}
	s := c.getSession(id)
	if s == nil {
		return ""
	}
	return s.targetID
}

//nolint:funlen,gocognit,cyclop
func (c *Connection) recvLoop() {
	c.logger.Debugf("Connection:recvLoop", "wsURL:%q", c.wsURL)
	for {
		_, buf, err := c.conn.ReadMessage()
		if err != nil {
			c.handleIOError(err)
			return
		}

		c.logger.Tracef("cdp:recv", "<- %s", buf)

		var msg cdproto.Message
		c.decoder = jlexer.Lexer{Data: buf}
		msg.UnmarshalEasyJSON(&c.decoder)
		if err := c.decoder.Error(); err != nil {
			select {
			case c.errorCh <- err:
				c.logger.Debugf("Connection:recvLoop:<-err", "wsURL:%q err:%v", c.wsURL, err)
			case <-c.done:
				c.logger.Debugf("Connection:recvLoop:<-c.done", "wsURL:%q", c.wsURL)
				return
			}
		}

		// Handle attachment and detachment from targets,
		// creating and deleting sessions as necessary.
		//nolint:nestif
		if msg.Method == cdproto.EventTargetAttachedToTarget {
			ev, err := cdproto.UnmarshalMessage(&msg)
			if err != nil {
				c.logger.Errorf("cdp", "%s", err)
				continue
			}
			eva := ev.(*target.EventAttachedToTarget) //nolint:forcetypeassert
			sid, tid := eva.SessionID, eva.TargetInfo.TargetID

			if c.onTargetAttachedToTarget != nil {
				// If onTargetAttachedToTarget is set, it will be called to determine
				// if a session should be created for the target.
				ok := c.onTargetAttachedToTarget(eva)
				if !ok {
					c.stopWaitingForDebugger(sid)
					continue
				}
			}

			c.sessionsMu.Lock()
			session := NewSession(c.ctx, c, sid, tid, c.logger, c.msgIDGen)
			c.logger.Debugf("Connection:recvLoop:EventAttachedToTarget", "sid:%v tid:%v wsURL:%q", sid, tid, c.wsURL)
			c.sessions[sid] = session
			c.sessionsMu.Unlock()
		} else if msg.Method == cdproto.EventTargetDetachedFromTarget {
			ev, err := cdproto.UnmarshalMessage(&msg)
			if err != nil {
				c.logger.Errorf("cdp", "%s", err)
				continue
			}
			evt := ev.(*target.EventDetachedFromTarget) //nolint:forcetypeassert
			sid := evt.SessionID
			tid := c.findTargetIDForLog(sid)
			ok := c.closeSession(sid, tid)
			if !ok {
				c.logger.Debugf(
					"Connection:recvLoop:EventDetachedFromTarget",
					"sid:%v tid:%v wsURL:%q, session not found",
					sid, tid, c.wsURL,
				)

				continue
			}
		}

		switch {
		case msg.SessionID != "" && (msg.Method != "" || msg.ID != 0):
			// TODO: possible data race - session can get removed after getting it here
			session := c.getSession(msg.SessionID)
			if session == nil {
				continue
			}
			if msg.Error != nil && msg.Error.Message == "No session with given id" {
				c.logger.Debugf("Connection:recvLoop", "sid:%v tid:%v wsURL:%q, closeSession #2",
					session.id, session.targetID, c.wsURL)
				c.closeSession(session.id, session.targetID)
				continue
			}

			select {
			case session.readCh <- &msg:
			case code := <-c.closeCh:
				c.logger.Debugf("Connection:recvLoop:<-c.closeCh", "sid:%v tid:%v wsURL:%v crashed:%t",
					session.id, session.targetID, c.wsURL, session.crashed)
				_ = c.close(code)
			case <-c.done:
				c.logger.Debugf("Connection:recvLoop:<-c.done", "sid:%v tid:%v wsURL:%v crashed:%t",
					session.id, session.targetID, c.wsURL, session.crashed)
				return
			}

		case msg.Method != "":
			c.logger.Debugf("Connection:recvLoop:msg.Method:emit", "sid:%v method:%q", msg.SessionID, msg.Method)
			ev, err := cdproto.UnmarshalMessage(&msg)
			if err != nil {
				c.logger.Errorf("cdp", "%s", err)
				continue
			}
			c.emit(string(msg.Method), ev)

		case msg.ID != 0:
			c.logger.Debugf("Connection:recvLoop:msg.ID:emit", "sid:%v method:%q", msg.SessionID, msg.Method)
			c.emit("", &msg)

		default:
			c.logger.Errorf("cdp", "ignoring malformed incoming message (missing id or method): %#v (message: %s)",
				msg, msg.Error.Message)
		}
	}
}

// stopWaitingForDebugger tells the browser to stop waiting for the
// debugger to attach to the page's session.
//
// Whether we're not sharing pages among browser contexts, Chromium
// still does so (since we're auto-attaching all browser targets).
// This means that if we don't stop waiting for the debugger, the
// browser will wait for the debugger to attach to the new page
// indefinitely, even if the page is not part of the browser context
// we're using.
//
// We don't return an error because the browser might have already
// closed the connection. In that case, handling the error would
// be redundant. This operation is best-effort.
func (c *Connection) stopWaitingForDebugger(sid target.SessionID) {
	msg := &cdproto.Message{
		ID:        c.msgIDGen.newID(),
		SessionID: sid,
		Method:    cdproto.MethodType(cdpruntime.CommandRunIfWaitingForDebugger),
	}
	err := c.send(c.ctx, msg, nil, nil)
	if err != nil {
		c.logger.Errorf("Connection:stopWaitingForDebugger", "sid:%v wsURL:%q, err:%v", sid, c.wsURL, err)
	}
}

func (c *Connection) send(
	ctx context.Context, msg *cdproto.Message, recvCh chan *cdproto.Message, res easyjson.Unmarshaler,
) error {
	select {
	case c.sendCh <- msg:
	case err := <-c.errorCh:
		c.logger.Debugf("Connection:send:<-c.errorCh", "wsURL:%q sid:%v, err:%v", c.wsURL, msg.SessionID, err)
		return fmt.Errorf("sending a message to browser: %w", err)
	case code := <-c.closeCh:
		c.logger.Debugf("Connection:send:<-c.closeCh", "wsURL:%q sid:%v, websocket code:%v", c.wsURL, msg.SessionID, code)
		_ = c.close(code)
		return fmt.Errorf("closing communication with browser: %w", &websocket.CloseError{Code: code})
	case <-ctx.Done():
		c.logger.Debugf("Connection:send:<-ctx.Done", "wsURL:%q sid:%v err:%v", c.wsURL, msg.SessionID, c.ctx.Err())
		return nil
	case <-c.done:
		c.logger.Debugf("Connection:send:<-c.done", "wsURL:%q sid:%v", c.wsURL, msg.SessionID)
		return nil
	}

	// Block waiting for response.
	if recvCh == nil {
		return nil
	}
	tid := c.findTargetIDForLog(msg.SessionID)
	select {
	case msg := <-recvCh:
		var sid target.SessionID
		tid = ""
		if msg != nil {
			sid = msg.SessionID
			tid = c.findTargetIDForLog(sid)
		}
		switch {
		case msg == nil:
			c.logger.Debugf("Connection:send", "wsURL:%q, err:ErrChannelClosed", c.wsURL)
			return ErrChannelClosed
		case msg.Error != nil:
			c.logger.Debugf("Connection:send", "sid:%v tid:%v wsURL:%q, msg err:%v", sid, tid, c.wsURL, msg.Error)
			return msg.Error
		case res != nil:
			return easyjson.Unmarshal(msg.Result, res)
		}
	case err := <-c.errorCh:
		c.logger.Debugf("Connection:send:<-c.errorCh #2", "sid:%v tid:%v wsURL:%q, err:%v", msg.SessionID, tid, c.wsURL, err)
		return err
	case code := <-c.closeCh:
		c.logger.Debugf("Connection:send:<-c.closeCh #2", "sid:%v tid:%v wsURL:%q, websocket code:%v",
			msg.SessionID, tid, c.wsURL, code)
		_ = c.close(code)
		return &websocket.CloseError{Code: code}
	case <-c.done:
		c.logger.Debugf("Connection:send:<-c.done #2", "sid:%v tid:%v wsURL:%q", msg.SessionID, tid, c.wsURL)
	case <-ctx.Done():
		c.logger.Debugf("Connection:send:<-ctx.Done()", "sid:%v tid:%v wsURL:%q err:%v",
			msg.SessionID, tid, c.wsURL, c.ctx.Err())
		return ctx.Err()
	case <-c.ctx.Done():
		c.logger.Debugf("Connection:send:<-c.ctx.Done()", "sid:%v tid:%v wsURL:%q err:%v",
			msg.SessionID, tid, c.wsURL, c.ctx.Err())
		return c.ctx.Err()
	}
	return nil
}

func (c *Connection) sendLoop() {
	c.logger.Debugf("Connection:sendLoop", "wsURL:%q, starts", c.wsURL)
	for {
		select {
		case msg := <-c.sendCh:
			c.encoder = jwriter.Writer{}
			msg.MarshalEasyJSON(&c.encoder)
			if err := c.encoder.Error; err != nil {
				sid := msg.SessionID
				tid := c.findTargetIDForLog(sid)
				select {
				case c.errorCh <- err:
					c.logger.Debugf("Connection:sendLoop:c.errorCh <- err", "sid:%v tid:%v wsURL:%q err:%v", sid, tid, c.wsURL, err)
				case <-c.done:
					c.logger.Debugf("Connection:sendLoop:<-c.done", "sid:%v tid:%v wsURL:%q", sid, tid, c.wsURL)
					return
				}
			}

			buf, _ := c.encoder.BuildBytes()
			c.logger.Tracef("cdp:send", "-> %s", buf)
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
			c.logger.Debugf("Connection:sendLoop:<-c.closeCh", "wsURL:%q code:%d", c.wsURL, code)
			_ = c.close(code)
			return
		case <-c.done:
			c.logger.Debugf("Connection:sendLoop:<-c.done#2", "wsURL:%q", c.wsURL)
			return
		case <-c.ctx.Done():
			c.logger.Debugf("connection:sendLoop", "returning, ctx.Err: %q", c.ctx.Err())
			return
		}
	}
}

// Close cleanly closes the WebSocket connection.
// It returns an error if sending the Close control frame fails.
//
// Optional code to override default websocket.CloseGoingAway (1001).
func (c *Connection) Close() {
	code := websocket.CloseNormalClosure
	c.logger.Debugf("connection:Close", "wsURL:%q code:%d", c.wsURL, code)
	_ = c.close(code)
}

// Execute implements cdproto.Executor and performs a synchronous send and receive.
func (c *Connection) Execute(
	ctx context.Context, method string, params easyjson.Marshaler, res easyjson.Unmarshaler,
) error {
	c.logger.Debugf("connection:Execute", "wsURL:%q method:%q", c.wsURL, method)
	id := c.msgIDGen.newID()

	// Setup event handler used to block for response to message being sent.
	ch := make(chan *cdproto.Message, 1)
	evCancelCtx, evCancelFn := context.WithCancel(ctx)
	chEvHandler := make(chan Event)
	go func() {
		for {
			select {
			case <-evCancelCtx.Done():
				c.logger.Debugf("connection:Execute:<-evCancelCtx.Done()", "wsURL:%q err:%v", c.wsURL, evCancelCtx.Err())
				return
			case ev := <-chEvHandler:
				msg, ok := ev.data.(*cdproto.Message)
				if ok && msg.ID == id {
					select {
					case <-evCancelCtx.Done():
						c.logger.Debugf("connection:Execute:<-evCancelCtx.Done()#2", "wsURL:%q err:%v", c.wsURL, evCancelCtx.Err())
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

	return c.send(evCancelCtx, msg, ch, res)
}

// IgnoreIOErrors signals that the connection will soon be closed, so that any
// received IO errors can be disregarded.
func (c *Connection) IgnoreIOErrors() {
	close(c.closing)
}

func (c *Connection) isClosing() (s bool) {
	select {
	case <-c.closing:
		s = true
	default:
	}

	return
}
