/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"go.k6.io/k6/api"
	"go.k6.io/k6/core/local"
	"go.k6.io/k6/errext"
	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/execution"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/consts"
	"go.k6.io/k6/metrics/engine"
	"go.k6.io/k6/output"
	"go.k6.io/k6/stats"
	"go.k6.io/k6/ui/pb"
)

// cmdRun handles the `k6 run` sub-command
type cmdRun struct {
	gs *globalState
}

// TODO: split apart some more
//nolint:funlen,gocognit,gocyclo,cyclop
func (c *cmdRun) run(cmd *cobra.Command, args []string) (err error) {
	printBanner(c.gs)
	defer func() {
		c.gs.logger.Debugf("Everything has finished, exiting k6 with error '%s'!", err)
	}()

	test, err := loadTest(c.gs, cmd, args, getConfig)
	if err != nil {
		return err
	}

	// Write the full consolidated *and derived* options back to the Runner.
	conf := test.derivedConfig
	if err = test.initRunner.SetOptions(conf.Options); err != nil {
		return err
	}

	globalCtx, globalCancel := context.WithCancel(c.gs.ctx)
	defer globalCancel()

	logger := c.gs.logger
	// Create a local execution scheduler wrapping the runner.
	logger.Debug("Initializing the execution scheduler...")
	execScheduler, err := local.NewExecutionScheduler(test.initRunner, test.builtInMetrics, logger)
	if err != nil {
		return err
	}

	// This is manually triggered after the Engine's Run() has completed,
	// and things like a single Ctrl+C don't affect it. We use it to make
	// sure that the progressbars finish updating with the latest execution
	// state one last time, after the test run has finished.
	progressCtx, progressCancel := context.WithCancel(globalCtx)
	defer progressCancel()
	initBar := execScheduler.GetInitProgressBar()
	progressBarWG := &sync.WaitGroup{}
	progressBarWG.Add(1)
	go func() {
		pbs := []*pb.ProgressBar{execScheduler.GetInitProgressBar()}
		for _, s := range execScheduler.GetExecutors() {
			pbs = append(pbs, s.GetProgress())
		}
		showProgress(progressCtx, c.gs, pbs, logger)
		progressBarWG.Done()
	}()

	// Create all outputs.
	executionPlan := execScheduler.GetExecutionPlan()
	outputs, err := createOutputs(c.gs, test, executionPlan)
	if err != nil {
		return err
	}

	metricsEngine, err := engine.NewMetricsEngine(
		test.metricsRegistry, execScheduler.GetState(),
		test.derivedConfig.Options, test.runtimeOptions, logger,
	)
	if err != nil {
		return err
	}

	if !test.runtimeOptions.NoSummary.Bool || !test.runtimeOptions.NoThresholds.Bool {
		// We'll need to pipe metrics to the MetricsEngine if either the
		// thresholds or the end-of-test summary are enabled.
		outputs = append(outputs, metricsEngine.CreateIngester())
	}

	errIsFromThresholds := false
	if !test.runtimeOptions.NoSummary.Bool {
		defer func() {
			if err != nil && !errIsFromThresholds {
				logger.Debug("The end-of-test summary won't be generated because the test run finished with an error")
				return
			}

			logger.Debug("Generating the end-of-test summary...")
			summaryResult, serr := test.initRunner.HandleSummary(globalCtx, &lib.Summary{
				Metrics:         metricsEngine.ObservedMetrics,
				RootGroup:       execScheduler.GetRunner().GetDefaultGroup(),
				TestRunDuration: execScheduler.GetState().GetCurrentTestRunDuration(),
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
				logger.WithError(serr).Error("Failed to handle the end-of-test summary")
			}
		}()
	}

	// lingerCtx is cancelled by Ctrl+C, and is used to wait for that event when
	// k6 was started with the --linger option.
	lingerCtx, lingerCancel := context.WithCancel(globalCtx)
	defer lingerCancel()

	// runCtx is used for the test run execution and is created with the special
	// execution.NewTestRunContext() function so that it can be aborted even
	// from sub-contexts while also attaching a reason for the abort.
	runCtx, runAbort := execution.NewTestRunContext(lingerCtx, logger)

	// We do this here so we can get any output URLs below.
	initBar.Modify(pb.WithConstProgress(0, "Starting outputs"))
	outputManager := output.NewManager(outputs, logger, func(err error) {
		if err != nil {
			logger.WithError(err).Error("Received error to stop from output")
		}
		// TODO: attach run status and exit code?
		runAbort(err)
	})
	samples := make(chan stats.SampleContainer, test.derivedConfig.MetricSamplesBufferSize.Int64)
	waitOutputsDone, err := outputManager.Start(samples)
	if err != nil {
		return err
	}
	defer func() {
		// We call waitOutputsDone() below, since the threshold calculations
		// need all of the metrics to be sent to the engine before we can run
		// them for the last time. But we need the threshold calculations, since
		// they may change the run status for the outputs here.
		runStatus := lib.RunStatusFinished
		if err != nil {
			runStatus = lib.RunStatusAbortedSystem
			var rserr lib.HasRunStatus
			if errors.As(err, &rserr) {
				runStatus = rserr.RunStatus()
			}
		}
		outputManager.SetRunStatus(runStatus)
		outputManager.StopOutputs()
	}()

	if !test.runtimeOptions.NoThresholds.Bool {
		finalizeThresholds := metricsEngine.StartThresholdCalculations(runAbort)

		defer func() {
			// This gets called after all of the outputs have stopped, so we are
			// sure there won't be any more metrics being sent.
			logger.Debug("Finalizing thresholds...")
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
					logger.WithError(tErr).Debug("Breached thresholds, but test already exited with another error")
				}
			}
		}()
	}

	defer func() {
		logger.Debug("Waiting for metric processing to finish...")
		close(samples)
		waitOutputsDone()
	}()

	// Spin up the REST API server, if not disabled.
	if c.gs.flags.address != "" { //nolint:nestif // TODO: fix
		initBar.Modify(pb.WithConstProgress(0, "Init API server"))
		server := api.NewAPIServer(
			runCtx, c.gs.flags.address, samples, metricsEngine, execScheduler, logger,
		)
		go func() {
			logger.Debugf("Starting the REST API server on '%s'", c.gs.flags.address)
			if aerr := server.ListenAndServe(); aerr != nil && !errors.Is(aerr, http.ErrServerClosed) {
				// Only exit k6 if the user has explicitly set the REST API address
				if cmd.Flags().Lookup("address").Changed {
					logger.WithError(aerr).Error("Error from API server")
					c.gs.osExit(int(exitcodes.CannotStartRESTAPI))
				} else {
					logger.WithError(aerr).Warn("Error from API server")
				}
			}
		}()
		defer func() {
			logger.Debugf("Gracefully shutting down the REST API server on '%s'...", c.gs.flags.address)
			if serr := server.Shutdown(globalCtx); serr != nil {
				logger.WithError(err).Debugf("The REST API server had an error shutting down")
			}
		}()
	}

	printExecutionDescription(
		c.gs, "local", args[0], "", conf, execScheduler.GetState().ExecutionTuple, executionPlan, outputs,
	)

	// Trap Interrupts, SIGINTs and SIGTERMs.
	gracefulStop := func(sig os.Signal) {
		logger.WithField("sig", sig).Debug("Stopping k6 in response to signal...")
		err = errext.WithExitCodeIfNone(fmt.Errorf("signal '%s' received", sig), exitcodes.ExternalAbort)
		err = lib.WithRunStatusIfNone(err, lib.RunStatusAbortedUser)
		runAbort(err)  // first abort the test run this way, to propagate the error
		lingerCancel() // cancel this context as well, since the user did Ctrl+C
	}
	hardStop := func(sig os.Signal) {
		logger.WithField("sig", sig).Error("Aborting k6 in response to signal")
		globalCancel() // not that it matters, given that os.Exit() will be called right after
	}
	stopSignalHandling := handleTestAbortSignals(c.gs, gracefulStop, hardStop)
	defer stopSignalHandling()

	// Initialize VUs and start the test
	err = execScheduler.Run(globalCtx, runCtx, samples)

	if !conf.NoUsageReport.Bool {
		reportDone := make(chan struct{})
		go func() {
			_ = reportUsage(execScheduler)
			close(reportDone)
		}()
		defer func() {
			select {
			case <-reportDone:
			case <-time.After(3 * time.Second):
			}
		}()
	}

	if conf.Linger.Bool {
		defer func() {
			msg := "The test is done, but --linger was enabled, so k6 is waiting for Ctrl+C to continue..."
			select {
			case <-lingerCtx.Done():
				// do nothing, we were interrupted by Ctrl+C already
			default:
				logger.Debug(msg)
				if !c.gs.flags.quiet {
					printToStdout(c.gs, msg)
				}
				<-lingerCtx.Done()
				logger.Debug("Ctrl+C received, exiting...")
			}
		}()
	}

	defer func() {
		logger.Debug("Waiting for progress bars to finish...")
		progressCancel()
		progressBarWG.Wait()
	}()

	// Check what the execScheduler.Run() error is.
	if err != nil {
		err = common.UnwrapGojaInterruptedError(err)
		logger.WithError(err).Debug("Test finished with an error")
		return errext.WithExitCodeIfNone(err, exitcodes.GenericEngine)
	}
	logger.Debug("Test finished cleanly")

	// Warn if no iterations could be completed.
	if execScheduler.GetState().GetFullIterationCount() == 0 {
		logger.Warn("No script iterations fully finished, consider making the test duration longer")
	}

	return nil
}

