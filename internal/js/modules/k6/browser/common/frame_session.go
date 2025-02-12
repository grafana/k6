package common

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext"
	"go.k6.io/k6/internal/js/modules/k6/browser/log"

	k6modules "go.k6.io/k6/js/modules"
	k6metrics "go.k6.io/k6/metrics"

	"github.com/chromedp/cdproto"
	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/inspector"
	cdplog "github.com/chromedp/cdproto/log"
	"github.com/chromedp/cdproto/network"
	cdppage "github.com/chromedp/cdproto/page"
	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/security"
	"github.com/chromedp/cdproto/target"
)

const utilityWorldName = "__k6_browser_utility_world__"

// CPUProfile is used in throttleCPU.
type CPUProfile struct {
	// rate as a slowdown factor (1 is no throttle, 2 is 2x slowdown, etc).
	Rate float64
}

/*
FrameSession is used for managing a frame's life-cycle, or in other words its full session.
It manages all the event listening while deferring the state storage to the Frame and FrameManager
structs.
*/
type FrameSession struct {
	ctx            context.Context
	session        session
	page           *Page
	parent         *FrameSession
	manager        *FrameManager
	networkManager *NetworkManager

	k6Metrics *k6ext.CustomMetrics

	targetID target.ID
	// windowID can be 0 when it is associated to an iframe or frame with no UI.
	windowID    browser.WindowID
	hasUIWindow bool

	// To understand the concepts of Isolated Worlds, Contexts and Frames and
	// the relationship betwween them have a look at the following doc:
	//nolint:lll
	// https://chromium.googlesource.com/chromium/src/+/master/third_party/blink/renderer/bindings/core/v8/V8BindingDesign.md
	contextIDToContextMu sync.Mutex
	contextIDToContext   map[cdpruntime.ExecutionContextID]*ExecutionContext
	isolatedWorlds       map[string]bool

	eventCh chan Event

	childSessions map[cdp.FrameID]*FrameSession
	vu            k6modules.VU

	logger *log.Logger

	// Keep a reference to the main frame span so we can end it
	// when FrameSession.ctx is Done
	mainFrameSpan trace.Span
	// The initial navigation when a new page is created navigates to about:blank.
	// We want to make sure that the the navigation span is created for this in
	// onFrameNavigated, but subsequent calls to onFrameNavigated in the same
	// mainframe never again create a navigation span.
	initialNavDone bool
}

// NewFrameSession initializes and returns a new FrameSession.
//
//nolint:funlen
func NewFrameSession(
	ctx context.Context, s session, p *Page, parent *FrameSession, tid target.ID, l *log.Logger, hasUIWindow bool,
) (_ *FrameSession, err error) {
	l.Debugf("NewFrameSession", "sid:%v tid:%v", s.ID(), tid)

	k6Metrics := k6ext.GetCustomMetrics(ctx)

	fs := FrameSession{
		ctx:                  ctx, // TODO: create cancelable context that can be used to cancel and close all child sessions
		session:              s,
		page:                 p,
		parent:               parent,
		manager:              p.frameManager,
		targetID:             tid,
		contextIDToContextMu: sync.Mutex{},
		contextIDToContext:   make(map[cdpruntime.ExecutionContextID]*ExecutionContext),
		isolatedWorlds:       make(map[string]bool),
		eventCh:              make(chan Event),
		childSessions:        make(map[cdp.FrameID]*FrameSession),
		vu:                   k6ext.GetVU(ctx),
		k6Metrics:            k6Metrics,
		logger:               l,
		hasUIWindow:          hasUIWindow,
	}

	if err := cdpruntime.RunIfWaitingForDebugger().Do(cdp.WithExecutor(fs.ctx, fs.session)); err != nil {
		return nil, fmt.Errorf("run if waiting for debugger to attach: %w", err)
	}

	var parentNM *NetworkManager
	if fs.parent != nil {
		parentNM = fs.parent.networkManager
	}
	fs.networkManager, err = NewNetworkManager(ctx, k6Metrics, s, fs.manager, parentNM, fs.manager.page)
	if err != nil {
		l.Debugf("NewFrameSession:NewNetworkManager", "sid:%v tid:%v err:%v",
			s.ID(), tid, err)
		return nil, err
	}

	// When a frame creates a new FrameSession without UI (e.g. some iframes) we cannot
	// retrieve the windowID. Doing so would lead to an error from chromium. For now all
	// iframes that are attached are setup with hasUIWindow as false which seems to work
	// as expected for iframes with and without UI elements.
	if fs.hasUIWindow {
		action := browser.GetWindowForTarget().WithTargetID(fs.targetID)
		if fs.windowID, _, err = action.Do(cdp.WithExecutor(fs.ctx, fs.session)); err != nil {
			l.Debugf(
				"NewFrameSession:GetWindowForTarget",
				"sid:%v tid:%v err:%v",
				s.ID(), tid, err)

			return nil, fmt.Errorf("getting browser window ID: %w", err)
		}
	}

	fs.initEvents()
	if err = fs.initFrameTree(); err != nil {
		l.Debugf(
			"NewFrameSession:initFrameTree",
			"sid:%v tid:%v err:%v",
			s.ID(), tid, err)

		return nil, err
	}
	if err = fs.initIsolatedWorld(utilityWorldName); err != nil {
		l.Debugf(
			"NewFrameSession:initIsolatedWorld",
			"sid:%v tid:%v err:%v",
			s.ID(), tid, err)

		return nil, err
	}
	if err = fs.initOptions(); err != nil {
		l.Debugf(
			"NewFrameSession:initOptions",
			"sid:%v tid:%v err:%v",
			s.ID(), tid, err)

		return nil, err
	}
	if err = fs.initDomains(); err != nil {
		l.Debugf(
			"NewFrameSession:initDomains",
			"sid:%v tid:%v err:%v",
			s.ID(), tid, err)

		return nil, err
	}

	return &fs, nil
}

