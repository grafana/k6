package tracing

import (
	"errors"
	"fmt"

	"github.com/dop251/goja"
	"go.k6.io/k6/js/common"
)

// options are the options that can be passed to the
// tracing.instrumentHTTP() method.
type options struct {
	// Propagation is the propagation format to use for the tracer.
	Propagator string `json:"propagator"`

	// Sampling is the sampling rate to use for the
	// tracer, expressed in percents within the
	// bounds: 0.0 <= n <= 1.0.
	Sampling float64 `json:"sampling"`

	// Baggage is a map of baggage items to add to the tracer.
	Baggage map[string]string `json:"baggage"`
}

// defaultSamplingRate is the default sampling rate applied to options.
const defaultSamplingRate float64 = 1.0

// newOptions returns a new options object from the given goja.Value.
//
// Note that if the sampling field value is absent, or nullish, we'll
// set it to the `defaultSamplingRate` value.
func newOptions(rt *goja.Runtime, from goja.Value) (options, error) {
	var opts options

	if err := rt.ExportTo(from, &opts); err != nil {
		return opts, fmt.Errorf("unable to parse options object; reason: %w", err)
	}

	fromSamplingValue := from.ToObject(rt).Get("sampling")
	if common.IsNullish(fromSamplingValue) {
		opts.Sampling = defaultSamplingRate
	}

	return opts, nil
}

func (i *options) validate() error {
	var (
		isW3C    = i.Propagator == W3CPropagatorName
		isJaeger = i.Propagator == JaegerPropagatorName
	)
	if !isW3C && !isJaeger {
		return fmt.Errorf("unknown propagator: %s", i.Propagator)
	}

	if i.Sampling < 0.0 || i.Sampling > 1.0 {
		return errors.New("sampling rate must be between 0.0 and 1.0")
	}

	// TODO: implement baggage support
	if i.Baggage != nil {
		return errors.New("baggage is not yet supported")
	}

	return nil
}
