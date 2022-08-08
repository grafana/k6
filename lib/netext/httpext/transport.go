/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
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

package httpext

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptrace"
	"strconv"
	"sync"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/netext"
	"go.k6.io/k6/metrics"
)

// transport is an implementation of http.RoundTripper that will measure and emit
// different metrics for each roundtrip
type transport struct {
	ctx              context.Context
	state            *lib.State
	tags             *metrics.TagSet
	responseCallback func(int) bool

	lastRequest     *unfinishedRequest
	lastRequestLock *sync.Mutex
}

// unfinishedRequest stores the request and the raw result returned from the
// underlying http.RoundTripper, but before its body has been read
type unfinishedRequest struct {
	ctx      context.Context
	tracer   *Tracer
	request  *http.Request
	response *http.Response
	err      error
}

// finishedRequest is produced once the request has been finalized; it is
// triggered either by a subsequent RoundTrip, or for the last request in the
// chain - by the MakeRequest function manually calling the transport method
// processLastSavedRequest(), after reading the HTTP response body.
type finishedRequest struct {
	*unfinishedRequest
	trail     *Trail
	tlsInfo   netext.TLSInfo
	errorCode errCode
	errorMsg  string
}

var _ http.RoundTripper = &transport{}

// newTransport returns a new http.RoundTripper implementation that wraps around
// the provided state's Transport. It uses a httpext.Tracer to measure all HTTP
// requests made through it and annotates and emits the recorded metric samples
// through the state.Samples channel.
func newTransport(
	ctx context.Context,
	state *lib.State,
	tags *metrics.TagSet,
	responseCallback func(int) bool,
) *transport {
	return &transport{
		ctx:              ctx,
		state:            state,
		tags:             tags,
		responseCallback: responseCallback,
		lastRequestLock:  new(sync.Mutex),
	}
}

// Helper method to finish the tracer trail, assemble the tag values and emits
// the metric samples for the supplied unfinished request.
//nolint:nestif,funlen
func (t *transport) measureAndEmitMetrics(unfReq *unfinishedRequest) *finishedRequest {
	trail := unfReq.tracer.Done()

	result := &finishedRequest{
		unfinishedRequest: unfReq,
		trail:             trail,
	}

	tags := t.tags
	enabledTags := t.state.Options.SystemTags
	urlEnabled := enabledTags.Has(metrics.TagURL)
	var setName bool
	if _, ok := tags.Get("name"); !ok && enabledTags.Has(metrics.TagName) {
		setName = true
	}
	if urlEnabled || setName {
		cleanURL := URL{u: unfReq.request.URL, URL: unfReq.request.URL.String()}.Clean()
		if urlEnabled {
			tags = tags.With("url", cleanURL)
		}
		if setName {
			tags = tags.With("name", cleanURL)
		}
	}

	if enabledTags.Has(metrics.TagMethod) {
		tags = tags.With("method", unfReq.request.Method)
	}

	if unfReq.err != nil {
		result.errorCode, result.errorMsg = errorCodeForError(unfReq.err)
		if enabledTags.Has(metrics.TagError) {
			tags = tags.With("error", result.errorMsg)
		}

		if enabledTags.Has(metrics.TagErrorCode) {
			tags = tags.With("error_code", strconv.Itoa(int(result.errorCode)))
		}

		if enabledTags.Has(metrics.TagStatus) {
			tags = tags.With("status", "0")
		}
	} else {
		if enabledTags.Has(metrics.TagStatus) {
			tags = tags.With("status", strconv.Itoa(unfReq.response.StatusCode))
		}
		if unfReq.response.StatusCode >= 400 {
			if enabledTags.Has(metrics.TagErrorCode) {
				result.errorCode = errCode(1000 + unfReq.response.StatusCode)
				tags = tags.With("error_code", strconv.Itoa(int(result.errorCode)))
			}
		}
		if enabledTags.Has(metrics.TagProto) {
			tags = tags.With("proto", unfReq.response.Proto)
		}

		if unfReq.response.TLS != nil {
			tlsInfo, oscp := netext.ParseTLSConnState(unfReq.response.TLS)
			if enabledTags.Has(metrics.TagTLSVersion) {
				tags = tags.With("tls_version", tlsInfo.Version)
			}
			if enabledTags.Has(metrics.TagOCSPStatus) {
				tags = tags.With("ocsp_status", oscp.Status)
			}
			result.tlsInfo = tlsInfo
		}
	}
	if enabledTags.Has(metrics.TagIP) && trail.ConnRemoteAddr != nil {
		if ip, _, err := net.SplitHostPort(trail.ConnRemoteAddr.String()); err == nil {
			tags = tags.With("ip", ip)
		}
	}
	var failed float64
	if t.responseCallback != nil {
		var statusCode int
		if unfReq.err == nil {
			statusCode = unfReq.response.StatusCode
		}
		expected := t.responseCallback(statusCode)
		if !expected {
			failed = 1
		}

		if enabledTags.Has(metrics.TagExpectedResponse) {
			tags = tags.With(metrics.TagExpectedResponse.String(), strconv.FormatBool(expected))
		}
	}

	trail.SaveSamples(t.state.BuiltinMetrics, tags)
	if t.responseCallback != nil {
		trail.Failed.Valid = true
		if failed == 1 {
			trail.Failed.Bool = true
		}
		trail.Samples = append(trail.Samples,
			metrics.Sample{
				Metric: t.state.BuiltinMetrics.HTTPReqFailed,
				Time:   trail.EndTime,
				Tags:   tags,
				Value:  failed,
			},
		)
	}
	metrics.PushIfNotDone(t.ctx, t.state.Samples, trail)
	return result
}

