package grpcext

import (
	"context"
	"net"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	oteltrace "go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	k6trace "go.k6.io/k6/v2/internal/lib/trace"
	"go.k6.io/k6/v2/metrics"
)

// attrRequestName is the k6-specific span attribute holding the name tag of the request.
const attrRequestName = "k6.grpc.request.name"

// spanKey is the context key of the span of an RPC, from TagRPC to its End stats.
type spanKey struct{}

// startSpan starts the span of an RPC, unary or stream, child of the span in ctx (the iteration
// span), and adds its trace context to the outgoing metadata of the RPC. See
// k6trace.StartClientSpan for its propagation to the server.
//
// It is called by the stats handler before each attempt of the RPC, so the headers sent by the
// attempt carry its span.
func startSpan(ctx context.Context, tracer oteltrace.Tracer, fullMethod string, rpc *rpcState) context.Context {
	// FromOutgoingContext returns a copy, so the metadata of the script is left untouched.
	md, _ := metadata.FromOutgoingContext(ctx)
	if md == nil {
		md = metadata.MD{}
	}

	method := strings.TrimPrefix(fullMethod, "/")
	attrs := []attribute.KeyValue{semconv.RPCSystemNameGRPC, semconv.RPCMethod(method)}
	attrs = append(attrs, serverAttributes(rpc.addr)...)
	if name, ok := rpc.tagsAndMeta.Tags.Get(metrics.TagName.String()); ok {
		attrs = append(attrs, attribute.String(attrRequestName, name))
	}

	ctx, span := k6trace.StartClientSpan(ctx, tracer, method, metadataCarrier(md), rpc.tagsAndMeta, attrs...)
	ctx = metadata.NewOutgoingContext(ctx, md)
	return context.WithValue(ctx, spanKey{}, span)
}

// endSpan completes and ends the span of an RPC in ctx, if any, from the error it ended with.
// As for the client spans of the gRPC semantic conventions, any status other than OK is an error.
func endSpan(ctx context.Context, err error) {
	span, ok := ctx.Value(spanKey{}).(oteltrace.Span)
	if !ok {
		return
	}
	code := status.Code(err)
	name := statusCodeName(code)
	span.SetAttributes(semconv.RPCResponseStatusCode(name))
	if code != codes.OK {
		span.SetAttributes(semconv.ErrorTypeKey.String(name))
	}
	k6trace.EndSpan(span, err)
}

// statusCodeNames are the canonical names of the gRPC status codes, used by the semantic
// conventions, e.g. DEADLINE_EXCEEDED, by code.
var statusCodeNames = [...]string{ //nolint:gochecknoglobals
	codes.OK:                 "OK",
	codes.Canceled:           "CANCELLED",
	codes.Unknown:            "UNKNOWN",
	codes.InvalidArgument:    "INVALID_ARGUMENT",
	codes.DeadlineExceeded:   "DEADLINE_EXCEEDED",
	codes.NotFound:           "NOT_FOUND",
	codes.AlreadyExists:      "ALREADY_EXISTS",
	codes.PermissionDenied:   "PERMISSION_DENIED",
	codes.ResourceExhausted:  "RESOURCE_EXHAUSTED",
	codes.FailedPrecondition: "FAILED_PRECONDITION",
	codes.Aborted:            "ABORTED",
	codes.OutOfRange:         "OUT_OF_RANGE",
	codes.Unimplemented:      "UNIMPLEMENTED",
	codes.Internal:           "INTERNAL",
	codes.Unavailable:        "UNAVAILABLE",
	codes.DataLoss:           "DATA_LOSS",
	codes.Unauthenticated:    "UNAUTHENTICATED",
}

// statusCodeName returns the canonical name of code, or its number when it isn't a known code.
func statusCodeName(code codes.Code) string {
	if int(code) < len(statusCodeNames) {
		return statusCodeNames[code]
	}
	return strconv.FormatUint(uint64(code), 10)
}

// defaultPort is the port gRPC connects to when the target has none, as for its dns resolver.
const defaultPort = 443

// serverAttributes returns the server.address and server.port attributes of the target the
// client is connected to, e.g. host:port or dns:///host:port. The port
// defaults to the gRPC one when the target has none.
func serverAttributes(target string) []attribute.KeyValue {
	endpoint := target
	if _, rest, ok := strings.Cut(target, "://"); ok {
		// The authority, often empty, comes before the endpoint.
		_, endpoint, _ = strings.Cut(rest, "/")
	}
	if endpoint == "" {
		return nil
	}

	host, rawPort, err := net.SplitHostPort(endpoint)
	if err != nil {
		return []attribute.KeyValue{semconv.ServerAddress(endpoint), semconv.ServerPort(defaultPort)}
	}
	port, err := strconv.Atoi(rawPort)
	if err != nil {
		port = defaultPort
	}
	return []attribute.KeyValue{semconv.ServerAddress(host), semconv.ServerPort(port)}
}

// setPeerAttributes adds the network.peer.address and network.peer.port attributes of the
// address the RPC was sent to, e.g. the IP the target resolved to, to the span in ctx, if any.
func setPeerAttributes(ctx context.Context, addr net.Addr) {
	span, ok := ctx.Value(spanKey{}).(oteltrace.Span)
	if !ok || addr == nil {
		return
	}
	host, rawPort, err := net.SplitHostPort(addr.String())
	if err != nil {
		span.SetAttributes(semconv.NetworkPeerAddress(addr.String()))
		return
	}
	span.SetAttributes(semconv.NetworkPeerAddress(host))
	if port, err := strconv.Atoi(rawPort); err == nil {
		span.SetAttributes(semconv.NetworkPeerPort(port))
	}
}

// metadataCarrier adapts the gRPC metadata to the OpenTelemetry propagators.
type metadataCarrier metadata.MD

// Get implements propagation.TextMapCarrier.
func (c metadataCarrier) Get(key string) string {
	if values := metadata.MD(c).Get(key); len(values) > 0 {
		return values[0]
	}
	return ""
}

// Set implements propagation.TextMapCarrier.
func (c metadataCarrier) Set(key, value string) {
	metadata.MD(c).Set(key, value)
}

// Keys implements propagation.TextMapCarrier.
func (c metadataCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}