func (c *cmdRun) flagSet() *pflag.FlagSet {
	flags := pflag.NewFlagSet("", pflag.ContinueOnError)
	flags.SortFlags = false
	flags.AddFlagSet(optionFlagSet())
	flags.AddFlagSet(runtimeOptionFlagSet(true))
	flags.AddFlagSet(configFlagSet())
	return flags
}

func getCmdRun(gs *globalState) *cobra.Command {
	c := &cmdRun{
		gs: gs,
	}

	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Start a load test",
		Long: `Start a load test.

This also exposes a REST API to interact with it. Various k6 subcommands offer
a commandline interface for interacting with it.`,
		Example: `
  # Run a single VU, once.
  k6 run script.js

  # Run a single VU, 10 times.
  k6 run -i 10 script.js

  # Run 5 VUs, splitting 10 iterations between them.
  k6 run -u 5 -i 10 script.js

  # Run 5 VUs for 10s.
  k6 run -u 5 -d 10s script.js

  # Ramp VUs from 0 to 100 over 10s, stay there for 60s, then 10s down to 0.
  k6 run -u 0 -s 10s:100 -s 60s -s 10s:0

  # Send metrics to an influxdb server
  k6 run -o influxdb=http://1.2.3.4:8086/k6`[1:],
		Args: exactArgsWithMsg(1, "arg should either be \"-\", if reading script from stdin, or a path to a script file"),
		RunE: c.run,
	}

	runCmd.Flags().SortFlags = false
	runCmd.Flags().AddFlagSet(c.flagSet())

	return runCmd
}

