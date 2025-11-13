package opentelemetry

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mstoykov/envconfig"
	"github.com/sirupsen/logrus"
	"go.k6.io/k6/errext"
	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/internal/build"
	"go.k6.io/k6/lib/types"
	"gopkg.in/guregu/null.v3"
)

const (
	// grpcExporterType GRPC exporter type
	// Deprecated: use grpcExporterProtocol
	grpcExporterType = "grpc"
	// httpExporterType HTTP exporter type
	// Deprecated: use httpExporterProtocol
	httpExporterType = "http"

	// grpcExporterProtocol GRPC exporter type
	grpcExporterProtocol = "grpc"
	// httpExporterProtocol HTTP exporter type
	httpExporterProtocol = "http/protobuf"
)

// Config represents the configuration for the OpenTelemetry output
type Config struct {
	// ServiceName is the name of the service to use for the metrics
	// export, if not set it will use "k6"
	ServiceName null.String `json:"serviceName" envconfig:"K6_OTEL_SERVICE_NAME"`
	// ServiceVersion is the version of the service to use for the metrics
	// export, if not set it will use k6's library version
	ServiceVersion null.String `json:"serviceVersion" envconfig:"K6_OTEL_SERVICE_VERSION"`
	// MetricPrefix is the prefix to use for the metrics
	MetricPrefix null.String `json:"metricPrefix" envconfig:"K6_OTEL_METRIC_PREFIX"`
	// FlushInterval is the interval at which to flush metrics from the k6
	FlushInterval types.NullDuration `json:"flushInterval" envconfig:"K6_OTEL_FLUSH_INTERVAL"`

	// ExporterType sets the type of OpenTelemetry Exporter to use
	// Deprecated: use ExporterProtocol
	ExporterType null.String `json:"exporterType" envconfig:"K6_OTEL_EXPORTER_TYPE"`
	// ExporterProtocol sets the protocol of OpenTelemetry Exporter to use
	ExporterProtocol null.String `json:"exporterProtocol" envconfig:"K6_OTEL_EXPORTER_PROTOCOL"`
	// ExportInterval configures the intervening time between metrics exports
	ExportInterval types.NullDuration `json:"exportInterval" envconfig:"K6_OTEL_EXPORT_INTERVAL"`

	// Headers in W3C Correlation-Context format without additional semicolon-delimited metadata (i.e. "k1=v1,k2=v2")
	Headers null.String `json:"headers" envconfig:"K6_OTEL_HEADERS"`

	// TLSInsecureSkipVerify disables verification of the server's certificate chain
	TLSInsecureSkipVerify null.Bool `json:"tlsInsecureSkipVerify" envconfig:"K6_OTEL_TLS_INSECURE_SKIP_VERIFY"`
	// TLSCertificate is the path to the certificate file (rootCAs) to use for the exporter's TLS connection
	TLSCertificate null.String `json:"tlsCertificate" envconfig:"K6_OTEL_TLS_CERTIFICATE"`
	// TLSClientCertificate is the path to the certificate file (must be PEM encoded data)
	// to use for the exporter's TLS connection
	TLSClientCertificate null.String `json:"tlsClientCertificate" envconfig:"K6_OTEL_TLS_CLIENT_CERTIFICATE"`
	// TLSClientKey is the path to the private key file (must be PEM encoded data) to use for the exporter's TLS connection
	TLSClientKey null.String `json:"tlsClientKey" envconfig:"K6_OTEL_TLS_CLIENT_KEY"`

	// HTTPExporterInsecure disables client transport security for the Exporter's HTTP
	// connection.
	HTTPExporterInsecure null.Bool `json:"httpExporterInsecure" envconfig:"K6_OTEL_HTTP_EXPORTER_INSECURE"`
	// HTTPExporterEndpoint sets the target endpoint the OpenTelemetry Exporter
	// will connect to.
	HTTPExporterEndpoint null.String `json:"httpExporterEndpoint" envconfig:"K6_OTEL_HTTP_EXPORTER_ENDPOINT"`
	// HTTPExporterURLPath sets the target URL path the OpenTelemetry Exporter
	HTTPExporterURLPath null.String `json:"httpExporterURLPath" envconfig:"K6_OTEL_HTTP_EXPORTER_URL_PATH"`

	// GRPCExporterEndpoint sets the target endpoint the OpenTelemetry Exporter
	// will connect to.
	GRPCExporterEndpoint null.String `json:"grpcExporterEndpoint" envconfig:"K6_OTEL_GRPC_EXPORTER_ENDPOINT"`
	// GRPCExporterInsecure disables client transport security for the Exporter's gRPC
	// connection.
	GRPCExporterInsecure null.Bool `json:"grpcExporterInsecure" envconfig:"K6_OTEL_GRPC_EXPORTER_INSECURE"`

	// SingleCounterForRate sets the feature flag defining how to export metrics defined as Rate type.
	// When it is set to true, metrics are exported as a single counter, using an attribute as discriminator.
	// When the opposite, the old method is used generating two different counters.
	SingleCounterForRate null.Bool `json:"singleCounterForRate" envconfig:"K6_OTEL_SINGLE_COUNTER_FOR_RATE"`
}

