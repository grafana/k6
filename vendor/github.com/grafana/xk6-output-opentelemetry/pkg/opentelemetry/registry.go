package opentelemetry

import (
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"
	otelMetric "go.opentelemetry.io/otel/metric"
)

// registry keep track of all metrics that have been used.
type registry struct {
	meter  otelMetric.Meter
	logger logrus.FieldLogger

	counters     sync.Map
	gauges       sync.Map
	histograms   sync.Map
	rateCounters sync.Map
}

// newRegistry creates a new registry.
func newRegistry(meter otelMetric.Meter, logger logrus.FieldLogger) *registry {
	return &registry{
		meter:  meter,
		logger: logger,
	}
}

func (r *registry) getOrCreateCounter(name, unit string) (otelMetric.Float64Counter, error) {
	if counter, ok := r.counters.Load(name); ok {
		if v, ok := counter.(otelMetric.Float64Counter); ok {
			return v, nil
		}

		return nil, fmt.Errorf("metric %q is not a counter", name)
	}

	opts := []otelMetric.Float64CounterOption{}
	if unit != "" {
		opts = append(opts, otelMetric.WithUnit(unit))
	}

	c, err := r.meter.Float64Counter(name, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create counter for %q: %w", name, err)
	}

	r.logger.Debugf("registered counter metric %q", name)

	r.counters.Store(name, c)
	return c, nil
}

func (r *registry) getOrCreateHistogram(name, unit string) (otelMetric.Float64Histogram, error) {
	if histogram, ok := r.histograms.Load(name); ok {
		if v, ok := histogram.(otelMetric.Float64Histogram); ok {
			return v, nil
		}

		return nil, fmt.Errorf("metric %q is not a histogram", name)
	}

	opts := []otelMetric.Float64HistogramOption{}
	if unit != "" {
		opts = append(opts, otelMetric.WithUnit(unit))
	}

	h, err := r.meter.Float64Histogram(name, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create histogram for %q: %w", name, err)
	}

	r.logger.Debugf("registered histogram metric %q", name)

	r.histograms.Store(name, h)
	return h, nil
}

func (r *registry) getOrCreateCountersForRate(name string) (otelMetric.Int64Counter, otelMetric.Int64Counter, error) {
	// k6's rate metric tracks how frequently a non-zero value occurs.
	// so to correctly calculate the rate in a metrics backend
	// we need to split the rate metric into two counters:
	// 2. number of non-zero occurrences
	// 1. the total number of occurrences

	nonZeroName := name + ".occurred"
	totalName := name + ".total"

	var err error
	var nonZeroCounter, totalCounter otelMetric.Int64Counter

	storedNonZeroCounter, ok := r.rateCounters.Load(nonZeroName)
	if !ok {
		nonZeroCounter, err = r.meter.Int64Counter(nonZeroName)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create counter for %q: %w", nonZeroName, err)
		}

		r.rateCounters.Store(nonZeroName, nonZeroCounter)
		r.logger.Debugf("registered counter metric %q", nonZeroName)
	} else {
		nonZeroCounter, ok = storedNonZeroCounter.(otelMetric.Int64Counter)
		if !ok {
			return nil, nil, fmt.Errorf("metric %q stored not as counter", nonZeroName)
		}
	}

	storedTotalCounter, ok := r.rateCounters.Load(totalName)
	if !ok {
		totalCounter, err = r.meter.Int64Counter(totalName)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create counter for %q: %w", totalName, err)
		}

		r.rateCounters.Store(totalName, totalCounter)
		r.logger.Debugf("registered counter metric %q", totalName)
	} else {
		totalCounter, ok = storedTotalCounter.(otelMetric.Int64Counter)
		if !ok {
			return nil, nil, fmt.Errorf("metric %q stored not as counter", totalName)
		}
	}

	return nonZeroCounter, totalCounter, nil
}

func (r *registry) getOrCreateGauge(name, unit string) (otelMetric.Float64Gauge, error) {
	if gauge, ok := r.gauges.Load(name); ok {
		if v, ok := gauge.(otelMetric.Float64Gauge); ok {
			return v, nil
		}

		return nil, fmt.Errorf("metric %q is not a gauge", name)
	}

	opts := []otelMetric.Float64GaugeOption{}
	if unit != "" {
		opts = append(opts, otelMetric.WithUnit(unit))
	}

	gauge, err := r.meter.Float64Gauge(name, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create gauge for %q: %w", name, err)
	}

	r.logger.Debugf("registered gauge metric %q ", name)

	r.gauges.Store(name, gauge)
	return gauge, nil
}
