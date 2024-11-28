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
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/consts"
	"go.k6.io/k6/metrics"
)

const defaultTestName = "k6 test"

func isCloudOutput(outputType string) bool {
	return outputType == builtinOutputCloud.String()
}

// createCloudTest performs some test and Cloud configuration validations and if everything
// looks good, then it creates a test run in the k6 Cloud, unless k6 is already running in the Cloud.
// It is also responsible for filling the test run id on the test options, so it can be used later.
// It returns the resulting Cloud configuration as a json.RawMessage, as expected by the Cloud output,
// or an error if something goes wrong.
func createCloudTest(
	gs *state.GlobalState, test *loadedAndConfiguredTest, executionPlan []lib.ExecutionStep,
	outputType, outputArg string,
) (json.RawMessage, error) {
	conf, warn, err := cloudapi.GetConsolidatedConfig(
		test.derivedConfig.Collectors[outputType],
		gs.Env,
		outputArg,
		test.derivedConfig.Options.Cloud,
		test.derivedConfig.Options.External,
	)
	if err != nil {
		return nil, err
	}

	if warn != "" {
		gs.Logger.Warn(warn)
	}

	logger := gs.Logger.WithFields(logrus.Fields{"output": "cloud"})

	if err := validateRequiredSystemTags(test.derivedConfig.Options.SystemTags); err != nil {
		return nil, err
	}

	if !conf.Name.Valid || conf.Name.String == "" {
		scriptPath := test.source.URL.String()
		if scriptPath == "" {
			// Script from stdin without a name, likely from stdin
			return nil, errors.New("script name not set, please specify K6_CLOUD_NAME or options.cloud.name")
		}

		conf.Name = null.StringFrom(filepath.Base(scriptPath))
	}
	if conf.Name.String == "-" {
		conf.Name = null.StringFrom(defaultTestName)
	}

	// We need to propagate the test run id to the derived config
	// and to the init runner options, so they can be used later.
	setTestRunId := func(id null.String) error {
		// TODO: Not sure if this is a good idea :thinking:
		runnerOpts := test.initRunner.GetOptions()
		runnerOpts.TestRunID = id
		err := test.initRunner.SetOptions(runnerOpts)
		if err != nil {
			return err
		}

		test.derivedConfig.TestRunID = id
		return nil
	}

	// If this is true, then it means that this code is being executed in the k6 Cloud.
	// Therefore, we don't need to continue with the test run creation,
	// as we don't need to create any test run.
	// Precisely, the identifier of the test run is conf.PushRefID.
	if conf.PushRefID.Valid {
		if err := setTestRunId(conf.PushRefID); err != nil {
			return nil, err
		}
		return cloudConfToRawMessage(conf)
	}

	// If not, we continue with the creation of the test run.
	thresholds := make(map[string][]string)
	for name, t := range test.derivedConfig.Thresholds {
		for _, threshold := range t.Thresholds {
			thresholds[name] = append(thresholds[name], threshold.Source)
		}
	}

	duration, testEnds := lib.GetEndOffset(executionPlan)
	if !testEnds {
		return nil, errors.New("tests with unspecified duration are not allowed when outputting data to k6 cloud")
	}

	if conf.MetricPushConcurrency.Int64 < 1 {
		return nil, fmt.Errorf("metrics push concurrency must be a positive number but is %d",
			conf.MetricPushConcurrency.Int64)
	}

	if conf.MaxTimeSeriesInBatch.Int64 < 1 {
		return nil, fmt.Errorf("max allowed number of time series in a single batch must be a positive number but is %d",
			conf.MaxTimeSeriesInBatch.Int64)
	}

	var testArchive *lib.Archive
	if !test.derivedConfig.NoArchiveUpload.Bool {
		testArchive = test.initRunner.MakeArchive()
	}

	testRun := &cloudapi.TestRun{
		Name:       conf.Name.String,
		ProjectID:  conf.ProjectID.Int64,
		VUsMax:     int64(lib.GetMaxPossibleVUs(executionPlan)),
		Thresholds: thresholds,
		Duration:   int64(duration / time.Second),
		Archive:    testArchive,
	}

	apiClient := cloudapi.NewClient(
		logger, conf.Token.String, conf.Host.String, consts.Version, conf.Timeout.TimeDuration())

	response, err := apiClient.CreateTestRun(testRun)
	if err != nil {
		return nil, err
	}

	if response.ConfigOverride != nil {
		logger.WithFields(logrus.Fields{"override": response.ConfigOverride}).Debug("overriding config options")
		conf = conf.Apply(*response.ConfigOverride)
	}

	testRunId := null.NewString(response.ReferenceID, true)
	if err := setTestRunId(testRunId); err != nil {
		return nil, err
	}

	return cloudConfToRawMessage(conf)
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
