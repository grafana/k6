package expv2

import (
	"context"

	"go.k6.io/k6/metrics"
	"go.k6.io/k6/output/cloud/expv2/pbcloud"
)

type pusher interface {
	push(referenceID string, samples *pbcloud.MetricSet) error
}

type metricsFlusher struct {
	referenceID                string
	bq                         *bucketQ
	client                     pusher
	aggregationPeriodInSeconds uint32
	maxSeriesInSingleBatch     int
}

// flush flushes the queued buckets sending them to the remote Cloud service.
// If the number of time series collected is bigger than maximum batch size
// then it splits in chunks.
func (f *metricsFlusher) flush(_ context.Context) error {
	// drain the buffer
	buckets := f.bq.PopAll()
	if len(buckets) < 1 {
		return nil
	}

	// Pivot the data structure from a slice of Timebuckets
	// to a metric set of time series where each has nested samples.
	//
	// The Protobuf payload structure has the metric as the first level
	// instead of the current slice of buckets that knows about the metric
	// only in deeply nested levels. So, we need to go through the buckets
	// and group them by metric. To avoid doing too many loops and allocations,
	// the metricSetBuilder is used for doing it during the traverse of the buckets.

	msb := newMetricSetBuilder(f.referenceID, f.aggregationPeriodInSeconds)
	for i := 0; i < len(buckets); i++ {
		msb.addTimeBucket(buckets[i])
		if len(msb.seriesIndex) < f.maxSeriesInSingleBatch {
			continue
		}

		// we hit the chunk size, let's flush
		err := f.client.push(f.referenceID, msb.MetricSet)
		if err != nil {
			return err
		}
		msb = newMetricSetBuilder(f.referenceID, f.aggregationPeriodInSeconds)
	}

	if len(msb.seriesIndex) < 1 {
		return nil
	}

	// send the last (or the unique) MetricSet chunk to the remote service
	return f.client.push(f.referenceID, msb.MetricSet)
}

type metricSetBuilder struct {
	MetricSet                  *pbcloud.MetricSet
	TestRunID                  string
	AggregationPeriodInSeconds uint32

	// TODO: If we will introduce the metricID then we could
	// just use it as map's key (map[uint64]pbcloud.Metric). It is faster.
	// Or maybe, when we will have a better vision around the dynamic tracking
	// for metrics (https://github.com/grafana/k6/issues/1321) then we could consider
	// if an array, with the length equals to the number of registered metrics,
	// could eventually work.
	//
	// TODO: we may evaluate to replace it with
	// map[*metrics.Metric][]*pbcloud.TimeSeries)
	// and use a sync.Pool for the series slice.
	// We need dedicated benchmarks before doing it.
	//
	// metrics tracks the related metric conversion
	// into a protobuf structure.
	metrics map[*metrics.Metric]*pbcloud.Metric

	// seriesIndex tracks the index of the time series XYZ
	// in the related slice in
	// metrics[XYZ].<pbcloud.Metric>.TimeSeries.
	// It supports the iterative process for appending
	// the aggregated measurements for each time series.
	seriesIndex map[metrics.TimeSeries]uint
}

func newMetricSetBuilder(testRunID string, aggrPeriodSec uint32) metricSetBuilder {
	return metricSetBuilder{
		TestRunID:                  testRunID,
		MetricSet:                  &pbcloud.MetricSet{},
		AggregationPeriodInSeconds: aggrPeriodSec,
		// TODO: evaluate if removing the pointer from pbcloud.Metric
		// is a better trade-off
		metrics:     make(map[*metrics.Metric]*pbcloud.Metric),
		seriesIndex: make(map[metrics.TimeSeries]uint),
	}
}

func (msb *metricSetBuilder) addTimeBucket(bucket timeBucket) {
	for timeSeries, sink := range bucket.Sinks {
		pbmetric, ok := msb.metrics[timeSeries.Metric]
		if !ok {
			pbmetric = &pbcloud.Metric{
				Name: timeSeries.Metric.Name,
				Type: mapMetricTypeProto(timeSeries.Metric.Type),
			}
			msb.metrics[timeSeries.Metric] = pbmetric
			msb.MetricSet.Metrics = append(msb.MetricSet.Metrics, pbmetric)
		}

		var pbTimeSeries *pbcloud.TimeSeries
		ix, ok := msb.seriesIndex[timeSeries]
		if !ok {
			pbTimeSeries = &pbcloud.TimeSeries{
				AggregationPeriod: msb.AggregationPeriodInSeconds,
				Labels:            mapTimeSeriesLabelsProto(timeSeries, msb.TestRunID),
			}
			pbmetric.TimeSeries = append(pbmetric.TimeSeries, pbTimeSeries)
			msb.seriesIndex[timeSeries] = uint(len(pbmetric.TimeSeries) - 1)
		} else {
			pbTimeSeries = pbmetric.TimeSeries[ix]
		}

		addBucketToTimeSeriesProto(
			pbTimeSeries, timeSeries.Metric.Type, bucket.Time, sink)
	}
}
