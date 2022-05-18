package tests

import (
	"os"
	"testing"
	"time"

	"github.com/grafana/xk6-browser/common"

	"github.com/stretchr/testify/require"
)

func TestWaitForFrameNavigationWithinDocument(t *testing.T) {
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
			tb := newTestBrowser(t, withFileServer())
			p := tb.NewPage(nil)

			resp := p.Goto(tb.staticURL("/nav_in_doc.html"), nil)
			require.NotNil(t, resp)

			// A click right away could possibly trigger navigation before we
			// had a chance to call WaitForNavigation below, so give it some
			// time to simulate the JS overhead, waiting for XHR response, etc.
			time.AfterFunc(timeout*time.Millisecond, func() { //nolint:durationcheck
				p.Click(tc.selector, nil)
			})

			done := make(chan struct{}, 1)
			go func() {
				require.NotPanics(t, func() {
					p.WaitForNavigation(tb.toGojaValue(&common.FrameWaitForNavigationOptions{
						Timeout: timeout * 3, // interpreted as ms
					}))
				})
				done <- struct{}{}
			}()

			select {
			case <-done:
			case <-time.After(timeout * 5 * time.Millisecond): //nolint:durationcheck
				t.Fatal("Test timed out")
			}
		})
	}
}
