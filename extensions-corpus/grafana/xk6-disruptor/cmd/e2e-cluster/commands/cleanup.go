package commands

import (
	"fmt"

	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/cluster"
	"github.com/spf13/cobra"
)

// BuildCleanupCmd returns the cleanup command
func BuildCleanupCmd() *cobra.Command {
	var name string
	var quiet bool

	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "deletes an e2e test cluster ",
		Long:  "deletes an e2e test cluster",
		RunE: func(cmd *cobra.Command, args []string) error { //nolint:revive
			if name == "" {
				return fmt.Errorf("--name is required")
			}
			return cluster.DeleteE2eCluster(name, quiet)
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "name of the cluster to delete. Required")
	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "do not report error if cluster does not exist")
	return cmd
}
