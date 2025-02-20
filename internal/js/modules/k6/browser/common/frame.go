package common

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/runtime"
	"github.com/grafana/sobek"

	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext"
	"go.k6.io/k6/internal/js/modules/k6/browser/log"

	k6modules "go.k6.io/k6/js/modules"
)

// maxRetry controls how many times to retry if an action fails.
const maxRetry = 1

type DocumentInfo struct {
	documentID string
	request    *Request
}

// DOMElementState represents a DOM element state.
type DOMElementState int

// Valid DOM element states.
const (
	DOMElementStateAttached DOMElementState = iota
	DOMElementStateDetached
	DOMElementStateVisible
	DOMElementStateHidden
)

func (s DOMElementState) String() string {
	return domElementStateToString[s]
}

var domElementStateToString = map[DOMElementState]string{ //nolint:gochecknoglobals
	DOMElementStateAttached: "attached",
	DOMElementStateDetached: "detached",
	DOMElementStateVisible:  "visible",
	DOMElementStateHidden:   "hidden",
}

var domElementStateToID = map[string]DOMElementState{ //nolint:gochecknoglobals
	"attached": DOMElementStateAttached,
	"detached": DOMElementStateDetached,
	"visible":  DOMElementStateVisible,
	"hidden":   DOMElementStateHidden,
}

// MarshalJSON marshals the enum as a quoted JSON string.
func (s DOMElementState) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	buffer.WriteString(domElementStateToString[s])
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

// UnmarshalJSON unmarshals a quoted JSON string to the enum value.
func (s *DOMElementState) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return fmt.Errorf("unmarshaling DOM element state: %w", err)
	}
	// Note that if the string cannot be found then it will be set to the zero value.
	*s = domElementStateToID[j]
	return nil
}

// Frame represents a frame in an HTML document.
type Frame struct {
	BaseEventEmitter

	ctx         context.Context
	page        *Page
	manager     *FrameManager
	parentFrame *Frame

	childFramesMu sync.RWMutex
	childFrames   map[*Frame]bool

	propertiesMu sync.RWMutex
	id           cdp.FrameID
	loaderID     string
	name         string
	url          string
	detached     bool
	vu           k6modules.VU

	// A life cycle event is only considered triggered for a frame if the entire
	// frame subtree has also had the life cycle event triggered.
	lifecycleEventsMu sync.RWMutex
	lifecycleEvents   map[LifecycleEvent]bool

	documentHandle *ElementHandle

	executionContextMu sync.RWMutex
	executionContexts  map[executionWorld]frameExecutionContext

	loadingStartedTime time.Time

	inflightRequestsMu sync.RWMutex
	inflightRequests   map[network.RequestID]bool

	currentDocument   *DocumentInfo
	pendingDocumentMu sync.RWMutex
	pendingDocument   *DocumentInfo

	log *log.Logger
}

// NewFrame creates a new HTML document frame.
func NewFrame(
	ctx context.Context, m *FrameManager, parentFrame *Frame, frameID cdp.FrameID, log *log.Logger,
) *Frame {
	if log.DebugMode() {
		var pfid string
		if parentFrame != nil {
			pfid = parentFrame.ID()
		}
		var sid string
		if m != nil && m.session != nil {
			sid = string(m.session.ID())
		}
		log.Debugf("NewFrame", "sid:%s fid:%s pfid:%s", sid, frameID, pfid)
	}

	return &Frame{
		BaseEventEmitter:  NewBaseEventEmitter(ctx),
		ctx:               ctx,
		page:              m.page,
		manager:           m,
		parentFrame:       parentFrame,
		childFrames:       make(map[*Frame]bool),
		id:                frameID,
		vu:                k6ext.GetVU(ctx),
		lifecycleEvents:   make(map[LifecycleEvent]bool),
		inflightRequests:  make(map[network.RequestID]bool),
		executionContexts: make(map[executionWorld]frameExecutionContext),
		currentDocument:   &DocumentInfo{},
		log:               log,
	}
}

func (f *Frame) addChildFrame(child *Frame) {
	f.log.Debugf("Frame:addChildFrame",
		"fid:%s cfid:%s furl:%q cfurl:%q",
		f.ID(), child.ID(), f.URL(), child.URL())

	f.childFramesMu.Lock()
	defer f.childFramesMu.Unlock()

	f.childFrames[child] = true
}

func (f *Frame) addRequest(id network.RequestID) {
	f.log.Debugf("Frame:addRequest", "fid:%s furl:%q rid:%s", f.ID(), f.URL(), id)

	f.inflightRequestsMu.Lock()
	defer f.inflightRequestsMu.Unlock()

	f.inflightRequests[id] = true
}

func (f *Frame) deleteRequest(id network.RequestID) {
	f.log.Debugf("Frame:deleteRequest", "fid:%s furl:%q rid:%s", f.ID(), f.URL(), id)

	f.inflightRequestsMu.Lock()
	defer f.inflightRequestsMu.Unlock()

	delete(f.inflightRequests, id)
}

func (f *Frame) clearLifecycle() {
	f.log.Debugf("Frame:clearLifecycle", "fid:%s furl:%q", f.ID(), f.URL())

	// clear lifecycle events
	f.lifecycleEventsMu.Lock()
	f.lifecycleEvents = make(map[LifecycleEvent]bool)
	f.lifecycleEventsMu.Unlock()

	// keep the request related to the document if present
	// in f.inflightRequests
	f.inflightRequestsMu.Lock()
	{
		// currentDocument may not always have a request
		// associated with it. see: frame_manager.go
		cdr := f.currentDocument.request

		inflightRequests := make(map[network.RequestID]bool)
		for req := range f.inflightRequests {
			if cdr != nil && req != cdr.requestID {
				continue
			}
			inflightRequests[req] = true
		}
		f.inflightRequests = inflightRequests
	}
	f.inflightRequestsMu.Unlock()
}

func (f *Frame) detach() error {
	f.log.Debugf("Frame:detach", "fid:%s furl:%q", f.ID(), f.URL())

	f.setDetached(true)
	if f.parentFrame != nil {
		f.parentFrame.removeChildFrame(f)
	}
	f.parentFrame = nil

	// detach() is called by the same frame Goroutine that manages execution
	// context switches. so this should be safe.
	// we don't need to protect the following with executionContextMu.
	if f.documentHandle == nil {
		return nil
	}
	if err := f.documentHandle.Dispose(); err != nil {
		return fmt.Errorf("disposing document handle while detaching frame: %w", err)
	}

	return nil
}

func (f *Frame) defaultTimeout() time.Duration {
	return f.manager.timeoutSettings.timeout()
}

func (f *Frame) document() (*ElementHandle, error) {
	f.log.Debugf("Frame:document", "fid:%s furl:%q", f.ID(), f.URL())

	if cdh, ok := f.cachedDocumentHandle(); ok {
		return cdh, nil
	}

	f.waitForExecutionContext(mainWorld)

	dh, err := f.newDocumentHandle()
	if err != nil {
		return nil, fmt.Errorf("getting new document handle: %w", err)
	}

	// each execution context switch modifies documentHandle.
	// see: nullContext().
	f.executionContextMu.Lock()
	defer f.executionContextMu.Unlock()
	f.documentHandle = dh

	return dh, nil
}

func (f *Frame) cachedDocumentHandle() (*ElementHandle, bool) {
	// each execution context switch modifies documentHandle.
	// see: nullContext().
	f.executionContextMu.RLock()
	defer f.executionContextMu.RUnlock()

	return f.documentHandle, f.documentHandle != nil
}

func (f *Frame) newDocumentHandle() (*ElementHandle, error) {
	result, err := f.evaluate(
		f.ctx,
		mainWorld,
		evalOptions{
			forceCallable: false,
			returnByValue: false,
		},
		"document",
	)
	if err != nil {
		return nil, fmt.Errorf("getting document element handle: %w", err)
	}
	if result == nil {
		return nil, fmt.Errorf("document element handle is nil")
	}
	dh, ok := result.(*ElementHandle)
	if !ok {
		return nil, fmt.Errorf("unexpected document handle type: %T", result)
	}

	return dh, nil
}

func (f *Frame) hasContext(world executionWorld) bool {
	f.executionContextMu.RLock()
	defer f.executionContextMu.RUnlock()

	return f.executionContexts[world] != nil
}

