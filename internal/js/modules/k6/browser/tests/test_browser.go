package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"go.k6.io/k6/internal/js/modules/k6/browser/browser"
	"go.k6.io/k6/internal/js/modules/k6/browser/chromium"
	"go.k6.io/k6/internal/js/modules/k6/browser/common"
	"go.k6.io/k6/internal/js/modules/k6/browser/env"
	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext"
	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext/k6test"

	k6httpmultibin "go.k6.io/k6/internal/lib/testutils/httpmultibin"
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

	logSystemResources(tb, "before browser launch", 0)
	b, pid, err := tbr.browserType.Launch(context.Background(), tbr.vu.Context())
	if err != nil {
		logSystemResources(tb, "after browser launch failure", 0)
		tb.Fatalf("testBrowser: %v", err)
	}
	tbr.Browser = b
	tbr.ctx = tbr.browserType.Ctx
	tbr.pid = pid
	tbr.wsURL = b.WsURL()
	logSystemResources(tb, "after browser launch", pid)
	tb.Cleanup(func() {
		if !tbr.skipClose {
			b.Close()
		}
		logSystemResources(tb, "after browser close", pid)
	})

	return tbr
}

// newTestBrowserVU initializes a new VU for browser testing.
// It returns the VU and a cancel function to stop the VU.
// VU contains the context with the custom metrics registry.
func newTestBrowserVU(tb testing.TB, tbr *testBrowser) (_ *k6test.VU, cancel func()) {
	tb.Helper()

	vu := k6test.NewVU(tb, k6test.WithSamples(tbr.samples))
	metricsCtx := k6ext.WithCustomMetrics(
		vu.Context(),
		k6ext.RegisterCustomMetrics(k6metrics.NewRegistry()),
	)
	ctx, cancel := context.WithCancel(metricsCtx)
	tb.Cleanup(cancel)
	vu.CtxField = ctx
	vu.InitEnvField.LookupEnv = tbr.lookupFunc

	mod := browser.New().NewModuleInstance(vu)
	jsMod, ok := mod.Exports().Default.(*browser.JSModule)
	require.Truef(tb, ok, "unexpected default mod export type %T", mod.Exports().Default)
	// Setting the mapped browser into the vu's sobek runtime.
	require.NoError(tb, vu.Runtime().Set("browser", jsMod.Browser))

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

// withIFrameContent sets up a handler for /iframe that serves a page embedding
// an iframe with the given content.
func withIFrameContent(iframeHTML string, iframeID string) func(*testBrowser) {
	return func(tb *testBrowser) {
		if !tb.isBrowserTypeInitialized {
			return
		}
		if tb.http == nil {
			apply := withHTTPServer()
			apply(tb)
		}

		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, err := w.Write([]byte(iframeHTML))
			require.NoError(tb.t, err)
		})
		srv := httptest.NewServer(mux)
		tb.t.Cleanup(func() {
			srv.Close()
		})

		tb.withIFrameURL(srv.URL, iframeID)
	}
}

// withIFrameURL sets up a handler for /iframe that serves a page embedding
// an iframe with the given URL.
func (tb *testBrowser) withIFrameURL(iframeURL string, iframeID string) {
	tb.t.Helper()

	if tb.http == nil {
		tb.t.Fatalf("You should enable HTTP test server, see: withHTTPServer option")
	}

	docHTML := fmt.Sprintf(`<!DOCTYPE html>
		<html>
		<head></head>
		<body>
			<iframe id="%s" src="%s"></iframe>
		</body>
		</html>`, iframeID, iframeURL)

	tb.withHandler("/iframe", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, err := w.Write([]byte(docHTML))
		require.NoError(tb.t, err)
	})
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

// GotoNewPage is a wrapper around testBrowser.NewPage and Page.Goto that fails
// the test if an error occurs. Added this helper to avoid boilerplate code in tests.
func (b *testBrowser) GotoNewPage(url string) *common.Page {
	b.t.Helper()

	p := b.NewPage(nil)
	opts := &common.FrameGotoOptions{
		Timeout: common.DefaultTimeout,
	}
	_, err := p.Goto(url, opts)
	require.NoError(b.t, err)

	return p
}

// GotoPage is a wrapper around Page.Goto that fails the test if an error occurs.
// Added this helper to avoid boilerplate code in tests.
func (b *testBrowser) GotoPage(p *common.Page, url string) {
	b.t.Helper()

	opts := &common.FrameGotoOptions{
		Timeout: common.DefaultTimeout,
	}
	_, err := p.Goto(url, opts)
	require.NoError(b.t, err)
}

