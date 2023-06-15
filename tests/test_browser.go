package tests

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/grafana/xk6-browser/chromium"
	"github.com/grafana/xk6-browser/common"
	"github.com/grafana/xk6-browser/env"
	"github.com/grafana/xk6-browser/k6ext"
	"github.com/grafana/xk6-browser/k6ext/k6test"

	k6http "go.k6.io/k6/js/modules/k6/http"
	k6httpmultibin "go.k6.io/k6/lib/testutils/httpmultibin"
	k6metrics "go.k6.io/k6/metrics"
)

const testBrowserStaticDir = "static"

// testBrowser is a test testBrowser for integration testing.
type testBrowser struct {
	t testing.TB

	ctx    context.Context
	cancel context.CancelFunc
	vu     *k6test.VU

	browserType *chromium.BrowserType
	pid         int // the browser process ID
	wsURL       string

	*common.Browser

	// http is set by the withHTTPServer option.
	http *k6httpmultibin.HTTPMultiBin
	// logCache is set by the withLogCache option.
	logCache *logCache
}

// newTestBrowser configures and launches a new chrome browser.
//
// It automatically closes it when `t` returns unless `withSkipClose` option is provided.
//
// The following opts are available to customize the testBrowser:
//   - withEnvLookup: provides a custom lookup function for environment variables.
//   - withFileServer: enables the HTTPMultiBin server and serves the given files.
//   - withHTTPServer: enables the HTTPMultiBin server.
//   - withLogCache: enables the log cache.
//   - withSamples: provides a channel to receive the browser metrics.
//   - withSkipClose: skips closing the browser when the test finishes.
func newTestBrowser(tb testing.TB, opts ...func(*testBrowserOptions)) *testBrowser {
	tb.Helper()

	tbr := &testBrowser{t: tb}
	tbopts := newTestBrowserOptions(tbr, opts...) // apply pre-init stage options.
	tbr.browserType, tbr.vu, tbr.cancel = newBrowserTypeWithVU(tb, tbopts)
	tb.Cleanup(tbr.cancel)
	tbopts.isBrowserTypeInitialized = true // some option require the browser type to be initialized.
	tbopts.apply(opts...)                  // apply post-init stage options.

	if tbopts.httpMultiBin {
		tbr.http = k6httpmultibin.NewHTTPMultiBin(tb)
		tbr.vu.StateField.TLSConfig = tbr.http.TLSClientConfig
		tbr.vu.StateField.Transport = tbr.http.HTTPTransport
	}
	if tbopts.fileServer {
		tbr = tbr.withFileServer()
	}

	b, pid, err := tbr.browserType.Launch(tbr.vu.Context())
	if err != nil {
		tb.Fatalf("testBrowser: %v", err)
	}
	cb, ok := b.(*common.Browser)
	if !ok {
		tb.Fatalf("testBrowser: unexpected browser %T", b)
	}
	tbr.Browser = cb
	tbr.ctx = tbr.browserType.Ctx
	tbr.pid = pid
	tbr.wsURL = cb.WsURL()
	tb.Cleanup(func() {
		select {
		case <-tbr.vu.Context().Done():
		default:
			if !tbopts.skipClose {
				cb.Close()
			}
		}
	})

	return tbr
}

// NewPage is a wrapper around Browser.NewPage that fails the test if an
// error occurs. Added this helper to avoid boilerplate code in tests.
func (b *testBrowser) NewPage(opts goja.Value) *common.Page {
	b.t.Helper()

	p, err := b.Browser.NewPage(opts)
	require.NoError(b.t, err)

	pp, ok := p.(*common.Page)
	require.Truef(b.t, ok, "want *common.Page, got %T", p)

	return pp
}

// url returns the listening HTTP test server's url combined with the given path.
func (b *testBrowser) url(path string) string {
	b.t.Helper()

	if b.http == nil {
		b.t.Fatalf("You should enable HTTP test server, see: withHTTPServer option")
	}
	return b.http.ServerHTTP.URL + path
}

// staticURL is a helper for URL("/`testBrowserStaticDir`/"+ path).
func (b *testBrowser) staticURL(path string) string {
	b.t.Helper()
	return b.url("/" + testBrowserStaticDir + "/" + path)
}

// context returns the testBrowser context.
func (b *testBrowser) context() context.Context { return b.ctx }

// cancelContext cancels the testBrowser context.
func (b *testBrowser) cancelContext() { b.cancel() }

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
	v, err := b.runtime().RunString(fmt.Sprintf(s, args...))
	if err != nil {
		return nil, fmt.Errorf("while running %q(%v): %w", s, args, err)
	}
	return v, nil
}

