package influxdb

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mstoykov/envconfig"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/lib/types"
)

// Config represents a k6's influxdb output configuration.
type Config struct {
	// Connection.
	Addr             null.String        `json:"addr" envconfig:"K6_INFLUXDB_ADDR"`
	Proxy            null.String        `json:"proxy,omitempty" envconfig:"K6_INFLUXDB_PROXY"`
	Username         null.String        `json:"username,omitempty" envconfig:"K6_INFLUXDB_USERNAME"`
	Password         null.String        `json:"password,omitempty" envconfig:"K6_INFLUXDB_PASSWORD"`
	Insecure         null.Bool          `json:"insecure,omitempty" envconfig:"K6_INFLUXDB_INSECURE"`
	PayloadSize      null.Int           `json:"payloadSize,omitempty" envconfig:"K6_INFLUXDB_PAYLOAD_SIZE"`
	PushInterval     types.NullDuration `json:"pushInterval,omitempty" envconfig:"K6_INFLUXDB_PUSH_INTERVAL"`
	ConcurrentWrites null.Int           `json:"concurrentWrites,omitempty" envconfig:"K6_INFLUXDB_CONCURRENT_WRITES"`

	// Samples.
	DB           null.String `json:"db" envconfig:"K6_INFLUXDB_DB"`
	Precision    null.String `json:"precision,omitempty" envconfig:"K6_INFLUXDB_PRECISION"`
	Retention    null.String `json:"retention,omitempty" envconfig:"K6_INFLUXDB_RETENTION"`
	Consistency  null.String `json:"consistency,omitempty" envconfig:"K6_INFLUXDB_CONSISTENCY"`
	TagsAsFields []string    `json:"tagsAsFields,omitempty" envconfig:"K6_INFLUXDB_TAGS_AS_FIELDS"`
}

// NewConfig creates a new InfluxDB output config with some default values.
func NewConfig() Config {
	c := Config{
		Addr:         null.NewString("http://localhost:8086", false),
		DB:           null.NewString("k6", false),
		TagsAsFields: []string{"vu", "iter", "url"},
		PushInterval: types.NewNullDuration(time.Second, false),

		// The minimum value of pow(2, N) for handling a stressful situation
		// with the default push interval set to 1s.
		// Concurrency is not expected for the normal use-case,
		// the response time should be lower than the push interval set value.
		// In case of spikes, the response time could go around 2s,
		// higher values will highlight a not sustainable situation
		// and the user should adjust the executed script
		// or the configuration based on the environment and rate expected.
		ConcurrentWrites: null.NewInt(4, false),
	}
	return c
}

// Apply applies a valid config options to the receiver.
func (c Config) Apply(cfg Config) Config {
	if cfg.Addr.Valid {
		c.Addr = cfg.Addr
	}
	if cfg.Proxy.Valid {
		c.Proxy = cfg.Proxy
	}
	if cfg.Username.Valid {
		c.Username = cfg.Username
	}
	if cfg.Password.Valid {
		c.Password = cfg.Password
	}
	if cfg.Insecure.Valid {
		c.Insecure = cfg.Insecure
	}
	if cfg.PayloadSize.Valid && cfg.PayloadSize.Int64 > 0 {
		c.PayloadSize = cfg.PayloadSize
	}
	if cfg.DB.Valid {
		c.DB = cfg.DB
	}
	if cfg.Precision.Valid {
		c.Precision = cfg.Precision
	}
	if cfg.Retention.Valid {
		c.Retention = cfg.Retention
	}
	if cfg.Consistency.Valid {
		c.Consistency = cfg.Consistency
	}
	if len(cfg.TagsAsFields) > 0 {
		c.TagsAsFields = cfg.TagsAsFields
	}
	if cfg.PushInterval.Valid {
		c.PushInterval = cfg.PushInterval
	}

	if cfg.ConcurrentWrites.Valid {
		c.ConcurrentWrites = cfg.ConcurrentWrites
	}
	return c
}

// ParseJSON parses the supplied JSON into a Config.
func ParseJSON(data json.RawMessage) (Config, error) {
	conf := Config{}
	err := json.Unmarshal(data, &conf)
	return conf, err
}

// ParseURL parses the supplied URL into a Config.
func ParseURL(text string) (Config, error) {
	c := Config{}
	u, err := url.Parse(text)
	if err != nil {
		return c, err
	}
	if u.Host != "" {
		c.Addr = null.StringFrom(u.Scheme + "://" + u.Host)
	}
	if db := strings.TrimPrefix(u.Path, "/"); db != "" {
		c.DB = null.StringFrom(db)
	}
	if u.User != nil {
		c.Username = null.StringFrom(u.User.Username())
		pass, _ := u.User.Password()
		c.Password = null.StringFrom(pass)
	}
	for k, vs := range u.Query() {
		switch k {
		case "insecure":
			switch vs[0] {
			case "":
			case "false":
				c.Insecure = null.BoolFrom(false)
			case "true":
				c.Insecure = null.BoolFrom(true)
			default:
				return c, fmt.Errorf("insecure must be true or false, not %s", vs[0])
			}
		case "payload_size":
			var size int
			size, err = strconv.Atoi(vs[0])
			if err != nil {
				return c, err
			}
			c.PayloadSize = null.IntFrom(int64(size))
		case "precision":
			c.Precision = null.StringFrom(vs[0])
		case "retention":
			c.Retention = null.StringFrom(vs[0])
		case "consistency":
			c.Consistency = null.StringFrom(vs[0])

		case "pushInterval":
			err = c.PushInterval.UnmarshalText([]byte(vs[0]))
			if err != nil {
				return c, err
			}
		case "concurrentWrites":
			var writes int
			writes, err = strconv.Atoi(vs[0])
			if err != nil {
				return c, err
			}
			c.ConcurrentWrites = null.IntFrom(int64(writes))
		case "tagsAsFields":
			c.TagsAsFields = vs
		default:
			return c, fmt.Errorf("unknown query parameter: %s", k)
		}
	}
	return c, err
}

// GetConsolidatedConfig combines {default config values + JSON config +
// environment vars + URL config values}, and returns the final result.
func GetConsolidatedConfig(jsonRawConf json.RawMessage, env map[string]string, url string) (Config, error) {
	result := NewConfig()
	if jsonRawConf != nil {
		jsonConf, err := ParseJSON(jsonRawConf)
		if err != nil {
			return result, err
		}
		result = result.Apply(jsonConf)
	}

	envConfig := Config{}
	if err := envconfig.Process("", &envConfig, func(key string) (string, bool) {
		v, ok := env[key]
		return v, ok
	}); err != nil {
		// TODO: get rid of envconfig and actually use the env parameter...
		return result, err
	}
	result = result.Apply(envConfig)

	if url != "" {
		urlConf, err := ParseURL(url)
		if err != nil {
			return result, err
		}
		result = result.Apply(urlConf)
	}

	return result, nil
}
