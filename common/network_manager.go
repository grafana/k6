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
	"fmt"
	"net"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/chromedp/cdproto"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
	"github.com/dop251/goja"
	k6common "go.k6.io/k6/js/common"
	k6lib "go.k6.io/k6/lib"
	k6types "go.k6.io/k6/lib/types"
	k6stats "go.k6.io/k6/stats"
)

// Ensure NetworkManager implements the EventEmitter interface
var _ EventEmitter = &NetworkManager{}

// NetworkManager manages all frames in HTML document
type NetworkManager struct {
	BaseEventEmitter

	ctx          context.Context
	logger       *Logger
	session      session
	parent       *NetworkManager
	frameManager *FrameManager
	credentials  *Credentials

	// TODO: manage inflight requests seperately (move them between the two maps as they transition from inflight -> completed)
	reqIDToRequest map[network.RequestID]*Request
	reqsMu         sync.RWMutex

	attemptedAuth map[network.RequestID]*Request

	extraHTTPHeaders               map[string]string
	offline                        bool
	userCacheDisabled              bool
	userReqInterceptionEnabled     bool
	protocolReqInterceptionEnabled bool
}

// NewNetworkManager creates a new network manager
func NewNetworkManager(
	ctx context.Context, session *Session, manager *FrameManager, parent *NetworkManager,
) (*NetworkManager, error) {
	state := k6lib.GetState(ctx)
	m := NetworkManager{
		BaseEventEmitter: NewBaseEventEmitter(ctx),
		ctx:              ctx,
		// TODO: Pass an internal logger instead of basing it on k6's logger?
		// See https://github.com/grafana/xk6-browser/issues/54
		logger:                         NewLogger(ctx, state.Logger, false, nil),
		session:                        session,
		parent:                         parent,
		frameManager:                   manager,
		credentials:                    nil,
		reqIDToRequest:                 make(map[network.RequestID]*Request),
		reqsMu:                         sync.RWMutex{},
		attemptedAuth:                  make(map[network.RequestID]*Request),
		extraHTTPHeaders:               make(map[string]string),
		offline:                        false,
		userCacheDisabled:              false,
		userReqInterceptionEnabled:     false,
		protocolReqInterceptionEnabled: false,
	}
	m.initEvents()
	if err := m.initDomains(); err != nil {
		return nil, err
	}
	return &m, nil
}

func (m *NetworkManager) deleteRequestByID(reqID network.RequestID) {
	m.reqsMu.Lock()
	defer m.reqsMu.Unlock()
	delete(m.reqIDToRequest, reqID)
}

func (m *NetworkManager) emitRequestMetrics(req *Request) {
	state := k6lib.GetState(m.ctx)

	tags := state.CloneTags()
	if state.Options.SystemTags.Has(k6stats.TagGroup) {
		tags["group"] = state.Group.Path
	}
	if state.Options.SystemTags.Has(k6stats.TagMethod) {
		tags["method"] = req.method
	}
	if state.Options.SystemTags.Has(k6stats.TagURL) {
		tags["url"] = req.URL()
	}

	sampleTags := k6stats.IntoSampleTags(&tags)
	k6stats.PushIfNotDone(m.ctx, state.Samples, k6stats.ConnectedSamples{
		Samples: []k6stats.Sample{
			{
				Metric: state.BuiltinMetrics.DataSent,
				Tags:   sampleTags,
				Value:  float64(req.Size().Total()),
				Time:   req.timestamp,
			},
		},
	})
}

