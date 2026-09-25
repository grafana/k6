package httpext

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"

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
	base   http.RoundTripper
	logger logrus.FieldLogger

	// authority is the origin (scheme, host, and port) the credentials belong
	// to. Redirects to any other origin must not reuse them.
	authority string

	// noChallenge records whether the wrapped transport found no usable digest
	// challenge in the latest 401 response, and thus returned it with its body
	// drained and closed. It is only accessed from RoundTrip's goroutine and
	// each request gets a new digestTransport, so it needs no synchronization.
	noChallenge bool
}

// newDigestTransport returns an http.RoundTripper that will perform HTTP digest
// authentication with the given credentials over the given transport.
// Credentials are only applied to requests for origin; other origins are sent
// without digest authentication.
func newDigestTransport(
	inner http.RoundTripper, username, password string, origin *url.URL, logger logrus.FieldLogger,
) *digestTransport {
	t := &digestTransport{
		base:      inner,
		logger:    logger,
		authority: canonicalAuthority(origin),
	}
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
	// The username and password live on the digest transport for the whole
	// http.Client.Do call, including redirects. Go strips Authorization on
	// cross-host redirects, but digest auth is added afterwards inside this
	// transport, so an open redirect could otherwise deliver a Digest
	// Authorization (the username and a password-derived response) to another
	// origin. Only the origin the credentials were configured for is
	// authenticated.
	if req != nil && req.URL != nil && canonicalAuthority(req.URL) != t.authority {
		return t.base.RoundTrip(req)
	}
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

// canonicalAuthority is the request origin digest credentials are scoped to.
// Default ports are filled in so http://host and http://host:80 match.
func canonicalAuthority(u *url.URL) string {
	if u == nil {
		return ""
	}
	port := u.Port()
	if port == "" {
		switch strings.ToLower(u.Scheme) {
		case "https":
			port = "443"
		case "http":
			port = "80"
		}
	}
	return strings.ToLower(u.Scheme) + "://" + strings.ToLower(net.JoinHostPort(u.Hostname(), port))
}

// interface assertion to catch changes in the wrapped types early
var _ http.RoundTripper = &digestTransport{}
