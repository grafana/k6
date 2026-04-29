package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"text/tabwriter"
	"time"

	k6cloud "github.com/grafana/k6-cloud-openapi-client-go/k6"
	"github.com/spf13/cobra"

	"go.k6.io/k6/v2/cloudapi"
	"go.k6.io/k6/v2/cmd/state"
	"go.k6.io/k6/v2/internal/build"
	v6cloudapi "go.k6.io/k6/v2/internal/cloudapi/v6"
)

var errMissingTestID = errors.New("test ID not specified, use --test-id")

type cmdCloudTestRunList struct {
	globalState *state.GlobalState
	testID      int64
	limit       int32
	all         bool
	since       time.Duration
	isJSON      bool
}

func getCmdCloudTestRunList(testRunCmd *cmdCloudTestRun) *cobra.Command {
	c := &cmdCloudTestRunList{
		globalState: testRunCmd.globalState,
	}

	exampleText := getExampleText(testRunCmd.globalState, `
  # List the most recent runs of a test
  $ {{.}} cloud test-run list --test-id 8410

  # List runs created in the last 7 days
  $ {{.}} cloud test-run list --test-id 8410 --since 168h

  # List every run of a test (auto-paginates)
  $ {{.}} cloud test-run list --test-id 8410 --all`[1:])

	listCmd := &cobra.Command{
		Use:     "list",
		Short:   "List Grafana Cloud k6 test runs",
		Long:    `List runs of a Grafana Cloud k6 load test, most recent first.`,
		Example: exampleText,
		Args:    cobra.NoArgs,
		RunE:    c.run,
	}

	listCmd.Flags().Int64Var(&c.testID, "test-id", 0,
		"ID of the load test to list runs for (required)")
	listCmd.Flags().Int32Var(&c.limit, "limit", 30,
		"maximum number of runs to return; ignored when --all is set")
	listCmd.Flags().BoolVar(&c.all, "all", false,
		"return every run, paginating through all results")
	listCmd.Flags().DurationVar(&c.since, "since", 0,
		"only show runs created within this relative duration (e.g. 24h, 168h)")
	listCmd.Flags().BoolVar(&c.isJSON, "json", false, "output run list in JSON format")

	return listCmd
}

func (c *cmdCloudTestRunList) run(_ *cobra.Command, _ []string) error {
	if c.testID <= 0 {
		return errMissingTestID
	}
	if c.testID < math.MinInt32 || c.testID > math.MaxInt32 {
		return fmt.Errorf("test ID %d overflows int32", c.testID)
	}

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

	if err := checkCloudLogin(cloudConfig); err != nil {
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

	if cloudConfig.StackID.Int64 < math.MinInt32 || cloudConfig.StackID.Int64 > math.MaxInt32 {
		return fmt.Errorf("stack ID %d overflows int32", cloudConfig.StackID.Int64)
	}
	client.SetStackID(int32(cloudConfig.StackID.Int64))

	opts := v6cloudapi.ListTestRunsOptions{
		Limit: c.limit,
		All:   c.all,
	}
	if c.since > 0 {
		opts.CreatedAfter = time.Now().Add(-c.since)
	}

	resp, err := client.ListTestRuns(c.globalState.Ctx, int32(c.testID), opts)
	if err != nil {
		return err
	}

	if c.isJSON {
		return c.outputJSON(resp.Value)
	}

	header := fmt.Sprintf("Runs of test %d:\n\n", c.testID)

	if len(resp.Value) == 0 {
		printToStdout(c.globalState, header+"No runs found.\n")
		return nil
	}

	printToStdout(c.globalState, header+formatTestRunTable(resp.Value))
	return nil
}

func (c *cmdCloudTestRunList) outputJSON(runs []k6cloud.TestRunApiModel) error {
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(runs); err != nil {
		return fmt.Errorf("failed to encode test run list: %w", err)
	}

	printToStdout(c.globalState, buf.String())
	return nil
}

func formatTestRunTable(runs []k6cloud.TestRunApiModel) string {
	const dateFormat = "2006-01-02 15:04"

	var buf strings.Builder
	w := tabwriter.NewWriter(&buf, 0, 0, 3, ' ', 0)
	_, _ = fmt.Fprintln(w, "ID\tSTATUS\tSTARTED\tDURATION\tVUS\tRESULT")
	for _, r := range runs {
		_, _ = fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\n",
			r.Id,
			r.Status,
			r.Created.UTC().Format(dateFormat),
			formatRunDuration(r.ExecutionDuration),
			nullableInt32String(r.MaxVus),
			nullableString(r.Result),
		)
	}
	_ = w.Flush()
	return buf.String()
}

func formatRunDuration(seconds int32) string {
	if seconds <= 0 {
		return "-"
	}
	return (time.Duration(seconds) * time.Second).String()
}

func nullableInt32String(v k6cloud.NullableInt32) string {
	if !v.IsSet() || v.Get() == nil {
		return "-"
	}
	return fmt.Sprintf("%d", *v.Get())
}

func nullableString(v k6cloud.NullableString) string {
	if !v.IsSet() || v.Get() == nil || *v.Get() == "" {
		return "-"
	}
	return *v.Get()
}
