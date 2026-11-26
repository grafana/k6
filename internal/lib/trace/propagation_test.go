package trace_test

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	oteltrace "go.opentelemetry.io/otel/trace"

	"go.k6.io/k6/v2/internal/lib/testutils"
	k6trace "go.k6.io/k6/v2/internal/lib/trace"
	"go.k6.io/k6/v2/internal/lib/trace/tracetest"
)

func TestExtractParentFromEnv(t *testing.T) {
	t.Parallel()

	const (
		traceID = "0af7651916cd43dd8448eb211c80319c"
		spanID  = "b7ad6b7169203331"
	)

	t.Run("sampled", func(t *testing.T) {
		t.Parallel()

		logger, hook := testutils.NewLoggerWithHook(t)
		ctx := k6trace.ExtractParentFromEnv(context.Background(), map[string]string{
			"TRACEPARENT": "00-" + traceID + "-" + spanID + "-01",
			"TRACESTATE":  "vendor=value",
		}, logger)

		sc := oteltrace.SpanContextFromContext(ctx)
		assert.True(t, sc.IsRemote())
		assert.True(t, sc.IsSampled())
		assert.Equal(t, traceID, sc.TraceID().String())
		assert.Equal(t, spanID, sc.SpanID().String())
		assert.Equal(t, "vendor=value", sc.TraceState().String())
		assert.Empty(t, hook.Drain())
	})

	t.Run("not sampled", func(t *testing.T) {
		t.Parallel()

		tp, rec := tracetest.NewProvider()
		tracer := k6trace.Tracer(tp, nativeTracing())
		logger, hook := testutils.NewLoggerWithHook(t)

		ctx := k6trace.ExtractParentFromEnv(context.Background(), map[string]string{
			"TRACEPARENT": "00-" + traceID + "-" + spanID + "-00",
		}, logger)
		_, run := k6trace.StartTestRun(ctx, tracer, logger)
		run.End()

		assert.False(t, run.SpanContext().IsSampled())
		assert.Equal(t, traceID, run.SpanContext().TraceID().String())
		assert.Empty(t, rec.Ended())

		entries := hook.Drain()
		require.NotEmpty(t, entries)
		assert.Equal(t, logrus.DebugLevel, entries[0].Level)
		assert.Contains(t, entries[0].Message, "not sampled")
	})

	t.Run("invalid", func(t *testing.T) {
		t.Parallel()

		logger, hook := testutils.NewLoggerWithHook(t)
		ctx := k6trace.ExtractParentFromEnv(context.Background(), map[string]string{
			"TRACEPARENT": "not-a-traceparent",
		}, logger)

		assert.False(t, oteltrace.SpanContextFromContext(ctx).IsValid())
		entries := hook.Drain()
		require.NotEmpty(t, entries)
		assert.Equal(t, logrus.WarnLevel, entries[0].Level)
		assert.NotContains(t, entries[0].Message, "not-a-traceparent")
	})

	t.Run("invalid tracestate", func(t *testing.T) {
		t.Parallel()

		logger, hook := testutils.NewLoggerWithHook(t)
		ctx := k6trace.ExtractParentFromEnv(context.Background(), map[string]string{
			"TRACEPARENT": "00-" + traceID + "-" + spanID + "-01",
			"TRACESTATE":  "vendor=ok,secretmember",
		}, logger)

		// The trace is continued, only the tracestate is dropped.
		sc := oteltrace.SpanContextFromContext(ctx)
		assert.Equal(t, traceID, sc.TraceID().String())
		assert.Empty(t, sc.TraceState().String())
		entries := hook.Drain()
		require.NotEmpty(t, entries)
		assert.Equal(t, logrus.WarnLevel, entries[0].Level)
		assert.Contains(t, entries[0].Message, k6trace.TraceStateEnvVar)
		for _, e := range entries {
			assert.NotContains(t, e.Message, "secretmember")
			assert.NotContains(t, e.Message, "vendor=ok")
		}
	})

	t.Run("unset", func(t *testing.T) {
		t.Parallel()

		logger, hook := testutils.NewLoggerWithHook(t)
		ctx := k6trace.ExtractParentFromEnv(context.Background(), nil, logger)
		assert.False(t, oteltrace.SpanContextFromContext(ctx).IsValid())
		assert.Empty(t, hook.Drain())
	})
}

func TestStartTestRunWithoutIncomingTrace(t *testing.T) {
	t.Parallel()

	tp, rec := tracetest.NewProvider()
	tracer := k6trace.Tracer(tp, nativeTracing())
	logger := testutils.NewLogger(t)

	_, run := k6trace.StartTestRun(context.Background(), tracer, logger)
	run.End()

	assert.False(t, rec.ByName(k6trace.SpanRun)[0].Parent().IsValid())
}
