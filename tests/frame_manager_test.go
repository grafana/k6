package tests

import (
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/xk6-browser/api"
	"github.com/grafana/xk6-browser/common"
)

func TestWaitForFrameNavigationWithinDocument(t *testing.T) {
	if os.Getenv("SKIP_FLAKY") == "true" {
		t.SkipNow()
	}
	t.Parallel()

	timeout := 5 * time.Second

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

			var done bool
			tb := newTestBrowser(t, withFileServer())
			err := tb.awaitWithTimeout(timeout,
				func() error {
					p := tb.NewPage(nil)

					gotoPromise := p.Goto(tb.staticURL("/nav_in_doc.html"), tb.toGojaValue(&common.FrameGotoOptions{
						WaitUntil: common.LifecycleEventNetworkIdle,
						Timeout:   time.Duration(timeout.Milliseconds()), // interpreted as ms
					}))

					tb.promiseThen(gotoPromise, func(resp api.Response) *goja.Promise {
						require.NotNil(t, resp)
						wfnPromise := p.WaitForNavigation(tb.toGojaValue(&common.FrameWaitForNavigationOptions{
							Timeout: time.Duration(timeout.Milliseconds()), // interpreted as ms
						}))
						cPromise := p.Click(tc.selector, nil)
						return tb.promiseAll(wfnPromise, cPromise)
					}).then(func() {
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

	var done bool
	require.NoError(t, tb.await(func() error {
		tb.promiseThen(p.Goto(tb.URL("/first"), tb.toGojaValue(&common.FrameGotoOptions{
			WaitUntil: common.LifecycleEventNetworkIdle,
			Timeout:   common.DefaultTimeout,
		})), func() *goja.Promise {
			var timeout time.Duration = 5000 // interpreted as ms
			wfnPromise := p.WaitForNavigation(tb.toGojaValue(&common.FrameWaitForNavigationOptions{
				Timeout: timeout, // interpreted as ms
			}))
			cPromise := p.Click(`a`, nil)
			return tb.promiseAll(wfnPromise, cPromise)
		}).then(func() {
			assert.Equal(t, "Second page", p.Title())
			done = true
		})

		return nil
	}))
	require.True(t, done)
}
