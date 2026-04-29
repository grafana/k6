package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/v2/cloudapi"
	"go.k6.io/k6/v2/cmd/state"
	"go.k6.io/k6/v2/errext"
	"go.k6.io/k6/v2/errext/exitcodes"
	"go.k6.io/k6/v2/internal/build"
	cloudapiv6 "go.k6.io/k6/v2/internal/cloudapi/v6"
	"go.k6.io/k6/v2/internal/ui/pb"
	"go.k6.io/k6/v2/lib"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	errCloudAuth = errors.New( //nolint:staticcheck // user-facing error message, capitalization is intentional
		"Run `k6 cloud login` to authenticate, or check the docs for other options at" +
			" https://grafana.com/docs/grafana-cloud/testing/k6/author-run/tokens-and-cli-authentication",
	)
	errMissingToken   = errors.New("access token not configured")
	errMissingStackID = errors.New("stack ID not configured")
)

// checkCloudLogin verifies that both a token and a stack are configured.
// Together they represent a complete Grafana Cloud login.
func checkCloudLogin(conf cloudapi.Config) error {
	const prefix = "Running cloud tests requires auth settings"
	if !conf.Token.Valid || conf.Token.String == "" {
		return fmt.Errorf("%s: %w.\n%w", prefix, errMissingToken, errCloudAuth)
	}
	if !conf.StackID.Valid || conf.StackID.Int64 == 0 {
		return fmt.Errorf("%s: %w.\n%w", prefix, errMissingStackID, errCloudAuth)
	}
	return nil
}

// errNoStackConfigured indicates that no Grafana Cloud stack has been set up,
// which is required for stack-scoped operations like listing projects.
var errNoStackConfigured = errors.New(
	"no stack configured. Please run `k6 cloud login` to set a default stack",
)

// cmdCloud handles the `k6 cloud` sub-command
type cmdCloud struct {
	gs *state.GlobalState

	showCloudLogs bool
	exitOnRunning bool
	uploadOnly    bool
}

