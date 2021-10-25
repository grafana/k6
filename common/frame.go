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
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/runtime"
	"github.com/dop251/goja"
	"github.com/grafana/xk6-browser/api"
	"github.com/pkg/errors"
	"go.k6.io/k6/js/common"
	"golang.org/x/net/context"
)

// Ensure frame implements the Frame interface
var _ api.Frame = &Frame{}

func frameActionFn(f *Frame, selector string, state DOMElementState, strict bool, fn ElementHandleActionFn, states []string, force, noWaitAfter bool, timeout time.Duration) func(apiCtx context.Context, resultCh chan interface{}, errCh chan error) {
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
		actFn := elementHandleActionFn(handle, states, fn, false, false, timeout)
		actFn(apiCtx, resultCh, errCh)
	}
}

func framePointerActionFn(f *Frame, selector string, state DOMElementState, strict bool, fn ElementHandlePointerActionFn, opts *ElementHandleBasePointerOptions) func(apiCtx context.Context, resultCh chan interface{}, errCh chan error) {
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
		pointerActFn := elementHandlePointerActionFn(handle, true, fn, opts)
		pointerActFn(apiCtx, resultCh, errCh)
	}
}

type DocumentInfo struct {
	documentID string
	request    *Request
}

// Frame represents a frame in an HTML document
type Frame struct {
	BaseEventEmitter

	ctx         context.Context
	page        *Page
	manager     *FrameManager
	parentFrame *Frame

	childFramesMu sync.RWMutex
	childFrames   map[api.Frame]bool
	id            cdp.FrameID
	loaderID      string
	name          string
	url           string
	detached      bool

	// A life cycle event is only considered triggered for a frame if the entire
	// frame subtree has also had the life cycle event triggered.
	lifecycleEventsMu      sync.RWMutex
	lifecycleEvents        map[LifecycleEvent]bool
	subtreeLifecycleEvents map[LifecycleEvent]bool

	documentHandle *ElementHandle

	mainExecutionContext             *ExecutionContext
	utilityExecutionContext          *ExecutionContext
	mainExecutionContextCh           chan bool
	utilityExecutionContextCh        chan bool
	mainExecutionContextHasWaited    int32
	utilityExecutionContextHasWaited int32

	loadingStartedTime time.Time

	networkIdleMu       sync.Mutex
	networkIdleCtx      context.Context
	networkIdleCancelFn context.CancelFunc

	inflightRequestsMu sync.RWMutex
	inflightRequests   map[network.RequestID]bool

	currentDocument *DocumentInfo
	pendingDocument *DocumentInfo
}

// NewFrame creates a new HTML document frame
func NewFrame(ctx context.Context, m *FrameManager, parentFrame *Frame, frameID cdp.FrameID) *Frame {
	return &Frame{
		BaseEventEmitter:          NewBaseEventEmitter(),
		ctx:                       ctx,
		page:                      m.page,
		manager:                   m,
		parentFrame:               parentFrame,
		childFrames:               make(map[api.Frame]bool),
		id:                        frameID,
		lifecycleEvents:           make(map[LifecycleEvent]bool),
		subtreeLifecycleEvents:    make(map[LifecycleEvent]bool),
		mainExecutionContextCh:    make(chan bool, 1),
		utilityExecutionContextCh: make(chan bool, 1),
		inflightRequests:          make(map[network.RequestID]bool),
		currentDocument:           &DocumentInfo{},
	}
}

func (f *Frame) addChildFrame(childFrame *Frame) {
	f.childFramesMu.Lock()
	f.childFrames[childFrame] = true
	f.childFramesMu.Unlock()
}

func (f *Frame) addRequest(requestID network.RequestID) {
	f.inflightRequestsMu.Lock()
	defer f.inflightRequestsMu.Unlock()

	f.inflightRequests[requestID] = true
}

func (f *Frame) deleteRequest(requestID network.RequestID) {
	f.inflightRequestsMu.Lock()
	defer f.inflightRequestsMu.Unlock()

	delete(f.inflightRequests, requestID)
}

func (f *Frame) getInflightRequestCount() int {
	f.inflightRequestsMu.RLock()
	defer f.inflightRequestsMu.RUnlock()
	return len(f.inflightRequests)
}

func (f *Frame) hasInflightRequest(requestID network.RequestID) bool {
	f.inflightRequestsMu.RLock()
	defer f.inflightRequestsMu.RUnlock()

	return f.inflightRequests[requestID]
}

