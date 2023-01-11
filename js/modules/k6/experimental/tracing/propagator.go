package tracing

import (
	"net/http"
)

// Propagator is an interface for trace context propagation
type Propagator interface {
	Propagate(traceID string) (http.Header, error)
}

// SampledPropagator is an interface for trace context propagation
// with support for sampling.
type SampledPropagator interface {
	Propagator
	ProbabilisticSampler

	// SampleFlag should return the sampling flag to use
	// as part of the trace context header, according to
	// the sampler's decision of whether or not a given
	// trace should be sampled.
	SampleFlag() string
}

const (
	// W3CPropagatorName is the name of the W3C trace context propagator
	W3CPropagatorName = "w3c"

	// W3CHeaderName is the name of the W3C trace context header
	W3CHeaderName = "traceparent"

	// W3CVersion is the version of the supported W3C trace context header.
	// The current specification assumes the version is set to 00.
	W3CVersion = "00"

	// W3CSampledTraceFlag is the trace-flag value for a sampled trace.
	W3CSampledTraceFlag = "01"

	// W3CUnsampledTraceFlag is the trace-flag value for an unsampled trace.
	W3CUnsampledTraceFlag = "00"
)

// W3CPropagator is a Propagator for the W3C trace context header
type W3CPropagator struct {
	Sampler
}

// Ensures that W3CPropagator implements the Propagator and
// SampledPropagator interface
var (
	_ Propagator        = &W3CPropagator{}
	_ SampledPropagator = &W3CPropagator{}
)

// NewW3CPropagator returns a new W3CPropagator with the given sampling rate.
//
// Note that we allocate the propagator on the heap to ensure we conform
// to the Sampler interface, as the ProbabilisticSampler's SetSamplingRate
// method has a pointer receiver.
func NewW3CPropagator(samplingRate int) *W3CPropagator {
	return &W3CPropagator{
		NewSampler(samplingRate),
	}
}

// Propagate returns a header with a random trace ID in the W3C format
func (p W3CPropagator) Propagate(traceID string) (http.Header, error) {
	parentID := randHexString(16)
	flags := p.SampleFlag()

	return http.Header{
		W3CHeaderName: {
			W3CVersion + "-" + traceID + "-" + parentID + "-" + flags,
		},
	}, nil
}

// SampleFlag returns the trace sample flag for the trace,
// based on the current sampling rate.
//
// It uses under the `Sampled` method under the hood to
// set the flag to either W3CSampledTraceFlag or W3CUnsampledTraceFlag,
// with a percent of chance based on the propagator's sampling
// rate.
func (p W3CPropagator) SampleFlag() string {
	if p.Sampled() {
		return W3CSampledTraceFlag
	}

	return W3CUnsampledTraceFlag
}

const (
	// JaegerPropagatorName is the name of the Jaeger trace context propagator
	JaegerPropagatorName = "jaeger"

	// JaegerHeaderName is the name of the Jaeger trace context header
	JaegerHeaderName = "uber-trace-id"

	// JaegerRootSpanID is the universal span ID of the root span.
	// Its value is zero, which is described in the Jaeger documentation as:
	// "0 value is valid and means “root span” (when not ignored)"
	JaegerRootSpanID = "0"

	// JaegerSampledTraceFlag is the trace-flag value for an unsampled trace.
	JaegerSampledTraceFlag = "0"

	// JaegerUnsampledTraceFlag is the trace-flag value for a sampled trace.
	JaegerUnsampledTraceFlag = "1"
)

// JaegerPropagator is a Propagator for the Jaeger trace context header
type JaegerPropagator struct {
	Sampler
}

// Ensure the JaegerPropagator implements the Propagator and
// SampledPropagator interface
var (
	_ Propagator        = &JaegerPropagator{}
	_ SampledPropagator = &JaegerPropagator{}
)

// NewJaegerPropagator returns a new JaegerPropagator with the given sampling rate.
//
// Note that we allocate the propagator on the heap to ensure we conform
// to the Sampler interface, as the ProbabilisticSampler's SetSamplingRate
// method has a pointer receiver.
func NewJaegerPropagator(samplingRate int) *JaegerPropagator {
	return &JaegerPropagator{
		NewSampler(samplingRate),
	}
}

// Propagate returns a header with a random trace ID in the Jaeger format
func (p *JaegerPropagator) Propagate(traceID string) (http.Header, error) {
	spanID := randHexString(8)
	flags := p.SampleFlag()

	return http.Header{
		JaegerHeaderName: {traceID + ":" + spanID + ":" + JaegerRootSpanID + ":" + flags},
	}, nil
}

// SampleFlag returns the trace sample flag for the trace,
// based on the current sampling rate.
//
// It uses under the `Sampled` method under the hood to
// set the flag to either W3CSampledTraceFlag or W3CUnsampledTraceFlag,
// with a percent of chance based on the propagator's sampling
// rate.
func (p *JaegerPropagator) SampleFlag() string {
	if p.Sampled() {
		return JaegerSampledTraceFlag
	}

	return JaegerUnsampledTraceFlag
}
