package httpext

import (
	"bytes"
	"io"
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
	dump, err := dumpOutgoingRequest(req, t.httpDebugOption == "full")
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

// dumpOutgoingRequest is httputil.DumpRequestOut without mutating req.Body.
// NTLM authentication (Azure go-ntlmssp) wraps the body in a type it can
// rewind; DumpRequestOut may replace that wrapper via GetBody, so a later
// handshake retry sees ContentLength>0 with an already-consumed body (#6659).
func dumpOutgoingRequest(req *http.Request, dumpBody bool) ([]byte, error) {
	dumpReq := req.Clone(req.Context())
	if dumpBody && req.Body != nil {
		if req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, err
			}
			dumpReq.Body = body
		} else {
			buf, err := io.ReadAll(req.Body)
			_ = req.Body.Close()
			if err != nil {
				return nil, err
			}
			restored := io.NopCloser(bytes.NewReader(buf))
			req.Body = restored
			dumpReq.Body = io.NopCloser(bytes.NewReader(buf))
		}
	} else {
		dumpReq.Body = nil
		dumpReq.ContentLength = 0
	}
	return httputil.DumpRequestOut(dumpReq, dumpBody)
}
