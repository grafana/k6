package cmd

import (
	"github.com/spf13/cobra"

	"go.k6.io/k6/cmd/state"
)

// getCmdLogin returns the `k6 login` sub-command, together with its children.
func getCmdLogin(gs *state.GlobalState) *cobra.Command {
	loginCmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with a service",
		Long: `Authenticate with a service.

Logging into a service changes the default when just "-o [type]" is passed with
no parameters, you can always override the stored credentials by passing some
on the commandline.`,
		Deprecated: `and will be removed in a future release. Please use the "k6 cloud login" command instead.
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Usage()
		},
	}
	loginCmd.AddCommand(
		getCmdLoginCloud(gs),
		getCmdLoginInfluxDB(gs),
	)

	return loginCmd
}
