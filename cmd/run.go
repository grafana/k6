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
	"sync"
	"time"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"go.k6.io/k6/api"
	"go.k6.io/k6/core"
	"go.k6.io/k6/core/local"
	"go.k6.io/k6/errext"
	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/consts"
	"go.k6.io/k6/ui/pb"
)

// cmdRun handles the `k6 run` sub-command
type cmdRun struct {
	gs *globalState
}

// TODO: split apart some more
//
//nolint:funlen,gocognit,gocyclo,cyclop
func (c *cmdRun) run(cmd *cobra.Command, args []string) (err error) {
	printBanner(c.gs)

	test, err := loadAndConfigureTest(c.gs, cmd, args, getConfig)
	if err != nil {
		return err
	}

	// Write the full consolidated *and derived* options back to the Runner.
	conf := test.derivedConfig
	testRunState, err := test.buildTestRunState(conf.Options)
	if err != nil {
		return err
	}

	// We prepare a bunch of contexts:
	//  - The runCtx is cancelled as soon as the Engine's run() lambda finishes,
	//    and can trigger things like the usage report and end of test summary.
	//    Crucially, metrics processing by the Engine will still work after this
	//    context is cancelled!
	//  - The lingerCtx is cancelled by Ctrl+C, and is used to wait for that
	//    event when k6 was ran with the --linger option.
	//  - The globalCtx is cancelled only after we're completely done with the
	//    test execution and any --linger has been cleared, so that the Engine
	//    can start winding down its metrics processing.
	globalCtx, globalCancel := context.WithCancel(c.gs.ctx)
	defer globalCancel()
	lingerCtx, lingerCancel := context.WithCancel(globalCtx)
	defer lingerCancel()
	runCtx, runCancel := context.WithCancel(lingerCtx)
	defer runCancel()

	logger := testRunState.Logger
	// Create a local execution scheduler wrapping the runner.
	logger.Debug("Initializing the execution scheduler...")
	execScheduler, err := local.NewExecutionScheduler(testRunState)
	if err != nil {
		return err
	}

	progressBarWG := &sync.WaitGroup{}
	progressBarWG.Add(1)
	defer progressBarWG.Wait()

	// This is manually triggered after the Engine's Run() has completed,
	// and things like a single Ctrl+C don't affect it. We use it to make
	// sure that the progressbars finish updating with the latest execution
	// state one last time, after the test run has finished.
	progressCtx, progressCancel := context.WithCancel(globalCtx)
	defer progressCancel()
	initBar := execScheduler.GetInitProgressBar()
	go func() {
		defer progressBarWG.Done()
		pbs := []*pb.ProgressBar{initBar}
		for _, s := range execScheduler.GetExecutors() {
			pbs = append(pbs, s.GetProgress())
		}
		showProgress(progressCtx, c.gs, pbs, logger)
	}()

	// Create all outputs.
	executionPlan := execScheduler.GetExecutionPlan()
	outputs, err := createOutputs(c.gs, test, executionPlan)
	if err != nil {
		return err
	}

	// TODO: create a MetricsEngine here and add its ingester to the list of
	// outputs (unless both NoThresholds and NoSummary were enabled)

	// TODO: remove this completely
	// Create the engine.
	initBar.Modify(pb.WithConstProgress(0, "Init engine"))
	engine, err := core.NewEngine(testRunState, execScheduler, outputs)
	if err != nil {
		return err
	}

	// Spin up the REST API server, if not disabled.
	if c.gs.flags.address != "" {
		initBar.Modify(pb.WithConstProgress(0, "Init API server"))

		apiWG := &sync.WaitGroup{}
		apiWG.Add(2)
		defer apiWG.Wait()

		srvCtx, srvCancel := context.WithCancel(globalCtx)
		defer srvCancel()

		// TODO: send the ExecutionState and MetricsEngine instead of the Engine
		srv := api.GetServer(c.gs.flags.address, engine, logger)
		go func() {
			defer apiWG.Done()
			logger.Debugf("Starting the REST API server on %s", c.gs.flags.address)
			if aerr := srv.ListenAndServe(); aerr != nil && !errors.Is(aerr, http.ErrServerClosed) {
				// Only exit k6 if the user has explicitly set the REST API address
				if cmd.Flags().Lookup("address").Changed {
					logger.WithError(aerr).Error("Error from API server")
					c.gs.osExit(int(exitcodes.CannotStartRESTAPI))
				} else {
					logger.WithError(aerr).Warn("Error from API server")
				}
			}
		}()
		go func() {
			defer apiWG.Done()
			<-srvCtx.Done()
			if aerr := srv.Close(); aerr != nil {
				logger.WithError(aerr).Debug("REST API server did not shut down correctly")
			}
		}()
	}

	// We do this here so we can get any output URLs below.
	initBar.Modify(pb.WithConstProgress(0, "Starting outputs"))
	// TODO: directly create the MutputManager here, not in the Engine
	err = engine.OutputManager.StartOutputs()
	if err != nil {
		return err
	}
	defer func() {
		engine.OutputManager.StopOutputs(err)
	}()

	printExecutionDescription(
		c.gs, "local", args[0], "", conf, execScheduler.GetState().ExecutionTuple, executionPlan, outputs,
	)

	// Trap Interrupts, SIGINTs and SIGTERMs.
	gracefulStop := func(sig os.Signal) {
		logger.WithField("sig", sig).Debug("Stopping k6 in response to signal...")
		lingerCancel() // stop the test run, metric processing is cancelled below
	}
	onHardStop := func(sig os.Signal) {
		logger.WithField("sig", sig).Error("Aborting k6 in response to signal")
		globalCancel() // not that it matters, given the following command...
	}
	stopSignalHandling := handleTestAbortSignals(c.gs, gracefulStop, onHardStop)
	defer stopSignalHandling()

	// Initialize the engine
	initBar.Modify(pb.WithConstProgress(0, "Init VUs..."))
	engineRun, engineWait, err := engine.Init(globalCtx, runCtx)
	if err != nil {
		err = common.UnwrapGojaInterruptedError(err)
		// Add a generic engine exit code if we don't have a more specific one
		return errext.WithExitCodeIfNone(err, exitcodes.GenericEngine)
	}

	// Init has passed successfully, so unless disabled, make sure we send a
	// usage report after the context is done.
	if !conf.NoUsageReport.Bool {
		reportDone := make(chan struct{})
		go func() {
			<-runCtx.Done()
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

	// Start the test run
	initBar.Modify(pb.WithConstProgress(0, "Starting test..."))
	err = engineRun()
	if err != nil {
		err = errext.WithExitCodeIfNone(common.UnwrapGojaInterruptedError(err), exitcodes.GenericEngine)
		logger.WithError(err).Debug("Engine terminated with an error")
	} else {
		logger.Debug("Engine run terminated cleanly")
	}
	runCancel()

	progressCancel()
	progressBarWG.Wait()

	executionState := execScheduler.GetState()
	// Warn if no iterations could be completed.
	if err == nil && executionState.GetFullIterationCount() == 0 {
		logger.Warn("No script iterations finished, consider making the test duration longer")
	}

	// Handle the end-of-test summary.
	if !testRunState.RuntimeOptions.NoSummary.Bool {
		engine.MetricsEngine.MetricsLock.Lock() // TODO: refactor so this is not needed
		summaryResult, hsErr := test.initRunner.HandleSummary(globalCtx, &lib.Summary{
			Metrics:         engine.MetricsEngine.ObservedMetrics,
			RootGroup:       execScheduler.GetRunner().GetDefaultGroup(),
			TestRunDuration: executionState.GetCurrentTestRunDuration(),
			NoColor:         c.gs.flags.noColor,
			UIState: lib.UIState{
				IsStdOutTTY: c.gs.stdOut.isTTY,
				IsStdErrTTY: c.gs.stdErr.isTTY,
			},
		})
		engine.MetricsEngine.MetricsLock.Unlock()
		if hsErr == nil {
			hsErr = handleSummaryResult(c.gs.fs, c.gs.stdOut, c.gs.stdErr, summaryResult)
		}
		if hsErr != nil {
			logger.WithError(hsErr).Error("failed to handle the end-of-test summary")
		}
	}

	if conf.Linger.Bool {
		select {
		case <-lingerCtx.Done():
			// do nothing, we were interrupted by Ctrl+C already
		default:
			logger.Debug("Linger set; waiting for Ctrl+C...")
			if !c.gs.flags.quiet {
				printToStdout(c.gs, "Linger set; waiting for Ctrl+C...")
			}
			<-lingerCtx.Done()
			logger.Debug("Ctrl+C received, exiting...")
		}
	}
	globalCancel() // signal the Engine that it should wind down
	logger.Debug("Waiting for engine processes to finish...")
	engineWait()
	logger.Debug("Everything has finished, exiting k6!")
	if test.keyLogger != nil {
		if klErr := test.keyLogger.Close(); klErr != nil {
			logger.WithError(klErr).Warn("Error while closing the SSLKEYLOGFILE")
		}
	}

	if engine.IsTainted() {
		if err == nil {
			err = errors.New("some thresholds have failed")
		} else {
			logger.Error("some thresholds have failed") // log this, even if there was already a previous error
		}
		err = errext.WithAbortReasonIfNone(
			errext.WithExitCodeIfNone(err, exitcodes.ThresholdsHaveFailed), errext.AbortedByThresholdsAfterTestEnd,
		)
	}
	return err
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