func (fs *FrameSession) emulateLocale() error {
	action := emulation.SetLocaleOverride().WithLocale(fs.page.browserCtx.opts.Locale)
	if err := action.Do(cdp.WithExecutor(fs.ctx, fs.session)); err != nil {
		if strings.Contains(err.Error(), "Another locale override is already in effect") {
			return nil
		}
		return fmt.Errorf("emulating locale %q: %w", fs.page.browserCtx.opts.Locale, err)
	}
	return nil
}

func (fs *FrameSession) emulateTimezone() error {
	action := emulation.SetTimezoneOverride(fs.page.browserCtx.opts.TimezoneID)
	if err := action.Do(cdp.WithExecutor(fs.ctx, fs.session)); err != nil {
		if strings.Contains(err.Error(), "Timezone override is already in effect") {
			return nil
		}
		return fmt.Errorf("emulating timezone %q: %w", fs.page.browserCtx.opts.TimezoneID, err)
	}
	return nil
}

func (fs *FrameSession) getNetworkManager() *NetworkManager {
	return fs.networkManager
}

func (fs *FrameSession) initDomains() error {
	actions := []Action{
		// TODO: can we get rid of the following by doing DOM related stuff in JS instead?
		dom.Enable(),
		cdplog.Enable(),
		cdpruntime.Enable(),
		target.SetAutoAttach(true, true).WithFlatten(true),
	}
	for _, action := range actions {
		if err := action.Do(cdp.WithExecutor(fs.ctx, fs.session)); err != nil {
			return fmt.Errorf("internal error while enabling %T: %w", action, err)
		}
	}
	return nil
}

//nolint:cyclop
func (fs *FrameSession) initEvents() {
	fs.logger.Debugf("NewFrameSession:initEvents",
		"sid:%v tid:%v", fs.session.ID(), fs.targetID)

	events := []string{
		cdproto.EventInspectorTargetCrashed,
	}
	fs.session.on(fs.ctx, events, fs.eventCh)
	if !fs.isMainFrame() {
		fs.initRendererEvents()
	}

	go func() {
		fs.logger.Debugf("NewFrameSession:initEvents:go",
			"sid:%v tid:%v", fs.session.ID(), fs.targetID)
		defer func() {
			// If there is an active span for main frame,
			// end it before exiting so it can be flushed
			if fs.mainFrameSpan != nil {
				// The url needs to be added here instead of at the start of the span
				// because at the start of the span we don't know the correct url for
				// the page we're navigating to. At the end of the span we do have this
				// information.
				fs.mainFrameSpan.SetAttributes(attribute.String("navigation.url", fs.manager.MainFrameURL()))
				fs.mainFrameSpan.End()
				fs.mainFrameSpan = nil
			}
			fs.logger.Debugf("NewFrameSession:initEvents:go:return",
				"sid:%v tid:%v", fs.session.ID(), fs.targetID)
		}()

		for {
			select {
			case <-fs.session.Done():
				fs.logger.Debugf("FrameSession:initEvents:go:session.done",
					"sid:%v tid:%v", fs.session.ID(), fs.targetID)
				return
			case <-fs.ctx.Done():
				fs.logger.Debugf("FrameSession:initEvents:go:ctx.Done",
					"sid:%v tid:%v", fs.session.ID(), fs.targetID)

				return
			case event := <-fs.eventCh:
				switch ev := event.data.(type) {
				case *inspector.EventTargetCrashed:
					fs.onTargetCrashed()
				case *cdplog.EventEntryAdded:
					fs.onLogEntryAdded(ev)
				case *cdppage.EventFrameAttached:
					fs.onFrameAttached(ev.FrameID, ev.ParentFrameID)
				case *cdppage.EventFrameDetached:
					fs.onFrameDetached(ev.FrameID, ev.Reason)
				case *cdppage.EventFrameNavigated:
					const initial = false
					fs.onFrameNavigated(ev.Frame, initial)
				case *cdppage.EventFrameRequestedNavigation:
					fs.onFrameRequestedNavigation(ev)
				case *cdppage.EventFrameStartedLoading:
					fs.onFrameStartedLoading(ev.FrameID)
				case *cdppage.EventFrameStoppedLoading:
					fs.onFrameStoppedLoading(ev.FrameID)
				case *cdppage.EventLifecycleEvent:
					fs.onPageLifecycle(ev)
				case *cdppage.EventNavigatedWithinDocument:
					fs.onPageNavigatedWithinDocument(ev)
				case *cdpruntime.EventConsoleAPICalled:
					fs.onConsoleAPICalled(ev)
				case *cdpruntime.EventExceptionThrown:
					fs.onExceptionThrown(ev)
				case *cdpruntime.EventExecutionContextCreated:
					fs.onExecutionContextCreated(ev)
				case *cdpruntime.EventExecutionContextDestroyed:
					fs.onExecutionContextDestroyed(ev.ExecutionContextID)
				case *cdpruntime.EventExecutionContextsCleared:
					fs.onExecutionContextsCleared()
				case *target.EventAttachedToTarget:
					fs.onAttachedToTarget(ev)
				case *target.EventDetachedFromTarget:
					fs.onDetachedFromTarget(ev)
				case *cdppage.EventJavascriptDialogOpening:
					fs.onEventJavascriptDialogOpening(ev)
				case *cdpruntime.EventBindingCalled:
					fs.onEventBindingCalled(ev)
				}
			}
		}
	}()
}

func (fs *FrameSession) onEventBindingCalled(event *cdpruntime.EventBindingCalled) {
	fs.logger.Debugf("FrameSessions:onEventBindingCalled",
		"sid:%v tid:%v name:%s payload:%s",
		fs.session.ID(), fs.targetID, event.Name, event.Payload)

	err := fs.parseAndEmitWebVitalMetric(event.Payload)
	if err != nil {
		fs.logger.Errorf("FrameSession:onEventBindingCalled", "failed to emit web vital metric: %v", err)
	}
}