// GetConsolidatedConfig combines the options' values from the different sources
// and returns the merged options. The Order of precedence used is documented
// in the k6 Documentation https://grafana.com/docs/k6/latest/using-k6/k6-options/how-to/#order-of-precedence.
func GetConsolidatedConfig(
	jsonRawConf json.RawMessage,
	env map[string]string,
	logger logrus.FieldLogger,
) (Config, error) {
	// We start from the defaults.
	cfg := newDefaultConfig()

	// Then, we apply the OTLP exporter environment variables. So, these are used as the "defaults", if defined,
	// while the k6-specific configuration options / environment variables remain with higher precedence, as follows.
	cfg, err := applyOTELEnvVars(cfg, env)
	if err != nil {
		return cfg, fmt.Errorf("parse standard OTEL environment variables options failed: %w", err)
	}

	if jsonRawConf != nil {
		jsonConf, err := parseJSON(jsonRawConf)
		if err != nil {
			return cfg, fmt.Errorf("parse JSON options failed: %w", err)
		}
		warnIfConfigMismatch(jsonConf, logger)
		cfg = cfg.Apply(jsonConf)
	}

	if len(env) > 0 {
		envConf, err := parseEnvs(env)
		if err != nil {
			return cfg, fmt.Errorf("parse environment variables options failed: %w", err)
		}
		warnIfConfigMismatch(envConf, logger)
		cfg = cfg.Apply(envConf)
	}

	if err := cfg.Validate(); err != nil {
		return cfg, errext.WithExitCodeIfNone(
			fmt.Errorf("error validating OpenTelemetry output config: %w", err),
			exitcodes.InvalidConfig,
		)
	}

	return cfg, nil
}

// newDefaultConfig creates a new default config with default values
func newDefaultConfig() Config {
	return Config{
		ServiceName:      null.NewString("k6", false),
		ServiceVersion:   null.NewString(build.Version, false),
		ExporterProtocol: null.NewString(grpcExporterProtocol, false),

		HTTPExporterInsecure: null.NewBool(false, false),
		HTTPExporterEndpoint: null.NewString("localhost:4318", false),
		HTTPExporterURLPath:  null.NewString("/v1/metrics", false),

		GRPCExporterInsecure: null.NewBool(false, false),
		GRPCExporterEndpoint: null.NewString("localhost:4317", false),

		ExportInterval: types.NewNullDuration(10*time.Second, false),
		FlushInterval:  types.NewNullDuration(1*time.Second, false),

		SingleCounterForRate: null.NewBool(true, false),
	}
}

// Apply applies the new config to the existing one
func (cfg Config) Apply(v Config) Config {
	if v.ServiceName.Valid {
		cfg.ServiceName = v.ServiceName
	}

	if v.ServiceVersion.Valid {
		cfg.ServiceVersion = v.ServiceVersion
	}

	if v.MetricPrefix.Valid {
		cfg.MetricPrefix = v.MetricPrefix
	}

	if v.FlushInterval.Valid {
		cfg.FlushInterval = v.FlushInterval
	}

	if v.ExporterProtocol.Valid {
		cfg.ExporterProtocol = v.ExporterProtocol
	}

	if v.ExporterType.Valid {
		cfg.ExporterType = v.ExporterType
	}

	if v.ExportInterval.Valid {
		cfg.ExportInterval = v.ExportInterval
	}

	if v.HTTPExporterInsecure.Valid {
		cfg.HTTPExporterInsecure = v.HTTPExporterInsecure
	}

	if v.HTTPExporterEndpoint.Valid {
		cfg.HTTPExporterEndpoint = v.HTTPExporterEndpoint
	}

	if v.HTTPExporterURLPath.Valid {
		cfg.HTTPExporterURLPath = v.HTTPExporterURLPath
	}

	if v.GRPCExporterEndpoint.Valid {
		cfg.GRPCExporterEndpoint = v.GRPCExporterEndpoint
	}

	if v.GRPCExporterInsecure.Valid {
		cfg.GRPCExporterInsecure = v.GRPCExporterInsecure
	}

	if v.TLSInsecureSkipVerify.Valid {
		cfg.TLSInsecureSkipVerify = v.TLSInsecureSkipVerify
	}

	if v.TLSCertificate.Valid {
		cfg.TLSCertificate = v.TLSCertificate
	}

	if v.TLSClientCertificate.Valid {
		cfg.TLSClientCertificate = v.TLSClientCertificate
	}

	if v.TLSClientKey.Valid {
		cfg.TLSClientKey = v.TLSClientKey
	}

	if v.Headers.Valid {
		cfg.Headers = v.Headers
	}

	if v.SingleCounterForRate.Valid {
		cfg.SingleCounterForRate = v.SingleCounterForRate
	}

	return cfg
}

