package tests

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/k6ext/k6test"
)

type browserContextProxyRequest struct {
	Method     string
	URL        string
	RequestURI string
	TestHeader string
}

type browserContextTargetRequest struct {
	Method       string
	Path         string
	RawQuery     string
	TestHeader   string
	ThroughProxy bool
}

func TestBrowserContextProxy(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		useProxy  bool
		wantProxy bool
	}{
		{
			name:      "without_proxy",
			useProxy:  false,
			wantProxy: false,
		},
		{
			name:      "with_proxy",
			useProxy:  true,
			wantProxy: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			targetRequests := make(chan browserContextTargetRequest, 4)
			targetServer := newBrowserContextProxyTarget(t, targetRequests)
			t.Cleanup(targetServer.Close)

			proxyRequests := make(chan browserContextProxyRequest, 4)
			proxyServer := newBrowserContextProxyServer(t, targetServer.URL, proxyRequests)
			t.Cleanup(proxyServer.Close)

			tb := newTestBrowser(t)
			targetURL := targetServer.URL + "/proxy-check?case=" + tc.name
			testHeader := "browser-context-" + tc.name

			bodyText := runBrowserContextProxyNavigation(t, tb, targetURL, proxyServer.URL, testHeader, tc.useProxy)

			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			t.Cleanup(cancel)

			targetRequest := receiveBrowserContextTargetRequest(t, ctx, targetRequests)
			assert.Equal(t, "GET", targetRequest.Method)
			assert.Equal(t, "/proxy-check", targetRequest.Path)
			assert.Equal(t, "case="+tc.name, targetRequest.RawQuery)
			assert.Equal(t, testHeader, targetRequest.TestHeader)
			assert.Equal(t, tc.wantProxy, targetRequest.ThroughProxy)

			if tc.wantProxy {
				assert.Equal(t, "proxied:"+testHeader, bodyText)

				proxyRequest := receiveBrowserContextProxyRequest(t, ctx, proxyRequests)
				assert.Equal(t, "GET", proxyRequest.Method)
				assert.Equal(t, targetURL, proxyRequest.URL)
				assert.Equal(t, targetURL, proxyRequest.RequestURI)
				assert.Equal(t, testHeader, proxyRequest.TestHeader)
			} else {
				assert.Equal(t, "direct:"+testHeader, bodyText)
				assertNoBrowserContextProxyRequest(t, proxyRequests)
			}
		})
	}
}

func TestBrowserContextProxyRequiresServer(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t)
	tb.vu.ActivateVU()
	tb.vu.StartIteration(t)
	defer tb.vu.EndIteration(t)

	gv, err := tb.vu.RunAsync(t, `
		await browser.newContext({ proxy: { bypass: 'foo' } });
	`)

	got := k6test.ToPromise(t, gv)
	assert.ErrorContains(t, err, "parsing browser.newContext options: proxy.server must be set")
	assert.Equal(t, sobek.PromiseStateRejected, got.State())
}

func runBrowserContextProxyNavigation(
	t *testing.T, tb *testBrowser, targetURL, proxyURL, testHeader string, useProxy bool,
) string {
	t.Helper()

	contextOptions := fmt.Sprintf(`{
		extraHTTPHeaders: { 'X-K6-Proxy-Test': %q },
	}`, testHeader)
	if useProxy {
		contextOptions = fmt.Sprintf(`{
			extraHTTPHeaders: { 'X-K6-Proxy-Test': %q },
			proxy: { server: %q, bypass: '<-loopback>' },
		}`, testHeader, proxyURL)
	}

	tb.vu.ActivateVU()
	tb.vu.StartIteration(t)

	gv, err := tb.vu.RunAsync(t, `
		const context = await browser.newContext(%s);
		const page = await context.newPage();
		try {
			await page.goto(%q, { waitUntil: 'load' });
			return await page.textContent('body');
		} finally {
			await page.close();
			await context.close();
		}
	`, contextOptions, targetURL)
	require.NoError(t, err)

	got := k6test.ToPromise(t, gv)
	require.Equal(t, sobek.PromiseStateFulfilled, got.State())
	return got.Result().String()
}

func newBrowserContextProxyTarget(
	t *testing.T, requests chan<- browserContextTargetRequest,
) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/proxy-check" {
			http.NotFound(w, r)
			return
		}

		throughProxy := r.Header.Get("X-K6-Proxy-Forwarded") == "true"
		requests <- browserContextTargetRequest{
			Method:       r.Method,
			Path:         r.URL.Path,
			RawQuery:     r.URL.RawQuery,
			TestHeader:   r.Header.Get("X-K6-Proxy-Test"),
			ThroughProxy: throughProxy,
		}

		prefix := "direct"
		if throughProxy {
			prefix = "proxied"
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, err := fmt.Fprintf(
			w,
			`<!doctype html><html><head><link rel="icon" href="data:,"></head><body>%s:%s</body></html>`,
			prefix,
			r.Header.Get("X-K6-Proxy-Test"),
		)
		if !assert.NoError(t, err) {
			return
		}
	}))
}

func newBrowserContextProxyServer(
	t *testing.T, targetServerURL string, requests chan<- browserContextProxyRequest,
) *httptest.Server {
	t.Helper()

	targetURL, err := url.Parse(targetServerURL)
	require.NoError(t, err)

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	t.Cleanup(transport.CloseIdleConnections)

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodConnect {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if !r.URL.IsAbs() {
			http.Error(w, "expected absolute-form proxy request", http.StatusBadRequest)
			return
		}

		if r.URL.Host == targetURL.Host && r.URL.Path == "/proxy-check" {
			requests <- browserContextProxyRequest{
				Method:     r.Method,
				URL:        r.URL.String(),
				RequestURI: r.RequestURI,
				TestHeader: r.Header.Get("X-K6-Proxy-Test"),
			}
		}

		outReq := r.Clone(r.Context())
		outReq.RequestURI = ""
		outReq.Header = r.Header.Clone()
		outReq.Header.Set("X-K6-Proxy-Forwarded", "true")

		resp, err := transport.RoundTrip(outReq)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close() //nolint:errcheck

		for key, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.WriteHeader(resp.StatusCode)
		_, err = io.Copy(w, resp.Body)
		if !assert.NoError(t, err) {
			return
		}
	}))
}

func receiveBrowserContextTargetRequest(
	t *testing.T, ctx context.Context, requests <-chan browserContextTargetRequest,
) browserContextTargetRequest {
	t.Helper()

	select {
	case req := <-requests:
		return req
	case <-ctx.Done():
		require.FailNow(t, "timed out waiting for target request")
	}

	return browserContextTargetRequest{}
}

func receiveBrowserContextProxyRequest(
	t *testing.T, ctx context.Context, requests <-chan browserContextProxyRequest,
) browserContextProxyRequest {
	t.Helper()

	select {
	case req := <-requests:
		return req
	case <-ctx.Done():
		require.FailNow(t, "timed out waiting for proxy request")
	}

	return browserContextProxyRequest{}
}

func assertNoBrowserContextProxyRequest(t *testing.T, requests <-chan browserContextProxyRequest) {
	t.Helper()

	select {
	case req := <-requests:
		require.Failf(t, "unexpected proxy request", "%+v", req)
	default:
	}
}