func (fs *FrameSession) parseAndEmitWebVitalMetric(object string) error {
	fs.logger.Debugf("FrameSession:parseAndEmitWebVitalMetric", "object:%s", object)

	wv := struct {
		ID             string
		Name           string
		Value          json.Number
		Rating         string
		Delta          json.Number
		NumEntries     json.Number
		NavigationType string
		URL            string
		SpanID         string
	}{}

	if err := json.Unmarshal([]byte(object), &wv); err != nil {
		return fmt.Errorf("json couldn't be parsed: %w", err)
	}

	metric, ok := fs.k6Metrics.WebVitals[wv.Name]
	if !ok {
		return fmt.Errorf("metric not registered %q", wv.Name)
	}

	value, err := wv.Value.Float64()
	if err != nil {
		return fmt.Errorf("value couldn't be parsed %q", wv.Value)
	}

	state := fs.vu.State()
	tags := state.Tags.GetCurrentValues().Tags
	if state.Options.SystemTags.Has(k6metrics.TagURL) {
		tags = handleURLTag(fs.page, wv.URL, http.MethodGet, tags)
	}

	tags = tags.With("rating", wv.Rating)

	now := time.Now()
	k6metrics.PushIfNotDone(fs.vu.Context(), state.Samples, k6metrics.ConnectedSamples{
		Samples: []k6metrics.Sample{
			{
				TimeSeries: k6metrics.TimeSeries{Metric: metric, Tags: tags},
				Value:      value,
				Time:       now,
			},
		},
	})

	_, span := TraceEvent(
		fs.ctx, fs.targetID.String(), "web_vital", wv.SpanID, trace.WithAttributes(
			attribute.String("web_vital.name", wv.Name),
			attribute.Float64("web_vital.value", value),
			attribute.String("web_vital.rating", wv.Rating),
		))
	defer span.End()

	return nil
}

func (fs *FrameSession) onEventJavascriptDialogOpening(event *cdppage.EventJavascriptDialogOpening) {
	fs.logger.Debugf("FrameSession:onEventJavascriptDialogOpening",
		"sid:%v tid:%v url:%v dialogType:%s",
		fs.session.ID(), fs.targetID, event.URL, event.Type)

	// Dialog type of beforeunload needs to accept the
	// dialog, instead of dismissing it. We're unable to
	// dismiss beforeunload dialog boxes at the moment as
	// it seems to pause the exec of any other action on
	// the page. I believe this is an issue in Chromium.
	action := cdppage.HandleJavaScriptDialog(false)
	if event.Type == cdppage.DialogTypeBeforeunload {
		action = cdppage.HandleJavaScriptDialog(true)
	}

	if err := action.Do(cdp.WithExecutor(fs.ctx, fs.session)); err != nil {
		fs.logger.Errorf("FrameSession:onEventJavascriptDialogOpening", "failed to dismiss dialog box: %v", err)
	}
}

func (fs *FrameSession) initFrameTree() error {
	fs.logger.Debugf("NewFrameSession:initFrameTree",
		"sid:%v tid:%v", fs.session.ID(), fs.targetID)

	action := cdppage.Enable()
	if err := action.Do(cdp.WithExecutor(fs.ctx, fs.session)); err != nil {
		return fmt.Errorf("enabling page domain: %w", err)
	}

	var frameTree *cdppage.FrameTree
	var err error

	// Recursively enumerate all existing frames in page to create initial in-memory structures
	// used for access and manipulation from JS.
	action2 := cdppage.GetFrameTree()
	if frameTree, err = action2.Do(cdp.WithExecutor(fs.ctx, fs.session)); err != nil {
		return fmt.Errorf("getting page frame tree: %w", err)
	} else if frameTree == nil {
		// This can happen with very short scripts when we might not have enough
		// time to initialize properly.
		return fmt.Errorf("got a nil page frame tree")
	}

	// Any new frame may have a child frame, not just mainframes.
	fs.handleFrameTree(frameTree, fs.isMainFrame())

	if fs.isMainFrame() {
		fs.initRendererEvents()
	}
	return nil
}

func (fs *FrameSession) initIsolatedWorld(name string) error {
	fs.logger.Debugf("NewFrameSession:initIsolatedWorld",
		"sid:%v tid:%v", fs.session.ID(), fs.targetID)

	action := cdppage.SetLifecycleEventsEnabled(true)
	if err := action.Do(cdp.WithExecutor(fs.ctx, fs.session)); err != nil {
		return fmt.Errorf("enabling page lifecycle events: %w", err)
	}

	if _, ok := fs.isolatedWorlds[name]; ok {
		fs.logger.Debugf("NewFrameSession:initIsolatedWorld",
			"sid:%v tid:%v, not found: %q",
			fs.session.ID(), fs.targetID, name)

		return nil
	}
	fs.isolatedWorlds[name] = true

	var frames []*Frame
	if fs.isMainFrame() {
		frames = fs.manager.Frames()
	} else {
		frame, ok := fs.manager.getFrameByID(cdp.FrameID(fs.targetID))
		if ok {
			frames = []*Frame{frame}
		}
	}
	for _, frame := range frames {
		// A frame could have been removed before we execute this, so don't wait around for a reply.
		_ = fs.session.ExecuteWithoutExpectationOnReply(
			fs.ctx,
			cdppage.CommandCreateIsolatedWorld,
			&cdppage.CreateIsolatedWorldParams{
				FrameID:             cdp.FrameID(frame.ID()),
				WorldName:           name,
				GrantUniveralAccess: true,
			},
			nil)
	}

	fs.logger.Debugf("NewFrameSession:initIsolatedWorld:AddScriptToEvaluateOnNewDocument",
		"sid:%v tid:%v", fs.session.ID(), fs.targetID)

	action2 := cdppage.AddScriptToEvaluateOnNewDocument(`//# sourceURL=` + evaluationScriptURL).
		WithWorldName(name)
	if _, err := action2.Do(cdp.WithExecutor(fs.ctx, fs.session)); err != nil {
		return fmt.Errorf("adding script to evaluate on new document: %w", err)
	}
	return nil
}

