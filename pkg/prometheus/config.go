package prometheus

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/kubernetes/helm/pkg/strvals"
	promConfig "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/storage/remote"
	"go.k6.io/k6/lib/types"
	"gopkg.in/guregu/null.v3"
)

type Config struct {
	Url                   null.String `json:"url" envconfig:"K6_PROMETHEUS_REMOTE_URL"` // here we assume that we won't need to distinguish from remote read URL
	InsecureSkipTLSVerify null.Bool   `json:"insecureSkipTLSVerify" envconfig:"K6_PROMETHEUS_INSECURE_SKIP_TLS_VERIFY"`
	// User          null.String        `json:"user" envconfig:"K6_PROMETHEUS_USER"`
	// Password      null.String        `json:"password" envconfig:"K6_PROMETHEUS_PASSWORD"`
	FlushPeriod types.NullDuration `json:"flushPeriod" envconfig:"K6_PROMETHEUS_FLUSH_PERIOD"`
}

func NewConfig() Config {
	return Config{
		Url:                   null.StringFrom(""),
		InsecureSkipTLSVerify: null.BoolFrom(false),
		FlushPeriod:           types.NullDurationFrom(1 * time.Second),
	}
}

func (conf Config) ConstructRemoteConfig() (*remote.ClientConfig, error) {
	httpConfig := promConfig.DefaultHTTPClientConfig
	httpConfig.TLSConfig = promConfig.TLSConfig{
		InsecureSkipVerify: conf.InsecureSkipTLSVerify.Bool,
	}
	// httpConfig.BasicAuth = &promConfig.BasicAuth{
	// 	Username: "YOUR USERNAME",
	// 	Password: "YOUR PASSWORD",
	// }

	u, err := url.Parse(conf.Url.String)
	if err != nil {
		return nil, err
	}

	remoteConfig := remote.ClientConfig{
		URL:              &promConfig.URL{u},
		Timeout:          model.Duration(time.Minute * 2),
		HTTPClientConfig: httpConfig,
		RetryOnRateLimit: false, // disables retries on HTTP status 429
	}
	return &remoteConfig, nil
}

// From here till the end of the file partial duplicates waiting for config refactor (#883)

func (base Config) Apply(applied Config) Config {
	fmt.Printf("%+v\n%+v\n\n", base, applied)
	if base.Url.Valid {
		base.Url = applied.Url
	}

	if base.InsecureSkipTLSVerify.Valid {
		base.InsecureSkipTLSVerify = applied.InsecureSkipTLSVerify
	}

	if base.FlushPeriod.Valid {
		base.FlushPeriod = applied.FlushPeriod
	}

	return base
}

// ParseArg takes an arg string and converts it to a config
func ParseArg(arg string) (Config, error) {
	c := Config{}
	params, err := strvals.Parse(arg)
	if err != nil {
		return c, err
	}

	if v, ok := params["url"].(string); ok {
		c.Url = null.StringFrom(v)
	}

	if v, ok := params["insecureSkipTLSVerify"].(bool); ok {
		c.InsecureSkipTLSVerify = null.BoolFrom(v)
	}

	if v, ok := params["flushPeriod"].(string); ok {
		if err := c.FlushPeriod.UnmarshalText([]byte(v)); err != nil {
			return c, err
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

	// envconfig is not processing some undefined vars (at least duration) so apply them manually
	if flushPeriod, flushPeriodDefined := env["K6_PROMETHEUS_FLUSH_PERIOD"]; flushPeriodDefined {
		if err := result.FlushPeriod.UnmarshalText([]byte(flushPeriod)); err != nil {
			return result, err
		}
	}

	if url, urlDefined := env["K6_PROMETHEUS_REMOTE_URL"]; urlDefined {
		result.Url = null.StringFrom(url)
	}

	if insecureSkipTLSVerify, insecureSkipTLSVerifyDefined := env["K6_PROMETHEUS_INSECURE_SKIP_TLS_VERIFY"]; insecureSkipTLSVerifyDefined {
		if b, err := strconv.ParseBool(insecureSkipTLSVerify); err != nil {
			return result, err
		} else {
			result.InsecureSkipTLSVerify = null.BoolFrom(b)
		}
	}

	if arg != "" {
		urlConf, err := ParseArg(arg)
		if err != nil {
			return result, err
		}
		result = result.Apply(urlConf)
	}

	return result, nil
}
