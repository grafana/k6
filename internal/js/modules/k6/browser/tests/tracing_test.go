package tests

import (
	"context"
	"encoding/binary"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/embedded"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/browser"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/k6ext/k6test"
	browsertrace "go.k6.io/k6/v2/internal/js/modules/k6/browser/trace"

	k6lib "go.k6.io/k6/v2/lib"
	k6metrics "go.k6.io/k6/v2/metrics"
)

const html = `
<!DOCTYPE html>
<html>

<head>
    <title>Clickable link test</title>
</head>

<body>
	<a id="top" href="#bottom">Go to bottom</a>
	<div class="main">
		<h3>Click Counter</h3>
		<button id="clickme">Click me: 0</button>
		<h3>Type input</h3>
		<input type="text" id="typeme">
	</div>
    <script>
	var button = document.getElementById("clickme"),
	count = 0;
	button.onclick = function() {
		count += 1;
		button.innerHTML = "Click me: " + count;
	};
    </script>
	<div id="bottom"></div>
</body>

</html>
`

// clientRedirectHTML performs a single client-initiated redirect on load (JS
// assigning location). The query-string guard prevents a redirect loop. Chrome
// emits two consecutive main-frame frameStartedLoading events for the resulting
// document, which used to create a spurious phantom navigation span.
const clientRedirectHTML = `<!DOCTYPE html>
<html>
<head><title>redirect</title></head>
<body>landing
<script>
  if (!location.search) {
    location.replace(location.pathname + '?var=B');
  }
</script>
</body>
</html>
`

// TestTracing verifies that all methods instrumented to generate
// traces behave correctly.
func TestTracing(t *testing.T) {
	t.Parallel()

	// Init tracing mocks
	tracer := &mockTracer{
		spans: make(map[string]struct{}),
	}
	tp := &mockTracerProvider{
		tracer: tracer,
	}
	// Start test server
	ts := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			_, err := fmt.Fprint(w, html)
			require.NoError(t, err)
		},
	))
	defer ts.Close()

	// Initialize VU and browser module
	vu := k6test.NewVU(t, k6test.WithTracerProvider(tp))

	rt := vu.Runtime()
	root := browser.New()
	mod := root.NewModuleInstance(vu)
	jsMod, ok := mod.Exports().Default.(*browser.JSModule)
	require.Truef(t, ok, "unexpected default mod export type %T", mod.Exports().Default)
	require.NoError(t, rt.Set("browser", jsMod.Browser))
	vu.ActivateVU()

	// Run the test
	vu.StartIteration(t)
	require.NoError(t, tracer.verifySpans("iteration"))
	setupTestTracing(t, rt)

	// NOTE: These tests run sequentially, not in parallel. This has been done
	// on purpose.
	testCases := []struct {
		name  string
		js    string
		spans []string
	}{
		{
			name: "browser.newPage",
			js:   "page = await browser.newPage()",
			spans: []string{
				"browser.newPage",
				"browser.newContext",
				"browserContext.newPage",
			},
		},
		{
			name: "page.goto",
			js:   fmt.Sprintf("page.goto('%s')", ts.URL),
			spans: []string{
				"page.goto",
				"navigation",
			},
		},
		{
			name: "page.screenshot",
			js:   "page.screenshot();",
			spans: []string{
				"page.screenshot",
			},
		},
		{
			name: "locator.click",
			js:   "page.locator('#clickme').click();",
			spans: []string{
				"locator.click",
			},
		},
		{
			name: "locator.type",
			js:   "page.locator('input#typeme').type('test');",
			spans: []string{
				"locator.type",
			},
		},
		{
			// In this test, since we're using waitForNavigation, which uses a
			// taskqueue, we need to manually clear it by calling page.close.
			// During normal testing runtime it would be cleared when we receive
			// a iterationEnd event from k6.
			name: "page.reload",
			js: `await Promise.all([
					page.waitForNavigation(),
					page.reload(),
				]);
				await page.close();`,
			spans: []string{
				"page.reload",
				"page.waitForNavigation",
				"page.close",
			},
		},
		{
			name: "page.waitForTimeout",
			js: `page = await browser.context().newPage();
				 await page.waitForTimeout(10);`,
			spans: []string{
				"browserContext.newPage",
				"page.waitForTimeout",
			},
		},
		{
			name: "web_vital",
			js:   "page.close();", // on page.close, web vitals are collected and fired/received.
			spans: []string{
				"web_vital",
				"page.close",
			},
		},
	}

	// Each sub test depends on the previous sub test, so they cannot be ran
	// in parallel.
	for _, tc := range testCases {
		assertJSInEventLoop(t, vu, tc.js)

		require.NoError(t, tracer.verifySpans(tc.spans...))
	}
}

