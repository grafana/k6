package httpext

import (
	"encoding/base64"
	"encoding/binary"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Azure/go-ntlmssp"
	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ntlmChallenge returns a minimal, valid NTLM type 2 (challenge) message.
func ntlmChallenge() string {
	msg := make([]byte, 48)
	copy(msg, "NTLMSSP\x00")
	binary.LittleEndian.PutUint32(msg[8:], 2)
	binary.LittleEndian.PutUint32(msg[20:], 1) // NTLMSSP_NEGOTIATE_UNICODE
	return base64.StdEncoding.EncodeToString(msg)
}

// TestHTTPDebugTransportKeepsRequestBodyForNTLM checks that logging a request in full
// does not stop the NTLM negotiator from replaying the request body during the handshake.
func TestHTTPDebugTransportKeepsRequestBodyForNTLM(t *testing.T) {
	t.Parallel()

	const payload = "abcdef"
	var bodies []string

	// A fake server that goes through the three steps of an NTLM handshake and
	// records the body it receives on each of them.
	server := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		// Like a real transport, close the body once it has been sent.
		_ = r.Body.Close()
		bodies = append(bodies, string(body))

		resp := &http.Response{
			StatusCode: http.StatusUnauthorized,
			Header:     http.Header{},
			Body:       http.NoBody,
			Request:    r,
		}
		switch len(bodies) {
		case 1:
			resp.Header.Set("WWW-Authenticate", "NTLM")
		case 2:
			resp.Header.Set("WWW-Authenticate", "NTLM "+ntlmChallenge())
		default:
			resp.StatusCode = http.StatusOK
		}
		return resp, nil
	})

	logger, _ := logtest.NewNullLogger()
	logger.SetLevel(logrus.DebugLevel)
	transport := ntlmssp.Negotiator{RoundTripper: httpDebugTransport{
		originalTransport: server,
		httpDebugOption:   "full",
		logger:            logger,
	}}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "http://example.com/", strings.NewReader(payload))
	require.NoError(t, err)
	req.SetBasicAuth("user", "pass")

	resp, err := transport.RoundTrip(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, []string{payload, payload, payload}, bodies,
		"the body must be sent on every step of the NTLM handshake")
}

func TestHTTPDebugTransportDumpsBodyWithoutConsumingIt(t *testing.T) {
	t.Parallel()

	logger, hook := logtest.NewNullLogger()
	var seen string
	inner := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		b, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		seen = string(b)
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Header: http.Header{}, Request: r}, nil
	})
	transport := httpDebugTransport{originalTransport: inner, httpDebugOption: "full", logger: logger}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "http://example.com/", strings.NewReader("payload"))
	require.NoError(t, err)

	resp, err := transport.RoundTrip(req)
	require.NoError(t, err)
	_ = resp.Body.Close()

	assert.Equal(t, "payload", seen)
	var logged []string
	for _, e := range hook.AllEntries() {
		logged = append(logged, e.Message)
	}
	assert.Contains(t, strings.Join(logged, "\n"), "payload")
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
