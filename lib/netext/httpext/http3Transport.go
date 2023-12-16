package httpext

import (
	"context"
	"errors"
	"net"
	"net/http"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
)

// transport is an implementation of http.RoundTripper that will measure and emit
// different metrics for each roundtrip
type http3Transport struct {
	ctx              context.Context
	state            *lib.State
	tagsAndMeta      *metrics.TagsAndMeta
	responseCallback func(int) bool
	roundTripper     http.RoundTripper
}

// newHTTP3Transport returns a new http.RoundTripper implementation that wraps around
// the provided RoundTripper.
func newHTTP3Transport(
	ctx context.Context,
	state *lib.State,
	tagsAndMeta *metrics.TagsAndMeta,
	responseCallback func(int) bool,
	http3Roundtripper http.RoundTripper,
) *http3Transport {
	return &http3Transport{
		ctx:              ctx,
		state:            state,
		tagsAndMeta:      tagsAndMeta,
		responseCallback: responseCallback,
		roundTripper:     http3Roundtripper,
	}
}

// RoundTrip is the implementation of http.RoundTripper
func (t *http3Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.roundTripper.RoundTrip(req)

	var netError net.Error
	if errors.As(err, &netError) && netError.Timeout() {
		var netOpError *net.OpError
		if errors.As(err, &netOpError) && netOpError.Op == "dial" {
			err = NewK6Error(tcpDialTimeoutErrorCode, tcpDialTimeoutErrorCodeMsg, netError)
		} else {
			err = NewK6Error(requestTimeoutErrorCode, requestTimeoutErrorCodeMsg, netError)
		}
	}

	return resp, err
}
