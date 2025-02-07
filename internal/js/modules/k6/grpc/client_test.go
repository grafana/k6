package grpc_test

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"
	"testing"

	k6grpc "go.k6.io/k6/internal/js/modules/k6/grpc"
	"go.k6.io/k6/internal/lib/netext/grpcext"
	"go.k6.io/k6/internal/lib/testutils/httpmultibin"
	grpcanytesting "go.k6.io/k6/internal/lib/testutils/httpmultibin/grpc_any_testing"
	"go.k6.io/k6/internal/lib/testutils/httpmultibin/grpc_testing"
	"go.k6.io/k6/internal/lib/testutils/httpmultibin/grpc_wrappers_testing"
	"go.k6.io/k6/metrics"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/golang/protobuf/ptypes/any"
	_struct "github.com/golang/protobuf/ptypes/struct"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/metadata"
	v1alphagrpc "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	grpcstats "google.golang.org/grpc/stats"
	"gopkg.in/guregu/null.v3"
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
			name: "AsyncInvokeInvalidParam",
			initString: codeBlock{code: `
				var client = new grpc.Client();
				client.load([], "../../../../lib/testutils/httpmultibin/grpc_testing/test.proto");`},
			vuString: codeBlock{
				code: `
				client.connect("GRPCBIN_ADDR");
				client.asyncInvoke("grpc.testing.TestService/EmptyCall", {}, { void: true }).then(function(resp) {
					throw new Error("should not be here")
				}, (err) => {
					throw new Error(err)
				})`,
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
			name: "InvokeDiscardResponseMessage",
			initString: codeBlock{
				code: `
				var client = new grpc.Client();
				client.load([], "../../../../lib/testutils/httpmultibin/grpc_testing/test.proto");`,
			},
			vuString: codeBlock{
				code: `
				client.connect("GRPCBIN_ADDR");
				client.invoke("grpc.testing.TestService/EmptyCall", {}, { discardResponseMessage: true })`,
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
			name: "InvokeDiscardResponseMessage",
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
				var resp = client.invoke("grpc.testing.TestService/EmptyCall", {}, { discardResponseMessage: true })
				if (resp.status !== grpc.StatusOK) {
					throw new Error("unexpected error: " + JSON.stringify(resp.error) + "or status: " + resp.status)
				}
				if (resp.message !== null) {
					throw new Error("unexpected message: " + JSON.stringify(resp.message))
				}`,
				asserts: func(t *testing.T, rb *httpmultibin.HTTPMultiBin, samples chan metrics.SampleContainer, _ error) {
					samplesBuf := metrics.GetBufferedSamples(samples)
					assertMetricEmitted(t, metrics.GRPCReqDurationName, samplesBuf, rb.Replacer.Replace("GRPCBIN_ADDR/grpc.testing.TestService/EmptyCall"))
				},
			},
		},
		{
			name: "AsyncInvoke",
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
				client.asyncInvoke("grpc.testing.TestService/EmptyCall", {}).then(function(resp) {
					if (resp.status !== grpc.StatusOK) {
						throw new Error("unexpected error: " + JSON.stringify(resp.error) + "or status: " + resp.status)
					}
				}, (err) => {
					throw new Error("unexpected error: " + err)
				})
				`,
				asserts: func(t *testing.T, rb *httpmultibin.HTTPMultiBin, samples chan metrics.SampleContainer, _ error) {
					samplesBuf := metrics.GetBufferedSamples(samples)
					assertMetricEmitted(t, metrics.GRPCReqDurationName, samplesBuf, rb.Replacer.Replace("GRPCBIN_ADDR/grpc.testing.TestService/EmptyCall"))
				},
			},
		},
		{
			name: "AsyncInvokeDiscardResponseMessage",
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
				client.asyncInvoke("grpc.testing.TestService/EmptyCall", {}, { discardResponseMessage: true }).then(function(resp) {
					if (resp.status !== grpc.StatusOK) {
						throw new Error("unexpected error: " + JSON.stringify(resp.error) + "or status: " + resp.status)
					}
					if (resp.message !== null) {
						throw new Error("unexpected message: " + JSON.stringify(resp.message))
					}
				}, (err) => {
					throw new Error("unexpected error: " + err)
				})
				`,
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
				tb.GRPCAnyStub.SumFunc = func(_ context.Context, req *grpcanytesting.SumRequest) (*grpcanytesting.SumReply, error) {
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
			name: "AsyncRequestMessage",
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
				client.asyncInvoke("grpc.testing.TestService/UnaryCall", { payload: { body: "6LSf6L295rWL6K+V"} }).then(function(resp) {
					if (resp.status !== grpc.StatusOK) {
						throw new Error("server did not receive the correct request message")
					}
				}, (err) => {
					throw new Error("unexpected error: " + err)
				});
				`},
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
			name: "AsyncResponseMessage",
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
				client.asyncInvoke("grpc.testing.TestService/UnaryCall", {}).then(function(resp) {
					if (!resp.message || resp.message.username !== "" || resp.message.oauthScope !== "水") {
						throw new Error("unexpected response message: " + JSON.stringify(resp.message))
					}
				}, (err) => {
					throw new Error("unexpected error: " + err)
				});
				`,
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
				tb.GRPCStub.EmptyCallFunc = func(_ context.Context, _ *grpc_testing.Empty) (*grpc_testing.Empty, error) {
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
				// this register both reflection APIs v1 and v1alpha
				reflection.Register(tb.ServerGRPC)

				tb.GRPCStub.EmptyCallFunc = func(_ context.Context, _ *grpc_testing.Empty) (*grpc_testing.Empty, error) {
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
			name: "ReflectV1Alpha_Invoke",
			setup: func(tb *httpmultibin.HTTPMultiBin) {
				// this register only v1alpha (this could be removed with removal v1alpha from grpc-go)
				s := tb.ServerGRPC
				svr := reflection.NewServer(reflection.ServerOptions{Services: s})
				v1alphagrpc.RegisterServerReflectionServer(s, svr)

				tb.GRPCStub.EmptyCallFunc = func(_ context.Context, _ *grpc_testing.Empty) (*grpc_testing.Empty, error) {
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
			name: "ReflectV1Invoke",
			setup: func(tb *httpmultibin.HTTPMultiBin) {
				// this register only reflection APIs v1
				reflection.RegisterV1(tb.ServerGRPC)

				tb.GRPCStub.EmptyCallFunc = func(_ context.Context, _ *grpc_testing.Empty) (*grpc_testing.Empty, error) {
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
		{
			name: "ValueReflection",
			setup: func(hb *httpmultibin.HTTPMultiBin) {
				reflection.Register(hb.ServerGRPC)

				srv := grpc_wrappers_testing.Register(hb.ServerGRPC)

				srv.TestValueImplementation = func(_ context.Context, in *_struct.Value) (*_struct.Value, error) {
					if in.GetNumberValue() == 12 {
						return &_struct.Value{
							Kind: &_struct.Value_NumberValue{
								NumberValue: 42,
							},
						}, nil
					}

					if in.GetStringValue() != "" {
						return &_struct.Value{
							Kind: &_struct.Value_StringValue{
								StringValue: "hey " + in.GetStringValue(),
							},
						}, nil
					}

					return &_struct.Value{
						Kind: &_struct.Value_StringValue{
							StringValue: "I don't know what to answer",
						},
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

				let respString = client.invoke("grpc.wrappers.testing.Service/TestValue", "John")
				if (respString.message !== "hey John") {
					throw new Error("expected to get 'hey John', but got a " + respString.message)
				}

				let respNumber = client.invoke("grpc.wrappers.testing.Service/TestValue", 12)
				if (respNumber.message !== 42) {
					throw new Error("expected to get '42', but got a " + respNumber.message)
				}

				let respBool = client.invoke("grpc.wrappers.testing.Service/TestValue", false)
				if (respBool.message !== "I don't know what to answer") {
					throw new Error("expected to get 'I don't know what to answer', but got a " + respBool.message)
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
			val, err = ts.RunOnEventLoop(tt.vuString.code)
			assertResponse(t, tt.vuString, err, val, ts)
		})
	}
}

func TestClient_TlsParameters(t *testing.T) {
	t.Parallel()

	clientAuthCA := []byte("-----BEGIN CERTIFICATE-----\nMIIBWzCCAQGgAwIBAgIJAIQMBgLi+DV6MAoGCCqGSM49BAMCMBAxDjAMBgNVBAMM\nBU15IENBMCAXDTIyMDEyMTEyMjkzNloYDzMwMjEwNTI0MTIyOTM2WjAQMQ4wDAYD\nVQQDDAVNeSBDQTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABHnrghULHa2hSa/C\nWimwCn42KWdlPqd6/zs3JgLIxTvBHJJlfbhWbBqtybqyovWd3QykHMIpx0NZmpYn\nG8FoWpmjQjBAMA4GA1UdDwEB/wQEAwIBBjAPBgNVHRMBAf8EBTADAQH/MB0GA1Ud\nDgQWBBSkukBA8lgFvvBJAYKsoSUR+PX71jAKBggqhkjOPQQDAgNIADBFAiEAiFF7\nY54CMNRSBSVMgd4mQgrzJInRH88KpLsQ7VeOAaQCIEa0vaLln9zxIDZQKocml4Db\nAEJr8tDzMKIds6sRTBT4\n-----END CERTIFICATE-----")
	clientAuth := "-----BEGIN CERTIFICATE-----\\nMIIBVzCB/6ADAgECAgkAg/SeNG3XqB0wCgYIKoZIzj0EAwIwEDEOMAwGA1UEAwwF\\nTXkgQ0EwIBcNMjIwMTIxMTUxMjM0WhgPMzAyMTA1MjQxNTEyMzRaMBExDzANBgNV\\nBAMMBmNsaWVudDBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABKM7OJQMYG4KLtDA\\ngZ8zOg2PimHMmQnjD2HtI4cSwIUJJnvHWLowbFe9fk6XeP9b3dK1ImUI++/EZdVr\\nABAcngejPzA9MA4GA1UdDwEB/wQEAwIBBjAMBgNVHRMBAf8EAjAAMB0GA1UdDgQW\\nBBSttJe1mcPEnBOZ6wvKPG4zL0m1CzAKBggqhkjOPQQDAgNHADBEAiBPSLgKA/r9\\nu/FW6W+oy6Odm1kdNMGCI472iTn545GwJgIgb3UQPOUTOj0IN4JLJYfmYyXviqsy\\nzk9eWNHFXDA9U6U=\\n-----END CERTIFICATE-----"
	clientAuthKey := ("-----BEGIN EC PRIVATE KEY-----\\nMHcCAQEEINDaMGkOT3thu1A0LfLJr3Jd011/aEG6OArmEQaujwgpoAoGCCqGSM49\\nAwEHoUQDQgAEozs4lAxgbgou0MCBnzM6DY+KYcyZCeMPYe0jhxLAhQkme8dYujBs\\nV71+Tpd4/1vd0rUiZQj778Rl1WsAEByeBw==\\n-----END EC PRIVATE KEY-----")
	clientAuthKeyEncrypted := ("-----BEGIN EC PRIVATE KEY-----\\nProc-Type: 4,ENCRYPTED\\nDEK-Info: AES-256-CBC,3E311E9B602231BFB5C752071EE7D652\\n\\nsAKeqbacug0v4ruE1A0CACwGVEGBQVOl1CiGVp5RsxgNZKXzMS6EsTTNLw378coF\\nKXbF+he05HIuzToOz2ANLXov1iCrVpotKVB4l2obTQvg+5VET902ky99Mc9Us7jd\\nUwW8LpXlSlhcNWuUfK6wyosL42TbcIxjqZWaESW+6ww=\\n-----END EC PRIVATE KEY-----")

	trivialKeyPassword := "abc123"
	trivialWrongKeyPassword := "abc321"

	// We need the certificate actually used by the httpmultibin in this test
	// as it is the same one for all tests we can get it once here
	// This also prevents changes to this to break this test
	// as it happened after https://github.com/golang/go/commit/6783377295e0878aa3ad821eefe3d7879064df6d
	p := httpmultibin.NewHTTPMultiBin(t)
	localHostCert := strings.ReplaceAll(string(pem.EncodeToMemory(
		&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: p.ServerHTTP2.TLS.Certificates[0].Certificate[0],
		})), "\n", `\n`)

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
			vuString:   codeBlock{code: fmt.Sprintf(`client.connect("GRPCBIN_ADDR", { tls: { cacerts: ["%s"], cert: "%s", key: "%s" }});`, localHostCert, clientAuth, clientAuthKey)},
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
				code: fmt.Sprintf(`client.connect("GRPCBIN_ADDR", { timeout: '5s', tls: { cert: "%s", key: "%s", password: "%s" }});`,
					clientAuth,
					clientAuthKeyEncrypted,
					trivialWrongKeyPassword,
				),
				err: "x509: decryption password incorrect",
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

	tt := testcase{
		initString: codeBlock{
			code: `
			var client = new grpc.Client();
			client.load([], "../../../../lib/testutils/httpmultibin/nested_types/nested_types.proto");`,
		},
	}

	val, err := ts.Run(tt.initString.code)
	assertResponse(t, tt.initString, err, val, ts)

	expectedTypes := []string{
		"grpc.testdata.nested.types.Outer",
		"grpc.testdata.nested.types.Outer.MiddleAA",
		"grpc.testdata.nested.types.Outer.MiddleAA.Inner",
		"grpc.testdata.nested.types.Outer.MiddleBB",
		"grpc.testdata.nested.types.Outer.MiddleBB.Inner",
		"grpc.testdata.nested.types.MeldOuter",
	}

	for _, expected := range expectedTypes {
		found, err := protoregistry.GlobalTypes.FindMessageByName(protoreflect.FullName(expected))

		assert.NotNil(t, found, "Expected to find the message type %s, but an error occurred", expected)
		assert.Nil(t, err, "It was not expected that there would be an error, but it got: %v", err)
	}
}

func TestClientLoadProtoAbsoluteRootWithFile(t *testing.T) {
	t.Parallel()

	ts := newTestState(t)
	rootPath := ts.VU.InitEnvField.CWD.JoinPath("../..").String()

	tt := testcase{
		initString: codeBlock{
			code: `
			var client = new grpc.Client();
			client.load(["` + rootPath + `"], "../../lib/testutils/httpmultibin/nested_types/nested_types.proto");`,
		},
	}

	val, err := ts.Run(tt.initString.code)
	assertResponse(t, tt.initString, err, val, ts)

	expectedTypes := []string{
		"grpc.testdata.nested.types.Outer",
		"grpc.testdata.nested.types.Outer.MiddleAA",
		"grpc.testdata.nested.types.Outer.MiddleAA.Inner",
		"grpc.testdata.nested.types.Outer.MiddleBB",
		"grpc.testdata.nested.types.Outer.MiddleBB.Inner",
		"grpc.testdata.nested.types.MeldOuter",
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
