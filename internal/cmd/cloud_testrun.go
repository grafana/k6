package cmd

import (
	"github.com/spf13/cobra"
	"go.k6.io/k6/v2/cmd/state"
)

type cmdCloudTestRun struct {
	globalState *state.GlobalState
}

func getCmdCloudTestRun(cloudCmd *cmdCloud) *cobra.Command {
	c := &cmdCloudTestRun{
		globalState: cloudCmd.gs,
	}

	exampleText := getExampleText(cloudCmd.gs, `
  # List the most recent runs of a test
  $ {{.}} cloud test-run list --test-id 8410

  # List runs created in the last 7 days
  $ {{.}} cloud test-run list --test-id 8410 --since 168h

  # List every run of a test (auto-paginates)
  $ {{.}} cloud test-run list --test-id 8410 --all --json`[1:])

	cloudTestRunCommand := &cobra.Command{
		Use:   "test-run",
		Short: "Work with Grafana Cloud k6 test runs",
		Long:  `Work with Grafana Cloud k6 test runs.`,

		Example: exampleText,
	}

	cloudUsageTemplate := getCloudUsageTemplate()

	listCmd := getCmdCloudTestRunList(c)
	listCmd.SetUsageTemplate(cloudUsageTemplate)
	listCmd.SetHelpTemplate(cloudUsageTemplate)
	cloudTestRunCommand.AddCommand(listCmd)

	cloudTestRunCommand.SetUsageTemplate(cloudUsageTemplate)

	return cloudTestRunCommand
}
