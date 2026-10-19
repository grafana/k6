package trace_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/propagation"
	oteltrace "go.opentelemetry.io/otel/trace"

	k6trace "go.k6.io/k6/v2/internal/lib/trace"
)

func TestContextWithRemoteParent(t *testing.T) {
	t.Parallel()

	t.Run("empty parent leaves the context unchanged", func(t *testing.T) {
		t.Parallel()

		ctx, err := k6trace.ContextWithRemoteParent(context.Background(), "", propagation.TraceContext{})
		require.NoError(t, err)
		assert.False(t, oteltrace.SpanContextFromContext(ctx).IsValid())
	})

	t.Run("valid traceparent", func(t *testing.T) {
		t.Parallel()

		ctx, err := k6trace.ContextWithRemoteParent(
			context.Background(), "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-00", propagation.TraceContext{},
		)
		require.NoError(t, err)
		sc := oteltrace.SpanContextFromContext(ctx)
		assert.True(t, sc.IsRemote())
		assert.False(t, sc.IsSampled())
		assert.Equal(t, "4bf92f3577b34da6a3ce929d0e0e4736", sc.TraceID().String())
		assert.Equal(t, "00f067aa0ba902b7", sc.SpanID().String())
	})

	for name, parent := range map[string]string{
		"not a traceparent":      "garbage",
		"bare span id":           "00f067aa0ba902b7",
		"all zero trace id":      "00-00000000000000000000000000000000-00f067aa0ba902b7-01",
		"truncated traceparent":  "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7",
		"unsupported version ff": "ff-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
	} {
		t.Run("invalid: "+name, func(t *testing.T) {
			t.Parallel()

			_, err := k6trace.ContextWithRemoteParent(context.Background(), parent, propagation.TraceContext{})
			require.ErrorIs(t, err, k6trace.ErrInvalidParent)
		})
	}
}
