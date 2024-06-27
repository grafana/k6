package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"go.k6.io/k6/event"
	"go.k6.io/k6/execution"
	"go.k6.io/k6/execution/local"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/lib/trace"
	"go.k6.io/k6/metrics"
	"go.k6.io/k6/metrics/engine"
	"go.k6.io/k6/output"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"go.k6.io/k6/cloudapi"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/errext"
	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/consts"
	"go.k6.io/k6/ui/pb"
)

// cmdCloudRun handles the `k6 cloud` sub-command
type cmdCloudRun struct {
	gs *state.GlobalState

	// localExecution is a flag to enable local execution of the test
	localExecution bool
	showCloudLogs  bool
	exitOnRunning  bool
	uploadOnly     bool

	// FIXME: this is brought directly from the `k6 run` command, and might be irrelevant here.
	loadLocallyConfiguredTestFn func(*cobra.Command, []string) (*loadedAndConfiguredTest, execution.Controller, error)
}

func getCmdCloudRun(gs *state.GlobalState) *cobra.Command {
	loadLocallyConfiguredTestFn := func(
		cmd *cobra.Command, args []string,
	) (*loadedAndConfiguredTest, execution.Controller, error) {
		test, err := loadAndConfigureLocalTest(gs, cmd, args, getCloudRunLocalConfig)
		return test, local.NewController(), err
	}

	c := &cmdCloudRun{
		gs:                          gs,
		localExecution:              false,
		showCloudLogs:               true,
		exitOnRunning:               false,
		uploadOnly:                  false,
		loadLocallyConfiguredTestFn: loadLocallyConfiguredTestFn,
	}

	exampleText := getExampleText(gs, `
  # Run a test in the Grafana k6 cloud
  $ {{.}} cloud run script.js

  # Run a test in the Grafana k6 cloud with a specific token
  $ {{.}} cloud run  --token <YOUR_API_TOKEN> script.js`[1:])

	// FIXME: when the command is "k6 cloud run" without an script/archive, we should display an error and the help
	cloudRunCmd := &cobra.Command{
		Use:   cloudRunCommandName,
		Short: "Run a test in the Grafana k6 cloud",
		Long: `Run a test in the Grafana k6 cloud.

This will execute the test in the Grafana k6 cloud service. Using this command requires to be authenticated
against the Grafana k6 cloud. Use the "k6 cloud login" command to authenticate.`,
		Example: exampleText,
		Args:    exactArgsWithMsg(1, "arg should either be \"-\", if reading script from stdin, or a path to a script file"),
		PreRunE: c.preRun,
		RunE:    c.run,
	}

	cloudRunCmd.Flags().SortFlags = false
	cloudRunCmd.Flags().AddFlagSet(c.flagSet())

	return cloudRunCmd
}

func (c *cmdCloudRun) flagSet() *pflag.FlagSet {
	flags := pflag.NewFlagSet("", pflag.ContinueOnError)
	flags.SortFlags = false

	// FIXME @oleiade: we should make a passe over the flags defined there, and define our own set with the ones needed
	// for local-execution.
	flags.AddFlagSet(optionFlagSet())

	// FIXME: this is brought directly from the `k6 run` command, and we might want to use a limited set of
	// those flags here?
	flags.AddFlagSet(runtimeOptionFlagSet(false))

	// TODO: Figure out a better way to handle the CLI flags
	flags.BoolVar(&c.exitOnRunning, "exit-on-running", c.exitOnRunning,
		"exits when test reaches the running status")
	flags.BoolVar(&c.showCloudLogs, "show-logs", c.showCloudLogs,
		"enable showing of logs when a test is executed in the cloud")
	flags.BoolVar(&c.uploadOnly, "upload-only", c.uploadOnly,
		"only upload the test to the cloud without actually starting a test run")
	flags.BoolVar(&c.localExecution, "local-execution", c.localExecution,
		"run the test locally and stream results to the Grafana Cloud k6")

	return flags
}

