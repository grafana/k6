package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"go.k6.io/k6/v2/cmd/state"
	cloudapiv6 "go.k6.io/k6/v2/internal/cloudapi/v6"
)

type cmdCloudLoadZoneList struct {
	globalState *state.GlobalState
	isJSON      bool
}

func getCmdCloudLoadZoneList(loadZoneCmd *cmdCloudLoadZone) *cobra.Command {
	c := &cmdCloudLoadZoneList{
		globalState: loadZoneCmd.globalState,
	}

	exampleText := getExampleText(loadZoneCmd.globalState, `
  # List all load zones available in the configured stack
  $ {{.}} cloud load-zone list`[1:])

	listCmd := &cobra.Command{
		Use:     "list",
		Short:   "List Grafana Cloud k6 load zones",
		Long:    `List all load zones available in the configured Grafana Cloud k6 stack.`,
		Example: exampleText,
		Args:    cobra.NoArgs,
		RunE:    c.run,
	}

	listCmd.Flags().BoolVar(&c.isJSON, "json", false, "output load zone list in JSON format")

	return listCmd
}

func (c *cmdCloudLoadZoneList) run(_ *cobra.Command, _ []string) error {
	client, cloudConfig, err := newCloudV6ClientFromConfig(
		c.globalState, "Listing cloud load zones requires auth settings")
	if err != nil {
		return err
	}

	loadZones, err := client.ListLoadZones(c.globalState.Ctx)
	if err != nil {
		return err
	}

	if c.isJSON {
		return c.outputJSON(loadZones)
	}

	stackHeader := fmt.Sprintf("Load zones for %s:\n\n", cloudStackName(cloudConfig))

	if len(loadZones) == 0 {
		printToStdout(c.globalState, stackHeader+"No load zones found.\n")
		return nil
	}

	printToStdout(c.globalState, stackHeader+formatLoadZoneTable(loadZones))
	return nil
}

func (c *cmdCloudLoadZoneList) outputJSON(loadZones []cloudapiv6.LoadZone) error {
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(loadZones); err != nil {
		return fmt.Errorf("failed to encode load zone list: %w", err)
	}

	printToStdout(c.globalState, buf.String())
	return nil
}

func formatLoadZoneTable(loadZones []cloudapiv6.LoadZone) string {
	var buf strings.Builder
	w := tabwriter.NewWriter(&buf, 0, 0, 3, ' ', 0)
	_, _ = fmt.Fprintln(w, "ID\tNAME\tTYPE\tAVAILABLE")
	for _, z := range loadZones {
		zoneType := "private"
		if z.Public {
			zoneType = "public"
		}
		available := "no"
		if z.Available {
			available = "yes"
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", z.K6LoadZoneID, z.Name, zoneType, available)
	}
	_ = w.Flush()
	return buf.String()
}
