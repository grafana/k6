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
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/loadimpact/k6/api"
	"github.com/loadimpact/k6/core"
	"github.com/loadimpact/k6/core/local"
	"github.com/loadimpact/k6/js"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/consts"
	"github.com/loadimpact/k6/loader"
	"github.com/loadimpact/k6/ui"
	"github.com/loadimpact/k6/ui/pb"
)

const (
	typeJS      = "js"
	typeArchive = "archive"

	thresholdHaveFailedErrorCode = 99
	setupTimeoutErrorCode        = 100
	teardownTimeoutErrorCode     = 101
	genericTimeoutErrorCode      = 102
	genericEngineErrorCode       = 103
	invalidConfigErrorCode       = 104
	externalAbortErrorCode       = 105
	cannotStartRESTAPIErrorCode  = 106
)

// TODO: fix this, global variables are not very testable...
//nolint:gochecknoglobals
var runType = os.Getenv("K6_TYPE")

// runCmd represents the run command.
var runCmd = &cobra.Command{
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
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: don't use a global... or maybe change the logger?
		logger := logrus.StandardLogger()

		// TODO: disable in quiet mode?
		_, _ = BannerColor.Fprintf(stdout, "\n%s\n\n", consts.Banner)

		initBar := pb.New(
			pb.WithConstLeft(" Init"),
			pb.WithConstProgress(0, "runner"),
		)
		printBar(initBar)

		// Create the Runner.
		pwd, err := os.Getwd()
		if err != nil {
			return err
		}
		filename := args[0]
		filesystems := loader.CreateFilesystems()
		src, err := loader.ReadSource(logger, filename, pwd, filesystems, os.Stdin)
		if err != nil {
			return err
		}

		runtimeOptions, err := getRuntimeOptions(cmd.Flags(), buildEnvMap(os.Environ()))
		if err != nil {
			return err
		}

		r, err := newRunner(logger, src, runType, filesystems, runtimeOptions)
		if err != nil {
			return err
		}

		modifyAndPrintBar(initBar, pb.WithConstProgress(0, "options"))

		cliConf, err := getConfig(cmd.Flags())
		if err != nil {
			return err
		}
		conf, err := getConsolidatedConfig(afero.NewOsFs(), cliConf, r)
		if err != nil {
			return err
		}

		conf, cerr := deriveAndValidateConfig(conf, r.IsExecutable)
		if cerr != nil {
			return ExitCode{error: cerr, Code: invalidConfigErrorCode}
		}

		// Write options back to the runner too.
		if err = r.SetOptions(conf.Options); err != nil {
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
		globalCtx, globalCancel := context.WithCancel(context.Background())
		defer globalCancel()
		lingerCtx, lingerCancel := context.WithCancel(globalCtx)
		defer lingerCancel()
		runCtx, runCancel := context.WithCancel(lingerCtx)
		defer runCancel()

		// Create a local execution scheduler wrapping the runner.
		modifyAndPrintBar(initBar, pb.WithConstProgress(0, "execution scheduler"))
		execScheduler, err := local.NewExecutionScheduler(r, logger)
		if err != nil {
			return err
		}

		executionState := execScheduler.GetState()

		// This is manually triggered after the Engine's Run() has completed,
		// and things like a single Ctrl+C don't affect it. We use it to make
		// sure that the progressbars finish updating with the latest execution
		// state one last time, after the test run has finished.
		progressCtx, progressCancel := context.WithCancel(globalCtx)
		defer progressCancel()
		initBar = execScheduler.GetInitProgressBar()
		progressBarWG := &sync.WaitGroup{}
		progressBarWG.Add(1)
		go func() {
			showProgress(progressCtx, conf, execScheduler, logger)
			progressBarWG.Done()
		}()

		// Create an engine.
		modifyAndPrintBar(initBar, pb.WithConstProgress(0, "Init engine"))
		engine, err := core.NewEngine(execScheduler, conf.Options, logger)
		if err != nil {
			return err
		}

		// TODO: refactor, the engine should have a copy of the config...
		// Configure the engine.
		if conf.NoThresholds.Valid {
			engine.NoThresholds = conf.NoThresholds.Bool
		}
		if conf.NoSummary.Valid {
			engine.NoSummary = conf.NoSummary.Bool
		}
		if conf.SummaryExport.Valid {
			engine.SummaryExport = conf.SummaryExport.String != ""
		}

		executionPlan := execScheduler.GetExecutionPlan()
		// Create a collector and assign it to the engine if requested.
		modifyAndPrintBar(initBar, pb.WithConstProgress(0, "Init metric outputs"))
		for _, out := range conf.Out {
			t, arg := parseCollector(out)
			collector, cerr := newCollector(logger, t, arg, src, conf, executionPlan)
			if cerr != nil {
				return cerr
			}
			if cerr = collector.Init(); cerr != nil {
				return cerr
			}
			engine.Collectors = append(engine.Collectors, collector)
		}

		// Spin up the REST API server, if not disabled.
		if address != "" {
			modifyAndPrintBar(initBar, pb.WithConstProgress(0, "Init API server"))
			go func() {
				logger.Debugf("Starting the REST API server on %s", address)
				if aerr := api.ListenAndServe(address, engine, logger); aerr != nil {
					// Only exit k6 if the user has explicitly set the REST API address
					if cmd.Flags().Lookup("address").Changed {
						logger.WithError(aerr).Error("Error from API server")
						os.Exit(cannotStartRESTAPIErrorCode)
					} else {
						logger.WithError(aerr).Warn("Error from API server")
					}
				}
			}()
		}

		printExecutionDescription(
			"local", filename, "", conf, execScheduler.GetState().ExecutionTuple,
			executionPlan, engine.Collectors)

		// Trap Interrupts, SIGINTs and SIGTERMs.
		sigC := make(chan os.Signal, 1)
		signal.Notify(sigC, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(sigC)
		go func() {
			sig := <-sigC
			logger.WithField("sig", sig).Debug("Stopping k6 in response to signal...")
			lingerCancel() // stop the test run, metric processing is cancelled below

			// If we get a second signal, we immediately exit, so something like
			// https://github.com/loadimpact/k6/issues/971 never happens again
			sig = <-sigC
			logger.WithField("sig", sig).Error("Aborting k6 in response to signal")
			globalCancel() // not that it matters, given the following command...
			os.Exit(externalAbortErrorCode)
		}()

		// Initialize the engine
		modifyAndPrintBar(initBar, pb.WithConstProgress(0, "Init VUs"))
		engineRun, engineWait, err := engine.Init(globalCtx, runCtx)
		if err != nil {
			return getExitCodeFromEngine(err)
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
		modifyAndPrintBar(initBar, pb.WithConstProgress(0, "Start test"))
		if err := engineRun(); err != nil {
			return getExitCodeFromEngine(err)
		}
		runCancel()
		logger.Debug("Engine run terminated cleanly")

		progressCancel()
		progressBarWG.Wait()

		// Warn if no iterations could be completed.
		if executionState.GetFullIterationCount() == 0 {
			logger.Warn("No script iterations finished, consider making the test duration longer")
		}

		data := ui.SummaryData{
			Metrics:   engine.Metrics,
			RootGroup: engine.ExecutionScheduler.GetRunner().GetDefaultGroup(),
			Time:      executionState.GetCurrentTestRunDuration(),
			TimeUnit:  conf.Options.SummaryTimeUnit.String,
		}
		// Print the end-of-test summary.
		if !conf.NoSummary.Bool {
			fprintf(stdout, "\n")

			s := ui.NewSummary(conf.SummaryTrendStats)
			s.SummarizeMetrics(stdout, "", data)

			fprintf(stdout, "\n")
		}

		if conf.SummaryExport.ValueOrZero() != "" {
			f, err := os.Create(conf.SummaryExport.String)
			if err != nil {
				logger.WithError(err).Error("failed to create summary export file")
			} else {
				defer func() {
					if err := f.Close(); err != nil {
						logger.WithError(err).Error("failed to close summary export file")
					}
				}()
				s := ui.NewSummary(conf.SummaryTrendStats)
				if err := s.SummarizeMetricsJSON(f, data); err != nil {
					logger.WithError(err).Error("failed to make summary export file")
				}
			}
		}

		if conf.Linger.Bool {
			select {
			case <-lingerCtx.Done():
				// do nothing, we were interrupted by Ctrl+C already
			default:
				logger.Info("Linger set; waiting for Ctrl+C...")
				<-lingerCtx.Done()
			}
		}
		globalCancel() // signal the Engine that it should wind down
		logger.Debug("Waiting for engine processes to finish...")
		engineWait()

		if engine.IsTainted() {
			return ExitCode{error: errors.New("some thresholds have failed"), Code: thresholdHaveFailedErrorCode}
		}
		return nil
	},
}

func getExitCodeFromEngine(err error) ExitCode {
	switch e := errors.Cause(err).(type) {
	case lib.TimeoutError:
		switch e.Place() {
		case consts.SetupFn:
			return ExitCode{error: err, Code: setupTimeoutErrorCode, Hint: e.Hint()}
		case consts.TeardownFn:
			return ExitCode{error: err, Code: teardownTimeoutErrorCode, Hint: e.Hint()}
		default:
			return ExitCode{error: err, Code: genericTimeoutErrorCode}
		}
	default:
		//nolint:golint
		return ExitCode{error: errors.New("Engine error"), Code: genericEngineErrorCode, Hint: err.Error()}
	}
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
	res, err := http.Post("https://reports.k6.io/", "application/json", bytes.NewBuffer(body))
	defer func() {
		if err == nil {
			_ = res.Body.Close()
		}
	}()

	return err
}

func runCmdFlagSet() *pflag.FlagSet {
	flags := pflag.NewFlagSet("", pflag.ContinueOnError)
	flags.SortFlags = false
	flags.AddFlagSet(optionFlagSet())
	flags.AddFlagSet(runtimeOptionFlagSet(true))
	flags.AddFlagSet(configFlagSet())

	// TODO: Figure out a better way to handle the CLI flags:
	// - the default values are specified in this way so we don't overwrire whatever
	//   was specified via the environment variables
	// - but we need to manually specify the DefValue, since that's the default value
	//   that will be used in the help/usage message - if we don't set it, the environment
	//   variables will affect the usage message
	// - and finally, global variables are not very testable... :/
	flags.StringVarP(&runType, "type", "t", runType, "override file `type`, \"js\" or \"archive\"")
	flags.Lookup("type").DefValue = ""
	return flags
}

func init() {
	RootCmd.AddCommand(runCmd)

	runCmd.Flags().SortFlags = false
	runCmd.Flags().AddFlagSet(runCmdFlagSet())
}

// Creates a new runner.
func newRunner(
	logger *logrus.Logger, src *loader.SourceData, typ string, filesystems map[string]afero.Fs, rtOpts lib.RuntimeOptions,
) (lib.Runner, error) {
	switch typ {
	case "":
		return newRunner(logger, src, detectType(src.Data), filesystems, rtOpts)
	case typeJS:
		return js.New(logger, src, filesystems, rtOpts)
	case typeArchive:
		arc, err := lib.ReadArchive(bytes.NewReader(src.Data))
		if err != nil {
			return nil, err
		}
		switch arc.Type {
		case typeJS:
			return js.NewFromArchive(logger, arc, rtOpts)
		default:
			return nil, errors.Errorf("archive requests unsupported runner: %s", arc.Type)
		}
	default:
		return nil, errors.Errorf("unknown -t/--type: %s", typ)
	}
}

func detectType(data []byte) string {
	if _, err := tar.NewReader(bytes.NewReader(data)).Next(); err == nil {
		return typeArchive
	}
	return typeJS
}
