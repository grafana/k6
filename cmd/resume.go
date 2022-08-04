package cmd

import (
	"github.com/spf13/cobra"
	"gopkg.in/guregu/null.v3"

	v1 "go.k6.io/k6/api/v1"
	"go.k6.io/k6/api/v1/client"
)

func getCmdResume(globalState *globalState) *cobra.Command {
	// resumeCmd represents the resume command
	resumeCmd := &cobra.Command{
		Use:   "resume",
		Short: "Resume a paused test",
		Long: `Resume a paused test.

  Use the global --address flag to specify the URL to the API server.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client.New(globalState.flags.address)
			if err != nil {
				return err
			}
			status, err := c.SetStatus(globalState.ctx, v1.Status{
				Paused: null.BoolFrom(false),
			})
			if err != nil {
				return err
			}

			return yamlPrint(globalState.stdOut, status)
		},
	}
	return resumeCmd
}