func (m *NetworkManager) emitResponseMetrics(resp *Response) {
	state := k6lib.GetState(m.ctx)

	tags := state.CloneTags()
	if state.Options.SystemTags.Has(k6stats.TagGroup) {
		tags["group"] = state.Group.Path
	}
	if state.Options.SystemTags.Has(k6stats.TagMethod) {
		tags["method"] = resp.request.method
	}
	if state.Options.SystemTags.Has(k6stats.TagURL) {
		tags["url"] = resp.url
	}
	if state.Options.SystemTags.Has(k6stats.TagIP) {
		tags["ip"] = resp.remoteAddress.IPAddress
	}
	if state.Options.SystemTags.Has(k6stats.TagStatus) {
		tags["status"] = strconv.Itoa(int(resp.status))
	}
	if state.Options.SystemTags.Has(k6stats.TagProto) {
		tags["proto"] = resp.protocol
	}

	tags["from_cache"] = strconv.FormatBool(resp.fromDiskCache)
	tags["from_prefetch_cache"] = strconv.FormatBool(resp.fromPrefetchCache)
	tags["from_service_worker"] = strconv.FormatBool(resp.fromServiceWorker)

	sampleTags := k6stats.IntoSampleTags(&tags)
	k6stats.PushIfNotDone(m.ctx, state.Samples, k6stats.ConnectedSamples{
		Samples: []k6stats.Sample{
			{
				Metric: state.BuiltinMetrics.HTTPReqs,
				Tags:   sampleTags,
				Value:  1,
				Time:   resp.timestamp,
			},
			{
				Metric: state.BuiltinMetrics.HTTPReqDuration,
				Tags:   sampleTags,

				// We're using diff between CDP protocol message timestamps here because the `Network.responseReceived.responseTime`
				// value seems to be in milliseconds rather than seconds as specified in the protocol docs and that causes
				// issues with the parsing and conversion to `time.Time`.
				// Have not spent time looking for the root cause of this in the Chromium source to file a bug report, and neither
				// Puppeteer nor Playwright seems to care about the `responseTime` value and don't use/expose it.
				Value: k6stats.D(resp.timestamp.Sub(resp.request.timestamp)),
				Time:  resp.timestamp,
			},
			{
				Metric: state.BuiltinMetrics.DataReceived,
				Tags:   sampleTags,
				Value:  float64(resp.Size().Total()),
				Time:   resp.timestamp,
			},
		},
	})
	if resp.timing != nil {
		k6stats.PushIfNotDone(m.ctx, state.Samples, k6stats.ConnectedSamples{
			Samples: []k6stats.Sample{
				{
					Metric: state.BuiltinMetrics.HTTPReqConnecting,
					Tags:   sampleTags,
					Value:  k6stats.D(time.Duration(resp.timing.ConnectEnd-resp.timing.ConnectStart) * time.Millisecond),
					Time:   resp.timestamp,
				},
				{
					Metric: state.BuiltinMetrics.HTTPReqTLSHandshaking,
					Tags:   sampleTags,
					Value:  k6stats.D(time.Duration(resp.timing.SslEnd-resp.timing.SslStart) * time.Millisecond),
					Time:   resp.timestamp,
				},
				{
					Metric: state.BuiltinMetrics.HTTPReqSending,
					Tags:   sampleTags,
					Value:  k6stats.D(time.Duration(resp.timing.SendEnd-resp.timing.SendStart) * time.Millisecond),
					Time:   resp.timestamp,
				},
				{
					Metric: state.BuiltinMetrics.HTTPReqReceiving,
					Tags:   sampleTags,
					Value:  k6stats.D(time.Duration(resp.timing.ReceiveHeadersEnd-resp.timing.SendEnd) * time.Millisecond),
					Time:   resp.timestamp,
				},
			},
		})
	}
}

func (m *NetworkManager) handleRequestRedirect(req *Request, redirectResponse *network.Response, timestamp *cdp.MonotonicTime) {
	resp := NewHTTPResponse(m.ctx, req, redirectResponse, timestamp)
	req.response = resp
	req.redirectChain = append(req.redirectChain, req)

	m.deleteRequestByID(req.requestID)
	m.emitResponseMetrics(resp)

	/*
		delete(m.attemptedAuth, req.interceptionID);
	*/

	m.emit(cdproto.EventNetworkResponseReceived, resp)
	m.emit(cdproto.EventNetworkLoadingFinished, req)
}

