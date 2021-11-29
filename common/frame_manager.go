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
	"fmt"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/dop251/goja"
	"github.com/grafana/xk6-browser/api"
	k6common "go.k6.io/k6/js/common"
)

// FrameManager manages all frames in a page and their life-cycles, it's a purely internal component.
type FrameManager struct {
	ctx             context.Context
	session         *Session
	page            *Page
	timeoutSettings *TimeoutSettings
	mainFrame       *Frame

	// Needed as the frames map will be accessed from multiple Go routines,
	// the main VU/JS go routine and the Go routine listening for CDP messages.
	framesMu sync.RWMutex
	frames   map[cdp.FrameID]*Frame

	inflightRequests map[network.RequestID]bool

	barriersMu sync.RWMutex
	barriers   []*Barrier

	logger *Logger
}

// NewFrameManager creates a new HTML document frame manager.
func NewFrameManager(ctx context.Context, session *Session, page *Page, timeoutSettings *TimeoutSettings, logger *Logger) *FrameManager {
	m := FrameManager{
		ctx:              ctx,
		session:          session,
		page:             page,
		timeoutSettings:  timeoutSettings,
		mainFrame:        nil,
		frames:           make(map[cdp.FrameID]*Frame),
		framesMu:         sync.RWMutex{},
		inflightRequests: make(map[network.RequestID]bool),
		barriers:         make([]*Barrier, 0),
		barriersMu:       sync.RWMutex{},
		logger:           logger,
	}
	return &m
}

func (m *FrameManager) addBarrier(b *Barrier) {
	m.barriersMu.Lock()
	defer m.barriersMu.Unlock()
	m.barriers = append(m.barriers, b)
}

func (m *FrameManager) removeBarrier(b *Barrier) {
	m.barriersMu.Lock()
	defer m.barriersMu.Unlock()
	index := -1
	for i, b2 := range m.barriers {
		if b == b2 {
			index = i
			break
		}
	}
	m.barriers = append(m.barriers[:index], m.barriers[index+1:]...)
}

func (m *FrameManager) dispose() {
	m.framesMu.RLock()
	defer m.framesMu.RUnlock()
	for _, f := range m.frames {
		f.stopNetworkIdleTimer()
	}
}

func (m *FrameManager) frameAbortedNavigation(frameID cdp.FrameID, errorText, documentID string) {
	m.framesMu.Lock()
	defer m.framesMu.Unlock()
	frame := m.frames[frameID]
	if frame == nil || frame.pendingDocument == nil {
		return
	}
	if documentID != "" && frame.pendingDocument.documentID != documentID {
		return
	}
	frame.pendingDocument = nil
	frame.emit(EventFrameNavigation, &NavigationEvent{
		url:         frame.url,
		name:        frame.name,
		newDocument: frame.pendingDocument,
		err:         errors.New(errorText),
	})
}

func (m *FrameManager) frameAttached(frameID cdp.FrameID, parentFrameID cdp.FrameID) {
	m.framesMu.Lock()
	defer m.framesMu.Unlock()
	if _, ok := m.frames[frameID]; ok {
		return
	}
	if parentFrame, ok := m.frames[parentFrameID]; ok {
		frame := NewFrame(m.ctx, m, parentFrame, frameID)
		m.frames[frameID] = frame
		parentFrame.addChildFrame(frame)
		m.page.emit(EventPageFrameAttached, frame)
	}
}

func (m *FrameManager) frameDetached(frameID cdp.FrameID) {
	m.framesMu.RLock()
	frame, ok := m.frames[frameID]
	m.framesMu.RUnlock()
	if ok {
		m.removeFramesRecursively(frame)
	}
}

func (m *FrameManager) frameLifecycleEvent(frameID cdp.FrameID, event LifecycleEvent) {
	frame := m.getFrameByID(frameID)
	if frame != nil {
		frame.onLifecycleEvent(event)
		m.mainFrame.recalculateLifecycle() // Recalculate life cycle state from the top
	}
}

func (m *FrameManager) frameLoadingStarted(frameID cdp.FrameID) {
	frame := m.getFrameByID(frameID)
	if frame != nil {
		frame.onLoadingStarted()
	}
}

