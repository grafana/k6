package tests

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/grafana/xk6-browser/api"
	"github.com/grafana/xk6-browser/chromium"
	"github.com/grafana/xk6-browser/k6ext"
	"github.com/grafana/xk6-browser/k6ext/k6test"

	k6http "go.k6.io/k6/js/modules/k6/http"
	k6httpmultibin "go.k6.io/k6/lib/testutils/httpmultibin"
	k6metrics "go.k6.io/k6/metrics"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

// testBrowser is a test testBrowser for integration testing.
type testBrowser struct {
	t testing.TB

	ctx      context.Context
	http     *k6httpmultibin.HTTPMultiBin
	vu       *k6test.VU
	logCache *logCache

	api.Browser
}

// newTestBrowser configures and launches a new chrome browser.
// It automatically closes it when `t` returns.
//
// opts provides a way to customize the newTestBrowser.
// see: withLaunchOptions for an example.
func newTestBrowser(tb testing.TB, opts ...any) *testBrowser {
	tb.Helper()

	// set default options and then customize them
	var (
		ctx                context.Context
		launchOpts         = defaultLaunchOpts()
		enableHTTPMultiBin = false
		enableFileServer   = false
		enableLogCache     = false
		skipClose          = false
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
		case skipCloseOption:
			skipClose = true
		}
	}

	vu := setupHTTPTestModuleInstance(tb)

	if ctx == nil {
		dummyCtx, cancel := context.WithCancel(vu.Context())
		tb.Cleanup(cancel)
		vu.CtxField = dummyCtx
	} else {
		// Attach the mock VU to the passed context
		ctx = k6ext.WithVU(ctx, vu)
		vu.CtxField = ctx
	}

	registry := k6metrics.NewRegistry()
	k6m := k6ext.RegisterCustomMetrics(registry)
	vu.CtxField = k6ext.WithCustomMetrics(vu.Context(), k6m)

	v := chromium.NewBrowserType(vu)
	bt, ok := v.(*chromium.BrowserType)
	if !ok {
		panic(fmt.Errorf("testBrowser: unexpected browser type %T", v))
	}
	vu.MoveToVUContext()
	// enable the HTTP test server only when necessary
	var (
		testServer *k6httpmultibin.HTTPMultiBin
		state      = vu.StateField
		rt         = vu.RuntimeField
		lc         *logCache
	)

	if enableLogCache {
		lc = attachLogCache(state.Logger)
	}
	if enableHTTPMultiBin {
		testServer = k6httpmultibin.NewHTTPMultiBin(tb)
		state.TLSConfig = testServer.TLSClientConfig
		state.Transport = testServer.HTTPTransport
	}
	b := bt.Launch(rt.ToValue(launchOpts))
	tb.Cleanup(func() {
		select {
		case <-vu.Context().Done():
		default:
			if !skipClose {
				b.Close()
			}
		}
	})

	tbr := &testBrowser{
		t:        tb,
		ctx:      bt.Ctx, // This context has the additional wrapping of common.WithLaunchOptions
		http:     testServer,
		vu:       vu,
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
		b.toGojaValue(pageFn),
		b.toGojaValue(frameID),
		b.toGojaValue(url)).
		AsElement().
		ContentFrame()
}

// runtime returns a VU runtime.
func (b *testBrowser) runtime() *goja.Runtime { return b.vu.Runtime() }

// toGojaValue converts a value to goja value.
func (b *testBrowser) toGojaValue(i any) goja.Value { return b.runtime().ToValue(i) }

// asGojaValue asserts that v is a goja value and returns v as a goja.value.
func (b *testBrowser) asGojaValue(v any) goja.Value {
	b.t.Helper()
	gv, ok := v.(goja.Value)
	require.Truef(b.t, ok, "want goja.Value; got %T", v)
	return gv
}

// asGojaBool asserts that v is a boolean goja value and returns v as a boolean.
func (b *testBrowser) asGojaBool(v any) bool {
	b.t.Helper()
	gv := b.asGojaValue(v)
	require.IsType(b.t, b.toGojaValue(true), gv)
	return gv.ToBoolean()
}

// runJavaScript in the goja runtime.
func (b *testBrowser) runJavaScript(s string, args ...any) (goja.Value, error) {
	b.t.Helper()
	return b.runtime().RunString(fmt.Sprintf(s, args...))
}

