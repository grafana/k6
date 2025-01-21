package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"go.k6.io/k6/internal/js/modules/k6/browser/chromium"
	"go.k6.io/k6/internal/js/modules/k6/browser/common"
	"go.k6.io/k6/internal/js/modules/k6/browser/env"
	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext"
	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext/k6test"

	k6httpmultibin "go.k6.io/k6/internal/lib/testutils/httpmultibin"
	k6http "go.k6.io/k6/js/modules/k6/http"
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

	// isBrowserTypeInitialized is true if the browser type has been
	// initialized with a VU. Some options can only be used in the
	// post-init stage and require the browser type to be initialized.
	isBrowserTypeInitialized bool

	// http is set by the withHTTPServer option.
	http *k6httpmultibin.HTTPMultiBin
	// logCache is set by the withLogCache option.
	logCache *logCache
	// lookupFunc is set by the withEnvLookup option.
	lookupFunc env.LookupFunc
	// samples is set by the withSamples option.
	samples chan k6metrics.SampleContainer
	// skipClose is set by the withSkipClose option.
	skipClose bool
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
func newTestBrowser(tb testing.TB, opts ...func(*testBrowser)) *testBrowser {
	tb.Helper()

	tbr := &testBrowser{t: tb}
	tbr.applyDefaultOptions()
	tbr.applyOptions(opts...) // apply pre-init stage options.
	tbr.vu, tbr.cancel = newTestBrowserVU(tb, tbr)
	tbr.browserType = chromium.NewBrowserType(tbr.vu)
	tbr.vu.ActivateVU()
	tbr.isBrowserTypeInitialized = true // some option require the browser type to be initialized.
	tbr.applyOptions(opts...)           // apply post-init stage options.

	b, pid, err := tbr.browserType.Launch(context.Background(), tbr.vu.Context())
	if err != nil {
		tb.Fatalf("testBrowser: %v", err)
	}
	tbr.Browser = b
	tbr.ctx = tbr.browserType.Ctx
	tbr.pid = pid
	tbr.wsURL = b.WsURL()
	tb.Cleanup(func() {
		select {
		case <-tbr.vu.Context().Done():
		default:
			if !tbr.skipClose {
				b.Close()
			}
		}
	})

	return tbr
}

// newTestBrowserVU initializes a new VU for browser testing.
// It returns the VU and a cancel function to stop the VU.
// VU contains the context with the custom metrics registry.
func newTestBrowserVU(tb testing.TB, tbr *testBrowser) (_ *k6test.VU, cancel func()) {
	tb.Helper()

	vu := k6test.NewVU(tb, k6test.WithSamples(tbr.samples))
	mi, ok := k6http.New().NewModuleInstance(vu).(*k6http.ModuleInstance)
	require.Truef(tb, ok, "want *k6http.ModuleInstance; got %T", mi)
	require.NoError(tb, vu.Runtime().Set("http", mi.Exports().Default))
	metricsCtx := k6ext.WithCustomMetrics(
		vu.Context(),
		k6ext.RegisterCustomMetrics(k6metrics.NewRegistry()),
	)
	ctx, cancel := context.WithCancel(metricsCtx)
	tb.Cleanup(cancel)
	vu.CtxField = ctx
	vu.InitEnvField.LookupEnv = tbr.lookupFunc

	return vu, cancel
}

// applyDefaultOptions applies the default options for the testBrowser.
func (b *testBrowser) applyDefaultOptions() {
	b.samples = make(chan k6metrics.SampleContainer, 1000)
	// default lookup function is env.Lookup so that we can
	// pass the environment variables while testing, i.e.: K6_BROWSER_LOG.
	b.lookupFunc = env.Lookup
}

// applyOptions applies the given options to the testBrowser.
func (b *testBrowser) applyOptions(opts ...func(*testBrowser)) {
	for _, opt := range opts {
		opt(b)
	}
}

// withEnvLookup sets the lookup function for environment variables.
//
// example:
//
//	b := TestBrowser(t, withEnvLookup(env.ConstLookup(env.BrowserHeadless, "0")))
func withEnvLookup(lookupFunc env.LookupFunc) func(*testBrowser) {
	return func(tb *testBrowser) { tb.lookupFunc = lookupFunc }
}

