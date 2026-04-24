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
	cloudapiv6 "go.k6.io/k6/v2/internal/cloudapi/v6"
	cloudsecrets "go.k6.io/k6/v2/internal/secretsource/cloud"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/metrics"
)

const (
	defaultTestName = "k6 test"
	testRunIDKey    = "K6_CLOUDRUN_TEST_RUN_ID"
)

// createCloudTest performs some test and Cloud configuration validations and if everything
// looks good, then it provisions a test run in the k6 Cloud using the provisioning API,
// meant to be used for streaming test results.
//
// This method is also responsible for filling the test run id on the test environment, so it can be used later,
// and to populate the Cloud configuration back with the values returned by the provisioning API,
// as expected by the Cloud output.
//
//nolint:funlen
func createCloudTest(gs *state.GlobalState, test *loadedAndConfiguredTest) error {
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

	// When a token is present but stack ID is absent, give a specific error so
	// the user knows exactly which variable to set, rather than the generic auth error.
	if conf.Token.Valid && conf.Token.String != "" && (!conf.StackID.Valid || conf.StackID.Int64 == 0) {
		return fmt.Errorf("K6_CLOUD_STACK_ID is required for --local-execution")
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

	logger := gs.Logger.WithFields(logrus.Fields{"output": builtinOutputCloud.String()})

	v6Client, err := cloudapiv6.NewClient(
		logger, conf.Token.String, conf.Hostv6.String, build.Version, conf.Timeout.TimeDuration())
	if err != nil {
		return err
	}
	v6Client.SetStackID(int32(conf.StackID.Int64)) //nolint:gosec

	var archive *lib.Archive
	if !test.derivedConfig.NoArchiveUpload.Bool {
		archive = test.makeArchive()
	}

	params := cloudapiv6.ProvisionParams{
		Name:          conf.Name.String,
		ProjectID:     int32(conf.ProjectID.Int64),                 //nolint:gosec
		MaxVUs:        int64(lib.GetMaxPossibleVUs(executionPlan)), //nolint:gosec
		TotalDuration: int64(duration / time.Second),
		Options:       map[string]any{},
		Archive:       archive,
	}

	result, err := v6Client.ProvisionLocalExecution(gs.Ctx, params)
	if err != nil {
		return err
	}

	// The provisioning API does not yet return SecretsConfig, so make a
	// separate POST /v1/tests call to obtain one. The v1 test run created
	// by this call is intentionally discarded; only SecretsConfig is used.
	// This bridge remains until the provisioning API exposes secrets config.
	if gs.CloudSecretSource != nil {
		v1Client := cloudapi.NewClient(
			logger, conf.Token.String, conf.Host.String, build.Version, conf.Timeout.TimeDuration())
		v1TestRun := &cloudapi.TestRun{
			Name:      conf.Name.String,
			ProjectID: conf.ProjectID.Int64,
			VUsMax:    params.MaxVUs,
			Duration:  params.TotalDuration,
		}
		if v1Resp, v1Err := v1Client.CreateTestRun(v1TestRun); v1Err != nil {
			logger.WithError(v1Err).Warn("cloud secret source: could not fetch SecretsConfig from v1 endpoint")
		} else if v1Resp.SecretsConfig != nil {
			gs.CloudSecretSource.SetConfig(&cloudsecrets.Config{
				Token:        result.RuntimeConfig.TestRunToken,
				Endpoint:     v1Resp.SecretsConfig.Endpoint,
				ResponsePath: v1Resp.SecretsConfig.ResponsePath,
			})
		}
	}

	// Store the test run id in the environment, so it can be used later.
	test.preInitState.RuntimeOptions.Env[testRunIDKey] = strconv.FormatInt(int64(result.TestRunID), 10)

	// Apply runtime config returned by the provisioning API.
	conf.MetricsPushURL = null.StringFrom(result.RuntimeConfig.Metrics.PushURL)
	conf.TestRunToken = null.StringFrom(result.RuntimeConfig.TestRunToken)
	if result.TestRunDetailsPageURL != "" {
		conf.TestRunDetails = null.StringFrom(result.TestRunDetailsPageURL)
	}

	raw, err := cloudConfToRawMessage(conf)
	if err != nil {
		return fmt.Errorf("could not serialize cloud configuration: %w", err)
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