func (m *FrameManager) frameLoadingStopped(frameID cdp.FrameID) {
	frame := m.getFrameByID(frameID)
	if frame != nil {
		frame.onLoadingStopped()
	}
}

func (m *FrameManager) frameNavigated(frameID cdp.FrameID, parentFrameID cdp.FrameID, documentID string, name string, url string, initial bool) error {
	// TODO: add test to make sure the navigated frame has correct ID, parent ID, loader ID, name and URL
	m.framesMu.Lock()
	defer m.framesMu.Unlock()

	isMainFrame := parentFrameID == ""
	frame := m.frames[frameID]

	if !(isMainFrame || frame != nil) {
		return errors.New("we either navigate top level or have old version of the navigated frame")
	}

	if frame != nil {
		for _, child := range frame.ChildFrames() {
			m.removeFramesRecursively(child.(*Frame))
		}
	}

	if isMainFrame {
		if frame != nil {
			// Update frame ID to retain frame identity on cross-process navigation.
			delete(m.frames, cdp.FrameID(frame.ID()))
			frame.setID(frameID)
		} else {
			// Initial main frame navigation.
			frame = NewFrame(m.ctx, m, nil, frameID)
		}
		m.frames[frameID] = frame
		m.mainFrame = frame
	}

	frame.navigated(name, url, documentID)

	var keepPending *DocumentInfo
	pendingDocument := frame.pendingDocument
	if pendingDocument != nil {
		if pendingDocument.documentID == "" {
			pendingDocument.documentID = documentID
		}
		if pendingDocument.documentID == documentID {
			// Committing a pending document.
			frame.currentDocument = pendingDocument
		} else {
			// Sometimes, we already have a new pending when the old one commits.
			// An example would be Chromium error page followed by a new navigation request,
			// where the error page commit arrives after Network.requestWillBeSent for the
			// new navigation.
			// We commit, but keep the pending request since it's not done yet.
			keepPending = pendingDocument
			frame.currentDocument = &DocumentInfo{
				documentID: documentID,
				request:    nil,
			}
		}
		frame.pendingDocument = nil
	} else {
		// No pending, just commit a new document.
		frame.currentDocument = &DocumentInfo{
			documentID: documentID,
			request:    nil,
		}
	}

	frame.clearLifecycle()
	frame.emit(EventFrameNavigation, &NavigationEvent{url: url, name: name, newDocument: frame.currentDocument})

	// TODO: when we add API support for storage we need to track origins
	// if !initial {
	// 	//f.page.frameNavigatedToNewDocument(f)
	// }

	// Restore pending if any (see comments above about keepPending).
	frame.pendingDocument = keepPending

	return nil
}

func (m *FrameManager) frameNavigatedWithinDocument(frameID cdp.FrameID, url string) {
	m.framesMu.Lock()
	defer m.framesMu.Unlock()
	frame := m.frames[frameID]
	if frame == nil {
		return
	}
	frame.url = url
	frame.emit(EventFrameNavigation, &NavigationEvent{url: url, name: frame.name})
}

func (m *FrameManager) frameRequestedNavigation(frameID cdp.FrameID, url string, documentID string) error {
	m.framesMu.Lock()
	defer m.framesMu.Unlock()

	frame := m.frames[frameID]
	if frame == nil {
		return fmt.Errorf("no frame exists with ID %q", frameID)
	}

	m.barriersMu.RLock()
	defer m.barriersMu.RUnlock()
	for _, b := range m.barriers {
		b.AddFrameNavigation(frame)
	}

	if frame.pendingDocument != nil && frame.pendingDocument.documentID == documentID {
		// Do not override request with nil
		return nil
	}

	frame.pendingDocument = &DocumentInfo{documentID: documentID}
	return nil
}

func (m *FrameManager) getFrameByID(id cdp.FrameID) *Frame {
	m.framesMu.RLock()
	defer m.framesMu.RUnlock()
	return m.frames[id]
}

func (m *FrameManager) removeChildFramesRecursively(frame *Frame) {
	for _, child := range frame.ChildFrames() {
		m.removeFramesRecursively(child.(*Frame))
	}
}

