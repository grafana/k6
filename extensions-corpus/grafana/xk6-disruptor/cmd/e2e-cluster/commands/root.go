package commands

import (
	"github.com/spf13/cobra"
)

// BuildRootCmd returns the root command for the e2e-env cli tool
func BuildRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "e2e-cluster",
		Short:         "maintain e2e test clusters",
		Long:          "A command for the setup and cleanup of e2e test clusters.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	return rootCmd
}
