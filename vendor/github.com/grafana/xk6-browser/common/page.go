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

	"github.com/chromedp/cdproto"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	cdppage "github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
	"github.com/dop251/goja"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/grafana/xk6-browser/k6ext"
	"github.com/grafana/xk6-browser/log"

	k6modules "go.k6.io/k6/js/modules"
)

// BlankPage represents a blank page.
const BlankPage = "about:blank"

const (
	webVitalBinding = "k6browserSendWebVitalMetric"

	eventPageConsoleAPICalled = "console"
)

// MediaType represents the type of media to emulate.
type MediaType string

const (
	// MediaTypeScreen represents the screen media type.
	MediaTypeScreen MediaType = "screen"

	// MediaTypePrint represents the print media type.
	MediaTypePrint MediaType = "print"
)

// ReducedMotion represents a browser reduce-motion setting.
type ReducedMotion string

// Valid reduce-motion options.
const (
	ReducedMotionReduce       ReducedMotion = "reduce"
	ReducedMotionNoPreference ReducedMotion = "no-preference"
)

func (r ReducedMotion) String() string {
	return reducedMotionToString[r]
}

var reducedMotionToString = map[ReducedMotion]string{ //nolint:gochecknoglobals
	ReducedMotionReduce:       "reduce",
	ReducedMotionNoPreference: "no-preference",
}

var reducedMotionToID = map[string]ReducedMotion{ //nolint:gochecknoglobals
	"reduce":        ReducedMotionReduce,
	"no-preference": ReducedMotionNoPreference,
}

// MarshalJSON marshals the enum as a quoted JSON string.
func (r ReducedMotion) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	buffer.WriteString(reducedMotionToString[r])
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

// UnmarshalJSON unmarshals a quoted JSON string to the enum value.
func (r *ReducedMotion) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return fmt.Errorf("unmarshaling %q to ReducedMotion: %w", b, err)
	}
	// Note that if the string cannot be found then it will be set to the zero value.
	*r = reducedMotionToID[j]
	return nil
}

// Screen represents a device screen.
type Screen struct {
	Width  int64 `js:"width"`
	Height int64 `js:"height"`
}

const (
	screenWidth  = "width"
	screenHeight = "height"
)

// Parse parses the given screen options.
func (s *Screen) Parse(ctx context.Context, screen goja.Value) error {
	rt := k6ext.Runtime(ctx)
	if screen != nil && !goja.IsUndefined(screen) && !goja.IsNull(screen) {
		screen := screen.ToObject(rt)
		for _, k := range screen.Keys() {
			switch k {
			case screenWidth:
				s.Width = screen.Get(k).ToInteger()
			case screenHeight:
				s.Height = screen.Get(k).ToInteger()
			}
		}
	}

	return nil
}

// ColorScheme represents a browser color scheme.
type ColorScheme string

// Valid color schemes.
const (
	ColorSchemeLight        ColorScheme = "light"
	ColorSchemeDark         ColorScheme = "dark"
	ColorSchemeNoPreference ColorScheme = "no-preference"
)

func (c ColorScheme) String() string {
	return colorSchemeToString[c]
}

var colorSchemeToString = map[ColorScheme]string{ //nolint:gochecknoglobals
	ColorSchemeLight:        "light",
	ColorSchemeDark:         "dark",
	ColorSchemeNoPreference: "no-preference",
}

var colorSchemeToID = map[string]ColorScheme{ //nolint:gochecknoglobals
	"light":         ColorSchemeLight,
	"dark":          ColorSchemeDark,
	"no-preference": ColorSchemeNoPreference,
}

// MarshalJSON marshals the enum as a quoted JSON string.
func (c ColorScheme) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	buffer.WriteString(colorSchemeToString[c])
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

// UnmarshalJSON unmarshals a quoted JSON string to the enum value.
func (c *ColorScheme) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return fmt.Errorf("unmarshaling %q to ColorScheme: %w", b, err)
	}
	// Note that if the string cannot be found then it will be set to the zero value.
	*c = colorSchemeToID[j]
	return nil
}

// EmulatedSize represents the emulated viewport and screen sizes.
type EmulatedSize struct {
	Viewport *Viewport
	Screen   *Screen
}

// NewEmulatedSize creates and returns a new EmulatedSize.
func NewEmulatedSize(viewport *Viewport, screen *Screen) *EmulatedSize {
	return &EmulatedSize{
		Viewport: viewport,
		Screen:   screen,
	}
}

type consoleEventHandlerFunc func(*ConsoleMessage)

// ConsoleMessage represents a page console message.
type ConsoleMessage struct {
	// Args represent the list of arguments passed to a console function call.
	Args []JSHandleAPI

	// Page is the page that produced the console message, if any.
	Page *Page

	// Text represents the text of the console message.
	Text string

	// Type is the type of the console message.
	// It can be one of 'log', 'debug', 'info', 'error', 'warning', 'dir', 'dirxml',
	// 'table', 'trace', 'clear', 'startGroup', 'startGroupCollapsed', 'endGroup',
	// 'assert', 'profile', 'profileEnd', 'count', 'timeEnd'.
	Type string
}

