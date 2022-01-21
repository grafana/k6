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
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/emulation"
	cdppage "github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/target"
	"github.com/dop251/goja"
	"github.com/grafana/xk6-browser/api"
	k6common "go.k6.io/k6/js/common"
)

// Ensure page implements the EventEmitter, Target and Page interfaces.
var _ EventEmitter = &Page{}
var _ api.Page = &Page{}

// Page stores Page/tab related context
type Page struct {
	BaseEventEmitter

	Keyboard    *Keyboard    `js:"keyboard"`    // Public JS API
	Mouse       *Mouse       `js:"mouse"`       // Public JS API
	Touchscreen *Touchscreen `js:"touchscreen"` // Public JS API

	ctx             context.Context
	session         *Session
	browserCtx      *BrowserContext
	targetID        target.ID
	opener          *Page
	frameManager    *FrameManager
	timeoutSettings *TimeoutSettings

	jsEnabled bool

	// protects from race between:
	// - Browser.initEvents.onDetachedFromTarget->Page.didClose
	// - FrameSession.initEvents.onFrameDetached->FrameManager.frameDetached.removeFramesRecursively->Page.IsClosed
	closedMu sync.RWMutex
	closed   bool

	// TODO: setter change these fields (mutex?)
	emulatedSize     *EmulatedSize
	mediaType        MediaType
	colorScheme      ColorScheme
	reducedMotion    ReducedMotion
	extraHTTPHeaders map[string]string

	backgroundPage bool

	mainFrameSession *FrameSession
	// TODO: FrameSession changes by attachFrameSession (mutex?)
	frameSessions map[cdp.FrameID]*FrameSession
	workers       map[target.SessionID]*Worker
	routes        []api.Route

	logger *Logger
}

// NewPage creates a new browser page context
func NewPage(
	ctx context.Context,
	session *Session,
	browserCtx *BrowserContext,
	targetID target.ID,
	opener *Page,
	backgroundPage bool,
	logger *Logger,
) (*Page, error) {
	p := Page{
		BaseEventEmitter: NewBaseEventEmitter(ctx),
		ctx:              ctx,
		session:          session,
		browserCtx:       browserCtx,
		targetID:         targetID,
		opener:           opener,
		backgroundPage:   backgroundPage,
		mediaType:        MediaTypeScreen,
		colorScheme:      browserCtx.opts.ColorScheme,
		reducedMotion:    browserCtx.opts.ReducedMotion,
		extraHTTPHeaders: browserCtx.opts.ExtraHTTPHeaders,
		timeoutSettings:  NewTimeoutSettings(browserCtx.timeoutSettings),
		Keyboard:         NewKeyboard(ctx, session),
		jsEnabled:        true,
		frameSessions:    make(map[cdp.FrameID]*FrameSession),
		workers:          make(map[target.SessionID]*Worker),
		routes:           make([]api.Route, 0),
		logger:           logger,
	}

	p.logger.Debugf("Page:NewPage", "sid:%v tid:%v backgroundPage:%t",
		p.sessionID(), targetID, backgroundPage)

	// We need to init viewport and screen size before initializing the main frame session,
	// as that's where the emulation is activated.
	if browserCtx.opts.Viewport != nil {
		p.emulatedSize = NewEmulatedSize(browserCtx.opts.Viewport, browserCtx.opts.Screen)
	}

	var err error
	p.frameManager = NewFrameManager(ctx, session, &p, browserCtx.timeoutSettings, p.logger)
	p.mainFrameSession, err = NewFrameSession(ctx, session, &p, nil, targetID, p.logger)
	if err != nil {
		p.logger.Debugf("Page:NewPage:NewFrameSession:return", "sid:%v tid:%v err:%v",
			p.sessionID(), targetID, err)

		return nil, err
	}
	p.frameSessions[cdp.FrameID(targetID)] = p.mainFrameSession
	p.Mouse = NewMouse(ctx, session, p.frameManager.MainFrame(), browserCtx.timeoutSettings, p.Keyboard)
	p.Touchscreen = NewTouchscreen(ctx, session, p.Keyboard)

	action := target.SetAutoAttach(true, true).WithFlatten(true)
	if err := action.Do(cdp.WithExecutor(p.ctx, p.session)); err != nil {
		return nil, fmt.Errorf("cannot execute %T: %w", action, err)
	}

	return &p, nil
}

