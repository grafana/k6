package grpc

import (
	"context"
	"net"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	oteltrace "go.opentelemetry.io/otel/trace"
	grpcCodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"

	"go.k6.io/k6/v2/internal/lib/netext/grpcext"
	k6trace "go.k6.io/k6/v2/internal/lib/trace"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/metrics"
)

const grpcTracerName = "go.k6.io/k6/grpc"

func startGRPCTrace(
	ctx context.Context, state *lib.State, address, method string, md metadata.MD,
	tagsAndMeta *metrics.TagsAndMeta,
) (context.Context, oteltrace.Span) {
	if state.TracerProvider == nil {
		return ctx, nil
	}

	service, operation := splitGRPCMethod(method)
	options := []oteltrace.SpanStartOption{
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			semconv.RPCSystemNameGRPC,
			attribute.String("rpc.service", service),
			semconv.RPCMethod(operation),
			semconv.ServerAddress(grpcServerHost(address)),
		),
	}
	if port := grpcServerPort(address); port != 0 {
		options = append(options, oteltrace.WithAttributes(semconv.ServerPort(port)))
	}

	ctx, span := state.TracerProvider.Tracer(grpcTracerName).Start(
		ctx, strings.TrimPrefix(method, "/"), options...,
	)
	spanContext := span.SpanContext()
	if !spanContext.IsValid() {
		return ctx, span
	}

	carrier := propagation.MapCarrier{}
	k6trace.PropagatorOrDefault(state.TracePropagator).Inject(ctx, carrier)
	for key, value := range carrier {
		md.Set(key, value)
	}
	if tagsAndMeta.Metadata == nil {
		tagsAndMeta.Metadata = make(map[string]string)
	}
	tagsAndMeta.Metadata["trace_id"] = spanContext.TraceID().String()
	return ctx, span
}

func endGRPCTrace(span oteltrace.Span, response *grpcext.InvokeResponse, err error) {
	if span == nil {
		return
	}
	if response != nil {
		span.SetAttributes(attribute.Int("rpc.grpc.status_code", int(response.Status)))
		if response.Status != grpcCodes.OK {
			span.SetStatus(codes.Error, response.Status.String())
		}
	}
	k6trace.EndSpan(span, err)
}

func splitGRPCMethod(method string) (string, string) {
	parts := strings.SplitN(strings.TrimPrefix(method, "/"), "/", 2)
	if len(parts) != 2 {
		return "", strings.TrimPrefix(method, "/")
	}
	return parts[0], parts[1]
}

func grpcServerHost(address string) string {
	host, _, err := net.SplitHostPort(address)
	if err == nil {
		return host
	}
	return address
}

func grpcServerPort(address string) int {
	_, port, err := net.SplitHostPort(address)
	if err != nil {
		return 0
	}
	result, _ := strconv.Atoi(port)
	return result
}