// NewPage is a wrapper around Browser.NewPage that fails the test if an
// error occurs. Added this helper to avoid boilerplate code in tests.
func (b *testBrowser) NewPage(opts *common.BrowserContextOptions) *common.Page {
	b.t.Helper()

	logSystemResources(b.t, "before NewPage", b.pid)
	p, err := b.Browser.NewPage(opts)
	if err != nil {
		logSystemResources(b.t, "after NewPage failure", b.pid)
		require.NoError(b.t, err)
	}
	logSystemResources(b.t, "after NewPage success", b.pid)

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

// LogSystemResources logs CPU and RAM usage information.
// This can be called manually from tests to log resources at specific points.
func (b *testBrowser) LogSystemResources(label string) {
	logSystemResources(b.t, label, b.pid)
}

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

// toPtr is a helper function to convert a value to a pointer.
func toPtr[T any](v T) *T {
	return &v
}

// logSystemResources logs CPU and RAM usage information to help diagnose resource constraints.
// This is useful for debugging test failures related to resource exhaustion.
// Logging is enabled when K6_BROWSER_TEST_LOG_RESOURCES environment variable is set to "1" or "true".
// browserPID is the process ID of the Chrome browser instance (0 if not available).
func logSystemResources(tb testing.TB, label string, browserPID int) {
	tb.Helper()

	// Only log if explicitly enabled via environment variable
	if os.Getenv("K6_BROWSER_TEST_LOG_RESOURCES") != "1" && os.Getenv("K6_BROWSER_TEST_LOG_RESOURCES") != "true" {
		return
	}

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	tb.Logf("=== System Resources [%s] ===", label)
	tb.Logf("Go Runtime Memory:")
	tb.Logf("  Alloc: %d MB (allocated heap objects)", memStats.Alloc/1024/1024)
	tb.Logf("  TotalAlloc: %d MB (total bytes allocated)", memStats.TotalAlloc/1024/1024)
	tb.Logf("  Sys: %d MB (total memory obtained from OS)", memStats.Sys/1024/1024)
	tb.Logf("  NumGC: %d (number of GC cycles)", memStats.NumGC)
	tb.Logf("  NumGoroutine: %d", runtime.NumGoroutine())

	// Log Chrome browser process resources if PID is available
	if browserPID > 0 {
		tb.Logf("Chrome Browser Process (PID: %d):", browserPID)
		switch runtime.GOOS {
		case "linux":
			logChromeProcessLinux(tb, browserPID)
		case "darwin":
			logChromeProcessDarwin(tb, browserPID)
		case "windows":
			logChromeProcessWindows(tb, browserPID)
		}
	}

	// Log system-wide memory and CPU usage
	switch runtime.GOOS {
	case "linux":
		logLinuxResources(tb)
	case "darwin":
		logDarwinResources(tb)
	case "windows":
		logWindowsResources(tb)
	default:
		tb.Logf("System resources: OS %s not supported for detailed resource logging", runtime.GOOS)
	}
	tb.Logf("=== End System Resources [%s] ===", label)
}

func logChromeProcessLinux(tb testing.TB, pid int) {
	tb.Helper()

	// Get process stats from /proc/[pid]/stat
	statPath := fmt.Sprintf("/proc/%d/stat", pid)
	if data, err := os.ReadFile(statPath); err == nil {
		fields := strings.Fields(string(data))
		if len(fields) >= 22 {
			// Field 14 (index 13): utime - CPU time spent in user mode (clock ticks)
			// Field 15 (index 14): stime - CPU time spent in kernel mode (clock ticks)
			// Field 22 (index 21): rss - Resident Set Size (pages)
			utime := fields[13]
			stime := fields[14]
			rssPages := fields[21]
			tb.Logf("    CPU time (user/kernel): %s/%s ticks", utime, stime)
			tb.Logf("    RSS: %s pages", rssPages)
		}
	}

	// Get memory info from /proc/[pid]/status
	statusPath := fmt.Sprintf("/proc/%d/status", pid)
	if data, err := os.ReadFile(statusPath); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "VmRSS:") ||
				strings.HasPrefix(line, "VmSize:") ||
				strings.HasPrefix(line, "VmPeak:") {
				tb.Logf("    %s", strings.TrimSpace(line))
			}
		}
	}

	// Get CPU and memory usage using ps
	if cmd := exec.Command("ps", "-p", fmt.Sprintf("%d", pid), "-o", "pid,%cpu,%mem,rss,vsz,time"); cmd != nil {
		if output, err := cmd.Output(); err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.TrimSpace(line) != "" {
					tb.Logf("    %s", strings.TrimSpace(line))
				}
			}
		}
	}
}

