package remotewrite

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.k6.io/k6/internal/output/prometheusrw/sigv4"

	"go.k6.io/k6/internal/output/prometheusrw/remote"
	"go.k6.io/k6/lib/types"
	"gopkg.in/guregu/null.v3"
)

const (
	defaultServerURL    = "http://localhost:9090/api/v1/write"
	defaultTimeout      = 5 * time.Second
	defaultPushInterval = 5 * time.Second
	defaultMetricPrefix = "k6_"
)

//nolint:gochecknoglobals
var defaultTrendStats = []string{"p(99)"}

// Config contains the configuration for the Output.
type Config struct {
	// ServerURL contains the absolute ServerURL for the Write endpoint where to flush the time series.
	ServerURL null.String `json:"url"`

	// Headers contains additional headers that should be included in the HTTP requests.
	Headers map[string]string `json:"headers"`

	// InsecureSkipTLSVerify skips TLS client side checks.
	InsecureSkipTLSVerify null.Bool `json:"insecureSkipTLSVerify"`

	// Username is the User for Basic Auth.
	Username null.String `json:"username"`

	// Password is the Password for the Basic Auth.
	Password null.String `json:"password"`

	// ClientCertificate is the public key of the SSL certificate.
	// It is expected the path of the certificate on the file system.
	// If it is required a dedicated Certifacate Authority then it should be added
	// to the conventional folders defined by the operating system's registry.
	ClientCertificate null.String `json:"clientCertificate"`

	// ClientCertificateKey is the private key of the SSL certificate.
	// It is expected the path of the certificate on the file system.
	ClientCertificateKey null.String `json:"clientCertificateKey"`

	// BearerToken if set is the token used for the `Authorization` header.
	BearerToken null.String `json:"bearerToken"`

	// PushInterval defines the time between flushes. The Output will wait the set time
	// before push a new set of time series to the endpoint.
	PushInterval types.NullDuration `json:"pushInterval"`

	// TrendAsNativeHistogram defines if the mapping for metrics defined as Trend type
	// should map to a Prometheus' Native Histogram.
	TrendAsNativeHistogram null.Bool `json:"trendAsNativeHistogram"`

	// TrendStats defines the stats to flush for Trend metrics.
	//
	// TODO: should we support K6_SUMMARY_TREND_STATS?
	TrendStats []string `json:"trendStats"`

	StaleMarkers null.Bool `json:"staleMarkers"`

	// SigV4Region is the AWS region where the workspace is.
	SigV4Region null.String `json:"sigV4Region"`

	// SigV4AccessKey is the AWS access key.
	SigV4AccessKey null.String `json:"sigV4AccessKey"`

	// SigV4SecretKey is the AWS secret key.
	SigV4SecretKey null.String `json:"sigV4SecretKey"`
}

// NewConfig creates an Output's configuration.
func NewConfig() Config {
	return Config{
		ServerURL:             null.StringFrom(defaultServerURL),
		InsecureSkipTLSVerify: null.BoolFrom(false),
		Username:              null.NewString("", false),
		Password:              null.NewString("", false),
		PushInterval:          types.NullDurationFrom(defaultPushInterval),
		Headers:               make(map[string]string),
		TrendStats:            defaultTrendStats,
		StaleMarkers:          null.BoolFrom(false),
		SigV4Region:           null.NewString("", false),
		SigV4AccessKey:        null.NewString("", false),
		SigV4SecretKey:        null.NewString("", false),
	}
}

