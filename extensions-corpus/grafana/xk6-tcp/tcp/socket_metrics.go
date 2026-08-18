package tcp

import (
	"strconv"
	"time"

	extensionapi "go.k6.io/k6-extension-api"
)

func (s *socket) currentTags() extensionapi.Tags {
	return s.metrics.host.CurrentTags()
}

func (s *socket) tags() extensionapi.Tags {
	tags := s.metrics.host.WithSystemTags(s.currentTags(), map[extensionapi.SystemTag]string{
		extensionapi.SystemTagProto: "TCP",
	})
	tags = tags.With(s.socketOpts.Tags)
	if s.connectOpts != nil {
		tags = tags.With(s.connectOpts.Tags).With(map[string]string{
			"host": s.connectOpts.Host,
			"port": strconv.Itoa(s.connectOpts.Port),
		})
	}
	if s.endpoints.remoteIP != "" {
		tags = s.metrics.host.WithSystemTags(tags, map[extensionapi.SystemTag]string{
			extensionapi.SystemTagIP: s.endpoints.remoteIP,
		})
	}
	return tags
}

func (s *socket) emit(samples ...extensionapi.Sample) {
	if err := s.metrics.host.Emit(s.vu.Context(), samples); err != nil && s.vu.Context().Err() == nil {
		s.log.Debug("TCP metric emission failed", "error", err)
	}
}

func (s *socket) addErrorMetrics(tags extensionapi.Tags) {
	s.emit(extensionapi.Sample{Metric: s.metrics.tcpErrors, Time: time.Now(), Value: 1, Tags: tags})
}

func (s *socket) addCounterMetrics(metric extensionapi.Metric, tags extensionapi.Tags) {
	s.emit(extensionapi.Sample{Metric: metric, Time: time.Now(), Value: 1, Tags: tags})
}

func (s *socket) addDurationMetrics(duration time.Duration, metric extensionapi.Metric, tags extensionapi.Tags) {
	s.emit(extensionapi.Sample{
		Metric: metric,
		Time:   time.Now(),
		Value:  float64(duration) / float64(time.Millisecond),
		Tags:   tags,
	})
}

func (s *socket) addDurationMetricsFor(metric extensionapi.Metric, tags extensionapi.Tags, fn func() error) error {
	start := time.Now()
	err := fn()
	s.addDurationMetrics(time.Since(start), metric, tags)
	return err
}
