package cmd

import (
	"go.k6.io/k6/cmd/state"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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
		RunE:    c.run,
	}

	uploadCloudCommand.Flags().AddFlagSet(c.flagSet())

	return uploadCloudCommand
}

// run is the code that runs when the user executes `k6 cloud upload`
func (c *cmdCloudUpload) run(cmd *cobra.Command, args []string) error {
	setup, err := prepareCloudTest(c.gs, cmd, args)
	if err != nil {
		return err
	}

	cloudTestRun, err := setup.client.UploadTestOnly(setup.name, setup.cloudConfig.ProjectID.Int64, setup.arc)
	if err != nil {
		return err
	}

	refID := cloudTestRun.ReferenceID
	cloudConfig := setup.cloudConfig
	if cloudTestRun.ConfigOverride != nil {
		cloudConfig = cloudConfig.Apply(*cloudTestRun.ConfigOverride)
	}

	return trackCloudTestProgress(c.gs, setup, refID, cloudConfig, false, false)
}

func (c *cmdCloudUpload) flagSet() *pflag.FlagSet {
	flags := pflag.NewFlagSet("", pflag.ContinueOnError)
	flags.SortFlags = false
	flags.AddFlagSet(optionFlagSet())
	flags.AddFlagSet(runtimeOptionFlagSet(false))
	return flags
}
