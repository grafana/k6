package opentelemetry

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// NewFloat64Gauge returns a new Float64Gauge.
func NewFloat64Gauge() *Float64Gauge {
	return &Float64Gauge{}
}

// Float64Gauge is a temporary implementation of the OpenTelemetry sync gauge
// it will be replaced by the official implementation once it's available.
//
// https://github.com/open-telemetry/opentelemetry-go/issues/3984
type Float64Gauge struct {
	observations sync.Map
}

// Callback implements the callback function for the underlying asynchronous gauge
// it observes the current state of all previous Set() calls.
func (f *Float64Gauge) Callback(_ context.Context, o metric.Float64Observer) error {
	var err error

	f.observations.Range(func(key, value interface{}) bool {
		var v float64

		// TODO: improve type assertion
		switch val := value.(type) {
		case float64:
			v = val
		case int64:
			v = float64(val)
		default:
			err = errors.New("unexpected type for value " + fmt.Sprintf("%T", val))
			return false
		}

		attrs, ok := key.(attribute.Set)
		if !ok {
			err = errors.New("unexpected type for key")
			return false
		}

		o.Observe(v, metric.WithAttributeSet(attrs))

		return true
	})

	return err
}

// Set sets the value of the gauge.
func (f *Float64Gauge) Set(val float64, attrs attribute.Set) {
	f.observations.Store(attrs, val)
}

// Delete deletes the gauge.
func (f *Float64Gauge) Delete(attrs attribute.Set) {
	f.observations.Delete(attrs)
}
