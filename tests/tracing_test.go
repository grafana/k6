package tests

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/embedded"

	"github.com/grafana/xk6-browser/browser"
	"github.com/grafana/xk6-browser/k6ext/k6test"
	browsertrace "github.com/grafana/xk6-browser/trace"

	k6lib "go.k6.io/k6/lib"
)

const html = `
<!DOCTYPE html>
<html>

<head>
    <title>Clickable link test</title>
</head>

<body>
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
			fmt.Fprint(w, html)
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
			name: "page.reload",
			js: `await Promise.all([
					page.waitForNavigation(),
					page.reload(),
			  	]);`,
			spans: []string{
				"page.reload",
				"page.waitForNavigation",
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

	mu    sync.Mutex
	spans map[string]struct{}
}

func (m *mockTracer) Start(
	ctx context.Context, spanName string, opts ...trace.SpanStartOption,
) (context.Context, trace.Span) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.spans[spanName] = struct{}{}

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
