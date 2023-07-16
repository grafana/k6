package cloudapi

import (
	"encoding/json"
	"time"

	"github.com/mstoykov/envconfig"
	"go.k6.io/k6/lib/types"
	"gopkg.in/guregu/null.v3"
)

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

	// Defines the max allowed number of time series in a single batch.
	MaxTimeSeriesInBatch null.Int `json:"maxTimeSeriesInBatch" envconfig:"K6_CLOUD_MAX_TIME_SERIES_IN_BATCH"`

	// PushRefID represents the test run id.
	// Note: It is a legacy name used by the backend, the code in k6 open-source
	// references it as test run id.
	// Currently, a renaming is not planned.
	PushRefID null.String `json:"pushRefID" envconfig:"K6_CLOUD_PUSH_REF_ID"`

	// The time interval between periodic API calls for sending samples to the cloud ingest service.
	MetricPushInterval types.NullDuration `json:"metricPushInterval" envconfig:"K6_CLOUD_METRIC_PUSH_INTERVAL"`

	// This is how many concurrent pushes will be done at the same time to the cloud
	MetricPushConcurrency null.Int `json:"metricPushConcurrency" envconfig:"K6_CLOUD_METRIC_PUSH_CONCURRENCY"`

	// Indicates whether to send traces to the k6 Insights backend service.
	TracesEnabled null.Bool `json:"tracesEnabled" envconfig:"K6_CLOUD_TRACES_ENABLED"`

	// The host of the k6 Insights backend service.
	TracesHost null.String `json:"traceHost" envconfig:"K6_CLOUD_TRACES_HOST"`

	// This is how many concurrent pushes will be done at the same time to the cloud
	TracesPushConcurrency null.Int `json:"tracesPushConcurrency" envconfig:"K6_CLOUD_TRACES_PUSH_CONCURRENCY"`

	// The time interval between periodic API calls for sending samples to the cloud ingest service.
	TracesPushInterval types.NullDuration `json:"tracesPushInterval" envconfig:"K6_CLOUD_TRACES_PUSH_INTERVAL"`

	// Aggregation docs:
	//
	// If AggregationPeriod is specified and if it is greater than 0, HTTP metric aggregation
	// with that period will be enabled. The general algorithm is this:
	// - HTTP trail samples will be collected separately and not
	//   included in the default sample buffer (which is directly sent
	//   to the cloud service every MetricPushInterval).
	// - On every AggregationCalcInterval, all collected HTTP Trails will be
	//   split into AggregationPeriod-sized time buckets (time slots) and
	//   then into sub-buckets according to their tags (each sub-bucket
	//   will contain only HTTP trails with the same sample tags -
	//   proto, staus, URL, method, etc.).
	// - If at that time the specified AggregationWaitPeriod has not passed
	//   for a particular time bucket, it will be left undisturbed until the next
	//   AggregationCalcInterval tick comes along.
	// - If AggregationWaitPeriod has passed for a time bucket, all of its
	//   sub-buckets will be traversed. Any sub-buckets that have less than
	//   AggregationMinSamples HTTP trails in them will not be aggregated.
	//   Instead the HTTP trails in them will just be individually added
	//   to the default sample buffer, like they would be if there was no
	//   aggregation.
	// - Sub-buckets with at least AggregationMinSamples HTTP trails on the
	//   other hand will be aggregated according to the algorithm below:
	//     - If AggregationSkipOutlierDetection is enabled, all of the collected
	//       HTTP trails in that sub-bucket will be directly aggregated into a single
	//       compoind metric sample, without any attempt at outlier detection.
	//       IMPORTANT: This is intended only for testing purposes only or, in
	//       extreme cases, when the resulting metrics' precision isn't very important,
	//       since it could lead to a huge loss of granularity and the masking
	//       of any outlier samples in the data.
	//     - By default (since AggregationSkipOutlierDetection is not enabled),
	//       the collected HTTP trails will be checked for outliers, so we don't lose
	//       granularity by accidentally aggregating them. That happens by finding
	//       the "quartiles" (by default the 75th and 25th percentiles) in the
	//       sub-bucket datapoints and using the inter-quartile range (IQR) to find
	//       any outliers (https://en.wikipedia.org/wiki/Interquartile_range#Outliers,
	//       though the specific parameters and coefficients can be customized
	//       by the AggregationOutlier{Radius,CoefLower,CoefUpper} options)
	//     - Depending on the number of samples in the sub-bucket, two different
	//       algorithms could be used to calculate the quartiles. If there are
	//       fewer samples (between AggregationMinSamples and AggregationOutlierAlgoThreshold),
	//       then a more precise but also more computationally-heavy sorting-based algorithm
	//       will be used. For sub-buckets with more samples, a lighter quickselect-based
	//       algorithm will be used, potentially with a very minor loss of precision.
	//     - Regardless of the used algorithm, once the quartiles for that sub-bucket
	//       are found and the IQR is calculated, every HTTP trail in the sub-bucket will
	//       be checked if it seems like an outlier. HTTP trails are evaluated by two different
	//       criteria whether they seem like outliers - by their total connection time (i.e.
	//       http_req_connecting + http_req_tls_handshaking) and by their total request time
	//       (i.e. http_req_sending + http_req_waiting + http_req_receiving). If any of those
	//       properties of an HTTP trail is out of the calculated "normal" bounds for the
	//       sub-bucket, it will be considered an outlier and will be sent to the cloud
	//       individually - it's simply added to the default sample buffer, like it would
	//       be if there was no aggregation.
	//     - Finally, all non-outliers are aggregated and the resultig single metric is also
	//       added to the default sample buffer for sending to the cloud ingest service
	//       on the next MetricPushInterval event.

	// If specified and is greater than 0, sample aggregation with that period is enabled
	AggregationPeriod types.NullDuration `json:"aggregationPeriod" envconfig:"K6_CLOUD_AGGREGATION_PERIOD"`

	// If aggregation is enabled, this is how often new HTTP trails will be sorted into buckets and sub-buckets and aggregated.
	AggregationCalcInterval types.NullDuration `json:"aggregationCalcInterval" envconfig:"K6_CLOUD_AGGREGATION_CALC_INTERVAL"`

	// If aggregation is enabled, this specifies how long we'll wait for period samples to accumulate before trying to aggregate them.
	AggregationWaitPeriod types.NullDuration `json:"aggregationWaitPeriod" envconfig:"K6_CLOUD_AGGREGATION_WAIT_PERIOD"`

	// If aggregation is enabled, but the collected samples for a certain AggregationPeriod after AggregationPushDelay has passed are less than this number, they won't be aggregated.
	AggregationMinSamples null.Int `json:"aggregationMinSamples" envconfig:"K6_CLOUD_AGGREGATION_MIN_SAMPLES"`

	// If this is enabled and a sub-bucket has more than AggregationMinSamples HTTP trails in it, they would all be
	// aggregated without attempting to find and separate any outlier metrics first.
	// IMPORTANT: This is intended for testing purposes only or, in extreme cases, when the result precision
	// isn't very important and the improved aggregation percentage would be worth the potentially huge loss
	// of metric granularity and possible masking of any outlier samples.
	AggregationSkipOutlierDetection null.Bool `json:"aggregationSkipOutlierDetection" envconfig:"K6_CLOUD_AGGREGATION_SKIP_OUTLIER_DETECTION"`

	// If aggregation and outlier detection are enabled, this option specifies the
	// number of HTTP trails in a sub-bucket that determine which quartile-calculating
	// algorithm would be used:
	// - for fewer samples (between MinSamples and OutlierAlgoThreshold), a more precise
	//   (i.e. supporting interpolation), but also more computationally-heavy sorting
	//   algorithm will be used to find the quartiles.
	// - if there are more samples than OutlierAlgoThreshold in the sub-bucket, a
	//   QuickSelect-based (https://en.wikipedia.org/wiki/Quickselect) algorithm will
	//   be used. It doesn't support interpolation, so there's a small loss of precision
	//   in the outlier detection, but it's not as resource-heavy as the sorting algorithm.
	AggregationOutlierAlgoThreshold null.Int `json:"aggregationOutlierAlgoThreshold" envconfig:"K6_CLOUD_AGGREGATION_OUTLIER_ALGO_THRESHOLD"`

	// The radius (as a fraction) from the median at which to sample Q1 and Q3.
	// By default it's one quarter (0.25) and if set to something different, the Q in IQR
	// won't make much sense... But this would allow us to select tighter sample groups for
	// aggregation if we want.
	AggregationOutlierIqrRadius null.Float `json:"aggregationOutlierIqrRadius" envconfig:"K6_CLOUD_AGGREGATION_OUTLIER_IQR_RADIUS"`

	// Connection or request times with how many IQRs below Q1 to consier as non-aggregatable outliers.
	AggregationOutlierIqrCoefLower null.Float `json:"aggregationOutlierIqrCoefLower" envconfig:"K6_CLOUD_AGGREGATION_OUTLIER_IQR_COEF_LOWER"`

	// Connection or request times with how many IQRs above Q3 to consier as non-aggregatable outliers.
	AggregationOutlierIqrCoefUpper null.Float `json:"aggregationOutlierIqrCoefUpper" envconfig:"K6_CLOUD_AGGREGATION_OUTLIER_IQR_COEF_UPPER"`

	// Deprecated: Remove this when migration from the cloud output v1 will be completed
	MaxMetricSamplesPerPackage null.Int `json:"maxMetricSamplesPerPackage" envconfig:"K6_CLOUD_MAX_METRIC_SAMPLES_PER_PACKAGE"`
}

