package tracing

// ProbabilisticSampler is an interface defining probabilistic sampling strategies.
type ProbabilisticSampler interface {
	// SetSamplingRate sets the sampling rate for the sampler.
	SetSamplingRate(percent int)

	// Sampled returns true if the trace should be sampled
	// false otherwise. The returned value is based on the
	// sampling rate set by SetSamplingRate().
	Sampled() bool
}

// Sampler implements the Sampler interface and allows
// to take probabilistic sampling decisions based on a sampling rate.
type Sampler struct {
	// samplingRate is a chance value defined as a percentage
	// value within 0 <= samplingRate <= 100 bounds.
	samplingRate int
}

// NewSampler returns a new ProbablisticSampler with the provided sampling rate.
func NewSampler(samplingRate int) Sampler {
	return Sampler{samplingRate: samplingRate}
}

// SetSamplingRate sets the sampling rate for the sampler.
func (ps *Sampler) SetSamplingRate(percent int) {
	ps.samplingRate = percent
}

// Sampled returns true if the trace should be sampled.
//
// Its return value is probabilistic, based on the selected
// sampling rate S, there is S percent chance that the
// returned value is true.
func (ps Sampler) Sampled() bool {
	return chance(ps.samplingRate)
}