func (m *FrameManager) removeFramesRecursively(frame *Frame) {
	for _, child := range frame.ChildFrames() {
		m.removeFramesRecursively(child.(*Frame))
	}
	frame.detach()

	m.framesMu.Lock()
	delete(m.frames, cdp.FrameID(frame.ID()))
	m.framesMu.Unlock()

	if !m.page.IsClosed() {
		m.page.emit(EventPageFrameDetached, frame)
	}
}

func (m *FrameManager) requestFailed(req *Request, canceled bool) {
	delete(m.inflightRequests, req.getID())
	defer m.page.emit(EventPageRequestFailed, req)

	frame := req.getFrame()
	if frame == nil {
		m.logger.Debugf("FrameManager:requestFailed", "frame is nil")
		return
	}
	frame.deleteRequest(req.getID())

	switch rc := frame.inflightRequestsLen(); {
	case rc == 0:
		frame.startNetworkIdleTimer()
	case rc <= 10:
		for reqID := range frame.inflightRequests {
			req := frame.requestByID(reqID)
			m.logger.Debugf("FrameManager:requestFailed", "reqID:%s inflightURL:%s frameID:%s", reqID, req.url, frame.id)
		}
	}

	if frame.pendingDocument == nil || frame.pendingDocument.request != req {
		return
	}
	errorText := req.errorText
	if canceled {
		errorText += "; maybe frame was detached?"
	}
	m.frameAbortedNavigation(frame.id, errorText, frame.pendingDocument.documentID)
}

func (m *FrameManager) requestFinished(req *Request) {
	delete(m.inflightRequests, req.getID())
	defer m.page.emit(EventPageRequestFinished, req)

	frame := req.getFrame()
	if frame == nil {
		return
	}
	frame.deleteRequest(req.getID())
	if frame.inflightRequestsLen() == 0 {
		frame.startNetworkIdleTimer()
	}
	/*
		else if frame.inflightRequestsLen() <= 10 {
			for reqID, _ := range frame.inflightRequests {
				req := frame.requestByID(reqID)
			}
		}
	*/
}

func (m *FrameManager) requestReceivedResponse(res *Response) {
	m.page.emit(EventPageResponse, res)
}

func (m *FrameManager) requestStarted(req *Request) {
	m.framesMu.Lock()
	defer m.framesMu.Unlock()
	defer m.page.emit(EventPageRequest, req)

	m.inflightRequests[req.getID()] = true
	frame := req.getFrame()
	if frame == nil {
		return
	}
	frame.addRequest(req.getID())
	if frame.inflightRequestsLen() == 1 {
		frame.stopNetworkIdleTimer()
	}
	if req.documentID != "" {
		frame.pendingDocument = &DocumentInfo{documentID: req.documentID, request: req}
	}
}

// Frames returns a list of frames on the page
func (m *FrameManager) Frames() []api.Frame {
	m.framesMu.RLock()
	defer m.framesMu.RUnlock()
	frames := make([]api.Frame, 0)
	for _, frame := range m.frames {
		frames = append(frames, frame)
	}
	return frames
}

// MainFrame returns the main frame of the page
func (m *FrameManager) MainFrame() *Frame {
	return m.mainFrame
}

