package grpc

import (
	"net"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/grafana/xk6-disruptor/pkg/agent/protocol"
	"github.com/grafana/xk6-disruptor/pkg/testutils/grpc/ping"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func Test_Validations(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title       string
		disruption  Disruption
		upstream    string
		expectError bool
	}{
		{
			title: "valid defaults",
			disruption: Disruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      0.0,
				StatusCode:     0,
				StatusMessage:  "",
			},
			upstream:    ":8080",
			expectError: false,
		},
		{
			title: "invalid upstream address",
			disruption: Disruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      0.0,
				StatusCode:     0,
				StatusMessage:  "",
			},
			upstream:    "",
			expectError: true,
		},
		{
			title: "variation larger than average delay",
			disruption: Disruption{
				AverageDelay:   100,
				DelayVariation: 200,
				ErrorRate:      0.0,
				StatusCode:     0,
				StatusMessage:  "",
			},
			upstream:    ":8080",
			expectError: true,
		},
		{
			title: "valid error rate",
			disruption: Disruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      0.1,
				StatusCode:     uint32(codes.Internal),
				StatusMessage:  "",
			},
			upstream:    ":8080",
			expectError: false,
		},
		{
			title: "valid delay and variation",
			disruption: Disruption{
				AverageDelay:   100,
				DelayVariation: 10,
				ErrorRate:      0.0,
				StatusCode:     0,
				StatusMessage:  "",
			},
			upstream:    ":8080",
			expectError: false,
		},
		{
			title: "invalid error code",
			disruption: Disruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      1.0,
				StatusCode:     0,
				StatusMessage:  "",
			},
			upstream:    ":8080",
			expectError: true,
		},
		{
			title: "negative error rate",
			disruption: Disruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      -1.0,
				StatusCode:     0,
				StatusMessage:  "",
			},
			upstream:    ":8080",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			listener, err := net.Listen("tcp", "localhost:0")
			if err != nil {
				t.Fatalf("could not create listener: %v", err)
			}

			_, err = NewProxy(
				listener,
				tc.upstream,
				tc.disruption,
			)
			if !tc.expectError && err != nil {
				t.Errorf("failed: %v", err)
			}

			if tc.expectError && err == nil {
				t.Errorf("should had failed")
			}
		})
	}
}

func Test_ProxyHandler(t *testing.T) {
	t.Parallel()

	type TestCase struct {
		title        string
		disruption   Disruption
		request      *ping.PingRequest
		response     *ping.PingResponse
		expectStatus codes.Code
	}

	// TODO: Add test for excluded endpoints
	testCases := []TestCase{
		{
			title: "default proxy",
			disruption: Disruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      0.0,
				StatusCode:     0,
				StatusMessage:  "",
			},
			request: &ping.PingRequest{
				Error:   0,
				Message: "ping",
			},
			response: &ping.PingResponse{
				Message: "ping",
			},
			expectStatus: codes.OK,
		},
		{
			title: "error injection",
			disruption: Disruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      1.0,
				StatusCode:     uint32(codes.Internal),
				StatusMessage:  "Internal server error",
			},
			request: &ping.PingRequest{
				Error:   0,
				Message: "ping",
			},
			response:     nil,
			expectStatus: codes.Internal,
		},
		{
			title: "delay injection",
			disruption: Disruption{
				AverageDelay:   10,
				DelayVariation: 0,
				ErrorRate:      0.0,
				StatusCode:     0,
				StatusMessage:  "",
			},
			request: &ping.PingRequest{
				Error:   0,
				Message: "ping",
			},
			response: &ping.PingResponse{
				Message: "ping",
			},
			expectStatus: codes.OK,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			upstreamListener, err := net.Listen("tcp", "localhost:0")
			if err != nil {
				t.Fatalf("error starting test upstream listener: %v", err)
			}
			srv := grpc.NewServer()
			ping.RegisterPingServiceServer(srv, ping.NewPingServer())
			go func() {
				if serr := srv.Serve(upstreamListener); err != nil {
					t.Logf("error in the server: %v", serr)
				}
			}()

			proxyListener, err := net.Listen("tcp", "localhost:0")
			if err != nil {
				t.Fatalf("error starting test proxy listener: %v", err)
			}

			proxy, err := NewProxy(proxyListener, upstreamListener.Addr().String(), tc.disruption)
			if err != nil {
				t.Errorf("error creating proxy: %v", err)
				return
			}
			defer func() {
				_ = proxy.Stop()
			}()

			go func() {
				if perr := proxy.Start(); perr != nil {
					t.Logf("error starting proxy: %v", perr)
				}
			}()

			time.Sleep(time.Second)

			// connect client to proxy
			conn, err := grpc.DialContext(
				t.Context(),
				proxyListener.Addr().String(),
				grpc.WithInsecure(),
			)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				_ = conn.Close()
			}()

			client := ping.NewPingServiceClient(conn)

			var headers metadata.MD
			response, err := client.Ping(
				t.Context(),
				tc.request,
				grpc.Header(&headers),
				grpc.WaitForReady(true),
			)
			if err != nil && tc.expectStatus == codes.OK {
				t.Errorf("unexpected error %v", err)
				return
			}

			// got an error but it is not due to the grpc status
			s, ok := status.FromError(err)
			if !ok {
				t.Errorf("unexpected error %v", err)
				return
			}

			if s.Code() != tc.expectStatus {
				t.Errorf("expected '%s' but got '%s'", tc.expectStatus.String(), s.Code().String())
				return
			}

			if !ping.CompareResponses(response, tc.response) {
				t.Errorf("expected '%s' but got '%s'", tc.response, response)
				return
			}
		})
	}
}

