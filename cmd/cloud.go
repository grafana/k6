package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"go.k6.io/k6/cloudapi"
	"go.k6.io/k6/errext"
	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/consts"
	"go.k6.io/k6/ui/pb"
)

// cmdCloud handles the `k6 cloud` sub-command
type cmdCloud struct {
	gs *globalState

	showCloudLogs bool
	exitOnRunning bool
}

func (c *cmdCloud) preRun(cmd *cobra.Command, args []string) error {
	// TODO: refactor (https://github.com/loadimpact/k6/issues/883)
	//
	// We deliberately parse the env variables, to validate for wrong
	// values, even if we don't subsequently use them (if the respective
	// CLI flag was specified, since it has a higher priority).
	if showCloudLogsEnv, ok := c.gs.envVars["K6_SHOW_CLOUD_LOGS"]; ok {
		showCloudLogsValue, err := strconv.ParseBool(showCloudLogsEnv)
		if err != nil {
			return fmt.Errorf("parsing K6_SHOW_CLOUD_LOGS returned an error: %w", err)
		}
		if !cmd.Flags().Changed("show-logs") {
			c.showCloudLogs = showCloudLogsValue
		}
	}

	if exitOnRunningEnv, ok := c.gs.envVars["K6_EXIT_ON_RUNNING"]; ok {
		exitOnRunningValue, err := strconv.ParseBool(exitOnRunningEnv)
		if err != nil {
			return fmt.Errorf("parsing K6_EXIT_ON_RUNNING returned an error: %w", err)
		}
		if !cmd.Flags().Changed("exit-on-running") {
			c.exitOnRunning = exitOnRunningValue
		}
	}

	return nil
}

// TODO: split apart some more
//nolint:funlen,gocognit,cyclop
func (c *cmdCloud) run(cmd *cobra.Command, args []string) error {
	printBanner(c.gs)

	progressBar := pb.New(
		pb.WithConstLeft("Init"),
		pb.WithConstProgress(0, "Loading test script..."),
	)
	printBar(c.gs, progressBar)

	test, err := loadAndConfigureTest(c.gs, cmd, args, getPartialConfig)
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

	// TODO: Fix this
	// We reuse cloud.Config for parsing options.ext.loadimpact, but this probably shouldn't be
	// done, as the idea of options.ext is that they are extensible without touching k6. But in
	// order for this to happen, we shouldn't actually marshall cloud.Config on top of it, because
	// it will be missing some fields that aren't actually mentioned in the struct.
	// So in order for use to copy the fields that we need for loadimpact's api we unmarshal in
	// map[string]interface{} and copy what we need if it isn't set already
	var tmpCloudConfig map[string]interface{}
	if val, ok := arc.Options.External["loadimpact"]; ok {
		dec := json.NewDecoder(bytes.NewReader(val))
		dec.UseNumber() // otherwise float64 are used
		if err = dec.Decode(&tmpCloudConfig); err != nil {
			return err
		}
	}

	// Cloud config
	cloudConfig, err := cloudapi.GetConsolidatedConfig(
		test.derivedConfig.Collectors["cloud"], c.gs.envVars, "", arc.Options.External)
	if err != nil {
		return err
	}
	if !cloudConfig.Token.Valid {
		return errors.New("Not logged in, please use `k6 login cloud`.") //nolint:golint,revive,stylecheck
	}
	if tmpCloudConfig == nil {
		tmpCloudConfig = make(map[string]interface{}, 3)
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
	arc.Options.External["loadimpact"], err = json.Marshal(tmpCloudConfig)
	if err != nil {
		return err
	}

	name := cloudConfig.Name.String
	if !cloudConfig.Name.Valid || cloudConfig.Name.String == "" {
		name = filepath.Base(test.sourceRootPath)
	}

	globalCtx, globalCancel := context.WithCancel(c.gs.ctx)
	defer globalCancel()

	logger := c.gs.logger

	// Start cloud test run
	modifyAndPrintBar(c.gs, progressBar, pb.WithConstProgress(0, "Validating script options"))
	client := cloudapi.NewClient(
		logger, cloudConfig.Token.String, cloudConfig.Host.String, consts.Version, cloudConfig.Timeout.TimeDuration())
	if err = client.ValidateOptions(arc.Options); err != nil {
		return err
	}

	modifyAndPrintBar(c.gs, progressBar, pb.WithConstProgress(0, "Uploading archive"))
	refID, err := client.StartCloudTestRun(name, cloudConfig.ProjectID.Int64, arc)
	if err != nil {
		return err
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

			if testProgress.RunStatus == lib.RunStatusFinished {
				testProgress.Progress = 1
			} else if testProgress.RunStatus == lib.RunStatusRunning {
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

		if (newTestProgress.RunStatus > lib.RunStatusRunning) ||
			(c.exitOnRunning && newTestProgress.RunStatus == lib.RunStatusRunning) {
			globalCancel()
			break
		}
	}

	if testProgress == nil {
		//nolint:stylecheck,golint
		return errext.WithExitCodeIfNone(errors.New("Test progress error"), exitcodes.CloudFailedToGetProgress)
	}

	if !c.gs.flags.quiet {
		valueColor := getColor(c.gs.flags.noColor || !c.gs.stdOut.isTTY, color.FgCyan)
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

func (c *cmdCloud) flagSet() *pflag.FlagSet {
	flags := pflag.NewFlagSet("", pflag.ContinueOnError)
	flags.SortFlags = false
	flags.AddFlagSet(optionFlagSet())
	flags.AddFlagSet(runtimeOptionFlagSet(false))

	// TODO: Figure out a better way to handle the CLI flags
	flags.BoolVar(&c.exitOnRunning, "exit-on-running", c.exitOnRunning,
		"exits when test reaches the running status")
	flags.BoolVar(&c.showCloudLogs, "show-logs", c.showCloudLogs,
		"enable showing of logs when a test is executed in the cloud")

	return flags
}

func getCmdCloud(gs *globalState) *cobra.Command {
	c := &cmdCloud{
		gs:            gs,
		showCloudLogs: true,
		exitOnRunning: false,
	}

	cloudCmd := &cobra.Command{
		Use:   "cloud",
		Short: "Run a test on the cloud",
		Long: `Run a test on the cloud.

This will execute the test on the k6 cloud service. Use "k6 login cloud" to authenticate.`,
		Example: `
        k6 cloud script.js`[1:],
		Args:    exactArgsWithMsg(1, "arg should either be \"-\", if reading script from stdin, or a path to a script file"),
		PreRunE: c.preRun,
		RunE:    c.run,
	}
	cloudCmd.Flags().SortFlags = false
	cloudCmd.Flags().AddFlagSet(c.flagSet())
	return cloudCmd
}
