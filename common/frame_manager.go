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
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/grafana/xk6-browser/api"
	"github.com/grafana/xk6-browser/k6ext"
	"github.com/grafana/xk6-browser/log"

	k6common "go.k6.io/k6/js/common"
	k6modules "go.k6.io/k6/js/modules"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/dop251/goja"
)

// FrameManager manages all frames in a page and their life-cycles, it's a purely internal component.
type FrameManager struct {
	ctx             context.Context
	session         session
	page            *Page
	timeoutSettings *TimeoutSettings

	// protects from the data race between:
	// - Frame.startNetworkIdleTimer.frameLifecycleEvent.recalculateLifecycle
	// - *FrameSession.initEvents.onFrameNavigated->FrameManager.frameNavigated
	mainFrameMu sync.RWMutex
	mainFrame   *Frame

	// Needed as the frames map will be accessed from multiple Go routines,
	// the main VU/JS go routine and the Go routine listening for CDP messages.
	framesMu sync.RWMutex
	frames   map[cdp.FrameID]*Frame

	inflightRequests map[network.RequestID]bool

	barriersMu sync.RWMutex
	barriers   []*Barrier

	vu k6modules.VU

	logger *log.Logger
	id     int64
}

// frameManagerID is used for giving a unique ID to a frame manager.
var frameManagerID int64

// NewFrameManager creates a new HTML document frame manager.
func NewFrameManager(
	ctx context.Context,
	s session,
	p *Page,
	ts *TimeoutSettings,
	l *log.Logger,
) *FrameManager {
	m := &FrameManager{
		ctx:              ctx,
		session:          s,
		page:             p,
		timeoutSettings:  ts,
		frames:           make(map[cdp.FrameID]*Frame),
		inflightRequests: make(map[network.RequestID]bool),
		barriers:         make([]*Barrier, 0),
		vu:               k6ext.GetVU(ctx),
		logger:           l,
		id:               atomic.AddInt64(&frameManagerID, 1),
	}

	m.logger.Debugf("FrameManager:New", "fmid:%d", m.ID())

	return m
}

func (m *FrameManager) addBarrier(b *Barrier) {
	m.logger.Debugf("FrameManager:addBarrier", "fmid:%d", m.ID())

	m.barriersMu.Lock()
	defer m.barriersMu.Unlock()
	m.barriers = append(m.barriers, b)
}

func (m *FrameManager) removeBarrier(b *Barrier) {
	m.logger.Debugf("FrameManager:removeBarrier", "fmid:%d", m.ID())

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
	m.logger.Debugf("FrameManager:dispose", "fmid:%d", m.ID())

	m.framesMu.RLock()
	defer m.framesMu.RUnlock()
	for _, f := range m.frames {
		f.stopNetworkIdleTimer()
	}
}

func (m *FrameManager) frameAbortedNavigation(frameID cdp.FrameID, errorText, documentID string) {
	m.logger.Debugf("FrameManager:frameAbortedNavigation",
		"fmid:%d fid:%v err:%s docid:%s",
		m.ID(), frameID, errorText, documentID)

	m.framesMu.Lock()
	defer m.framesMu.Unlock()

	frame := m.frames[frameID]
	if frame == nil || frame.pendingDocument == nil {
		return
	}
	if documentID != "" && frame.pendingDocument.documentID != documentID {
		return
	}

	m.logger.Debugf("FrameManager:frameAbortedNavigation:emit:EventFrameNavigation",
		"fmid:%d fid:%v err:%s docid:%s fname:%s furl:%s",
		m.ID(), frameID, errorText, documentID, frame.Name(), frame.URL())

	ne := &NavigationEvent{
		url:         frame.URL(),
		name:        frame.Name(),
		newDocument: frame.pendingDocument,
		err:         errors.New(errorText),
	}
	frame.pendingDocument = nil
	frame.emit(EventFrameNavigation, ne)
}