const cloudRunCommandName string = "run"

//nolint:dupl // remove this statement once the migration from the `k6 cloud` to the `k6 cloud run` is complete.
func (c *cmdCloudRun) preRun(cmd *cobra.Command, _ []string) error {
	// TODO: refactor (https://github.com/loadimpact/k6/issues/883)
	//
	// We deliberately parse the env variables, to validate for wrong
	// values, even if we don't subsequently use them (if the respective
	// CLI flag was specified, since it has a higher priority).
	if showCloudLogsEnv, ok := c.gs.Env["K6_SHOW_CLOUD_LOGS"]; ok {
		showCloudLogsValue, err := strconv.ParseBool(showCloudLogsEnv)
		if err != nil {
			return fmt.Errorf("parsing K6_SHOW_CLOUD_LOGS returned an error: %w", err)
		}
		if !cmd.Flags().Changed("show-logs") {
			c.showCloudLogs = showCloudLogsValue
		}
	}

	if exitOnRunningEnv, ok := c.gs.Env["K6_EXIT_ON_RUNNING"]; ok {
		exitOnRunningValue, err := strconv.ParseBool(exitOnRunningEnv)
		if err != nil {
			return fmt.Errorf("parsing K6_EXIT_ON_RUNNING returned an error: %w", err)
		}
		if !cmd.Flags().Changed("exit-on-running") {
			c.exitOnRunning = exitOnRunningValue
		}
	}
	if uploadOnlyEnv, ok := c.gs.Env["K6_CLOUD_UPLOAD_ONLY"]; ok {
		uploadOnlyValue, err := strconv.ParseBool(uploadOnlyEnv)
		if err != nil {
			return fmt.Errorf("parsing K6_CLOUD_UPLOAD_ONLY returned an error: %w", err)
		}
		if !cmd.Flags().Changed("upload-only") {
			c.uploadOnly = uploadOnlyValue
		}
	}

	return nil
}

// TODO: split apart some more
func (c *cmdCloudRun) run(cmd *cobra.Command, args []string) error {
	if c.localExecution {
		return c.executeLocally(cmd, args)
	}

	return c.executeInTheCloud(cmd, args)
}

