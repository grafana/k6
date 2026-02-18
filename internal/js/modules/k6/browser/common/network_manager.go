package common

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.k6.io/k6/internal/js/modules/k6/browser/log"

	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext"

	k6modules "go.k6.io/k6/js/modules"
	k6lib "go.k6.io/k6/lib"
	k6netext "go.k6.io/k6/lib/netext"
	k6types "go.k6.io/k6/lib/types"
	k6metrics "go.k6.io/k6/metrics"

	"github.com/chromedp/cdproto"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
)

// Credentials holds HTTP authentication credentials.
type Credentials struct {
	Username string `js:"username"`
	Password string `js:"password"` //nolint:gosec
}

// IsEmpty returns true if the credentials are empty.
func (c Credentials) IsEmpty() bool {
	c = Credentials{
		Username: strings.TrimSpace(c.Username),
		Password: strings.TrimSpace(c.Password),
	}
	return c == (Credentials{})
}

type eventInterceptor interface {
	urlTagName(urlTag string, method string) (string, bool)
	onRequest(request *Request)
	onResponse(response *Response)
	onRequestFinished(request *Request)
	onRequestFailed(request *Request)
}

// NetworkManager manages all frames in HTML document.
type NetworkManager struct {
	BaseEventEmitter

	ctx              context.Context
	logger           *log.Logger
	session          session
	parent           *NetworkManager
	frameManager     *FrameManager
	credentials      Credentials
	resolver         k6netext.Resolver
	vu               k6modules.VU
	customMetrics    *k6ext.CustomMetrics
	eventInterceptor eventInterceptor
	errorReasons     map[string]network.ErrorReason

	// TODO: manage inflight requests separately (move them between the two maps
	// as they transition from inflight -> completed)
	reqIDToRequest map[network.RequestID]*Request
	reqsMu         sync.RWMutex

	// These two maps are used to store the events so we can call onRequest with both of them,
	// regardless of the order of the events
	reqIDToRequestWillBeSentEvent map[network.RequestID]*network.EventRequestWillBeSent
	eventsWillBeSentMu            sync.RWMutex
	reqIDToRequestPausedEvent     map[network.RequestID]*fetch.EventRequestPaused
	eventsPausedMu                sync.RWMutex

	attemptedAuth map[fetch.RequestID]bool

	extraHTTPHeaders               map[string]string
	offline                        bool
	networkProfile                 NetworkProfile
	userCacheDisabled              bool
	userReqInterceptionEnabled     bool
	protocolReqInterceptionEnabled bool

	wg sync.WaitGroup
}

// NewNetworkManager creates a new network manager.
func NewNetworkManager(
	ctx context.Context,
	customMetrics *k6ext.CustomMetrics,
	s session,
	fm *FrameManager,
	parent *NetworkManager,
	ei eventInterceptor,
) (*NetworkManager, error) {
	vu := k6ext.GetVU(ctx)
	state := vu.State()

	resolver, err := newResolver(state.Options.DNS)
	if err != nil {
		return nil, fmt.Errorf("newResolver(%+v): %w", state.Options.DNS, err)
	}

	m := NetworkManager{
		BaseEventEmitter: NewBaseEventEmitter(ctx),
		ctx:              ctx,
		// TODO: Pass an internal logger instead of basing it on k6's logger?
		// See https://go.k6.io/k6/js/modules/k6/browser/issues/54
		logger:                        log.New(state.Logger, GetIterationID(ctx)),
		session:                       s,
		parent:                        parent,
		frameManager:                  fm,
		resolver:                      resolver,
		vu:                            vu,
		customMetrics:                 customMetrics,
		reqIDToRequest:                make(map[network.RequestID]*Request),
		reqIDToRequestWillBeSentEvent: make(map[network.RequestID]*network.EventRequestWillBeSent),
		reqIDToRequestPausedEvent:     make(map[network.RequestID]*fetch.EventRequestPaused),
		attemptedAuth:                 make(map[fetch.RequestID]bool),
		extraHTTPHeaders:              make(map[string]string),
		networkProfile:                NewNetworkProfile(),
		eventInterceptor:              ei,
		errorReasons:                  errorReasons(),
	}
	m.initEvents()
	if err := m.initDomains(); err != nil {
		return nil, err
	}

	return &m, nil
}

