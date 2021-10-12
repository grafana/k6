/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2020 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package grpc

import (
	"bytes"
	"context"
	"errors"
	"net/url"
	"os"
	"runtime"
	"strings"
	"testing"

	"google.golang.org/grpc/reflection"

	"github.com/dop251/goja"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	grpcstats "google.golang.org/grpc/stats"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/grpc_testing"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/fsext"
	"go.k6.io/k6/lib/metrics"
	"go.k6.io/k6/lib/testutils/httpmultibin"
	"go.k6.io/k6/stats"
)

const isWindows = runtime.GOOS == "windows"

// codeBlock represents an execution of a k6 script.
type codeBlock struct {
	code       string
	val        interface{}
	err        string
	windowsErr string
	asserts    func(*testing.T, *httpmultibin.HTTPMultiBin, chan stats.SampleContainer, error)
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
		rt      *goja.Runtime
		vuState *lib.State
		env     *common.InitEnvironment
		httpBin *httpmultibin.HTTPMultiBin
		samples chan stats.SampleContainer
	}
	setup := func(t *testing.T) testState {
		t.Helper()

		root, err := lib.NewGroup("", nil)
		require.NoError(t, err)
		tb := httpmultibin.NewHTTPMultiBin(t)
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
			BuiltinMetrics: metrics.RegisterBuiltinMetrics(
				metrics.NewRegistry(),
			),
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

		rt := goja.New()
		rt.SetFieldNameMapper(common.FieldNameMapper{})

		return testState{
			rt:      rt,
			httpBin: tb,
			vuState: state,
			env:     initEnv,
			samples: samples,
		}
	}

	assertMetricEmitted := func(
		t *testing.T,
		metricName string,
		sampleContainers []stats.SampleContainer,
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
			client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`,
				val: []MethodInfo{{MethodInfo: grpc.MethodInfo{Name: "EmptyCall", IsClientStream: false, IsServerStream: false}, Package: "grpc.testing", Service: "TestService", FullMethod: "/grpc.testing.TestService/EmptyCall"}, {MethodInfo: grpc.MethodInfo{Name: "UnaryCall", IsClientStream: false, IsServerStream: false}, Package: "grpc.testing", Service: "TestService", FullMethod: "/grpc.testing.TestService/UnaryCall"}, {MethodInfo: grpc.MethodInfo{Name: "StreamingOutputCall", IsClientStream: false, IsServerStream: true}, Package: "grpc.testing", Service: "TestService", FullMethod: "/grpc.testing.TestService/StreamingOutputCall"}, {MethodInfo: grpc.MethodInfo{Name: "StreamingInputCall", IsClientStream: true, IsServerStream: false}, Package: "grpc.testing", Service: "TestService", FullMethod: "/grpc.testing.TestService/StreamingInputCall"}, {MethodInfo: grpc.MethodInfo{Name: "FullDuplexCall", IsClientStream: true, IsServerStream: true}, Package: "grpc.testing", Service: "TestService", FullMethod: "/grpc.testing.TestService/FullDuplexCall"}, {MethodInfo: grpc.MethodInfo{Name: "HalfDuplexCall", IsClientStream: true, IsServerStream: true}, Package: "grpc.testing", Service: "TestService", FullMethod: "/grpc.testing.TestService/HalfDuplexCall"}},
			},
		},
		{
			name: "ConnectInit",
			initString: codeBlock{
				code: `
			var client = new grpc.Client();
			client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");
			client.connect();`,
				err: "connecting to a gRPC server in the init context is not supported",
			},
		},
		{
			name: "InvokeInit",
			initString: codeBlock{
				code: `
			var client = new grpc.Client();
			client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");
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
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");
				client.invoke("grpc.testing.TestService/EmptyCall", {})`,
				err: "invoking RPC methods in the init context is not supported",
			},
		},
		{
			name: "UnknownConnectParam",
			initString: codeBlock{code: `
				var client = new grpc.Client();
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`},
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
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`,
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
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`},
			vuString: codeBlock{code: `client.connect("GRPCBIN_ADDR", { timeout: "1h3s" });`},
		},
		{
			name: "ConnectIntegerTimeout",
			initString: codeBlock{code: `
				var client = new grpc.Client();
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`},
			vuString: codeBlock{code: `client.connect("GRPCBIN_ADDR", { timeout: 3000 });`},
		},
		{
			name: "ConnectFloatTimeout",
			initString: codeBlock{code: `
				var client = new grpc.Client();
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`},
			vuString: codeBlock{code: `client.connect("GRPCBIN_ADDR", { timeout: 3456.3 });`},
		},
		{
			name: "Connect",
			initString: codeBlock{code: `
				var client = new grpc.Client();
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`},
			vuString: codeBlock{code: `client.connect("GRPCBIN_ADDR");`},
		},
		{
			name: "InvokeNotFound",
			initString: codeBlock{code: `
				var client = new grpc.Client();
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`},
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
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`},
			vuString: codeBlock{
				code: `
				client.connect("GRPCBIN_ADDR");
				client.invoke("grpc.testing.TestService/EmptyCall", {}, { void: true })`,
				err: `unknown param: "void"`,
			},
		},
		{
			name: "InvokeInvalidTimeoutType",
			initString: codeBlock{code: `
				var client = new grpc.Client();
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`},
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
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`},
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
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`},
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
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`},
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
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`,
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
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`},
			setup: func(tb *httpmultibin.HTTPMultiBin) {
				tb.GRPCStub.EmptyCallFunc = func(context.Context, *grpc_testing.Empty) (*grpc_testing.Empty, error) {
					return &grpc_testing.Empty{}, nil
				}
			},
			vuString: codeBlock{code: `
				client.connect("GRPCBIN_ADDR");
				var resp = client.invoke("grpc.testing.TestService/EmptyCall", {})
				if (resp.status !== grpc.StatusOK) {
					throw new Error("unexpected error status: " + resp.status)
				}`},
		},
		{
			name: "Invoke",
			initString: codeBlock{code: `
				var client = new grpc.Client();
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`},
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
					throw new Error("unexpected error status: " + resp.status)
				}`,
				asserts: func(t *testing.T, rb *httpmultibin.HTTPMultiBin, samples chan stats.SampleContainer, _ error) {
					samplesBuf := stats.GetBufferedSamples(samples)
					assertMetricEmitted(t, metrics.GRPCReqDurationName, samplesBuf, rb.Replacer.Replace("GRPCBIN_ADDR/grpc.testing.TestService/EmptyCall"))
				},
			},
		},
		{
			name: "RequestMessage",
			initString: codeBlock{
				code: `
				var client = new grpc.Client();
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`,
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
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`,
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
				var resp = client.invoke("grpc.testing.TestService/EmptyCall", {}, { headers: { "X-Load-Tester": "k6" } })
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
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`,
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
				asserts: func(t *testing.T, rb *httpmultibin.HTTPMultiBin, samples chan stats.SampleContainer, _ error) {
					samplesBuf := stats.GetBufferedSamples(samples)
					assertMetricEmitted(t, metrics.GRPCReqDurationName, samplesBuf, rb.Replacer.Replace("GRPCBIN_ADDR/grpc.testing.TestService/UnaryCall"))
				},
			},
		},
		{
			name: "ResponseError",
			initString: codeBlock{
				code: `
				var client = new grpc.Client();
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`,
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
				asserts: func(t *testing.T, rb *httpmultibin.HTTPMultiBin, samples chan stats.SampleContainer, _ error) {
					samplesBuf := stats.GetBufferedSamples(samples)
					assertMetricEmitted(t, metrics.GRPCReqDurationName, samplesBuf, rb.Replacer.Replace("GRPCBIN_ADDR/grpc.testing.TestService/EmptyCall"))
				},
			},
		},
		{
			name: "ResponseHeaders",
			initString: codeBlock{
				code: `
				var client = new grpc.Client();
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`,
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
				asserts: func(t *testing.T, rb *httpmultibin.HTTPMultiBin, samples chan stats.SampleContainer, _ error) {
					samplesBuf := stats.GetBufferedSamples(samples)
					assertMetricEmitted(t, metrics.GRPCReqDurationName, samplesBuf, rb.Replacer.Replace("GRPCBIN_ADDR/grpc.testing.TestService/EmptyCall"))
				},
			},
		},
		{
			name: "ResponseTrailers",
			initString: codeBlock{
				code: `
				var client = new grpc.Client();
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`,
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
				asserts: func(t *testing.T, rb *httpmultibin.HTTPMultiBin, samples chan stats.SampleContainer, _ error) {
					samplesBuf := stats.GetBufferedSamples(samples)
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
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`,
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
			name: "Close",
			initString: codeBlock{
				code: `
				var client = new grpc.Client();
				client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");`,
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

			// create and attach the GRPC module to goja
			ctx := common.WithRuntime(context.Background(), ts.rt)
			ctx = common.WithInitEnv(ctx, ts.env)
			err := ts.rt.Set("grpc", common.Bind(ts.rt, New(), &ctx))
			require.NoError(t, err)

			// setup necessary environment if needed by a test
			if tt.setup != nil {
				tt.setup(ts.httpBin)
			}

			replace := func(code string) (goja.Value, error) {
				return ts.rt.RunString(ts.httpBin.Replacer.Replace(code))
			}

			val, err := replace(tt.initString.code)
			assertResponse(t, tt.initString, err, val, ts)

			ctx = lib.WithState(ctx, ts.vuState)
			val, err = replace(tt.vuString.code)
			assertResponse(t, tt.vuString, err, val, ts)
		})
	}
}

func TestGoja(t *testing.T) {
	t.Parallel()
	t.Skip()
	// ctx := common.WithInitEnv(common.WithRuntime(context.Background(), rt), initEnv)
	// require.NoError(t, rt.Set("grpc", common.Bind(rt, New(), &ctx)))

	// if tt.setup != nil {
	// 	tt.setup(tb)
	// }
	// val, err := rt.RunString(tb.Replacer.Replace(tt.initString.code))
	// assertResponse(t, tt.initString, err, val, tb, samples)

	// ctx = lib.WithState(ctx, state)
	// val, err = rt.RunString(tb.Replacer.Replace(tt.vuString.code))
	// assertResponse(t, tt.vuString, err, val, tb, samples)
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

			debugStat(tt.stat, logger.WithField("source", "test"), "full")
			assert.Contains(t, b.String(), tt.expected)
		})
	}
}
