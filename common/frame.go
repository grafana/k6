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
	"github.com/chromedp/cdproto/runtime"
	"github.com/dop251/goja"
	k6common "go.k6.io/k6/js/common"

	"github.com/grafana/xk6-browser/api"
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

	log *Logger
}

// NewFrame creates a new HTML document frame.
func NewFrame(ctx context.Context, m *FrameManager, parentFrame *Frame, frameID cdp.FrameID, log *Logger) *Frame {
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
		return nil, fmt.Errorf("newDocumentHandle: %w", err)
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
		k6common.GetRuntime(f.ctx).ToValue("document"),
	)
	if err != nil {
		return nil, fmt.Errorf("cannot evaluate in main execution context: %w", err)
	}
	if result == nil {
		return nil, errors.New("evaluate result is nil in main execution context")
	}
	dh, ok := result.(*ElementHandle)
	if !ok {
		return nil, fmt.Errorf("invalid document handle")
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
	apiCtx context.Context,
	world executionWorld, predicateFn goja.Value,
	polling PollingType, interval int64, timeout time.Duration,
	args ...goja.Value,
) (interface{}, error) {
	f.log.Debugf(
		"Frame:waitForFunction",
		"fid:%s furl:%q world:%s pt:%s timeout:%s",
		f.ID(), f.URL(), world, polling, timeout)

	f.waitForExecutionContext(world)

	f.executionContextMu.RLock()
	defer f.executionContextMu.RUnlock()

	execCtx := f.executionContexts[world]
	if execCtx == nil {
		return nil, fmt.Errorf("cannot find execution context: %q", world)
	}
	injected, err := execCtx.getInjectedScript(apiCtx)
	if err != nil {
		return nil, err
	}

	rt := k6common.GetRuntime(f.ctx)
	pageFn := rt.ToValue(`
		(injected, predicate, polling, timeout, ...args) => {
			return injected.waitForPredicateFunction(predicate, polling, timeout, ...args);
		}
	`)
	predicate := ""
	_, isCallable := goja.AssertFunction(predicateFn)
	if !isCallable {
		predicate = fmt.Sprintf("return (%s);", predicateFn.ToString().String())
	} else {
		predicate = fmt.Sprintf("return (%s)(...args);", predicateFn.ToString().String())
	}
	opts := evalOptions{
		forceCallable: true,
		returnByValue: false,
	}
	result, err := execCtx.eval(
		apiCtx, opts, pageFn, append([]goja.Value{
			rt.ToValue(injected),
			rt.ToValue(predicate),
			rt.ToValue(polling),
		}, args...)...)
	if err != nil {
		return nil, fmt.Errorf("frame cannot wait for function: %w", err)
	}
	return result, nil
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
		return nil, fmt.Errorf("wait for selector %q did not result in any nodes", selector)
	}

	// We always return ElementHandles in the main execution context (aka "DOM world")
	f.executionContextMu.RLock()
	defer f.executionContextMu.RUnlock()

	ec := f.executionContexts[mainWorld]
	if ec == nil {
		return nil, fmt.Errorf("wait for selector %q cannot find execution context: %q", selector, mainWorld)
	}
	// an element should belong to the current execution context.
	// otherwise, we should adopt it to this execution context.
	if ec != handle.execCtx {
		defer handle.Dispose()
		if handle, err = ec.adoptElementHandle(handle); err != nil {
			return nil, fmt.Errorf("wait for selector %q cannot adopt element handle: %w", selector, err)
		}
	}

	return handle, nil
}

func (f *Frame) AddScriptTag(opts goja.Value) {
	rt := k6common.GetRuntime(f.ctx)
	k6common.Throw(rt, errors.New("Frame.AddScriptTag() has not been implemented yet"))
	applySlowMo(f.ctx)
}

func (f *Frame) AddStyleTag(opts goja.Value) {
	rt := k6common.GetRuntime(f.ctx)
	k6common.Throw(rt, errors.New("Frame.AddStyleTag() has not been implemented yet"))
	applySlowMo(f.ctx)
}

