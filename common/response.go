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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/dop251/goja"
	"github.com/grafana/xk6-browser/api"
	k6common "go.k6.io/k6/js/common"
	k6lib "go.k6.io/k6/lib"
)

// Ensure Response implements the api.Response interface
var _ api.Response = &Response{}

// RemoteAddress contains informationa about a remote target
type RemoteAddress struct {
	IPAddress string `json:"ipAddress"`
	Port      int64  `json:"port"`
}

// SecurityDetails contains informationa about the security details of a TLS connection
type SecurityDetails struct {
	SubjectName string   `json:"subjectName"`
	Issuer      string   `json:"issuer"`
	ValidFrom   int64    `json:"validFrom"`
	ValidTo     int64    `json:"validTo"`
	Protocol    string   `json:"protocol"`
	SANList     []string `json:"sanList"`
}

// Response represents a browser HTTP response
type Response struct {
	ctx               context.Context
	logger            *Logger
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
	responseTime      time.Time
	timing            *network.ResourceTiming

	cachedJSON interface{}
}

// NewHTTPResponse creates a new HTTP response
func NewHTTPResponse(ctx context.Context, req *Request, resp *network.Response, timestamp *cdp.MonotonicTime) *Response {
	state := k6lib.GetState(ctx)
	r := Response{
		ctx: ctx,
		// TODO: Pass an internal logger instead of basing it on k6's logger?
		// See https://github.com/grafana/xk6-browser/issues/54
		logger:            NewLogger(ctx, state.Logger, false, nil),
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
		responseTime:      time.Time{},
		timing:            resp.Timing,
	}

	for n, v := range resp.Headers {
		switch v := v.(type) {
		case string:
			if _, ok := r.headers[n]; !ok {
				r.headers[n] = []string{v}
			} else {
				r.headers[n] = append(r.headers[n], v)
			}
		}
	}

	if resp.ResponseTime != nil {
		r.responseTime = resp.ResponseTime.Time()
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
	body, err := action.Do(cdp.WithExecutor(r.ctx, r.request.frame.manager.session))
	if err != nil {
		return err
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

func (r *Response) AllHeaders() map[string]string {
	// TODO: fix this data to include "ExtraInfo" header data
	headers := make(map[string]string)
	for n, v := range r.headers {
		headers[strings.ToLower(n)] = strings.Join(v, ",")
	}
	return headers
}

// Body returns the response body as a binary buffer
func (r *Response) Body() goja.ArrayBuffer {
	rt := k6common.GetRuntime(r.ctx)
	if r.status >= 300 && r.status <= 399 {
		k6common.Throw(rt, errors.New("Response body is unavailable for redirect responses"))
	}
	if err := r.fetchBody(); err != nil {
		k6common.Throw(rt, err)
	}
	r.bodyMu.RLock()
	defer r.bodyMu.RUnlock()
	return rt.NewArrayBuffer(r.body)
}

// bodySize returns the size in bytes of the response body.
// It first attempts to get the size specified in the content-length
// header, and if unavailable falls back to the size as returned by CDP in
// r.fetchBody(). This is because the CDP Network.getResponseBody call
// is unreliable, see https://github.com/ChromeDevTools/devtools-protocol/issues/12#issuecomment-306947275 .
func (r *Response) bodySize() int64 {
	if v, ok := r.headers["content-length"]; ok && len(v) > 0 {
		cl, err := strconv.ParseInt(v[0], 10, 64)
		if err == nil {
			return cl
		}
		r.logger.Warnf("cdp", "error parsing content-length header: %s", err)
	}

	if err := r.fetchBody(); err != nil {
		r.logger.Warnf("cdp", "error fetching response body for '%s': %s", r.url, err)
	}

	r.bodyMu.RLock()
	defer r.bodyMu.RUnlock()
	return int64(len(r.body))
}

// Finished waits for response to finish, return error if request failed
func (r *Response) Finished() bool {
	// TODO: should return nil|Error
	rt := k6common.GetRuntime(r.ctx)
	k6common.Throw(rt, errors.New("Response.finished() has not been implemented yet"))
	return false
}

// Frame returns the frame within which the response was received
func (r *Response) Frame() api.Frame {
	return r.request.frame
}

func (r *Response) HeaderValue(name string) goja.Value {
	rt := k6common.GetRuntime(r.ctx)
	headers := r.AllHeaders()
	val, ok := headers[name]
	if !ok {
		return goja.Null()
	}
	return rt.ToValue(val)
}

func (r *Response) HeaderValues(name string) []string {
	headers := r.AllHeaders()
	return strings.Split(headers[name], ",")
}

// FromCache returns whether this response was served from disk cache
func (r *Response) FromCache() bool {
	return r.fromDiskCache
}

// FromPrefetchCache returns whether this response was served from prefetch cache
func (r *Response) FromPrefetchCache() bool {
	return r.fromPrefetchCache
}

// FromServiceWorker returns whether this response was served by a service worker
func (r *Response) FromServiceWorker() bool {
	return r.fromServiceWorker
}

// Headers returns the response headers
func (r *Response) Headers() map[string]string {
	headers := make(map[string]string)
	for n, v := range r.headers {
		headers[n] = strings.Join(v, ",")
	}
	return headers
}

func (r *Response) HeadersArray() []api.HTTPHeader {
	headers := make([]api.HTTPHeader, 0)
	for n, vals := range r.headers {
		for _, v := range vals {
			headers = append(headers, api.HTTPHeader{Name: n, Value: v})
		}
	}
	return headers
}

// JSON returns the response body as JSON data
func (r *Response) JSON() goja.Value {
	rt := k6common.GetRuntime(r.ctx)
	if r.cachedJSON == nil {
		if err := r.fetchBody(); err != nil {
			k6common.Throw(rt, err)
		}

		var v interface{}
		r.bodyMu.RLock()
		defer r.bodyMu.RUnlock()
		if err := json.Unmarshal(r.body, &v); err != nil {
			k6common.Throw(rt, err)
		}
		r.cachedJSON = v
	}
	return rt.ToValue(r.cachedJSON)
}

// Ok returns true if status code of response if considered ok, otherwise returns false
func (r *Response) Ok() bool {
	if r.status == 0 || (r.status >= 200 && r.status <= 299) {
		return true
	}
	return false
}

// Request returns the request that led to this response
func (r *Response) Request() api.Request {
	return r.request
}

func (r *Response) SecurityDetails() goja.Value {
	rt := k6common.GetRuntime(r.ctx)
	return rt.ToValue(r.securityDetails)
}

// ServerAdd returns the remote address of the server
func (r *Response) ServerAddr() goja.Value {
	rt := k6common.GetRuntime(r.ctx)
	return rt.ToValue(r.remoteAddress)
}

func (r *Response) Size() api.HTTPMessageSize {
	return api.HTTPMessageSize{
		Body:    r.bodySize(),
		Headers: r.headersSize(),
	}
}

// Status returns the response status code
func (r *Response) Status() int64 {
	return r.status
}

// StatusText returns the response status text
func (r *Response) StatusText() string {
	return r.statusText
}

// Text returns the response body as a string
func (r *Response) Text() string {
	rt := k6common.GetRuntime(r.ctx)
	if err := r.fetchBody(); err != nil {
		k6common.Throw(rt, err)
	}
	r.bodyMu.RLock()
	defer r.bodyMu.RUnlock()
	return string(r.body)
}

// URL returns the request URL
func (r *Response) URL() string {
	return r.url
}
