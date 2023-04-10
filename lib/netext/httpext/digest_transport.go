package httpext

import (
	"io"
	"net/http"

	digest "github.com/Soontao/goHttpDigestClient"
)

type digestTransport struct {
	originalTransport http.RoundTripper
}

// RoundTrip handles digest auth by behaving like an http.RoundTripper
//
// TODO: fix - this is a preliminary solution and is somewhat broken! we're
// always making 2 HTTP requests when digest authentication is enabled... we
// should cache the nonces and behave more like a browser... or we should
// ditch the hacky http.RoundTripper approach and write our own client...
//
// Github issue: https://github.com/k6io/k6/issues/800
func (t digestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Make the initial request authentication params to compute the
	// authorization header
	username := req.URL.User.Username()
	password, _ := req.URL.User.Password()

	// Remove the user data from the URL to avoid sending the authorization
	// header for basic auth
	req.URL.User = nil

	noAuthResponse, err := t.originalTransport.RoundTrip(req)
	if err != nil || noAuthResponse.StatusCode != http.StatusUnauthorized {
		// If there was an error, or if the remote server didn't respond with
		// status 401, we simply return, so the upstream code can deal with it.
		return noAuthResponse, err
	}

	respBody, err := io.ReadAll(noAuthResponse.Body)
	if err != nil {
		return nil, err
	}
	_ = noAuthResponse.Body.Close()

	// Calculate the Authorization header
	// TODO: determine if we actually need the body, since I'm not sure that's
	// what the `entity` means... maybe a moot point if we change the used
	// digest auth library...
	challenge := digest.GetChallengeFromHeader(&noAuthResponse.Header)
	challenge.ComputeResponse(req.Method, req.URL.RequestURI(), string(respBody), username, password)
	authorization := challenge.ToAuthorizationStr()
	req.Header.Set(digest.KEY_AUTHORIZATION, authorization)

	if req.GetBody != nil {
		// Reset the request body if we need to
		req.Body, err = req.GetBody()
		if err != nil {
			return nil, err
		}
	}

	// Actually make the HTTP request with the proper Authorization
	return t.originalTransport.RoundTrip(req)
}
