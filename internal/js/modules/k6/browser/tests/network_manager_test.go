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
	k6metrics "go.k6.io/k6/metrics"

	k6lib "go.k6.io/k6/lib"
	k6types "go.k6.io/k6/lib/types"
)

// TestURLSkipRequest checks that, since https://github.com/grafana/k6/commit/f29064ef, k6 doesn't
// handle navigation requests for local urls, like: blob:... or data:..., and so it doesn't emit errors
// neither (e.g. in case of a non-existing blob), in contraposition to what Playwright does.
func TestURLSkipRequest(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withLogCache())
	p := tb.NewPage(nil)

	_, err := p.Goto(
		"data:text/html,hello",
		&common.FrameGotoOptions{Timeout: common.DefaultTimeout},
	)
	require.NoError(t, err)
	tb.logCache.assertContains(t, "skipping request handling of data URL")

	// In this test, we're checking that the network manager is skipping request handling of certain URLs,
	// but Browser navigation still happens.
	//
	// However, until Chrome 139.x, there was a subtle bug causing no valid NavigationEvent to be emitted.
	// An event being considered valid by having a document id that matches the document id of the frame
	// that navigated to that url. So, until that, we expected the following [p.Goto] call to timeout.
	//
	// Since Chrome 140.x, this bug has been fixed, and now the navigation to a non-existing blob behaves
	// similarly to what happens in the lines above, no timing out but succeeding with normality.
	//
	// Note that, in spite of non-existing blob (we cannot define a valid one because k6 lacks general support
	// for Blob and URL WebAPIs), this doesn't return any error because as stated in the TestURLSkipRequest docs,
	// k6 intentionally skips that request handling, thus not throwing the net::ERR_FILE_NOT_FOUND that Chrome
	// throws in such case, as Playwright does.
	_, err = p.Goto(
		"blob:something",
		&common.FrameGotoOptions{Timeout: common.DefaultTimeout},
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
	require.Nil(t, res)
	require.Error(t, err)
	require.ErrorContains(t, err, `navigating frame to "http://host.test/": net::ERR_BLOCKED_BY_CLIENT`)
	tb.logCache.assertContains(t, "was aborted: hostname host.test matches a blocked pattern")

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
	require.Nil(t, res)
	require.Error(t, err)
	require.ErrorContains(t, err, `navigating frame to "http://10.0.0.1:8000/": net::ERR_BLOCKED_BY_CLIENT`)
	tb.logCache.assertContains(t, `was aborted: IP 10.0.0.1 is in a blacklisted range "10.0.0.0/8"`)

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

// TestNetworkManagerCloseMetricEmissionRace reproduces the race
// condition where NetworkManager emits metrics during the engine
// shutdown. Reproduces the issue 4203 when run with the -race flag.
func TestNetworkManagerCloseMetricEmissionRace(t *testing.T) {
	t.Parallel()

	samples := make(chan k6metrics.SampleContainer, 5)
	tb := newTestBrowser(t, withHTTPServer(), withSamples(samples), withSkipClose())
	tb.vu.StartIteration(t)
	page := tb.NewPage(nil)

	tb.withHandler("/foo", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `<html><head></head><body>loaded</body></html>`)
	})
	tb.withHandler("/ping", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "pong")
	})

	opts := &common.FrameGotoOptions{Timeout: common.DefaultTimeout}
	_, err := page.Goto(tb.url("/foo"), opts)
	require.NoError(t, err)

	// Fire a burst of fetch requests from JS so that many HTTP-metric
	// emission goroutines are active or queued when we close the page.
	_, err = page.Evaluate(`() => {
		const promises = [];
		for (let i = 0; i < 5; i++) {
			promises.push(fetch('/ping?i=' + i));
		}
		return Promise.all(promises).then(() => 'done');
	}`)
	require.NoError(t, err)

	// Close the page in a goroutine. On a correct implementation,
	// Close blocks until all metric-emitting goroutines finish.
	closeDone := make(chan error, 1)
	go func() { closeDone <- page.Close() }()

	defer close(samples)
	go func() {
		for range samples {
		}
	}()

	// Wait for Close to return.
	select {
	case err := <-closeDone:
		require.NoError(t, err)
	case <-time.After(30 * time.Second):
		t.Fatal("page.Close() did not return after draining samples")
	}
}