// Page stores Page/tab related context.
type Page struct {
	BaseEventEmitter

	Keyboard    *Keyboard
	Mouse       *Mouse
	Touchscreen *Touchscreen

	ctx context.Context

	// what it really needs is an executor with
	// SessionID and TargetID
	session session

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

	eventCh         chan Event
	eventHandlers   map[string][]consoleEventHandlerFunc
	eventHandlersMu sync.RWMutex

	mainFrameSession *FrameSession
	frameSessions    map[cdp.FrameID]*FrameSession
	frameSessionsMu  sync.RWMutex
	workers          map[target.SessionID]*Worker
	routes           []any // TODO: Implement
	vu               k6modules.VU

	logger *log.Logger
}

// NewPage creates a new browser page context.
func NewPage(
	ctx context.Context,
	s session,
	bctx *BrowserContext,
	tid target.ID,
	opener *Page,
	bp bool,
	logger *log.Logger,
) (*Page, error) {
	p := Page{
		BaseEventEmitter: NewBaseEventEmitter(ctx),
		ctx:              ctx,
		session:          s,
		browserCtx:       bctx,
		targetID:         tid,
		opener:           opener,
		backgroundPage:   bp,
		mediaType:        MediaTypeScreen,
		colorScheme:      bctx.opts.ColorScheme,
		reducedMotion:    bctx.opts.ReducedMotion,
		extraHTTPHeaders: bctx.opts.ExtraHTTPHeaders,
		timeoutSettings:  NewTimeoutSettings(bctx.timeoutSettings),
		Keyboard:         NewKeyboard(ctx, s),
		jsEnabled:        true,
		eventCh:          make(chan Event),
		eventHandlers:    make(map[string][]consoleEventHandlerFunc),
		frameSessions:    make(map[cdp.FrameID]*FrameSession),
		workers:          make(map[target.SessionID]*Worker),
		vu:               k6ext.GetVU(ctx),
		logger:           logger,
	}

	p.logger.Debugf("Page:NewPage", "sid:%v tid:%v backgroundPage:%t",
		p.sessionID(), tid, bp)

	// We need to init viewport and screen size before initializing the main frame session,
	// as that's where the emulation is activated.
	if bctx.opts.Viewport != nil {
		p.emulatedSize = NewEmulatedSize(bctx.opts.Viewport, bctx.opts.Screen)
	}

	var err error
	p.frameManager = NewFrameManager(ctx, s, &p, p.timeoutSettings, p.logger)
	p.mainFrameSession, err = NewFrameSession(ctx, s, &p, nil, tid, p.logger)
	if err != nil {
		p.logger.Debugf("Page:NewPage:NewFrameSession:return", "sid:%v tid:%v err:%v",
			p.sessionID(), tid, err)

		return nil, err
	}
	p.frameSessionsMu.Lock()
	p.frameSessions[cdp.FrameID(tid)] = p.mainFrameSession
	p.frameSessionsMu.Unlock()
	p.Mouse = NewMouse(ctx, s, p.frameManager.MainFrame(), bctx.timeoutSettings, p.Keyboard)
	p.Touchscreen = NewTouchscreen(ctx, s, p.Keyboard)

	p.initEvents()

	action := target.SetAutoAttach(true, true).WithFlatten(true)
	if err := action.Do(cdp.WithExecutor(p.ctx, p.session)); err != nil {
		return nil, fmt.Errorf("internal error while auto attaching to browser pages: %w", err)
	}

	add := runtime.AddBinding(webVitalBinding)
	if err := add.Do(cdp.WithExecutor(p.ctx, p.session)); err != nil {
		return nil, fmt.Errorf("internal error while adding binding to page: %w", err)
	}

	if err := bctx.applyAllInitScripts(&p); err != nil {
		return nil, fmt.Errorf("internal error while applying init scripts to page: %w", err)
	}

	return &p, nil
}

func (p *Page) initEvents() {
	p.logger.Debugf("Page:initEvents",
		"sid:%v tid:%v", p.session.ID(), p.targetID)

	events := []string{
		cdproto.EventRuntimeConsoleAPICalled,
	}
	p.session.on(p.ctx, events, p.eventCh)

	go func() {
		p.logger.Debugf("Page:initEvents:go",
			"sid:%v tid:%v", p.session.ID(), p.targetID)
		defer func() {
			p.logger.Debugf("Page:initEvents:go:return",
				"sid:%v tid:%v", p.session.ID(), p.targetID)
		}()

		for {
			select {
			case <-p.session.Done():
				p.logger.Debugf("Page:initEvents:go:session.done",
					"sid:%v tid:%v", p.session.ID(), p.targetID)
				return
			case <-p.ctx.Done():
				p.logger.Debugf("Page:initEvents:go:ctx.Done",
					"sid:%v tid:%v", p.session.ID(), p.targetID)
				return
			case event := <-p.eventCh:
				if ev, ok := event.data.(*cdpruntime.EventConsoleAPICalled); ok {
					p.onConsoleAPICalled(ev)
				}
			}
		}
	}()
}

func (p *Page) closeWorker(sessionID target.SessionID) {
	p.logger.Debugf("Page:closeWorker", "sid:%v", sessionID)

	if worker, ok := p.workers[sessionID]; ok {
		worker.didClose()
		delete(p.workers, sessionID)
	}
}

func (p *Page) defaultTimeout() time.Duration {
	return p.timeoutSettings.timeout()
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

	p.emit(EventPageCrash, p)
}

