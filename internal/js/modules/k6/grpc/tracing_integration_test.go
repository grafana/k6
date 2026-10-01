package grpc_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"

	"go.k6.io/k6/v2/internal/lib/testutils/grpcservice"
	"go.k6.io/k6/v2/internal/lib/testutils/httpmultibin/grpc_testing"
	moduletrace "go.k6.io/k6/v2/lib/trace"

	xk6grpc "go.k6.io/k6/v2/internal/js/modules/k6/grpc"
)

type operationTraceExporter struct {
	mu    sync.Mutex
	spans []sdktrace.ReadOnlySpan
}

func (e *operationTraceExporter) ExportSpans(_ context.Context, spans []sdktrace.ReadOnlySpan) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.spans = append(e.spans, spans...)
	return nil
}

func (*operationTraceExporter) Shutdown(context.Context) error { return nil }

func (e *operationTraceExporter) exportedSpans() []sdktrace.ReadOnlySpan {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]sdktrace.ReadOnlySpan(nil), e.spans...)
}

type traceHealthServer struct {
	healthgrpc.UnimplementedHealthServer
	metadata chan metadata.MD
}

func (s *traceHealthServer) Check(
	ctx context.Context, _ *healthgrpc.HealthCheckRequest,
) (*healthgrpc.HealthCheckResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	s.metadata <- md.Copy()
	return &healthgrpc.HealthCheckResponse{Status: healthgrpc.HealthCheckResponse_SERVING}, nil
}

func TestOperationTracing(t *testing.T) {
	t.Parallel()

	ts := newTestState(t)
	unaryMetadata := make(chan metadata.MD, 2)
	ts.httpBin.GRPCStub.EmptyCallFunc = func(
		ctx context.Context, _ *grpc_testing.Empty,
	) (*grpc_testing.Empty, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		unaryMetadata <- md.Copy()
		return &grpc_testing.Empty{}, nil
	}
	healthMetadata := make(chan metadata.MD, 1)
	healthgrpc.RegisterHealthServer(ts.httpBin.ServerGRPC, &traceHealthServer{metadata: healthMetadata})
	streamMetadata := make(chan metadata.MD, 1)
	stub := &featureExplorerStub{}
	stub.listFeatures = func(_ *grpcservice.Rectangle, stream grpcservice.FeatureExplorer_ListFeaturesServer) error {
		md, _ := metadata.FromIncomingContext(stream.Context())
		streamMetadata <- md.Copy()
		return nil
	}
	grpcservice.RegisterFeatureExplorerServer(ts.httpBin.ServerGRPC, stub)

	tracingSet, err := moduletrace.ParseSet("grpc")
	require.NoError(t, err)
	ts.VU.InitEnvField.Tracing = tracingSet
	m, ok := xk6grpc.New().NewModuleInstance(ts.VU).(*xk6grpc.ModuleInstance)
	require.True(t, ok)
	require.NoError(t, ts.VU.Runtime().Set("grpc", m.Exports().Named))

	_, err = ts.Run(`
		var client = new grpc.Client();
		client.load([], "../../../../lib/testutils/httpmultibin/grpc_testing/test.proto");
		client.load([], "../../../../lib/testutils/grpcservice/route_guide.proto");
	`)
	require.NoError(t, err)

	ts.ToVUContext()
	exporter := &operationTraceExporter{}
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	ts.VU.State().TracerProvider = provider
	parentCtx, parent := provider.Tracer("test").Start(ts.VU.Context(), "iteration")
	ts.VU.CtxField = parentCtx

	_, err = ts.Run(`
		client.connect("GRPCBIN_ADDR");
		client.invoke("grpc.testing.TestService/EmptyCall", {});
		client.healthCheck();
	`)
	require.NoError(t, err)

	_, err = ts.RunOnEventLoop(`
		client.asyncInvoke("grpc.testing.TestService/EmptyCall", {}).then(
			() => call("async-done"),
			(error) => { throw error; },
		);
	`)
	require.NoError(t, err)

	_, err = ts.RunOnEventLoop(`
		const stream = new grpc.Stream(client, "main.FeatureExplorer/ListFeatures");
		stream.on("end", () => call("stream-end"));
		stream.on("error", (error) => { throw error; });
		stream.write({
			lo: { latitude: 1, longitude: 2 },
			hi: { latitude: 3, longitude: 4 },
		});
	`)
	require.NoError(t, err)
	_, err = ts.Run(`client.close();`)
	require.NoError(t, err)

	require.Equal(t, []string{"async-done", "stream-end"}, ts.callRecorder.Recorded())
	for _, md := range []metadata.MD{<-unaryMetadata, <-unaryMetadata, <-healthMetadata, <-streamMetadata} {
		require.NotEmpty(t, md.Get("traceparent"))
	}

	spans := exporter.exportedSpans()
	require.Len(t, spans, 4)
	names := make(map[string]int, len(spans))
	for _, span := range spans {
		names[span.Name()]++
		require.Equal(t, parent.SpanContext().SpanID(), span.Parent().SpanID())
		require.Equal(t, oteltrace.SpanKindClient, span.SpanKind())
	}
	require.Equal(t, map[string]int{
		"grpc.testing.TestService/EmptyCall": 2,
		"grpc.health.v1.Health/Check":        1,
		"main.FeatureExplorer/ListFeatures":  1,
	}, names)
	parent.End()
}
