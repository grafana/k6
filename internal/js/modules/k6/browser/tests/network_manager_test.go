package tests

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/js/modules/k6/browser/common"

	k6lib "go.k6.io/k6/lib"
	k6types "go.k6.io/k6/lib/types"
)

func TestURLSkipRequest(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withLogCache())
	p := tb.NewPage(nil)

	opts := &common.FrameGotoOptions{
		Timeout: common.DefaultTimeout,
	}
	_, err := p.Goto(
		"data:text/html,hello",
		opts,
	)
	require.NoError(t, err)
	tb.logCache.assertContains(t, "skipping request handling of data URL")

	_, err = p.Goto(
		"blob:something",
		opts,
	)
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

	opts := &common.FrameGotoOptions{
		Timeout: common.DefaultTimeout,
	}
	res, err := p.Goto(
		"http://host.test/",
		opts,
	)
	require.NoError(t, err)
	require.Nil(t, res)
	tb.logCache.assertContains(t, "was interrupted: hostname host.test is in a blocked pattern")

	res, err = p.Goto(
		tb.url("/get"),
		opts,
	)
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
	opts := &common.FrameGotoOptions{
		Timeout: common.DefaultTimeout,
	}
	res, err := p.Goto(
		"http://10.0.0.1:8000/",
		opts,
	)
	require.NoError(t, err)
	require.Nil(t, res)
	tb.logCache.assertContains(t, `was interrupted: IP 10.0.0.1 is in a blacklisted range "10.0.0.0/8"`)

	// Ensure other requests go through
	res, err = p.Goto(
		tb.url("/get"),
		opts,
	)
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

		bcopts := common.DefaultBrowserContextOptions()
		bcopts.HTTPCredentials = common.Credentials{
			Username: validUser,
			Password: validPassword,
		}
		bc, err := browser.NewContext(bcopts)
		require.NoError(t, err)

		p, err := bc.NewPage()
		require.NoError(t, err)

		url := browser.url(
			fmt.Sprintf("/basic-auth/%s/%s", user, pass),
		)
		opts := &common.FrameGotoOptions{
			WaitUntil: common.LifecycleEventLoad,
			Timeout:   common.DefaultTimeout,
		}
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

// See issue #1072 for more details.
func TestInterceptBeforePageLoad(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withHTTPServer())

	// changing the main frame to another URL will trigger a redirect
	// before the page is loaded. this will cause a deadlock because
	// the page's response body won't be available yet due to request
	// interception.
	//
	// the reason for this is that the browser will intercept the
	// request and pause it, but the response body won't be available
	// until the page is loaded, which won't happen because the
	// request is paused.
	tb.withHandler("/neverFinishesLoading", func(w http.ResponseWriter, r *http.Request) {
		const runBeforePageOnLoad = `
			// immediately redirect to another page before the page is
			// loaded. browsers wait for scripts to finish executing
			// before firing the load event, so this will cause the
			// page to never finish loading.
			window.location.href='/trap';
		`
		_, err := fmt.Fprintf(w, `
			<html>
				<head>
					<script>
						%s
					</script>
				</head>
				<body />
			</html>
		`, runBeforePageOnLoad)
		require.NoError(t, err)
	})

	// this handler will be called before the main page is loaded
	tb.withHandler("/trap", func(w http.ResponseWriter, r *http.Request) {
		_, err := fmt.Fprint(w, "ok")
		require.NoError(t, err)
	})

	// go to the main page and wait for the redirect to happen
	// before the page is loaded (LifecycleEventDOMContentLoad).
	gotoPage := func() error {
		p := tb.NewPage(nil)

		opts := &common.FrameGotoOptions{
			WaitUntil: common.LifecycleEventDOMContentLoad,
			Timeout:   common.DefaultTimeout,
		}
		_, err := p.Goto(
			tb.url("/neverFinishesLoading"),
			opts,
		)

		return err
	}

	// enable interception to pause the redirect in the main page
	blocked, err := k6types.NewNullHostnameTrie([]string{"foo.com"})
	require.NoError(t, err)
	tb.vu.State().Options.BlockedHostnames = blocked

	// go to the main page and cut short with a timeout
	// if it takes too long. in a buggy case, this will
	// deadlock and never return.
	//
	// a five seconds timeout is plenty for this bug to
	// manifest.
	ctx, cancel := context.WithTimeout(tb.ctx, 5*time.Second)
	defer cancel()
	err = tb.run(ctx, gotoPage)
	require.NoError(t, err)
}
