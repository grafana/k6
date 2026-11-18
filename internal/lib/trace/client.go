package trace

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	oteltrace "go.opentelemetry.io/otel/trace"

	"go.k6.io/k6/v2/metrics"
)

// MetadataTraceID is the metric metadata key of the trace ID of a request, which the outputs use
// to correlate the request metric samples with the request trace.
const MetadataTraceID = "trace_id"

// traceparentHeader is the W3C Trace Context header.
const traceparentHeader = "Traceparent"

// jaegerTraceHeader is the header carrying the trace context in the Jaeger propagation format.
const jaegerTraceHeader = "Uber-Trace-Id"

// StartClientSpan starts the span of a request sent by a VU, e.g. an HTTP or gRPC request: a
// client span, child of the span in ctx (the iteration span).
//
// The span is propagated to the server in carrier, the request headers or metadata, with the W3C
// Trace Context. A trace context already in carrier, set by the script, is kept: the server is
// then not part of the span trace. Otherwise, when the span is sampled, its trace ID is added to
// tagsAndMeta, the metadata of the request metric samples, under MetadataTraceID.
func StartClientSpan(
	ctx context.Context,
	tracer oteltrace.Tracer,
	name string,
	carrier propagation.TextMapCarrier,
	tagsAndMeta *metrics.TagsAndMeta,
	attrs ...attribute.KeyValue,
) (context.Context, oteltrace.Span) {
	ctx, span := tracer.Start(ctx, name,
		oteltrace.WithSpanKind(oteltrace.SpanKindClient), oteltrace.WithAttributes(attrs...))

	// A trace context set by the script is kept, either W3C Trace Context or Jaeger's
	// uber-trace-id header.
	if carrier.Get(traceparentHeader) != "" || carrier.Get(jaegerTraceHeader) != "" {
		return ctx, span
	}

	// Only the W3C Trace Context propagator is supported.
	propagation.TraceContext{}.Inject(ctx, carrier)
	if span.SpanContext().IsSampled() {
		tagsAndMeta.SetMetadata(MetadataTraceID, span.SpanContext().TraceID().String())
	}
	return ctx, span
}
