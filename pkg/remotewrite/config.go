package remotewrite

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/kubernetes/helm/pkg/strvals"
	promConfig "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/storage/remote"
	"go.k6.io/k6/lib/types"
	"gopkg.in/guregu/null.v3"
)

const (
	defaultPrometheusTimeout = time.Minute
	defaultFlushPeriod       = time.Second
	defaultMetricPrefix      = "k6_"
)

type Config struct {
	Mapping null.String `json:"mapping" envconfig:"K6_PROMETHEUS_MAPPING"`

	Url null.String `json:"url" envconfig:"K6_PROMETHEUS_REMOTE_URL"` // here, in the name of env variable, we assume that we won't need to distinguish between remote write URL vs remote read URL

	Headers map[string]string `json:"headers" envconfig:"K6_PROMETHEUS_HEADERS"`

	InsecureSkipTLSVerify null.Bool   `json:"insecureSkipTLSVerify" envconfig:"K6_PROMETHEUS_INSECURE_SKIP_TLS_VERIFY"`
	CACert                null.String `json:"caCertFile" envconfig:"K6_CA_CERT_FILE"`

	User     null.String `json:"user" envconfig:"K6_PROMETHEUS_USER"`
	Password null.String `json:"password" envconfig:"K6_PROMETHEUS_PASSWORD"`

	FlushPeriod types.NullDuration `json:"flushPeriod" envconfig:"K6_PROMETHEUS_FLUSH_PERIOD"`

	KeepTags    null.Bool `json:"keepTags" envconfig:"K6_KEEP_TAGS"`
	KeepNameTag null.Bool `json:"keepNameTag" envconfig:"K6_KEEP_NAME_TAG"`
}

func NewConfig() Config {
	return Config{
		Mapping:               null.StringFrom("prometheus"),
		Url:                   null.StringFrom("http://localhost:9090/api/v1/write"),
		InsecureSkipTLSVerify: null.BoolFrom(true),
		CACert:                null.NewString("", false),
		User:                  null.NewString("", false),
		Password:              null.NewString("", false),
		FlushPeriod:           types.NullDurationFrom(defaultFlushPeriod),
		KeepTags:              null.BoolFrom(true),
		KeepNameTag:           null.BoolFrom(false),
		Headers:               make(map[string]string),
	}
}

func (conf Config) ConstructRemoteConfig() (*remote.ClientConfig, error) {
	httpConfig := promConfig.DefaultHTTPClientConfig

	httpConfig.TLSConfig = promConfig.TLSConfig{
		InsecureSkipVerify: conf.InsecureSkipTLSVerify.Bool,
	}

	// if insecureSkipTLSVerify is switched off, use the certificate file
	if !conf.InsecureSkipTLSVerify.Bool {
		httpConfig.TLSConfig.CAFile = conf.CACert.String
	}

	// if at least valid user was configured, use basic auth
	if conf.User.Valid {
		httpConfig.BasicAuth = &promConfig.BasicAuth{
			Username: conf.User.String,
			Password: promConfig.Secret(conf.Password.String),
		}
	}
	// TODO: consider if the auth logic should be enforced here
	// (e.g. if insecureSkipTLSVerify is switched off, then check for non-empty certificate file and auth, etc.)

	u, err := url.Parse(conf.Url.String)
	if err != nil {
		return nil, err
	}

	remoteConfig := remote.ClientConfig{
		URL:              &promConfig.URL{URL: u},
		Timeout:          model.Duration(defaultPrometheusTimeout),
		HTTPClientConfig: httpConfig,
		RetryOnRateLimit: true,
		Headers:          conf.Headers,
	}
	return &remoteConfig, nil
}

// From here till the end of the file partial duplicates waiting for config refactor (k6 #883)

func (base Config) Apply(applied Config) Config {
	if applied.Mapping.Valid {
		base.Mapping = applied.Mapping
	}

	if applied.Url.Valid {
		base.Url = applied.Url
	}

	if applied.InsecureSkipTLSVerify.Valid {
		base.InsecureSkipTLSVerify = applied.InsecureSkipTLSVerify
	}

	if applied.CACert.Valid {
		base.CACert = applied.CACert
	}

	if applied.User.Valid {
		base.User = applied.User
	}

	if applied.Password.Valid {
		base.Password = applied.Password
	}

	if applied.FlushPeriod.Valid {
		base.FlushPeriod = applied.FlushPeriod
	}

	if applied.KeepTags.Valid {
		base.KeepTags = applied.KeepTags
	}

	if applied.KeepNameTag.Valid {
		base.KeepNameTag = applied.KeepNameTag
	}

	if len(applied.Headers) > 0 {
		for k, v := range applied.Headers {
			base.Headers[k] = v
		}
	}

	return base
}