func errorReasons() map[string]network.ErrorReason {
	return map[string]network.ErrorReason{
		"aborted":              network.ErrorReasonAborted,
		"accessdenied":         network.ErrorReasonAccessDenied,
		"addressunreachable":   network.ErrorReasonAddressUnreachable,
		"blockedbyclient":      network.ErrorReasonBlockedByClient,
		"blockedbyresponse":    network.ErrorReasonBlockedByResponse,
		"connectionaborted":    network.ErrorReasonConnectionAborted,
		"connectionclosed":     network.ErrorReasonConnectionClosed,
		"connectionfailed":     network.ErrorReasonConnectionFailed,
		"connectionrefused":    network.ErrorReasonConnectionRefused,
		"connectionreset":      network.ErrorReasonConnectionReset,
		"internetdisconnected": network.ErrorReasonInternetDisconnected,
		"namenotresolved":      network.ErrorReasonNameNotResolved,
		"timedout":             network.ErrorReasonTimedOut,
		"failed":               network.ErrorReasonFailed,
	}
}

// Returns a new Resolver.
// Copied with minor changes from
// https://github.com/grafana/k6/blob/fb70bc6f3d3f22a40e65f32deea3cea1b6d70a76/js/runner.go#L459
func newResolver(conf k6types.DNSConfig) (k6netext.Resolver, error) {
	ttl, err := parseTTL(conf.TTL.String)
	if err != nil {
		return nil, fmt.Errorf("parsing TTL: %w", err)
	}

	dnsSel := conf.Select
	if !dnsSel.Valid {
		dnsSel = k6types.DefaultDNSConfig().Select
	}
	dnsPol := conf.Policy
	if !dnsPol.Valid {
		dnsPol = k6types.DefaultDNSConfig().Policy
	}
	return k6netext.NewResolver(
		net.LookupIP, ttl, dnsSel.DNSSelect, dnsPol.DNSPolicy), nil
}

// Parse a string representation of TTL to time.Duration.
// Copied from https://github.com/grafana/k6/blob/fb70bc6f3d3f22a40e65f32deea3cea1b6d70a76/js/runner.go#L479
func parseTTL(ttlS string) (time.Duration, error) {
	ttl := time.Duration(0)
	switch ttlS {
	case "inf":
		// cache "infinitely"
		ttl = time.Hour * 24 * 365
	case "0":
		// disable cache
	case "":
		ttlS = k6types.DefaultDNSConfig().TTL.String
		fallthrough
	default:
		var err error
		ttl, err = k6types.ParseExtendedDuration(ttlS)
		if ttl < 0 || err != nil {
			return ttl, fmt.Errorf("invalid DNS TTL: %s", ttlS)
		}
	}
	return ttl, nil
}

func (m *NetworkManager) deleteRequestByID(reqID network.RequestID) {
	m.reqsMu.Lock()
	defer m.reqsMu.Unlock()
	delete(m.reqIDToRequest, reqID)
}

func (m *NetworkManager) emitRequestMetrics(req *Request) {
	state := m.vu.State()

	tags := state.Tags.GetCurrentValues().Tags
	if state.Options.SystemTags.Has(k6metrics.TagMethod) {
		tags = tags.With("method", req.method)
	}
	if state.Options.SystemTags.Has(k6metrics.TagURL) {
		tags = handleURLTag(m.eventInterceptor, req.URL(), req.method, tags)
	}
	tags = tags.With("resource_type", req.ResourceType())

	pushIfNotDone(m.vu.Context(), m.logger, state.Samples, k6metrics.ConnectedSamples{
		Samples: []k6metrics.Sample{
			{
				TimeSeries: k6metrics.TimeSeries{Metric: m.customMetrics.BrowserDataSent, Tags: tags},
				Value:      float64(req.Size().Total()),
				Time:       req.wallTime,
			},
		},
	})
}

