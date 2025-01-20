package expv2

import (
	"time"

	"github.com/sirupsen/logrus"

	"go.k6.io/k6/internal/output/cloud/expv2/pbcloud"
	"go.k6.io/k6/metrics"
)

type pusher interface {
	push(samples *pbcloud.MetricSet) error
}

type metricsFlusher struct {
	testRunID                  string
	bq                         *bucketQ
	client                     pusher
	logger                     logrus.FieldLogger
	discardedLabels            map[string]struct{}
	aggregationPeriodInSeconds uint32
	maxSeriesInBatch           int
	batchPushConcurrency       int
}

// flush flushes the queued buckets sending them to the remote Cloud service.
// If the number of time series collected is bigger than maximum batch size
// then it splits in chunks.
func (f *metricsFlusher) flush() error {
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

	var (
		start       = time.Now()
		batches     []*pbcloud.MetricSet
		seriesCount int
	)

	defer func() {
		f.logger.
			WithField("t", time.Since(start)).
			WithField("series", seriesCount).
			WithField("buckets", len(buckets)).
			WithField("batches", len(batches)).Debug("Flush the queued buckets")
	}()

	msb := newMetricSetBuilder(f.testRunID, f.aggregationPeriodInSeconds)
	for i := 0; i < len(buckets); i++ {
		for timeSeries, sink := range buckets[i].Sinks {
			msb.addTimeSeries(buckets[i].Time, timeSeries, sink)
			if len(msb.seriesIndex) < f.maxSeriesInBatch {
				continue
			}

			// We hit the batch size, let's flush
			seriesCount += len(msb.seriesIndex)
			batches = append(batches, msb.MetricSet)
			f.reportDiscardedLabels(msb.discardedLabels)

			// Reset the builder
			msb = newMetricSetBuilder(f.testRunID, f.aggregationPeriodInSeconds)
		}
	}

	// send the last (or the unique) MetricSet chunk to the remote service
	if len(msb.seriesIndex) != 0 {
		seriesCount += len(msb.seriesIndex)
		batches = append(batches, msb.MetricSet)
		f.reportDiscardedLabels(msb.discardedLabels)
	}

	return f.flushBatches(batches)
}

func (f *metricsFlusher) flushBatches(batches []*pbcloud.MetricSet) error {
	var (
		workers  = min(len(batches), f.batchPushConcurrency)
		errs     = make(chan error, workers)
		feed     = make(chan *pbcloud.MetricSet)
		finalErr error
	)

	for i := 0; i < workers; i++ {
		go func() {
			for chunk := range feed {
				if err := f.client.push(chunk); err != nil {
					errs <- err
					return
				}
			}
			errs <- nil
		}()
	}

outer:
	for i := 0; i < len(batches); i++ {
		select {
		case err := <-errs:
			workers--
			finalErr = err
			break outer
		case feed <- batches[i]:
		}
	}

	close(feed)

	for ; workers != 0; workers-- {
		err := <-errs
		if err != nil && finalErr == nil {
			finalErr = err
		}
	}
	return finalErr
}

func (f *metricsFlusher) reportDiscardedLabels(discardedLabels map[string]struct{}) {
	for key := range discardedLabels {
		if _, ok := f.discardedLabels[key]; ok {
			continue
		}

		f.discardedLabels[key] = struct{}{}
		f.logger.Warnf("Tag %s has been discarded since it is reserved for Cloud operations.", key)
	}
}

type metricSetBuilder struct {
	MetricSet *pbcloud.MetricSet

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
	seriesIndex map[metrics.TimeSeries]int

	// discardedLabels tracks the labels that have been discarded
	// since they are reserved for internal usage by the Cloud service.
	discardedLabels map[string]struct{}
}

func newMetricSetBuilder(testRunID string, aggrPeriodSec uint32) metricSetBuilder {
	builder := metricSetBuilder{
		MetricSet: &pbcloud.MetricSet{},
		// TODO: evaluate if removing the pointer from pbcloud.Metric
		// is a better trade-off
		metrics:         make(map[*metrics.Metric]*pbcloud.Metric),
		seriesIndex:     make(map[metrics.TimeSeries]int),
		discardedLabels: nil,
	}
	builder.MetricSet.TestRunId = testRunID
	builder.MetricSet.AggregationPeriod = aggrPeriodSec
	return builder
}

func (msb *metricSetBuilder) addTimeSeries(timestamp int64, timeSeries metrics.TimeSeries, sink metricValue) {
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
	if ix, ok := msb.seriesIndex[timeSeries]; !ok {
		labels, discardedLabels := mapTimeSeriesLabelsProto(timeSeries.Tags)
		msb.recordDiscardedLabels(discardedLabels)

		pbTimeSeries = &pbcloud.TimeSeries{
			Labels: labels,
		}
		pbmetric.TimeSeries = append(pbmetric.TimeSeries, pbTimeSeries)
		msb.seriesIndex[timeSeries] = len(pbmetric.TimeSeries) - 1
	} else {
		pbTimeSeries = pbmetric.TimeSeries[ix]
	}
	addBucketToTimeSeriesProto(pbTimeSeries, timeSeries.Metric.Type, timestamp, sink)
}

func (msb *metricSetBuilder) recordDiscardedLabels(labels []string) {
	if len(labels) == 0 {
		return
	}

	if msb.discardedLabels == nil {
		msb.discardedLabels = make(map[string]struct{})
	}

	for _, key := range labels {
		if _, ok := msb.discardedLabels[key]; ok {
			continue
		}

		msb.discardedLabels[key] = struct{}{}
	}
}
