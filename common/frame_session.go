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
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"

	"github.com/grafana/xk6-browser/api"

	k6modules "go.k6.io/k6/js/modules"
	k6metrics "go.k6.io/k6/metrics"

	"github.com/chromedp/cdproto"
	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/inspector"
	"github.com/chromedp/cdproto/log"
	"github.com/chromedp/cdproto/network"
	cdppage "github.com/chromedp/cdproto/page"
	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/security"
	"github.com/chromedp/cdproto/target"
	"github.com/sirupsen/logrus"
)

const utilityWorldName = "__k6_browser_utility_world__"

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
	k6Metrics      *CustomK6Metrics

	targetID target.ID
	windowID browser.WindowID

	// To understand the concepts of Isolated Worlds, Contexts and Frames and
	// the relationship betwween them have a look at the following doc:
	// https://chromium.googlesource.com/chromium/src/+/master/third_party/blink/renderer/bindings/core/v8/V8BindingDesign.md
	contextIDToContextMu sync.Mutex
	contextIDToContext   map[cdpruntime.ExecutionContextID]*ExecutionContext
	isolatedWorlds       map[string]bool

	eventCh chan Event

	childSessions map[cdp.FrameID]*FrameSession
	vu            k6modules.VU

	logger *Logger
	// logger that will properly serialize RemoteObject instances
	serializer *logrus.Logger
}

// NewFrameSession initializes and returns a new FrameSession.
//nolint:funlen
func NewFrameSession(
	ctx context.Context, s session, p *Page, parent *FrameSession, tid target.ID, l *Logger,
) (_ *FrameSession, err error) {
	l.Debugf("NewFrameSession", "sid:%v tid:%v", s.ID(), tid)

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
		vu:                   GetVU(ctx),
		k6Metrics:            GetCustomK6Metrics(ctx),
		logger:               l,
		serializer: &logrus.Logger{
			Out:       l.log.Out,
			Level:     l.log.Level,
			Formatter: &consoleLogFormatter{l.log.Formatter},
		},
	}

	var parentNM *NetworkManager
	if fs.parent != nil {
		parentNM = fs.parent.networkManager
	}
	fs.networkManager, err = NewNetworkManager(ctx, s, fs.manager, parentNM)
	if err != nil {
		l.Debugf("NewFrameSession:NewNetworkManager", "sid:%v tid:%v err:%v",
			s.ID(), tid, err)
		return nil, err
	}

	action := browser.GetWindowForTarget().WithTargetID(fs.targetID)
	if fs.windowID, _, err = action.Do(cdp.WithExecutor(fs.ctx, fs.session)); err != nil {
		l.Debugf(
			"NewFrameSession:GetWindowForTarget",
			"sid:%v tid:%v err:%v",
			s.ID(), tid, err)

		return nil, fmt.Errorf(`unable to get window ID: %w`, err)
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
		return fmt.Errorf(`unable to set locale: %w`, err)
	}
	return nil
}

func (fs *FrameSession) emulateTimezone() error {
	action := emulation.SetTimezoneOverride(fs.page.browserCtx.opts.TimezoneID)
	if err := action.Do(cdp.WithExecutor(fs.ctx, fs.session)); err != nil {
		if strings.Contains(err.Error(), "Timezone override is already in effect") {
			return nil
		}
		return fmt.Errorf(`unable to set timezone ID: %w`, err)
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
		log.Enable(),
		cdpruntime.Enable(),
		target.SetAutoAttach(true, true).WithFlatten(true),
	}
	for _, action := range actions {
		if err := action.Do(cdp.WithExecutor(fs.ctx, fs.session)); err != nil {
			return fmt.Errorf("unable to execute %T: %w", action, err)
		}
	}
	return nil
}

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
		defer fs.logger.Debugf("NewFrameSession:initEvents:go:return",
			"sid:%v tid:%v", fs.session.ID(), fs.targetID)

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
					fs.onTargetCrashed(ev)
				case *log.EventEntryAdded:
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
				}
			}
		}
	}()
}

