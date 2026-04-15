package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/cloudapi"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/errext"
	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/internal/build"
	"go.k6.io/k6/internal/ui/pb"
	"go.k6.io/k6/lib"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// errUserUnauthenticated represents an authentication error when trying to use
// Grafana Cloud without being logged in or having a valid token.
//
//nolint:staticcheck // the error is shown to the user so here punctuation and capital are required
var errUserUnauthenticated = errors.New("To run tests in Grafana Cloud, you must first authenticate." +
	" Run the `k6 cloud login` command, or check the docs" +
	" https://grafana.com/docs/grafana-cloud/testing/k6/author-run/tokens-and-cli-authentication" +
	" for additional authentication methods.")

// cloudTestSetup holds the prepared state for a cloud test,
// shared between cloud run and cloud upload subcommands.
type cloudTestSetup struct {
	test           *loadedAndConfiguredTest
	arc            *lib.Archive
	cloudConfig    cloudapi.Config
	tmpCloudConfig map[string]any
	client         *cloudapi.Client
	name           string
	progressBar    *pb.ProgressBar
}

// prepareCloudTest loads and configures the test, consolidates cloud config,
// creates the cloud API client, validates options, and resolves the project ID.
//
//nolint:funlen
func prepareCloudTest(gs *state.GlobalState, cmd *cobra.Command, args []string) (*cloudTestSetup, error) {
	test, err := loadAndConfigureLocalTest(gs, cmd, args, getPartialConfig)
	if err != nil {
		return nil, err
	}

	// It's important to NOT set the derived options back to the runner
	// here, only the consolidated ones. Otherwise, if the script used
	// an execution shortcut option (e.g. `iterations` or `duration`),
	// we will have multiple conflicting execution options since the
	// derivation will set `scenarios` as well.
	if err := test.initRunner.SetOptions(test.consolidatedConfig.Options); err != nil {
		return nil, err
	}

	// TODO: validate for usage of execution segment
	// TODO: validate for externally controlled executor (i.e. executors that aren't distributable)
	// TODO: move those validations to a separate function and reuse validateConfig()?
	printBanner(gs)

	progressBar := pb.New(
		pb.WithConstLeft("Init"),
		pb.WithConstProgress(0, "Loading test script..."),
	)
	printBar(gs, progressBar)

	modifyAndPrintBar(gs, progressBar, pb.WithConstProgress(0, "Building the archive..."))
	arc := test.makeArchive()

	tmpCloudConfig, err := cloudapi.GetTemporaryCloudConfig(arc.Options.Cloud)
	if err != nil {
		return nil, err
	}

	// Cloud config
	cloudConfig, warn, err := cloudapi.GetConsolidatedConfig(
		test.derivedConfig.Collectors["cloud"], gs.Env, "", arc.Options.Cloud)
	if err != nil {
		return nil, err
	}
	if !cloudConfig.Token.Valid {
		return nil, errUserUnauthenticated
	}

	// Display config warning if needed
	if warn != "" {
		modifyAndPrintBar(gs, progressBar, pb.WithConstProgress(0, "Warning: "+warn))
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
		return nil, err
	}

	arc.Options.Cloud = b

	name := cloudConfig.Name.String
	if !cloudConfig.Name.Valid || cloudConfig.Name.String == "" {
		name = filepath.Base(test.sourceRootPath)
	}

	logger := gs.Logger

	// Start cloud test run
	modifyAndPrintBar(gs, progressBar, pb.WithConstProgress(0, "Validating script options"))
	client := cloudapi.NewClient(
		logger, cloudConfig.Token.String, cloudConfig.Host.String, build.Version, cloudConfig.Timeout.TimeDuration())
	if cloudConfig.StackID.Valid {
		client.SetStackID(cloudConfig.StackID.Int64)
	}
	if err = client.ValidateOptions(arc.Options); err != nil {
		return nil, err
	}

	if cloudConfig.ProjectID.Int64 == 0 {
		if err := resolveAndSetProjectID(gs, &cloudConfig, tmpCloudConfig, arc); err != nil {
			return nil, err
		}
	}

	modifyAndPrintBar(gs, progressBar, pb.WithConstProgress(0, "Uploading archive"))

	return &cloudTestSetup{
		test:           test,
		arc:            arc,
		cloudConfig:    cloudConfig,
		tmpCloudConfig: tmpCloudConfig,
		client:         client,
		name:           name,
		progressBar:    progressBar,
	}, nil
}

