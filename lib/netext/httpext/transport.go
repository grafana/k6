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
	tags             map[string]string
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
	tags map[string]string,
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

	tags := map[string]string{}
	for k, v := range t.tags {
		tags[k] = v
	}

	result := &finishedRequest{
		unfinishedRequest: unfReq,
		trail:             trail,
	}

	enabledTags := t.state.Options.SystemTags
	urlEnabled := enabledTags.Has(metrics.TagURL)
	var setName bool
	if _, ok := tags["name"]; !ok && enabledTags.Has(metrics.TagName) {
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

	if enabledTags.Has(metrics.TagMethod) {
		tags["method"] = unfReq.request.Method
	}

	if unfReq.err != nil {
		result.errorCode, result.errorMsg = errorCodeForError(unfReq.err)
		if enabledTags.Has(metrics.TagError) {
			tags["error"] = result.errorMsg
		}

		if enabledTags.Has(metrics.TagErrorCode) {
			tags["error_code"] = strconv.Itoa(int(result.errorCode))
		}

		if enabledTags.Has(metrics.TagStatus) {
			tags["status"] = "0"
		}
	} else {
		if enabledTags.Has(metrics.TagStatus) {
			tags["status"] = strconv.Itoa(unfReq.response.StatusCode)
		}
		if unfReq.response.StatusCode >= 400 {
			if enabledTags.Has(metrics.TagErrorCode) {
				result.errorCode = errCode(1000 + unfReq.response.StatusCode)
				tags["error_code"] = strconv.Itoa(int(result.errorCode))
			}
		}
		if enabledTags.Has(metrics.TagProto) {
			tags["proto"] = unfReq.response.Proto
		}

		if unfReq.response.TLS != nil {
			tlsInfo, oscp := netext.ParseTLSConnState(unfReq.response.TLS)
			if enabledTags.Has(metrics.TagTLSVersion) {
				tags["tls_version"] = tlsInfo.Version
			}
			if enabledTags.Has(metrics.TagOCSPStatus) {
				tags["ocsp_status"] = oscp.Status
			}
			result.tlsInfo = tlsInfo
		}
	}
	if enabledTags.Has(metrics.TagIP) && trail.ConnRemoteAddr != nil {
		if ip, _, err := net.SplitHostPort(trail.ConnRemoteAddr.String()); err == nil {
			tags["ip"] = ip
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
			tags[metrics.TagExpectedResponse.String()] = strconv.FormatBool(expected)
		}
	}

	finalTags := metrics.IntoSampleTags(&tags)
	builtinMetrics := t.state.BuiltinMetrics
	trail.SaveSamples(builtinMetrics, finalTags)
	if t.responseCallback != nil {
		trail.Failed.Valid = true
		if failed == 1 {
			trail.Failed.Bool = true
		}
		trail.Samples = append(trail.Samples,
			metrics.Sample{
				Metric: builtinMetrics.HTTPReqFailed, Time: trail.EndTime, Tags: finalTags, Value: failed,
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
