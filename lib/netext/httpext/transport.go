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
	"net"
	"net/http"
	"net/http/httptrace"
	"strconv"
	"sync"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/netext"
	"github.com/loadimpact/k6/stats"
)

// transport is an implementation of http.RoundTripper that will measure and emit
// different metrics for each roundtrip
type transport struct {
	ctx   context.Context
	state *lib.State
	tags  map[string]string

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
	tags map[string]string,
) *transport {
	return &transport{
		ctx:             ctx,
		state:           state,
		tags:            tags,
		lastRequestLock: new(sync.Mutex),
	}
}

// Helper method to finish the tracer trail, assemble the tag values and emits
// the metric samples for the supplied unfinished request.
func (t *transport) measureAndEmitMetrics(unfReq *unfinishedRequest) *finishedRequest {
	trail := unfReq.tracer.Done()

	tags := map[string]string{}
	for k, v := range t.tags {
		tags[k] = v
	}

	result := &finishedRequest{
		unfinishedRequest: unfReq,
		trail:             trail,
	}

	enabledTags := t.state.Options.SystemTags

	urlEnabled := enabledTags.Has(stats.TagURL)
	var setName bool
	if _, ok := tags["name"]; !ok && enabledTags.Has(stats.TagName) {
		setName = true
	}
	if urlEnabled || setName {
		cleanURL := URL{u: unfReq.request.URL, URL: unfReq.request.URL.String()}.Clean()
		if urlEnabled {
			tags["url"] = cleanURL
		}
		if setName {
			tags["name"] = cleanURL
		}
	}

	if enabledTags.Has(stats.TagMethod) {
		tags["method"] = unfReq.request.Method
	}

	if unfReq.err != nil {
		result.errorCode, result.errorMsg = errorCodeForError(unfReq.err)
		if enabledTags.Has(stats.TagError) {
			tags["error"] = result.errorMsg
		}

		if enabledTags.Has(stats.TagErrorCode) {
			tags["error_code"] = strconv.Itoa(int(result.errorCode))
		}

		if enabledTags.Has(stats.TagStatus) {
			tags["status"] = "0"
		}
	} else {
		if enabledTags.Has(stats.TagStatus) {
			tags["status"] = strconv.Itoa(unfReq.response.StatusCode)
		}
		if unfReq.response.StatusCode >= 400 {
			if enabledTags.Has(stats.TagErrorCode) {
				result.errorCode = errCode(1000 + unfReq.response.StatusCode)
				tags["error_code"] = strconv.Itoa(int(result.errorCode))
			}
		}
		if enabledTags.Has(stats.TagProto) {
			tags["proto"] = unfReq.response.Proto
		}

		if unfReq.response.TLS != nil {
			tlsInfo, oscp := netext.ParseTLSConnState(unfReq.response.TLS)
			if enabledTags.Has(stats.TagTLSVersion) {
				tags["tls_version"] = tlsInfo.Version
			}
			if enabledTags.Has(stats.TagOCSPStatus) {
				tags["ocsp_status"] = oscp.Status
			}
			result.tlsInfo = tlsInfo
		}
	}
	if enabledTags.Has(stats.TagIP) && trail.ConnRemoteAddr != nil {
		if ip, _, err := net.SplitHostPort(trail.ConnRemoteAddr.String()); err == nil {
			tags["ip"] = ip
		}
	}

	trail.SaveSamples(stats.IntoSampleTags(&tags))
	stats.PushIfNotDone(t.ctx, t.state.Samples, trail)

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

	t.saveCurrentRequest(&unfinishedRequest{
		ctx:      ctx,
		tracer:   tracer,
		request:  req,
		response: resp,
		err:      err,
	})

	return resp, err
}
