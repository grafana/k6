package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/cloudapi"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/internal/build"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
)

const (
	defaultTestName = "k6 test"
	testRunIDKey    = "K6_CLOUDRUN_TEST_RUN_ID"
)

// createCloudTest performs some test and Cloud configuration validations and if everything
// looks good, then it creates a test run in the k6 Cloud, using the Cloud API, meant to be used
// for streaming test results.
//
// This method is also responsible for filling the test run id on the test environment, so it can be used later,
// and to populate the Cloud configuration back in case the Cloud API returned some overrides,
// as expected by the Cloud output.
//
//nolint:funlen
func createCloudTest(gs *state.GlobalState, test *loadedAndConfiguredTest) error {
	// Otherwise, we continue normally with the creation of the test run in the k6 Cloud backend services.
	conf, warn, err := cloudapi.GetConsolidatedConfig(
		test.derivedConfig.Collectors[builtinOutputCloud.String()],
		gs.Env,
		"", // Historically used for -o cloud=..., no longer used (deprecated).
		test.derivedConfig.Options.Cloud,
		test.derivedConfig.Options.External,
	)
	if err != nil {
		return err
	}

	if warn != "" {
		gs.Logger.Warn(warn)
	}

	// If not, we continue with some validations and the creation of the test run.
	if err := validateRequiredSystemTags(test.derivedConfig.Options.SystemTags); err != nil {
		return err
	}

	if !conf.Name.Valid || conf.Name.String == "" {
		scriptPath := test.source.URL.String()
		if scriptPath == "" {
			// Script from stdin without a name, likely from stdin
			return errors.New("script name not set, please specify K6_CLOUD_NAME or options.cloud.name")
		}

		conf.Name = null.StringFrom(filepath.Base(scriptPath))
	}
	if conf.Name.String == "-" {
		conf.Name = null.StringFrom(defaultTestName)
	}

	thresholds := make(map[string][]string)
	for name, t := range test.derivedConfig.Thresholds {
		for _, threshold := range t.Thresholds {
			thresholds[name] = append(thresholds[name], threshold.Source)
		}
	}

	et, err := lib.NewExecutionTuple(
		test.derivedConfig.Options.ExecutionSegment,
		test.derivedConfig.Options.ExecutionSegmentSequence,
	)
	if err != nil {
		return err
	}
	executionPlan := test.derivedConfig.Options.Scenarios.GetFullExecutionRequirements(et)

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

	var testArchive *lib.Archive
	if !test.derivedConfig.NoArchiveUpload.Bool {
		testArchive = test.initRunner.MakeArchive()
	}

	testRun := &cloudapi.TestRun{
		Name:       conf.Name.String,
		ProjectID:  conf.ProjectID.Int64,
		VUsMax:     int64(lib.GetMaxPossibleVUs(executionPlan)), //nolint:gosec
		Thresholds: thresholds,
		Duration:   int64(duration / time.Second),
		Archive:    testArchive,
	}

	logger := gs.Logger.WithFields(logrus.Fields{"output": builtinOutputCloud.String()})

	apiClient := cloudapi.NewClient(
		logger, conf.Token.String, conf.Host.String, build.Version, conf.Timeout.TimeDuration())

	response, err := apiClient.CreateTestRun(testRun)
	if err != nil {
		return err
	}

	// We store the test run id in the environment, so it can be used later.
	test.preInitState.RuntimeOptions.Env[testRunIDKey] = response.ReferenceID

	// If the Cloud API returned configuration overrides, we apply them to the current configuration.
	// Then, we serialize the overridden configuration back, so it can be used by the Cloud output.
	if response.ConfigOverride != nil {
		logger.WithFields(logrus.Fields{"override": response.ConfigOverride}).Debug("overriding config options")

		raw, err := cloudConfToRawMessage(conf.Apply(*response.ConfigOverride))
		if err != nil {
			return fmt.Errorf("could not serialize overridden cloud configuration: %w", err)
		}

		if test.derivedConfig.Collectors == nil {
			test.derivedConfig.Collectors = make(map[string]json.RawMessage)
		}
		test.derivedConfig.Collectors[builtinOutputCloud.String()] = raw
	}

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
