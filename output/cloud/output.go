// Package cloud implements an Output that flushes to the k6 Cloud platform.
package cloud

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"go.k6.io/k6/cloudapi"
	"go.k6.io/k6/errext"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/consts"
	"go.k6.io/k6/metrics"
	"go.k6.io/k6/output"
	cloudv2 "go.k6.io/k6/output/cloud/expv2"
	cloudv1 "go.k6.io/k6/output/cloud/v1"
	"gopkg.in/guregu/null.v3"
)

// TestName is the default k6 Cloud test name
const TestName = "k6 test"

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
	apiVersion1
	apiVersion2
)

// Output sends result data to the k6 Cloud service.
type Output struct {
	versionedOutput

	logger    logrus.FieldLogger
	config    cloudapi.Config
	testRunID string

	executionPlan []lib.ExecutionStep
	duration      int64 // in seconds
	thresholds    map[string][]*metrics.Threshold

	client       *cloudapi.Client
	testStopFunc func(error)
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
	conf, err := cloudapi.GetConsolidatedConfig(
		params.JSONConfig, params.Environment, params.ConfigArgument, params.ScriptOptions.External)
	if err != nil {
		return nil, err
	}

	if err := validateRequiredSystemTags(params.ScriptOptions.SystemTags); err != nil {
		return nil, err
	}

	logger := params.Logger.WithFields(logrus.Fields{"output": "cloud"})

	if !conf.Name.Valid || conf.Name.String == "" {
		scriptPath := params.ScriptPath.String()
		if scriptPath == "" {
			// Script from stdin without a name, likely from stdin
			return nil, errors.New("script name not set, please specify K6_CLOUD_NAME or options.ext.loadimpact.name")
		}

		conf.Name = null.StringFrom(filepath.Base(scriptPath))
	}
	if conf.Name.String == "-" {
		conf.Name = null.StringFrom(TestName)
	}

	duration, testEnds := lib.GetEndOffset(params.ExecutionPlan)
	if !testEnds {
		return nil, errors.New("tests with unspecified duration are not allowed when outputting data to k6 cloud")
	}

	if conf.MetricPushConcurrency.Int64 < 1 {
		return nil, fmt.Errorf("metrics push concurrency must be a positive number but is %d",
			conf.MetricPushConcurrency.Int64)
	}

	if conf.MaxMetricSamplesPerPackage.Int64 < 1 {
		return nil, fmt.Errorf("metric samples per package must be a positive number but is %d",
			conf.MaxMetricSamplesPerPackage.Int64)
	}

	if conf.MaxTimeSeriesInBatch.Int64 < 1 {
		return nil, fmt.Errorf("max allowed number of time series in a single batch must be a positive number but is %d",
			conf.MaxTimeSeriesInBatch.Int64)
	}

	apiClient := cloudapi.NewClient(
		logger, conf.Token.String, conf.Host.String, consts.Version, conf.Timeout.TimeDuration())

	return &Output{
		config:        conf,
		client:        apiClient,
		executionPlan: params.ExecutionPlan,
		duration:      int64(duration / time.Second),
		logger:        logger,
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
	if out.config.PushRefID.Valid {
		out.testRunID = out.config.PushRefID.String
		out.logger.WithField("testRunId", out.testRunID).Debug("Directly pushing metrics without init")
		return out.startVersionedOutput()
	}

	thresholds := make(map[string][]string)
	for name, t := range out.thresholds {
		for _, threshold := range t {
			thresholds[name] = append(thresholds[name], threshold.Source)
		}
	}

	testRun := &cloudapi.TestRun{
		Name:       out.config.Name.String,
		ProjectID:  out.config.ProjectID.Int64,
		VUsMax:     int64(lib.GetMaxPossibleVUs(out.executionPlan)),
		Thresholds: thresholds,
		Duration:   out.duration,
	}

	response, err := out.client.CreateTestRun(testRun)
	if err != nil {
		return err
	}
	out.testRunID = response.ReferenceID

	if response.ConfigOverride != nil {
		out.logger.WithFields(logrus.Fields{
			"override": response.ConfigOverride,
		}).Debug("overriding config options")
		out.config = out.config.Apply(*response.ConfigOverride)
	}

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

	// TODO: move here the creation of a new cloudapi.Client
	// so in the case the config has been overwritten the client uses the correct
	// value.
	//
	// This logic is handled individually by each single output, it has the downside
	// that we could break the logic and not catch easly it.

	switch out.config.APIVersion.Int64 {
	case int64(apiVersion1):
		out.versionedOutput, err = cloudv1.New(out.logger, out.config, out.client)
	case int64(apiVersion2):
		out.versionedOutput, err = cloudv2.New(out.logger, out.config, out.client)
	default:
		err = fmt.Errorf("v%d is an unexpected version", out.config.APIVersion.Int64)
	}

	if err != nil {
		return err
	}

	out.versionedOutput.SetTestRunID(out.testRunID)
	out.versionedOutput.SetTestRunStopCallback(out.testStopFunc)
	return out.versionedOutput.Start()
}