func (fs *FrameSession) initFrameTree() error {
	fs.logger.Debugf("NewFrameSession:initFrameTree",
		"sid:%v tid:%v", fs.session.ID(), fs.targetID)

	action := cdppage.Enable()
	if err := action.Do(cdp.WithExecutor(fs.ctx, fs.session)); err != nil {
		return fmt.Errorf("unable to enable page domain: %w", err)
	}

	var frameTree *cdppage.FrameTree
	var err error

	// Recursively enumerate all existing frames in page to create initial in-memory structures
	// used for access and manipulation from JS.
	action2 := cdppage.GetFrameTree()
	if frameTree, err = action2.Do(cdp.WithExecutor(fs.ctx, fs.session)); err != nil {
		return fmt.Errorf("unable to get page frame tree: %w", err)
	} else if frameTree == nil {
		// This can happen with very short scripts when we might not have enough
		// time to initialize properly.
		return fmt.Errorf("got a nil page frame tree")
	}

	if fs.isMainFrame() {
		fs.handleFrameTree(frameTree)
		fs.initRendererEvents()
	}
	return nil
}

func (fs *FrameSession) initIsolatedWorld(name string) error {
	fs.logger.Debugf("NewFrameSession:initIsolatedWorld",
		"sid:%v tid:%v", fs.session.ID(), fs.targetID)

	action := cdppage.SetLifecycleEventsEnabled(true)
	if err := action.Do(cdp.WithExecutor(fs.ctx, fs.session)); err != nil {
		return fmt.Errorf(`unable to enable page lifecycle events: %w`, err)
	}

	if _, ok := fs.isolatedWorlds[name]; ok {
		fs.logger.Debugf("NewFrameSession:initIsolatedWorld",
			"sid:%v tid:%v, not found: %q",
			fs.session.ID(), fs.targetID, name)

		return nil
	}
	fs.isolatedWorlds[name] = true

	var frames []api.Frame
	if fs.isMainFrame() {
		frames = fs.manager.Frames()
	} else {
		frames = []api.Frame{fs.manager.getFrameByID(cdp.FrameID(fs.targetID))}
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
		return fmt.Errorf("unable to add script to evaluate for isolated world: %w", err)
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
	fs.updateExtraHTTPHeaders(true)

	var reqIntercept bool
	if state.Options.BlockedHostnames.Trie != nil ||
		len(state.Options.BlacklistIPs) > 0 {
		reqIntercept = true
	}
	if err := fs.updateRequestInterception(reqIntercept); err != nil {
		return err
	}

	fs.updateOffline(true)
	fs.updateHTTPCredentials(true)
	if err := fs.updateEmulateMedia(true); err != nil {
		return err
	}

	// if (screencastOptions)
	//   promises.push(this._startVideoRecording(screencastOptions));

	/*for (const source of this._crPage._browserContext._evaluateOnNewDocumentSources)
	      promises.push(this._evaluateOnNewDocument(source, 'main'));
	  for (const source of this._crPage._page._evaluateOnNewDocumentSources)
	      promises.push(this._evaluateOnNewDocument(source, 'main'));*/

	optActions = append(optActions, cdpruntime.RunIfWaitingForDebugger())

	for _, action := range optActions {
		if err := action.Do(cdp.WithExecutor(fs.ctx, fs.session)); err != nil {
			return fmt.Errorf("unable to execute %T: %w", action, err)
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
	}
	fs.session.on(fs.ctx, events, fs.eventCh)
}

func (fs *FrameSession) isMainFrame() bool {
	return fs.targetID == fs.page.targetID
}

func (fs *FrameSession) handleFrameTree(frameTree *cdppage.FrameTree) {
	fs.logger.Debugf("FrameSession:handleFrameTree",
		"sid:%v tid:%v", fs.session.ID(), fs.targetID)

	if frameTree.Frame.ParentID != "" {
		fs.onFrameAttached(frameTree.Frame.ID, frameTree.Frame.ParentID)
	}
	fs.onFrameNavigated(frameTree.Frame, true)
	if frameTree.ChildFrames == nil {
		return
	}
	for _, child := range frameTree.ChildFrames {
		fs.handleFrameTree(child)
	}
}

func (fs *FrameSession) navigateFrame(frame *Frame, url, referrer string) (string, error) {
	fs.logger.Debugf("FrameSession:navigateFrame",
		"sid:%v tid:%v url:%q referrer:%q",
		fs.session.ID(), fs.targetID, url, referrer)

	action := cdppage.Navigate(url).WithReferrer(referrer).WithFrameID(cdp.FrameID(frame.ID()))
	_, documentID, errorText, err := action.Do(cdp.WithExecutor(fs.ctx, fs.session))
	if err != nil {
		err = fmt.Errorf("%s at %q: %w", errorText, url, err)
	}
	return documentID.String(), err
}

func (fs *FrameSession) onConsoleAPICalled(event *cdpruntime.EventConsoleAPICalled) {
	l := fs.serializer.
		WithTime(event.Timestamp.Time()).
		WithField("source", "browser-console-api")

	if s := fs.vu.State(); s.Group.Path != "" {
		l = l.WithField("group", s.Group.Path)
	}

	var parsedObjects []interface{}
	for _, robj := range event.Args {
		i, err := parseRemoteObject(robj)
		if err != nil {
			handleParseRemoteObjectErr(fs.ctx, err, l)
		}
		parsedObjects = append(parsedObjects, i)
	}

	l = l.WithField("objects", parsedObjects)

	switch event.Type {
	case "log", "info":
		l.Info()
	case "warning":
		l.Warn()
	case "error":
		l.Error()
	default:
		l.Debug()
	}
}

func (fs *FrameSession) onExceptionThrown(event *cdpruntime.EventExceptionThrown) {
	fs.page.emit(EventPageError, event.ExceptionDetails)
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
		k6Throw(fs.ctx, "unable to unmarshal JSON: %w", err)
	}
	var world executionWorld
	frame := fs.manager.getFrameByID(i.FrameID)
	if frame != nil {
		if i.IsDefault {
			world = mainWorld
		} else if event.Context.Name == utilityWorldName && !frame.hasContext(utilityWorld) {
			// In case of multiple sessions to the same target, there's a race between
			// connections so we might end up creating multiple isolated worlds.
			// We can use either.
			world = utilityWorld
		}
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

	fs.manager.frameDetached(frameID)
}

func (fs *FrameSession) onFrameNavigated(frame *cdp.Frame, initial bool) {
	fs.logger.Debugf("FrameSession:onFrameNavigated",
		"sid:%v tid:%v fid:%v",
		fs.session.ID(), fs.targetID, frame.ID)

	err := fs.manager.frameNavigated(frame.ID, frame.ParentID, frame.LoaderID.String(), frame.Name, frame.URL+frame.URLFragment, initial)
	if err != nil {
		k6Throw(fs.ctx, "cannot handle frame navigation: %w", err)
	}
}

func (fs *FrameSession) onFrameRequestedNavigation(event *cdppage.EventFrameRequestedNavigation) {
	fs.logger.Debugf("FrameSession:onFrameRequestedNavigation",
		"sid:%v tid:%v fid:%v url:%q",
		fs.session.ID(), fs.targetID, event.FrameID, event.URL)

	if event.Disposition == "currentTab" {
		err := fs.manager.frameRequestedNavigation(event.FrameID, event.URL, "")
		if err != nil {
			k6Throw(fs.ctx, "cannot handle frame requested navigation: %w", err)
		}
	}
}

func (fs *FrameSession) onFrameStartedLoading(frameID cdp.FrameID) {
	fs.logger.Debugf("FrameSession:onFrameStartedLoading",
		"sid:%v tid:%v fid:%v",
		fs.session.ID(), fs.targetID, frameID)

	fs.manager.frameLoadingStarted(frameID)
}

func (fs *FrameSession) onFrameStoppedLoading(frameID cdp.FrameID) {
	fs.logger.Debugf("FrameSession:onFrameStoppedLoading",
		"sid:%v tid:%v fid:%v",
		fs.session.ID(), fs.targetID, frameID)

	fs.manager.frameLoadingStopped(frameID)
}

func (fs *FrameSession) onLogEntryAdded(event *log.EventEntryAdded) {
	l := fs.logger.log.
		WithTime(event.Entry.Timestamp.Time()).
		WithField("source", "browser").
		WithField("url", event.Entry.URL).
		WithField("browser_source", event.Entry.Source.String()).
		WithField("line_number", event.Entry.LineNumber)
	if s := fs.vu.State(); s.Group.Path != "" {
		l = l.WithField("group", s.Group.Path)
	}
	switch event.Entry.Level {
	case "info":
		l.Info(event.Entry.Text)
	case "warning":
		l.Warn(event.Entry.Text)
	case "error":
		l.WithField("stacktrace", event.Entry.StackTrace).Error(event.Entry.Text)
	default:
		l.Debug(event.Entry.Text)
	}
}

func (fs *FrameSession) onPageLifecycle(event *cdppage.EventLifecycleEvent) {
	fs.logger.Debugf("FrameSession:onPageLifecycle",
		"sid:%v tid:%v fid:%v event:%s eventTime:%q",
		fs.session.ID(), fs.targetID, event.FrameID, event.Name, event.Timestamp.Time())

	frame := fs.manager.getFrameByID(event.FrameID)
	if frame == nil {
		return
	}

	switch event.Name {
	case "init", "commit":
		frame.initTime = event.Timestamp.Time()
		return
	case "load":
		fs.manager.frameLifecycleEvent(event.FrameID, LifecycleEventLoad)
	case "DOMContentLoaded":
		fs.manager.frameLifecycleEvent(event.FrameID, LifecycleEventDOMContentLoad)
	}

	eventToMetric := map[string]*k6metrics.Metric{
		"load":                 fs.k6Metrics.BrowserLoaded,
		"DOMContentLoaded":     fs.k6Metrics.BrowserDOMContentLoaded,
		"firstPaint":           fs.k6Metrics.BrowserFirstPaint,
		"firstContentfulPaint": fs.k6Metrics.BrowserFirstContentfulPaint,
		"firstMeaningfulPaint": fs.k6Metrics.BrowserFirstMeaningfulPaint,
	}

	if m, ok := eventToMetric[event.Name]; ok {
		frame.emitMetric(m, event.Timestamp.Time())
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
		err = fs.attachIFrameToTarget(ti, sid)
	case "worker":
		err = fs.attachWorkerToTarget(ti, sid)
	default:
		// Just unblock (debugger continue) these targets and detach from them.
		s := fs.page.browserCtx.getSession(sid)
		_ = s.ExecuteWithoutExpectationOnReply(fs.ctx, cdpruntime.CommandRunIfWaitingForDebugger, nil, nil)
		_ = s.ExecuteWithoutExpectationOnReply(fs.ctx, target.CommandDetachFromTarget,
			&target.DetachFromTargetParams{SessionID: s.id}, nil)
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
		k6Throw(fs.ctx, "cannot attach %v: %w", ti.Type, err)
	}
}

// attachIFrameToTarget attaches an IFrame target to a given session.
func (fs *FrameSession) attachIFrameToTarget(ti *target.Info, sid target.SessionID) error {
	fr := fs.manager.getFrameByID(cdp.FrameID(ti.TargetID))
	if fr == nil {
		// IFrame should be attached to fs.page with EventFrameAttached
		// event before.
		fs.logger.Debugf("FrameSession:attachIFrameToTarget:return",
			"sid:%v tid:%v esid:%v etid:%v ebctxid:%v type:%q, nil frame",
			fs.session.ID(), fs.targetID,
			sid, ti.TargetID, ti.BrowserContextID, ti.Type)
		return nil
	}
	// Remove all children of the previously attached frame.
	fs.manager.removeChildFramesRecursively(fr)

	nfs, err := NewFrameSession(
		fs.ctx,
		fs.page.browserCtx.getSession(sid),
		fs.page, fs, ti.TargetID,
		fs.logger)
	if err != nil {
		return fmt.Errorf("cannot attach iframe target (%v) to session (%v): %w",
			ti.TargetID, sid, err)
	}
	fs.page.attachFrameSession(cdp.FrameID(ti.TargetID), nfs)

	return nil
}

// attachWorkerToTarget attaches a Worker target to a given session.
func (fs *FrameSession) attachWorkerToTarget(ti *target.Info, sid target.SessionID) error {
	w, err := NewWorker(fs.ctx, fs.page.browserCtx.getSession(sid), ti.TargetID, ti.URL)
	if err != nil {
		return fmt.Errorf("cannot attach worker target (%v) to session (%v): %w",
			ti.TargetID, sid, err)
	}
	fs.page.workers[sid] = w

	return nil
}

func (fs *FrameSession) onDetachedFromTarget(event *target.EventDetachedFromTarget) {
	fs.logger.Debugf("FrameSession:onDetachedFromTarget",
		"sid:%v tid:%v esid:%v",
		fs.session.ID(), fs.targetID, event.SessionID)

	fs.page.closeWorker(event.SessionID)
}

func (fs *FrameSession) onTargetCrashed(event *inspector.EventTargetCrashed) {
	fs.logger.Debugf("FrameSession:onTargetCrashed", "sid:%v tid:%v", fs.session.ID(), fs.targetID)

	// TODO:?
	fs.session.(*Session).markAsCrashed() //nolint:forcetypeassert
	fs.page.didCrash()
}

func (fs *FrameSession) updateEmulateMedia(initial bool) error {
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
		return fmt.Errorf("unable to execute %T: %w", action, err)
	}
	return nil
}

func (fs *FrameSession) updateExtraHTTPHeaders(initial bool) {
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
		fs.networkManager.SetExtraHTTPHeaders(mergedHeaders)
	}
}

