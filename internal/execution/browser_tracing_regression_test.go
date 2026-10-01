package execution_test

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"go.k6.io/k6/v2/internal/event"
	"go.k6.io/k6/v2/internal/execution"
	"go.k6.io/k6/v2/internal/execution/local"
	"go.k6.io/k6/v2/internal/js"
	k6trace "go.k6.io/k6/v2/internal/lib/trace"
	"go.k6.io/k6/v2/internal/loader"
	moduletrace "go.k6.io/k6/v2/lib/trace"
	"go.k6.io/k6/v2/metrics"
)

// TestBrowserSpansSurviveLateTracingResolution guards against a regression
// where the browser module's own operation spans (browser.newContext,
// page.goto, etc.) silently stopped being created under --tracing=browser.
//
// The real k6 CLI (internal/cmd/run.go) loads/compiles the script -- which,
// for a script that imports k6/browser, triggers the browser module's
// NewModuleInstance once as part of module-graph evaluation -- BEFORE the
// resolved --tracing value is attached to the test's TestPreInitState. The
// browser module used to cache its tracing-enabled flag on that very first
// NewModuleInstance call, permanently latching it to disabled regardless of
// the flag. Every other span kind (k6.run, k6.scenario, k6.vu, iteration) is
// resolved later, at VU-activation/iteration time, so they were unaffected
// and this went unnoticed until only browser's own spans were missing.
//
// This test reproduces that exact ordering: build the runner (which loads
// the script) BEFORE setting TestPreInitState.Tracing, exactly like
// internal/cmd/run.go does, then runs the test and asserts the browser spans
// are present as real children of the iteration span.
func TestBrowserSpansSurviveLateTracingResolution(t *testing.T) {
	t.Parallel()

	script := `
		import { browser } from 'k6/browser';
		export const options = {
			scenarios: {
				ui: {
					executor: 'shared-iterations',
					iterations: 1,
					vus: 1,
					options: { browser: { type: 'chromium' } },
				},
			},
		};
		export default async function () {
			const context = await browser.newContext();
			const page = await context.newPage();
			await page.goto('about:blank');
			await page.close();
		}
	`

	piState := getTestPreInitState(t)
	piState.LookupEnv = func(string) (string, bool) { return "", false }
	piState.Events = event.NewEventSystem(100, piState.Logger)

	exporter := &tracingRecordingExporter{}
	sdkProvider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	piState.TracerProvider = &k6trace.TracerProvider{TracerProvider: sdkProvider}

	// Load the script first, mirroring internal/cmd/run.go's
	// c.loadConfiguredTest(cmd, args) call: this is what triggers the
	// browser module's first NewModuleInstance, before Tracing is resolved.
	sourceData := &loader.SourceData{URL: &url.URL{Path: "/script.js"}, Data: []byte(script)}
	moduleResolver := js.NewModuleResolver(loader.Dir(sourceData.URL), piState, nil)
	runner, err := js.New(piState, sourceData, nil, moduleResolver)
	require.NoError(t, err)

	// Only now does the resolved --tracing value get attached, exactly like
	// internal/cmd/run.go's test.preInitState.Tracing = tracingSet, which
	// happens after loadConfiguredTest returns.
	tracingSet, err := moduletrace.ParseSet("browser")
	require.NoError(t, err)
	piState.Tracing = tracingSet

	testRunState := getTestRunState(t, piState, runner.GetOptions(), runner)

	execScheduler, err := execution.NewScheduler(testRunState, local.NewController())
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	samples := make(chan metrics.SampleContainer, 200)
	go func() {
		for {
			select {
			case <-samples:
			case <-ctx.Done():
				return
			}
		}
	}()

	stopEmission, err := execScheduler.Init(ctx, samples)
	require.NoError(t, err)
	t.Cleanup(func() {
		stopEmission()
		close(samples)
	})

	runCtx, runSpan := sdkProvider.Tracer("test").Start(ctx, "k6.run")
	require.NoError(t, execScheduler.Run(ctx, runCtx, samples))
	runSpan.End()
	require.NoError(t, sdkProvider.Shutdown(context.Background()))

	iteration := exporter.findByName("iteration")
	require.NotNil(t, iteration, "expected an iteration span to have been exported")

	for _, name := range []string{"browser.newContext", "browserContext.newPage", "page.goto", "page.close"} {
		span := exporter.findByName(name)
		require.NotNil(t, span, "expected a %q span to have been exported", name)
		require.True(t, span.Parent().IsValid())
		require.Equal(t, iteration.SpanContext().TraceID(), span.SpanContext().TraceID(),
			"%q should be in the same trace as the iteration span", name)
	}
}