func (p *Page) closeWorker(sessionID target.SessionID) {
	p.logger.Debugf("Page:closeWorker", "sid:%v", sessionID)

	if worker, ok := p.workers[sessionID]; ok {
		worker.didClose()
		delete(p.workers, sessionID)
	}
}

func (p *Page) defaultTimeout() time.Duration {
	return time.Duration(p.timeoutSettings.timeout()) * time.Second
}

func (p *Page) didClose() {
	p.logger.Debugf("Page:didClose", "sid:%v", p.sessionID())

	p.closedMu.Lock()
	{
		p.closed = true
	}
	p.closedMu.Unlock()

	p.emit(EventPageClose, p)
}

func (p *Page) didCrash() {
	p.logger.Debugf("Page:didCrash", "sid:%v", p.sessionID())

	p.frameManager.dispose()
	p.emit(EventPageCrash, p)
}

func (p *Page) evaluateOnNewDocument(source string) {
	// TODO: implement
}

func (p *Page) getFrameElement(f *Frame) (handle *ElementHandle, _ error) {
	if f == nil {
		p.logger.Debugf("Page:getFrameElement", "sid:%v frame:nil", p.sessionID())
	} else {
		p.logger.Debugf("Page:getFrameElement", "sid:%v fid:%s furl:%s",
			p.sessionID(), f.ID(), f.URL())
	}

	parent := f.parentFrame
	if parent == nil {
		return nil, errors.New("frame has been detached 1")
	}

	parentSession := p.getFrameSession(cdp.FrameID(parent.ID()))
	action := dom.GetFrameOwner(cdp.FrameID(f.ID()))
	backendNodeId, _, err := action.Do(cdp.WithExecutor(p.ctx, parentSession.session))
	if err != nil {
		if strings.Contains(err.Error(), "frame with the given id was not found") {
			return nil, errors.New("frame has been detached")
		}
		return nil, fmt.Errorf("unable to get frame owner: %w", err)
	}

	parent = f.parentFrame
	if parent == nil {
		return nil, errors.New("frame has been detached 2")
	}
	return parent.adoptBackendNodeID(mainWorld, backendNodeId)
}

func (p *Page) getOwnerFrame(apiCtx context.Context, h *ElementHandle) cdp.FrameID {
	p.logger.Debugf("Page:getOwnerFrame", "sid:%v", p.sessionID())

	// document.documentElement has frameId of the owner frame
	rt := k6common.GetRuntime(p.ctx)
	pageFn := rt.ToValue(`
		node => {
			const doc = node;
      		if (doc.documentElement && doc.documentElement.ownerDocument === doc)
        		return doc.documentElement;
      		return node.ownerDocument ? node.ownerDocument.documentElement : null;
		}
	`)

	opts := evalOptions{
		forceCallable: true,
		returnByValue: false,
	}
	result, err := h.execCtx.eval(apiCtx, opts, pageFn, []goja.Value{rt.ToValue(h)}...)
	if err != nil {
		p.logger.Debugf("Page:getOwnerFrame:return", "sid:%v err:%v", p.sessionID(), err)
		return ""
	}
	switch result.(type) {
	case nil:
		p.logger.Debugf("Page:getOwnerFrame:return", "sid:%v result:nil", p.sessionID())
		return ""
	}

	documentElement := result.(*ElementHandle)
	if documentElement == nil {
		p.logger.Debugf("Page:getOwnerFrame:return", "sid:%v docel:nil", p.sessionID())
		return ""
	}
	if documentElement.remoteObject.ObjectID == "" {
		p.logger.Debugf("Page:getOwnerFrame:return", "sid:%v robjid:%q", p.sessionID(), "")
		return ""
	}

	action := dom.DescribeNode().WithObjectID(documentElement.remoteObject.ObjectID)
	node, err := action.Do(cdp.WithExecutor(p.ctx, p.session))
	if err != nil {
		p.logger.Debugf("Page:getOwnerFrame:DescribeNode:return", "sid:%v err:%v", p.sessionID(), err)
		return ""
	}

	frameID := node.FrameID
	documentElement.Dispose()
	return frameID
}

func (p *Page) attachFrameSession(fid cdp.FrameID, fs *FrameSession) {
	p.logger.Debugf("Page:attachFrameSession", "sid:%v fid=%v", p.session.id, fid)
	fs.page.frameSessions[fid] = fs
}

