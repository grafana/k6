package grpc

import (
	"bytes"
	"context"
	"encoding/base64"
	"net"
	"testing"
	"time"

	"github.com/grafana/sobek"
	"go.k6.io/k6/v2/internal/lib/netext/grpcext"
	grpc_testing "go.k6.io/k6/v2/internal/lib/testutils/httpmultibin/grpc_testing"
	"go.k6.io/k6/v2/js/modulestest"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/metrics"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

const benchmarkClientInvokeMethod = "grpc.testing.TestService/UnaryCall"

func BenchmarkClientInvokeResponse(b *testing.B) {
	for _, responseSize := range []struct {
		name string
		size int
	}{
		{name: "128B", size: 128},
		{name: "16KiB", size: 16 * 1024},
	} {
		b.Run(responseSize.name, func(b *testing.B) {
			client, request := newBenchmarkClient(b, responseSize.size)
			expectedBodyLength := base64.StdEncoding.EncodedLen(responseSize.size)

			response, err := client.Invoke(benchmarkClientInvokeMethod, request, nil)
			if err != nil {
				b.Fatal(err)
			}
			validateBenchmarkResponse(b, response.Message, expectedBodyLength)

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				response, err := client.Invoke(benchmarkClientInvokeMethod, request, nil)
				if err != nil {
					b.Fatal(err)
				}
				validateBenchmarkResponse(b, response.Message, expectedBodyLength)
			}
		})
	}
}

func newBenchmarkClient(b *testing.B, responseSize int) (*Client, sobek.Value) {
	b.Helper()

	runtime := modulestest.NewRuntime(b)
	registry := metrics.NewRegistry()
	runtime.MoveToVUContext(&lib.State{
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(registry),
		Options: lib.Options{
			SystemTags: metrics.NewSystemTagSet(),
		},
		Samples: make(chan metrics.SampleContainer),
		Tags:    lib.NewVUStateTags(registry.RootTagSet()),
	})

	client := &Client{
		vu:    runtime.VU,
		types: new(protoregistry.Types),
	}
	descriptors := &descriptorpb.FileDescriptorSet{
		File: walkFileDescriptors(make(map[string]struct{}), grpc_testing.File_test_grpc_testing_test_proto),
	}
	if _, err := client.convertToMethodInfo(descriptors); err != nil {
		b.Fatal(err)
	}

	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	grpc_testing.RegisterTestServiceServer(server, benchmarkTestService{
		response: &grpc_testing.SimpleResponse{
			Payload: &grpc_testing.Payload{
				Body: bytes.Repeat([]byte("x"), responseSize),
			},
		},
	})
	go func() { _ = server.Serve(listener) }()
	b.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn, err := grpcext.Dial(ctx, "bufconn", client.types,
		grpc.WithBlock(), //nolint:staticcheck // grpcext.Dial uses grpc.DialContext, which supports WithBlock.
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return listener.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		b.Fatal(err)
	}
	client.conn = conn
	b.Cleanup(func() { _ = client.Close() })

	return client, runtime.VU.Runtime().ToValue(map[string]any{"responseSize": responseSize})
}

func validateBenchmarkResponse(b *testing.B, message any, expectedBodyLength int) {
	b.Helper()

	response, ok := message.(map[string]any)
	if !ok {
		b.Fatalf("response message has type %T, want map[string]any", message)
	}
	payload, ok := response["payload"].(map[string]any)
	if !ok {
		b.Fatalf("response payload has type %T, want map[string]any", response["payload"])
	}
	body, ok := payload["body"].(string)
	if !ok {
		b.Fatalf("response body has type %T, want string", payload["body"])
	}
	if len(body) != expectedBodyLength {
		b.Fatalf("response body has length %d, want %d", len(body), expectedBodyLength)
	}
}

type benchmarkTestService struct {
	grpc_testing.UnimplementedTestServiceServer

	response *grpc_testing.SimpleResponse
}

func (s benchmarkTestService) UnaryCall(context.Context, *grpc_testing.SimpleRequest) (*grpc_testing.SimpleResponse, error) {
	return s.response, nil
}
