package httpext

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// rewindOnlyBody can only be replayed if the same instance is kept on the
// request. Replacing it with a fresh GetBody() result (as DumpRequestOut does)
// leaves later retries unable to rewind, matching go-ntlmssp's negotiatorBody.
type rewindOnlyBody struct {
	buf []byte
	off int
}

func (b *rewindOnlyBody) Read(p []byte) (int, error) {
	if b.off >= len(b.buf) {
		return 0, io.EOF
	}
	n := copy(p, b.buf[b.off:])
	b.off += n
	return n, nil
}

func (b *rewindOnlyBody) Close() error { return nil }

func (b *rewindOnlyBody) rewind() {
	b.off = 0
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// replayOnce401 mimics the NTLM handshake: send the request, and on 401 rewind
// the original body wrapper and send it again.
type replayOnce401 struct {
	next http.RoundTripper
}

func (t replayOnce401) RoundTrip(req *http.Request) (*http.Response, error) {
	body, _ := req.Body.(*rewindOnlyBody)
	if body != nil {
		body.rewind()
	}
	resp, err := t.next.RoundTrip(req)
	if err != nil || resp.StatusCode != http.StatusUnauthorized {
		return resp, err
	}
	if body != nil {
		body.rewind()
		req.Body = body
	}
	return t.next.RoundTrip(req)
}

func emptyResp(status int) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(nil)),
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
	}
}

func TestHTTPDebugFullPreservesRewindableBodyAcrossAuthRetry(t *testing.T) {
	t.Parallel()

	payload := []byte("abcdef")
	var got []int
	inner := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		n, err := io.Copy(io.Discard, req.Body)
		require.NoError(t, err)
		got = append(got, int(n))
		if len(got) == 1 {
			return emptyResp(http.StatusUnauthorized), nil
		}
		return emptyResp(http.StatusOK), nil
	})

	logs := bytes.Buffer{}
	logger := logrus.New()
	logger.SetOutput(&logs)
	logger.SetLevel(logrus.InfoLevel)

	debug := httpDebugTransport{
		originalTransport: inner,
		httpDebugOption:   "full",
		logger:            logger,
	}

	body := &rewindOnlyBody{buf: payload}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "http://example.com/ntlm", body)
	require.NoError(t, err)
	req.ContentLength = int64(len(payload))
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(payload)), nil
	}

	resp, err := replayOnce401{next: debug}.RoundTrip(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, []int{len(payload), len(payload)}, got)
	require.Contains(t, logs.String(), "abcdef")
}
