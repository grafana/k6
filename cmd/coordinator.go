package cmd

import (
	"net"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.k6.io/k6/execution/distributed"
	"google.golang.org/grpc"
)

// cmdCoordinator handles the `k6 coordinator` sub-command
type cmdCoordinator struct {
	gs            *globalState
	gRPCAddress   string
	instanceCount int
}

func (c *cmdCoordinator) run(cmd *cobra.Command, args []string) error {
	test, err := loadLocalTest(c.gs, cmd, args, getPartialConfig)
	if err != nil {
		return err
	}

	// Only consolidated options, not derived
	err = test.initRunner.SetOptions(test.consolidatedConfig.Options)
	if err != nil {
		return err
	}

	// TODO: start a MetricsEngine

	coordinator, err := distributed.NewCoordinatorServer(
		c.instanceCount, test.initRunner.MakeArchive(), c.gs.logger,
	)
	if err != nil {
		return err
	}

	c.gs.logger.Infof("Starting gRPC server on %s", c.gRPCAddress)
	listener, err := net.Listen("tcp", c.gs.flags.address)
	if err != nil {
		return err
	}

	grpcServer := grpc.NewServer() // TODO: add auth and a whole bunch of other options
	distributed.RegisterDistributedTestServer(grpcServer, coordinator)

	return grpcServer.Serve(listener)
}

func (c *cmdCoordinator) flagSet() *pflag.FlagSet {
	flags := pflag.NewFlagSet("", pflag.ContinueOnError)
	flags.SortFlags = false
	flags.AddFlagSet(optionFlagSet())
	flags.AddFlagSet(runtimeOptionFlagSet(false))
	flags.StringVar(&c.gRPCAddress, "grpc-addr", "localhost:6566", "address on which to bind the gRPC server")
	flags.IntVar(&c.instanceCount, "instance-count", 1, "number of distributed instances")
	return flags
}

func getCmdCoordnator(gs *globalState) *cobra.Command {
	c := &cmdCoordinator{
		gs: gs,
	}

	coordinatorCmd := &cobra.Command{
		Use:   "coordinator",
		Short: "Start a distributed load test",
		Long:  `TODO`,
		Args:  cobra.ExactArgs(1),
		RunE:  c.run,
	}

	coordinatorCmd.Flags().SortFlags = false
	coordinatorCmd.Flags().AddFlagSet(c.flagSet())

	return coordinatorCmd
}
