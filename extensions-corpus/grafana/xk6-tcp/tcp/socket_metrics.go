package tcp

import (
	"strconv"
	"time"

	"go.k6.io/k6/v2/metrics"
)

func (s *socket) currentTags() *metrics.TagSet {
	return s.vu.State().Tags.GetCurrentValues().Tags
}

func (s *socket) tags() *metrics.TagSet {
	tags := s.currentTags()

	tags = tags.With("proto", "TCP")
	tags = addToTagSet(tags, s.socketOpts.Tags)

	if s.connectOpts != nil {
		tags = tags.WithTagsFromMap(s.connectOpts.Tags).
			With("host", s.connectOpts.Host).
			With("port", strconv.Itoa(s.connectOpts.Port))
	}

	if len(s.endpoints.remoteIP) > 0 {
		tags = tags.With("ip", s.endpoints.remoteIP)
	}

	return tags
}

func (s *socket) addErrorMetrics(ts *metrics.TagSet) {
	metrics.PushIfNotDone(s.vu.Context(), s.vu.State().Samples, metrics.Samples{
		metrics.Sample{
			TimeSeries: metrics.TimeSeries{
				Metric: s.metrics.tcpErrors,
				Tags:   ts,
			},
			Time:  time.Now(),
			Value: float64(1),
		},
	})
}

func (s *socket) addCounterMetrics(metric *metrics.Metric, ts *metrics.TagSet) {
	metrics.PushIfNotDone(s.vu.Context(), s.vu.State().Samples, metrics.Samples{
		metrics.Sample{
			TimeSeries: metrics.TimeSeries{
				Metric: metric,
				Tags:   ts,
			},
			Time:  time.Now(),
			Value: float64(1),
		},
	})
}

func (s *socket) addDurationMetrics(duration time.Duration, metric *metrics.Metric, ts *metrics.TagSet) {
	metrics.PushIfNotDone(s.vu.Context(), s.vu.State().Samples, metrics.Samples{
		metrics.Sample{
			TimeSeries: metrics.TimeSeries{
				Metric: metric,
				Tags:   ts,
			},
			Time:  time.Now(),
			Value: float64(duration.Milliseconds()),
		},
	})
}

func (s *socket) addDurationMetricsFor(metric *metrics.Metric, ts *metrics.TagSet, fn func() error) error {
	start := time.Now()

	err := fn()
	duration := time.Since(start)

	s.addDurationMetrics(duration, metric, ts)

	return err
}
