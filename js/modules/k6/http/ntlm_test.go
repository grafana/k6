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
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ThomsonReutersEikon/go-ntlm/ntlm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var challenges map[string]*ntlm.ChallengeMessage

func TestGetCredentialsFromHeader(t *testing.T) {
	user, pass, err := getCredentialsFromHeader("Basic Ym9iOnBhc3M=")
	require.NoError(t, err)

	assert.Equal(t, "bob", user)
	assert.Equal(t, "pass", pass)
}

func TestNTLMServer(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(ntlmHandler("bob", "pass")))
	defer s.Close()

	client := &http.Client{
		Transport: NTLMNegotiator{
			RoundTripper: &http.Transport{},
		},
	}

	url := strings.Replace(s.URL, "http://", "http://bob:pass@", -1)

	req, _ := http.NewRequest("GET", url, nil)
	res, err := client.Do(req)
	require.NoError(t, err)

	body, _ := ioutil.ReadAll(res.Body)
	assert.Equal(t, "authenticated", string(body))
}
