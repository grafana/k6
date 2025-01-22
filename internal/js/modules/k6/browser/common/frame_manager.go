package common

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext"
	"go.k6.io/k6/internal/js/modules/k6/browser/log"

	k6modules "go.k6.io/k6/js/modules"

	"github.com/chromedp/cdproto/cdp"
	cdppage "github.com/chromedp/cdproto/page"
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

	barriersMu sync.RWMutex
	barriers   []*Barrier

	vu k6modules.VU

	logger *log.Logger
	id     int64
}

// frameManagerID is used for giving a unique ID to a frame manager.
var frameManagerID int64 //nolint:gochecknoglobals // TODO(@mstoykov): move it to the module instance

// NewFrameManager creates a new HTML document frame manager.
func NewFrameManager(
	ctx context.Context,
	s session,
	p *Page,
	ts *TimeoutSettings,
	l *log.Logger,
) *FrameManager {
	m := &FrameManager{
		ctx:             ctx,
		session:         s,
		page:            p,
		timeoutSettings: ts,
		frames:          make(map[cdp.FrameID]*Frame),
		barriers:        make([]*Barrier, 0),
		vu:              k6ext.GetVU(ctx),
		logger:          l,
		id:              atomic.AddInt64(&frameManagerID, 1),
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

func (m *FrameManager) frameAbortedNavigation(frameID cdp.FrameID, errorText, documentID string) {
	m.logger.Debugf("FrameManager:frameAbortedNavigation",
		"fmid:%d fid:%v err:%s docid:%s",
		m.ID(), frameID, errorText, documentID)

	m.framesMu.Lock()
	defer m.framesMu.Unlock()

	frame := m.frames[frameID]
	if frame == nil {
		return
	}

	frame.pendingDocumentMu.Lock()

	if frame.pendingDocument == nil {
		frame.pendingDocumentMu.Unlock()
		return
	}
	if documentID != "" && frame.pendingDocument.documentID != documentID {
		frame.pendingDocumentMu.Unlock()
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

	frame.pendingDocumentMu.Unlock()

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
	}
}

func (m *FrameManager) frameDetached(frameID cdp.FrameID, reason cdppage.FrameDetachedReason) error {
	m.logger.Debugf("FrameManager:frameDetached", "fmid:%d fid:%v", m.ID(), frameID)

	frame, ok := m.getFrameByID(frameID)
	if !ok {
		m.logger.Debugf("FrameManager:frameDetached:return",
			"fmid:%d fid:%v cannot find frame",
			m.ID(), frameID)
		return nil
	}

	// This helps prevent an iframe and its child frames from being removed
	// when the type of detach is a swap. After this detach event usually
	// the iframe navigates, which requires the frames to be present for the
	// navigate to work.
	fs, ok := m.page.getFrameSession(frameID)
	if ok {
		m.logger.Debugf("FrameManager:frameDetached:sessionFound",
			"fmid:%d fid:%v fsID1:%v fsID2:%v found session for frame",
			m.ID(), frameID, fs.session.ID(), m.session.ID())

		if fs.session.ID() != m.session.ID() {
			m.logger.Debugf("FrameManager:frameDetached:notSameSession:return",
				"fmid:%d fid:%v event session and frame session do not match",
				m.ID(), frameID)
			return nil
		}
	}

	if reason == cdppage.FrameDetachedReasonSwap {
		// When a local frame is swapped out for a remote
		// frame, we want to keep the current frame which is
		// still referenced by the (incoming) remote frame, but
		// remove all its child frames.
		return m.removeChildFramesRecursively(frame)
	}

	return m.removeFramesRecursively(frame)
}

func (m *FrameManager) frameLifecycleEvent(frameID cdp.FrameID, event LifecycleEvent) {
	m.logger.Debugf("FrameManager:frameLifecycleEvent",
		"fmid:%d fid:%v event:%s",
		m.ID(), frameID, lifecycleEventToString[event])

	frame, ok := m.getFrameByID(frameID)
	if ok {
		frame.onLifecycleEvent(event)
	}
}

func (m *FrameManager) frameLoadingStarted(frameID cdp.FrameID) {
	m.logger.Debugf("FrameManager:frameLoadingStarted",
		"fmid:%d fid:%v", m.ID(), frameID)

	frame, ok := m.getFrameByID(frameID)
	if ok {
		frame.onLoadingStarted()
	}
}

func (m *FrameManager) frameLoadingStopped(frameID cdp.FrameID) {
	m.logger.Debugf("FrameManager:frameLoadingStopped",
		"fmid:%d fid:%v", m.ID(), frameID)

	frame, ok := m.getFrameByID(frameID)
	if ok {
		frame.onLoadingStopped()
	}
}

//nolint:funlen
func (m *FrameManager) frameNavigated(
	frameID cdp.FrameID, parentFrameID cdp.FrameID, documentID string, name string, url string, initial bool,
) error {
	m.logger.Debugf("FrameManager:frameNavigated",
		"fmid:%d fid:%v pfid:%v docid:%s fname:%s furl:%s initial:%t",
		m.ID(), frameID, parentFrameID, documentID, name, url, initial)

	// TODO: add test to make sure the navigated frame has correct ID, parent ID, loader ID, name and URL
	m.framesMu.Lock()
	defer m.framesMu.Unlock()

	isMainFrame := parentFrameID == ""
	frame := m.frames[frameID]

	if !isMainFrame && frame == nil {
		m.logger.Debugf("FrameManager:frameNavigated:nil frame",
			"fmid:%d fid:%v pfid:%v docid:%s fname:%s furl:%s initial:%t",
			m.ID(), frameID, parentFrameID, documentID, name, url, initial)

		// If the frame is nil at this point, then the cause of this is likely
		// due to chrome not sending a frameAttached event ahead of time. This
		// isn't a bug in chrome, and seems to be intended behavior. Instead
		// of worrying about the nil frame and causing the test to fail when
		// the frame is nil, we can instead return early. The frame will
		// be initialized when getFrameTree CDP request is made, which will
		// call onFrameAttached and onFrameNavigated.

		return nil
	}

	m.logger.Debugf("FrameManager:frameNavigated:removeFrames",
		"fmid:%d fid:%v pfid:%v docid:%s fname:%s furl:%s initial:%t",
		m.ID(), frameID, parentFrameID, documentID, name, url, initial)

	if frame != nil {
		m.framesMu.Unlock()
		for _, child := range frame.ChildFrames() {
			if err := m.removeFramesRecursively(child); err != nil {
				m.framesMu.Lock()
				return fmt.Errorf("removing child frames recursively: %w", err)
			}
		}
		m.framesMu.Lock()
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

	frame.pendingDocumentMu.Lock()
	defer frame.pendingDocumentMu.Unlock()

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

		// If a frame doesn't exist then the call to this method (which
		// originates from a EventFrameRequestedNavigation CDP event) is on a
		// stale frame that no longer exists in memory.
		return nil
	}

	m.barriersMu.RLock()
	defer m.barriersMu.RUnlock()
	for _, b := range m.barriers {
		m.logger.Debugf("FrameManager:frameRequestedNavigation:AddFrameNavigation",
			"fmid:%d fid:%v furl:%s url:%s docid:%s", m.ID(), frameID, frame.URL(), url, documentID)

		b.AddFrameNavigation(frame)
	}

	frame.pendingDocumentMu.Lock()
	defer frame.pendingDocumentMu.Unlock()

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

// getFrameByID finds a frame with id. If found, it returns the frame and true,
// otherwise, it returns nil and false.
func (m *FrameManager) getFrameByID(id cdp.FrameID) (*Frame, bool) {
	m.framesMu.RLock()
	defer m.framesMu.RUnlock()

	frame, ok := m.frames[id]

	return frame, ok
}

func (m *FrameManager) removeChildFramesRecursively(frame *Frame) error {
	for _, child := range frame.ChildFrames() {
		if err := m.removeFramesRecursively(child); err != nil {
			return fmt.Errorf("removing child frames recursively: %w", err)
		}
	}

	return nil
}

func (m *FrameManager) removeFramesRecursively(frame *Frame) error {
	for _, child := range frame.ChildFrames() {
		m.logger.Debugf("FrameManager:removeFramesRecursively",
			"fmid:%d cfid:%v pfid:%v cfname:%s cfurl:%s",
			m.ID(), child.ID(), frame.ID(), child.Name(), child.URL())

		if err := m.removeFramesRecursively(child); err != nil {
			return fmt.Errorf("removing frames recursively: %w", err)
		}
	}

	if err := frame.detach(); err != nil {
		return fmt.Errorf("removing frames recursively: detaching frame: %w", err)
	}

	m.framesMu.Lock()
	m.logger.Debugf("FrameManager:removeFramesRecursively:delParentFrame",
		"fmid:%d fid:%v fname:%s furl:%s",
		m.ID(), frame.ID(), frame.Name(), frame.URL())

	delete(m.frames, cdp.FrameID(frame.ID()))
	m.framesMu.Unlock()

	return nil
}

func (m *FrameManager) requestFailed(req *Request, canceled bool) {
	m.logger.Debugf("FrameManager:requestFailed", "fmid:%d rurl:%s", m.ID(), req.URL())

	frame := req.getFrame()
	if frame == nil {
		m.logger.Debugf("FrameManager:requestFailed", "frame is nil")
		return
	}
	frame.deleteRequest(req.getID())

	frame.pendingDocumentMu.RLock()
	if frame.pendingDocument == nil || frame.pendingDocument.request != req {
		m.logger.Debugf("FrameManager:requestFailed:return", "fmid:%d pdoc:nil", m.ID())
		frame.pendingDocumentMu.RUnlock()
		return
	}

	errorText := req.errorText
	if canceled {
		errorText += "; maybe frame was detached?"
	}

	docID := frame.pendingDocument.documentID
	frame.pendingDocumentMu.RUnlock()

	m.frameAbortedNavigation(cdp.FrameID(frame.ID()), errorText, docID)
}

func (m *FrameManager) requestFinished(req *Request) {
	m.logger.Debugf("FrameManager:requestFinished", "fmid:%d rurl:%s",
		m.ID(), req.URL())

	frame := req.getFrame()
	if frame == nil {
		m.logger.Debugf("FrameManager:requestFinished:return",
			"fmid:%d rurl:%s frame:nil", m.ID(), req.URL())
		return
	}
	frame.deleteRequest(req.getID())
	/*
		else if frame.inflightRequestsLen() <= 10 {
			for reqID, _ := range frame.inflightRequests {
				req := frame.requestByID(reqID)
			}
		}
	*/
}

func (m *FrameManager) requestStarted(req *Request) {
	m.logger.Debugf("FrameManager:requestStarted", "fmid:%d rurl:%s", m.ID(), req.URL())

	m.framesMu.Lock()
	defer m.framesMu.Unlock()

	frame := req.getFrame()
	if frame == nil {
		m.logger.Debugf("FrameManager:requestStarted:return",
			"fmid:%d rurl:%s frame:nil", m.ID(), req.URL())
		return
	}

	frame.addRequest(req.getID())
	if req.documentID != "" {
		frame.pendingDocumentMu.Lock()
		frame.pendingDocument = &DocumentInfo{documentID: req.documentID, request: req}
		frame.pendingDocumentMu.Unlock()
	}
	m.logger.Debugf("FrameManager:requestStarted", "fmid:%d rurl:%s pdoc:nil", m.ID(), req.URL())
}

// Frames returns a list of frames on the page.
func (m *FrameManager) Frames() []*Frame {
	m.framesMu.RLock()
	defer m.framesMu.RUnlock()
	frames := make([]*Frame, 0)
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

// MainFrameURL returns the main frame's url.
func (m *FrameManager) MainFrameURL() string {
	m.mainFrameMu.RLock()
	defer m.mainFrameMu.RUnlock()

	return m.mainFrame.URL()
}

// NavigateFrame will navigate specified frame to specified URL.
//
//nolint:funlen
func (m *FrameManager) NavigateFrame(frame *Frame, url string, parsedOpts *FrameGotoOptions) (*Response, error) {
	var (
		fmid = m.ID()
		fid  = frame.ID()
		furl = frame.URL()
	)
	m.logger.Debugf("FrameManager:NavigateFrame",
		"fmid:%d fid:%v furl:%s url:%s", fmid, fid, furl, url)
	defer m.logger.Debugf("FrameManager:NavigateFrame:return",
		"fmid:%d fid:%v furl:%s url:%s", fmid, fid, furl, url)

	timeoutCtx, timeoutCancelFn := context.WithTimeout(m.ctx, parsedOpts.Timeout)
	defer timeoutCancelFn()

	newDocIDCh := make(chan string, 1)
	navEvtCh, navEvtCancel := createWaitForEventHandler(
		timeoutCtx, frame, []string{EventFrameNavigation},
		func(data any) bool {
			newDocID := <-newDocIDCh
			if evt, ok := data.(*NavigationEvent); ok {
				if evt.newDocument != nil {
					return evt.newDocument.documentID == newDocID
				}
			}
			return false
		})
	defer navEvtCancel()

	lifecycleEvtCh, lifecycleEvtCancel := createWaitForEventPredicateHandler(
		timeoutCtx, frame, []string{EventFrameAddLifecycle},
		func(data any) bool {
			le, ok := data.(FrameLifecycleEvent)
			if !ok {
				return false
			}
			// skip the initial blank page if we are navigating to a non-blank page.
			// otherwise, we will get a lifecycle event for the initial blank page
			// and return prematurely before waiting for the navigation to complete.
			if url != BlankPage && le.URL == BlankPage {
				m.logger.Debugf(
					"FrameManager:NavigateFrame:createWaitForEventPredicateHandler",
					"fmid:%d fid:%v furl:%s url:%s waitUntil:%s event.lifecycle:%q event.url:%q skipping %s",
					fmid, fid, furl, url, parsedOpts.WaitUntil, le.Event, le.URL, BlankPage,
				)
				return false
			}

			return le.Event == parsedOpts.WaitUntil
		})
	defer lifecycleEvtCancel()

	fs, ok := frame.page.getFrameSession(cdp.FrameID(frame.ID()))
	if !ok {
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
		return nil, fmt.Errorf("navigating to %q: %w", url, err)
	}

	if newDocumentID == "" {
		// It's a navigation within the same document (e.g. via anchor links or
		// the History API), so don't wait for a response nor any lifecycle
		// events.
		return nil, nil //nolint:nilnil
	}

	// unblock the waiter goroutine
	newDocIDCh <- newDocumentID

	wrapTimeoutError := func(err error) error {
		if errors.Is(err, context.DeadlineExceeded) {
			err = &k6ext.UserFriendlyError{
				Err:     err,
				Timeout: parsedOpts.Timeout,
			}
			return fmt.Errorf("navigating to %q: %w", url, err)
		}
		m.logger.Debugf("FrameManager:NavigateFrame",
			"fmid:%d fid:%v furl:%s url:%s timeoutCtx done: %v",
			fmid, fid, furl, url, err)

		return err // TODO maybe wrap this as well?
	}

	var resp *Response
	select {
	case evt := <-navEvtCh:
		if e, ok := evt.(*NavigationEvent); ok {
			req := e.newDocument.request
			// Request could be nil in case of navigation to e.g. BlankPage.
			if req != nil {
				req.responseMu.RLock()
				resp = req.response
				req.responseMu.RUnlock()
			}
		}
	case <-timeoutCtx.Done():
		return nil, wrapTimeoutError(timeoutCtx.Err())
	}

	select {
	case <-lifecycleEvtCh:
	case <-timeoutCtx.Done():
		return nil, wrapTimeoutError(timeoutCtx.Err())
	}

	return resp, nil
}

// Page returns the page that this frame manager belongs to.
func (m *FrameManager) Page() *Page {
	if m.page != nil {
		return m.page
	}
	return nil
}

// ID returns the unique ID of a FrameManager value.
func (m *FrameManager) ID() int64 {
	return atomic.LoadInt64(&m.id)
}
