// Package opentelemetry performs output operations for the opentelemetry extension
package opentelemetry

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	otelMetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"

	"go.k6.io/k6/metrics"
	"go.k6.io/k6/output"
)

// Output implements the lib.Output interface
type Output struct {
	output.SampleBuffer

	config          Config
	periodicFlusher *output.PeriodicFlusher
	logger          logrus.FieldLogger

	meterProvider   *metric.MeterProvider
	metricsRegistry *registry
}

var _ output.WithStopWithTestError = new(Output)

// New creates an instance of the collector
func New(p output.Params) (*Output, error) {
	conf, err := GetConsolidatedConfig(p.JSONConfig, p.Environment)
	if err != nil {
		return nil, err
	}

	if conf.ExporterType.Valid {
		p.Logger.Warn("Exporter type is deprecated, please migrate to exporter protocol")
	}

	return &Output{
		config: conf,
		logger: p.Logger,
	}, nil
}

// Description returns a human-readable description of the output that will be shown in `k6 run`
func (o *Output) Description() string {
	return fmt.Sprintf("opentelemetry (%s)", o.config)
}

// StopWithTestError flushes all remaining metrics and finalizes the test run
func (o *Output) StopWithTestError(_ error) error {
	o.logger.Debug("Stopping...")
	defer o.logger.Debug("Stopped!")

	o.periodicFlusher.Stop()

	if err := o.meterProvider.Shutdown(context.Background()); err != nil {
		o.logger.WithError(err).Error("can't shutdown OpenTelemetry metric provider")
	}

	return nil
}

// Stop just implements an old interface (output.Output)
func (o *Output) Stop() error {
	return o.StopWithTestError(nil)
}

// Start performs initialization tasks prior to Engine using the output
func (o *Output) Start() error {
	o.logger.Debug("Starting output...")

	if !o.config.SingleCounterForRate.Bool {
		o.logger.Warn("Exporting rate metrics as a pair of counters is deprecated" +
			" and will be removed in future releases. Please migrate to the new format.")
	}

	exp, err := getExporter(o.config)
	if err != nil {
		return fmt.Errorf("failed to create OpenTelemetry exporter: %w", err)
	}

	res, err := resource.Merge(resource.Default(),
		resource.NewSchemaless(
			semconv.ServiceName(o.config.ServiceName.String),
			semconv.ServiceVersion(o.config.ServiceVersion.String),
		))
	if err != nil {
		return fmt.Errorf("failed to create OpenTelemetry resource: %w", err)
	}

	// Define custom histogram buckets: 50ms to 60s
	// Buckets in milliseconds: 50, 75, 100, 150, 200, 300, 500, 750, 1000, 1500, 2000, 3000, 5000, 7500, 10000, 15000, 20000, 30000, 45000, 60000
	histogramBuckets := []float64{
		50, 75, 100, 150, 200, 300, 500, 750,
		1000, 1500, 2000, 3000, 5000, 7500,
		10000, 15000, 20000, 30000, 45000, 60000,
	}

	// Create histogram aggregation with custom buckets
	histogramAggregation := metric.AggregationExplicitBucketHistogram{
		Boundaries: histogramBuckets,
		NoMinMax:   false,
	}

	// Create a view function that matches any histogram and applies custom buckets
	histogramViewFunc := func(i metric.Instrument) (metric.Stream, bool) {
		// Only apply to histogram instruments
		if i.Kind != metric.InstrumentKindHistogram {
			return metric.Stream{}, false
		}

		o.logger.Debugf("Applying custom histogram buckets to instrument: %s (kind: %v)", i.Name, i.Kind)

		// Return the stream with custom aggregation for all histograms
		return metric.Stream{
			Name:        i.Name,
			Description: i.Description,
			Unit:        i.Unit,
			Aggregation: histogramAggregation,
		}, true
	}

	o.logger.Debugf("Created histogram view with %d custom boundaries ranging from %.0fms to %.0fms",
		len(histogramBuckets), histogramBuckets[0], histogramBuckets[len(histogramBuckets)-1])

	meterProvider := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(
			metric.NewPeriodicReader(
				exp,
				metric.WithInterval(o.config.ExportInterval.TimeDuration()),
			),
		),
		metric.WithView(histogramViewFunc),
	)
	pf, err := output.NewPeriodicFlusher(o.config.FlushInterval.TimeDuration(), o.flushMetrics)
	if err != nil {
		return err
	}

	o.logger.Debug("Started!")
	o.periodicFlusher = pf
	o.meterProvider = meterProvider
	o.metricsRegistry = newRegistry(meterProvider.Meter("k6"), o.logger)

	return nil
}