func reportUsage(execScheduler *local.ExecutionScheduler) error {
	execState := execScheduler.GetState()
	executorConfigs := execScheduler.GetExecutorConfigs()

	executors := make(map[string]int)
	for _, ec := range executorConfigs {
		executors[ec.GetType()]++
	}

	body, err := json.Marshal(map[string]interface{}{
		"k6_version": consts.Version,
		"executors":  executors,
		"vus_max":    execState.GetInitializedVUsCount(),
		"iterations": execState.GetFullIterationCount(),
		"duration":   execState.GetCurrentTestRunDuration().String(),
		"goos":       runtime.GOOS,
		"goarch":     runtime.GOARCH,
	})
	if err != nil {
		return err
	}
	res, err := http.Post("https://reports.k6.io/", "application/json", bytes.NewBuffer(body)) //nolint:noctx
	defer func() {
		if err == nil {
			_ = res.Body.Close()
		}
	}()

	return err
}

func handleSummaryResult(fs afero.Fs, stdOut, stdErr io.Writer, result map[string]io.Reader) error {
	var errs []error

	getWriter := func(path string) (io.Writer, error) {
		switch path {
		case "stdout":
			return stdOut, nil
		case "stderr":
			return stdErr, nil
		default:
			return fs.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o666)
		}
	}

	for path, value := range result {
		if writer, err := getWriter(path); err != nil {
			errs = append(errs, fmt.Errorf("could not open '%s': %w", path, err))
		} else if n, err := io.Copy(writer, value); err != nil {
			errs = append(errs, fmt.Errorf("error saving summary to '%s' after %d bytes: %w", path, n, err))
		}
	}

	return consolidateErrorMessage(errs, "Could not save some summary information:")
}