func (m *NetworkManager) initDomains() error {
	actions := []Action{network.Enable()}

	// Only enable the Fetch domain if necessary, as it has a performance overhead.
	if m.userReqInterceptionEnabled {
		actions = append(actions,
			network.SetCacheDisabled(true),
			fetch.Enable().WithPatterns([]*fetch.RequestPattern{{URLPattern: "*"}}))
	}
	for _, action := range actions {
		if err := action.Do(cdp.WithExecutor(m.ctx, m.session)); err != nil {
			return fmt.Errorf("unable to execute %T: %w", action, err)
		}
	}

	return nil
}

func (m *NetworkManager) initEvents() {
	chHandler := make(chan Event)
	m.session.on(m.ctx, []string{
		cdproto.EventNetworkLoadingFailed,
		cdproto.EventNetworkLoadingFinished,
		cdproto.EventNetworkRequestWillBeSent,
		cdproto.EventNetworkRequestServedFromCache,
		cdproto.EventNetworkResponseReceived,
		cdproto.EventFetchRequestPaused,
	}, chHandler)

	go func() {
		for m.handleEvents(chHandler) {
		}
	}()
}

func (m *NetworkManager) handleEvents(in <-chan Event) bool {
	select {
	case <-m.ctx.Done():
		return false
	case event := <-in:
		select {
		case <-m.ctx.Done():
			return false
		default:
		}
		switch ev := event.data.(type) {
		case *network.EventLoadingFailed:
			m.onLoadingFailed(ev)
		case *network.EventLoadingFinished:
			m.onLoadingFinished(ev)
		case *network.EventRequestWillBeSent:
			m.onRequest(ev, "")
		case *network.EventRequestServedFromCache:
			m.onRequestServedFromCache(ev)
		case *network.EventResponseReceived:
			m.onResponseReceived(ev)
		case *fetch.EventRequestPaused:
			m.onRequestPaused(ev)
		}
	}
	return true
}

func (m *NetworkManager) onLoadingFailed(event *network.EventLoadingFailed) {
	req := m.requestFromID(event.RequestID)
	if req == nil {
		// TODO: add handling of iframe document requests starting in one session and ending up in another
		return
	}
	req.setErrorText(event.ErrorText)
	req.responseEndTiming = float64(event.Timestamp.Time().Unix()-req.timestamp.Unix()) * 1000
	m.deleteRequestByID(event.RequestID)
	m.frameManager.requestFailed(req, event.Canceled)
}

func (m *NetworkManager) onLoadingFinished(event *network.EventLoadingFinished) {
	req := m.requestFromID(event.RequestID)
	if req == nil {
		// Handling of iframe document request starting in parent session and ending up in iframe session.
		if m.parent != nil {
			reqFromParent := m.parent.requestFromID(event.RequestID)

			// Main requests have matching loaderID and requestID.
			if reqFromParent != nil || reqFromParent.getDocumentID() == event.RequestID.String() {
				m.reqsMu.Lock()
				m.reqIDToRequest[event.RequestID] = reqFromParent
				m.reqsMu.Unlock()
				m.parent.deleteRequestByID(event.RequestID)
				req = reqFromParent
			} else {
				return
			}
		} else {
			return
		}
	}
	req.responseEndTiming = float64(event.Timestamp.Time().Unix()-req.timestamp.Unix()) * 1000
	m.deleteRequestByID(event.RequestID)
	m.frameManager.requestFinished(req)
}