// This test is testing to ensure that correct number of navigation spans are created
// and they are created in the correct order.
func TestNavigationSpanCreation(t *testing.T) {
	t.Parallel()
	setup := func(t *testing.T) (*mockTracer, *httptest.Server, *k6test.VU) {
		t.Helper()
		tracer := &mockTracer{
			spans: make(map[string]struct{}),
		}
		tp := &mockTracerProvider{
			tracer: tracer,
		}
		// Start test server
		ts := httptest.NewServer(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				_, err := fmt.Fprint(w, html)
				require.NoError(t, err)
			},
		))
		t.Cleanup(ts.Close)

		// Initialize VU and browser module
		vu := k6test.NewVU(t, k6test.WithTracerProvider(tp))

		rt := vu.Runtime()
		root := browser.New()
		mod := root.NewModuleInstance(vu)
		jsMod, ok := mod.Exports().Default.(*browser.JSModule)
		require.Truef(t, ok, "unexpected default mod export type %T", mod.Exports().Default)
		require.NoError(t, rt.Set("browser", jsMod.Browser))
		vu.ActivateVU()
		return tracer, ts, vu
	}

	testCases := []struct {
		name     string
		js       string
		expected []string
	}{
		{
			name: "goto",
			js: `
				page = await browser.newPage();
				await page.goto('%s', {waitUntil:'networkidle'});
				await page.close();
				`,
			expected: []string{
				"iteration",
				"browser.newPage",
				"browser.newContext",
				"browserContext.newPage",
				"navigation", // created when a new page is created
				"page.goto",
				"navigation", // created when a navigation occurs after goto
				"page.close",
			},
		},
		{
			name: "reload",
			js: `
				page = await browser.newPage();
				await page.goto('%s', {waitUntil:'networkidle'});
				await page.reload({waitUntil:'networkidle'});
				await page.close();
				`,
			expected: []string{
				"iteration",
				"browser.newPage",
				"browser.newContext",
				"browserContext.newPage",
				"navigation", // created when a new page is created
				"page.goto",
				"navigation", // created when a navigation occurs after goto
				"page.reload",
				"navigation", // created when a navigation occurs after reload
				"page.close",
			},
		},
		{
			name: "go_back",
			js: `
				page = await browser.newPage();
				await page.goto('%s', {waitUntil:'networkidle'});
				await page.goBack();
				await page.close();
				`,
			expected: []string{
				"iteration",
				"browser.newPage",
				"browser.newContext",
				"browserContext.newPage",
				"navigation", // created when a new page is created
				"page.goto",
				"navigation", // created when a navigation occurs after goto
				"page.goBack",
				"navigation", // created when going back to the previous page
				"page.close",
			},
		},
		{
			name: "same_page_navigation",
			js: `
				page = await browser.newPage();
				await page.goto('%s', {waitUntil:'networkidle'});
				await Promise.all([
					page.waitForNavigation(),
					page.locator('a[id=\"top\"]').click(),
				]);
				await page.close();
				`,
			expected: []string{
				"iteration",
				"browser.newPage",
				"browser.newContext",
				"browserContext.newPage",
				"navigation", // created when a new page is created
				"page.goto",
				"navigation", // created when a navigation occurs after goto
				"page.waitForNavigation",
				"locator.click",
				"navigation", // created when navigating within the same page
				"page.close",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Init tracing mocks
			tracer, ts, vu := setup(t)
			// Run the test
			vu.StartIteration(t)
			defer vu.EndIteration(t)

			assertJSInEventLoop(t, vu, fmt.Sprintf(tc.js, ts.URL))

			got := tracer.cloneOrderedSpans()
			// We can't use assert.Equal since the order of the span creation
			// changes slightly on every test run. Instead we're going to make
			// sure that the slice matches but not the order.
			assert.ElementsMatch(t, tc.expected, got, fmt.Sprintf("%s failed", tc.name))
		})
	}
}

