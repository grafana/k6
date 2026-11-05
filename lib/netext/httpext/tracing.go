package httpext

import (
	"context"
	"errors"
	"net/url"
	"strconv"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	oteltrace "go.opentelemetry.io/otel/trace"

	k6trace "go.k6.io/k6/v2/internal/lib/trace"
	"go.k6.io/k6/v2/metrics"
)

// metadataTraceID is the metric metadata key of the trace ID of a request, which the outputs use
// to correlate the HTTP metric samples with the request trace.
const metadataTraceID = "trace_id"

// startSpan starts the span of an HTTP request, child of the span in ctx (the iteration span),
// covering all its round trips, e.g. its redirects. It propagates the span to the server, and
// adds its trace ID to the metadata of the request metric samples.
func startSpan(
	ctx context.Context, tracer oteltrace.Tracer, preq *ParsedHTTPRequest,
) (context.Context, oteltrace.Span) {
	req := preq.Req
	attrs := []attribute.KeyValue{
		semconv.HTTPRequestMethodKey.String(req.Method),
		semconv.ServerAddress(req.URL.Hostname()),
		semconv.URLFull(preq.URL.Clean()),
	}
	if port, ok := serverPort(req.URL); ok {
		attrs = append(attrs, semconv.ServerPort(port))
	}
	if name, ok := preq.TagsAndMeta.Tags.Get(metrics.TagName.String()); ok {
		attrs = append(attrs, semconv.URLTemplate(name))
	}
	ctx, span := tracer.Start(ctx, req.Method,
		oteltrace.WithSpanKind(oteltrace.SpanKindClient), oteltrace.WithAttributes(attrs...))

	// A trace context set by the script is kept.
	if req.Header.Get("Traceparent") != "" {
		return ctx, span
	}

	// Only support the W3C Trace Context propagator for injecting trace context into the request headers.
	propagation.TraceContext{}.Inject(ctx, propagation.HeaderCarrier(req.Header))
	if span.SpanContext().IsSampled() {
		preq.TagsAndMeta.SetMetadata(metadataTraceID, span.SpanContext().TraceID().String())
	}
	return ctx, span
}

// endSpan completes and ends the span of an HTTP request, from the result of MakeRequest. Like
// for http_req_failed, a response the response callback of the script expects is not an error,
// even with a 4xx or 5xx status. Without a response callback, 4xx and 5xx statuses are errors.
func endSpan(span oteltrace.Span, preq *ParsedHTTPRequest, resp *Response, err error) {
	if err == nil && resp != nil && resp.Error != "" {
		err = errors.New(resp.Error)
	}

	if err != nil {
		span.SetAttributes(semconv.ErrorTypeKey.String(strconv.Itoa(spanErrorCode(resp, err))))
	} else {
		span.SetAttributes(semconv.HTTPResponseStatusCode(resp.Status))
		expected := resp.Status < 400
		if preq.ResponseCallback != nil {
			expected = preq.ResponseCallback(resp.Status)
		}
		if !expected {
			span.SetAttributes(semconv.ErrorTypeKey.String(strconv.Itoa(resp.Status)))
			// The reason is inferred from the status code, so no description is set.
			span.SetStatus(codes.Error, "")
		}
	}

	k6trace.EndSpan(span, err)
}

// spanErrorCode returns the k6 error code of a failed request. resp is nil when the error is
// thrown to the script, and the code is then derived from err.
func spanErrorCode(resp *Response, err error) int {
	if resp != nil && resp.ErrorCode != 0 {
		return resp.ErrorCode
	}
	code, _ := errorCodeForError(err)
	return int(code)
}

// serverPort returns the port of u, or the default port of its scheme when it isn't explicit.
func serverPort(u *url.URL) (int, bool) {
	if port, err := strconv.Atoi(u.Port()); err == nil {
		return port, true
	}
	switch u.Scheme {
	case "https":
		return 443, true
	case "http":
		return 80, true
	default:
		return 0, false
	}
}