func (m *NetworkManager) onRequest(event *network.EventRequestWillBeSent, interceptionID string) {
	var redirectChain []*Request = nil
	if event.RedirectResponse != nil {
		req := m.requestFromID(event.RequestID)
		if req != nil {
			m.handleRequestRedirect(req, event.RedirectResponse, event.Timestamp)
			redirectChain = req.redirectChain
		}
	} else {
		redirectChain = make([]*Request, 0)
	}

	for _, r := range redirectChain {
		m.emitRequestMetrics(r)
	}

	var frame *Frame = nil
	if event.FrameID != "" {
		frame = m.frameManager.getFrameByID(event.FrameID)
	}
	if frame == nil {
		m.logger.Debugf("NetworkManager", "frame is nil")
	}

	req, err := NewRequest(m.ctx, event, frame, redirectChain, interceptionID, m.userReqInterceptionEnabled)
	if err != nil {
		m.logger.Errorf("NetworkManager", "cannot create Request: %s", err)
		return
	}
	if req.url.Scheme == "data" {
		m.logger.Debugf("NetworkManager", "skipped request handling of data URL")
		return
	}
	m.reqsMu.Lock()
	m.reqIDToRequest[event.RequestID] = req
	m.reqsMu.Unlock()
	m.emitRequestMetrics(req)
	m.frameManager.requestStarted(req)
}

func (m *NetworkManager) onRequestPaused(event *fetch.EventRequestPaused) {
	m.logger.Debugf("NetworkManager:onRequestPaused",
		"sid:%s url:%v", m.session.ID(), event.Request.URL)
	defer m.logger.Debugf("NetworkManager:onRequestPaused:return",
		"sid:%s url:%v", m.session.ID(), event.Request.URL)

	var (
		failReason string
		state      = k6lib.GetState(m.ctx)
	)

	defer func() { m.failOrContinueRequest(event, failReason) }()

	purl, err := url.Parse(event.Request.URL)
	if err != nil {
		m.logger.Errorf("NetworkManager:onRequestPaused",
			"error parsing URL: %s", err.Error())
		return
	}

	failReason = handleBlockedHosts(purl, state.Options.BlockedHostnames.Trie)
}

func (m *NetworkManager) failOrContinueRequest(event *fetch.EventRequestPaused, failReason string) {
	if failReason != "" {
		action := fetch.FailRequest(event.RequestID, network.ErrorReasonBlockedByClient)
		if err := action.Do(cdp.WithExecutor(m.ctx, m.session)); err != nil {
			m.logger.Errorf("NetworkManager:onRequestPaused",
				"error interrupting request: %s", err.Error())
		} else {
			m.logger.Warnf("NetworkManager:onRequestPaused",
				"request %s %s was interrupted: %s", event.Request.Method, event.Request.URL, failReason)
			return
		}
	}
	action := fetch.ContinueRequest(event.RequestID)
	if err := action.Do(cdp.WithExecutor(m.ctx, m.session)); err != nil {
		m.logger.Errorf("NetworkManager:onRequestPaused",
			"error continuing request: %s", err.Error())
	}
}

func handleBlockedHosts(u *url.URL, blockedHosts *k6types.HostnameTrie) string {
	ip := net.ParseIP(u.Host)
	if ip == nil {
		if match, blocked := blockedHosts.Contains(u.Host); blocked {
			return fmt.Sprintf("hostname %s is in a blocked pattern (%s)", u.Host, match)
		}
	}
	return ""
}

func (m *NetworkManager) onRequestServedFromCache(event *network.EventRequestServedFromCache) {
	req := m.requestFromID(event.RequestID)
	if req != nil {
		req.setLoadedFromCache(true)
	}
}

func (m *NetworkManager) onResponseReceived(event *network.EventResponseReceived) {
	req := m.requestFromID(event.RequestID)
	if req == nil {
		return
	}
	resp := NewHTTPResponse(m.ctx, req, event.Response, event.Timestamp)
	req.response = resp
	if req.url.Scheme != "data" {
		m.emitResponseMetrics(resp)
	}
	m.frameManager.requestReceivedResponse(resp)
}

func (m *NetworkManager) requestFromID(reqID network.RequestID) *Request {
	m.reqsMu.RLock()
	defer m.reqsMu.RUnlock()
	return m.reqIDToRequest[reqID]
}

