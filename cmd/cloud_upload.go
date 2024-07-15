package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"path/filepath"

	"go.k6.io/k6/cloudapi"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/consts"
	"go.k6.io/k6/ui/pb"
)

// cmdCloudUpload handles the `k6 cloud upload` sub-command
type cmdCloudUpload struct {
	gs *state.GlobalState
}

func getCmdCloudUpload(gs *state.GlobalState) *cobra.Command {
	c := &cmdCloudUpload{
		gs: gs,
	}

	exampleText := getExampleText(gs, `
  # Upload the test to the k6 Cloud without actually starting a test run
  $ {{.}} cloud upload script.js

  # Upload the test to the k6 Cloud with a specific token
  $ {{.}} cloud upload --token <YOUR_API_TOKEN> script.js`[1:])

	cloudRunCmd := &cobra.Command{
		Use:   cloudUploadCommandName,
		Short: "Upload the test to the Grafana k6 Cloud",
		Long: `Upload the test to the Grafana k6 Cloud.

This will upload the test to the Grafana k6 Cloud service. Using this command requires to be authenticated
against the Grafana k6 Cloud. Use the "k6 cloud login" command to authenticate.`,
		Example: exampleText,
		Args:    exactArgsWithMsg(1, "arg should either be \"-\", if reading script from stdin, or a path to a script file"),
		RunE:    c.run,
	}

	cloudRunCmd.Flags().SortFlags = false
	cloudRunCmd.Flags().AddFlagSet(c.flagSet())

	return cloudRunCmd
}

func (c *cmdCloudUpload) run(cmd *cobra.Command, args []string) error {
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

	logger := c.gs.Logger

	// Start cloud test upload
	modifyAndPrintBar(c.gs, progressBar, pb.WithConstProgress(0, "Validating script options"))
	client := cloudapi.NewClient(
		logger, cloudConfig.Token.String, cloudConfig.Host.String, consts.Version, cloudConfig.Timeout.TimeDuration())
	if err = client.ValidateOptions(arc.Options); err != nil {
		return err
	}

	modifyAndPrintBar(c.gs, progressBar, pb.WithConstProgress(0, "Uploading archive"))

	var cloudTestRun *cloudapi.CreateTestRunResponse
	cloudTestRun, err = client.UploadTestOnly(name, cloudConfig.ProjectID.Int64, arc)
	if err != nil {
		return err
	}

	refID := cloudTestRun.ReferenceID
	if cloudTestRun.ConfigOverride != nil {
		cloudConfig = cloudConfig.Apply(*cloudTestRun.ConfigOverride)
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

	if !c.gs.Flags.Quiet {
		valueColor := getColor(c.gs.Flags.NoColor || !c.gs.Stdout.IsTTY, color.FgCyan)
		printToStdout(c.gs, fmt.Sprintf(
			"     test status: %s\n", valueColor.Sprint("Archived"),
		))
	} else {
		logger.WithField("run_status", "Archived").Debug("Test uploaded")
	}

	return nil
}

func (c *cmdCloudUpload) flagSet() *pflag.FlagSet {
	flags := pflag.NewFlagSet("", pflag.ContinueOnError)
	flags.SortFlags = false
	flags.AddFlagSet(optionFlagSet())
	flags.AddFlagSet(runtimeOptionFlagSet(false))

	return flags
}

const cloudUploadCommandName string = "upload"
