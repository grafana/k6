package tracing

import (
	"net/http"
)

// Propagator is an interface for trace context propagation
type Propagator interface {
	Propagate(traceID string) (http.Header, error)
}

const (
	// W3CPropagatorName is the name of the W3C trace context propagator
	W3CPropagatorName = "w3c"

	// W3CHeaderName is the name of the W3C trace context header
	W3CHeaderName = "traceparent"

	// W3CVersion is the version of the supported W3C trace context header.
	// The current specification assumes the version is set to 00.
	W3CVersion = "00"

	// W3CUnsampledTraceFlag is the trace-flag value for an unsampled trace.
	W3CUnsampledTraceFlag = "00"

	// W3CSampledTraceFlag is the trace-flag value for a sampled trace.
	W3CSampledTraceFlag = "01"
)

// W3CPropagator is a Propagator for the W3C trace context header
type W3CPropagator struct {
	// Sampler is used to determine whether or not a trace should be sampled.
	Sampler
}

// NewW3CPropagator returns a new W3CPropagator using the provided sampler
// to base its sampling decision upon.
//
// Note that we allocate the propagator on the heap to ensure we conform
// to the Sampler interface, as the [Sampler.SetSamplingRate]
// method has a pointer receiver.
func NewW3CPropagator(s Sampler) *W3CPropagator {
	return &W3CPropagator{
		Sampler: s,
	}
}

// Propagate returns a header with a random trace ID in the W3C format
func (p *W3CPropagator) Propagate(traceID string) (http.Header, error) {
	parentID := randHexString(16)
	flags := pick(p.ShouldSample(), W3CSampledTraceFlag, W3CUnsampledTraceFlag)

	return http.Header{
		W3CHeaderName: {
			W3CVersion + "-" + traceID + "-" + parentID + "-" + flags,
		},
	}, nil
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

	// JaegerUnsampledTraceFlag is the trace-flag value for an unsampled trace.
	JaegerUnsampledTraceFlag = "0"

	// JaegerSampledTraceFlag is the trace-flag value for a sampled trace.
	JaegerSampledTraceFlag = "1"
)

// JaegerPropagator is a Propagator for the Jaeger trace context header
type JaegerPropagator struct {
	// Sampler is used to determine whether or not a trace should be sampled.
	Sampler
}

// NewJaegerPropagator returns a new JaegerPropagator with the given sampler.
func NewJaegerPropagator(s Sampler) *JaegerPropagator {
	return &JaegerPropagator{
		Sampler: s,
	}
}

// Propagate returns a header with a random trace ID in the Jaeger format
func (p *JaegerPropagator) Propagate(traceID string) (http.Header, error) {
	spanID := randHexString(8)
	flags := pick(p.ShouldSample(), JaegerSampledTraceFlag, JaegerUnsampledTraceFlag)

	return http.Header{
		JaegerHeaderName: {traceID + ":" + spanID + ":" + JaegerRootSpanID + ":" + flags},
	}, nil
}

// Pick returns either the left or right value, depending on the value of the `decision`
// boolean value.
func pick[T any](decision bool, lhs, rhs T) T {
	if decision {
		return lhs
	}

	return rhs
}

var (
	// Ensures that W3CPropagator implements the Propagator interface
	_ Propagator = &W3CPropagator{}

	// Ensures that W3CPropagator implements the Sampler interface
	_ Sampler = &W3CPropagator{}

	// Ensures the JaegerPropagator implements the Propagator interface
	_ Propagator = &JaegerPropagator{}

	// Ensures the JaegerPropagator implements the Sampler interface
	_ Sampler = &JaegerPropagator{}
)
