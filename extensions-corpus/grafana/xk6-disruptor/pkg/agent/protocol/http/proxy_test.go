package http

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/grafana/xk6-disruptor/pkg/agent/protocol"
)

func Test_Validations(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title       string
		disruption  Disruption
		upstream    string
		expectError bool
	}{
		{
			title: "valid defaults",
			disruption: Disruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      0.0,
				ErrorCode:      0,
				Excluded:       nil,
			},
			upstream:    "http://127.0.0.1:80",
			expectError: false,
		},
		{
			title: "invalid upstream address",
			disruption: Disruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      0.0,
				ErrorCode:      0,
				Excluded:       nil,
			},
			upstream:    "",
			expectError: true,
		},
		{
			title: "variation larger than average delay",
			disruption: Disruption{
				AverageDelay:   100,
				DelayVariation: 200,
				ErrorRate:      0.0,
				ErrorCode:      0,
				Excluded:       nil,
			},
			upstream:    "http://127.0.0.1:80",
			expectError: true,
		},
		{
			title: "valid error rate",
			disruption: Disruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      0.1,
				ErrorCode:      500,
				Excluded:       nil,
			},
			upstream:    "http://127.0.0.1:80",
			expectError: false,
		},
		{
			title: "valid delay and variation",
			disruption: Disruption{
				AverageDelay:   100,
				DelayVariation: 10,
				ErrorRate:      0.0,
				ErrorCode:      0,
				Excluded:       nil,
			},
			upstream:    "http://127.0.0.1:80",
			expectError: false,
		},
		{
			title: "invalid error code",
			disruption: Disruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      1.0,
				ErrorCode:      0,
				Excluded:       nil,
			},
			upstream:    "http://127.0.0.1:80",
			expectError: true,
		},
		{
			title: "negative error rate",
			disruption: Disruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      -1.0,
				ErrorCode:      0,
				Excluded:       nil,
			},
			upstream:    "http://127.0.0.1:80",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			listener, err := net.Listen("tcp", "localhost:0")
			if err != nil {
				t.Fatalf("error starting test proxy listener: %v", err)
			}

			_, err = NewProxy(
				listener,
				tc.upstream,
				tc.disruption,
			)
			if !tc.expectError && err != nil {
				t.Errorf("failed: %v", err)
			}

			if tc.expectError && err == nil {
				t.Errorf("should had failed")
			}
		})
	}
}