// withFileServer enables the HTTP test server and serves a file server
// for static files.
//
// see: WithFileServer
//
// example:
//
//	b := TestBrowser(t, withFileServer())
func withFileServer() func(*testBrowser) {
	return func(tb *testBrowser) {
		if !tb.isBrowserTypeInitialized {
			return
		}
		if tb.http == nil {
			// file server needs HTTP server.
			apply := withHTTPServer()
			apply(tb)
		}
		_ = tb.withFileServer()
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
		slash = string(os.PathSeparator) //nolint:forbidigo
		path  = slash + testBrowserStaticDir + slash
	)

	fs := http.FileServer(http.Dir(testBrowserStaticDir))

	return b.withHandler("/"+testBrowserStaticDir+"/", http.StripPrefix(path, fs).ServeHTTP)
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
func withHTTPServer() func(*testBrowser) {
	return func(tb *testBrowser) {
		if !tb.isBrowserTypeInitialized {
			return
		}
		if tb.http != nil {
			// already initialized.
			return
		}
		tb.http = k6httpmultibin.NewHTTPMultiBin(tb.t)
		tb.vu.StateField.TLSConfig = tb.http.TLSClientConfig
		tb.vu.StateField.Transport = tb.http.HTTPTransport
	}
}

// withLogCache enables the log cache.
//
// example:
//
//	b := TestBrowser(t, withLogCache())
func withLogCache() func(*testBrowser) {
	return func(tb *testBrowser) {
		if !tb.isBrowserTypeInitialized {
			return
		}
		tb.logCache = attachLogCache(tb.t, tb.vu.StateField.Logger)
	}
}

// withSamples is used to indicate we want to use a bidirectional channel
// so that the test can read the metrics being emitted to the channel.
func withSamples(sc chan k6metrics.SampleContainer) func(*testBrowser) {
	return func(tb *testBrowser) { tb.samples = sc }
}

// withSkipClose skips calling Browser.Close() in t.Cleanup().
// It indicates that we shouldn't call Browser.Close() in
// t.Cleanup(), since it will presumably be done by the test.
//
// example:
//
//	b := TestBrowser(t, withSkipClose())
func withSkipClose() func(*testBrowser) {
	return func(tb *testBrowser) { tb.skipClose = true }
}

// NewPage is a wrapper around Browser.NewPage that fails the test if an
// error occurs. Added this helper to avoid boilerplate code in tests.
func (b *testBrowser) NewPage(opts *common.BrowserContextOptions) *common.Page {
	b.t.Helper()

	p, err := b.Browser.NewPage(opts)
	require.NoError(b.t, err)

	return p
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
func (b *testBrowser) runtime() *sobek.Runtime { return b.vu.Runtime() }

// toSobekValue converts a value to sobek value.
func (b *testBrowser) toSobekValue(i any) sobek.Value { return b.runtime().ToValue(i) }

// runJavaScript in the sobek runtime.
func (b *testBrowser) runJavaScript(s string, args ...any) (sobek.Value, error) { //nolint:unparam
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

// convert is a helper function to convert any value to a given type.
// returns a pointer to the converted value for convenience.
//
// underneath, it uses json.Marshal and json.Unmarshal to do the conversion.
func convert[T any](tb testing.TB, from any, to *T) *T {
	tb.Helper()
	buf, err := json.Marshal(from)
	require.NoError(tb, err)
	require.NoError(tb, json.Unmarshal(buf, to))
	return to
}

// asBool asserts that v is a boolean and returns v as a boolean.
func asBool(tb testing.TB, v any) bool {
	tb.Helper()
	require.IsType(tb, true, v)
	b, ok := v.(bool)
	require.True(tb, ok)
	return b
}

// asString asserts that v is a boolean and returns v as a boolean.
func asString(tb testing.TB, v any) string {
	tb.Helper()
	require.IsType(tb, "", v)
	s, ok := v.(string)
	require.True(tb, ok)
	return s
}