func (t *transport) saveCurrentRequest(currentRequest *unfinishedRequest) {
	t.lastRequestLock.Lock()
	unprocessedRequest := t.lastRequest
	t.lastRequest = currentRequest
	t.lastRequestLock.Unlock()

	if unprocessedRequest != nil {
		// This shouldn't happen, since we have one transport per request, but just in case...
		t.state.Logger.Warnf("TracerTransport: unexpected unprocessed request for %s", unprocessedRequest.request.URL)
		t.measureAndEmitMetrics(unprocessedRequest)
	}
}

func (t *transport) processLastSavedRequest(lastErr error) *finishedRequest {
	t.lastRequestLock.Lock()
	unprocessedRequest := t.lastRequest
	t.lastRequest = nil
	t.lastRequestLock.Unlock()

	if unprocessedRequest != nil {
		// We don't want to overwrite any previous errors, but if there were
		// none and we (i.e. the MakeRequest() function) have one, save it
		// before we emit the metrics.
		if unprocessedRequest.err == nil && lastErr != nil {
			unprocessedRequest.err = lastErr
		}

		return t.measureAndEmitMetrics(unprocessedRequest)
	}
	return nil
}

// RoundTrip is the implementation of http.RoundTripper
func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.processLastSavedRequest(nil)

	ctx := req.Context()
	tracer := &Tracer{}
	reqWithTracer := req.WithContext(httptrace.WithClientTrace(ctx, tracer.Trace()))
	resp, err := t.state.Transport.RoundTrip(reqWithTracer)

	var netError net.Error
	if errors.As(err, &netError) && netError.Timeout() {
		var netOpError *net.OpError
		if errors.As(err, &netOpError) && netOpError.Op == "dial" {
			err = NewK6Error(tcpDialTimeoutErrorCode, tcpDialTimeoutErrorCodeMsg, netError)
		} else {
			err = NewK6Error(requestTimeoutErrorCode, requestTimeoutErrorCodeMsg, netError)
		}
	}

	t.saveCurrentRequest(&unfinishedRequest{
		ctx:      ctx,
		tracer:   tracer,
		request:  req,
		response: resp,
		err:      err,
	})

	return resp, err
}