func (f *Frame) hasLifecycleEventFired(event LifecycleEvent) bool {
	f.lifecycleEventsMu.RLock()
	defer f.lifecycleEventsMu.RUnlock()

	return f.lifecycleEvents[event]
}

func (f *Frame) navigated(name string, url string, loaderID string) {
	f.log.Debugf("Frame:navigated", "fid:%s furl:%q lid:%s name:%q url:%q", f.ID(), f.URL(), loaderID, name, url)

	f.propertiesMu.Lock()
	defer f.propertiesMu.Unlock()
	f.name = name
	f.url = url
	f.loaderID = loaderID
}

func (f *Frame) nullContext(execCtxID runtime.ExecutionContextID) {
	f.log.Debugf("Frame:nullContext", "fid:%s furl:%q ectxid:%d ", f.ID(), f.URL(), execCtxID)

	f.executionContextMu.Lock()
	defer f.executionContextMu.Unlock()

	if ec := f.executionContexts[mainWorld]; ec != nil && ec.ID() == execCtxID {
		f.executionContexts[mainWorld] = nil
		f.documentHandle = nil
		return
	}
	if ec := f.executionContexts[utilityWorld]; ec != nil && ec.ID() == execCtxID {
		f.executionContexts[utilityWorld] = nil
	}
}

func (f *Frame) onLifecycleEvent(event LifecycleEvent) {
	f.log.Debugf("Frame:onLifecycleEvent", "fid:%s furl:%q event:%s", f.ID(), f.URL(), event)

	f.lifecycleEventsMu.Lock()
	f.lifecycleEvents[event] = true
	f.lifecycleEventsMu.Unlock()

	f.emit(EventFrameAddLifecycle, FrameLifecycleEvent{
		URL:   f.URL(),
		Event: event,
	})
}

func (f *Frame) onLoadingStarted() {
	f.log.Debugf("Frame:onLoadingStarted", "fid:%s furl:%q", f.ID(), f.URL())

	f.loadingStartedTime = time.Now()
}

func (f *Frame) onLoadingStopped() {
	f.log.Debugf("Frame:onLoadingStopped", "fid:%s furl:%q", f.ID(), f.URL())

	// TODO: We should start a timer here and allow the user
	//       to set how long to wait until after onLoadingStopped
	//       has occurred. The reason we may want a timeout here
	//       are for websites where they perform many network
	//       requests so it may take a long time for us to see
	//       a networkIdle event or we may never see one if the
	//       website never stops performing network requests.
}

func (f *Frame) position() (*Position, error) {
	frame, ok := f.manager.getFrameByID(cdp.FrameID(f.page.targetID))
	if !ok {
		return nil, fmt.Errorf("could not find frame with id %s", f.page.targetID)
	}
	if frame == f.page.frameManager.MainFrame() {
		return &Position{X: 0, Y: 0}, nil
	}
	element, err := frame.FrameElement()
	if err != nil {
		return nil, err
	}

	box := element.BoundingBox()

	return &Position{X: box.X, Y: box.Y}, nil
}

func (f *Frame) removeChildFrame(child *Frame) {
	f.log.Debugf("Frame:removeChildFrame", "fid:%s furl:%q cfid:%s curl:%q",
		f.ID(), f.URL(), child.ID(), child.URL())

	f.childFramesMu.Lock()
	defer f.childFramesMu.Unlock()

	delete(f.childFrames, child)
}

func (f *Frame) setContext(world executionWorld, execCtx frameExecutionContext) {
	f.executionContextMu.Lock()
	defer f.executionContextMu.Unlock()

	f.log.Debugf("Frame:setContext", "fid:%s furl:%q ectxid:%d world:%s",
		f.ID(), f.URL(), execCtx.ID(), world)

	if !world.valid() {
		err := fmt.Errorf("unknown world: %q, it should be either main or utility", world)
		panic(err)
	}

	// There is a race condition when it comes to attaching iframes and the
	// execution context that apply to these frames. What usually occurs is:
	//
	// 1. An exec context for about:blank is first set;
	// 2. A new set event is received for exec context for the url pointing
	//    to the actual destination for the iframe;
	// 3. Finally the execution context for about:blank is destroyed, not
	//    for the second execution context.
	//
	// This is the order of events when iframes are in use on a site, and
	// so it is safe to nil the original execution context and overwrite it
	// with the second one.
	//
	// The exec context destroyed event will not remove the new exec context
	// since the ids do not match.
	//
	// If we didn't overwrite the first execCtx with the new one, then
	// waitForExecutionContext could end up waiting indefinitely since all
	// execCtx were destroyed.
	if f.executionContexts[world] != nil {
		f.log.Debugf("Frame:setContext", "fid:%s furl:%q ectxid:%d world:%s, overriding existing world",
			f.ID(), f.URL(), execCtx.ID(), world)

		f.executionContexts[world] = nil
		f.documentHandle = nil
	}

	f.executionContexts[world] = execCtx
	f.log.Debugf("Frame:setContext", "fid:%s furl:%q ectxid:%d world:%s, world set",
		f.ID(), f.URL(), execCtx.ID(), world)
}

func (f *Frame) setID(id cdp.FrameID) {
	f.propertiesMu.Lock()
	defer f.propertiesMu.Unlock()

	f.id = id
}

func (f *Frame) waitForExecutionContext(world executionWorld) {
	f.log.Debugf("Frame:waitForExecutionContext", "fid:%s furl:%q world:%s",
		f.ID(), f.URL(), world)

	t := time.NewTicker(50 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			if f.hasContext(world) {
				return
			}
		case <-f.ctx.Done():
			return
		}
	}
}

func (f *Frame) waitForSelectorRetry(
	selector string, opts *FrameWaitForSelectorOptions, retry int,
) (h *ElementHandle, err error) {
	for ; retry >= 0; retry-- {
		if h, err = f.waitForSelector(selector, opts); err == nil {
			return h, nil
		}
	}

	return nil, err
}

