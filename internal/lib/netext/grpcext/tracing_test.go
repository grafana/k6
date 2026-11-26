package grpcext

import (
	"context"
	"io"
	"net"
	"strconv"
	"sync"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"go.k6.io/k6/v2/internal/features"
	k6trace "go.k6.io/k6/v2/internal/lib/trace"
	"go.k6.io/k6/v2/internal/lib/trace/tracetest"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/metrics"
)

// tracingServer is a gRPC server accepting any method, which records the traceparent metadata of
// the RPCs it receives. Its NoOp method fails with NOT_FOUND, and its LotsOfReplies method sends
// two messages.
type tracingServer struct {
	addr string

	mu           sync.Mutex
	traceparents [][]string
}

func newTracingServer(t *testing.T) *tracingServer {
	t.Helper()
	ts := &tracingServer{}
	srv := grpc.NewServer(grpc.UnknownServiceHandler(func(_ any, stream grpc.ServerStream) error {
		md, _ := metadata.FromIncomingContext(stream.Context())
		ts.mu.Lock()
		ts.traceparents = append(ts.traceparents, md.Get("traceparent"))
		ts.mu.Unlock()

		method, _ := grpc.MethodFromServerStream(stream)
		if err := stream.RecvMsg(new(emptypb.Empty)); err != nil {
			return err
		}
		switch method {
		case "/hello.HelloService/NoOp":
			return status.Error(codes.NotFound, "no such thing")
		case "/hello.HelloService/LotsOfReplies":
			for range 2 {
				if err := stream.SendMsg(new(emptypb.Empty)); err != nil {
					return err
				}
			}
			return nil
		default:
			return stream.SendMsg(new(emptypb.Empty))
		}
	}))
	lis, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	ts.addr = lis.Addr().String()
	return ts
}

func (ts *tracingServer) received() [][]string {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return append([][]string(nil), ts.traceparents...)
}

// grpcTracingFixture makes RPCs with native tracing enabled, under a parent span.
type grpcTracingFixture struct {
	conn    *Conn
	state   *lib.State
	samples chan metrics.SampleContainer
	rec     *tracetest.Recorder
	ctx     context.Context
	parent  oteltrace.Span
}

