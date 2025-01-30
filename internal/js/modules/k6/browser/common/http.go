package common

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"

	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext"
	"go.k6.io/k6/internal/js/modules/k6/browser/log"

	k6modules "go.k6.io/k6/js/modules"
)

// These ResourceTypes are duplicates of CDP's network.ResourceType. We want to work
// with our version of ResourceType to catch any breaking changes early.
const (
	ResourceTypeDocument           string = "Document"
	ResourceTypeStylesheet         string = "Stylesheet"
	ResourceTypeImage              string = "Image"
	ResourceTypeMedia              string = "Media"
	ResourceTypeFont               string = "Font"
	ResourceTypeScript             string = "Script"
	ResourceTypeTextTrack          string = "TextTrack"
	ResourceTypeXHR                string = "XHR"
	ResourceTypeFetch              string = "Fetch"
	ResourceTypePrefetch           string = "Prefetch"
	ResourceTypeEventSource        string = "EventSource"
	ResourceTypeWebSocket          string = "WebSocket"
	ResourceTypeManifest           string = "Manifest"
	ResourceTypeSignedExchange     string = "SignedExchange"
	ResourceTypePing               string = "Ping"
	ResourceTypeCSPViolationReport string = "CSPViolationReport"
	ResourceTypePreflight          string = "Preflight"
	ResourceTypeOther              string = "Other"
	ResourceTypeUnknown            string = "Unknown"
)

// HTTPHeader is a single HTTP header.
type HTTPHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// HTTPMessageSize are the sizes in bytes of the HTTP message header and body.
type HTTPMessageSize struct {
	Headers int64 `json:"headers"`
	Body    int64 `json:"body"`
}

// Total returns the total size in bytes of the HTTP message.
func (s HTTPMessageSize) Total() int64 {
	return s.Headers + s.Body
}

// Request represents a browser HTTP request.
type Request struct {
	ctx           context.Context
	frame         *Frame
	responseMu    sync.RWMutex
	response      *Response
	redirectChain []*Request
	requestID     network.RequestID
	documentID    string
	url           *url.URL
	method        string
	headers       map[string][]string
	// For now we're only going to work with the 0th entry of postDataEntries.
	// We've not been able to reproduce a situation where more than one entry
	// occupies the slice. Once we have a better idea of when more than one
	// entry is in postDataEntries, we should look to export a new API.
	postDataEntries     []string
	resourceType        string
	isNavigationRequest bool
	allowInterception   bool
	interceptionID      string
	fromMemoryCache     bool
	errorText           string
	// offset is the difference between the timestamp and wallTime fields.
	//
	// The cdp package (and the CDP protocol) uses the monotonic time
	// when calculating timestamps. And the cdp package does so by
	// getting it from the local machine's last boot time. This causes
	// a time skew between the timestamp and the machine's walltime.
	//
	// Since the cdp package uses monotonic time in timestamp fields, we
	// need to calculate the timestamp with the monotonic difference.
	//
	// See issue #533 for more details.
	offset            time.Duration
	timestamp         time.Time
	wallTime          time.Time
	responseEndTiming float64
	vu                k6modules.VU
}

// NewRequestParams are input parameters for NewRequest.
type NewRequestParams struct {
	event             *network.EventRequestWillBeSent
	frame             *Frame
	redirectChain     []*Request
	interceptionID    string
	allowInterception bool
}