func (fs *FrameSession) initOptions() error {
	fs.logger.Debugf("NewFrameSession:initOptions",
		"sid:%v tid:%v", fs.session.ID(), fs.targetID)

	var (
		opts       = fs.manager.page.browserCtx.opts
		optActions = []Action{}
		state      = fs.vu.State()
	)

	if fs.isMainFrame() {
		optActions = append(optActions, emulation.SetFocusEmulationEnabled(true))
		if err := fs.updateViewport(); err != nil {
			fs.logger.Debugf("NewFrameSession:initOptions:updateViewport",
				"sid:%v tid:%v, err:%v",
				fs.session.ID(), fs.targetID, err)
			return err
		}
	}
	if opts.BypassCSP {
		optActions = append(optActions, cdppage.SetBypassCSP(true))
	}
	if opts.IgnoreHTTPSErrors {
		optActions = append(optActions, security.SetIgnoreCertificateErrors(true))
	}
	if opts.HasTouch {
		optActions = append(optActions, emulation.SetTouchEmulationEnabled(true))
	}
	if !opts.JavaScriptEnabled {
		optActions = append(optActions, emulation.SetScriptExecutionDisabled(true))
	}
	if opts.UserAgent != "" || opts.Locale != "" {
		optActions = append(optActions, emulation.SetUserAgentOverride(opts.UserAgent).WithAcceptLanguage(opts.Locale))
	}
	if opts.Locale != "" {
		if err := fs.emulateLocale(); err != nil {
			return err
		}
	}
	if opts.TimezoneID != "" {
		if err := fs.emulateTimezone(); err != nil {
			return err
		}
	}
	if err := fs.updateGeolocation(true); err != nil {
		return err
	}
	if err := fs.updateExtraHTTPHeaders(true); err != nil {
		return err
	}

	var reqIntercept bool
	if state.Options.BlockedHostnames.Trie != nil ||
		len(state.Options.BlacklistIPs) > 0 {
		reqIntercept = true
	}
	if err := fs.updateRequestInterception(reqIntercept); err != nil {
		return err
	}

	if err := fs.updateOffline(true); err != nil {
		return err
	}
	if err := fs.updateHTTPCredentials(true); err != nil {
		return err
	}
	if err := fs.updateEmulateMedia(); err != nil {
		return err
	}

	for _, action := range optActions {
		if err := action.Do(cdp.WithExecutor(fs.ctx, fs.session)); err != nil {
			return fmt.Errorf("internal error while initializing frame %T: %w", action, err)
		}
	}

	return nil
}

func (fs *FrameSession) initRendererEvents() {
	fs.logger.Debugf("NewFrameSession:initEvents:initRendererEvents",
		"sid:%v tid:%v", fs.session.ID(), fs.targetID)

	events := []string{
		cdproto.EventLogEntryAdded,
		cdproto.EventPageFileChooserOpened,
		cdproto.EventPageFrameAttached,
		cdproto.EventPageFrameDetached,
		cdproto.EventPageFrameNavigated,
		cdproto.EventPageFrameRequestedNavigation,
		cdproto.EventPageFrameStartedLoading,
		cdproto.EventPageFrameStoppedLoading,
		cdproto.EventPageJavascriptDialogOpening,
		cdproto.EventPageLifecycleEvent,
		cdproto.EventPageNavigatedWithinDocument,
		cdproto.EventRuntimeConsoleAPICalled,
		cdproto.EventRuntimeExceptionThrown,
		cdproto.EventRuntimeExecutionContextCreated,
		cdproto.EventRuntimeExecutionContextDestroyed,
		cdproto.EventRuntimeExecutionContextsCleared,
		cdproto.EventTargetAttachedToTarget,
		cdproto.EventTargetDetachedFromTarget,
		cdproto.EventRuntimeBindingCalled,
	}
	fs.session.on(fs.ctx, events, fs.eventCh)
}

func (fs *FrameSession) isMainFrame() bool {
	return fs.targetID == fs.page.targetID
}

func (fs *FrameSession) handleFrameTree(frameTree *cdppage.FrameTree, initialFrame bool) {
	fs.logger.Debugf("FrameSession:handleFrameTree",
		"fid:%v sid:%v tid:%v", frameTree.Frame.ID, fs.session.ID(), fs.targetID)

	if frameTree.Frame.ParentID != "" {
		fs.onFrameAttached(frameTree.Frame.ID, frameTree.Frame.ParentID)
	}
	fs.onFrameNavigated(frameTree.Frame, initialFrame)
	if frameTree.ChildFrames == nil {
		return
	}
	for _, child := range frameTree.ChildFrames {
		fs.handleFrameTree(child, initialFrame)
	}
}

func (fs *FrameSession) navigateFrame(frame *Frame, url, referrer string) (string, error) {
	fs.logger.Debugf("FrameSession:navigateFrame",
		"sid:%v fid:%s tid:%v url:%q referrer:%q",
		fs.session.ID(), frame.ID(), fs.targetID, url, referrer)

	action := cdppage.Navigate(url).WithReferrer(referrer).WithFrameID(cdp.FrameID(frame.ID()))
	_, documentID, errorText, err := action.Do(cdp.WithExecutor(fs.ctx, fs.session))
	if err != nil {
		if errorText == "" {
			err = fmt.Errorf("%w", err)
		} else {
			err = fmt.Errorf("%q: %w", errorText, err)
		}
	}
	return documentID.String(), err
}

func (fs *FrameSession) onConsoleAPICalled(event *cdpruntime.EventConsoleAPICalled) {
	l := fs.logger.
		WithTime(event.Timestamp.Time()).
		WithField("source", "browser").
		WithField("browser_source", "console-api")

	/* accessing the state Group while not on the eventloop is racy
	if s := fs.vu.State(); s.Group.Path != "" {
		l = l.WithField("group", s.Group.Path)
	}
	*/

	parsedObjects := make([]string, 0, len(event.Args))
	for _, robj := range event.Args {
		s, err := parseConsoleRemoteObject(fs.logger, robj)
		if err != nil {
			fs.logger.Errorf("onConsoleAPICalled", "failed to parse console message %v", err)
		}
		parsedObjects = append(parsedObjects, s)
	}

	msg := strings.Join(parsedObjects, " ")

	// this is where debug & other console.* apis will default to (such as
	// console.table).
	l.Debug(msg)
}

