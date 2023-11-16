package trace

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTracerProviderParamsFromConfigLine(t *testing.T) {
	t.Parallel()

	testCases := [...]struct {
		name      string
		line      string
		expParams tracerProviderParams
		expErr    error
	}{
		{
			name:      "default otel",
			line:      "otel",
			expParams: defaultTracerProviderParams(),
		},
		{
			name: "otel with insecure url and custom path",
			line: "otel=http://localhost:4444/custom/traces",
			expParams: tracerProviderParams{
				proto:    "http",
				endpoint: "localhost:4444",
				urlPath:  "/custom/traces",
				insecure: true,
				headers:  make(map[string]string),
			},
		},
		{
			name: "otel with secure url and no port",
			line: "otel=https://localhost",
			expParams: tracerProviderParams{
				proto:    "http",
				endpoint: "localhost",
				headers:  make(map[string]string),
			},
		},
		{
			name: "otel with grpc proto",
			line: "otel=https://localhost,proto=grpc",
			expParams: tracerProviderParams{
				proto:    "grpc",
				endpoint: "localhost",
				headers:  make(map[string]string),
			},
		},
		{
			name: "otel with headers",
			line: "otel=https://localhost,header.Authorization=token ***,header.other=test",
			expParams: tracerProviderParams{
				proto:    "http",
				endpoint: "localhost",
				headers: map[string]string{
					"Authorization": "token ***",
					"other":         "test",
				},
			},
		},
		{
			name:   "error invalid output",
			line:   "invalid",
			expErr: ErrInvalidTracesOutput,
		},
		{
			name:   "error invalid scheme",
			line:   "otel=invalid://localhost:4444/traces",
			expErr: ErrInvalidURLScheme,
		},
		{
			name:   "error invalid proto",
			line:   "otel=http://localhost:4444,proto=invalid",
			expErr: ErrInvalidProto,
		},
		{
			name:   "error invalid grpc proto with URL path",
			line:   "otel=http://localhost:4444/url/path,proto=grpc",
			expErr: ErrInvalidGRPCWithURLPath,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			params, err := tracerProviderParamsFromConfigLine(tc.line)
			if err != nil {
				require.ErrorIs(t, err, tc.expErr)
				return
			}
			require.Equal(t, tc.expParams, params)
		})
	}
}
