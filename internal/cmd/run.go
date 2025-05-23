package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/errext"
	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/internal/api"
	"go.k6.io/k6/internal/event"
	"go.k6.io/k6/internal/execution"
	"go.k6.io/k6/internal/execution/local"
	"go.k6.io/k6/internal/lib/summary"
	"go.k6.io/k6/internal/lib/trace"
	"go.k6.io/k6/internal/metrics/engine"
	summaryoutput "go.k6.io/k6/internal/output/summary"
	"go.k6.io/k6/internal/ui/pb"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/fsext"
	"go.k6.io/k6/metrics"
	"go.k6.io/k6/output"
)

// cmdRun handles the `k6 run` sub-command
type cmdRun struct {
	gs *state.GlobalState

	// TODO: figure out something more elegant?
	loadConfiguredTest func(cmd *cobra.Command, args []string) (*loadedAndConfiguredTest, execution.Controller, error)
}

const (
	// We use an excessively high timeout to wait for event processing to complete,
	// since prematurely proceeding before it is done could create bigger problems.
	// In practice, this effectively acts as no timeout, and the user will have to
	// kill k6 if a hang happens, which is the behavior without events anyway.
	waitEventDoneTimeout = 30 * time.Minute

	// This timeout should be long enough to flush all remaining traces, but still
	// provides a safeguard to not block indefinitely.
	waitForTracerProviderStopTimeout = 3 * time.Minute
)

