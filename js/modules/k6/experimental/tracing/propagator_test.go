package tracing

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPropagate(t *testing.T) {
	t.Parallel()

	traceID := "abc123"

	t.Run("W3C Propagator", func(t *testing.T) {
		t.Parallel()

		sampler := mockSampler{decision: true}
		propagator := NewW3CPropagator(sampler)

		gotHeader, gotErr := propagator.Propagate(traceID)

		assert.NoError(t, gotErr)
		assert.Contains(t, gotHeader, W3CHeaderName)

		//nolint:staticcheck // as traceparent is not a canonical header
		headerContent := gotHeader[W3CHeaderName][0]
		assert.True(t, strings.HasPrefix(headerContent, W3CVersion+"-"+traceID+"-"))
	})

	t.Run("W3C propagator with sampled trace", func(t *testing.T) {
		t.Parallel()

		sampler := mockSampler{decision: true}
		propagator := NewW3CPropagator(sampler)

		gotHeader, gotErr := propagator.Propagate(traceID)
		require.NoError(t, gotErr)
		require.Contains(t, gotHeader, W3CHeaderName)

		//nolint:staticcheck // as traceparent is not a canonical header
		assert.True(t, strings.HasSuffix(gotHeader[W3CHeaderName][0], "-01"))
	})

	t.Run("W3C propagator with unsampled trace", func(t *testing.T) {
		t.Parallel()

		sampler := mockSampler{decision: false}
		propagator := NewW3CPropagator(sampler)

		gotHeader, gotErr := propagator.Propagate(traceID)
		require.NoError(t, gotErr)
		require.Contains(t, gotHeader, W3CHeaderName)

		//nolint:staticcheck // as traceparent is not a canonical header
		assert.True(t, strings.HasSuffix(gotHeader[W3CHeaderName][0], "-00"))
	})

	t.Run("Jaeger Propagator", func(t *testing.T) {
		t.Parallel()

		sampler := mockSampler{decision: true}
		propagator := NewJaegerPropagator(sampler)

		gotHeader, gotErr := propagator.Propagate(traceID)

		assert.NoError(t, gotErr)
		assert.Contains(t, gotHeader, JaegerHeaderName)

		//nolint:staticcheck // as traceparent is not a canonical header
		headerContent := gotHeader[JaegerHeaderName][0]
		assert.True(t, strings.HasPrefix(headerContent, traceID+":"))
		assert.True(t, strings.HasSuffix(headerContent, ":0:1"))
	})

	t.Run("Jaeger propagator with sampled trace", func(t *testing.T) {
		t.Parallel()

		sampler := mockSampler{decision: true}
		propagator := NewJaegerPropagator(sampler)

		gotHeader, gotErr := propagator.Propagate(traceID)
		require.NoError(t, gotErr)
		require.Contains(t, gotHeader, JaegerHeaderName)

		//nolint:staticcheck // as traceparent is not a canonical header
		assert.True(t, strings.HasSuffix(gotHeader[JaegerHeaderName][0], ":1"))
	})

	t.Run("Jaeger propagator with unsampled trace", func(t *testing.T) {
		t.Parallel()

		sampler := mockSampler{decision: false}
		propagator := NewJaegerPropagator(sampler)

		gotHeader, gotErr := propagator.Propagate(traceID)
		require.NoError(t, gotErr)
		require.Contains(t, gotHeader, JaegerHeaderName)

		//nolint:staticcheck // as traceparent is not a canonical header
		assert.True(t, strings.HasSuffix(gotHeader[JaegerHeaderName][0], ":0"))
	})
}

type mockSampler struct {
	decision bool
}

func (m mockSampler) ShouldSample() bool {
	return m.decision
}
