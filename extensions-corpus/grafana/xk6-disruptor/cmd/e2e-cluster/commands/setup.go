package commands

import (
	"fmt"

	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/cluster"
	"github.com/spf13/cobra"
)

// BuildSetupCmd returns the setup command
func BuildSetupCmd() *cobra.Command {
	name := cluster.DefaultE2eClusterConfig().Name
	port := cluster.DefaultE2eClusterConfig().IngressPort
	var images []string

	cmd := &cobra.Command{
		Use:   "setup",
		Short: "creates and configures an e2e test cluster ",
		Long:  "creates and configures an e2e test cluster with default options.",
		RunE: func(cmd *cobra.Command, args []string) error { //nolint:revive
			cluster, err := cluster.BuildE2eCluster(
				cluster.DefaultE2eClusterConfig(),
				cluster.WithEnvOverride(false),
				cluster.WithName(name),
				cluster.WithIngressPort(port),
				cluster.WithImages(images...),
			)
			if err != nil {
				return fmt.Errorf("failed to create cluster: %w", err)
			}

			//nolint:forbidigo // allow printf as this is a CLI tool
			fmt.Printf("cluster %q created\n", cluster.Name())
			return nil
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", name, "name of the cluster")
	cmd.Flags().Int32VarP(&port, "port", "p", port, "ingress port")
	cmd.Flags().StringArrayVarP(&images, "image", "i", cluster.DefaultE2eClusterConfig().Images,
		"additional image to pre-load in the cluster. Can be specified multiple times for loading multiple images.")

	return cmd
}
