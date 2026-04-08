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

	v1Client := c.newV1CloudClient(cfg)
	globalCtx, globalCancel := context.WithCancel(c.gs.Ctx)
	defer globalCancel()

	modifyAndPrintBar(c.gs, progressBar, pb.WithConstProgress(0, "Uploading archive"))

	cloudTestRun, err := v1Client.StartCloudTestRun(name, cfg.ProjectID.Int64, arc)
	if err != nil {
		return fmt.Errorf("starting cloud test run: %w", err)
	}

	refID := cloudTestRun.ReferenceID
	if cloudTestRun.ConfigOverride != nil {
		cfg = cfg.Apply(*cloudTestRun.ConfigOverride)
	}

	testURL := v1api.URLForResults(refID, cfg)
	return c.waitV1CloudTest(globalCtx, globalCancel, progressBar, v1Client, cfg, refID, test, testURL)
}

func (c *cmdCloud) runUploadOnly(test *loadedAndConfiguredTest, cfg v1api.Config,
	tmpCloudConfig map[string]any, arc *lib.Archive, name string, progressBar *pb.ProgressBar,
) error {
	return c.runV1CloudTest(test, cfg, tmpCloudConfig, arc, progressBar,
		func(client *v1api.Client, cfg v1api.Config) (*v1api.CreateTestRunResponse, error) {
			cloudTestRun, err := client.UploadTestOnly(name, cfg.ProjectID.Int64, arc)
			if err != nil {
				return nil, fmt.Errorf("uploading cloud test: %w", err)
			}

			return cloudTestRun, nil
		},
	)
}

func (c *cmdCloud) runV1CloudTest(
	test *loadedAndConfiguredTest,
	cfg v1api.Config,
	tmpCloudConfig map[string]any,
	arc *lib.Archive,
	progressBar *pb.ProgressBar,
	start func(*v1api.Client, v1api.Config) (*v1api.CreateTestRunResponse, error),
) error {
	client, cfg, err := c.prepareV1CloudClient(cfg, tmpCloudConfig, arc, progressBar)
	if err != nil {
		return err
	}

	globalCtx, globalCancel := context.WithCancel(c.gs.Ctx)
	defer globalCancel()

	modifyAndPrintBar(c.gs, progressBar, pb.WithConstProgress(0, "Uploading archive"))

	cloudTestRun, err := start(client, cfg)
	if err != nil {
		return err
	}

	refID := cloudTestRun.ReferenceID
	if cloudTestRun.ConfigOverride != nil {
		cfg = cfg.Apply(*cloudTestRun.ConfigOverride)
	}

	testURL := v1api.URLForResults(refID, cfg)
	return c.waitV1CloudTest(globalCtx, globalCancel, progressBar, client, cfg, refID, test, testURL)
}

func (c *cmdCloud) prepareV1CloudClient(cfg v1api.Config, tmpCloudConfig map[string]any, arc *lib.Archive,
	progressBar *pb.ProgressBar,
) (*v1api.Client, v1api.Config, error) {
	client := c.newV1CloudClient(cfg)
	err := c.validateCloudOptions(&cfg, tmpCloudConfig, arc, progressBar, func(v1api.Config) error {
		return client.ValidateOptions(arc.Options)
	})
	if err != nil {
		return nil, cfg, err
	}

	return client, cfg, nil
}

