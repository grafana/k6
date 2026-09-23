package httpext

import (
	"errors"
	"net/http"

	"github.com/icholy/digest"
	"github.com/sirupsen/logrus"
)

// digestTransport adapts github.com/icholy/digest.Transport to k6's behavior
// expectations: a 401 response that carries no usable digest challenge must be
// surfaced to the caller as a regular response.
//
// digest.Transport always drains and closes the body of a 401 response while
// looking for its WWW-Authenticate challenge. When the server offers no digest
// authentication at all (e.g. it only supports Negotiate or NTLM, as IIS
// commonly does), it returns that response with the body already closed, and
// any attempt to read it afterwards fails with
// "http: read on closed response body".
type digestTransport struct {
	inner  *digest.Transport
	logger logrus.FieldLogger

	// noChallenge records whether the wrapped transport found no usable digest
	// challenge in the latest 401 response, and thus returned it with its body
	// drained and closed. It is only accessed from RoundTrip's goroutine and
	// each request gets a new digestTransport, so it needs no synchronization.
	noChallenge bool
}

// newDigestTransport returns an http.RoundTripper that will perform HTTP digest
// authentication with the given credentials over the given transport.
func newDigestTransport(
	inner http.RoundTripper, username, password string, logger logrus.FieldLogger,
) *digestTransport {
	t := &digestTransport{logger: logger}
	t.inner = &digest.Transport{
		Username:  username,
		Password:  password,
		Transport: inner,
		FindChallenge: func(h http.Header) (*digest.Challenge, error) {
			chal, err := digest.FindChallenge(h)
			if err != nil {
				if !errors.Is(err, digest.ErrNoChallenge) && t.logger != nil {
					t.logger.Warnf("Malformed digest authentication challenge: %v", err)
				}
				// Report the challenge as absent so that digest.Transport
				// returns the original 401 response instead of a transport
				// error. This also covers parsed challenges with unsupported
				// digest algorithms, which digest.FindChallenge skips.
				t.noChallenge = true
				return nil, digest.ErrNoChallenge
			}
			t.noChallenge = false
			return chal, nil
		},
	}
	return t
}

// RoundTrip implements http.RoundTripper and performs the digest authentication
// handshake on top of the wrapped transport.
func (t *digestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	res, err := t.inner.RoundTrip(req)
	if err == nil && t.noChallenge && res != nil && res.StatusCode == http.StatusUnauthorized {
		// The body of the challenge-less 401 response was drained and closed by
		// digest.Transport, so replace it with an empty readable one. This way
		// the caller sees the actual 401 response and its headers (including
		// the WWW-Authenticate methods the server does support) instead of a
		// confusing body read error.
		res.Body = http.NoBody
	}
	return res, err
}

// interface assertion to catch changes in the wrapped types early
var _ http.RoundTripper = &digestTransport{}
