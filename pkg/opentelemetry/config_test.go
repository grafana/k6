package opentelemetry

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.k6.io/k6/output"
)

func TestConfig(t *testing.T) {
	t.Parallel()
	// TODO: add more cases
	testCases := map[string]struct {
		jsonRaw        json.RawMessage
		env            map[string]string
		arg            string
		expectedConfig Config
		err            string
	}{
		"default": {
			expectedConfig: Config{
				ReceiverType:         grpcReceiverType,
				GRPCReceiverEndpoint: "localhost:4317",
				PushInterval:         1 * time.Second,
				FlushInterval:        1 * time.Second,
			},
		},

		"overwrite": {
			env: map[string]string{"K6_OTEL_GRPC_RECEIVER_ENDPOINT": "else", "K6_OTEL_PUSH_INTERVAL": "4ms"},
			expectedConfig: Config{
				ReceiverType:         grpcReceiverType,
				GRPCReceiverEndpoint: "else",
				PushInterval:         4 * time.Millisecond,
				FlushInterval:        1 * time.Second,
			},
		},

		"early error": {
			env: map[string]string{"K6_OTEL_GRPC_RECEIVER_ENDPOINT": "else", "K6_OTEL_PUSH_INTERVAL": "4something"},
			err: `time: unknown unit "something" in duration "4something"`,
		},

		"unsupported receiver type": {
			env: map[string]string{"K6_OTEL_GRPC_RECEIVER_ENDPOINT": "else", "K6_OTEL_PUSH_INTERVAL": "4m", "K6_OTEL_RECEIVER_TYPE": "http"},
			err: `error validating config: unsupported receiver type "http", currently only "grpc" supported`,
		},
	}

	for name, testCase := range testCases {
		testCase := testCase
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			config, err := NewConfig(output.Params{Environment: testCase.env})
			if testCase.err != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), testCase.err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, testCase.expectedConfig, config)
		})
	}
}
