package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/sampler"
	"time"
)

func printMetrics() {
	for name, m := range sampler.DefaultSampler.Metrics {
		switch m.Type {
		case sampler.GaugeType:
			last := m.Last()
			if last == 0 {
				continue
			}
			log.WithField("val", applyIntent(m, last)).Infof("Metric: %s", name)
		case sampler.CounterType:
			sum := m.Sum()
			if sum == 0 {
				continue
			}
			log.WithField("num", applyIntent(m, sum)).Infof("Metric: %s", name)
		case sampler.StatsType:
			log.WithFields(log.Fields{
				"min": applyIntent(m, m.Min()),
				"max": applyIntent(m, m.Max()),
				"avg": applyIntent(m, m.Avg()),
				"med": applyIntent(m, m.Med()),
			}).Infof("Metric: %s", name)
		}
	}
}

func commitMetrics() {
	if err := sampler.DefaultSampler.Commit(); err != nil {
		log.WithError(err).Error("Couldn't write samples!")
	}
}

func closeMetrics() {
	if err := sampler.DefaultSampler.Commit(); err != nil {
		log.WithError(err).Error("Couldn't close sampler!")
	}
}

func applyIntent(m *sampler.Metric, v int64) interface{} {
	if m.Intent == sampler.TimeIntent {
		return time.Duration(v)
	}
	return v
}