func (c *cmdCloud) newV1CloudClient(cfg v1api.Config) *v1api.Client {
	client := v1api.NewClient(c.gs.Logger, cfg.Token.String, cfg.Host.String, build.Version, cfg.Timeout.TimeDuration())
	if cfg.StackID.Valid {
		client.SetStackID(cfg.StackID.Int64)
	}

	return client
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

func (c *cmdCloud) waitV1CloudTest(globalCtx context.Context, globalCancel context.CancelFunc,
	progressBar *pb.ProgressBar, client *v1api.Client, cfg v1api.Config, refID string,
	test *loadedAndConfiguredTest, testURL string,
) error {
	logger := c.gs.Logger

	gracefulStop := func(sig os.Signal) {
		logger.WithField("sig", sig).Print("Stopping cloud test run in response to signal...")
		go func() {
			err := client.StopCloudTestRun(refID)
			if err == nil {
				logger.Info("Successfully sent signal to stop the cloud test, now waiting for it to actually stop...")
				globalCancel()
				return
			}

			logger.WithError(err).Error("stopping cloud test run")
			globalCancel()
		}()
	}
	onHardStop := func(sig os.Signal) {
		logger.WithField("sig", sig).Error("Aborting k6 in response to signal, we won't wait for the test to end.")
	}
	stopSignalHandling := handleTestAbortSignals(c.gs, gracefulStop, onHardStop)
	defer stopSignalHandling()

	maxDuration, err := c.printCloudExecution(test, progressBar, testURL)
	if err != nil {
		return err
	}

	testProgress, err := c.pollCloudTestProgress(
		globalCtx, globalCancel, progressBar, client, cfg, refID, maxDuration,
	)
	if err != nil {
		return err
	}

	return c.reportCloudTestResult(testProgress)
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

func (c *cmdCloud) pollCloudTestProgress(globalCtx context.Context, globalCancel context.CancelFunc,
	progressBar *pb.ProgressBar, client *v1api.Client, cfg v1api.Config, refID string,
	maxDuration time.Duration,
) (*v1api.TestProgressResponse, error) {
	var startTime time.Time
	var mu sync.Mutex
	var progress *v1api.TestProgressResponse

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

			return v1CloudTestProgress(progress, &startTime, maxDuration)
		},
		func(context.Context) (bool, error) {
			next, err := client.GetTestProgress(refID)
			if err != nil {
				c.gs.Logger.WithError(err).Error("Test progress error")
				return false, nil
			}

			mu.Lock()
			defer mu.Unlock()
			progress = next

			return next.RunStatus > v1api.RunStatusRunning ||
				(c.exitOnRunning && next.RunStatus == v1api.RunStatusRunning), nil
		},
		func(ctx context.Context) {
			if !c.showCloudLogs {
				return
			}

			c.gs.Logger.Debug("Connecting to cloud logs server...")
			if err := cfg.StreamLogsToLogger(ctx, c.gs.Logger, refID, 0); err != nil {
				c.gs.Logger.WithError(err).Error("error while tailing cloud logs")
			}
		},
	); err != nil {
		return nil, err
	}

	mu.Lock()
	defer mu.Unlock()
	if progress == nil {
		return nil, errext.WithExitCodeIfNone(errors.New("test progress error"), exitcodes.CloudFailedToGetProgress)
	}

	return progress, nil
}

func runningCloudProgress(startTime *time.Time) time.Duration {
	if startTime.IsZero() {
		*startTime = time.Now()
	}

	return time.Since(*startTime)
}

func runningCloudStatus(spent, maxDuration time.Duration) string {
	if spent > maxDuration {
		return maxDuration.String()
	}

	return fmt.Sprintf("%s/%s", pb.GetFixedLengthDuration(spent, maxDuration), maxDuration)
}

func v1CloudTestProgress(testProgress *v1api.TestProgressResponse, startTime *time.Time,
	maxDuration time.Duration,
) (float64, string) {
	statusText := testProgress.RunStatusText

	switch testProgress.RunStatus { //nolint:exhaustive
	case v1api.RunStatusFinished:
		testProgress.Progress = 1
	case v1api.RunStatusRunning:
		spent := runningCloudProgress(startTime)
		statusText = runningCloudStatus(spent, maxDuration)
	}

	return testProgress.Progress, statusText
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

func (c *cmdCloud) reportCloudTestResult(testProgress *v1api.TestProgressResponse) error {
	c.reportCloudTestStatus(testProgress.RunStatusText)

	if testProgress.ResultStatus != v1api.ResultStatusFailed {
		return nil
	}

	// Although by looking at [ResultStatus] and [RunStatus] isn't self-explanatory,
	// the scenario when the test run has finished, but it failed is an exceptional case for those situations
	// when thresholds have been crossed (failed). So, we report this situation as such.
	if testProgress.RunStatus == v1api.RunStatusFinished ||
		testProgress.RunStatus == v1api.RunStatusAbortedThreshold {
		//nolint:staticcheck
		return errext.WithExitCodeIfNone(errors.New("Thresholds have been crossed"), exitcodes.ThresholdsHaveFailed)
	}

	// TODO: use different exit codes for failed thresholds vs failed test (e.g. aborted by system/limit)
	return errext.WithExitCodeIfNone(errors.New("The test has failed"), exitcodes.CloudTestRunFailed) //nolint:staticcheck
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