// Run the given functions in parallel and waits for them to finish.
func (b *testBrowser) run(ctx context.Context, fs ...func() error) error { //nolint:unused,deadcode
	b.t.Helper()

	g, ctx := errgroup.WithContext(ctx)
	for _, f := range fs {
		f := f
		g.Go(func() error {
			errc := make(chan error, 1)
			go func() {
				errc <- f()
			}()
			select {
			case err := <-errc:
				return err
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	}

	return g.Wait()
}

// awaitWithTimeout is the same as await but takes a timeout and times out the function after the time runs out.
func (b *testBrowser) awaitWithTimeout(timeout time.Duration, fn func() error) error {
	b.t.Helper()
	errC := make(chan error)
	go func() {
		defer close(errC)
		errC <- fn()
	}()

	// use timer instead of time.After to not leak time.After for the duration of the timeout
	t := time.NewTimer(timeout)
	defer t.Stop()

	select {
	case err := <-errC:
		return err
	case <-t.C:
		return fmt.Errorf("test timed out after %s", timeout)
	}
}

func (b *testBrowser) promiseAll(promises ...*goja.Promise) testPromise {
	b.t.Helper()
	rt := b.runtime()
	val, err := rt.RunString(`(function(...promises) { return Promise.all(...promises) })`)
	require.NoError(b.t, err)
	cal, ok := goja.AssertFunction(val)
	require.True(b.t, ok)
	valPromises := make([]goja.Value, len(promises))
	for i, promise := range promises {
		valPromises[i] = rt.ToValue(promise)
	}
	val, err = cal(goja.Undefined(), rt.ToValue(valPromises))
	require.NoError(b.t, err)
	newPromise, ok := val.Export().(*goja.Promise)
	require.True(b.t, ok)

	return testPromise{
		Promise: newPromise,
		call:    cal,
		tb:      b,
	}
}

func (b *testBrowser) promise(promise *goja.Promise) testPromise {
	b.t.Helper()
	rt := b.runtime()
	val, err := rt.RunString(`
		(function(promise, resolve, reject) {
			return promise.then(resolve, reject)
		})
	`)
	require.NoError(b.t, err)
	cal, ok := goja.AssertFunction(val)
	require.True(b.t, ok)

	return testPromise{
		Promise: promise,
		call:    cal,
		tb:      b,
	}
}

type testPromise struct {
	*goja.Promise
	tb   *testBrowser
	call goja.Callable
}

func (t testPromise) then(resolve any, reject ...any) testPromise {
	var (
		val goja.Value
		err error
		rt  = t.tb.runtime()
	)
	switch len(reject) {
	case 0:
		val, err = t.call(goja.Undefined(), rt.ToValue(t.Promise), rt.ToValue(resolve))
	case 1:
		val, err = t.call(goja.Undefined(), rt.ToValue(t.Promise), rt.ToValue(resolve), rt.ToValue(reject[0]))
	default:
		panic("too many arguments to promiseThen")
	}
	require.NoError(t.tb.t, err)

	p, ok := val.Export().(*goja.Promise)
	require.True(t.tb.t, ok)

	return t.tb.promise(p)
}

// launchOptions provides a way to customize browser type
// launch options in tests.
type launchOptions struct {
	Args     []any  `js:"args"`
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
//	b := TestBrowser(t, withLaunchOptions{
//	    SlowMo:  "100s",
//	    Timeout: "30s",
//	})
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
//	b := TestBrowser(t, withHTTPServer())
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
//	b := TestBrowser(t, withFileServer())
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
//	b := TestBrowser(t, withLogCache())
func withLogCache() logCacheOption {
	return struct{}{}
}

// skipCloseOption is used to indicate that we shouldn't call Browser.Close() in
// t.Cleanup(), since it will presumably be done by the test.
type skipCloseOption struct{}

// withSkipClose skips calling Browser.Close() in t.Cleanup().
//
// example:
//
//	b := TestBrowser(t, withSkipClose())
func withSkipClose() skipCloseOption {
	return struct{}{}
}

func setupHTTPTestModuleInstance(tb testing.TB) *k6test.VU {
	tb.Helper()

	var (
		vu   = k6test.NewVU(tb)
		root = k6http.New()
	)

	mi, ok := root.NewModuleInstance(vu).(*k6http.ModuleInstance)
	require.True(tb, ok)

	require.NoError(tb, vu.Runtime().Set("http", mi.Exports().Default))

	return vu
}