func (f *Frame) clearLifecycle() {
	f.lifecycleEventsMu.RLock()
	for k := range f.lifecycleEvents {
		f.lifecycleEvents[k] = false
	}
	f.lifecycleEventsMu.RUnlock()
	f.page.frameManager.mainFrame.recalculateLifecycle()

	f.inflightRequestsMu.Lock()
	defer f.inflightRequestsMu.Unlock()
	inflightRequests := make(map[network.RequestID]bool)
	for req := range f.inflightRequests {
		if req == f.currentDocument.request.requestID {
			inflightRequests[req] = true
		}
	}
	f.stopNetworkIdleTimer()
	if len(f.inflightRequests) == 0 {
		f.startNetworkIdleTimer()
	}
}

func (f *Frame) defaultTimeout() time.Duration {
	return time.Duration(f.manager.timeoutSettings.timeout()) * time.Second
}

func (f *Frame) detach() {
	f.stopNetworkIdleTimer()
	f.detached = true
	if f.parentFrame != nil {
		f.parentFrame.removeChildFrame(f)
	}
	f.parentFrame = nil
	if f.documentHandle != nil {
		f.documentHandle.Dispose()
	}
}

func (f *Frame) document() (*ElementHandle, error) {
	rt := common.GetRuntime(f.ctx)
	if f.documentHandle != nil {
		return f.documentHandle, nil
	}
	f.waitForExecutionContext("main")
	result, err := f.mainExecutionContext.evaluate(f.ctx, false, false, rt.ToValue("document"), nil)
	if err != nil {
		return nil, err
	}
	f.documentHandle = result.(*ElementHandle)
	return f.documentHandle, err
}

func (f *Frame) getLoadingStartedTime() time.Time {
	return f.loadingStartedTime
}

func (f *Frame) hasContext(world string) bool {
	switch world {
	case "main":
		return f.mainExecutionContext != nil
	case "utility":
		return f.utilityExecutionContext != nil
	}
	return false // Should never reach here!
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
	f.name = name
	f.url = url
	f.loaderID = loaderID
	f.page.emit(EventPageFrameNavigated, f)
}

func (f *Frame) nullContext(execCtxID runtime.ExecutionContextID) {
	if f.mainExecutionContext != nil && f.mainExecutionContext.id == execCtxID {
		f.mainExecutionContext = nil
		f.documentHandle = nil
	} else if f.utilityExecutionContext != nil && f.utilityExecutionContext.id == execCtxID {
		f.utilityExecutionContext = nil
	}
}

func (f *Frame) nullContexts() {
	f.mainExecutionContext = nil
	f.utilityExecutionContext = nil
	f.documentHandle = nil
}

func (f *Frame) onLifecycleEvent(event LifecycleEvent) {
	f.lifecycleEventsMu.Lock()
	defer f.lifecycleEventsMu.Unlock()
	if ok := f.lifecycleEvents[event]; ok {
		return
	}
	f.lifecycleEvents[event] = true
}

func (f *Frame) onLoadingStarted() {
	f.loadingStartedTime = time.Now()
}

func (f *Frame) onLoadingStopped() {
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
	if frame == f.page.frameManager.mainFrame {
		return &Position{X: 0, Y: 0}
	}
	element := frame.FrameElement()
	box := element.BoundingBox()
	return &Position{X: box.X, Y: box.Y}
}

func (f *Frame) recalculateLifecycle() {
	// Start with triggered events.
	var events map[LifecycleEvent]bool = make(map[LifecycleEvent]bool)
	f.lifecycleEventsMu.Lock()
	for k, v := range f.lifecycleEvents {
		events[k] = v
	}
	f.lifecycleEventsMu.Unlock()

	// Only consider a life cycle event as fired if it has triggered for all of subtree.
	f.childFramesMu.RLock()
	for child := range f.childFrames {
		child.(*Frame).recalculateLifecycle()
		for k := range events {
			if !child.(*Frame).hasSubtreeLifecycleEventFired(k) {
				delete(events, k)
			}
		}
	}
	f.childFramesMu.RUnlock()

	// Check if any of the fired events should be considered fired when looking at the entire subtree.
	mainFrame := f.manager.mainFrame
	for k := range events {
		if !f.hasSubtreeLifecycleEventFired(k) {
			f.emit(EventFrameAddLifecycle, k)
			if f == mainFrame && k == LifecycleEventLoad {
				f.page.emit(EventPageLoad, nil)
			} else if f == mainFrame && k == LifecycleEventDOMContentLoad {
				f.page.emit(EventPageDOMContentLoaded, nil)
			}
		}
	}

	// Emit removal events
	for k := range f.subtreeLifecycleEvents {
		if ok := events[k]; !ok {
			f.emit(EventFrameRemoveLifecycle, k)
		}
	}

	f.lifecycleEventsMu.Lock()
	f.subtreeLifecycleEvents = make(map[LifecycleEvent]bool)
	for k, v := range events {
		f.subtreeLifecycleEvents[k] = v
	}
	f.lifecycleEventsMu.Unlock()
}

