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
	"net/url"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/dop251/goja"
	k6common "go.k6.io/k6/js/common"

	"github.com/grafana/xk6-browser/api"
)

// Ensure Request implements the api.Request interface.
var _ api.Request = &Request{}

// Request represents a browser HTTP request.
type Request struct {
	ctx                 context.Context
	frame               *Frame
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
	timestamp           time.Time
	wallTime            time.Time
	responseEndTiming   float64
}

// NewRequest creates a new HTTP request.
func NewRequest(
	ctx context.Context, event *network.EventRequestWillBeSent, f *Frame,
	redirectChain []*Request, interceptionID string, allowInterception bool,
) (*Request, error) {
	documentID := cdp.LoaderID("")
	if event.RequestID == network.RequestID(event.LoaderID) && event.Type == "Document" {
		documentID = event.LoaderID
	}

	u, err := url.Parse(event.Request.URL)
	if err != nil {
		return nil, fmt.Errorf("cannot parse URL: %w", err)
	}

	r := Request{
		ctx:                 ctx,
		frame:               f,
		response:            nil,
		redirectChain:       redirectChain,
		requestID:           event.RequestID,
		documentID:          documentID.String(),
		url:                 u,
		method:              event.Request.Method,
		headers:             make(map[string][]string),
		postData:            event.Request.PostData,
		resourceType:        event.Type.String(),
		isNavigationRequest: string(event.RequestID) == string(event.LoaderID) && event.Type == network.ResourceTypeDocument,
		allowInterception:   allowInterception,
		interceptionID:      interceptionID,
		fromMemoryCache:     false,
		errorText:           "",
		timestamp:           event.Timestamp.Time(),
		wallTime:            event.WallTime.Time(),
	}
	for n, v := range event.Request.Headers {
		switch v := v.(type) {
		case string:
			if _, ok := r.headers[n]; !ok {
				r.headers[n] = []string{v}
			} else {
				r.headers[n] = append(r.headers[n], v)
			}
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
	rt := k6common.GetRuntime(r.ctx)
	k6common.Throw(rt, errors.New("Request.failure() has not been implemented yet"))
	return nil
}

// Frame returns the frame within which the request was made.
func (r *Request) Frame() api.Frame {
	return r.frame
}

func (r *Request) HeaderValue(name string) goja.Value {
	rt := k6common.GetRuntime(r.ctx)
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
	rt := k6common.GetRuntime(r.ctx)
	return rt.NewArrayBuffer([]byte(r.postData))
}

// PostDataJSON returns the request post data as a JS object.
func (r *Request) PostDataJSON() string {
	rt := k6common.GetRuntime(r.ctx)
	k6common.Throw(rt, errors.New("Request.postDataJSON() has not been implemented yet"))
	return ""
}

func (r *Request) RedirectedFrom() api.Request {
	rt := k6common.GetRuntime(r.ctx)
	k6common.Throw(rt, errors.New("Request.redirectedFrom() has not been implemented yet"))
	return nil
}

func (r *Request) RedirectedTo() api.Request {
	rt := k6common.GetRuntime(r.ctx)
	k6common.Throw(rt, errors.New("Request.redirectedTo() has not been implemented yet"))
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
	rt := k6common.GetRuntime(r.ctx)
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