// NewRequest creates a new HTTP request.
func NewRequest(ctx context.Context, logger *log.Logger, rp NewRequestParams) (*Request, error) {
	ev := rp.event

	documentID := cdp.LoaderID("")
	if ev.RequestID == network.RequestID(ev.LoaderID) && ev.Type == "Document" {
		documentID = ev.LoaderID
	}

	u, err := url.Parse(ev.Request.URL)
	if err != nil {
		var uerr *url.Error
		if errors.As(err, &uerr) {
			err = uerr.Err
		}
		return nil, fmt.Errorf("parsing URL %q: %w", ev.Request.URL, err)
	}

	isNavigationRequest := string(ev.RequestID) == string(ev.LoaderID) &&
		ev.Type == network.ResourceTypeDocument

	pd := make([]string, 0, len(ev.Request.PostDataEntries))
	for _, i := range ev.Request.PostDataEntries {
		if i == nil {
			continue
		}

		decodedBytes, err := base64.StdEncoding.DecodeString(i.Bytes)
		if err != nil {
			return nil, fmt.Errorf("decoding postData %q: %w", i.Bytes, err)
		}

		pd = append(pd, string(decodedBytes))
	}

	r := Request{
		url:                 u,
		frame:               rp.frame,
		redirectChain:       rp.redirectChain,
		requestID:           ev.RequestID,
		method:              ev.Request.Method,
		postDataEntries:     pd,
		resourceType:        validateResourceType(logger, ev.Type.String()),
		isNavigationRequest: isNavigationRequest,
		allowInterception:   rp.allowInterception,
		interceptionID:      rp.interceptionID,
		timestamp:           ev.Timestamp.Time(),
		wallTime:            ev.WallTime.Time(),
		offset:              ev.WallTime.Time().Sub(ev.Timestamp.Time()),
		documentID:          documentID.String(),
		headers:             make(map[string][]string),
		ctx:                 ctx,
		vu:                  k6ext.GetVU(ctx),
	}
	for n, v := range ev.Request.Headers {
		if s, ok := v.(string); ok {
			r.headers[n] = append(r.headers[n], s)
		}
	}

	return &r, nil
}

// validateResourceType will validate network.ResourceType string values against our own
// ResourceType string values.
//   - If a new network.ResourceType is added, this will log a warn and return
//     ResourceTypeUnknown.
//   - If an existing network.ResourceType is amended, this will log a warn and return
//     ResourceTypeUnknown.
//   - If a network.ResourceType is deleted then we will get a compilation error.
func validateResourceType(logger *log.Logger, t string) string {
	switch t {
	case ResourceTypeDocument:
	case ResourceTypeStylesheet:
	case ResourceTypeImage:
	case ResourceTypeMedia:
	case ResourceTypeFont:
	case ResourceTypeScript:
	case ResourceTypeTextTrack:
	case ResourceTypeXHR:
	case ResourceTypeFetch:
	case ResourceTypePrefetch:
	case ResourceTypeEventSource:
	case ResourceTypeWebSocket:
	case ResourceTypeManifest:
	case ResourceTypeSignedExchange:
	case ResourceTypePing:
	case ResourceTypeCSPViolationReport:
	case ResourceTypePreflight:
	case ResourceTypeOther:
	default:
		t = ResourceTypeUnknown
		logger.Warnf("http:resourceType", "unknown network.ResourceType %q detected", t)
	}

	return t
}

func (r *Request) getFrame() *Frame {
	return r.frame
}

func (r *Request) getID() network.RequestID {
	return r.requestID
}

func (r *Request) getDocumentID() string {
	return r.documentID
}

func (r *Request) headersSize() int64 {
	size := 4 // 4 = 2 spaces + 2 line breaks (GET /path \r\n)
	size += len(r.method)
	size += len(r.url.Path)
	size += 8 // httpVersion
	for n, v := range r.headers {
		size += len(n) + len(strings.Join(v, "")) + 4 // 4 = ': ' + '\r\n'
	}
	return int64(size)
}

func (r *Request) setErrorText(errorText string) {
	r.errorText = errorText
}

func (r *Request) setLoadedFromCache(fromMemoryCache bool) {
	r.fromMemoryCache = fromMemoryCache
}

// AllHeaders returns all the request headers.
func (r *Request) AllHeaders() map[string]string {
	// TODO: fix this data to include "ExtraInfo" header data
	headers := make(map[string]string)
	for n, v := range r.headers {
		headers[strings.ToLower(n)] = strings.Join(v, ",")
	}
	return headers
}

// Frame returns the frame within which the request was made.
func (r *Request) Frame() *Frame {
	return r.frame
}

// HeaderValue returns the value of the given header.
func (r *Request) HeaderValue(name string) (string, bool) {
	headers := r.AllHeaders()
	val, ok := headers[strings.ToLower(name)]
	return val, ok
}

// Headers returns the request headers.
func (r *Request) Headers() map[string]string {
	headers := make(map[string]string)
	for n, v := range r.headers {
		headers[n] = strings.Join(v, ",")
	}
	return headers
}

// HeadersArray returns the request headers as an array of objects.
func (r *Request) HeadersArray() []HTTPHeader {
	headers := make([]HTTPHeader, 0)
	for n, vals := range r.headers {
		for _, v := range vals {
			headers = append(headers, HTTPHeader{Name: n, Value: v})
		}
	}
	return headers
}