// We should consider building an API around this as it could be useful
// information about the user's website not handling exceptions.
func (fs *FrameSession) onExceptionThrown(event *cdpruntime.EventExceptionThrown) {
	fs.logger.Debugf("FrameSession:onExceptionThrown",
		"sid:%v tid:%v url:%s line:%d col:%d text:%s",
		fs.session.ID(), fs.targetID, event.ExceptionDetails.URL,
		event.ExceptionDetails.LineNumber, event.ExceptionDetails.ColumnNumber,
		event.ExceptionDetails.Text)
}

func (fs *FrameSession) onExecutionContextCreated(event *cdpruntime.EventExecutionContextCreated) {
	fs.logger.Debugf("FrameSession:onExecutionContextCreated",
		"sid:%v tid:%v ectxid:%d",
		fs.session.ID(), fs.targetID, event.Context.ID)

	auxData := event.Context.AuxData
	var i struct {
		FrameID   cdp.FrameID `json:"frameId"`
		IsDefault bool        `json:"isDefault"`
		Type      string      `json:"type"`
	}
	if err := json.Unmarshal(auxData, &i); err != nil {
		k6ext.Panic(fs.ctx, "unmarshaling executionContextCreated event JSON: %w", err)
	}

	frame, ok := fs.manager.getFrameByID(i.FrameID)
	if !ok {
		fs.logger.Debugf("FrameSession:onExecutionContextCreated:return",
			"sid:%v tid:%v ectxid:%d missing frame",
			fs.session.ID(), fs.targetID, event.Context.ID)
		return
	}

	var world executionWorld
	if i.IsDefault {
		world = mainWorld
	} else if event.Context.Name == utilityWorldName && !frame.hasContext(utilityWorld) {
		// In case of multiple sessions to the same target, there's a race between
		// connections so we might end up creating multiple isolated worlds.
		// We can use either.
		world = utilityWorld
	}

	if i.Type == "isolated" {
		fs.isolatedWorlds[event.Context.Name] = true
	}
	context := NewExecutionContext(fs.ctx, fs.session, frame, event.Context.ID, fs.logger)
	if world != "" {
		fs.logger.Debugf("FrameSession:setContext",
			"sid:%v fid:%v ectxid:%d",
			fs.session.ID(), frame.ID(), event.Context.ID)
		frame.setContext(world, context)
	}
	fs.contextIDToContextMu.Lock()
	fs.contextIDToContext[event.Context.ID] = context
	fs.contextIDToContextMu.Unlock()
}

func (fs *FrameSession) onExecutionContextDestroyed(execCtxID cdpruntime.ExecutionContextID) {
	fs.logger.Debugf("FrameSession:onExecutionContextDestroyed",
		"sid:%v tid:%v ectxid:%d",
		fs.session.ID(), fs.targetID, execCtxID)

	fs.contextIDToContextMu.Lock()
	defer fs.contextIDToContextMu.Unlock()
	context, ok := fs.contextIDToContext[execCtxID]
	if !ok {
		return
	}
	if context.Frame() != nil {
		context.Frame().nullContext(execCtxID)
	}
	delete(fs.contextIDToContext, execCtxID)
}

func (fs *FrameSession) onExecutionContextsCleared() {
	fs.logger.Debugf("FrameSession:onExecutionContextsCleared",
		"sid:%v tid:%v", fs.session.ID(), fs.targetID)

	fs.contextIDToContextMu.Lock()
	defer fs.contextIDToContextMu.Unlock()

	for _, context := range fs.contextIDToContext {
		if context.Frame() != nil {
			context.Frame().nullContext(context.id)
		}
	}
	for k := range fs.contextIDToContext {
		delete(fs.contextIDToContext, k)
	}
}

func (fs *FrameSession) onFrameAttached(frameID cdp.FrameID, parentFrameID cdp.FrameID) {
	fs.logger.Debugf("FrameSession:onFrameAttached",
		"sid:%v tid:%v fid:%v pfid:%v",
		fs.session.ID(), fs.targetID, frameID, parentFrameID)

	// TODO: add handling for cross-process frame transitioning
	fs.manager.frameAttached(frameID, parentFrameID)
}

func (fs *FrameSession) onFrameDetached(frameID cdp.FrameID, reason cdppage.FrameDetachedReason) {
	fs.logger.Debugf("FrameSession:onFrameDetached",
		"sid:%v tid:%v fid:%v reason:%s",
		fs.session.ID(), fs.targetID, frameID, reason)

	if err := fs.manager.frameDetached(frameID, reason); err != nil {
		k6ext.Panic(fs.ctx, "handling frameDetached event: %w", err)
	}
}

func (fs *FrameSession) onFrameNavigated(frame *cdp.Frame, initial bool) {
	fs.logger.Debugf("FrameSession:onFrameNavigated",
		"sid:%v tid:%v fid:%v",
		fs.session.ID(), fs.targetID, frame.ID)

	err := fs.manager.frameNavigated(
		frame.ID, frame.ParentID, frame.LoaderID.String(),
		frame.Name, frame.URL+frame.URLFragment, initial)
	if err != nil {
		k6ext.Panic(fs.ctx, "handling frameNavigated event to %q: %w",
			frame.URL+frame.URLFragment, err)
	}

	// Only create a navigation span once from here, since a new page navigating
	// to about:blank doesn't call onFrameStartedLoading. All subsequent
	// navigations call onFrameStartedLoading.
	if fs.initialNavDone {
		return
	}

	fs.initialNavDone = true
	fs.processNavigationSpan(frame.ID)
}