// ParseArg takes an arg string and converts it to a config
func ParseArg(arg string) (Config, error) {
	var c Config
	params, err := strvals.Parse(arg)
	if err != nil {
		return c, err
	}

	if v, ok := params["mapping"].(string); ok {
		c.Mapping = null.StringFrom(v)
	}

	if v, ok := params["url"].(string); ok {
		c.Url = null.StringFrom(v)
	}

	if v, ok := params["insecureSkipTLSVerify"].(bool); ok {
		c.InsecureSkipTLSVerify = null.BoolFrom(v)
	}

	if v, ok := params["caCertFile"].(string); ok {
		c.CACert = null.StringFrom(v)
	}

	if v, ok := params["user"].(string); ok {
		c.User = null.StringFrom(v)
	}

	if v, ok := params["password"].(string); ok {
		c.Password = null.StringFrom(v)
	}

	if v, ok := params["flushPeriod"].(string); ok {
		if err := c.FlushPeriod.UnmarshalText([]byte(v)); err != nil {
			return c, err
		}
	}

	if v, ok := params["keepTags"].(bool); ok {
		c.KeepTags = null.BoolFrom(v)
	}

	if v, ok := params["keepNameTag"].(bool); ok {
		c.KeepNameTag = null.BoolFrom(v)
	}

	c.Headers = make(map[string]string)
	if v, ok := params["headers"].(map[string]interface{}); ok {
		for k, v := range v {
			if v, ok := v.(string); ok {
				c.Headers[k] = v
			}
		}
	}

	return c, nil
}

// GetConsolidatedConfig combines {default config values + JSON config +
// environment vars + arg config values}, and returns the final result.
func GetConsolidatedConfig(jsonRawConf json.RawMessage, env map[string]string, arg string) (Config, error) {
	result := NewConfig()
	if jsonRawConf != nil {
		jsonConf := Config{}
		if err := json.Unmarshal(jsonRawConf, &jsonConf); err != nil {
			return result, err
		}
		result = result.Apply(jsonConf)
	}

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
	if flushPeriod, flushPeriodDefined := env["K6_PROMETHEUS_FLUSH_PERIOD"]; flushPeriodDefined {
		if err := result.FlushPeriod.UnmarshalText([]byte(flushPeriod)); err != nil {
			return result, err
		}
	}

	if mapping, mappingDefined := env["K6_PROMETHEUS_MAPPING"]; mappingDefined {
		result.Mapping = null.StringFrom(mapping)
	}

	if url, urlDefined := env["K6_PROMETHEUS_REMOTE_URL"]; urlDefined {
		result.Url = null.StringFrom(url)
	}

	if b, err := getEnvBool(env, "K6_PROMETHEUS_INSECURE_SKIP_TLS_VERIFY"); err != nil {
		return result, err
	} else {
		if b.Valid {
			// apply only if valid, to keep default option otherwise
			result.InsecureSkipTLSVerify = b
		}
	}

	if ca, caDefined := env["K6_CA_CERT_FILE"]; caDefined {
		result.CACert = null.StringFrom(ca)
	}

	if user, userDefined := env["K6_PROMETHEUS_USER"]; userDefined {
		result.User = null.StringFrom(user)
	}

	if password, passwordDefined := env["K6_PROMETHEUS_PASSWORD"]; passwordDefined {
		result.Password = null.StringFrom(password)
	}

	if b, err := getEnvBool(env, "K6_KEEP_TAGS"); err != nil {
		return result, err
	} else {
		if b.Valid {
			result.KeepTags = b
		}
	}

	if b, err := getEnvBool(env, "K6_KEEP_NAME_TAG"); err != nil {
		return result, err
	} else {
		if b.Valid {
			result.KeepNameTag = b
		}
	}

	envHeaders := getEnvMap(env, "K6_PROMETHEUS_HEADERS_")
	for k, v := range envHeaders {
		result.Headers[k] = v
	}

	if arg != "" {
		argConf, err := ParseArg(arg)
		if err != nil {
			return result, err
		}

		result = result.Apply(argConf)
	}

	return result, nil
}