// NewConfig creates a new Config instance with default values for some fields.
func NewConfig() Config {
	return Config{
		Host:                  null.NewString("https://ingest.k6.io", false),
		LogsTailURL:           null.NewString("wss://cloudlogs.k6.io/api/v1/tail", false),
		WebAppURL:             null.NewString("https://app.k6.io", false),
		MetricPushInterval:    types.NewNullDuration(1*time.Second, false),
		MetricPushConcurrency: null.NewInt(1, false),

		TracesEnabled:         null.NewBool(false, false),
		TracesHost:            null.NewString("insights.k6.io:4443", false),
		TracesPushInterval:    types.NewNullDuration(1*time.Second, false),
		TracesPushConcurrency: null.NewInt(1, false),

		MaxMetricSamplesPerPackage: null.NewInt(100000, false),
		Timeout:                    types.NewNullDuration(1*time.Minute, false),
		APIVersion:                 null.NewInt(1, false),

		// The set value (1000) is selected for performance reasons.
		// Any change to this value should be first discussed with internal stakeholders.
		MaxTimeSeriesInBatch: null.NewInt(1000, false),

		// Aggregation is disabled by default, since AggregationPeriod has no default value
		// but if it's enabled manually or from the cloud service, those are the default values it will use:
		AggregationCalcInterval:         types.NewNullDuration(3*time.Second, false),
		AggregationWaitPeriod:           types.NewNullDuration(5*time.Second, false),
		AggregationMinSamples:           null.NewInt(25, false),
		AggregationOutlierAlgoThreshold: null.NewInt(75, false),
		AggregationOutlierIqrRadius:     null.NewFloat(0.25, false),

		// Since we're measuring durations, the upper coefficient is slightly
		// lower, since outliers from that side are more interesting than ones
		// close to zero.
		AggregationOutlierIqrCoefLower: null.NewFloat(1.5, false),
		AggregationOutlierIqrCoefUpper: null.NewFloat(1.3, false),
	}
}

