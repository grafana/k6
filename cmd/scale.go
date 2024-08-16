package cmd

import (
	"errors"

	"github.com/spf13/cobra"

	v1 "go.k6.io/k6/api/v1"
	"go.k6.io/k6/api/v1/client"
	"go.k6.io/k6/cmd/state"
)

func getCmdScale(gs *state.GlobalState) *cobra.Command {
	// scaleCmd represents the scale command
	scaleCmd := &cobra.Command{
		Use:   "scale",
		Short: "Scale a running test",
		Long: `Scale a running test.

  Use the global --address flag to specify the URL to the API server.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			vus := getNullInt64(cmd.Flags(), "vus")
			maxVUs := getNullInt64(cmd.Flags(), "max")
			if !vus.Valid && !maxVUs.Valid {
				return errors.New("Specify either -u/--vus or -m/--max") //nolint:golint,stylecheck
			}

			c, err := client.New(gs.Flags.Address)
			if err != nil {
				return err
			}
			status, err := c.SetStatus(gs.Ctx, v1.Status{VUs: vus, VUsMax: maxVUs})
			if err != nil {
				return err
			}

			return yamlPrint(gs.Stdout, status)
		},
	}

	scaleCmd.Flags().Int64P("vus", "u", 1, "number of virtual users")
	scaleCmd.Flags().Int64P("max", "m", 0, "max available virtual users")

	return scaleCmd
}
