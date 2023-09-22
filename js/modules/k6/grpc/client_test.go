package grpc_test

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"strings"
	"testing"

	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"gopkg.in/guregu/null.v3"

	"github.com/golang/protobuf/ptypes/any"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	grpcstats "google.golang.org/grpc/stats"
	"google.golang.org/grpc/status"

	k6grpc "go.k6.io/k6/js/modules/k6/grpc"
	"go.k6.io/k6/lib/netext/grpcext"
	"go.k6.io/k6/lib/testutils/httpmultibin"
	grpcanytesting "go.k6.io/k6/lib/testutils/httpmultibin/grpc_any_testing"
	"go.k6.io/k6/lib/testutils/httpmultibin/grpc_testing"
	"go.k6.io/k6/lib/testutils/httpmultibin/grpc_wrappers_testing"
	"go.k6.io/k6/metrics"
)

func TestClient(t *testing.T) {
	t.Parallel()

	tests := []testcase{
		{
			name: "BadTLS",
			setup: func(tb *httpmultibin.HTTPMultiBin) {
				// changing the pointer's value
				// for affecting the lib.State
				// that uses the same pointer
				*tb.TLSClientConfig = tls.Config{
					MinVersion: tls.VersionTLS13,
				}
			},
			initString: codeBlock{
				code: `var client = new grpc.Client();`,
			},
			vuString: codeBlock{
				code: `client.connect("GRPCBIN_ADDR", {timeout: '1s'})`,
				err:  "certificate signed by unknown authority",
			},
		},
		{
			name: "New",
			initString: codeBlock{
				code: `
			var client = new grpc.Client();
			if (!client) throw new Error("no client created")`,
			},
		},
		{
			name: "LoadNotFound",
			initString: codeBlock{
				code: `
			var client = new grpc.Client();
			client.load([], "./does_not_exist.proto");`,
				err: "no such file or directory",
				// (rogchap) this is a bit of a hack as windows reports a different system error than unix.
				windowsErr: "The system cannot find the file specified",
			},
		},
		{
			name: "Load",
			initString: codeBlock{
				code: `
			var client = new grpc.Client();
			client.load([], "../../../../lib/testutils/httpmultibin/grpc_testing/test.proto");`,
				val: []k6grpc.MethodInfo{{MethodInfo: grpc.MethodInfo{Name: "EmptyCall", IsClientStream: false, IsServerStream: false}, Package: "grpc.testing", Service: "TestService", FullMethod: "/grpc.testing.TestService/EmptyCall"}, {MethodInfo: grpc.MethodInfo{Name: "UnaryCall", IsClientStream: false, IsServerStream: false}, Package: "grpc.testing", Service: "TestService", FullMethod: "/grpc.testing.TestService/UnaryCall"}, {MethodInfo: grpc.MethodInfo{Name: "StreamingOutputCall", IsClientStream: false, IsServerStream: true}, Package: "grpc.testing", Service: "TestService", FullMethod: "/grpc.testing.TestService/StreamingOutputCall"}, {MethodInfo: grpc.MethodInfo{Name: "StreamingInputCall", IsClientStream: true, IsServerStream: false}, Package: "grpc.testing", Service: "TestService", FullMethod: "/grpc.testing.TestService/StreamingInputCall"}, {MethodInfo: grpc.MethodInfo{Name: "FullDuplexCall", IsClientStream: true, IsServerStream: true}, Package: "grpc.testing", Service: "TestService", FullMethod: "/grpc.testing.TestService/FullDuplexCall"}, {MethodInfo: grpc.MethodInfo{Name: "HalfDuplexCall", IsClientStream: true, IsServerStream: true}, Package: "grpc.testing", Service: "TestService", FullMethod: "/grpc.testing.TestService/HalfDuplexCall"}},
			},
		},
		{
			name: "LoadProtosetNotFound",
			initString: codeBlock{
				code: `
			var client = new grpc.Client();
			client.loadProtoset("./does_not_exist.protoset");`,
				err: "couldn't open protoset",
			},
		},
		{
			name: "LoadProtosetWrongFormat",
			initString: codeBlock{
				code: `
			var client = new grpc.Client();
			client.loadProtoset("../../../../lib/testutils/httpmultibin/grpc_protoset_testing/test_message.proto");`,
				err: "couldn't unmarshal protoset",
			},
		},
		{
			name: "LoadProtoset",
			initString: codeBlock{
				code: `
			var client = new grpc.Client();
			client.loadProtoset("../../../../lib/testutils/httpmultibin/grpc_protoset_testing/test.protoset");`,
				val: []k6grpc.MethodInfo{
					{
						MethodInfo: grpc.MethodInfo{Name: "Test", IsClientStream: false, IsServerStream: false},
						Package:    "grpc.protoset.testing", Service: "TestService", FullMethod: "/grpc.protoset.testing.TestService/Test",
					},
				},
			},
		},
		{
			name: "ConnectInit",
			initString: codeBlock{
				code: `
			var client = new grpc.Client();
			client.load([], "../../../../lib/testutils/httpmultibin/grpc_testing/test.proto");
			client.connect();`,
				err: "connecting to a gRPC server in the init context is not supported",
			},
		},
		{
			name: "InvokeInit",
			initString: codeBlock{
				code: `
			var client = new grpc.Client();
			client.load([], "../../../../lib/testutils/httpmultibin/grpc_testing/test.proto");
			var err = client.invoke();
			throw new Error(err)`,
				err: "invoking RPC methods in the init context is not supported",
			},
		},
		{
			name: "NoConnect",
			initString: codeBlock{
				code: `
				var client = new grpc.Client();
				client.load([], "../../../../lib/testutils/httpmultibin/grpc_testing/test.proto");
				client.invoke("grpc.testing.TestService/EmptyCall", {})`,
				err: "invoking RPC methods in the init context is not supported",
			},
		},
		{
			name: "UnknownConnectParam",
			initString: codeBlock{code: `
				var client = new grpc.Client();
				client.load([], "../../../../lib/testutils/httpmultibin/grpc_testing/test.proto");`},
			vuString: codeBlock{
				code: `client.connect("GRPCBIN_ADDR", { name: "k6" });`,
				err:  `unknown connect param: "name"`,
			},
		},
		{
			name: "ConnectInvalidTimeout",
			initString: codeBlock{
				code: `
				var client = new grpc.Client();
				client.load([], "../../../../lib/testutils/httpmultibin/grpc_testing/test.proto");`,
			},
			vuString: codeBlock{
				code: `client.connect("GRPCBIN_ADDR", { timeout: "k6" });`,
				err:  "invalid duration",
			},
		},
		{
			name: "ConnectStringTimeout",
			initString: codeBlock{code: `
				var client = new grpc.Client();
				client.load([], "../../../../lib/testutils/httpmultibin/grpc_testing/test.proto");`},
			vuString: codeBlock{code: `client.connect("GRPCBIN_ADDR", { timeout: "1h3s" });`},
		},
		{
			name: "ConnectIntegerTimeout",
			initString: codeBlock{code: `
				var client = new grpc.Client();
				client.load([], "../../../../lib/testutils/httpmultibin/grpc_testing/test.proto");`},
			vuString: codeBlock{code: `client.connect("GRPCBIN_ADDR", { timeout: 3000 });`},
		},
		{
			name: "ConnectFloatTimeout",
			initString: codeBlock{code: `
				var client = new grpc.Client();
				client.load([], "../../../../lib/testutils/httpmultibin/grpc_testing/test.proto");`},
			vuString: codeBlock{code: `client.connect("GRPCBIN_ADDR", { timeout: 3456.3 });`},
		},
		{
			name: "Connect",
			initString: codeBlock{code: `
				var client = new grpc.Client();
				client.load([], "../../../../lib/testutils/httpmultibin/grpc_testing/test.proto");`},
			vuString: codeBlock{code: `client.connect("GRPCBIN_ADDR");`},
		},
		{
			name: "InvokeNotFound",
			initString: codeBlock{code: `
				var client = new grpc.Client();
				client.load([], "../../../../lib/testutils/httpmultibin/grpc_testing/test.proto");`},
			vuString: codeBlock{
				code: `
				client.connect("GRPCBIN_ADDR");
				client.invoke("foo/bar", {})`,
				err: `method "/foo/bar" not found in file descriptors`,
			},
		},
		{
			name: "InvokeInvalidParam",
			initString: codeBlock{code: `
				var client = new grpc.Client();
				client.load([], "../../../../lib/testutils/httpmultibin/grpc_testing/test.proto");`},
			vuString: codeBlock{
				code: `
				client.connect("GRPCBIN_ADDR");
				client.invoke("grpc.testing.TestService/EmptyCall", {}, { void: true })`,
				err: `unknown param: "void"`,
			},
		},
		{
			name: "InvokeNilRequest",
			initString: codeBlock{code: `
				var client = new grpc.Client();
				client.load([], "../../../../lib/testutils/httpmultibin/grpc_testing/test.proto");`},
			vuString: codeBlock{
				code: `
				client.connect("GRPCBIN_ADDR");
				client.invoke("grpc.testing.TestService/EmptyCall")`,
				err: `request cannot be nil`,
			},
		},
		{
			name: "InvokeInvalidTimeoutType",
			initString: codeBlock{code: `
				var client = new grpc.Client();
				client.load([], "../../../../lib/testutils/httpmultibin/grpc_testing/test.proto");`},
			vuString: codeBlock{
				code: `
				client.connect("GRPCBIN_ADDR");
				client.invoke("grpc.testing.TestService/EmptyCall", {}, { timeout: true })`,
				err: "invalid timeout value: unable to use type bool as a duration value",
			},
		},
		{
			name: "InvokeInvalidTimeout",
			initString: codeBlock{code: `
				var client = new grpc.Client();
				client.load([], "../../../../lib/testutils/httpmultibin/grpc_testing/test.proto");`},
			vuString: codeBlock{
				code: `
				client.connect("GRPCBIN_ADDR");
				client.invoke("grpc.testing.TestService/EmptyCall", {}, { timeout: "please" })`,
				err: "invalid duration",
			},
		},
		{
			name: "InvokeStringTimeout",
			initString: codeBlock{code: `
				var client = new grpc.Client();
				client.load([], "../../../../lib/testutils/httpmultibin/grpc_testing/test.proto");`},
			vuString: codeBlock{
				code: `
				client.connect("GRPCBIN_ADDR");
				client.invoke("grpc.testing.TestService/EmptyCall", {}, { timeout: "1h42m" })`,
			},
		},
		{
			name: "InvokeFloatTimeout",
			initString: codeBlock{code: `
				var client = new grpc.Client();
				client.load([], "../../../../lib/testutils/httpmultibin/grpc_testing/test.proto");`},
			vuString: codeBlock{
				code: `
				client.connect("GRPCBIN_ADDR");
				client.invoke("grpc.testing.TestService/EmptyCall", {}, { timeout: 400.50 })`,
			},
		},
		{
			name: "InvokeIntegerTimeout",
			initString: codeBlock{
				code: `
				var client = new grpc.Client();
				client.load([], "../../../../lib/testutils/httpmultibin/grpc_testing/test.proto");`,
			},
			vuString: codeBlock{
				code: `
				client.connect("GRPCBIN_ADDR");
				client.invoke("grpc.testing.TestService/EmptyCall", {}, { timeout: 2000 })`,
			},
		},
		{
			name: "Invoke",
			initString: codeBlock{code: `
				var client = new grpc.Client();
				client.load([], "../../../../lib/testutils/httpmultibin/grpc_testing/test.proto");`},
			setup: func(tb *httpmultibin.HTTPMultiBin) {
				tb.GRPCStub.EmptyCallFunc = func(context.Context, *grpc_testing.Empty) (*grpc_testing.Empty, error) {
					return &grpc_testing.Empty{}, nil
				}
			},
			vuString: codeBlock{
				code: `
				client.connect("GRPCBIN_ADDR");
				var resp = client.invoke("grpc.testing.TestService/EmptyCall", {})
				if (resp.status !== grpc.StatusOK) {
					throw new Error("unexpected error: " + JSON.stringify(resp.error) + "or status: " + resp.status)
				}`,
				asserts: func(t *testing.T, rb *httpmultibin.HTTPMultiBin, samples chan metrics.SampleContainer, _ error) {
					samplesBuf := metrics.GetBufferedSamples(samples)
					assertMetricEmitted(t, metrics.GRPCReqDurationName, samplesBuf, rb.Replacer.Replace("GRPCBIN_ADDR/grpc.testing.TestService/EmptyCall"))
				},
			},
		},
		{
			name: "InvokeAnyProto",
			initString: codeBlock{code: `
				var client = new grpc.Client();
				client.load([], "../../../../lib/testutils/httpmultibin/grpc_any_testing/any_test.proto");`},
			setup: func(tb *httpmultibin.HTTPMultiBin) {
				tb.GRPCAnyStub.SumFunc = func(ctx context.Context, req *grpcanytesting.SumRequest) (*grpcanytesting.SumReply, error) {
					var sumRequestData grpcanytesting.SumRequestData
					if err := req.Data.UnmarshalTo(&sumRequestData); err != nil {
						return nil, err
					}

					sumReplyData := &grpcanytesting.SumReplyData{
						V:   sumRequestData.A + sumRequestData.B,
						Err: "",
					}
					sumReply := &grpcanytesting.SumReply{
						Data: &any.Any{},
					}
					if err := sumReply.Data.MarshalFrom(sumReplyData); err != nil {
						return nil, err
					}

					return sumReply, nil
				}
			},
			vuString: codeBlock{
				code: `
				client.connect("GRPCBIN_ADDR");
				var resp = client.invoke("grpc.any.testing.AnyTestService/Sum",  {
					data: {
						"@type": "type.googleapis.com/grpc.any.testing.SumRequestData",
						"a": 1,
						"b": 2,
					},
				})
				if (resp.status !== grpc.StatusOK) {
					throw new Error("unexpected error: " + JSON.stringify(resp.error) + "or status: " + resp.status)
				}
				if (resp.message.data.v !== "3") {
					throw new Error("unexpected resp message data")
				}`,
				asserts: func(t *testing.T, rb *httpmultibin.HTTPMultiBin, samples chan metrics.SampleContainer, _ error) {
					samplesBuf := metrics.GetBufferedSamples(samples)
					assertMetricEmitted(t, metrics.GRPCReqDurationName, samplesBuf, rb.Replacer.Replace("GRPCBIN_ADDR/grpc.any.testing.AnyTestService/Sum"))
				},
			},
		},
		{
			name: "RequestMessage",
			initString: codeBlock{
				code: `
				var client = new grpc.Client();
				client.load([], "../../../../lib/testutils/httpmultibin/grpc_testing/test.proto");`,
			},
			setup: func(tb *httpmultibin.HTTPMultiBin) {
				tb.GRPCStub.UnaryCallFunc = func(_ context.Context, req *grpc_testing.SimpleRequest) (*grpc_testing.SimpleResponse, error) {
					if req.Payload == nil || string(req.Payload.Body) != "负载测试" {
						return nil, status.Error(codes.InvalidArgument, "")
					}
					return &grpc_testing.SimpleResponse{}, nil
				}
			},
			vuString: codeBlock{code: `
				client.connect("GRPCBIN_ADDR");
				var resp = client.invoke("grpc.testing.TestService/UnaryCall", { payload: { body: "6LSf6L295rWL6K+V"} })
				if (resp.status !== grpc.StatusOK) {
					throw new Error("server did not receive the correct request message")
				}`},
		},
		{
			name: "RequestHeaders",
			initString: codeBlock{
				code: `
				var client = new grpc.Client();
				client.load([], "../../../../lib/testutils/httpmultibin/grpc_testing/test.proto");`,
			},
			setup: func(tb *httpmultibin.HTTPMultiBin) {
				tb.GRPCStub.EmptyCallFunc = func(ctx context.Context, _ *grpc_testing.Empty) (*grpc_testing.Empty, error) {
					md, ok := metadata.FromIncomingContext(ctx)
					if !ok || len(md["x-load-tester"]) == 0 || md["x-load-tester"][0] != "k6" {
						return nil, status.Error(codes.FailedPrecondition, "")
					}

					return &grpc_testing.Empty{}, nil
				}
			},
			vuString: codeBlock{code: `
				client.connect("GRPCBIN_ADDR");
				var resp = client.invoke("grpc.testing.TestService/EmptyCall", {}, { metadata: { "X-Load-Tester": "k6" } })
				if (resp.status !== grpc.StatusOK) {
					throw new Error("failed to send correct headers in the request")
				}
			`},
		},
		{
			name: "RequestBinHeaders",
			initString: codeBlock{
				code: `
				var client = new grpc.Client();
				client.load([], "../../../../lib/testutils/httpmultibin/grpc_testing/test.proto");`,
			},
			setup: func(tb *httpmultibin.HTTPMultiBin) {
				tb.GRPCStub.EmptyCallFunc = func(ctx context.Context, _ *grpc_testing.Empty) (*grpc_testing.Empty, error) {
					md, ok := metadata.FromIncomingContext(ctx)
					if !ok || len(md["x-load-tester-bin"]) == 0 || md["x-load-tester-bin"][0] != string([]byte{2, 200}) {
						return nil, status.Error(codes.FailedPrecondition, "")
					}

					return &grpc_testing.Empty{}, nil
				}
			},
			vuString: codeBlock{code: `
				client.connect("GRPCBIN_ADDR");
				var resp = client.invoke("grpc.testing.TestService/EmptyCall", {}, { metadata: { "X-Load-Tester-bin": new Uint8Array([2, 200]) } })
				if (resp.status !== grpc.StatusOK) {
					throw new Error("failed to send correct headers in the request")
				}
			`},
		},
		{
			name: "ResponseMessage",
			initString: codeBlock{
				code: `
				var client = new grpc.Client();
				client.load([], "../../../../lib/testutils/httpmultibin/grpc_testing/test.proto");`,
			},
			setup: func(tb *httpmultibin.HTTPMultiBin) {
				tb.GRPCStub.UnaryCallFunc = func(context.Context, *grpc_testing.SimpleRequest) (*grpc_testing.SimpleResponse, error) {
					return &grpc_testing.SimpleResponse{
						OauthScope: "水",
					}, nil
				}
			},
			vuString: codeBlock{
				code: `
				client.connect("GRPCBIN_ADDR");
				var resp = client.invoke("grpc.testing.TestService/UnaryCall", {})
				if (!resp.message || resp.message.username !== "" || resp.message.oauthScope !== "水") {
					throw new Error("unexpected response message: " + JSON.stringify(resp.message))
				}`,
				asserts: func(t *testing.T, rb *httpmultibin.HTTPMultiBin, samples chan metrics.SampleContainer, _ error) {
					samplesBuf := metrics.GetBufferedSamples(samples)
					assertMetricEmitted(t, metrics.GRPCReqDurationName, samplesBuf, rb.Replacer.Replace("GRPCBIN_ADDR/grpc.testing.TestService/UnaryCall"))
				},
			},
		},
		{
			name: "ResponseError",
			initString: codeBlock{
				code: `
				var client = new grpc.Client();
				client.load([], "../../../../lib/testutils/httpmultibin/grpc_testing/test.proto");`,
			},
			setup: func(tb *httpmultibin.HTTPMultiBin) {
				tb.GRPCStub.EmptyCallFunc = func(context.Context, *grpc_testing.Empty) (*grpc_testing.Empty, error) {
					return nil, status.Error(codes.DataLoss, "foobar")
				}
			},
			vuString: codeBlock{
				code: `
				client.connect("GRPCBIN_ADDR");
				var resp = client.invoke("grpc.testing.TestService/EmptyCall", {})
				if (resp.status !== grpc.StatusDataLoss) {
					throw new Error("unexpected error status: " + resp.status)
				}
				if (!resp.error || resp.error.message !== "foobar" || resp.error.code !== 15) {
					throw new Error("unexpected error object: " + JSON.stringify(resp.error.code))
				}`,
				asserts: func(t *testing.T, rb *httpmultibin.HTTPMultiBin, samples chan metrics.SampleContainer, _ error) {
					samplesBuf := metrics.GetBufferedSamples(samples)
					assertMetricEmitted(t, metrics.GRPCReqDurationName, samplesBuf, rb.Replacer.Replace("GRPCBIN_ADDR/grpc.testing.TestService/EmptyCall"))
				},
			},
		},
		{
			name: "ResponseHeaders",
			initString: codeBlock{
				code: `
				var client = new grpc.Client();
				client.load([], "../../../../lib/testutils/httpmultibin/grpc_testing/test.proto");`,
			},
			setup: func(tb *httpmultibin.HTTPMultiBin) {
				tb.GRPCStub.EmptyCallFunc = func(ctx context.Context, _ *grpc_testing.Empty) (*grpc_testing.Empty, error) {
					md := metadata.Pairs("foo", "bar")
					_ = grpc.SetHeader(ctx, md)
					return &grpc_testing.Empty{}, nil
				}
			},
			vuString: codeBlock{
				code: `
				client.connect("GRPCBIN_ADDR");
				var resp = client.invoke("grpc.testing.TestService/EmptyCall", {})
				if (resp.status !== grpc.StatusOK) {
					throw new Error("unexpected error status: " + resp.status)
				}
				if (!resp.headers || !resp.headers["foo"] || resp.headers["foo"][0] !== "bar") {
					throw new Error("unexpected headers object: " + JSON.stringify(resp.trailers))
				}`,
				asserts: func(t *testing.T, rb *httpmultibin.HTTPMultiBin, samples chan metrics.SampleContainer, _ error) {
					samplesBuf := metrics.GetBufferedSamples(samples)
					assertMetricEmitted(t, metrics.GRPCReqDurationName, samplesBuf, rb.Replacer.Replace("GRPCBIN_ADDR/grpc.testing.TestService/EmptyCall"))
				},
			},
		},
		{
			name: "ResponseTrailers",
			initString: codeBlock{
				code: `
				var client = new grpc.Client();
				client.load([], "../../../../lib/testutils/httpmultibin/grpc_testing/test.proto");`,
			},
			setup: func(tb *httpmultibin.HTTPMultiBin) {
				tb.GRPCStub.EmptyCallFunc = func(ctx context.Context, _ *grpc_testing.Empty) (*grpc_testing.Empty, error) {
					md := metadata.Pairs("foo", "bar")
					_ = grpc.SetTrailer(ctx, md)
					return &grpc_testing.Empty{}, nil
				}
			},
			vuString: codeBlock{
				code: `
				client.connect("GRPCBIN_ADDR");
				var resp = client.invoke("grpc.testing.TestService/EmptyCall", {})
				if (resp.status !== grpc.StatusOK) {
					throw new Error("unexpected error status: " + resp.status)
				}
				if (!resp.trailers || !resp.trailers["foo"] || resp.trailers["foo"][0] !== "bar") {
					throw new Error("unexpected trailers object: " + JSON.stringify(resp.trailers))
				}`,
				asserts: func(t *testing.T, rb *httpmultibin.HTTPMultiBin, samples chan metrics.SampleContainer, _ error) {
					samplesBuf := metrics.GetBufferedSamples(samples)
					assertMetricEmitted(t, metrics.GRPCReqDurationName, samplesBuf, rb.Replacer.Replace("GRPCBIN_ADDR/grpc.testing.TestService/EmptyCall"))
				},
			},
		},
		{
			name: "LoadNotInit",
			setup: func(tb *httpmultibin.HTTPMultiBin) {
				tb.GRPCStub.EmptyCallFunc = func(ctx context.Context, _ *grpc_testing.Empty) (*grpc_testing.Empty, error) {
					md := metadata.Pairs("foo", "bar")
					_ = grpc.SetTrailer(ctx, md)
					return &grpc_testing.Empty{}, nil
				}
			},
			initString: codeBlock{
				code: `
				var client = new grpc.Client();
				client.load([], "../../../../lib/testutils/httpmultibin/grpc_testing/test.proto");`,
			},
			vuString: codeBlock{
				code: `client.load()`,
				err:  "load must be called in the init context",
			},
		},
		{
			name: "ReflectUnregistered",
			initString: codeBlock{
				code: `var client = new grpc.Client();`,
			},
			vuString: codeBlock{
				code: `client.connect("GRPCBIN_ADDR", {reflect: true})`,
				err:  "rpc error: code = Unimplemented desc = unknown service grpc.reflection.v1alpha.ServerReflection",
			},
		},
		{
			name: "Reflect",
			setup: func(tb *httpmultibin.HTTPMultiBin) {
				reflection.Register(tb.ServerGRPC)
			},
			initString: codeBlock{
				code: `var client = new grpc.Client();`,
			},
			vuString: codeBlock{
				code: `client.connect("GRPCBIN_ADDR", {reflect: true})`,
			},
		},
		{
			name: "ReflectV1",
			setup: func(tb *httpmultibin.HTTPMultiBin) {
				reflection.RegisterV1(tb.ServerGRPC)
			},
			initString: codeBlock{
				code: `var client = new grpc.Client();`,
			},
			vuString: codeBlock{
				code: `client.connect("GRPCBIN_ADDR", {reflect: true})`,
			},
		},
		{
			name: "ReflectBadParam",
			setup: func(tb *httpmultibin.HTTPMultiBin) {
				reflection.Register(tb.ServerGRPC)
			},
			initString: codeBlock{
				code: `var client = new grpc.Client();`,
			},
			vuString: codeBlock{
				code: `client.connect("GRPCBIN_ADDR", {reflect: "true"})`,
				err:  `invalid reflect value`,
			},
		},
		{
			name: "ReflectInvokeNoExist",
			setup: func(tb *httpmultibin.HTTPMultiBin) {
				reflection.Register(tb.ServerGRPC)
				tb.GRPCStub.EmptyCallFunc = func(ctx context.Context, _ *grpc_testing.Empty) (*grpc_testing.Empty, error) {
					return &grpc_testing.Empty{}, nil
				}
			},
			initString: codeBlock{
				code: `var client = new grpc.Client();`,
			},
			vuString: codeBlock{
				code: `
					client.connect("GRPCBIN_ADDR", {reflect: true})
					client.invoke("foo/bar", {})
				`,
				err: `method "/foo/bar" not found in file descriptors`,
			},
		},
		{
			name: "ReflectInvoke",
			setup: func(tb *httpmultibin.HTTPMultiBin) {
				reflection.Register(tb.ServerGRPC)
				tb.GRPCStub.EmptyCallFunc = func(ctx context.Context, _ *grpc_testing.Empty) (*grpc_testing.Empty, error) {
					return &grpc_testing.Empty{}, nil
				}
			},
			initString: codeBlock{
				code: `var client = new grpc.Client();`,
			},
			vuString: codeBlock{
				code: `
					client.connect("GRPCBIN_ADDR", {reflect: true})
					client.invoke("grpc.testing.TestService/EmptyCall", {})
				`,
			},
		},
		{
			name: "MaxReceiveSizeBadParam",
			setup: func(tb *httpmultibin.HTTPMultiBin) {
				reflection.Register(tb.ServerGRPC)
			},
			initString: codeBlock{
				code: `var client = new grpc.Client();`,
			},
			vuString: codeBlock{
				code: `client.connect("GRPCBIN_ADDR", {maxReceiveSize: "error"})`,
				err:  `invalid maxReceiveSize value: '"error"', it needs to be an integer`,
			},
		},
		{
			name: "MaxReceiveSizeNonPositiveInteger",
			setup: func(tb *httpmultibin.HTTPMultiBin) {
				reflection.Register(tb.ServerGRPC)
			},
			initString: codeBlock{
				code: `var client = new grpc.Client();`,
			},
			vuString: codeBlock{
				code: `client.connect("GRPCBIN_ADDR", {maxReceiveSize: -1})`,
				err:  `invalid maxReceiveSize value: '-1, it needs to be a positive integer`,
			},
		},
		{
			name: "ReceivedMessageLargerThanMax",
			initString: codeBlock{
				code: `
				var client = new grpc.Client();
				client.load([], "../../../../lib/testutils/httpmultibin/grpc_testing/test.proto");`,
			},
			setup: func(tb *httpmultibin.HTTPMultiBin) {
				tb.GRPCStub.UnaryCallFunc = func(_ context.Context, req *grpc_testing.SimpleRequest) (*grpc_testing.SimpleResponse, error) {
					response := &grpc_testing.SimpleResponse{}
					response.Payload = req.Payload
					return response, nil
				}
			},
			vuString: codeBlock{
				code: `
				client.connect("GRPCBIN_ADDR", {maxReceiveSize: 1})
				var resp = client.invoke("grpc.testing.TestService/UnaryCall", { payload: { body: "testMaxReceiveSize"} })
				if (resp.status == grpc.StatusResourceExhausted) {
					throw new Error(resp.error.message)
				}
				`,
				err: `received message larger than max`,
			},
		},
		{
			name: "MaxSendSizeBadParam",
			setup: func(tb *httpmultibin.HTTPMultiBin) {
				reflection.Register(tb.ServerGRPC)
			},
			initString: codeBlock{
				code: `var client = new grpc.Client();`,
			},
			vuString: codeBlock{
				code: `client.connect("GRPCBIN_ADDR", {maxSendSize: "error"})`,
				err:  `invalid maxSendSize value: '"error"', it needs to be an integer`,
			},
		},
		{
			name: "MaxSendSizeNonPositiveInteger",
			setup: func(tb *httpmultibin.HTTPMultiBin) {
				reflection.Register(tb.ServerGRPC)
			},
			initString: codeBlock{
				code: `var client = new grpc.Client();`,
			},
			vuString: codeBlock{
				code: `client.connect("GRPCBIN_ADDR", {maxSendSize: -1})`,
				err:  `invalid maxSendSize value: '-1, it needs to be a positive integer`,
			},
		},
		{
			name: "SentMessageLargerThanMax",
			initString: codeBlock{
				code: `
				var client = new grpc.Client();
				client.load([], "../../../../lib/testutils/httpmultibin/grpc_testing/test.proto");`,
			},
			setup: func(tb *httpmultibin.HTTPMultiBin) {
				tb.GRPCStub.UnaryCallFunc = func(context.Context, *grpc_testing.SimpleRequest) (*grpc_testing.SimpleResponse, error) {
					return &grpc_testing.SimpleResponse{}, nil
				}
			},
			vuString: codeBlock{
				code: `
				client.connect("GRPCBIN_ADDR", {maxSendSize: 1})
				var resp = client.invoke("grpc.testing.TestService/UnaryCall", { payload: { body: "testMaxSendSize"} })
				if (resp.status == grpc.StatusResourceExhausted) {
					throw new Error(resp.error.message)
				}
				`,
				err: `trying to send message larger than max`,
			},
		},
		{
			name: "Close",
			initString: codeBlock{
				code: `
				var client = new grpc.Client();
				client.load([], "../../../../lib/testutils/httpmultibin/grpc_testing/test.proto");`,
			},
			vuString: codeBlock{
				code: `
			client.close();
			client.invoke();`,
				err: "no gRPC connection",
			},
		},
		{
			name: "Wrappers",
			setup: func(hb *httpmultibin.HTTPMultiBin) {
				srv := grpc_wrappers_testing.Register(hb.ServerGRPC)

				srv.TestStringImplementation = func(_ context.Context, sv *wrappers.StringValue) (*wrappers.StringValue, error) {
					return &wrapperspb.StringValue{
						Value: "hey " + sv.Value,
					}, nil
				}
			},
			initString: codeBlock{
				code: `
				const client = new grpc.Client();
				client.load([], "../../../../lib/testutils/httpmultibin/grpc_wrappers_testing/test.proto");
				`,
			},
			vuString: codeBlock{
				code: `
				client.connect("GRPCBIN_ADDR");

				let respString = client.invoke("grpc.wrappers.testing.Service/TestString", "John")
				if (respString.message !== "hey John") {
					throw new Error("expected to get 'hey John', but got a " + respString.message)
				}
			`,
			},
		},
		{
			name: "WrappersWithReflection",
			setup: func(hb *httpmultibin.HTTPMultiBin) {
				reflection.Register(hb.ServerGRPC)

				srv := grpc_wrappers_testing.Register(hb.ServerGRPC)

				srv.TestIntegerImplementation = func(_ context.Context, iv *wrappers.Int64Value) (*wrappers.Int64Value, error) {
					return &wrappers.Int64Value{
						Value: 2 * iv.Value,
					}, nil
				}

				srv.TestStringImplementation = func(_ context.Context, sv *wrappers.StringValue) (*wrappers.StringValue, error) {
					return &wrapperspb.StringValue{
						Value: "hey " + sv.Value,
					}, nil
				}

				srv.TestBooleanImplementation = func(_ context.Context, bv *wrappers.BoolValue) (*wrappers.BoolValue, error) {
					return &wrapperspb.BoolValue{
						Value: bv.Value != true,
					}, nil
				}

				srv.TestDoubleImplementation = func(_ context.Context, bv *wrappers.DoubleValue) (*wrappers.DoubleValue, error) {
					return &wrapperspb.DoubleValue{
						Value: bv.Value * 2,
					}, nil
				}
			},
			initString: codeBlock{
				code: `
				const client = new grpc.Client();
				`,
			},
			vuString: codeBlock{
				code: `
				client.connect("GRPCBIN_ADDR", {reflect: true});

				let respString = client.invoke("grpc.wrappers.testing.Service/TestString", "John")
				if (respString.message !== "hey John") {
					throw new Error("expected to get 'hey John', but got a " + respString.message)
				}

				let respInt = client.invoke("grpc.wrappers.testing.Service/TestInteger", "3")
				if (respInt.message !== "6") {
					throw new Error("expected to get '6', but got a " + respInt.message)
				}

				let respDouble = client.invoke("grpc.wrappers.testing.Service/TestDouble", "2.7")
				if (respDouble.message !== 5.4) {
					throw new Error("expected to get '5.4', but got a " + respDouble.message)
				}
			`,
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ts := newTestState(t)

			// setup necessary environment if needed by a test
			if tt.setup != nil {
				tt.setup(ts.httpBin)
			}

			val, err := ts.Run(tt.initString.code)
			assertResponse(t, tt.initString, err, val, ts)

			ts.ToVUContext()

			val, err = ts.Run(tt.vuString.code)
			assertResponse(t, tt.vuString, err, val, ts)
		})
	}
}

