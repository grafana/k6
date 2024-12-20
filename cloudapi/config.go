package cloudapi

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/mstoykov/envconfig"
	"go.k6.io/k6/lib/types"
	"gopkg.in/guregu/null.v3"
)

// LegacyCloudConfigKey is the key used in the JSON config for the cloud output.
const LegacyCloudConfigKey = "loadimpact"

// Config holds all the necessary data and options for sending metrics to the k6 Cloud.
//
//nolint:lll
type Config struct {
	// TODO: refactor common stuff between cloud execution and output
	Token     null.String `json:"token" envconfig:"K6_CLOUD_TOKEN"`
	ProjectID null.Int    `json:"projectID" envconfig:"K6_CLOUD_PROJECT_ID"`
	Name      null.String `json:"name" envconfig:"K6_CLOUD_NAME"`

	Host    null.String        `json:"host" envconfig:"K6_CLOUD_HOST"`
	Timeout types.NullDuration `json:"timeout" envconfig:"K6_CLOUD_TIMEOUT"`

	LogsTailURL    null.String `json:"-" envconfig:"K6_CLOUD_LOGS_TAIL_URL"`
	WebAppURL      null.String `json:"webAppURL" envconfig:"K6_CLOUD_WEB_APP_URL"`
	TestRunDetails null.String `json:"testRunDetails" envconfig:"K6_CLOUD_TEST_RUN_DETAILS"`
	NoCompress     null.Bool   `json:"noCompress" envconfig:"K6_CLOUD_NO_COMPRESS"`
	StopOnError    null.Bool   `json:"stopOnError" envconfig:"K6_CLOUD_STOP_ON_ERROR"`
	APIVersion     null.Int    `json:"apiVersion" envconfig:"K6_CLOUD_API_VERSION"`

	// PushRefID is the identifier used by k6 Cloud to correlate all the things that
	// belong to the same test run/execution. Currently, it is equivalent to the test run id.
	// But, in the future, or in future solutions (e.g. Synthetic Monitoring), there might be
	// no test run id, and we may still need an identifier to correlate all the things.
	PushRefID null.String `json:"pushRefID" envconfig:"K6_CLOUD_PUSH_REF_ID"`

	// Defines the max allowed number of time series in a single batch.
	MaxTimeSeriesInBatch null.Int `json:"maxTimeSeriesInBatch" envconfig:"K6_CLOUD_MAX_TIME_SERIES_IN_BATCH"`

	// The time interval between periodic API calls for sending samples to the cloud ingest service.
	MetricPushInterval types.NullDuration `json:"metricPushInterval" envconfig:"K6_CLOUD_METRIC_PUSH_INTERVAL"`

	// This is how many concurrent pushes will be done at the same time to the cloud
	MetricPushConcurrency null.Int `json:"metricPushConcurrency" envconfig:"K6_CLOUD_METRIC_PUSH_CONCURRENCY"`

	// If specified and is greater than 0, sample aggregation with that period is enabled
	AggregationPeriod types.NullDuration `json:"aggregationPeriod" envconfig:"K6_CLOUD_AGGREGATION_PERIOD"`

	// If aggregation is enabled, this specifies how long we'll wait for period samples to accumulate before trying to aggregate them.
	AggregationWaitPeriod types.NullDuration `json:"aggregationWaitPeriod" envconfig:"K6_CLOUD_AGGREGATION_WAIT_PERIOD"`

	// Indicates whether to send traces to the k6 Insights backend service.
	TracesEnabled null.Bool `json:"tracesEnabled" envconfig:"K6_CLOUD_TRACES_ENABLED"`

	// The host of the k6 Insights backend service.
	TracesHost null.String `json:"traceHost" envconfig:"K6_CLOUD_TRACES_HOST"`

	// This is how many concurrent pushes will be done at the same time to the cloud
	TracesPushConcurrency null.Int `json:"tracesPushConcurrency" envconfig:"K6_CLOUD_TRACES_PUSH_CONCURRENCY"`

	// The time interval between periodic API calls for sending samples to the cloud ingest service.
	TracesPushInterval types.NullDuration `json:"tracesPushInterval" envconfig:"K6_CLOUD_TRACES_PUSH_INTERVAL"`
}

