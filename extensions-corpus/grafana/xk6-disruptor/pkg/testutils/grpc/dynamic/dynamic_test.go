package dynamic

import (
	"testing"

	grpcutil "github.com/grafana/xk6-disruptor/pkg/testutils/grpc"
	"github.com/grafana/xk6-disruptor/pkg/testutils/grpc/ping"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func Test_PingServer(t *testing.T) {
	t.Parallel()

	type TestCase struct {
		title         string
		request       string
		response      string
		expectStatus  codes.Code
		expectHeaders map[string]string
	}

	testCases := []TestCase{
		{
			title:         "ping without error",
			request:       "{ \"error\":   0, \"message\": \"ping\"}",
			response:      "{\"message\": \"ping\"}",
			expectStatus:  codes.OK,
			expectHeaders: map[string]string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			l := bufconn.Listen(100)
			srv := grpc.NewServer()
			defer srv.Stop()

			ping.RegisterPingServiceServer(srv, ping.NewPingServer())
			reflection.Register(srv)

			go func() {
				if err := srv.Serve(l); err != nil {
					t.Logf("error in the server: %v", err)
				}
			}()

			client, err := NewClientWithDialOptions(
				"buffcon",
				"disruptor.testproto.PingService",
				grpc.WithInsecure(),
				grpc.WithContextDialer(grpcutil.BuffconnDialer(l)),
			)
			if err != nil && tc.expectStatus == codes.OK {
				t.Errorf("unexpected error creating client %v", err)
				return
			}

			err = client.Connect(t.Context())
			if err != nil && tc.expectStatus == codes.OK {
				t.Errorf("unexpected error connecting to service %v", err)
				return
			}

			input := [][]byte{}
			input = append(input, []byte(tc.request))

			_, err = client.Invoke(t.Context(), "ping", input)
			if err != nil && tc.expectStatus == codes.OK {
				t.Errorf("unexpected error %v", err)
				return
			}

			// got an error but it is not due to the grpc status
			_, ok := status.FromError(err)
			if !ok {
				t.Errorf("unexpected error %v", err)
				return
			}
		})
	}
}