func (p *Page) evaluateOnNewDocument(source string) error {
	p.logger.Debugf("Page:evaluateOnNewDocument", "sid:%v", p.sessionID())

	action := page.AddScriptToEvaluateOnNewDocument(source)
	_, err := action.Do(cdp.WithExecutor(p.ctx, p.session))
	if err != nil {
		return fmt.Errorf("evaluating script on document: %w", err)
	}

	return nil
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

	rootFrame := f
	for ; rootFrame.parentFrame != nil; rootFrame = rootFrame.parentFrame {
	}

	parentSession := p.getFrameSession(cdp.FrameID(rootFrame.ID()))
	action := dom.GetFrameOwner(cdp.FrameID(f.ID()))
	backendNodeId, _, err := action.Do(cdp.WithExecutor(p.ctx, parentSession.session))
	if err != nil {
		if strings.Contains(err.Error(), "frame with the given id was not found") {
			return nil, errors.New("frame has been detached")
		}
		return nil, fmt.Errorf("getting frame owner: %w", err)
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
	pageFn := `
		node => {
			const doc = node;
      		if (doc.documentElement && doc.documentElement.ownerDocument === doc)
        		return doc.documentElement;
      		return node.ownerDocument ? node.ownerDocument.documentElement : null;
		}
	`

	opts := evalOptions{
		forceCallable: true,
		returnByValue: false,
	}
	result, err := h.execCtx.eval(apiCtx, opts, pageFn, h)
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
	p.logger.Debugf("Page:attachFrameSession", "sid:%v fid=%v", p.session.ID(), fid)
	p.frameSessionsMu.Lock()
	defer p.frameSessionsMu.Unlock()
	fs.page.frameSessions[fid] = fs
}

func (p *Page) getFrameSession(frameID cdp.FrameID) *FrameSession {
	p.logger.Debugf("Page:getFrameSession", "sid:%v fid:%v", p.sessionID(), frameID)
	p.frameSessionsMu.RLock()
	defer p.frameSessionsMu.RUnlock()
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

	p.frameSessionsMu.RLock()
	defer p.frameSessionsMu.RUnlock()

	for _, fs := range p.frameSessions {
		fs.updateExtraHTTPHeaders(false)
	}
}

func (p *Page) updateGeolocation() error {
	p.logger.Debugf("Page:updateGeolocation", "sid:%v", p.sessionID())

	p.frameSessionsMu.RLock()
	defer p.frameSessionsMu.RUnlock()

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

	p.frameSessionsMu.RLock()
	defer p.frameSessionsMu.RUnlock()

	for _, fs := range p.frameSessions {
		fs.updateOffline(false)
	}
}

func (p *Page) updateHttpCredentials() {
	p.logger.Debugf("Page:updateHttpCredentials", "sid:%v", p.sessionID())

	p.frameSessionsMu.RLock()
	defer p.frameSessionsMu.RUnlock()

	for _, fs := range p.frameSessions {
		fs.updateHTTPCredentials(false)
	}
}

func (p *Page) viewportSize() Size {
	return Size{
		Width:  float64(p.emulatedSize.Viewport.Width),
		Height: float64(p.emulatedSize.Viewport.Height),
	}
}

// AddInitScript adds script to run in all new frames.
func (p *Page) AddInitScript(script goja.Value, arg goja.Value) {
	k6ext.Panic(p.ctx, "Page.addInitScript(script, arg) has not been implemented yet")
}

// AddScriptTag is not implemented.
func (p *Page) AddScriptTag(opts goja.Value) {
	k6ext.Panic(p.ctx, "Page.addScriptTag(opts) has not been implemented yet")
}

// AddStyleTag is not implemented.
func (p *Page) AddStyleTag(opts goja.Value) {
	k6ext.Panic(p.ctx, "Page.addStyleTag(opts) has not been implemented yet")
}

// BringToFront activates the browser tab for this page.
func (p *Page) BringToFront() {
	p.logger.Debugf("Page:BringToFront", "sid:%v", p.sessionID())

	action := cdppage.BringToFront()
	if err := action.Do(cdp.WithExecutor(p.ctx, p.session)); err != nil {
		k6ext.Panic(p.ctx, "bringing page to front: %w", err)
	}
}

// Check checks an element matching the provided selector.
func (p *Page) Check(selector string, opts goja.Value) {
	p.logger.Debugf("Page:Check", "sid:%v selector:%s", p.sessionID(), selector)

	p.MainFrame().Check(selector, opts)
}

// Uncheck unchecks an element matching the provided selector.
func (p *Page) Uncheck(selector string, opts goja.Value) {
	p.logger.Debugf("Page:Uncheck", "sid:%v selector:%s", p.sessionID(), selector)

	p.MainFrame().Uncheck(selector, opts)
}

// IsChecked returns true if the first element that matches the selector
// is checked. Otherwise, returns false.
func (p *Page) IsChecked(selector string, opts goja.Value) bool {
	p.logger.Debugf("Page:IsChecked", "sid:%v selector:%s", p.sessionID(), selector)

	return p.MainFrame().IsChecked(selector, opts)
}

// Click clicks an element matching provided selector.
func (p *Page) Click(selector string, opts *FrameClickOptions) error {
	p.logger.Debugf("Page:Click", "sid:%v selector:%s", p.sessionID(), selector)

	return p.MainFrame().Click(selector, opts)
}

// Close closes the page.
func (p *Page) Close(_ goja.Value) error {
	p.logger.Debugf("Page:Close", "sid:%v", p.sessionID())
	_, span := TraceAPICall(p.ctx, p.targetID.String(), "page.close")
	defer span.End()

	// forcing the pagehide event to trigger web vitals metrics.
	v := `() => window.dispatchEvent(new Event('pagehide'))`
	ctx, cancel := context.WithTimeout(p.ctx, p.defaultTimeout())
	defer cancel()
	_, err := p.MainFrame().EvaluateWithContext(ctx, v)
	if err != nil {
		p.logger.Warnf("Page:Close", "failed to hide page: %v", err)
	}

	add := runtime.RemoveBinding(webVitalBinding)
	if err := add.Do(cdp.WithExecutor(p.ctx, p.session)); err != nil {
		return fmt.Errorf("internal error while removing binding from page: %w", err)
	}

	action := target.CloseTarget(p.targetID)
	err = action.Do(cdp.WithExecutor(p.ctx, p.session))
	if err != nil {
		// When a close target command is sent to the browser via CDP,
		// the browser will start to cleanup and the first thing it
		// will do is return a target.EventDetachedFromTarget, which in
		// our implementation will close the session connection (this
		// does not close the CDP websocket, just removes the session
		// so no other CDP calls can be made with the session ID).
		// This can result in the session's context being closed while
		// we're waiting for the response to come back from the browser
		// for this current command (it's racey).
		if errors.Is(err, context.Canceled) {
			return nil
		}

		return fmt.Errorf("closing a page: %w", err)
	}

	return nil
}

// Content returns the HTML content of the page.
func (p *Page) Content() string {
	p.logger.Debugf("Page:Content", "sid:%v", p.sessionID())

	return p.MainFrame().Content()
}

// Context closes the page.
func (p *Page) Context() *BrowserContext {
	return p.browserCtx
}

// Dblclick double clicks an element matching provided selector.
func (p *Page) Dblclick(selector string, opts goja.Value) {
	p.logger.Debugf("Page:Dblclick", "sid:%v selector:%s", p.sessionID(), selector)

	p.MainFrame().Dblclick(selector, opts)
}

// DispatchEvent dispatches an event on the page to the element that matches the provided selector.
func (p *Page) DispatchEvent(selector string, typ string, eventInit any, opts *FrameDispatchEventOptions) error {
	p.logger.Debugf("Page:DispatchEvent", "sid:%v selector:%s", p.sessionID(), selector)

	return p.MainFrame().DispatchEvent(selector, typ, eventInit, opts)
}

// DragAndDrop is not implemented.
func (p *Page) DragAndDrop(source string, target string, opts goja.Value) {
	k6ext.Panic(p.ctx, "Page.DragAndDrop(source, target, opts) has not been implemented yet")
}

func (p *Page) EmulateMedia(opts goja.Value) {
	p.logger.Debugf("Page:EmulateMedia", "sid:%v", p.sessionID())

	parsedOpts := NewPageEmulateMediaOptions(p.mediaType, p.colorScheme, p.reducedMotion)
	if err := parsedOpts.Parse(p.ctx, opts); err != nil {
		k6ext.Panic(p.ctx, "parsing emulateMedia options: %w", err)
	}

	p.mediaType = parsedOpts.Media
	p.colorScheme = parsedOpts.ColorScheme
	p.reducedMotion = parsedOpts.ReducedMotion

	p.frameSessionsMu.RLock()
	for _, fs := range p.frameSessions {
		if err := fs.updateEmulateMedia(false); err != nil {
			p.frameSessionsMu.RUnlock()
			k6ext.Panic(p.ctx, "emulating media: %w", err)
		}
	}
	p.frameSessionsMu.RUnlock()

	applySlowMo(p.ctx)
}

// EmulateVisionDeficiency activates/deactivates emulation of a vision deficiency.
func (p *Page) EmulateVisionDeficiency(typ string) {
	p.logger.Debugf("Page:EmulateVisionDeficiency", "sid:%v typ:%s", p.sessionID(), typ)

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
		k6ext.Panic(p.ctx, "unsupported vision deficiency: '%s'", typ)
	}

	action := emulation.SetEmulatedVisionDeficiency(t)
	if err := action.Do(cdp.WithExecutor(p.ctx, p.session)); err != nil {
		k6ext.Panic(p.ctx, "setting emulated vision deficiency %q: %w", typ, err)
	}

	applySlowMo(p.ctx)
}

