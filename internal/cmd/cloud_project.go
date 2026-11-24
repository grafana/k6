package cmd

import (
	"github.com/spf13/cobra"
	"go.k6.io/k6/v2/cmd/state"
)

type cmdCloudProject struct {
	globalState *state.GlobalState
}

func getCmdCloudProject(gs *state.GlobalState) *cobra.Command {
	c := &cmdCloudProject{
		globalState: gs,
	}

	exampleText := getExampleText(gs, `
  # List all projects in the configured stack
  $ {{.}} cloud project list

  # List projects in JSON format
  $ {{.}} cloud project list --json`[1:])

	cloudProjectCommand := &cobra.Command{
		Use:   "project",
		Short: "Work with Grafana Cloud k6 projects",
		Long:  `Work with Grafana Cloud k6 projects.`,

		Example: exampleText,
	}

	cloudUsageTemplate := getCloudUsageTemplate()

	listCmd := getCmdCloudProjectList(c)
	listCmd.SetUsageTemplate(cloudUsageTemplate)
	listCmd.SetHelpTemplate(cloudUsageTemplate)
	cloudProjectCommand.AddCommand(listCmd)

	cloudProjectCommand.SetUsageTemplate(cloudUsageTemplate)

	return cloudProjectCommand
}
