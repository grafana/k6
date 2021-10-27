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
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/emulation"
	cdppage "github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/target"
	"github.com/dop251/goja"
	"github.com/grafana/xk6-browser/api"
	"github.com/pkg/errors"
	k6common "go.k6.io/k6/js/common"
	"golang.org/x/net/context"
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
	viewport        *Viewport
	timeoutSettings *TimeoutSettings

	jsEnabled        bool
	closed           bool
	backgroundPage   bool
	mediaType        MediaType
	colorScheme      ColorScheme
	reducedMotion    ReducedMotion
	extraHTTPHeaders map[string]string
	emulatedSize     *EmulatedSize

	mainFrameSession *FrameSession
	frameSessions    map[cdp.FrameID]*FrameSession
	workers          map[target.SessionID]*Worker
	routes           []api.Route
}

// NewPage creates a new browser page context
func NewPage(ctx context.Context, session *Session, browserCtx *BrowserContext, targetID target.ID, opener *Page, backgroundPage bool) (*Page, error) {
	p := Page{
		BaseEventEmitter: NewBaseEventEmitter(ctx),
		ctx:              ctx,
		session:          session,
		browserCtx:       browserCtx,
		targetID:         targetID,
		opener:           opener,
		closed:           false,
		backgroundPage:   backgroundPage,
		mediaType:        MediaTypeScreen,
		colorScheme:      browserCtx.opts.ColorScheme,
		reducedMotion:    browserCtx.opts.ReducedMotion,
		extraHTTPHeaders: browserCtx.opts.ExtraHTTPHeaders,
		emulatedSize:     nil,
		frameManager:     nil,
		viewport:         nil,
		timeoutSettings:  NewTimeoutSettings(browserCtx.timeoutSettings),
		Keyboard:         NewKeyboard(ctx, session),
		Mouse:            nil,
		Touchscreen:      nil,
		jsEnabled:        true,
		mainFrameSession: nil,
		frameSessions:    make(map[cdp.FrameID]*FrameSession),
		workers:          make(map[target.SessionID]*Worker),
		routes:           make([]api.Route, 0),
	}

	var err error
	p.frameManager = NewFrameManager(p.ctx, session, &p, browserCtx.timeoutSettings)
	p.mainFrameSession, err = NewFrameSession(ctx, session, &p, nil, targetID)
	if err != nil {
		return nil, err
	}
	p.frameSessions[cdp.FrameID(targetID)] = p.mainFrameSession
	p.Mouse = NewMouse(p.ctx, session, p.frameManager.MainFrame(), browserCtx.timeoutSettings, p.Keyboard)
	p.Touchscreen = NewTouchscreen(p.ctx, session, p.Keyboard)

	if browserCtx.opts.Viewport != nil {
		p.emulatedSize = NewEmulatedSize(browserCtx.opts.Viewport, browserCtx.opts.Screen)
	}

	if err := p.initEvents(); err != nil {
		return nil, err
	}

	return &p, nil
}

func (p *Page) initEvents() error {
	action := target.SetAutoAttach(true, true).WithFlatten(true)
	if err := action.Do(cdp.WithExecutor(p.ctx, p.session)); err != nil {
		return fmt.Errorf("unable to execute %T: %v", action, err)
	}
	return nil
}

func (p *Page) closeWorker(sessionID target.SessionID) {
	if worker, ok := p.workers[sessionID]; ok {
		worker.didClose()
		delete(p.workers, sessionID)
	}
}

func (p *Page) defaultTimeout() time.Duration {
	return time.Duration(p.timeoutSettings.timeout()) * time.Second
}

func (p *Page) didClose() {
	p.closed = true
	p.emit(EventPageClose, p)
}

func (p *Page) didCrash() {
	p.frameManager.dispose()
	p.emit(EventPageCrash, p)
}

func (p *Page) evaluateOnNewDocument(source string) {
	// TODO: implement
}