func newGRPCTracingFixture(
	t *testing.T, addr string, enabled bool, opts ...sdktrace.TracerProviderOption,
) *grpcTracingFixture {
	t.Helper()
	tp, rec := tracetest.NewProvider(opts...)
	registry := metrics.NewRegistry()
	samples := make(chan metrics.SampleContainer, 100)
	state := &lib.State{
		Options:        lib.Options{SystemTags: &metrics.DefaultSystemTagSet},
		Samples:        samples,
		Logger:         logrus.New(),
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(registry),
		Tags:           lib.NewVUStateTags(registry.RootTagSet()),
	}
	ctx := t.Context()
	parent := oteltrace.SpanFromContext(ctx)
	if enabled {
		tracer := k6trace.Tracer(tp, &features.Flags{NativeTracing: true})
		state.Tracer = tracer
		ctx, parent = tracer.Start(ctx, "parent")
	}

	conn, err := Dial(ctx, addr, nil,
		grpc.WithStatsHandler(statsHandler{getState: func() *lib.State { return state }}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	return &grpcTracingFixture{conn: conn, state: state, samples: samples, rec: rec, ctx: ctx, parent: parent}
}

func (f *grpcTracingFixture) invoke(t *testing.T, method string, md metadata.MD) *InvokeResponse {
	t.Helper()
	tagsAndMeta := f.state.Tags.GetCurrentValues()
	tagsAndMeta.SetSystemTagOrMeta(metrics.TagName, "my-call")
	res, err := f.conn.Invoke(f.ctx, InvokeRequest{
		Method:           "/hello.HelloService/" + method,
		MethodDescriptor: methodFromProto(method),
		Message:          []byte(`{}`),
		TagsAndMeta:      &tagsAndMeta,
		Metadata:         md,
	})
	require.NoError(t, err)
	return res
}

// traceIDMetadata returns the trace ID metadata of the grpc_req_duration sample of the RPC.
func (f *grpcTracingFixture) traceIDMetadata(t *testing.T) (string, bool) {
	t.Helper()
	require.Len(t, f.samples, 1)
	samples := (<-f.samples).GetSamples()
	require.Len(t, samples, 1)
	assert.Equal(t, f.state.BuiltinMetrics.GRPCReqDuration, samples[0].Metric)
	traceID, ok := samples[0].Metadata[k6trace.MetadataTraceID]
	return traceID, ok
}

func traceparentOf(s sdktrace.ReadOnlySpan) string {
	sc := s.SpanContext()
	return "00-" + sc.TraceID().String() + "-" + sc.SpanID().String() + "-01"
}

func spanAttrs(s sdktrace.ReadOnlySpan) map[string]any {
	attrs := map[string]any{}
	for _, kv := range s.Attributes() {
		attrs[string(kv.Key)] = kv.Value.AsInterface()
	}
	return attrs
}

func TestRPCTracing(t *testing.T) {
	t.Parallel()

	t.Run("unary span and propagation", func(t *testing.T) {
		t.Parallel()
		srv := newTracingServer(t)
		f := newGRPCTracingFixture(t, srv.addr, true)

		res := f.invoke(t, "SayHello", metadata.New(map[string]string{"x-custom": "kept"}))
		require.Equal(t, codes.OK, res.Status)
		f.parent.End()

		spans := f.rec.ByName("hello.HelloService/SayHello")
		require.Len(t, spans, 1)
		span := spans[0]
		assert.Equal(t, oteltrace.SpanKindClient, span.SpanKind())
		assert.Equal(t, f.parent.SpanContext().SpanID(), span.Parent().SpanID())
		assert.Equal(t, f.parent.SpanContext().TraceID(), span.SpanContext().TraceID())
		assert.Equal(t, otelcodes.Unset, span.Status().Code)

		_, rawPort, err := net.SplitHostPort(srv.addr)
		require.NoError(t, err)
		port, err := strconv.Atoi(rawPort)
		require.NoError(t, err)
		assert.Equal(t, map[string]any{
			"rpc.system.name":          "grpc",
			"rpc.method":               "hello.HelloService/SayHello",
			"rpc.response.status_code": "OK",
			"server.address":           "127.0.0.1",
			"server.port":              int64(port),
			"network.peer.address":     "127.0.0.1",
			"network.peer.port":        int64(port),
			"k6.grpc.request.name":     "my-call",
		}, spanAttrs(span))

		traceID, _ := f.traceIDMetadata(t)
		assert.Equal(t, span.SpanContext().TraceID().String(), traceID, "the samples link to the trace")
		assert.Equal(t, [][]string{{traceparentOf(span)}}, srv.received(),
			"the server must be the child of the RPC span")
	})

	t.Run("error status", func(t *testing.T) {
		t.Parallel()
		srv := newTracingServer(t)
		f := newGRPCTracingFixture(t, srv.addr, true)

		res := f.invoke(t, "NoOp", nil)
		require.Equal(t, codes.NotFound, res.Status)

		spans := f.rec.ByName("hello.HelloService/NoOp")
		require.Len(t, spans, 1)
		assert.Equal(t, otelcodes.Error, spans[0].Status().Code)
		attrs := spanAttrs(spans[0])
		assert.Equal(t, "NOT_FOUND", attrs["rpc.response.status_code"])
		assert.Equal(t, "NOT_FOUND", attrs["error.type"])
	})

	t.Run("stream span ends with the stream", func(t *testing.T) {
		t.Parallel()
		srv := newTracingServer(t)
		f := newGRPCTracingFixture(t, srv.addr, true)

		tagsAndMeta := f.state.Tags.GetCurrentValues()
		stream, err := f.conn.NewStream(f.ctx, StreamRequest{
			Method:           "/hello.HelloService/LotsOfReplies",
			MethodDescriptor: methodFromProto("LotsOfReplies"),
			TagsAndMeta:      &tagsAndMeta,
		})
		require.NoError(t, err)
		require.NoError(t, stream.Send([]byte(`{"greeting":"hi"}`)))
		require.NoError(t, stream.CloseSend())

		for range 2 {
			_, err = stream.ReceiveConverted()
			require.NoError(t, err)
			assert.Empty(t, f.rec.Ended(), "the span is open while the stream is")
		}
		_, err = stream.ReceiveConverted()
		require.ErrorIs(t, err, io.EOF)

		spans := f.rec.ByName("hello.HelloService/LotsOfReplies")
		require.Len(t, spans, 1)
		assert.Equal(t, f.parent.SpanContext().SpanID(), spans[0].Parent().SpanID())
		assert.Equal(t, "OK", spanAttrs(spans[0])["rpc.response.status_code"])
		assert.Equal(t, [][]string{{traceparentOf(spans[0])}}, srv.received())
		assert.Equal(t, spans[0].SpanContext().TraceID().String(), tagsAndMeta.Metadata[k6trace.MetadataTraceID],
			"the stream metric samples link to the trace")
	})

	t.Run("script traceparent is kept", func(t *testing.T) {
		t.Parallel()
		const own = "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"
		srv := newTracingServer(t)
		f := newGRPCTracingFixture(t, srv.addr, true)

		f.invoke(t, "SayHello", metadata.New(map[string]string{"traceparent": own}))

		require.Len(t, f.rec.ByName("hello.HelloService/SayHello"), 1)
		_, hasTraceID := f.traceIDMetadata(t)
		assert.False(t, hasTraceID, "the samples aren't linked to a trace the server isn't part of")
		assert.Equal(t, [][]string{{own}}, srv.received())
	})

	t.Run("not sampled", func(t *testing.T) {
		t.Parallel()
		srv := newTracingServer(t)
		f := newGRPCTracingFixture(t, srv.addr, true, sdktrace.WithSampler(sdktrace.NeverSample()))

		f.invoke(t, "SayHello", nil)

		_, hasTraceID := f.traceIDMetadata(t)
		assert.False(t, hasTraceID, "the samples aren't linked to a trace that isn't exported")
	})

	t.Run("health check span", func(t *testing.T) {
		t.Parallel()
		srv := newTracingServer(t)
		f := newGRPCTracingFixture(t, srv.addr, true)

		tagsAndMeta := f.state.Tags.GetCurrentValues()
		_, err := f.conn.HealthCheck(f.ctx, "", &tagsAndMeta)
		require.NoError(t, err)

		spans := f.rec.ByName("grpc.health.v1.Health/Check")
		require.Len(t, spans, 1)
		assert.Equal(t, f.parent.SpanContext().SpanID(), spans[0].Parent().SpanID())
		assert.Equal(t, "OK", spanAttrs(spans[0])["rpc.response.status_code"])
		assert.Equal(t, [][]string{{traceparentOf(spans[0])}}, srv.received())
		traceID, _ := f.traceIDMetadata(t)
		assert.Equal(t, spans[0].SpanContext().TraceID().String(), traceID,
			"the samples link to the trace")
	})

	t.Run("k6 RPCs aren't traced", func(t *testing.T) {
		t.Parallel()
		srv := newTracingServer(t)
		f := newGRPCTracingFixture(t, srv.addr, true)

		// The server doesn't implement reflection, only the RPC matters here.
		_, _ = f.conn.Reflect(f.ctx)

		assert.Empty(t, f.rec.Ended())
		assert.Equal(t, [][]string{nil}, srv.received())
	})

	t.Run("disabled", func(t *testing.T) {
		t.Parallel()
		srv := newTracingServer(t)
		f := newGRPCTracingFixture(t, srv.addr, false)

		f.invoke(t, "SayHello", nil)

		assert.Empty(t, f.rec.Ended())
		_, hasTraceID := f.traceIDMetadata(t)
		assert.False(t, hasTraceID)
		assert.Equal(t, [][]string{nil}, srv.received())
	})
}

func TestServerAttributes(t *testing.T) {
	t.Parallel()

	hostPort := func(host string, port int) []attribute.KeyValue {
		return []attribute.KeyValue{attribute.String("server.address", host), attribute.Int("server.port", port)}
	}
	tests := map[string][]attribute.KeyValue{
		"127.0.0.1:50051":               hostPort("127.0.0.1", 50051),
		"dns:///example.com:8443":       hostPort("example.com", 8443),
		"dns://8.8.8.8/example.com:443": hostPort("example.com", 443),
		"passthrough:///example.com:80": hostPort("example.com", 80),
		"[::1]:50051":                   hostPort("::1", 50051),
		"example.com":                   hostPort("example.com", 443),
		"dns:///example.com":            hostPort("example.com", 443),
		"":                              nil,
	}
	for target, want := range tests {
		assert.Equal(t, want, serverAttributes(target), target)
	}
}