// IsNavigationRequest returns whether this was a navigation request or not.
func (r *Request) IsNavigationRequest() bool {
	return r.isNavigationRequest
}

// Method returns the request method.
func (r *Request) Method() string {
	return r.method
}

// PostData returns the request post data, if any.
//
// If will not attempt to fetch the data if it should have some but nothing is
// cached locally: https://go.k6.io/k6/js/modules/k6/browser/issues/1470
//
// This relies on PostDataEntries. It will only ever return the 0th entry.
// TODO: Create a PostDataEntries API when we have a better idea of when that
// is needed.
func (r *Request) PostData() string {
	if len(r.postDataEntries) > 0 {
		return r.postDataEntries[0]
	}

	return ""
}

// PostDataBuffer returns the request post data as an ArrayBuffer.
//
// If will not attempt to fetch the data if it should have some but nothing is
// cached locally: https://go.k6.io/k6/js/modules/k6/browser/issues/1470
//
// This relies on PostDataEntries. It will only ever return the 0th entry.
// TODO: Create a PostDataEntries API when we have a better idea of when that
// is needed.
func (r *Request) PostDataBuffer() []byte {
	if len(r.postDataEntries) > 0 {
		return []byte(r.postDataEntries[0])
	}

	return nil
}

// ResourceType returns the request resource type.
func (r *Request) ResourceType() string {
	return r.resourceType
}

// Response returns the response for the request, if received.
func (r *Request) Response() *Response {
	return r.response
}

// Size returns the size of the request.
func (r *Request) Size() HTTPMessageSize {
	var b int64
	for _, p := range r.postDataEntries {
		b += int64(len(p))
	}
	return HTTPMessageSize{
		Body:    b,
		Headers: r.headersSize(),
	}
}

// resourceTiming is the type returned from request.timing.
type resourceTiming struct {
	StartTime             float64 `js:"startTime"`
	DomainLookupStart     float64 `js:"domainLookupStart"`
	DomainLookupEnd       float64 `js:"domainLookupEnd"`
	ConnectStart          float64 `js:"connectStart"`
	SecureConnectionStart float64 `js:"secureConnectionStart"`
	ConnectEnd            float64 `js:"connectEnd"`
	RequestStart          float64 `js:"requestStart"`
	ResponseStart         float64 `js:"responseStart"`
	ResponseEnd           float64 `js:"responseEnd"`
}

// Timing returns the request timing information.
func (r *Request) Timing() *resourceTiming {
	if r.response == nil {
		return nil
	}

	timing := r.response.timing

	return &resourceTiming{
		StartTime:             (timing.RequestTime - float64(r.timestamp.Unix()) + float64(r.wallTime.Unix())) * 1000,
		DomainLookupStart:     timing.DNSStart,
		DomainLookupEnd:       timing.DNSEnd,
		ConnectStart:          timing.ConnectStart,
		SecureConnectionStart: timing.SslStart,
		ConnectEnd:            timing.ConnectEnd,
		RequestStart:          timing.SendStart,
		ResponseStart:         timing.ReceiveHeadersEnd,
		ResponseEnd:           r.responseEndTiming,
	}
}

// URL returns the request URL.
func (r *Request) URL() string {
	return r.url.String()
}

// RemoteAddress contains informationa about a remote target.
type RemoteAddress struct {
	IPAddress string `json:"ipAddress" js:"ipAddress"`
	Port      int64  `json:"port" js:"port"`
}

// SecurityDetails contains informationa about the security details of a TLS connection.
type SecurityDetails struct {
	SubjectName string   `json:"subjectName" js:"subjectName"`
	Issuer      string   `json:"issuer" js:"issuer"`
	ValidFrom   int64    `json:"validFrom" js:"validFrom"`
	ValidTo     int64    `json:"validTo" js:"validTo"`
	Protocol    string   `json:"protocol" js:"protocol"`
	SANList     []string `json:"sanList" js:"sanList"`
}

