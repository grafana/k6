package cmd

import (
	"strings"

	"github.com/spf13/cobra"
	"go.k6.io/k6/cmd/state"
)

type cmdCloudProject struct {
	globalState *state.GlobalState
}

func getCmdCloudProject(cloudCmd *cmdCloud) *cobra.Command {
	c := &cmdCloudProject{
		globalState: cloudCmd.gs,
	}

	exampleText := getExampleText(cloudCmd.gs, `
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

	defaultUsageTemplate := (&cobra.Command{}).UsageTemplate()
	defaultUsageTemplate = strings.ReplaceAll(defaultUsageTemplate, "FlagUsages", "FlagUsagesWrapped 120")

	listCmd := getCmdCloudProjectList(c)
	listCmd.SetUsageTemplate(defaultUsageTemplate)
	cloudProjectCommand.AddCommand(listCmd)

	cloudProjectCommand.SetUsageTemplate(`Usage:
  {{.CommandPath}} <command> [flags]

Available Commands:{{range .Commands}}{{if .IsAvailableCommand}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}

Examples:
{{.Example}}
Flags:
  -h, --help   Show help
{{if .HasExample}}
{{end}}
Use "{{.CommandPath}} <command> --help" for more information about a command.
`)

	return cloudProjectCommand
}
