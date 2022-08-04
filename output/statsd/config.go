package statsd

import (
	"encoding/json"
	"time"

	"github.com/mstoykov/envconfig"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/metrics"
)

// config defines the StatsD configuration.
type config struct {
	Addr         null.String         `json:"addr,omitempty" envconfig:"K6_STATSD_ADDR"`
	BufferSize   null.Int            `json:"bufferSize,omitempty" envconfig:"K6_STATSD_BUFFER_SIZE"`
	Namespace    null.String         `json:"namespace,omitempty" envconfig:"K6_STATSD_NAMESPACE"`
	PushInterval types.NullDuration  `json:"pushInterval,omitempty" envconfig:"K6_STATSD_PUSH_INTERVAL"`
	TagBlocklist metrics.EnabledTags `json:"tagBlocklist,omitempty" envconfig:"K6_STATSD_TAG_BLOCKLIST"`
	EnableTags   null.Bool           `json:"enableTags,omitempty" envconfig:"K6_STATSD_ENABLE_TAGS"`
}

func processTags(t metrics.EnabledTags, tags map[string]string) []string {
	var res []string
	for key, value := range tags {
		if value != "" && !t[key] {
			res = append(res, key+":"+value)
		}
	}
	return res
}

// Apply saves config non-zero config values from the passed config in the receiver.
func (c config) Apply(cfg config) config {
	if cfg.Addr.Valid {
		c.Addr = cfg.Addr
	}
	if cfg.BufferSize.Valid {
		c.BufferSize = cfg.BufferSize
	}
	if cfg.Namespace.Valid {
		c.Namespace = cfg.Namespace
	}
	if cfg.PushInterval.Valid {
		c.PushInterval = cfg.PushInterval
	}
	if cfg.TagBlocklist != nil {
		c.TagBlocklist = cfg.TagBlocklist
	}
	if cfg.EnableTags.Valid {
		c.EnableTags = cfg.EnableTags
	}

	return c
}

// newConfig creates a new Config instance with default values for some fields.
func newConfig() config {
	return config{
		Addr:         null.NewString("localhost:8125", false),
		BufferSize:   null.NewInt(20, false),
		Namespace:    null.NewString("k6.", false),
		PushInterval: types.NewNullDuration(1*time.Second, false),
		TagBlocklist: (metrics.TagVU | metrics.TagIter | metrics.TagURL).Map(),
		EnableTags:   null.NewBool(false, false),
	}
}

// getConsolidatedConfig combines {default config values + JSON config +
// environment vars}, and returns the final result.
func getConsolidatedConfig(jsonRawConf json.RawMessage, env map[string]string, _ string) (config, error) {
	result := newConfig()
	if jsonRawConf != nil {
		jsonConf := config{}
		if err := json.Unmarshal(jsonRawConf, &jsonConf); err != nil {
			return result, err
		}
		result = result.Apply(jsonConf)
	}

	envConfig := config{}
	_ = env // TODO: get rid of envconfig and actually use the env parameter...
	if err := envconfig.Process("", &envConfig, func(key string) (string, bool) {
		v, ok := env[key]
		return v, ok
	}); err != nil {
		return result, err
	}
	result = result.Apply(envConfig)

	return result, nil
}