func (m *NetworkManager) emitResponseMetrics(resp *Response, req *Request) {
	state := m.vu.State()

	// In some scenarios we might not receive a ResponseReceived CDP event, in
	// which case the response won't be created. So to emit as much metric data
	// as possible we set some sensible defaults instead.
	var (
		status, bodySize                    int64
		ipAddress, protocol                 string
		fromCache, fromPreCache, fromSvcWrk bool
		url                                 = req.url.String()
		wallTime                            = time.Now()
		failed                              float64
	)
	if resp != nil {
		status = resp.status
		bodySize = resp.Size().Total()
		ipAddress = resp.remoteAddress.IPAddress
		protocol = resp.protocol
		fromCache = resp.fromDiskCache
		fromPreCache = resp.fromPrefetchCache
		fromSvcWrk = resp.fromServiceWorker
		wallTime = resp.wallTime
		url = resp.url
		// Assuming that a failure is when status
		// is not between 200 and 399 (inclusive).
		if status < 200 || status > 399 {
			failed = 1
		}
	} else {
		m.logger.Debugf("NetworkManager:emitResponseMetrics",
			"response is nil url:%s method:%s", req.url, req.method)
	}

	tags := state.Tags.GetCurrentValues().Tags
	if state.Options.SystemTags.Has(k6metrics.TagMethod) {
		tags = tags.With("method", req.method)
	}
	if state.Options.SystemTags.Has(k6metrics.TagURL) {
		tags = handleURLTag(m.eventInterceptor, url, req.method, tags)
	}
	if state.Options.SystemTags.Has(k6metrics.TagIP) {
		tags = tags.With("ip", ipAddress)
	}
	if state.Options.SystemTags.Has(k6metrics.TagStatus) {
		tags = tags.With("status", strconv.Itoa(int(status)))
	}
	if state.Options.SystemTags.Has(k6metrics.TagProto) {
		tags = tags.With("proto", protocol)
	}

	tags = tags.With("from_cache", strconv.FormatBool(fromCache))
	tags = tags.With("from_prefetch_cache", strconv.FormatBool(fromPreCache))
	tags = tags.With("from_service_worker", strconv.FormatBool(fromSvcWrk))
	tags = tags.With("resource_type", req.ResourceType())

	pushIfNotDone(m.vu.Context(), m.logger, state.Samples, k6metrics.ConnectedSamples{
		Samples: []k6metrics.Sample{
			{
				TimeSeries: k6metrics.TimeSeries{Metric: m.customMetrics.BrowserHTTPReqDuration, Tags: tags},
				Value:      k6metrics.D(wallTime.Sub(req.wallTime)),
				Time:       wallTime,
			},
			{
				TimeSeries: k6metrics.TimeSeries{Metric: m.customMetrics.BrowserDataReceived, Tags: tags},
				Value:      float64(bodySize),
				Time:       wallTime,
			},
		},
	})

	if resp != nil && resp.timing != nil {
		pushIfNotDone(m.vu.Context(), m.logger, state.Samples, k6metrics.ConnectedSamples{
			Samples: []k6metrics.Sample{
				{
					TimeSeries: k6metrics.TimeSeries{Metric: m.customMetrics.BrowserHTTPReqFailed, Tags: tags},
					Value:      failed,
					Time:       wallTime,
				},
			},
		})
	}
}

// handleURLTag will check if the url tag needs to be grouped by testing
// against user supplied regex. If there's a match a user supplied name will
// be used instead of the url for the url tag, otherwise the url will be used.
func handleURLTag(mi eventInterceptor, url string, method string, tags *k6metrics.TagSet) *k6metrics.TagSet {
	if newTagName, urlMatched := mi.urlTagName(url, method); urlMatched {
		tags = tags.With("url", newTagName)
		tags = tags.With("name", newTagName)
		return tags
	}

	tags = tags.With("url", url)
	tags = tags.With("name", url)

	return tags
}

func (m *NetworkManager) handleRequestRedirect(
	req *Request, redirectResponse *network.Response, timestamp *cdp.MonotonicTime,
) {
	resp := NewHTTPResponse(m.ctx, req, redirectResponse, timestamp)
	req.responseMu.Lock()
	req.response = resp
	req.responseMu.Unlock()
	req.redirectChain = append(req.redirectChain, req)

	m.emitResponseMetrics(resp, req)
	m.deleteRequestByID(req.requestID)

	/*
		delete(m.attemptedAuth, req.interceptionID);
	*/

	m.eventInterceptor.onResponse(resp)
	m.eventInterceptor.onRequestFinished(req)
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
			return fmt.Errorf("initializing networking %T: %w", action, err)
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
		cdproto.EventFetchAuthRequired,
	}, chHandler)

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		for m.handleEvents(chHandler) {
		}
	}()
}

func (m *NetworkManager) handleEvents(in <-chan Event) bool {
	select {
	case <-m.ctx.Done():
		return false
	case <-m.session.Done():
		return false
	case event := <-in:
		select {
		case <-m.ctx.Done():
			return false
		case <-m.session.Done():
			return false
		default:
		}
		switch ev := event.data.(type) {
		case *network.EventLoadingFailed:
			m.onLoadingFailed(ev)
		case *network.EventLoadingFinished:
			m.onLoadingFinished(ev)
		case *network.EventRequestWillBeSent:
			m.onRequestWillBeSent(ev)
		case *network.EventRequestServedFromCache:
			m.onRequestServedFromCache(ev)
		case *network.EventResponseReceived:
			m.onResponseReceived(ev)
		case *fetch.EventRequestPaused:
			m.onRequestPaused(ev)
		case *fetch.EventAuthRequired:
			m.onAuthRequired(ev)
		}
	}
	return true
}