// TODO: split apart some more
//
//nolint:funlen,gocognit,gocyclo,cyclop
func (c *cmdRun) run(cmd *cobra.Command, args []string) (err error) {
	var logger logrus.FieldLogger = c.gs.Logger
	defer func() {
		if err == nil {
			logger.Debug("Everything has finished, exiting k6 normally!")
		} else {
			logger.WithError(err).Debug("Everything has finished, exiting k6 with an error!")
		}
	}()
	printBanner(c.gs)

	globalCtx, globalCancel := context.WithCancel(c.gs.Ctx)
	defer globalCancel()

	// lingerCtx is cancelled by Ctrl+C, and is used to wait for that event when
	// k6 was started with the --linger option.
	lingerCtx, lingerCancel := context.WithCancel(globalCtx)
	defer lingerCancel()

	// runCtx is used for the test run execution and is created with the special
	// execution.NewTestRunContext() function so that it can be aborted even
	// from sub-contexts while also attaching a reason for the abort.
	runCtx, runAbort := execution.NewTestRunContext(lingerCtx, logger)

	emitEvent := func(evt *event.Event) func() {
		waitDone := c.gs.Events.Emit(evt)
		return func() {
			waitCtx, waitCancel := context.WithTimeout(globalCtx, waitEventDoneTimeout)
			defer waitCancel()
			if werr := waitDone(waitCtx); werr != nil {
				logger.WithError(werr).Warn()
			}
		}
	}

	defer func() {
		waitExitDone := emitEvent(&event.Event{
			Type: event.Exit,
			Data: &event.ExitData{Error: err},
		})
		waitExitDone()
		c.gs.Events.UnsubscribeAll()
	}()

	test, controller, err := c.loadConfiguredTest(cmd, args)
	if err != nil {
		return err
	}
	if test.keyLogger != nil {
		defer func() {
			if klErr := test.keyLogger.Close(); klErr != nil {
				logger.WithError(klErr).Warn("Error while closing the SSLKEYLOGFILE")
			}
		}()
	}

	if err = c.setupTracerProvider(globalCtx, test); err != nil {
		return err
	}
	waitTracesFlushed := func() {
		ctx, cancel := context.WithTimeout(globalCtx, waitForTracerProviderStopTimeout)
		defer cancel()
		if tpErr := test.preInitState.TracerProvider.Shutdown(ctx); tpErr != nil {
			logger.Errorf("The tracer provider didn't stop gracefully: %v", tpErr)
		}
	}

	// Write the full consolidated *and derived* options back to the Runner.
	conf := test.derivedConfig
	testRunState, err := test.buildTestRunState(conf.Options)
	if err != nil {
		return err
	}

	// Create a local execution scheduler wrapping the runner.
	logger.Debug("Initializing the execution scheduler...")
	execScheduler, err := execution.NewScheduler(testRunState, controller)
	if err != nil {
		return err
	}

	backgroundProcesses := &sync.WaitGroup{}
	defer backgroundProcesses.Wait()

	// This is manually triggered after the Engine's Run() has completed,
	// and things like a single Ctrl+C don't affect it. We use it to make
	// sure that the progressbars finish updating with the latest execution
	// state one last time, after the test run has finished.
	progressCtx, progressCancel := context.WithCancel(globalCtx)
	defer progressCancel()

	initBar := execScheduler.GetInitProgressBar()
	backgroundProcesses.Add(1)
	go func() {
		defer backgroundProcesses.Done()
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

	outputs = append(outputs, testRunState.GroupSummary)

	metricsEngine, err := engine.NewMetricsEngine(testRunState.Registry, logger)
	if err != nil {
		return err
	}

	// We'll need to pipe metrics to the MetricsEngine and process them if any
	// of these are enabled: thresholds, end-of-test summary
	shouldProcessMetrics := !testRunState.RuntimeOptions.NoSummary.Bool ||
		!testRunState.RuntimeOptions.NoThresholds.Bool
	var metricsIngester *engine.OutputIngester
	if shouldProcessMetrics {
		err = metricsEngine.InitSubMetricsAndThresholds(conf.Options, testRunState.RuntimeOptions.NoThresholds.Bool)
		if err != nil {
			return err
		}
		// We'll need to pipe metrics to the MetricsEngine if either the
		// thresholds or the end-of-test summary are enabled.
		metricsIngester = metricsEngine.CreateIngester()
		outputs = append(outputs, metricsIngester)
	}

	executionState := execScheduler.GetState()
	if !testRunState.RuntimeOptions.NoSummary.Bool { //nolint:nestif
		// Despite having the revamped [summary.Summary], we still keep the use of the
		// [lib.LegacySummary] for multiple backwards compatibility options,
		// to be deprecated by v1.0 and likely removed or replaced by v2.0:
		// - the `legacy` summary mode (which keeps the old summary format/display).
		// - the data structure for custom `handleSummary()` implementations.
		// - the data structure for the JSON (--summary-export) output.
		legacySummary := func() *lib.LegacySummary {
			return &lib.LegacySummary{
				Metrics:         metricsEngine.ObservedMetrics,
				RootGroup:       testRunState.GroupSummary.Group(),
				TestRunDuration: executionState.GetCurrentTestRunDuration(),
				NoColor:         c.gs.Flags.NoColor,
				UIState: lib.UIState{
					IsStdOutTTY: c.gs.Stdout.IsTTY,
					IsStdErrTTY: c.gs.Stderr.IsTTY,
				},
			}
		}

		sm, err := summary.ValidateMode(testRunState.RuntimeOptions.SummaryMode.String)
		if err != nil {
			logger.WithError(err).Warnf(
				"invalid summary mode %q, falling back to \"compact\" (default)",
				testRunState.RuntimeOptions.SummaryMode.String,
			)
		}

		switch sm {
		// TODO: Remove this code block once we stop supporting the legacy summary, and just leave the default.
		case summary.ModeLegacy:
			// At the end of the test run
			defer func() {
				logger.Debug("Generating the end-of-test summary...")

				summaryResult, hsErr := test.initRunner.HandleSummary(globalCtx, legacySummary(), nil)
				if hsErr == nil {
					hsErr = handleSummaryResult(c.gs.FS, c.gs.Stdout, c.gs.Stderr, summaryResult)
				}
				if hsErr != nil {
					logger.WithError(hsErr).Error("failed to handle the end-of-test summary")
				}
			}()
		default:
			// Instantiates the summary output
			summaryOutput, err := summaryoutput.New(output.Params{
				RuntimeOptions: testRunState.RuntimeOptions,
				Logger:         c.gs.Logger,
			})
			if err != nil {
				logger.WithError(err).Error("failed to initialize the end-of-test summary output")
			}
			outputs = append(outputs, summaryOutput)

			// At the end of the test run
			defer func() {
				logger.Debug("Generating the end-of-test summary...")

				summary := summaryOutput.Summary(
					executionState.GetCurrentTestRunDuration(),
					metricsEngine.ObservedMetrics,
					test.initRunner.GetOptions(),
				)

				// TODO: We should probably try to move these out of the summary,
				// likely as an additional argument like options.
				summary.NoColor = c.gs.Flags.NoColor
				summary.EnableColors = !summary.NoColor && c.gs.Stdout.IsTTY

				summaryResult, hsErr := test.initRunner.HandleSummary(globalCtx, legacySummary(), summary)
				if hsErr == nil {
					hsErr = handleSummaryResult(c.gs.FS, c.gs.Stdout, c.gs.Stderr, summaryResult)
				}
				if hsErr != nil {
					logger.WithError(hsErr).Error("failed to handle the end-of-test summary")
				}
			}()
		}
	}

	waitInitDone := emitEvent(&event.Event{Type: event.Init})

	// Create and start the outputs. We do it quite early to get any output URLs
	// or other details below. It also allows us to ensure when they have
	// flushed their samples and when they have stopped in the defer statements.
	initBar.Modify(pb.WithConstProgress(0, "Starting outputs"))
	outputManager := output.NewManager(outputs, logger, func(err error) {
		if err != nil {
			logger.WithError(err).Error("Received error to stop from output")
		}
		// TODO: attach run status and exit code?
		runAbort(err)
	})
	samples := make(chan metrics.SampleContainer, test.derivedConfig.MetricSamplesBufferSize.Int64)
	// Spin up the REST API server, if not disabled.
	if c.gs.Flags.Address != "" { //nolint:nestif
		initBar.Modify(pb.WithConstProgress(0, "Init API server"))

		// We cannot use backgroundProcesses here, since we need the REST API to
		// be down before we can close the samples channel above and finish the
		// processing the metrics pipeline.
		apiWG := &sync.WaitGroup{}
		apiWG.Add(2)
		defer apiWG.Wait()

		srvCtx, srvCancel := context.WithCancel(globalCtx)
		defer srvCancel()

		srv := api.GetServer(
			runCtx,
			c.gs.Flags.Address, c.gs.Flags.ProfilingEnabled,
			testRunState,
			samples,
			metricsEngine,
			execScheduler,
		)
		go func() {
			defer apiWG.Done()
			logger.Debugf("Starting the REST API server on %s", c.gs.Flags.Address)
			if c.gs.Flags.ProfilingEnabled {
				logger.Debugf("Profiling exposed on http://%s/debug/pprof/", c.gs.Flags.Address)
			}
			if aerr := srv.ListenAndServe(); aerr != nil && !errors.Is(aerr, http.ErrServerClosed) {
				// Only exit k6 if the user has explicitly set the REST API address
				if cmd.Flags().Lookup("address").Changed {
					logger.WithError(aerr).Error("Error from API server")
					c.gs.OSExit(int(exitcodes.CannotStartRESTAPI))
				} else {
					logger.WithError(aerr).Warn("Error from API server")
				}
			}
		}()
		go func() {
			defer apiWG.Done()
			<-srvCtx.Done()
			shutdCtx, shutdCancel := context.WithTimeout(globalCtx, 1*time.Second)
			defer shutdCancel()
			if aerr := srv.Shutdown(shutdCtx); aerr != nil {
				logger.WithError(aerr).Debug("REST API server did not shut down correctly")
			}
		}()
	}

	waitOutputsFlushed, stopOutputs, err := outputManager.Start(samples)
	if err != nil {
		return err
	}
	defer func() {
		logger.Debug("Stopping outputs...")
		// We call waitOutputsFlushed() below because the threshold calculations
		// need all of the metrics to be sent to the MetricsEngine before we can
		// calculate them one last time. We need the threshold calculated here,
		// since they may change the run status for the outputs.
		stopOutputs(err)
	}()

	if !testRunState.RuntimeOptions.NoThresholds.Bool {
		finalizeThresholds := metricsEngine.StartThresholdCalculations(
			metricsIngester, runAbort, executionState.GetCurrentTestRunDuration,
		)
		handleFinalThresholdCalculation := func() {
			// This gets called after the Samples channel has been closed and
			// the OutputManager has flushed all of the cached samples to
			// outputs (including MetricsEngine's ingester). So we are sure
			// there won't be any more metrics being sent.
			logger.Debug("Finalizing thresholds...")
			breachedThresholds := finalizeThresholds()
			if len(breachedThresholds) == 0 {
				return
			}
			tErr := errext.WithAbortReasonIfNone(
				errext.WithExitCodeIfNone(
					fmt.Errorf("thresholds on metrics '%s' have been crossed", strings.Join(breachedThresholds, ", ")),
					exitcodes.ThresholdsHaveFailed,
				), errext.AbortedByThresholdsAfterTestEnd)

			if err == nil {
				err = tErr
			} else {
				logger.WithError(tErr).Debug("Crossed thresholds, but test already exited with another error")
			}
		}
		if finalizeThresholds != nil {
			defer handleFinalThresholdCalculation()
		}
	}

	defer func() {
		logger.Debug("Waiting for metrics and traces processing to finish...")
		close(samples)

		ww := [...]func(){
			waitOutputsFlushed,
			waitTracesFlushed,
		}
		var wg sync.WaitGroup
		wg.Add(len(ww))
		for _, w := range ww {
			go func() {
				w()
				wg.Done()
			}()
		}
		wg.Wait()

		logger.Debug("Metrics and traces processing finished!")
	}()

	printExecutionDescription(
		c.gs, "local", args[0], "", conf, executionState.ExecutionTuple, executionPlan, outputs,
	)

	// Trap Interrupts, SIGINTs and SIGTERMs.
	// TODO: move upwards, right after runCtx is created
	gracefulStop := func(sig os.Signal) {
		logger.WithField("sig", sig).Debug("Stopping k6 in response to signal...")
		// first abort the test run this way, to propagate the error
		runAbort(errext.WithAbortReasonIfNone(
			errext.WithExitCodeIfNone(
				fmt.Errorf("test run was aborted because k6 received a '%s' signal", sig), exitcodes.ExternalAbort,
			), errext.AbortedByUser,
		))
		lingerCancel() // cancel this context as well, since the user did Ctrl+C
	}
	onHardStop := func(sig os.Signal) {
		logger.WithField("sig", sig).Error("Aborting k6 in response to signal")
		globalCancel() // not that it matters, given that os.Exit() will be called right after
	}
	stopSignalHandling := handleTestAbortSignals(c.gs, gracefulStop, onHardStop)
	defer stopSignalHandling()

	// Initialize the VUs and executors
	stopVUEmission, err := execScheduler.Init(runCtx, samples)
	if err != nil {
		return err
	}
	defer stopVUEmission()

	if conf.Linger.Bool {
		defer func() {
			msg := "The test is done, but --linger was enabled, so k6 is waiting for Ctrl+C to continue..."
			select {
			case <-lingerCtx.Done():
				// do nothing, we were interrupted by Ctrl+C already
			default:
				logger.Debug(msg)
				if !c.gs.Flags.Quiet {
					printToStdout(c.gs, msg)
				}
				<-lingerCtx.Done()
				logger.Debug("Ctrl+C received, exiting...")
			}
		}()
	}

	waitInitDone()

	waitTestStartDone := emitEvent(&event.Event{Type: event.TestStart})
	waitTestStartDone()

	// Start the test! However, we won't immediately return if there was an
	// error, we still have things to do.
	err = execScheduler.Run(globalCtx, runCtx, samples)

	waitTestEndDone := emitEvent(&event.Event{Type: event.TestEnd})
	defer waitTestEndDone()

	// Init has passed successfully, so unless disabled, make sure we send a
	// usage report after the context is done.
	if !conf.NoUsageReport.Bool {
		backgroundProcesses.Add(1)
		go func() {
			defer backgroundProcesses.Done()
			reportCtx, reportCancel := context.WithTimeout(globalCtx, 3*time.Second)
			defer reportCancel()
			logger.Debug("Sending usage report...")

			if rerr := reportUsage(reportCtx, execScheduler, test); rerr != nil {
				logger.WithError(rerr).Debug("Error sending usage report")
			} else {
				logger.Debug("Usage report sent successfully")
			}
		}()
	}

	// Check what the execScheduler.Run() error is.
	if err != nil {
		err = common.UnwrapSobekInterruptedError(err)
		logger.WithError(err).Debug("Test finished with an error")
		return err
	}

	// Warn if no iterations could be completed.
	if executionState.GetFullIterationCount() == 0 {
		logger.Warn("No script iterations fully finished, consider making the test duration longer")
	}

	// The execution module enables users to mark a test as failed, while letting the test
	// execution complete. As such, we check the test status here, after the test run has finished, and
	// ensure we return an error indicating that the test run was marked as failed, and the proper
	// exit code is used.
	if testRunState.TestStatus.Failed() {
		return errext.WithExitCodeIfNone(
			fmt.Errorf("test run was marked as failed"),
			exitcodes.MarkedAsFailed,
		)
	}

	logger.Debug("Test finished cleanly")

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

func (c *cmdRun) setupTracerProvider(ctx context.Context, test *loadedAndConfiguredTest) error {
	ro := test.preInitState.RuntimeOptions
	if ro.TracesOutput.String == "none" {
		test.preInitState.TracerProvider = trace.NewNoopTracerProvider()
		return nil
	}

	tp, err := trace.TracerProviderFromConfigLine(ctx, ro.TracesOutput.String)
	if err != nil {
		return err
	}
	test.preInitState.TracerProvider = tp

	return nil
}

func getCmdRun(gs *state.GlobalState) *cobra.Command {
	c := &cmdRun{
		gs: gs,
		loadConfiguredTest: func(cmd *cobra.Command, args []string) (*loadedAndConfiguredTest, execution.Controller, error) {
			test, err := loadAndConfigureLocalTest(gs, cmd, args, getConfig)
			return test, local.NewController(), err
		},
	}

	exampleText := getExampleText(gs, `
  # Run a single VU, once.
  {{.}} run script.js

  # Run a single VU, 10 times.
  {{.}} run -i 10 script.js

  # Run 5 VUs, splitting 10 iterations between them.
  {{.}} run -u 5 -i 10 script.js

  # Run 5 VUs for 10s.
  {{.}} run -u 5 -d 10s script.js

  # Ramp VUs from 0 to 100 over 10s, stay there for 60s, then 10s down to 0.
  {{.}} run -u 0 -s 10s:100 -s 60s:100 -s 10s:0

  # Send metrics to an influxdb server
  {{.}} run -o influxdb=http://1.2.3.4:8086/k6`[1:])

	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Start a test",
		Long: `Start a test. This also exposes a REST API to interact with it. Various k6 subcommands offer
a commandline interface for interacting with it.`,
		Example: exampleText,
		Args:    exactArgsWithMsg(1, "arg should either be \"-\", if reading script from stdin, or a path to a script file"),
		RunE:    c.run,
	}

	runCmd.Flags().SortFlags = false
	runCmd.Flags().AddFlagSet(c.flagSet())

	return runCmd
}

func handleSummaryResult(fs fsext.Fs, stdOut, stdErr io.Writer, result map[string]io.Reader) error {
	var errs []error

	getWriter := func(path string) (io.Writer, error) {
		switch path {
		case "stdout":
			return stdOut, nil
		case "stderr":
			return stdErr, nil
		default:
			return fs.OpenFile(path, syscall.O_WRONLY|syscall.O_CREAT|syscall.O_TRUNC, 0o666)
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
