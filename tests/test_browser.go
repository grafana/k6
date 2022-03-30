/*
 *
 * xk6-browser - a browser automation extension for k6
 * Copyright (C) 2021 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package tests

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/require"
	k6http "go.k6.io/k6/js/modules/k6/http"
	k6modulestest "go.k6.io/k6/js/modulestest"
	k6lib "go.k6.io/k6/lib"
	k6httpmultibin "go.k6.io/k6/lib/testutils/httpmultibin"
	k6stats "go.k6.io/k6/stats"

	"github.com/grafana/xk6-browser/api"
	"github.com/grafana/xk6-browser/chromium"
	"github.com/grafana/xk6-browser/common"
)

// testBrowser is a test testBrowser for integration testing.
type testBrowser struct {
	t        testing.TB
	ctx      context.Context
	rt       *goja.Runtime
	state    *k6lib.State
	http     *k6httpmultibin.HTTPMultiBin
	logCache *logCache
	samples  chan<- k6stats.SampleContainer
	api.Browser
}

// newTestBrowser configures and launches a new chrome browser.
// It automatically closes it when `t` returns.
//
// opts provides a way to customize the newTestBrowser.
// see: withLaunchOptions for an example.
//nolint:funlen,cyclop
func newTestBrowser(tb testing.TB, opts ...interface{}) *testBrowser {
	tb.Helper()

	// set default options and then customize them
	var (
		launchOpts         = defaultLaunchOpts()
		enableHTTPMultiBin = false
		enableFileServer   = false
		enableLogCache     = false
		ctx                context.Context
	)
	for _, opt := range opts {
		switch opt := opt.(type) {
		case withLaunchOptions:
			launchOpts = opt
		case httpServerOption:
			enableHTTPMultiBin = true
		case fileServerOption:
			enableFileServer = true
			enableHTTPMultiBin = true
		case logCacheOption:
			enableLogCache = true
		case withContext:
			ctx = opt
		}
	}

	mockVU := setupHTTPTestModuleInstance(tb)

	if ctx == nil {
		dummyCtx, cancel := context.WithCancel(mockVU.CtxField)
		tb.Cleanup(cancel)
		mockVU.CtxField = dummyCtx
	} else {
		// Attach the mock VU to the passed context
		ctx = common.WithVU(ctx, mockVU)
		mockVU.CtxField = ctx
	}

	var (
		state = mockVU.StateField
		rt    = mockVU.RuntimeField
	)

	// enable the HTTP test server only when necessary
	var testServer *k6httpmultibin.HTTPMultiBin
	if enableHTTPMultiBin {
		testServer = k6httpmultibin.NewHTTPMultiBin(tb)
		state.TLSConfig = testServer.TLSClientConfig
		state.Transport = testServer.HTTPTransport
	}

	var lc *logCache
	if enableLogCache {
		lc = attachLogCache(state.Logger)
	}

	// launch the browser
	bt := chromium.NewBrowserType(mockVU.CtxField).(*chromium.BrowserType) //nolint:forcetypeassert
	b := bt.Launch(rt.ToValue(launchOpts))
	tb.Cleanup(func() {
		select {
		case <-mockVU.CtxField.Done():
		default:
			b.Close()
		}
	})

	tbr := &testBrowser{
		t:        tb,
		ctx:      bt.Ctx, // This context has the additional wrapping of common.WithLaunchOptions
		rt:       rt,
		state:    state,
		http:     testServer,
		samples:  state.Samples,
		logCache: lc,
		Browser:  b,
	}
	if enableFileServer {
		tbr = tbr.withFileServer()
	}

	return tbr
}

// withHandler adds the given handler to the HTTP test server and makes it
// accessible with the given pattern.
func (b *testBrowser) withHandler(pattern string, handler http.HandlerFunc) *testBrowser {
	b.t.Helper()

	if b.http == nil {
		b.t.Fatalf("You should enable HTTP test server, see: withHTTPServer option")
	}
	b.http.Mux.Handle(pattern, handler)
	return b
}

const testBrowserStaticDir = "static"

// withFileServer serves a file server using the HTTP test server that is
// accessible via `testBrowserStaticDir` prefix.
//
// This method is for enabling the static file server after starting a test
// browser. For early starting the file server see withFileServer function.
func (b *testBrowser) withFileServer() *testBrowser {
	b.t.Helper()

	const (
		slash = string(os.PathSeparator)
		path  = slash + testBrowserStaticDir + slash
	)

	fs := http.FileServer(http.Dir(testBrowserStaticDir))

	return b.withHandler(path, http.StripPrefix(path, fs).ServeHTTP)
}

// URL returns the listening HTTP test server's URL combined with the given path.
func (b *testBrowser) URL(path string) string {
	b.t.Helper()

	if b.http == nil {
		b.t.Fatalf("You should enable HTTP test server, see: withHTTPServer option")
	}
	return b.http.ServerHTTP.URL + path
}

// staticURL is a helper for URL("/`testBrowserStaticDir`/"+ path).
func (b *testBrowser) staticURL(path string) string {
	b.t.Helper()

	return b.URL("/" + testBrowserStaticDir + "/" + path)
}

// attachFrame attaches the frame to the page and returns it.
func (b *testBrowser) attachFrame(page api.Page, frameID string, url string) api.Frame {
	b.t.Helper()

	pageFn := `
	async (frameId, url) => {
		const frame = document.createElement('iframe');
		frame.src = url;
		frame.id = frameId;
		document.body.appendChild(frame);
		await new Promise(x => frame.onload = x);
		return frame;
	}
	`

	return page.EvaluateHandle(
		b.rt.ToValue(pageFn),
		b.rt.ToValue(frameID),
		b.rt.ToValue(url)).
		AsElement().
		ContentFrame()
}

// launchOptions provides a way to customize browser type
// launch options in tests.
type launchOptions struct {
	Debug    bool   `js:"debug"`
	Headless bool   `js:"headless"`
	SlowMo   string `js:"slowMo"`
	Timeout  string `js:"timeout"`
}

// withLaunchOptions is a helper for increasing readability
// in tests while customizing the browser type launch options.
//
// example:
//
//    b := TestBrowser(t, withLaunchOptions{
//        SlowMo:  "100s",
//        Timeout: "30s",
//    })
type withLaunchOptions = launchOptions

// defaultLaunchOptions returns defaults for browser type launch options.
// TestBrowser uses this for launching a browser type by default.
func defaultLaunchOpts() launchOptions {
	headless := true
	if v, found := os.LookupEnv("XK6_BROWSER_TEST_HEADLESS"); found {
		headless, _ = strconv.ParseBool(v)
	}

	return launchOptions{
		Headless: headless,
		SlowMo:   "0s",
		Timeout:  "30s",
	}
}

// httpServerOption is used to detect whether to enable the HTTP test
// server.
type httpServerOption struct{}

// withHTTPServer enables the HTTP test server.
//
// example:
//
//    b := TestBrowser(t, withHTTPServer())
func withHTTPServer() httpServerOption {
	return struct{}{}
}

// fileServerOption is used to detect whether enable the static file
// server.
type fileServerOption struct{}

// withFileServer enables the HTTP test server and serves a file server
// for static files.
//
// see: WithFileServer
//
// example:
//
//    b := TestBrowser(t, withFileServer())
func withFileServer() fileServerOption {
	return struct{}{}
}

// withContext is used to detect whether to use a custom context in the test
// browser.
type withContext = context.Context

// logCacheOption is used to detect whether to enable the log cache.
type logCacheOption struct{}

// withLogCache enables the log cache.
//
// example:
//
//    b := TestBrowser(t, withLogCache())
func withLogCache() logCacheOption {
	return struct{}{}
}

func setupHTTPTestModuleInstance(tb testing.TB) *k6modulestest.VU {
	tb.Helper()

	var (
		mockVU = newMockVU(tb)
		root   = k6http.New()
	)

	mi, ok := root.NewModuleInstance(mockVU).(*k6http.ModuleInstance)
	require.True(tb, ok)

	require.NoError(tb, mockVU.RuntimeField.Set("http", mi.Exports().Default))

	return mockVU
}
