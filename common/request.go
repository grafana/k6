package common

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/grafana/xk6-browser/api"
	"github.com/grafana/xk6-browser/k6ext"

	k6modules "go.k6.io/k6/js/modules"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/dop251/goja"
)

// Ensure Request implements the api.Request interface.
var _ api.Request = &Request{}

// Request represents a browser HTTP request.
type Request struct {
	ctx                 context.Context
	frame               *Frame
	responseMu          sync.RWMutex
	response            *Response
	redirectChain       []*Request
	requestID           network.RequestID
	documentID          string
	url                 *url.URL
	method              string
	headers             map[string][]string
	postData            string
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
func NewRequest(ctx context.Context, rp NewRequestParams) (*Request, error) {
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

	r := Request{
		url:                 u,
		frame:               rp.frame,
		redirectChain:       rp.redirectChain,
		requestID:           ev.RequestID,
		method:              ev.Request.Method,
		postData:            ev.Request.PostData,
		resourceType:        ev.Type.String(),
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

func (r *Request) AllHeaders() map[string]string {
	// TODO: fix this data to include "ExtraInfo" header data
	headers := make(map[string]string)
	for n, v := range r.headers {
		headers[strings.ToLower(n)] = strings.Join(v, ",")
	}
	return headers
}

func (r *Request) Failure() goja.Value {
	k6ext.Panic(r.ctx, "Request.failure() has not been implemented yet")
	return nil
}

// Frame returns the frame within which the request was made.
func (r *Request) Frame() api.Frame {
	return r.frame
}

func (r *Request) HeaderValue(name string) goja.Value {
	rt := r.vu.Runtime()
	headers := r.AllHeaders()
	val, ok := headers[name]
	if !ok {
		return goja.Null()
	}
	return rt.ToValue(val)
}

// Headers returns the request headers.
func (r *Request) Headers() map[string]string {
	headers := make(map[string]string)
	for n, v := range r.headers {
		headers[n] = strings.Join(v, ",")
	}
	return headers
}

func (r *Request) HeadersArray() []api.HTTPHeader {
	headers := make([]api.HTTPHeader, 0)
	for n, vals := range r.headers {
		for _, v := range vals {
			headers = append(headers, api.HTTPHeader{Name: n, Value: v})
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
func (r *Request) PostData() string {
	return r.postData
}

// PostDataBuffer returns the request post data as an ArrayBuffer.
func (r *Request) PostDataBuffer() goja.ArrayBuffer {
	rt := r.vu.Runtime()
	return rt.NewArrayBuffer([]byte(r.postData))
}

// PostDataJSON returns the request post data as a JS object.
func (r *Request) PostDataJSON() string {
	k6ext.Panic(r.ctx, "Request.postDataJSON() has not been implemented yet")
	return ""
}

func (r *Request) RedirectedFrom() api.Request {
	k6ext.Panic(r.ctx, "Request.redirectedFrom() has not been implemented yet")
	return nil
}

func (r *Request) RedirectedTo() api.Request {
	k6ext.Panic(r.ctx, "Request.redirectedTo() has not been implemented yet")
	return nil
}

// ResourceType returns the request resource type.
func (r *Request) ResourceType() string {
	return r.resourceType
}

// Response returns the response for the request, if received.
func (r *Request) Response() api.Response {
	return r.response
}

func (r *Request) Size() api.HTTPMessageSize {
	return api.HTTPMessageSize{
		Body:    int64(len(r.postData)),
		Headers: r.headersSize(),
	}
}

func (r *Request) Timing() goja.Value {
	rt := r.vu.Runtime()
	timing := r.response.timing
	return rt.ToValue(&ResourceTiming{
		StartTime:             (timing.RequestTime - float64(r.timestamp.Unix()) + float64(r.wallTime.Unix())) * 1000,
		DomainLookupStart:     timing.DNSStart,
		DomainLookupEnd:       timing.DNSEnd,
		ConnectStart:          timing.ConnectStart,
		SecureConnectionStart: timing.SslStart,
		ConnectEnd:            timing.ConnectEnd,
		RequestStart:          timing.SendStart,
		ResponseStart:         timing.ReceiveHeadersEnd,
		ResponseEnd:           r.responseEndTiming,
	})
}

// URL returns the request URL.
func (r *Request) URL() string {
	return r.url.String()
}
