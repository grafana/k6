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
type W3CPropagator struct{}

// Propagate returns a header with a random trace ID in the W3C format
func (p *W3CPropagator) Propagate(traceID string) (http.Header, error) {
	parentID := randHexString(16)

	return http.Header{
		W3CHeaderName: {
			W3CVersion + "-" + traceID + "-" + parentID + "-" + W3CSampledTraceFlag,
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
)

// JaegerPropagator is a Propagator for the Jaeger trace context header
type JaegerPropagator struct{}

// Propagate returns a header with a random trace ID in the Jaeger format
func (p *JaegerPropagator) Propagate(traceID string) (http.Header, error) {
	spanID := randHexString(8)
	// flags set to 1 means the span is sampled
	flags := "1"

	return http.Header{
		JaegerHeaderName: {traceID + ":" + spanID + ":" + JaegerRootSpanID + ":" + flags},
	}, nil
}