func (p *Page) getFrameSession(frameID cdp.FrameID) *FrameSession {
	p.logger.Debugf("Page:getFrameSession", "sid:%v fid:%v", p.sessionID(), frameID)

	return p.frameSessions[frameID]
}

func (p *Page) hasRoutes() bool {
	return len(p.routes) > 0
}

func (p *Page) resetViewport() error {
	p.logger.Debugf("Page:resetViewport", "sid:%v", p.sessionID())

	action := emulation.SetDeviceMetricsOverride(0, 0, 0, false)
	return action.Do(cdp.WithExecutor(p.ctx, p.session))
}

func (p *Page) setEmulatedSize(emulatedSize *EmulatedSize) error {
	p.logger.Debugf("Page:setEmulatedSize", "sid:%v", p.sessionID())

	p.emulatedSize = emulatedSize
	return p.mainFrameSession.updateViewport()
}

func (p *Page) setViewportSize(viewportSize *Size) error {
	p.logger.Debugf("Page:setViewportSize", "sid:%v vps:%v",
		p.sessionID(), viewportSize)

	viewport := &Viewport{
		Width:  int64(viewportSize.Width),
		Height: int64(viewportSize.Height),
	}
	screen := &Screen{
		Width:  int64(viewportSize.Width),
		Height: int64(viewportSize.Height),
	}
	return p.setEmulatedSize(NewEmulatedSize(viewport, screen))
}

func (p *Page) updateExtraHTTPHeaders() {
	p.logger.Debugf("Page:updateExtraHTTPHeaders", "sid:%v", p.sessionID())

	for _, fs := range p.frameSessions {
		fs.updateExtraHTTPHeaders(false)
	}
}

func (p *Page) updateGeolocation() error {
	p.logger.Debugf("Page:updateGeolocation", "sid:%v", p.sessionID())

	for _, fs := range p.frameSessions {
		p.logger.Debugf("Page:updateGeolocation:frameSession",
			"sid:%v tid:%v wid:%v",
			p.sessionID(), fs.targetID, fs.windowID)

		if err := fs.updateGeolocation(false); err != nil {
			p.logger.Debugf("Page:updateGeolocation:frameSession:return",
				"sid:%v tid:%v wid:%v err:%v",
				p.sessionID(), fs.targetID, fs.windowID, err)

			return err
		}
	}
	return nil
}

func (p *Page) updateOffline() {
	p.logger.Debugf("Page:updateOffline", "sid:%v", p.sessionID())

	for _, fs := range p.frameSessions {
		fs.updateOffline(false)
	}
}

func (p *Page) updateHttpCredentials() {
	p.logger.Debugf("Page:updateHttpCredentials", "sid:%v", p.sessionID())

	for _, fs := range p.frameSessions {
		fs.updateHttpCredentials(false)
	}
}

func (p *Page) viewportSize() Size {
	return Size{
		Width:  float64(p.emulatedSize.Viewport.Width),
		Height: float64(p.emulatedSize.Viewport.Height),
	}
}

// AddInitScript adds script to run in all new frames
func (p *Page) AddInitScript(script goja.Value, arg goja.Value) {
	rt := k6common.GetRuntime(p.ctx)
	k6common.Throw(rt, errors.New("Page.addInitScript(script, arg) has not been implemented yet"))
}

func (p *Page) AddScriptTag(opts goja.Value) {
	rt := k6common.GetRuntime(p.ctx)
	k6common.Throw(rt, errors.New("Page.addScriptTag(opts) has not been implemented yet"))
}

func (p *Page) AddStyleTag(opts goja.Value) {
	rt := k6common.GetRuntime(p.ctx)
	k6common.Throw(rt, errors.New("Page.addStyleTag(opts) has not been implemented yet"))
}

// BringToFront activates the browser tab for this page
func (p *Page) BringToFront() {
	p.logger.Debugf("Page:BringToFront", "sid:%v", p.sessionID())

	rt := k6common.GetRuntime(p.ctx)
	action := cdppage.BringToFront()
	if err := action.Do(cdp.WithExecutor(p.ctx, p.session)); err != nil {
		k6common.Throw(rt, fmt.Errorf("unable to bring page to front: %w", err))
	}
}