func (m *NetworkManager) setRequestInterception(value bool) error {
	m.userReqInterceptionEnabled = value
	return m.updateProtocolRequestInterception()
}

func (m *NetworkManager) updateProtocolCacheDisabled() error {
	action := network.SetCacheDisabled(m.userCacheDisabled)
	if err := action.Do(cdp.WithExecutor(m.ctx, m.session)); err != nil {
		return fmt.Errorf("unable to toggle cache on/off: %w", err)
	}
	return nil
}

func (m *NetworkManager) updateProtocolRequestInterception() error {
	enabled := m.userReqInterceptionEnabled
	if enabled == m.protocolReqInterceptionEnabled {
		return nil
	}
	m.protocolReqInterceptionEnabled = enabled
	if enabled {
		actions := []Action{
			network.SetCacheDisabled(true),
			fetch.Enable().
				WithHandleAuthRequests(true).
				WithPatterns([]*fetch.RequestPattern{
					{URLPattern: "*"},
				}),
		}
		for _, action := range actions {
			if err := action.Do(cdp.WithExecutor(m.ctx, m.session)); err != nil {
				return fmt.Errorf("unable to execute %T: %w", action, err)
			}
		}
	} else {
		actions := []Action{
			network.SetCacheDisabled(false),
			fetch.Disable(),
		}
		for _, action := range actions {
			if err := action.Do(cdp.WithExecutor(m.ctx, m.session)); err != nil {
				return fmt.Errorf("unable to execute %T: %w", action, err)
			}
		}
	}
	return nil
}

// Authenticate sets HTTP authentication credentials to use
func (m *NetworkManager) Authenticate(credentials *Credentials) {
	m.credentials = credentials
	if err := m.updateProtocolRequestInterception(); err != nil {
		rt := k6common.GetRuntime(m.ctx)
		k6common.Throw(rt, err)
	}
}

// ExtraHTTPHeaders returns the currently set extra HTTP request headers
func (m *NetworkManager) ExtraHTTPHeaders() goja.Value {
	rt := k6common.GetRuntime(m.ctx)
	return rt.ToValue(m.extraHTTPHeaders)
}

// SetExtraHTTPHeaders sets extra HTTP request headers to be sent with every request
func (m *NetworkManager) SetExtraHTTPHeaders(headers network.Headers) {
	rt := k6common.GetRuntime(m.ctx)
	action := network.SetExtraHTTPHeaders(headers)
	if err := action.Do(cdp.WithExecutor(m.ctx, m.session)); err != nil {
		k6common.Throw(rt, fmt.Errorf("unable to set extra HTTP headers: %w", err))
	}
}

// SetOfflineMode toggles offline mode on/off
func (m *NetworkManager) SetOfflineMode(offline bool) {
	rt := k6common.GetRuntime(m.ctx)
	if m.offline == offline {
		return
	}
	m.offline = offline

	action := network.EmulateNetworkConditions(m.offline, 0, -1, -1)
	if err := action.Do(cdp.WithExecutor(m.ctx, m.session)); err != nil {
		k6common.Throw(rt, fmt.Errorf("unable to set offline mode: %w", err))
	}
}

// SetUserAgent overrides the browser user agent string
func (m *NetworkManager) SetUserAgent(userAgent string) {
	rt := k6common.GetRuntime(m.ctx)
	action := emulation.SetUserAgentOverride(userAgent)
	if err := action.Do(cdp.WithExecutor(m.ctx, m.session)); err != nil {
		k6common.Throw(rt, fmt.Errorf("unable to set user agent override: %w", err))
	}
}

// SetCacheEnabled toggles cache on/off
func (m *NetworkManager) SetCacheEnabled(enabled bool) {
	m.userCacheDisabled = !enabled
	if err := m.updateProtocolCacheDisabled(); err != nil {
		rt := k6common.GetRuntime(m.ctx)
		k6common.Throw(rt, err)
	}
}