func (fs *FrameSession) processNavigationSpan(id cdp.FrameID) {
	newFrame, ok := fs.manager.getFrameByID(id)
	if !ok {
		return
	}

	// Trace navigation only for the main frame.
	// TODO: How will this affect sub frames such as iframes?
	if newFrame.page.frameManager.MainFrame() != newFrame {
		return
	}

	// End the navigation span if it is non-nil
	if fs.mainFrameSpan != nil {
		// The url needs to be added here instead of at the start of the span
		// because at the start of the span we don't know the correct url for
		// the page we're navigating to. At the end of the span we do have this
		// information.
		fs.mainFrameSpan.SetAttributes(attribute.String("navigation.url", fs.manager.MainFrameURL()))
		fs.mainFrameSpan.End()
	}

	_, fs.mainFrameSpan = TraceNavigation(
		fs.ctx, fs.targetID.String(),
	)

	spanID := fs.mainFrameSpan.SpanContext().SpanID().String()

	// Set k6SpanId property in the page so it can be retrieved when pushing
	// the Web Vitals events from the page execution context and used to
	// correlate them with the navigation span to which they belong to.
	setSpanIDProp := func() {
		js := fmt.Sprintf("window.k6SpanId = '%s';", spanID)
		err := newFrame.EvaluateGlobal(fs.ctx, js)
		if err != nil {
			fs.logger.Errorf(
				"FrameSession:onFrameNavigated", "error on evaluating window.k6SpanId: %v", err,
			)
		}
	}

	// Executing a CDP command in the event parsing goroutine might deadlock in some cases.
	// For example a deadlock happens if the content loaded in the frame that has navigated
	// includes a JavaScript initiated dialog which we have to explicitly accept or dismiss
	// (see onEventJavascriptDialogOpening). In that case our EvaluateGlobal call can't be
	// executed, as the browser is waiting for us to accept/dismiss the JS dialog, but we
	// can't act on that because the event parsing goroutine is stuck in onFrameNavigated.
	// Because in this case the action is to set an attribute to the global object (window)
	// it should be safe to just execute this in a separate goroutine.
	go setSpanIDProp()
}

func (fs *FrameSession) onFrameRequestedNavigation(event *cdppage.EventFrameRequestedNavigation) {
	fs.logger.Debugf("FrameSession:onFrameRequestedNavigation",
		"sid:%v tid:%v fid:%v url:%q",
		fs.session.ID(), fs.targetID, event.FrameID, event.URL)

	if event.Disposition == "currentTab" {
		err := fs.manager.frameRequestedNavigation(event.FrameID, event.URL, "")
		if err != nil {
			k6ext.Panic(fs.ctx, "handling frameRequestedNavigation event to %q: %w", event.URL, err)
		}
	}
}

func (fs *FrameSession) onFrameStartedLoading(frameID cdp.FrameID) {
	fs.logger.Debugf("FrameSession:onFrameStartedLoading",
		"sid:%v tid:%v fid:%v",
		fs.session.ID(), fs.targetID, frameID)

	fs.processNavigationSpan(frameID)

	fs.manager.frameLoadingStarted(frameID)
}

func (fs *FrameSession) onFrameStoppedLoading(frameID cdp.FrameID) {
	fs.logger.Debugf("FrameSession:onFrameStoppedLoading",
		"sid:%v tid:%v fid:%v",
		fs.session.ID(), fs.targetID, frameID)

	fs.manager.frameLoadingStopped(frameID)
}

func (fs *FrameSession) onLogEntryAdded(event *cdplog.EventEntryAdded) {
	l := fs.logger.
		WithTime(event.Entry.Timestamp.Time()).
		WithField("source", "browser").
		WithField("url", event.Entry.URL).
		WithField("browser_source", event.Entry.Source.String()).
		WithField("line_number", event.Entry.LineNumber)

	/* accessing the state Group while not on the eventloop is racy
	if s := fs.vu.State(); s.Group.Path != "" {
		l = l.WithField("group", s.Group.Path)
	}
	*/

	l.Debug(event.Entry.Text)
}

func (fs *FrameSession) onPageLifecycle(event *cdppage.EventLifecycleEvent) {
	fs.logger.Debugf("FrameSession:onPageLifecycle",
		"sid:%v tid:%v fid:%v event:%s eventTime:%q",
		fs.session.ID(), fs.targetID, event.FrameID, event.Name, event.Timestamp.Time())

	_, ok := fs.manager.getFrameByID(event.FrameID)
	if !ok {
		return
	}

	switch event.Name {
	case "load":
		fs.manager.frameLifecycleEvent(event.FrameID, LifecycleEventLoad)
	case "DOMContentLoaded":
		fs.manager.frameLifecycleEvent(event.FrameID, LifecycleEventDOMContentLoad)
	case "networkIdle":
		fs.manager.frameLifecycleEvent(event.FrameID, LifecycleEventNetworkIdle)
	}
}

func (fs *FrameSession) onPageNavigatedWithinDocument(event *cdppage.EventNavigatedWithinDocument) {
	fs.logger.Debugf("FrameSession:onPageNavigatedWithinDocument",
		"sid:%v tid:%v fid:%v",
		fs.session.ID(), fs.targetID, event.FrameID)

	fs.manager.frameNavigatedWithinDocument(event.FrameID, event.URL)
}