func TestClient_TlsParameters(t *testing.T) {
	t.Parallel()

	testingKey := func(s string) string {
		t.Helper()
		return strings.ReplaceAll(s, "TESTING KEY", "PRIVATE KEY")
	}

	clientAuthCA := []byte("-----BEGIN CERTIFICATE-----\nMIIBWzCCAQGgAwIBAgIJAIQMBgLi+DV6MAoGCCqGSM49BAMCMBAxDjAMBgNVBAMM\nBU15IENBMCAXDTIyMDEyMTEyMjkzNloYDzMwMjEwNTI0MTIyOTM2WjAQMQ4wDAYD\nVQQDDAVNeSBDQTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABHnrghULHa2hSa/C\nWimwCn42KWdlPqd6/zs3JgLIxTvBHJJlfbhWbBqtybqyovWd3QykHMIpx0NZmpYn\nG8FoWpmjQjBAMA4GA1UdDwEB/wQEAwIBBjAPBgNVHRMBAf8EBTADAQH/MB0GA1Ud\nDgQWBBSkukBA8lgFvvBJAYKsoSUR+PX71jAKBggqhkjOPQQDAgNIADBFAiEAiFF7\nY54CMNRSBSVMgd4mQgrzJInRH88KpLsQ7VeOAaQCIEa0vaLln9zxIDZQKocml4Db\nAEJr8tDzMKIds6sRTBT4\n-----END CERTIFICATE-----")
	localHostCert := "-----BEGIN CERTIFICATE-----\\nMIIDOTCCAiGgAwIBAgIQSRJrEpBGFc7tNb1fb5pKFzANBgkqhkiG9w0BAQsFADAS\\nMRAwDgYDVQQKEwdBY21lIENvMCAXDTcwMDEwMTAwMDAwMFoYDzIwODQwMTI5MTYw\\nMDAwWjASMRAwDgYDVQQKEwdBY21lIENvMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8A\\nMIIBCgKCAQEA6Gba5tHV1dAKouAaXO3/ebDUU4rvwCUg/CNaJ2PT5xLD4N1Vcb8r\\nbFSW2HXKq+MPfVdwIKR/1DczEoAGf/JWQTW7EgzlXrCd3rlajEX2D73faWJekD0U\\naUgz5vtrTXZ90BQL7WvRICd7FlEZ6FPOcPlumiyNmzUqtwGhO+9ad1W5BqJaRI6P\\nYfouNkwR6Na4TzSj5BrqUfP0FwDizKSJ0XXmh8g8G9mtwxOSN3Ru1QFc61Xyeluk\\nPOGKBV/q6RBNklTNe0gI8usUMlYyoC7ytppNMW7X2vodAelSu25jgx2anj9fDVZu\\nh7AXF5+4nJS4AAt0n1lNY7nGSsdZas8PbQIDAQABo4GIMIGFMA4GA1UdDwEB/wQE\\nAwICpDATBgNVHSUEDDAKBggrBgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MB0GA1Ud\\nDgQWBBStsdjh3/JCXXYlQryOrL4Sh7BW5TAuBgNVHREEJzAlggtleGFtcGxlLmNv\\nbYcEfwAAAYcQAAAAAAAAAAAAAAAAAAAAATANBgkqhkiG9w0BAQsFAAOCAQEAxWGI\\n5NhpF3nwwy/4yB4i/CwwSpLrWUa70NyhvprUBC50PxiXav1TeDzwzLx/o5HyNwsv\\ncxv3HdkLW59i/0SlJSrNnWdfZ19oTcS+6PtLoVyISgtyN6DpkKpdG1cOkW3Cy2P2\\n+tK/tKHRP1Y/Ra0RiDpOAmqn0gCOFGz8+lqDIor/T7MTpibL3IxqWfPrvfVRHL3B\\ngrw/ZQTTIVjjh4JBSW3WyWgNo/ikC1lrVxzl4iPUGptxT36Cr7Zk2Bsg0XqwbOvK\\n5d+NTDREkSnUbie4GeutujmX3Dsx88UiV6UY/4lHJa6I5leHUNOHahRbpbWeOfs/\\nWkBKOclmOV2xlTVuPw==\\n-----END CERTIFICATE-----"
	clientAuth := "-----BEGIN CERTIFICATE-----\\nMIIBVzCB/6ADAgECAgkAg/SeNG3XqB0wCgYIKoZIzj0EAwIwEDEOMAwGA1UEAwwF\\nTXkgQ0EwIBcNMjIwMTIxMTUxMjM0WhgPMzAyMTA1MjQxNTEyMzRaMBExDzANBgNV\\nBAMMBmNsaWVudDBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABKM7OJQMYG4KLtDA\\ngZ8zOg2PimHMmQnjD2HtI4cSwIUJJnvHWLowbFe9fk6XeP9b3dK1ImUI++/EZdVr\\nABAcngejPzA9MA4GA1UdDwEB/wQEAwIBBjAMBgNVHRMBAf8EAjAAMB0GA1UdDgQW\\nBBSttJe1mcPEnBOZ6wvKPG4zL0m1CzAKBggqhkjOPQQDAgNHADBEAiBPSLgKA/r9\\nu/FW6W+oy6Odm1kdNMGCI472iTn545GwJgIgb3UQPOUTOj0IN4JLJYfmYyXviqsy\\nzk9eWNHFXDA9U6U=\\n-----END CERTIFICATE-----"
	clientAuthKey := testingKey("-----BEGIN EC TESTING KEY-----\\nMHcCAQEEINDaMGkOT3thu1A0LfLJr3Jd011/aEG6OArmEQaujwgpoAoGCCqGSM49\\nAwEHoUQDQgAEozs4lAxgbgou0MCBnzM6DY+KYcyZCeMPYe0jhxLAhQkme8dYujBs\\nV71+Tpd4/1vd0rUiZQj778Rl1WsAEByeBw==\\n-----END EC TESTING KEY-----")
	clientAuthKeyEncrypted := testingKey("-----BEGIN EC TESTING KEY-----\\nProc-Type: 4,ENCRYPTED\\nDEK-Info: AES-256-CBC,3E311E9B602231BFB5C752071EE7D652\\n\\nsAKeqbacug0v4ruE1A0CACwGVEGBQVOl1CiGVp5RsxgNZKXzMS6EsTTNLw378coF\\nKXbF+he05HIuzToOz2ANLXov1iCrVpotKVB4l2obTQvg+5VET902ky99Mc9Us7jd\\nUwW8LpXlSlhcNWuUfK6wyosL42TbcIxjqZWaESW+6ww=\\n-----END EC TESTING KEY-----")
	clientAuthBad := "-----BEGIN CERTIFICATE-----\\nMIIB2TCCAX6gAwIBAgIUJIZKiR78AH2ioZ+Jae/sElgH85kwCgYIKoZIzj0EAwIw\\nEDEOMAwGA1UEAwwFTXkgQ0EwHhcNMjMwNzA3MTAyNjQ2WhcNMjQwNzA2MTAyNjQ2\\nWjARMQ8wDQYDVQQDDAZjbGllbnQwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAASj\\nOziUDGBuCi7QwIGfMzoNj4phzJkJ4w9h7SOHEsCFCSZ7x1i6MGxXvX5Ol3j/W93S\\ntSJlCPvvxGXVawAQHJ4Ho4G0MIGxMAkGA1UdEwQCMAAwEQYJYIZIAYb4QgEBBAQD\\nAgWgMCwGCWCGSAGG+EIBDQQfFh1Mb2NhbCBUZXN0IENsaWVudCBDZXJ0aWZpY2F0\\nZTAdBgNVHQ4EFgQUrbSXtZnDxJwTmesLyjxuMy9JtQswHwYDVR0jBBgwFoAUpLpA\\nQPJYBb7wSQGCrKElEfj1+9YwDgYDVR0PAQH/BAQDAgXgMBMGA1UdJQQMMAoGCCsG\\nAQUFBwMEMAoGCCqGSM49BAMCA0kAMEYCIQDcHrzug3V3WvUU+tEKhG1C4cPG5rPJ\\n/y3oOoM0roOnsgIhAP23UmiC6Qdgj+MOhXWSaNt3exWvlxdKmLm2edkxaTs+\\n-----END CERTIFICATE-----"

	trivialKeyPassword := "abc123"

	tests := []testcase{
		{
			name:       "ConnectTlsEmptyTlsSuccess",
			initString: codeBlock{code: "var client = new grpc.Client();"},
			vuString: codeBlock{
				code: `client.connect("GRPCBIN_ADDR", { tls: { }});`,
			},
		},
		{
			name:       "ConnectTlsInvalidTlsParamCertType",
			initString: codeBlock{code: "var client = new grpc.Client();"},
			vuString: codeBlock{
				code: `client.connect("GRPCBIN_ADDR", { tls: { cert: 0 }});`,
				err:  `invalid grpc.connect() parameters: invalid tls cert value: 'map[string]interface {}{"cert":0}', it needs to be a PEM formatted string`,
			},
		},
		{
			name:       "ConnectTlsInvalidTlsParamKeyType",
			initString: codeBlock{code: "var client = new grpc.Client();"},
			vuString: codeBlock{
				code: `client.connect("GRPCBIN_ADDR", { tls: { cert: "", key: 0 }});`,
				err:  `invalid grpc.connect() parameters: invalid tls key value: 'map[string]interface {}{"cert":"", "key":0}', it needs to be a PEM formatted string`,
			},
		},
		{
			name:       "ConnectTlsInvalidTlsParamPasswordType",
			initString: codeBlock{code: "var client = new grpc.Client();"},
			vuString: codeBlock{
				code: `client.connect("GRPCBIN_ADDR", { tls: { cert: "", key: "", password: 0 }});`,
				err:  `invalid grpc.connect() parameters: invalid tls password value: 'map[string]interface {}{"cert":"", "key":"", "password":0}', it needs to be a string`,
			},
		},
		{
			name:       "ConnectTlsInvalidTlsParamCACertsType",
			initString: codeBlock{code: "var client = new grpc.Client();"},
			vuString: codeBlock{
				code: `client.connect("GRPCBIN_ADDR", { tls: { cert: "", key: "", cacerts: 0 }});`,
				err:  `invalid grpc.connect() parameters: invalid tls cacerts value: 'map[string]interface {}{"cacerts":0, "cert":"", "key":""}', it needs to be a string or an array of PEM formatted strings`,
			},
		},
		{
			name: "ConnectTls",
			setup: func(tb *httpmultibin.HTTPMultiBin) {
				clientCAPool := x509.NewCertPool()
				clientCAPool.AppendCertsFromPEM(clientAuthCA)
				tb.ServerHTTP2.TLS.ClientAuth = tls.RequireAndVerifyClientCert
				tb.ServerHTTP2.TLS.ClientCAs = clientCAPool
			},
			initString: codeBlock{code: "var client = new grpc.Client();"},
			vuString:   codeBlock{code: fmt.Sprintf(`client.connect("GRPCBIN_ADDR", { tls: { cacerts: "%s", cert: "%s", key: "%s" }});`, localHostCert, clientAuth, clientAuthKey)},
		},
		{
			name: "ConnectTlsEncryptedKey",
			setup: func(tb *httpmultibin.HTTPMultiBin) {
				clientCAPool := x509.NewCertPool()
				clientCAPool.AppendCertsFromPEM(clientAuthCA)
				tb.ServerHTTP2.TLS.ClientAuth = tls.RequireAndVerifyClientCert
				tb.ServerHTTP2.TLS.ClientCAs = clientCAPool
			},
			initString: codeBlock{code: "var client = new grpc.Client();"},
			vuString:   codeBlock{code: fmt.Sprintf(`client.connect("GRPCBIN_ADDR", { tls: { cacerts: ["%s"], cert: "%s", key: "%s", password: "%s" }});`, localHostCert, clientAuth, clientAuthKeyEncrypted, trivialKeyPassword)},
		},
		{
			name:       "ConnectTlsEncryptedKeyDecryptionFailed",
			initString: codeBlock{code: "var client = new grpc.Client();"},
			vuString: codeBlock{
				code: fmt.Sprintf(`client.connect("GRPCBIN_ADDR", { timeout: '5s', tls: { cert: "%s", key: "%s", password: "abc321" }});`,
					clientAuth,
					clientAuthKeyEncrypted,
				),
				err: "x509: decryption password incorrect",
			},
		},
		{
			name: "ConnectTlsClientCertNoClientAuth",
			setup: func(tb *httpmultibin.HTTPMultiBin) {
				clientCAPool := x509.NewCertPool()
				clientCAPool.AppendCertsFromPEM(clientAuthCA)
				tb.ServerHTTP2.TLS.ClientAuth = tls.RequireAndVerifyClientCert
				tb.ServerHTTP2.TLS.ClientCAs = clientCAPool
			},
			initString: codeBlock{code: `var client = new grpc.Client();`},
			vuString: codeBlock{
				code: fmt.Sprintf(`client.connect("GRPCBIN_ADDR", { tls: { cacerts: ["%s"], cert: "%s", key: "%s" }});`,
					localHostCert,
					clientAuthBad,
					clientAuthKey),
				err: "remote error: tls: bad certificate",
			},
		},
		{
			name: "ConnectTlsClientCertWithPasswordNoClientAuth",
			setup: func(tb *httpmultibin.HTTPMultiBin) {
				clientCAPool := x509.NewCertPool()
				clientCAPool.AppendCertsFromPEM(clientAuthCA)
				tb.ServerHTTP2.TLS.ClientAuth = tls.RequireAndVerifyClientCert
				tb.ServerHTTP2.TLS.ClientCAs = clientCAPool
			},
			initString: codeBlock{code: `var client = new grpc.Client();`},
			vuString: codeBlock{
				code: fmt.Sprintf(`
				client.connect("GRPCBIN_ADDR", { tls: { cacerts: ["%s"], cert: "%s", key: "%s", password: "%s" }});
				`,
					localHostCert,
					clientAuthBad,
					clientAuthKeyEncrypted,
					trivialKeyPassword),
				err: "remote error: tls: bad certificate",
			},
		},
		{
			name: "ConnectTlsInvokeSuccess",
			setup: func(tb *httpmultibin.HTTPMultiBin) {
				clientCAPool := x509.NewCertPool()
				clientCAPool.AppendCertsFromPEM(clientAuthCA)
				tb.ServerHTTP2.TLS.ClientAuth = tls.RequireAndVerifyClientCert
				tb.ServerHTTP2.TLS.ClientCAs = clientCAPool
				tb.GRPCStub.EmptyCallFunc = func(context.Context, *grpc_testing.Empty) (*grpc_testing.Empty, error) {
					return &grpc_testing.Empty{}, nil
				}
			},
			initString: codeBlock{code: `
				var client = new grpc.Client();
				client.load([], "../../../../lib/testutils/httpmultibin/grpc_testing/test.proto");`},
			vuString: codeBlock{
				code: fmt.Sprintf(`
				client.connect("GRPCBIN_ADDR", { timeout: '5s', tls: { cacerts: ["%s"], cert: "%s", key: "%s" }});
				var resp = client.invoke("grpc.testing.TestService/EmptyCall", {})
				if (resp.status !== grpc.StatusOK) {
					throw new Error("unexpected error: " + JSON.stringify(resp.error) + "or status: " + resp.status)
				}`,
					localHostCert,
					clientAuth,
					clientAuthKey),
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ts := newTestState(t)

			// setup necessary environment if needed by a test
			if tt.setup != nil {
				tt.setup(ts.httpBin)
			}

			val, err := ts.Run(tt.initString.code)
			assertResponse(t, tt.initString, err, val, ts)

			ts.ToVUContext()

			val, err = ts.Run(tt.vuString.code)
			assertResponse(t, tt.vuString, err, val, ts)
		})
	}
}

