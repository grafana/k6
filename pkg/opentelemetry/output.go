// Package opentelemetry performs output operations for the opentelemetry extension
package opentelemetry

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
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

	meterProvider *metric.MeterProvider
	meter         otelMetric.Meter

	counters       sync.Map
	upDownCounters sync.Map
	histograms     sync.Map
}

var _ output.WithStopWithTestError = new(Output)

// New creates an instance of the collector
func New(p output.Params) (*Output, error) {
	conf, err := NewConfig(p)
	if err != nil {
		return nil, err
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

	if err := o.meterProvider.Shutdown(context.Background()); err != nil {
		o.logger.WithError(err).Error("can't shutdown OpenTelemetry metric provider")
	}

	o.periodicFlusher.Stop()

	return nil
}

// Stop just implements an old interface (output.Output)
func (o *Output) Stop() error {
	return o.StopWithTestError(nil)
}

// Start performs initialization tasks prior to Engine using the output
func (o *Output) Start() error {
	o.logger.Debug("Starting output...")

	exp, err := getExporter(o.config)
	if err != nil {
		return fmt.Errorf("failed to create OpenTelemetry exporter: %w", err)
	}

	res, err := resource.Merge(resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL,
			semconv.ServiceName(o.config.ServiceName),
			semconv.ServiceVersion(o.config.ServiceVersion),
		))
	if err != nil {
		return fmt.Errorf("failed to create OpenTelemetry resource: %w", err)
	}

	meterProvider := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(
			metric.NewPeriodicReader(
				exp,
				metric.WithInterval(o.config.ExportInterval),
			),
		),
	)

	pf, err := output.NewPeriodicFlusher(o.config.FlushInterval, o.flushMetrics)
	if err != nil {
		return err
	}

	o.logger.Debug("Started!")
	o.periodicFlusher = pf
	o.meterProvider = meterProvider
	o.meter = meterProvider.Meter("k6")

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

	switch entry.Metric.Type {
	case metrics.Counter:
		counter, err := o.getOrCreateCounter(name)
		if err != nil {
			return err
		}

		counter.Add(ctx, entry.Value, otelMetric.WithAttributes(MapTagSet(entry.Tags)...))
	case metrics.Gauge:
		gauge, err := o.getOrCreateUpDownCounter(name)
		if err != nil {
			return err
		}

		gauge.Add(ctx, entry.Value, otelMetric.WithAttributes(MapTagSet(entry.Tags)...))
	case metrics.Trend:
		trend, err := o.getOrCreateHistogram(name)
		if err != nil {
			return err
		}

		trend.Record(ctx, entry.Value, otelMetric.WithAttributes(MapTagSet(entry.Tags)...))
	default:
		// TODO: add support for other metric types
		o.logger.Debugf("Drop unsupported metric type: %s", entry.Metric.Name)
	}

	return nil
}

func normalizeMetricName(cfg Config, name string) string {
	return cfg.MetricPrefix + name
}

func (o *Output) getOrCreateCounter(name string) (otelMetric.Float64Counter, error) {
	if counter, ok := o.counters.Load(name); ok {
		if v, ok := counter.(otelMetric.Float64Counter); ok {
			return v, nil
		}

		return nil, fmt.Errorf("metric %q is not a counter", name)
	}

	c, err := o.meter.Float64Counter(name)
	if err != nil {
		return nil, fmt.Errorf("failed to create counter for %q: %w", name, err)
	}

	o.logger.Debugf("registered counter metric %q", name)

	o.counters.Store(name, c)
	return c, nil
}

func (o *Output) getOrCreateHistogram(name string) (otelMetric.Float64Histogram, error) {
	if histogram, ok := o.histograms.Load(name); ok {
		if v, ok := histogram.(otelMetric.Float64Histogram); ok {
			return v, nil
		}

		return nil, fmt.Errorf("metric %q is not a histogram", name)
	}

	h, err := o.meter.Float64Histogram(name)
	if err != nil {
		return nil, fmt.Errorf("failed to create histogram for %q: %w", name, err)
	}

	o.logger.Debugf("registered histogram metric %q", name)

	o.histograms.Store(name, h)
	return h, nil
}

func (o *Output) getOrCreateUpDownCounter(name string) (otelMetric.Float64UpDownCounter, error) {
	if counter, ok := o.upDownCounters.Load(name); ok {
		if v, ok := counter.(otelMetric.Float64UpDownCounter); ok {
			return v, nil
		}

		return nil, fmt.Errorf("metric %q is not an up/down counter", name)
	}

	c, err := o.meter.Float64UpDownCounter(name)
	if err != nil {
		return nil, fmt.Errorf("failed to create up/down counter for %q: %w", name, err)
	}

	o.logger.Debugf("registered up/down counter (gauge) metric %q ", name)

	o.upDownCounters.Store(name, c)
	return c, nil
}
