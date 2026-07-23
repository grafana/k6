package tests

import (
	"context"
	"sync"
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"

	k6event "go.k6.io/k6/v2/internal/event"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/browser"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/chromium"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/env"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/k6ext/k6test"
)

func TestBrowserTypeConnect(t *testing.T) {
	t.Parallel()

	// Start a test browser so we can get its WS URL
	// and use it to connect through BrowserType.Connect.
	tb := newTestBrowser(t)
	vu := k6test.NewVU(t)
	bt := chromium.NewBrowserType(vu)
	vu.ActivateVU()

	b, err := bt.Connect(context.Background(), context.Background(), tb.wsURL)
	require.NoError(t, err)
	t.Cleanup(b.Close)
	_, err = b.NewPage(nil)
	require.NoError(t, err)
}

func TestBrowserTypeConnectOverCDP(t *testing.T) {
	t.Parallel()

	// Start a test browser so we can get its WS URL
	// and connect through BrowserType.ConnectOverCDP.
	tb := newTestBrowser(t)
	vu := k6test.NewVU(t)
	bt := chromium.NewBrowserType(vu)
	vu.ActivateVU()

	b, err := bt.ConnectOverCDP(context.Background(), tb.wsURL)
	require.NoError(t, err)
	t.Cleanup(b.Close)

	_, err = b.NewPage(nil)
	require.NoError(t, err)
}

func TestBrowserTypeConnectOverCDPValidation(t *testing.T) {
	t.Parallel()

	// Invalid endpoints must fail immediately, before any connection attempt,
	// so no test browser is needed.
	vu := k6test.NewVU(t)
	bt := chromium.NewBrowserType(vu)
	vu.ActivateVU()

	for _, wsEndpoint := range []string{
		"",                      // empty (e.g. an undefined value or failed lookup)
		"   ",                   // blank
		"http://localhost:9222", // wrong scheme
		"localhost:9222",        // missing ws/wss scheme
	} {
		_, err := bt.ConnectOverCDP(context.Background(), wsEndpoint)
		require.Errorf(t, err, "endpoint %q should be rejected", wsEndpoint)
	}
}

// TestChromiumConnectOverCDPAppliesBrowserTimeout verifies that ConnectOverCDP
// parses browser options from the environment. E.g., with K6_BROWSER_TIMEOUT=1ms
// a browser operation (e.g., NewPage) must time out.
func TestChromiumConnectOverCDPAppliesBrowserTimeout(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t)
	vu := k6test.NewVU(t, env.ConstLookup(env.BrowserGlobalTimeout, "1ms"))
	bt := chromium.NewBrowserType(vu)
	vu.ActivateVU()

	b, err := bt.ConnectOverCDP(context.Background(), tb.wsURL)
	require.NoError(t, err)
	t.Cleanup(b.Close)

	_, err = b.NewPage(nil)
	require.Error(t, err, "NewPage should time out with K6_BROWSER_TIMEOUT=1ms")
}

// setupChromiumVU builds a VU with the browser module's chromium object bound
// to the "chromium" global and an iteration started, ready to run
// connectOverCDP scripts via RunAsync.
//
// It marks the scenario as remote (options.browser.remote), which is required
// for chromium.connectOverCDP and tells k6 not to launch a managed browser.
func setupChromiumVU(t *testing.T, opts ...any) *k6test.VU {
	t.Helper()

	vu := k6test.NewVU(t, opts...)
	mod := browser.New().NewModuleInstance(vu)
	jsMod, ok := mod.Exports().Default.(*browser.JSModule)
	require.Truef(t, ok, "unexpected default mod export type %T", mod.Exports().Default)

	vu.ActivateVU()
	// Gate: connectOverCDP requires options.browser.remote to be set.
	vu.StateField.Options.Scenarios["default"].GetScenarioOptions().Browser["remote"] = true
	vu.StartIteration(t)
	vu.SetVar(t, "chromium", jsMod.Chromium)

	return vu
}

// TestChromiumConnectOverCDPRequiresRemote verifies the gate: without
// options.browser.remote set, chromium.connectOverCDP is rejected up front,
// before any connection attempt (so no test browser is needed).
func TestChromiumConnectOverCDPRequiresRemote(t *testing.T) {
	t.Parallel()

	vu := k6test.NewVU(t)
	mod := browser.New().NewModuleInstance(vu)
	jsMod, ok := mod.Exports().Default.(*browser.JSModule)
	require.Truef(t, ok, "unexpected default mod export type %T", mod.Exports().Default)

	vu.ActivateVU()
	// Intentionally do NOT set options.browser.remote.
	vu.StartIteration(t)
	vu.SetVar(t, "chromium", jsMod.Chromium)

	p := vu.RunPromise(t, `
		try {
			await chromium.connectOverCDP("ws://localhost:9222");
			return "NO_REJECTION";
		} catch (e) {
			return String(e);
		}
	`)
	require.Equal(t, sobek.PromiseStateFulfilled, p.State())
	require.Contains(t, p.Result().String(), "options.browser.remote")
}