func Test_ProxyMetrics(t *testing.T) {
	t.Parallel()

	type TestCase struct {
		title           string
		disruption      Disruption
		skipRequest     bool
		expectedMetrics map[string]uint
	}

	// TODO: Add test for excluded endpoints
	testCases := []TestCase{
		{
			title: "no requests",
			disruption: Disruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      0.0,
				StatusCode:     0,
				StatusMessage:  "",
			},
			skipRequest: true,
			expectedMetrics: map[string]uint{
				protocol.MetricRequests:          0,
				protocol.MetricRequestsDisrupted: 0,
				protocol.MetricRequestsExcluded:  0,
			},
		},
		{
			title: "passthrough",
			disruption: Disruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      0.0,
				StatusCode:     0,
				StatusMessage:  "",
			},
			expectedMetrics: map[string]uint{
				protocol.MetricRequests:          1,
				protocol.MetricRequestsDisrupted: 0,
				protocol.MetricRequestsExcluded:  0,
			},
		},
		{
			title: "error injection",
			disruption: Disruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      1.0,
				StatusCode:     uint32(codes.Internal),
				StatusMessage:  "Internal server error",
			},
			expectedMetrics: map[string]uint{
				protocol.MetricRequests:          1,
				protocol.MetricRequestsDisrupted: 1,
				protocol.MetricRequestsExcluded:  0,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			upstreamListener, err := net.Listen("tcp", "localhost:0")
			if err != nil {
				t.Fatalf("error starting test upstream listener: %v", err)
			}
			srv := grpc.NewServer()
			ping.RegisterPingServiceServer(srv, ping.NewPingServer())
			go func() {
				if serr := srv.Serve(upstreamListener); err != nil {
					t.Logf("error in the server: %v", serr)
				}
			}()

			proxyListener, err := net.Listen("tcp", "localhost:0")
			if err != nil {
				t.Fatalf("error starting test proxy listener: %v", err)
			}

			proxy, err := NewProxy(proxyListener, upstreamListener.Addr().String(), tc.disruption)
			if err != nil {
				t.Errorf("error creating proxy: %v", err)
				return
			}
			defer func() {
				_ = proxy.Stop()
			}()

			go func() {
				if perr := proxy.Start(); perr != nil {
					t.Logf("error starting proxy: %v", perr)
				}
			}()

			time.Sleep(time.Second)

			// connect client to proxy
			conn, err := grpc.DialContext(
				t.Context(),
				proxyListener.Addr().String(),
				grpc.WithInsecure(),
			)
			if err != nil {
				t.Fatal(err)
			}

			defer func() {
				_ = conn.Close()
			}()

			if !tc.skipRequest {
				client := ping.NewPingServiceClient(conn)

				var headers metadata.MD
				_, _ = client.Ping(
					t.Context(),
					&ping.PingRequest{
						Error:   0,
						Message: "ping",
					},
					grpc.Header(&headers),
					grpc.WaitForReady(true),
				)
			}

			metrics := proxy.Metrics()

			if diff := cmp.Diff(tc.expectedMetrics, metrics); diff != "" {
				t.Fatalf("expected metrics do not match returned:\n%s", diff)
			}
		})
	}
}