func (c *cmdCloudRun) executeInTheCloud(cmd *cobra.Command, args []string) error {
	printBanner(c.gs)

	progressBar := pb.New(
		pb.WithConstLeft("Init"),
		pb.WithConstProgress(0, "Loading test script..."),
	)
	printBar(c.gs, progressBar)

	test, err := loadAndConfigureLocalTest(c.gs, cmd, args, getPartialConfig)
	if err != nil {
		return err
	}

	// It's important to NOT set the derived options back to the runner
	// here, only the consolidated ones. Otherwise, if the script used
	// an execution shortcut option (e.g. `iterations` or `duration`),
	// we will have multiple conflicting execution options since the
	// derivation will set `scenarios` as well.
	testRunState, err := test.buildTestRunState(test.consolidatedConfig.Options)
	if err != nil {
		return err
	}

	// TODO: validate for usage of execution segment
	// TODO: validate for externally controlled executor (i.e. executors that aren't distributable)
	// TODO: move those validations to a separate function and reuse validateConfig()?

	modifyAndPrintBar(c.gs, progressBar, pb.WithConstProgress(0, "Building the archive..."))
	arc := testRunState.Runner.MakeArchive()

	tmpCloudConfig, err := cloudapi.GetTemporaryCloudConfig(arc.Options.Cloud, arc.Options.External)
	if err != nil {
		return err
	}

	// Cloud config
	cloudConfig, warn, err := cloudapi.GetConsolidatedConfig(
		test.derivedConfig.Collectors["cloud"], c.gs.Env, "", arc.Options.Cloud, arc.Options.External)
	if err != nil {
		return err
	}
	if !cloudConfig.Token.Valid {
		return errors.New( //nolint:golint
			"not logged in, please login to the Grafana k6 Cloud " +
				"using the `k6 cloud login` command",
		)
	}

	// Display config warning if needed
	if warn != "" {
		modifyAndPrintBar(c.gs, progressBar, pb.WithConstProgress(0, "Warning: "+warn))
	}

	if cloudConfig.Token.Valid {
		tmpCloudConfig["token"] = cloudConfig.Token
	}
	if cloudConfig.Name.Valid {
		tmpCloudConfig["name"] = cloudConfig.Name
	}
	if cloudConfig.ProjectID.Valid {
		tmpCloudConfig["projectID"] = cloudConfig.ProjectID
	}

	if arc.Options.External == nil {
		arc.Options.External = make(map[string]json.RawMessage)
	}

	b, err := json.Marshal(tmpCloudConfig)
	if err != nil {
		return err
	}

	arc.Options.Cloud = b
	arc.Options.External[cloudapi.LegacyCloudConfigKey] = b

	name := cloudConfig.Name.String
	if !cloudConfig.Name.Valid || cloudConfig.Name.String == "" {
		name = filepath.Base(test.sourceRootPath)
	}

	globalCtx, globalCancel := context.WithCancel(c.gs.Ctx)
	defer globalCancel()

	logger := c.gs.Logger

	// Start cloud test run
	modifyAndPrintBar(c.gs, progressBar, pb.WithConstProgress(0, "Validating script options"))
	client := cloudapi.NewClient(
		logger, cloudConfig.Token.String, cloudConfig.Host.String, consts.Version, cloudConfig.Timeout.TimeDuration())
	if err = client.ValidateOptions(arc.Options); err != nil {
		return err
	}

	modifyAndPrintBar(c.gs, progressBar, pb.WithConstProgress(0, "Uploading archive"))

	var cloudTestRun *cloudapi.CreateTestRunResponse
	if c.uploadOnly {
		cloudTestRun, err = client.UploadTestOnly(name, cloudConfig.ProjectID.Int64, arc)
	} else {
		cloudTestRun, err = client.StartCloudTestRun(name, cloudConfig.ProjectID.Int64, arc)
	}

	if err != nil {
		return err
	}

	refID := cloudTestRun.ReferenceID
	if cloudTestRun.ConfigOverride != nil {
		cloudConfig = cloudConfig.Apply(*cloudTestRun.ConfigOverride)
	}

	// Trap Interrupts, SIGINTs and SIGTERMs.
	gracefulStop := func(sig os.Signal) {
		logger.WithField("sig", sig).Print("Stopping cloud test run in response to signal...")
		// Do this in a separate goroutine so that if it blocks, the
		// second signal can still abort the process execution.
		go func() {
			stopErr := client.StopCloudTestRun(refID)
			if stopErr != nil {
				logger.WithError(stopErr).Error("Stop cloud test error")
			} else {
				logger.Info("Successfully sent signal to stop the cloud test, now waiting for it to actually stop...")
			}
			globalCancel()
		}()
	}
	onHardStop := func(sig os.Signal) {
		logger.WithField("sig", sig).Error("Aborting k6 in response to signal, we won't wait for the test to end.")
	}
	stopSignalHandling := handleTestAbortSignals(c.gs, gracefulStop, onHardStop)
	defer stopSignalHandling()

	et, err := lib.NewExecutionTuple(test.derivedConfig.ExecutionSegment, test.derivedConfig.ExecutionSegmentSequence)
	if err != nil {
		return err
	}
	testURL := cloudapi.URLForResults(refID, cloudConfig)
	executionPlan := test.derivedConfig.Scenarios.GetFullExecutionRequirements(et)
	printExecutionDescription(
		c.gs, "cloud", test.sourceRootPath, testURL, test.derivedConfig, et, executionPlan, nil,
	)

	modifyAndPrintBar(
		c.gs, progressBar,
		pb.WithConstLeft("Run "), pb.WithConstProgress(0, "Initializing the cloud test"),
	)

	progressCtx, progressCancel := context.WithCancel(globalCtx)
	progressBarWG := &sync.WaitGroup{}
	progressBarWG.Add(1)
	defer progressBarWG.Wait()
	defer progressCancel()
	go func() {
		showProgress(progressCtx, c.gs, []*pb.ProgressBar{progressBar}, logger)
		progressBarWG.Done()
	}()

	var (
		startTime   time.Time
		maxDuration time.Duration
	)
	maxDuration, _ = lib.GetEndOffset(executionPlan)

	testProgressLock := &sync.Mutex{}
	var testProgress *cloudapi.TestProgressResponse
	progressBar.Modify(
		pb.WithProgress(func() (float64, []string) {
			testProgressLock.Lock()
			defer testProgressLock.Unlock()

			if testProgress == nil {
				return 0, []string{"Waiting..."}
			}

			statusText := testProgress.RunStatusText

			if testProgress.RunStatus == cloudapi.RunStatusFinished {
				testProgress.Progress = 1
			} else if testProgress.RunStatus == cloudapi.RunStatusRunning {
				if startTime.IsZero() {
					startTime = time.Now()
				}
				spent := time.Since(startTime)
				if spent > maxDuration {
					statusText = maxDuration.String()
				} else {
					statusText = fmt.Sprintf("%s/%s", pb.GetFixedLengthDuration(spent, maxDuration), maxDuration)
				}
			}

			return testProgress.Progress, []string{statusText}
		}),
	)

	ticker := time.NewTicker(time.Millisecond * 2000)
	if c.showCloudLogs {
		go func() {
			logger.Debug("Connecting to cloud logs server...")
			if err := cloudConfig.StreamLogsToLogger(globalCtx, logger, refID, 0); err != nil {
				logger.WithError(err).Error("error while tailing cloud logs")
			}
		}()
	}

	for range ticker.C {
		newTestProgress, progressErr := client.GetTestProgress(refID)
		if progressErr != nil {
			logger.WithError(progressErr).Error("Test progress error")
			continue
		}

		testProgressLock.Lock()
		testProgress = newTestProgress
		testProgressLock.Unlock()

		if (newTestProgress.RunStatus > cloudapi.RunStatusRunning) ||
			(c.exitOnRunning && newTestProgress.RunStatus == cloudapi.RunStatusRunning) {
			globalCancel()
			break
		}
	}

	if testProgress == nil {
		//nolint:stylecheck,golint
		return errext.WithExitCodeIfNone(errors.New("Test progress error"), exitcodes.CloudFailedToGetProgress)
	}

	if !c.gs.Flags.Quiet {
		valueColor := getColor(c.gs.Flags.NoColor || !c.gs.Stdout.IsTTY, color.FgCyan)
		printToStdout(c.gs, fmt.Sprintf(
			"     test status: %s\n", valueColor.Sprint(testProgress.RunStatusText),
		))
	} else {
		logger.WithField("run_status", testProgress.RunStatusText).Debug("Test finished")
	}

	if testProgress.ResultStatus == cloudapi.ResultStatusFailed {
		// TODO: use different exit codes for failed thresholds vs failed test (e.g. aborted by system/limit)
		//nolint:stylecheck,golint
		return errext.WithExitCodeIfNone(errors.New("The test has failed"), exitcodes.CloudTestRunFailed)
	}

	return nil
}