func (fs *FrameSession) onAttachedToTarget(event *target.EventAttachedToTarget) {
	var (
		ti  = event.TargetInfo
		sid = event.SessionID
		err error
	)

	fs.logger.Debugf("FrameSession:onAttachedToTarget",
		"sid:%v tid:%v esid:%v etid:%v ebctxid:%v type:%q",
		fs.session.ID(), fs.targetID, event.SessionID,
		event.TargetInfo.TargetID, event.TargetInfo.BrowserContextID,
		event.TargetInfo.Type)

	session := fs.page.browserCtx.getSession(event.SessionID)
	if session == nil {
		fs.logger.Debugf("FrameSession:onAttachedToTarget:NewFrameSession",
			"sid:%v tid:%v esid:%v etid:%v ebctxid:%v type:%q err:nil session",
			fs.session.ID(), fs.targetID, event.SessionID,
			event.TargetInfo.TargetID, event.TargetInfo.BrowserContextID,
			event.TargetInfo.Type)
		return
	}

	switch ti.Type {
	case "iframe":
		err = fs.attachIFrameToTarget(ti, session)
	case "worker":
		err = fs.attachWorkerToTarget(ti, session)
	default:
		// Just unblock (debugger continue) these targets and detach from them.
		_ = session.ExecuteWithoutExpectationOnReply(fs.ctx, cdpruntime.CommandRunIfWaitingForDebugger, nil, nil)
		_ = session.ExecuteWithoutExpectationOnReply(fs.ctx, target.CommandDetachFromTarget,
			&target.DetachFromTargetParams{SessionID: session.id}, nil)
	}
	if err == nil {
		return
	}
	// Handle or ignore errors.
	var reason string
	defer func() {
		fs.logger.Debugf("FrameSession:onAttachedToTarget:return",
			"sid:%v tid:%v esid:%v etid:%v ebctxid:%v type:%q reason:%s",
			fs.session.ID(), fs.targetID, sid,
			ti.TargetID, ti.BrowserContextID,
			ti.Type, reason)
	}()
	// Ignore errors if we're no longer connected to browser.
	// This happens when there is no browser but we still want to
	// attach a frame/worker to it.
	if !fs.page.browserCtx.browser.IsConnected() {
		reason = "browser disconnected"
		return // ignore
	}
	// Final chance:
	// Ignore the error if the context was canceled, otherwise,
	// throw a k6 error.
	select {
	case <-fs.ctx.Done():
		reason = "frame session context canceled"
		return
	case <-session.done:
		reason = "session closed"
		return
	default:
		// Ignore context canceled error to gracefully handle shutting down
		// of the extension. This may happen because of generated events
		// while a frame session is being created.
		if errors.Is(err, context.Canceled) {
			reason = "context canceled"
			return // ignore
		}
		reason = "fatal"
		k6ext.Panic(fs.ctx, "attaching %v: %w", ti.Type, err)
	}
}

// attachIFrameToTarget attaches an IFrame target to a given session.
func (fs *FrameSession) attachIFrameToTarget(ti *target.Info, session *Session) error {
	sid := session.ID()
	fr, ok := fs.manager.getFrameByID(cdp.FrameID(ti.TargetID))
	if !ok {
		// IFrame should be attached to fs.page with EventFrameAttached
		// event before.
		fs.logger.Debugf("FrameSession:attachIFrameToTarget:return",
			"sid:%v tid:%v esid:%v etid:%v ebctxid:%v type:%q, nil frame",
			fs.session.ID(), fs.targetID,
			sid, ti.TargetID, ti.BrowserContextID, ti.Type)
		return nil
	}
	// Remove all children of the previously attached frame.
	err := fs.manager.removeChildFramesRecursively(fr)
	if err != nil {
		fs.logger.Debugf("FrameSession:attachIFrameToTarget:return",
			"sid:%v tid:%v esid:%v etid:%v ebctxid:%v type:%q, can't remove child frames recursively: %q",
			fs.session.ID(), fs.targetID, sid, ti.TargetID, ti.BrowserContextID, ti.Type, err)
	}

	nfs, err := NewFrameSession(
		fs.ctx,
		session,
		fs.page, fs, ti.TargetID,
		fs.logger,
		false)
	if err != nil {
		return fmt.Errorf("attaching iframe target ID %v to session ID %v: %w",
			ti.TargetID, sid, err)
	}

	if err := fs.page.attachFrameSession(cdp.FrameID(ti.TargetID), nfs); err != nil {
		return err
	}

	return nil
}

// attachWorkerToTarget attaches a Worker target to a given session.
func (fs *FrameSession) attachWorkerToTarget(ti *target.Info, session *Session) error {
	w, err := NewWorker(fs.ctx, session, ti.TargetID, ti.URL)
	if err != nil {
		return fmt.Errorf("attaching worker target ID %v to session ID %v: %w",
			ti.TargetID, session.ID(), err)
	}
	fs.page.workers[session.ID()] = w

	return nil
}

func (fs *FrameSession) onDetachedFromTarget(event *target.EventDetachedFromTarget) {
	fs.logger.Debugf("FrameSession:onDetachedFromTarget",
		"sid:%v tid:%v esid:%v",
		fs.session.ID(), fs.targetID, event.SessionID)

	fs.page.removeWorker(event.SessionID)
}

func (fs *FrameSession) onTargetCrashed() {
	fs.logger.Debugf("FrameSession:onTargetCrashed", "sid:%v tid:%v", fs.session.ID(), fs.targetID)

	// TODO:?
	s, ok := fs.session.(*Session)
	if !ok {
		k6ext.Panic(fs.ctx, "unexpected type %T", fs.session)
	}
	s.markAsCrashed()
}

func (fs *FrameSession) updateEmulateMedia() error {
	fs.logger.Debugf("NewFrameSession:updateEmulateMedia", "sid:%v tid:%v", fs.session.ID(), fs.targetID)

	features := make([]*emulation.MediaFeature, 0)

	switch fs.page.colorScheme {
	case ColorSchemeLight:
		features = append(features, &emulation.MediaFeature{Name: "prefers-color-scheme", Value: "light"})
	case ColorSchemeDark:
		features = append(features, &emulation.MediaFeature{Name: "prefers-color-scheme", Value: "dark"})
	default:
		features = append(features, &emulation.MediaFeature{Name: "prefers-color-scheme", Value: ""})
	}

	switch fs.page.reducedMotion {
	case ReducedMotionReduce:
		features = append(features, &emulation.MediaFeature{Name: "prefers-reduced-motion", Value: "reduce"})
	default:
		features = append(features, &emulation.MediaFeature{Name: "prefers-reduced-motion", Value: ""})
	}

	action := emulation.SetEmulatedMedia().
		WithMedia(string(fs.page.mediaType)).
		WithFeatures(features)
	if err := action.Do(cdp.WithExecutor(fs.ctx, fs.session)); err != nil {
		return fmt.Errorf("internal error while updating emulated media: %w", err)
	}
	return nil
}