func Test_ProxyHandler(t *testing.T) {
	t.Parallel()

	type TestCase struct {
		title           string
		disruption      Disruption
		method          string
		path            string
		statusCode      int
		upstreamHeaders http.Header
		upstreamBody    []byte
		expectedStatus  int
		expectedHeaders http.Header
		expectedBody    []byte
	}

	testCases := []TestCase{
		{
			title: "default proxy",
			disruption: Disruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      0.0,
				ErrorCode:      0,
				Excluded:       nil,
			},
			path:           "",
			statusCode:     200,
			upstreamBody:   []byte("content body"),
			expectedStatus: 200,
			expectedBody:   []byte("content body"),
		},
		{
			title: "Error code 500",
			disruption: Disruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      1.0,
				ErrorCode:      500,
				Excluded:       nil,
			},
			path:           "",
			statusCode:     200,
			upstreamBody:   []byte("content body"),
			expectedStatus: 500,
			expectedBody:   []byte(""),
		},
		{
			title: "Exclude path",
			disruption: Disruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      1.0,
				ErrorCode:      500,
				Excluded:       []string{"/excluded/path"},
			},
			path:           "/excluded/path",
			statusCode:     200,
			upstreamBody:   []byte("content body"),
			expectedStatus: 200,
			expectedBody:   []byte("content body"),
		},
		{
			title: "Not Excluded path",
			disruption: Disruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      1.0,
				ErrorCode:      500,
				Excluded:       []string{"/excluded/path"},
			},
			path:           "/non-excluded/path",
			statusCode:     200,
			upstreamBody:   []byte("content body"),
			expectedStatus: 500,
			expectedBody:   []byte(""),
		},
		{
			title: "Error code 500 with body template",
			disruption: Disruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      1.0,
				ErrorCode:      500,
				ErrorBody:      "{\"error\": 500, \"message\":\"internal server error\"}",
				Excluded:       nil,
			},
			path:           "",
			statusCode:     200,
			upstreamBody:   []byte("content body"),
			expectedStatus: 500,
			expectedBody:   []byte("{\"error\": 500, \"message\":\"internal server error\"}"),
		},
		{
			title: "Headers are preserved when endpoint is skipped",
			disruption: Disruption{
				Excluded: []string{"/excluded"},
			},
			path:       "/excluded",
			statusCode: 200,
			upstreamHeaders: http.Header{
				"X-Test-Header": []string{"A-Test"},
			},
			upstreamBody:   []byte("content body"),
			expectedStatus: 200,
			expectedHeaders: http.Header{
				"X-Test-Header": []string{"A-Test"},
			},
			expectedBody: []byte("content body"),
		},
		{
			title: "Headers are preserved when errors are not injected",
			disruption: Disruption{
				ErrorRate: 0,
			},
			statusCode: 200,
			upstreamHeaders: http.Header{
				"X-Test-Header": []string{"A-Test"},
			},
			upstreamBody:   []byte("content body"),
			expectedStatus: 200,
			expectedHeaders: http.Header{
				"X-Test-Header": []string{"A-Test"},
			},
			expectedBody: []byte("content body"),
		},
		{
			title: "Headers are discarded when errors are injected",
			disruption: Disruption{
				ErrorRate: 1.0,
				ErrorCode: 500,
			},
			statusCode: 200,
			upstreamHeaders: http.Header{
				"X-Test-Header": []string{"A-Test"},
			},
			upstreamBody:    []byte("content body"),
			expectedStatus:  500,
			expectedHeaders: http.Header{},
			expectedBody:    nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			upstreamServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
				for k, values := range tc.upstreamHeaders {
					for _, v := range values {
						rw.Header().Add(k, v)
					}
				}
				rw.WriteHeader(tc.statusCode)

				_, err := rw.Write(tc.upstreamBody)
				if err != nil {
					t.Fatalf("writing upstream body: %v", err)
				}
			}))

			upstreamURL, err := url.Parse(upstreamServer.URL)
			if err != nil {
				t.Fatalf("error parsing httptest url")
			}

			handler := &httpHandler{
				upstreamURL: *upstreamURL,
				disruption:  tc.disruption,
				metrics:     protocol.NewMetricMap(supportedMetrics()...),
			}

			proxyServer := httptest.NewServer(handler)

			req, err := http.NewRequest(tc.method, proxyServer.URL+tc.path, nil)
			if err != nil {
				t.Fatalf("building request to proxy: %v", err)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("making request to proxy: %v", err)
			}

			if tc.expectedStatus != resp.StatusCode {
				t.Fatalf("expected status code '%d' but '%d' received ", tc.expectedStatus, resp.StatusCode)
			}

			// Remove standard response headers so we don't need to specify them on every test case.
			resp.Header.Del("Content-Length")
			resp.Header.Del("Content-Type")
			resp.Header.Del("Date")

			// Compare headers only if either expected or returned have items.
			// We have to check for length explicitly as otherwise a nil map would not be equal to an empty map.
			if len(tc.upstreamHeaders) > 0 || len(tc.expectedHeaders) > 0 {
				if diff := cmp.Diff(tc.expectedHeaders, resp.Header); diff != "" {
					t.Fatalf("Expected headers did not match returned:\n%s", diff)
				}
			}

			var body bytes.Buffer
			_, _ = io.Copy(&body, resp.Body)
			if !bytes.Equal(tc.expectedBody, body.Bytes()) {
				t.Fatalf("expected body '%s' but '%s' received ", tc.expectedBody, body.Bytes())
			}
		})
	}
}

// TODO: This test covers metrics generated by the handler, but not the proxy. The reason for this is that the proxy is
// currently not easily testable, as it coupled with `http.ListenAndServe`.
func Test_Metrics(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name            string
		config          Disruption
		endpoints       []string
		expectedMetrics map[string]uint
	}{
		{
			name: "no requests",
			expectedMetrics: map[string]uint{
				protocol.MetricRequests:          0,
				protocol.MetricRequestsExcluded:  0,
				protocol.MetricRequestsDisrupted: 0,
			},
		},
		{
			name: "requests",
			config: Disruption{
				Excluded:  []string{"/excluded"},
				ErrorRate: 1.0,
				ErrorCode: http.StatusTeapot,
			},
			endpoints: []string{"/included", "/excluded"},
			expectedMetrics: map[string]uint{
				protocol.MetricRequests:          2,
				protocol.MetricRequestsExcluded:  1,
				protocol.MetricRequestsDisrupted: 1,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			upstreamServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
				rw.WriteHeader(http.StatusOK)
			}))

			upstreamURL, err := url.Parse(upstreamServer.URL)
			if err != nil {
				t.Fatalf("error parsing httptest url")
			}

			metrics := protocol.NewMetricMap(supportedMetrics()...)

			handler := &httpHandler{
				upstreamURL: *upstreamURL,
				disruption:  tc.config,
				metrics:     metrics,
			}

			proxyServer := httptest.NewServer(handler)

			for _, endpoint := range tc.endpoints {
				_, err = http.Get(proxyServer.URL + endpoint)
				if err != nil {
					t.Fatalf("requesting %s: %v", endpoint, err)
				}
			}

			if diff := cmp.Diff(tc.expectedMetrics, metrics.Map()); diff != "" {
				t.Fatalf("expected metrics do not match output:\n%s", diff)
			}
		})
	}
}
