package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/v2/cloudapi"
	"go.k6.io/k6/v2/cmd/state"
	"go.k6.io/k6/v2/internal/build"
	"go.k6.io/k6/v2/internal/cloudapi/provisioning"
	cloudsecrets "go.k6.io/k6/v2/internal/secretsource/cloud"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/lib/types"
	"go.k6.io/k6/v2/metrics"
)

const (
	defaultTestName = "k6 test"
	testRunIDKey    = "K6_CLOUDRUN_TEST_RUN_ID"
)

// createCloudTest performs test and Cloud configuration validations and prepares the test run.
// It creates a new test run in the k6 Cloud backend unless a PushRefID is provided,
// in which case it reuses an existing test run.
//
// This method also sets the test run ID in the environment so it can be used later,
// and applies any Cloud configuration overrides returned by the API.
//
//nolint:funlen
func createCloudTest(gs *state.GlobalState, test *loadedAndConfiguredTest) error {
	// Otherwise, we continue normally with the creation of the test run in the k6 Cloud backend services.
	conf, warn, err := cloudapi.GetConsolidatedConfig(
		test.derivedConfig.Collectors[builtinOutputCloud.String()],
		gs.Env,
		"", // Historically used for -o cloud=..., no longer used (deprecated).
		test.derivedConfig.Cloud,
	)
	if err != nil {
		return err
	}

	if warn != "" {
		gs.Logger.Warn(warn)
	}

	if err := checkCloudLogin(conf); err != nil {
		return err
	}

	// If not, we continue with some validations and the creation of the test run.
	if err := validateRequiredSystemTags(test.derivedConfig.SystemTags); err != nil {
		return err
	}

	if conf.Name, err = resolveCloudTestName(conf.Name, test.source.URL.String()); err != nil {
		return err
	}

	thresholds := make(map[string][]string)
	for name, t := range test.derivedConfig.Thresholds {
		for _, threshold := range t.Thresholds {
			thresholds[name] = append(thresholds[name], threshold.Source)
		}
	}

	et, err := lib.NewExecutionTuple(
		test.derivedConfig.ExecutionSegment,
		test.derivedConfig.ExecutionSegmentSequence,
	)
	if err != nil {
		return err
	}
	executionPlan := test.derivedConfig.Scenarios.GetFullExecutionRequirements(et)

	duration, testEnds := lib.GetEndOffset(executionPlan)
	if !testEnds {
		return errors.New("tests with unspecified duration are not allowed when outputting data to k6 cloud")
	}

	if conf.MetricPushConcurrency.Int64 < 1 {
		return fmt.Errorf("metrics push concurrency must be a positive number but is %d",
			conf.MetricPushConcurrency.Int64)
	}

	if conf.MaxTimeSeriesInBatch.Int64 < 1 {
		return fmt.Errorf("max allowed number of time series in a single batch must be a positive number but is %d",
			conf.MaxTimeSeriesInBatch.Int64)
	}

	if conf.PushRefID.Valid && conf.PushRefID.String != "" {
		test.preInitState.RuntimeOptions.Env[testRunIDKey] = conf.PushRefID.String
		gs.Logger.Debug("using PushRefID, skipping CreateTestRun")
		return nil
	}

	var testArchive *lib.Archive
	if !test.derivedConfig.NoArchiveUpload.Bool {
		testArchive = test.makeArchive()
	}

	logger := gs.Logger.WithFields(logrus.Fields{"output": builtinOutputCloud.String()})

	// The stack ID is required (checkCloudLogin enforces it) and passed as
	// int64; provisioning.NewClient handles the int32 boundary internally.
	provClient, err := provisioning.NewClient(
		logger, conf.Token.String, conf.Hostv6.String, build.Version,
		conf.StackID.Int64,
		conf.Timeout.TimeDuration(),
	)
	if err != nil {
		return err
	}

	if conf.ProjectID.Int64 == 0 {
		projectID, err := resolveDefaultProjectID(gs, &conf)
		if err != nil {
			return err
		}
		if projectID > 0 {
			conf.ProjectID = null.IntFrom(projectID)
		}
	}

	// Marshal the resolved lib.Options as json.RawMessage for the request
	// body's `options` field. No map[string]any round-trip — preserves
	// typed precision from lib.Options custom marshalers.
	optionsJSON, err := json.Marshal(test.derivedConfig.Options)
	if err != nil {
		return fmt.Errorf("marshalling options: %w", err)
	}

	ctx := gs.Ctx
	result, err := provClient.ProvisionLocalExecution(ctx, provisioning.ProvisionParams{
		Name:          conf.Name.String,
		ProjectID:     conf.ProjectID.Int64,
		MaxVUs:        int64(lib.GetMaxPossibleVUs(executionPlan)), //nolint:gosec
		TotalDuration: int64(duration / time.Second),
		Options:       json.RawMessage(optionsJSON),
		Archive:       testArchive,
	})
	if err != nil {
		return err
	}

	// Stash testRunID in env (existing pattern).
	test.preInitState.RuntimeOptions.Env[testRunIDKey] = strconv.FormatInt(result.TestRunID, 10)

	// Apply runtime_config metrics tuning to the Config. Malformed duration
	// strings from the backend are logged as warnings, not failures.
	conf = conf.Apply(buildConfigFromRuntimeConfig(logger, result.RuntimeConfig))

	// Set the Config fields that the cloud Output reads for metrics push.
	conf.MetricsPushURL = null.StringFrom(result.RuntimeConfig.Metrics.PushURL)
	conf.TestRunToken = null.StringFrom(result.RuntimeConfig.TestRunToken)

	// Set the test run details page URL for the output banner.
	conf.TestRunDetails = null.StringFrom(result.TestRunDetailsPageURL)

	// Register secrets config (always present in the provisioning response).
	if gs.CloudSecretSource != nil {
		gs.CloudSecretSource.SetConfig(&cloudsecrets.Config{
			Token:        result.RuntimeConfig.TestRunToken,
			Endpoint:     result.RuntimeConfig.Secrets.Endpoint,
			ResponsePath: result.RuntimeConfig.Secrets.ResponsePath,
		})
	}

	// Serialise overridden Config back into the collectors map (existing pattern).
	raw, err := cloudConfToRawMessage(conf)
	if err != nil {
		return fmt.Errorf("could not serialize overridden cloud configuration: %w", err)
	}
	if test.derivedConfig.Collectors == nil {
		test.derivedConfig.Collectors = make(map[string]json.RawMessage)
	}
	test.derivedConfig.Collectors[builtinOutputCloud.String()] = raw

	return nil
}

