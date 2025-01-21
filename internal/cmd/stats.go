package cmd

import (
	"github.com/spf13/cobra"

	"go.k6.io/k6/api/v1/client"
	"go.k6.io/k6/cmd/state"
)

func getCmdStats(gs *state.GlobalState) *cobra.Command {
	// statsCmd represents the stats command
	statsCmd := &cobra.Command{
		Use:   "stats",
		Short: "Show test metrics",
		Long: `Show test metrics.

  Use the global --address flag to specify the URL to the API server.`,
		RunE: func(_ *cobra.Command, _ []string) error {
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
