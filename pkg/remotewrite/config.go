package remotewrite

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/xk6-output-prometheus-remote/pkg/remote"
	"go.k6.io/k6/lib/types"
	"gopkg.in/guregu/null.v3"
)

const (
	defaultServerURL    = "http://localhost:9090/api/v1/write"
	defaultTimeout      = 5 * time.Second
	defaultPushInterval = 5 * time.Second
	defaultMetricPrefix = "k6_"
)

var defaultTrendStats = []string{"p(99)"}

type Config struct {
	// ServerURL contains the absolute ServerURL for the Write endpoint where to flush the time series.
	ServerURL null.String `json:"url" envconfig:"K6_PROMETHEUS_RW_SERVER_URL"`

	// Headers contains additional headers that should be included in the HTTP requests.
	Headers map[string]string `json:"headers" envconfig:"K6_PROMETHEUS_RW_HEADERS"`

	// InsecureSkipTLSVerify skips TLS client side checks.
	InsecureSkipTLSVerify null.Bool `json:"insecureSkipTLSVerify" envconfig:"K6_PROMETHEUS_RW_INSECURE_SKIP_TLS_VERIFY"`

	// Username is the User for Basic Auth.
	Username null.String `json:"username" envconfig:"K6_PROMETHEUS_RW_USERNAME"`

	// Password is the Password for the Basic Auth.
	Password null.String `json:"password" envconfig:"K6_PROMETHEUS_RW_PASSWORD"`

	// PushInterval defines the time between flushes. The Output will wait the set time
	// before push a new set of time series to the endpoint.
	PushInterval types.NullDuration `json:"pushInterval" envconfig:"K6_PROMETHEUS_RW_PUSH_INTERVAL"`

	// TrendAsNativeHistogram defines if the mapping for metrics defined as Trend type
	// should map to a Prometheus' Native Histogram.
	TrendAsNativeHistogram null.Bool `json:"trendAsNativeHistogram" envconfig:"K6_PROMETHEUS_RW_TREND_AS_NATIVE_HISTOGRAM"`

	// TrendStats defines the stats to flush for Trend metrics.
	//
	// TODO: should we support K6_SUMMARY_TREND_STATS?
	TrendStats []string `json:"trendStats" envconfig:"K6_PROMETHEUS_RW_TREND_STATS"`
}

// NewConfig creates an Output's configuration.
func NewConfig() Config {
	return Config{
		ServerURL:             null.StringFrom(defaultServerURL),
		InsecureSkipTLSVerify: null.BoolFrom(true),
		Username:              null.NewString("", false),
		Password:              null.NewString("", false),
		PushInterval:          types.NullDurationFrom(defaultPushInterval),
		Headers:               make(map[string]string),
		TrendStats:            defaultTrendStats,
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
		InsecureSkipVerify: conf.InsecureSkipTLSVerify.Bool,
	}

	if len(conf.Headers) > 0 {
		hc.Headers = make(http.Header)
		for k, v := range conf.Headers {
			hc.Headers.Add(k, v)
		}
	}
	return &hc, nil
}

// Apply merges applied Config into base.
func (base Config) Apply(applied Config) Config {
	if applied.ServerURL.Valid {
		base.ServerURL = applied.ServerURL
	}

	if applied.InsecureSkipTLSVerify.Valid {
		base.InsecureSkipTLSVerify = applied.InsecureSkipTLSVerify
	}

	if applied.Username.Valid {
		base.Username = applied.Username
	}

	if applied.Password.Valid {
		base.Password = applied.Password
	}

	if applied.PushInterval.Valid {
		base.PushInterval = applied.PushInterval
	}

	if applied.TrendAsNativeHistogram.Valid {
		base.TrendAsNativeHistogram = applied.TrendAsNativeHistogram
	}

	if len(applied.Headers) > 0 {
		for k, v := range applied.Headers {
			base.Headers[k] = v
		}
	}

	if len(applied.TrendStats) > 0 {
		base.TrendStats = make([]string, len(applied.TrendStats))
		copy(base.TrendStats, applied.TrendStats)
	}

	return base
}

// GetConsolidatedConfig combines the options' values from the different sources
// and returns the merged options. The Order of precedence used is documented
// in the k6 Documentation https://k6.io/docs/using-k6/k6-options/how-to/#order-of-precedence.
func GetConsolidatedConfig(jsonRawConf json.RawMessage, env map[string]string, url string) (Config, error) {
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

func parseEnvs(env map[string]string) (Config, error) {
	var c Config

	getEnvBool := func(env map[string]string, name string) (null.Bool, error) {
		if v, vDefined := env[name]; vDefined {
			if b, err := strconv.ParseBool(v); err != nil {
				return null.NewBool(false, false), err
			} else {
				return null.BoolFrom(b), nil
			}
		}
		return null.NewBool(false, false), nil
	}

	getEnvMap := func(env map[string]string, prefix string) map[string]string {
		result := make(map[string]string)
		for ek, ev := range env {
			if strings.HasPrefix(ek, prefix) {
				k := strings.TrimPrefix(ek, prefix)
				result[k] = ev
			}
		}
		return result
	}

	// envconfig is not processing some undefined vars (at least duration) so apply them manually
	if pushInterval, pushIntervalDefined := env["K6_PROMETHEUS_RW_PUSH_INTERVAL"]; pushIntervalDefined {
		if err := c.PushInterval.UnmarshalText([]byte(pushInterval)); err != nil {
			return c, err
		}
	}

	if url, urlDefined := env["K6_PROMETHEUS_RW_SERVER_URL"]; urlDefined {
		c.ServerURL = null.StringFrom(url)
	}

	if b, err := getEnvBool(env, "K6_PROMETHEUS_RW_INSECURE_SKIP_TLS_VERIFY"); err != nil {
		return c, err
	} else {
		if b.Valid {
			c.InsecureSkipTLSVerify = b
		}
	}

	if user, userDefined := env["K6_PROMETHEUS_RW_USERNAME"]; userDefined {
		c.Username = null.StringFrom(user)
	}

	if password, passwordDefined := env["K6_PROMETHEUS_RW_PASSWORD"]; passwordDefined {
		c.Password = null.StringFrom(password)
	}

	envHeaders := getEnvMap(env, "K6_PROMETHEUS_RW_HEADERS_")
	for k, v := range envHeaders {
		if c.Headers == nil {
			c.Headers = make(map[string]string)
		}
		c.Headers[k] = v
	}

	if b, err := getEnvBool(env, "K6_PROMETHEUS_RW_TREND_AS_NATIVE_HISTOGRAM"); err != nil {
		return c, err
	} else {
		if b.Valid {
			c.TrendAsNativeHistogram = b
		}
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
		//
		//if v, ok := params["trendStats"].(string); ok && len(v) > 0 {
		//c.TrendStats = strings.Split(v, ",")
		//}

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