// validateRequiredSystemTags checks if all required tags are present.
func validateRequiredSystemTags(scriptTags *metrics.SystemTagSet) error {
	var missingRequiredTags []string
	requiredTags := metrics.SystemTagSet(metrics.TagName |
		metrics.TagMethod |
		metrics.TagStatus |
		metrics.TagError |
		metrics.TagCheck |
		metrics.TagGroup)
	for _, tag := range metrics.SystemTagValues() {
		if requiredTags.Has(tag) && !scriptTags.Has(tag) {
			missingRequiredTags = append(missingRequiredTags, tag.String())
		}
	}
	if len(missingRequiredTags) > 0 {
		return fmt.Errorf(
			"the cloud output needs the following system tags enabled: %s",
			strings.Join(missingRequiredTags, ", "),
		)
	}
	return nil
}

func cloudConfToRawMessage(conf cloudapi.Config) (json.RawMessage, error) {
	var buff bytes.Buffer
	enc := json.NewEncoder(&buff)
	if err := enc.Encode(conf); err != nil {
		return nil, err
	}
	return buff.Bytes(), nil
}

// resolveCloudTestName returns the test name from the config, or derives it from
// the script path when the config name is unset. A name of "-" is replaced
// with the default test name.
func resolveCloudTestName(name null.String, scriptPath string) (null.String, error) {
	if !name.Valid || name.String == "" {
		if scriptPath == "" {
			return name, errors.New("script name not set, please specify K6_CLOUD_NAME or options.cloud.name")
		}
		name = null.StringFrom(filepath.Base(scriptPath))
	}
	if name.String == "-" {
		name = null.StringFrom(defaultTestName)
	}
	return name, nil
}

// buildConfigFromRuntimeConfig maps provisioning RuntimeConfig fields
// to cloudapi.Config fields.
//
// If the backend returns an unparseable duration string, a warning is
// logged and the cloudapi.Config default is kept. Graceful degradation
// is preferable to blocking the user for a backend-side bug.
func buildConfigFromRuntimeConfig(
	logger logrus.FieldLogger, rc provisioning.RuntimeConfig,
) cloudapi.Config {
	cfg := cloudapi.Config{}

	// metrics.push_interval (string nullable) → MetricPushInterval (NullDuration)
	if v := rc.Metrics.PushInterval; v != nil {
		if d, err := time.ParseDuration(*v); err == nil {
			cfg.MetricPushInterval = types.NewNullDuration(d, true)
		} else {
			logger.WithError(err).WithField("field", "push_interval").
				Warn("invalid duration in runtime_config; using default")
		}
	}
	// metrics.push_concurrency (int32 nullable) → MetricPushConcurrency (null.Int)
	if v := rc.Metrics.PushConcurrency; v != nil {
		cfg.MetricPushConcurrency = null.IntFrom(int64(*v))
	}
	// metrics.aggregation_period (string nullable) → AggregationPeriod (NullDuration)
	if v := rc.Metrics.AggregationPeriod; v != nil {
		if d, err := time.ParseDuration(*v); err == nil {
			cfg.AggregationPeriod = types.NewNullDuration(d, true)
		} else {
			logger.WithError(err).WithField("field", "aggregation_period").
				Warn("invalid duration in runtime_config; using default")
		}
	}
	// metrics.aggregation_wait_period (string nullable) → AggregationWaitPeriod
	if v := rc.Metrics.AggregationWaitPeriod; v != nil {
		if d, err := time.ParseDuration(*v); err == nil {
			cfg.AggregationWaitPeriod = types.NewNullDuration(d, true)
		} else {
			logger.WithError(err).WithField("field", "aggregation_wait_period").
				Warn("invalid duration in runtime_config; using default")
		}
	}
	// metrics.max_samples_per_package (int32 nullable) → MaxTimeSeriesInBatch
	// (semantically the same field — per-batch series cap)
	if v := rc.Metrics.MaxSamplesPerPackage; v != nil {
		cfg.MaxTimeSeriesInBatch = null.IntFrom(int64(*v))
	}

	// rc.Metrics.AggregationMinSamples is intentionally not mapped.
	// expv2 has no min-samples aggregation knob; adding one would
	// require changes to output/cloud/expv2/{collect.go, flush.go}.

	return cfg
}