// Check clicks the first element found that matches selector.
func (f *Frame) Check(selector string, opts goja.Value) {
	f.log.Debugf("Frame:Check", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	rt := k6common.GetRuntime(f.ctx)
	parsedOpts := NewFrameCheckOptions(f.defaultTimeout())
	err := parsedOpts.Parse(f.ctx, opts)
	if err != nil {
		k6common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle, p *Position) (interface{}, error) {
		return nil, handle.setChecked(apiCtx, true, p)
	}
	actFn := f.newPointerAction(
		selector, DOMElementStateAttached, parsedOpts.Strict, fn, &parsedOpts.ElementHandleBasePointerOptions,
	)
	_, err = callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		k6common.Throw(rt, err)
	}

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

	rt := k6common.GetRuntime(f.ctx)
	parsedOpts := NewFrameClickOptions(f.defaultTimeout())
	err := parsedOpts.Parse(f.ctx, opts)
	if err != nil {
		k6common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle, p *Position) (interface{}, error) {
		return nil, handle.click(p, parsedOpts.ToMouseClickOptions())
	}
	actFn := f.newPointerAction(
		selector, DOMElementStateAttached, parsedOpts.Strict, fn, &parsedOpts.ElementHandleBasePointerOptions,
	)
	_, err = callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		k6common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
}

// Content returns the HTML content of the frame.
func (f *Frame) Content() string {
	f.log.Debugf("Frame:Content", "fid:%s furl:%q", f.ID(), f.URL())

	rt := k6common.GetRuntime(f.ctx)
	js := `let content = '';
		if (document.doctype) {
			content = new XMLSerializer().serializeToString(document.doctype);
		}
		if (document.documentElement) {
			content += document.documentElement.outerHTML;
		}
		return content;`
	return f.Evaluate(rt.ToValue(js)).(string)
}