func TestDebugStat(t *testing.T) {
	t.Parallel()

	tests := [...]struct {
		name     string
		stat     grpcstats.RPCStats
		expected string
	}{
		{
			"OutHeader",
			&grpcstats.OutHeader{},
			"Out Header:",
		},
		{
			"OutTrailer",
			&grpcstats.OutTrailer{
				Trailer: metadata.MD{
					"x-trail": []string{"out"},
				},
			},
			"Out Trailer:",
		},
		{
			"OutPayload",
			&grpcstats.OutPayload{
				Payload: &grpc_testing.SimpleRequest{
					FillUsername: true,
				},
			},
			"fill_username:",
		},
		{
			"InHeader",
			&grpcstats.InHeader{
				Header: metadata.MD{
					"x-head": []string{"in"},
				},
			},
			"x-head: in",
		},
		{
			"InTrailer",
			&grpcstats.InTrailer{
				Trailer: metadata.MD{
					"x-trail": []string{"in"},
				},
			},
			"x-trail: in",
		},
		{
			"InPayload",
			&grpcstats.InPayload{
				Payload: &grpc_testing.SimpleResponse{
					Username: "k6-user",
				},
			},
			"username:",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var b bytes.Buffer
			logger := logrus.New()
			logger.Out = &b

			grpcext.DebugStat(logger.WithField("source", "test"), tt.stat, "full")
			assert.Contains(t, b.String(), tt.expected)
		})
	}
}

