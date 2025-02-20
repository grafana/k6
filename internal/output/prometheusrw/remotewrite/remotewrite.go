// Package remotewrite is a k6 output that sends metrics to a Prometheus remote write endpoint.
package remotewrite

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.k6.io/k6/internal/output/prometheusrw/remote"
	"go.k6.io/k6/internal/output/prometheusrw/stale"

	"go.k6.io/k6/metrics"
	"go.k6.io/k6/output"

	prompb "buf.build/gen/go/prometheus/prometheus/protocolbuffers/go"
	"github.com/sirupsen/logrus"
)

var _ output.Output = new(Output)

// Output is a k6 output that sends metrics to a Prometheus remote write endpoint.
type Output struct {
	output.SampleBuffer

	config             Config
	logger             logrus.FieldLogger
	now                func() time.Time
	periodicFlusher    *output.PeriodicFlusher
	tsdb               map[metrics.TimeSeries]*seriesWithMeasure
	trendStatsResolver map[string]func(*metrics.TrendSink) float64

	// TODO: copy the prometheus/remote.WriteClient interface and depend on it
	client *remote.WriteClient
}

// New creates a new Output instance.
func New(params output.Params) (*Output, error) {
	logger := params.Logger.WithFields(logrus.Fields{"output": "Prometheus remote write"})

	config, err := GetConsolidatedConfig(params.JSONConfig, params.Environment, params.ConfigArgument)
	if err != nil {
		return nil, err
	}

	clientConfig, err := config.RemoteConfig()
	if err != nil {
		return nil, err
	}

	wc, err := remote.NewWriteClient(config.ServerURL.String, clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize the Prometheus remote write client: %w", err)
	}

	o := &Output{
		client: wc,
		config: config,
		// TODO: consider to do this function millisecond-based
		// so we don't need to truncate all the time we invoke it.
		// Before we should analyze if in some cases is it useful to have it in ns.
		now:    time.Now,
		logger: logger,
		tsdb:   make(map[metrics.TimeSeries]*seriesWithMeasure),
	}

	if len(config.TrendStats) > 0 {
		if err := o.setTrendStatsResolver(config.TrendStats); err != nil {
			return nil, err
		}
	}
	return o, nil
}

// Description returns a short human-readable description of the output.
func (o *Output) Description() string {
	return fmt.Sprintf("Prometheus remote write (%s)", o.config.ServerURL.String)
}

// Start initializes the output.
func (o *Output) Start() error {
	d := o.config.PushInterval.TimeDuration()
	periodicFlusher, err := output.NewPeriodicFlusher(d, o.flush)
	if err != nil {
		return err
	}
	o.periodicFlusher = periodicFlusher
	o.logger.WithField("flushtime", d).Debug("Output initialized")
	return nil
}

// Stop stops the output.
func (o *Output) Stop() error {
	o.logger.Debug("Stopping the output")
	defer o.logger.Debug("Output stopped")
	o.periodicFlusher.Stop()

	if !o.config.StaleMarkers.Bool {
		return nil
	}
	staleMarkers := o.staleMarkers()
	if len(staleMarkers) < 1 {
		o.logger.Debug("No time series to mark as stale")
		return nil
	}
	o.logger.WithField("staleMarkers", len(staleMarkers)).Debug("Marking time series as stale")

	err := o.client.Store(context.Background(), staleMarkers)
	if err != nil {
		return fmt.Errorf("marking time series as stale failed: %w", err)
	}
	return nil
}

// staleMarkers maps all the seen time series with a stale marker.
func (o *Output) staleMarkers() []*prompb.TimeSeries {
	// Add 1ms so in the extreme case that the time frame
	// between the last and the next flush operation is under-millisecond,
	// we can avoid the sample being seen as a duplicate,
	// if we force it in the future.
	// It is essential because if it overlaps, the remote write discards the last sample,
	// so the stale marker and the metric will remain active for the next 5 min
	// as the default logic without stale markers.
	timestamp := o.now().
		Truncate(time.Millisecond).Add(1 * time.Millisecond).UnixMilli()

	staleMarkers := make([]*prompb.TimeSeries, 0, len(o.tsdb))
	for _, swm := range o.tsdb {
		series := swm.MapPrompb()
		// series' length is expected to be equal to 1 for most of the cases
		// the unique exception where more than 1 is expected is when
		// trend stats have been configured with multiple values.
		for _, s := range series {
			if len(s.Samples) < 1 {
				if len(s.Histograms) < 1 {
					panic("data integrity check: samples and native histograms" +
						" can't be empty at the same time")
				}
				s.Samples = append(s.Samples, &prompb.Sample{})
			}

			s.Samples[0].Value = stale.Marker
			s.Samples[0].Timestamp = timestamp
		}
		staleMarkers = append(staleMarkers, series...)
	}
	return staleMarkers
}