// Check checks an element matching provided selector
func (p *Page) Check(selector string, opts goja.Value) {
	p.logger.Debugf("Page:Check", "sid:%v selector:%s", p.sessionID(), selector)

	p.MainFrame().Check(selector, opts)
}

// Click clicks an element matching provided selector
func (p *Page) Click(selector string, opts goja.Value) {
	p.logger.Debugf("Page:Click", "sid:%v selector:%s", p.sessionID(), selector)

	p.MainFrame().Click(selector, opts)
}

// Close closes the page
func (p *Page) Close(opts goja.Value) {
	p.logger.Debugf("Page:Close", "sid:%v", p.sessionID())

	p.browserCtx.Close()
}

// Content returns the HTML content of the page
func (p *Page) Content() string {
	p.logger.Debugf("Page:Content", "sid:%v", p.sessionID())

	return p.MainFrame().Content()
}

// Context closes the page
func (p *Page) Context() api.BrowserContext {
	return p.browserCtx
}

// Dblclick double clicks an element matching provided selector
func (p *Page) Dblclick(selector string, opts goja.Value) {
	p.logger.Debugf("Page:Dblclick", "sid:%v selector:%s", p.sessionID(), selector)

	p.MainFrame().Dblclick(selector, opts)
}

func (p *Page) DispatchEvent(selector string, typ string, eventInit goja.Value, opts goja.Value) {
	p.logger.Debugf("Page:DispatchEvent", "sid:%v selector:%s", p.sessionID(), selector)

	p.MainFrame().DispatchEvent(selector, typ, eventInit, opts)
}

func (p *Page) DragAndDrop(source string, target string, opts goja.Value) {
	rt := k6common.GetRuntime(p.ctx)
	k6common.Throw(rt, errors.New("Page.DragAndDrop(source, target, opts) has not been implemented yet"))
}

func (p *Page) EmulateMedia(opts goja.Value) {
	p.logger.Debugf("Page:EmulateMedia", "sid:%v", p.sessionID())

	rt := k6common.GetRuntime(p.ctx)
	parsedOpts := NewPageEmulateMediaOptions(p.mediaType, p.colorScheme, p.reducedMotion)
	if err := parsedOpts.Parse(p.ctx, opts); err != nil {
		k6common.Throw(rt, fmt.Errorf("failed parsing options: %w", err))
	}

	p.mediaType = parsedOpts.Media
	p.colorScheme = parsedOpts.ColorScheme
	p.reducedMotion = parsedOpts.ReducedMotion

	for _, fs := range p.frameSessions {
		if err := fs.updateEmulateMedia(false); err != nil {
			k6common.Throw(rt, err)
		}
	}

	applySlowMo(p.ctx)
}

// EmulateVisionDeficiency activates/deactivates emulation of a vision deficiency
func (p *Page) EmulateVisionDeficiency(typ string) {
	p.logger.Debugf("Page:EmulateVisionDeficiency", "sid:%v typ:%s", p.sessionID(), typ)

	rt := k6common.GetRuntime(p.ctx)
	validTypes := map[string]emulation.SetEmulatedVisionDeficiencyType{
		"achromatopsia": emulation.SetEmulatedVisionDeficiencyTypeAchromatopsia,
		"blurredVision": emulation.SetEmulatedVisionDeficiencyTypeBlurredVision,
		"deuteranopia":  emulation.SetEmulatedVisionDeficiencyTypeDeuteranopia,
		"none":          emulation.SetEmulatedVisionDeficiencyTypeNone,
		"protanopia":    emulation.SetEmulatedVisionDeficiencyTypeProtanopia,
		"tritanopia":    emulation.SetEmulatedVisionDeficiencyTypeTritanopia,
	}
	t, ok := validTypes[typ]
	if !ok {
		k6common.Throw(rt, fmt.Errorf("unsupported vision deficiency: '%s'", typ))
	}

	action := emulation.SetEmulatedVisionDeficiency(t)
	if err := action.Do(cdp.WithExecutor(p.ctx, p.session)); err != nil {
		k6common.Throw(rt, fmt.Errorf("unable to set emulated vision deficiency '%s': %w", typ, err))
	}

	applySlowMo(p.ctx)
}

// Evaluate runs JS code within the execution context of the main frame of the page
func (p *Page) Evaluate(pageFunc goja.Value, args ...goja.Value) interface{} {
	p.logger.Debugf("Page:Evaluate", "sid:%v", p.sessionID())

	return p.MainFrame().Evaluate(pageFunc, args...)
}