// Response represents a browser HTTP response.
type Response struct {
	ctx               context.Context
	logger            *log.Logger
	request           *Request
	remoteAddress     *RemoteAddress
	securityDetails   *SecurityDetails
	protocol          string
	url               string
	status            int64
	statusText        string
	bodyMu            sync.RWMutex
	body              []byte
	headers           map[string][]string
	fromDiskCache     bool
	fromServiceWorker bool
	fromPrefetchCache bool
	timestamp         time.Time
	wallTime          time.Time
	timing            *network.ResourceTiming
	vu                k6modules.VU

	cachedJSON any
}

// NewHTTPResponse creates a new HTTP response.
func NewHTTPResponse(
	ctx context.Context, req *Request, resp *network.Response, timestamp *cdp.MonotonicTime,
) *Response {
	vu := k6ext.GetVU(ctx)
	state := vu.State()
	r := Response{
		ctx: ctx,
		// TODO: Pass an internal logger instead of basing it on k6's logger?
		// See https://go.k6.io/k6/js/modules/k6/browser/issues/54
		logger:            log.New(state.Logger, GetIterationID(ctx)),
		request:           req,
		remoteAddress:     &RemoteAddress{IPAddress: resp.RemoteIPAddress, Port: resp.RemotePort},
		securityDetails:   nil,
		protocol:          resp.Protocol,
		url:               resp.URL,
		status:            resp.Status,
		statusText:        resp.StatusText,
		body:              nil,
		headers:           make(map[string][]string),
		fromDiskCache:     resp.FromDiskCache,
		fromServiceWorker: resp.FromServiceWorker,
		fromPrefetchCache: resp.FromPrefetchCache,
		timestamp:         timestamp.Time(),
		wallTime:          timestamp.Time().Add(req.offset),
		timing:            resp.Timing,
		vu:                vu,
	}

	for n, v := range resp.Headers {
		s, ok := v.(string)
		if !ok {
			continue
		}
		r.headers[n] = append(r.headers[n], s)
	}

	if resp.SecurityDetails != nil {
		r.securityDetails = &SecurityDetails{
			SubjectName: resp.SecurityDetails.SubjectName,
			Issuer:      resp.SecurityDetails.Issuer,
			ValidFrom:   resp.SecurityDetails.ValidFrom.Time().Unix(),
			ValidTo:     resp.SecurityDetails.ValidTo.Time().Unix(),
			Protocol:    resp.SecurityDetails.Protocol,
			SANList:     resp.SecurityDetails.SanList,
		}
	}

	return &r
}

func (r *Response) fetchBody() error {
	cached := func() bool {
		r.bodyMu.RLock()
		defer r.bodyMu.RUnlock()

		return r.body != nil || r.request.frame == nil
	}
	if cached() {
		return nil
	}
	action := network.GetResponseBody(r.request.requestID)

	// Try to fetch the response body. If the request to retrieve the response
	// body is too "quick" then the response body is not available. After
	// retrying we have a better chance of getting the response body.
	var body []byte
	var err error
	maxRetries := 5
	for i := 0; i <= maxRetries; i++ {
		body, err = action.Do(cdp.WithExecutor(r.ctx, r.request.frame.manager.session))
		if err == nil {
			break
		}
		if strings.Contains(err.Error(), "No data found for resource with given identifier") {
			if i == maxRetries {
				break
			}
			time.Sleep(100 * time.Millisecond)
			continue
		}
		break
	}
	if err != nil {
		return fmt.Errorf("fetching response body: %w", err)
	}
	r.bodyMu.Lock()
	r.body = body
	r.bodyMu.Unlock()

	return nil
}

func (r *Response) headersSize() int64 {
	size := 4 // 4 = 2 spaces + 2 line breaks (HTTP/1.1 200 OK\r\n)
	size += 8 // httpVersion
	size += 3 // statusCode
	size += len(r.statusText)
	for n, v := range r.headers {
		size += len(n) + len(strings.Join(v, "")) + 4 // 4 = ': ' + '\r\n'
	}
	size += 2 // '\r\n'
	return int64(size)
}

// AllHeaders returns all the response headers.
func (r *Response) AllHeaders() map[string]string {
	// TODO: fix this data to include "ExtraInfo" header data
	headers := make(map[string]string)
	for n, v := range r.headers {
		headers[strings.ToLower(n)] = strings.Join(v, ",")
	}
	return headers
}

