package tests

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/xk6-browser/common"

	k6lib "go.k6.io/k6/lib"
	k6types "go.k6.io/k6/lib/types"
)

func TestURLSkipRequest(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withLogCache())
	p := tb.NewPage(nil)

	_, err := p.Goto("data:text/html,hello", nil)
	require.NoError(t, err)
	tb.logCache.assertContains(t, "skipping request handling of data URL")

	_, err = p.Goto("blob:something", nil)
	require.NoError(t, err)
	tb.logCache.assertContains(t, "skipping request handling of blob URL")
}

func TestBlockHostnames(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withHTTPServer(), withLogCache())

	blocked, err := k6types.NewNullHostnameTrie([]string{"*.test"})
	require.NoError(t, err)
	tb.vu.State().Options.BlockedHostnames = blocked

	p := tb.NewPage(nil)

	res, err := p.Goto("http://host.test/", nil)
	require.NoError(t, err)
	require.Nil(t, res)
	tb.logCache.assertContains(t, "was interrupted: hostname host.test is in a blocked pattern")

	res, err = p.Goto(tb.url("/get"), nil)
	require.NoError(t, err)
	assert.NotNil(t, res)
}

func TestBlockIPs(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withHTTPServer(), withLogCache())

	ipnet, err := k6lib.ParseCIDR("10.0.0.0/8")
	require.NoError(t, err)
	tb.vu.State().Options.BlacklistIPs = []*k6lib.IPNet{ipnet}

	p := tb.NewPage(nil)
	res, err := p.Goto("http://10.0.0.1:8000/", nil)
	require.NoError(t, err)
	require.Nil(t, res)
	tb.logCache.assertContains(t, `was interrupted: IP 10.0.0.1 is in a blacklisted range "10.0.0.0/8"`)

	// Ensure other requests go through
	res, err = p.Goto(tb.url("/get"), nil)
	require.NoError(t, err)
	assert.NotNil(t, res)
}

func TestBasicAuth(t *testing.T) {
	t.Parallel()

	const (
		validUser     = "validuser"
		validPassword = "validpass"
	)

	auth := func(tb testing.TB, user, pass string) *common.Response {
		tb.Helper()

		browser := newTestBrowser(t, withHTTPServer())
		bc, err := browser.NewContext(
			browser.toGojaValue(struct {
				HttpCredentials *common.Credentials `js:"httpCredentials"` //nolint:revive
			}{
				HttpCredentials: &common.Credentials{
					Username: user,
					Password: pass,
				},
			}))
		require.NoError(t, err)
		p, err := bc.NewPage()
		require.NoError(t, err)

		opts := browser.toGojaValue(struct {
			WaitUntil string `js:"waitUntil"`
		}{
			WaitUntil: "load",
		})
		url := browser.url(fmt.Sprintf("/basic-auth/%s/%s", validUser, validPassword))
		res, err := p.Goto(url, opts)
		require.NoError(t, err)

		return res
	}

	t.Run("valid", func(t *testing.T) {
		t.Parallel()

		resp := auth(t, validUser, validPassword)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, int(resp.Status()))
	})
	t.Run("invalid", func(t *testing.T) {
		t.Parallel()

		resp := auth(t, "invalidUser", "invalidPassword")
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusUnauthorized, int(resp.Status()))
	})
}
