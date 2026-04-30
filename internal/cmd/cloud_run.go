package cmd

import (
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

	"go.k6.io/k6/v2/cloudapi"
	"go.k6.io/k6/v2/cmd/state"
	"go.k6.io/k6/v2/errext"
	"go.k6.io/k6/v2/errext/exitcodes"
	"go.k6.io/k6/v2/internal/build"
	cloudapiv6 "go.k6.io/k6/v2/internal/cloudapi/v6"
	"go.k6.io/k6/v2/internal/execution"
	"go.k6.io/k6/v2/internal/execution/local"
	"go.k6.io/k6/v2/internal/ui/pb"
	"go.k6.io/k6/v2/lib"
)

const cloudRunCommandName string = "run"

type cmdCloudRun struct {
	gs *state.GlobalState

	// localExecution stores the state of the --local-execution flag.
	localExecution bool

	// linger stores the state of the --linger flag.
	linger bool

	// noUsageReport stores the state of the --no-usage-report flag.
	noUsageReport bool

	// noArchiveUpload stores the state of the --no-archive-upload flag.
	//
	// This flag indicates to the local execution mode to not send the test
	// archive to the cloud service.
	noArchiveUpload bool

	// noCloudSecrets stores the state of the --no-cloud-secrets flag.
	noCloudSecrets bool

	// showCloudLogs stores the state of the --show-logs flag (or the
	// K6_SHOW_CLOUD_LOGS env var).
	showCloudLogs bool

	// exitOnRunning stores the state of the --exit-on-running flag (or the
	// K6_EXIT_ON_RUNNING env var).
	exitOnRunning bool

	// runCmd holds an instance of the k6 run command that we store
	// in order to be able to call its run method to support
	// the --local-execution flag mode.
	runCmd *cmdRun
}

func getCmdCloudRun(gs *state.GlobalState) *cobra.Command {
	// We instantiate the run command here to be able to call its run method
	// when the --local-execution flag is set.
	runCmd := &cmdRun{
		gs: gs,

		// We override the loadConfiguredTest func to use the local execution
		// configuration which enforces the use of the cloud output among other
		// side effects.
		loadConfiguredTest: func(cmd *cobra.Command, args []string) (
			*loadedAndConfiguredTest,
			execution.Controller,
			error,
		) {
			test, err := loadAndConfigureLocalTest(gs, cmd, args, getCloudRunLocalExecutionConfig)
			return test, local.NewController(), err
		},
	}

	cloudRunCmd := &cmdCloudRun{
		gs:            gs,
		runCmd:        runCmd,
		showCloudLogs: true,
	}

	exampleText := getExampleText(gs, `
  # Run a test script in Grafana Cloud
  $ {{.}} cloud run script.js

  # Run a test archive in Grafana Cloud
  $ {{.}} cloud run archive.tar

  # Read a test script or archive from stdin
  $ {{.}} cloud run - < script.js`[1:])

	thisCmd := &cobra.Command{
		Use:     cloudRunCommandName,
		Short:   "Run a test in Grafana Cloud",
		Long:    "Run a test in Grafana Cloud. Requires authentication via \"k6 cloud login\".",
		Example: exampleText,
		Args: exactArgsWithMsg(1,
			"the k6 cloud run command expects a single argument consisting in either a path to a script or "+
				"archive file, or the \"-\" symbol indicating the script or archive should be read from stdin",
		),
		PreRunE: cloudRunCmd.preRun,
		RunE:    cloudRunCmd.run,
	}

	thisCmd.Flags().SortFlags = false
	thisCmd.Flags().AddFlagSet(cloudRunCmd.flagSet())

	return thisCmd
}

func (c *cmdCloudRun) preRun(cmd *cobra.Command, _ []string) error {
	if c.localExecution {
		if cmd.Flags().Changed("exit-on-running") {
			return errext.WithExitCodeIfNone(
				fmt.Errorf("the --local-execution flag is not compatible with the --exit-on-running flag"),
				exitcodes.InvalidConfig,
			)
		}

		if cmd.Flags().Changed("show-logs") {
			return errext.WithExitCodeIfNone(
				fmt.Errorf("the --local-execution flag is not compatible with the --show-logs flag"),
				exitcodes.InvalidConfig,
			)
		}

		return nil
	}

	if c.linger {
		return errext.WithExitCodeIfNone(
			fmt.Errorf("the --linger flag can only be used in conjunction with the --local-execution flag"),
			exitcodes.InvalidConfig,
		)
	}

	if c.noCloudSecrets {
		return errext.WithExitCodeIfNone(
			fmt.Errorf("the --no-cloud-secrets flag can only be used in conjunction with the --local-execution flag"),
			exitcodes.InvalidConfig,
		)
	}

	// TODO: refactor (https://github.com/grafana/k6/issues/883)
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

	return nil
}

