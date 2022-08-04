package cmd

import (
	"github.com/spf13/cobra"

	"go.k6.io/k6/api/v1/client"
)

func getCmdStatus(globalState *globalState) *cobra.Command {
	// statusCmd represents the status command
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show test status",
		Long: `Show test status.

  Use the global --address flag to specify the URL to the API server.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client.New(globalState.flags.address)
			if err != nil {
				return err
			}
			status, err := c.Status(globalState.ctx)
			if err != nil {
				return err
			}

			return yamlPrint(globalState.stdOut, status)
		},
	}
	return statusCmd
}
