package httpext

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httputil"
	"slices"
	"strings"

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
	dump, err := httputil.DumpRequestOut(req, t.httpDebugOption == "full")
	if err != nil {
		t.logger.Error(err)
	}
	t.logger.WithField("request_id", requestID).Infof("Request:\n%s\n",
		bytes.ReplaceAll(dump, []byte("\r\n"), []byte{'\n'}))
}

func (t httpDebugTransport) debugResponse(res *http.Response, requestID string) {
	if res == nil {
		return
	}
	includeBody := t.httpDebugOption == "full"
	decoded, decodedOK := decodeHTTPDebugBody(res, includeBody)
	dump, err := httputil.DumpResponse(res, includeBody && !decodedOK)
	if err != nil {
		t.logger.Error(err)
		return
	}
	if decodedOK {
		dump = append(dump, decoded...)
	}
	t.logger.WithField("request_id", requestID).Infof("Response:\n%s\n",
		bytes.ReplaceAll(dump, []byte("\r\n"), []byte{'\n'}))
}

// decodeHTTPDebugBody decompresses the response for --http-debug=full dumps.
// k6 sets DisableCompression so httputil.DumpResponse would otherwise print the
// still-compressed bytes while later response handling already decodes them.
func decodeHTTPDebugBody(res *http.Response, includeBody bool) ([]byte, bool) {
	if !includeBody || res.Body == nil || res.Body == http.NoBody {
		return nil, false
	}
	enc := res.Header.Get("Content-Encoding")
	if enc == "" {
		return nil, false
	}

	raw, err := io.ReadAll(res.Body)
	_ = res.Body.Close()
	res.Body = io.NopCloser(bytes.NewReader(raw))
	if err != nil || len(raw) == 0 {
		return nil, false
	}

	decoded := raw
	rc := &readCloser{bytes.NewReader(decoded)}
	hadDecoder := false
	for _, contentEncoding := range slices.Backward(strings.Split(enc, ",")) {
		contentEncoding := strings.TrimSpace(contentEncoding)
		compression, cerr := CompressionTypeString(contentEncoding)
		if cerr != nil {
			continue
		}
		decoder, derr := pickDecoder(compression, rc)
		if derr != nil {
			return nil, false
		}
		out, rerr := io.ReadAll(decoder)
		if c, ok := decoder.(io.Closer); ok {
			_ = c.Close()
		}
		if rerr != nil {
			return nil, false
		}
		decoded = out
		rc = &readCloser{bytes.NewReader(decoded)}
		hadDecoder = true
	}
	if !hadDecoder {
		return nil, false
	}
	return decoded, true
}
