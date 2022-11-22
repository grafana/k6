package remotewrite

import (
	"sort"
	"time"

	prompb "go.buf.build/grpc/go/prometheus/prometheus"
	"go.k6.io/k6/metrics"
)

type extendedTrendSink struct {
	*metrics.TrendSink
}

func newExtendedTrendSink() *extendedTrendSink {
	return &extendedTrendSink{
		TrendSink: &metrics.TrendSink{},
	}
}

// MapPrompb converts a k6 time series and its relative
// Sink into the equivalent TimeSeries model as defined from
// the Remote write specification.
func (sink *extendedTrendSink) MapPrompb(series metrics.TimeSeries, t time.Time) []*prompb.TimeSeries {
	// Prometheus metric system does not support Trend so this mapping will
	// store a counter for the number of reported values and gauges to keep
	// track of aggregated values. Also store a sum of the values to allow
	// the calculation of moving averages.
	// TODO: when Prometheus implements support for sparse histograms, re-visit this implementation

	tg := &trendAsGauges{
		series:    make([]*prompb.TimeSeries, 0, 8),
		labels:    MapSeries(series),
		timestamp: t.UnixMilli(),
	}
	tg.FindNameIndex()

	tg.Append("count", float64(sink.Count))
	tg.Append("sum", sink.Sum)
	tg.Append("min", sink.Min)
	tg.Append("max", sink.Max)
	tg.Append("avg", sink.Avg)
	tg.Append("med", sink.P(0.5))
	tg.Append("p90", sink.P(0.9))
	tg.Append("p95", sink.P(0.95))
	return tg.series
}

type trendAsGauges struct {
	// series is the slice of the converted TimeSeries.
	series []*prompb.TimeSeries

	// labels are the shared labels between all the Gauges.
	labels []*prompb.Label

	// timestamp is the shared timestamp in ms between all the Gauges.
	timestamp int64

	// ixname is the slice's index
	// of the __name__ Label item.
	//
	// 16 bytes should be enough for the max length
	// an higher value will probably generate
	// serious issues in other places.
	ixname uint16
}

func (tg *trendAsGauges) Append(suffix string, v float64) {
	ts := &prompb.TimeSeries{
		Labels:  make([]*prompb.Label, len(tg.labels)),
		Samples: make([]*prompb.Sample, 1),
	}
	for i := 0; i < len(tg.labels); i++ {
		ts.Labels[i] = &prompb.Label{
			Name:  tg.labels[i].Name,
			Value: tg.labels[i].Value,
		}
	}
	ts.Labels[tg.ixname].Value += "_" + suffix

	ts.Samples[0] = &prompb.Sample{
		Timestamp: tg.timestamp,
		Value:     v,
	}
	tg.series = append(tg.series, ts)
}

// FindNameIndex finds the __name__ label's index
// if it is different from the most common expected case
// then it caches the value.
// The labels slice is expected to be sorted.
func (tg *trendAsGauges) FindNameIndex() {
	if tg.labels[0].Name == namelbl {
		// ixname is expected to be the first in most of the cases
		// the default value is already 0
		return
	}

	// in the case __name__ is not the first
	// then search for its position

	i := sort.Search(len(tg.labels), func(i int) bool {
		return tg.labels[i].Name == namelbl
	})

	if i < len(tg.labels) && tg.labels[i].Name == namelbl {
		tg.ixname = uint16(i)
	}
}
