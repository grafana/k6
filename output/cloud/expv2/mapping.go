package expv2

import (
	"fmt"
	"strings"

	"github.com/mstoykov/atlas"
	"go.k6.io/k6/internal/output/cloud/expv2/pbcloud"
	"go.k6.io/k6/metrics"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TODO: unit test
func mapTimeSeriesLabelsProto(tags *metrics.TagSet) ([]*pbcloud.Label, []string) {
	labels := make([]*pbcloud.Label, 0, ((*atlas.Node)(tags)).Len())
	var discardedLabels []string

	// TODO: move this as a shared func
	// https://github.com/grafana/k6/issues/2764
	n := (*atlas.Node)(tags)
	if n.Len() < 1 {
		return labels, discardedLabels
	}
	for !n.IsRoot() {
		prev, key, value := n.Data()

		n = prev
		if !isReservedLabelName(key) {
			labels = append(labels, &pbcloud.Label{Name: key, Value: value})
			continue
		}

		if discardedLabels == nil {
			discardedLabels = make([]string, 0, 1)
		}

		discardedLabels = append(discardedLabels, key)
	}
	return labels, discardedLabels
}

func isReservedLabelName(name string) bool {
	// this is a reserved label prefix for the prometheus
	if strings.HasPrefix(name, "__") {
		return true
	}

	return name == "test_run_id"
}

// TODO: unit test
func mapMetricTypeProto(mt metrics.MetricType) pbcloud.MetricType {
	var mtype pbcloud.MetricType
	switch mt {
	case metrics.Counter:
		mtype = pbcloud.MetricType_METRIC_TYPE_COUNTER
	case metrics.Gauge:
		mtype = pbcloud.MetricType_METRIC_TYPE_GAUGE
	case metrics.Rate:
		mtype = pbcloud.MetricType_METRIC_TYPE_RATE
	case metrics.Trend:
		mtype = pbcloud.MetricType_METRIC_TYPE_TREND
	}
	return mtype
}

// TODO: unit test
func addBucketToTimeSeriesProto(
	timeSeries *pbcloud.TimeSeries,
	mt metrics.MetricType,
	time int64,
	value metricValue,
) {
	if timeSeries.Samples == nil {
		initTimeSeriesSamples(timeSeries, mt)
	}

	switch typedMetricValue := value.(type) {
	case *counter:
		samples := timeSeries.GetCounterSamples()
		samples.Values = append(samples.Values, &pbcloud.CounterValue{
			Time:  timestampAsProto(time),
			Value: typedMetricValue.Sum,
		})
	case *gauge:
		samples := timeSeries.GetGaugeSamples()
		samples.Values = append(samples.Values, &pbcloud.GaugeValue{
			Time:  timestampAsProto(time),
			Last:  typedMetricValue.Last,
			Min:   typedMetricValue.Max,
			Max:   typedMetricValue.Min,
			Avg:   typedMetricValue.Avg,
			Count: typedMetricValue.Count,
		})
	case *rate:
		samples := timeSeries.GetRateSamples()
		samples.Values = append(samples.Values, &pbcloud.RateValue{
			Time:         timestampAsProto(time),
			NonzeroCount: typedMetricValue.NonZeroCount,
			TotalCount:   typedMetricValue.Total,
		})
	case *histogram:
		samples := timeSeries.GetTrendHdrSamples()
		samples.Values = append(samples.Values, histogramAsProto(typedMetricValue, time))
	default:
		panic(fmt.Sprintf("MetricType %q is not supported", mt))
	}
}

// TODO: unit test
func initTimeSeriesSamples(timeSeries *pbcloud.TimeSeries, mt metrics.MetricType) {
	switch mt {
	case metrics.Counter:
		timeSeries.Samples = &pbcloud.TimeSeries_CounterSamples{
			CounterSamples: &pbcloud.CounterSamples{},
		}
	case metrics.Gauge:
		timeSeries.Samples = &pbcloud.TimeSeries_GaugeSamples{
			GaugeSamples: &pbcloud.GaugeSamples{},
		}
	case metrics.Rate:
		timeSeries.Samples = &pbcloud.TimeSeries_RateSamples{
			RateSamples: &pbcloud.RateSamples{},
		}
	case metrics.Trend:
		timeSeries.Samples = &pbcloud.TimeSeries_TrendHdrSamples{
			TrendHdrSamples: &pbcloud.TrendHdrSamples{},
		}
	}
}

func timestampAsProto(unixnano int64) *timestamppb.Timestamp {
	sec := unixnano / 1e9
	return &timestamppb.Timestamp{
		Seconds: sec,
		// sub-second precision for aggregation is not expected
		// so we don't waste cpu-time computing nanos.
		//
		// In the case this assumption is changed then enable them
		// by int32(unixnano - (sec * 1e9))
		Nanos: 0,
	}
}
