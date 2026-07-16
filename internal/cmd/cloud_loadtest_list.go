package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/v2/cloudapi"
	"go.k6.io/k6/v2/cmd/state"
	"go.k6.io/k6/v2/internal/build"
	v6cloudapi "go.k6.io/k6/v2/internal/cloudapi/v6"
)

type cmdCloudTestList struct {
	globalState *state.GlobalState
	projectID   int64
	isJSON      bool
}

func getCmdCloudTestList(testCmd *cmdCloudTest) *cobra.Command {
	c := &cmdCloudTestList{
		globalState: testCmd.globalState,
	}

	exampleText := getExampleText(testCmd.globalState, `
  # List tests in the default project
  $ {{.}} cloud test list

  # List tests in a specific project
  $ {{.}} cloud test list --project-id 1234`[1:])

	listCmd := &cobra.Command{
		Use:     "list",
		Short:   "List Grafana Cloud k6 tests",
		Long:    `List load tests in a Grafana Cloud k6 project.`,
		Example: exampleText,
		Args:    cobra.NoArgs,
		RunE:    c.run,
	}

	listCmd.Flags().Int64Var(&c.projectID, "project-id", 0,
		"ID of the project to list tests for; defaults to the configured project")
	listCmd.Flags().BoolVar(&c.isJSON, "json", false, "output test list in JSON format")

	return listCmd
}

func (c *cmdCloudTestList) run(cmd *cobra.Command, _ []string) error {
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

	if err := checkCloudLoginFor(cloudConfig, "Listing cloud tests requires auth settings"); err != nil {
		return err
	}

	projectID, err := c.resolveProjectID(cloudConfig, cmd.Flags().Changed("project-id"))
	if err != nil {
		return err
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

	if err := client.SetStackID(cloudConfig.StackID.Int64); err != nil {
		return err
	}

	tests, err := client.ListLoadTests(c.globalState.Ctx, projectID)
	if err != nil {
		return err
	}

	if c.isJSON {
		return c.outputJSON(tests)
	}

	header := fmt.Sprintf("Tests in project %d:\n\n", projectID)

	if len(tests) == 0 {
		printToStdout(c.globalState, header+
			"No tests found.\n"+
			"To create a test, visit https://grafana.com/docs/grafana-cloud/testing/k6/author-run/test-builder/\n")
		return nil
	}

	printToStdout(c.globalState, header+formatLoadTestTable(tests))
	return nil
}

// resolveProjectID returns the project ID to list tests for. An explicit
// --project-id flag outranks every configuration layer, so it is applied on top
// of the already-consolidated config before resolution is delegated to the
// shared resolveDefaultProjectID helper (config projectID, then the stack's
// default project). Unlike `k6 cloud run`, listing needs a concrete project, so
// a zero result is treated as an error.
func (c *cmdCloudTestList) resolveProjectID(cloudConfig cloudapi.Config, projectIDSet bool) (int64, error) {
	if projectIDSet {
		if c.projectID <= 0 {
			return 0, errNoProjectConfigured
		}
		cloudConfig.ProjectID = null.IntFrom(c.projectID)
	}

	id, err := resolveDefaultProjectID(c.globalState, &cloudConfig)
	if err != nil {
		return 0, err
	}
	if id == 0 {
		return 0, errNoProjectConfigured
	}

	return id, nil
}

func (c *cmdCloudTestList) outputJSON(tests []v6cloudapi.LoadTest) error {
	// If tests is nil, initialize it to an empty slice to avoid encoding it as null.
	if tests == nil {
		tests = []v6cloudapi.LoadTest{}
	}

	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(tests); err != nil {
		return fmt.Errorf("failed to encode test list: %w", err)
	}

	printToStdout(c.globalState, buf.String())
	return nil
}

func formatLoadTestTable(tests []v6cloudapi.LoadTest) string {
	const dateFormat = "2006-01-02 15:04"

	var buf strings.Builder
	w := tabwriter.NewWriter(&buf, 0, 0, 3, ' ', 0)
	_, _ = fmt.Fprintln(w, "ID\tNAME\tCREATED\tUPDATED")
	for _, t := range tests {
		_, _ = fmt.Fprintf(w, "%d\t%s\t%s\t%s\n",
			t.ID, t.Name,
			t.Created.UTC().Format(dateFormat),
			t.Updated.UTC().Format(dateFormat),
		)
	}
	_ = w.Flush()
	return buf.String()
}