// Validate validates the config
func (cfg Config) Validate() error {
	if cfg.ServiceName.String == "" {
		return errors.New("providing service name is required")
	}
	// TODO: it's not actually required, but we should probably have a default
	// check if it works without it
	if cfg.ServiceVersion.String == "" {
		return errors.New("providing service version is required")
	}
	if err := cfg.validateExporterProtocol(); err != nil {
		return err
	}
	if err := cfg.validateExporterType(); err != nil {
		return err
	}
	return nil
}

func (cfg Config) validateExporterType() error {
	if cfg.ExporterType.String != "" {
		if cfg.ExporterType.String != httpExporterType && cfg.ExporterType.String != grpcExporterType {
			return fmt.Errorf(
				"unsupported exporter type %q, only %q and %q are supported",
				cfg.ExporterType.String,
				grpcExporterType,
				httpExporterType,
			)
		}
		switch cfg.ExporterType.String {
		case grpcExporterType:
			if cfg.GRPCExporterEndpoint.String == "" {
				return errors.New("gRPC exporter endpoint is required")
			}
		case httpExporterType:
			endpoint := cfg.HTTPExporterEndpoint.String
			if endpoint == "" {
				return errors.New("HTTP exporter endpoint is required")
			}

			if strings.HasPrefix(endpoint, "http://") ||
				strings.HasPrefix(endpoint, "https://") {
				return errors.New("HTTP exporter endpoint must only be host and port, no scheme")
			}
		}
	}
	return nil
}

func (cfg Config) validateExporterProtocol() error {
	if cfg.ExporterProtocol.String != "" {
		if cfg.ExporterProtocol.String != grpcExporterProtocol && cfg.ExporterProtocol.String != httpExporterProtocol {
			return fmt.Errorf(
				"unsupported exporter protocol %q, only %q and %q are supported",
				cfg.ExporterProtocol.String,
				grpcExporterProtocol,
				httpExporterProtocol,
			)
		}
		switch cfg.ExporterProtocol.String {
		case grpcExporterProtocol:
			if cfg.GRPCExporterEndpoint.String == "" {
				return errors.New("gRPC exporter endpoint is required")
			}
		case httpExporterProtocol:
			endpoint := cfg.HTTPExporterEndpoint.String
			if endpoint == "" {
				return errors.New("HTTP exporter endpoint is required")
			}

			if strings.HasPrefix(endpoint, "http://") ||
				strings.HasPrefix(endpoint, "https://") {
				return errors.New("HTTP exporter endpoint must only be host and port, no scheme")
			}
		}
	}
	return nil
}

// String returns a string representation of the config
func (cfg Config) String() string {
	var endpoint string
	exporter := cfg.ExporterType.String

	if cfg.ExporterType.String == httpExporterType {
		endpoint = "http"
		if !cfg.HTTPExporterInsecure.Bool {
			endpoint += "s"
		}

		endpoint += "://" + cfg.HTTPExporterEndpoint.String + cfg.HTTPExporterURLPath.String
	} else {
		endpoint = cfg.GRPCExporterEndpoint.String

		if cfg.GRPCExporterInsecure.Bool {
			exporter += " (insecure)"
		}
	}

	return fmt.Sprintf("%s, %s", exporter, endpoint)
}

// parseJSON parses the supplied JSON into a Config.
func parseJSON(data json.RawMessage) (Config, error) {
	var c Config
	err := json.Unmarshal(data, &c)
	return c, err
}

