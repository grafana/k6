package tests

import (
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/xk6-browser/common"
	"github.com/grafana/xk6-browser/k6ext"
)

func TestWaitForFrameNavigationWithinDocument(t *testing.T) {
	t.Parallel()

	if os.Getenv("SKIP_FLAKY") == "true" {
		t.SkipNow()
	}

	const timeout = 5 * time.Second

	testCases := []struct {
		name, selector string
	}{
		{name: "history", selector: "a#nav-history"},
		{name: "anchor", selector: "a#nav-anchor"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t, withFileServer())
			p := tb.NewPage(nil)

			opts := tb.toGojaValue(&common.FrameGotoOptions{
				WaitUntil: common.LifecycleEventNetworkIdle,
				Timeout:   time.Duration(timeout.Milliseconds()), // interpreted as ms
			})
			resp, err := p.Goto(tb.staticURL("/nav_in_doc.html"), opts)
			require.NoError(t, err)
			require.NotNil(t, resp)

			var done bool
			err = tb.awaitWithTimeout(timeout, func() error {
				// TODO
				// remove this once we have finished our work on the mapping layer.
				// for now: provide a fake promise
				fakePromise := k6ext.Promise(tb.vu.Context(), func() (result any, reason error) {
					return nil, nil
				})
				tb.promise(fakePromise).
					then(func() testPromise {
						waitForNav := p.WaitForNavigation(tb.toGojaValue(&common.FrameWaitForNavigationOptions{
							Timeout: time.Duration(timeout.Milliseconds()), // interpreted as ms
						}))
						click := p.Click(tc.selector, nil)
						return tb.promiseAll(waitForNav, click)
					}).
					then(func() {
						done = true
					})

				return nil
			})
			require.NoError(t, err)
			require.True(t, done)
		})
	}
}

func TestWaitForFrameNavigation(t *testing.T) {
	tb := newTestBrowser(t, withHTTPServer())
	p := tb.NewPage(nil)

	tb.withHandler("/first", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `
		<html>
			<head>
				<title>First page</title>
			</head>
			<body>
				<a href="/second">click me</a>
			</body>
		</html>
		`)
	})
	tb.withHandler("/second", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `
		<html>
			<head>
				<title>Second page</title>
			</head>
			<body>
				<a href="/first">click me</a>
			</body>
		</html>
		`)
	})

	opts := tb.toGojaValue(&common.FrameGotoOptions{
		WaitUntil: common.LifecycleEventNetworkIdle,
		Timeout:   common.DefaultTimeout,
	})
	_, err := p.Goto(tb.URL("/first"), opts)
	require.NoError(t, err)

	var done bool
	err = tb.await(func() error {
		// TODO
		// remove this once we have finished our work on the mapping layer.
		// for now: provide a fake promise
		fakePromise := k6ext.Promise(tb.vu.Context(), func() (result any, reason error) {
			return nil, nil
		})
		tb.promise(fakePromise).
			then(func() testPromise {
				var timeout time.Duration = 5000 // interpreted as ms
				wfnPromise := p.WaitForNavigation(tb.toGojaValue(&common.FrameWaitForNavigationOptions{
					Timeout: timeout, // interpreted as ms
				}))
				cPromise := p.Click(`a`, nil)
				return tb.promiseAll(wfnPromise, cPromise)
			}).
			then(func() {
				assert.Equal(t, "Second page", p.Title())
				done = true
			})

		return nil
	})
	require.NoError(t, err)
	require.True(t, done)
}