// TestNavigationSpanPhantomOnClientRedirect asserts that a client-initiated
// redirect does not create a spurious phantom navigation span. Navigating to a
// page that redirects itself once should yield exactly one navigation span per
// committed main-frame document plus the initial about:blank: about:blank, the
// landing document, and the redirect target (3), not 4.
func TestNavigationSpanPhantomOnClientRedirect(t *testing.T) {
	t.Parallel()

	tracer := &mockTracer{spans: make(map[string]struct{})}
	tp := &mockTracerProvider{tracer: tracer}

	ts := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, err := fmt.Fprint(w, clientRedirectHTML)
			require.NoError(t, err)
		},
	))
	t.Cleanup(ts.Close)

	vu := k6test.NewVU(t, k6test.WithTracerProvider(tp))
	rt := vu.Runtime()
	root := browser.New()
	mod := root.NewModuleInstance(vu)
	jsMod, ok := mod.Exports().Default.(*browser.JSModule)
	require.Truef(t, ok, "unexpected default mod export type %T", mod.Exports().Default)
	require.NoError(t, rt.Set("browser", jsMod.Browser))
	vu.ActivateVU()
	vu.StartIteration(t)
	defer vu.EndIteration(t)

	js := fmt.Sprintf(`
		page = await browser.newPage();
		await page.goto('%s', {waitUntil:'load'});
		await page.waitForTimeout(1500);
		await page.close();
		`, ts.URL)
	assertJSInEventLoop(t, vu, js)

	got := tracer.cloneOrderedSpans()
	navs := 0
	for _, s := range got {
		if s == "navigation" {
			navs++
		}
	}
	// about:blank + landing document + redirect target = 3. A phantom
	// rotation on the redirect's duplicate frameStartedLoading would make 4.
	assert.Equalf(t, 3, navs,
		"expected 3 navigation spans (no phantom), got %d; spans: %v", navs, got)
}

// TestNavigationSpanAfterNonCommittingLoad asserts that a main-frame load that
// never commits a document (a download or a 204 response) does not stop the
// next real navigation from rotating a new navigation span. Such loads fire
// frameStartedLoading and frameStoppedLoading but no frameNavigated.
func TestNavigationSpanAfterNonCommittingLoad(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
	}{
		{name: "download", path: "/download"},
		{name: "no_content", path: "/no-content"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tracer := &mockTracer{spans: make(map[string]struct{})}
			tp := &mockTracerProvider{tracer: tracer}

			ts := httptest.NewServer(http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/download":
						w.Header().Set("Content-Type", "application/octet-stream")
						w.Header().Set("Content-Disposition", `attachment; filename="file.bin"`)
						_, err := fmt.Fprint(w, "data")
						require.NoError(t, err)
					case "/no-content":
						w.WriteHeader(http.StatusNoContent)
					default:
						w.Header().Set("Content-Type", "text/html; charset=utf-8")
						_, err := fmt.Fprint(w, html)
						require.NoError(t, err)
					}
				},
			))
			t.Cleanup(ts.Close)

			vu := k6test.NewVU(t, k6test.WithTracerProvider(tp))
			rt := vu.Runtime()
			root := browser.New()
			mod := root.NewModuleInstance(vu)
			jsMod, ok := mod.Exports().Default.(*browser.JSModule)
			require.Truef(t, ok, "unexpected default mod export type %T", mod.Exports().Default)
			require.NoError(t, rt.Set("browser", jsMod.Browser))
			vu.ActivateVU()
			vu.StartIteration(t)
			defer vu.EndIteration(t)

			assertJSInEventLoop(t, vu, fmt.Sprintf(`
				page = await browser.newPage();
				await page.goto('%s/a', {waitUntil:'load'});
				await page.evaluate((p) => { location.href = p; }, '%s');
				await page.waitForTimeout(1000);
				`, ts.URL, tt.path))
			_ = tracer.cloneOrderedSpans()

			assertJSInEventLoop(t, vu, fmt.Sprintf(`
				await page.goto('%s/b', {waitUntil:'load'});
				await page.close();
				`, ts.URL))

			got := tracer.cloneOrderedSpans()
			assert.Containsf(t, got, "navigation",
				"navigating after a non-committing load should rotate a new navigation span; spans: %v", got)
		})
	}
}

