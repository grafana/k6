package summary

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_extractPercentileThresholdSource(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		source          string
		expAgg          string
		expPercentile   float64
		expIsPercentile bool
	}{
		"empty source": {
			source: "", expAgg: "", expPercentile: 0, expIsPercentile: false,
		},
		"invalid source": {
			source: "rate<<<<0.01", expAgg: "", expPercentile: 0, expIsPercentile: false,
		},
		"rate threshold": {
			source: "rate<0.01", expAgg: "", expPercentile: 0, expIsPercentile: false,
		},
		"integer percentile": {
			source: "p(95)<200", expAgg: "p(95)", expPercentile: 95, expIsPercentile: true,
		},
		"floating-point percentile": {
			source: "p(99.9)<=200", expAgg: "p(99.9)", expPercentile: 99.9, expIsPercentile: true,
		},
		"percentile with whitespaces": {
			source: "  p(99.9)  <  200  ", expAgg: "p(99.9)", expPercentile: 99.9, expIsPercentile: true,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			agg, percentile, isPercentile := extractPercentileThresholdSource(tc.source)
			assert.Equal(t, tc.expAgg, agg)
			assert.Equal(t, tc.expPercentile, percentile)
			assert.Equal(t, tc.expIsPercentile, isPercentile)
		})
	}
}