func (o *Output) flushMetrics() {
	samples := o.GetBufferedSamples()
	start := time.Now()
	var count, errCount int
	for _, sc := range samples {
		samples := sc.GetSamples()

		for _, sample := range samples {
			if err := o.dispatch(sample); err != nil {
				o.logger.WithError(err).Error("Error dispatching sample")
				errCount++

				continue
			}
			count++
		}
	}

	if count > 0 {
		o.logger.
			WithField("t", time.Since(start)).
			WithField("count", count).
			Debug("registered metrics in OpenTelemetry metric provider")
	}

	if errCount > 0 {
		o.logger.
			WithField("t", time.Since(start)).
			WithField("count", errCount).
			Warn("can't flush some metrics")
	}
}

func (o *Output) dispatch(entry metrics.Sample) error {
	ctx := context.Background()
	name := normalizeMetricName(o.config, entry.Metric.Name)

	attributeSet := newAttributeSet(entry.Tags)
	attributeSetOpt := otelMetric.WithAttributeSet(attributeSet)

	unit := normalizeUnit(entry.Metric.Contains)

	switch entry.Metric.Type {
	case metrics.Counter:
		counter, err := o.metricsRegistry.getOrCreateCounter(name, unit)
		if err != nil {
			return err
		}

		counter.Add(ctx, entry.Value, attributeSetOpt)
	case metrics.Gauge:
		gauge, err := o.metricsRegistry.getOrCreateGauge(name, unit)
		if err != nil {
			return err
		}

		gauge.Record(ctx, entry.Value, attributeSetOpt)
	case metrics.Trend:
		trend, err := o.metricsRegistry.getOrCreateHistogram(name, unit)
		if err != nil {
			return err
		}

		trend.Record(ctx, entry.Value, attributeSetOpt)
	case metrics.Rate:
		var err error
		if o.config.SingleCounterForRate.Bool {
			err = o.singleCounterForRate(ctx, name, attributeSetOpt, entry)
		} else {
			// Deprecated path, remove with https://github.com/grafana/k6/issues/5185
			err = o.pairOfCountersForRate(ctx, name, attributeSetOpt, entry)
		}
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("metric %q has unsupported metric type", entry.Metric.Name)
	}
	return nil
}

func (o *Output) pairOfCountersForRate(
	ctx context.Context,
	metricName string,
	attributeSetOpt otelMetric.MeasurementOption,
	entry metrics.Sample,
) error {
	nonZero, total, err := o.metricsRegistry.getOrCreateCountersForRate(metricName)
	if err != nil {
		return fmt.Errorf("get or create counter for Rate metric %q: %w", metricName, err)
	}
	if entry.Value != 0 {
		nonZero.Add(ctx, 1, attributeSetOpt)
	}
	total.Add(ctx, 1, attributeSetOpt)
	return nil
}

func (o *Output) singleCounterForRate(
	ctx context.Context,
	metricName string,
	attributeSetOpt otelMetric.MeasurementOption,
	entry metrics.Sample,
) error {
	rate, err := o.metricsRegistry.getOrCreateCounterForRate(metricName)
	if err != nil {
		return fmt.Errorf("get or create counter for Rate metric %q: %w", metricName, err)
	}
	var valueType string
	if entry.Value != 0 {
		valueType = "nonzero"
	} else {
		valueType = "zero"
	}
	valset := attribute.NewSet(attribute.String("condition", valueType))
	rate.Add(ctx, 1, attributeSetOpt, otelMetric.WithAttributeSet(valset))
	return nil
}

func normalizeMetricName(cfg Config, name string) string {
	return cfg.MetricPrefix.String + name
}

func normalizeUnit(vt metrics.ValueType) string {
	switch vt {
	case metrics.Time:
		return "ms"
	case metrics.Data:
		return "By"
	default:
		return ""
	}
}
