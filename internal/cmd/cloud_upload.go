package cmd

import (
	"go.k6.io/k6/v2/cmd/state"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const cloudUploadCommandName = "upload"

type cmdCloudUpload struct {
	globalState *state.GlobalState
}

func getCmdCloudUpload(gs *state.GlobalState) *cobra.Command {
	c := &cmdCloudUpload{
		globalState: gs,
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

func (c *cmdCloudUpload) preRun(cmd *cobra.Command, _ []string) error {
	// The upload command doesn't expose the --show-logs/--exit-on-running flags,
	// but we still validate the corresponding env variables for wrong values.
	var showCloudLogs, exitOnRunning bool
	return applyCloudEnvOverrides(c.globalState, cmd, &showCloudLogs, &exitOnRunning)
}

// run is the code that runs when the user executes `k6 cloud upload`
func (c *cmdCloudUpload) run(cmd *cobra.Command, args []string) error {
	return runCloudTest(c.globalState, cmd, args, false, false, true)
}

func (c *cmdCloudUpload) flagSet() *pflag.FlagSet {
	flags := pflag.NewFlagSet("", pflag.ContinueOnError)
	flags.SortFlags = false
	flags.AddFlagSet(optionFlagSet())
	flags.AddFlagSet(runtimeOptionFlagSet(false))
	return flags
}