// setTrendStatsResolver sets the resolver for the Trend stats.
//
// TODO: refactor, the code can be improved
func (o *Output) setTrendStatsResolver(trendStats []string) error {
	trendStatsCopy := make([]string, 0, len(trendStats))
	hasSum := false
	// copy excluding sum
	for _, stat := range trendStats {
		if stat == "sum" {
			hasSum = true
			continue
		}
		trendStatsCopy = append(trendStatsCopy, stat)
	}
	resolvers, err := metrics.GetResolversForTrendColumns(trendStatsCopy)
	if err != nil {
		return err
	}
	// sum is not supported from GetResolversForTrendColumns
	// so if it has been requested
	// it adds it specifically
	if hasSum {
		resolvers["sum"] = func(t *metrics.TrendSink) float64 {
			return t.Total()
		}
	}
	o.trendStatsResolver = make(TrendStatsResolver, len(resolvers))
	for stat, fn := range resolvers {
		statKey := stat

		// the config passes percentiles with p(x) form, for example p(95),
		// but the mapping generates series name in the form p95.
		//
		// TODO: maybe decoupling mapping from the stat resolver keys?
		if strings.HasPrefix(statKey, "p(") {
			statKey = stat[2 : len(statKey)-1]             // trim the parenthesis
			statKey = strings.ReplaceAll(statKey, ".", "") // remove dots, p(0.95) => p095
			statKey = "p" + statKey
		}
		o.trendStatsResolver[statKey] = fn
	}
	return nil
}

func (o *Output) flush() {
	var (
		start = time.Now()
		nts   int
	)

	defer func() {
		d := time.Since(start)
		okmsg := "Successful flushed time series to remote write endpoint"
		if d > time.Duration(o.config.PushInterval.Duration) {
			// There is no intermediary storage so warn if writing to remote write endpoint becomes too slow
			o.logger.WithField("nts", nts).
				Warnf("%s but it took %s while flush period is %s. Some samples may be dropped.",
					okmsg, d.String(), o.config.PushInterval.String())
		} else {
			o.logger.WithField("nts", nts).WithField("took", d).Debug(okmsg)
		}
	}()

	samplesContainers := o.GetBufferedSamples()
	if len(samplesContainers) < 1 {
		o.logger.Debug("no buffered samples, skip the flushing operation")
		return
	}

	// Remote write endpoint accepts TimeSeries structure defined in gRPC. It must:
	// a) contain Labels array
	// b) have a __name__ label: without it, metric might be unquerable or even rejected
	// as a metric without a name. This behaviour depends on underlying storage used.
	// c) not have duplicate timestamps within 1 timeseries, see https://github.com/prometheus/prometheus/issues/9210
	// Prometheus write handler processes only some fields as of now, so here we'll add only them.

	promTimeSeries := o.convertToPbSeries(samplesContainers)
	nts = len(promTimeSeries)
	o.logger.WithField("nts", nts).Debug("Converted samples to Prometheus TimeSeries")

	if err := o.client.Store(context.Background(), promTimeSeries); err != nil {
		o.logger.WithError(err).Error("Failed to send the time series data to the endpoint")
		return
	}
}

func (o *Output) convertToPbSeries(samplesContainers []metrics.SampleContainer) []*prompb.TimeSeries {
	// The seen map is required because the samples containers
	// could have several samples for the same time series
	//  in this way, we can aggregate and flush them in a unique value
	//  without overloading the remote write endpoint.
	//
	// It is also essential because the core generates timestamps
	// with a higher precision (ns) than Prometheus (ms),
	// so we need to aggregate all the samples in the same time bucket.
	// More context can be found in the issue
	// https://github.com/grafana/xk6-output-prometheus-remote/issues/11
	seen := make(map[metrics.TimeSeries]struct{})

	for _, samplesContainer := range samplesContainers {
		samples := samplesContainer.GetSamples()

		for _, sample := range samples {
			truncTime := sample.Time.Truncate(time.Millisecond)
			swm, ok := o.tsdb[sample.TimeSeries]
			if !ok {
				// TODO: encapsulate the trend arguments into a Trend Mapping factory
				swm = newSeriesWithMeasure(sample.TimeSeries, o.config.TrendAsNativeHistogram.Bool, o.trendStatsResolver)
				swm.Latest = truncTime
				o.tsdb[sample.TimeSeries] = swm
				seen[sample.TimeSeries] = struct{}{}
			} else { //nolint:gocritic
				// FIXME: remove the gocritic linter inhibition as soon as the rest of the todo are done
				// save as a seen item only when the samples have a time greater than
				// the previous saved, otherwise some implementations
				// could see it as a duplicate and generate warnings (e.g. Mimir)
				if truncTime.After(swm.Latest) {
					swm.Latest = truncTime
					seen[sample.TimeSeries] = struct{}{}
				}

				// If current == previous:
				// the current received time before being truncated had a higher precision.
				// It's fine to aggregate them but we avoid to add to the seen map because:
				// - in the case it is a new flush operation then we avoid delivering
				//   for not generating duplicates
				// - in the case it is in the same operation but across sample containers
				//   then the time series should be already on the seen map and we can skip
				//   to re-add it.

				// If current < previous:
				// - in the case current is a new flush operation, it shouldn't happen,
				//   for this reason, we can avoid creating a dedicated logic.
				//   TODO: We should evaluate if it would be better to have a defensive condition
				//   for handling it, logging a warning or returning an error
				//   and avoid aggregating the value.
				// - in the case current is in the same operation but across sample containers
				//   it's fine to aggregate
				//   but same as for the equal condition it can rely on the previous seen value.
			}
			swm.Measure.Add(sample)
		}
	}

	pbseries := make([]*prompb.TimeSeries, 0, len(seen))
	for s := range seen {
		pbseries = append(pbseries, o.tsdb[s].MapPrompb()...)
	}
	return pbseries
}

