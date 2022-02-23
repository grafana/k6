/*
 *
 * xk6-browser - a browser automation extension for k6
 * Copyright (C) 2021 Load Impact
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

package tests

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	k6lib "go.k6.io/k6/lib"
	k6types "go.k6.io/k6/lib/types"

	"github.com/grafana/xk6-browser/api"
	"github.com/grafana/xk6-browser/common"
)

func TestURLSkipRequest(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withLogCache())
	p := tb.NewPage(nil)

	p.Goto("data:text/html,hello", nil)
	assert.True(t, tb.logCache.contains("skipped request handling of data URL"))

	p.Goto("blob:something", nil)
	assert.True(t, tb.logCache.contains("skipped request handling of blob URL"))
}

func TestBlockHostnames(t *testing.T) {
	tb := newTestBrowser(t, withHTTPServer(), withLogCache())

	blocked, err := k6types.NewNullHostnameTrie([]string{"*.test"})
	require.NoError(t, err)
	tb.state.Options.BlockedHostnames = blocked

	p := tb.NewPage(nil)
	res := p.Goto("http://host.test/", nil)
	assert.Nil(t, res)

	assert.True(t, tb.logCache.contains("was interrupted: hostname host.test is in a blocked pattern"))

	// Ensure other requests go through
	resp := p.Goto(tb.URL("/get"), nil)
	assert.NotNil(t, resp)
}

func TestBlockIPs(t *testing.T) {
	tb := newTestBrowser(t, withHTTPServer(), withLogCache())

	ipnet, err := k6lib.ParseCIDR("10.0.0.0/8")
	require.NoError(t, err)
	tb.state.Options.BlacklistIPs = []*k6lib.IPNet{ipnet}

	p := tb.NewPage(nil)
	res := p.Goto("http://10.0.0.1:8000/", nil)
	assert.Nil(t, res)

	assert.True(t, tb.logCache.contains(
		`was interrupted: IP 10.0.0.1 is in a blacklisted range "10.0.0.0/8"`))

	// Ensure other requests go through
	resp := p.Goto(tb.URL("/get"), nil)
	assert.NotNil(t, resp)
}

func TestBasicAuth(t *testing.T) {
	const (
		validUser     = "validuser"
		validPassword = "validpass"
	)

	browser := newTestBrowser(t, withHTTPServer())

	auth := func(tb testing.TB, user, pass string) api.Response {
		tb.Helper()

		return browser.NewContext(
			browser.rt.ToValue(struct {
				HttpCredentials *common.Credentials `js:"httpCredentials"` //nolint:revive
			}{
				HttpCredentials: &common.Credentials{
					Username: user,
					Password: pass,
				},
			})).
			NewPage().
			Goto(
				browser.URL(fmt.Sprintf("/basic-auth/%s/%s", validUser, validPassword)),
				browser.rt.ToValue(struct {
					WaitUntil string `js:"waitUntil"`
				}{
					WaitUntil: "load",
				}),
			)
	}

	t.Run("valid", func(t *testing.T) {
		resp := auth(t, validUser, validPassword)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, int(resp.Status()))
	})
	t.Run("invalid", func(t *testing.T) {
		resp := auth(t, "invalidUser", "invalidPassword")
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusUnauthorized, int(resp.Status()))
	})
}