// Evaluate runs JS code within the execution context of the main frame of the page.
func (p *Page) Evaluate(pageFunc string, args ...any) any {
	p.logger.Debugf("Page:Evaluate", "sid:%v", p.sessionID())

	return p.MainFrame().Evaluate(pageFunc, args...)
}

// EvaluateHandle runs JS code within the execution context of the main frame of the page.
func (p *Page) EvaluateHandle(pageFunc string, args ...any) (JSHandleAPI, error) {
	p.logger.Debugf("Page:EvaluateHandle", "sid:%v", p.sessionID())

	h, err := p.MainFrame().EvaluateHandle(pageFunc, args...)
	if err != nil {
		return nil, fmt.Errorf("evaluating handle for page: %w", err)
	}
	return h, nil
}

// ExposeBinding is not implemented.
func (p *Page) ExposeBinding(name string, callback goja.Callable, opts goja.Value) {
	k6ext.Panic(p.ctx, "Page.exposeBinding(name, callback) has not been implemented yet")
}

// ExposeFunction is not implemented.
func (p *Page) ExposeFunction(name string, callback goja.Callable) {
	k6ext.Panic(p.ctx, "Page.exposeFunction(name, callback) has not been implemented yet")
}

func (p *Page) Fill(selector string, value string, opts goja.Value) {
	p.logger.Debugf("Page:Fill", "sid:%v selector:%s", p.sessionID(), selector)

	p.MainFrame().Fill(selector, value, opts)
}

