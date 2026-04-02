package opentelemetry

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.k6.io/k6/internal/build"
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
				ServiceName:          null.NewString("k6", false),
				ServiceVersion:       null.NewString(build.Version, false),
				ExporterProtocol:     null.NewString(grpcExporterProtocol, false),
				HTTPExporterInsecure: null.NewBool(false, false),
				HTTPExporterEndpoint: null.NewString("localhost:4318", false),
				HTTPExporterURLPath:  null.NewString("/v1/metrics", false),
				GRPCExporterInsecure: null.NewBool(false, false),
				GRPCExporterEndpoint: null.NewString("localhost:4317", false),
				ExportInterval:       types.NewNullDuration(10*time.Second, false),
				FlushInterval:        types.NewNullDuration(1*time.Second, false),
				SingleCounterForRate: null.NewBool(true, false),
			},
		},

		"environment success merge": {
			env: map[string]string{"K6_OTEL_GRPC_EXPORTER_ENDPOINT": "else", "K6_OTEL_EXPORT_INTERVAL": "4ms"},
			expectedConfig: Config{
				ServiceName:          null.NewString("k6", false),
				ServiceVersion:       null.NewString(build.Version, false),
				ExporterProtocol:     null.NewString(grpcExporterProtocol, false),
				HTTPExporterInsecure: null.NewBool(false, false),
				HTTPExporterEndpoint: null.NewString("localhost:4318", false),
				HTTPExporterURLPath:  null.NewString("/v1/metrics", false),
				GRPCExporterInsecure: null.NewBool(false, false),
				GRPCExporterEndpoint: null.NewString("else", true),
				ExportInterval:       types.NewNullDuration(4*time.Millisecond, true),
				FlushInterval:        types.NewNullDuration(1*time.Second, false),
				SingleCounterForRate: null.NewBool(true, false),
			},
		},

		"environment complete overwrite": {
			env: map[string]string{
				"K6_OTEL_SERVICE_NAME":             "foo",
				"K6_OTEL_SERVICE_VERSION":          "v0.0.99",
				"K6_OTEL_EXPORTER_TYPE":            "http",
				"K6_OTEL_EXPORTER_PROTOCOL":        "http/protobuf",
				"K6_OTEL_EXPORT_INTERVAL":          "4ms",
				"K6_OTEL_HTTP_EXPORTER_INSECURE":   "true",
				"K6_OTEL_HTTP_EXPORTER_ENDPOINT":   "localhost:5555",
				"K6_OTEL_HTTP_EXPORTER_URL_PATH":   "/foo/bar",
				"K6_OTEL_GRPC_EXPORTER_INSECURE":   "true",
				"K6_OTEL_GRPC_EXPORTER_ENDPOINT":   "else",
				"K6_OTEL_FLUSH_INTERVAL":           "13s",
				"K6_OTEL_TLS_INSECURE_SKIP_VERIFY": "true",
				"K6_OTEL_TLS_CERTIFICATE":          "cert_path",
				"K6_OTEL_TLS_CLIENT_CERTIFICATE":   "client_cert_path",
				"K6_OTEL_TLS_CLIENT_KEY":           "client_key_path",
				"K6_OTEL_HEADERS":                  "key1=value1,key2=value2",
				"K6_OTEL_SINGLE_COUNTER_FOR_RATE":  "false",
			},
			expectedConfig: Config{
				ServiceName:           null.NewString("foo", true),
				ServiceVersion:        null.NewString("v0.0.99", true),
				ExporterType:          null.NewString(httpExporterType, true),
				ExporterProtocol:      null.NewString(httpExporterProtocol, true),
				ExportInterval:        types.NewNullDuration(4*time.Millisecond, true),
				HTTPExporterInsecure:  null.NewBool(true, true),
				HTTPExporterEndpoint:  null.NewString("localhost:5555", true),
				HTTPExporterURLPath:   null.NewString("/foo/bar", true),
				GRPCExporterInsecure:  null.NewBool(true, true),
				GRPCExporterEndpoint:  null.NewString("else", true),
				FlushInterval:         types.NewNullDuration(13*time.Second, true),
				TLSInsecureSkipVerify: null.NewBool(true, true),
				TLSCertificate:        null.NewString("cert_path", true),
				TLSClientCertificate:  null.NewString("client_cert_path", true),
				TLSClientKey:          null.NewString("client_key_path", true),
				Headers:               null.NewString("key1=value1,key2=value2", true),
				SingleCounterForRate:  null.NewBool(false, true),
			},
		},

		"OTEL environment variables": {
			env: map[string]string{
				"OTEL_SERVICE_NAME": "otel-service",
			},
			expectedConfig: Config{
				ServiceName:          null.NewString("otel-service", true),
				ServiceVersion:       null.NewString(build.Version, false),
				ExporterProtocol:     null.NewString(grpcExporterProtocol, false),
				HTTPExporterInsecure: null.NewBool(false, false),
				HTTPExporterEndpoint: null.NewString("localhost:4318", false),
				HTTPExporterURLPath:  null.NewString("/v1/metrics", false),
				GRPCExporterInsecure: null.NewBool(false, false),
				GRPCExporterEndpoint: null.NewString("localhost:4317", false),
				ExportInterval:       types.NewNullDuration(10*time.Second, false),
				FlushInterval:        types.NewNullDuration(1*time.Second, false),
				SingleCounterForRate: null.NewBool(true, false),
			},
		},

		"JSON complete overwrite": {
			jsonRaw: json.RawMessage(
				`{` +
					`"serviceName":"bar",` +
					`"serviceVersion":"v2.0.99",` +
					`"exporterType":"http",` +
					`"exporterProtocol":"http/protobuf",` +
					`"exportInterval":"15ms",` +
					`"httpExporterInsecure":true,` +
					`"httpExporterEndpoint":"localhost:5555",` +
					`"httpExporterURLPath":"/foo/bar",` +
					`"grpcExporterInsecure":true,` +
					`"grpcExporterEndpoint":"else",` +
					`"flushInterval":"13s",` +
					`"tlsInsecureSkipVerify":true,` +
					`"tlsCertificate":"cert_path",` +
					`"tlsClientCertificate":"client_cert_path",` +
					`"tlsClientKey":"client_key_path",` +
					`"headers":"key1=value1,key2=value2",` +
					`"singleCounterForRate":false` +
					`}`,
			),
			expectedConfig: Config{
				ServiceName:           null.NewString("bar", true),
				ServiceVersion:        null.NewString("v2.0.99", true),
				ExporterType:          null.NewString(httpExporterType, true),
				ExporterProtocol:      null.NewString(httpExporterProtocol, true),
				ExportInterval:        types.NewNullDuration(15*time.Millisecond, true),
				HTTPExporterInsecure:  null.NewBool(true, true),
				HTTPExporterEndpoint:  null.NewString("localhost:5555", true),
				HTTPExporterURLPath:   null.NewString("/foo/bar", true),
				GRPCExporterInsecure:  null.NewBool(true, true),
				GRPCExporterEndpoint:  null.NewString("else", true),
				FlushInterval:         types.NewNullDuration(13*time.Second, true),
				TLSInsecureSkipVerify: null.NewBool(true, true),
				TLSCertificate:        null.NewString("cert_path", true),
				TLSClientCertificate:  null.NewString("client_cert_path", true),
				TLSClientKey:          null.NewString("client_key_path", true),
				Headers:               null.NewString("key1=value1,key2=value2", true),
				SingleCounterForRate:  null.NewBool(false, true),
			},
		},

		"JSON success merge": {
			jsonRaw: json.RawMessage(`{"exporterType":"http","httpExporterEndpoint":"localhost:5566","httpExporterURLPath":"/lorem/ipsum","exportInterval":"15ms"}`),
			expectedConfig: Config{
				ServiceName:          null.NewString("k6", false),
				ServiceVersion:       null.NewString(build.Version, false),
				ExporterType:         null.NewString(httpExporterType, true),
				ExporterProtocol:     null.NewString(grpcExporterType, false),
				HTTPExporterInsecure: null.NewBool(false, false),
				HTTPExporterEndpoint: null.NewString("localhost:5566", true),
				HTTPExporterURLPath:  null.NewString("/lorem/ipsum", true),
				GRPCExporterInsecure: null.NewBool(false, false),              // default
				GRPCExporterEndpoint: null.NewString("localhost:4317", false), // default
				ExportInterval:       types.NewNullDuration(15*time.Millisecond, true),
				FlushInterval:        types.NewNullDuration(1*time.Second, false),
				SingleCounterForRate: null.NewBool(true, false),
			},
		},
		"no scheme in http exporter protocol": {
			jsonRaw: json.RawMessage(`{"exporterProtocol":"http/protobuf","httpExporterEndpoint":"http://localhost:5566","httpExporterURLPath":"/lorem/ipsum", "exportInterval":"15ms"}`),
			err:     `config: HTTP exporter endpoint must only be host and port, no scheme`,
		},

		"no scheme in http exporter type": {
			jsonRaw: json.RawMessage(`{"exporterType":"http","httpExporterEndpoint":"http://localhost:5566","httpExporterURLPath":"/lorem/ipsum", "exportInterval":"15ms"}`),
			err:     `config: HTTP exporter endpoint must only be host and port, no scheme`,
		},

		"early error env": {
			env: map[string]string{"K6_OTEL_GRPC_EXPORTER_ENDPOINT": "else", "K6_OTEL_EXPORT_INTERVAL": "4something"},
			err: `time: unknown unit "something" in duration "4something"`,
		},

		"early error JSON": {
			jsonRaw: json.RawMessage(`{"exportInterval":"4something"}`),
			err:     `time: unknown unit "something" in duration "4something"`,
		},

		"unsupported exporter type": {
			env: map[string]string{"K6_OTEL_GRPC_EXPORTER_ENDPOINT": "else", "K6_OTEL_EXPORT_INTERVAL": "4m", "K6_OTEL_EXPORTER_TYPE": "socket"},
			err: `error validating OpenTelemetry output config: unsupported exporter type "socket", only "grpc" and "http" are supported`,
		},

		"unsupported exporter protocol": {
			env: map[string]string{"K6_OTEL_GRPC_EXPORTER_ENDPOINT": "else", "K6_OTEL_EXPORT_INTERVAL": "4m", "K6_OTEL_EXPORTER_PROTOCOL": "socket"},
			err: `error validating OpenTelemetry output config: unsupported exporter protocol "socket", only "grpc" and "http/protobuf" are supported`,
		},

		"missing required": {
			jsonRaw: json.RawMessage(`{"exporterType":"http","httpExporterEndpoint":"","httpExporterURLPath":"/lorem/ipsum"}`),
			err:     `HTTP exporter endpoint is required`,
		},
	}

	for name, testCase := range testCases {
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

func TestConfigString(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		config   Config
		expected string
	}{
		"default grpc": {
			config: Config{
				ExporterProtocol:     null.NewString(grpcExporterProtocol, false),
				GRPCExporterEndpoint: null.NewString("localhost:4317", false),
				GRPCExporterInsecure: null.NewBool(false, false),
			},
			expected: "grpc, localhost:4317",
		},
		"grpc with ExporterProtocol set": {
			config: Config{
				ExporterProtocol:     null.NewString(grpcExporterProtocol, true),
				GRPCExporterEndpoint: null.NewString("localhost:4317", true),
				GRPCExporterInsecure: null.NewBool(false, true),
			},
			expected: "grpc, localhost:4317",
		},
		"grpc insecure with ExporterProtocol set": {
			config: Config{
				ExporterProtocol:     null.NewString(grpcExporterProtocol, true),
				GRPCExporterEndpoint: null.NewString("localhost:4317", true),
				GRPCExporterInsecure: null.NewBool(true, true),
			},
			expected: "grpc (insecure), localhost:4317",
		},
		"http/protobuf with ExporterProtocol set": {
			config: Config{
				ExporterProtocol:     null.NewString(httpExporterProtocol, true),
				HTTPExporterEndpoint: null.NewString("localhost:4318", true),
				HTTPExporterURLPath:  null.NewString("/v1/metrics", true),
				HTTPExporterInsecure: null.NewBool(false, true),
			},
			expected: "http/protobuf, https://localhost:4318/v1/metrics",
		},
		"http/protobuf insecure with ExporterProtocol set": {
			config: Config{
				ExporterProtocol:     null.NewString(httpExporterProtocol, true),
				HTTPExporterEndpoint: null.NewString("localhost:4318", true),
				HTTPExporterURLPath:  null.NewString("/v1/metrics", true),
				HTTPExporterInsecure: null.NewBool(true, true),
			},
			expected: "http/protobuf, http://localhost:4318/v1/metrics",
		},
		"deprecated ExporterType http": {
			config: Config{
				ExporterType:         null.NewString(httpExporterType, true),
				HTTPExporterEndpoint: null.NewString("localhost:4318", true),
				HTTPExporterURLPath:  null.NewString("/v1/metrics", true),
				HTTPExporterInsecure: null.NewBool(false, true),
			},
			expected: "http/protobuf, https://localhost:4318/v1/metrics",
		},
		"deprecated ExporterType grpc": {
			config: Config{
				ExporterType:         null.NewString(grpcExporterType, true),
				GRPCExporterEndpoint: null.NewString("localhost:4317", true),
				GRPCExporterInsecure: null.NewBool(false, true),
			},
			expected: "grpc, localhost:4317",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			result := testCase.config.String()
			require.Equal(t, testCase.expected, result)
		})
	}
}
