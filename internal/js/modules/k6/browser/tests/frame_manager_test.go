// practically none of this work on windows
//go:build !windows

package tests

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
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
				_, err := p.WaitForNavigation(opts, nil)
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
		_, err := p.WaitForNavigation(opts, nil)
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

func TestFrameManagerRequestStartedWithRoutes(t *testing.T) {
	t.Parallel()

	jsRegexCheckerMock := func(pattern, url string) (bool, error) {
		matched, err := regexp.MatchString(fmt.Sprintf("http://[^/]*%s", pattern), url)
		if err != nil {
			return false, fmt.Errorf("error matching regex: %w", err)
		}

		return matched, nil
	}

	tests := []struct {
		name                   string
		routePath              string
		routeHandler           func(*common.Route)
		routeHandlerCallsCount int
		apiHandlerCallsCount   int
	}{
		{
			name:                   "request_without_routes",
			routeHandlerCallsCount: 0,
			apiHandlerCallsCount:   2,
		},
		{
			name:      "continue_request_with_matching_string_route",
			routePath: "/data/first",
			routeHandler: func(route *common.Route) {
				route.Continue()
			},
			routeHandlerCallsCount: 1,
			apiHandlerCallsCount:   2,
		},
		{
			name:      "continue_request_with_non_matching_string_route",
			routePath: "/data/third",
			routeHandler: func(route *common.Route) {
				route.Continue()
			},
			routeHandlerCallsCount: 0,
			apiHandlerCallsCount:   2,
		},
		{
			name:      "continue_request_with_multiple_matching_regex_route",
			routePath: "/data/.*",
			routeHandler: func(route *common.Route) {
				route.Continue()
			},
			routeHandlerCallsCount: 2,
			apiHandlerCallsCount:   2,
		},
		{
			name:      "abort_first_request",
			routePath: "/data/first",
			routeHandler: func(route *common.Route) {
				route.Abort("failed")
			},
			routeHandlerCallsCount: 1,
			apiHandlerCallsCount:   0, // Second API call is not made because the first throws an error
		},
		{
			name:      "abort_second_request",
			routePath: "/data/second",
			routeHandler: func(route *common.Route) {
				route.Abort("failed")
			},
			routeHandlerCallsCount: 1,
			apiHandlerCallsCount:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t, withHTTPServer())
			p := tb.NewPage(nil)

			// Track behavior
			routeHandlerCalls := 0
			apiHandlerCalls := 0

			// Set up handlers for test resources
			tb.withHandler("/test", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/html")
				_, err := fmt.Fprintf(w, `
				<html>
					<head>
						<title>Test Page</title>
					</head>
					<body>
						<h1>Test</h1>
						<script type="module">
							await fetchData();
							async function fetchData() {
								await fetch('/data/first');
								await fetch('/data/second');
							}
						</script>
					</body>
				</html>
				`)
				require.NoError(t, err)
			})

			tb.withHandler("/data/first", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, err := fmt.Fprint(w, `{"data": "First data"}`)
				require.NoError(t, err)
				apiHandlerCalls++
			})

			tb.withHandler("/data/second", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, err := fmt.Fprint(w, `{"data": "Second data"}`)
				require.NoError(t, err)
				apiHandlerCalls++
			})

			// Set up route if needed
			if tt.routeHandler != nil {
				routeHandler := func(route *common.Route) error {
					routeHandlerCalls++
					tt.routeHandler(route)
					return nil
				}

				err := p.Route(tt.routePath, routeHandler, jsRegexCheckerMock)
				require.NoError(t, err)
			}

			// Navigate to trigger requests - this will internally call requestStarted
			opts := &common.FrameGotoOptions{
				WaitUntil: common.LifecycleEventNetworkIdle,
				Timeout:   common.DefaultTimeout,
			}

			_, err := p.Goto(tb.url("/test"), opts)
			require.NoError(t, err)

			assert.Equal(t, tt.routeHandlerCallsCount, routeHandlerCalls)
			assert.Equal(t, tt.apiHandlerCallsCount, apiHandlerCalls)
		})
	}
}
