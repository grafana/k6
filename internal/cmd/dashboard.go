package cmd

import (
	"github.com/grafana/xk6-dashboard/dashboard"
	"github.com/spf13/cobra"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/subcommand"
)

func getDashboardCmd(gs *state.GlobalState) *cobra.Command {
	c := dashboard.NewCommand(gs)

	c.Use = "dashboard"

	return c
}

// Register the "dashboard" subcommand extension.
// Just to have at least one registered subcommand.
func init() {
	subcommand.RegisterExtension("dashboard", getDashboardCmd)
}