// waitForSelector will wait for the given selector to reach a defined state in
// opts.
//
// It will auto retry on certain errors until the retryCount is below 0. The
// retry workaround is needed since the underlying DOM can change when the
// wait action is performed during a navigation.
func (f *Frame) waitForSelector(selector string, opts *FrameWaitForSelectorOptions) (*ElementHandle, error) {
	f.log.Debugf("Frame:waitForSelector", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	handle, err := f.waitFor(selector, opts, 20)
	if err != nil {
		return nil, err
	}
	if handle == nil {
		return nil, fmt.Errorf("waiting for selector %q did not result in any nodes", selector)
	}

	// We always return ElementHandles in the main execution context (aka "DOM world")
	f.executionContextMu.RLock()
	defer f.executionContextMu.RUnlock()

	ec := f.executionContexts[mainWorld]
	if ec == nil {
		return nil, fmt.Errorf("waiting for selector %q: execution context %q not found", selector, mainWorld)
	}

	// an element should belong to the current execution context.
	// otherwise, we should adopt it to this execution context.
	adopted := handle
	if ec != handle.execCtx {
		if adopted, err = ec.adoptElementHandle(handle); err != nil {
			return nil, fmt.Errorf("waiting for selector %q: adopting element handle: %w", selector, err)
		}

		if err = handle.Dispose(); err != nil {
			f.log.Warnf(
				"Frame:waitForSelector",
				"fid:%s furl:%q sel:%q disposing element handle: %v",
				f.ID(), f.URL(), selector, err,
			)
		}
	}

	return adopted, nil
}

func (f *Frame) waitFor(
	selector string, opts *FrameWaitForSelectorOptions, retryCount int,
) (_ *ElementHandle, rerr error) {
	f.log.Debugf("Frame:waitFor", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	retryCount--
	if retryCount < 0 {
		return nil, errors.New("waitFor retry threshold reached")
	}

	document, err := f.document()
	if err != nil {
		if strings.Contains(err.Error(), "Cannot find context with specified id") {
			return f.waitFor(selector, opts, retryCount)
		}
		return nil, err
	}

	handle, err := document.waitForSelector(f.ctx, selector, opts)
	if err != nil {
		if strings.Contains(err.Error(), "Inspected target navigated or closed") {
			return f.waitFor(selector, opts, retryCount)
		}
		if strings.Contains(err.Error(), "Cannot find context with specified id") {
			return f.waitFor(selector, opts, retryCount)
		}
		if strings.Contains(err.Error(), "Execution context was destroyed") {
			return f.waitFor(selector, opts, retryCount)
		}
	}

	return handle, err
}

// ChildFrames returns a list of child frames.
func (f *Frame) ChildFrames() []*Frame {
	f.childFramesMu.RLock()
	defer f.childFramesMu.RUnlock()

	l := make([]*Frame, 0, len(f.childFrames))
	for child := range f.childFrames {
		l = append(l, child)
	}
	return l
}

// Click clicks the first element found that matches selector.
func (f *Frame) Click(selector string, opts *FrameClickOptions) error {
	f.log.Debugf("Frame:Click", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	if err := f.click(selector, opts); err != nil {
		return fmt.Errorf("clicking on %q: %w", selector, err)
	}

	applySlowMo(f.ctx)

	return nil
}

func (f *Frame) click(selector string, opts *FrameClickOptions) error {
	click := func(apiCtx context.Context, handle *ElementHandle, p *Position) (any, error) {
		return nil, handle.click(p, opts.ToMouseClickOptions())
	}
	act := f.newPointerAction(
		selector, DOMElementStateAttached, opts.Strict, click, &opts.ElementHandleBasePointerOptions,
	)
	if _, err := call(f.ctx, act, opts.Timeout); err != nil {
		return errorFromDOMError(err)
	}

	return nil
}

// Check clicks the first element found that matches selector.
func (f *Frame) Check(selector string, popts *FrameCheckOptions) error {
	f.log.Debugf("Frame:Check", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	if err := f.check(selector, popts); err != nil {
		return fmt.Errorf("checking %q: %w", selector, err)
	}

	applySlowMo(f.ctx)

	return nil
}

func (f *Frame) check(selector string, opts *FrameCheckOptions) error {
	check := func(apiCtx context.Context, handle *ElementHandle, p *Position) (any, error) {
		return nil, handle.setChecked(apiCtx, true, p)
	}
	act := f.newPointerAction(
		selector, DOMElementStateAttached, opts.Strict, check, &opts.ElementHandleBasePointerOptions,
	)
	if _, err := call(f.ctx, act, opts.Timeout); err != nil {
		return errorFromDOMError(err)
	}

	return nil
}

func (f *Frame) setChecked(selector string, checked bool, opts *FrameCheckOptions) error {
	setChecked := func(apiCtx context.Context, handle *ElementHandle, p *Position) (any, error) {
		return nil, handle.setChecked(apiCtx, checked, p)
	}
	act := f.newPointerAction(
		selector, DOMElementStateAttached, opts.Strict, setChecked, &opts.ElementHandleBasePointerOptions,
	)
	if _, err := call(f.ctx, act, opts.Timeout); err != nil {
		return errorFromDOMError(err)
	}

	return nil
}

// SetChecked sets the checked state of the first element found that matches the selector.
func (f *Frame) SetChecked(selector string, checked bool, popts *FrameCheckOptions) error {
	f.log.Debugf("Frame:SetChecked", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	if err := f.setChecked(selector, checked, popts); err != nil {
		return fmt.Errorf("setting checked %q: %w", selector, err)
	}

	applySlowMo(f.ctx)

	return nil
}

// Uncheck the first found element that matches the selector.
func (f *Frame) Uncheck(selector string, popts *FrameUncheckOptions) error {
	f.log.Debugf("Frame:Uncheck", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	if err := f.uncheck(selector, popts); err != nil {
		return fmt.Errorf("unchecking %q: %w", selector, err)
	}

	applySlowMo(f.ctx)

	return nil
}

func (f *Frame) uncheck(selector string, opts *FrameUncheckOptions) error {
	uncheck := func(apiCtx context.Context, handle *ElementHandle, p *Position) (any, error) {
		return nil, handle.setChecked(apiCtx, false, p)
	}
	act := f.newPointerAction(
		selector, DOMElementStateAttached, opts.Strict, uncheck, &opts.ElementHandleBasePointerOptions,
	)
	if _, err := call(f.ctx, act, opts.Timeout); err != nil {
		return errorFromDOMError(err)
	}

	return nil
}

// IsChecked returns true if the first element that matches the selector
// is checked. Otherwise, returns false.
func (f *Frame) IsChecked(selector string, opts *FrameIsCheckedOptions) (bool, error) {
	f.log.Debugf("Frame:IsChecked", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	checked, err := f.isChecked(selector, opts)
	if err != nil {
		return false, fmt.Errorf("checking element is checked %q: %w", selector, err)
	}

	return checked, nil
}

func (f *Frame) isChecked(selector string, opts *FrameIsCheckedOptions) (bool, error) {
	isChecked := func(apiCtx context.Context, handle *ElementHandle) (any, error) {
		v, err := handle.isChecked(apiCtx, 0) // Zero timeout when checking state
		if errors.Is(err, ErrTimedOut) {      // We don't care about timeout errors here!
			return v, nil
		}
		return v, err
	}
	act := f.newAction(
		selector, DOMElementStateAttached, opts.Strict, isChecked, []string{}, false, true, opts.Timeout,
	)
	v, err := call(f.ctx, act, opts.Timeout)
	if err != nil {
		return false, errorFromDOMError(err)
	}

	bv, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("checking is %q checked: unexpected type %T", selector, v)
	}

	return bv, nil
}

// Content returns the HTML content of the frame.
func (f *Frame) Content() (string, error) {
	f.log.Debugf("Frame:Content", "fid:%s furl:%q", f.ID(), f.URL())

	js := `() => {
		let content = '';
		if (document.doctype) {
			content = new XMLSerializer().serializeToString(document.doctype);
		}
		if (document.documentElement) {
			content += document.documentElement.outerHTML;
		}
		return content;
	}`

	v, err := f.Evaluate(js)
	if err != nil {
		return "", fmt.Errorf("getting frame content: %w", err)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("getting frame content: expected string, got %T", v)
	}

	return s, nil
}

// Dblclick double clicks an element matching provided selector.
func (f *Frame) Dblclick(selector string, popts *FrameDblclickOptions) error {
	f.log.Debugf("Frame:DblClick", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	if err := f.dblclick(selector, popts); err != nil {
		return fmt.Errorf("double clicking on %q: %w", selector, err)
	}

	applySlowMo(f.ctx)

	return nil
}

// dblclick is like Dblclick but takes parsed options and neither throws
// an error, or applies slow motion.
func (f *Frame) dblclick(selector string, opts *FrameDblclickOptions) error {
	dblclick := func(_ context.Context, eh *ElementHandle, p *Position) (any, error) {
		return nil, eh.dblclick(p, opts.ToMouseClickOptions())
	}
	act := f.newPointerAction(
		selector, DOMElementStateAttached, opts.Strict, dblclick, &opts.ElementHandleBasePointerOptions,
	)
	if _, err := call(f.ctx, act, opts.Timeout); err != nil {
		return errorFromDOMError(err)
	}

	return nil
}

// DispatchEvent dispatches an event for the first element matching the selector.
func (f *Frame) DispatchEvent(selector, typ string, eventInit any, opts *FrameDispatchEventOptions) error {
	f.log.Debugf("Frame:DispatchEvent", "fid:%s furl:%q sel:%q typ:%q", f.ID(), f.URL(), selector, typ)

	if err := f.dispatchEvent(selector, typ, eventInit, opts); err != nil {
		return fmt.Errorf("dispatching frame event %q to %q: %w", typ, selector, err)
	}
	applySlowMo(f.ctx)

	return nil
}

// dispatchEvent is like DispatchEvent but takes parsed options and neither throws
// an error, or applies slow motion.
func (f *Frame) dispatchEvent(selector, typ string, eventInit any, opts *FrameDispatchEventOptions) error {
	dispatchEvent := func(apiCtx context.Context, handle *ElementHandle) (any, error) {
		return handle.dispatchEvent(apiCtx, typ, eventInit)
	}
	const (
		force       = false
		noWaitAfter = false
	)
	act := f.newAction(
		selector, DOMElementStateAttached, opts.Strict, dispatchEvent, []string{},
		force, noWaitAfter, opts.Timeout,
	)
	if _, err := call(f.ctx, act, opts.Timeout); err != nil {
		return errorFromDOMError(err)
	}

	return nil
}

// EvaluateWithContext will evaluate provided page function within an execution context.
// The passed in context will be used instead of the frame's context. The context must
// be a derivative of one that contains the sobek runtime.
func (f *Frame) EvaluateWithContext(ctx context.Context, pageFunc string, args ...any) (any, error) {
	f.log.Debugf("Frame:EvaluateWithContext", "fid:%s furl:%q", f.ID(), f.URL())

	f.waitForExecutionContext(mainWorld)

	opts := evalOptions{
		forceCallable: true,
		returnByValue: true,
	}
	result, err := f.evaluate(ctx, mainWorld, opts, pageFunc, args...)
	if err != nil {
		return nil, fmt.Errorf("evaluating JS: %w", err)
	}

	applySlowMo(ctx)

	return result, nil
}

// Evaluate will evaluate provided page function within an execution context.
func (f *Frame) Evaluate(pageFunc string, args ...any) (any, error) {
	f.log.Debugf("Frame:Evaluate", "fid:%s furl:%q", f.ID(), f.URL())

	return f.EvaluateWithContext(f.ctx, pageFunc, args...)
}

// EvaluateGlobal will evaluate the given JS code in the global object.
func (f *Frame) EvaluateGlobal(ctx context.Context, js string) error {
	action := runtime.Evaluate(js).WithAwaitPromise(true)

	var (
		exceptionDetails *runtime.ExceptionDetails
		err              error
	)
	if _, exceptionDetails, err = action.Do(cdp.WithExecutor(ctx, f.manager.session)); err != nil {
		return fmt.Errorf("evaluating JS in global context: %w", err)
	}
	if exceptionDetails != nil {
		return fmt.Errorf("%s", parseExceptionDetails(exceptionDetails))
	}

	return nil
}

// EvaluateHandle will evaluate provided page function within an execution context.
func (f *Frame) EvaluateHandle(pageFunc string, args ...any) (handle JSHandleAPI, _ error) {
	f.log.Debugf("Frame:EvaluateHandle", "fid:%s furl:%q", f.ID(), f.URL())

	evalHandle := func() (JSHandleAPI, error) {
		f.executionContextMu.RLock()
		defer f.executionContextMu.RUnlock()

		ec := f.executionContexts[mainWorld]
		if ec == nil {
			return nil, fmt.Errorf("evaluating handle for frame: execution context %q not found", mainWorld)
		}
		return ec.EvalHandle(f.ctx, pageFunc, args...) //nolint:wrapcheck
	}

	f.waitForExecutionContext(mainWorld)
	handle, err := evalHandle()
	if err != nil {
		return nil, fmt.Errorf("evaluating handle for frame: %w", err)
	}

	applySlowMo(f.ctx)

	return handle, nil
}

// Fill fills out the first element found that matches the selector.
func (f *Frame) Fill(selector, value string, popts *FrameFillOptions) error {
	f.log.Debugf("Frame:Fill", "fid:%s furl:%q sel:%q val:%q", f.ID(), f.URL(), selector, value)

	if err := f.fill(selector, value, popts); err != nil {
		return fmt.Errorf("filling %q with %q: %w", selector, value, err)
	}

	applySlowMo(f.ctx)

	return nil
}

func (f *Frame) fill(selector, value string, opts *FrameFillOptions) error {
	fill := func(apiCtx context.Context, handle *ElementHandle) (any, error) {
		return nil, handle.fill(apiCtx, value)
	}
	act := f.newAction(
		selector, DOMElementStateAttached, opts.Strict,
		fill, []string{"visible", "enabled", "editable"},
		opts.Force, opts.NoWaitAfter, opts.Timeout,
	)
	if _, err := call(f.ctx, act, opts.Timeout); err != nil {
		return errorFromDOMError(err)
	}

	return nil
}

// Focus focuses on the first element that matches the selector.
func (f *Frame) Focus(selector string, popts *FrameBaseOptions) error {
	f.log.Debugf("Frame:Focus", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	if err := f.focus(selector, popts); err != nil {
		return fmt.Errorf("focusing %q: %w", selector, err)
	}

	applySlowMo(f.ctx)

	return nil
}

func (f *Frame) focus(selector string, opts *FrameBaseOptions) error {
	focus := func(apiCtx context.Context, handle *ElementHandle) (any, error) {
		return nil, handle.focus(apiCtx, true)
	}
	act := f.newAction(
		selector, DOMElementStateAttached, opts.Strict, focus,
		[]string{}, false, true, opts.Timeout,
	)
	if _, err := call(f.ctx, act, opts.Timeout); err != nil {
		return errorFromDOMError(err)
	}

	return nil
}

// FrameElement returns the element handle for the frame.
func (f *Frame) FrameElement() (*ElementHandle, error) {
	f.log.Debugf("Frame:FrameElement", "fid:%s furl:%q", f.ID(), f.URL())

	element, err := f.page.getFrameElement(f)
	if err != nil {
		return nil, fmt.Errorf("getting frame element: %w", err)
	}
	return element, nil
}

// GetAttribute of the first element found that matches the selector.
// The second return value is true if the attribute exists, and false otherwise.
func (f *Frame) GetAttribute(selector, name string, popts *FrameBaseOptions) (string, bool, error) {
	f.log.Debugf("Frame:GetAttribute", "fid:%s furl:%q sel:%q name:%s", f.ID(), f.URL(), selector, name)

	s, ok, err := f.getAttribute(selector, name, popts)
	if err != nil {
		return "", false, fmt.Errorf("getting attribute %q of %q: %w", name, selector, err)
	}

	return s, ok, nil
}

func (f *Frame) getAttribute(selector, name string, opts *FrameBaseOptions) (string, bool, error) {
	getAttribute := func(apiCtx context.Context, handle *ElementHandle) (any, error) {
		return handle.getAttribute(apiCtx, name)
	}
	act := f.newAction(
		selector, DOMElementStateAttached, opts.Strict, getAttribute,
		[]string{}, false, true, opts.Timeout,
	)
	v, err := call(f.ctx, act, opts.Timeout)
	if err != nil {
		return "", false, errorFromDOMError(err)
	}
	if v == nil {
		return "", false, nil
	}
	s, ok := v.(string)
	if !ok {
		return "", false, fmt.Errorf("unexpected type %T (expecting string)", v)
	}

	return s, true, nil
}

// Referrer returns the referrer of the frame from the network manager
// of the frame's session.
// It's an internal method not to be exposed as a JS API.
func (f *Frame) Referrer() string {
	nm := f.manager.page.mainFrameSession.getNetworkManager()
	return nm.extraHTTPHeaders["referer"]
}

// NavigationTimeout returns the navigation timeout of the frame.
// It's an internal method not to be exposed as a JS API.
func (f *Frame) NavigationTimeout() time.Duration {
	return f.manager.timeoutSettings.navigationTimeout()
}

// Goto will navigate the frame to the specified URL and return a HTTP response object.
func (f *Frame) Goto(url string, opts *FrameGotoOptions) (*Response, error) {
	resp, err := f.manager.NavigateFrame(f, url, opts)
	if err != nil {
		return nil, fmt.Errorf("navigating frame to %q: %w", url, err)
	}
	applySlowMo(f.ctx)

	// Since response will be in an interface, it will never be nil,
	// so we need to return nil explicitly.
	if resp == nil {
		return nil, nil //nolint:nilnil
	}

	return resp, nil
}

// Hover moves the pointer over the first element that matches the selector.
func (f *Frame) Hover(selector string, popts *FrameHoverOptions) error {
	f.log.Debugf("Frame:Hover", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	if err := f.hover(selector, popts); err != nil {
		return fmt.Errorf("hovering %q: %w", selector, err)
	}

	applySlowMo(f.ctx)

	return nil
}

func (f *Frame) hover(selector string, opts *FrameHoverOptions) error {
	hover := func(apiCtx context.Context, handle *ElementHandle, p *Position) (any, error) {
		return nil, handle.hover(apiCtx, p)
	}
	act := f.newPointerAction(
		selector, DOMElementStateAttached, opts.Strict, hover, &opts.ElementHandleBasePointerOptions,
	)
	if _, err := call(f.ctx, act, opts.Timeout); err != nil {
		return errorFromDOMError(err)
	}

	return nil
}

// InnerHTML returns the innerHTML attribute of the first element found
// that matches the selector.
func (f *Frame) InnerHTML(selector string, popts *FrameInnerHTMLOptions) (string, error) {
	f.log.Debugf("Frame:InnerHTML", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	v, err := f.innerHTML(selector, popts)
	if err != nil {
		return "", fmt.Errorf("getting inner HTML of %q: %w", selector, err)
	}

	return v, nil
}

func (f *Frame) innerHTML(selector string, opts *FrameInnerHTMLOptions) (string, error) {
	innerHTML := func(apiCtx context.Context, handle *ElementHandle) (any, error) {
		return handle.innerHTML(apiCtx)
	}
	act := f.newAction(
		selector, DOMElementStateAttached, opts.Strict, innerHTML,
		[]string{}, false, true, opts.Timeout,
	)
	v, err := call(f.ctx, act, opts.Timeout)
	if err != nil {
		return "", errorFromDOMError(err)
	}
	if v == nil {
		return "", nil
	}
	gv, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("unexpected type %T", v)
	}

	return gv, nil
}

// InnerText returns the inner text of the first element found
// that matches the selector.
func (f *Frame) InnerText(selector string, popts *FrameInnerTextOptions) (string, error) {
	f.log.Debugf("Frame:InnerText", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	v, err := f.innerText(selector, popts)
	if err != nil {
		return "", fmt.Errorf("getting inner text of %q: %w", selector, err)
	}

	return v, nil
}

func (f *Frame) innerText(selector string, opts *FrameInnerTextOptions) (string, error) {
	innerText := func(apiCtx context.Context, handle *ElementHandle) (any, error) {
		return handle.innerText(apiCtx)
	}
	act := f.newAction(
		selector, DOMElementStateAttached, opts.Strict, innerText,
		[]string{}, false, true, opts.Timeout,
	)
	v, err := call(f.ctx, act, opts.Timeout)
	if err != nil {
		return "", errorFromDOMError(err)
	}
	if v == nil {
		return "", nil
	}
	gv, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("unexpected type %T", v)
	}

	return gv, nil
}

// InputValue returns the input value of the first element found that matches the selector.
func (f *Frame) InputValue(selector string, popts *FrameInputValueOptions) (string, error) {
	f.log.Debugf("Frame:InputValue", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	v, err := f.inputValue(selector, popts)
	if err != nil {
		return "", fmt.Errorf("getting input value of %q: %w", selector, err)
	}

	return v, nil
}

func (f *Frame) inputValue(selector string, opts *FrameInputValueOptions) (string, error) {
	inputValue := func(apiCtx context.Context, handle *ElementHandle) (any, error) {
		return handle.inputValue(apiCtx)
	}
	act := f.newAction(
		selector, DOMElementStateAttached, opts.Strict, inputValue,
		[]string{}, false, true, opts.Timeout,
	)
	v, err := call(f.ctx, act, opts.Timeout)
	if err != nil {
		return "", errorFromDOMError(err)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("unexpected type %T", v)
	}

	return s, nil
}

// IsDetached returns whether the frame is detached or not.
func (f *Frame) IsDetached() bool {
	f.propertiesMu.RLock()
	defer f.propertiesMu.RUnlock()

	return f.detached
}

// setDetached sets the frame detachment.
func (f *Frame) setDetached(detached bool) {
	f.propertiesMu.Lock()
	defer f.propertiesMu.Unlock()

	f.detached = detached
}

// IsEditable returns true if the first element that matches the selector
// is editable. Otherwise, returns false.
func (f *Frame) IsEditable(selector string, opts *FrameIsEditableOptions) (bool, error) {
	f.log.Debugf("Frame:IsEditable", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	editable, err := f.isEditable(selector, opts)
	if err != nil {
		return false, fmt.Errorf("checking is %q editable: %w", selector, err)
	}

	return editable, nil
}

func (f *Frame) isEditable(selector string, opts *FrameIsEditableOptions) (bool, error) {
	isEditable := func(apiCtx context.Context, handle *ElementHandle) (any, error) {
		v, err := handle.isEditable(apiCtx, 0) // Zero timeout when checking state
		if errors.Is(err, ErrTimedOut) {       // We don't care about timeout errors here!
			return v, nil
		}
		return v, err
	}
	act := f.newAction(
		selector, DOMElementStateAttached, opts.Strict, isEditable, []string{}, false, true, opts.Timeout,
	)
	v, err := call(f.ctx, act, opts.Timeout)
	if err != nil {
		return false, errorFromDOMError(err)
	}

	bv, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("checking is %q editable: unexpected type %T", selector, v)
	}

	return bv, nil
}

// IsEnabled returns true if the first element that matches the selector
// is enabled. Otherwise, returns false.
func (f *Frame) IsEnabled(selector string, opts *FrameIsEnabledOptions) (bool, error) {
	f.log.Debugf("Frame:IsEnabled", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	enabled, err := f.isEnabled(selector, opts)
	if err != nil {
		return false, fmt.Errorf("checking is %q enabled: %w", selector, err)
	}

	return enabled, nil
}

func (f *Frame) isEnabled(selector string, opts *FrameIsEnabledOptions) (bool, error) {
	isEnabled := func(apiCtx context.Context, handle *ElementHandle) (any, error) {
		v, err := handle.isEnabled(apiCtx, 0) // Zero timeout when checking state
		if errors.Is(err, ErrTimedOut) {      // We don't care about timeout errors here!
			return v, nil
		}
		return v, err
	}
	act := f.newAction(
		selector, DOMElementStateAttached, opts.Strict, isEnabled, []string{}, false, true, opts.Timeout,
	)
	v, err := call(f.ctx, act, opts.Timeout)
	if err != nil {
		return false, errorFromDOMError(err)
	}

	bv, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("checking is %q enabled: unexpected type %T", selector, v)
	}

	return bv, nil
}

// IsDisabled returns true if the first element that matches the selector
// is disabled. Otherwise, returns false.
func (f *Frame) IsDisabled(selector string, opts *FrameIsDisabledOptions) (bool, error) {
	f.log.Debugf("Frame:IsDisabled", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	disabled, err := f.isDisabled(selector, opts)
	if err != nil {
		return false, fmt.Errorf("checking is %q disabled: %w", selector, err)
	}

	return disabled, nil
}

func (f *Frame) isDisabled(selector string, opts *FrameIsDisabledOptions) (bool, error) {
	isDisabled := func(apiCtx context.Context, handle *ElementHandle) (any, error) {
		v, err := handle.isDisabled(apiCtx, 0) // Zero timeout when checking state
		if errors.Is(err, ErrTimedOut) {       // We don't care about timeout errors here!
			return v, nil
		}
		return v, err
	}
	act := f.newAction(
		selector, DOMElementStateAttached, opts.Strict, isDisabled, []string{}, false, true, opts.Timeout,
	)
	v, err := call(f.ctx, act, opts.Timeout)
	if err != nil {
		return false, errorFromDOMError(err)
	}

	bv, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("checking is %q disabled: unexpected type %T", selector, v)
	}

	return bv, nil
}

// IsHidden returns true if the first element that matches the selector
// is hidden. Otherwise, returns false.
func (f *Frame) IsHidden(selector string, opts *FrameIsHiddenOptions) (bool, error) {
	f.log.Debugf("Frame:IsHidden", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	hidden, err := f.isHidden(selector, opts)
	if err != nil {
		return false, err
	}

	return hidden, nil
}

func (f *Frame) isHidden(selector string, opts *FrameIsHiddenOptions) (bool, error) {
	isHidden := func(apiCtx context.Context, handle *ElementHandle) (any, error) {
		v, err := handle.isHidden(apiCtx) // Zero timeout when checking state
		if errors.Is(err, ErrTimedOut) {  // We don't care about timeout errors here!
			return v, nil
		}
		return v, err
	}
	v, err := f.runActionOnSelector(f.ctx, selector, opts.Strict, isHidden, func() bool { return true })
	if err != nil {
		return false, fmt.Errorf("checking is %q hidden: %w", selector, err)
	}

	return v, nil
}

// IsVisible returns true if the first element that matches the selector
// is visible. Otherwise, returns false.
func (f *Frame) IsVisible(selector string, opts *FrameIsVisibleOptions) (bool, error) {
	f.log.Debugf("Frame:IsVisible", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	visible, err := f.isVisible(selector, opts)
	if err != nil {
		return false, err
	}

	return visible, nil
}

func (f *Frame) isVisible(selector string, opts *FrameIsVisibleOptions) (bool, error) {
	isVisible := func(apiCtx context.Context, handle *ElementHandle) (any, error) {
		v, err := handle.isVisible(apiCtx) // Zero timeout when checking state
		if errors.Is(err, ErrTimedOut) {   // We don't care about timeout errors here!
			return v, nil
		}
		return v, err
	}
	v, err := f.runActionOnSelector(f.ctx, selector, opts.Strict, isVisible, func() bool { return false })
	if err != nil {
		return false, fmt.Errorf("checking is %q visible: %w", selector, err)
	}

	return v, nil
}

// ID returns the frame id.
func (f *Frame) ID() string {
	f.propertiesMu.RLock()
	defer f.propertiesMu.RUnlock()

	return f.id.String()
}

// Locator creates and returns a new locator for this frame.
func (f *Frame) Locator(selector string, opts sobek.Value) *Locator {
	f.log.Debugf("Frame:Locator", "fid:%s furl:%q selector:%q opts:%+v", f.ID(), f.URL(), selector, opts)

	return NewLocator(f.ctx, selector, f, f.log)
}

// LoaderID returns the ID of the frame that loaded this frame.
func (f *Frame) LoaderID() string {
	f.propertiesMu.RLock()
	defer f.propertiesMu.RUnlock()

	return f.loaderID
}

// Name returns the frame name.
func (f *Frame) Name() string {
	f.propertiesMu.RLock()
	defer f.propertiesMu.RUnlock()

	return f.name
}

// Query runs a selector query against the document tree, returning the first matching element or
// "null" if no match is found.
func (f *Frame) Query(selector string, strict bool) (*ElementHandle, error) {
	f.log.Debugf("Frame:Query", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	document, err := f.document()
	if err != nil {
		return nil, fmt.Errorf("getting document: %w", err)
	}
	return document.Query(selector, strict)
}

// QueryAll runs a selector query against the document tree, returning all matching elements.
func (f *Frame) QueryAll(selector string) ([]*ElementHandle, error) {
	f.log.Debugf("Frame:QueryAll", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	document, err := f.document()
	if err != nil {
		return nil, fmt.Errorf("getting document: %w", err)
	}
	return document.QueryAll(selector)
}

// Page returns page that owns frame.
func (f *Frame) Page() *Page {
	return f.manager.page
}

// ParentFrame returns the parent frame, if one exists.
func (f *Frame) ParentFrame() *Frame {
	return f.parentFrame
}

// Press presses the given key for the first element found that matches the selector.
func (f *Frame) Press(selector, key string, opts *FramePressOptions) error {
	f.log.Debugf("Frame:Press", "fid:%s furl:%q sel:%q key:%q", f.ID(), f.URL(), selector, key)

	if err := f.press(selector, key, opts); err != nil {
		return fmt.Errorf("pressing %q on %q: %w", key, selector, err)
	}

	applySlowMo(f.ctx)

	return nil
}

func (f *Frame) press(selector, key string, opts *FramePressOptions) error {
	press := func(apiCtx context.Context, handle *ElementHandle) (any, error) {
		return nil, handle.press(apiCtx, key, opts.ToKeyboardOptions())
	}
	act := f.newAction(
		selector, DOMElementStateAttached, opts.Strict, press,
		[]string{}, false, opts.NoWaitAfter, opts.Timeout,
	)
	if _, err := call(f.ctx, act, opts.Timeout); err != nil {
		return errorFromDOMError(err)
	}

	return nil
}

// SelectOption selects the given options and returns the array of
// option values of the first element found that matches the selector.
func (f *Frame) SelectOption(selector string, values []any, popts *FrameSelectOptionOptions) ([]string, error) {
	f.log.Debugf("Frame:SelectOption", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	v, err := f.selectOption(selector, values, popts)
	if err != nil {
		return nil, fmt.Errorf("selecting option on %q: %w", selector, err)
	}

	applySlowMo(f.ctx)

	return v, nil
}

func (f *Frame) selectOption(selector string, values []any, opts *FrameSelectOptionOptions) ([]string, error) {
	selectOption := func(apiCtx context.Context, handle *ElementHandle) (any, error) {
		return handle.selectOption(apiCtx, values)
	}
	act := f.newAction(
		selector, DOMElementStateAttached, opts.Strict, selectOption,
		[]string{}, opts.Force, opts.NoWaitAfter, opts.Timeout,
	)
	v, err := call(f.ctx, act, opts.Timeout)
	if err != nil {
		return nil, errorFromDOMError(err)
	}
	selectHandle, ok := v.(jsHandle)
	if !ok {
		return nil, fmt.Errorf("unexpected select element type %T", v)
	}

	// pack the selected <option> values inside <select> into a slice
	optHandles, err := selectHandle.getProperties()
	if err != nil {
		return nil, fmt.Errorf("getProperties: %w", err)
	}
	vals := make([]string, 0, len(optHandles))
	for _, oh := range optHandles {
		val, err := oh.JSONValue()
		if err != nil {
			return nil, fmt.Errorf("reading value: %w", err)
		}
		vals = append(vals, val)
		if err := oh.dispose(); err != nil {
			return nil, fmt.Errorf("optionHandle.dispose: %w", err)
		}
	}
	if err := selectHandle.dispose(); err != nil {
		return nil, fmt.Errorf("selectHandle.dispose: %w", err)
	}

	return vals, nil
}

// SetContent replaces the entire HTML document content.
func (f *Frame) SetContent(html string, _ *FrameSetContentOptions) error {
	f.log.Debugf("Frame:SetContent", "fid:%s furl:%q", f.ID(), f.URL())

	// TODO(@inancgumus): Respect the FrameSetContentOptions before executing the action.
	// A solution is similar to the logic in `WaitForLoadState`.

	js := `(html) => {
		window.stop();
		document.open();
		document.write(html);
		document.close();
	}`

	f.waitForExecutionContext(utilityWorld)

	eopts := evalOptions{
		forceCallable: true,
		returnByValue: true,
	}
	if _, err := f.evaluate(f.ctx, utilityWorld, eopts, js, html); err != nil {
		return fmt.Errorf("setting content: %w", err)
	}

	applySlowMo(f.ctx)

	return nil
}

// SetInputFiles sets input files for the selected element.
func (f *Frame) SetInputFiles(selector string, pfiles *Files, popts *FrameSetInputFilesOptions) error {
	f.log.Debugf("Frame:SetInputFiles", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	if err := f.setInputFiles(selector, pfiles, popts); err != nil {
		return fmt.Errorf("setting input files on %q: %w", selector, err)
	}

	applySlowMo(f.ctx)

	return nil
}

// Tap the first element that matches the selector.
func (f *Frame) Tap(selector string, opts *FrameTapOptions) error {
	f.log.Debugf("Frame:Tap", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	if err := f.tap(selector, opts); err != nil {
		return fmt.Errorf("tapping on %q: %w", selector, err)
	}

	applySlowMo(f.ctx)

	return nil
}

func (f *Frame) tap(selector string, opts *FrameTapOptions) error {
	tap := func(apiCtx context.Context, handle *ElementHandle, p *Position) (any, error) {
		return nil, handle.tap(apiCtx, p)
	}
	act := f.newPointerAction(
		selector, DOMElementStateAttached, opts.Strict, tap, &opts.ElementHandleBasePointerOptions,
	)
	if _, err := call(f.ctx, act, opts.Timeout); err != nil {
		return errorFromDOMError(err)
	}

	return nil
}

func (f *Frame) setInputFiles(selector string, files *Files, opts *FrameSetInputFilesOptions) error {
	setInputFiles := func(apiCtx context.Context, handle *ElementHandle) (any, error) {
		return nil, handle.setInputFiles(apiCtx, files)
	}
	act := f.newAction(
		selector, DOMElementStateAttached, opts.Strict,
		setInputFiles, []string{},
		opts.Force, opts.NoWaitAfter, opts.Timeout,
	)

	if _, err := call(f.ctx, act, opts.Timeout); err != nil {
		return errorFromDOMError(err)
	}

	return nil
}

// TextContent returns the textContent attribute of the first element found
// that matches the selector. The second return value is true if the returned
// text content is not null or empty, and false otherwise.
func (f *Frame) TextContent(selector string, popts *FrameTextContentOptions) (string, bool, error) {
	f.log.Debugf("Frame:TextContent", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	v, ok, err := f.textContent(selector, popts)
	if err != nil {
		return "", false, fmt.Errorf("getting text content of %q: %w", selector, err)
	}

	return v, ok, nil
}

func (f *Frame) textContent(selector string, opts *FrameTextContentOptions) (string, bool, error) {
	TextContent := func(apiCtx context.Context, handle *ElementHandle) (any, error) {
		return handle.textContent(apiCtx)
	}
	act := f.newAction(
		selector, DOMElementStateAttached, opts.Strict, TextContent,
		[]string{}, false, true, opts.Timeout,
	)
	v, err := call(f.ctx, act, opts.Timeout)
	if err != nil {
		return "", false, errorFromDOMError(err)
	}
	if v == nil {
		return "", false, nil
	}
	s, ok := v.(string)
	if !ok {
		return "", false, fmt.Errorf("unexpected type %T (expecting string)", v)
	}

	return s, true, nil
}

// Timeout will return the default timeout or the one set by the user.
// It's an internal method not to be exposed as a JS API.
func (f *Frame) Timeout() time.Duration {
	return f.defaultTimeout()
}

// Title returns the title of the frame.
func (f *Frame) Title() (string, error) {
	f.log.Debugf("Frame:Title", "fid:%s furl:%q", f.ID(), f.URL())

	js := `() => document.title`
	v, err := f.Evaluate(js)
	if err != nil {
		return "", fmt.Errorf("getting frame title: %w", err)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("getting frame title: expected string, got %T", v)
	}

	return s, nil
}

// Type text on the first element found matches the selector.
func (f *Frame) Type(selector, text string, popts *FrameTypeOptions) error {
	f.log.Debugf("Frame:Type", "fid:%s furl:%q sel:%q text:%q", f.ID(), f.URL(), selector, text)

	if err := f.typ(selector, text, popts); err != nil {
		return fmt.Errorf("typing %q in %q: %w", text, selector, err)
	}

	applySlowMo(f.ctx)

	return nil
}

func (f *Frame) typ(selector, text string, opts *FrameTypeOptions) error {
	typeText := func(apiCtx context.Context, handle *ElementHandle) (any, error) {
		return nil, handle.typ(apiCtx, text, opts.ToKeyboardOptions())
	}
	act := f.newAction(
		selector, DOMElementStateAttached, opts.Strict, typeText,
		[]string{}, false, opts.NoWaitAfter, opts.Timeout,
	)
	if _, err := call(f.ctx, act, opts.Timeout); err != nil {
		return errorFromDOMError(err)
	}

	return nil
}

// URL returns the frame URL.
func (f *Frame) URL() string {
	if f == nil {
		return ""
	}

	f.propertiesMu.RLock()
	defer f.propertiesMu.RUnlock()

	return f.url
}

// URL set the frame URL.
func (f *Frame) setURL(url string) {
	defer f.propertiesMu.Unlock()
	f.propertiesMu.Lock()

	f.url = url
}

// WaitForFunction waits for the given predicate to return a truthy value.
func (f *Frame) WaitForFunction(js string, opts *FrameWaitForFunctionOptions, jsArgs ...any) (any, error) {
	f.log.Debugf("Frame:WaitForFunction", "fid:%s furl:%q", f.ID(), f.URL())

	var polling any = opts.Polling
	if opts.Polling == PollingInterval {
		polling = opts.Interval
	}

	result, err := f.waitForFunction(f.ctx, mainWorld, js,
		polling, opts.Timeout, jsArgs...)
	if err != nil {
		return nil, err
	}

	// prevent passing a non-nil interface to the upper layers.
	if result == nil {
		return nil, nil //nolint:nilnil
	}

	return result, err
}

func (f *Frame) waitForFunction(
	apiCtx context.Context, world executionWorld, js string,
	polling any, timeout time.Duration, args ...any,
) (any, error) {
	f.log.Debugf(
		"Frame:waitForFunction",
		"fid:%s furl:%q world:%s poll:%s timeout:%s",
		f.ID(), f.URL(), world, polling, timeout)

	f.waitForExecutionContext(world)

	f.executionContextMu.RLock()
	defer f.executionContextMu.RUnlock()

	execCtx := f.executionContexts[world]
	if execCtx == nil {
		return nil, fmt.Errorf("waiting for function: execution context %q not found", world)
	}
	injected, err := execCtx.getInjectedScript(apiCtx)
	if err != nil {
		return nil, fmt.Errorf("getting injected script: %w", err)
	}

	pageFn := `
		(injected, predicate, polling, timeout, ...args) => {
			return injected.waitForPredicateFunction(predicate, polling, timeout, ...args);
		}
	`

	// First evaluate the predicate function itself to get its handle.
	opts := evalOptions{forceCallable: false, returnByValue: false}
	handle, err := execCtx.eval(apiCtx, opts, js)
	if err != nil {
		return nil, fmt.Errorf("waiting for function, getting handle: %w", err)
	}

	// Then evaluate the injected function call, passing it the predicate
	// function handle and the rest of the arguments.
	opts = evalOptions{forceCallable: true, returnByValue: false}
	result, err := execCtx.eval(
		apiCtx, opts, pageFn, append([]any{
			injected,
			handle,
			polling,
			timeout.Milliseconds(), // The JS value is in ms integers
		}, args...)...)
	if err != nil {
		return nil, fmt.Errorf("waiting for function, polling: %w", err)
	}
	// prevent passing a non-nil interface to the upper layers.
	if result == nil {
		return nil, nil //nolint:nilnil
	}

	return result, nil
}

// WaitForLoadState waits for the given load state to be reached.
// This will unblock if that lifecycle event has already been received.
func (f *Frame) WaitForLoadState(state string, popts *FrameWaitForLoadStateOptions) error {
	f.log.Debugf("Frame:WaitForLoadState", "fid:%s furl:%q state:%s", f.ID(), f.URL(), state)
	defer f.log.Debugf("Frame:WaitForLoadState:return", "fid:%s furl:%q state:%s", f.ID(), f.URL(), state)

	timeoutCtx, timeoutCancel := context.WithTimeout(f.ctx, popts.Timeout)
	defer timeoutCancel()

	waitUntil := LifecycleEventLoad
	if state != "" {
		if err := waitUntil.UnmarshalText([]byte(state)); err != nil {
			return fmt.Errorf("unmarshaling wait for load state %q: %w", state, err)
		}
	}

	lifecycleEvent, lifecycleEventCancel := createWaitForEventPredicateHandler(
		timeoutCtx,
		f,
		[]string{EventFrameAddLifecycle},
		func(data any) bool {
			if le, ok := data.(FrameLifecycleEvent); ok {
				return le.Event == waitUntil
			}
			return false
		})
	defer lifecycleEventCancel()

	if f.hasLifecycleEventFired(waitUntil) {
		return nil
	}

	select {
	case <-lifecycleEvent:
	case <-timeoutCtx.Done():
		return fmt.Errorf("waiting for load state %q: %w", state, timeoutCtx.Err())
	}

	return nil
}

// WaitForNavigation waits for the given navigation lifecycle event to happen.
func (f *Frame) WaitForNavigation(opts *FrameWaitForNavigationOptions) (*Response, error) {
	f.log.Debugf("Frame:WaitForNavigation",
		"fid:%s furl:%s", f.ID(), f.URL())
	defer f.log.Debugf("Frame:WaitForNavigation:return",
		"fid:%s furl:%s", f.ID(), f.URL())

	timeoutCtx, timeoutCancel := context.WithTimeout(f.ctx, opts.Timeout)

	navEvtCh, navEvtCancel := createWaitForEventHandler(timeoutCtx, f, []string{EventFrameNavigation},
		func(data any) bool {
			return true // Both successful and failed navigations are considered
		})

	lifecycleEvtCh, lifecycleEvtCancel := createWaitForEventPredicateHandler(
		timeoutCtx, f, []string{EventFrameAddLifecycle},
		func(data any) bool {
			if le, ok := data.(FrameLifecycleEvent); ok {
				return le.Event == opts.WaitUntil
			}
			return false
		})

	handleTimeoutError := func(err error) error {
		f.log.Debugf("Frame:WaitForNavigation",
			"fid:%v furl:%s timeoutCtx done: %v", f.ID(), f.URL(), err)
		if err != nil {
			e := &k6ext.UserFriendlyError{
				Err:     err,
				Timeout: opts.Timeout,
			}
			return fmt.Errorf("waiting for navigation: %w", e)
		}

		return nil
	}

	defer func() {
		timeoutCancel()
		navEvtCancel()
		lifecycleEvtCancel()
	}()

	var (
		resp       *Response
		sameDocNav bool
	)
	select {
	case evt := <-navEvtCh:
		if e, ok := evt.(*NavigationEvent); ok {
			if e.newDocument == nil {
				sameDocNav = true
				break
			}
			// request could be nil if navigating to e.g. BlankPage.
			req := e.newDocument.request
			if req != nil {
				req.responseMu.RLock()
				resp = req.response
				req.responseMu.RUnlock()
			}
		}
	case <-timeoutCtx.Done():
		return nil, handleTimeoutError(timeoutCtx.Err())
	}

	// A lifecycle event won't be received when navigating within the same
	// document, so don't wait for it. The event might've also already been
	// fired once we're here, so also skip waiting in that case.
	if !sameDocNav && !f.hasLifecycleEventFired(opts.WaitUntil) {
		select {
		case <-lifecycleEvtCh:
		case <-timeoutCtx.Done():
			return nil, handleTimeoutError(timeoutCtx.Err())
		}
	}

	// Since response will be in an interface, it will never be nil,
	// so we need to return nil explicitly.
	if resp == nil {
		return nil, nil //nolint:nilnil
	}

	return resp, nil
}

// WaitForSelector waits for the given selector to match the waiting criteria.
func (f *Frame) WaitForSelector(selector string, popts *FrameWaitForSelectorOptions) (*ElementHandle, error) {
	handle, err := f.waitForSelectorRetry(selector, popts, maxRetry)
	if err != nil {
		return nil, fmt.Errorf("waiting for selector %q: %w", selector, err)
	}

	return handle, nil
}

// WaitForTimeout waits the specified amount of milliseconds.
func (f *Frame) WaitForTimeout(timeout int64) {
	to := time.Duration(timeout) * time.Millisecond

	f.log.Debugf("Frame:WaitForTimeout", "fid:%s furl:%q timeout:%s", f.ID(), f.URL(), to)
	defer f.log.Debugf("Frame:WaitForTimeout:return", "fid:%s furl:%q timeout:%s", f.ID(), f.URL(), to)

	select {
	case <-f.ctx.Done():
	case <-time.After(to):
	}
}

func (f *Frame) adoptBackendNodeID(world executionWorld, id cdp.BackendNodeID) (*ElementHandle, error) {
	f.log.Debugf("Frame:adoptBackendNodeID", "fid:%s furl:%q world:%s id:%d", f.ID(), f.URL(), world, id)

	f.executionContextMu.RLock()
	defer f.executionContextMu.RUnlock()

	ec := f.executionContexts[world]
	if ec == nil {
		return nil, fmt.Errorf("execution context %q not found", world)
	}
	return ec.adoptBackendNodeID(id)
}

func (f *Frame) evaluate(
	apiCtx context.Context,
	world executionWorld,
	opts evalOptions, pageFunc string, args ...any,
) (any, error) {
	f.log.Debugf("Frame:evaluate", "fid:%s furl:%q world:%s opts:%s", f.ID(), f.URL(), world, opts)

	f.executionContextMu.RLock()
	defer f.executionContextMu.RUnlock()

	ec := f.executionContexts[world]
	if ec == nil {
		return nil, fmt.Errorf("execution context %q not found", world)
	}

	eh, err := ec.eval(apiCtx, opts, pageFunc, args...)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return eh, nil
}

// frameExecutionContext represents a JS execution context that belongs to Frame.
type frameExecutionContext interface {
	// adoptBackendNodeID adopts specified backend node into this execution
	// context from another execution context.
	adoptBackendNodeID(backendNodeID cdp.BackendNodeID) (*ElementHandle, error)

	// adoptElementHandle adopts the specified element handle into this
	// execution context from another execution context.
	adoptElementHandle(elementHandle *ElementHandle) (*ElementHandle, error)

	// eval evaluates the provided JavaScript within this execution context and
	// returns a value or handle.
	eval(
		apiCtx context.Context, opts evalOptions, js string, args ...any,
	) (res any, err error)

	// getInjectedScript returns a JS handle to the injected script of helper
	// functions.
	getInjectedScript(apiCtx context.Context) (JSHandleAPI, error)

	// Eval evaluates the provided JavaScript within this execution context and
	// returns a value or handle.
	Eval(apiCtx context.Context, js string, args ...any) (any, error)

	// EvalHandle evaluates the provided JavaScript within this execution
	// context and returns a JSHandle.
	EvalHandle(apiCtx context.Context, js string, args ...any) (JSHandleAPI, error)

	// Frame returns the frame that this execution context belongs to.
	Frame() *Frame

	// id returns the CDP runtime ID of this execution context.
	ID() runtime.ExecutionContextID
}

func (f *Frame) runActionOnSelector(
	ctx context.Context, selector string, strict bool, fn elementHandleActionFunc, nullResponder func() bool,
) (bool, error) {
	handle, err := f.Query(selector, strict)
	if err != nil {
		return false, fmt.Errorf("query: %w", err)
	}
	if handle == nil {
		f.log.Debugf("Frame:runActionOnSelector:nilHandler", "fid:%s furl:%q selector:%s", f.ID(), f.URL(), selector)
		return nullResponder(), err
	}

	v, err := fn(ctx, handle)
	if err != nil {
		return false, fmt.Errorf("calling function: %w", err)
	}

	bv, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("unexpected type %T", v)
	}

	return bv, nil
}

//nolint:unparam
func (f *Frame) newAction(
	selector string, state DOMElementState, strict bool, fn elementHandleActionFunc, states []string,
	force, noWaitAfter bool, timeout time.Duration,
) func(apiCtx context.Context, resultCh chan any, errCh chan error) {
	// We execute a frame action in the following steps:
	// 1. Find element matching specified selector
	// 2. Wait for it to reach specified DOM state
	// 3. Run element handle action (incl. actionability checks)
	return func(apiCtx context.Context, resultCh chan any, errCh chan error) {
		waitOpts := NewFrameWaitForSelectorOptions(f.defaultTimeout())
		waitOpts.State = state
		waitOpts.Strict = strict
		handle, err := f.waitForSelector(selector, waitOpts)
		if err != nil {
			select {
			case <-apiCtx.Done():
			case errCh <- err:
			}
			return
		}
		if handle == nil {
			select {
			case <-apiCtx.Done():
			case resultCh <- nil:
			}
			return
		}
		f := handle.newAction(states, fn, force, noWaitAfter, timeout)
		f(apiCtx, resultCh, errCh)
	}
}

//nolint:unparam
func (f *Frame) newPointerAction(
	selector string, state DOMElementState, strict bool, fn elementHandlePointerActionFunc,
	opts *ElementHandleBasePointerOptions,
) func(apiCtx context.Context, resultCh chan any, errCh chan error) {
	// We execute a frame pointer action in the following steps:
	// 1. Find element matching specified selector
	// 2. Wait for it to reach specified DOM state
	// 3. Run element handle action (incl. actionability checks)
	return func(apiCtx context.Context, resultCh chan any, errCh chan error) {
		waitOpts := NewFrameWaitForSelectorOptions(f.defaultTimeout())
		waitOpts.State = state
		waitOpts.Strict = strict
		handle, err := f.waitForSelector(selector, waitOpts)
		if err != nil {
			select {
			case <-apiCtx.Done():
			case errCh <- err:
			}
			return
		}
		if handle == nil {
			select {
			case <-apiCtx.Done():
			case resultCh <- nil:
			}
			return
		}
		f := handle.newPointerAction(fn, opts)
		f(apiCtx, resultCh, errCh)
	}
}
