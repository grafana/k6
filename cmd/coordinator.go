package cmd

import (
	"fmt"
	"net"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.k6.io/k6/errext"
	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/execution"
	"go.k6.io/k6/execution/distributed"
	"go.k6.io/k6/lib"
	"google.golang.org/grpc"
)

// cmdCoordinator handles the `k6 coordinator` sub-command
type cmdCoordinator struct {
	gs            *globalState
	gRPCAddress   string
	instanceCount int
}

//nolint:funlen // TODO: split apart
func (c *cmdCoordinator) run(cmd *cobra.Command, args []string) (err error) {
	ctx, abort := execution.NewTestRunContext(c.gs.ctx, c.gs.logger)

	test, err := loadLocalTest(c.gs, cmd, args, getPartialConfig)
	if err != nil {
		return err
	}

	// Only consolidated options, not derived
	err = test.initRunner.SetOptions(test.consolidatedConfig.Options)
	if err != nil {
		return err
	}

	shouldProcessMetrics := !test.runtimeOptions.NoSummary.Bool || !test.runtimeOptions.NoThresholds.Bool
	metricsEngine, err := test.newMetricsEngine(shouldProcessMetrics, c.gs.logger)
	if err != nil {
		return err
	}

	coordinator, err := distributed.NewCoordinatorServer(
		c.instanceCount, test.initRunner.MakeArchive(), metricsEngine, c.gs.logger,
	)
	if err != nil {
		return err
	}

	errIsFromThresholds := false
	if !test.runtimeOptions.NoSummary.Bool {
		defer func() {
			if err != nil && !errIsFromThresholds {
				c.gs.logger.Debug("The end-of-test summary won't be generated because the test run finished with an error")
				return
			}

			c.gs.logger.Debug("Generating the end-of-test summary...")
			summaryResult, serr := test.initRunner.HandleSummary(ctx, &lib.Summary{
				Metrics:         metricsEngine.ObservedMetrics,
				RootGroup:       test.initRunner.GetDefaultGroup(),
				TestRunDuration: coordinator.GetCurrentTestRunDuration(),
				NoColor:         c.gs.flags.noColor,
				UIState: lib.UIState{
					IsStdOutTTY: c.gs.stdOut.isTTY,
					IsStdErrTTY: c.gs.stdErr.isTTY,
				},
			})
			if serr == nil {
				serr = handleSummaryResult(c.gs.fs, c.gs.stdOut, c.gs.stdErr, summaryResult)
			}
			if serr != nil {
				c.gs.logger.WithError(serr).Error("Failed to handle the end-of-test summary")
			}
		}()
	}

	if !test.runtimeOptions.NoThresholds.Bool {
		getCurrentTestDuration := coordinator.GetCurrentTestRunDuration
		finalizeThresholds := metricsEngine.StartThresholdCalculations(abort, getCurrentTestDuration)

		defer func() {
			// This gets called after all of the outputs have stopped, so we are
			// sure there won't be any more metrics being sent.
			c.gs.logger.Debug("Finalizing thresholds...")
			breachedThresholds := finalizeThresholds()
			if len(breachedThresholds) > 0 {
				tErr := errext.WithExitCodeIfNone(
					fmt.Errorf("thresholds on metrics %s have been breached", strings.Join(breachedThresholds, ", ")),
					exitcodes.ThresholdsHaveFailed,
				)
				tErr = lib.WithRunStatusIfNone(tErr, lib.RunStatusAbortedThreshold)
				if err == nil {
					errIsFromThresholds = true
					err = tErr
				} else {
					c.gs.logger.WithError(tErr).Debug("Breached thresholds, but test already exited with another error")
				}
			}
		}()
	}

	c.gs.logger.Infof("Starting gRPC server on %s", c.gRPCAddress)
	listener, err := net.Listen("tcp", c.gRPCAddress)
	if err != nil {
		return err
	}

	grpcServer := grpc.NewServer() // TODO: add auth and a whole bunch of other options
	distributed.RegisterDistributedTestServer(grpcServer, coordinator)

	go func() {
		err := grpcServer.Serve(listener)
		c.gs.logger.Debugf("gRPC server end: %s", err)
	}()
	coordinator.Wait()
	c.gs.logger.Infof("All done!")
	return nil
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
