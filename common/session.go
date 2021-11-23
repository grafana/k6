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
	"errors"
	"sync/atomic"

	"github.com/chromedp/cdproto"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/target"
	"github.com/mailru/easyjson"
	k6lib "go.k6.io/k6/lib"
)

// Ensure Session implements the EventEmitter and Executor interfaces
var _ EventEmitter = &Session{}
var _ cdp.Executor = &Session{}

// Session represents a CDP session to a target
type Session struct {
	BaseEventEmitter

	ctx      context.Context
	conn     *Connection
	id       target.SessionID
	targetID target.ID
	msgID    int64
	readCh   chan *cdproto.Message
	done     chan struct{}
	closed   bool
	crashed  bool
}

// NewSession creates a new session
func NewSession(ctx context.Context, conn *Connection, id target.SessionID, tid target.ID) *Session {
	s := Session{
		BaseEventEmitter: NewBaseEventEmitter(ctx),
		ctx:              ctx,
		conn:             conn,
		id:               id,
		targetID:         tid,
		readCh:           make(chan *cdproto.Message),
		done:             make(chan struct{}),
	}
	s.conn.logger.Infof("Session:NewSession", "sid=%v tid=%v", id, tid)
	go s.readLoop()
	return &s
}

func (s *Session) close() {
	s.conn.logger.Infof("Session:close", "sid=%v tid=%v", s.id, s.targetID)
	if s.closed {
		s.conn.logger.Infof("Session:close", "already closed, sid=%v tid=%v", s.id, s.targetID)
		return
	}

	// Stop the read loop
	close(s.done)
	s.closed = true

	s.emit(EventSessionClosed, nil)
}

func (s *Session) markAsCrashed() {
	s.conn.logger.Infof("Session:markAsCrashed", "sid=%v tid=%v", s.id, s.targetID)
	s.crashed = true
}

// Wraps conn.ReadMessage in a channel
func (s *Session) readLoop() {
	state := k6lib.GetState(s.ctx)

	for {
		select {
		case msg := <-s.readCh:
			ev, err := cdproto.UnmarshalMessage(msg)
			if err != nil {
				s.conn.logger.Infof("Session:readLoop:<-s.readCh", "sid=%v tid=%v cannot unmarshal: %v", s.id, s.targetID, err)
				if _, ok := err.(cdp.ErrUnknownCommandOrEvent); ok {
					// This is most likely an event received from an older
					// Chrome which a newer cdproto doesn't have, as it is
					// deprecated. Ignore that error, and emit raw cdproto.Message.
					s.emit("", msg)
					continue
				}
				state.Logger.Errorf("%s", err)
				continue
			}
			s.emit(string(msg.Method), ev)
		case <-s.done:
			s.conn.logger.Errorf("Session:readLoop:<-s.done", "sid=%v tid=%v", s.id, s.targetID)
			return
		}
	}
}

// Execute implements the cdp.Executor interface
func (s *Session) Execute(ctx context.Context, method string, params easyjson.Marshaler, res easyjson.Unmarshaler) error {
	s.conn.logger.Infof("Session:Execute", "sid=%v tid=%v method=%q", s.id, s.targetID, method)
	// Certain methods aren't available to the user directly.
	if method == target.CommandCloseTarget {
		return errors.New("to close the target, cancel its context")
	}
	if s.crashed {
		s.conn.logger.Infof("Session:Execute", "sid=%v tid=%v method=%q, returns: crashed", s.id, s.targetID, method)
		return ErrTargetCrashed
	}

	id := atomic.AddInt64(&s.msgID, 1)

	// Setup event handler used to block for response to message being sent.
	ch := make(chan *cdproto.Message, 1)
	evCancelCtx, evCancelFn := context.WithCancel(ctx)
	chEvHandler := make(chan Event)
	go func() {
		for {
			select {
			case <-evCancelCtx.Done():
				s.conn.logger.Errorf("Session:Execute:<-evCancelCtx.Done()", "sid=%v tid=%v method=%q, returns", s.id, s.targetID, method)
				return
			case ev := <-chEvHandler:
				// s.conn.logger.Infof("Session:Execute:<-chEvHandler", "sid=%v method=%q", s.id, method)
				if msg, ok := ev.data.(*cdproto.Message); ok && msg.ID == id {
					select {
					case <-evCancelCtx.Done():
						s.conn.logger.Errorf("Session:Execute:<-evCancelCtx.Done():2", "sid=%v tid=%v method=%q, returns", s.id, s.targetID, method)
					case ch <- msg:
						// s.conn.logger.Infof("Session:Execute:<-chEvHandler:ch <- msg", "sid=%v method=%q", s.id, method)
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
		ID:        id,
		SessionID: s.id,
		Method:    cdproto.MethodType(method),
		Params:    buf,
	}
	s.conn.logger.Infof("Session:Execute:s.conn.send", "sid=%v method=%q", s.id, method)
	return s.conn.send(msg, ch, res)
}

func (s *Session) ExecuteWithoutExpectationOnReply(ctx context.Context, method string, params easyjson.Marshaler, res easyjson.Unmarshaler) error {
	s.conn.logger.Infof("Session:ExecuteWithoutExpectationOnReply", "sid=%v tid=%v method=%q", s.id, s.targetID, method)
	// Certain methods aren't available to the user directly.
	if method == target.CommandCloseTarget {
		return errors.New("to close the target, cancel its context")
	}
	if s.crashed {
		s.conn.logger.Infof("Session:ExecuteWithoutExpectationOnReply", "sid=%v tid=%v method=%q, ErrTargetCrashed", s.id, s.targetID, method)
		return ErrTargetCrashed
	}

	id := atomic.AddInt64(&s.msgID, 1)

	// Send the message
	var buf []byte
	if params != nil {
		var err error
		buf, err = easyjson.Marshal(params)
		if err != nil {
			s.conn.logger.Infof("Session:ExecuteWithoutExpectationOnReply:Marshal", "sid=%v tid=%v method=%q err=%v", s.id, s.targetID, method, err)
			return err
		}
	}
	msg := &cdproto.Message{
		ID: id,
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
	return s.conn.send(msg, nil, res)
}
