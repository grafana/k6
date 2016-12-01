package js

import (
	"github.com/loadimpact/k6/stats"
	"time"
)

func (a JSAPI) MetricAdd(m *stats.Metric, v float64, tags map[string]string) {
	t := time.Now()
	s := stats.Sample{Metric: m, Time: t, Tags: tags, Value: v}
	a.vu.Samples = append(a.vu.Samples, s)
}