// TestWebVitalSpanBelongsToItsPage asserts that web vitals buffered on page A
// are flushed under A's navigation span when the page navigates to B, rather
// than under B's. The correlation relies on the buffer being flushed before the
// navigation span rotates; flushing after it would attribute A's vitals to B.
func TestWebVitalSpanBelongsToItsPage(t *testing.T) {
	t.Parallel()

	tracer := &parentRecordingTracer{}
	tp := &mockTracerProvider{tracer: tracer}
	samples := make(chan k6metrics.SampleContainer, 1000)

	ts := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, err := fmt.Fprint(w, html)
			require.NoError(t, err)
		},
	))
	t.Cleanup(ts.Close)

	vu := k6test.NewVU(t, k6test.WithTracerProvider(tp), k6test.WithSamples(samples))
	rt := vu.Runtime()
	root := browser.New()
	mod := root.NewModuleInstance(vu)
	jsMod, ok := mod.Exports().Default.(*browser.JSModule)
	require.Truef(t, ok, "unexpected default mod export type %T", mod.Exports().Default)
	require.NoError(t, rt.Set("browser", jsMod.Browser))
	vu.ActivateVU()
	vu.StartIteration(t)
	defer vu.EndIteration(t)

	pageA, pageB := ts.URL+"/a", ts.URL+"/b"
	assertJSInEventLoop(t, vu, fmt.Sprintf(`
		page = await browser.newPage();
		await page.goto('%s', {waitUntil:'load'});
		await page.waitForTimeout(500);
		await page.goto('%s', {waitUntil:'load'});
		await page.waitForTimeout(500);
		await page.close();
		`, pageA, pageB))

	// The number of web vitals emitted for each page.
	want := map[string]int{}
	for _, sc := range k6metrics.GetBufferedSamples(samples) {
		for _, s := range sc.GetSamples() {
			if !strings.HasPrefix(s.Metric.Name, "browser_web_vital_") {
				continue
			}
			url, _ := s.Tags.Get("url")
			want[url]++
		}
	}
	require.Positivef(t, want[pageA], "no web vitals emitted for page A; got %v", want)
	require.Positivef(t, want[pageB], "no web vitals emitted for page B; got %v", want)

	// The number of web_vital spans parented by each page's navigation span.
	navURL := map[trace.SpanID]string{}
	for _, s := range tracer.recorded() {
		if s.name == "navigation" {
			navURL[s.id] = s.attr("navigation.url")
		}
	}
	got := map[string]int{}
	for _, s := range tracer.recorded() {
		if s.name == "web_vital" {
			got[navURL[s.parent]]++
		}
	}

	assert.Equalf(t, want[pageA], got[pageA], "page A web_vital spans; got per page %v", got)
	assert.Equalf(t, want[pageB], got[pageB], "page B web_vital spans; got per page %v", got)
}

