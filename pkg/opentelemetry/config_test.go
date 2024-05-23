package opentelemetry

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/output"

	k6Const "go.k6.io/k6/lib/consts"
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
				ServiceName:          "k6",
				ServiceVersion:       k6Const.Version,
				ExporterType:         grpcExporterType,
				HTTPExporterInsecure: null.NewBool(false, true),
				HTTPExporterEndpoint: "localhost:4318",
				HTTPExporterURLPath:  "/v1/metrics",
				GRPCExporterInsecure: null.NewBool(false, true),
				GRPCExporterEndpoint: "localhost:4317",
				ExportInterval:       1 * time.Second,
				FlushInterval:        1 * time.Second,
			},
		},

		"overwrite": {
			env: map[string]string{"K6_OTEL_GRPC_EXPORTER_ENDPOINT": "else", "K6_OTEL_EXPORT_INTERVAL": "4ms"},
			expectedConfig: Config{
				ServiceName:          "k6",
				ServiceVersion:       k6Const.Version,
				ExporterType:         grpcExporterType,
				HTTPExporterInsecure: null.NewBool(false, true),
				HTTPExporterEndpoint: "localhost:4318",
				HTTPExporterURLPath:  "/v1/metrics",
				GRPCExporterInsecure: null.NewBool(false, true),
				GRPCExporterEndpoint: "else",
				ExportInterval:       4 * time.Millisecond,
				FlushInterval:        1 * time.Second,
			},
		},

		"early error": {
			env: map[string]string{"K6_OTEL_GRPC_EXPORTER_ENDPOINT": "else", "K6_OTEL_EXPORT_INTERVAL": "4something"},
			err: `time: unknown unit "something" in duration "4something"`,
		},

		"unsupported receiver type": {
			env: map[string]string{"K6_OTEL_GRPC_EXPORTER_ENDPOINT": "else", "K6_OTEL_EXPORT_INTERVAL": "4m", "K6_OTEL_EXPORTER_TYPE": "socket"},
			err: `error validating OpenTelemetry output config: unsupported exporter type "socket", currently only "grpc" and "http" are supported`,
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
