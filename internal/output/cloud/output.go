// Package cloud implements an Output that flushes to the k6 Cloud platform.
package cloud

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/v2/cloudapi"
	"go.k6.io/k6/v2/errext"
	"go.k6.io/k6/v2/internal/build"
	v6cloudapi "go.k6.io/k6/v2/internal/cloudapi/v6"
	"go.k6.io/k6/v2/internal/usage"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/metrics"
	"go.k6.io/k6/v2/output"
	cloudv2 "go.k6.io/k6/v2/output/cloud/expv2"
)

const (
	defaultTestName = "k6 test"
	testRunIDKey    = "K6_CLOUDRUN_TEST_RUN_ID"
)

// versionedOutput represents an output implementing
// metrics samples aggregation and flushing to the
// Cloud remote service.
//
// It mainly differs from output.Output
// because it does not define Stop (that is deprecated)
// and Description.
type versionedOutput interface {
	Start() error
	StopWithTestError(testRunErr error) error

	SetTestRunStopCallback(func(error))
	SetTestRunID(id string)

	AddMetricSamples(samples []metrics.SampleContainer)
}

type apiVersion int64

const (
	apiVersionUndefined apiVersion = iota
	apiVersion1                    // disabled
	apiVersion2
)

// Output sends result data to the k6 Cloud service.
type Output struct {
	versionedOutput

	ctx       context.Context
	logger    logrus.FieldLogger
	config    cloudapi.Config
	testRunID string

	executionPlan []lib.ExecutionStep
	duration      int64 // in seconds
	thresholds    map[string][]*metrics.Threshold

	// testArchive is the test archive to be uploaded to the cloud
	// before the output is Start()-ed.
	//
	// It is set by the SetArchive method. If it is nil, the output
	// will not upload any test archive, such as when the user
	// uses the --no-archive-upload flag.
	testArchive *lib.Archive

	client       *cloudapi.Client
	v6Client     *v6cloudapi.Client // non-nil only on the provisioning path
	testStopFunc func(error)

	usage *usage.Usage
}

// Verify that Output implements the wanted interfaces
var _ interface {
	output.WithStopWithTestError
	output.WithThresholds
	output.WithTestRunStop
} = &Output{}

// New creates a new cloud output.
func New(params output.Params) (output.Output, error) {
	return newOutput(params)
}

// New creates a new cloud output.
func newOutput(params output.Params) (*Output, error) {
	conf, warn, err := cloudapi.GetConsolidatedConfig(
		params.JSONConfig,
		params.Environment,
		params.ConfigArgument,
		params.ScriptOptions.Cloud,
	)
	if err != nil {
		return nil, err
	}

	if warn != "" {
		params.Logger.Warn(warn)
	}

	if err := validateRequiredSystemTags(params.ScriptOptions.SystemTags); err != nil {
		return nil, err
	}

	logger := params.Logger.WithFields(logrus.Fields{"output": "cloud"})

	if !conf.Name.Valid || conf.Name.String == "" {
		scriptPath := params.ScriptPath.String()
		if scriptPath == "" {
			// Script from stdin without a name, likely from stdin
			return nil, errors.New("script name not set, please specify K6_CLOUD_NAME or options.cloud.name")
		}

		conf.Name = null.StringFrom(filepath.Base(scriptPath))
	}
	if conf.Name.String == "-" {
		conf.Name = null.StringFrom(defaultTestName)
	}

	duration, testEnds := lib.GetEndOffset(params.ExecutionPlan)
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

	apiClient := cloudapi.NewClient(
		logger, conf.Token.String, conf.Host.String, build.Version, conf.Timeout.TimeDuration())

	ctx := params.Ctx
	if ctx == nil {
		ctx = context.Background()
	}

	return &Output{
		ctx:           ctx,
		config:        conf,
		client:        apiClient,
		executionPlan: params.ExecutionPlan,
		duration:      int64(duration / time.Second),
		logger:        logger,
		usage:         params.Usage,
		testRunID:     params.RuntimeOptions.Env[testRunIDKey],
	}, nil
}

