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

	"go.k6.io/k6/cloudapi"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/errext"
	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/internal/build"
	v6cloudapi "go.k6.io/k6/internal/cloudapi/v6"
	"go.k6.io/k6/internal/ui/pb"
	"go.k6.io/k6/lib"
	"gopkg.in/guregu/null.v3"

	"github.com/fatih/color"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// errUserUnauthenticated represents an authentication error when trying to use
// Grafana Cloud k6 without being logged in or having a valid token.
//
//nolint:staticcheck // the error is shown to the user so here punctuation and capital are required
var errUserUnauthenticated = errors.New("To run tests with Grafana Cloud k6, you must first authenticate." +
	" Run the `k6 cloud login` command, or check the docs" +
	" https://grafana.com/docs/grafana-cloud/testing/k6/author-run/tokens-and-cli-authentication" +
	" for additional authentication methods.")

// cmdCloud handles the `k6 cloud` sub-command
type cmdCloud struct {
	gs *state.GlobalState

	showCloudLogs bool
	exitOnRunning bool
	uploadOnly    bool
}

func (c *cmdCloud) preRun(cmd *cobra.Command, _ []string) error {
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
//
//nolint:funlen,gocognit,cyclop
func (c *cmdCloud) run(cmd *cobra.Command, args []string) error {
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
	printBanner(c.gs)

	progressBar := pb.New(
		pb.WithConstLeft("Init"),
		pb.WithConstProgress(0, "Loading test script..."),
	)
	printBar(c.gs, progressBar)

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
		return errUserUnauthenticated
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
		logger, cloudConfig.Token.String, cloudConfig.Host.String, build.Version, cloudConfig.Timeout.TimeDuration())

	var refID string
	// TODO: Should handle upload only with v6 API as well
	//nolint:nestif
	if cloudConfig.StackID.Valid && cloudConfig.StackID.Int64 != 0 && !c.uploadOnly {
		var stopSignalHandling func()
		refID, stopSignalHandling, err = startCloudTestRunV6(
			c.gs, logger, progressBar, &cloudConfig, tmpCloudConfig, name, arc, globalCancel)
		if err != nil {
			return err
		}

		defer stopSignalHandling()
	} else {
		if cloudConfig.StackID.Valid {
			client.SetStackID(cloudConfig.StackID.Int64)
		}
		if err = client.ValidateOptions(arc.Options); err != nil {
			return err
		}

		if cloudConfig.ProjectID.Int64 == 0 {
			if err := resolveAndSetProjectID(c.gs, &cloudConfig, tmpCloudConfig, arc); err != nil {
				return err
			}
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

		refID = cloudTestRun.ReferenceID
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
	}

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

			switch testProgress.RunStatus { //nolint:exhaustive
			case cloudapi.RunStatusFinished:
				testProgress.Progress = 1
			case cloudapi.RunStatusRunning:
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
		//nolint:staticcheck
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
		// Although by looking at [ResultStatus] and [RunStatus] isn't self-explanatory,
		// the scenario when the test run has finished, but it failed is an exceptional case for those situations
		// when thresholds have been crossed (failed). So, we report this situation as such.
		if testProgress.RunStatus == cloudapi.RunStatusFinished ||
			testProgress.RunStatus == cloudapi.RunStatusAbortedThreshold {
			//nolint:staticcheck
			return errext.WithExitCodeIfNone(errors.New("Thresholds have been crossed"), exitcodes.ThresholdsHaveFailed)
		}

		// TODO: use different exit codes for failed thresholds vs failed test (e.g. aborted by system/limit)
		return errext.WithExitCodeIfNone(errors.New("The test has failed"), exitcodes.CloudTestRunFailed) //nolint:staticcheck
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
	flags.BoolVar(&c.uploadOnly, "upload-only", c.uploadOnly,
		"only upload the test to the cloud without actually starting a test run")
	if err := flags.MarkDeprecated("upload-only", "use \"k6 cloud upload\" instead"); err != nil {
		panic(err) // Should never happen
	}

	return flags
}

func getCmdCloud(gs *state.GlobalState) *cobra.Command {
	c := &cmdCloud{
		gs:            gs,
		showCloudLogs: true,
		exitOnRunning: false,
		uploadOnly:    false,
	}

	exampleText := getExampleText(gs, `
  # [deprecated] Run a k6 script in the Grafana Cloud k6
  $ {{.}} cloud script.js

  # [deprecated] Run a k6 archive in the Grafana Cloud k6
  $ {{.}} cloud archive.tar

  # Authenticate with Grafana Cloud k6
  $ {{.}} cloud login

  # Run a k6 script in the Grafana Cloud k6
  $ {{.}} cloud run script.js

  # Run a k6 archive in the Grafana Cloud k6
  $ {{.}} cloud run archive.tar`[1:])

	cloudCmd := &cobra.Command{
		Use:   "cloud",
		Short: "Run a test on the cloud",
		Long: `The original behavior of the "k6 cloud" command described below is deprecated.
In future versions, the "cloud" command will only display a help text and will no longer run tests
in Grafana Cloud k6. To continue running tests in the cloud, please transition to using the "k6 cloud run" command.

Run a test in the Grafana Cloud k6.

This will archive test script(s), including all necessary resources, and execute the test in the Grafana Cloud k6
service. Be sure to run the "k6 cloud login" command prior to authenticate with Grafana Cloud k6.`,
		Args:    exactCloudArgs(),
		PreRunE: c.preRun,
		RunE:    c.run,
		Example: exampleText,
	}

	// Register `k6 cloud` subcommands
	cloudCmd.AddCommand(getCmdCloudRun(c))
	cloudCmd.AddCommand(getCmdCloudLogin(gs))
	cloudCmd.AddCommand(getCmdCloudUpload(c))

	cloudCmd.Flags().SortFlags = false
	cloudCmd.Flags().AddFlagSet(c.flagSet())

	return cloudCmd
}

func resolveDefaultProjectID(
	gs *state.GlobalState,
	cloudConfig *cloudapi.Config,
) (int64, error) {
	// Priority: projectID -> default stack from config
	if cloudConfig.ProjectID.Valid && cloudConfig.ProjectID.Int64 > 0 {
		return cloudConfig.ProjectID.Int64, nil
	}
	if cloudConfig.StackID.Valid && cloudConfig.StackID.Int64 != 0 {
		if cloudConfig.DefaultProjectID.Valid && cloudConfig.DefaultProjectID.Int64 > 0 {
			stackName := cloudConfig.StackURL.String
			if !cloudConfig.StackURL.Valid {
				stackName = fmt.Sprintf("stack-%d", cloudConfig.StackID.Int64)
			}
			gs.Logger.Warnf("No projectID specified, using default project of the %s stack\n", stackName)
			return cloudConfig.DefaultProjectID.Int64, nil
		}
		return 0, fmt.Errorf(
			"default stack configured but the default project ID is not available - " +
				"please run `k6 cloud login` to refresh your configuration")
	}

	// Return 0 to let the backend pick the project (old behavior)
	return 0, nil
}

func resolveAndSetProjectID(
	gs *state.GlobalState,
	cloudConfig *cloudapi.Config,
	tmpCloudConfig map[string]interface{},
	arc *lib.Archive,
) error {
	projectID, err := resolveDefaultProjectID(gs, cloudConfig)
	if err != nil {
		return err
	}
	if projectID > 0 {
		tmpCloudConfig["projectID"] = projectID

		b, err := json.Marshal(tmpCloudConfig)
		if err != nil {
			return err
		}

		arc.Options.Cloud = b
		arc.Options.External[cloudapi.LegacyCloudConfigKey] = b

		cloudConfig.ProjectID = null.IntFrom(projectID)
	}
	if !cloudConfig.StackID.Valid || cloudConfig.StackID.Int64 == 0 {
		fallBackMsg := ""
		if !cloudConfig.ProjectID.Valid || cloudConfig.ProjectID.Int64 == 0 {
			fallBackMsg = "Falling back to the first available stack. "
		}
		gs.Logger.Warn("DEPRECATED: No stack specified. " + fallBackMsg +
			"Consider setting a default stack via the `k6 cloud login` command or the `K6_CLOUD_STACK_ID` " +
			"environment variable as this will become mandatory in the next major release.")
	}
	return nil
}

func exactCloudArgs() cobra.PositionalArgs {
	return func(_ *cobra.Command, args []string) error {
		const baseErrMsg = `the "k6 cloud" command expects either a subcommand such as "run" or "login", or ` +
			"a single argument consisting in a path to a script/archive, or the `-` symbol instructing " +
			"the command to read the test content from stdin"

		if len(args) == 0 {
			return fmt.Errorf(baseErrMsg + "; " + "received no arguments")
		}

		hasSubcommand := args[0] == "run" || args[0] == "login"
		if len(args) > 1 && !hasSubcommand {
			return fmt.Errorf(
				baseErrMsg+"; "+"received %d arguments %q, and %s is not a valid subcommand",
				len(args), strings.Join(args, " "), args[0],
			)
		}

		return nil
	}
}

// startCloudTestRunV6 starts a cloud test run using the v6 API client and returns a reference ID
// (either of the test if uploadOnly is true or of the test run) and a function to stop signal handling.
func startCloudTestRunV6(
	gs *state.GlobalState,
	logger logrus.FieldLogger,
	progressBar *pb.ProgressBar,
	cloudConfig *cloudapi.Config,
	tmpCloudConfig map[string]interface{},
	testName string,
	arc *lib.Archive,
	globalCancel context.CancelFunc,
) (string, func(), error) {
	client, err := v6cloudapi.NewClient(
		logger, cloudConfig.Token.String, cloudConfig.Hostv6.String, build.Version, cloudConfig.Timeout.TimeDuration())
	if err != nil {
		return "", nil, err
	}
	if err := client.SetStackID(cloudConfig.StackID.Int64); err != nil {
		return "", nil, err
	}

	cloudConfig.WebAppURL = null.StringFrom(cloudConfig.StackURL.String + "/a/k6-app")

	if cloudConfig.ProjectID.Int64 == 0 {
		if err := resolveAndSetProjectID(gs, cloudConfig, tmpCloudConfig, arc); err != nil {
			return "", nil, err
		}
	}

	if err = client.ValidateOptions(cloudConfig.ProjectID.Int64, arc.Options); err != nil {
		return "", nil, err
	}

	modifyAndPrintBar(gs, progressBar, pb.WithConstProgress(0, "Uploading archive"))

	cloudTestRun, err := client.CreateAndStartCloudTestRun(testName, cloudConfig.ProjectID.Int64, arc)
	if err != nil {
		return "", nil, err
	}

	refID := cloudTestRun.Id

	// Trap Interrupts, SIGINTs and SIGTERMs.
	gracefulStop := func(sig os.Signal) {
		logger.WithField("sig", sig).Print("Stopping cloud test run in response to signal...")
		// Do this in a separate goroutine so that if it blocks, the
		// second signal can still abort the process execution.
		go func() {
			stopErr := client.StopCloudTestRun(int64(refID))
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
	stopSignalHandling := handleTestAbortSignals(gs, gracefulStop, onHardStop)

	return strconv.FormatInt(int64(refID), 10), stopSignalHandling, nil
}