// parseEnvs parses the supplied environment variables into a Config.
func parseEnvs(env map[string]string) (Config, error) {
	cfg := Config{}

	err := envconfig.Process("K6_OTEL_", &cfg, func(key string) (string, bool) {
		v, ok := env[key]
		return v, ok
	})

	return cfg, err
}

// applyOTELEnvVars applies the OTLP exporter environment variables, if defined, to the supplied Config.
// As per OTLP Exporter configuration specification, signal-specific variables (e.g.,
// OTEL_EXPORTER_OTLP_METRICS_PROTOCOL) take precedence over general ones (e.g., OTEL_EXPORTER_OTLP_PROTOCOL).
//
//nolint:gocognit,cyclop,funlen
func applyOTELEnvVars(defaultCfg Config, env map[string]string) (Config, error) {
	stdCfg := Config{}

	if serviceName, ok := env["OTEL_SERVICE_NAME"]; ok {
		stdCfg.ServiceName = null.StringFrom(serviceName)
	}

	if exporterProtocol, ok := env["OTEL_EXPORTER_OTLP_METRICS_PROTOCOL"]; ok {
		stdCfg.ExporterProtocol = null.StringFrom(exporterProtocol)
	} else if exporterProtocol, ok := env["OTEL_EXPORTER_OTLP_PROTOCOL"]; ok {
		stdCfg.ExporterProtocol = null.StringFrom(exporterProtocol)
	}

	if exportInterval, ok := env["OTEL_METRIC_EXPORT_INTERVAL"]; ok {
		exportIntervalDuration, err := types.ParseExtendedDuration(exportInterval)
		if err != nil {
			return Config{}, err
		}
		stdCfg.ExportInterval = types.NullDurationFrom(exportIntervalDuration)
	}

	if exporterHeaders, ok := env["OTEL_EXPORTER_OTLP_METRICS_HEADERS"]; ok {
		stdCfg.Headers = null.StringFrom(exporterHeaders)
	} else if exporterHeaders, ok := env["OTEL_EXPORTER_OTLP_HEADERS"]; ok {
		stdCfg.Headers = null.StringFrom(exporterHeaders)
	}

	if exporterCertificate, ok := env["OTEL_EXPORTER_OTLP_METRICS_CERTIFICATE"]; ok {
		stdCfg.TLSCertificate = null.StringFrom(exporterCertificate)
	} else if exporterCertificate, ok := env["OTEL_EXPORTER_OTLP_CERTIFICATE"]; ok {
		stdCfg.TLSCertificate = null.StringFrom(exporterCertificate)
	}

	if exporterClientCertificate, ok := env["OTEL_EXPORTER_OTLP_METRICS_CLIENT_CERTIFICATE"]; ok {
		stdCfg.TLSClientCertificate = null.StringFrom(exporterClientCertificate)
	} else if exporterClientCertificate, ok := env["OTEL_EXPORTER_OTLP_CLIENT_CERTIFICATE"]; ok {
		stdCfg.TLSClientCertificate = null.StringFrom(exporterClientCertificate)
	}

	if exporterClientKey, ok := env["OTEL_EXPORTER_OTLP_METRICS_CLIENT_KEY"]; ok {
		stdCfg.TLSClientKey = null.StringFrom(exporterClientKey)
	} else if exporterClientKey, ok := env["OTEL_EXPORTER_OTLP_CLIENT_KEY"]; ok {
		stdCfg.TLSClientKey = null.StringFrom(exporterClientKey)
	}

	isHTTP := stdCfg.ExporterProtocol.Valid && stdCfg.ExporterProtocol.String == httpExporterProtocol

	var exporterInsecureBoolVar string
	if exporterInsecure, ok := env["OTEL_EXPORTER_OTLP_METRICS_INSECURE"]; ok {
		exporterInsecureBoolVar = exporterInsecure
	} else if exporterInsecure, ok := env["OTEL_EXPORTER_OTLP_INSECURE"]; ok {
		exporterInsecureBoolVar = exporterInsecure
	}

	if exporterInsecureBoolVar != "" {
		exporterInsecureBool, err := strconv.ParseBool(exporterInsecureBoolVar)
		if err != nil {
			return Config{}, err
		}

		if isHTTP {
			stdCfg.HTTPExporterInsecure = null.BoolFrom(exporterInsecureBool)
		} else {
			stdCfg.GRPCExporterInsecure = null.BoolFrom(exporterInsecureBool)
		}
	}

	var (
		exporterEndpointVar    string
		exporterEndpointString string
	)
	if exporterEndpointVal, ok := env["OTEL_EXPORTER_OTLP_METRICS_ENDPOINT"]; ok {
		exporterEndpointVar = "OTEL_EXPORTER_OTLP_METRICS_ENDPOINT"
		exporterEndpointString = exporterEndpointVal
	} else if exporterEndpointVal, ok := env["OTEL_EXPORTER_OTLP_ENDPOINT"]; ok {
		exporterEndpointVar = "OTEL_EXPORTER_OTLP_ENDPOINT"
		exporterEndpointString = exporterEndpointVal
	}

	if exporterEndpointString != "" {
		exporterEndpoint, insecure, err := parseOTELEndpoint(exporterEndpointString, isHTTP)
		if err != nil {
			return Config{}, fmt.Errorf("failed to parse %s: %w", exporterEndpointVar, err)
		}

		if isHTTP {
			stdCfg.HTTPExporterEndpoint = exporterEndpoint
			stdCfg.HTTPExporterInsecure = insecure
		} else {
			stdCfg.GRPCExporterEndpoint = exporterEndpoint
		}

		if !isHTTP && insecure.Valid {
			stdCfg.GRPCExporterInsecure = insecure
		}
	}

	return defaultCfg.Apply(stdCfg), nil
}