func (m *FrameManager) frameAttached(frameID cdp.FrameID, parentFrameID cdp.FrameID) {
	m.logger.Debugf("FrameManager:frameAttached", "fmid:%d fid:%v pfid:%v",
		m.ID(), frameID, parentFrameID)

	m.framesMu.Lock()
	defer m.framesMu.Unlock()

	if _, ok := m.frames[frameID]; ok {
		m.logger.Debugf("FrameManager:frameAttached:return",
			"fmid:%d fid:%v pfid:%v cannot find frame",
			m.ID(), frameID, parentFrameID)

		return
	}
	if parentFrame, ok := m.frames[parentFrameID]; ok {
		frame := NewFrame(m.ctx, m, parentFrame, frameID, m.logger)
		// TODO: create a addFrame func
		m.frames[frameID] = frame
		parentFrame.addChildFrame(frame)

		m.logger.Debugf("FrameManager:frameAttached:emit:EventPageFrameAttached",
			"fmid:%d fid:%v pfid:%v", m.ID(), frameID, parentFrameID)

		m.page.emit(EventPageFrameAttached, frame)
	}
}

func (m *FrameManager) frameDetached(frameID cdp.FrameID) {
	m.logger.Debugf("FrameManager:frameDetached", "fmid:%d fid:%v", m.ID(), frameID)

	// TODO: use getFrameByID here
	m.framesMu.RLock()
	frame, ok := m.frames[frameID]
	m.framesMu.RUnlock()
	if !ok {
		m.logger.Debugf("FrameManager:frameDetached:return",
			"fmid:%d fid:%v cannot find frame",
			m.ID(), frameID)
		return
	}
	// TODO: possible data race? the frame may have gone.
	m.removeFramesRecursively(frame)
}

func (m *FrameManager) frameLifecycleEvent(frameID cdp.FrameID, event LifecycleEvent) {
	m.logger.Debugf("FrameManager:frameLifecycleEvent",
		"fmid:%d fid:%v event:%s",
		m.ID(), frameID, lifecycleEventToString[event])

	frame := m.getFrameByID(frameID)
	if frame != nil {
		frame.onLifecycleEvent(event)
		m.MainFrame().recalculateLifecycle() // Recalculate life cycle state from the top
	}
}

func (m *FrameManager) frameLoadingStarted(frameID cdp.FrameID) {
	m.logger.Debugf("FrameManager:frameLoadingStarted",
		"fmid:%d fid:%v", m.ID(), frameID)

	frame := m.getFrameByID(frameID)
	if frame != nil {
		frame.onLoadingStarted()
	}
}

func (m *FrameManager) frameLoadingStopped(frameID cdp.FrameID) {
	m.logger.Debugf("FrameManager:frameLoadingStopped",
		"fmid:%d fid:%v", m.ID(), frameID)

	frame := m.getFrameByID(frameID)
	if frame != nil {
		frame.onLoadingStopped()
	}
}

