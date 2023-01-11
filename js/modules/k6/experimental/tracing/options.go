package tracing

import (
	"errors"
	"fmt"
)

// options are the options that can be passed to the
// tracing.instrumentHTTP() method.
type options struct {
	// Propagation is the propagation format to use for the tracer.
	Propagator string `js:"propagator"`

	// Sampling is the sampling rate to use for the
	// tracer, expressed in percents: 0 <= n <= 100.
	Sampling *int `json:"sampling"`

	// Baggage is a map of baggage items to add to the tracer.
	Baggage map[string]string `js:"baggage"`
}

func (i *options) validate() error {
	var (
		isW3C    = i.Propagator == W3CPropagatorName
		isJaeger = i.Propagator == JaegerPropagatorName
	)
	if !isW3C && !isJaeger {
		return fmt.Errorf("unknown propagator: %s", i.Propagator)
	}

	if i.Sampling != nil && (*i.Sampling < 0 || *i.Sampling > 100) {
		return errors.New("sampling rate must be between 0 and 100")
	}

	// TODO: implement baggage support
	if i.Baggage != nil {
		return errors.New("baggage is not yet supported")
	}

	return nil
}