// validateRequiredSystemTags checks if all required tags are present.
func validateRequiredSystemTags(scriptTags *metrics.SystemTagSet) error {
	missingRequiredTags := []string{}
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

// Start calls the k6 Cloud API to initialize the test run, and then starts the
// goroutine that would listen for metric samples and send them to the cloud.
func (out *Output) Start() error {
	// TODO: PushRefID is set by k6-operator when it passes a pre-existing
	// test_run_id. This branch preserves legacy v1 behaviour until k6-operator
	// is updated to supply runtime_config fields (grafana/k6-cloud#4283).
	if out.config.PushRefID.Valid {
		out.testRunID = out.config.PushRefID.String
	}

	if out.testRunID != "" {
		out.logger.WithField("testRunId", out.testRunID).Debug("Directly pushing metrics without init")
		// When the test run ID was provided externally by the provisioning path
		// (K6_CLOUDRUN_TEST_RUN_ID, set by createCloudTest / k6 cloud run --local-execution),
		// create a v6 client so that testFinished() uses NotifyTestRunCompleted instead of
		// the legacy POST /v1/tests/{id}. We detect the provisioning path by the presence
		// of a valid stack ID — the PLZ/PushRefID legacy paths do not set it.
		if !out.config.PushRefID.Valid && out.config.StackID.Valid && out.config.StackID.Int64 != 0 {
			v6c, err := v6cloudapi.NewClient(
				out.logger, out.config.Token.String, out.config.Hostv6.String,
				build.Version, out.config.Timeout.TimeDuration())
			if err != nil {
				out.logger.WithError(err).Warn("Could not create v6 client; falling back to legacy testFinished")
			} else {
				v6c.SetStackID(int32(out.config.StackID.Int64)) //nolint:gosec
				out.v6Client = v6c
			}
		}
		return out.startVersionedOutput()
	}

	// Provisioning path: requires stack ID
	if !out.config.StackID.Valid || out.config.StackID.Int64 == 0 {
		return fmt.Errorf("K6_CLOUD_STACK_ID is required for k6 run --out cloud")
	}

	v6c, err := v6cloudapi.NewClient(
		out.logger, out.config.Token.String, out.config.Hostv6.String,
		build.Version, out.config.Timeout.TimeDuration())
	if err != nil {
		return fmt.Errorf("failed to create v6 cloud client: %w", err)
	}
	v6c.SetStackID(int32(out.config.StackID.Int64)) //nolint:gosec

	params := v6cloudapi.ProvisionParams{
		Name:          out.config.Name.String,
		ProjectID:     int32(out.config.ProjectID.Int64),               //nolint:gosec
		MaxVUs:        int64(lib.GetMaxPossibleVUs(out.executionPlan)), //nolint:gosec
		TotalDuration: out.duration,
		Options:       map[string]any{},
		Archive:       out.testArchive, // nil if --no-archive-upload (hardcoded nil on this path)
	}

	result, err := v6c.ProvisionLocalExecution(out.ctx, params)
	if err != nil {
		return fmt.Errorf("provisioning local execution failed: %w", err)
	}

	out.testRunID = strconv.FormatInt(int64(result.TestRunID), 10)
	out.config.MetricsPushURL = null.StringFrom(result.RuntimeConfig.Metrics.PushURL)
	out.config.TestRunToken = null.StringFrom(result.RuntimeConfig.TestRunToken)
	out.v6Client = v6c

	err = out.startVersionedOutput()
	if err != nil {
		return fmt.Errorf("the Gateway Output failed to start a versioned output: %w", err)
	}

	out.logger.WithFields(logrus.Fields{
		"name":      out.config.Name,
		"projectId": out.config.ProjectID,
		"duration":  out.duration,
		"testRunId": out.testRunID,
	}).Debug("Started!")
	return nil
}

// Description returns the URL with the test run results.
func (out *Output) Description() string {
	return fmt.Sprintf("cloud (%s)", cloudapi.URLForResults(out.testRunID, out.config))
}

// SetThresholds receives the thresholds before the output is Start()-ed.
func (out *Output) SetThresholds(scriptThresholds map[string]metrics.Thresholds) {
	thresholds := make(map[string][]*metrics.Threshold)
	for name, t := range scriptThresholds {
		thresholds[name] = append(thresholds[name], t.Thresholds...)
	}
	out.thresholds = thresholds
}

// SetTestRunStopCallback receives the function that stops the engine on error
func (out *Output) SetTestRunStopCallback(stopFunc func(error)) {
	out.testStopFunc = stopFunc
}

// SetArchive receives the test artifact to be uploaded to the cloud
// before the output is Start()-ed.
func (out *Output) SetArchive(archive *lib.Archive) {
	out.testArchive = archive
}

var _ output.WithArchive = &Output{}

// Stop gracefully stops all metric emission from the output: when all metric
// samples are emitted, it makes a cloud API call to finish the test run.
//
// Deprecated: use StopWithTestError() instead.
func (out *Output) Stop() error {
	return out.StopWithTestError(nil)
}

// StopWithTestError gracefully stops all metric emission from the output: when
// all metric samples are emitted, it makes a cloud API call to finish the test
// run. If testErr was specified, it extracts the RunStatus from it.
func (out *Output) StopWithTestError(testErr error) error {
	err := out.versionedOutput.StopWithTestError(testErr)
	if err != nil {
		out.logger.WithError(err).Error("An error occurred stopping the output")
		// to notify the cloud backend we have no return here
	}

	out.logger.Debug("Metric emission stopped, calling cloud API...")
	err = out.testFinished(testErr)
	if err != nil {
		out.logger.WithFields(logrus.Fields{"error": err}).Warn("Failed to send test finished to the cloud")
		return err
	}
	out.logger.Debug("Cloud output successfully stopped!")
	return nil
}

func (out *Output) testFinished(testErr error) error {
	if out.testRunID == "" || out.config.PushRefID.Valid {
		return nil
	}

	if out.v6Client != nil {
		// Provisioning path: notify via v6 API
		testRunID64, err := strconv.ParseInt(out.testRunID, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid test run ID %q: %w", out.testRunID, err)
		}
		return out.v6Client.NotifyTestRunCompleted(out.ctx, int32(testRunID64), testErr) //nolint:gosec
	}

	// Legacy v1 path (testRunID set externally, no provisioning client)
	testTainted := false
	thresholdResults := make(cloudapi.ThresholdResult)
	for name, thresholds := range out.thresholds {
		thresholdResults[name] = make(map[string]bool)
		for _, t := range thresholds {
			thresholdResults[name][t.Source] = t.LastFailed
			if t.LastFailed {
				testTainted = true
			}
		}
	}

	runStatus := out.getRunStatus(testErr)
	out.logger.WithFields(logrus.Fields{
		"ref":        out.testRunID,
		"tainted":    testTainted,
		"run_status": runStatus,
	}).Debug("Sending test finished")

	return out.client.TestFinished(out.testRunID, thresholdResults, testTainted, runStatus)
}

// getRunStatus determines the run status of the test based on the error.
func (out *Output) getRunStatus(testErr error) cloudapi.RunStatus {
	if testErr == nil {
		return cloudapi.RunStatusFinished
	}

	var err errext.HasAbortReason
	if errors.As(testErr, &err) {
		abortReason := err.AbortReason()
		switch abortReason {
		case errext.AbortedByUser:
			return cloudapi.RunStatusAbortedUser
		case errext.AbortedByThreshold:
			return cloudapi.RunStatusAbortedThreshold
		case errext.AbortedByScriptError:
			return cloudapi.RunStatusAbortedScriptError
		case errext.AbortedByScriptAbort:
			return cloudapi.RunStatusAbortedUser // TODO: have a better value than this?
		case errext.AbortedByTimeout:
			return cloudapi.RunStatusAbortedLimit
		case errext.AbortedByOutput:
			return cloudapi.RunStatusAbortedSystem
		case errext.AbortedByThresholdsAfterTestEnd:
			// The test run finished normally, it wasn't prematurely aborted by
			// anything while running, but the thresholds failed at the end and
			// k6 will return an error and a non-zero exit code to the user.
			//
			// However, failures are tracked somewhat differently by the k6
			// cloud compared to k6 OSS. It doesn't have a single pass/fail
			// variable with multiple failure states, like k6's exit codes.
			// Instead, it has two variables, result_status and run_status.
			//
			// The status of the thresholds is tracked by the binary
			// result_status variable, which signifies whether the thresholds
			// passed or failed (failure also called "tainted" in some places of
			// the API here). The run_status signifies whether the test run
			// finished normally and has a few fixed failures values.
			//
			// So, this specific k6 error will be communicated to the cloud only
			// via result_status, while the run_status will appear normal.
			return cloudapi.RunStatusFinished
		}
	}

	// By default, the catch-all error is "aborted by system", but let's log that
	out.logger.WithError(testErr).Debug("unknown test error classified as 'aborted by system'")
	return cloudapi.RunStatusAbortedSystem
}

func (out *Output) startVersionedOutput() error {
	if out.testRunID == "" {
		return errors.New("TestRunID is required")
	}
	var err error

	usageErr := out.usage.Strings("cloud/test_run_id", out.testRunID)
	if usageErr != nil {
		out.logger.Warning("Couldn't report test run id to usage as part of writing to k6 cloud")
	}

	// TODO: move here the creation of a new cloudapi.Client
	// so in the case the config has been overwritten the client uses the correct
	// value.
	//
	// This logic is handled individually by each single output, it has the downside
	// that we could break the logic and not catch easly it.

	switch out.config.APIVersion.Int64 {
	case int64(apiVersion1):
		err = errors.New("v1 is not supported anymore")
	case int64(apiVersion2):
		out.versionedOutput, err = cloudv2.New(out.logger, out.config, out.client)
	default:
		err = fmt.Errorf("v%d is an unexpected version", out.config.APIVersion.Int64)
	}

	if err != nil {
		return err
	}

	out.SetTestRunID(out.testRunID)
	out.versionedOutput.SetTestRunStopCallback(out.testStopFunc)
	return out.versionedOutput.Start()
}
