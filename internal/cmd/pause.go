package cmd

import (
	"github.com/spf13/cobra"
	"gopkg.in/guregu/null.v3"

	v1 "go.k6.io/k6/api/v1"
	"go.k6.io/k6/api/v1/client"
	"go.k6.io/k6/cmd/state"
)

func getCmdPause(gs *state.GlobalState) *cobra.Command {
	// pauseCmd represents the pause command
	pauseCmd := &cobra.Command{
		Use:   "pause",
		Short: "Pause a running test",
		Long: `Pause a running test.

  Use the global --address flag to specify the URL to the API server.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			c, err := client.New(gs.Flags.Address)
			if err != nil {
				return err
			}
			status, err := c.SetStatus(gs.Ctx, v1.Status{
				Paused: null.BoolFrom(true),
			})
			if err != nil {
				return err
			}
			return yamlPrint(gs.Stdout, status)
		},
	}
	return pauseCmd
}
