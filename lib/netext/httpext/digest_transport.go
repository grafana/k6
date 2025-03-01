package httpext

import (
	"net/http"

	"github.com/icholy/digest"
)

type digestTransport struct {
	originalTransport http.RoundTripper
}

// RoundTrip handles digest auth by behaving like an http.RoundTripper
//
// GitHub PR: https://github.com/grafana/k6/pull/4599
func (t digestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	username := req.URL.User.Username()
	password, _ := req.URL.User.Password()

	// Remove the user data from the URL to avoid sending the authorization
	// header for basic auth
	req.URL.User = nil

	digestTr := &digest.Transport{
		Username:  username,
		Password:  password,
		Transport: t.originalTransport,
	}

	return digestTr.RoundTrip(req)
}