// executeLocally is the function handling the local execution of the cloud tests.
//
// It is adapted from the `k6 run` command implementation, with tweaks to make it specific to the
// `k6 cloud run` command.
//
// TODO: We should align the features we activate here with what's needed in a cloud/local-execution context.
// for instance, is linger needed? is REST API needed?
func (c *cmdCloudRun) executeLocally(cmd *cobra.Command, args []string) error {
	var err error

	var logger logrus.FieldLogger = c.gs.Logger
	defer func() {
		if err == nil {
			logger.Debug("Everything has finished, exiting k6 normally!")
		} else {
			logger.WithError(err).Debug("Everything has finished, exiting k6 with an error!")
		}
	}()
	printBanner(c.gs)

	globalCtx, globalCancel := context.WithCancel(c.gs.Ctx)
	defer globalCancel()

	// lingerCtx is cancelled by Ctrl+C, and is used to wait for that event when
	// k6 was started with the --linger option.
	lingerCtx, lingerCancel := context.WithCancel(globalCtx)
	defer lingerCancel()

	// runCtx is used for the test run execution and is created with the special
	// execution.NewTestRunContext() function so that it can be aborted even
	// from sub-contexts while also attaching a reason for the abort.
	runCtx, runAbort := execution.NewTestRunContext(lingerCtx, logger)

	emitEvent := func(evt *event.Event) func() {
		waitDone := c.gs.Events.Emit(evt)
		return func() {
			waitCtx, waitCancel := context.WithTimeout(globalCtx, waitEventDoneTimeout)
			defer waitCancel()
			if werr := waitDone(waitCtx); werr != nil {
				logger.WithError(werr).Warn()
			}
		}
	}

	defer func() {
		waitExitDone := emitEvent(&event.Event{
			Type: event.Exit,
			Data: &event.ExitData{Error: err},
		})
		waitExitDone()
		c.gs.Events.UnsubscribeAll()
	}()

	test, controller, err := c.loadLocallyConfiguredTestFn(cmd, args)
	if err != nil {
		return err
	}
	if test.keyLogger != nil {
		defer func() {
			if klErr := test.keyLogger.Close(); klErr != nil {
				logger.WithError(klErr).Warn("Error while closing the SSLKEYLOGFILE")
			}
		}()
	}

	if err = c.setupTracerProvider(globalCtx, test); err != nil {
		return err
	}
	waitTracesFlushed := func() {
		ctx, cancel := context.WithTimeout(globalCtx, waitForTracerProviderStopTimeout)
		defer cancel()
		if tpErr := test.preInitState.TracerProvider.Shutdown(ctx); tpErr != nil {
			logger.Errorf("The tracer provider didn't stop gracefully: %v", tpErr)
		}
	}

	// Write the full consolidated *and derived* options back to the Runner.
	conf := test.derivedConfig
	testRunState, err := test.buildTestRunState(conf.Options)
	if err != nil {
		return err
	}

	// Create a local execution scheduler wrapping the runner.
	logger.Debug("Initializing the execution scheduler...")
	execScheduler, err := execution.NewScheduler(testRunState, controller)
	if err != nil {
		return err
	}

	backgroundProcesses := &sync.WaitGroup{}
	defer backgroundProcesses.Wait()

	// This is manually triggered after the Engine's Run() has completed,
	// and things like a single Ctrl+C don't affect it. We use it to make
	// sure that the progressbars finish updating with the latest execution
	// state one last time, after the test run has finished.
	progressCtx, progressCancel := context.WithCancel(globalCtx)
	defer progressCancel()

	initBar := execScheduler.GetInitProgressBar()
	backgroundProcesses.Add(1)
	go func() {
		defer backgroundProcesses.Done()
		pbs := []*pb.ProgressBar{initBar}
		for _, s := range execScheduler.GetExecutors() {
			pbs = append(pbs, s.GetProgress())
		}
		showProgress(progressCtx, c.gs, pbs, logger)
	}()

	// Create all outputs.
	executionPlan := execScheduler.GetExecutionPlan()
	outputs, err := createOutputs(c.gs, test, executionPlan)
	if err != nil {
		return err
	}

	outputs = append(outputs, testRunState.GroupSummary)

	metricsEngine, err := engine.NewMetricsEngine(testRunState.Registry, logger)
	if err != nil {
		return err
	}

	// We'll need to pipe metrics to the MetricsEngine and process them if any
	// of these are enabled: thresholds, end-of-test summary
	shouldProcessMetrics := (!testRunState.RuntimeOptions.NoSummary.Bool ||
		!testRunState.RuntimeOptions.NoThresholds.Bool)
	var metricsIngester *engine.OutputIngester
	if shouldProcessMetrics {
		err = metricsEngine.InitSubMetricsAndThresholds(conf.Options, testRunState.RuntimeOptions.NoThresholds.Bool)
		if err != nil {
			return err
		}
		// We'll need to pipe metrics to the MetricsEngine if either the
		// thresholds or the end-of-test summary are enabled.
		metricsIngester = metricsEngine.CreateIngester()
		outputs = append(outputs, metricsIngester)
	}

	executionState := execScheduler.GetState()
	if !testRunState.RuntimeOptions.NoSummary.Bool {
		defer func() {
			logger.Debug("Generating the end-of-test summary...")
			summaryResult, hsErr := test.initRunner.HandleSummary(globalCtx, &lib.Summary{
				Metrics:         metricsEngine.ObservedMetrics,
				RootGroup:       testRunState.GroupSummary.Group(),
				TestRunDuration: executionState.GetCurrentTestRunDuration(),
				NoColor:         c.gs.Flags.NoColor,
				UIState: lib.UIState{
					IsStdOutTTY: c.gs.Stdout.IsTTY,
					IsStdErrTTY: c.gs.Stderr.IsTTY,
				},
			})
			if hsErr == nil {
				hsErr = handleSummaryResult(c.gs.FS, c.gs.Stdout, c.gs.Stderr, summaryResult)
			}
			if hsErr != nil {
				logger.WithError(hsErr).Error("failed to handle the end-of-test summary")
			}
		}()
	}

	waitInitDone := emitEvent(&event.Event{Type: event.Init})

	// Create and start the outputs. We do it quite early to get any output URLs
	// or other details below. It also allows us to ensure when they have
	// flushed their samples and when they have stopped in the defer statements.
	initBar.Modify(pb.WithConstProgress(0, "Starting outputs"))
	outputManager := output.NewManager(outputs, logger, func(err error) {
		if err != nil {
			logger.WithError(err).Error("Received error to stop from output")
		}
		// TODO: attach run status and exit code?
		runAbort(err)
	})
	samples := make(chan metrics.SampleContainer, test.derivedConfig.MetricSamplesBufferSize.Int64)
	waitOutputsFlushed, stopOutputs, err := outputManager.Start(samples)
	if err != nil {
		return err
	}
	defer func() {
		logger.Debug("Stopping outputs...")
		// We call waitOutputsFlushed() below because the threshold calculations
		// need all of the metrics to be sent to the MetricsEngine before we can
		// calculate them one last time. We need the threshold calculated here,
		// since they may change the run status for the outputs.
		stopOutputs(err)
	}()

	if !testRunState.RuntimeOptions.NoThresholds.Bool {
		finalizeThresholds := metricsEngine.StartThresholdCalculations(
			metricsIngester, runAbort, executionState.GetCurrentTestRunDuration,
		)
		handleFinalThresholdCalculation := func() {
			// This gets called after the Samples channel has been closed and
			// the OutputManager has flushed all of the cached samples to
			// outputs (including MetricsEngine's ingester). So we are sure
			// there won't be any more metrics being sent.
			logger.Debug("Finalizing thresholds...")
			breachedThresholds := finalizeThresholds()
			if len(breachedThresholds) == 0 {
				return
			}
			tErr := errext.WithAbortReasonIfNone(
				errext.WithExitCodeIfNone(
					fmt.Errorf("thresholds on metrics '%s' have been crossed", strings.Join(breachedThresholds, ", ")),
					exitcodes.ThresholdsHaveFailed,
				), errext.AbortedByThresholdsAfterTestEnd)

			if err == nil {
				err = tErr
			} else {
				logger.WithError(tErr).Debug("Crossed thresholds, but test already exited with another error")
			}
		}
		if finalizeThresholds != nil {
			defer handleFinalThresholdCalculation()
		}
	}

	defer func() {
		logger.Debug("Waiting for metrics and traces processing to finish...")
		close(samples)

		ww := [...]func(){
			waitOutputsFlushed,
			waitTracesFlushed,
		}
		var wg sync.WaitGroup
		wg.Add(len(ww))
		for _, w := range ww {
			w := w
			go func() {
				w()
				wg.Done()
			}()
		}
		wg.Wait()

		logger.Debug("Metrics and traces processing finished!")
	}()

	// NOTE (@oleiade): k6 cloud run --local-execution does not need to spin up the REST API server
	// Spin up the REST API server, if not disabled.
	//if c.gs.Flags.Address != "" { //nolint:nestif
	//	initBar.Modify(pb.WithConstProgress(0, "Init API server"))
	//
	//	// We cannot use backgroundProcesses here, since we need the REST API to
	//	// be down before we can close the samples channel above and finish the
	//	// processing the metrics pipeline.
	//	apiWG := &sync.WaitGroup{}
	//	apiWG.Add(2)
	//	defer apiWG.Wait()
	//
	//	srvCtx, srvCancel := context.WithCancel(globalCtx)
	//	defer srvCancel()
	//
	//	srv := api.GetServer(
	//		runCtx,
	//		c.gs.Flags.Address, c.gs.Flags.ProfilingEnabled,
	//		testRunState,
	//		samples,
	//		metricsEngine,
	//		execScheduler,
	//	)
	//	go func() {
	//		defer apiWG.Done()
	//		logger.Debugf("Starting the REST API server on %s", c.gs.Flags.Address)
	//		if c.gs.Flags.ProfilingEnabled {
	//			logger.Debugf("Profiling exposed on http://%s/debug/pprof/", c.gs.Flags.Address)
	//		}
	//		if aerr := srv.ListenAndServe(); aerr != nil && !errors.Is(aerr, http.ErrServerClosed) {
	//			// Only exit k6 if the user has explicitly set the REST API address
	//			if cmd.Flags().Lookup("address").Changed {
	//				logger.WithError(aerr).Error("Error from API server")
	//				c.gs.OSExit(int(exitcodes.CannotStartRESTAPI))
	//			} else {
	//				logger.WithError(aerr).Warn("Error from API server")
	//			}
	//		}
	//	}()
	//	go func() {
	//		defer apiWG.Done()
	//		<-srvCtx.Done()
	//		shutdCtx, shutdCancel := context.WithTimeout(globalCtx, 1*time.Second)
	//		defer shutdCancel()
	//		if aerr := srv.Shutdown(shutdCtx); aerr != nil {
	//			logger.WithError(aerr).Debug("REST API server did not shut down correctly")
	//		}
	//	}()
	//}

	printExecutionDescription(
		c.gs, "local", args[0], "", conf, executionState.ExecutionTuple, executionPlan, outputs,
	)

	// Trap Interrupts, SIGINTs and SIGTERMs.
	// TODO: move upwards, right after runCtx is created
	gracefulStop := func(sig os.Signal) {
		logger.WithField("sig", sig).Debug("Stopping k6 in response to signal...")
		// first abort the test run this way, to propagate the error
		runAbort(errext.WithAbortReasonIfNone(
			errext.WithExitCodeIfNone(
				fmt.Errorf("test run was aborted because k6 received a '%s' signal", sig), exitcodes.ExternalAbort,
			), errext.AbortedByUser,
		))
		lingerCancel() // cancel this context as well, since the user did Ctrl+C
	}
	onHardStop := func(sig os.Signal) {
		logger.WithField("sig", sig).Error("Aborting k6 in response to signal")
		globalCancel() // not that it matters, given that os.Exit() will be called right after
	}
	stopSignalHandling := handleTestAbortSignals(c.gs, gracefulStop, onHardStop)
	defer stopSignalHandling()

	// Initialize the VUs and executors
	stopVUEmission, err := execScheduler.Init(runCtx, samples)
	if err != nil {
		return err
	}
	defer stopVUEmission()

	// NOTE (@oleiade): k6 cloud run --local-execution does not need to wait for the test to start?
	//if conf.Linger.Bool {
	//	defer func() {
	//		msg := "The test is done, but --linger was enabled, so k6 is waiting for Ctrl+C to continue..."
	//		select {
	//		case <-lingerCtx.Done():
	//			// do nothing, we were interrupted by Ctrl+C already
	//		default:
	//			logger.Debug(msg)
	//			if !c.gs.Flags.Quiet {
	//				printToStdout(c.gs, msg)
	//			}
	//			<-lingerCtx.Done()
	//			logger.Debug("Ctrl+C received, exiting...")
	//		}
	//	}()
	//}

	waitInitDone()

	waitTestStartDone := emitEvent(&event.Event{Type: event.TestStart})
	waitTestStartDone()

	// Start the test! However, we won't immediately return if there was an
	// error, we still have things to do.
	err = execScheduler.Run(globalCtx, runCtx, samples)

	waitTestEndDone := emitEvent(&event.Event{Type: event.TestEnd})
	defer waitTestEndDone()

	// Init has passed successfully, so unless disabled, make sure we send a
	// usage report after the context is done.
	if !conf.NoUsageReport.Bool {
		backgroundProcesses.Add(1)
		go func() {
			defer backgroundProcesses.Done()
			reportCtx, reportCancel := context.WithTimeout(globalCtx, 3*time.Second)
			defer reportCancel()
			logger.Debug("Sending usage report...")

			if rerr := reportUsage(reportCtx, execScheduler, test); rerr != nil {
				logger.WithError(rerr).Debug("Error sending usage report")
			} else {
				logger.Debug("Usage report sent successfully")
			}
		}()
	}

	// Check what the execScheduler.Run() error is.
	if err != nil {
		err = common.UnwrapGojaInterruptedError(err)
		logger.WithError(err).Debug("Test finished with an error")
		return err
	}

	// Warn if no iterations could be completed.
	if executionState.GetFullIterationCount() == 0 {
		logger.Warn("No script iterations fully finished, consider making the test duration longer")
	}

	logger.Debug("Test finished cleanly")

	return nil
}

