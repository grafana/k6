package cmd

import (
	"go.k6.io/k6/cmd/state"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const cloudUploadCommandName = "upload"

type cmdCloudUpload struct {
	globalState *state.GlobalState

	// deprecatedCloudCmd holds an instance of the k6 cloud command that we store
	// in order to be able to call its run method to support the cloud upload
	// feature
	deprecatedCloudCmd *cmdCloud
}

func getCmdCloudUpload(cloudCmd *cmdCloud) *cobra.Command {
	c := &cmdCloudUpload{
		globalState:        cloudCmd.gs,
		deprecatedCloudCmd: cloudCmd,
	}

	// uploadCloudCommand represents the 'cloud upload' command
	exampleText := getExampleText(cloudCmd.gs, `
  # Upload the test script and its resources to the Grafana Cloud k6 without actually starting a test run
  $ {{.}} cloud upload script.js`[1:])

	uploadCloudCommand := &cobra.Command{
		Use:   cloudUploadCommandName,
		Short: "Upload the test script to the Grafana Cloud k6",
		Long: `Upload the test script and its resources to the Grafana Cloud k6.

This will upload the test script and its resources to the Grafana Cloud k6 service. 
Using this command requires to be authenticated against the Grafana Cloud k6. 
Use the "k6 cloud login" command to authenticate.
`,
		Example: exampleText,
		Args:    exactArgsWithMsg(1, "arg should either be \"-\", if reading script from stdin, or a path to a script file"),
		PreRunE: c.preRun,
		RunE:    c.run,
	}

	uploadCloudCommand.Flags().AddFlagSet(c.flagSet())

	return uploadCloudCommand
}

func (c *cmdCloudUpload) preRun(cmd *cobra.Command, args []string) error {
	return c.deprecatedCloudCmd.preRun(cmd, args)
}

// run is the code that runs when the user executes `k6 cloud upload`
func (c *cmdCloudUpload) run(cmd *cobra.Command, args []string) error {
	c.deprecatedCloudCmd.uploadOnly = true
	return c.deprecatedCloudCmd.run(cmd, args)
}

func (c *cmdCloudUpload) flagSet() *pflag.FlagSet {
	flags := pflag.NewFlagSet("", pflag.ContinueOnError)
	flags.SortFlags = false
	flags.AddFlagSet(optionFlagSet())
	flags.AddFlagSet(runtimeOptionFlagSet(false))
	return flags
}
