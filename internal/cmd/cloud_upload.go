package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"go.k6.io/k6/v2/cloudapi"
	"go.k6.io/k6/v2/cmd/state"
	"go.k6.io/k6/v2/internal/build"
	cloudapiv6 "go.k6.io/k6/v2/internal/cloudapi/v6"
	"go.k6.io/k6/v2/internal/ui/pb"
	"go.k6.io/k6/v2/lib"
)

const cloudUploadCommandName = "upload"

type cmdCloudUpload struct {
	gs *state.GlobalState
}

func getCmdCloudUpload(gs *state.GlobalState) *cobra.Command {
	c := &cmdCloudUpload{
		gs: gs,
	}

	// uploadCloudCommand represents the 'cloud upload' command
	exampleText := getExampleText(gs, `
  # Upload a test to Grafana Cloud without running it
  $ {{.}} cloud upload script.js`[1:])

	uploadCloudCommand := &cobra.Command{
		Use:     cloudUploadCommandName,
		Short:   "Upload a test to Grafana Cloud",
		Long:    "Upload a test to Grafana Cloud without running it. Requires authentication via \"k6 cloud login\".",
		Example: exampleText,
		Args:    exactArgsWithMsg(1, "arg should either be \"-\", if reading script from stdin, or a path to a script file"),
		PreRunE: c.preRun,
		RunE:    c.run,
	}

	uploadCloudCommand.Flags().AddFlagSet(c.flagSet())

	return uploadCloudCommand
}

func (c *cmdCloudUpload) preRun(_ *cobra.Command, _ []string) error {
	// `cloud upload` does not honor any cloud-specific env vars: the streaming
	// log and exit-on-running options are only meaningful for `cloud run`.
	return nil
}

// run uploads a test archive to Grafana Cloud without starting a test run.
func (c *cmdCloudUpload) run(cmd *cobra.Command, args []string) error {
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

func (c *cmdCloudUpload) printTestStatus(status string) {
	if !c.gs.Flags.Quiet {
		valueColor := getColor(c.gs.Flags.NoColor || !c.gs.Stdout.IsTTY, color.FgCyan)
		printToStdout(c.gs, fmt.Sprintf(
			"     test status: %s\n", valueColor.Sprint(status),
		))
	} else {
		c.gs.Logger.WithField("run_status", status).Debug("Test finished")
	}
}

func (c *cmdCloudUpload) flagSet() *pflag.FlagSet {
	flags := pflag.NewFlagSet("", pflag.ContinueOnError)
	flags.SortFlags = false
	flags.AddFlagSet(optionFlagSet())
	flags.AddFlagSet(runtimeOptionFlagSet(false))
	return flags
}
