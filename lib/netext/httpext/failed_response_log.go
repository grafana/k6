package httpext

import (
	"context"
	"fmt"
	"maps"
	"net/http"

	"github.com/sirupsen/logrus"

	"go.k6.io/k6/v2/lib"
)

// maxLoggedFailedResponseBodySize is the maximum number of bytes of a response body that will be
// included in a failed response log entry. Longer bodies are truncated to avoid flooding the logs.
const maxLoggedFailedResponseBodySize = 64 * 1024

// Reasons used for failed response log entries.
const (
	FailedResponseReasonRequestError = "request error"
	FailedResponseReasonUnexpected   = "unexpected response"
	FailedResponseReasonCheckFailed  = "check failed"
)

// LogFailedResponse logs the details (status, error, headers and body) of a failed HTTP response.
// It is used when the logFailedResponses option is enabled. Extra fields (e.g. the name of a failed
// check) can be provided and are added to the log entry.
func LogFailedResponse(logger logrus.FieldLogger, resp *Response, reason string, extra logrus.Fields) {
	if logger == nil || resp == nil {
		return
	}

	fields := logrus.Fields{
		"source": "failed-response",
		"reason": reason,
		"status": resp.Status,
		"url":    resp.URL,
	}
	if resp.Request != nil {
		fields["method"] = resp.Request.Method
	}
	if resp.Error != "" {
		fields["error"] = resp.Error
	}
	if resp.ErrorCode != 0 {
		fields["error_code"] = resp.ErrorCode
	}
	fields["headers"] = resp.Headers
	fields["body"] = failedResponseBody(resp.Body)
	maps.Copy(fields, extra)

	logger.WithFields(fields).Warn("Failed HTTP response")
}

// logFailedRequest logs the response of a request made by MakeRequest if it is considered failed:
// either the request errored (e.g. timeout, connection refused), or the response was unexpected.
// A response is unexpected according to the responseCallback when one is set (i.e. the same
// semantics as the http_req_failed metric), otherwise when its status code is >= 400.
func logFailedRequest(
	ctx context.Context, state *lib.State, resp *Response, finishedReq *finishedRequest, resErr error,
) {
	extra := logrus.Fields{"vu": state.VUID, "iter": state.Iteration}
	var reason string
	switch {
	case resErr != nil:
		// Do *not* log errors caused by the context being cancelled (e.g. the test ending).
		select {
		case <-ctx.Done():
			return
		default:
		}
		reason = FailedResponseReasonRequestError
		if resp.Error == "" {
			extra["error"] = resErr.Error()
		}
	case finishedReq != nil && finishedReq.trail.Failed.Valid:
		if !finishedReq.trail.Failed.Bool {
			return
		}
		reason = FailedResponseReasonUnexpected
	case resp.Status >= http.StatusBadRequest:
		reason = FailedResponseReasonUnexpected
	default:
		return
	}

	LogFailedResponse(state.Logger, resp, reason, extra)
}

func failedResponseBody(body any) string {
	var s string
	switch b := body.(type) {
	case nil:
		return ""
	case string:
		s = b
	case []byte:
		s = string(b)
	default:
		s = fmt.Sprintf("%v", b)
	}
	if len(s) > maxLoggedFailedResponseBodySize {
		return fmt.Sprintf("%s... [truncated, %d bytes total]", s[:maxLoggedFailedResponseBodySize], len(s))
	}
	return s
}
