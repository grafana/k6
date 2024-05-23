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

	counters       sync.Map
	upDownCounters sync.Map
	histograms     sync.Map
}

// newRegistry creates a new registry.
func newRegistry(meter otelMetric.Meter, logger logrus.FieldLogger) *registry {
	return &registry{
		meter:  meter,
		logger: logger,
	}
}

func (r *registry) getOrCreateCounter(name string) (otelMetric.Float64Counter, error) {
	if counter, ok := r.counters.Load(name); ok {
		if v, ok := counter.(otelMetric.Float64Counter); ok {
			return v, nil
		}

		return nil, fmt.Errorf("metric %q is not a counter", name)
	}

	c, err := r.meter.Float64Counter(name)
	if err != nil {
		return nil, fmt.Errorf("failed to create counter for %q: %w", name, err)
	}

	r.logger.Debugf("registered counter metric %q", name)

	r.counters.Store(name, c)
	return c, nil
}

func (r *registry) getOrCreateHistogram(name string) (otelMetric.Float64Histogram, error) {
	if histogram, ok := r.histograms.Load(name); ok {
		if v, ok := histogram.(otelMetric.Float64Histogram); ok {
			return v, nil
		}

		return nil, fmt.Errorf("metric %q is not a histogram", name)
	}

	h, err := r.meter.Float64Histogram(name)
	if err != nil {
		return nil, fmt.Errorf("failed to create histogram for %q: %w", name, err)
	}

	r.logger.Debugf("registered histogram metric %q", name)

	r.histograms.Store(name, h)
	return h, nil
}

func (r *registry) getOrCreateUpDownCounter(name string) (otelMetric.Float64UpDownCounter, error) {
	if counter, ok := r.upDownCounters.Load(name); ok {
		if v, ok := counter.(otelMetric.Float64UpDownCounter); ok {
			return v, nil
		}

		return nil, fmt.Errorf("metric %q is not an up/down counter", name)
	}

	c, err := r.meter.Float64UpDownCounter(name)
	if err != nil {
		return nil, fmt.Errorf("failed to create up/down counter for %q: %w", name, err)
	}

	r.logger.Debugf("registered up/down counter (gauge) metric %q ", name)

	r.upDownCounters.Store(name, c)
	return c, nil
}
