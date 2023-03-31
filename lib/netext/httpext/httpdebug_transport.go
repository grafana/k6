package httpext

import (
	"bytes"
	"net/http"
	"net/http/httputil"

	uuid "github.com/nu7hatch/gouuid"
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
//   - https://github.com/k6io/k6/issues/986
//   - https://github.com/k6io/k6/issues/1042
//   - https://github.com/k6io/k6/issues/774
func (t httpDebugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	id, _ := uuid.NewV4()
	t.debugRequest(req, id.String())
	resp, err := t.originalTransport.RoundTrip(req)
	t.debugResponse(resp, id.String())
	return resp, err
}

func (t httpDebugTransport) debugRequest(req *http.Request, requestID string) {
	dump, err := httputil.DumpRequestOut(req, t.httpDebugOption == "full")
	if err != nil {
		t.logger.Error(err)
	}
	t.logger.WithField("request_id", requestID).Infof("Request:\n%s\n",
		bytes.ReplaceAll(dump, []byte("\r\n"), []byte{'\n'}))
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