func (p *Page) EvaluateHandle(pageFunc goja.Value, args ...goja.Value) api.JSHandle {
	p.logger.Debugf("Page:EvaluateHandle", "sid:%v", p.sessionID())

	return p.MainFrame().EvaluateHandle(pageFunc, args...)
}

func (p *Page) ExposeBinding(name string, callback goja.Callable, opts goja.Value) {
	rt := k6common.GetRuntime(p.ctx)
	k6common.Throw(rt, errors.New("Page.exposeBinding(name, callback) has not been implemented yet"))
}

func (p *Page) ExposeFunction(name string, callback goja.Callable) {
	rt := k6common.GetRuntime(p.ctx)
	k6common.Throw(rt, errors.New("Page.exposeFunction(name, callback) has not been implemented yet"))
}

func (p *Page) Fill(selector string, value string, opts goja.Value) {
	p.logger.Debugf("Page:Fill", "sid:%v selector:%s", p.sessionID(), selector)

	p.MainFrame().Fill(selector, value, opts)
}

func (p *Page) Focus(selector string, opts goja.Value) {
	p.logger.Debugf("Page:Focus", "sid:%v selector:%s", p.sessionID(), selector)

	p.MainFrame().Focus(selector, opts)
}

func (p *Page) Frame(frameSelector goja.Value) api.Frame {
	rt := k6common.GetRuntime(p.ctx)
	k6common.Throw(rt, errors.New("Page.frame(frameSelector) has not been implemented yet"))
	return nil
}

// Frames returns a list of frames on the page
func (p *Page) Frames() []api.Frame {
	return p.frameManager.Frames()
}

func (p *Page) GetAttribute(selector string, name string, opts goja.Value) goja.Value {
	p.logger.Debugf("Page:GetAttribute", "sid:%v selector:%s name:%s",
		p.sessionID(), selector, name)

	return p.MainFrame().GetAttribute(selector, name, opts)
}

func (p *Page) GoBack(opts goja.Value) api.Response {
	rt := k6common.GetRuntime(p.ctx)
	k6common.Throw(rt, errors.New("Page.goBack(opts) has not been implemented yet"))
	return nil
}

func (p *Page) GoForward(opts goja.Value) api.Response {
	rt := k6common.GetRuntime(p.ctx)
	k6common.Throw(rt, errors.New("Page.goForward(opts) has not been implemented yet"))
	return nil
}

// Goto will navigate the page to the specified URL and return a HTTP response object
func (p *Page) Goto(url string, opts goja.Value) api.Response {
	p.logger.Debugf("Page:Goto", "sid:%v url:%q", p.sessionID(), url)

	return p.MainFrame().Goto(url, opts)
}

func (p *Page) Hover(selector string, opts goja.Value) {
	p.logger.Debugf("Page:Hover", "sid:%v selector:%s", p.sessionID(), selector)

	p.MainFrame().Hover(selector, opts)
}

func (p *Page) InnerHTML(selector string, opts goja.Value) string {
	p.logger.Debugf("Page:InnerHTML", "sid:%v selector:%s", p.sessionID(), selector)

	return p.MainFrame().InnerHTML(selector, opts)
}

func (p *Page) InnerText(selector string, opts goja.Value) string {
	p.logger.Debugf("Page:InnerText", "sid:%v selector:%s", p.sessionID(), selector)

	return p.MainFrame().InnerText(selector, opts)
}

func (p *Page) InputValue(selector string, opts goja.Value) string {
	p.logger.Debugf("Page:InputValue", "sid:%v selector:%s", p.sessionID(), selector)

	return p.MainFrame().InputValue(selector, opts)
}

func (p *Page) IsChecked(selector string, opts goja.Value) bool {
	p.logger.Debugf("Page:IsChecked", "sid:%v selector:%s", p.sessionID(), selector)

	return p.MainFrame().IsChecked(selector, opts)
}

func (p *Page) IsClosed() bool {
	p.closedMu.RLock()
	defer p.closedMu.RUnlock()

	return p.closed
}

func (p *Page) IsDisabled(selector string, opts goja.Value) bool {
	p.logger.Debugf("Page:IsDisabled", "sid:%v selector:%s", p.sessionID(), selector)

	return p.MainFrame().IsDisabled(selector, opts)
}