type seriesWithMeasure struct {
	metrics.TimeSeries
	Measure metrics.Sink

	// Latest tracks the latest time
	// when the measure has been updated
	//
	// TODO: the logic for this value should stay directly
	// in a method in struct
	Latest time.Time

	// TODO: maybe add some caching for the mapping?
}

// TODO: add unit tests
func (swm seriesWithMeasure) MapPrompb() []*prompb.TimeSeries {
	var newts []*prompb.TimeSeries

	mapMonoSeries := func(s metrics.TimeSeries, suffix string, t time.Time) prompb.TimeSeries {
		return prompb.TimeSeries{
			Labels: MapSeries(s, suffix),
			Samples: []*prompb.Sample{
				{Timestamp: t.UnixMilli()},
			},
		}
	}

	//nolint:forcetypeassert
	switch swm.Metric.Type {
	case metrics.Counter:
		ts := mapMonoSeries(swm.TimeSeries, "total", swm.Latest)
		ts.Samples[0].Value = swm.Measure.(*metrics.CounterSink).Value
		newts = []*prompb.TimeSeries{&ts}

	case metrics.Gauge:
		ts := mapMonoSeries(swm.TimeSeries, "", swm.Latest)
		ts.Samples[0].Value = swm.Measure.(*metrics.GaugeSink).Value
		newts = []*prompb.TimeSeries{&ts}

	case metrics.Rate:
		ts := mapMonoSeries(swm.TimeSeries, "rate", swm.Latest)
		// pass zero duration here because time is useless for formatting rate
		rateVals := swm.Measure.(*metrics.RateSink).Format(time.Duration(0))
		ts.Samples[0].Value = rateVals["rate"]
		newts = []*prompb.TimeSeries{&ts}

	case metrics.Trend:
		// TODO:
		//	- Add a PrompbMapSinker interface
		//    and implements it on all the sinks "extending" them.
		//  - Call directly MapPrompb on Measure without any type assertion.
		trend, ok := swm.Measure.(prompbMapper)
		if !ok {
			panic("Measure for Trend types must implement MapPromPb")
		}
		newts = trend.MapPrompb(swm.TimeSeries, swm.Latest)

	default:
		panic(
			fmt.Sprintf(
				"the output reached an unrecoverable state; unable to recognize processed metric %s's type `%s`",
				swm.Metric.Name,
				swm.Metric.Type,
			),
		)
	}
	return newts
}

type prompbMapper interface {
	MapPrompb(series metrics.TimeSeries, t time.Time) []*prompb.TimeSeries
}

func newSeriesWithMeasure(
	series metrics.TimeSeries,
	trendAsNativeHistogram bool,
	tsr TrendStatsResolver,
) *seriesWithMeasure {
	var sink metrics.Sink
	switch series.Metric.Type {
	case metrics.Counter:
		sink = &metrics.CounterSink{}
	case metrics.Gauge:
		sink = &metrics.GaugeSink{}
	case metrics.Trend:
		// TODO: refactor encapsulating in a factory method
		if trendAsNativeHistogram {
			sink = newNativeHistogramSink(series.Metric)
		} else {
			var err error
			sink, err = newExtendedTrendSink(tsr)
			if err != nil {
				// the resolver must be already validated
				panic(err)
			}
		}
	case metrics.Rate:
		sink = &metrics.RateSink{}
	default:
		panic(fmt.Sprintf("metric type %q unsupported", series.Metric.Type.String()))
	}
	return &seriesWithMeasure{
		TimeSeries: series,
		Measure:    sink,
	}
}
