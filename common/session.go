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
	"errors"
	"sync/atomic"

	"github.com/chromedp/cdproto"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/target"
	"github.com/mailru/easyjson"
	"go.k6.io/k6/lib"
	"golang.org/x/net/context"
)

// Ensure Session implements the EventEmitter and Executor interfaces
var _ EventEmitter = &Session{}
var _ cdp.Executor = &Session{}

// Session represents a CDP session to a target
type Session struct {
	BaseEventEmitter

	ctx     context.Context
	conn    *Connection
	id      target.SessionID
	msgID   int64
	readCh  chan *cdproto.Message
	done    chan struct{}
	closed  bool
	crashed bool
}

// NewSession creates a new session
func NewSession(ctx context.Context, conn *Connection, id target.SessionID) *Session {
	s := Session{
		BaseEventEmitter: NewBaseEventEmitter(ctx),
		ctx:              ctx,
		conn:             conn,
		id:               id,
		msgID:            0,
		readCh:           make(chan *cdproto.Message),
		done:             make(chan struct{}),
	}
	go s.readLoop()
	return &s
}

func (s *Session) close() {
	if s.closed {
		return
	}

	// Stop the read loop
	close(s.done)
	s.closed = true

	s.emit(EventSessionClosed, nil)
}

func (s *Session) markAsCrashed() {
	s.crashed = true
}

// Wraps conn.ReadMessage in a channel
func (s *Session) readLoop() {
	state := lib.GetState(s.ctx)

	for {
		select {
		case msg := <-s.readCh:
			ev, err := cdproto.UnmarshalMessage(msg)
			if err != nil {
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
			return
		}
	}
}

// Execute implements the cdp.Executor interface
func (s *Session) Execute(ctx context.Context, method string, params easyjson.Marshaler, res easyjson.Unmarshaler) error {
	// Certain methods aren't available to the user directly.
	if method == target.CommandCloseTarget {
		return errors.New("to close the target, cancel its context")
	}
	if s.crashed {
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
				return
			case ev := <-chEvHandler:
				if msg, ok := ev.data.(*cdproto.Message); ok && msg.ID == id {
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
	return s.conn.send(msg, ch, res)
}

func (s *Session) ExecuteWithoutExpectationOnReply(ctx context.Context, method string, params easyjson.Marshaler, res easyjson.Unmarshaler) error {
	// Certain methods aren't available to the user directly.
	if method == target.CommandCloseTarget {
		return errors.New("to close the target, cancel its context")
	}
	if s.crashed {
		return ErrTargetCrashed
	}

	id := atomic.AddInt64(&s.msgID, 1)

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
	return s.conn.send(msg, nil, res)
}
