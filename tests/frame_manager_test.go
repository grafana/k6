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

	var timeout time.Duration = 5000 // interpreted as ms

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

			errc := make(chan error)
			go func() {
				tb := newTestBrowser(t, withFileServer())
				p := tb.NewPage(nil)

				resp := p.Goto(tb.staticURL("/nav_in_doc.html"), nil)
				require.NotNil(t, resp)

				// Callbacks that are initiated internally by click and WaitForNavigation
				// need to be called from the event loop itself, otherwise the callback
				// doesn't work. The await below needs to first return before the callback
				// will resolve/reject.
				var wfnPromise, cPromise *goja.Promise
				err := tb.await(func() error {
					wfnPromise = p.WaitForNavigation(tb.toGojaValue(&common.FrameWaitForNavigationOptions{
						Timeout: timeout, // interpreted as ms
					}))
					cPromise = p.Click(tc.selector, nil)

					assert.Equal(t, goja.PromiseStatePending, wfnPromise.State())
					assert.Equal(t, goja.PromiseStatePending, cPromise.State())

					return nil
				})
				if err != nil {
					errc <- err
				}

				assert.Equal(t, goja.PromiseStateFulfilled, wfnPromise.State())
				assert.Equal(t, goja.PromiseStateFulfilled, cPromise.State())

				errc <- nil
			}()

			select {
			case err := <-errc:
				assert.NoError(t, err)
			case <-time.After(time.Duration(int64(timeout)) * time.Millisecond):
				t.Fatal("Test timed out")
			}
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

	var wfnPromise, cPromise *goja.Promise
	err := tb.await(func() error {
		wfnPromise = p.WaitForNavigation(nil)
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