func (m *NetworkManager) onLoadingFailed(event *network.EventLoadingFailed) {
	req, ok := m.requestFromID(event.RequestID)
	if !ok {
		// TODO: add handling of iframe document requests starting in one session and ending up in another
		return
	}

	req.setErrorText(event.ErrorText)
	req.responseEndTiming = float64(event.Timestamp.Time().Unix()-req.timestamp.Unix()) * 1000
	m.eventInterceptor.onRequestFailed(req)
	m.deleteRequestByID(event.RequestID)
	m.frameManager.requestFailed(req, event.Canceled)
}

func (m *NetworkManager) onLoadingFinished(event *network.EventLoadingFinished) {
	req := m.requestForOnLoadingFinished(event.RequestID)
	// the request was not created yet.
	if req == nil {
		return
	}

	req.responseEndTiming = float64(event.Timestamp.Time().Unix()-req.timestamp.Unix()) * 1000
	m.deleteRequestByID(event.RequestID)
	m.frameManager.requestFinished(req)
	m.eventInterceptor.onRequestFinished(req)

	// Skip data and blob URLs when emitting metrics, since they're internal to the browser.
	if isInternalURL(req.url) {
		return
	}
	emitResponseMetrics := func() {
		req.responseMu.RLock()
		m.emitResponseMetrics(req.response, req)
		req.responseMu.RUnlock()
	}
	if !req.allowInterception {
		emitResponseMetrics()
		return
	}
	// When request interception is enabled, we need to process requestPaused messages
	// from CDP in order to get the response for the request. However, we can't process
	// them until the request is unblocked. Since we're blocking the NetworkManager
	// goroutine here, we need to spawn a new goroutine to allow the requestPaused
	// messages to be processed by the NetworkManager.
	//
	// This happens when the main page request redirects before it finishes loading.
	// So the new redirect request will be blocked until the main page finishes loading.
	// The main page will wait forever since its subrequest is blocked.
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		emitResponseMetrics()
	}()
}

// requestForOnLoadingFinished returns the request for the given request ID.
func (m *NetworkManager) requestForOnLoadingFinished(rid network.RequestID) *Request {
	r, ok := m.requestFromID(rid)

	// Immediately return if the request is found.
	if ok {
		return r
	}

	// Handle IFrame document requests starting in one session and ending up in another.
	if m.parent == nil {
		return nil
	}

	pr, ok := m.parent.requestFromID(rid)
	if !ok {
		return nil
	}
	// Requests eminating from the parent have matching requestIDs.
	if pr.getDocumentID() != rid.String() {
		return nil
	}

	// Switch the request to the parent request.
	m.reqsMu.Lock()
	m.reqIDToRequest[rid] = pr
	m.reqsMu.Unlock()
	m.parent.deleteRequestByID(rid)

	return pr
}

func isInternalURL(u *url.URL) bool {
	return u.Scheme == "data" || u.Scheme == "blob"
}