func (m *FrameManager) frameNavigated(frameID cdp.FrameID, parentFrameID cdp.FrameID, documentID string, name string, url string, initial bool) error {
	m.logger.Debugf("FrameManager:frameNavigated",
		"fmid:%d fid:%v pfid:%v docid:%s fname:%s furl:%s initial:%t",
		m.ID(), frameID, parentFrameID, documentID, name, url, initial)

	// TODO: add test to make sure the navigated frame has correct ID, parent ID, loader ID, name and URL
	m.framesMu.Lock()
	defer m.framesMu.Unlock()

	isMainFrame := parentFrameID == ""
	frame := m.frames[frameID]

	if !(isMainFrame || frame != nil) {
		return errors.New("we either navigate top level or have old version of the navigated frame")
	}

	m.logger.Debugf("FrameManager:frameNavigated:removeFrames",
		"fmid:%d fid:%v pfid:%v docid:%s fname:%s furl:%s initial:%t",
		m.ID(), frameID, parentFrameID, documentID, name, url, initial)

	if frame != nil {
		for _, child := range frame.ChildFrames() {
			m.removeFramesRecursively(child.(*Frame))
		}
	}

	var mainFrame *Frame
	if isMainFrame && frame == nil {
		m.logger.Debugf("FrameManager:frameNavigated:MainFrame:initialMainFrameNavigation",
			"fmid:%d fid:%v pfid:%v docid:%s fname:%s furl:%s initial:%t",
			m.ID(), frameID, parentFrameID, documentID, name, url, initial)

		// Initial main frame navigation.
		frame = NewFrame(m.ctx, m, nil, frameID, m.logger)
		mainFrame = frame
	} else if isMainFrame && frame.ID() != string(frameID) {
		m.logger.Debugf("FrameManager:frameNavigated:MainFrame:delete",
			"fmid:%d fid:%v pfid:%v docid:%s fname:%s furl:%s initial:%t oldfid:%v",
			m.ID(), frameID, parentFrameID, documentID, name, url, initial, frame.ID())

		// Update frame ID to retain frame identity on cross-process navigation.
		delete(m.frames, cdp.FrameID(frame.ID()))
		frame.setID(frameID)
		mainFrame = frame
	}
	if mainFrame != nil {
		m.frames[frameID] = frame
		m.setMainFrame(frame)
	}

	frame.navigated(name, url, documentID)

	var (
		keepPending     *DocumentInfo
		pendingDocument = frame.pendingDocument
	)
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

	m.logger.Debugf("FrameManager:frameNavigated",
		"fmid:%d fid:%v pfid:%v docid:%s fname:%s furl:%s initial:%t pdoc:nil - fcurdoc:%v",
		m.ID(), frameID, parentFrameID, documentID, name, url, initial, documentID)

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
	m.logger.Debugf("FrameManager:frameNavigatedWithinDocument",
		"fmid:%d fid:%v url:%s", m.ID(), frameID, url)

	m.framesMu.Lock()
	defer m.framesMu.Unlock()

	frame := m.frames[frameID]
	if frame == nil {
		m.logger.Debugf("FrameManager:frameNavigatedWithinDocument:nilFrame:return",
			"fmid:%d fid:%v url:%s", m.ID(), frameID, url)

		return
	}

	m.logger.Debugf("FrameManager:frameNavigatedWithinDocument",
		"fmid:%d fid:%v furl:%s url:%s", m.ID(), frameID, frame.URL(), url)

	frame.setURL(url)
	frame.emit(EventFrameNavigation, &NavigationEvent{url: url, name: frame.Name()})
}

func (m *FrameManager) frameRequestedNavigation(frameID cdp.FrameID, url string, documentID string) error {
	m.logger.Debugf("FrameManager:frameRequestedNavigation",
		"fmid:%d fid:%v url:%s docid:%s", m.ID(), frameID, url, documentID)

	m.framesMu.Lock()
	defer m.framesMu.Unlock()

	frame := m.frames[frameID]
	if frame == nil {
		m.logger.Debugf("FrameManager:frameRequestedNavigation:nilFrame:return",
			"fmid:%d fid:%v url:%s docid:%s", m.ID(), frameID, url, documentID)

		return fmt.Errorf("no frame exists with ID %s", frameID)
	}

	m.barriersMu.RLock()
	defer m.barriersMu.RUnlock()
	for _, b := range m.barriers {
		m.logger.Debugf("FrameManager:frameRequestedNavigation:AddFrameNavigation",
			"fmid:%d fid:%v furl:%s url:%s docid:%s", m.ID(), frameID, frame.URL(), url, documentID)

		b.AddFrameNavigation(frame)
	}

	if frame.pendingDocument != nil && frame.pendingDocument.documentID == documentID {
		m.logger.Debugf("FrameManager:frameRequestedNavigation:return",
			"fmid:%d fid:%v furl:%s url:%s docid:%s pdocid:%s pdoc:dontSet",
			m.ID(), frameID, frame.URL(), url, documentID,
			frame.pendingDocument.documentID)

		// Do not override request with nil
		return nil
	}

	m.logger.Debugf("FrameManager:frameRequestedNavigation:return",
		"fmid:%d fid:%v furl:%s url:%s docid:%s pdoc:set",
		m.ID(), frameID, frame.URL(), url, documentID)

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
		m.logger.Debugf("FrameManager:removeFramesRecursively",
			"fmid:%d cfid:%v pfid:%v cfname:%s cfurl:%s",
			m.ID(), child.ID(), frame.ID(), child.Name(), child.URL())

		m.removeFramesRecursively(child.(*Frame))
	}

	frame.detach()

	m.framesMu.Lock()
	m.logger.Debugf("FrameManager:removeFramesRecursively:delParentFrame",
		"fmid:%d fid:%v fname:%s furl:%s",
		m.ID(), frame.ID(), frame.Name(), frame.URL())

	delete(m.frames, cdp.FrameID(frame.ID()))
	m.framesMu.Unlock()

	if !m.page.IsClosed() {
		m.logger.Debugf("FrameManager:removeFramesRecursively:emit:EventPageFrameDetached",
			"fmid:%d fid:%v fname:%s furl:%s",
			m.ID(), frame.ID(), frame.Name(), frame.URL())

		m.page.emit(EventPageFrameDetached, frame)
	}
}

