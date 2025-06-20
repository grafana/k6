package v1

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/metrics"
)

func TestNullMetricTypeJSON(t *testing.T) {
	t.Parallel()

	values := map[NullMetricType]string{
		{}:                      `null`,
		{metrics.Counter, true}: `"counter"`,
		{metrics.Gauge, true}:   `"gauge"`,
		{metrics.Trend, true}:   `"trend"`,
		{metrics.Rate, true}:    `"rate"`,
	}
	t.Run("Marshal", func(t *testing.T) {
		t.Parallel()

		for mt, val := range values {
			t.Run(val, func(t *testing.T) {
				t.Parallel()

				data, err := json.Marshal(mt)
				assert.NoError(t, err)
				assert.Equal(t, val, string(data))
			})
		}
	})
	t.Run("Unmarshal", func(t *testing.T) {
		t.Parallel()

		for mt, val := range values {
			t.Run(val, func(t *testing.T) {
				t.Parallel()

				var value NullMetricType
				assert.NoError(t, json.Unmarshal([]byte(val), &value))
				assert.Equal(t, mt, value)
			})
		}
	})
}

func TestNullValueTypeJSON(t *testing.T) {
	t.Parallel()

	values := map[NullValueType]string{
		{}:                      `null`,
		{metrics.Default, true}: `"default"`,
		{metrics.Time, true}:    `"time"`,
	}
	t.Run("Marshal", func(t *testing.T) {
		t.Parallel()

		for mt, val := range values {
			t.Run(val, func(t *testing.T) {
				t.Parallel()

				data, err := json.Marshal(mt)
				assert.NoError(t, err)
				assert.Equal(t, val, string(data))
			})
		}
	})
	t.Run("Unmarshal", func(t *testing.T) {
		t.Parallel()

		for mt, val := range values {
			t.Run(val, func(t *testing.T) {
				t.Parallel()

				var value NullValueType
				assert.NoError(t, json.Unmarshal([]byte(val), &value))
				assert.Equal(t, mt, value)
			})
		}
	})
}

func TestNewMetric(t *testing.T) {
	t.Parallel()

	old, err := metrics.NewRegistry().NewMetric("test_metric", metrics.Trend, metrics.Time)
	require.NoError(t, err)
	old.Tainted = null.BoolFrom(true)
	m := NewMetric(old, 0)
	assert.Equal(t, "test_metric", m.Name)
	assert.True(t, m.Type.Valid)
	assert.Equal(t, metrics.Trend, m.Type.Type)
	assert.True(t, m.Contains.Valid)
	assert.True(t, m.Tainted.Bool)
	assert.True(t, m.Tainted.Valid)
	assert.Equal(t, metrics.Time, m.Contains.Type)
	assert.NotEmpty(t, m.Sample)
}