func logLinuxResources(tb testing.TB) {
	tb.Helper()

	// Get memory info from /proc/meminfo
	if data, err := os.ReadFile("/proc/meminfo"); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "MemTotal:") ||
				strings.HasPrefix(line, "MemAvailable:") ||
				strings.HasPrefix(line, "MemFree:") {
				tb.Logf("  %s", strings.TrimSpace(line))
			}
		}
	}

	// Get CPU load average
	if data, err := os.ReadFile("/proc/loadavg"); err == nil {
		tb.Logf("  Load Average: %s", strings.TrimSpace(string(data)))
	}

	// Try to get top output for current process CPU usage
	if cmd := exec.Command("top", "-bn1", "-p", fmt.Sprintf("%d", os.Getpid())); cmd != nil {
		if output, err := cmd.Output(); err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.Contains(line, "%CPU") || strings.Contains(line, fmt.Sprintf("%d", os.Getpid())) {
					tb.Logf("  %s", strings.TrimSpace(line))
				}
			}
		}
	}
}

func logChromeProcessDarwin(tb testing.TB, pid int) {
	tb.Helper()

	// Get process info using ps
	if cmd := exec.Command("ps", "-p", fmt.Sprintf("%d", pid), "-o", "pid,%cpu,%mem,rss,vsz,time"); cmd != nil {
		if output, err := cmd.Output(); err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.TrimSpace(line) != "" {
					tb.Logf("    %s", strings.TrimSpace(line))
				}
			}
		}
	}

	// Get detailed process info using top
	if cmd := exec.Command("top", "-l", "1", "-pid", fmt.Sprintf("%d", pid)); cmd != nil {
		if output, err := cmd.Output(); err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.Contains(line, fmt.Sprintf("%d", pid)) {
					tb.Logf("    %s", strings.TrimSpace(line))
				}
			}
		}
	}
}

func logDarwinResources(tb testing.TB) {
	tb.Helper()

	// Get memory info using vm_stat
	if cmd := exec.Command("vm_stat"); cmd != nil {
		if output, err := cmd.Output(); err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.Contains(line, "Pages free") ||
					strings.Contains(line, "Pages active") ||
					strings.Contains(line, "Pages inactive") ||
					strings.Contains(line, "Pages speculative") {
					tb.Logf("  %s", strings.TrimSpace(line))
				}
			}
		}
	}

	// Get CPU info using top
	if cmd := exec.Command("top", "-l", "1", "-pid", fmt.Sprintf("%d", os.Getpid())); cmd != nil {
		if output, err := cmd.Output(); err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.Contains(line, "CPU usage") || strings.Contains(line, fmt.Sprintf("%d", os.Getpid())) {
					tb.Logf("  %s", strings.TrimSpace(line))
				}
			}
		}
	}
}

func logChromeProcessWindows(tb testing.TB, pid int) {
	tb.Helper()

	// Get process memory and CPU info using wmic
	if cmd := exec.Command("wmic", "process", "where", fmt.Sprintf("ProcessId=%d", pid), "get", "ProcessId,PageFileUsage,WorkingSetSize,PercentProcessorTime", "/format:list"); cmd != nil {
		if output, err := cmd.Output(); err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "PageFileUsage=") ||
					strings.HasPrefix(trimmed, "WorkingSetSize=") ||
					strings.HasPrefix(trimmed, "PercentProcessorTime=") {
					tb.Logf("    %s", trimmed)
				}
			}
		}
	}

	// Alternative: use tasklist for memory info
	if cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH"); cmd != nil {
		if output, err := cmd.Output(); err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.TrimSpace(line) != "" {
					tb.Logf("    %s", strings.TrimSpace(line))
				}
			}
		}
	}
}

func logWindowsResources(tb testing.TB) {
	tb.Helper()

	// Get memory info using wmic
	if cmd := exec.Command("wmic", "OS", "get", "TotalVisibleMemorySize,FreePhysicalMemory", "/format:list"); cmd != nil {
		if output, err := cmd.Output(); err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.Contains(line, "TotalVisibleMemorySize") ||
					strings.Contains(line, "FreePhysicalMemory") {
					tb.Logf("  %s", strings.TrimSpace(line))
				}
			}
		}
	}

	// Get CPU info using wmic
	if cmd := exec.Command("wmic", "cpu", "get", "LoadPercentage", "/format:list"); cmd != nil {
		if output, err := cmd.Output(); err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.Contains(line, "LoadPercentage") {
					tb.Logf("  %s", strings.TrimSpace(line))
				}
			}
		}
	}
}
