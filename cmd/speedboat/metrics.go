package main

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/sampler"
	"time"
)

func printMetrics() {
	for name, metric := range sampler.DefaultSampler.Metrics {
		text := fmt.Sprintf("Metric: %s", name)
		switch metric.Type {
		case sampler.CounterType:
			last := metric.Entries[len(metric.Entries)-1]
			log.WithField("val", applyIntent(metric, last.Value)).Info(text)
		case sampler.StatsType:
			log.WithFields(log.Fields{
				"min": applyIntent(metric, metric.Min()),
				"max": applyIntent(metric, metric.Max()),
				"avg": applyIntent(metric, metric.Avg()),
				"med": applyIntent(metric, metric.Med()),
			}).Info(text)
		}
	}
}

func applyIntent(m *sampler.Metric, v int64) interface{} {
	if m.Intent == sampler.TimeIntent {
		return time.Duration(v)
	}
	return v
}