// RemoteConfig creates a configuration for the HTTP Remote-write client.
func (conf Config) RemoteConfig() (*remote.HTTPConfig, error) {
	hc := remote.HTTPConfig{
		Timeout: defaultTimeout,
	}

	// if at least valid user was configured, use basic auth
	if conf.Username.Valid {
		hc.BasicAuth = &remote.BasicAuth{
			Username: conf.Username.String,
			Password: conf.Password.String,
		}
	}

	hc.TLSConfig = &tls.Config{
		InsecureSkipVerify: conf.InsecureSkipTLSVerify.Bool, //nolint:gosec
	}

	if conf.ClientCertificate.Valid && conf.ClientCertificateKey.Valid {
		cert, err := tls.LoadX509KeyPair(conf.ClientCertificate.String, conf.ClientCertificateKey.String)
		if err != nil {
			return nil, fmt.Errorf("failed to load the TLS certificate: %w", err)
		}
		hc.TLSConfig.Certificates = []tls.Certificate{cert}
	}

	if isSigV4PartiallyConfigured(conf.SigV4Region, conf.SigV4AccessKey, conf.SigV4SecretKey) {
		return nil, errors.New(
			"sigv4 seems to be partially configured. All of " +
				"K6_PROMETHEUS_RW_SIGV4_REGION, K6_PROMETHEUS_RW_SIGV4_ACCESS_KEY, K6_PROMETHEUS_RW_SIGV4_SECRET_KEY " +
				"must all be set. Unset all to bypass sigv4",
		)
	}

	if conf.SigV4Region.Valid && conf.SigV4AccessKey.Valid && conf.SigV4SecretKey.Valid {
		hc.SigV4 = &sigv4.Config{
			Region:             conf.SigV4Region.String,
			AwsAccessKeyID:     conf.SigV4AccessKey.String,
			AwsSecretAccessKey: conf.SigV4SecretKey.String,
		}
	}

	if len(conf.Headers) > 0 {
		hc.Headers = make(http.Header)
		for k, v := range conf.Headers {
			hc.Headers.Add(k, v)
		}
	}

	if conf.BearerToken.String != "" {
		if hc.Headers == nil {
			hc.Headers = make(http.Header)
		}
		hc.Headers.Set("Authorization", "Bearer "+conf.BearerToken.String)
	}

	return &hc, nil
}

// Apply merges applied Config into base.
func (conf Config) Apply(applied Config) Config {
	if applied.ServerURL.Valid {
		conf.ServerURL = applied.ServerURL
	}

	if applied.InsecureSkipTLSVerify.Valid {
		conf.InsecureSkipTLSVerify = applied.InsecureSkipTLSVerify
	}

	if applied.Username.Valid {
		conf.Username = applied.Username
	}

	if applied.Password.Valid {
		conf.Password = applied.Password
	}

	if applied.BearerToken.Valid {
		conf.BearerToken = applied.BearerToken
	}

	if applied.SigV4Region.Valid {
		conf.SigV4Region = applied.SigV4Region
	}

	if applied.SigV4AccessKey.Valid {
		conf.SigV4AccessKey = applied.SigV4AccessKey
	}

	if applied.SigV4SecretKey.Valid {
		conf.SigV4SecretKey = applied.SigV4SecretKey
	}

	if applied.PushInterval.Valid {
		conf.PushInterval = applied.PushInterval
	}

	if applied.TrendAsNativeHistogram.Valid {
		conf.TrendAsNativeHistogram = applied.TrendAsNativeHistogram
	}

	if applied.StaleMarkers.Valid {
		conf.StaleMarkers = applied.StaleMarkers
	}

	if len(applied.Headers) > 0 {
		for k, v := range applied.Headers {
			conf.Headers[k] = v
		}
	}

	if len(applied.TrendStats) > 0 {
		conf.TrendStats = make([]string, len(applied.TrendStats))
		copy(conf.TrendStats, applied.TrendStats)
	}

	if applied.ClientCertificate.Valid {
		conf.ClientCertificate = applied.ClientCertificate
	}

	if applied.ClientCertificateKey.Valid {
		conf.ClientCertificateKey = applied.ClientCertificateKey
	}

	return conf
}