// trackCloudTestProgress handles signal trapping, progress bar display,
// log streaming, and polling the cloud API for test progress until completion.
//
//nolint:funlen,gocognit
func trackCloudTestProgress(
	gs *state.GlobalState,
	setup *cloudTestSetup,
	refID string,
	cloudConfig cloudapi.Config,
	showCloudLogs bool,
	exitOnRunning bool,
) error {
	globalCtx, globalCancel := context.WithCancel(gs.Ctx)
	defer globalCancel()

	logger := gs.Logger

	// Trap Interrupts, SIGINTs and SIGTERMs.
	gracefulStop := func(sig os.Signal) {
		logger.WithField("sig", sig).Print("Stopping cloud test run in response to signal...")
		// Do this in a separate goroutine so that if it blocks, the
		// second signal can still abort the process execution.
		go func() {
			stopErr := setup.client.StopCloudTestRun(refID)
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
	defer stopSignalHandling()

	et, err := lib.NewExecutionTuple(
		setup.test.derivedConfig.ExecutionSegment,
		setup.test.derivedConfig.ExecutionSegmentSequence,
	)
	if err != nil {
		return err
	}
	testURL := cloudapi.URLForResults(refID, cloudConfig)
	executionPlan := setup.test.derivedConfig.Scenarios.GetFullExecutionRequirements(et)
	printExecutionDescription(
		gs, "cloud", setup.test.sourceRootPath, testURL, setup.test.derivedConfig, et, executionPlan, nil,
	)

	modifyAndPrintBar(
		gs, setup.progressBar,
		pb.WithConstLeft("Run "), pb.WithConstProgress(0, "Initializing the cloud test"),
	)

	progressCtx, progressCancel := context.WithCancel(globalCtx)
	progressBarWG := &sync.WaitGroup{}
	progressBarWG.Add(1)
	defer progressBarWG.Wait()
	defer progressCancel()
	go func() {
		showProgress(progressCtx, gs, []*pb.ProgressBar{setup.progressBar}, logger)
		progressBarWG.Done()
	}()

	var (
		startTime   time.Time
		maxDuration time.Duration
	)
	maxDuration, _ = lib.GetEndOffset(executionPlan)

	testProgressLock := &sync.Mutex{}
	var testProgress *cloudapi.TestProgressResponse
	setup.progressBar.Modify(
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
	if showCloudLogs {
		go func() {
			logger.Debug("Connecting to cloud logs server...")
			if err := cloudConfig.StreamLogsToLogger(globalCtx, logger, refID, 0); err != nil {
				logger.WithError(err).Error("error while tailing cloud logs")
			}
		}()
	}

	for range ticker.C {
		newTestProgress, progressErr := setup.client.GetTestProgress(refID)
		if progressErr != nil {
			logger.WithError(progressErr).Error("Test progress error")
			continue
		}

		testProgressLock.Lock()
		testProgress = newTestProgress
		testProgressLock.Unlock()

		if (newTestProgress.RunStatus > cloudapi.RunStatusRunning) ||
			(exitOnRunning && newTestProgress.RunStatus == cloudapi.RunStatusRunning) {
			globalCancel()
			break
		}
	}

	if testProgress == nil {
		//nolint:staticcheck
		return errext.WithExitCodeIfNone(errors.New("Test progress error"), exitcodes.CloudFailedToGetProgress)
	}

	if !gs.Flags.Quiet {
		valueColor := getColor(gs.Flags.NoColor || !gs.Stdout.IsTTY, color.FgCyan)
		printToStdout(gs, fmt.Sprintf(
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

func getCmdCloud(gs *state.GlobalState) *cobra.Command {
	exampleText := getExampleText(gs, `
  # Authenticate with Grafana Cloud k6
  $ {{.}} cloud login

  # Run a test script in Grafana Cloud k6
  $ {{.}} cloud run script.js

  # Run a k6 archive in Grafana Cloud k6
  $ {{.}} cloud run archive.tar

  # Upload a test script to Grafana Cloud k6
  $ {{.}} cloud upload script.js`[1:])

	cloudCmd := &cobra.Command{
		Use:   "cloud",
		Short: "Run a test on the cloud",
		Long: `Manage Grafana Cloud k6 tests.

This command provides subcommands for interacting with Grafana Cloud k6:
- "run": Run a test in Grafana Cloud k6
- "login": Authenticate with Grafana Cloud k6
- "upload": Upload a test script to Grafana Cloud k6 without running it

The direct usage of "k6 cloud script.js" has been removed in v2.0.0.
Please use "k6 cloud run script.js" instead.`,
		Args:    exactCloudArgs(),
		Example: exampleText,
	}

	// Register `k6 cloud` subcommands with default usage template
	defaultUsageTemplate := (&cobra.Command{}).UsageTemplate()
	defaultUsageTemplate = strings.ReplaceAll(defaultUsageTemplate, "FlagUsages", "FlagUsagesWrapped 120")

	runCmd := getCmdCloudRun(gs)
	runCmd.SetUsageTemplate(defaultUsageTemplate)
	cloudCmd.AddCommand(runCmd)

	loginCmd := getCmdCloudLogin(gs)
	loginCmd.SetUsageTemplate(defaultUsageTemplate)
	cloudCmd.AddCommand(loginCmd)

	uploadCmd := getCmdCloudUpload(gs)
	uploadCmd.SetUsageTemplate(defaultUsageTemplate)
	cloudCmd.AddCommand(uploadCmd)

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

func exactCloudArgs() cobra.PositionalArgs {
	return func(_ *cobra.Command, args []string) error {
		const baseErrMsg = `the "k6 cloud" command requires a subcommand such as "run", "login", or "upload"`

		if len(args) == 0 {
			return fmt.Errorf(baseErrMsg + "; " + "received no arguments")
		}

		hasSubcommand := args[0] == "run" || args[0] == "login" || args[0] == "upload"
		if !hasSubcommand {
			return fmt.Errorf(
				baseErrMsg+"; "+
					`direct script execution has been removed in v2.0.0. `+
					`To run "%s", use: k6 cloud run %s`,
				args[0], args[0],
			)
		}

		return nil
	}
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
