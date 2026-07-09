package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"go.k6.io/k6/v2/cloudapi"
	"go.k6.io/k6/v2/cmd/state"
	"go.k6.io/k6/v2/internal/build"
	cloudapiv6 "go.k6.io/k6/v2/internal/cloudapi/v6"
)

type cmdCloudProjectList struct {
	globalState *state.GlobalState
	isJSON      bool
}

func getCmdCloudProjectList(projectCmd *cmdCloudProject) *cobra.Command {
	c := &cmdCloudProjectList{
		globalState: projectCmd.globalState,
	}

	exampleText := getExampleText(projectCmd.globalState, `
  # List all projects in the configured stack
  $ {{.}} cloud project list`[1:])

	listCmd := &cobra.Command{
		Use:     "list",
		Short:   "List Grafana Cloud k6 projects",
		Long:    `List all projects in the configured Grafana Cloud k6 stack.`,
		Example: exampleText,
		Args:    cobra.NoArgs,
		RunE:    c.run,
	}

	listCmd.Flags().BoolVar(&c.isJSON, "json", false, "output project list in JSON format")

	return listCmd
}

func (c *cmdCloudProjectList) run(_ *cobra.Command, _ []string) error {
	currentDiskConf, err := readDiskConfig(c.globalState)
	if err != nil {
		return err
	}

	currentJSONConfigRaw := currentDiskConf.Collectors["cloud"]

	cloudConfig, warn, err := cloudapi.GetConsolidatedConfig(
		currentJSONConfigRaw, c.globalState.Env, "", nil)
	if err != nil {
		return err
	}
	if warn != "" {
		c.globalState.Logger.Warn(warn)
	}

	if err := checkCloudLoginFor(cloudConfig, "Listing cloud projects requires auth settings"); err != nil {
		return err
	}

	client, err := cloudapiv6.NewClient(
		c.globalState.Logger,
		cloudConfig.Token.String,
		cloudConfig.Hostv6.String,
		build.Version,
		cloudConfig.Timeout.TimeDuration(),
	)
	if err != nil {
		return err
	}

	if cloudConfig.StackID.Int64 < math.MinInt32 || cloudConfig.StackID.Int64 > math.MaxInt32 {
		return fmt.Errorf("stack ID %d overflows int32", cloudConfig.StackID.Int64)
	}
	client.SetStackID(int32(cloudConfig.StackID.Int64))

	projects, err := client.ListProjects(c.globalState.Ctx)
	if err != nil {
		return err
	}

	if c.isJSON {
		return c.outputJSON(projects)
	}

	stackName := cloudConfig.StackURL.String
	if !cloudConfig.StackURL.Valid {
		stackName = fmt.Sprintf("stack-%d", cloudConfig.StackID.Int64)
	}
	stackHeader := fmt.Sprintf("Projects for %s:\n\n", stackName)

	if len(projects) == 0 {
		printToStdout(c.globalState, stackHeader+
			"No projects found.\n"+
			"To create a project, visit https://grafana.com/docs/grafana-cloud/testing/k6/projects-and-users/projects/\n")
		return nil
	}

	printToStdout(c.globalState, stackHeader+formatProjectTable(projects))
	return nil
}

func (c *cmdCloudProjectList) outputJSON(projects []cloudapiv6.Project) error {
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(projects); err != nil {
		return fmt.Errorf("failed to encode project list: %w", err)
	}

	printToStdout(c.globalState, buf.String())
	return nil
}

func formatProjectTable(projects []cloudapiv6.Project) string {
	var buf strings.Builder
	w := tabwriter.NewWriter(&buf, 0, 0, 3, ' ', 0)
	_, _ = fmt.Fprintln(w, "ID\tNAME\tDEFAULT")
	for _, p := range projects {
		def := "no"
		if p.IsDefault {
			def = "yes"
		}
		_, _ = fmt.Fprintf(w, "%d\t%s\t%s\n", p.ID, p.Name, def)
	}
	_ = w.Flush()
	return buf.String()
}
