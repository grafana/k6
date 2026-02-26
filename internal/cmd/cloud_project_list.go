package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"text/tabwriter"

	k6cloud "github.com/grafana/k6-cloud-openapi-client-go/k6"
	"github.com/spf13/cobra"

	"go.k6.io/k6/cloudapi"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/internal/build"
	v6cloudapi "go.k6.io/k6/internal/cloudapi/v6"
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
	if !checkIfMigrationCompleted(c.globalState) {
		if err := migrateLegacyConfigFileIfAny(c.globalState); err != nil {
			return err
		}
	}

	currentDiskConf, err := readDiskConfig(c.globalState)
	if err != nil {
		return err
	}

	currentJSONConfigRaw := currentDiskConf.Collectors["cloud"]

	cloudConfig, warn, err := cloudapi.GetConsolidatedConfig(
		currentJSONConfigRaw, c.globalState.Env, "", nil, nil)
	if err != nil {
		return err
	}
	if warn != "" {
		c.globalState.Logger.Warn(warn)
	}

	if !cloudConfig.Token.Valid || cloudConfig.Token.String == "" {
		return errUserUnauthenticated
	}

	if !cloudConfig.StackID.Valid || cloudConfig.StackID.Int64 == 0 {
		return errors.New(
			"no stack configured. Please run `k6 cloud login --stack <your-stack>` to set a default stack",
		)
	}

	client, err := v6cloudapi.NewClient(
		c.globalState.Logger,
		cloudConfig.Token.String,
		cloudConfig.Hostv6.String,
		build.Version,
		cloudConfig.Timeout.TimeDuration(),
	)
	if err != nil {
		return err
	}
	client.SetStackID(cloudConfig.StackID.Int64)

	resp, err := client.ListProjects()
	if err != nil {
		return err
	}

	if c.isJSON {
		return c.outputJSON(resp.Value)
	}

	stackHeader := fmt.Sprintf("Projects for stack %d:\n\n", cloudConfig.StackID.Int64)

	if len(resp.Value) == 0 {
		printToStdout(c.globalState, stackHeader+
			"No projects found.\n"+
			"To create a project, visit https://grafana.com/docs/grafana-cloud/testing/k6/projects/\n")
		return nil
	}

	printToStdout(c.globalState, stackHeader+formatProjectTable(resp.Value))
	return nil
}

func (c *cmdCloudProjectList) outputJSON(projects []k6cloud.ProjectApiModel) error {
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

func formatProjectTable(projects []k6cloud.ProjectApiModel) string {
	var buf strings.Builder
	w := tabwriter.NewWriter(&buf, 0, 0, 3, ' ', 0)
	_, _ = fmt.Fprintln(w, "ID\tNAME\tDEFAULT")
	for _, p := range projects {
		def := "no"
		if p.IsDefault {
			def = "yes"
		}
		_, _ = fmt.Fprintf(w, "%d\t%s\t%s\n", p.Id, p.Name, def)
	}
	_ = w.Flush()
	return buf.String()
}