func (p *Page) getFrameElement(f *Frame) (*ElementHandle, error) {
	parent := f.parentFrame
	if parent == nil {
		return nil, errors.Errorf("frame has been detached 1")
	}

	parentSession := p.getFrameSession(parent.id)
	action := dom.GetFrameOwner(f.id)
	backendNodeId, _, err := action.Do(cdp.WithExecutor(p.ctx, parentSession.session))
	if err != nil {
		if strings.Contains(err.Error(), "frame with the given id was not found") {
			return nil, fmt.Errorf("frame has been detached")
		}
		return nil, fmt.Errorf("unable to get frame owner: %w", err)
	}

	parent = f.parentFrame
	if parent == nil {
		return nil, errors.Errorf("frame has been detached 2")
	}
	handle, err := parent.mainExecutionContext.adoptBackendNodeId(backendNodeId)
	return handle, err
}

func (p *Page) getOwnerFrame(apiCtx context.Context, h *ElementHandle) cdp.FrameID {
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
	result, err := h.execCtx.evaluate(apiCtx, true, false, pageFn, []goja.Value{rt.ToValue(h)}...)
	switch result.(type) {
	case nil:
		return ""
	}

	documentElement := result.(*ElementHandle)
	if documentElement == nil {
		return ""
	}
	if documentElement.remoteObject.ObjectID == "" {
		return ""
	}

	action := dom.DescribeNode().WithObjectID(documentElement.remoteObject.ObjectID)
	node, err := action.Do(cdp.WithExecutor(p.ctx, p.session))
	if err != nil {
		return ""
	}

	frameID := node.FrameID
	documentElement.Dispose()
	return frameID
}

func (p *Page) getFrameSession(frameID cdp.FrameID) *FrameSession {
	return p.frameSessions[frameID]
}

func (p *Page) hasRoutes() bool {
	return len(p.routes) > 0
}

func (p *Page) updateExtraHTTPHeaders() {
	for _, fs := range p.frameSessions {
		fs.updateExtraHTTPHeaders(false)
	}
}

func (p *Page) updateGeolocation() error {
	for _, fs := range p.frameSessions {
		if err := fs.updateGeolocation(false); err != nil {
			return err
		}
	}
	return nil
}

func (p *Page) updateOffline() {
	for _, fs := range p.frameSessions {
		fs.updateOffline(false)
	}
}

func (p *Page) updateHttpCredentials() {
	for _, fs := range p.frameSessions {
		fs.updateHttpCredentials(false)
	}
}

func (p *Page) setEmulatedSize(emulatedSize *EmulatedSize) error {
	return p.mainFrameSession.updateViewport()
}

// AddInitScript adds script to run in all new frames
func (p *Page) AddInitScript(script goja.Value, arg goja.Value) {
	rt := k6common.GetRuntime(p.ctx)
	k6common.Throw(rt, errors.Errorf("Page.addInitScript(script, arg) has not been implemented yet!"))
}

func (p *Page) AddScriptTag(opts goja.Value) {
	rt := k6common.GetRuntime(p.ctx)
	k6common.Throw(rt, errors.Errorf("Page.addScriptTag(opts) has not been implemented yet!"))
}

func (p *Page) AddStyleTag(opts goja.Value) {
	rt := k6common.GetRuntime(p.ctx)
	k6common.Throw(rt, errors.Errorf("Page.addStyleTag(opts) has not been implemented yet!"))
}

// BrintToFront activates the browser tab for this page
func (p *Page) BrintToFront() {
	rt := k6common.GetRuntime(p.ctx)
	action := cdppage.BringToFront()
	if err := action.Do(cdp.WithExecutor(p.ctx, p.session)); err != nil {
		k6common.Throw(rt, fmt.Errorf("unable to bring page to front: %w", err))
	}
}

// Check checks an element matching provided selector
func (p *Page) Check(selector string, opts goja.Value) {
	p.MainFrame().Check(selector, opts)
}

// Click clicks an element matching provided selector
func (p *Page) Click(selector string, opts goja.Value) {
	p.MainFrame().Click(selector, opts)
}

// Close closes the page
func (p *Page) Close(opts goja.Value) {
	p.browserCtx.Close()
}

// Content returns the HTML content of the page
func (p *Page) Content() string {
	return p.MainFrame().Content()
}

// Context closes the page
func (p *Page) Context() api.BrowserContext {
	return p.browserCtx
}

// Dblclick double clicks an element matching provided selector
func (p *Page) Dblclick(selector string, opts goja.Value) {
	p.MainFrame().Dblclick(selector, opts)
}

