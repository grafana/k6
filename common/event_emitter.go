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
)

// Ensure BaseEventEmitter implements the EventEmitter interface
var _ EventEmitter = &BaseEventEmitter{}

const (
	// Browser
	EventBrowserDisconnected string = "disconnected"

	// BrowserContext
	EventBrowserContextClose string = "close"
	EventBrowserContextPage  string = "page"

	// Connection
	EventConnectionClose string = "close"

	// Frame
	EventFrameNavigation      string = "navigation"
	EventFrameAddLifecycle    string = "addlifecycle"
	EventFrameRemoveLifecycle string = "removelifecycle"

	// Page
	EventPageClose            string = "close"
	EventPageConsole          string = "console"
	EventPageCrash            string = "crash"
	EventPageDialog           string = "dialog"
	EventPageDOMContentLoaded string = "domcontentloaded"
	EventPageDownload         string = "download"
	EventPageFilechooser      string = "filechooser"
	EventPageFrameAttached    string = "frameattached"
	EventPageFrameDetached    string = "framedetached"
	EventPageFrameNavigated   string = "framenavigated"
	EventPageLoad             string = "load"
	EventPageError            string = "pageerror"
	EventPagePopup            string = "popup"
	EventPageRequest          string = "request"
	EventPageRequestFailed    string = "requestfailed"
	EventPageRequestFinished  string = "requestfinished"
	EventPageResponse         string = "response"
	EventPageWebSocket        string = "websocket"
	EventPageWorker           string = "worker"

	// Session
	EventSessionClosed string = "close"

	// Worker
	EventWorkerClose string = "close"
)

// Event as emitted by an EventEmiter
type Event struct {
	typ  string
	data interface{}
}

type NavigationEvent struct {
	newDocument *DocumentInfo
	url         string
	name        string
	err         error
}

type eventHandler struct {
	ctx context.Context
	ch  chan Event
}

// EventEmitter that all event emitters need to implement
type EventEmitter interface {
	emit(event string, data interface{})
	on(ctx context.Context, events []string, ch chan Event)
	onAll(ctx context.Context, ch chan Event)
}

// BaseEventEmitter emits events to registered handlers
type BaseEventEmitter struct {
	handlers    map[string][]eventHandler
	handlersAll []eventHandler

	handlersCh chan func() chan struct{}
	ctx        context.Context
}

// NewBaseEventEmitter creates a new instance of a base event emitter
func NewBaseEventEmitter(ctx context.Context) BaseEventEmitter {
	bem := BaseEventEmitter{
		handlers:    make(map[string][]eventHandler),
		handlersAll: make([]eventHandler, 0),
		handlersCh:  make(chan func() chan struct{}),
		ctx:         ctx,
	}
	go bem.handleHandlers(ctx)
	return bem
}

// handleHandlers handles handlers in a single Goroutine.
// It basically process one request at a time for synchronization.
func (e *BaseEventEmitter) handleHandlers(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case fn := <-e.handlersCh:
			select {
			case <-ctx.Done():
				return
			default:
			}
			done := fn()
			done <- struct{}{}
		}
	}
}

// sync is a helper for sychronized access to the BaseEventEmitter.
func (e *BaseEventEmitter) sync(fn func()) {
	done := make(chan struct{})
	select {
	case <-e.ctx.Done():
		return
	case e.handlersCh <- func() chan struct{} {
		fn()
		return done
	}:
	}
	<-done
}

func (e *BaseEventEmitter) emit(event string, data interface{}) {
	e.sync(func() {
		handlers := e.handlers[event]
		for i := 0; i < len(handlers); {
			handler := handlers[i]
			select {
			case <-handler.ctx.Done():
				handlers = append(handlers[:i], handlers[i+1:]...)
				continue
			default:
				go func() {
					handler.ch <- Event{event, data}
				}()
				i++
			}
		}
		e.handlers[event] = handlers

		handlers = e.handlersAll
		for i := 0; i < len(handlers); {
			handler := handlers[i]
			select {
			case <-handler.ctx.Done():
				handlers = append(handlers[:i], handlers[i+1:]...)
				continue
			default:
				go func() {
					handler.ch <- Event{event, data}
				}()
				i++
			}
		}
		e.handlersAll = handlers
	})
}

// On registers a handler for a specific event
func (e *BaseEventEmitter) on(ctx context.Context, events []string, ch chan Event) {
	e.sync(func() {
		for _, event := range events {
			_, ok := e.handlers[event]
			if !ok {
				e.handlers[event] = make([]eventHandler, 0)
			}
			eh := eventHandler{ctx, ch}
			e.handlers[event] = append(e.handlers[event], eh)
		}
	})
}

// OnAll registers a handler for all events
func (e *BaseEventEmitter) onAll(ctx context.Context, ch chan Event) {
	e.sync(func() {
		e.handlersAll = append(e.handlersAll, eventHandler{ctx, ch})
	})
}
