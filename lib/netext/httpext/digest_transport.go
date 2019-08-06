/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package httpext

import (
	"io/ioutil"
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
// Github issue: https://github.com/loadimpact/k6/issues/800
func (t digestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Make an initial request authentication params to compute the
	// authorization header
	username := req.URL.User.Username()
	password, _ := req.URL.User.Password()

	// Removing user from URL to avoid sending the authorization header fo basic auth
	req.URL.User = nil

	noAuthResponse, err := t.originalTransport.RoundTrip(req)
	if err != nil || noAuthResponse.StatusCode != http.StatusUnauthorized {
		return noAuthResponse, err
	}

	respBody, err := ioutil.ReadAll(noAuthResponse.Body)
	if err != nil {
		return nil, err
	}
	_ = noAuthResponse.Body.Close()

	challenge := digest.GetChallengeFromHeader(&noAuthResponse.Header)
	challenge.ComputeResponse(req.Method, req.URL.RequestURI(), string(respBody), username, password)
	authorization := challenge.ToAuthorizationStr()

	req.Header.Set(digest.KEY_AUTHORIZATION, authorization)
	if req.GetBody != nil {
		req.Body, err = req.GetBody()
		if err != nil {
			return nil, err
		}
	}
	return t.originalTransport.RoundTrip(req)
}