func (f *Frame) removeChildFrame(childFrame *Frame) {
	f.childFramesMu.Lock()
	delete(f.childFrames, childFrame)
	f.childFramesMu.Unlock()
}

func (f *Frame) requestByID(reqID network.RequestID) *Request {
	frameSession := f.page.getFrameSession(f.id)
	if frameSession == nil {
		frameSession = f.page.mainFrameSession
	}
	return frameSession.networkManager.requestFromID(reqID)
}

func (f *Frame) setContext(world string, execCtx *ExecutionContext) {
	if world == "main" {
		f.mainExecutionContext = execCtx
		if len(f.mainExecutionContextCh) == 0 {
			f.mainExecutionContextCh <- true
		}
	} else if world == "utility" {
		f.utilityExecutionContext = execCtx
		if len(f.utilityExecutionContextCh) == 0 {
			f.utilityExecutionContextCh <- true
		}
	}
}

func (f *Frame) setID(id cdp.FrameID) {
	f.id = id
}

func (f *Frame) startNetworkIdleTimer() {
	if f.hasLifecycleEventFired(LifecycleEventNetworkIdle) || f.detached {
		return
	}
	if f.networkIdleCancelFn != nil {
		f.networkIdleCancelFn()
	}
	f.networkIdleMu.Lock()
	f.networkIdleCtx, f.networkIdleCancelFn = context.WithCancel(f.ctx)
	f.networkIdleMu.Unlock()
	go func() {
		f.networkIdleMu.Lock()
		doneCh := f.networkIdleCtx.Done()
		f.networkIdleMu.Unlock()
		select {
		case <-doneCh:
		case <-time.After(LifeCycleNetworkIdleTimeout):
			f.manager.frameLifecycleEvent(f.id, LifecycleEventNetworkIdle)
		}
	}()
}

func (f *Frame) stopNetworkIdleTimer() {
	if f.networkIdleCancelFn != nil {
		f.networkIdleCancelFn()
		f.networkIdleMu.Lock()
		f.networkIdleCtx = nil
		f.networkIdleCancelFn = nil
		f.networkIdleMu.Unlock()
	}
}

func (f *Frame) waitForExecutionContext(world string) {
	if world == "main" && atomic.CompareAndSwapInt32(&f.mainExecutionContextHasWaited, 0, 1) {
		select {
		case <-f.ctx.Done():
		case <-f.mainExecutionContextCh:
		}
	} else if world == "utility" && atomic.CompareAndSwapInt32(&f.utilityExecutionContextHasWaited, 0, 1) {
		select {
		case <-f.ctx.Done():
		case <-f.utilityExecutionContextCh:
		}
	}
}

