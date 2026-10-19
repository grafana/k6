package trace

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/otel/propagation"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// ErrInvalidParent indicates that the configured remote parent cannot be extracted.
var ErrInvalidParent = errors.New("invalid trace parent")

// ContextWithRemoteParent returns ctx carrying the remote span context in parent, which is the
// value of the propagator's primary header (e.g. a W3C traceparent). An empty parent returns ctx
// unchanged.
func ContextWithRemoteParent(
	ctx context.Context, parent string, propagator propagation.TextMapPropagator,
) (context.Context, error) {
	if parent == "" {
		return ctx, nil
	}

	fields := propagator.Fields()
	if len(fields) == 0 {
		return ctx, fmt.Errorf("%w %q", ErrInvalidParent, parent)
	}

	parentCtx := propagator.Extract(ctx, propagation.MapCarrier{fields[0]: parent})
	if !oteltrace.SpanContextFromContext(parentCtx).IsValid() {
		return ctx, fmt.Errorf("%w %q", ErrInvalidParent, parent)
	}

	return parentCtx, nil
}
