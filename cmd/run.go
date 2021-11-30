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
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"go.k6.io/k6/api"
	"go.k6.io/k6/core"
	"go.k6.io/k6/core/local"
	"go.k6.io/k6/errext"
	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/js"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/consts"
	"go.k6.io/k6/lib/metrics"
	"go.k6.io/k6/loader"
	"go.k6.io/k6/ui/pb"
)

const (
	typeJS      = "js"
	typeArchive = "archive"
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
			_, _ = fmt.Fprintf(stdout, "\n%s\n\n", getBanner(noColor || !stdoutTTY))

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

			osEnvironment := buildEnvMap(os.Environ())
			runtimeOptions, err := getRuntimeOptions(cmd.Flags(), osEnvironment)
			if err != nil {
				return err
			}

			registry := metrics.NewRegistry()
			builtinMetrics := metrics.RegisterBuiltinMetrics(registry)
			initRunner, err := newRunner(logger, src, runType, filesystems, runtimeOptions, builtinMetrics, registry)
			if err != nil {
				return common.UnwrapGojaInterruptedError(err)
			}

			logger.Debug("Getting the script options...")

			cliConf, err := getConfig(cmd.Flags())
			if err != nil {
				return err
			}
			conf, err := getConsolidatedConfig(afero.NewOsFs(), cliConf, initRunner.GetOptions())
			if err != nil {
				return err
			}

			conf, err = deriveAndValidateConfig(conf, initRunner.IsExecutable)
			if err != nil {
				return err
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

			// Create all outputs.
			executionPlan := execScheduler.GetExecutionPlan()
			outputs, err := createOutputs(conf.Out, src, conf, runtimeOptions, executionPlan, osEnvironment, logger)
			if err != nil {
				return err
			}

			// Create the engine.
			initBar.Modify(pb.WithConstProgress(0, "Init engine"))
			engine, err := core.NewEngine(execScheduler, conf.Options, runtimeOptions, outputs, logger, builtinMetrics)
			if err != nil {
				return err
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
							os.Exit(int(exitcodes.CannotStartRESTAPI))
						} else {
							logger.WithError(aerr).Warn("Error from API server")
						}
					}
				}()
			}

			// We do this here so we can get any output URLs below.
			initBar.Modify(pb.WithConstProgress(0, "Starting outputs"))
			err = engine.StartOutputs()
			if err != nil {
				return err
			}
			defer engine.StopOutputs()

			printExecutionDescription(
				"local", filename, "", conf, execScheduler.GetState().ExecutionTuple,
				executionPlan, outputs, noColor || !stdoutTTY)

			// Trap Interrupts, SIGINTs and SIGTERMs.
			sigC := make(chan os.Signal, 1)
			signal.Notify(sigC, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
			defer signal.Stop(sigC)
			go func() {
				sig := <-sigC
				logger.WithField("sig", sig).Debug("Stopping k6 in response to signal...")
				lingerCancel() // stop the test run, metric processing is cancelled below

				// If we get a second signal, we immediately exit, so something like
				// https://github.com/k6io/k6/issues/971 never happens again
				sig = <-sigC
				logger.WithField("sig", sig).Error("Aborting k6 in response to signal")
				globalCancel() // not that it matters, given the following command...
				os.Exit(int(exitcodes.ExternalAbort))
			}()

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
			var interrupt error
			err = engineRun()
			if err != nil {
				err = common.UnwrapGojaInterruptedError(err)
				if common.IsInterruptError(err) {
					// Don't return here since we need to work with --linger,
					// show the end-of-test summary and exit cleanly.
					interrupt = err
				}
				if !conf.Linger.Bool && interrupt == nil {
					return errext.WithExitCodeIfNone(err, exitcodes.GenericEngine)
				}
			}
			runCancel()
			logger.Debug("Engine run terminated cleanly")

			progressCancel()
			progressBarWG.Wait()

			executionState := execScheduler.GetState()
			// Warn if no iterations could be completed.
			if executionState.GetFullIterationCount() == 0 {
				logger.Warn("No script iterations finished, consider making the test duration longer")
			}

			// Handle the end-of-test summary.
			if !runtimeOptions.NoSummary.Bool {
				summaryResult, err := initRunner.HandleSummary(globalCtx, &lib.Summary{
					Metrics:         engine.Metrics,
					RootGroup:       engine.ExecutionScheduler.GetRunner().GetDefaultGroup(),
					TestRunDuration: executionState.GetCurrentTestRunDuration(),
					NoColor:         noColor,
					UIState: lib.UIState{
						IsStdOutTTY: stdoutTTY,
						IsStdErrTTY: stderrTTY,
					},
				})
				if err == nil {
					err = handleSummaryResult(afero.NewOsFs(), stdout, stderr, summaryResult)
				}
				if err != nil {
					logger.WithError(err).Error("failed to handle the end-of-test summary")
				}
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
			if interrupt != nil {
				return interrupt
			}
			if engine.IsTainted() {
				return errext.WithExitCodeIfNone(errors.New("some thresholds have failed"), exitcodes.ThresholdsHaveFailed)
			}
			return nil
		},
	}

	runCmd.Flags().SortFlags = false
	runCmd.Flags().AddFlagSet(runCmdFlagSet())

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
	builtinMetrics *metrics.BuiltinMetrics, registry *metrics.Registry,
) (runner lib.Runner, err error) {
	switch typ {
	case "":
		runner, err = newRunner(logger, src, detectType(src.Data), filesystems, rtOpts, builtinMetrics, registry)
	case typeJS:
		runner, err = js.New(logger, src, filesystems, rtOpts, builtinMetrics, registry)
	case typeArchive:
		var arc *lib.Archive
		arc, err = lib.ReadArchive(bytes.NewReader(src.Data))
		if err != nil {
			return nil, err
		}
		switch arc.Type {
		case typeJS:
			runner, err = js.NewFromArchive(logger, arc, rtOpts, builtinMetrics, registry)
		default:
			return nil, fmt.Errorf("archive requests unsupported runner: %s", arc.Type)
		}
	default:
		return nil, fmt.Errorf("unknown -t/--type: %s", typ)
	}

	return runner, err
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
			return fs.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
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