func TestChromiumConnectOverCDP(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t)
	vu := setupChromiumVU(t)
	_, err := vu.RunAsync(t, `
		const b = await chromium.connectOverCDP("%s");
		const p = await b.newPage();
		await p.close();
		await b.close();
	`, tb.wsURL)
	require.NoError(t, err)
}

// TestChromiumConnectOverCDPConcurrent is a regression test for a data race
// where two concurrent chromium.connectOverCDP calls in the same iteration
// used to race.
func TestChromiumConnectOverCDPConcurrent(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t)
	vu := setupChromiumVU(t)
	_, err := vu.RunAsync(t, `
		await Promise.all([
			chromium.connectOverCDP("%s"),
			chromium.connectOverCDP("%s"),
		].map(async (p) => {
			const b = await p;
			const pg = await b.newPage();
			await pg.close();
			await b.close();
		}));
	`, tb.wsURL, tb.wsURL)
	require.NoError(t, err)
}

func TestChromiumConnectOverCDPAutoClose(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t)
	vu := setupChromiumVU(t)
	_, err := vu.RunAsync(t, `
		const b = await chromium.connectOverCDP("%s");
		const p = await b.newPage();
		await p.close();
		if (!b.isConnected()) {
			throw new Error("expected browser to be connected before iteration end");
		}
		// Stash the browser so we can check its state after the iteration
		// ends, once the registry sweep has had a chance to run.
		globalThis.__cdpBrowser = b;
		// Intentionally NOT calling b.close().
	`, tb.wsURL)
	require.NoError(t, err)

	// End the iteration; the registry sweep must close the connected browser.
	vu.EndIteration(t)

	// Verify the sweep actually closed the browser, instead of just trusting
	// that EndIteration didn't error out.
	got := vu.RunPromise(t, "return globalThis.__cdpBrowser.isConnected();")
	require.False(t, got.Result().ToBoolean(), "expected browser to be disconnected after iteration end")
}

func TestChromiumConnectOverCDPExplicitClose(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t)
	vu := setupChromiumVU(t)
	_, err := vu.RunAsync(t, `
		const b = await chromium.connectOverCDP("%s");
		const p = await b.newPage();
		await p.close();
		await b.close();
	`, tb.wsURL)
	require.NoError(t, err)

	// Ending the iteration after an explicit close must not panic or
	// double-close (untrack removed it from the sweep set).
	vu.EndIteration(t)
}

// TestChromiumConnectOverCDPExitSweep verifies that a connected browser left
// open when the test exits (e.g., SIGTERM mid-iteration) is closed by the
// registry's Exit sweep (clear), not only by the per-iteration IterEnd sweep.
func TestChromiumConnectOverCDPExitSweep(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t)
	vu := setupChromiumVU(t)
	_, err := vu.RunAsync(t, `
		const b = await chromium.connectOverCDP("%s");
		const p = await b.newPage();
		await p.close();
		// Stash the browser so we can check its state after the Exit sweep,
		// and intentionally do NOT close it or end the iteration.
		globalThis.__cdpBrowser = b;
	`, tb.wsURL)
	require.NoError(t, err)

	// Fire the global Exit event, as k6 does on process exit / SIGTERM. The
	// registry's clear() must close any still-connected browsers.
	events, ok := vu.EventsField.Global.(*k6event.System)
	require.True(t, ok, "want *k6event.System; got %T", events)
	waitDone := events.Emit(&k6event.Event{Type: k6event.Exit})
	require.NoError(t, waitDone(context.Background()), "error waiting on Exit done")

	// Verify the Exit sweep actually closed the connected browser.
	got := vu.RunPromise(t, "return globalThis.__cdpBrowser.isConnected();")
	require.False(t, got.Result().ToBoolean(), "expected browser to be disconnected after Exit sweep")
}

// spanInfo is a minimal snapshot of a recorded span.
type spanInfo struct {
	name     string
	traceID  oteltrace.TraceID
	spanID   oteltrace.SpanID
	parentID oteltrace.SpanID
	attrs    []attribute.KeyValue
}

