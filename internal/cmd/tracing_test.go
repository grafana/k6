package cmd

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	k6trace "go.k6.io/k6/v2/internal/lib/trace"
	"go.k6.io/k6/v2/lib"
	moduletrace "go.k6.io/k6/v2/lib/trace"
)

func TestTracingCanBeEnabledWithoutOutput(t *testing.T) {
	t.Parallel()

	provider, ctx, _, tracingSet, err := newTracerProvider(t.Context(), lib.RuntimeOptions{
		Tracing:      null.StringFrom("http"),
		TracesOutput: null.StringFrom("none"),
	})
	require.NoError(t, err)
	require.True(t, tracingSet.Enabled("http"))
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })

	_, span := provider.Tracer("test").Start(ctx, "span")
	require.True(t, span.SpanContext().IsValid())
	require.True(t, span.SpanContext().IsSampled())
	span.End()
}

func TestTracingDisabledUsesNoopProvider(t *testing.T) {
	t.Parallel()

	provider, ctx, _, tracingSet, err := newTracerProvider(t.Context(), lib.RuntimeOptions{})
	require.NoError(t, err)
	require.False(t, tracingSet.Any())
	_, span := provider.Tracer("test").Start(ctx, "span")
	require.False(t, span.SpanContext().IsValid())
}

func TestTracingRejectsOutputWhenExplicitlyDisabled(t *testing.T) {
	t.Parallel()

	provider, _, _, tracingSet, err := newTracerProvider(t.Context(), lib.RuntimeOptions{
		Tracing:      null.StringFrom("none"),
		TracesOutput: null.StringFrom("otel"),
	})
	require.Nil(t, provider)
	require.False(t, tracingSet.Any())
	require.ErrorContains(t, err, "requires tracing to be enabled")
}

func TestTracingParentAndSamplingConfiguration(t *testing.T) {
	t.Parallel()

	provider, ctx, _, _, err := newTracerProvider(t.Context(), lib.RuntimeOptions{
		Tracing:          null.StringFrom("http"),
		TracesOutput:     null.StringFrom("none"),
		TracesParent:     null.StringFrom("00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"),
		TracesSampler:    null.StringFrom("parentbased_traceidratio"),
		TracesSamplerArg: null.StringFrom("0"),
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })

	runCtx, runSpan := k6trace.StartTestRun(ctx, provider, logrus.StandardLogger())
	require.Equal(t,
		"4bf92f3577b34da6a3ce929d0e0e4736",
		runSpan.SpanContext().TraceID().String(),
	)
	require.True(t, runSpan.SpanContext().IsSampled(), "the run inherits its sampled remote parent")

	_, iterationSpan := k6trace.StartIteration(runCtx, provider, k6trace.IterationInfo{}, true)
	require.True(t, iterationSpan.SpanContext().IsValid())
	require.False(t, iterationSpan.SpanContext().IsSampled(), "the iteration is an independently sampled root")
	require.NotEqual(t, runSpan.SpanContext().TraceID(), iterationSpan.SpanContext().TraceID())
	iterationSpan.End()
	runSpan.End()
}

func TestTracingUnknownModuleErrors(t *testing.T) {
	t.Parallel()

	provider, _, _, tracingSet, err := newTracerProvider(t.Context(), lib.RuntimeOptions{
		Tracing: null.StringFrom("bogus"),
	})
	require.Nil(t, provider)
	require.False(t, tracingSet.Any())
	require.ErrorIs(t, err, moduletrace.ErrUnknownModule)
}

func TestTracingAllMixedWithNameErrors(t *testing.T) {
	t.Parallel()

	provider, _, _, tracingSet, err := newTracerProvider(t.Context(), lib.RuntimeOptions{
		Tracing: null.StringFrom("all,http"),
	})
	require.Nil(t, provider)
	require.False(t, tracingSet.Any())
	require.ErrorIs(t, err, moduletrace.ErrInvalidTracing)
}

func TestTracingEmptyTokenErrors(t *testing.T) {
	t.Parallel()

	provider, _, _, tracingSet, err := newTracerProvider(t.Context(), lib.RuntimeOptions{
		Tracing: null.StringFrom("http,"),
	})
	require.Nil(t, provider)
	require.False(t, tracingSet.Any())
	require.ErrorIs(t, err, moduletrace.ErrInvalidTracing)
}

func TestTracingOutputAloneEnablesBrowserByDefault(t *testing.T) {
	t.Parallel()

	// When --tracing is entirely unset, --traces-output alone must keep
	// working exactly like it did before --tracing existed: browser gets
	// traced automatically, other modules don't. The exporter client dials
	// lazily, so no live collector is needed for this test.
	provider, _, _, tracingSet, err := newTracerProvider(t.Context(), lib.RuntimeOptions{
		TracesOutput: null.StringFrom("otel=http://127.0.0.1:4317"),
		Tracing:      null.String{}, // explicitly unset/invalid
	})
	require.NoError(t, err)
	require.True(t, tracingSet.Enabled("browser"))
	require.False(t, tracingSet.Enabled("http"))
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })
}

func TestTracingExplicitOverridesLegacyBrowserDefault(t *testing.T) {
	t.Parallel()

	provider, _, _, tracingSet, err := newTracerProvider(t.Context(), lib.RuntimeOptions{
		Tracing:      null.StringFrom("http"),
		TracesOutput: null.StringFrom("none"),
	})
	require.NoError(t, err)
	require.True(t, tracingSet.Enabled("http"))
	require.False(t, tracingSet.Enabled("browser"))
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })
}
