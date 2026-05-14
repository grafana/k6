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

  Use the global --address flag or the K6_ADDRESS environment variable to specify
  the URL to the API server.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			if gs.Flags.Address == "" {
				return errors.New("The REST API server is disabled, but this command needs" + //nolint:staticcheck
					" to read metrics from it. Enable it by setting --address or K6_ADDRESS.")
			}
			c, err := client.New(gs.Flags.Address)
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
