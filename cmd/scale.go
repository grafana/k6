package cmd

import (
	"errors"

	"github.com/spf13/cobra"

	v1 "go.k6.io/k6/api/v1"
	"go.k6.io/k6/api/v1/client"
)

func getCmdScale(globalState *globalState) *cobra.Command {
	// scaleCmd represents the scale command
	scaleCmd := &cobra.Command{
		Use:   "scale",
		Short: "Scale a running test",
		Long: `Scale a running test.

  Use the global --address flag to specify the URL to the API server.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			vus := getNullInt64(cmd.Flags(), "vus")
			max := getNullInt64(cmd.Flags(), "max")
			if !vus.Valid && !max.Valid {
				return errors.New("Specify either -u/--vus or -m/--max") //nolint:golint,stylecheck
			}

			c, err := client.New(globalState.flags.address)
			if err != nil {
				return err
			}
			status, err := c.SetStatus(globalState.ctx, v1.Status{VUs: vus, VUsMax: max})
			if err != nil {
				return err
			}

			return yamlPrint(globalState.stdOut, status)
		},
	}

	scaleCmd.Flags().Int64P("vus", "u", 1, "number of virtual users")
	scaleCmd.Flags().Int64P("max", "m", 0, "max available virtual users")

	return scaleCmd
}
