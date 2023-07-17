package grpc

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"runtime"
	"strings"
	"testing"

	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	"github.com/dop251/goja"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	grpcstats "google.golang.org/grpc/stats"
	"google.golang.org/grpc/status"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/js/modulestest"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/fsext"
	"go.k6.io/k6/lib/netext/grpcext"
	"go.k6.io/k6/lib/testutils"
	"go.k6.io/k6/lib/testutils/httpmultibin"
	grpcanytesting "go.k6.io/k6/lib/testutils/httpmultibin/grpc_any_testing"
	"go.k6.io/k6/lib/testutils/httpmultibin/grpc_testing"
	"go.k6.io/k6/metrics"
)

const isWindows = runtime.GOOS == "windows"

// codeBlock represents an execution of a k6 script.
type codeBlock struct {
	code       string
	val        interface{}
	err        string
	windowsErr string
	asserts    func(*testing.T, *httpmultibin.HTTPMultiBin, chan metrics.SampleContainer, error)
}

type testcase struct {
	name       string
	setup      func(*httpmultibin.HTTPMultiBin)
	initString codeBlock // runs in the init context
	vuString   codeBlock // runs in the vu context
}

func TestClient(t *testing.T) {
	t.Parallel()

	type testState struct {
		*modulestest.Runtime
		httpBin *httpmultibin.HTTPMultiBin
		samples chan metrics.SampleContainer
	}
	setup := func(t *testing.T) testState {
		t.Helper()

		tb := httpmultibin.NewHTTPMultiBin(t)
		samples := make(chan metrics.SampleContainer, 1000)
		testRuntime := modulestest.NewRuntime(t)

		cwd, err := os.Getwd() //nolint:golint,forbidigo
		require.NoError(t, err)
		fs := fsext.NewOsFs()
		if isWindows {
			fs = fsext.NewTrimFilePathSeparatorFs(fs)
		}
		testRuntime.VU.InitEnvField.CWD = &url.URL{Path: cwd}
		testRuntime.VU.InitEnvField.FileSystems = map[string]fsext.Fs{"file": fs}

		return testState{
			Runtime: testRuntime,
			httpBin: tb,
			samples: samples,
		}
	}

	assertMetricEmitted := func(
		t *testing.T,
		metricName string,
		sampleContainers []metrics.SampleContainer,
		url string,
	) {
		seenMetric := false

		for _, sampleContainer := range sampleContainers {
			for _, sample := range sampleContainer.GetSamples() {
				surl, ok := sample.Tags.Get("url")
				assert.True(t, ok)
				if surl == url {
					if sample.Metric.Name == metricName {
						seenMetric = true
					}
				}
			}
		}
		assert.True(t, seenMetric, "url %s didn't emit %s", url, metricName)
	}

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
				val: []MethodInfo{{MethodInfo: grpc.MethodInfo{Name: "EmptyCall", IsClientStream: false, IsServerStream: false}, Package: "grpc.testing", Service: "TestService", FullMethod: "/grpc.testing.TestService/EmptyCall"}, {MethodInfo: grpc.MethodInfo{Name: "UnaryCall", IsClientStream: false, IsServerStream: false}, Package: "grpc.testing", Service: "TestService", FullMethod: "/grpc.testing.TestService/UnaryCall"}, {MethodInfo: grpc.MethodInfo{Name: "StreamingOutputCall", IsClientStream: false, IsServerStream: true}, Package: "grpc.testing", Service: "TestService", FullMethod: "/grpc.testing.TestService/StreamingOutputCall"}, {MethodInfo: grpc.MethodInfo{Name: "StreamingInputCall", IsClientStream: true, IsServerStream: false}, Package: "grpc.testing", Service: "TestService", FullMethod: "/grpc.testing.TestService/StreamingInputCall"}, {MethodInfo: grpc.MethodInfo{Name: "FullDuplexCall", IsClientStream: true, IsServerStream: true}, Package: "grpc.testing", Service: "TestService", FullMethod: "/grpc.testing.TestService/FullDuplexCall"}, {MethodInfo: grpc.MethodInfo{Name: "HalfDuplexCall", IsClientStream: true, IsServerStream: true}, Package: "grpc.testing", Service: "TestService", FullMethod: "/grpc.testing.TestService/HalfDuplexCall"}},
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
				val: []MethodInfo{
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
	}

	assertResponse := func(t *testing.T, cb codeBlock, err error, val goja.Value, ts testState) {
		if isWindows && cb.windowsErr != "" && err != nil {
			err = errors.New(strings.ReplaceAll(err.Error(), cb.windowsErr, cb.err))
		}
		if cb.err == "" {
			assert.NoError(t, err)
		} else {
			require.Error(t, err)
			assert.Contains(t, err.Error(), cb.err)
		}
		if cb.val != nil {
			require.NotNil(t, val)
			assert.Equal(t, cb.val, val.Export())
		}
		if cb.asserts != nil {
			cb.asserts(t, ts.httpBin, ts.samples, err)
		}
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ts := setup(t)

			m, ok := New().NewModuleInstance(ts.VU).(*ModuleInstance)
			require.True(t, ok)
			require.NoError(t, ts.VU.Runtime().Set("grpc", m.Exports().Named))

			// setup necessary environment if needed by a test
			if tt.setup != nil {
				tt.setup(ts.httpBin)
			}

			replace := func(code string) (goja.Value, error) {
				return ts.VU.Runtime().RunString(ts.httpBin.Replacer.Replace(code))
			}

			val, err := replace(tt.initString.code)
			assertResponse(t, tt.initString, err, val, ts)

			registry := metrics.NewRegistry()
			root, err := lib.NewGroup("", nil)
			require.NoError(t, err)

			state := &lib.State{
				Group:     root,
				Dialer:    ts.httpBin.Dialer,
				TLSConfig: ts.httpBin.TLSClientConfig,
				Samples:   ts.samples,
				Options: lib.Options{
					SystemTags: metrics.NewSystemTagSet(
						metrics.TagName,
						metrics.TagURL,
					),
					UserAgent: null.StringFrom("k6-test"),
				},
				BuiltinMetrics: metrics.RegisterBuiltinMetrics(registry),
				Tags:           lib.NewVUStateTags(registry.RootTagSet()),
			}
			ts.MoveToVUContext(state)
			val, err = replace(tt.vuString.code)
			assertResponse(t, tt.vuString, err, val, ts)
		})
	}
}

