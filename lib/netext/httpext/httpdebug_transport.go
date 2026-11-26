package httpext

import (
	"bytes"
	"net/http"
	"net/http/httputil"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"go.k6.io/k6/v2/internal/lib/netext/httpdebug"
)

type httpDebugTransport struct {
	originalTransport http.RoundTripper
	httpDebugOption   string
	redact            bool
	logger            logrus.FieldLogger
}

// RoundTrip prints passing HTTP requests and received responses
//
// TODO: massively improve this, because the printed information can be wrong:
//   - https://github.com/k6io/k6/issues/986
//   - https://github.com/k6io/k6/issues/1042
//   - https://github.com/k6io/k6/issues/774
func (t httpDebugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	id := uuid.NewString()
	t.debugRequest(req, id)
	resp, err := t.originalTransport.RoundTrip(req)
	t.debugResponse(resp, id)
	return resp, err
}

func (t httpDebugTransport) debugRequest(req *http.Request, requestID string) {
	withBody := t.httpDebugOption == "full"
	if withBody && req.Body != nil && req.Body != http.NoBody && req.GetBody != nil {
		// httputil.DumpRequestOut drains req.Body and then replaces it with a new reader,
		// which would be left consumed after the request is sent. A wrapping RoundTripper
		// that resends the request (e.g. NTLM authentication) needs the body it installed
		// to remain the one that is set on the request, so dump a copy that gets its own
		// body from GetBody instead and leave the original request untouched.
		if body, err := req.GetBody(); err == nil {
			req = req.Clone(req.Context())
			req.Body = body
		}
	}
	dump, err := httputil.DumpRequestOut(req, withBody)
	if err != nil {
		t.logger.Error(err)
	}
	t.logger.WithField("request_id", requestID).Infof("Request:\n%s\n", t.format(dump))
}

func (t httpDebugTransport) debugResponse(res *http.Response, requestID string) {
	if res != nil {
		dump, err := httputil.DumpResponse(res, t.httpDebugOption == "full")
		if err != nil {
			t.logger.Error(err)
		}
		t.logger.WithField("request_id", requestID).Infof("Response:\n%s\n", t.format(dump))
	}
}

func (t httpDebugTransport) format(dump []byte) []byte {
	dump = bytes.ReplaceAll(dump, []byte("\r\n"), []byte{'\n'})
	if t.redact {
		dump = httpdebug.RedactDump(dump)
	}
	return dump
}
