// Package cloud implements an Output that flushes to the k6 Cloud platform.
package cloud

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/cloudapi"
	"go.k6.io/k6/errext"
	"go.k6.io/k6/output"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/consts"
	"go.k6.io/k6/lib/netext/httpext"
	"go.k6.io/k6/metrics"
)

// TestName is the default k6 Cloud test name
const TestName = "k6 test"

// Output sends result data to the k6 Cloud service.
type Output struct {
	logger      logrus.FieldLogger
	config      cloudapi.Config
	referenceID string

	executionPlan []lib.ExecutionStep
	duration      int64 // in seconds
	thresholds    map[string][]*metrics.Threshold
	client        *MetricsClient

	bufferMutex      sync.Mutex
	bufferHTTPTrails []*httpext.Trail
	bufferSamples    []*Sample

	// TODO: optimize this
	//
	// Since the real-time metrics refactoring (https://github.com/k6io/k6/pull/678),
	// we should no longer have to handle metrics that have times long in the past. So instead of a
	// map, we can probably use a simple slice (or even an array!) as a ring buffer to store the
	// aggregation buckets. This should save us a some time, since it would make the lookups and WaitPeriod
	// checks basically O(1). And even if for some reason there are occasional metrics with past times that
	// don't fit in the chosen ring buffer size, we could just send them along to the buffer unaggregated
	aggrBuckets map[int64]aggregationBucket

	stopSendingMetrics chan struct{}
	stopAggregation    chan struct{}
	aggregationDone    *sync.WaitGroup
	stopOutput         chan struct{}
	outputDone         *sync.WaitGroup
	testStopFunc       func(error)
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

	if !(conf.MetricPushConcurrency.Int64 > 0) {
		return nil, fmt.Errorf("metrics push concurrency must be a positive number but is %d",
			conf.MetricPushConcurrency.Int64)
	}

	if !(conf.MaxMetricSamplesPerPackage.Int64 > 0) {
		return nil, fmt.Errorf("metric samples per package must be a positive number but is %d",
			conf.MaxMetricSamplesPerPackage.Int64)
	}

	apiClient := cloudapi.NewClient(
		logger, conf.Token.String, conf.Host.String, consts.Version, conf.Timeout.TimeDuration())

	return &Output{
		config:        conf,
		client:        NewMetricsClient(apiClient, logger, conf.Host.String, conf.NoCompress.Bool),
		executionPlan: params.ExecutionPlan,
		duration:      int64(duration / time.Second),
		aggrBuckets:   map[int64]aggregationBucket{},
		logger:        logger,

		stopSendingMetrics: make(chan struct{}),
		stopAggregation:    make(chan struct{}),
		aggregationDone:    &sync.WaitGroup{},
		stopOutput:         make(chan struct{}),
		outputDone:         &sync.WaitGroup{},
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
		out.referenceID = out.config.PushRefID.String
		out.logger.WithField("referenceId", out.referenceID).Debug("directly pushing metrics without init")
		out.startBackgroundProcesses()
		return nil
	}

	thresholds := make(map[string][]string)

	for name, t := range out.thresholds {
		for _, threshold := range t {
			thresholds[name] = append(thresholds[name], threshold.Source)
		}
	}
	maxVUs := lib.GetMaxPossibleVUs(out.executionPlan)

	testRun := &cloudapi.TestRun{
		Name:       out.config.Name.String,
		ProjectID:  out.config.ProjectID.Int64,
		VUsMax:     int64(maxVUs),
		Thresholds: thresholds,
		Duration:   out.duration,
	}

	response, err := out.client.CreateTestRun(testRun)
	if err != nil {
		return err
	}
	out.referenceID = response.ReferenceID

	if response.ConfigOverride != nil {
		out.logger.WithFields(logrus.Fields{
			"override": response.ConfigOverride,
		}).Debug("overriding config options")
		out.config = out.config.Apply(*response.ConfigOverride)
	}

	out.startBackgroundProcesses()

	out.logger.WithFields(logrus.Fields{
		"name":        out.config.Name,
		"projectId":   out.config.ProjectID,
		"duration":    out.duration,
		"referenceId": out.referenceID,
	}).Debug("Started!")
	return nil
}

// Description returns the URL with the test run results.
func (out *Output) Description() string {
	return fmt.Sprintf("cloud (%s)", cloudapi.URLForResults(out.referenceID, out.config))
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
	out.logger.Debug("Stopping the cloud output...")
	close(out.stopAggregation)
	out.aggregationDone.Wait() // could be a no-op, if we have never started the aggregation
	out.logger.Debug("Aggregation stopped, stopping metric emission...")
	close(out.stopOutput)
	out.outputDone.Wait()
	out.logger.Debug("Metric emission stopped, calling cloud API...")
	err := out.testFinished(testErr)
	if err != nil {
		out.logger.WithFields(logrus.Fields{"error": err}).Warn("Failed to send test finished to the cloud")
	} else {
		out.logger.Debug("Cloud output successfully stopped!")
	}
	return err
}

func (out *Output) testFinished(testErr error) error {
	if out.referenceID == "" || out.config.PushRefID.Valid {
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
		"ref":        out.referenceID,
		"tainted":    testTainted,
		"run_status": runStatus,
	}).Debug("Sending test finished")

	return out.client.TestFinished(out.referenceID, thresholdResults, testTainted, runStatus)
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
