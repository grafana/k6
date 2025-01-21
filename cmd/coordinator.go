package cmd

import (
	"net"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/execution/distributed"
	"google.golang.org/grpc"
)

// cmdCoordinator handles the `k6 coordinator` sub-command
type cmdCoordinator struct {
	gs            *state.GlobalState
	gRPCAddress   string
	instanceCount int
}

func (c *cmdCoordinator) run(cmd *cobra.Command, args []string) (err error) {
	test, err := loadAndConfigureLocalTest(c.gs, cmd, args, getPartialConfig)
	if err != nil {
		return err
	}

	coordinator, err := distributed.NewCoordinatorServer(
		c.instanceCount, test.initRunner.MakeArchive(), c.gs.Logger,
	)
	if err != nil {
		return err
	}

	c.gs.Logger.Infof("Starting gRPC server on %s", c.gRPCAddress)
	listener, err := net.Listen("tcp", c.gRPCAddress)
	if err != nil {
		return err
	}

	grpcServer := grpc.NewServer() // TODO: add auth and a whole bunch of other options
	distributed.RegisterDistributedTestServer(grpcServer, coordinator)

	go func() {
		err := grpcServer.Serve(listener)
		c.gs.Logger.Debugf("gRPC server end: %s", err)
	}()
	coordinator.Wait()
	c.gs.Logger.Infof("All done!")
	return nil
}

func (c *cmdCoordinator) flagSet() *pflag.FlagSet {
	flags := pflag.NewFlagSet("", pflag.ContinueOnError)
	flags.SortFlags = false
	flags.AddFlagSet(optionFlagSet())
	flags.AddFlagSet(runtimeOptionFlagSet(false))

	// TODO: add support bi-directional gRPC authentication and authorization
	flags.StringVar(&c.gRPCAddress, "grpc-addr", "localhost:6566", "address on which to bind the gRPC server")

	// TODO: add some better way to specify the test, e.g. an execution segment
	// sequence + some sort of a way to map instances with specific segments
	// (e.g. key-value tags that can be matched to every execution segment, with
	// each instance advertising its own tags when it connects).
	flags.IntVar(&c.instanceCount, "instance-count", 1, "number of distributed instances")
	return flags
}

func getCmdCoordnator(gs *state.GlobalState) *cobra.Command {
	c := &cmdCoordinator{
		gs: gs,
	}

	coordinatorCmd := &cobra.Command{
		Use:    "coordinator",
		Short:  "Start a distributed load test",
		Long:   `TODO`,
		RunE:   c.run,
		Hidden: true, // TODO: remove when officially released
	}

	coordinatorCmd.Flags().SortFlags = false
	coordinatorCmd.Flags().AddFlagSet(c.flagSet())

	return coordinatorCmd
}