func (p *Page) IsEditable(selector string, opts goja.Value) bool {
	p.logger.Debugf("Page:IsEditable", "sid:%v selector:%s", p.sessionID(), selector)

	return p.MainFrame().IsEditable(selector, opts)
}

func (p *Page) IsEnabled(selector string, opts goja.Value) bool {
	p.logger.Debugf("Page:IsEnabled", "sid:%v selector:%s", p.sessionID(), selector)

	return p.MainFrame().IsEnabled(selector, opts)
}

func (p *Page) IsHidden(selector string, opts goja.Value) bool {
	p.logger.Debugf("Page:IsHidden", "sid:%v selector:%s", p.sessionID(), selector)

	return p.MainFrame().IsHidden(selector, opts)
}

func (p *Page) IsVisible(selector string, opts goja.Value) bool {
	p.logger.Debugf("Page:IsVisible", "sid:%v selector:%s", p.sessionID(), selector)

	return p.MainFrame().IsVisible(selector, opts)
}

// MainFrame returns the main frame on the page
func (p *Page) MainFrame() api.Frame {
	mf := p.frameManager.MainFrame()

	if mf == nil {
		p.logger.Debugf("Page:MainFrame", "sid:%v", p.sessionID())
	} else {
		p.logger.Debugf("Page:MainFrame",
			"sid:%v mfid:%v mflid:%v mfurl:%v",
			p.sessionID(), mf.id, mf.loaderID, mf.url)

	}

	return mf
}

// Opener returns the opener of the target
func (p *Page) Opener() api.Page {
	return p.opener
}

func (p *Page) Pause() {
	rt := k6common.GetRuntime(p.ctx)
	k6common.Throw(rt, errors.New("Page.pause() has not been implemented yet"))
}

func (p *Page) Pdf(opts goja.Value) goja.ArrayBuffer {
	rt := k6common.GetRuntime(p.ctx)
	k6common.Throw(rt, errors.New("Page.pdf(opts) has not been implemented yet"))
	return rt.NewArrayBuffer([]byte{})
}

func (p *Page) Press(selector string, key string, opts goja.Value) {
	p.logger.Debugf("Page:Press", "sid:%v selector:%s", p.sessionID(), selector)

	p.MainFrame().Press(selector, key, opts)
}

func (p *Page) Query(selector string) api.ElementHandle {
	p.logger.Debugf("Page:Query", "sid:%v selector:%s", p.sessionID(), selector)

	return p.frameManager.MainFrame().Query(selector)
}

func (p *Page) QueryAll(selector string) []api.ElementHandle {
	p.logger.Debugf("Page:QueryAll", "sid:%v selector:%s", p.sessionID(), selector)

	return p.frameManager.MainFrame().QueryAll(selector)
}