func (fs *FrameSession) updateExtraHTTPHeaders(initial bool) error {
	fs.logger.Debugf("NewFrameSession:updateExtraHTTPHeaders", "sid:%v tid:%v", fs.session.ID(), fs.targetID)

	// Merge extra headers from browser context and page, where page specific headers ake precedence.
	mergedHeaders := make(network.Headers)
	for k, v := range fs.page.browserCtx.opts.ExtraHTTPHeaders {
		mergedHeaders[k] = v
	}
	for k, v := range fs.page.extraHTTPHeaders {
		mergedHeaders[k] = v
	}
	if !initial || len(mergedHeaders) > 0 {
		if err := fs.networkManager.SetExtraHTTPHeaders(mergedHeaders); err != nil {
			return fmt.Errorf("updating extra HTTP headers: %w", err)
		}
	}

	return nil
}

func (fs *FrameSession) updateGeolocation(initial bool) error {
	fs.logger.Debugf("NewFrameSession:updateGeolocation", "sid:%v tid:%v", fs.session.ID(), fs.targetID)

	geolocation := fs.page.browserCtx.opts.Geolocation
	if !initial || geolocation != nil {
		action := emulation.SetGeolocationOverride().
			WithLatitude(geolocation.Latitude).
			WithLongitude(geolocation.Longitude).
			WithAccuracy(geolocation.Accuracy)
		if err := action.Do(cdp.WithExecutor(fs.ctx, fs.session)); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	return nil
}

func (fs *FrameSession) updateHTTPCredentials(initial bool) error {
	fs.logger.Debugf("NewFrameSession:updateHttpCredentials", "sid:%v tid:%v", fs.session.ID(), fs.targetID)

	credentials := fs.page.browserCtx.opts.HTTPCredentials
	if !initial || !credentials.IsEmpty() {
		return fs.networkManager.Authenticate(credentials)
	}

	return nil
}

func (fs *FrameSession) updateOffline(initial bool) error {
	fs.logger.Debugf("NewFrameSession:updateOffline", "sid:%v tid:%v", fs.session.ID(), fs.targetID)

	offline := fs.page.browserCtx.opts.Offline
	if !initial || offline {
		if err := fs.networkManager.SetOfflineMode(offline); err != nil {
			return fmt.Errorf("updating offline mode for frame %v: %w", fs.targetID, err)
		}
	}

	return nil
}

func (fs *FrameSession) throttleNetwork(networkProfile NetworkProfile) error {
	fs.logger.Debugf("NewFrameSession:throttleNetwork", "sid:%v tid:%v", fs.session.ID(), fs.targetID)

	return fs.networkManager.ThrottleNetwork(networkProfile)
}

func (fs *FrameSession) throttleCPU(cpuProfile CPUProfile) error {
	action := emulation.SetCPUThrottlingRate(cpuProfile.Rate)
	if err := action.Do(cdp.WithExecutor(fs.ctx, fs.session)); err != nil {
		return fmt.Errorf("throttling CPU: %w", err)
	}

	return nil
}

func (fs *FrameSession) updateRequestInterception(enable bool) error {
	fs.logger.Debugf("NewFrameSession:updateRequestInterception",
		"sid:%v tid:%v on:%v",
		fs.session.ID(),
		fs.targetID, enable)

	return fs.networkManager.setRequestInterception(enable || fs.page.hasRoutes())
}

func (fs *FrameSession) updateViewport() error {
	fs.logger.Debugf("NewFrameSession:updateViewport", "sid:%v tid:%v", fs.session.ID(), fs.targetID)

	// other frames don't have viewports and,
	// this method shouldn't be called for them.
	// this is just a sanity check.
	if !fs.isMainFrame() {
		err := fmt.Errorf("updateViewport should be called only in the main frame."+
			" (sid:%v tid:%v)", fs.session.ID(), fs.targetID)
		panic(err)
	}

	opts := fs.page.browserCtx.opts
	emulatedSize := fs.page.emulatedSize
	if emulatedSize == nil {
		return nil
	}
	viewport := emulatedSize.Viewport
	screen := emulatedSize.Screen

	orientation := emulation.ScreenOrientation{
		Angle: 0.0,
		Type:  emulation.OrientationTypePortraitPrimary,
	}
	if viewport.Width > viewport.Height {
		orientation.Angle = 90.0
		orientation.Type = emulation.OrientationTypeLandscapePrimary
	}
	action := emulation.SetDeviceMetricsOverride(viewport.Width, viewport.Height, opts.DeviceScaleFactor, opts.IsMobile).
		WithScreenOrientation(&orientation).
		WithScreenWidth(screen.Width).
		WithScreenHeight(screen.Height)
	if err := action.Do(cdp.WithExecutor(fs.ctx, fs.session)); err != nil {
		return fmt.Errorf("emulating viewport: %w", err)
	}

	if fs.hasUIWindow {
		// add an inset to viewport depending on the operating system.
		// this won't add an inset if we're running in headless mode.
		viewport = viewport.recalculateInset(
			fs.page.browserCtx.browser.browserOpts.Headless,
			runtime.GOOS,
		)
		action2 := browser.SetWindowBounds(fs.windowID, &browser.Bounds{
			Width:  viewport.Width,
			Height: viewport.Height,
		})
		if err := action2.Do(cdp.WithExecutor(fs.ctx, fs.session)); err != nil {
			return fmt.Errorf("setting window bounds: %w", err)
		}
	}

	return nil
}

func (fs *FrameSession) executionContextForID(
	executionContextID cdpruntime.ExecutionContextID,
) (*ExecutionContext, error) {
	fs.contextIDToContextMu.Lock()
	defer fs.contextIDToContextMu.Unlock()

	if exc, ok := fs.contextIDToContext[executionContextID]; ok {
		return exc, nil
	}

	return nil, fmt.Errorf("no execution context found for id: %v", executionContextID)
}