// NewConfig creates a new Config instance with default values for some fields.
func NewConfig() Config {
	return Config{
		APIVersion:            null.NewInt(2, false),
		Host:                  null.NewString("https://ingest.k6.io", false),
		LogsTailURL:           null.NewString("wss://cloudlogs.k6.io/api/v1/tail", false),
		WebAppURL:             null.NewString("https://app.k6.io", false),
		MetricPushInterval:    types.NewNullDuration(1*time.Second, false),
		MetricPushConcurrency: null.NewInt(1, false),
		Timeout:               types.NewNullDuration(1*time.Minute, false),

		// The set value (1000) is selected for performance reasons.
		// Any change to this value should be first discussed with internal stakeholders.
		MaxTimeSeriesInBatch: null.NewInt(1000, false),

		// TODO: the following values were used by the previous default version (v1).
		// We decided to keep the same values mostly for having a smoother migration to v2.
		// Because the previous version's aggregation config, a few lines below, is overwritten
		// by the remote service with the same values that we are now setting here for v2.
		// When the migration will be completed we may evaluate to re-discuss them
		// as we may evaluate to reduce these values - especially the waiting period.
		// A more specific request about waiting period is mentioned in the link below:
		// https://github.com/grafana/k6/blob/44e1e63aadb66784ff0a12b8d9821a0fdc9e7467/output/cloud/expv2/collect.go#L72-L77
		AggregationPeriod:     types.NewNullDuration(3*time.Second, false),
		AggregationWaitPeriod: types.NewNullDuration(8*time.Second, false),

		TracesEnabled:         null.NewBool(true, false),
		TracesHost:            null.NewString("grpc-k6-api-prod-prod-us-east-0.grafana.net:443", false),
		TracesPushInterval:    types.NewNullDuration(1*time.Second, false),
		TracesPushConcurrency: null.NewInt(1, false),
	}
}

// Apply saves config non-zero config values from the passed config in the receiver.
//
//nolint:cyclop
func (c Config) Apply(cfg Config) Config {
	if cfg.Token.Valid {
		c.Token = cfg.Token
	}
	if cfg.ProjectID.Valid && cfg.ProjectID.Int64 > 0 {
		c.ProjectID = cfg.ProjectID
	}
	if cfg.Name.Valid && cfg.Name.String != "" {
		c.Name = cfg.Name
	}
	if cfg.Host.Valid && cfg.Host.String != "" {
		c.Host = cfg.Host
	}
	if cfg.LogsTailURL.Valid && cfg.LogsTailURL.String != "" {
		c.LogsTailURL = cfg.LogsTailURL
	}
	if cfg.PushRefID.Valid {
		c.PushRefID = cfg.PushRefID
	}
	if cfg.WebAppURL.Valid {
		c.WebAppURL = cfg.WebAppURL
	}
	if cfg.TestRunDetails.Valid {
		c.TestRunDetails = cfg.TestRunDetails
	}
	if cfg.NoCompress.Valid {
		c.NoCompress = cfg.NoCompress
	}
	if cfg.StopOnError.Valid {
		c.StopOnError = cfg.StopOnError
	}
	if cfg.Timeout.Valid {
		c.Timeout = cfg.Timeout
	}
	if cfg.APIVersion.Valid {
		c.APIVersion = cfg.APIVersion
	}
	if cfg.MaxTimeSeriesInBatch.Valid {
		c.MaxTimeSeriesInBatch = cfg.MaxTimeSeriesInBatch
	}
	if cfg.MetricPushInterval.Valid {
		c.MetricPushInterval = cfg.MetricPushInterval
	}
	if cfg.MetricPushConcurrency.Valid {
		c.MetricPushConcurrency = cfg.MetricPushConcurrency
	}
	if cfg.TracesEnabled.Valid {
		c.TracesEnabled = cfg.TracesEnabled
	}
	if cfg.TracesHost.Valid {
		c.TracesHost = cfg.TracesHost
	}
	if cfg.TracesPushInterval.Valid {
		c.TracesPushInterval = cfg.TracesPushInterval
	}
	if cfg.TracesPushConcurrency.Valid {
		c.TracesPushConcurrency = cfg.TracesPushConcurrency
	}
	if cfg.AggregationPeriod.Valid {
		c.AggregationPeriod = cfg.AggregationPeriod
	}
	if cfg.AggregationWaitPeriod.Valid {
		c.AggregationWaitPeriod = cfg.AggregationWaitPeriod
	}
	return c
}

