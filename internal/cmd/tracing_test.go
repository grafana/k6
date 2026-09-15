package cmd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	k6trace "go.k6.io/k6/v2/internal/lib/trace"
	"go.k6.io/k6/v2/lib"
)

func TestTracingCanBeEnabledWithoutOutput(t *testing.T) {
	t.Parallel()

	provider, ctx, _, err := newTracerProvider(t.Context(), lib.RuntimeOptions{
		TracingEnabled: null.BoolFrom(true),
		TracesOutput:   null.StringFrom("none"),
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })

	_, span := provider.Tracer("test").Start(ctx, "span")
	require.True(t, span.SpanContext().IsValid())
	require.True(t, span.SpanContext().IsSampled())
	span.End()
}

func TestTracingDisabledUsesNoopProvider(t *testing.T) {
	t.Parallel()

	provider, ctx, _, err := newTracerProvider(t.Context(), lib.RuntimeOptions{})
	require.NoError(t, err)
	_, span := provider.Tracer("test").Start(ctx, "span")
	require.False(t, span.SpanContext().IsValid())
}

func TestTracingRejectsOutputWhenExplicitlyDisabled(t *testing.T) {
	t.Parallel()

	_, _, _, err := newTracerProvider(t.Context(), lib.RuntimeOptions{
		TracingEnabled: null.BoolFrom(false),
		TracesOutput:   null.StringFrom("otel"),
	})
	require.ErrorContains(t, err, "requires tracing to be enabled")
}

func TestTracingParentAndSamplingConfiguration(t *testing.T) {
	t.Parallel()

	provider, ctx, _, err := newTracerProvider(t.Context(), lib.RuntimeOptions{
		TracingEnabled:   null.BoolFrom(true),
		TracesOutput:     null.StringFrom("none"),
		TracesParent:     null.StringFrom("00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"),
		TracesSampler:    null.StringFrom("parentbased_traceidratio"),
		TracesSamplerArg: null.StringFrom("0"),
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })

	runCtx, runSpan := k6trace.StartTestRun(ctx, provider)
	require.Equal(t,
		"4bf92f3577b34da6a3ce929d0e0e4736",
		runSpan.SpanContext().TraceID().String(),
	)
	require.True(t, runSpan.SpanContext().IsSampled(), "the run inherits its sampled remote parent")

	_, iterationSpan := k6trace.StartIteration(runCtx, provider, k6trace.IterationInfo{})
	require.True(t, iterationSpan.SpanContext().IsValid())
	require.False(t, iterationSpan.SpanContext().IsSampled(), "the iteration is an independently sampled root")
	require.NotEqual(t, runSpan.SpanContext().TraceID(), iterationSpan.SpanContext().TraceID())
	iterationSpan.End()
	runSpan.End()
}
