package tracing

import (
	"math/rand"
)

// Sampler is an interface defining a sampling strategy.
type Sampler interface {
	// ShouldSample returns true if the trace should be sampled
	// false otherwise.
	ShouldSample() bool
}

// ProbabilisticSampler implements the ProbabilisticSampler interface and allows
// to take probabilistic sampling decisions based on a sampling rate.
type ProbabilisticSampler struct {
	// random is a random number generator used by the sampler.
	random *rand.Rand

	// samplingRate is a chance value defined as a percentage
	// value within 0.0 <= samplingRate <= 1.0 bounds.
	samplingRate float64
}

// NewProbabilisticSampler returns a new ProbablisticSampler with the provided sampling rate.
//
// Note that the sampling rate is a percentage value within 0.0 <= samplingRate <= 1.0 bounds.
// If the provided sampling rate is outside of this range, it will be clamped to the closest
// bound.
func NewProbabilisticSampler(samplingRate float64) *ProbabilisticSampler {
	// Ensure that the sampling rate is within the 0.0 <= samplingRate <= 1.0 bounds.
	if samplingRate < 0.0 {
		samplingRate = 0.0
	} else if samplingRate > 1.0 {
		samplingRate = 1.0
	}

	return &ProbabilisticSampler{samplingRate: samplingRate}
}

// ShouldSample returns true if the trace should be sampled.
//
// Its return value is probabilistic, based on the selected
// sampling rate S, there is S percent chance that the
// returned value is true.
func (ps ProbabilisticSampler) ShouldSample() bool {
	return chance(ps.random, ps.samplingRate)
}

// Ensure that ProbabilisticSampler implements the Sampler interface.
var _ Sampler = &ProbabilisticSampler{}

// AlwaysOnSampler implements the Sampler interface and allows to bypass
// sampling decisions by returning true for all Sampled() calls.
//
// This is useful in cases where the user either does not provide
// the sampling option, or set it to 100% as it will avoid any
// call to the random number generator.
type AlwaysOnSampler struct{}

// NewAlwaysOnSampler returns a new AlwaysSampledSampler.
func NewAlwaysOnSampler() *AlwaysOnSampler {
	return &AlwaysOnSampler{}
}

// ShouldSample always returns true.
func (AlwaysOnSampler) ShouldSample() bool { return true }

// Ensure that AlwaysOnSampler implements the Sampler interface.
var _ Sampler = &AlwaysOnSampler{}