func (p *Page) DispatchEvent(selector string, typ string, eventInit goja.Value, opts goja.Value) {
	p.MainFrame().DispatchEvent(selector, typ, eventInit, opts)
}

func (p *Page) DragAndDrop(source string, target string, opts goja.Value) {
	rt := k6common.GetRuntime(p.ctx)
	k6common.Throw(rt, errors.Errorf("Page.DragAndDrop(source, target, opts) has not been implemented yet!"))
}

func (p *Page) EmulateMedia(opts goja.Value) {
	rt := k6common.GetRuntime(p.ctx)
	parsedOpts := NewPageEmulateMediaOptions(p.mediaType, p.colorScheme, p.reducedMotion)
	if err := parsedOpts.Parse(p.ctx, opts); err != nil {
		k6common.Throw(rt, fmt.Errorf("failed parsing options: %v", err))
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
	return p.MainFrame().Evaluate(pageFunc, args...)
}

func (p *Page) EvaluateHandle(pageFunc goja.Value, args ...goja.Value) api.JSHandle {
	return p.MainFrame().EvaluateHandle(pageFunc, args...)
}

func (p *Page) ExposeBinding(name string, callback goja.Callable, opts goja.Value) {
	rt := k6common.GetRuntime(p.ctx)
	k6common.Throw(rt, errors.Errorf("Page.exposeBinding(name, callback) has not been implemented yet!"))
}

func (p *Page) ExposeFunction(name string, callback goja.Callable) {
	rt := k6common.GetRuntime(p.ctx)
	k6common.Throw(rt, errors.Errorf("Page.exposeFunction(name, callback) has not been implemented yet!"))
}

func (p *Page) Fill(selector string, value string, opts goja.Value) {
	p.MainFrame().Fill(selector, value, opts)
}

func (p *Page) Focus(selector string, opts goja.Value) {
	p.MainFrame().Focus(selector, opts)
}

func (p *Page) Frame(frameSelector goja.Value) api.Frame {
	rt := k6common.GetRuntime(p.ctx)
	k6common.Throw(rt, errors.Errorf("Page.frame(frameSelector) has not been implemented yet!"))
	return nil
}

// Frames returns a list of frames on the page
func (p *Page) Frames() []api.Frame {
	return p.frameManager.Frames()
}

func (p *Page) GetAttribute(selector string, name string, opts goja.Value) goja.Value {
	return p.MainFrame().GetAttribute(selector, name, opts)
}

func (p *Page) GoBack(opts goja.Value) api.Response {
	rt := k6common.GetRuntime(p.ctx)
	k6common.Throw(rt, errors.Errorf("Page.goBack(opts) has not been implemented yet!"))
	return nil
}

func (p *Page) GoForward(opts goja.Value) api.Response {
	rt := k6common.GetRuntime(p.ctx)
	k6common.Throw(rt, errors.Errorf("Page.goForward(opts) has not been implemented yet!"))
	return nil
}

// Goto will navigate the page to the specified URL and return a HTTP response object
func (p *Page) Goto(url string, opts goja.Value) api.Response {
	return p.MainFrame().Goto(url, opts)
}

func (p *Page) Hover(selector string, opts goja.Value) {
	p.MainFrame().Hover(selector, opts)
}

func (p *Page) InnerHTML(selector string, opts goja.Value) string {
	return p.MainFrame().InnerHTML(selector, opts)
}

func (p *Page) InnerText(selector string, opts goja.Value) string {
	return p.MainFrame().InnerText(selector, opts)
}

func (p *Page) InputValue(selector string, opts goja.Value) string {
	return p.MainFrame().InputValue(selector, opts)
}

func (p *Page) IsChecked(selector string, opts goja.Value) bool {
	return p.MainFrame().IsChecked(selector, opts)
}

func (p *Page) IsClosed() bool {
	return p.closed
}

func (p *Page) IsDisabled(selector string, opts goja.Value) bool {
	return p.MainFrame().IsDisabled(selector, opts)
}

func (p *Page) IsEditable(selector string, opts goja.Value) bool {
	return p.MainFrame().IsEditable(selector, opts)
}

func (p *Page) IsEnabled(selector string, opts goja.Value) bool {
	return p.MainFrame().IsEnabled(selector, opts)
}

func (p *Page) IsHidden(selector string, opts goja.Value) bool {
	return p.MainFrame().IsHidden(selector, opts)
}

func (p *Page) IsVisible(selector string, opts goja.Value) bool {
	return p.MainFrame().IsVisible(selector, opts)
}

// MainFrame returns the main frame on the page
func (p *Page) MainFrame() api.Frame {
	return p.frameManager.MainFrame()
}

// Opener returns the opener of the target
func (p *Page) Opener() api.Page {
	return p.opener
}

func (p *Page) Pause() {
	rt := k6common.GetRuntime(p.ctx)
	k6common.Throw(rt, errors.Errorf("Page.pause() has not been implemented yet!"))
}

func (p *Page) Pdf(opts goja.Value) goja.ArrayBuffer {
	rt := k6common.GetRuntime(p.ctx)
	k6common.Throw(rt, errors.Errorf("Page.pdf(opts) has not been implemented yet!"))
	return rt.NewArrayBuffer([]byte{})
}

func (p *Page) Press(selector string, key string, opts goja.Value) {
	p.MainFrame().Press(selector, key, opts)
}

func (p *Page) Query(selector string) api.ElementHandle {
	return p.frameManager.MainFrame().Query(selector)
}

func (p *Page) QueryAll(selector string) []api.ElementHandle {
	return p.frameManager.MainFrame().QueryAll(selector)
}

// Reload will reload the current page
func (p *Page) Reload(opts goja.Value) api.Response {
	rt := k6common.GetRuntime(p.ctx)
	parsedOpts := NewPageReloadOptions(LifecycleEventLoad, p.defaultTimeout())
	if err := parsedOpts.Parse(p.ctx, opts); err != nil {
		k6common.Throw(rt, fmt.Errorf("failed parsing options: %w", err))
	}

	ch, evCancelFn := createWaitForEventHandler(p.ctx, p.frameManager.mainFrame, []string{EventFrameNavigation}, func(data interface{}) bool {
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
		waitForEvent(p.ctx, p.frameManager.mainFrame, []string{EventFrameAddLifecycle}, func(data interface{}) bool {
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
	k6common.Throw(rt, errors.Errorf("Page.route(url, handler) has not been implemented yet!"))
}

// Screenshot will instruct Chrome to save a screenshot of the current page and save it to specified file
func (p *Page) Screenshot(opts goja.Value) goja.ArrayBuffer {
	rt := k6common.GetRuntime(p.ctx)
	parsedOpts := NewPageScreenshotOptions()
	if err := parsedOpts.Parse(p.ctx, opts); err != nil {
		k6common.Throw(rt, fmt.Errorf("failed parsing options: %w", err))
	}

	var buf []byte
	clip := parsedOpts.Clip
	format := parsedOpts.Format

	// Infer file format by path
	if parsedOpts.Path != "" && parsedOpts.Format != "png" && parsedOpts.Format != "jpeg" {
		if strings.HasSuffix(parsedOpts.Path, ".jpg") || strings.HasSuffix(parsedOpts.Path, ".jpeg") {
			format = "jpeg"
		}
	}

	var capture *cdppage.CaptureScreenshotParams
	capture = cdppage.CaptureScreenshot()

	// Setup viewport or full page screenshot capture based on options
	if parsedOpts.Clip.Width > 0 && parsedOpts.Clip.Height > 0 {
		_, _, contentSize, err := cdppage.GetLayoutMetrics().Do(cdp.WithExecutor(p.ctx, p.session))
		if err != nil {
			k6common.Throw(rt, fmt.Errorf("unable to get layout metrics: %w", err))
		}
		width, height := int64(math.Ceil(contentSize.Width)), int64(math.Ceil(contentSize.Height))
		action := emulation.SetDeviceMetricsOverride(width, height, 1, false).
			WithScreenOrientation(&emulation.ScreenOrientation{
				Type:  emulation.OrientationTypePortraitPrimary,
				Angle: 0,
			})
		if err = action.Do(cdp.WithExecutor(p.ctx, p.session)); err != nil {
			k6common.Throw(rt, fmt.Errorf("unable to set screen width and height: %w", err))
		}
		clip = cdppage.Viewport{
			X:      contentSize.X,
			Y:      contentSize.Y,
			Width:  contentSize.Width,
			Height: contentSize.Height,
			Scale:  1,
		}
	}

	if clip.Width > 0 && clip.Height > 0 {
		capture = capture.WithClip(&clip)
	}

	// Add common options
	capture.WithQuality(parsedOpts.Quality)
	switch format {
	case "jpeg":
		capture.WithFormat(cdppage.CaptureScreenshotFormatJpeg)
	default:
		capture.WithFormat(cdppage.CaptureScreenshotFormatPng)
	}

	// Make background transparent for PNG captures if requested
	if parsedOpts.OmitBackground && format == "png" {
		action := emulation.SetDefaultBackgroundColorOverride().
			WithColor(&cdp.RGBA{R: 0, G: 0, B: 0, A: 0})
		if err := action.Do(cdp.WithExecutor(p.ctx, p.session)); err != nil {
			k6common.Throw(rt, fmt.Errorf("unable to set screenshot background transparency: %w", err))
		}
	}

	// Capture screenshot
	buf, err := capture.Do(cdp.WithExecutor(p.ctx, p.session))
	if err != nil {
		k6common.Throw(rt, fmt.Errorf("unable to capture screenshot of page '%s': %w", p.frameManager.MainFrame().URL(), err))
	}

	// Reset background
	if parsedOpts.OmitBackground && format == "png" {
		action := emulation.SetDefaultBackgroundColorOverride()
		if err := action.Do(cdp.WithExecutor(p.ctx, p.session)); err != nil {
			k6common.Throw(rt, fmt.Errorf("unable to reset screenshot background color: %w", err))
		}
	}

	// TODO: Reset viewport

	// Save screenshot capture to file
	// TODO: we should not write to disk here but put it on some queue for async disk writes
	if parsedOpts.Path != "" {
		dir := filepath.Dir(parsedOpts.Path)
		if err := os.MkdirAll(dir, 0775); err != nil {
			k6common.Throw(rt, fmt.Errorf("unable to create directory for screenshot of page '%s': %w", p.frameManager.MainFrame().URL(), err))
		}
		if err := ioutil.WriteFile(parsedOpts.Path, buf, 0664); err != nil {
			k6common.Throw(rt, fmt.Errorf("unable to save screenshot of page '%s' to file: %w", p.frameManager.MainFrame().URL(), err))
		}
	}

	return rt.NewArrayBuffer(buf)
}

func (p *Page) SelectOption(selector string, values goja.Value, opts goja.Value) []string {
	return p.MainFrame().SelectOption(selector, values, opts)
}

func (p *Page) SetContent(html string, opts goja.Value) {
	p.MainFrame().SetContent(html, opts)
}

// SetDefaultNavigationTimeout sets the default navigation timeout in milliseconds
func (p *Page) SetDefaultNavigationTimeout(timeout int64) {
	p.timeoutSettings.setDefaultNavigationTimeout(timeout)
}

// SetDefaultTimeout sets the default maximum timeout in milliseconds
func (p *Page) SetDefaultTimeout(timeout int64) {
	p.timeoutSettings.setDefaultTimeout(timeout)
}

// SetExtraHTTPHeaders sets default HTTP headers for page and whole frame hierarchy
func (p *Page) SetExtraHTTPHeaders(headers map[string]string) {
	p.extraHTTPHeaders = headers
	p.updateHttpCredentials()
}

func (p *Page) SetInputFiles(selector string, files goja.Value, opts goja.Value) {
	rt := k6common.GetRuntime(p.ctx)
	k6common.Throw(rt, errors.Errorf("Page.textContent(selector, opts) has not been implemented yet!"))
	// TODO: needs slowMo
}

// SetViewportSize will update the viewport width and height
func (p *Page) SetViewportSize(viewportSize goja.Value) {
	rt := k6common.GetRuntime(p.ctx)
	viewport := NewViewport()
	if err := viewport.Parse(p.ctx, viewportSize); err != nil {
		k6common.Throw(rt, fmt.Errorf("failed parsing viewport: %w", err))
	}
	screen := NewScreen()
	if err := viewport.Parse(p.ctx, viewportSize); err != nil {
		k6common.Throw(rt, fmt.Errorf("failed parsing screen: %w", err))
	}
	if err := p.setEmulatedSize(NewEmulatedSize(viewport, screen)); err != nil {
		k6common.Throw(rt, err)
	}
	applySlowMo(p.ctx)
}

func (p *Page) Tap(selector string, opts goja.Value) {
	p.MainFrame().Tap(selector, opts)
}

func (p *Page) TextContent(selector string, opts goja.Value) string {
	return p.MainFrame().TextContent(selector, opts)
}

func (p *Page) Title() string {
	rt := k6common.GetRuntime(p.ctx)
	js := `() => document.title`
	return p.Evaluate(rt.ToValue(js)).(goja.Value).String()
}

func (p *Page) Type(selector string, text string, opts goja.Value) {
	p.MainFrame().Type(selector, text, opts)
}

func (p *Page) Uncheck(selector string, opts goja.Value) {
	p.MainFrame().Uncheck(selector, opts)
}

func (p *Page) Unroute(url goja.Value, handler goja.Callable) {
	rt := k6common.GetRuntime(p.ctx)
	k6common.Throw(rt, errors.Errorf("Page.unroute(url, handler) has not been implemented yet!"))
}

// URL returns the location of the page
func (p *Page) URL() string {
	rt := k6common.GetRuntime(p.ctx)
	return p.Evaluate(rt.ToValue("document.location.toString()")).(string)
}

// Video returns information of recorded video
func (p *Page) Video() api.Video {
	rt := k6common.GetRuntime(p.ctx)
	k6common.Throw(rt, errors.Errorf("Page.video() has not been implemented yet!"))
	return nil
}

// ViewportSize will return information on the viewport width and height
func (p *Page) ViewportSize() map[string]float64 {
	size := make(map[string]float64, 2)
	size["width"] = float64(p.emulatedSize.Viewport.Width)
	size["height"] = float64(p.emulatedSize.Viewport.Height)
	return size
}

// WaitForEvent waits for the specified event to trigger
func (p *Page) WaitForEvent(event string, optsOrPredicate goja.Value) interface{} {
	rt := k6common.GetRuntime(p.ctx)
	k6common.Throw(rt, errors.Errorf("Page.waitForEvent(event, optsOrPredicate) has not been implemented yet!"))
	return nil
}

// WaitForFunction waits for the given predicate to return a truthy value
func (p *Page) WaitForFunction(pageFunc goja.Value, arg goja.Value, opts goja.Value) api.JSHandle {
	return p.frameManager.MainFrame().WaitForFunction(pageFunc, opts, arg)
}

// WaitForLoadState waits for the specified page life cycle event
func (p *Page) WaitForLoadState(state string, opts goja.Value) {
	p.frameManager.MainFrame().WaitForLoadState(state, opts)
}

// WaitForNavigation waits for the given navigation lifecycle event to happen
func (p *Page) WaitForNavigation(opts goja.Value) api.Response {
	return p.frameManager.MainFrame().WaitForNavigation(opts)
}

func (p *Page) WaitForRequest(urlOrPredicate, opts goja.Value) api.Request {
	rt := k6common.GetRuntime(p.ctx)
	k6common.Throw(rt, errors.Errorf("Page.waitForRequest(urlOrPredicate, opts) has not been implemented yet!"))
	return nil
}

func (p *Page) WaitForResponse(urlOrPredicate, opts goja.Value) api.Response {
	rt := k6common.GetRuntime(p.ctx)
	k6common.Throw(rt, errors.Errorf("Page.waitForResponse(urlOrPredicate, opts) has not been implemented yet!"))
	return nil
}

// WaitForSelector waits for the given selector to match the waiting criteria
func (p *Page) WaitForSelector(selector string, opts goja.Value) api.ElementHandle {
	return p.frameManager.MainFrame().WaitForSelector(selector, opts)
}

// WaitForTimeout waits the specified number of milliseconds
func (p *Page) WaitForTimeout(timeout int64) {
	p.frameManager.MainFrame().WaitForTimeout(timeout)
}

// Workers returns all WebWorkers of page
func (p *Page) Workers() []api.Worker {
	workers := make([]api.Worker, len(p.workers))
	for _, w := range p.workers {
		workers = append(workers, w)
	}
	return workers
}