// GetConsolidatedConfig combines the options' values from the different sources
// and returns the merged options. The Order of precedence used is documented
// in the k6 Documentation https://k6.io/docs/using-k6/k6-options/how-to/#order-of-precedence.
func GetConsolidatedConfig(jsonRawConf json.RawMessage, env map[string]string, _ string) (Config, error) {
	result := NewConfig()
	if jsonRawConf != nil {
		jsonConf, err := parseJSON(jsonRawConf)
		if err != nil {
			return result, fmt.Errorf("parse JSON options failed: %w", err)
		}
		result = result.Apply(jsonConf)
	}

	if len(env) > 0 {
		envConf, err := parseEnvs(env)
		if err != nil {
			return result, fmt.Errorf("parse environment variables options failed: %w", err)
		}
		result = result.Apply(envConf)
	}

	// TODO: define a way for defining Output's options
	// then support them.
	// url is the third GetConsolidatedConfig's argument which is omitted for now
	//nolint:gocritic
	//
	//if url != "" {
	//urlConf, err := parseArg(url)
	//if err != nil {
	//return result, fmt.Errorf("parse argument string options failed: %w", err)
	//}
	//result = result.Apply(urlConf)
	//}

	return result, nil
}

func envBool(env map[string]string, name string) (null.Bool, error) {
	if v, vDefined := env[name]; vDefined {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return null.NewBool(false, false), err
		}

		return null.BoolFrom(b), nil
	}
	return null.NewBool(false, false), nil
}

func envMap(env map[string]string, prefix string) map[string]string {
	result := make(map[string]string)
	for ek, ev := range env {
		if strings.HasPrefix(ek, prefix) {
			k := strings.TrimPrefix(ek, prefix)
			result[k] = ev
		}
	}
	return result
}

// TODO: try to migrate to github.com/mstoykov/envconfig like it's done on other projects?
func parseEnvs(env map[string]string) (Config, error) { //nolint:funlen
	c := Config{
		Headers: make(map[string]string),
	}

	if pushInterval, pushIntervalDefined := env["K6_PROMETHEUS_RW_PUSH_INTERVAL"]; pushIntervalDefined {
		if err := c.PushInterval.UnmarshalText([]byte(pushInterval)); err != nil {
			return c, err
		}
	}

	if url, urlDefined := env["K6_PROMETHEUS_RW_SERVER_URL"]; urlDefined {
		c.ServerURL = null.StringFrom(url)
	}

	if b, err := envBool(env, "K6_PROMETHEUS_RW_INSECURE_SKIP_TLS_VERIFY"); err != nil {
		return c, err
	} else if b.Valid {
		c.InsecureSkipTLSVerify = b
	}

	if user, userDefined := env["K6_PROMETHEUS_RW_USERNAME"]; userDefined {
		c.Username = null.StringFrom(user)
	}

	if password, passwordDefined := env["K6_PROMETHEUS_RW_PASSWORD"]; passwordDefined {
		c.Password = null.StringFrom(password)
	}

	if clientCertificate, certDefined := env["K6_PROMETHEUS_RW_CLIENT_CERTIFICATE"]; certDefined {
		c.ClientCertificate = null.StringFrom(clientCertificate)
	}

	if clientCertificateKey, certDefined := env["K6_PROMETHEUS_RW_CLIENT_CERTIFICATE_KEY"]; certDefined {
		c.ClientCertificateKey = null.StringFrom(clientCertificateKey)
	}

	if token, tokenDefined := env["K6_PROMETHEUS_RW_BEARER_TOKEN"]; tokenDefined {
		c.BearerToken = null.StringFrom(token)
	}

	envHeaders := envMap(env, "K6_PROMETHEUS_RW_HEADERS_")
	for k, v := range envHeaders {
		c.Headers[k] = v
	}

	if headers, headersDefined := env["K6_PROMETHEUS_RW_HTTP_HEADERS"]; headersDefined {
		for _, kvPair := range strings.Split(headers, ",") {
			header := strings.Split(kvPair, ":")
			if len(header) != 2 {
				return c, fmt.Errorf("the provided header (%s) does not respect the expected format <header key>:<value>", kvPair)
			}
			c.Headers[header[0]] = header[1]
		}
	}

	if sigV4Region, sigV4RegionDefined := env["K6_PROMETHEUS_RW_SIGV4_REGION"]; sigV4RegionDefined {
		c.SigV4Region = null.StringFrom(sigV4Region)
	}

	if sigV4AccessKey, sigV4AccessKeyDefined := env["K6_PROMETHEUS_RW_SIGV4_ACCESS_KEY"]; sigV4AccessKeyDefined {
		c.SigV4AccessKey = null.StringFrom(sigV4AccessKey)
	}

	if sigV4SecretKey, sigV4SecretKeyDefined := env["K6_PROMETHEUS_RW_SIGV4_SECRET_KEY"]; sigV4SecretKeyDefined {
		c.SigV4SecretKey = null.StringFrom(sigV4SecretKey)
	}

	if b, err := envBool(env, "K6_PROMETHEUS_RW_TREND_AS_NATIVE_HISTOGRAM"); err != nil {
		return c, err
	} else if b.Valid {
		c.TrendAsNativeHistogram = b
	}

	if b, err := envBool(env, "K6_PROMETHEUS_RW_STALE_MARKERS"); err != nil {
		return c, err
	} else if b.Valid {
		c.StaleMarkers = b
	}

	if trendStats, trendStatsDefined := env["K6_PROMETHEUS_RW_TREND_STATS"]; trendStatsDefined {
		c.TrendStats = strings.Split(trendStats, ",")
	}

	return c, nil
}

