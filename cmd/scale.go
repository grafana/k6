package cmd

import (
	"errors"

	"github.com/spf13/cobra"

	v1 "github.com/liuxd6825/k6server/api/v1"
	"github.com/liuxd6825/k6server/api/v1/client"
	"github.com/liuxd6825/k6server/cmd/state"
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
			max := getNullInt64(cmd.Flags(), "max")
			if !vus.Valid && !max.Valid {
				return errors.New("Specify either -u/--vus or -m/--max") //nolint:golint,stylecheck
			}

			c, err := client.New(gs.Flags.Address)
			if err != nil {
				return err
			}
			status, err := c.SetStatus(gs.Ctx, v1.Status{VUs: vus, VUsMax: max})
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