// Apply saves config non-zero config values from the passed config in the receiver.
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
	if cfg.MaxMetricSamplesPerPackage.Valid {
		c.MaxMetricSamplesPerPackage = cfg.MaxMetricSamplesPerPackage
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
	if cfg.AggregationCalcInterval.Valid {
		c.AggregationCalcInterval = cfg.AggregationCalcInterval
	}
	if cfg.AggregationWaitPeriod.Valid {
		c.AggregationWaitPeriod = cfg.AggregationWaitPeriod
	}
	if cfg.AggregationMinSamples.Valid {
		c.AggregationMinSamples = cfg.AggregationMinSamples
	}
	if cfg.AggregationSkipOutlierDetection.Valid {
		c.AggregationSkipOutlierDetection = cfg.AggregationSkipOutlierDetection
	}
	if cfg.AggregationOutlierAlgoThreshold.Valid {
		c.AggregationOutlierAlgoThreshold = cfg.AggregationOutlierAlgoThreshold
	}
	if cfg.AggregationOutlierIqrRadius.Valid {
		c.AggregationOutlierIqrRadius = cfg.AggregationOutlierIqrRadius
	}
	if cfg.AggregationOutlierIqrCoefLower.Valid {
		c.AggregationOutlierIqrCoefLower = cfg.AggregationOutlierIqrCoefLower
	}
	if cfg.AggregationOutlierIqrCoefUpper.Valid {
		c.AggregationOutlierIqrCoefUpper = cfg.AggregationOutlierIqrCoefUpper
	}
	return c
}

// MergeFromExternal merges three fields from the JSON in a loadimpact key of
// the provided external map. Used for options.ext.loadimpact settings.
func MergeFromExternal(external map[string]json.RawMessage, conf *Config) error {
	if val, ok := external["loadimpact"]; ok {
		// TODO: Important! Separate configs and fix the whole 2 configs mess!
		tmpConfig := Config{}
		if err := json.Unmarshal(val, &tmpConfig); err != nil {
			return err
		}
		// Only take out the ProjectID, Name and Token from the options.ext.loadimpact map:
		if tmpConfig.ProjectID.Valid {
			conf.ProjectID = tmpConfig.ProjectID
		}
		if tmpConfig.Name.Valid {
			conf.Name = tmpConfig.Name
		}
		if tmpConfig.Token.Valid {
			conf.Token = tmpConfig.Token
		}
	}
	return nil
}

// GetConsolidatedConfig combines the default config values with the JSON config
// values and environment variables and returns the final result.
func GetConsolidatedConfig(
	jsonRawConf json.RawMessage, env map[string]string, configArg string, external map[string]json.RawMessage,
) (Config, error) {
	result := NewConfig()
	if jsonRawConf != nil {
		jsonConf := Config{}
		if err := json.Unmarshal(jsonRawConf, &jsonConf); err != nil {
			return result, err
		}
		result = result.Apply(jsonConf)
	}
	if err := MergeFromExternal(external, &result); err != nil {
		return result, err
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

	if configArg != "" {
		result.Name = null.StringFrom(configArg)
	}

	return result, nil
}