// run executes a test on Grafana Cloud (or locally with cloud output when
// `--local-execution` is set).
//
//nolint:funlen,gocognit,cyclop
func (c *cmdCloudRun) run(cmd *cobra.Command, args []string) error {
	if c.localExecution {
		c.runCmd.loadConfiguredTest = func(*cobra.Command, []string) (*loadedAndConfiguredTest, execution.Controller, error) {
			test, err := loadAndConfigureLocalTest(c.runCmd.gs, cmd, args, getCloudRunLocalExecutionConfig)
			if err != nil {
				return nil, nil, fmt.Errorf("could not load and configure the test: %w", err)
			}

			if err := createCloudTest(c.runCmd.gs, test); err != nil {
				if errors.Is(err, errCloudAuth) {
					return nil, nil, err
				}
				return nil, nil, fmt.Errorf("could not create the cloud test run: %w", err)
			}

			return test, local.NewController(), nil
		}
		return c.runCmd.run(cmd, args)
	}

	// When running the `k6 cloud run` command explicitly disable the usage report.
	c.noUsageReport = true

	test, err := loadAndConfigureLocalTest(c.gs, cmd, args, getPartialConfig)
	if err != nil {
		return err
	}

	// It's important to NOT set the derived options back to the runner
	// here, only the consolidated ones. Otherwise, if the script used
	// an execution shortcut option (e.g. `iterations` or `duration`),
	// we will have multiple conflicting execution options since the
	// derivation will set `scenarios` as well.
	if err := test.initRunner.SetOptions(test.consolidatedConfig.Options); err != nil {
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
	arc := test.makeArchive()

	tmpCloudConfig, err := cloudapi.GetTemporaryCloudConfig(arc.Options.Cloud)
	if err != nil {
		return err
	}

	// Cloud config
	cloudConfig, warn, err := cloudapi.GetConsolidatedConfig(
		test.derivedConfig.Collectors["cloud"], c.gs.Env, "", arc.Options.Cloud)
	if err != nil {
		return err
	}
	if err := checkCloudLogin(cloudConfig); err != nil {
		return err
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

	name := cloudConfig.Name.String
	if !cloudConfig.Name.Valid || cloudConfig.Name.String == "" {
		name = filepath.Base(test.sourceRootPath)
	}

	globalCtx, globalCancel := context.WithCancel(c.gs.Ctx)
	defer globalCancel()

	logger := c.gs.Logger

	// Start cloud test run
	modifyAndPrintBar(c.gs, progressBar, pb.WithConstProgress(0, "Validating script options"))
	client, err := cloudapiv6.NewClient(
		logger, cloudConfig.Token.String, cloudConfig.Hostv6.String, build.Version, cloudConfig.Timeout.TimeDuration())
	if err != nil {
		return err
	}
	projectID, err := prepCloudTestRun(globalCtx, c.gs, client, &cloudConfig, tmpCloudConfig, arc)
	if err != nil {
		return err
	}

	modifyAndPrintBar(c.gs, progressBar, pb.WithConstProgress(0, "Uploading archive"))

	loadTest, err := client.UploadTest(globalCtx, name, projectID, arc)
	if err != nil {
		return fmt.Errorf("uploading test: %w", err)
	}

	run, err := client.StartTest(globalCtx, loadTest.GetId())
	if err != nil {
		return fmt.Errorf("starting test: %w", err)
	}
	testRunID := run.GetId()

	// Trap Interrupts, SIGINTs and SIGTERMs.
	gracefulStop := func(sig os.Signal) {
		logger.WithField("sig", sig).Print("Stopping cloud test run in response to signal...")
		// Do this in a separate goroutine so that if it blocks, the
		// second signal can still abort the process execution.
		go func() {
			stopErr := client.StopTest(context.WithoutCancel(globalCtx), testRunID)
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
	testURL := run.GetTestRunDetailsPageUrl()
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

	testProgressLock := &sync.Mutex{}
	var testProgress *cloudapiv6.TestProgress
	progressBar.Modify(
		pb.WithProgress(func() (float64, []string) {
			testProgressLock.Lock()
			defer testProgressLock.Unlock()

			if testProgress == nil {
				return 0, []string{"Waiting..."}
			}
			if testProgress.IsRunning() {
				est := testProgress.Estimated()
				return testProgress.Progress(), []string{
					fmt.Sprintf("%s/%s", pb.GetFixedLengthDuration(testProgress.Elapsed(), est), est),
				}
			}
			return testProgress.Progress(), []string{testProgress.FormatStatus()}
		}),
	)

	ticker := time.NewTicker(time.Millisecond * 2000)
	if c.showCloudLogs {
		refID := strconv.FormatInt(int64(testRunID), 10)
		go func() {
			logger.Debug("Connecting to cloud logs server...")
			if err := cloudConfig.StreamLogsToLogger(globalCtx, logger, refID, 0); err != nil {
				logger.WithError(err).Error("error while tailing cloud logs")
			}
		}()
	}

	for range ticker.C {
		newTestProgress, progressErr := client.FetchTest(context.WithoutCancel(globalCtx), testRunID)
		if progressErr != nil {
			logger.WithError(progressErr).Error("Test progress error")
			continue
		}

		testProgressLock.Lock()
		testProgress = newTestProgress
		testProgressLock.Unlock()

		if newTestProgress.IsFinished() ||
			(c.exitOnRunning && newTestProgress.IsRunning()) {
			globalCancel()
			break
		}
	}

	// Stop progress rendering before printing final status to avoid ghost bars.
	progressCancel()
	progressBarWG.Wait()

	if testProgress == nil {
		//nolint:staticcheck
		return errext.WithExitCodeIfNone(errors.New("Test progress error"), exitcodes.CloudFailedToGetProgress)
	}

	c.printTestStatus(testProgress.FormatStatus())

	if testProgress.ThresholdsFailed() {
		//nolint:staticcheck
		return errext.WithExitCodeIfNone(errors.New("Thresholds have been crossed"), exitcodes.ThresholdsHaveFailed)
	}
	if testProgress.TestFailed() {
		//nolint:staticcheck
		return errext.WithExitCodeIfNone(errors.New("The test has failed"), exitcodes.CloudTestRunFailed)
	}

	return nil
}

func (c *cmdCloudRun) printTestStatus(status string) {
	if !c.gs.Flags.Quiet {
		valueColor := getColor(c.gs.Flags.NoColor || !c.gs.Stdout.IsTTY, color.FgCyan)
		printToStdout(c.gs, fmt.Sprintf(
			"     test status: %s\n", valueColor.Sprint(status),
		))
	} else {
		c.gs.Logger.WithField("run_status", status).Debug("Test finished")
	}
}

func (c *cmdCloudRun) flagSet() *pflag.FlagSet {
	flags := pflag.NewFlagSet("", pflag.ContinueOnError)
	flags.SortFlags = false
	flags.AddFlagSet(optionFlagSet())
	flags.AddFlagSet(runtimeOptionFlagSet(false))

	flags.BoolVar(&c.localExecution, "local-execution", c.localExecution,
		"executes the test locally instead of in the cloud")
	flags.BoolVar(
		&c.linger,
		"linger",
		c.linger,
		"only when using the local-execution mode, keeps the API server alive past the test end",
	)
	flags.BoolVar(
		&c.noUsageReport,
		"no-usage-report",
		c.noUsageReport,
		"only when using the local-execution mode, don't send anonymous usage "+
			"stats (https://grafana.com/docs/k6/latest/set-up/usage-collection/)",
	)
	flags.BoolVar(
		&c.noArchiveUpload,
		"no-archive-upload",
		c.noArchiveUpload,
		"only when using the local-execution mode, don't upload the test archive to the cloud service",
	)
	flags.BoolVar(
		&c.noCloudSecrets,
		"no-cloud-secrets",
		c.noCloudSecrets,
		"only when using the local-execution mode, don't automatically configure the cloud secret source",
	)

	// TODO: Figure out a better way to handle the CLI flags
	flags.BoolVar(&c.exitOnRunning, "exit-on-running", c.exitOnRunning,
		"exits when test reaches the running status")
	flags.BoolVar(&c.showCloudLogs, "show-logs", c.showCloudLogs,
		"enable showing of logs when a test is executed in the cloud")

	return flags
}

func getCloudRunLocalExecutionConfig(flags *pflag.FlagSet) (Config, error) {
	opts, err := getOptions(flags)
	if err != nil {
		return Config{}, err
	}

	// When running locally, we force the output to be cloud.
	out := []string{"cloud"}

	return Config{
		Options:         opts,
		Out:             out,
		Linger:          getNullBool(flags, "linger"),
		NoUsageReport:   getNullBool(flags, "no-usage-report"),
		NoArchiveUpload: getNullBool(flags, "no-archive-upload"),
	}, nil
}
