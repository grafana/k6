package tests

import (
	"os"
	"testing"
	"time"

	"github.com/grafana/xk6-browser/common"

	"github.com/stretchr/testify/require"
)

func TestWaitForFrameNavigationWithinDocument(t *testing.T) {
	if os.Getenv("SKIP_FLAKY") == "true" {
		t.SkipNow()
	}
	t.Parallel()

	var timeout time.Duration = 200
	if os.Getenv("CI") == "true" {
		// Increase the timeout on underprovisioned CI machines to minimize
		// chances of intermittent failures.
		timeout *= 3
	}

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

			errc := make(chan error, 1)
			go func() {
				tb := newTestBrowser(t, withFileServer())
				p := tb.NewPage(nil)

				resp := p.Goto(tb.staticURL("/nav_in_doc.html"), nil)
				require.NotNil(t, resp)

				// A click right away could possibly trigger navigation before we
				// had a chance to call WaitForNavigation below, so give it some
				// time to simulate the JS overhead, waiting for XHR
				// response, etc.
				<-time.After(timeout * time.Millisecond) //nolint:durationcheck

				// if one of the promises panics, err will contain the error
				errc <- tb.await(func() error {
					_ = p.Click(tc.selector, nil)
					_ = p.WaitForNavigation(tb.toGojaValue(&common.FrameWaitForNavigationOptions{
						Timeout: 3 * timeout, // interpreted as ms
					}))
					return nil
				})
			}()
			select {
			case err := <-errc:
				require.NoError(t, err)
			case <-time.After(5 * timeout * time.Millisecond): //nolint:durationcheck
				t.Fatal("Test timed out")
			}
		})
	}
}
