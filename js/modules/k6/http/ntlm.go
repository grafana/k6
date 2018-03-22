/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2018 Load Impact
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

package http

import (
	"bytes"
	"encoding/base64"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/ThomsonReutersEikon/go-ntlm/ntlm"
	"github.com/pkg/errors"
)

// NTLMNegotiator is a http.Roundtripper that automatically converts basic authentication to
// NTLM authentication.
type NTLMNegotiator struct{ http.RoundTripper }

func (n NTLMNegotiator) RoundTrip(req *http.Request) (res *http.Response, err error) {
	// Use default round tripper if not provided
	rt := n.RoundTripper
	if rt == nil {
		rt = http.DefaultTransport
	}

	username, password, err := getCredentialsFromHeader(req.Header.Get("Authorization"))
	if err != nil {
		return nil, errors.Wrap(err, "get basic credentials from header failed")
	}

	// Save request body
	body := bytes.Buffer{}
	if req.Body != nil {
		_, err = body.ReadFrom(req.Body)
		if err != nil {
			return nil, err
		}

		if err := req.Body.Close(); err != nil {
			return nil, err
		}
		req.Body = ioutil.NopCloser(bytes.NewReader(body.Bytes()))
	}

	// Sending first request to get challenge data
	req.Header.Set("Authorization", "NTLM TlRMTVNTUAABAAAABoIIAAAAAAAAAAAAAAAAAAAAAAA=")
	res, err = rt.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusUnauthorized {
		return res, err
	}

	if _, err := io.Copy(ioutil.Discard, res.Body); err != nil {
		return nil, err
	}
	if err := res.Body.Close(); err != nil {
		return nil, err
	}
	req.Body = ioutil.NopCloser(bytes.NewReader(body.Bytes()))

	// retrieve Www-Authenticate header from response
	ntlmChallenge := res.Header.Get("WWW-Authenticate")
	if ntlmChallenge == "" {
		return nil, errors.New("Empty WWW-Authenticate header")
	}

	challengeBytes, err := base64.StdEncoding.DecodeString(strings.Replace(ntlmChallenge, "NTLM ", "", -1))
	if err != nil {
		return nil, err
	}

	session, err := ntlm.CreateClientSession(ntlm.Version2, ntlm.ConnectionlessMode)
	if err != nil {
		return nil, err
	}

	session.SetUserInfo(username, password, "")

	// parse NTLM challenge
	challenge, err := ntlm.ParseChallengeMessage(challengeBytes)
	if err != nil {
		return nil, err
	}

	err = session.ProcessChallengeMessage(challenge)
	if err != nil {
		return nil, err
	}

	// authenticate user
	authenticate, err := session.GenerateAuthenticateMessage()
	if err != nil {
		return nil, err
	}

	// set NTLM Authorization header
	req.Header.Set("Authorization", "NTLM "+base64.StdEncoding.EncodeToString(authenticate.Bytes()))

	return rt.RoundTrip(req)
}

func getCredentialsFromHeader(header string) (string, string, error) {
	credBytes, err := base64.StdEncoding.DecodeString(strings.Replace(header, "Basic ", "", -1))
	if err != nil {
		return "", "", err
	}

	parts := strings.SplitN(string(credBytes), ":", 2)
	return parts[0], parts[1], nil
}
