package httpext

import (
	"context"
	"errors"
	"net/http"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	oteltrace "go.opentelemetry.io/otel/trace"

	k6trace "go.k6.io/k6/v2/internal/lib/trace"
	"go.k6.io/k6/v2/lib"
)

const httpTracerName = "go.k6.io/k6/http"

func startHTTPTrace(
	ctx context.Context, state *lib.State, request *ParsedHTTPRequest,
) (context.Context, oteltrace.Span) {
	if state.TracerProvider == nil {
		return ctx, nil
	}

	method := request.Req.Method
	ctx, span := state.TracerProvider.Tracer(httpTracerName).Start(
		ctx,
		"http.request",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			semconv.HTTPRequestMethodKey.String(method),
			semconv.URLFull(request.URL.Clean()),
			semconv.ServerAddress(request.Req.URL.Hostname()),
		),
	)
	spanContext := span.SpanContext()
	if !spanContext.IsValid() {
		return ctx, span
	}
	k6trace.PropagatorOrDefault(state.TracePropagator).Inject(
		ctx, propagation.HeaderCarrier(request.Req.Header),
	)

	if request.TagsAndMeta.Metadata == nil {
		request.TagsAndMeta.Metadata = make(map[string]string)
	}
	request.TagsAndMeta.Metadata["trace_id"] = spanContext.TraceID().String()

	return ctx, span
}

func endHTTPTrace(span oteltrace.Span, response *Response, requestErr error) {
	if response != nil {
		if response.Status != 0 {
			span.SetAttributes(semconv.HTTPResponseStatusCode(response.Status))
			if requestErr == nil && response.Status >= http.StatusInternalServerError {
				span.SetStatus(codes.Error, response.StatusText)
			}
		}
		if requestErr == nil && response.Error != "" {
			requestErr = errors.New(response.Error)
		}
	}
	k6trace.EndSpan(span, requestErr)
}
