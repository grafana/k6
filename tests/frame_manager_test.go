package tests

import (
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/grafana/xk6-browser/common"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

			tb := newTestBrowser(t, withFileServer())
			err := tb.awaitWithTimeout(timeout,
				func() error {
					p := tb.NewPage(nil)

					resp := p.Goto(tb.staticURL("/nav_in_doc.html"), tb.toGojaValue(&common.FrameGotoOptions{
						WaitUntil: common.LifecycleEventNetworkIdle,
						Timeout:   time.Duration(timeout.Milliseconds()), // interpreted as ms
					}))
					require.NotNil(t, resp)
					wfnPromise := p.WaitForNavigation(tb.toGojaValue(&common.FrameWaitForNavigationOptions{
						Timeout: time.Duration(timeout.Milliseconds()), // interpreted as ms
					}))
					cPromise := p.Click(tc.selector, nil)
					tb.promiseThen(tb.promiseAll(wfnPromise, cPromise), func(_ goja.Value) {
						// this is a bit pointless :shrug:
						assert.Equal(t, goja.PromiseStateFulfilled, wfnPromise.State())
						assert.Equal(t, goja.PromiseStateFulfilled, cPromise.State())
					}, func(val goja.Value) { t.Fatal(val) })

					return nil
				})
			require.NoError(t, err)
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

	require.NotNil(t, p.Goto(tb.URL("/first"), tb.toGojaValue(&common.FrameGotoOptions{
		WaitUntil: common.LifecycleEventNetworkIdle,
		Timeout:   common.DefaultTimeout,
	})))

	var timeout time.Duration = 5000 // interpreted as ms

	var wfnPromise, cPromise *goja.Promise
	err := tb.await(func() error {
		wfnPromise = p.WaitForNavigation(tb.toGojaValue(&common.FrameWaitForNavigationOptions{
			Timeout: timeout, // interpreted as ms
		}))
		cPromise = p.Click(`a`, nil)

		assert.Equal(t, goja.PromiseStatePending, wfnPromise.State())
		assert.Equal(t, goja.PromiseStatePending, cPromise.State())

		return nil
	})
	require.NoError(t, err)

	assert.Equal(t, goja.PromiseStateFulfilled, wfnPromise.State())
	assert.Equal(t, goja.PromiseStateFulfilled, cPromise.State())
	assert.Equal(t, "Second page", p.Title())
}
