package remotewrite

import (
	"fmt"
	"sort"
	"time"

	prompb "buf.build/gen/go/prometheus/prometheus/protocolbuffers/go"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"go.k6.io/k6/metrics"
)

// TrendStatsResolver is a map of trend stats name and their relative resolver function
type TrendStatsResolver map[string]func(*metrics.TrendSink) float64

type extendedTrendSink struct {
	*metrics.TrendSink

	trendStats map[string]func(*metrics.TrendSink) float64
}

func newExtendedTrendSink(tsr TrendStatsResolver) (*extendedTrendSink, error) {
	if len(tsr) < 1 {
		return nil, fmt.Errorf("trend stats resolver is empty")
	}
	return &extendedTrendSink{
		TrendSink:  metrics.NewTrendSink(),
		trendStats: tsr,
	}, nil
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
		series: make([]*prompb.TimeSeries, 0, len(sink.trendStats)),
		// TODO: should we add the base unit suffix?
		// It could depends from the decision for other metric types
		// Does k6_http_req_duration_seconds_count make sense?
		labels:    MapSeries(series, ""),
		timestamp: t.UnixMilli(),
	}
	tg.CacheNameIndex()

	for stat, statfn := range sink.trendStats {
		tg.Append(stat, adaptUnit(series.Metric.Contains, statfn(sink.TrendSink)))
	}
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

// CacheNameIndex finds the __name__ label's index
// if it is different from the most common expected case
// then it caches the value.
// The labels slice is expected to be sorted.
func (tg *trendAsGauges) CacheNameIndex() {
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
		tg.ixname = uint16(i) //nolint:gosec
	}
}

type nativeHistogramSink struct {
	H prometheus.Histogram
}

func newNativeHistogramSink(m *metrics.Metric) *nativeHistogramSink {
	return &nativeHistogramSink{
		H: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name: m.Name,
			// 1.1 is the starting value suggested by Prometheus'
			// It sounds good considering the general purpose
			// it have to address.
			// In the future, we could consider to add more tuning
			// if it will be required.
			NativeHistogramBucketFactor: 1.1,
		}),
	}
}

func (sink *nativeHistogramSink) Add(s metrics.Sample) {
	// The Prometheus' convention is to use seconds
	// as time unit.
	//
	// It isn't a requirement but having the current factor fixed to 1.1 then
	// have seconds is beneficial for having a better resolution.
	//
	// The assumption is that an higher precision is required
	// in case of under-second and more relaxed in case of higher values.
	// If the Value type is not defined any assumption can be done
	// because the Sample's Value could contains any unit.
	sink.H.Observe(adaptUnit(s.Metric.Contains, s.Value))
}

// TODO: create a smaller Sink interface for this Output.
// Sink with only Add and MapPrompb methods should be enough.
// One method interfaces could be even better, to be checked.

// P implements metrics.Sink.
func (*nativeHistogramSink) P(_ float64) float64 {
	panic("Native Histogram Sink has no support of percentile (P)")
}

// Format implements metrics.Sink.
func (*nativeHistogramSink) Format(_ time.Duration) map[string]float64 {
	panic("Native Histogram Sink has no support of formatting (Format)")
}

// IsEmpty implements metrics.Sink.
func (*nativeHistogramSink) IsEmpty() bool {
	panic("Native Histogram Sink has no support of emptiness check (IsEmpty)")
}

// Drain implements metrics.Sink.
func (*nativeHistogramSink) Drain() ([]byte, error) {
	panic("Native Histogram Sink has no support of draining")
}

// Merge implements metrics.Sink.
func (*nativeHistogramSink) Merge(_ []byte) error {
	panic("Native Histogram Sink has no support of merging")
}

// MapPrompb maps the Trend type to the experimental Native Histogram.
func (sink *nativeHistogramSink) MapPrompb(series metrics.TimeSeries, t time.Time) []*prompb.TimeSeries {
	suffix := baseUnit(series.Metric.Contains)
	labels := MapSeries(series, suffix)
	timestamp := t.UnixMilli()

	return []*prompb.TimeSeries{
		{
			Labels: labels,
			Histograms: []*prompb.Histogram{
				histogramToHistogramProto(timestamp, sink.H),
			},
		},
	}
}

func histogramToHistogramProto(timestamp int64, h prometheus.Histogram) *prompb.Histogram {
	// TODO: research more if a better way is possible.
	metric := &dto.Metric{}
	if err := h.Write(metric); err != nil {
		panic(fmt.Errorf("failed to convert Native Histogram to the related Protobuf: %w", err))
	}
	hmetric := metric.Histogram

	return &prompb.Histogram{
		Count:          &prompb.Histogram_CountInt{CountInt: *hmetric.SampleCount},
		Sum:            *hmetric.SampleSum,
		Schema:         *hmetric.Schema,
		ZeroThreshold:  *hmetric.ZeroThreshold,
		ZeroCount:      &prompb.Histogram_ZeroCountInt{ZeroCountInt: *hmetric.ZeroCount},
		NegativeSpans:  toBucketSpanProto(hmetric.NegativeSpan),
		NegativeDeltas: hmetric.NegativeDelta,
		PositiveSpans:  toBucketSpanProto(hmetric.PositiveSpan),
		PositiveDeltas: hmetric.PositiveDelta,
		Timestamp:      timestamp,
	}
}

func toBucketSpanProto(s []*dto.BucketSpan) []*prompb.BucketSpan {
	spans := make([]*prompb.BucketSpan, len(s))
	for i := 0; i < len(s); i++ {
		spans[i] = &prompb.BucketSpan{Offset: *s[i].Offset, Length: *s[i].Length}
	}
	return spans
}

func baseUnit(vt metrics.ValueType) string {
	switch vt {
	case metrics.Time:
		return "seconds"
	case metrics.Data:
		return "bytes"
	default:
		return ""
	}
}

// adaptUnit converts the generated value into the expected base unit
// as requested by the Prometheus convention.
//
// Time: converted to seconds from milliseconds.
// Data: k6 emits it in Bytes so it already fine.
// Other: use the submitted unit.
func adaptUnit(vt metrics.ValueType, v float64) float64 {
	if vt == metrics.Time {
		return v / 1000
	}
	return v
}
