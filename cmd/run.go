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
	"fmt"
	"io"
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

//nolint:funlen,gocognit,gocyclo
func getRunCmd(ctx context.Context, logger *logrus.Logger) *cobra.Command {
	// runCmd represents the run command.
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
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: disable in quiet mode?
			_, _ = BannerColor.Fprintf(stdout, "\n%s\n\n", consts.Banner())

			logger.Debug("Initializing the runner...")

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

			initRunner, err := newRunner(logger, src, runType, filesystems, runtimeOptions)
			if err != nil {
				return err
			}

			logger.Debug("Getting the script options...")

			cliConf, err := getConfig(cmd.Flags())
			if err != nil {
				return err
			}
			conf, err := getConsolidatedConfig(afero.NewOsFs(), cliConf, initRunner)
			if err != nil {
				return err
			}

			conf, cerr := deriveAndValidateConfig(conf, initRunner.IsExecutable)
			if cerr != nil {
				return ExitCode{error: cerr, Code: invalidConfigErrorCode}
			}

			// Write options back to the runner too.
			if err = initRunner.SetOptions(conf.Options); err != nil {
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
			globalCtx, globalCancel := context.WithCancel(ctx)
			defer globalCancel()
			lingerCtx, lingerCancel := context.WithCancel(globalCtx)
			defer lingerCancel()
			runCtx, runCancel := context.WithCancel(lingerCtx)
			defer runCancel()

			// Create a local execution scheduler wrapping the runner.
			logger.Debug("Initializing the execution scheduler...")
			execScheduler, err := local.NewExecutionScheduler(initRunner, logger)
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
			initBar := execScheduler.GetInitProgressBar()
			progressBarWG := &sync.WaitGroup{}
			progressBarWG.Add(1)

			go func() {
				pbs := []*pb.ProgressBar{execScheduler.GetInitProgressBar()}
				for _, s := range execScheduler.GetExecutors() {
					pbs = append(pbs, s.GetProgress())
				}
				showProgress(progressCtx, conf, pbs, logger)
				progressBarWG.Done()
			}()

			// Create an engine.
			initBar.Modify(pb.WithConstProgress(0, "Init engine"))
			engine, err := core.NewEngine(execScheduler, conf.Options, runtimeOptions, logger)
			if err != nil {
				return err
			}

			executionPlan := execScheduler.GetExecutionPlan()
			// Create a collector and assign it to the engine if requested.
			initBar.Modify(pb.WithConstProgress(0, "Init metric outputs"))
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
				initBar.Modify(pb.WithConstProgress(0, "Init API server"))
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
			// start reading user input
			if !runtimeOptions.NoSummary.Bool {
				line := make(chan string)
				go func() {
					var s string
					for {
						_, scanErr := fmt.Scan(&s)
						if scanErr != nil {
							logger.WithError(scanErr).Error("failed to scan user input")
						}
						line <- s
					}
				}()

				go func() {
					for {
						ll := <-line
						if ll == "R" {
							printSummaryResults(globalCtx, initRunner, engine, executionState, logger, stdout, stderr)
						}
					}
				}()
			}

			// Initialize the engine
			initBar.Modify(pb.WithConstProgress(0, "Init VUs..."))
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
			initBar.Modify(pb.WithConstProgress(0, "Starting test..."))
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

			// Handle the end-of-test summary.
			if !runtimeOptions.NoSummary.Bool {
				printSummaryResults(globalCtx, initRunner, engine, executionState, logger, stdout, stderr)
			}

			if conf.Linger.Bool {
				select {
				case <-lingerCtx.Done():
					// do nothing, we were interrupted by Ctrl+C already
				default:
					logger.Debug("Linger set; waiting for Ctrl+C...")
					fprintf(stdout, "Linger set; waiting for Ctrl+C...")
					<-lingerCtx.Done()
					logger.Debug("Ctrl+C received, exiting...")
				}
			}
			globalCancel() // signal the Engine that it should wind down
			logger.Debug("Waiting for engine processes to finish...")
			engineWait()
			logger.Debug("Everything has finished, exiting k6!")
			if engine.IsTainted() {
				return ExitCode{error: errors.New("some thresholds have failed"), Code: thresholdHaveFailedErrorCode}
			}
			return nil
		},
	}

	runCmd.Flags().SortFlags = false
	runCmd.Flags().AddFlagSet(runCmdFlagSet())

	return runCmd
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

func printSummaryResults(
	globalCtx context.Context,
	runner lib.Runner,
	engine *core.Engine,
	executionState *lib.ExecutionState,
	log *logrus.Logger,
	stdOut, stdErr io.Writer,
) {
	engine.MetricsLock.Lock()
	summaryResult, err := runner.HandleSummary(globalCtx, &lib.Summary{
		Metrics:         engine.Metrics,
		RootGroup:       engine.ExecutionScheduler.GetRunner().GetDefaultGroup(),
		TestRunDuration: executionState.GetCurrentTestRunDuration(),
	})
	engine.MetricsLock.Unlock()

	if err == nil {
		hErr := handleSummaryResult(afero.NewOsFs(), stdOut, stdErr, summaryResult)
		if hErr != nil {
			log.WithError(hErr).Error("failed to handle summary result")
		}
	}
	if err != nil {
		log.WithError(err).Error("failed to handle the end-of-test summary")
	}
}
