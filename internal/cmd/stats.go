package cmd

import (
	"errors"

	"github.com/spf13/cobra"

	"go.k6.io/k6/v2/api/v1/client"
	"go.k6.io/k6/v2/cmd/state"
)

func getCmdStats(gs *state.GlobalState) *cobra.Command {
	// statsCmd represents the stats command
	statsCmd := &cobra.Command{
		Use:    "stats",
		Short:  "Show test metrics",
		Hidden: true,
		Long: `Show test metrics.

  Use the global --http-api-addr flag or the K6_HTTP_API_ADDR environment variable to specify
  the URL to the API server.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			if gs.Flags.HTTPAPIAddr == "" {
				return errors.New("The HTTP API server is disabled, but this command needs" + //nolint:staticcheck
					" to read metrics from it. Enable it by setting --http-api-addr or K6_HTTP_API_ADDR.")
			}
			c, err := client.New(gs.Flags.HTTPAPIAddr)
			if err != nil {
				return err
			}
			metrics, err := c.Metrics(gs.Ctx)
			if err != nil {
				return err
			}

			return yamlPrint(gs.Stdout, metrics)
		},
	}
	return statsCmd
}