// Run the given functions in parallel and waits for them to finish.
func (b *testBrowser) run(ctx context.Context, fs ...func() error) error {
	b.t.Helper()

	g, ctx := errgroup.WithContext(ctx)
	for _, f := range fs {
		f := f
		g.Go(func() error {
			errc := make(chan error, 1)
			go func() { errc <- f() }()
			select {
			case err := <-errc:
				return err
			case <-ctx.Done():
				if err := ctx.Err(); err != nil {
					return fmt.Errorf("while running %T: %w", f, err)
				}
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return fmt.Errorf("while waiting for %T: %w", fs, err)
	}

	return nil
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

// testBrowserOptions is a helper for creating testBrowser options.
type testBrowserOptions struct {
	// testBrowser field provides access to the testBrowser instance for
	// the options to modify it.
	testBrowser *testBrowser
	// isBrowserTypeInitialized is true if the
	// browser type has been initialized with a VU.
	isBrowserTypeInitialized bool

	// options

	fileServer   bool
	httpMultiBin bool
	samples      chan k6metrics.SampleContainer
	skipClose    bool
	lookupFunc   env.LookupFunc
}

// newTestBrowserOptions creates a new testBrowserOptions with the given options.
// call apply to reapply the options.
func newTestBrowserOptions(tb *testBrowser, opts ...func(*testBrowserOptions)) *testBrowserOptions {
	// default lookup function is env.Lookup so that we can
	// pass the environment variables while testing, i.e.: K6_BROWSER_LOG.
	tbo := &testBrowserOptions{
		testBrowser: tb,
		samples:     make(chan k6metrics.SampleContainer, 1000),
		lookupFunc:  env.Lookup,
	}
	tbo.apply(opts...)

	return tbo
}

// apply applies the given options to the testBrowserOptions.
func (tbo *testBrowserOptions) apply(opts ...func(*testBrowserOptions)) {
	for _, opt := range opts {
		opt(tbo)
	}
}

// withEnvLookup sets the lookup function for environment variables.
//
// example:
//
//	b := TestBrowser(t, withEnvLookup(env.ConstLookup(env.BrowserHeadless, "0")))
func withEnvLookup(lookupFunc env.LookupFunc) func(*testBrowserOptions) {
	return func(tb *testBrowserOptions) { tb.lookupFunc = lookupFunc }
}

// withFileServer enables the HTTP test server and serves a file server
// for static files.
//
// see: WithFileServer
//
// example:
//
//	b := TestBrowser(t, withFileServer())
func withFileServer() func(tb *testBrowserOptions) {
	return func(tb *testBrowserOptions) {
		tb.httpMultiBin = true
		tb.fileServer = true
	}
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

// withHTTPServer enables the HTTP test server.
// It is used to detect whether to enable the HTTP test server.
//
// example:
//
//	b := TestBrowser(t, withHTTPServer())
func withHTTPServer() func(tb *testBrowserOptions) {
	return func(tb *testBrowserOptions) { tb.httpMultiBin = true }
}

// withLogCache enables the log cache.
//
// example:
//
//	b := TestBrowser(t, withLogCache())
func withLogCache() func(tb *testBrowserOptions) {
	return func(opts *testBrowserOptions) {
		if !opts.isBrowserTypeInitialized {
			return
		}
		tb := opts.testBrowser
		tb.logCache = attachLogCache(tb.t, tb.vu.StateField.Logger)
	}
}

// withSamples is used to indicate we want to use a bidirectional channel
// so that the test can read the metrics being emitted to the channel.
func withSamples(sc chan k6metrics.SampleContainer) func(tb *testBrowserOptions) {
	return func(tb *testBrowserOptions) { tb.samples = sc }
}

// withSkipClose skips calling Browser.Close() in t.Cleanup().
// It indicates that we shouldn't call Browser.Close() in
// t.Cleanup(), since it will presumably be done by the test.
//
// example:
//
//	b := TestBrowser(t, withSkipClose())
func withSkipClose() func(tb *testBrowserOptions) {
	return func(tb *testBrowserOptions) { tb.skipClose = true }
}

// newBrowserTypeWithVU creates a new browser type with a VU.
func newBrowserTypeWithVU(tb testing.TB, opts *testBrowserOptions) (
	_ *chromium.BrowserType,
	_ *k6test.VU,
	cancel func(),
) {
	tb.Helper()

	vu := k6test.NewVU(tb, k6test.WithSamples(opts.samples))
	mi, ok := k6http.New().NewModuleInstance(vu).(*k6http.ModuleInstance)
	require.Truef(tb, ok, "want *k6http.ModuleInstance; got %T", mi)
	require.NoError(tb, vu.Runtime().Set("http", mi.Exports().Default))
	metricsCtx := k6ext.WithCustomMetrics(
		vu.Context(),
		k6ext.RegisterCustomMetrics(k6metrics.NewRegistry()),
	)
	ctx, cancel := context.WithCancel(metricsCtx)
	vu.CtxField = ctx
	vu.InitEnvField.LookupEnv = opts.lookupFunc

	bt := chromium.NewBrowserType(vu)
	vu.RestoreVUState()

	return bt, vu, cancel
}
