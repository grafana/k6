package cmd

import (
	"github.com/spf13/cobra"
	"go.k6.io/k6/cmd/state"
)

const cloudRunCommandName string = "run"

func getCmdCloudRun(gs *state.GlobalState) *cobra.Command {
	deprecatedCloudCmd := &cmdCloud{
		gs:            gs,
		showCloudLogs: true,
		exitOnRunning: false,
		uploadOnly:    false,
	}

	exampleText := getExampleText(gs, `
  # Run a test script in Grafana Cloud k6
  $ {{.}} cloud run script.js

  # Run a test archive in Grafana Cloud k6
  $ {{.}} cloud run archive.tar

  # Read a test script or archive from stdin and run it in Grafana Cloud k6
  $ {{.}} cloud run - < script.js`[1:])

	cloudRunCmd := &cobra.Command{
		Use:   cloudRunCommandName,
		Short: "Run a test in Grafana Cloud k6",
		Long: `Run a test in Grafana Cloud k6.

This will archive test script(s), including all necessary resources, and execute the test in the Grafana Cloud k6
service. Using this command requires to be authenticated against Grafana Cloud k6.
Use the "k6 cloud login" command to authenticate.`,
		Example: exampleText,
		Args: exactArgsWithMsg(1,
			"the k6 cloud run command expects a single argument consisting in either a path to a script or "+
				"archive file, or the \"-\" symbol indicating the script or archive should be read from stdin",
		),
		PreRunE: deprecatedCloudCmd.preRun,
		RunE:    deprecatedCloudCmd.run,
	}

	cloudRunCmd.Flags().SortFlags = false
	cloudRunCmd.Flags().AddFlagSet(deprecatedCloudCmd.flagSet())

	return cloudRunCmd
}