func (p *Page) Focus(selector string, opts goja.Value) {
	p.logger.Debugf("Page:Focus", "sid:%v selector:%s", p.sessionID(), selector)

	p.MainFrame().Focus(selector, opts)
}

// Frame is not implemented.
func (p *Page) Frame(_ goja.Value) *Frame {
	k6ext.Panic(p.ctx, "Page.frame(frameSelector) has not been implemented yet")
	return nil
}

// Frames returns a list of frames on the page.
func (p *Page) Frames() []*Frame {
	return p.frameManager.Frames()
}

// GetAttribute returns the attribute value of the element matching the provided selector.
func (p *Page) GetAttribute(selector string, name string, opts goja.Value) any {
	p.logger.Debugf("Page:GetAttribute", "sid:%v selector:%s name:%s",
		p.sessionID(), selector, name)

	return p.MainFrame().GetAttribute(selector, name, opts)
}

// GetKeyboard returns the keyboard for the page.
func (p *Page) GetKeyboard() *Keyboard {
	return p.Keyboard
}

// GetMouse returns the mouse for the page.
func (p *Page) GetMouse() *Mouse {
	return p.Mouse
}

// GetTouchscreen returns the touchscreen for the page.
func (p *Page) GetTouchscreen() *Touchscreen {
	return p.Touchscreen
}

// GoBack is not implemented.
func (p *Page) GoBack(_ goja.Value) *Response {
	k6ext.Panic(p.ctx, "Page.goBack(opts) has not been implemented yet")
	return nil
}

// GoForward is not implemented.
func (p *Page) GoForward(_ goja.Value) *Response {
	k6ext.Panic(p.ctx, "Page.goForward(opts) has not been implemented yet")
	return nil
}

