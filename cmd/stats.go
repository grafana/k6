package cmd

import (
	"github.com/spf13/cobra"

	"go.k6.io/k6/api/v1/client"
)

func getCmdStats(globalState *globalState) *cobra.Command {
	// statsCmd represents the stats command
	statsCmd := &cobra.Command{
		Use:   "stats",
		Short: "Show test metrics",
		Long: `Show test metrics.

  Use the global --address flag to specify the URL to the API server.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client.New(globalState.flags.address)
			if err != nil {
				return err
			}
			metrics, err := c.Metrics(globalState.ctx)
			if err != nil {
				return err
			}

			return yamlPrint(globalState.stdOut, metrics)
		},
	}
	return statsCmd
}
