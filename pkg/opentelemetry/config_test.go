package opentelemetry

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	k6Const "go.k6.io/k6/lib/consts"
	"go.k6.io/k6/lib/types"
	"gopkg.in/guregu/null.v3"
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
				ServiceName:          null.StringFrom("k6"),
				ServiceVersion:       null.StringFrom(k6Const.Version),
				ExporterType:         null.StringFrom(grpcExporterType),
				HTTPExporterInsecure: null.NewBool(false, true),
				HTTPExporterEndpoint: null.StringFrom("localhost:4318"),
				HTTPExporterURLPath:  null.StringFrom("/v1/metrics"),
				GRPCExporterInsecure: null.NewBool(false, true),
				GRPCExporterEndpoint: null.StringFrom("localhost:4317"),
				ExportInterval:       types.NullDurationFrom(1 * time.Second),
				FlushInterval:        types.NullDurationFrom(1 * time.Second),
			},
		},

		"environment success overwrite": {
			env: map[string]string{"K6_OTEL_GRPC_EXPORTER_ENDPOINT": "else", "K6_OTEL_EXPORT_INTERVAL": "4ms"},
			expectedConfig: Config{
				ServiceName:          null.StringFrom("k6"),
				ServiceVersion:       null.StringFrom(k6Const.Version),
				ExporterType:         null.StringFrom(grpcExporterType),
				HTTPExporterInsecure: null.NewBool(false, true),
				HTTPExporterEndpoint: null.StringFrom("localhost:4318"),
				HTTPExporterURLPath:  null.StringFrom("/v1/metrics"),
				GRPCExporterInsecure: null.NewBool(false, true),
				GRPCExporterEndpoint: null.StringFrom("else"),
				ExportInterval:       types.NullDurationFrom(4 * time.Millisecond),
				FlushInterval:        types.NullDurationFrom(1 * time.Second),
			},
		},

		"JSON success overwrite": {
			jsonRaw: json.RawMessage(`{"exporterType":"http","httpExporterEndpoint":"http://localhost:5566","httpExporterURLPath":"/lorem/ipsum", "exportInterval":"15ms"}`),
			expectedConfig: Config{
				ServiceName:          null.StringFrom("k6"),
				ServiceVersion:       null.StringFrom(k6Const.Version),
				ExporterType:         null.StringFrom(httpExporterType),
				HTTPExporterInsecure: null.NewBool(false, true),
				HTTPExporterEndpoint: null.StringFrom("http://localhost:5566"),
				HTTPExporterURLPath:  null.StringFrom("/lorem/ipsum"),
				GRPCExporterInsecure: null.NewBool(false, true),         // default
				GRPCExporterEndpoint: null.StringFrom("localhost:4317"), // default
				ExportInterval:       types.NullDurationFrom(15 * time.Millisecond),
				FlushInterval:        types.NullDurationFrom(1 * time.Second),
			},
		},

		"early error env": {
			env: map[string]string{"K6_OTEL_GRPC_EXPORTER_ENDPOINT": "else", "K6_OTEL_EXPORT_INTERVAL": "4something"},
			err: `time: unknown unit "something" in duration "4something"`,
		},

		"early error JSON": {
			jsonRaw: json.RawMessage(`{"exportInterval":"4something"}`),
			err:     `time: unknown unit "something" in duration "4something"`,
		},

		"unsupported receiver type": {
			env: map[string]string{"K6_OTEL_GRPC_EXPORTER_ENDPOINT": "else", "K6_OTEL_EXPORT_INTERVAL": "4m", "K6_OTEL_EXPORTER_TYPE": "socket"},
			err: `error validating OpenTelemetry output config: unsupported exporter type "socket", currently only "grpc" and "http" are supported`,
		},

		"missing required": {
			jsonRaw: json.RawMessage(`{"exporterType":"http","httpExporterEndpoint":"","httpExporterURLPath":"/lorem/ipsum"}`),
			err:     `HTTP exporter endpoint is required`,
		},
	}

	for name, testCase := range testCases {
		testCase := testCase
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			config, err := GetConsolidatedConfig(testCase.jsonRaw, testCase.env)
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