// parentRecordingTracer records every started span with its parent span ID and
// the attributes set on it, so tests can assert which span an event belongs to.
type parentRecordingTracer struct {
	embedded.Tracer

	mu     sync.Mutex
	nextID uint64
	spans  []*recordedSpan
}

func (r *parentRecordingTracer) Start(
	ctx context.Context, spanName string, _ ...trace.SpanStartOption,
) (context.Context, trace.Span) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.nextID++
	var id trace.SpanID
	binary.BigEndian.PutUint64(id[:], r.nextID)
	s := &recordedSpan{
		name:   spanName,
		id:     id,
		parent: trace.SpanContextFromContext(ctx).SpanID(),
		sc:     trace.NewSpanContext(trace.SpanContextConfig{TraceID: trace.TraceID{1}, SpanID: id}),
		attrs:  map[string]string{},
	}
	r.spans = append(r.spans, s)

	return trace.ContextWithSpanContext(ctx, s.sc), s
}

func (r *parentRecordingTracer) recorded() []*recordedSpan {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]*recordedSpan(nil), r.spans...)
}

type recordedSpan struct {
	browsertrace.NoopSpan

	name   string
	id     trace.SpanID
	parent trace.SpanID
	sc     trace.SpanContext

	mu    sync.Mutex
	attrs map[string]string
}

func (s *recordedSpan) SpanContext() trace.SpanContext { return s.sc }

func (s *recordedSpan) SetAttributes(kv ...attribute.KeyValue) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, a := range kv {
		s.attrs[string(a.Key)] = a.Value.String()
	}
}

func (s *recordedSpan) attr(key string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.attrs[key]
}

func setupTestTracing(t *testing.T, rt *sobek.Runtime) {
	t.Helper()

	// Declare a global page var that we can use
	// throughout the test cases
	_, err := rt.RunString("var page;")
	require.NoError(t, err)

	// Set a sleep function so we can use it to wait
	// for async WebVitals processing
	err = rt.Set("sleep", func(d int) {
		time.Sleep(time.Duration(d) * time.Millisecond)
	})
	require.NoError(t, err)
}

func assertJSInEventLoop(t *testing.T, vu *k6test.VU, js string) {
	t.Helper()

	f := fmt.Sprintf(
		"test = async function() { %s; }",
		js)

	rt := vu.Runtime()
	_, err := rt.RunString(f)
	require.NoError(t, err)

	test, ok := sobek.AssertFunction(rt.Get("test"))
	require.True(t, ok)

	err = vu.Loop.Start(func() error {
		_, err := test(sobek.Undefined())
		return err
	})
	require.NoError(t, err)
}

type mockTracerProvider struct {
	k6lib.TracerProvider

	tracer trace.Tracer
}

func (m *mockTracerProvider) Tracer(
	name string, options ...trace.TracerOption,
) trace.Tracer {
	return m.tracer
}

type mockTracer struct {
	embedded.Tracer

	mu           sync.Mutex
	spans        map[string]struct{}
	orderedSpans []string
}

func (m *mockTracer) Start(
	ctx context.Context, spanName string, opts ...trace.SpanStartOption,
) (context.Context, trace.Span) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.spans[spanName] = struct{}{}

	// Ignore web_vital spans since they're non deterministic.
	if spanName != "web_vital" {
		m.orderedSpans = append(m.orderedSpans, spanName)
	}

	return ctx, browsertrace.NoopSpan{}
}

func (m *mockTracer) verifySpans(spanNames ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, sn := range spanNames {
		if _, ok := m.spans[sn]; !ok {
			return fmt.Errorf("%q span was not found", sn)
		}
		delete(m.spans, sn)
	}

	return nil
}

func (m *mockTracer) cloneOrderedSpans() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	c := make([]string, len(m.orderedSpans))
	copy(c, m.orderedSpans)

	m.orderedSpans = []string{}

	return c
}
