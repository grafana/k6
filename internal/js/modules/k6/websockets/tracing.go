package websockets

import (
	"context"
	"net/http"
	"net/url"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	oteltrace "go.opentelemetry.io/otel/trace"

	k6trace "go.k6.io/k6/v2/internal/lib/trace"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/metrics"
)

const webSocketsTracerName = "go.k6.io/k6/websocket"

// startSessionTrace starts a WebSocket session span and injects its context into the handshake
// headers.
func startSessionTrace(
	ctx context.Context, state *lib.State, target *url.URL, headers http.Header,
	tagsAndMeta *metrics.TagsAndMeta,
) (context.Context, oteltrace.Span) {
	if state.TracerProvider == nil {
		return ctx, nil
	}

	ctx, span := state.TracerProvider.Tracer(webSocketsTracerName).Start(
		ctx,
		"websocket.session",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			semconv.URLFull(target.Redacted()),
			semconv.ServerAddress(target.Hostname()),
			attribute.String("network.protocol.name", "websocket"),
		),
	)
	spanContext := span.SpanContext()
	if !spanContext.IsValid() {
		return ctx, span
	}

	k6trace.PropagatorOrDefault(state.TracePropagator).Inject(ctx, propagation.HeaderCarrier(headers))
	if tagsAndMeta.Metadata == nil {
		tagsAndMeta.Metadata = make(map[string]string)
	}
	tagsAndMeta.Metadata["trace_id"] = spanContext.TraceID().String()
	return ctx, span
}

// endSessionTrace records the handshake status and session error, then ends the span.
func endSessionTrace(span oteltrace.Span, handshakeStatus int, err error) {
	if span == nil {
		return
	}
	if handshakeStatus != 0 {
		span.SetAttributes(semconv.HTTPResponseStatusCode(handshakeStatus))
		if err == nil && handshakeStatus >= http.StatusBadRequest {
			span.SetStatus(codes.Error, http.StatusText(handshakeStatus))
		}
	}
	k6trace.EndSpan(span, err)
}
