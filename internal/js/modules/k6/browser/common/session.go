package common

import (
	"context"
	"errors"

	"github.com/chromedp/cdproto"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/target"
	"github.com/mailru/easyjson"

	"go.k6.io/k6/internal/js/modules/k6/browser/log"
)

// Session represents a CDP session to a target.
type Session struct {
	BaseEventEmitter

	conn     *Connection
	id       target.SessionID
	targetID target.ID
	msgIDGen msgIDGenerator
	readCh   chan *cdproto.Message
	done     chan struct{}
	closed   bool
	crashed  bool

	logger *log.Logger
}

// NewSession creates a new session.
func NewSession(
	ctx context.Context, conn *Connection, id target.SessionID, tid target.ID, logger *log.Logger, msgIDGen msgIDGenerator,
) *Session {
	s := Session{
		BaseEventEmitter: NewBaseEventEmitter(ctx),
		conn:             conn,
		id:               id,
		targetID:         tid,
		readCh:           make(chan *cdproto.Message),
		done:             make(chan struct{}),
		msgIDGen:         msgIDGen,

		logger: logger,
	}
	s.logger.Debugf("Session:NewSession", "sid:%v tid:%v", id, tid)
	go s.readLoop()
	return &s
}

// ID returns session ID.
func (s *Session) ID() target.SessionID {
	return s.id
}

// TargetID returns session's target ID.
func (s *Session) TargetID() target.ID {
	return s.targetID
}

func (s *Session) close() {
	s.logger.Debugf("Session:close", "sid:%v tid:%v", s.id, s.targetID)
	if s.closed {
		s.logger.Debugf("Session:close", "already closed, sid:%v tid:%v", s.id, s.targetID)
		return
	}

	// Stop the read loop
	close(s.done)
	s.closed = true
}

func (s *Session) markAsCrashed() {
	s.logger.Debugf("Session:markAsCrashed", "sid:%v tid:%v", s.id, s.targetID)
	s.crashed = true
}

// Wraps conn.ReadMessage in a channel.
func (s *Session) readLoop() {
	for {
		select {
		case msg := <-s.readCh:
			ev, err := cdproto.UnmarshalMessage(msg)
			if errors.Is(err, cdp.ErrUnknownCommandOrEvent("")) && msg.Method == "" {
				// Results from commands may not always have methods in them.
				// This is the reason of this error. So it's harmless.
				//
				// Also:
				// This is most likely an event received from an older
				// Chrome which a newer cdproto doesn't have, as it is
				// deprecated. Ignore that error, and emit raw cdproto.Message.
				s.emit("", msg)
				continue
			}
			if err != nil {
				s.logger.Debugf("Session:readLoop:<-s.readCh", "sid:%v tid:%v cannot unmarshal: %v", s.id, s.targetID, err)
				continue
			}
			s.emit(string(msg.Method), ev)
		case <-s.done:
			s.logger.Debugf("Session:readLoop:<-s.done", "sid:%v tid:%v", s.id, s.targetID)
			return
		}
	}
}

// Execute implements the cdp.Executor interface.
func (s *Session) Execute(
	ctx context.Context, method string, params easyjson.Marshaler, res easyjson.Unmarshaler,
) error {
	s.logger.Debugf("Session:Execute", "sid:%v tid:%v method:%q", s.id, s.targetID, method)
	if s.crashed {
		s.logger.Debugf("Session:Execute:return", "sid:%v tid:%v method:%q crashed", s.id, s.targetID, method)
		return ErrTargetCrashed
	}

	id := s.msgIDGen.newID()

	// Setup event handler used to block for response to message being sent.
	ch := make(chan *cdproto.Message, 1)
	evCancelCtx, evCancelFn := context.WithCancel(ctx)
	chEvHandler := make(chan Event)
	go func() {
		for {
			select {
			case <-evCancelCtx.Done():
				s.logger.Debugf("Session:Execute:<-evCancelCtx.Done():return", "sid:%v tid:%v method:%q",
					s.id, s.targetID, method)
				return
			case ev := <-chEvHandler:
				if msg, ok := ev.data.(*cdproto.Message); ok && msg.ID == id {
					select {
					case <-evCancelCtx.Done():
						s.logger.Debugf("Session:Execute:<-evCancelCtx.Done():2:return", "sid:%v tid:%v method:%q",
							s.id, s.targetID, method)
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
	s.onAll(evCancelCtx, chEvHandler)
	defer evCancelFn() // Remove event handler

	s.logger.Debugf("Session:Execute:s.conn.send", "sid:%v tid:%v method:%q", s.id, s.targetID, method)

	var buf []byte
	if params != nil {
		var err error
		buf, err = easyjson.Marshal(params)
		if err != nil {
			return err
		}
	}
	msg := &cdproto.Message{
		ID:        id,
		SessionID: s.id,
		Method:    cdproto.MethodType(method),
		Params:    buf,
	}
	return s.conn.send(contextWithDoneChan(ctx, s.done), msg, ch, res)
}

func (s *Session) ExecuteWithoutExpectationOnReply(
	ctx context.Context, method string, params easyjson.Marshaler, res easyjson.Unmarshaler,
) error {
	s.logger.Debugf("Session:ExecuteWithoutExpectationOnReply", "sid:%v tid:%v method:%q", s.id, s.targetID, method)
	if s.crashed {
		s.logger.Debugf("Session:ExecuteWithoutExpectationOnReply", "sid:%v tid:%v method:%q, ErrTargetCrashed",
			s.id, s.targetID, method)
		return ErrTargetCrashed
	}

	s.logger.Debugf("Session:Execute:s.conn.send", "sid:%v tid:%v method:%q", s.id, s.targetID, method)

	var buf []byte
	if params != nil {
		var err error
		buf, err = easyjson.Marshal(params)
		if err != nil {
			s.logger.Debugf("Session:ExecuteWithoutExpectationOnReply:Marshal", "sid:%v tid:%v method:%q err=%v",
				s.id, s.targetID, method, err)
			return err
		}
	}
	msg := &cdproto.Message{
		ID: s.msgIDGen.newID(),
		// We use different sessions to send messages to "targets"
		// (browser, page, frame etc.) in CDP.
		//
		// If we don't specify a session (a session ID in the JSON message),
		// it will be a message for the browser target.
		//
		// With a session specified (set using cdp.WithExecutor(ctx, session)),
		// it will properly route the CDP message to the correct target
		// (page, frame etc.).
		//
		// The difference between using Connection and Session to send
		// and receive CDP messages basically, they both implement
		// the cdp.Executor interface but one adds a sessionID to
		// the CPD messages:
		SessionID: s.id,
		Method:    cdproto.MethodType(method),
		Params:    buf,
	}
	return s.conn.send(contextWithDoneChan(ctx, s.done), msg, nil, res)
}

// Done returns a channel that is closed when this session is closed.
func (s *Session) Done() <-chan struct{} {
	return s.done
}

// Closed returns true if this session is closed.
func (s *Session) Closed() bool {
	select {
	case <-s.done:
		return true
	default:
		return false
	}
}