func TestClientLoadProto(t *testing.T) {
	t.Parallel()

	ts := newTestState(t)

	code := `
		var client = new grpc.Client();
		client.load([], "../../../../lib/testutils/httpmultibin/grpc_testing/nested_types.proto");`

	_, err := ts.VU.Runtime().RunString(ts.httpBin.Replacer.Replace(code))
	assert.Nil(t, err, "It was not expected that there would be an error, but it got: %v", err)

	expectedTypes := []string{
		"grpc.testing.Outer",
		"grpc.testing.Outer.MiddleAA",
		"grpc.testing.Outer.MiddleAA.Inner",
		"grpc.testing.Outer.MiddleBB",
		"grpc.testing.Outer.MiddleBB.Inner",
		"grpc.testing.MeldOuter",
	}

	for _, expected := range expectedTypes {
		found, err := protoregistry.GlobalTypes.FindMessageByName(protoreflect.FullName(expected))

		assert.NotNil(t, found, "Expected to find the message type %s, but an error occurred", expected)
		assert.Nil(t, err, "It was not expected that there would be an error, but it got: %v", err)
	}
}

func TestClientConnectionReflectMetadata(t *testing.T) {
	t.Parallel()

	ts := newTestState(t)

	reflection.Register(ts.httpBin.ServerGRPC)

	initString := codeBlock{
		code: `var client = new grpc.Client();`,
	}
	vuString := codeBlock{
		code: `client.connect("GRPCBIN_ADDR", {reflect: true, reflectMetadata: {"x-test": "custom-header-for-reflection"}})`,
	}

	val, err := ts.Run(initString.code)
	assertResponse(t, initString, err, val, ts)

	ts.ToVUContext()

	// this should trigger logging of the outgoing gRPC metadata
	ts.VU.State().Options.HTTPDebug = null.NewString("full", true)

	val, err = ts.Run(vuString.code)
	assertResponse(t, vuString, err, val, ts)

	entries := ts.loggerHook.Drain()

	// since we enable debug logging, we should see the metadata in the logs
	foundReflectionCall := false
	for _, entry := range entries {
		if strings.Contains(entry.Message, "ServerReflection/ServerReflectionInfo") {
			foundReflectionCall = true

			// check that the metadata is present
			assert.Contains(t, entry.Message, "x-test: custom-header-for-reflection")
			// check that user-agent header is present
			assert.Contains(t, entry.Message, "user-agent: k6-test")
		}
	}

	assert.True(t, foundReflectionCall, "expected to find a reflection call in the logs, but didn't")
}