func (m *FrameManager) requestFailed(req *Request, canceled bool) {
	m.logger.Debugf("FrameManager:requestFailed", "fmid:%d rurl:%s", m.ID(), req.URL())

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

			m.logger.Debugf("FrameManager:requestFailed:rc<=10",
				"reqID:%s inflightURL:%s frameID:%s",
				reqID, req.URL(), frame.ID())
		}
	}

	if frame.pendingDocument == nil || frame.pendingDocument.request != req {
		m.logger.Debugf("FrameManager:requestFailed:return", "fmid:%d pdoc:nil", m.ID())
		return
	}
	errorText := req.errorText
	if canceled {
		errorText += "; maybe frame was detached?"
	}
	m.frameAbortedNavigation(cdp.FrameID(frame.ID()), errorText,
		frame.pendingDocument.documentID)
}

func (m *FrameManager) requestFinished(req *Request) {
	m.logger.Debugf("FrameManager:requestFinished", "fmid:%d rurl:%s",
		m.ID(), req.URL())

	delete(m.inflightRequests, req.getID())
	defer m.page.emit(EventPageRequestFinished, req)

	frame := req.getFrame()
	if frame == nil {
		m.logger.Debugf("FrameManager:requestFinished:return",
			"fmid:%d rurl:%s frame:nil", m.ID(), req.URL())
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
	m.logger.Debugf("FrameManager:requestReceivedResponse", "fmid:%d rurl:%s", m.ID(), res.URL())

	m.page.emit(EventPageResponse, res)
}

func (m *FrameManager) requestStarted(req *Request) {
	m.logger.Debugf("FrameManager:requestStarted", "fmid:%d rurl:%s", m.ID(), req.URL())

	m.framesMu.Lock()
	defer m.framesMu.Unlock()
	defer m.page.emit(EventPageRequest, req)

	m.inflightRequests[req.getID()] = true
	frame := req.getFrame()
	if frame == nil {
		m.logger.Debugf("FrameManager:requestStarted:return",
			"fmid:%d rurl:%s frame:nil", m.ID(), req.URL())
		return
	}

	frame.addRequest(req.getID())
	if frame.inflightRequestsLen() == 1 {
		frame.stopNetworkIdleTimer()
	}
	if req.documentID != "" {
		frame.pendingDocument = &DocumentInfo{documentID: req.documentID, request: req}
	}
	m.logger.Debugf("FrameManager:requestStarted", "fmid:%d rurl:%s pdoc:nil", m.ID(), req.URL())
}

// Frames returns a list of frames on the page.
func (m *FrameManager) Frames() []api.Frame {
	m.framesMu.RLock()
	defer m.framesMu.RUnlock()
	frames := make([]api.Frame, 0)
	for _, frame := range m.frames {
		frames = append(frames, frame)
	}
	return frames
}

// MainFrame returns the main frame of the page.
func (m *FrameManager) MainFrame() *Frame {
	m.mainFrameMu.RLock()
	defer m.mainFrameMu.RUnlock()

	return m.mainFrame
}

// setMainFrame sets the main frame of the page.
func (m *FrameManager) setMainFrame(f *Frame) {
	m.mainFrameMu.Lock()
	defer m.mainFrameMu.Unlock()

	m.logger.Debugf("FrameManager:setMainFrame",
		"fmid:%d fid:%v furl:%s",
		m.ID(), f.ID(), f.URL())

	m.mainFrame = f
}

// NavigateFrame will navigate specified frame to specified URL.
func (m *FrameManager) NavigateFrame(frame *Frame, url string, opts goja.Value) api.Response {
	var (
		fmid = m.ID()
		fid  = frame.ID()
		furl = frame.URL()
	)
	m.logger.Debugf("FrameManager:NavigateFrame",
		"fmid:%d fid:%v furl:%s url:%s", fmid, fid, furl, url)
	defer m.logger.Debugf("FrameManager:NavigateFrame:return",
		"fmid:%d fid:%v furl:%s url:%s", fmid, fid, furl, url)

	rt := m.vu.Runtime()
	netMgr := m.page.mainFrameSession.getNetworkManager()
	defaultReferer := netMgr.extraHTTPHeaders["referer"]
	parsedOpts := NewFrameGotoOptions(defaultReferer, time.Duration(m.timeoutSettings.navigationTimeout())*time.Second)
	if err := parsedOpts.Parse(m.ctx, opts); err != nil {
		k6ext.Panic(m.ctx, "parsing frame navigation options to %q: %v", url, err)
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

	fs := frame.page.getFrameSession(cdp.FrameID(frame.ID()))
	if fs == nil {
		m.logger.Debugf("FrameManager:NavigateFrame",
			"fmid:%d fid:%v furl:%s url:%s fs:nil",
			fmid, fid, furl, url)

		// Attaching an iframe to an existing page doesn't seem to trigger a "Target.attachedToTarget" event
		// from the browser even when "Target.setAutoAttach" is true. If this is the case fallback to the
		// main frame's session.
		fs = frame.page.mainFrameSession
	}
	newDocumentID, err := fs.navigateFrame(frame, url, parsedOpts.Referer)
	if err != nil {
		k6ext.Panic(m.ctx, "navigating to %q: %v", url, err)
	}

	var event *NavigationEvent
	if newDocumentID != "" {
		m.logger.Debugf("FrameManager:NavigateFrame",
			"fmid:%d fid:%v furl:%s url:%s newDocID:%s",
			fmid, fid, furl, url, newDocumentID)

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
			k6ext.Panic(m.ctx, "navigating to %q: %v", url, err)
		}

		event = data.(*NavigationEvent)
		if event.newDocument.documentID != newDocumentID {
			m.logger.Debugf("FrameManager:NavigateFrame:interrupted",
				"fmid:%d fid:%v furl:%s url:%s docID:%s newDocID:%s",
				fmid, fid, furl, url, event.newDocument.documentID, newDocumentID)
		} else if event.err != nil &&
			// TODO: A more graceful way of avoiding Throw()?
			!(netMgr.userReqInterceptionEnabled &&
				strings.Contains(event.err.Error(), "ERR_BLOCKED_BY_CLIENT")) {
			k6common.Throw(rt, event.err)
		}
	} else {
		m.logger.Debugf("FrameManager:NavigateFrame",
			"fmid:%d fid:%v furl:%s url:%s newDocID:0",
			fmid, fid, furl, url)

		select {
		case <-timeoutCtx.Done():
			if timeoutCtx.Err() == context.DeadlineExceeded {
				k6ext.Panic(m.ctx, "navigating to %q: %s after %s", url, ErrTimedOut, parsedOpts.Timeout)
			}
		case data := <-chSameDoc:
			event = data.(*NavigationEvent)
		}
	}

	if !frame.hasSubtreeLifecycleEventFired(parsedOpts.WaitUntil) {
		m.logger.Debugf("FrameManager:NavigateFrame",
			"fmid:%d fid:%v furl:%s url:%s hasSubtreeLifecycleEventFired:false",
			fmid, fid, furl, url)

		select {
		case <-timeoutCtx.Done():
			if timeoutCtx.Err() == context.DeadlineExceeded {
				k6ext.Panic(m.ctx, "navigating to %q: %s after %s", url, ErrTimedOut, parsedOpts.Timeout)
			}
		case <-chWaitUntilCh:
		}
	}

	var resp *Response
	if event.newDocument != nil {
		req := event.newDocument.request
		if req != nil && req.response != nil {
			resp = req.response
		}
	}
	return resp
}

// Page returns the page that this frame manager belongs to.
func (m *FrameManager) Page() api.Page {
	if m.page != nil {
		return m.page
	}
	return nil
}

// WaitForFrameNavigation waits for the given navigation lifecycle event to happen.
func (m *FrameManager) WaitForFrameNavigation(frame *Frame, opts goja.Value) api.Response {
	m.logger.Debugf("FrameManager:WaitForFrameNavigation",
		"fmid:%d fid:%s furl:%s",
		m.ID(), frame.ID(), frame.URL())
	defer m.logger.Debugf("FrameManager:WaitForFrameNavigation:return",
		"fmid:%d fid:%s furl:%s",
		m.ID(), frame.ID(), frame.URL())

	parsedOpts := NewFrameWaitForNavigationOptions(time.Duration(m.timeoutSettings.timeout()) * time.Second)
	if err := parsedOpts.Parse(m.ctx, opts); err != nil {
		k6ext.Panic(m.ctx, "parsing wait for frame navigation options: %v", err)
	}

	ch, evCancelFn := createWaitForEventHandler(m.ctx, frame, []string{EventFrameNavigation},
		func(data interface{}) bool {
			return true // Both successful and failed navigations are considered
		})
	defer evCancelFn() // Remove event handler

	var event *NavigationEvent
	select {
	case <-m.ctx.Done():
		// ignore: the extension is shutting down
		m.logger.Warnf("FrameManager:WaitForFrameNavigation:<-ctx.Done",
			"fmid:%d furl:%s err:%v",
			m.ID(), frame.URL(), m.ctx.Err())
		return nil
	case <-time.After(parsedOpts.Timeout):
		k6ext.Panic(m.ctx, "waiting for frame navigation timed out after %s", parsedOpts.Timeout)
	case data := <-ch:
		event = data.(*NavigationEvent)
	}

	if event.newDocument == nil {
		// In case of navigation within the same document (e.g. via an anchor
		// link or the History API), there is no new document and a
		// LifecycleEvent will not be fired, so we don't need to wait for it.
		return nil
	}

	if frame.hasSubtreeLifecycleEventFired(parsedOpts.WaitUntil) {
		m.logger.Debugf("FrameManager:WaitForFrameNavigation",
			"fmid:%d furl:%s hasSubtreeLifecycleEventFired:true",
			m.ID(), frame.URL())

		_, err := waitForEvent(m.ctx, frame, []string{EventFrameAddLifecycle}, func(data interface{}) bool {
			return data.(LifecycleEvent) == parsedOpts.WaitUntil
		}, parsedOpts.Timeout)
		if err != nil {
			k6ext.Panic(m.ctx, "waiting for frame navigation until %q: %v", parsedOpts.WaitUntil, err)
		}
	}

	return event.newDocument.request.response
}

// ID returns the unique ID of a FrameManager value.
func (m *FrameManager) ID() int64 {
	return atomic.LoadInt64(&m.id)
}
