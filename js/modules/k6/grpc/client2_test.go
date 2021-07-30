package grpc

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/grpc_testing"
	"net/url"
	"os"
	"testing"

	"github.com/dop251/goja"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/fsext"
	"go.k6.io/k6/lib/metrics"
	"go.k6.io/k6/lib/testutils/httpmultibin"
	"go.k6.io/k6/stats"
)

func TestClient2(t *testing.T) {
	type test struct {
		name        string
		context     context.Context
		vuctx       bool
		initString  string
		setupserver func(*httpmultibin.HTTPMultiBin)
		vuString    string
		error       string
		val         interface{}
		asserts     func(*testing.T, *httpmultibin.HTTPMultiBin, chan stats.SampleContainer)
	}
	tests := []test{
		{
			name: "New",
			vuString: `
			var client = new grpc.Client();
			if (!client) throw new Error("no client created")`,
		},
		{
			name: "LoadNotFound",
			vuString: `
			var client = new grpc.Client();
			client.load([], "./does_not_exist.proto");`,
			error: "no such file or directory", // Fix the windows thing here
		},
		{
			name: "Load",
			vuString: `
			var client = new grpc.Client();
			client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`,
			val: []MethodInfo{{MethodInfo: grpc.MethodInfo{Name: "EmptyCall", IsClientStream: false, IsServerStream: false}, Package: "grpc.testing", Service: "TestService", FullMethod: "/grpc.testing.TestService/EmptyCall"}, {MethodInfo: grpc.MethodInfo{Name: "UnaryCall", IsClientStream: false, IsServerStream: false}, Package: "grpc.testing", Service: "TestService", FullMethod: "/grpc.testing.TestService/UnaryCall"}, {MethodInfo: grpc.MethodInfo{Name: "StreamingOutputCall", IsClientStream: false, IsServerStream: true}, Package: "grpc.testing", Service: "TestService", FullMethod: "/grpc.testing.TestService/StreamingOutputCall"}, {MethodInfo: grpc.MethodInfo{Name: "StreamingInputCall", IsClientStream: true, IsServerStream: false}, Package: "grpc.testing", Service: "TestService", FullMethod: "/grpc.testing.TestService/StreamingInputCall"}, {MethodInfo: grpc.MethodInfo{Name: "FullDuplexCall", IsClientStream: true, IsServerStream: true}, Package: "grpc.testing", Service: "TestService", FullMethod: "/grpc.testing.TestService/FullDuplexCall"}, {MethodInfo: grpc.MethodInfo{Name: "HalfDuplexCall", IsClientStream: true, IsServerStream: true}, Package: "grpc.testing", Service: "TestService", FullMethod: "/grpc.testing.TestService/HalfDuplexCall"}},
		},
		{
			name: "ConnectInit",
			vuString: `
			var client = new grpc.Client();
			client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");
			client.connect();`,
			error: "connecting to a gRPC server in the init context is not supported",
		},
		{
			name: "InvokeInit",
			vuString: `
			var client = new grpc.Client();
			client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");
			var err = client.invoke();
			throw new Error(err)`,
			error: "invoking RPC methods in the init context is not supported",
		},
		{
			name: "NoConnect",
			initString: `
				var client = new grpc.Client();
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`,
			vuString: `client.invoke("grpc.testing.TestService/EmptyCall", {})`,
			error:    "invoking RPC methods in the init context is not supported",
		},
		{
			name: "UnknownConnectParam",
			initString: `
				var client = new grpc.Client();
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`,
			vuString: `client.connect("GRPCBIN_ADDR", { name: "k6" });`,
			error:    `unknown connect param: "name"`,
			vuctx:    true,
		},
		{
			name: "ConnectInvalidTimeout",
			initString: `
				var client = new grpc.Client();
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`,
			vuString: `client.connect("GRPCBIN_ADDR", { timeout: "k6" });`,
			error:    "invalid duration",
			vuctx:    true,
		},
		{
			name: "ConnectStringTimeout",
			initString: `
				var client = new grpc.Client();
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`,
			vuString: `client.connect("GRPCBIN_ADDR", { timeout: "1h3s" });`,
			vuctx:    true,
		},
		{
			name: "ConnectIntegerTimeout",
			initString: `
				var client = new grpc.Client();
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`,
			vuString: `client.connect("GRPCBIN_ADDR", { timeout: 3000 });`,
			vuctx:    true,
		},
		{
			name: "Connect",
			initString: `
				var client = new grpc.Client();
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`,
			vuString: `client.connect("GRPCBIN_ADDR");`,
			vuctx:    true,
		},
		{
			name: "InvokeNotFound",
			initString: `
				var client = new grpc.Client();
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`,
			vuString: `
				client.connect("GRPCBIN_ADDR");
				client.invoke("foo/bar", {})`,
			error: `method "/foo/bar" not found in file descriptors`,
			vuctx: true,
		},
		{
			name: "InvokeInvalidParam",
			initString: `
				var client = new grpc.Client();
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`,
			vuString: `
				client.connect("GRPCBIN_ADDR");
				client.invoke("grpc.testing.TestService/EmptyCall", {}, { void: true })`,
			error: `unknown param: "void"`,
			vuctx: true,
		},
		{
			name: "InvokeInvalidTimeoutType",
			initString: `
				var client = new grpc.Client();
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`,
			vuString: `
				client.connect("GRPCBIN_ADDR");
				client.invoke("grpc.testing.TestService/EmptyCall", {}, { timeout: true })`,
			error: "invalid timeout value: unable to use type bool as a duration value",
			vuctx: true,
		},
		{
			name: "InvokeInvalidTimeout",
			initString: `
				var client = new grpc.Client();
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`,
			vuString: `
				client.connect("GRPCBIN_ADDR");
				client.invoke("grpc.testing.TestService/EmptyCall", {}, { timeout: "please" })`,
			error: "invalid duration",
			vuctx: true,
		},
		{
			name: "InvokeStringTimeout",
			initString: `
				var client = new grpc.Client();
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`,
			vuString: `
				client.connect("GRPCBIN_ADDR");
				client.invoke("grpc.testing.TestService/EmptyCall", {}, { timeout: "1h42m" })`,
			vuctx: true,
		},
		{
			name: "InvokeFloatTimeout",
			initString: `
				var client = new grpc.Client();
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`,
			vuString: `
				client.connect("GRPCBIN_ADDR");
				client.invoke("grpc.testing.TestService/EmptyCall", {}, { timeout: 400.50 })`,
			vuctx: true,
		},
		{
			name: "InvokeIntegerTimeout",
			initString: `
				var client = new grpc.Client();
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`,
			vuString: `
				client.connect("GRPCBIN_ADDR");
				client.invoke("grpc.testing.TestService/EmptyCall", {}, { timeout: 2000 })`,
			vuctx: true,
		},
		{
			name: "Invoke",
			initString: `
				var client = new grpc.Client();
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`,
			setupserver: func(tb *httpmultibin.HTTPMultiBin) {
				tb.GRPCStub.EmptyCallFunc = func(context.Context, *grpc_testing.Empty) (*grpc_testing.Empty, error) {
					return &grpc_testing.Empty{}, nil
				}
			},
			vuString: `
				client.connect("GRPCBIN_ADDR");
				var resp = client.invoke("grpc.testing.TestService/EmptyCall", {})
				if (resp.status !== grpc.StatusOK) {
					throw new Error("unexpected error status: " + resp.status)
				}`,
			vuctx: true,
		},
		{
			name: "Invoke",
			initString: `
				var client = new grpc.Client();
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`,
			setupserver: func(tb *httpmultibin.HTTPMultiBin) {
				tb.GRPCStub.EmptyCallFunc = func(context.Context, *grpc_testing.Empty) (*grpc_testing.Empty, error) {
					return &grpc_testing.Empty{}, nil
				}
			},
			vuString: `
				client.connect("GRPCBIN_ADDR");
				var resp = client.invoke("grpc.testing.TestService/EmptyCall", {})
				if (resp.status !== grpc.StatusOK) {
					throw new Error("unexpected error status: " + resp.status)
				}`,
			asserts: func(t *testing.T, rb *httpmultibin.HTTPMultiBin, samples chan stats.SampleContainer) {
				samplesBuf := stats.GetBufferedSamples(samples)
				assertMetricEmitted(t, metrics.GRPCReqDuration, samplesBuf, rb.Replacer.Replace("GRPCBIN_ADDR/grpc.testing.TestService/EmptyCall"))
			},
			vuctx: true,
		},
		{
			name: "RequestMessage",
			initString: `
				var client = new grpc.Client();
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`,
			setupserver: func(tb *httpmultibin.HTTPMultiBin) {
				tb.GRPCStub.UnaryCallFunc = func(_ context.Context, req *grpc_testing.SimpleRequest) (*grpc_testing.SimpleResponse, error) {
					if req.Payload == nil || string(req.Payload.Body) != "负载测试" {
						return nil, status.Error(codes.InvalidArgument, "")
					}
					return &grpc_testing.SimpleResponse{}, nil
				}
			},
			vuString: `
				client.connect("GRPCBIN_ADDR");
				var resp = client.invoke("grpc.testing.TestService/UnaryCall", { payload: { body: "6LSf6L295rWL6K+V"} })
				if (resp.status !== grpc.StatusOK) {
					throw new Error("server did not receive the correct request message")
				}
		`,
			vuctx: true,
		},
		{
			name: "RequestHeaders",
			initString: `
				var client = new grpc.Client();
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`,
			setupserver: func(tb *httpmultibin.HTTPMultiBin) {
				tb.GRPCStub.EmptyCallFunc = func(ctx context.Context, _ *grpc_testing.Empty) (*grpc_testing.Empty, error) {
					md, ok := metadata.FromIncomingContext(ctx)
					if !ok || len(md["x-load-tester"]) == 0 || md["x-load-tester"][0] != "k6" {
						return nil, status.Error(codes.FailedPrecondition, "")
					}

					return &grpc_testing.Empty{}, nil
				}
			},
			vuString: `
				client.connect("GRPCBIN_ADDR");
				var resp = client.invoke("grpc.testing.TestService/EmptyCall", {}, { headers: { "X-Load-Tester": "k6" } })
				if (resp.status !== grpc.StatusOK) {
					throw new Error("failed to send correct headers in the request")
				}
			`,
			vuctx: true,
		},
		{
			name: "ResponseMessage",
			initString: `
				var client = new grpc.Client();
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`,
			setupserver: func(tb *httpmultibin.HTTPMultiBin) {
				tb.GRPCStub.UnaryCallFunc = func(context.Context, *grpc_testing.SimpleRequest) (*grpc_testing.SimpleResponse, error) {
					return &grpc_testing.SimpleResponse{
						OauthScope: "水",
					}, nil
				}
			},
			vuString: `
				client.connect("GRPCBIN_ADDR");
				var resp = client.invoke("grpc.testing.TestService/UnaryCall", {})
				if (!resp.message || resp.message.username !== "" || resp.message.oauthScope !== "水") {
					throw new Error("unexpected response message: " + JSON.stringify(resp.message))
				}`,
			asserts: func(t *testing.T, rb *httpmultibin.HTTPMultiBin, samples chan stats.SampleContainer) {
				samplesBuf := stats.GetBufferedSamples(samples)
				assertMetricEmitted(t, metrics.GRPCReqDuration, samplesBuf, rb.Replacer.Replace("GRPCBIN_ADDR/grpc.testing.TestService/UnaryCall"))
			},
			vuctx: true,
		},
		{
			name: "ResponseError",
			initString: `
				var client = new grpc.Client();
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`,
			setupserver: func(tb *httpmultibin.HTTPMultiBin) {
				tb.GRPCStub.EmptyCallFunc = func(context.Context, *grpc_testing.Empty) (*grpc_testing.Empty, error) {
					return nil, status.Error(codes.DataLoss, "foobar")
				}
			},
			vuString: `
				client.connect("GRPCBIN_ADDR");
				var resp = client.invoke("grpc.testing.TestService/EmptyCall", {})
				if (resp.status !== grpc.StatusDataLoss) {
					throw new Error("unexpected error status: " + resp.status)
				}
				if (!resp.error || resp.error.message !== "foobar" || resp.error.code !== 15) {
					throw new Error("unexpected error object: " + JSON.stringify(resp.error.code))
				}`,
			asserts: func(t *testing.T, rb *httpmultibin.HTTPMultiBin, samples chan stats.SampleContainer) {
				samplesBuf := stats.GetBufferedSamples(samples)
				assertMetricEmitted(t, metrics.GRPCReqDuration, samplesBuf, rb.Replacer.Replace("GRPCBIN_ADDR/grpc.testing.TestService/EmptyCall"))
			},
			vuctx: true,
		},
		{
			name: "ResponseHeaders",
			initString: `
				var client = new grpc.Client();
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`,
			setupserver: func(tb *httpmultibin.HTTPMultiBin) {
				tb.GRPCStub.EmptyCallFunc = func(ctx context.Context, _ *grpc_testing.Empty) (*grpc_testing.Empty, error) {
					md := metadata.Pairs("foo", "bar")
					_ = grpc.SetHeader(ctx, md)
					return &grpc_testing.Empty{}, nil
				}
			},
			vuString: `
				client.connect("GRPCBIN_ADDR");
				var resp = client.invoke("grpc.testing.TestService/EmptyCall", {})
				if (resp.status !== grpc.StatusOK) {
					throw new Error("unexpected error status: " + resp.status)
				}
				if (!resp.headers || !resp.headers["foo"] || resp.headers["foo"][0] !== "bar") {
					throw new Error("unexpected headers object: " + JSON.stringify(resp.trailers))
				}`,
			asserts: func(t *testing.T, rb *httpmultibin.HTTPMultiBin, samples chan stats.SampleContainer) {
				samplesBuf := stats.GetBufferedSamples(samples)
				assertMetricEmitted(t, metrics.GRPCReqDuration, samplesBuf, rb.Replacer.Replace("GRPCBIN_ADDR/grpc.testing.TestService/EmptyCall"))
			},
			vuctx: true,
		},
		{
			name: "ResponseTrailers",
			initString: `
				var client = new grpc.Client();
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`,
			setupserver: func(tb *httpmultibin.HTTPMultiBin) {
				tb.GRPCStub.EmptyCallFunc = func(ctx context.Context, _ *grpc_testing.Empty) (*grpc_testing.Empty, error) {
					md := metadata.Pairs("foo", "bar")
					_ = grpc.SetTrailer(ctx, md)
					return &grpc_testing.Empty{}, nil
				}
			},
			vuString: `
				client.connect("GRPCBIN_ADDR");
				var resp = client.invoke("grpc.testing.TestService/EmptyCall", {})
				if (resp.status !== grpc.StatusOK) {
					throw new Error("unexpected error status: " + resp.status)
				}
				if (!resp.trailers || !resp.trailers["foo"] || resp.trailers["foo"][0] !== "bar") {
					throw new Error("unexpected trailers object: " + JSON.stringify(resp.trailers))
				}
		`,
			asserts: func(t *testing.T, rb *httpmultibin.HTTPMultiBin, samples chan stats.SampleContainer) {
				samplesBuf := stats.GetBufferedSamples(samples)
				assertMetricEmitted(t, metrics.GRPCReqDuration, samplesBuf, rb.Replacer.Replace("GRPCBIN_ADDR/grpc.testing.TestService/EmptyCall"))
			},
			vuctx: true,
		},
		{
			name: "LoadNotInit",
			setupserver: func(tb *httpmultibin.HTTPMultiBin) {
				tb.GRPCStub.EmptyCallFunc = func(ctx context.Context, _ *grpc_testing.Empty) (*grpc_testing.Empty, error) {
					md := metadata.Pairs("foo", "bar")
					_ = grpc.SetTrailer(ctx, md)
					return &grpc_testing.Empty{}, nil
				}
			},
			initString: `
				var client = new grpc.Client();
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`,
			vuString: `client.load()`,
			asserts: func(t *testing.T, rb *httpmultibin.HTTPMultiBin, samples chan stats.SampleContainer) {
				samplesBuf := stats.GetBufferedSamples(samples)
				assertMetricEmitted(t, metrics.GRPCReqDuration, samplesBuf, rb.Replacer.Replace("GRPCBIN_ADDR/grpc.testing.TestService/EmptyCall"))
			},
			error: "load must be called in the init context",
			vuctx: true,
		},
		{
			name: "Close",
			initString: `
				var client = new grpc.Client();
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`,
			vuString: `
			client.close();
			client.invoke();`,
			error: "no gRPC connection",
			vuctx: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err, runtime, state, initEnv, tb, samples := setup(t)
			ctx := common.WithRuntime(context.Background(), runtime)
			ctx = common.WithInitEnv(ctx, initEnv)
			runtime.Set("grpc", common.Bind(runtime, New(), &ctx))
			if test.setupserver != nil {
				test.setupserver(tb)
			}
			if test.initString != "" {
				runtime.RunString(tb.Replacer.Replace(test.initString))
			}
			if test.vuctx {
				ctx = lib.WithState(ctx, state)
			}
			val, err := runtime.RunString(tb.Replacer.Replace(test.vuString))
			if test.error == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), test.error)
			}
			if test.val != nil {
				require.NotNil(t, test.val)
				assert.Equal(t, test.val, val.Export())
			}
			if test.asserts != nil {
				test.asserts(t, tb, samples)
			}
		})
	}
}

