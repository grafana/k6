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

	"gopkg.in/guregu/null.v3"

	v1api "go.k6.io/k6/cloudapi"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/errext"
	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/internal/build"
	v6cloudapi "go.k6.io/k6/internal/cloudapi/v6"
	"go.k6.io/k6/internal/ui/pb"
	"go.k6.io/k6/lib"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// errUserUnauthenticated represents an authentication error when trying to use
// Grafana Cloud without being logged in or having a valid token.
//
//nolint:staticcheck // the error is shown to the user so here punctuation and capital are required
var errUserUnauthenticated = errors.New("To run tests in Grafana Cloud, you must first authenticate." +
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
func (c *cmdCloud) run(cmd *cobra.Command, args []string) error {
	if err := c.validateCloudRunArgs(cmd, args); err != nil {
		return err
	}

	test, err := loadAndConfigureLocalTest(c.gs, cmd, args, getPartialConfig)
	if err != nil {
		return fmt.Errorf("loading local test: %w", err)
	}

	// It's important to NOT set the derived options back to the runner
	// here, only the consolidated ones. Otherwise, if the script used
	// an execution shortcut option (e.g. `iterations` or `duration`),
	// we will have multiple conflicting execution options since the
	// derivation will set `scenarios` as well.
	testRunState, err := test.buildTestRunState(test.consolidatedConfig.Options)
	if err != nil {
		return fmt.Errorf("building test run state: %w", err)
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

	tmpCloudConfig, err := v1api.GetTemporaryCloudConfig(arc.Options.Cloud)
	if err != nil {
		return fmt.Errorf("reading temporary cloud config: %w", err)
	}

	// Cloud config
	cfg, warn, err := v1api.GetConsolidatedConfig(
		test.derivedConfig.Collectors["cloud"], c.gs.Env, "", arc.Options.Cloud)
	if err != nil {
		return fmt.Errorf("building cloud config: %w", err)
	}
	if !cfg.Token.Valid {
		return errUserUnauthenticated
	}

	// Display config warning if needed
	if warn != "" {
		modifyAndPrintBar(c.gs, progressBar, pb.WithConstProgress(0, "Warning: "+warn))
	}

	if cfg.Token.Valid {
		tmpCloudConfig["token"] = cfg.Token
	}
	if cfg.Name.Valid {
		tmpCloudConfig["name"] = cfg.Name
	}
	if cfg.ProjectID.Valid {
		tmpCloudConfig["projectID"] = cfg.ProjectID
	}

	if arc.Options.External == nil {
		arc.Options.External = make(map[string]json.RawMessage)
	}

	b, err := json.Marshal(tmpCloudConfig)
	if err != nil {
		return fmt.Errorf("marshaling cloud config: %w", err)
	}

	arc.Options.Cloud = b

	name := cfg.Name.String
	if !cfg.Name.Valid || cfg.Name.String == "" {
		name = filepath.Base(test.sourceRootPath)
	}

	if c.uploadOnly {
		return c.runUploadOnly(test, cfg, tmpCloudConfig, arc, name, progressBar)
	}

	return c.runRemote(test, cfg, tmpCloudConfig, arc, name, progressBar)
}

func (c *cmdCloud) validateCloudRunArgs(cmd *cobra.Command, args []string) error {
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

	return nil
}

func (c *cmdCloud) runRemote(test *loadedAndConfiguredTest, cfg v1api.Config, tmpCloudConfig map[string]any,
	arc *lib.Archive, name string, progressBar *pb.ProgressBar,
) error {
	client, err := c.newRemoteClient(test, cfg, arc)
	if err != nil {
		return fmt.Errorf("creating remote client: %w", err)
	}
	err = c.validateCloudOptions(&cfg, tmpCloudConfig, arc, progressBar, func(cfg v1api.Config) error {
		return client.ValidateOptions(c.gs.Ctx, cfg.ProjectID.Int64, arc.Options)
	})
	if err != nil {
		return fmt.Errorf("validating remote options: %w", err)
	}

	globalCtx, globalCancel := context.WithCancel(c.gs.Ctx)
	defer globalCancel()

	modifyAndPrintBar(c.gs, progressBar, pb.WithConstProgress(0, "Uploading archive"))

	testRun, err := client.StartTest(globalCtx, name, cfg.ProjectID.Int64, arc)
	if err != nil {
		return fmt.Errorf("starting cloud test run: %w", err)
	}

	logger := c.gs.Logger

	gracefulStop := func(sig os.Signal) {
		logger.WithField("sig", sig).Print("Stopping cloud test run in response to signal...")
		go func() {
			err := client.StopTest(context.WithoutCancel(globalCtx), testRun.ID)
			if err != nil {
				logger.WithError(err).Error("stopping cloud test run")
				globalCancel()
				return
			}

			logger.Info("Successfully sent signal to stop the cloud test, now waiting for it to actually stop...")
			globalCancel()
		}()
	}
	onHardStop := func(sig os.Signal) {
		logger.WithField("sig", sig).Error("Aborting k6 in response to signal, we won't wait for the test to end.")
	}
	stopSignalHandling := handleTestAbortSignals(c.gs, gracefulStop, onHardStop)
	defer stopSignalHandling()

	if _, err = c.printCloudExecution(test, progressBar, testRun.WebAppURL); err != nil {
		return fmt.Errorf("printing cloud execution: %w", err)
	}

	testProgress, err := c.pollRemoteCloudTestProgress(globalCtx, globalCancel, progressBar, client, testRun.ID)
	if err != nil {
		return fmt.Errorf("polling remote test progress: %w", err)
	}

	c.reportCloudTestStatus(v6cloudapi.FormatStatus(testProgress.Status))
	return mapRemoteCloudTestResult(testProgress)
}

func (c *cmdCloud) runUploadOnly(test *loadedAndConfiguredTest, cfg v1api.Config,
	tmpCloudConfig map[string]any, arc *lib.Archive, name string, progressBar *pb.ProgressBar,
) error {
	client, err := c.newRemoteClient(test, cfg, arc)
	if err != nil {
		return fmt.Errorf("creating remote client: %w", err)
	}
	err = c.validateCloudOptions(&cfg, tmpCloudConfig, arc, progressBar, func(cfg v1api.Config) error {
		return client.ValidateOptions(c.gs.Ctx, cfg.ProjectID.Int64, arc.Options)
	})
	if err != nil {
		return fmt.Errorf("validating remote options: %w", err)
	}

	modifyAndPrintBar(c.gs, progressBar, pb.WithConstProgress(0, "Uploading archive"))

	testID, err := client.UploadTest(c.gs.Ctx, name, cfg.ProjectID.Int64, arc)
	if err != nil {
		return fmt.Errorf("uploading cloud test: %w", err)
	}

	testURL, err := v1api.URLForTest(testID, cfg)
	if err != nil {
		c.gs.Logger.WithError(err).Warn("Could not build test page URL")
	}

	et, err := lib.NewExecutionTuple(test.derivedConfig.ExecutionSegment, test.derivedConfig.ExecutionSegmentSequence)
	if err != nil {
		return fmt.Errorf("building execution tuple: %w", err)
	}
	executionPlan := test.derivedConfig.Scenarios.GetFullExecutionRequirements(et)
	printExecutionDescription(c.gs, "cloud", test.sourceRootPath, testURL, test.derivedConfig, et, executionPlan, nil)

	c.reportCloudTestStatus("Uploaded")
	return nil
}

func (c *cmdCloud) newRemoteClient(test *loadedAndConfiguredTest, cfg v1api.Config,
	arc *lib.Archive,
) (*v6cloudapi.Client, error) {
	remoteCfg, err := v6cloudapi.GetConsolidatedConfig(
		test.derivedConfig.Collectors["cloud"], c.gs.Env, "", arc.Options.Cloud,
	)
	if err != nil {
		return nil, fmt.Errorf("building cloud config: %w", err)
	}

	client, err := v6cloudapi.NewClient(
		c.gs.Logger,
		cfg.Token.String,
		remoteCfg.Host.String,
		build.Version,
		cfg.Timeout.TimeDuration(),
	)
	if err != nil {
		return nil, fmt.Errorf("creating client: %w", err)
	}
	if cfg.StackID.Valid {
		client.SetStackID(cfg.StackID.Int64)
	}

	return client, nil
}

func (c *cmdCloud) validateCloudOptions(cfg *v1api.Config, tmpCloudConfig map[string]any,
	arc *lib.Archive, progressBar *pb.ProgressBar, validate func(v1api.Config) error,
) error {
	modifyAndPrintBar(c.gs, progressBar, pb.WithConstProgress(0, "Validating script options"))

	if cfg.ProjectID.Int64 == 0 {
		err := resolveAndSetProjectID(c.gs, cfg, tmpCloudConfig, arc)
		if err != nil {
			return fmt.Errorf("resolving project ID: %w", err)
		}
	}

	if err := validate(*cfg); err != nil {
		return fmt.Errorf("validating options: %w", err)
	}

	return nil
}

func (c *cmdCloud) pollCloudProgress(globalCtx context.Context, globalCancel context.CancelFunc,
	progressBar *pb.ProgressBar, progress func() (float64, string),
	fetch func(context.Context) (bool, error), streamLogs func(context.Context),
) error {
	progressBar.Modify(
		pb.WithProgress(func() (float64, []string) {
			value, status := progress()
			return value, []string{status}
		}),
	)

	progressCtx, progressCancel := context.WithCancel(globalCtx)
	progressBarWG := &sync.WaitGroup{}
	progressBarWG.Add(1)
	defer progressBarWG.Wait()
	defer progressCancel()
	go func() {
		showProgress(progressCtx, c.gs, []*pb.ProgressBar{progressBar}, c.gs.Logger)
		progressBarWG.Done()
	}()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	if streamLogs != nil {
		go streamLogs(progressCtx)
	}

	for range ticker.C {
		done, err := fetch(context.WithoutCancel(progressCtx))
		if err != nil {
			return err
		}

		if done {
			globalCancel()
			break
		}
	}

	return nil
}

func (c *cmdCloud) printCloudExecution(test *loadedAndConfiguredTest, progressBar *pb.ProgressBar,
	testURL string,
) (time.Duration, error) {
	et, err := lib.NewExecutionTuple(test.derivedConfig.ExecutionSegment, test.derivedConfig.ExecutionSegmentSequence)
	if err != nil {
		return 0, fmt.Errorf("building execution tuple: %w", err)
	}
	executionPlan := test.derivedConfig.Scenarios.GetFullExecutionRequirements(et)
	printExecutionDescription(c.gs, "cloud", test.sourceRootPath, testURL, test.derivedConfig, et, executionPlan, nil)

	modifyAndPrintBar(
		c.gs, progressBar,
		pb.WithConstLeft("Run "), pb.WithConstProgress(0, "Initializing the cloud test"),
	)

	maxDuration, _ := lib.GetEndOffset(executionPlan)
	return maxDuration, nil
}

func (c *cmdCloud) pollRemoteCloudTestProgress(globalCtx context.Context, globalCancel context.CancelFunc,
	progressBar *pb.ProgressBar, client *v6cloudapi.Client, testRunID int64,
) (*v6cloudapi.TestRunProgress, error) {
	var maxProgress float64
	var mu sync.Mutex
	var progress *v6cloudapi.TestRunProgress

	if err := c.pollCloudProgress(
		globalCtx,
		globalCancel,
		progressBar,
		func() (float64, string) {
			mu.Lock()
			defer mu.Unlock()

			if progress == nil {
				return 0, "Waiting..."
			}

			value, status := cloudTestProgress(maxProgress, progress)
			maxProgress = value

			return value, status
		},
		func(ctx context.Context) (bool, error) {
			next, err := client.FetchTest(ctx, testRunID)
			if err != nil {
				return false, fmt.Errorf("fetching test run: %w", err)
			}

			mu.Lock()
			defer mu.Unlock()
			progress = next
			if next.IsFinished() {
				maxProgress = 1
			}

			return next.IsFinished() || c.exitOnRunning && next.IsRunning(), nil
		},
		nil,
	); err != nil {
		return nil, fmt.Errorf("polling test progress: %w", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if progress == nil {
		err := errors.New("test progress error")
		return nil, errext.WithExitCodeIfNone(err, exitcodes.CloudFailedToGetProgress)
	}

	return progress, nil
}

func cloudTestProgress(maxProgress float64, testProgress *v6cloudapi.TestRunProgress) (float64, string) {
	progress := max(maxProgress, testProgress.Progress())
	if testProgress.IsFinished() {
		progress = 1
	}

	if !testProgress.IsRunning() || testProgress.EstimatedDuration <= 0 {
		return progress, v6cloudapi.FormatStatus(testProgress.Status)
	}

	spent := time.Duration(testProgress.ExecutionDuration) * time.Second
	maxDuration := time.Duration(testProgress.EstimatedDuration) * time.Second
	status := fmt.Sprintf("%s/%s", pb.GetFixedLengthDuration(spent, maxDuration), maxDuration)

	return progress, status
}

func (c *cmdCloud) reportCloudTestStatus(statusText string) {
	if !c.gs.Flags.Quiet {
		valueColor := getColor(c.gs.Flags.NoColor || !c.gs.Stdout.IsTTY, color.FgCyan)
		printToStdout(c.gs, fmt.Sprintf(
			"     test status: %s\n", valueColor.Sprint(statusText),
		))
	} else {
		c.gs.Logger.WithField("run_status", statusText).Debug("Test finished")
	}
}

func mapRemoteCloudTestResult(testProgress *v6cloudapi.TestRunProgress) error {
	if testProgress.Result == v6cloudapi.ResultFailed &&
		(testProgress.IsFinished() || testProgress.Status == v6cloudapi.StatusAbortedThreshold) {
		return errext.WithExitCodeIfNone(errThresholdsCrossed, exitcodes.ThresholdsHaveFailed)
	}
	if testProgress.Result == v6cloudapi.ResultFailed || testProgress.Result == v6cloudapi.ResultError {
		return errext.WithExitCodeIfNone(errTestFailed, exitcodes.CloudTestRunFailed)
	}

	return nil
}

type cloudRunError string

func (e cloudRunError) Error() string {
	s := string(e)
	return strings.ToUpper(s[:1]) + s[1:]
}

const (
	errThresholdsCrossed cloudRunError = "thresholds have been crossed"
	errTestFailed        cloudRunError = "the test has failed"
)

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
  # [deprecated] Run a test script in Grafana Cloud
  $ {{.}} cloud script.js

  # [deprecated] Run a test archive in Grafana Cloud
  $ {{.}} cloud archive.tar

  # Authenticate with Grafana Cloud
  $ {{.}} cloud login

  # Run a test script in Grafana Cloud
  $ {{.}} cloud run script.js

  # Run a test archive in Grafana Cloud
  $ {{.}} cloud run archive.tar`[1:])

	cloudCmd := &cobra.Command{
		Use:     "cloud",
		Short:   "Run and manage Grafana Cloud tests",
		Long:    "Run and manage tests in Grafana Cloud.",
		Example: exampleText,
		PreRunE: c.preRun,
		RunE:    c.run,
	}

	// Register `k6 cloud` subcommands with default usage template
	defaultUsageTemplate := (&cobra.Command{}).UsageTemplate()
	defaultUsageTemplate = strings.ReplaceAll(defaultUsageTemplate, "FlagUsages", "FlagUsagesWrapped 120")

	runCmd := getCmdCloudRun(c)
	runCmd.SetUsageTemplate(defaultUsageTemplate)
	cloudCmd.AddCommand(runCmd)

	loginCmd := getCmdCloudLogin(gs)
	loginCmd.SetUsageTemplate(defaultUsageTemplate)
	cloudCmd.AddCommand(loginCmd)

	uploadCmd := getCmdCloudUpload(c)
	uploadCmd.SetUsageTemplate(defaultUsageTemplate)
	cloudCmd.AddCommand(uploadCmd)

	cloudCmd.Flags().SortFlags = false
	cloudCmd.Flags().AddFlagSet(c.flagSet())

	cloudCmd.SetUsageTemplate(`Usage:
  {{.CommandPath}} [command]

Commands:{{range .Commands}}{{if (or (eq .Name "login") (eq .Name "run"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{range .Commands}}` +
		`{{if and .IsAvailableCommand (ne .Name "login") (ne .Name "run")}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}

Flags:
  -h, --help   Show help
{{if .HasExample}}
Examples:
{{.Example}}
{{end}}
Use "{{.CommandPath}} [command] --help" for more information about a command.
`)

	return cloudCmd
}

func resolveDefaultProjectID(gs *state.GlobalState, cloudConfig *v1api.Config) (int64, error) {
	// Priority: projectID -> default stack from config
	if cloudConfig.ProjectID.Valid && cloudConfig.ProjectID.Int64 > 0 {
		return cloudConfig.ProjectID.Int64, nil
	}
	if !cloudConfig.StackID.Valid || cloudConfig.StackID.Int64 == 0 {
		return 0, nil
	}
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

func resolveAndSetProjectID(gs *state.GlobalState, cloudConfig *v1api.Config, tmpCloudConfig map[string]any,
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
