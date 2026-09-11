package opentelemetry

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"
)

// TestGetExporterBasicAuth verifies that configuring HTTP Basic Auth via
// K6_OTEL_HTTP_EXPORTER_USERNAME/PASSWORD does not panic when no headers are
// configured through K6_OTEL_HEADERS. Previously the headers map was nil and
// assigning the Authorization header caused a "assignment to entry in nil map"
// panic.
func TestGetExporterBasicAuth(t *testing.T) {
	t.Parallel()

	httpBase := func() Config {
		return Config{
			ExporterProtocol:     null.StringFrom(httpExporterProtocol),
			HTTPExporterEndpoint: null.StringFrom("localhost:4318"),
			HTTPExporterURLPath:  null.StringFrom("/v1/metrics"),
		}
	}

	grpcBase := func() Config {
		return Config{
			ExporterProtocol:     null.StringFrom(grpcExporterProtocol),
			GRPCExporterEndpoint: null.StringFrom("localhost:4317"),
		}
	}

	testCases := map[string]struct {
		cfg func() Config
	}{
		"http basic auth without headers": {
			cfg: func() Config {
				cfg := httpBase()
				cfg.HTTPUsername = null.StringFrom("user")
				cfg.HTTPPassword = null.StringFrom("pass")
				return cfg
			},
		},
		"http basic auth with headers": {
			cfg: func() Config {
				cfg := httpBase()
				cfg.HTTPUsername = null.StringFrom("user")
				cfg.HTTPPassword = null.StringFrom("pass")
				cfg.Headers = null.StringFrom("X-Scope-OrgID=test")
				return cfg
			},
		},
		"http headers only": {
			cfg: func() Config {
				cfg := httpBase()
				cfg.Headers = null.StringFrom("X-Scope-OrgID=test")
				return cfg
			},
		},
		"http no auth no headers": {
			cfg: httpBase,
		},
		"grpc basic auth without headers": {
			cfg: func() Config {
				cfg := grpcBase()
				cfg.HTTPUsername = null.StringFrom("user")
				cfg.HTTPPassword = null.StringFrom("pass")
				return cfg
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			exporter, err := getExporter(tc.cfg())
			require.NoError(t, err)
			require.NotNil(t, exporter)
		})
	}
}
