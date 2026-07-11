package cmd

import (
	"github.com/spf13/cobra"
	"go.k6.io/k6/v2/cmd/state"
)

type cmdCloudLoadZone struct {
	globalState *state.GlobalState
}

func getCmdCloudLoadZone(cloudCmd *cmdCloud) *cobra.Command {
	c := &cmdCloudLoadZone{
		globalState: cloudCmd.gs,
	}

	exampleText := getExampleText(cloudCmd.gs, `
  # List all load zones available in the configured stack
  $ {{.}} cloud load-zone list

  # List load zones in JSON format
  $ {{.}} cloud load-zone list --json`[1:])

	cloudLoadZoneCommand := &cobra.Command{
		Use:   "load-zone",
		Short: "Work with Grafana Cloud k6 load zones",
		Long:  `Work with Grafana Cloud k6 load zones.`,

		Example: exampleText,
	}

	cloudUsageTemplate := getCloudUsageTemplate()

	listCmd := getCmdCloudLoadZoneList(c)
	listCmd.SetUsageTemplate(cloudUsageTemplate)
	listCmd.SetHelpTemplate(cloudUsageTemplate)
	cloudLoadZoneCommand.AddCommand(listCmd)

	cloudLoadZoneCommand.SetUsageTemplate(cloudUsageTemplate)

	return cloudLoadZoneCommand
}
