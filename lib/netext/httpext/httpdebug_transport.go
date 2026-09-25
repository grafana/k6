package httpext

import (
	"bytes"
	"net/http"
	"net/http/httputil"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type httpDebugTransport struct {
	originalTransport http.RoundTripper
	httpDebugOption   string
	logger            logrus.FieldLogger
}

// RoundTrip prints passing HTTP requests and received responses
//
// TODO: massively improve this, because the printed information can be wrong:
//   - https://github.com/k6io/k6/issues/1042
//   - https://github.com/k6io/k6/issues/774
func (t httpDebugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	id := uuid.NewString()
	dump, dumpErr := httputil.DumpRequestOut(req, t.httpDebugOption == "full")
	resp, err := t.originalTransport.RoundTrip(req)
	if dumpErr != nil {
		t.logger.Error(dumpErr)
	} else {
		proto := ""
		if resp != nil {
			proto = resp.Proto
		}
		t.logger.WithField("request_id", id).Infof("Request:\n%s\n",
			bytes.ReplaceAll(replaceRequestDumpProto(dump, proto), []byte("\r\n"), []byte{'\n'}))
	}
	t.debugResponse(resp, id)
	return resp, err
}

// replaceRequestDumpProto rewrites the request-line protocol after the real
// connection is known. DumpRequestOut always writes HTTP/1.1, including for HTTP/2.
func replaceRequestDumpProto(dump []byte, proto string) []byte {
	if proto == "" {
		return dump
	}
	eol := bytes.Index(dump, []byte("\r\n"))
	if eol < 0 {
		return dump
	}
	line := dump[:eol]
	sp := bytes.LastIndexByte(line, ' ')
	if sp < 0 || !bytes.HasPrefix(line[sp+1:], []byte("HTTP/")) {
		return dump
	}
	if bytes.Equal(line[sp+1:], []byte(proto)) {
		return dump
	}
	out := make([]byte, 0, len(dump)-len(line[sp+1:])+len(proto))
	out = append(out, line[:sp+1]...)
	out = append(out, proto...)
	out = append(out, dump[eol:]...)
	return out
}

func (t httpDebugTransport) debugResponse(res *http.Response, requestID string) {
	if res != nil {
		dump, err := httputil.DumpResponse(res, t.httpDebugOption == "full")
		if err != nil {
			t.logger.Error(err)
		}
		t.logger.WithField("request_id", requestID).Infof("Response:\n%s\n",
			bytes.ReplaceAll(dump, []byte("\r\n"), []byte{'\n'}))
	}
}
