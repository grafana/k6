package json

import (
	"github.com/loadimpact/k6/stats"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestWrapWithNilArg(t *testing.T) {
	out := Wrap(nil)
	assert.Equal(t, out, (*Envelope)(nil))
}

func TestWrapWithUnusedType(t *testing.T) {
	out := Wrap(JSONSample{})
	assert.Equal(t, out, (*Envelope)(nil))
}

func TestWrapWithSample(t *testing.T) {
	out := Wrap(stats.Sample{
		Metric: &stats.Metric{},
	})
	assert.NotEqual(t, out, (*Envelope)(nil))
}

func TestWrapWithMetricPointer(t *testing.T) {
	out := Wrap(&stats.Metric{})
	assert.NotEqual(t, out, (*Envelope)(nil))
}

func TestWrapWithMetric(t *testing.T) {
	out := Wrap(stats.Metric{})
	assert.Equal(t, out, (*Envelope)(nil))
}