// Body returns the response body as a bytes buffer.
func (r *Response) Body() ([]byte, error) {
	if r.status >= 300 && r.status <= 399 {
		return nil, fmt.Errorf("response body is unavailable for redirect responses")
	}
	if err := r.fetchBody(); err != nil {
		return nil, fmt.Errorf("getting response body: %w", err)
	}

	r.bodyMu.RLock()
	defer r.bodyMu.RUnlock()

	return r.body, nil
}

// bodySize returns the size in bytes of the response body.
func (r *Response) bodySize() int64 {
	// Skip redirect responses
	if r.status >= 300 && r.status <= 399 {
		return 0
	}

	if err := r.fetchBody(); err != nil {
		r.logger.Debugf("Response:bodySize:fetchBody",
			"url:%s method:%s err:%s", r.url, r.request.method, err)
	}

	r.bodyMu.RLock()
	defer r.bodyMu.RUnlock()

	return int64(len(r.body))
}

// Frame returns the frame within which the response was received.
func (r *Response) Frame() *Frame {
	return r.request.frame
}

// HeaderValue returns the value of the given header.
// Returns true if the header is present, false otherwise.
func (r *Response) HeaderValue(name string) (string, bool) {
	headers := r.AllHeaders()
	v, ok := headers[strings.ToLower(name)]
	return v, ok
}

// HeaderValues returns the values of the given header.
func (r *Response) HeaderValues(name string) []string {
	headers := r.AllHeaders()
	return strings.Split(headers[name], ",")
}

// FromCache returns whether this response was served from disk cache.
func (r *Response) FromCache() bool {
	return r.fromDiskCache
}

// FromPrefetchCache returns whether this response was served from prefetch cache.
func (r *Response) FromPrefetchCache() bool {
	return r.fromPrefetchCache
}

// FromServiceWorker returns whether this response was served by a service worker.
func (r *Response) FromServiceWorker() bool {
	return r.fromServiceWorker
}

// Headers returns the response headers.
func (r *Response) Headers() map[string]string {
	headers := make(map[string]string)
	for n, v := range r.headers {
		headers[n] = strings.Join(v, ",")
	}
	return headers
}

// HeadersArray returns the response headers as an array of objects.
func (r *Response) HeadersArray() []HTTPHeader {
	headers := make([]HTTPHeader, 0)
	for n, vals := range r.headers {
		for _, v := range vals {
			headers = append(headers, HTTPHeader{Name: n, Value: v})
		}
	}
	return headers
}

// JSON returns the response body as JSON data.
func (r *Response) JSON() (any, error) {
	if r.cachedJSON != nil {
		return r.cachedJSON, nil
	}
	if err := r.fetchBody(); err != nil {
		return nil, fmt.Errorf("getting response body: %w", err)
	}

	r.bodyMu.RLock()
	defer r.bodyMu.RUnlock()

	var v any
	if err := json.Unmarshal(r.body, &v); err != nil {
		return nil, fmt.Errorf("unmarshalling response body to JSON: %w", err)
	}
	r.cachedJSON = v

	return v, nil
}

// Ok returns true if status code of response if considered ok, otherwise returns false.
func (r *Response) Ok() bool {
	if r.status == 0 || (r.status >= 200 && r.status <= 299) {
		return true
	}
	return false
}

// Request returns the request that led to this response.
func (r *Response) Request() *Request {
	return r.request
}

// SecurityDetails returns the security details of the response.
func (r *Response) SecurityDetails() *SecurityDetails {
	return r.securityDetails
}

// ServerAddr returns the remote address of the server.
func (r *Response) ServerAddr() *RemoteAddress {
	return r.remoteAddress
}

// Size returns the size in bytes of the response.
func (r *Response) Size() HTTPMessageSize {
	return HTTPMessageSize{
		Body:    r.bodySize(),
		Headers: r.headersSize(),
	}
}

// Status returns the response status code.
func (r *Response) Status() int64 {
	return r.status
}

// StatusText returns the response status text.
func (r *Response) StatusText() string {
	return r.statusText
}

// Text returns the response body as a string.
func (r *Response) Text() (string, error) {
	if err := r.fetchBody(); err != nil {
		return "", fmt.Errorf("getting response body as text: %w", err)
	}

	r.bodyMu.RLock()
	defer r.bodyMu.RUnlock()

	return string(r.body), nil
}

// URL returns the request URL.
func (r *Response) URL() string {
	return r.url
}
