package main

import (
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/sampler"
	"io"
	stdlog "log"
	"time"
)

type LogMetricsOutput struct {
	Writer io.Writer

	encoder *json.Encoder
}

func (o *LogMetricsOutput) Write(m *sampler.Metric, e *sampler.Entry) error {
	if o.encoder == nil {
		o.encoder = json.NewEncoder(o.Writer)
	}
	return o.encoder.Encode(e)
}

func (o *LogMetricsOutput) Commit() error {
	return nil
}

func printMetrics(l *stdlog.Logger) {
	for name, m := range sampler.DefaultSampler.Metrics {
		switch m.Type {
		case sampler.GaugeType:
			l.Printf("%s val=%v\n", name, applyIntent(m, m.Last()))
		case sampler.CounterType:
			l.Printf("%s num=%v\n", name, applyIntent(m, m.Sum()))
		case sampler.StatsType:
			l.Printf("%s min=%v max=%v avg=%v med=%v\n", name,
				applyIntent(m, m.Min()),
				applyIntent(m, m.Max()),
				applyIntent(m, m.Avg()),
				applyIntent(m, m.Med()),
			)
		}
	}
}

func commitMetrics() {
	if err := sampler.DefaultSampler.Commit(); err != nil {
		log.WithError(err).Error("Couldn't write samples!")
	}
}

func applyIntent(m *sampler.Metric, v int64) interface{} {
	if m.Intent == sampler.TimeIntent {
		return time.Duration(v)
	}
	return fmt.Sprint(v)
}
