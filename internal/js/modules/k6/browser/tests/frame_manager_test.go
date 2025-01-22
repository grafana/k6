// practically none of this work on windows
//go:build !windows

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
)

func TestWaitForFrameNavigationWithinDocument(t *testing.T) {
	t.Parallel()

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

			opts := &common.FrameGotoOptions{
				WaitUntil: common.LifecycleEventNetworkIdle,
				Timeout:   timeout,
			}
			resp, err := p.Goto(tb.staticURL("/nav_in_doc.html"), opts)
			require.NoError(t, err)
			require.NotNil(t, resp)

			waitForNav := func() error {
				opts := &common.FrameWaitForNavigationOptions{Timeout: timeout}
				_, err := p.WaitForNavigation(opts)
				return err
			}
			click := func() error {
				return p.Click(tc.selector, common.NewFrameClickOptions(p.Timeout()))
			}
			ctx, cancel := context.WithTimeout(tb.ctx, timeout)
			defer cancel()
			err = tb.run(ctx, waitForNav, click)
			require.NoError(t, err)
		})
	}
}

func TestWaitForFrameNavigation(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withHTTPServer())
	p := tb.NewPage(nil)

	tb.withHandler("/first", func(w http.ResponseWriter, _ *http.Request) {
		_, err := fmt.Fprintf(w, `
		<html>
			<head>
				<title>First page</title>
			</head>
			<body>
				<a href="/second">click me</a>
			</body>
		</html>
		`)
		require.NoError(t, err)
	})
	tb.withHandler("/second", func(w http.ResponseWriter, _ *http.Request) {
		_, err := fmt.Fprintf(w, `
		<html>
			<head>
				<title>Second page</title>
			</head>
			<body>
				<a href="/first">click me</a>
			</body>
		</html>
		`)
		require.NoError(t, err)
	})

	opts := &common.FrameGotoOptions{
		WaitUntil: common.LifecycleEventNetworkIdle,
		Timeout:   common.DefaultTimeout,
	}
	_, err := p.Goto(tb.url("/first"), opts)
	require.NoError(t, err)

	waitForNav := func() error {
		opts := &common.FrameWaitForNavigationOptions{
			Timeout: 5000 * time.Millisecond,
		}
		_, err := p.WaitForNavigation(opts)
		return err
	}
	click := func() error {
		return p.Click(`a`, common.NewFrameClickOptions(p.Timeout()))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = tb.run(ctx, waitForNav, click)
	require.NoError(t, err)

	title, err := p.Title()
	require.NoError(t, err)
	assert.Equal(t, "Second page", title)
}