func (m *NetworkManager) onRequest(event *network.EventRequestWillBeSent,
	requestPausedEvent *fetch.EventRequestPaused,
) {
	m.logger.Debugf("NetworkManager:onRequest", "url:%s method:%s type:%s fid:%s Starting onRequest",
		event.Request.URL, event.Request.Method, event.Initiator.Type, event.FrameID)
	var redirectChain []*Request = nil
	if event.RedirectResponse != nil {
		req, ok := m.requestFromID(event.RequestID)
		if ok {
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
	var ok bool
	if event.FrameID != "" {
		frame, ok = m.frameManager.getFrameByID(event.FrameID)
	}
	if !ok && requestPausedEvent != nil && requestPausedEvent.FrameID != "" {
		frame, ok = m.frameManager.getFrameByID(requestPausedEvent.FrameID)
	}

	// Check if it's main resource request interception (targetID === main frame id).
	if !ok && m.frameManager.page != nil && event.FrameID != "" &&
		event.FrameID == cdp.FrameID(m.frameManager.page.targetID) {
		// Main resource request for the page is being intercepted so the Frame is not created
		// yet. Precreate it here for the purposes of request interception. It will be updated
		// later as soon as the request continues and we receive frame tree from the page.
		m.frameManager.frameAttached(event.FrameID, "")
		frame, ok = m.frameManager.getFrameByID(event.FrameID)
	}

	if !ok {
		m.logger.Debugf("NetworkManager:onRequest", "url:%s method:%s type:%s fid:%s frame is nil",
			event.Request.URL, event.Request.Method, event.Initiator.Type, event.FrameID)
	}

	var interceptionID fetch.RequestID
	if requestPausedEvent != nil {
		interceptionID = requestPausedEvent.RequestID
	}

	req, err := NewRequest(m.ctx, m.logger, NewRequestParams{
		event:             event,
		frame:             frame,
		redirectChain:     redirectChain,
		interceptionID:    interceptionID,
		allowInterception: m.userReqInterceptionEnabled,
	})
	if err != nil {
		m.logger.Errorf("NetworkManager", "creating request: %s", err)
		return
	}
	// Skip data and blob URLs, since they're internal to the browser.
	if isInternalURL(req.url) {
		m.logger.Debugf("NetworkManager", "skipping request handling of %s URL", req.url.Scheme)
		return
	}
	m.reqsMu.Lock()
	m.reqIDToRequest[event.RequestID] = req
	m.reqsMu.Unlock()
	m.emitRequestMetrics(req)
	m.frameManager.requestStarted(req)

	m.eventInterceptor.onRequest(req)
}

// onRequestWillBeSent calls the onRequest method:
// - right away, if request interception is disabled
// - only if we first received the onRequestPaused event, if request interception is enabled;
// otherwise, it stores the event in a map to be processed when the onRequestPaused event arrives
func (m *NetworkManager) onRequestWillBeSent(event *network.EventRequestWillBeSent) {
	m.logger.Debugf("NetworkManager:onRequestWillBeSent", "url:%s method:%s type:%s fid:%s Starting onRequestWillBeSent",
		event.Request.URL, event.Request.Method, event.Initiator.Type, event.FrameID)

	if m.protocolReqInterceptionEnabled {
		requestID := event.RequestID
		if requestPausedEvent, ok := m.pausedEventFromReqID(requestID); ok {
			m.onRequest(event, requestPausedEvent)
			m.eventsPausedMu.Lock()
			delete(m.reqIDToRequestPausedEvent, requestID)
			m.eventsPausedMu.Unlock()
		} else {
			m.eventsWillBeSentMu.Lock()
			m.reqIDToRequestWillBeSentEvent[requestID] = event
			m.eventsWillBeSentMu.Unlock()
		}
	} else {
		m.onRequest(event, nil)
	}
}

// onRequestPaused can send one of these two CDP events:
// - Fetch.failRequest if the URL is part of the blocked hosts or IPs
// - Fetch.continueRequest if the request is not blocked and no route is configured
// In both case, if we first received the onRequestWillBeSent event, we call onRequest
// otherwise, it stores the event in a map to be processed when the onRequestWillBeSent event arrives
func (m *NetworkManager) onRequestPaused(event *fetch.EventRequestPaused) {
	m.logger.Debugf("NetworkManager:onRequestPaused",
		"url:%v sid:%s", event.Request.URL, m.session.ID())
	defer m.logger.Debugf("NetworkManager:onRequestPaused:return",
		"url:%v sid:%s", event.Request.URL, m.session.ID())

	var failErr error

	defer func() {
		requestID := event.NetworkID
		if requestWillBeSentEvent, ok := m.willBeSentEventFromReqID(requestID); ok {
			m.onRequest(requestWillBeSentEvent, event)
			m.eventsWillBeSentMu.Lock()
			delete(m.reqIDToRequestWillBeSentEvent, requestID)
			m.eventsWillBeSentMu.Unlock()
		} else {
			m.eventsPausedMu.Lock()
			m.reqIDToRequestPausedEvent[requestID] = event
			m.eventsPausedMu.Unlock()
		}

		if failErr != nil {
			action := fetch.FailRequest(event.RequestID, network.ErrorReasonBlockedByClient)
			if err := action.Do(cdp.WithExecutor(m.ctx, m.session)); err != nil {
				// Avoid logging as error when context is canceled.
				// Most probably this happens when trying to fail a site's background request
				// while the iteration is ending and therefore the browser context is being closed.
				if errors.Is(err, context.Canceled) {
					m.logger.Debug("NetworkManager:onRequestPaused", "context canceled interrupting request")
				} else {
					m.logger.Errorf("NetworkManager:onRequestPaused", "interrupting request: %s", err)
				}
				return
			}
			m.logger.Warnf("NetworkManager:onRequestPaused",
				"request %s %s was aborted: %s", event.Request.Method, event.Request.URL, failErr)

			return
		}

		// If no route was added, continue all requests
		if m.frameManager.page == nil || !m.frameManager.page.hasRoutes() {
			err := m.ContinueRequest(event.RequestID, ContinueOptions{}, nil)
			if err != nil {
				m.logger.Errorf("NetworkManager:onRequestPaused",
					"continuing request %s %s: %s", event.Request.Method, event.Request.URL, err)
			}
		}
	}()

	purl, err := url.Parse(event.Request.URL)
	if err != nil {
		m.logger.Errorf("NetworkManager:onRequestPaused",
			"parsing URL %q: %s", event.Request.URL, err)
		return
	}

	var (
		host  = purl.Hostname()
		ip    = net.ParseIP(host)
		state = m.vu.State()
	)
	if ip != nil {
		failErr = checkBlockedIPs(ip, state.Options.BlacklistIPs)
		return
	}
	failErr = checkBlockedHosts(host, state.Options.BlockedHostnames.Trie)
	if failErr != nil {
		return
	}

	// Do one last check of the resolved IP
	ip, err = m.resolver.LookupIP(host)
	if err != nil {
		m.logger.Debugf("NetworkManager:onRequestPaused",
			"resolving %q: %s", host, err)
		return
	}
	failErr = checkBlockedIPs(ip, state.Options.BlacklistIPs)
}

func checkBlockedHosts(host string, blockedHosts *k6types.HostnameTrie) error {
	if blockedHosts == nil {
		return nil
	}
	if match, blocked := blockedHosts.Contains(host); blocked {
		return fmt.Errorf("hostname %s matches a blocked pattern %q", host, match)
	}
	return nil
}

func checkBlockedIPs(ip net.IP, blockedIPs []*k6lib.IPNet) error {
	for _, ipnet := range blockedIPs {
		if ipnet.Contains(ip) {
			// TODO: Return netext.BlackListedIPError here once its private
			// fields are exported, or there's a constructor for it.
			return fmt.Errorf("IP %s is in a blacklisted range %q", ip, ipnet)
		}
	}
	return nil
}

func (m *NetworkManager) onAuthRequired(event *fetch.EventAuthRequired) {
	var (
		res = fetch.AuthChallengeResponseResponseDefault
		rid = event.RequestID

		username, password string
	)

	switch {
	case m.attemptedAuth[rid]:
		delete(m.attemptedAuth, rid)
		res = fetch.AuthChallengeResponseResponseCancelAuth
	case !m.credentials.IsEmpty():
		// TODO: remove requests from attemptedAuth when:
		//       - request is redirected
		//       - loading finished
		m.attemptedAuth[rid] = true
		res = fetch.AuthChallengeResponseResponseProvideCredentials
		// The Fetch.AuthChallengeResponse docs mention username and password should only be set
		// if the response is ProvideCredentials.
		// See: https://chromedevtools.github.io/devtools-protocol/tot/Fetch/#type-AuthChallengeResponse
		username, password = m.credentials.Username, m.credentials.Password
	}
	err := fetch.ContinueWithAuth(
		rid,
		&fetch.AuthChallengeResponse{
			Response: res,
			Username: username,
			Password: password,
		},
	).Do(cdp.WithExecutor(m.ctx, m.session))
	if err != nil {
		m.logger.Debugf("NetworkManager:onAuthRequired", "continueWithAuth url:%q err:%v", event.Request.URL, err)
	} else {
		m.logger.Debugf("NetworkManager:onAuthRequired", "continueWithAuth url:%q OK", event.Request.URL)
	}
}

func (m *NetworkManager) onRequestServedFromCache(event *network.EventRequestServedFromCache) {
	req, ok := m.requestFromID(event.RequestID)
	if ok {
		req.setLoadedFromCache(true)
	}
}

func (m *NetworkManager) onResponseReceived(event *network.EventResponseReceived) {
	req, ok := m.requestFromID(event.RequestID)

	if !ok {
		return
	}
	resp := NewHTTPResponse(m.ctx, req, event.Response, event.Timestamp)
	req.responseMu.Lock()
	req.response = resp
	req.responseMu.Unlock()

	m.logger.Debugf("FrameManager:onResponseReceived", "rid:%s rurl:%s", event.RequestID, resp.URL())

	m.eventInterceptor.onResponse(resp)
}

func (m *NetworkManager) requestFromID(reqID network.RequestID) (*Request, bool) {
	m.reqsMu.RLock()
	defer m.reqsMu.RUnlock()

	r, ok := m.reqIDToRequest[reqID]

	return r, ok
}

func (m *NetworkManager) willBeSentEventFromReqID(reqID network.RequestID) (*network.EventRequestWillBeSent, bool) {
	m.eventsWillBeSentMu.RLock()
	defer m.eventsWillBeSentMu.RUnlock()

	e, ok := m.reqIDToRequestWillBeSentEvent[reqID]

	return e, ok
}

func (m *NetworkManager) pausedEventFromReqID(reqID network.RequestID) (*fetch.EventRequestPaused, bool) {
	m.eventsPausedMu.RLock()
	defer m.eventsPausedMu.RUnlock()

	e, ok := m.reqIDToRequestPausedEvent[reqID]

	return e, ok
}

func (m *NetworkManager) setRequestInterception(value bool) error {
	m.userReqInterceptionEnabled = value
	return m.updateProtocolRequestInterception()
}

func (m *NetworkManager) updateProtocolCacheDisabled() error {
	action := network.SetCacheDisabled(m.userCacheDisabled)
	if err := action.Do(cdp.WithExecutor(m.ctx, m.session)); err != nil {
		errAction := "enabling"
		if m.userCacheDisabled {
			errAction = "disabling"
		}
		return fmt.Errorf("%s network cache: %w", errAction, err)
	}
	return nil
}

func (m *NetworkManager) updateProtocolRequestInterception() error {
	enabled := m.userReqInterceptionEnabled
	if enabled == m.protocolReqInterceptionEnabled {
		return nil
	}

	m.protocolReqInterceptionEnabled = enabled
	m.logger.Debugf("NetworkManager:updateProtocolRequestInterception",
		"updating request interception to %t (session: %s)", enabled, m.session.ID())

	actions := []Action{
		network.SetCacheDisabled(true),
		fetch.Enable().
			WithHandleAuthRequests(true).
			WithPatterns([]*fetch.RequestPattern{
				{
					URLPattern:   "*",
					RequestStage: fetch.RequestStageRequest,
				},
			}),
	}
	if !enabled {
		actions = []Action{
			network.SetCacheDisabled(false),
			fetch.Disable(),
		}
	}
	for _, action := range actions {
		if err := action.Do(cdp.WithExecutor(m.ctx, m.session)); err != nil {
			return fmt.Errorf("internal error while updating protocol request interception %T: %w", action, err)
		}
	}

	return nil
}

// Authenticate sets HTTP authentication credentials to use.
func (m *NetworkManager) Authenticate(credentials Credentials) error {
	m.credentials = credentials
	if !credentials.IsEmpty() {
		m.userReqInterceptionEnabled = true
	}
	if err := m.updateProtocolRequestInterception(); err != nil {
		return fmt.Errorf("setting authentication credentials: %w", err)
	}

	return nil
}

func (m *NetworkManager) AbortRequest(requestID fetch.RequestID, errorReason string) error {
	m.logger.Debugf("NetworkManager:AbortRequest", "aborting request (id: %s, errorReason: %s)",
		requestID, errorReason)
	netErrorReason, ok := m.errorReasons[errorReason]
	if !ok {
		return fmt.Errorf("unknown error code: %s", errorReason)
	}

	action := fetch.FailRequest(requestID, netErrorReason)
	if err := action.Do(cdp.WithExecutor(m.ctx, m.session)); err != nil {
		// Avoid logging as error when context is canceled.
		// Most probably this happens when trying to fail a site's background request
		// while the iteration is ending and therefore the browser context is being closed.
		if errors.Is(err, context.Canceled) {
			m.logger.Debug("NetworkManager:AbortRequest", "context canceled interrupting request")
		} else {
			return fmt.Errorf("fail to abort request (id: %s): %w", requestID, err)
		}
	}

	return nil
}

func (m *NetworkManager) ContinueRequest(
	requestID fetch.RequestID,
	opts ContinueOptions,
	originalHeaders []HTTPHeader,
) error {
	m.logger.Debugf("NetworkManager:ContinueRequest", "continuing request (id: %s)", requestID)
	action := fetch.ContinueRequest(requestID)

	if len(opts.Headers) > 0 {
		action = action.WithHeaders(toFetchHeaders(opts.Headers))
	}
	if opts.URL != "" {
		action = action.WithURL(opts.URL)
	}
	if opts.Method != "" {
		action = action.WithMethod(opts.Method)
	}
	if len(opts.PostData) > 0 {
		b64PostData := base64.StdEncoding.EncodeToString(opts.PostData)
		action = action.WithPostData(b64PostData)
	}

	if err := action.Do(cdp.WithExecutor(m.ctx, m.session)); err != nil {
		// Avoid logging as error when context is canceled.
		// Most probably this happens when trying to fail a site's background request
		// while the iteration is ending and therefore the browser context is being closed.
		if errors.Is(err, context.Canceled) {
			m.logger.Debug("NetworkManager:ContinueRequest", "context canceled continuing request")
			return nil
		}

		// This error message is an internal issue, rather than something that the user can
		// action on. It's also usually ok to ignore since it means that the page has navigated
		// away or something has occurred which means that the request is no longer needed and
		// isn't being tracked by chromium.
		if strings.Contains(err.Error(), "Invalid InterceptionId") {
			m.logger.Debugf("NetworkManager:ContinueRequest", "invalid interception ID (%s) continuing request: %s",
				requestID, err)
			return nil
		}

		return fmt.Errorf("fail to continue request (id: %s): %w", requestID, err)
	}

	return nil
}

func (m *NetworkManager) FulfillRequest(request *Request, opts FulfillOptions) error {
	responseCode := int64(http.StatusOK)
	if opts.Status != 0 {
		responseCode = opts.Status
	}

	action := fetch.FulfillRequest(request.interceptionID, responseCode)

	if opts.ContentType != "" {
		opts.Headers = append(opts.Headers, HTTPHeader{
			Name:  "Content-Type",
			Value: opts.ContentType,
		})
	}

	headers := toFetchHeaders(opts.Headers)
	if len(headers) > 0 {
		action = action.WithResponseHeaders(headers)
	}

	if len(opts.Body) > 0 {
		b64Body := base64.StdEncoding.EncodeToString(opts.Body)
		action = action.WithBody(b64Body)
	}

	if err := action.Do(cdp.WithExecutor(m.ctx, m.session)); err != nil {
		// Avoid logging as error when context is canceled.
		// Most probably this happens when trying to fail a site's background request
		// while the iteration is ending and therefore the browser context is being closed.
		if errors.Is(err, context.Canceled) {
			m.logger.Debug("NetworkManager:FulfillRequest", "context canceled fulfilling request")
			return nil
		}

		return fmt.Errorf("fail to fulfill request (id: %s): %w",
			request.interceptionID, err)
	}

	return nil
}

func toFetchHeaders(headers []HTTPHeader) []*fetch.HeaderEntry {
	if len(headers) == 0 {
		return nil
	}

	fetchHeaders := make([]*fetch.HeaderEntry, len(headers))
	for i, header := range headers {
		fetchHeaders[i] = &fetch.HeaderEntry{
			Name:  header.Name,
			Value: header.Value,
		}
	}
	return fetchHeaders
}

// SetExtraHTTPHeaders sets extra HTTP request headers to be sent with every request.
func (m *NetworkManager) SetExtraHTTPHeaders(headers network.Headers) error {
	err := network.
		SetExtraHTTPHeaders(headers).
		Do(cdp.WithExecutor(m.ctx, m.session))
	if err != nil {
		return fmt.Errorf("setting extra HTTP headers: %w", err)
	}

	return nil
}

// SetOfflineMode toggles offline mode on/off.
func (m *NetworkManager) SetOfflineMode(offline bool) error {
	if m.offline == offline {
		return nil
	}
	m.offline = offline

	action := network.EmulateNetworkConditions(
		m.offline,
		m.networkProfile.Latency,
		m.networkProfile.Download,
		m.networkProfile.Upload,
	)
	if err := action.Do(cdp.WithExecutor(m.ctx, m.session)); err != nil {
		return fmt.Errorf("emulating network conditions: %w", err)
	}

	return nil
}

// ThrottleNetwork changes the network attributes in chrome to simulate slower
// networks e.g. a slow 3G connection.
func (m *NetworkManager) ThrottleNetwork(networkProfile NetworkProfile) error {
	if m.networkProfile == networkProfile {
		return nil
	}
	m.networkProfile = networkProfile

	action := network.EmulateNetworkConditions(
		m.offline,
		m.networkProfile.Latency,
		m.networkProfile.Download,
		m.networkProfile.Upload,
	)
	if err := action.Do(cdp.WithExecutor(m.ctx, m.session)); err != nil {
		return fmt.Errorf("throttling network: %w", err)
	}

	return nil
}

// SetUserAgent overrides the browser user agent string.
func (m *NetworkManager) SetUserAgent(userAgent string) {
	action := emulation.SetUserAgentOverride(userAgent)
	if err := action.Do(cdp.WithExecutor(m.ctx, m.session)); err != nil {
		k6ext.Panicf(m.ctx, "setting user agent: %w", err)
	}
}

// SetCacheEnabled toggles cache on/off.
func (m *NetworkManager) SetCacheEnabled(enabled bool) {
	m.userCacheDisabled = !enabled
	if err := m.updateProtocolCacheDisabled(); err != nil {
		k6ext.Panicf(m.ctx, "%v", err)
	}
}

func (m *NetworkManager) wait() {
	m.wg.Wait()
}