func (c *cmdCloudRun) setupTracerProvider(ctx context.Context, test *loadedAndConfiguredTest) error {
	ro := test.preInitState.RuntimeOptions
	if ro.TracesOutput.String == "none" {
		test.preInitState.TracerProvider = trace.NewNoopTracerProvider()
		return nil
	}

	tp, err := trace.TracerProviderFromConfigLine(ctx, ro.TracesOutput.String)
	if err != nil {
		return err
	}
	test.preInitState.TracerProvider = tp

	return nil
}

// getCloudRunLocalConfig is an adaptation of the [getConfig] helper function which
// has been tailored to the needs of the local execution of cloud tests, and predefine the
// set of outputs used by k6 to be solely the cloud output.
func getCloudRunLocalConfig(flags *pflag.FlagSet) (Config, error) {
	opts, err := getOptions(flags)
	if err != nil {
		return Config{}, err
	}

	// When performing a local execution, we predefine the set of outputs to be
	// solely the cloud output.
	outputs := []string{"cloud"}

	return Config{
		Options: opts,
		Out:     outputs,

		// FIXME: should we keep this option for cloud local execution?
		Linger: getNullBool(flags, "linger"),

		// FIXME: should we keep this option for local execution?
		NoUsageReport: getNullBool(flags, "no-usage-report"),
	}, nil
}
