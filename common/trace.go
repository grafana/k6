package common

import (
	"context"

	"go.opentelemetry.io/otel/trace"
)

// Tracer defines the interface with the tracing methods used in the common package.
type Tracer interface {
	TraceAPICall(
		ctx context.Context, targetID string, spanName string, opts ...trace.SpanStartOption,
	) (context.Context, trace.Span)
	TraceNavigation(
		ctx context.Context, targetID string, url string, opts ...trace.SpanStartOption,
	) (context.Context, trace.Span)
	TraceEvent(
		ctx context.Context, targetID string, eventName string, spanID string, opts ...trace.SpanStartOption,
	) (context.Context, trace.Span)
}