// Goto will navigate the page to the specified URL and return a HTTP response object.
func (p *Page) Goto(url string, opts *FrameGotoOptions) (*Response, error) {
	p.logger.Debugf("Page:Goto", "sid:%v url:%q", p.sessionID(), url)
	_, span := TraceAPICall(
		p.ctx,
		p.targetID.String(),
		"page.goto",
		trace.WithAttributes(attribute.String("page.goto.url", url)),
	)
	defer span.End()

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

// IsHidden will look for an element in the dom with given selector and see if
// the element is hidden. It will not wait for a match to occur. If no elements
// match `false` will be returned.
func (p *Page) IsHidden(selector string, opts goja.Value) (bool, error) {
	p.logger.Debugf("Page:IsHidden", "sid:%v selector:%s", p.sessionID(), selector)

	return p.MainFrame().IsHidden(selector, opts)
}

// IsVisible will look for an element in the dom with given selector. It will
// not wait for a match to occur. If no elements match `false` will be returned.
func (p *Page) IsVisible(selector string, opts goja.Value) (bool, error) {
	p.logger.Debugf("Page:IsVisible", "sid:%v selector:%s", p.sessionID(), selector)

	return p.MainFrame().IsVisible(selector, opts)
}

// Locator creates and returns a new locator for this page (main frame).
func (p *Page) Locator(selector string, opts goja.Value) *Locator {
	p.logger.Debugf("Page:Locator", "sid:%s sel: %q opts:%+v", p.sessionID(), selector, opts)

	return p.MainFrame().Locator(selector, opts)
}

// MainFrame returns the main frame on the page.
func (p *Page) MainFrame() *Frame {
	mf := p.frameManager.MainFrame()

	if mf == nil {
		p.logger.Debugf("Page:MainFrame", "sid:%v", p.sessionID())
	} else {
		p.logger.Debugf("Page:MainFrame",
			"sid:%v mfid:%v mflid:%v mfurl:%v",
			p.sessionID(), mf.id, mf.loaderID, mf.URL())
	}

	return mf
}

// Referrer returns the page's referrer.
// It's an internal method not to be exposed as a JS API.
func (p *Page) Referrer() string {
	nm := p.mainFrameSession.getNetworkManager()
	return nm.extraHTTPHeaders["referer"]
}

// NavigationTimeout returns the page's navigation timeout.
// It's an internal method not to be exposed as a JS API.
func (p *Page) NavigationTimeout() time.Duration {
	return p.frameManager.timeoutSettings.navigationTimeout()
}

// On subscribes to a page event for which the given handler will be executed
// passing in the ConsoleMessage associated with the event.
// The only accepted event value is 'console'.
func (p *Page) On(event string, handler func(*ConsoleMessage)) error {
	if event != eventPageConsoleAPICalled {
		return fmt.Errorf("unknown page event: %q, must be %q", event, eventPageConsoleAPICalled)
	}

	p.eventHandlersMu.Lock()
	defer p.eventHandlersMu.Unlock()

	if _, ok := p.eventHandlers[eventPageConsoleAPICalled]; !ok {
		p.eventHandlers[eventPageConsoleAPICalled] = make([]consoleEventHandlerFunc, 0, 1)
	}
	p.eventHandlers[eventPageConsoleAPICalled] = append(p.eventHandlers[eventPageConsoleAPICalled], handler)

	return nil
}

// Opener returns the opener of the target.
func (p *Page) Opener() *Page {
	return p.opener
}

// Pause is not implemented.
func (p *Page) Pause() {
	k6ext.Panic(p.ctx, "Page.pause() has not been implemented yet")
}

// Pdf is not implemented.
func (p *Page) Pdf(opts goja.Value) []byte {
	k6ext.Panic(p.ctx, "Page.pdf(opts) has not been implemented yet")
	return nil
}

func (p *Page) Press(selector string, key string, opts goja.Value) {
	p.logger.Debugf("Page:Press", "sid:%v selector:%s", p.sessionID(), selector)

	p.MainFrame().Press(selector, key, opts)
}

// Query returns the first element matching the specified selector.
func (p *Page) Query(selector string) (*ElementHandle, error) {
	p.logger.Debugf("Page:Query", "sid:%v selector:%s", p.sessionID(), selector)

	return p.frameManager.MainFrame().Query(selector, StrictModeOff)
}

// QueryAll returns all elements matching the specified selector.
func (p *Page) QueryAll(selector string) ([]*ElementHandle, error) {
	p.logger.Debugf("Page:QueryAll", "sid:%v selector:%s", p.sessionID(), selector)

	return p.frameManager.MainFrame().QueryAll(selector)
}

// Reload will reload the current page.
func (p *Page) Reload(opts goja.Value) *Response { //nolint:funlen,cyclop
	p.logger.Debugf("Page:Reload", "sid:%v", p.sessionID())
	_, span := TraceAPICall(p.ctx, p.targetID.String(), "page.reload")
	defer span.End()

	parsedOpts := NewPageReloadOptions(
		LifecycleEventLoad,
		p.timeoutSettings.navigationTimeout(),
	)
	if err := parsedOpts.Parse(p.ctx, opts); err != nil {
		k6ext.Panic(p.ctx, "parsing reload options: %w", err)
	}

	timeoutCtx, timeoutCancelFn := context.WithTimeout(p.ctx, parsedOpts.Timeout)
	defer timeoutCancelFn()

	ch, evCancelFn := createWaitForEventHandler(
		timeoutCtx, p.frameManager.MainFrame(), []string{EventFrameNavigation},
		func(data any) bool {
			return true // Both successful and failed navigations are considered
		},
	)
	defer evCancelFn() // Remove event handler

	lifecycleEvtCh, lifecycleEvtCancel := createWaitForEventPredicateHandler(
		timeoutCtx, p.frameManager.MainFrame(), []string{EventFrameAddLifecycle},
		func(data any) bool {
			if le, ok := data.(FrameLifecycleEvent); ok {
				return le.Event == parsedOpts.WaitUntil
			}
			return false
		})
	defer lifecycleEvtCancel()

	action := cdppage.Reload()
	if err := action.Do(cdp.WithExecutor(p.ctx, p.session)); err != nil {
		k6ext.Panic(p.ctx, "reloading page: %w", err)
	}

	wrapTimeoutError := func(err error) error {
		if errors.Is(err, context.DeadlineExceeded) {
			err = &k6ext.UserFriendlyError{
				Err:     err,
				Timeout: parsedOpts.Timeout,
			}
			return fmt.Errorf("reloading page: %w", err)
		}
		p.logger.Debugf("Page:Reload", "timeoutCtx done: %v", err)

		return err // TODO maybe wrap this as well?
	}

	var event *NavigationEvent
	select {
	case <-p.ctx.Done():
	case <-timeoutCtx.Done():
		k6ext.Panic(p.ctx, "%w", wrapTimeoutError(timeoutCtx.Err()))
	case data := <-ch:
		event = data.(*NavigationEvent)
	}

	var resp *Response
	req := event.newDocument.request
	if req != nil {
		req.responseMu.RLock()
		resp = req.response
		req.responseMu.RUnlock()
	}

	select {
	case <-lifecycleEvtCh:
	case <-timeoutCtx.Done():
		k6ext.Panic(p.ctx, "%w", wrapTimeoutError(timeoutCtx.Err()))
	}

	applySlowMo(p.ctx)

	return resp
}

// Route is not implemented.
func (p *Page) Route(url goja.Value, handler goja.Callable) {
	k6ext.Panic(p.ctx, "Page.route(url, handler) has not been implemented yet")
}

// Screenshot will instruct Chrome to save a screenshot of the current page and save it to specified file.
func (p *Page) Screenshot(opts *PageScreenshotOptions, sp ScreenshotPersister) ([]byte, error) {
	spanCtx, span := TraceAPICall(p.ctx, p.targetID.String(), "page.screenshot")
	defer span.End()

	span.SetAttributes(attribute.String("screenshot.path", opts.Path))

	s := newScreenshotter(spanCtx, sp)
	buf, err := s.screenshotPage(p, opts)
	if err != nil {
		return nil, fmt.Errorf("taking screenshot of page: %w", err)
	}

	return buf, err
}

func (p *Page) SelectOption(selector string, values goja.Value, opts goja.Value) []string {
	p.logger.Debugf("Page:SelectOption", "sid:%v selector:%s", p.sessionID(), selector)

	return p.MainFrame().SelectOption(selector, values, opts)
}

func (p *Page) SetContent(html string, opts goja.Value) {
	p.logger.Debugf("Page:SetContent", "sid:%v", p.sessionID())

	p.MainFrame().SetContent(html, opts)
}

// SetDefaultNavigationTimeout sets the default navigation timeout in milliseconds.
func (p *Page) SetDefaultNavigationTimeout(timeout int64) {
	p.logger.Debugf("Page:SetDefaultNavigationTimeout", "sid:%v timeout:%d", p.sessionID(), timeout)

	p.timeoutSettings.setDefaultNavigationTimeout(time.Duration(timeout) * time.Millisecond)
}

// SetDefaultTimeout sets the default maximum timeout in milliseconds.
func (p *Page) SetDefaultTimeout(timeout int64) {
	p.logger.Debugf("Page:SetDefaultTimeout", "sid:%v timeout:%d", p.sessionID(), timeout)

	p.timeoutSettings.setDefaultTimeout(time.Duration(timeout) * time.Millisecond)
}

// SetExtraHTTPHeaders sets default HTTP headers for page and whole frame hierarchy.
func (p *Page) SetExtraHTTPHeaders(headers map[string]string) {
	p.logger.Debugf("Page:SetExtraHTTPHeaders", "sid:%v", p.sessionID())

	p.extraHTTPHeaders = headers
	p.updateExtraHTTPHeaders()
}

// SetInputFiles sets input files for the selected element.
func (p *Page) SetInputFiles(selector string, files goja.Value, opts goja.Value) error {
	p.logger.Debugf("Page:SetInputFiles", "sid:%v selector:%s", p.sessionID(), selector)

	return p.MainFrame().SetInputFiles(selector, files, opts)
}

// SetViewportSize will update the viewport width and height.
func (p *Page) SetViewportSize(viewportSize goja.Value) {
	p.logger.Debugf("Page:SetViewportSize", "sid:%v", p.sessionID())

	s := &Size{}
	if err := s.Parse(p.ctx, viewportSize); err != nil {
		k6ext.Panic(p.ctx, "parsing viewport size: %w", err)
	}
	if err := p.setViewportSize(s); err != nil {
		k6ext.Panic(p.ctx, "setting viewport size: %w", err)
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

// Timeout will return the default timeout or the one set by the user.
// It's an internal method not to be exposed as a JS API.
func (p *Page) Timeout() time.Duration {
	return p.defaultTimeout()
}

func (p *Page) Title() string {
	p.logger.Debugf("Page:Title", "sid:%v", p.sessionID())

	// TODO: return error

	v := `() => document.title`
	return p.Evaluate(v).(string) //nolint:forcetypeassert
}

// ThrottleCPU will slow the CPU down from chrome's perspective to simulate
// a test being run on a slower device.
func (p *Page) ThrottleCPU(cpuProfile CPUProfile) error {
	p.logger.Debugf("Page:ThrottleCPU", "sid:%v", p.sessionID())

	p.frameSessionsMu.RLock()
	defer p.frameSessionsMu.RUnlock()

	for _, fs := range p.frameSessions {
		if err := fs.throttleCPU(cpuProfile); err != nil {
			return err
		}
	}

	return nil
}

// ThrottleNetwork will slow the network down to simulate a slow network e.g.
// simulating a slow 3G connection.
func (p *Page) ThrottleNetwork(networkProfile NetworkProfile) error {
	p.logger.Debugf("Page:ThrottleNetwork", "sid:%v", p.sessionID())

	p.frameSessionsMu.RLock()
	defer p.frameSessionsMu.RUnlock()

	for _, fs := range p.frameSessions {
		if err := fs.throttleNetwork(networkProfile); err != nil {
			return err
		}
	}

	return nil
}

func (p *Page) Type(selector string, text string, opts goja.Value) {
	p.logger.Debugf("Page:Type", "sid:%v selector:%s text:%s", p.sessionID(), selector, text)

	p.MainFrame().Type(selector, text, opts)
}

// Unroute is not implemented.
func (p *Page) Unroute(url goja.Value, handler goja.Callable) {
	k6ext.Panic(p.ctx, "Page.unroute(url, handler) has not been implemented yet")
}

// URL returns the location of the page.
func (p *Page) URL() string {
	p.logger.Debugf("Page:URL", "sid:%v", p.sessionID())

	// TODO: return error

	v := `() => document.location.toString()`
	return p.Evaluate(v).(string) //nolint:forcetypeassert
}

// Video returns information of recorded video.
func (p *Page) Video() any { // TODO: implement
	k6ext.Panic(p.ctx, "Page.video() has not been implemented yet")
	return nil
}

// ViewportSize will return information on the viewport width and height.
func (p *Page) ViewportSize() map[string]float64 {
	p.logger.Debugf("Page:ViewportSize", "sid:%v", p.sessionID())

	vps := p.viewportSize()
	return map[string]float64{
		"width":  vps.Width,
		"height": vps.Height,
	}
}

// WaitForEvent waits for the specified event to trigger.
func (p *Page) WaitForEvent(event string, optsOrPredicate goja.Value) any {
	k6ext.Panic(p.ctx, "Page.waitForEvent(event, optsOrPredicate) has not been implemented yet")
	return nil
}

// WaitForFunction waits for the given predicate to return a truthy value.
func (p *Page) WaitForFunction(js string, opts *FrameWaitForFunctionOptions, jsArgs ...any) (any, error) {
	p.logger.Debugf("Page:WaitForFunction", "sid:%v", p.sessionID())
	return p.frameManager.MainFrame().WaitForFunction(js, opts, jsArgs...)
}

// WaitForLoadState waits for the specified page life cycle event.
func (p *Page) WaitForLoadState(state string, opts goja.Value) {
	p.logger.Debugf("Page:WaitForLoadState", "sid:%v state:%q", p.sessionID(), state)

	p.frameManager.MainFrame().WaitForLoadState(state, opts)
}

// WaitForNavigation waits for the given navigation lifecycle event to happen.
func (p *Page) WaitForNavigation(opts *FrameWaitForNavigationOptions) (*Response, error) {
	p.logger.Debugf("Page:WaitForNavigation", "sid:%v", p.sessionID())
	_, span := TraceAPICall(p.ctx, p.targetID.String(), "page.waitForNavigation")
	defer span.End()

	return p.frameManager.MainFrame().WaitForNavigation(opts)
}

// WaitForRequest is not implemented.
func (p *Page) WaitForRequest(_, _ goja.Value) *Request {
	k6ext.Panic(p.ctx, "Page.waitForRequest(urlOrPredicate, opts) has not been implemented yet")
	return nil
}

// WaitForResponse is not implemented.
func (p *Page) WaitForResponse(_, _ goja.Value) *Response {
	k6ext.Panic(p.ctx, "Page.waitForResponse(urlOrPredicate, opts) has not been implemented yet")
	return nil
}

// WaitForSelector waits for the given selector to match the waiting criteria.
func (p *Page) WaitForSelector(selector string, opts goja.Value) (*ElementHandle, error) {
	p.logger.Debugf("Page:WaitForSelector",
		"sid:%v stid:%v ptid:%v selector:%s",
		p.sessionID(), p.session.TargetID(), p.targetID, selector)

	return p.frameManager.MainFrame().WaitForSelector(selector, opts)
}

// WaitForTimeout waits the specified number of milliseconds.
func (p *Page) WaitForTimeout(timeout int64) {
	p.logger.Debugf("Page:WaitForTimeout", "sid:%v timeout:%d", p.sessionID(), timeout)

	p.frameManager.MainFrame().WaitForTimeout(timeout)
}

// Workers returns all WebWorkers of page.
func (p *Page) Workers() []*Worker {
	workers := make([]*Worker, 0, len(p.workers))
	for _, w := range p.workers {
		workers = append(workers, w)
	}
	return workers
}

// TargetID retrieve the unique id that is associated to this page.
// For internal use only.
func (p *Page) TargetID() string {
	return p.targetID.String()
}

func (p *Page) onConsoleAPICalled(event *cdpruntime.EventConsoleAPICalled) {
	// If there are no handlers for EventConsoleAPICalled, return
	p.eventHandlersMu.RLock()
	if _, ok := p.eventHandlers[eventPageConsoleAPICalled]; !ok {
		p.eventHandlersMu.RUnlock()
		return
	}
	p.eventHandlersMu.RUnlock()

	m, err := p.consoleMsgFromConsoleEvent(event)
	if err != nil {
		p.logger.Errorf("Page:onConsoleAPICalled", "building console message: %v", err)
		return
	}

	p.eventHandlersMu.RLock()
	defer p.eventHandlersMu.RUnlock()
	for _, h := range p.eventHandlers[eventPageConsoleAPICalled] {
		h := h
		h(m)
	}
}

func (p *Page) consoleMsgFromConsoleEvent(e *cdpruntime.EventConsoleAPICalled) (*ConsoleMessage, error) {
	execCtx, err := p.executionContextForID(e.ExecutionContextID)
	if err != nil {
		return nil, err
	}

	var (
		objects       = make([]string, 0, len(e.Args))
		objectHandles = make([]JSHandleAPI, 0, len(e.Args))
	)

	for _, robj := range e.Args {
		s, err := parseConsoleRemoteObject(p.logger, robj)
		if err != nil {
			p.logger.Errorf("consoleMsgFromConsoleEvent", "failed to parse console message %v", err)
		}

		objects = append(objects, s)
		objectHandles = append(objectHandles, NewJSHandle(
			p.ctx, p.session, execCtx, execCtx.Frame(), robj, p.logger,
		))
	}

	return &ConsoleMessage{
		Args: objectHandles,
		Page: p,
		Text: textForConsoleEvent(e, objects),
		Type: e.Type.String(),
	}, nil
}

// executionContextForID returns the page ExecutionContext for the given ID.
func (p *Page) executionContextForID(
	executionContextID cdpruntime.ExecutionContextID,
) (*ExecutionContext, error) {
	p.frameSessionsMu.RLock()
	defer p.frameSessionsMu.RUnlock()

	for _, fs := range p.frameSessions {
		if exc, err := fs.executionContextForID(executionContextID); err == nil {
			return exc, nil
		}
	}

	return nil, fmt.Errorf("no execution context found for id: %v", executionContextID)
}

// sessionID returns the Page's session ID.
// It should be used internally in the Page.
func (p *Page) sessionID() (sid target.SessionID) {
	if p != nil && p.session != nil {
		sid = p.session.ID()
	}
	return sid
}

// textForConsoleEvent generates the text representation for a consoleAPICalled event
// mimicking Playwright's behavior.
func textForConsoleEvent(e *cdpruntime.EventConsoleAPICalled, args []string) string {
	if e.Type.String() == "dir" || e.Type.String() == "dirxml" ||
		e.Type.String() == "table" {
		if len(e.Args) > 0 {
			// These commands accept a single arg
			return e.Args[0].Description
		}
		return ""
	}

	return strings.Join(args, " ")
}