func setup(t *testing.T) (error, *goja.Runtime, *lib.State, *common.InitEnvironment, *httpmultibin.HTTPMultiBin, chan stats.SampleContainer) {
	tb := httpmultibin.NewHTTPMultiBin(t)
	root, err := lib.NewGroup("", nil)
	assert.NoError(t, err)
	runtime := goja.New()
	runtime.SetFieldNameMapper(common.FieldNameMapper{})
	samples := make(chan stats.SampleContainer, 1000)
	state := &lib.State{
		Group:     root,
		Dialer:    tb.Dialer,
		TLSConfig: tb.TLSClientConfig,
		Samples:   samples,
		Options: lib.Options{
			SystemTags: stats.NewSystemTagSet(
				stats.TagName,
				stats.TagURL,
			),
			UserAgent: null.StringFrom("k6-test"),
		},
	}
	cwd, err := os.Getwd()
	require.NoError(t, err)
	fs := afero.NewOsFs()
	if isWindows {
		fs = fsext.NewTrimFilePathSeparatorFs(fs)
	}
	initEnv := &common.InitEnvironment{
		Logger: logrus.New(),
		CWD:    &url.URL{Path: cwd},
		FileSystems: map[string]afero.Fs{
			"file": fs,
		},
	}
	return err, runtime, state, initEnv, tb, samples
}
