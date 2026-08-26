package ping

import (
	"testing"

	grpcutils "github.com/grafana/xk6-disruptor/pkg/testutils/grpc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	status "google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func Test_PingServer(t *testing.T) {
	t.Parallel()

	type TestCase struct {
		title         string
		request       *PingRequest
		response      *PingResponse
		expectStatus  codes.Code
		expectHeaders map[string]string
	}

	testCases := []TestCase{
		{
			title: "ping without error",
			request: &PingRequest{
				Error:   0,
				Message: "ping",
			},
			response: &PingResponse{
				Message: "ping",
			},
			expectStatus:  codes.OK,
			expectHeaders: map[string]string{},
		},
		{
			title: "ping with error",
			request: &PingRequest{
				Error:   int32(codes.Internal),
				Message: "ping",
			},
			response:      nil,
			expectStatus:  codes.Internal,
			expectHeaders: map[string]string{},
		},
		{
			title: "ping with headers",
			request: &PingRequest{
				Error:   int32(codes.OK),
				Message: "ping",
				Headers: map[string]string{
					"ping-header": "ping-header-value",
				},
			},
			response: &PingResponse{
				Message: "ping",
			},
			expectStatus: codes.OK,
			expectHeaders: map[string]string{
				"ping-header": "ping-header-value",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			l := bufconn.Listen(1)
			srv := grpc.NewServer()
			defer srv.Stop()

			RegisterPingServiceServer(srv, NewPingServer())
			go func() {
				if err := srv.Serve(l); err != nil {
					t.Logf("error in the server: %v", err)
				}
			}()

			conn, err := grpc.DialContext(
				t.Context(),
				"bufnet",
				grpc.WithContextDialer(grpcutils.BuffconnDialer(l)),
				grpc.WithInsecure(),
			)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				_ = conn.Close()
			}()

			client := NewPingServiceClient(conn)

			var headers metadata.MD
			response, err := client.Ping(t.Context(), tc.request, grpc.Header(&headers))

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

			if !CompareResponses(response, tc.response) {
				t.Errorf("expected '%s' but got '%s'", tc.response, response)
				return
			}

			if !CompareHeaders(headers, tc.expectHeaders) {
				t.Errorf("expected '%v' but got '%v'", tc.expectHeaders, headers)
				return
			}
		})
	}
}
