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
	"os"
	"os/signal"
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
)

//TODO: fix this, global variables are not very testable...
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
		//TODO: disable in quiet mode?
		_, _ = BannerColor.Fprintf(stdout, "\n%s\n\n", consts.Banner)

		initBar := pb.New(pb.WithConstLeft("   init"))

		// Create the Runner.
		printBar(initBar, "runner")
		pwd, err := os.Getwd()
		if err != nil {
			return err
		}
		filename := args[0]
		filesystems := loader.CreateFilesystems()
		src, err := loader.ReadSource(filename, pwd, filesystems, os.Stdin)
		if err != nil {
			return err
		}

		runtimeOptions, err := getRuntimeOptions(cmd.Flags(), buildEnvMap(os.Environ()))
		if err != nil {
			return err
		}

		r, err := newRunner(src, runType, filesystems, runtimeOptions)
		if err != nil {
			return err
		}

		printBar(initBar, "options")

		cliConf, err := getConfig(cmd.Flags())
		if err != nil {
			return err
		}
		conf, err := getConsolidatedConfig(afero.NewOsFs(), cliConf, r)
		if err != nil {
			return err
		}

		conf, cerr := deriveAndValidateConfig(conf)
		if cerr != nil {
			return ExitCode{error: cerr, Code: invalidConfigErrorCode}
		}

		// Write options back to the runner too.
		if err = r.SetOptions(conf.Options); err != nil {
			return err
		}

		//TODO: don't use a global... or maybe change the logger?
		logger := logrus.StandardLogger()

		ctx, cancel := context.WithCancel(context.Background()) //TODO: move even earlier?
		defer cancel()

		// Create a local execution scheduler wrapping the runner.
		printBar(initBar, "execution scheduler")
		execScheduler, err := local.NewExecutionScheduler(r, logger)
		if err != nil {
			return err
		}

		executionState := execScheduler.GetState()
		initBar = execScheduler.GetInitProgressBar()
		progressBarWG := &sync.WaitGroup{}
		progressBarWG.Add(1)
		go func() {
			showProgress(ctx, conf, execScheduler)
			progressBarWG.Done()
		}()

		// Create an engine.
		initBar.Modify(pb.WithConstProgress(0, "Init engine"))
		engine, err := core.NewEngine(execScheduler, conf.Options, logger)
		if err != nil {
			return err
		}

		//TODO: the engine should just probably have a copy of the config...
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

		// Create a collector and assign it to the engine if requested.
		initBar.Modify(pb.WithConstProgress(0, "Init metric outputs"))
		for _, out := range conf.Out {
			t, arg := parseCollector(out)
			collector, err := newCollector(t, arg, src, conf, execScheduler.GetExecutionPlan())
			if err != nil {
				return err
			}
			if err := collector.Init(); err != nil {
				return err
			}
			engine.Collectors = append(engine.Collectors, collector)
		}

		// Create an API server.
		if address != "" {
			initBar.Modify(pb.WithConstProgress(0, "Init API server"))
			go func() {
				if err := api.ListenAndServe(address, engine); err != nil {
					logger.WithError(err).Warn("Error from API server")
				}
			}()
		}

		// Write the big banner.
		{
			out := "-"
			link := ""

			for idx, collector := range engine.Collectors {
				if out != "-" {
					out = out + "; " + conf.Out[idx]
				} else {
					out = conf.Out[idx]
				}

				if l := collector.Link(); l != "" {
					link = link + " (" + l + ")"
				}
			}

			fprintf(stdout, "   executor: %s\n", ui.ValueColor.Sprint("local"))
			fprintf(stdout, "     output: %s%s\n", ui.ValueColor.Sprint(out), ui.ExtraColor.Sprint(link))
			fprintf(stdout, "     script: %s\n", ui.ValueColor.Sprint(filename))
			fprintf(stdout, "\n")

			plan := execScheduler.GetExecutionPlan()
			executorConfigs := execScheduler.GetExecutorConfigs()
			maxDuration, _ := lib.GetEndOffset(plan)

			fprintf(stdout, "  execution: %s\n", ui.ValueColor.Sprintf(
				"(%.2f%%) %d executors, %d max VUs, %s max duration (incl. graceful stop):",
				conf.ExecutionSegment.FloatLength()*100, len(executorConfigs),
				lib.GetMaxPossibleVUs(plan), maxDuration),
			)
			for _, ec := range executorConfigs {
				fprintf(stdout, "           * %s: %s\n",
					ec.GetName(), ec.GetDescription(conf.ExecutionSegment))
			}
			fprintf(stdout, "\n")
		}

		// Run the engine with a cancellable context.
		errC := make(chan error)
		go func() {
			initBar.Modify(pb.WithConstProgress(0, "Init VUs"))
			if err := engine.Init(ctx); err != nil {
				errC <- err
			} else {
				initBar.Modify(pb.WithConstProgress(0, "Start test"))
				errC <- engine.Run(ctx)
			}
		}()

		// Trap Interrupts, SIGINTs and SIGTERMs.
		sigC := make(chan os.Signal, 1)
		signal.Notify(sigC, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(sigC)

		// If the user hasn't opted out: report usage.
		//TODO: fix
		//TODO: move to a separate function
		/*
			if !conf.NoUsageReport.Bool {
				go func() {
					u := "http://k6reports.loadimpact.com/"
					mime := "application/json"
					var endTSeconds float64
					if endT := engine.Executor.GetEndTime(); endT.Valid {
						endTSeconds = time.Duration(endT.Duration).Seconds()
					}
					var stagesEndTSeconds float64
					if stagesEndT := lib.SumStages(engine.Executor.GetStages()); stagesEndT.Valid {
						stagesEndTSeconds = time.Duration(stagesEndT.Duration).Seconds()
					}
					body, err := json.Marshal(map[string]interface{}{
						"k6_version":  Version,
						"vus_max":     engine.Executor.GetVUsMax(),
						"iterations":  engine.Executor.GetEndIterations(),
						"duration":    endTSeconds,
						"st_duration": stagesEndTSeconds,
						"goos":        runtime.GOOS,
						"goarch":      runtime.GOARCH,
					})
					if err != nil {
						panic(err) // This should never happen!!
					}
					_, _ = http.Post(u, mime, bytes.NewBuffer(body))
				}()
			}
		*/

		// Ticker for progress bar updates. Less frequent updates for non-TTYs, none if quiet.
		updateFreq := 50 * time.Millisecond
		if !stdoutTTY {
			updateFreq = 1 * time.Second
		}
		ticker := time.NewTicker(updateFreq)
		if quiet || conf.HTTPDebug.Valid && conf.HTTPDebug.String != "" {
			ticker.Stop()
		}
	mainLoop:
		for {
			select {
			case <-ticker.C:
				if quiet || !stdoutTTY {
					l := logrus.WithFields(logrus.Fields{
						"t": executionState.GetCurrentTestRunDuration(),
						"i": executionState.GetFullIterationCount(),
					})
					fn := l.Info
					if quiet {
						fn = l.Debug
					}
					if executionState.IsPaused() {
						fn("Paused")
					} else {
						fn("Running")
					}
				}
			case err := <-errC:
				cancel()
				if err == nil {
					logger.Debug("Engine terminated cleanly")
					break mainLoop
				}

				switch e := errors.Cause(err).(type) {
				case lib.TimeoutError:
					switch e.Place() {
					case "setup":
						return ExitCode{error: err, Code: setupTimeoutErrorCode, Hint: e.Hint()}
					case "teardown":
						return ExitCode{error: err, Code: teardownTimeoutErrorCode, Hint: e.Hint()}
					default:
						return ExitCode{error: err, Code: genericTimeoutErrorCode}
					}
				default:
					//nolint:golint
					return ExitCode{error: errors.New("Engine error"), Code: genericEngineErrorCode, Hint: err.Error()}
				}
			case sig := <-sigC:
				logger.WithField("sig", sig).Debug("Exiting in response to signal")
				cancel()
				//TODO: Actually exit on a second Ctrl+C, even if some of the iterations are stuck.
				// This is currently problematic because of https://github.com/loadimpact/k6/issues/971,
				// but with uninterruptible iterations it will be even more problematic.
			}
		}
		if quiet || !stdoutTTY {
			e := logger.WithFields(logrus.Fields{
				"t": executionState.GetCurrentTestRunDuration(),
				"i": executionState.GetFullIterationCount(),
			})
			fn := e.Info
			if quiet {
				fn = e.Debug
			}
			fn("Test finished")
		}

		progressBarWG.Wait()

		// Warn if no iterations could be completed.
		if executionState.GetFullIterationCount() == 0 {
			logger.Warn("No data generated, because no script iterations finished, consider making the test duration longer")
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
				logrus.WithError(err).Error("failed to create summary export file")
			} else {
				defer func() {
					if err := f.Close(); err != nil {
						logrus.WithError(err).Error("failed to close summary export file")
					}
				}()
				s := ui.NewSummary(conf.SummaryTrendStats)
				if err := s.SummarizeMetricsJSON(f, data); err != nil {
					logrus.WithError(err).Error("failed to make summary export file")
				}
			}
		}

		if conf.Linger.Bool {
			logger.Info("Linger set; waiting for Ctrl+C...")
			<-sigC
		}

		if engine.IsTainted() {
			return ExitCode{error: errors.New("some thresholds have failed"), Code: thresholdHaveFailedErrorCode}
		}
		return nil
	},
}

func runCmdFlagSet() *pflag.FlagSet {
	flags := pflag.NewFlagSet("", pflag.ContinueOnError)
	flags.SortFlags = false
	flags.AddFlagSet(optionFlagSet())
	flags.AddFlagSet(runtimeOptionFlagSet(true))
	flags.AddFlagSet(configFlagSet())

	//TODO: Figure out a better way to handle the CLI flags:
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
	src *loader.SourceData, typ string, filesystems map[string]afero.Fs, rtOpts lib.RuntimeOptions,
) (lib.Runner, error) {
	switch typ {
	case "":
		return newRunner(src, detectType(src.Data), filesystems, rtOpts)
	case typeJS:
		return js.New(src, filesystems, rtOpts)
	case typeArchive:
		arc, err := lib.ReadArchive(bytes.NewReader(src.Data))
		if err != nil {
			return nil, err
		}
		switch arc.Type {
		case typeJS:
			return js.NewFromArchive(arc, rtOpts)
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