func (f *Frame) waitForFunction(apiCtx context.Context, world string, predicateFn goja.Value, polling PollingType, interval int64, timeout time.Duration, args ...goja.Value) (interface{}, error) {
	rt := common.GetRuntime(f.ctx)
	f.waitForExecutionContext(world)
	execCtx := f.mainExecutionContext
	if world == "utility" {
		execCtx = f.utilityExecutionContext
	}
	injected, err := execCtx.getInjectedScript(apiCtx)
	if err != nil {
		return nil, err
	}
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
	result, err := execCtx.evaluate(
		apiCtx, true, false, pageFn, append([]goja.Value{
			rt.ToValue(injected),
			rt.ToValue(predicate),
			rt.ToValue(polling),
		}, args...)...)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (f *Frame) waitForSelector(selector string, opts *FrameWaitForSelectorOptions) (*ElementHandle, error) {
	document, err := f.document()
	if err != nil {
		return nil, err
	}

	handle, err := document.waitForSelector(f.ctx, selector, opts)
	if err != nil {
		return nil, err
	}
	if handle == nil {
		return nil, fmt.Errorf("wait for selector didn't resulted in any nodes")
	}

	// We always return ElementHandles in the main execution context (aka "DOM world")
	if handle.execCtx != f.mainExecutionContext {
		defer handle.Dispose()
		handle, err = f.mainExecutionContext.adoptElementHandle(handle)
		if err != nil {
			return nil, err
		}
	}

	return handle, nil
}

func (f *Frame) AddScriptTag(opts goja.Value) {
	rt := common.GetRuntime(f.ctx)
	common.Throw(rt, errors.Errorf("Frame.AddScriptTag() has not been implemented yet!"))
	applySlowMo(f.ctx)
}

func (f *Frame) AddStyleTag(opts goja.Value) {
	rt := common.GetRuntime(f.ctx)
	common.Throw(rt, errors.Errorf("Frame.AddStyleTag() has not been implemented yet!"))
	applySlowMo(f.ctx)
}

// Check clicks the first element found that matches selector
func (f *Frame) Check(selector string, opts goja.Value) {
	rt := common.GetRuntime(f.ctx)
	parsedOpts := NewFrameCheckOptions(f.defaultTimeout())
	err := parsedOpts.Parse(f.ctx, opts)
	if err != nil {
		common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle, p *Position) (interface{}, error) {
		return nil, handle.setChecked(apiCtx, true, p)
	}
	actFn := framePointerActionFn(f, selector, DOMElementStateAttached, parsedOpts.Strict, fn, &parsedOpts.ElementHandleBasePointerOptions)
	_, err = callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
}

// ChildFrames returns a list of child frames
func (f *Frame) ChildFrames() []api.Frame {
	f.childFramesMu.RLock()
	defer f.childFramesMu.RUnlock()
	l := make([]api.Frame, len(f.childFrames))
	for child := range f.childFrames {
		l = append(l, child)
	}
	return l
}

// Click clicks the first element found that matches selector
func (f *Frame) Click(selector string, opts goja.Value) {
	rt := common.GetRuntime(f.ctx)
	parsedOpts := NewFrameClickOptions(f.defaultTimeout())
	err := parsedOpts.Parse(f.ctx, opts)
	if err != nil {
		common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle, p *Position) (interface{}, error) {
		return nil, handle.click(p, parsedOpts.ToMouseClickOptions())
	}
	actFn := framePointerActionFn(f, selector, DOMElementStateAttached, parsedOpts.Strict, fn, &parsedOpts.ElementHandleBasePointerOptions)
	_, err = callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
}

// Content returns the HTML content of the frame
func (f *Frame) Content() string {
	rt := common.GetRuntime(f.ctx)
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

// Dblclick double clicks an element matching provided selector
func (f *Frame) Dblclick(selector string, opts goja.Value) {
	rt := common.GetRuntime(f.ctx)
	parsedOpts := NewFrameDblClickOptions(f.defaultTimeout())
	err := parsedOpts.Parse(f.ctx, opts)
	if err != nil {
		common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle, p *Position) (interface{}, error) {
		return nil, handle.dblClick(p, parsedOpts.ToMouseClickOptions())
	}
	actFn := framePointerActionFn(f, selector, DOMElementStateAttached, parsedOpts.Strict, fn, &parsedOpts.ElementHandleBasePointerOptions)
	_, err = callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
}

func (f *Frame) DispatchEvent(selector string, typ string, eventInit goja.Value, opts goja.Value) {
	rt := common.GetRuntime(f.ctx)
	parsedOpts := NewFrameDblClickOptions(f.defaultTimeout())
	if err := parsedOpts.Parse(f.ctx, opts); err != nil {
		common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return handle.dispatchEvent(apiCtx, typ, eventInit)
	}
	actFn := frameActionFn(f, selector, DOMElementStateAttached, parsedOpts.Strict, fn, []string{}, parsedOpts.Force, parsedOpts.NoWaitAfter, parsedOpts.Timeout)
	_, err := callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
}

// Evaluate will evaluate provided page function within an execution context
func (f *Frame) Evaluate(pageFunc goja.Value, args ...goja.Value) interface{} {
	rt := common.GetRuntime(f.ctx)
	f.waitForExecutionContext("main")
	i, err := f.mainExecutionContext.Evaluate(f.ctx, pageFunc, args...)
	if err != nil {
		common.Throw(rt, err)
	}
	applySlowMo(f.ctx)
	return i
}

// EvaluateHandle will evaluate provided page function within an execution context
func (f *Frame) EvaluateHandle(pageFunc goja.Value, args ...goja.Value) api.JSHandle {
	rt := common.GetRuntime(f.ctx)
	f.waitForExecutionContext("main")
	handle, err := f.mainExecutionContext.EvaluateHandle(f.ctx, pageFunc, args...)
	if err != nil {
		common.Throw(rt, err)
	}
	applySlowMo(f.ctx)
	return handle
}

func (f *Frame) Fill(selector string, value string, opts goja.Value) {
	rt := common.GetRuntime(f.ctx)
	parsedOpts := NewFrameFillOptions(f.defaultTimeout())
	if err := parsedOpts.Parse(f.ctx, opts); err != nil {
		common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return handle.fill(apiCtx, value)
	}
	actFn := frameActionFn(f, selector, DOMElementStateAttached, parsedOpts.Strict, fn, []string{"visible", "enabled", "editable"}, parsedOpts.Force, parsedOpts.NoWaitAfter, parsedOpts.Timeout)
	_, err := callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
}

// Focus fetches an element with selector and focuses it
func (f *Frame) Focus(selector string, opts goja.Value) {
	rt := common.GetRuntime(f.ctx)
	parsedOpts := NewFrameBaseOptions(f.defaultTimeout())
	if err := parsedOpts.Parse(f.ctx, opts); err != nil {
		common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return nil, handle.focus(apiCtx, true)
	}
	actFn := frameActionFn(f, selector, DOMElementStateAttached, parsedOpts.Strict, fn, []string{}, false, true, parsedOpts.Timeout)
	_, err := callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
}

func (f *Frame) FrameElement() api.ElementHandle {
	rt := common.GetRuntime(f.ctx)
	element, err := f.page.getFrameElement(f)
	if err != nil {
		common.Throw(rt, err)
	}
	return element
}

func (f *Frame) GetAttribute(selector string, name string, opts goja.Value) goja.Value {
	rt := common.GetRuntime(f.ctx)
	parsedOpts := NewFrameBaseOptions(f.defaultTimeout())
	if err := parsedOpts.Parse(f.ctx, opts); err != nil {
		common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return handle.getAttribute(apiCtx, name)
	}
	actFn := frameActionFn(f, selector, DOMElementStateAttached, parsedOpts.Strict, fn, []string{}, false, true, parsedOpts.Timeout)
	value, err := callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
	return value.(goja.Value)
}

// Goto will navigate the frame to the specified URL and return a HTTP response object
func (f *Frame) Goto(url string, opts goja.Value) api.Response {
	resp := f.manager.NavigateFrame(f, url, opts)
	applySlowMo(f.ctx)
	return resp
}

// Hover hovers an element identified by provided selector
func (f *Frame) Hover(selector string, opts goja.Value) {
	rt := common.GetRuntime(f.ctx)
	parsedOpts := NewFrameHoverOptions(f.defaultTimeout())
	err := parsedOpts.Parse(f.ctx, opts)
	if err != nil {
		common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle, p *Position) (interface{}, error) {
		return nil, handle.hover(apiCtx, p)
	}
	actFn := framePointerActionFn(f, selector, DOMElementStateAttached, parsedOpts.Strict, fn, &parsedOpts.ElementHandleBasePointerOptions)
	_, err = callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
}

func (f *Frame) InnerHTML(selector string, opts goja.Value) string {
	rt := common.GetRuntime(f.ctx)
	parsedOpts := NewFrameInnerHTMLOptions(f.defaultTimeout())
	if err := parsedOpts.Parse(f.ctx, opts); err != nil {
		common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return handle.innerHTML(apiCtx)
	}
	actFn := frameActionFn(f, selector, DOMElementStateAttached, parsedOpts.Strict, fn, []string{}, false, true, parsedOpts.Timeout)
	value, err := callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
	return value.(string)
}

func (f *Frame) InnerText(selector string, opts goja.Value) string {
	rt := common.GetRuntime(f.ctx)
	parsedOpts := NewFrameInnerHTMLOptions(f.defaultTimeout())
	if err := parsedOpts.Parse(f.ctx, opts); err != nil {
		common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return handle.innerText(apiCtx)
	}
	actFn := frameActionFn(f, selector, DOMElementStateAttached, parsedOpts.Strict, fn, []string{}, false, true, parsedOpts.Timeout)
	value, err := callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
	return value.(string)
}

func (f *Frame) InputValue(selector string, opts goja.Value) string {
	rt := common.GetRuntime(f.ctx)
	parsedOpts := NewFrameInputValueOptions(f.defaultTimeout())
	if err := parsedOpts.Parse(f.ctx, opts); err != nil {
		common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return handle.inputValue(apiCtx)
	}
	actFn := frameActionFn(f, selector, DOMElementStateAttached, parsedOpts.Strict, fn, []string{}, false, true, parsedOpts.Timeout)
	value, err := callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
	return value.(goja.Value).String()
}

func (f *Frame) IsChecked(selector string, opts goja.Value) bool {
	rt := common.GetRuntime(f.ctx)
	parsedOpts := NewFrameIsCheckedOptions(f.defaultTimeout())
	if err := parsedOpts.Parse(f.ctx, opts); err != nil {
		common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		value, err := handle.isChecked(apiCtx, 0) // Zero timeout when checking state
		if err == ErrTimedOut {                   // We don't care about timeout errors here!
			return value, nil
		}
		return value, err
	}
	actFn := frameActionFn(f, selector, DOMElementStateAttached, parsedOpts.Strict, fn, []string{}, false, true, parsedOpts.Timeout)
	value, err := callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
	return value.(bool)
}

// IsDetached returns whether the frame is detached or not
func (f *Frame) IsDetached() bool {
	return f.detached
}

func (f *Frame) IsDisabled(selector string, opts goja.Value) bool {
	rt := common.GetRuntime(f.ctx)
	parsedOpts := NewFrameIsDisabledOptions(f.defaultTimeout())
	if err := parsedOpts.Parse(f.ctx, opts); err != nil {
		common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		value, err := handle.isDisabled(apiCtx, 0) // Zero timeout when checking state
		if err == ErrTimedOut {                    // We don't care about timeout errors here!
			return value, nil
		}
		return value, err
	}
	actFn := frameActionFn(f, selector, DOMElementStateAttached, parsedOpts.Strict, fn, []string{}, false, true, parsedOpts.Timeout)
	value, err := callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
	return value.(bool)
}

func (f *Frame) IsEditable(selector string, opts goja.Value) bool {
	rt := common.GetRuntime(f.ctx)
	parsedOpts := NewFrameIsEditableOptions(f.defaultTimeout())
	if err := parsedOpts.Parse(f.ctx, opts); err != nil {
		common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		value, err := handle.isEditable(apiCtx, 0) // Zero timeout when checking state
		if err == ErrTimedOut {                    // We don't care about timeout errors here!
			return value, nil
		}
		return value, err
	}
	actFn := frameActionFn(f, selector, DOMElementStateAttached, parsedOpts.Strict, fn, []string{}, false, true, parsedOpts.Timeout)
	value, err := callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
	return value.(bool)
}

func (f *Frame) IsEnabled(selector string, opts goja.Value) bool {
	rt := common.GetRuntime(f.ctx)
	parsedOpts := NewFrameIsEnabledOptions(f.defaultTimeout())
	if err := parsedOpts.Parse(f.ctx, opts); err != nil {
		common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		value, err := handle.isEnabled(apiCtx, 0) // Zero timeout when checking state
		if err == ErrTimedOut {                   // We don't care about timeout errors here!
			return value, nil
		}
		return value, err
	}
	actFn := frameActionFn(f, selector, DOMElementStateAttached, parsedOpts.Strict, fn, []string{}, false, true, parsedOpts.Timeout)
	value, err := callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
	return value.(bool)
}

func (f *Frame) IsHidden(selector string, opts goja.Value) bool {
	rt := common.GetRuntime(f.ctx)
	parsedOpts := NewFrameIsHiddenOptions(f.defaultTimeout())
	if err := parsedOpts.Parse(f.ctx, opts); err != nil {
		common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		value, err := handle.isHidden(apiCtx, 0) // Zero timeout when checking state
		if err == ErrTimedOut {                  // We don't care about timeout errors here!
			return value, nil
		}
		return value, err
	}
	actFn := frameActionFn(f, selector, DOMElementStateAttached, parsedOpts.Strict, fn, []string{}, false, true, parsedOpts.Timeout)
	value, err := callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
	return value.(bool)
}

func (f *Frame) IsVisible(selector string, opts goja.Value) bool {
	rt := common.GetRuntime(f.ctx)
	parsedOpts := NewFrameIsVisibleOptions(f.defaultTimeout())
	if err := parsedOpts.Parse(f.ctx, opts); err != nil {
		common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		value, err := handle.isVisible(apiCtx, 0) // Zero timeout when checking state
		if err == ErrTimedOut {                   // We don't care about timeout errors here!
			return value, nil
		}
		return value, err
	}
	actFn := frameActionFn(f, selector, DOMElementStateAttached, parsedOpts.Strict, fn, []string{}, false, true, parsedOpts.Timeout)
	value, err := callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
	return value.(bool)
}

// ID returns the frame id
func (f *Frame) ID() string {
	return f.id.String()
}

// LoaderID returns the ID of the frame that loaded this frame
func (f *Frame) LoaderID() string {
	return f.loaderID
}

// Name returns the frame name
func (f *Frame) Name() string {
	return f.name
}

// Query runs a selector query against the document tree, returning the first matching element or
// "null" if no match is found
func (f *Frame) Query(selector string) api.ElementHandle {
	rt := common.GetRuntime(f.ctx)
	document, err := f.document()
	if err != nil {
		common.Throw(rt, err)
	}
	value := document.Query(selector)
	if value != nil {
		return value
	}
	return nil
}

func (f *Frame) QueryAll(selector string) []api.ElementHandle {
	rt := common.GetRuntime(f.ctx)
	document, err := f.document()
	if err != nil {
		common.Throw(rt, err)
	}
	value := document.QueryAll(selector)
	if value != nil {
		return value
	}
	return nil
}

// Page returns page that owns frame
func (f *Frame) Page() api.Page {
	return f.manager.page
}

// ParentFrame returns the parent frame, if one exists
func (f *Frame) ParentFrame() api.Frame {
	return f.parentFrame
}

func (f *Frame) Press(selector string, key string, opts goja.Value) {
	rt := common.GetRuntime(f.ctx)
	parsedOpts := NewFramePressOptions(f.defaultTimeout())
	if err := parsedOpts.Parse(f.ctx, opts); err != nil {
		common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return nil, handle.press(apiCtx, key, parsedOpts.ToKeyboardOptions())
	}
	actFn := frameActionFn(f, selector, DOMElementStateAttached, parsedOpts.Strict, fn, []string{}, false, parsedOpts.NoWaitAfter, parsedOpts.Timeout)
	_, err := callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
}

func (f *Frame) SelectOption(selector string, values goja.Value, opts goja.Value) []string {
	rt := common.GetRuntime(f.ctx)
	parsedOpts := NewFrameSelectOptionOptions(f.defaultTimeout())
	if err := parsedOpts.Parse(f.ctx, opts); err != nil {
		common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return handle.selectOption(apiCtx, values)
	}
	actFn := frameActionFn(f, selector, DOMElementStateAttached, parsedOpts.Strict, fn, []string{}, parsedOpts.Force, parsedOpts.NoWaitAfter, parsedOpts.Timeout)
	value, err := callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		common.Throw(rt, err)
	}

	arrayHandle := value.(api.JSHandle)
	properties := arrayHandle.GetProperties()
	strArr := make([]string, len(properties))
	for _, property := range properties {
		strArr = append(strArr, property.JSONValue().String())
		property.Dispose()
	}
	arrayHandle.Dispose()

	applySlowMo(f.ctx)
	return strArr
}

// SetContent replaces the entire HTML document content
func (f *Frame) SetContent(html string, opts goja.Value) {
	rt := common.GetRuntime(f.ctx)
	parsedOpts := NewFrameSetContentOptions(f.defaultTimeout())
	if err := parsedOpts.Parse(f.ctx, opts); err != nil {
		common.Throw(rt, fmt.Errorf("failed parsing options: %v", err))
	}

	js := `(html) => {
		window.stop();
		document.open();
		document.write(html);
		document.close();
	}`
	f.waitForExecutionContext("utility")
	_, err := f.utilityExecutionContext.evaluate(f.ctx, true, true, rt.ToValue(js), rt.ToValue(html))
	if err != nil {
		common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
}

func (f *Frame) SetInputFiles(selector string, files goja.Value, opts goja.Value) {
	rt := common.GetRuntime(f.ctx)
	common.Throw(rt, errors.Errorf("Frame.setInputFiles(selector, files, opts) has not been implemented yet!"))
	// TODO: needs slowMo
}

func (f *Frame) Tap(selector string, opts goja.Value) {
	rt := common.GetRuntime(f.ctx)
	parsedOpts := NewFrameTapOptions(f.defaultTimeout())
	err := parsedOpts.Parse(f.ctx, opts)
	if err != nil {
		common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle, p *Position) (interface{}, error) {
		return nil, handle.tap(apiCtx, p)
	}
	actFn := framePointerActionFn(f, selector, DOMElementStateAttached, parsedOpts.Strict, fn, &parsedOpts.ElementHandleBasePointerOptions)
	_, err = callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
}

func (f *Frame) TextContent(selector string, opts goja.Value) string {
	rt := common.GetRuntime(f.ctx)
	parsedOpts := NewFrameTextContentOptions(f.defaultTimeout())
	if err := parsedOpts.Parse(f.ctx, opts); err != nil {
		common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return handle.textContent(apiCtx)
	}
	actFn := frameActionFn(f, selector, DOMElementStateAttached, parsedOpts.Strict, fn, []string{}, false, true, parsedOpts.Timeout)
	value, err := callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
	return value.(string)
}

func (f *Frame) Title() string {
	rt := common.GetRuntime(f.ctx)
	return f.Evaluate(rt.ToValue("document.title")).(string)
}

func (f *Frame) Type(selector string, text string, opts goja.Value) {
	rt := common.GetRuntime(f.ctx)
	parsedOpts := NewFrameTypeOptions(f.defaultTimeout())
	if err := parsedOpts.Parse(f.ctx, opts); err != nil {
		common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return nil, handle.typ(apiCtx, text, parsedOpts.ToKeyboardOptions())
	}
	actFn := frameActionFn(f, selector, DOMElementStateAttached, parsedOpts.Strict, fn, []string{}, false, parsedOpts.NoWaitAfter, parsedOpts.Timeout)
	_, err := callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
}

func (f *Frame) Uncheck(selector string, opts goja.Value) {
	rt := common.GetRuntime(f.ctx)
	parsedOpts := NewFrameUncheckOptions(f.defaultTimeout())
	err := parsedOpts.Parse(f.ctx, opts)
	if err != nil {
		common.Throw(rt, err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle, p *Position) (interface{}, error) {
		return nil, handle.setChecked(apiCtx, false, p)
	}
	actFn := framePointerActionFn(f, selector, DOMElementStateAttached, parsedOpts.Strict, fn, &parsedOpts.ElementHandleBasePointerOptions)
	_, err = callApiWithTimeout(f.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		common.Throw(rt, err)
	}

	applySlowMo(f.ctx)
}

// URL returns the frame URL
func (f *Frame) URL() string {
	return f.url
}

// WaitForFunction waits for the given predicate to return a truthy value
func (f *Frame) WaitForFunction(pageFunc goja.Value, opts goja.Value, args ...goja.Value) api.JSHandle {
	rt := common.GetRuntime(f.ctx)
	parsedOpts := NewFrameWaitForFunctionOptions(f.defaultTimeout())
	err := parsedOpts.Parse(f.ctx, opts)
	if err != nil {
		common.Throw(rt, errors.Errorf("failed parsing options: %v", err))
	}

	handle, err := f.waitForFunction(f.ctx, "utility", pageFunc, parsedOpts.Polling, parsedOpts.Interval, parsedOpts.Timeout, args...)
	if err != nil {
		common.Throw(rt, err)
	}
	return handle.(api.JSHandle)
}

// WaitForLoadState waits for the given load state to be reached
func (f *Frame) WaitForLoadState(state string, opts goja.Value) {
	rt := common.GetRuntime(f.ctx)
	parsedOpts := NewFrameWaitForLoadStateOptions(f.defaultTimeout())
	err := parsedOpts.Parse(f.ctx, opts)
	if err != nil {
		common.Throw(rt, errors.Errorf("failed parsing options: %v", err))
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

	waitForEvent(f.ctx, f, []string{EventFrameAddLifecycle}, func(data interface{}) bool {
		return data.(LifecycleEvent) == waitUntil
	}, parsedOpts.Timeout)
}

// WaitForNavigation waits for the given navigation lifecycle event to happen
func (f *Frame) WaitForNavigation(opts goja.Value) api.Response {
	return f.manager.WaitForFrameNavigation(f, opts)
}

// WaitForSelector waits for the given selector to match the waiting criteria
func (f *Frame) WaitForSelector(selector string, opts goja.Value) api.ElementHandle {
	rt := common.GetRuntime(f.ctx)
	parsedOpts := NewFrameWaitForSelectorOptions(f.defaultTimeout())
	if err := parsedOpts.Parse(f.ctx, opts); err != nil {
		common.Throw(rt, fmt.Errorf("failed parsing options: %v", err))
	}
	handle, err := f.waitForSelector(selector, parsedOpts)
	if err != nil {
		common.Throw(rt, err)
	}
	return handle
}

// WaitForTimeout waits the specified amount of milliseconds
func (f *Frame) WaitForTimeout(timeout int64) {
	select {
	case <-f.ctx.Done():
	case <-time.After(time.Duration(timeout) * time.Millisecond):
	}
}