// Dblclick double clicks an element matching provided selector.
func (f *Frame) Dblclick(selector string, opts goja.Value) {
	f.log.Debugf("Frame:DblClick", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	rt := k6common.GetRuntime(f.ctx)
	parsedOpts := NewFrameDblClickOptions(f.defaultTimeout())
	err := parsedOpts.Parse(f.ctx, opts)
	if err != nil {
		k6common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle, p *Position) (interface{}, error) {
		return nil, handle.dblClick(p, parsedOpts.ToMouseClickOptions())
	}
	actFn := f.newPointerAction(
		selector, DOMElementStateAttached, parsedOpts.Strict, fn, &parsedOpts.ElementHandleBasePointerOptions,
	)
	_, err = callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		k6common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
}

func (f *Frame) DispatchEvent(selector string, typ string, eventInit goja.Value, opts goja.Value) {
	f.log.Debugf("Frame:DispatchEvent", "fid:%s furl:%q sel:%q typ:%s", f.ID(), f.URL(), selector, typ)

	rt := k6common.GetRuntime(f.ctx)
	parsedOpts := NewFrameDblClickOptions(f.defaultTimeout())
	if err := parsedOpts.Parse(f.ctx, opts); err != nil {
		k6common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return handle.dispatchEvent(apiCtx, typ, eventInit)
	}
	actFn := f.newAction(
		selector, DOMElementStateAttached, parsedOpts.Strict, fn, []string{},
		parsedOpts.Force, parsedOpts.NoWaitAfter, parsedOpts.Timeout,
	)
	_, err := callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		k6common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
}

// Evaluate will evaluate provided page function within an execution context.
func (f *Frame) Evaluate(pageFunc goja.Value, args ...goja.Value) interface{} {
	f.log.Debugf("Frame:Evaluate", "fid:%s furl:%q", f.ID(), f.URL())

	rt := k6common.GetRuntime(f.ctx)

	f.waitForExecutionContext(mainWorld)

	opts := evalOptions{
		forceCallable: true,
		returnByValue: true,
	}
	result, err := f.evaluate(f.ctx, mainWorld, opts, pageFunc, args...)
	if err != nil {
		k6common.Throw(rt, err)
	}

	applySlowMo(f.ctx)

	return result
}

// EvaluateHandle will evaluate provided page function within an execution context.
func (f *Frame) EvaluateHandle(pageFunc goja.Value, args ...goja.Value) (handle api.JSHandle) {
	f.log.Debugf("Frame:EvaluateHandle", "fid:%s furl:%q", f.ID(), f.URL())

	rt := k6common.GetRuntime(f.ctx)

	f.waitForExecutionContext(mainWorld)

	var err error
	f.executionContextMu.RLock()
	{
		ec := f.executionContexts[mainWorld]
		if ec == nil {
			k6common.Throw(rt, fmt.Errorf("cannot find execution context: %q", mainWorld))
		}
		handle, err = ec.EvalHandle(f.ctx, pageFunc, args...)
	}
	f.executionContextMu.RUnlock()
	if err != nil {
		k6common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
	return handle
}

func (f *Frame) Fill(selector string, value string, opts goja.Value) {
	f.log.Debugf("Frame:Fill", "fid:%s furl:%q sel:%q val:%s", f.ID(), f.URL(), selector, value)

	rt := k6common.GetRuntime(f.ctx)
	parsedOpts := NewFrameFillOptions(f.defaultTimeout())
	if err := parsedOpts.Parse(f.ctx, opts); err != nil {
		k6common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return handle.fill(apiCtx, value)
	}
	actFn := f.newAction(
		selector, DOMElementStateAttached, parsedOpts.Strict, fn, []string{"visible", "enabled", "editable"},
		parsedOpts.Force, parsedOpts.NoWaitAfter, parsedOpts.Timeout,
	)
	_, err := callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		k6common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
}

// Focus fetches an element with selector and focuses it.
func (f *Frame) Focus(selector string, opts goja.Value) {
	f.log.Debugf("Frame:Focus", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	rt := k6common.GetRuntime(f.ctx)
	parsedOpts := NewFrameBaseOptions(f.defaultTimeout())
	if err := parsedOpts.Parse(f.ctx, opts); err != nil {
		k6common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return nil, handle.focus(apiCtx, true)
	}
	actFn := f.newAction(
		selector, DOMElementStateAttached, parsedOpts.Strict, fn, []string{}, false, true, parsedOpts.Timeout,
	)
	_, err := callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		k6common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
}

func (f *Frame) FrameElement() api.ElementHandle {
	f.log.Debugf("Frame:FrameElement", "fid:%s furl:%q", f.ID(), f.URL())

	rt := k6common.GetRuntime(f.ctx)
	element, err := f.page.getFrameElement(f)
	if err != nil {
		k6common.Throw(rt, err)
	}
	return element
}

func (f *Frame) GetAttribute(selector string, name string, opts goja.Value) goja.Value {
	f.log.Debugf("Frame:GetAttribute", "fid:%s furl:%q sel:%q name:%s", f.ID(), f.URL(), selector, name)

	rt := k6common.GetRuntime(f.ctx)
	parsedOpts := NewFrameBaseOptions(f.defaultTimeout())
	if err := parsedOpts.Parse(f.ctx, opts); err != nil {
		k6common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return handle.getAttribute(apiCtx, name)
	}
	actFn := f.newAction(
		selector, DOMElementStateAttached, parsedOpts.Strict, fn, []string{}, false, true, parsedOpts.Timeout,
	)
	value, err := callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		k6common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
	return value.(goja.Value)
}

// Goto will navigate the frame to the specified URL and return a HTTP response object.
func (f *Frame) Goto(url string, opts goja.Value) api.Response {
	resp := f.manager.NavigateFrame(f, url, opts)
	applySlowMo(f.ctx)
	return resp
}

// Hover hovers an element identified by provided selector.
func (f *Frame) Hover(selector string, opts goja.Value) {
	f.log.Debugf("Frame:Hover", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	rt := k6common.GetRuntime(f.ctx)
	parsedOpts := NewFrameHoverOptions(f.defaultTimeout())
	err := parsedOpts.Parse(f.ctx, opts)
	if err != nil {
		k6common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle, p *Position) (interface{}, error) {
		return nil, handle.hover(apiCtx, p)
	}
	actFn := f.newPointerAction(
		selector, DOMElementStateAttached, parsedOpts.Strict, fn, &parsedOpts.ElementHandleBasePointerOptions,
	)
	_, err = callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		k6common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
}

func (f *Frame) InnerHTML(selector string, opts goja.Value) string {
	f.log.Debugf("Frame:InnerHTML", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	rt := k6common.GetRuntime(f.ctx)
	parsedOpts := NewFrameInnerHTMLOptions(f.defaultTimeout())
	if err := parsedOpts.Parse(f.ctx, opts); err != nil {
		k6common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return handle.innerHTML(apiCtx)
	}
	actFn := f.newAction(
		selector, DOMElementStateAttached, parsedOpts.Strict, fn, []string{}, false, true, parsedOpts.Timeout,
	)
	value, err := callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		k6common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
	return value.(string)
}

func (f *Frame) InnerText(selector string, opts goja.Value) string {
	f.log.Debugf("Frame:InnerText", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	rt := k6common.GetRuntime(f.ctx)
	parsedOpts := NewFrameInnerHTMLOptions(f.defaultTimeout())
	if err := parsedOpts.Parse(f.ctx, opts); err != nil {
		k6common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return handle.innerText(apiCtx)
	}
	actFn := f.newAction(
		selector, DOMElementStateAttached, parsedOpts.Strict, fn, []string{}, false, true, parsedOpts.Timeout,
	)
	value, err := callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		k6common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
	return value.(string)
}

func (f *Frame) InputValue(selector string, opts goja.Value) string {
	f.log.Debugf("Frame:InputValue", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	rt := k6common.GetRuntime(f.ctx)
	parsedOpts := NewFrameInputValueOptions(f.defaultTimeout())
	if err := parsedOpts.Parse(f.ctx, opts); err != nil {
		k6common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return handle.inputValue(apiCtx)
	}
	actFn := f.newAction(
		selector, DOMElementStateAttached, parsedOpts.Strict, fn, []string{}, false, true, parsedOpts.Timeout,
	)
	value, err := callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		k6common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
	return value.(goja.Value).String()
}

func (f *Frame) IsChecked(selector string, opts goja.Value) bool {
	f.log.Debugf("Frame:IsChecked", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	rt := k6common.GetRuntime(f.ctx)
	parsedOpts := NewFrameIsCheckedOptions(f.defaultTimeout())
	if err := parsedOpts.Parse(f.ctx, opts); err != nil {
		k6common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		value, err := handle.isChecked(apiCtx, 0) // Zero timeout when checking state
		if err == ErrTimedOut {                   // We don't care about timeout errors here!
			return value, nil
		}
		return value, err
	}
	actFn := f.newAction(
		selector, DOMElementStateAttached, parsedOpts.Strict, fn, []string{}, false, true, parsedOpts.Timeout,
	)
	value, err := callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		k6common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
	return value.(bool)
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

func (f *Frame) IsDisabled(selector string, opts goja.Value) bool {
	f.log.Debugf("Frame:IsDisabled", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	rt := k6common.GetRuntime(f.ctx)
	parsedOpts := NewFrameIsDisabledOptions(f.defaultTimeout())
	if err := parsedOpts.Parse(f.ctx, opts); err != nil {
		k6common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		value, err := handle.isDisabled(apiCtx, 0) // Zero timeout when checking state
		if err == ErrTimedOut {                    // We don't care about timeout errors here!
			return value, nil
		}
		return value, err
	}
	actFn := f.newAction(
		selector, DOMElementStateAttached, parsedOpts.Strict, fn, []string{}, false, true, parsedOpts.Timeout,
	)
	value, err := callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		k6common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
	return value.(bool)
}

func (f *Frame) IsEditable(selector string, opts goja.Value) bool {
	f.log.Debugf("Frame:IsEditable", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	rt := k6common.GetRuntime(f.ctx)
	parsedOpts := NewFrameIsEditableOptions(f.defaultTimeout())
	if err := parsedOpts.Parse(f.ctx, opts); err != nil {
		k6common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		value, err := handle.isEditable(apiCtx, 0) // Zero timeout when checking state
		if err == ErrTimedOut {                    // We don't care about timeout errors here!
			return value, nil
		}
		return value, err
	}
	actFn := f.newAction(
		selector, DOMElementStateAttached, parsedOpts.Strict, fn, []string{}, false, true, parsedOpts.Timeout,
	)
	value, err := callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		k6common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
	return value.(bool)
}

func (f *Frame) IsEnabled(selector string, opts goja.Value) bool {
	f.log.Debugf("Frame:IsEnabled", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	rt := k6common.GetRuntime(f.ctx)
	parsedOpts := NewFrameIsEnabledOptions(f.defaultTimeout())
	if err := parsedOpts.Parse(f.ctx, opts); err != nil {
		k6common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		value, err := handle.isEnabled(apiCtx, 0) // Zero timeout when checking state
		if err == ErrTimedOut {                   // We don't care about timeout errors here!
			return value, nil
		}
		return value, err
	}
	actFn := f.newAction(
		selector, DOMElementStateAttached, parsedOpts.Strict, fn, []string{}, false, true, parsedOpts.Timeout,
	)
	value, err := callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		k6common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
	return value.(bool)
}

func (f *Frame) IsHidden(selector string, opts goja.Value) bool {
	f.log.Debugf("Frame:IsHidden", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	rt := k6common.GetRuntime(f.ctx)
	parsedOpts := NewFrameIsHiddenOptions(f.defaultTimeout())
	if err := parsedOpts.Parse(f.ctx, opts); err != nil {
		k6common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		value, err := handle.isHidden(apiCtx, 0) // Zero timeout when checking state
		if err == ErrTimedOut {                  // We don't care about timeout errors here!
			return value, nil
		}
		return value, err
	}
	actFn := f.newAction(
		selector, DOMElementStateAttached, parsedOpts.Strict, fn, []string{}, false, true, parsedOpts.Timeout,
	)
	value, err := callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		k6common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
	return value.(bool)
}

func (f *Frame) IsVisible(selector string, opts goja.Value) bool {
	f.log.Debugf("Frame:IsVisible", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	rt := k6common.GetRuntime(f.ctx)
	parsedOpts := NewFrameIsVisibleOptions(f.defaultTimeout())
	if err := parsedOpts.Parse(f.ctx, opts); err != nil {
		k6common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		value, err := handle.isVisible(apiCtx, 0) // Zero timeout when checking state
		if err == ErrTimedOut {                   // We don't care about timeout errors here!
			return value, nil
		}
		return value, err
	}
	actFn := f.newAction(
		selector, DOMElementStateAttached, parsedOpts.Strict, fn, []string{}, false, true, parsedOpts.Timeout,
	)
	value, err := callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		k6common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
	return value.(bool)
}

// ID returns the frame id.
func (f *Frame) ID() string {
	f.propertiesMu.RLock()
	defer f.propertiesMu.RUnlock()

	return f.id.String()
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

	rt := k6common.GetRuntime(f.ctx)
	document, err := f.document()
	if err != nil {
		k6common.Throw(rt, err)
	}
	value := document.Query(selector)
	if value != nil {
		return value
	}
	return nil
}

func (f *Frame) QueryAll(selector string) []api.ElementHandle {
	f.log.Debugf("Frame:QueryAll", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	rt := k6common.GetRuntime(f.ctx)
	document, err := f.document()
	if err != nil {
		k6common.Throw(rt, err)
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

func (f *Frame) Press(selector string, key string, opts goja.Value) {
	f.log.Debugf("Frame:Press", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	rt := k6common.GetRuntime(f.ctx)
	parsedOpts := NewFramePressOptions(f.defaultTimeout())
	if err := parsedOpts.Parse(f.ctx, opts); err != nil {
		k6common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return nil, handle.press(apiCtx, key, parsedOpts.ToKeyboardOptions())
	}
	actFn := f.newAction(
		selector, DOMElementStateAttached, parsedOpts.Strict, fn, []string{}, false,
		parsedOpts.NoWaitAfter, parsedOpts.Timeout,
	)
	_, err := callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		k6common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
}

func (f *Frame) SelectOption(selector string, values goja.Value, opts goja.Value) []string {
	f.log.Debugf("Frame:SelectOption", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	rt := k6common.GetRuntime(f.ctx)
	parsedOpts := NewFrameSelectOptionOptions(f.defaultTimeout())
	if err := parsedOpts.Parse(f.ctx, opts); err != nil {
		k6common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return handle.selectOption(apiCtx, values)
	}
	actFn := f.newAction(
		selector, DOMElementStateAttached, parsedOpts.Strict, fn, []string{},
		parsedOpts.Force, parsedOpts.NoWaitAfter, parsedOpts.Timeout,
	)
	value, err := callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		k6common.Throw(rt, err)
	}

	arrayHandle, ok := value.(api.JSHandle)
	if !ok {
		k6common.Throw(rt, err)
	}
	properties := arrayHandle.GetProperties()
	strArr := make([]string, 0, len(properties))
	for _, property := range properties {
		strArr = append(strArr, property.JSONValue().String())
		property.Dispose()
	}
	arrayHandle.Dispose()

	applySlowMo(f.ctx)
	return strArr
}

// SetContent replaces the entire HTML document content.
func (f *Frame) SetContent(html string, opts goja.Value) {
	f.log.Debugf("Frame:SetContent", "fid:%s furl:%q", f.ID(), f.URL())

	rt := k6common.GetRuntime(f.ctx)
	parsedOpts := NewFrameSetContentOptions(f.defaultTimeout())
	if err := parsedOpts.Parse(f.ctx, opts); err != nil {
		k6common.Throw(rt, fmt.Errorf("failed parsing options: %w", err))
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
	if _, err := f.evaluate(f.ctx, utilityWorld, eopts, rt.ToValue(js), rt.ToValue(html)); err != nil {
		k6common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
}

func (f *Frame) SetInputFiles(selector string, files goja.Value, opts goja.Value) {
	rt := k6common.GetRuntime(f.ctx)
	k6common.Throw(rt, errors.New("Frame.setInputFiles(selector, files, opts) has not been implemented yet"))
	// TODO: needs slowMo
}

func (f *Frame) Tap(selector string, opts goja.Value) {
	f.log.Debugf("Frame:Tap", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	rt := k6common.GetRuntime(f.ctx)
	parsedOpts := NewFrameTapOptions(f.defaultTimeout())
	err := parsedOpts.Parse(f.ctx, opts)
	if err != nil {
		k6common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle, p *Position) (interface{}, error) {
		return nil, handle.tap(apiCtx, p)
	}
	actFn := f.newPointerAction(
		selector, DOMElementStateAttached, parsedOpts.Strict, fn, &parsedOpts.ElementHandleBasePointerOptions,
	)
	_, err = callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		k6common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
}

func (f *Frame) TextContent(selector string, opts goja.Value) string {
	f.log.Debugf("Frame:TextContent", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	rt := k6common.GetRuntime(f.ctx)
	parsedOpts := NewFrameTextContentOptions(f.defaultTimeout())
	if err := parsedOpts.Parse(f.ctx, opts); err != nil {
		k6common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return handle.textContent(apiCtx)
	}
	actFn := f.newAction(
		selector, DOMElementStateAttached, parsedOpts.Strict, fn, []string{}, false, true, parsedOpts.Timeout,
	)
	value, err := callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		k6common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
	return value.(string)
}

func (f *Frame) Title() string {
	f.log.Debugf("Frame:Title", "fid:%s furl:%q", f.ID(), f.URL())

	rt := k6common.GetRuntime(f.ctx)
	return f.Evaluate(rt.ToValue("document.title")).(string)
}

func (f *Frame) Type(selector string, text string, opts goja.Value) {
	f.log.Debugf("Frame:Type", "fid:%s furl:%q sel:%q text:%s", f.ID(), f.URL(), selector, text)

	rt := k6common.GetRuntime(f.ctx)
	parsedOpts := NewFrameTypeOptions(f.defaultTimeout())
	if err := parsedOpts.Parse(f.ctx, opts); err != nil {
		k6common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return nil, handle.typ(apiCtx, text, parsedOpts.ToKeyboardOptions())
	}
	actFn := f.newAction(
		selector, DOMElementStateAttached, parsedOpts.Strict, fn, []string{}, false,
		parsedOpts.NoWaitAfter, parsedOpts.Timeout,
	)
	_, err := callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		k6common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
}

func (f *Frame) Uncheck(selector string, opts goja.Value) {
	f.log.Debugf("Frame:Uncheck", "fid:%s furl:%q sel:%q", f.ID(), f.URL(), selector)

	rt := k6common.GetRuntime(f.ctx)
	parsedOpts := NewFrameUncheckOptions(f.defaultTimeout())
	err := parsedOpts.Parse(f.ctx, opts)
	if err != nil {
		k6common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle, p *Position) (interface{}, error) {
		return nil, handle.setChecked(apiCtx, false, p)
	}
	actFn := f.newPointerAction(
		selector, DOMElementStateAttached, parsedOpts.Strict, fn, &parsedOpts.ElementHandleBasePointerOptions,
	)
	_, err = callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		k6common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
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
func (f *Frame) WaitForFunction(pageFunc goja.Value, opts goja.Value, args ...goja.Value) api.JSHandle {
	f.log.Debugf("Frame:WaitForFunction", "fid:%s furl:%q", f.ID(), f.URL())

	rt := k6common.GetRuntime(f.ctx)
	parsedOpts := NewFrameWaitForFunctionOptions(f.defaultTimeout())
	err := parsedOpts.Parse(f.ctx, opts)
	if err != nil {
		k6common.Throw(rt, fmt.Errorf("failed parsing options: %w", err))
	}

	f.executionContextMu.RLock()
	defer f.executionContextMu.RUnlock()

	handle, err := f.waitForFunction(f.ctx, utilityWorld, pageFunc,
		parsedOpts.Polling, parsedOpts.Interval, parsedOpts.Timeout, args...)
	if err != nil {
		k6common.Throw(rt, err)
	}
	return handle.(api.JSHandle)
}

// WaitForLoadState waits for the given load state to be reached.
func (f *Frame) WaitForLoadState(state string, opts goja.Value) {
	f.log.Debugf("Frame:WaitForLoadState", "fid:%s furl:%q state:%s", f.ID(), f.URL(), state)
	defer f.log.Debugf("Frame:WaitForLoadState:return", "fid:%s furl:%q state:%s", f.ID(), f.URL(), state)

	parsedOpts := NewFrameWaitForLoadStateOptions(f.defaultTimeout())
	err := parsedOpts.Parse(f.ctx, opts)
	if err != nil {
		k6Throw(f.ctx, "cannot parse waitForLoadState options: %v", err)
	}

	waitUntil := LifecycleEventLoad
	switch state {
	case "domcontentloaded":
		waitUntil = LifecycleEventDOMContentLoad
	case "networkidle":
		waitUntil = LifecycleEventNetworkIdle
	}

	if f.hasLifecycleEventFired(waitUntil) {
		return
	}

	_, err = waitForEvent(f.ctx, f, []string{EventFrameAddLifecycle}, func(data interface{}) bool {
		return data.(LifecycleEvent) == waitUntil
	}, parsedOpts.Timeout)
	if err != nil {
		k6Throw(f.ctx, "cannot waitForEvent: %v", err)
	}
}

// WaitForNavigation waits for the given navigation lifecycle event to happen.
func (f *Frame) WaitForNavigation(opts goja.Value) api.Response {
	return f.manager.WaitForFrameNavigation(f, opts)
}

// WaitForSelector waits for the given selector to match the waiting criteria.
func (f *Frame) WaitForSelector(selector string, opts goja.Value) api.ElementHandle {
	rt := k6common.GetRuntime(f.ctx)
	parsedOpts := NewFrameWaitForSelectorOptions(f.defaultTimeout())
	if err := parsedOpts.Parse(f.ctx, opts); err != nil {
		k6common.Throw(rt, fmt.Errorf("failed parsing options: %w", err))
	}
	handle, err := f.waitForSelectorRetry(selector, parsedOpts, maxRetry)
	if err != nil {
		k6common.Throw(rt, err)
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
		return nil, fmt.Errorf("cannot find execution context: %q for %d", world, id)
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
		return nil, fmt.Errorf("cannot find execution context: %q", world)
	}
	eh, err := ec.eval(apiCtx, opts, pageFunc, args...)
	if err != nil {
		return nil, fmt.Errorf("frame cannot evaluate: %w", err)
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

	// eval will evaluate provided callable within this execution
	// context and return by value or handle.
	eval(
		apiCtx context.Context,
		opts evalOptions,
		pageFunc goja.Value, args ...goja.Value,
	) (res interface{}, err error)

	// getInjectedScript returns a JS handle to the injected script of helper
	// functions.
	getInjectedScript(apiCtx context.Context) (api.JSHandle, error)

	// Eval will evaluate provided page function within this execution
	// context.
	Eval(
		apiCtx context.Context,
		pageFunc goja.Value, args ...goja.Value,
	) (interface{}, error)

	// EvalHandle will evaluate provided page function within this
	// execution context.
	EvalHandle(
		apiCtx context.Context,
		pageFunc goja.Value, args ...goja.Value,
	) (api.JSHandle, error)

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