func TestClient_ConnectionSharingParameter(t *testing.T) {
	t.Parallel()
	type testState struct {
		*modulestest.Runtime
		httpBin *httpmultibin.HTTPMultiBin
		samples chan metrics.SampleContainer
	}

	setup := func(t *testing.T) testState {
		t.Helper()

		tb := httpmultibin.NewHTTPMultiBin(t)
		samples := make(chan metrics.SampleContainer, 1000)
		testRuntime := modulestest.NewRuntime(t)

		cwd, err := os.Getwd() //nolint:golint,forbidigo
		require.NoError(t, err)
		fs := fsext.NewOsFs()
		if isWindows {
			fs = fsext.NewTrimFilePathSeparatorFs(fs)
		}
		testRuntime.VU.InitEnvField.CWD = &url.URL{Path: cwd}
		testRuntime.VU.InitEnvField.FileSystems = map[string]fsext.Fs{"file": fs}

		return testState{
			Runtime: testRuntime,
			httpBin: tb,
			samples: samples,
		}
	}

	tests := []testcase{
		{
			name: "ConnectConnectionSharingDefault",
			initString: codeBlock{code: `
				var client0 = new grpc.Client();
				var client1 = new grpc.Client();`},
			vuString: codeBlock{
				code: `
					client0.connect("GRPCBIN_ADDR");
					client1.connect("GRPCBIN_ADDR");
					if (client0.isSameConnection(client1)) {
						throw new Error("unexpected connection equality");
					}`,
			},
		},
		{
			name: "ConnectConnectionSharingFalseFalse",
			initString: codeBlock{code: `
				var client0 = new grpc.Client();
				var client1 = new grpc.Client();`},
			vuString: codeBlock{
				code: `
					client0.connect("GRPCBIN_ADDR", { connectionSharing: false});
					client1.connect("GRPCBIN_ADDR", { connectionSharing: false});
					if (client0.isSameConnection(client1)) {
						throw new Error("unexpected connection equality");
					}`,
			},
		},
		{
			name: "ConnectConnectionSharingFalseTrue",
			initString: codeBlock{code: `
				var client0 = new grpc.Client();
				var client1 = new grpc.Client();`},
			vuString: codeBlock{
				code: `
					client0.connect("GRPCBIN_ADDR", { connectionSharing: true});
					client1.connect("GRPCBIN_ADDR", { connectionSharing: false});
					if (client0.isSameConnection(client1)) {
						throw new Error("unexpected connection equality");
					}`,
			},
		},
		{
			name: "ConnectConnectionSharingTrueTrue",
			initString: codeBlock{code: `
				var client0 = new grpc.Client();
				var client1 = new grpc.Client();`},
			vuString: codeBlock{
				code: `
					client0.connect("GRPCBIN_ADDR", { connectionSharing: true});
					client1.connect("GRPCBIN_ADDR", { connectionSharing: true});
					if (!client0.isSameConnection(client1)) {
						throw new Error("unexpected connection inequality");
					}`,
			},
		},
		{
			name: "ConnectConnectionSharingTrueFalseTrue",
			initString: codeBlock{code: `
				var client0 = new grpc.Client();
				var client1 = new grpc.Client();
				var client2 = new grpc.Client();`},
			vuString: codeBlock{
				code: `
					client0.connect("GRPCBIN_ADDR", { connectionSharing: true});
					client1.connect("GRPCBIN_ADDR", { connectionSharing: false});
					client2.connect("GRPCBIN_ADDR", { connectionSharing: true});
					if (client0.isSameConnection(client1)) {
						throw new Error("unexpected connection equality");
					}
					if (!client0.isSameConnection(client2)) {
						throw new Error("unexpected connection inequality");
					}`,
			},
		},
		{
			name: "ConnectConnectionSharingMax2Default",
			initString: codeBlock{code: `
				var client0 = new grpc.Client();
				var client1 = new grpc.Client();
				var client2 = new grpc.Client();
				const maxSharing = 2;
				`},
			vuString: codeBlock{
				code: `
					client0.connect("GRPCBIN_ADDR", { connectionSharing: maxSharing });
					client1.connect("GRPCBIN_ADDR", { connectionSharing: maxSharing });
					client2.connect("GRPCBIN_ADDR", { connectionSharing: maxSharing });
					if (!client0.isSameConnection(client1)) {
						throw new Error("unexpected connection inequality");
					}
					if (client0.isSameConnection(client2)) {
						throw new Error("unexpected connection equality");
					}`,
			},
		},
		{
			name: "ConnectConnectionSharingValue1",
			initString: codeBlock{code: `
				var client0 = new grpc.Client();
				const maxSharing = 1;
				`},
			vuString: codeBlock{
				code: `client0.connect("GRPCBIN_ADDR", { connectionSharing: maxSharing });`,
				err:  "it needs to be boolean or a positive integer > 1",
			},
		},
		{
			// First 100 connections (0 - 99) will be shared, last connection: [100] will be a new connection
			// Only 2 connections total will be opened
			name: "ConnectConnectionSharingValue100",
			initString: codeBlock{code: `
				const clients = [];
                for(var i = 0; i < 101; i++) {
					clients.push(new grpc.Client());
				}`},
			vuString: codeBlock{
				code: `
                for(let i = 0; i < 101; i++) {
					clients[i].connect("GRPCBIN_ADDR", { connectionSharing: true });
				}
				for(var i = 1; i < 100; i++) {
					if (!clients[i].isSameConnection(clients[0])) {
						throw new Error("unexpected connection inequality:" + i);
					}
				}
				if (clients[100].isSameConnection(clients[0])) {
					throw new Error("unexpected connection equality");
				}`,
			},
		},
		{
			name: "ConnectConnectionSharingValueBad",
			initString: codeBlock{code: `
				var client0 = new grpc.Client();
				const maxSharing = "on";
				`},
			vuString: codeBlock{
				code: `client0.connect("GRPCBIN_ADDR", { connectionSharing: maxSharing });`,
				err:  "it needs to be boolean or a positive integer > 1",
			},
		},
	}
	assertResponse := func(t *testing.T, cb codeBlock, err error, val goja.Value, ts testState) {
		if isWindows && cb.windowsErr != "" && err != nil {
			err = errors.New(strings.ReplaceAll(err.Error(), cb.windowsErr, cb.err))
		}
		if cb.err == "" {
			assert.NoError(t, err)
		} else {
			require.Error(t, err)
			assert.Contains(t, err.Error(), cb.err)
		}
		if cb.val != nil {
			require.NotNil(t, val)
			assert.Equal(t, cb.val, val.Export())
		}
		if cb.asserts != nil {
			cb.asserts(t, ts.httpBin, ts.samples, err)
		}
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ts := setup(t)

			m, ok := New().NewModuleInstance(ts.VU).(*ModuleInstance)
			require.True(t, ok)
			require.NoError(t, ts.VU.Runtime().Set("grpc", m.Exports().Named))

			// setup necessary environment if needed by a test
			if tt.setup != nil {
				tt.setup(ts.httpBin)
			}

			replace := func(code string) (goja.Value, error) {
				return ts.VU.Runtime().RunString(ts.httpBin.Replacer.Replace(code))
			}

			val, err := replace(tt.initString.code)
			assertResponse(t, tt.initString, err, val, ts)

			registry := metrics.NewRegistry()
			root, err := lib.NewGroup("", nil)
			require.NoError(t, err)

			state := &lib.State{
				Group:     root,
				Dialer:    ts.httpBin.Dialer,
				TLSConfig: ts.httpBin.TLSClientConfig,
				Samples:   ts.samples,
				Options: lib.Options{
					SystemTags: metrics.NewSystemTagSet(
						metrics.TagName,
						metrics.TagURL,
					),
					UserAgent: null.StringFrom("k6-test"),
				},
				BuiltinMetrics: metrics.RegisterBuiltinMetrics(registry),
				Tags:           lib.NewVUStateTags(registry.RootTagSet()),
			}
			ts.MoveToVUContext(state)
			val, err = replace(tt.vuString.code)
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

	type testState struct {
		*modulestest.Runtime
		httpBin *httpmultibin.HTTPMultiBin
		samples chan metrics.SampleContainer
	}

	setup := func(t *testing.T) testState {
		t.Helper()

		tb := httpmultibin.NewHTTPMultiBin(t)
		samples := make(chan metrics.SampleContainer, 1000)
		testRuntime := modulestest.NewRuntime(t)

		cwd, err := os.Getwd() //nolint:golint,forbidigo
		require.NoError(t, err)
		fs := fsext.NewOsFs()
		if isWindows {
			fs = fsext.NewTrimFilePathSeparatorFs(fs)
		}
		testRuntime.VU.InitEnvField.CWD = &url.URL{Path: cwd}
		testRuntime.VU.InitEnvField.FileSystems = map[string]fsext.Fs{"file": fs}

		return testState{
			Runtime: testRuntime,
			httpBin: tb,
			samples: samples,
		}
	}

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
	assertResponse := func(t *testing.T, cb codeBlock, err error, val goja.Value, ts testState) {
		if isWindows && cb.windowsErr != "" && err != nil {
			err = errors.New(strings.ReplaceAll(err.Error(), cb.windowsErr, cb.err))
		}
		if cb.err == "" {
			assert.NoError(t, err)
		} else {
			require.Error(t, err)
			assert.Contains(t, err.Error(), cb.err)
		}
		if cb.val != nil {
			require.NotNil(t, val)
			assert.Equal(t, cb.val, val.Export())
		}
		if cb.asserts != nil {
			cb.asserts(t, ts.httpBin, ts.samples, err)
		}
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ts := setup(t)

			m, ok := New().NewModuleInstance(ts.VU).(*ModuleInstance)
			require.True(t, ok)
			require.NoError(t, ts.VU.Runtime().Set("grpc", m.Exports().Named))

			// setup necessary environment if needed by a test
			if tt.setup != nil {
				tt.setup(ts.httpBin)
			}

			replace := func(code string) (goja.Value, error) {
				return ts.VU.Runtime().RunString(ts.httpBin.Replacer.Replace(code))
			}

			val, err := replace(tt.initString.code)
			assertResponse(t, tt.initString, err, val, ts)

			registry := metrics.NewRegistry()
			root, err := lib.NewGroup("", nil)
			require.NoError(t, err)

			state := &lib.State{
				Group:     root,
				Dialer:    ts.httpBin.Dialer,
				TLSConfig: ts.httpBin.TLSClientConfig,
				Samples:   ts.samples,
				Options: lib.Options{
					SystemTags: metrics.NewSystemTagSet(
						metrics.TagName,
						metrics.TagURL,
					),
					UserAgent: null.StringFrom("k6-test"),
				},
				BuiltinMetrics: metrics.RegisterBuiltinMetrics(registry),
				Tags:           lib.NewVUStateTags(registry.RootTagSet()),
			}
			ts.MoveToVUContext(state)
			val, err = replace(tt.vuString.code)
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

func TestClientInvokeHeadersDeprecated(t *testing.T) {
	t.Parallel()

	registry := metrics.NewRegistry()

	logHook := testutils.NewLogHook(logrus.WarnLevel)
	testLog := logrus.New()
	testLog.AddHook(logHook)
	testLog.SetOutput(io.Discard)

	rt := goja.New()
	c := Client{
		vu: &modulestest.VU{
			StateField: &lib.State{
				BuiltinMetrics: metrics.RegisterBuiltinMetrics(registry),
				Logger:         testLog,
				Tags:           lib.NewVUStateTags(registry.RootTagSet()),
			},
			RuntimeField: rt,
		},
	}

	params := rt.ToValue(map[string]interface{}{
		"headers": map[string]interface{}{
			"X-HEADER-FOO": "bar",
		},
	})
	_, err := c.parseInvokeParams(params)
	require.NoError(t, err)

	entries := logHook.Drain()
	require.Len(t, entries, 1)
	require.Contains(t, entries[0].Message, "headers property is deprecated")
}

func TestClientLoadProto(t *testing.T) {
	t.Parallel()

	type testState struct {
		*modulestest.Runtime
		httpBin *httpmultibin.HTTPMultiBin
		samples chan metrics.SampleContainer
	}
	setup := func(t *testing.T) testState {
		t.Helper()

		tb := httpmultibin.NewHTTPMultiBin(t)
		samples := make(chan metrics.SampleContainer, 1000)
		testRuntime := modulestest.NewRuntime(t)

		cwd, err := os.Getwd() //nolint:forbidigo
		require.NoError(t, err)
		fs := fsext.NewOsFs()
		if isWindows {
			fs = fsext.NewTrimFilePathSeparatorFs(fs)
		}
		testRuntime.VU.InitEnvField.CWD = &url.URL{Path: cwd}
		testRuntime.VU.InitEnvField.FileSystems = map[string]fsext.Fs{"file": fs}

		return testState{
			Runtime: testRuntime,
			httpBin: tb,
			samples: samples,
		}
	}

	ts := setup(t)

	m, ok := New().NewModuleInstance(ts.VU).(*ModuleInstance)
	require.True(t, ok)
	require.NoError(t, ts.VU.Runtime().Set("grpc", m.Exports().Named))

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
