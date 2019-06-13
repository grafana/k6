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

// transport is an implemenation of http.RoundTripper that will measure and emit
// different metrics for each roundtrip
type transport struct {
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

// NewTransport returns a new Transport wrapping around the provide Roundtripper and will send
// samples on the provided channel adding the tags in accordance to the Options provided
func newTransport(
	state *lib.State,
	tags map[string]string,
) *transport {
	return &transport{
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
	if unfReq.err != nil {
		result.errorCode, result.errorMsg = errorCodeForError(unfReq.err)
		if enabledTags["error"] {
			tags["error"] = result.errorMsg
		}

		if enabledTags["error_code"] {
			tags["error_code"] = strconv.Itoa(int(result.errorCode))
		}

		if enabledTags["status"] {
			tags["status"] = "0"
		}
	} else {
		if enabledTags["url"] {
			tags["url"] = unfReq.request.URL.String()
		}
		if enabledTags["status"] {
			tags["status"] = strconv.Itoa(unfReq.response.StatusCode)
		}
		if unfReq.response.StatusCode >= 400 {
			if enabledTags["error_code"] {
				result.errorCode = errCode(1000 + unfReq.response.StatusCode)
				tags["error_code"] = strconv.Itoa(int(result.errorCode))
			}
		}
		if enabledTags["proto"] {
			tags["proto"] = unfReq.response.Proto
		}

		if unfReq.response.TLS != nil {
			tlsInfo, oscp := netext.ParseTLSConnState(unfReq.response.TLS)
			if enabledTags["tls_version"] {
				tags["tls_version"] = tlsInfo.Version
			}
			if enabledTags["ocsp_status"] {
				tags["ocsp_status"] = oscp.Status
			}
			result.tlsInfo = tlsInfo
		}
	}
	if enabledTags["ip"] && trail.ConnRemoteAddr != nil {
		if ip, _, err := net.SplitHostPort(trail.ConnRemoteAddr.String()); err == nil {
			tags["ip"] = ip
		}
	}

	trail.SaveSamples(stats.IntoSampleTags(&tags))
	stats.PushIfNotCancelled(unfReq.ctx, t.state.Samples, trail)

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

func (t *transport) processLastSavedRequest() *finishedRequest {
	t.lastRequestLock.Lock()
	unprocessedRequest := t.lastRequest
	t.lastRequest = nil
	t.lastRequestLock.Unlock()

	if unprocessedRequest != nil {
		return t.measureAndEmitMetrics(unprocessedRequest)
	}
	return nil
}

// RoundTrip is the implementation of http.RoundTripper
func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.processLastSavedRequest()

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