// GetConsolidatedConfig combines the default config values with the JSON config
// values and environment variables and returns the final result.
// it also returns a warning message that could be shown to the user.
// to bring some attention to the fact that the user.
func GetConsolidatedConfig(
	jsonRawConf json.RawMessage,
	env map[string]string,
	configArg string,
	cloudConfig json.RawMessage,
	external map[string]json.RawMessage,
) (Config, string, error) {
	warn := ""

	result := NewConfig()
	if jsonRawConf != nil {
		jsonConf := Config{}
		if err := json.Unmarshal(jsonRawConf, &jsonConf); err != nil {
			return result, warn, err
		}
		result = result.Apply(jsonConf)
	}

	if err := mergeFromCloudOptionAndExternal(cloudConfig, external, &result); err != nil {
		return result, warn, err
	}

	// We want to show a warning if the user is using the only old way of defining the config.
	// Note: Since the migration to the options.cloud is a long process, this warning is planned
	// to be emitted for a long time (1-2 years), after some point, and depending on the state
	// of migration we could re-evaluate this warning.
	if cloudConfig == nil && external != nil {
		if _, ok := external[LegacyCloudConfigKey]; ok {
			warn = "The options.ext.loadimpact option is deprecated, please use options.cloud instead"
		}
	}

	envConfig := Config{}
	if err := envconfig.Process("", &envConfig, func(key string) (string, bool) {
		v, ok := env[key]
		return v, ok
	}); err != nil {
		// TODO: get rid of envconfig and actually use the env parameter...
		return result, warn, err
	}
	result = result.Apply(envConfig)

	if configArg != "" {
		result.Name = null.StringFrom(configArg)
	}

	return result, warn, nil
}

// mergeFromCloudOptionAndExternal merges three fields from the JSON in a cloud key of
// the provided external map. Used for options.cloud settings.
func mergeFromCloudOptionAndExternal(
	cloudConfig json.RawMessage,
	external map[string]json.RawMessage,
	conf *Config,
) error {
	source := pickSource(cloudConfig, external)
	if source == nil {
		return nil
	}

	// Original comment
	// TODO: Important! Separate configs and fix the whole 2 configs mess!
	tmpConfig := Config{}
	if err := json.Unmarshal(source, &tmpConfig); err != nil {
		return err
	}
	// Only take out the ProjectID, Name and Token from the options.cloud (or legacy loadimpact struct) map:
	if tmpConfig.ProjectID.Valid {
		conf.ProjectID = tmpConfig.ProjectID
	}
	if tmpConfig.Name.Valid {
		conf.Name = tmpConfig.Name
	}
	if tmpConfig.Token.Valid {
		conf.Token = tmpConfig.Token
	}

	return nil
}

// GetTemporaryCloudConfig returns a temporary cloud config.
// Original comment
// TODO: Fix this
// We reuse cloud.Config for parsing options.cloud (or legacy loadimpact struct), but this probably shouldn't be
// done, as the idea of options.ext is that they are extensible without touching k6. But in
// order for this to happen, we shouldn't actually marshal cloud.Config on top of it, because
// it will be missing some fields that aren't actually mentioned in the struct.
// So in order for use to copy the fields that we need for k6 cloud's api we unmarshal in
// map[string]interface{} and copy what we need if it isn't set already
func GetTemporaryCloudConfig(
	cloudConfig json.RawMessage,
	external map[string]json.RawMessage,
) (map[string]interface{}, error) {
	tmpCloudConfig := make(map[string]interface{}, 3)

	source := pickSource(cloudConfig, external)
	if source == nil {
		return tmpCloudConfig, nil
	}

	dec := json.NewDecoder(bytes.NewReader(source))
	dec.UseNumber() // otherwise float64 are used
	if err := dec.Decode(&tmpCloudConfig); err != nil {
		return nil, err
	}

	return tmpCloudConfig, nil
}

// pickSource returns the config source to use.
func pickSource(
	cloudConfig json.RawMessage,
	external map[string]json.RawMessage,
) json.RawMessage {
	// priority is the new way of defining the config
	// via options.cloud
	if cloudConfig != nil {
		return cloudConfig
	}

	// fallback to the old way of defining the config
	if val, ok := external[LegacyCloudConfigKey]; ok {
		return val
	}

	return nil
}