// parseJSON parses the supplied JSON into a Config.
func parseJSON(data json.RawMessage) (Config, error) {
	var c Config
	err := json.Unmarshal(data, &c)
	return c, err
}

// parseArg parses the supplied string of arguments into a Config.
func parseArg(text string) (Config, error) {
	var c Config
	opts := strings.Split(text, ",")

	for _, opt := range opts {
		r := strings.SplitN(opt, "=", 2)
		if len(r) != 2 {
			return c, fmt.Errorf("couldn't parse argument %q as option", opt)
		}
		key, v := r[0], r[1]
		switch key {
		case "url":
			c.ServerURL = null.StringFrom(v)
		case "insecureSkipTLSVerify":
			if err := c.InsecureSkipTLSVerify.UnmarshalText([]byte(v)); err != nil {
				return c, fmt.Errorf("insecureSkipTLSVerify value must be true or false, not %q", v)
			}
		case "username":
			c.Username = null.StringFrom(v)
		case "password":
			c.Password = null.StringFrom(v)
		case "pushInterval":
			if err := c.PushInterval.UnmarshalText([]byte(v)); err != nil {
				return c, err
			}
		case "trendAsNativeHistogram":
			if err := c.TrendAsNativeHistogram.UnmarshalText([]byte(v)); err != nil {
				return c, fmt.Errorf("trendAsNativeHistogram value must be true or false, not %q", v)
			}

		// TODO: add the support for trendStats
		// strvals doesn't support the same format used by --summary-trend-stats
		// using the comma as the separator, because it is already used for
		// dividing the keys.
		//nolint:gocritic
		//
		//if v, ok := params["trendStats"].(string); ok && len(v) > 0 {
		//c.TrendStats = strings.Split(v, ",")
		//}

		case "clientCertificate":
			c.ClientCertificate = null.StringFrom(v)
		case "clientCertificateKey":
			c.ClientCertificateKey = null.StringFrom(v)

		default:
			if !strings.HasPrefix(key, "headers.") {
				return c, fmt.Errorf("%q is an unknown option's key", r[0])
			}
			if c.Headers == nil {
				c.Headers = make(map[string]string)
			}
			c.Headers[strings.TrimPrefix(key, "headers.")] = v
		}
	}

	return c, nil
}

func isSigV4PartiallyConfigured(region, accessKey, secretKey null.String) bool {
	hasRegion := region.Valid && len(strings.TrimSpace(region.String)) != 0
	hasAccessID := accessKey.Valid && len(strings.TrimSpace(accessKey.String)) != 0
	hasSecretAccessKey := secretKey.Valid && len(strings.TrimSpace(secretKey.String)) != 0
	// either they are all set, or all not set. False if partial
	isComplete := (hasRegion && hasAccessID && hasSecretAccessKey) || (!hasRegion && !hasAccessID && !hasSecretAccessKey)
	return !isComplete
}