// spanRecorder is a minimal sdktrace.SpanProcessor that records started spans
// (for parent/attribute inspection) and which span IDs have ended.
type spanRecorder struct {
	mu      sync.Mutex
	started []spanInfo
	ended   map[oteltrace.SpanID]bool
}

func newSpanRecorder() *spanRecorder {
	return &spanRecorder{ended: make(map[oteltrace.SpanID]bool)}
}

func (r *spanRecorder) OnStart(_ context.Context, s sdktrace.ReadWriteSpan) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.started = append(r.started, spanInfo{
		name:     s.Name(),
		traceID:  s.SpanContext().TraceID(),
		spanID:   s.SpanContext().SpanID(),
		parentID: s.Parent().SpanID(),
		attrs:    s.Attributes(),
	})
}

func (r *spanRecorder) OnEnd(s sdktrace.ReadOnlySpan) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ended[s.SpanContext().SpanID()] = true
}

func (r *spanRecorder) Shutdown(context.Context) error   { return nil }
func (r *spanRecorder) ForceFlush(context.Context) error { return nil }

func (r *spanRecorder) find(name string) (spanInfo, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range r.started {
		if s.name == name {
			return s, true
		}
	}
	return spanInfo{}, false
}

func (r *spanRecorder) isEnded(id oteltrace.SpanID) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ended[id]
}

func spanAttrInt64(t *testing.T, s spanInfo, key string) int64 {
	t.Helper()
	for _, kv := range s.attrs {
		if string(kv.Key) == key {
			return kv.Value.AsInt64()
		}
	}
	t.Fatalf("attribute %q not found on span %q", key, s.name)
	return 0
}

// TestChromiumConnectOverCDPTraceLinkage verifies that a call made over a
// connectOverCDP browser produces a span parented under an iteration root
// span keyed by the real iteration number, and that root span is
// ended at IterEnd.
// It uses a recording tracer provider to inspect the span tree.
func TestChromiumConnectOverCDPTraceLinkage(t *testing.T) {
	t.Parallel()

	rec := newSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))

	tb := newTestBrowser(t)
	vu := setupChromiumVU(t, k6test.WithTracerProvider(tp))
	wantIter := vu.State().Iteration

	_, err := vu.RunAsync(t, `
		const b = await chromium.connectOverCDP("%s");
		const p = await b.newPage();
		await p.close();
		await b.close();
	`, tb.wsURL)
	require.NoError(t, err)

	// End the iteration so the iteration root span is closed.
	vu.EndIteration(t)

	iterSpan, ok := rec.find("iteration")
	require.True(t, ok, "expected an 'iteration' root span")
	newPageSpan, ok := rec.find("browser.newPage")
	require.True(t, ok, "expected a 'browser.newPage' span")

	// The browser API span must be parented under the iteration trace that
	// connectOverCDP established. If startConnectTrace did not wire the tracer
	// into the browser's context, browser.newPage would be a noop/orphaned span
	// and these assertions would fail.
	require.Equal(t, iterSpan.traceID, newPageSpan.traceID,
		"browser.newPage span should be in the same trace as the iteration span")
	require.Equal(t, iterSpan.spanID, newPageSpan.parentID,
		"browser.newPage span should be parented under the iteration span")

	// The iteration span must be keyed by the real iteration number and ended
	// when the iteration ends.
	require.Equal(t, wantIter, spanAttrInt64(t, iterSpan, "test.iteration.number"))
	require.True(t, rec.isEnded(iterSpan.spanID), "iteration span should be ended at IterEnd")
}

func TestBrowserTypeLaunchToConnect(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t)
	bp := newTestBrowserProxy(t, tb)

	// Export WS URL env var
	// pointing to test browser proxy
	vu := k6test.NewVU(t, env.ConstLookup(env.WebSocketURLs, bp.wsURL()))

	// We have to call launch method through JS API in sobek
	// to take mapping layer into account, instead of calling
	// BrowserType.Launch method directly
	root := browser.New()
	mod := root.NewModuleInstance(vu)
	jsMod, ok := mod.Exports().Default.(*browser.JSModule)
	require.Truef(t, ok, "unexpected default mod export type %T", mod.Exports().Default)

	vu.ActivateVU()
	vu.StartIteration(t)

	vu.SetVar(t, "browser", jsMod.Browser)
	_, err := vu.RunAsync(t, `
		const p = await browser.newPage();
		await p.close();
	`)
	require.NoError(t, err)

	// Verify the proxy, which's WS URL was set as
	// K6_BROWSER_WS_URL, has received a connection req
	require.True(t, bp.connected)
	// Verify that no new process pids have been added
	// to pid registry
	require.Len(t, root.PidRegistry.Pids(), 0)
}