func (c *cmdCloud) preRun(cmd *cobra.Command, _ []string) error {
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

// TODO: split apart some more
//
//nolint:funlen,gocognit,cyclop
func (c *cmdCloud) run(cmd *cobra.Command, args []string) error {
	// If no args provided and called from main cloud command, show helpful error
	if cmd.Name() == "cloud" && len(args) == 0 {
		return errors.New("the \"k6 cloud\" command expects either a subcommand such as \"run\" or \"login\", " +
			"or a single argument consisting in a path to a script/archive, or the `-` symbol instructing " +
			"the command to read the test content from stdin; received no arguments")
	}

	// Show deprecation warning only when running tests directly via "k6 cloud <file>"
	// (not when using subcommands like "k6 cloud run")
	if cmd.Name() == "cloud" && len(args) > 0 {
		c.gs.Logger.Warn("Running tests directly with \"k6 cloud <file>\" is deprecated. " +
			"Use \"k6 cloud run <file>\" instead. This behavior will be removed in a future release.")
	}

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

	if c.uploadOnly {
		et, err := lib.NewExecutionTuple(test.derivedConfig.ExecutionSegment, test.derivedConfig.ExecutionSegmentSequence)
		if err != nil {
			return err
		}
		executionPlan := test.derivedConfig.Scenarios.GetFullExecutionRequirements(et)
		testURL := fmt.Sprintf("%s/a/k6-app/tests/%d", strings.TrimSuffix(cloudConfig.StackURL.String, "/"), loadTest.GetId())
		printExecutionDescription(
			c.gs, "cloud", test.sourceRootPath, testURL, test.derivedConfig, et, executionPlan, nil,
		)
		modifyAndPrintBar(c.gs, progressBar, pb.WithConstLeft("Run "), pb.WithConstProgress(1.0, "Archived"))
		c.printTestStatus("Archived")
		return nil
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

func (c *cmdCloud) printTestStatus(status string) {
	if !c.gs.Flags.Quiet {
		valueColor := getColor(c.gs.Flags.NoColor || !c.gs.Stdout.IsTTY, color.FgCyan)
		printToStdout(c.gs, fmt.Sprintf(
			"     test status: %s\n", valueColor.Sprint(status),
		))
	} else {
		c.gs.Logger.WithField("run_status", status).Debug("Test finished")
	}
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

func getCloudUsageTemplate() string {
	return `{{.Short}}

Usage:{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{else if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}

Available Commands:{{range .Commands}}{{if .IsAvailableCommand}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}

Flags:
{{.LocalFlags.FlagUsagesWrapped 120 | trimTrailingWhitespaces}}
{{if .HasExample}}
Examples:
{{.Example}}
{{end}}{{if .HasAvailableSubCommands}}
Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`
}

func getCmdCloud(gs *state.GlobalState) *cobra.Command {
	c := &cmdCloud{
		gs:            gs,
		showCloudLogs: true,
		exitOnRunning: false,
	}

	exampleText := getExampleText(gs, `
  # Authenticate with Grafana Cloud
  $ {{.}} cloud login

  # Run a test script in Grafana Cloud
  $ {{.}} cloud run script.js

  # Run a test script locally and stream results to Grafana Cloud
  $ {{.}} cloud run --local-execution script.js

  # Run a test archive in Grafana Cloud
  $ {{.}} cloud run archive.tar

  # [deprecated] Run a test script in Grafana Cloud
  $ {{.}} cloud script.js

  # [deprecated] Run a test archive in Grafana Cloud
  $ {{.}} cloud archive.tar`[1:])

	cloudCmd := &cobra.Command{
		Use:     "cloud",
		Short:   "Run and manage Grafana Cloud tests",
		Long:    "Run and manage tests in Grafana Cloud.",
		Example: exampleText,
		PreRunE: c.preRun,
		RunE: func(cmd *cobra.Command, args []string) error {
			// If no args provided, show help
			if len(args) == 0 {
				return cmd.Help()
			}
			return c.run(cmd, args)
		},
	}

	// Register `k6 cloud` subcommands with default usage template
	defaultUsageTemplate := (&cobra.Command{}).UsageTemplate()
	defaultUsageTemplate = strings.ReplaceAll(defaultUsageTemplate, "FlagUsages", "FlagUsagesWrapped 120")

	runCmd := getCmdCloudRun(c)
	runCmd.SetUsageTemplate(defaultUsageTemplate)
	runCmd.SetHelpTemplate((&cobra.Command{}).HelpTemplate())
	cloudCmd.AddCommand(runCmd)

	loginCmd := getCmdCloudLogin(gs)
	loginCmd.SetUsageTemplate(defaultUsageTemplate)
	loginCmd.SetHelpTemplate((&cobra.Command{}).HelpTemplate())
	cloudCmd.AddCommand(loginCmd)

	uploadCmd := getCmdCloudUpload(c)
	uploadCmd.SetUsageTemplate(defaultUsageTemplate)
	uploadCmd.SetHelpTemplate((&cobra.Command{}).HelpTemplate())
	cloudCmd.AddCommand(uploadCmd)

	projectCmd := getCmdCloudProject(c)
	cloudCmd.AddCommand(projectCmd)

	testRunCmd := getCmdCloudTestRun(c)
	cloudCmd.AddCommand(testRunCmd)

	cloudCmd.Flags().SortFlags = false
	cloudCmd.Flags().AddFlagSet(c.flagSet())

	// Use custom template similar to root - hardcode flags to avoid showing global flags
	cloudTemplate := getCloudUsageTemplate()
	cloudCmd.SetUsageTemplate(cloudTemplate)
	cloudCmd.SetHelpTemplate(cloudTemplate)

	return cloudCmd
}

// prepCloudTestRun wires stack and project IDs into the client, validates
// script options, and resolves a default project when none was specified.
// Returns the project ID as used by subsequent v6 calls.
func prepCloudTestRun(
	ctx context.Context, gs *state.GlobalState,
	client *cloudapiv6.Client,
	cloudConfig *cloudapi.Config, tmpCloudConfig map[string]any, arc *lib.Archive,
) (int32, error) {
	toInt32 := func(v int64) (int32, error) {
		if v < math.MinInt32 || v > math.MaxInt32 {
			return 0, fmt.Errorf("value %d overflows int32", v)
		}
		return int32(v), nil
	}

	stackID, err := toInt32(cloudConfig.StackID.Int64)
	if err != nil {
		return 0, err
	}
	client.SetStackID(stackID)

	projectID, err := toInt32(cloudConfig.ProjectID.Int64)
	if err != nil {
		return 0, err
	}

	if projectID == 0 {
		if err := resolveAndSetProjectID(gs, cloudConfig, tmpCloudConfig, arc); err != nil {
			return 0, err
		}
		projectID, err = toInt32(cloudConfig.ProjectID.Int64)
		if err != nil {
			return 0, err
		}
	}

	if err := client.ValidateOptions(ctx, projectID, arc.Options); err != nil {
		return 0, err
	}

	return projectID, nil
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
	tmpCloudConfig map[string]any,
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

		cloudConfig.ProjectID = null.IntFrom(projectID)
	}
	return nil
}
