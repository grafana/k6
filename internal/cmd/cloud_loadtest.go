package cmd

import (
	"github.com/spf13/cobra"
	"go.k6.io/k6/v2/cmd/state"
)

type cmdCloudTest struct {
	globalState *state.GlobalState
}

func getCmdCloudTest(cloudCmd *cmdCloud) *cobra.Command {
	c := &cmdCloudTest{
		globalState: cloudCmd.gs,
	}

	exampleText := getExampleText(cloudCmd.gs, `
  # List tests in the default project
  $ {{.}} cloud test list

  # List tests in a specific project
  $ {{.}} cloud test list --project-id 1234

  # List tests in JSON format
  $ {{.}} cloud test list --json`[1:])

	cloudTestCommand := &cobra.Command{
		Use:   "test",
		Short: "Work with Grafana Cloud k6 tests",
		Long:  `Work with Grafana Cloud k6 tests.`,

		Example: exampleText,
	}

	cloudUsageTemplate := getCloudUsageTemplate()

	listCmd := getCmdCloudTestList(c)
	listCmd.SetUsageTemplate(cloudUsageTemplate)
	listCmd.SetHelpTemplate(cloudUsageTemplate)
	cloudTestCommand.AddCommand(listCmd)

	cloudTestCommand.SetUsageTemplate(cloudUsageTemplate)

	return cloudTestCommand
}