// NavigateFrame will navigate specified frame to specifed URL
func (m *FrameManager) NavigateFrame(frame *Frame, url string, opts goja.Value) api.Response {
	rt := k6common.GetRuntime(m.ctx)
	defaultReferer := m.page.mainFrameSession.getNetworkManager().extraHTTPHeaders["referer"]
	parsedOpts := NewFrameGotoOptions(defaultReferer, time.Duration(m.timeoutSettings.navigationTimeout())*time.Second)
	if err := parsedOpts.Parse(m.ctx, opts); err != nil {
		k6common.Throw(rt, fmt.Errorf("failed parsing options: %w", err))
	}

	timeoutCtx, timeoutCancelFn := context.WithTimeout(m.ctx, parsedOpts.Timeout)
	defer timeoutCancelFn()

	chSameDoc, evCancelFn := createWaitForEventHandler(timeoutCtx, frame, []string{EventFrameNavigation}, func(data interface{}) bool {
		return data.(*NavigationEvent).newDocument == nil
	})
	defer evCancelFn() // Remove event handler

	chWaitUntilCh, evCancelFn2 := createWaitForEventHandler(timeoutCtx, frame, []string{EventFrameAddLifecycle}, func(data interface{}) bool {
		return data.(LifecycleEvent) == parsedOpts.WaitUntil
	})
	defer evCancelFn2() // Remove event handler

	fs := frame.page.getFrameSession(frame.id)
	if fs == nil {
		// Attaching an iframe to an existing page doesn't seem to trigger a "Target.attachedToTarget" event
		// from the browser even when "Target.setAutoAttach" is true. If this is the case fallback to the
		// main frame's session.
		fs = frame.page.mainFrameSession
	}
	newDocumentID, err := fs.navigateFrame(frame, url, parsedOpts.Referer)
	if err != nil {
		k6common.Throw(rt, err)
	}

	var event *NavigationEvent
	if newDocumentID != "" {
		data, err := waitForEvent(m.ctx, frame, []string{EventFrameNavigation}, func(data interface{}) bool {
			ev := data.(*NavigationEvent)

			// We are interested either in this specific document, or any other document that
			// did commit and replaced the expected document.
			if ev.newDocument != nil && (ev.newDocument.documentID == newDocumentID || ev.err == nil) {
				return true
			}
			return false
		}, parsedOpts.Timeout)
		if err != nil {
			k6common.Throw(rt, err)
		}
		event = data.(*NavigationEvent)
		if event.newDocument.documentID != newDocumentID {
			k6common.Throw(rt, errors.New("navigation interrupted by another one"))
		} else if event.err != nil {
			k6common.Throw(rt, event.err)
		}
	} else {
		select {
		case <-timeoutCtx.Done():
			if timeoutCtx.Err() == context.DeadlineExceeded {
				k6common.Throw(rt, ErrTimedOut)
			}
		case data := <-chSameDoc:
			event = data.(*NavigationEvent)
		}
	}

	if !frame.hasSubtreeLifecycleEventFired(parsedOpts.WaitUntil) {
		select {
		case <-timeoutCtx.Done():
			if timeoutCtx.Err() == context.DeadlineExceeded {
				k6common.Throw(rt, ErrTimedOut)
			}
		case <-chWaitUntilCh:
		}
	}

	var resp *Response
	if event.newDocument != nil {
		req := event.newDocument.request
		if req != nil {
			if req.response != nil {
				resp = req.response
			}
		}
	}
	return resp
}

// Page returns the page that this frame manager belongs to
func (m *FrameManager) Page() api.Page {
	if m.page != nil {
		return m.page
	}
	return nil
}

// WaitForFrameNavigation waits for the given navigation lifecycle event to happen
func (m *FrameManager) WaitForFrameNavigation(frame *Frame, opts goja.Value) api.Response {
	rt := k6common.GetRuntime(m.ctx)
	parsedOpts := NewFrameWaitForNavigationOptions(time.Duration(m.timeoutSettings.timeout()) * time.Second)
	if err := parsedOpts.Parse(m.ctx, opts); err != nil {
		k6common.Throw(rt, fmt.Errorf("failed parsing options: %w", err))
	}

	ch, evCancelFn := createWaitForEventHandler(m.ctx, frame, []string{EventFrameNavigation}, func(data interface{}) bool {
		return true // Both successful and failed navigations are considered
	})
	defer evCancelFn() // Remove event handler

	var event *NavigationEvent
	select {
	case <-m.ctx.Done():
	case <-time.After(parsedOpts.Timeout):
		k6common.Throw(rt, ErrTimedOut)
	case data := <-ch:
		event = data.(*NavigationEvent)
	}

	if frame.hasSubtreeLifecycleEventFired(parsedOpts.WaitUntil) {
		waitForEvent(m.ctx, frame, []string{EventFrameAddLifecycle}, func(data interface{}) bool {
			return data.(LifecycleEvent) == parsedOpts.WaitUntil
		}, parsedOpts.Timeout)
	}

	var resp *Response
	req := event.newDocument.request
	if req != nil {
		if req.response != nil {
			resp = req.response
		}
	}
	return resp
}
