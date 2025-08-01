// practically none of this work on windows
//go:build !windows

package tests

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
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

	type callsCount struct {
		routeHandler int
		firstApi     int
		secondApi    int
	}
	tests := []struct {
		name         string
		routePath    string
		routeHandler func(*common.Route)
		callsCount   callsCount
	}{
		{
			name: "request_without_routes",
			callsCount: callsCount{
				routeHandler: 0,
				firstApi:     1,
				secondApi:    1,
			},
		},
		{
			name:      "continue_request_with_matching_string_route",
			routePath: "/data/first",
			routeHandler: func(route *common.Route) {
				err := route.Continue(common.ContinueOptions{})
				assert.NoError(t, err)
			},
			callsCount: callsCount{
				routeHandler: 1,
				firstApi:     1,
				secondApi:    1,
			},
		},
		{
			name:      "continue_request_with_non_matching_string_route",
			routePath: "/data/third",
			routeHandler: func(route *common.Route) {
				err := route.Continue(common.ContinueOptions{})
				assert.NoError(t, err)
			},
			callsCount: callsCount{
				routeHandler: 0,
				firstApi:     1,
				secondApi:    1,
			},
		},
		{
			name:      "continue_request_with_multiple_matching_regex_route",
			routePath: "/data/.*",
			routeHandler: func(route *common.Route) {
				err := route.Continue(common.ContinueOptions{})
				assert.NoError(t, err)
			},
			callsCount: callsCount{
				routeHandler: 2,
				firstApi:     1,
				secondApi:    1,
			},
		},
		{
			name:      "continue_request_with_opts",
			routePath: "/data/first",
			routeHandler: func(route *common.Route) {
				newURL := strings.Replace(route.Request().URL(), "/data/first", "/data/second", 1)
				err := route.Continue(common.ContinueOptions{
					URL: newURL,
				})
				assert.NoError(t, err)
			},
			callsCount: callsCount{
				routeHandler: 1,
				firstApi:     0,
				secondApi:    2,
			},
		},
		{
			name:      "abort_first_request",
			routePath: "/data/first",
			routeHandler: func(route *common.Route) {
				err := route.Abort("failed")
				assert.NoError(t, err)
			},
			callsCount: callsCount{
				routeHandler: 1,
				firstApi:     0,
				secondApi:    0, // Second API call is not made because the first throws an error
			},
		},
		{
			name:      "abort_second_request",
			routePath: "/data/second",
			routeHandler: func(route *common.Route) {
				err := route.Abort("failed")
				assert.NoError(t, err)
			},
			callsCount: callsCount{
				routeHandler: 1,
				firstApi:     1,
				secondApi:    0,
			},
		},
		{
			name:      "fulfill_request",
			routePath: "/data/first",
			routeHandler: func(route *common.Route) {
				err := route.Fulfill(common.FulfillOptions{
					Body:        []byte(`{"data": "Fulfilled data"}`),
					ContentType: "application/json",
					Headers: []common.HTTPHeader{
						{Name: "Access-Control-Allow-Origin", Value: "*"},
					},
					Status: 200,
				})
				assert.NoError(t, err)
			},
			callsCount: callsCount{
				routeHandler: 1,
				firstApi:     0,
				secondApi:    1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t, withHTTPServer())
			p := tb.NewPage(nil)

			// Track number of calls in each function
			calls := callsCount{
				routeHandler: 0,
				firstApi:     0,
				secondApi:    0,
			}

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
				calls.firstApi++
			})

			tb.withHandler("/data/second", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, err := fmt.Fprint(w, `{"data": "Second data"}`)
				require.NoError(t, err)
				calls.secondApi++
			})

			// Set up route if needed
			if tt.routeHandler != nil {
				routeHandler := func(route *common.Route) error {
					calls.routeHandler++
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

			assert.Equal(t, tt.callsCount.routeHandler, calls.routeHandler)
			assert.Equal(t, tt.callsCount.firstApi, calls.firstApi)
			assert.Equal(t, tt.callsCount.secondApi, calls.secondApi)
		})
	}
}
