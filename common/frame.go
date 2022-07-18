package common

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/grafana/xk6-browser/api"
	"github.com/grafana/xk6-browser/k6ext"
	"github.com/grafana/xk6-browser/log"

	k6modules "go.k6.io/k6/js/modules"
	k6metrics "go.k6.io/k6/metrics"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/runtime"
	"github.com/dop251/goja"
)

// maxRetry controls how many times to retry if an action fails.
const maxRetry = 1

// Ensure frame implements the Frame interface.
var _ api.Frame = &Frame{}

type DocumentInfo struct {
	documentID string
	request    *Request
}

// Frame represents a frame in an HTML document.
type Frame struct {
	BaseEventEmitter

	ctx         context.Context
	page        *Page
	manager     *FrameManager
	parentFrame *Frame

	childFramesMu sync.RWMutex
	childFrames   map[api.Frame]bool

	propertiesMu sync.RWMutex
	id           cdp.FrameID
	loaderID     string
	name         string
	url          string
	detached     bool
	vu           k6modules.VU
	initTime     time.Time

	// A life cycle event is only considered triggered for a frame if the entire
	// frame subtree has also had the life cycle event triggered.
	lifecycleEventsMu      sync.RWMutex
	lifecycleEvents        map[LifecycleEvent]bool
	subtreeLifecycleEvents map[LifecycleEvent]bool

	documentHandle *ElementHandle

	executionContextMu sync.RWMutex
	executionContexts  map[executionWorld]frameExecutionContext

	loadingStartedTime time.Time

	networkIdleCh chan struct{}

	inflightRequestsMu sync.RWMutex
	inflightRequests   map[network.RequestID]bool

	currentDocument *DocumentInfo
	pendingDocument *DocumentInfo

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
		BaseEventEmitter:       NewBaseEventEmitter(ctx),
		ctx:                    ctx,
		page:                   m.page,
		manager:                m,
		parentFrame:            parentFrame,
		childFrames:            make(map[api.Frame]bool),
		id:                     frameID,
		vu:                     k6ext.GetVU(ctx),
		lifecycleEvents:        make(map[LifecycleEvent]bool),
		subtreeLifecycleEvents: make(map[LifecycleEvent]bool),
		inflightRequests:       make(map[network.RequestID]bool),
		executionContexts:      make(map[executionWorld]frameExecutionContext),
		currentDocument:        &DocumentInfo{},
		networkIdleCh:          make(chan struct{}),
		log:                    log,
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

func (f *Frame) inflightRequestsLen() int {
	f.inflightRequestsMu.RLock()
	defer f.inflightRequestsMu.RUnlock()

	return len(f.inflightRequests)
}

func (f *Frame) clearLifecycle() {
	f.log.Debugf("Frame:clearLifecycle", "fid:%s furl:%q", f.ID(), f.URL())

	// clear lifecycle events
	f.lifecycleEventsMu.Lock()
	{
		for e := range f.lifecycleEvents {
			f.lifecycleEvents[e] = false
		}
	}
	f.lifecycleEventsMu.Unlock()

	f.page.frameManager.MainFrame().recalculateLifecycle()

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

	f.stopNetworkIdleTimer()
	if f.inflightRequestsLen() == 0 {
		f.startNetworkIdleTimer()
	}
}

func (f *Frame) recalculateLifecycle() {
	f.log.Debugf("Frame:recalculateLifecycle", "fid:%s furl:%q", f.ID(), f.URL())

	// Start with triggered events.
	events := make(map[LifecycleEvent]bool)
	f.lifecycleEventsMu.RLock()
	{
		for k, v := range f.lifecycleEvents {
			events[k] = v
		}
	}
	f.lifecycleEventsMu.RUnlock()

	// Only consider a life cycle event as fired if it has triggered for all of subtree.
	f.childFramesMu.RLock()
	{
		for child := range f.childFrames {
			cf := child.(*Frame)
			// a precaution for preventing a deadlock in *Frame.childFramesMu
			if cf == f {
				continue
			}
			cf.recalculateLifecycle()
			for k := range events {
				if !cf.hasSubtreeLifecycleEventFired(k) {
					delete(events, k)
				}
			}
		}
	}
	f.childFramesMu.RUnlock()

	// Check if any of the fired events should be considered fired when looking at the entire subtree.
	mainFrame := f.manager.MainFrame()
	for k := range events {
		if f.hasSubtreeLifecycleEventFired(k) {
			continue
		}
		f.emit(EventFrameAddLifecycle, k)

		if f != mainFrame {
			continue
		}
		switch k {
		case LifecycleEventLoad:
			f.page.emit(EventPageLoad, nil)
		case LifecycleEventDOMContentLoad:
			f.page.emit(EventPageDOMContentLoaded, nil)
		}
	}

	// Emit removal events
	f.lifecycleEventsMu.RLock()
	{
		for k := range f.subtreeLifecycleEvents {
			if ok := events[k]; !ok {
				f.emit(EventFrameRemoveLifecycle, k)
			}
		}
	}
	f.lifecycleEventsMu.RUnlock()

	f.lifecycleEventsMu.Lock()
	{
		f.subtreeLifecycleEvents = make(map[LifecycleEvent]bool)
		for k, v := range events {
			f.subtreeLifecycleEvents[k] = v
		}
	}
	f.lifecycleEventsMu.Unlock()
}

func (f *Frame) stopNetworkIdleTimer() {
	f.log.Debugf("Frame:stopNetworkIdleTimer", "fid:%s furl:%q", f.ID(), f.URL())

	select {
	case f.networkIdleCh <- struct{}{}:
	default:
	}
}

func (f *Frame) startNetworkIdleTimer() {
	f.log.Debugf("Frame:startNetworkIdleTimer", "fid:%s furl:%q", f.ID(), f.URL())

	if f.hasLifecycleEventFired(LifecycleEventNetworkIdle) || f.IsDetached() {
		return
	}

	f.stopNetworkIdleTimer()

	go func() {
		select {
		case <-f.ctx.Done():
		case <-f.networkIdleCh:
		case <-time.After(LifeCycleNetworkIdleTimeout):
			f.manager.frameLifecycleEvent(cdp.FrameID(f.ID()), LifecycleEventNetworkIdle)
		}
	}()
}

func (f *Frame) detach() {
	f.log.Debugf("Frame:detach", "fid:%s furl:%q", f.ID(), f.URL())

	f.stopNetworkIdleTimer()
	f.setDetached(true)
	if f.parentFrame != nil {
		f.parentFrame.removeChildFrame(f)
	}
	f.parentFrame = nil
	// detach() is called by the same frame Goroutine that manages execution
	// context switches. so this should be safe.
	// we don't need to protect the following with executionContextMu.
	if f.documentHandle != nil {
		f.documentHandle.Dispose()
	}
}

func (f *Frame) defaultTimeout() time.Duration {
	return time.Duration(f.manager.timeoutSettings.timeout()) * time.Second
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

func (f *Frame) emitMetric(m *k6metrics.Metric, t time.Time) {
	value := k6metrics.D(t.Sub(f.initTime))
	f.log.Debugf("Frame:emitMetric", "fid:%s furl:%q m:%s init:%q t:%q v:%f",
		f.ID(), f.URL(), m.Name, f.initTime, t, value)

	if f.initTime.IsZero() {
		// Internal race condition: we haven't processed the init/commit event
		// yet, so the value will be wrong and emitting the metric would skew
		// the results (i.e. the value would be in the order of years). Choose
		// the lesser of 2 wrongs for now and ignore it instead.
		// See https://github.com/grafana/xk6-browser/discussions/142#discussioncomment-2416943
		return
	}

	state := f.vu.State()
	tags := state.CloneTags()
	if state.Options.SystemTags.Has(k6metrics.TagURL) {
		tags["url"] = f.URL()
	}
	sampleTags := k6metrics.IntoSampleTags(&tags)
	k6metrics.PushIfNotDone(f.ctx, state.Samples, k6metrics.ConnectedSamples{
		Samples: []k6metrics.Sample{
			{
				Metric: m,
				Tags:   sampleTags,
				Value:  value,
				Time:   time.Now(),
			},
		},
	})
}

func (f *Frame) newDocumentHandle() (*ElementHandle, error) {
	result, err := f.evaluate(
		f.ctx,
		mainWorld,
		evalOptions{
			forceCallable: false,
			returnByValue: false,
		},
		f.vu.Runtime().ToValue("document"),
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

func (f *Frame) hasSubtreeLifecycleEventFired(event LifecycleEvent) bool {
	f.lifecycleEventsMu.RLock()
	defer f.lifecycleEventsMu.RUnlock()

	return f.subtreeLifecycleEvents[event]
}

func (f *Frame) navigated(name string, url string, loaderID string) {
	f.log.Debugf("Frame:navigated", "fid:%s furl:%q lid:%s name:%q url:%q", f.ID(), f.URL(), loaderID, name, url)

	f.propertiesMu.Lock()
	defer f.propertiesMu.Unlock()
	f.name = name
	f.url = url
	f.loaderID = loaderID
	f.page.emit(EventPageFrameNavigated, f)
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
	defer f.lifecycleEventsMu.Unlock()

	if ok := f.lifecycleEvents[event]; ok {
		return
	}
	f.lifecycleEvents[event] = true
}

func (f *Frame) onLoadingStarted() {
	f.log.Debugf("Frame:onLoadingStarted", "fid:%s furl:%q", f.ID(), f.URL())

	f.loadingStartedTime = time.Now()
}

func (f *Frame) onLoadingStopped() {
	f.log.Debugf("Frame:onLoadingStopped", "fid:%s furl:%q", f.ID(), f.URL())

	f.lifecycleEventsMu.Lock()
	defer f.lifecycleEventsMu.Unlock()

	f.lifecycleEvents[LifecycleEventDOMContentLoad] = true
	f.lifecycleEvents[LifecycleEventLoad] = true
	f.lifecycleEvents[LifecycleEventNetworkIdle] = true
}

func (f *Frame) position() *Position {
	frame := f.manager.getFrameByID(cdp.FrameID(f.page.targetID))
	if frame == nil {
		return nil
	}
	if frame == f.page.frameManager.MainFrame() {
		return &Position{X: 0, Y: 0}
	}
	element := frame.FrameElement()
	box := element.BoundingBox()
	return &Position{X: box.X, Y: box.Y}
}

func (f *Frame) removeChildFrame(child *Frame) {
	f.log.Debugf("Frame:removeChildFrame", "fid:%s furl:%q cfid:%s curl:%q",
		f.ID(), f.URL(), child.ID(), child.URL())

	f.childFramesMu.Lock()
	defer f.childFramesMu.Unlock()

	delete(f.childFrames, child)
}

func (f *Frame) requestByID(reqID network.RequestID) *Request {
	frameSession := f.page.getFrameSession(cdp.FrameID(f.ID()))
	if frameSession == nil {
		frameSession = f.page.mainFrameSession
	}
	return frameSession.networkManager.requestFromID(reqID)
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

	if f.executionContexts[world] != nil {
		f.log.Debugf("Frame:setContext", "fid:%s furl:%q ectxid:%d world:%s, world exists",
			f.ID(), f.URL(), execCtx.ID(), world)
		return
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

	t := time.NewTimer(50 * time.Millisecond)
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

func (f *Frame) waitForFunction(
	apiCtx context.Context, world executionWorld, js string,
	polling interface{}, timeout time.Duration, args ...interface{},
) (*goja.Promise, error) {
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

	cb := f.vu.RegisterCallback()
	rt := f.vu.Runtime()
	promise, resolve, reject := rt.NewPromise()

	go func() {
		// First evaluate the predicate function itself to get its handle.
		opts := evalOptions{forceCallable: false, returnByValue: false}
		handle, err := execCtx.eval(apiCtx, opts, js)
		if err != nil {
			cb(func() error {
				reject(fmt.Errorf("waiting for function promise rejected: %w", err))
				return nil
			})
			return
		}

		// Then evaluate the injected function call, passing it the predicate
		// function handle and the rest of the arguments.
		opts = evalOptions{forceCallable: true, returnByValue: false}
		result, err := execCtx.eval(
			apiCtx, opts, pageFn, append([]interface{}{
				injected,
				handle,
				polling,
				timeout.Milliseconds(), // The JS value is in ms integers
			}, args...)...)
		if err != nil {
			cb(func() error {
				reject(fmt.Errorf("waiting for function promise rejected: %w", err))
				return nil
			})
			return
		}
		cb(func() error {
			resolve(result)
			return nil
		})
	}()

	return promise, nil
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

func (f *Frame) waitForSelector(selector string, opts *FrameWaitForSelectorOptions) (*ElementHandle, error) {
	f.log.Debugf("Frame:waitForSelector", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	document, err := f.document()
	if err != nil {
		return nil, err
	}

	handle, err := document.waitForSelector(f.ctx, selector, opts)
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
	if ec != handle.execCtx {
		defer handle.Dispose()
		if handle, err = ec.adoptElementHandle(handle); err != nil {
			return nil, fmt.Errorf("adopting element handle while waiting for selector %q: %w", selector, err)
		}
	}

	return handle, nil
}

func (f *Frame) AddScriptTag(opts goja.Value) {
	k6ext.Panic(f.ctx, "Frame.AddScriptTag() has not been implemented yet")
	applySlowMo(f.ctx)
}

func (f *Frame) AddStyleTag(opts goja.Value) {
	k6ext.Panic(f.ctx, "Frame.AddStyleTag() has not been implemented yet")
	applySlowMo(f.ctx)
}

// ChildFrames returns a list of child frames.
func (f *Frame) ChildFrames() []api.Frame {
	f.childFramesMu.RLock()
	defer f.childFramesMu.RUnlock()

	l := make([]api.Frame, 0, len(f.childFrames))
	for child := range f.childFrames {
		l = append(l, child)
	}
	return l
}

// Click clicks the first element found that matches selector.
func (f *Frame) Click(selector string, opts goja.Value) {
	f.log.Debugf("Frame:Click", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	popts := NewFrameClickOptions(f.defaultTimeout())
	if err := popts.Parse(f.ctx, opts); err != nil {
		k6ext.Panic(f.ctx, "parsing click options %q: %w", selector, err)
	}
	if err := f.click(selector, popts); err != nil {
		k6ext.Panic(f.ctx, "clicking on %q: %w", selector, err)
	}
	applySlowMo(f.ctx)
}

func (f *Frame) click(selector string, opts *FrameClickOptions) error {
	click := func(apiCtx context.Context, handle *ElementHandle, p *Position) (interface{}, error) {
		return nil, handle.click(p, opts.ToMouseClickOptions())
	}
	act := f.newPointerAction(
		selector, DOMElementStateAttached, opts.Strict, click, &opts.ElementHandleBasePointerOptions,
	)
	if _, err := callApiWithTimeout(f.ctx, act, opts.Timeout); err != nil {
		return errorFromDOMError(err.Error())
	}

	return nil
}

// Check clicks the first element found that matches selector.
func (f *Frame) Check(selector string, opts goja.Value) {
	f.log.Debugf("Frame:Check", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	popts := NewFrameCheckOptions(f.defaultTimeout())
	if err := popts.Parse(f.ctx, opts); err != nil {
		k6ext.Panic(f.ctx, "parsing new frame check options: %w", err)
	}
	if err := f.check(selector, popts); err != nil {
		k6ext.Panic(f.ctx, "checking %q: %w", selector, err)
	}
	applySlowMo(f.ctx)
}

func (f *Frame) check(selector string, opts *FrameCheckOptions) error {
	check := func(apiCtx context.Context, handle *ElementHandle, p *Position) (interface{}, error) {
		return nil, handle.setChecked(apiCtx, true, p)
	}
	act := f.newPointerAction(
		selector, DOMElementStateAttached, opts.Strict, check, &opts.ElementHandleBasePointerOptions,
	)
	if _, err := callApiWithTimeout(f.ctx, act, opts.Timeout); err != nil {
		return errorFromDOMError(err.Error())
	}

	return nil
}

// Uncheck the first found element that matches the selector.
func (f *Frame) Uncheck(selector string, opts goja.Value) {
	f.log.Debugf("Frame:Uncheck", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	popts := NewFrameUncheckOptions(f.defaultTimeout())
	if err := popts.Parse(f.ctx, opts); err != nil {
		k6ext.Panic(f.ctx, "parsing frame uncheck options %q: %w", selector, err)
	}
	if err := f.uncheck(selector, popts); err != nil {
		k6ext.Panic(f.ctx, "unchecking %q: %w", selector, err)
	}
	applySlowMo(f.ctx)
}

func (f *Frame) uncheck(selector string, opts *FrameUncheckOptions) error {
	uncheck := func(apiCtx context.Context, handle *ElementHandle, p *Position) (interface{}, error) {
		return nil, handle.setChecked(apiCtx, false, p)
	}
	act := f.newPointerAction(
		selector, DOMElementStateAttached, opts.Strict, uncheck, &opts.ElementHandleBasePointerOptions,
	)
	if _, err := callApiWithTimeout(f.ctx, act, opts.Timeout); err != nil {
		return errorFromDOMError(err.Error())
	}

	return nil
}

// IsChecked returns true if the first element that matches the selector
// is checked. Otherwise, returns false.
func (f *Frame) IsChecked(selector string, opts goja.Value) bool {
	f.log.Debugf("Frame:IsChecked", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	popts := NewFrameIsCheckedOptions(f.defaultTimeout())
	if err := popts.Parse(f.ctx, opts); err != nil {
		k6ext.Panic(f.ctx, "parsing is checked options: %w", err)
	}
	checked, err := f.isChecked(selector, popts)
	if err != nil {
		k6ext.Panic(f.ctx, "checking element is checked %q: %w", selector, err)
	}

	return checked
}

func (f *Frame) isChecked(selector string, opts *FrameIsCheckedOptions) (bool, error) {
	isChecked := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		v, err := handle.isChecked(apiCtx, 0) // Zero timeout when checking state
		if errors.Is(err, ErrTimedOut) {      // We don't care about timeout errors here!
			return v, nil
		}
		return v, err
	}
	act := f.newAction(
		selector, DOMElementStateAttached, opts.Strict, isChecked, []string{}, false, true, opts.Timeout,
	)
	v, err := callApiWithTimeout(f.ctx, act, opts.Timeout)
	if err != nil {
		return false, errorFromDOMError(err.Error())
	}

	bv, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("checking is %q checked: unexpected type %T", selector, v)
	}

	return bv, nil
}

// Content returns the HTML content of the frame.
func (f *Frame) Content() string {
	f.log.Debugf("Frame:Content", "fid:%s furl:%q", f.ID(), f.URL())

	rt := f.vu.Runtime()
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

	return gojaValueToString(f.ctx, f.Evaluate(rt.ToValue(js)))
}

// Dblclick double clicks an element matching provided selector.
func (f *Frame) Dblclick(selector string, opts goja.Value) {
	f.log.Debugf("Frame:DblClick", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	popts := NewFrameDblClickOptions(f.defaultTimeout())
	if err := popts.Parse(f.ctx, opts); err != nil {
		k6ext.Panic(f.ctx, "parsing double click options: %w", err)
	}
	if err := f.dblclick(selector, popts); err != nil {
		k6ext.Panic(f.ctx, "double clicking on %q: %w", selector, err)
	}
	applySlowMo(f.ctx)
}

// dblclick is like Dblclick but takes parsed options and neither throws
// an error, or applies slow motion.
func (f *Frame) dblclick(selector string, opts *FrameDblclickOptions) error {
	dblclick := func(apiCtx context.Context, eh *ElementHandle, p *Position) (interface{}, error) {
		return nil, eh.dblClick(p, opts.ToMouseClickOptions())
	}
	act := f.newPointerAction(
		selector, DOMElementStateAttached, opts.Strict, dblclick, &opts.ElementHandleBasePointerOptions,
	)
	if _, err := callApiWithTimeout(f.ctx, act, opts.Timeout); err != nil {
		return errorFromDOMError(err.Error())
	}

	return nil
}

// DispatchEvent dispatches an event for the first element matching the selector.
func (f *Frame) DispatchEvent(selector, typ string, eventInit, opts goja.Value) {
	f.log.Debugf("Frame:DispatchEvent", "fid:%s furl:%q sel:%q typ:%q", f.ID(), f.URL(), selector, typ)

	popts := NewFrameDispatchEventOptions(f.defaultTimeout())
	if err := popts.Parse(f.ctx, opts); err != nil {
		k6ext.Panic(f.ctx, "parsing dispatch event options: %w", err)
	}
	if err := f.dispatchEvent(selector, typ, eventInit, popts); err != nil {
		k6ext.Panic(f.ctx, "dispatching event %q to %q: %w", typ, selector, err)
	}
	applySlowMo(f.ctx)
}

// dispatchEvent is like DispatchEvent but takes parsed options and neither throws
// an error, or applies slow motion.
func (f *Frame) dispatchEvent(selector, typ string, eventInit goja.Value, opts *FrameDispatchEventOptions) error {
	dispatchEvent := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
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
	if _, err := callApiWithTimeout(f.ctx, act, opts.Timeout); err != nil {
		return errorFromDOMError(err.Error())
	}

	return nil
}

// Evaluate will evaluate provided page function within an execution context.
func (f *Frame) Evaluate(pageFunc goja.Value, args ...goja.Value) interface{} {
	f.log.Debugf("Frame:Evaluate", "fid:%s furl:%q", f.ID(), f.URL())

	f.waitForExecutionContext(mainWorld)

	opts := evalOptions{
		forceCallable: true,
		returnByValue: true,
	}
	result, err := f.evaluate(f.ctx, mainWorld, opts, pageFunc, args...)
	if err != nil {
		k6ext.Panic(f.ctx, "evaluating JS: %v", err)
	}

	applySlowMo(f.ctx)

	return result
}

// EvaluateHandle will evaluate provided page function within an execution context.
func (f *Frame) EvaluateHandle(pageFunc goja.Value, args ...goja.Value) (handle api.JSHandle) {
	f.log.Debugf("Frame:EvaluateHandle", "fid:%s furl:%q", f.ID(), f.URL())

	f.waitForExecutionContext(mainWorld)

	var err error
	f.executionContextMu.RLock()
	{
		ec := f.executionContexts[mainWorld]
		if ec == nil {
			k6ext.Panic(f.ctx, "execution context %q not found", mainWorld)
		}
		handle, err = ec.EvalHandle(f.ctx, pageFunc, args...)
	}
	f.executionContextMu.RUnlock()
	if err != nil {
		k6ext.Panic(f.ctx, "evaluating handle: %w", err)
	}

	applySlowMo(f.ctx)
	return handle
}

// Fill fills out the first element found that matches the selector.
func (f *Frame) Fill(selector, value string, opts goja.Value) {
	f.log.Debugf("Frame:Fill", "fid:%s furl:%q sel:%q val:%q", f.ID(), f.URL(), selector, value)

	popts := NewFrameFillOptions(f.defaultTimeout())
	if err := popts.Parse(f.ctx, opts); err != nil {
		k6ext.Panic(f.ctx, "parsing fill options: %w", err)
	}
	if err := f.fill(selector, value, popts); err != nil {
		k6ext.Panic(f.ctx, "filling %q with %q: %w", selector, value, err)
	}
	applySlowMo(f.ctx)
}

func (f *Frame) fill(selector, value string, opts *FrameFillOptions) error {
	fill := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return nil, handle.fill(apiCtx, value)
	}
	act := f.newAction(
		selector, DOMElementStateAttached, opts.Strict,
		fill, []string{"visible", "enabled", "editable"},
		opts.Force, opts.NoWaitAfter, opts.Timeout,
	)
	if _, err := callApiWithTimeout(f.ctx, act, opts.Timeout); err != nil {
		return errorFromDOMError(err.Error())
	}

	return nil
}

// Focus focuses on the first element that matches the selector.
func (f *Frame) Focus(selector string, opts goja.Value) {
	f.log.Debugf("Frame:Focus", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	popts := NewFrameBaseOptions(f.defaultTimeout())
	if err := popts.Parse(f.ctx, opts); err != nil {
		k6ext.Panic(f.ctx, "parsing focus options: %w", err)
	}
	if err := f.focus(selector, popts); err != nil {
		k6ext.Panic(f.ctx, "focusing %q: %w", selector, err)
	}
	applySlowMo(f.ctx)
}

func (f *Frame) focus(selector string, opts *FrameBaseOptions) error {
	focus := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return nil, handle.focus(apiCtx, true)
	}
	act := f.newAction(
		selector, DOMElementStateAttached, opts.Strict, focus,
		[]string{}, false, true, opts.Timeout,
	)
	if _, err := callApiWithTimeout(f.ctx, act, opts.Timeout); err != nil {
		return errorFromDOMError(err.Error())
	}

	return nil
}

func (f *Frame) FrameElement() api.ElementHandle {
	f.log.Debugf("Frame:FrameElement", "fid:%s furl:%q", f.ID(), f.URL())

	element, err := f.page.getFrameElement(f)
	if err != nil {
		k6ext.Panic(f.ctx, "getting frame element: %w", err)
	}
	return element
}

// GetAttribute of the first element found that matches the selector.
func (f *Frame) GetAttribute(selector, name string, opts goja.Value) goja.Value {
	f.log.Debugf("Frame:GetAttribute", "fid:%s furl:%q sel:%q name:%s", f.ID(), f.URL(), selector, name)

	popts := NewFrameBaseOptions(f.defaultTimeout())
	if err := popts.Parse(f.ctx, opts); err != nil {
		k6ext.Panic(f.ctx, "parse: %w", err)
	}
	v, err := f.getAttribute(selector, name, popts)
	if err != nil {
		k6ext.Panic(f.ctx, "getting attribute %q of %q: %w", name, selector, err)
	}

	applySlowMo(f.ctx)

	return v
}

func (f *Frame) getAttribute(selector, name string, opts *FrameBaseOptions) (goja.Value, error) {
	getAttribute := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return handle.getAttribute(apiCtx, name)
	}
	act := f.newAction(
		selector, DOMElementStateAttached, opts.Strict, getAttribute,
		[]string{}, false, true, opts.Timeout,
	)
	v, err := callApiWithTimeout(f.ctx, act, opts.Timeout)
	if err != nil {
		return nil, errorFromDOMError(err.Error())
	}
	gv, ok := v.(goja.Value)
	if !ok {
		return nil, fmt.Errorf("getting %q attribute of %q: unexpected type %T", name, selector, v)
	}

	return gv, nil
}

// Goto will navigate the frame to the specified URL and return a HTTP response object.
func (f *Frame) Goto(url string, opts goja.Value) api.Response {
	resp := f.manager.NavigateFrame(f, url, opts)
	applySlowMo(f.ctx)
	return resp
}

// Hover moves the pointer over the first element that matches the selector.
func (f *Frame) Hover(selector string, opts goja.Value) {
	f.log.Debugf("Frame:Hover", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	popts := NewFrameHoverOptions(f.defaultTimeout())
	if err := popts.Parse(f.ctx, opts); err != nil {
		k6ext.Panic(f.ctx, "parsing hover options: %w", err)
	}
	if err := f.hover(selector, popts); err != nil {
		k6ext.Panic(f.ctx, "hovering %q: %w", selector, err)
	}

	applySlowMo(f.ctx)
}

func (f *Frame) hover(selector string, opts *FrameHoverOptions) error {
	hover := func(apiCtx context.Context, handle *ElementHandle, p *Position) (interface{}, error) {
		return nil, handle.hover(apiCtx, p)
	}
	act := f.newPointerAction(
		selector, DOMElementStateAttached, opts.Strict, hover, &opts.ElementHandleBasePointerOptions,
	)
	if _, err := callApiWithTimeout(f.ctx, act, opts.Timeout); err != nil {
		return errorFromDOMError(err.Error())
	}

	return nil
}

// InnerHTML returns the innerHTML attribute of the first element found
// that matches the selector.
func (f *Frame) InnerHTML(selector string, opts goja.Value) string {
	f.log.Debugf("Frame:InnerHTML", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	popts := NewFrameInnerHTMLOptions(f.defaultTimeout())
	if err := popts.Parse(f.ctx, opts); err != nil {
		k6ext.Panic(f.ctx, "parsing inner HTML options: %w", err)
	}
	v, err := f.innerHTML(selector, popts)
	if err != nil {
		k6ext.Panic(f.ctx, "getting inner HTML of %q: %w", selector, err)
	}

	applySlowMo(f.ctx)

	return v
}

func (f *Frame) innerHTML(selector string, opts *FrameInnerHTMLOptions) (string, error) {
	innerHTML := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return handle.innerHTML(apiCtx)
	}
	act := f.newAction(
		selector, DOMElementStateAttached, opts.Strict, innerHTML,
		[]string{}, false, true, opts.Timeout,
	)
	v, err := callApiWithTimeout(f.ctx, act, opts.Timeout)
	if err != nil {
		return "", errorFromDOMError(err.Error())
	}
	if v == nil {
		return "", nil
	}
	gv, ok := v.(goja.Value)
	if !ok {
		return "", fmt.Errorf("getting inner html of %q: unexpected type %T", selector, v)
	}

	return gv.String(), nil
}

// InnerText returns the inner text of the first element found
// that matches the selector.
func (f *Frame) InnerText(selector string, opts goja.Value) string {
	f.log.Debugf("Frame:InnerText", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	popts := NewFrameInnerTextOptions(f.defaultTimeout())
	if err := popts.Parse(f.ctx, opts); err != nil {
		k6ext.Panic(f.ctx, "parsing inner text options: %w", err)
	}
	v, err := f.innerText(selector, popts)
	if err != nil {
		k6ext.Panic(f.ctx, "getting inner text of %q: %w", selector, err)
	}

	applySlowMo(f.ctx)

	return v
}

func (f *Frame) innerText(selector string, opts *FrameInnerTextOptions) (string, error) {
	innerText := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return handle.innerText(apiCtx)
	}
	act := f.newAction(
		selector, DOMElementStateAttached, opts.Strict, innerText,
		[]string{}, false, true, opts.Timeout,
	)
	v, err := callApiWithTimeout(f.ctx, act, opts.Timeout)
	if err != nil {
		return "", errorFromDOMError(err.Error())
	}
	if v == nil {
		return "", nil
	}
	gv, ok := v.(goja.Value)
	if !ok {
		return "", fmt.Errorf("getting inner text of %q: unexpected type %T", selector, v)
	}

	return gv.String(), nil
}

// InputValue returns the input value of the first element found
// that matches the selector.
func (f *Frame) InputValue(selector string, opts goja.Value) string {
	f.log.Debugf("Frame:InputValue", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	popts := NewFrameInputValueOptions(f.defaultTimeout())
	if err := popts.Parse(f.ctx, opts); err != nil {
		k6ext.Panic(f.ctx, "parsing input value options: %w", err)
	}
	v, err := f.inputValue(selector, popts)
	if err != nil {
		k6ext.Panic(f.ctx, "getting input value of %q: %w", selector, err)
	}

	return v
}

func (f *Frame) inputValue(selector string, opts *FrameInputValueOptions) (string, error) {
	inputValue := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return handle.inputValue(apiCtx)
	}
	act := f.newAction(
		selector, DOMElementStateAttached, opts.Strict, inputValue,
		[]string{}, false, true, opts.Timeout,
	)
	v, err := callApiWithTimeout(f.ctx, act, opts.Timeout)
	if err != nil {
		return "", errorFromDOMError(err.Error())
	}
	gv, ok := v.(goja.Value)
	if !ok {
		return "", fmt.Errorf("getting input value of %q: unexpected type %T", selector, v)
	}

	return gv.String(), nil
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
func (f *Frame) IsEditable(selector string, opts goja.Value) bool {
	f.log.Debugf("Frame:IsEditable", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	popts := NewFrameIsEditableOptions(f.defaultTimeout())
	if err := popts.Parse(f.ctx, opts); err != nil {
		k6ext.Panic(f.ctx, "%w", err)
	}
	editable, err := f.isEditable(selector, popts)
	if err != nil {
		k6ext.Panic(f.ctx, "checking is %q editable: %w", selector, err)
	}

	return editable
}

func (f *Frame) isEditable(selector string, opts *FrameIsEditableOptions) (bool, error) {
	isEditable := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		v, err := handle.isEditable(apiCtx, 0) // Zero timeout when checking state
		if errors.Is(err, ErrTimedOut) {       // We don't care about timeout errors here!
			return v, nil
		}
		return v, err
	}
	act := f.newAction(
		selector, DOMElementStateAttached, opts.Strict, isEditable, []string{}, false, true, opts.Timeout,
	)
	v, err := callApiWithTimeout(f.ctx, act, opts.Timeout)
	if err != nil {
		return false, errorFromDOMError(err.Error())
	}

	bv, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("checking is %q editable: unexpected type %T", selector, v)
	}

	return bv, nil
}

// IsEnabled returns true if the first element that matches the selector
// is enabled. Otherwise, returns false.
func (f *Frame) IsEnabled(selector string, opts goja.Value) bool {
	f.log.Debugf("Frame:IsEnabled", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	popts := NewFrameIsEnabledOptions(f.defaultTimeout())
	if err := popts.Parse(f.ctx, opts); err != nil {
		k6ext.Panic(f.ctx, "parsing is enabled options: %w", err)
	}
	enabled, err := f.isEnabled(selector, popts)
	if err != nil {
		k6ext.Panic(f.ctx, "checking is %q enabled: %w", selector, err)
	}

	return enabled
}

func (f *Frame) isEnabled(selector string, opts *FrameIsEnabledOptions) (bool, error) {
	isEnabled := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		v, err := handle.isEnabled(apiCtx, 0) // Zero timeout when checking state
		if errors.Is(err, ErrTimedOut) {      // We don't care about timeout errors here!
			return v, nil
		}
		return v, err
	}
	act := f.newAction(
		selector, DOMElementStateAttached, opts.Strict, isEnabled, []string{}, false, true, opts.Timeout,
	)
	v, err := callApiWithTimeout(f.ctx, act, opts.Timeout)
	if err != nil {
		return false, errorFromDOMError(err.Error())
	}

	bv, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("checking is %q enabled: unexpected type %T", selector, v)
	}

	return bv, nil
}

// IsDisabled returns true if the first element that matches the selector
// is disabled. Otherwise, returns false.
func (f *Frame) IsDisabled(selector string, opts goja.Value) bool {
	f.log.Debugf("Frame:IsDisabled", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	popts := NewFrameIsDisabledOptions(f.defaultTimeout())
	if err := popts.Parse(f.ctx, opts); err != nil {
		k6ext.Panic(f.ctx, "parsing is disabled options: %w", err)
	}
	disabled, err := f.isDisabled(selector, popts)
	if err != nil {
		k6ext.Panic(f.ctx, "checking is %q disabled: %w", selector, err)
	}

	return disabled
}

func (f *Frame) isDisabled(selector string, opts *FrameIsDisabledOptions) (bool, error) {
	isDisabled := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		v, err := handle.isDisabled(apiCtx, 0) // Zero timeout when checking state
		if errors.Is(err, ErrTimedOut) {       // We don't care about timeout errors here!
			return v, nil
		}
		return v, err
	}
	act := f.newAction(
		selector, DOMElementStateAttached, opts.Strict, isDisabled, []string{}, false, true, opts.Timeout,
	)
	v, err := callApiWithTimeout(f.ctx, act, opts.Timeout)
	if err != nil {
		return false, errorFromDOMError(err.Error())
	}

	bv, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("checking is %q disabled: unexpected type %T", selector, v)
	}

	return bv, nil
}

// IsHidden returns true if the first element that matches the selector
// is hidden. Otherwise, returns false.
func (f *Frame) IsHidden(selector string, opts goja.Value) bool {
	f.log.Debugf("Frame:IsHidden", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	popts := NewFrameIsHiddenOptions(f.defaultTimeout())
	if err := popts.Parse(f.ctx, opts); err != nil {
		k6ext.Panic(f.ctx, "parsing is hidden options: %w", err)
	}
	hidden, err := f.isHidden(selector, popts)
	if err != nil {
		k6ext.Panic(f.ctx, "checking is %q hidden: %w", selector, err)
	}

	return hidden
}

func (f *Frame) isHidden(selector string, opts *FrameIsHiddenOptions) (bool, error) {
	isHidden := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		v, err := handle.isHidden(apiCtx, 0) // Zero timeout when checking state
		if errors.Is(err, ErrTimedOut) {     // We don't care about timeout errors here!
			return v, nil
		}
		return v, err
	}
	act := f.newAction(
		selector, DOMElementStateAttached, opts.Strict, isHidden, []string{}, false, true, opts.Timeout,
	)
	v, err := callApiWithTimeout(f.ctx, act, opts.Timeout)
	if err != nil {
		return false, errorFromDOMError(err.Error())
	}

	bv, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("checking is %q hidden: unexpected type %T", selector, v)
	}

	return bv, nil
}

// IsVisible returns true if the first element that matches the selector
// is visible. Otherwise, returns false.
func (f *Frame) IsVisible(selector string, opts goja.Value) bool {
	f.log.Debugf("Frame:IsVisible", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	popts := NewFrameIsVisibleOptions(f.defaultTimeout())
	if err := popts.Parse(f.ctx, opts); err != nil {
		k6ext.Panic(f.ctx, "parsing is visible options: %w", err)
	}
	visible, err := f.isVisible(selector, popts)
	if err != nil {
		k6ext.Panic(f.ctx, "checking is %q visible: %w", selector, err)
	}

	return visible
}

func (f *Frame) isVisible(selector string, opts *FrameIsVisibleOptions) (bool, error) {
	isVisible := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		v, err := handle.isVisible(apiCtx, 0) // Zero timeout when checking state
		if errors.Is(err, ErrTimedOut) {      // We don't care about timeout errors here!
			return v, nil
		}
		return v, err
	}
	act := f.newAction(
		selector, DOMElementStateAttached, opts.Strict, isVisible, []string{}, false, true, opts.Timeout,
	)
	v, err := callApiWithTimeout(f.ctx, act, opts.Timeout)
	if err != nil {
		return false, errorFromDOMError(err.Error())
	}

	bv, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("checking is %q visible: unexpected type %T", selector, v)
	}

	return bv, nil
}

// ID returns the frame id.
func (f *Frame) ID() string {
	f.propertiesMu.RLock()
	defer f.propertiesMu.RUnlock()

	return f.id.String()
}

// Locator creates and returns a new locator for this frame.
func (f *Frame) Locator(selector string, opts goja.Value) api.Locator {
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
func (f *Frame) Query(selector string) api.ElementHandle {
	f.log.Debugf("Frame:Query", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	document, err := f.document()
	if err != nil {
		k6ext.Panic(f.ctx, "getting document: %w", err)
	}
	value := document.Query(selector)
	if value != nil {
		return value
	}
	return nil
}

func (f *Frame) QueryAll(selector string) []api.ElementHandle {
	f.log.Debugf("Frame:QueryAll", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	document, err := f.document()
	if err != nil {
		k6ext.Panic(f.ctx, "getting document: %w", err)
	}
	value := document.QueryAll(selector)
	if value != nil {
		return value
	}
	return nil
}

// Page returns page that owns frame.
func (f *Frame) Page() api.Page {
	return f.manager.page
}

// ParentFrame returns the parent frame, if one exists.
func (f *Frame) ParentFrame() api.Frame {
	return f.parentFrame
}

// Press presses the given key for the first element found that matches the selector.
func (f *Frame) Press(selector, key string, opts goja.Value) {
	f.log.Debugf("Frame:Press", "fid:%s furl:%q sel:%q key:%q", f.ID(), f.URL(), selector, key)

	popts := NewFramePressOptions(f.defaultTimeout())
	if err := popts.Parse(f.ctx, opts); err != nil {
		k6ext.Panic(f.ctx, "parsing press options: %w", err)
	}
	if err := f.press(selector, key, popts); err != nil {
		k6ext.Panic(f.ctx, "pressing %q on %q: %w", key, selector, err)
	}

	applySlowMo(f.ctx)
}

func (f *Frame) press(selector, key string, opts *FramePressOptions) error {
	press := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return nil, handle.press(apiCtx, key, opts.ToKeyboardOptions())
	}
	act := f.newAction(
		selector, DOMElementStateAttached, opts.Strict, press,
		[]string{}, false, opts.NoWaitAfter, opts.Timeout,
	)
	if _, err := callApiWithTimeout(f.ctx, act, opts.Timeout); err != nil {
		return errorFromDOMError(err.Error())
	}

	return nil
}

// SelectOption selects the given options and returns the array of
// option values of the first element found that matches the selector.
func (f *Frame) SelectOption(selector string, values goja.Value, opts goja.Value) []string {
	f.log.Debugf("Frame:SelectOption", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	popts := NewFrameSelectOptionOptions(f.defaultTimeout())
	if err := popts.Parse(f.ctx, opts); err != nil {
		k6ext.Panic(f.ctx, "parsing select option options: %w", err)
	}
	v, err := f.selectOption(selector, values, popts)
	if err != nil {
		k6ext.Panic(f.ctx, "selecting option on %q: %w", selector, err)
	}

	applySlowMo(f.ctx)

	return v
}

func (f *Frame) selectOption(selector string, values goja.Value, opts *FrameSelectOptionOptions) ([]string, error) {
	selectOption := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return handle.selectOption(apiCtx, values)
	}
	act := f.newAction(
		selector, DOMElementStateAttached, opts.Strict, selectOption,
		[]string{}, opts.Force, opts.NoWaitAfter, opts.Timeout,
	)
	v, err := callApiWithTimeout(f.ctx, act, opts.Timeout)
	if err != nil {
		return nil, errorFromDOMError(err.Error())
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
		vals = append(vals, oh.JSONValue().String())
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
func (f *Frame) SetContent(html string, opts goja.Value) {
	f.log.Debugf("Frame:SetContent", "fid:%s furl:%q", f.ID(), f.URL())

	parsedOpts := NewFrameSetContentOptions(f.defaultTimeout())
	if err := parsedOpts.Parse(f.ctx, opts); err != nil {
		k6ext.Panic(f.ctx, "parsing set content options: %w", err)
	}

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
	rt := f.vu.Runtime()
	if _, err := f.evaluate(f.ctx, utilityWorld, eopts, rt.ToValue(js), rt.ToValue(html)); err != nil {
		k6ext.Panic(f.ctx, "setting content: %w", err)
	}

	applySlowMo(f.ctx)
}

func (f *Frame) SetInputFiles(selector string, files goja.Value, opts goja.Value) {
	k6ext.Panic(f.ctx, "Frame.setInputFiles(selector, files, opts) has not been implemented yet")
	// TODO: needs slowMo
}

// Tap the first element that matches the selector.
func (f *Frame) Tap(selector string, opts goja.Value) {
	f.log.Debugf("Frame:Tap", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	popts := NewFrameTapOptions(f.defaultTimeout())
	if err := popts.Parse(f.ctx, opts); err != nil {
		k6ext.Panic(f.ctx, "parsing tap options: %w", err)
	}
	if err := f.tap(selector, popts); err != nil {
		k6ext.Panic(f.ctx, "tapping on %q: %w", selector, err)
	}

	applySlowMo(f.ctx)
}

func (f *Frame) tap(selector string, opts *FrameTapOptions) error {
	tap := func(apiCtx context.Context, handle *ElementHandle, p *Position) (interface{}, error) {
		return nil, handle.tap(apiCtx, p)
	}
	act := f.newPointerAction(
		selector, DOMElementStateAttached, opts.Strict, tap, &opts.ElementHandleBasePointerOptions,
	)
	if _, err := callApiWithTimeout(f.ctx, act, opts.Timeout); err != nil {
		return errorFromDOMError(err.Error())
	}

	return nil
}

// TextContent returns the textContent attribute of the first element found
// that matches the selector.
func (f *Frame) TextContent(selector string, opts goja.Value) string {
	f.log.Debugf("Frame:TextContent", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	popts := NewFrameTextContentOptions(f.defaultTimeout())
	if err := popts.Parse(f.ctx, opts); err != nil {
		k6ext.Panic(f.ctx, "parsing text content options: %w", err)
	}
	v, err := f.textContent(selector, popts)
	if err != nil {
		k6ext.Panic(f.ctx, "getting text content of %q: %w", selector, err)
	}

	applySlowMo(f.ctx)

	return v
}

func (f *Frame) textContent(selector string, opts *FrameTextContentOptions) (string, error) {
	TextContent := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return handle.textContent(apiCtx)
	}
	act := f.newAction(
		selector, DOMElementStateAttached, opts.Strict, TextContent,
		[]string{}, false, true, opts.Timeout,
	)
	v, err := callApiWithTimeout(f.ctx, act, opts.Timeout)
	if err != nil {
		return "", errorFromDOMError(err.Error())
	}
	if v == nil {
		return "", nil
	}
	gv, ok := v.(goja.Value)
	if !ok {
		return "", fmt.Errorf("getting text content of %q: unexpected type %T", selector, v)
	}

	return gv.String(), nil
}

func (f *Frame) Title() string {
	f.log.Debugf("Frame:Title", "fid:%s furl:%q", f.ID(), f.URL())

	rt := f.vu.Runtime()
	return f.Evaluate(rt.ToValue("document.title")).(string)
}

// Type text on the first element found matches the selector.
func (f *Frame) Type(selector, text string, opts goja.Value) {
	f.log.Debugf("Frame:Type", "fid:%s furl:%q sel:%q text:%q", f.ID(), f.URL(), selector, text)

	popts := NewFrameTypeOptions(f.defaultTimeout())
	if err := popts.Parse(f.ctx, opts); err != nil {
		k6ext.Panic(f.ctx, "parsing type options: %w", err)
	}
	if err := f.typ(selector, text, popts); err != nil {
		k6ext.Panic(f.ctx, "typing %q in %q: %w", text, selector, err)
	}

	applySlowMo(f.ctx)
}

func (f *Frame) typ(selector, text string, opts *FrameTypeOptions) error {
	typeText := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return nil, handle.typ(apiCtx, text, opts.ToKeyboardOptions())
	}
	act := f.newAction(
		selector, DOMElementStateAttached, opts.Strict, typeText,
		[]string{}, false, opts.NoWaitAfter, opts.Timeout,
	)
	if _, err := callApiWithTimeout(f.ctx, act, opts.Timeout); err != nil {
		return errorFromDOMError(err.Error())
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
func (f *Frame) WaitForFunction(fn goja.Value, opts goja.Value, jsArgs ...goja.Value) *goja.Promise {
	f.log.Debugf("Frame:WaitForFunction", "fid:%s furl:%q", f.ID(), f.URL())

	parsedOpts := NewFrameWaitForFunctionOptions(f.defaultTimeout())
	err := parsedOpts.Parse(f.ctx, opts)
	if err != nil {
		k6ext.Panic(f.ctx, "parsing waitForFunction options: %w", err)
	}

	f.executionContextMu.RLock()
	defer f.executionContextMu.RUnlock()

	js := fn.ToString().String()
	_, isCallable := goja.AssertFunction(fn)
	if !isCallable {
		js = fmt.Sprintf("() => (%s)", js)
	}

	args := make([]interface{}, 0, len(jsArgs))
	for _, a := range jsArgs {
		args = append(args, a.Export())
	}

	var polling interface{} = parsedOpts.Polling
	if parsedOpts.Polling == PollingInterval {
		polling = parsedOpts.Interval
	}

	promise, err := f.waitForFunction(f.ctx, mainWorld, js,
		polling, parsedOpts.Timeout, args...)
	if err != nil {
		k6ext.Panic(f.ctx, "%w", err)
	}

	return promise
}

// WaitForLoadState waits for the given load state to be reached.
func (f *Frame) WaitForLoadState(state string, opts goja.Value) {
	f.log.Debugf("Frame:WaitForLoadState", "fid:%s furl:%q state:%s", f.ID(), f.URL(), state)
	defer f.log.Debugf("Frame:WaitForLoadState:return", "fid:%s furl:%q state:%s", f.ID(), f.URL(), state)

	parsedOpts := NewFrameWaitForLoadStateOptions(f.defaultTimeout())
	err := parsedOpts.Parse(f.ctx, opts)
	if err != nil {
		k6ext.Panic(f.ctx, "parsing waitForLoadState %q options: %v", state, err)
	}

	waitUntil := LifecycleEventLoad
	if state != "" {
		if err = waitUntil.UnmarshalText([]byte(state)); err != nil {
			k6ext.Panic(f.ctx, "waiting for load state: %v", err)
		}
	}

	if f.hasLifecycleEventFired(waitUntil) {
		return
	}

	_, err = waitForEvent(f.ctx, f, []string{EventFrameAddLifecycle}, func(data interface{}) bool {
		return data.(LifecycleEvent) == waitUntil
	}, parsedOpts.Timeout)
	if err != nil {
		k6ext.Panic(f.ctx, "waiting for load state %q: %v", state, err)
	}
}

// WaitForNavigation waits for the given navigation lifecycle event to happen.
func (f *Frame) WaitForNavigation(opts goja.Value) api.Response {
	return f.manager.WaitForFrameNavigation(f, opts)
}

// WaitForSelector waits for the given selector to match the waiting criteria.
func (f *Frame) WaitForSelector(selector string, opts goja.Value) api.ElementHandle {
	parsedOpts := NewFrameWaitForSelectorOptions(f.defaultTimeout())
	if err := parsedOpts.Parse(f.ctx, opts); err != nil {
		k6ext.Panic(f.ctx, "parsing wait for selector %q options: %w", selector, err)
	}
	handle, err := f.waitForSelectorRetry(selector, parsedOpts, maxRetry)
	if err != nil {
		k6ext.Panic(f.ctx, "waiting for selector %q: %w", selector, err)
	}
	return handle
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
	opts evalOptions, pageFunc goja.Value, args ...goja.Value,
) (interface{}, error) {
	f.log.Debugf("Frame:evaluate", "fid:%s furl:%q world:%s opts:%s", f.ID(), f.URL(), world, opts)

	f.executionContextMu.RLock()
	defer f.executionContextMu.RUnlock()

	ec := f.executionContexts[world]
	if ec == nil {
		return nil, fmt.Errorf("execution context %q not found", world)
	}

	evalArgs := make([]interface{}, 0, len(args))
	for _, a := range args {
		evalArgs = append(evalArgs, a.Export())
	}
	eh, err := ec.eval(apiCtx, opts, pageFunc.ToString().String(), evalArgs...)
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
		apiCtx context.Context, opts evalOptions, js string, args ...interface{},
	) (res interface{}, err error)

	// getInjectedScript returns a JS handle to the injected script of helper
	// functions.
	getInjectedScript(apiCtx context.Context) (api.JSHandle, error)

	// Eval evaluates the provided JavaScript within this execution context and
	// returns a value or handle.
	Eval(apiCtx context.Context, js goja.Value, args ...goja.Value) (interface{}, error)

	// EvalHandle evaluates the provided JavaScript within this execution
	// context and returns a JSHandle.
	EvalHandle(apiCtx context.Context, js goja.Value, args ...goja.Value) (api.JSHandle, error)

	// Frame returns the frame that this execution context belongs to.
	Frame() *Frame

	// id returns the CDP runtime ID of this execution context.
	ID() runtime.ExecutionContextID
}

//nolint:unparam
func (f *Frame) newAction(
	selector string, state DOMElementState, strict bool, fn elementHandleActionFunc, states []string,
	force, noWaitAfter bool, timeout time.Duration,
) func(apiCtx context.Context, resultCh chan interface{}, errCh chan error) {
	// We execute a frame action in the following steps:
	// 1. Find element matching specified selector
	// 2. Wait for it to reach specified DOM state
	// 3. Run element handle action (incl. actionability checks)
	return func(apiCtx context.Context, resultCh chan interface{}, errCh chan error) {
		waitOpts := NewFrameWaitForSelectorOptions(f.defaultTimeout())
		waitOpts.State = state
		waitOpts.Strict = strict
		handle, err := f.waitForSelector(selector, waitOpts)
		if err != nil {
			errCh <- err
			return
		}
		if handle == nil {
			resultCh <- nil
			return
		}
		f := handle.newAction(states, fn, false, false, timeout)
		f(apiCtx, resultCh, errCh)
	}
}

//nolint:unparam
func (f *Frame) newPointerAction(
	selector string, state DOMElementState, strict bool, fn elementHandlePointerActionFunc,
	opts *ElementHandleBasePointerOptions,
) func(apiCtx context.Context, resultCh chan interface{}, errCh chan error) {
	// We execute a frame pointer action in the following steps:
	// 1. Find element matching specified selector
	// 2. Wait for it to reach specified DOM state
	// 3. Run element handle action (incl. actionability checks)
	return func(apiCtx context.Context, resultCh chan interface{}, errCh chan error) {
		waitOpts := NewFrameWaitForSelectorOptions(f.defaultTimeout())
		waitOpts.State = state
		waitOpts.Strict = strict
		handle, err := f.waitForSelector(selector, waitOpts)
		if err != nil {
			errCh <- err
			return
		}
		if handle == nil {
			resultCh <- nil
			return
		}
		f := handle.newPointerAction(fn, opts)
		f(apiCtx, resultCh, errCh)
	}
}
