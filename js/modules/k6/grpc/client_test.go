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
	"strings"
	"testing"

	"github.com/dop251/goja"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	grpcstats "google.golang.org/grpc/stats"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/grpc_testing"
	"gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/lib/testutils/httpmultibin"
	"github.com/loadimpact/k6/stats"
)

func assertMetricEmitted(t *testing.T, metric *stats.Metric, sampleContainers []stats.SampleContainer, url string) {
	seenMetric := false

	for _, sampleContainer := range sampleContainers {
		for _, sample := range sampleContainer.GetSamples() {
			surl, ok := sample.Tags.Get("url")
			assert.True(t, ok)
			if surl == url {
				if sample.Metric == metric {
					seenMetric = true
				}
			}
		}
	}
	assert.True(t, seenMetric, "url %s didn't emit %s", url, metric.Name)
}

func TestClient(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)
	defer tb.Cleanup()
	sr := tb.Replacer.Replace

	root, err := lib.NewGroup("", nil)
	assert.NoError(t, err)

	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})
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

	ctx := common.WithRuntime(context.Background(), rt)

	rt.Set("grpc", common.Bind(rt, New(), &ctx))

	t.Run("New", func(t *testing.T) {
		_, err := common.RunString(rt, `
			var client = grpc.newClient();
			if (!client) throw new Error("no client created")
		`)
		assert.NoError(t, err)
	})

	t.Run("LoadNotFound", func(t *testing.T) {
		_, err := common.RunString(rt, `
			client.load([], "./does_not_exist.proto");
		`)
		if !assert.Error(t, err) {
			return
		}

		// (rogchap) this is a bit of a hack as windows reports a different system error than unix
		errStr := strings.Replace(err.Error(), "The system cannot find the file specified", "no such file or directory", 1)

		assert.Contains(t, errStr, "no such file or directory")
	})

	t.Run("Load", func(t *testing.T) {
		respV, err := common.RunString(rt, `
			client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");
		`)
		if !assert.NoError(t, err) {
			return
		}
		resp := respV.Export()
		assert.IsType(t, []MethodInfo{}, resp)
		assert.Len(t, resp, 6)
	})

	t.Run("ConnectInit", func(t *testing.T) {
		_, err := common.RunString(rt, `
			client.connect();
		`)
		if !assert.Error(t, err) {
			return
		}
		assert.Contains(t, err.Error(), "connecting to a gRPC server in the init context is not supported")
	})

	t.Run("invokeInit", func(t *testing.T) {
		_, err := common.RunString(rt, `
			var err = client.invoke();
			throw new Error(err)
		`)
		if !assert.Error(t, err) {
			return
		}
		assert.Contains(t, err.Error(), "invoking RPC methods in the init context is not supported")
	})

	ctx = lib.WithState(ctx, state)

	t.Run("NoConnect", func(t *testing.T) {
		_, err := common.RunString(rt, `
			client.invoke("grpc.testing.TestService/EmptyCall", {})
		`)
		if !assert.Error(t, err) {
			return
		}
		assert.Contains(t, err.Error(), "no gRPC connection, you must call connect first")
	})

	t.Run("UnknownConnectParam", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
			client.connect("GRPCBIN_ADDR", { name: "k6" });
		`))
		if !assert.Error(t, err) {
			return
		}
		assert.Contains(t, err.Error(), "unknown connect param: \"name\"")
	})

	t.Run("ConnectInvalidTimeout", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
			client.connect("GRPCBIN_ADDR", { timeout: "k6" });
		`))
		if !assert.Error(t, err) {
			return
		}
		assert.Contains(t, err.Error(), "unable to parse \"timeout\"")
	})

	t.Run("ConnectStringTimeout", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
			client.connect("GRPCBIN_ADDR", { timeout: "1h3s" });
		`))
		assert.NoError(t, err)
	})

	t.Run("ConnectFloatTimeout", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
			client.connect("GRPCBIN_ADDR", { timeout: 3456.3 });
		`))
		assert.NoError(t, err)
	})

	t.Run("ConnectIntegerTimeout", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
			client.connect("GRPCBIN_ADDR", { timeout: 3000 });
		`))
		assert.NoError(t, err)
	})

	t.Run("Connect", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
			client.connect("GRPCBIN_ADDR");
		`))
		assert.NoError(t, err)
	})

	t.Run("InvokeNotFound", func(t *testing.T) {
		_, err := common.RunString(rt, `
			client.invoke("foo/bar", {})
		`)
		if !assert.Error(t, err) {
			return
		}
		assert.Contains(t, err.Error(), "method \"/foo/bar\" not found in file descriptors")
	})

	t.Run("InvokeInvalidParam", func(t *testing.T) {
		_, err := common.RunString(rt, `
			client.invoke("grpc.testing.TestService/EmptyCall", {}, { void: true })
		`)
		if !assert.Error(t, err) {
			return
		}
		assert.Contains(t, err.Error(), "unknown param: \"void\"")
	})

	t.Run("InvokeInvalidTimeoutType", func(t *testing.T) {
		_, err := common.RunString(rt, `
			client.invoke("grpc.testing.TestService/EmptyCall", {}, { timeout: true })
		`)
		if !assert.Error(t, err) {
			return
		}
		assert.Contains(t, err.Error(), "unable to use type bool as a timeout value")
	})

	t.Run("InvokeInvalidTimeout", func(t *testing.T) {
		_, err := common.RunString(rt, `
			client.invoke("grpc.testing.TestService/EmptyCall", {}, { timeout: "please" })
		`)
		if !assert.Error(t, err) {
			return
		}
		assert.Contains(t, err.Error(), " unable to parse \"timeout\"")
	})

	t.Run("InvokeStringTimeout", func(t *testing.T) {
		_, err := common.RunString(rt, `
			client.invoke("grpc.testing.TestService/EmptyCall", {}, { timeout: "1h42m" })
		`)
		assert.NoError(t, err)
	})

	t.Run("InvokeFloatTimeout", func(t *testing.T) {
		_, err := common.RunString(rt, `
			client.invoke("grpc.testing.TestService/EmptyCall", {}, { timeout: 400.50 })
		`)
		assert.NoError(t, err)
	})

	t.Run("InvokeIntegerTimeout", func(t *testing.T) {
		_, err := common.RunString(rt, `
			client.invoke("grpc.testing.TestService/EmptyCall", {}, { timeout: 2000 })
		`)
		assert.NoError(t, err)
	})

	t.Run("Invoke", func(t *testing.T) {
		tb.GRPCStub.EmptyCallFunc = func(context.Context, *grpc_testing.Empty) (*grpc_testing.Empty, error) {
			return &grpc_testing.Empty{}, nil
		}
		_, err := common.RunString(rt, `
			var resp = client.invoke("grpc.testing.TestService/EmptyCall", {})
			if (resp.status !== grpc.StatusOK) {
				throw new Error("unexpected error status: " + resp.status)
			}
		`)
		assert.NoError(t, err)
		samplesBuf := stats.GetBufferedSamples(samples)
		assertMetricEmitted(t, metrics.GRPCReqDuration, samplesBuf, sr("GRPCBIN_ADDR/grpc.testing.TestService/EmptyCall"))
	})

	t.Run("RequestMessage", func(t *testing.T) {
		tb.GRPCStub.UnaryCallFunc = func(_ context.Context, req *grpc_testing.SimpleRequest) (*grpc_testing.SimpleResponse, error) {
			if req.Payload == nil || string(req.Payload.Body) != "负载测试" {
				return nil, status.Error(codes.InvalidArgument, "")
			}

			return &grpc_testing.SimpleResponse{}, nil
		}
		_, err := common.RunString(rt, `
			var resp = client.invoke("grpc.testing.TestService/UnaryCall", { payload: { body: "6LSf6L295rWL6K+V"} })
			if (resp.status !== grpc.StatusOK) {
				throw new Error("server did not receive the correct request message")
			}
		`)
		assert.NoError(t, err)
	})

	t.Run("RequestHeaders", func(t *testing.T) {
		tb.GRPCStub.EmptyCallFunc = func(ctx context.Context, _ *grpc_testing.Empty) (*grpc_testing.Empty, error) {
			md, ok := metadata.FromIncomingContext(ctx)
			if !ok || len(md["x-load-tester"]) == 0 || md["x-load-tester"][0] != "k6" {
				return nil, status.Error(codes.FailedPrecondition, "")
			}

			return &grpc_testing.Empty{}, nil
		}
		_, err := common.RunString(rt, `
			var resp = client.invoke("grpc.testing.TestService/EmptyCall", {}, { headers: { "X-Load-Tester": "k6" } })
			if (resp.status !== grpc.StatusOK) {
				throw new Error("failed to send correct headers in the request")
			}
		`)
		assert.NoError(t, err)
	})

	t.Run("ResponseMessage", func(t *testing.T) {
		tb.GRPCStub.UnaryCallFunc = func(context.Context, *grpc_testing.SimpleRequest) (*grpc_testing.SimpleResponse, error) {
			return &grpc_testing.SimpleResponse{
				OauthScope: "水",
			}, nil
		}
		_, err := common.RunString(rt, `
			var resp = client.invoke("grpc.testing.TestService/UnaryCall", {})
			if (!resp.message || resp.message.username !== "" || resp.message.oauthScope !== "水") {
				throw new Error("unexpected response message: " + JSON.stringify(resp.message))
			}
		`)
		assert.NoError(t, err)
		samplesBuf := stats.GetBufferedSamples(samples)
		assertMetricEmitted(t, metrics.GRPCReqDuration, samplesBuf, sr("GRPCBIN_ADDR/grpc.testing.TestService/UnaryCall"))
	})

	t.Run("ResponseError", func(t *testing.T) {
		tb.GRPCStub.EmptyCallFunc = func(context.Context, *grpc_testing.Empty) (*grpc_testing.Empty, error) {
			return nil, status.Error(codes.DataLoss, "foobar")
		}
		_, err := common.RunString(rt, `
			var resp = client.invoke("grpc.testing.TestService/EmptyCall", {})
			if (resp.status !== grpc.StatusDataLoss) {
				throw new Error("unexpected error status: " + resp.status)
			}
			if (!resp.error || resp.error.message !== "foobar" || resp.error.code !== 15) {
				throw new Error("unexpected error object: " + JSON.stringify(resp.error.code))
			}
		`)
		assert.NoError(t, err)
		samplesBuf := stats.GetBufferedSamples(samples)
		assertMetricEmitted(t, metrics.GRPCReqDuration, samplesBuf, sr("GRPCBIN_ADDR/grpc.testing.TestService/EmptyCall"))
	})

	t.Run("ResponseHeaders", func(t *testing.T) {
		tb.GRPCStub.EmptyCallFunc = func(ctx context.Context, _ *grpc_testing.Empty) (*grpc_testing.Empty, error) {
			md := metadata.Pairs("foo", "bar")
			_ = grpc.SetHeader(ctx, md)

			return &grpc_testing.Empty{}, nil
		}
		_, err := common.RunString(rt, `
			var resp = client.invoke("grpc.testing.TestService/EmptyCall", {})
			if (resp.status !== grpc.StatusOK) {
				throw new Error("unexpected error status: " + resp.status)
			}
			if (!resp.headers || !resp.headers["foo"] || resp.headers["foo"][0] !== "bar") {
				throw new Error("unexpected headers object: " + JSON.stringify(resp.trailers))
			}
		`)
		assert.NoError(t, err)
	})

	t.Run("ResponseTrailers", func(t *testing.T) {
		tb.GRPCStub.EmptyCallFunc = func(ctx context.Context, _ *grpc_testing.Empty) (*grpc_testing.Empty, error) {
			md := metadata.Pairs("foo", "bar")
			_ = grpc.SetTrailer(ctx, md)

			return &grpc_testing.Empty{}, nil
		}
		_, err := common.RunString(rt, `
			var resp = client.invoke("grpc.testing.TestService/EmptyCall", {})
			if (resp.status !== grpc.StatusOK) {
				throw new Error("unexpected error status: " + resp.status)
			}
			if (!resp.trailers || !resp.trailers["foo"] || resp.trailers["foo"][0] !== "bar") {
				throw new Error("unexpected trailers object: " + JSON.stringify(resp.trailers))
			}
		`)
		assert.NoError(t, err)
	})

	t.Run("LoadNotInit", func(t *testing.T) {
		_, err := common.RunString(rt, "client.load()")
		if !assert.Error(t, err) {
			return
		}
		assert.Contains(t, err.Error(), "load must be called in the init context")
	})

	t.Run("Close", func(t *testing.T) {
		_, err := common.RunString(rt, `
			client.close();
			client.invoke();
		`)
		if !assert.Error(t, err) {
			return
		}
		assert.Contains(t, err.Error(), "no gRPC connection")
	})
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
