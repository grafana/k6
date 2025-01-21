package common

import (
	"context"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	browsertrace "go.k6.io/k6/internal/js/modules/k6/browser/trace"
)

// Tracer defines the interface with the tracing methods used in the common package.
type Tracer interface {
	TraceAPICall(
		ctx context.Context, targetID string, spanName string, opts ...trace.SpanStartOption,
	) (context.Context, trace.Span)
	TraceNavigation(
		ctx context.Context, targetID string, opts ...trace.SpanStartOption,
	) (context.Context, trace.Span)
	TraceEvent(
		ctx context.Context, targetID string, eventName string, spanID string, opts ...trace.SpanStartOption,
	) (context.Context, trace.Span)
}

// TraceAPICall is a helper method that retrieves the Tracer from the given ctx and
// calls its TraceAPICall implementation. If the Tracer is not present in the given
// ctx, it returns a noopSpan and the given context.
func TraceAPICall(
	ctx context.Context, targetID string, spanName string, opts ...trace.SpanStartOption,
) (context.Context, trace.Span) {
	if tracer := GetTracer(ctx); tracer != nil {
		return tracer.TraceAPICall(ctx, targetID, spanName, opts...)
	}
	return ctx, browsertrace.NoopSpan{}
}

// TraceNavigation is a helper method that retrieves the Tracer from the given ctx and
// calls its TraceNavigation implementation. If the Tracer is not present in the given
// ctx, it returns a noopSpan and the given context.
func TraceNavigation(
	ctx context.Context, targetID string, opts ...trace.SpanStartOption,
) (context.Context, trace.Span) {
	if tracer := GetTracer(ctx); tracer != nil {
		return tracer.TraceNavigation(ctx, targetID, opts...)
	}
	return ctx, browsertrace.NoopSpan{}
}

// TraceEvent is a helper method that retrieves the Tracer from the given ctx and
// calls its TraceEvent implementation. If the Tracer is not present in the given
// ctx, it returns a noopSpan and the given context.
func TraceEvent(
	ctx context.Context, targetID string, eventName string, spanID string, options ...trace.SpanStartOption,
) (context.Context, trace.Span) {
	if tracer := GetTracer(ctx); tracer != nil {
		return tracer.TraceEvent(ctx, targetID, eventName, spanID, options...)
	}
	return ctx, browsertrace.NoopSpan{}
}

// spanRecordError will set the status of the span to error and record the
// error on the span. Check the documentation for trace.SetStatus and
// trace.RecordError for more details.
func spanRecordError(span trace.Span, err error) {
	span.SetStatus(codes.Error, err.Error())
	span.RecordError(err)
}
