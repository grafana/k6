package expv2

import (
	"fmt"
	"time"

	"github.com/mstoykov/atlas"
	"go.k6.io/k6/metrics"
	"go.k6.io/k6/output/cloud/expv2/pbcloud"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TODO: unit test
func mapTimeSeriesLabelsProto(timeSeries metrics.TimeSeries, testRunID string) []*pbcloud.Label {
	labels := make([]*pbcloud.Label, 0, ((*atlas.Node)(timeSeries.Tags)).Len()+2)
	labels = append(labels,
		&pbcloud.Label{Name: "__name__", Value: timeSeries.Metric.Name},
		&pbcloud.Label{Name: "test_run_id", Value: testRunID})

	// TODO: move it as a shared func
	// https://github.com/grafana/k6/issues/2764
	n := (*atlas.Node)(timeSeries.Tags)
	if n.Len() < 1 {
		return labels
	}
	for !n.IsRoot() {
		prev, key, value := n.Data()
		labels = append(labels, &pbcloud.Label{Name: key, Value: value})
		n = prev
	}
	return labels
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
	time time.Time,
	value metricValue,
) {
	if timeSeries.Samples == nil {
		initTimeSeriesSamples(timeSeries, mt)
	}

	switch typedMetricValue := value.(type) {
	case *counter:
		samples := timeSeries.GetCounterSamples()
		samples.Values = append(samples.Values, &pbcloud.CounterValue{
			Time:  timestamppb.New(time),
			Value: typedMetricValue.Sum,
		})
	case *gauge:
		samples := timeSeries.GetGaugeSamples()
		samples.Values = append(samples.Values, &pbcloud.GaugeValue{
			Time:  timestamppb.New(time),
			Last:  typedMetricValue.Last,
			Min:   typedMetricValue.Max,
			Max:   typedMetricValue.Min,
			Avg:   typedMetricValue.Avg,
			Count: typedMetricValue.Count,
		})
	case *rate:
		samples := timeSeries.GetRateSamples()
		samples.Values = append(samples.Values, &pbcloud.RateValue{
			Time:         timestamppb.New(time),
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
