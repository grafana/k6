package executor

import "go.k6.io/k6/metrics"

func sumMetricValues(samples chan metrics.SampleContainer, metricName string) (sum float64) { //nolint:unparam
	for _, sc := range metrics.GetBufferedSamples(samples) {
		samples := sc.GetSamples()
		for _, s := range samples {
			if s.Metric.Name == metricName {
				sum += s.Value
			}
		}
	}
	return sum
}