func (fs *FrameSession) updateGeolocation(initial bool) error {
	fs.logger.Debugf("NewFrameSession:updateGeolocation", "sid:%v tid:%v", fs.session.ID(), fs.targetID)

	geolocation := fs.page.browserCtx.opts.Geolocation
	if !initial || geolocation != nil {
		action := emulation.SetGeolocationOverride().
			WithLatitude(geolocation.Latitude).
			WithLongitude(geolocation.Longitude).
			WithAccuracy(geolocation.Accurracy)
		if err := action.Do(cdp.WithExecutor(fs.ctx, fs.session)); err != nil {
			return fmt.Errorf("unable to set geolocation override: %w", err)
		}
	}
	return nil
}

func (fs *FrameSession) updateHTTPCredentials(initial bool) {
	fs.logger.Debugf("NewFrameSession:updateHttpCredentials", "sid:%v tid:%v", fs.session.ID(), fs.targetID)

	credentials := fs.page.browserCtx.opts.HttpCredentials
	if !initial || credentials != nil {
		fs.networkManager.Authenticate(credentials)
	}
}

func (fs *FrameSession) updateOffline(initial bool) {
	fs.logger.Debugf("NewFrameSession:updateOffline", "sid:%v tid:%v", fs.session.ID(), fs.targetID)

	offline := fs.page.browserCtx.opts.Offline
	if !initial || offline {
		fs.networkManager.SetOfflineMode(offline)
	}
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
		return fmt.Errorf("unable to emulate viewport: %w", err)
	}

	// add an inset to viewport depending on the operating system.
	// this won't add an inset if we're running in headless mode.
	viewport.calculateInset(
		fs.page.browserCtx.browser.launchOpts.Headless,
		runtime.GOOS,
	)
	action2 := browser.SetWindowBounds(fs.windowID, &browser.Bounds{
		Width:  viewport.Width,
		Height: viewport.Height,
	})
	if err := action2.Do(cdp.WithExecutor(fs.ctx, fs.session)); err != nil {
		return fmt.Errorf("unable to set window bounds: %w", err)
	}

	return nil
}