// parseOTELEndpoint tries to parse the given string as an OTEL endpoint URL, extracts the endpoint and whether the
// connection should be secure or not. It honors what's said in the OTEL exporter docs:
// https://opentelemetry.io/docs/specs/otel/protocol/exporter/.
//
// IMPORTANT: Meant to be used only for parsing OTEL_EXPORTER_OTLP_ENDPOINT-like environment variables, not for k6
// configuration options, as Config.HTTPExporterEndpoint, for instance, doesn't accept a scheme on the URLs, while
// the OTEL exporter configuration requires it (for HTTP) or accepts it (for gRPC).
func parseOTELEndpoint(endpoint string, isHTTP bool) (null.String, null.Bool, error) {
	isHTTPScheme := strings.HasPrefix(endpoint, "http://")
	isHTTPSScheme := strings.HasPrefix(endpoint, "https://")

	// For HTTP/S, the scheme is required. For gRPC, it's optional.
	if !isHTTPScheme && !isHTTPSScheme {
		if isHTTP {
			return null.String{}, null.Bool{}, errors.New("endpoint must contain the scheme (http or https)")
		}
		// gRPC accepts any form allowed by the underlying client, so use as-is.
		return null.StringFrom(endpoint), null.Bool{}, nil
	}

	// Parse the URL to extract components.
	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return null.String{}, null.Bool{}, err
	}

	host := parsedURL.Host
	port := parsedURL.Port()

	if host == "" {
		return null.String{}, null.Bool{}, errors.New("endpoint must contain a host")
	}

	// HTTP requires a port, gRPC doesn't.
	if isHTTP && port == "" {
		return null.String{}, null.Bool{}, errors.New("endpoint must contain host and port")
	}

	// From OTEL docs: A scheme of "https" indicates a secure connection and takes precedence over the insecure
	// configuration setting. The same applies to "http", but indicating insecure connection.
	var insecure null.Bool
	if isHTTPSScheme {
		insecure = null.BoolFrom(false)
	} else {
		insecure = null.BoolFrom(true)
	}

	return null.StringFrom(host), insecure, nil
}

// warnIfConfigMismatch writes a warning log message in case of discrepancies between the configured
// Config.ExporterProtocol and other configuration attributes. E.g., in case the user explicitly configured
// Config.HTTPExporterEndpoint while the chosen protocol is `grpc`.
func warnIfConfigMismatch(cfg Config, logger logrus.FieldLogger) {
	exporterProtocol := cfg.ExporterProtocol.String
	switch {
	case exporterProtocol == grpcExporterProtocol && anyHTTPOptionSet(cfg):
		logger.Warn("Configuration mismatch detected: the gRPC exporter type is set, but also some HTTP " +
			"configuration options")
	case exporterProtocol == httpExporterProtocol && anyGRPCOptionSet(cfg):
		logger.Warn("Configuration mismatch detected: the HTTP exporter type is set, but also some gRPC " +
			"configuration options")
	}
}

func anyHTTPOptionSet(cfg Config) bool {
	return cfg.HTTPExporterInsecure.Valid ||
		cfg.HTTPExporterEndpoint.Valid ||
		cfg.HTTPExporterURLPath.Valid
}

func anyGRPCOptionSet(cfg Config) bool {
	return cfg.GRPCExporterInsecure.Valid ||
		cfg.GRPCExporterEndpoint.Valid
}