// Reload will reload the current page
func (p *Page) Reload(opts goja.Value) api.Response {
	p.logger.Debugf("Page:Reload", "sid:%v", p.sessionID())

	rt := k6common.GetRuntime(p.ctx)
	parsedOpts := NewPageReloadOptions(LifecycleEventLoad, p.defaultTimeout())
	if err := parsedOpts.Parse(p.ctx, opts); err != nil {
		k6common.Throw(rt, fmt.Errorf("failed parsing options: %w", err))
	}

	ch, evCancelFn := createWaitForEventHandler(p.ctx, p.frameManager.MainFrame(), []string{EventFrameNavigation}, func(data interface{}) bool {
		return true // Both successful and failed navigations are considered
	})
	defer evCancelFn() // Remove event handler

	action := cdppage.Reload()
	if err := action.Do(cdp.WithExecutor(p.ctx, p.session)); err != nil {
		k6common.Throw(rt, fmt.Errorf("unable to reload page: %w", err))
	}

	var event *NavigationEvent
	select {
	case <-p.ctx.Done():
	case <-time.After(parsedOpts.Timeout):
		k6common.Throw(rt, ErrTimedOut)
	case data := <-ch:
		event = data.(*NavigationEvent)
	}

	if p.frameManager.mainFrame.hasSubtreeLifecycleEventFired(parsedOpts.WaitUntil) {
		_, _ = waitForEvent(p.ctx, p.frameManager.MainFrame(), []string{EventFrameAddLifecycle}, func(data interface{}) bool {
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
	applySlowMo(p.ctx)
	return resp
}

func (p *Page) Route(url goja.Value, handler goja.Callable) {
	rt := k6common.GetRuntime(p.ctx)
	k6common.Throw(rt, errors.New("Page.route(url, handler) has not been implemented yet"))
}

// Screenshot will instruct Chrome to save a screenshot of the current page and save it to specified file
func (p *Page) Screenshot(opts goja.Value) goja.ArrayBuffer {
	rt := k6common.GetRuntime(p.ctx)
	parsedOpts := NewPageScreenshotOptions()
	if err := parsedOpts.Parse(p.ctx, opts); err != nil {
		k6common.Throw(rt, fmt.Errorf("failed parsing options: %w", err))
	}
	s := newScreenshotter(p.ctx)
	buf, err := s.screenshotPage(p, parsedOpts)
	if err != nil {
		k6common.Throw(rt, fmt.Errorf("cannot capture screenshot: %w", err))
	}
	return rt.NewArrayBuffer(*buf)
}

func (p *Page) SelectOption(selector string, values goja.Value, opts goja.Value) []string {
	p.logger.Debugf("Page:SelectOption", "sid:%v selector:%s", p.sessionID(), selector)

	return p.MainFrame().SelectOption(selector, values, opts)
}

func (p *Page) SetContent(html string, opts goja.Value) {
	p.logger.Debugf("Page:SetContent", "sid:%v", p.sessionID())

	p.MainFrame().SetContent(html, opts)
}

// SetDefaultNavigationTimeout sets the default navigation timeout in milliseconds
func (p *Page) SetDefaultNavigationTimeout(timeout int64) {
	p.logger.Debugf("Page:SetDefaultNavigationTimeout", "sid:%v timeout:%d", p.sessionID(), timeout)

	p.timeoutSettings.setDefaultNavigationTimeout(timeout)
}

// SetDefaultTimeout sets the default maximum timeout in milliseconds
func (p *Page) SetDefaultTimeout(timeout int64) {
	p.logger.Debugf("Page:SetDefaultTimeout", "sid:%v timeout:%d", p.sessionID(), timeout)

	p.timeoutSettings.setDefaultTimeout(timeout)
}

// SetExtraHTTPHeaders sets default HTTP headers for page and whole frame hierarchy
func (p *Page) SetExtraHTTPHeaders(headers map[string]string) {
	p.logger.Debugf("Page:SetExtraHTTPHeaders", "sid:%v", p.sessionID())

	p.extraHTTPHeaders = headers
	p.updateExtraHTTPHeaders()
}

func (p *Page) SetInputFiles(selector string, files goja.Value, opts goja.Value) {
	rt := k6common.GetRuntime(p.ctx)
	k6common.Throw(rt, errors.New("Page.textContent(selector, opts) has not been implemented yet"))
	// TODO: needs slowMo
}

// SetViewportSize will update the viewport width and height
func (p *Page) SetViewportSize(viewportSize goja.Value) {
	p.logger.Debugf("Page:SetViewportSize", "sid:%v", p.sessionID())

	rt := k6common.GetRuntime(p.ctx)
	s := &Size{}
	if err := s.Parse(p.ctx, viewportSize); err != nil {
		k6common.Throw(rt, fmt.Errorf("failed parsing size: %w", err))
	}
	if err := p.setViewportSize(s); err != nil {
		k6common.Throw(rt, err)
	}
	applySlowMo(p.ctx)
}

func (p *Page) Tap(selector string, opts goja.Value) {
	p.logger.Debugf("Page:SetViewportSize", "sid:%v selector:%s", p.sessionID(), selector)

	p.MainFrame().Tap(selector, opts)
}

func (p *Page) TextContent(selector string, opts goja.Value) string {
	p.logger.Debugf("Page:TextContent", "sid:%v selector:%s", p.sessionID(), selector)

	return p.MainFrame().TextContent(selector, opts)
}

func (p *Page) Title() string {
	p.logger.Debugf("Page:Title", "sid:%v", p.sessionID())

	rt := k6common.GetRuntime(p.ctx)
	js := `() => document.title`
	return p.Evaluate(rt.ToValue(js)).(goja.Value).String()
}

func (p *Page) Type(selector string, text string, opts goja.Value) {
	p.logger.Debugf("Page:Type", "sid:%v selector:%s text:%s", p.sessionID(), selector, text)

	p.MainFrame().Type(selector, text, opts)
}

func (p *Page) Uncheck(selector string, opts goja.Value) {
	p.logger.Debugf("Page:Uncheck", "sid:%v selector:%s", p.sessionID(), selector)

	p.MainFrame().Uncheck(selector, opts)
}

func (p *Page) Unroute(url goja.Value, handler goja.Callable) {
	rt := k6common.GetRuntime(p.ctx)
	k6common.Throw(rt, errors.New("Page.unroute(url, handler) has not been implemented yet"))
}

// URL returns the location of the page
func (p *Page) URL() string {
	rt := k6common.GetRuntime(p.ctx)
	return p.Evaluate(rt.ToValue("document.location.toString()")).(string)
}

// Video returns information of recorded video
func (p *Page) Video() api.Video {
	rt := k6common.GetRuntime(p.ctx)
	k6common.Throw(rt, errors.New("Page.video() has not been implemented yet"))
	return nil
}

// ViewportSize will return information on the viewport width and height
func (p *Page) ViewportSize() map[string]float64 {
	p.logger.Debugf("Page:ViewportSize", "sid:%v", p.sessionID())

	vps := p.viewportSize()
	return map[string]float64{
		"width":  vps.Width,
		"height": vps.Height,
	}
}

// WaitForEvent waits for the specified event to trigger
func (p *Page) WaitForEvent(event string, optsOrPredicate goja.Value) interface{} {
	rt := k6common.GetRuntime(p.ctx)
	k6common.Throw(rt, errors.New("Page.waitForEvent(event, optsOrPredicate) has not been implemented yet"))
	return nil
}

// WaitForFunction waits for the given predicate to return a truthy value
func (p *Page) WaitForFunction(pageFunc goja.Value, arg goja.Value, opts goja.Value) api.JSHandle {
	p.logger.Debugf("Page:WaitForFunction", "sid:%v", p.sessionID())

	return p.frameManager.MainFrame().WaitForFunction(pageFunc, opts, arg)
}

// WaitForLoadState waits for the specified page life cycle event
func (p *Page) WaitForLoadState(state string, opts goja.Value) {
	p.logger.Debugf("Page:WaitForLoadState", "sid:%v state:%q", p.sessionID(), state)

	p.frameManager.MainFrame().WaitForLoadState(state, opts)
}

// WaitForNavigation waits for the given navigation lifecycle event to happen
func (p *Page) WaitForNavigation(opts goja.Value) api.Response {
	p.logger.Debugf("Page:WaitForNavigation", "sid:%v", p.sessionID())

	return p.frameManager.MainFrame().WaitForNavigation(opts)
}

func (p *Page) WaitForRequest(urlOrPredicate, opts goja.Value) api.Request {
	rt := k6common.GetRuntime(p.ctx)
	k6common.Throw(rt, errors.New("Page.waitForRequest(urlOrPredicate, opts) has not been implemented yet"))
	return nil
}

func (p *Page) WaitForResponse(urlOrPredicate, opts goja.Value) api.Response {
	rt := k6common.GetRuntime(p.ctx)
	k6common.Throw(rt, errors.New("Page.waitForResponse(urlOrPredicate, opts) has not been implemented yet"))
	return nil
}

// WaitForSelector waits for the given selector to match the waiting criteria
func (p *Page) WaitForSelector(selector string, opts goja.Value) api.ElementHandle {
	p.logger.Debugf("Page:WaitForSelector",
		"sid:%v stid:%v ptid:%v selector:%s",
		p.sessionID(), p.session.targetID, p.targetID, selector)

	return p.frameManager.MainFrame().WaitForSelector(selector, opts)
}

// WaitForTimeout waits the specified number of milliseconds
func (p *Page) WaitForTimeout(timeout int64) {
	p.logger.Debugf("Page:WaitForTimeout", "sid:%v timeout:%d", p.sessionID(), timeout)

	p.frameManager.MainFrame().WaitForTimeout(timeout)
}

// Workers returns all WebWorkers of page
func (p *Page) Workers() []api.Worker {
	workers := make([]api.Worker, 0, len(p.workers))
	for _, w := range p.workers {
		workers = append(workers, w)
	}
	return workers
}

// sessionID returns the Page's session ID.
// It should be used internally in the Page.
func (p *Page) sessionID() (sid target.SessionID) {
	if p != nil && p.session != nil {
		sid = p.session.id
	}
	return sid
}
