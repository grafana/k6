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
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/loadimpact/k6/api"
	"github.com/loadimpact/k6/core"
	"github.com/loadimpact/k6/core/local"
	"github.com/loadimpact/k6/js"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/loader"
	"github.com/loadimpact/k6/ui"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	null "gopkg.in/guregu/null.v3"
)

const (
	typeJS      = "js"
	typeArchive = "archive"
)

var (
	runType       = os.Getenv("K6_TYPE")
	runNoSetup    = os.Getenv("K6_NO_SETUP") != ""
	runNoTeardown = os.Getenv("K6_NO_TEARDOWN") != ""
)

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
		_, _ = BannerColor.Fprint(stdout, Banner+"\n\n")

		initBar := ui.ProgressBar{
			Width: 60,
			Left:  func() string { return "    init" },
		}

		// Create the Runner.
		fprintf(stdout, "%s runner\r", initBar.String())
		pwd, err := os.Getwd()
		if err != nil {
			return err
		}
		filename := args[0]
		fs := afero.NewOsFs()
		src, err := readSource(filename, pwd, fs, os.Stdin)
		if err != nil {
			return err
		}

		runtimeOptions, err := getRuntimeOptions(cmd.Flags())
		if err != nil {
			return err
		}

		r, err := newRunner(src, runType, fs, runtimeOptions)
		if err != nil {
			return err
		}

		fprintf(stdout, "%s options\r", initBar.String())

		cliConf, err := getConfig(cmd.Flags())
		if err != nil {
			return err
		}
		conf, err := getConsolidatedConfig(fs, cliConf, r)
		if err != nil {
			return err
		}

		// If -m/--max isn't specified, figure out the max that should be needed.
		if !conf.VUsMax.Valid {
			conf.VUsMax = null.NewInt(conf.VUs.Int64, conf.VUs.Valid)
			for _, stage := range conf.Stages {
				if stage.Target.Valid && stage.Target.Int64 > conf.VUsMax.Int64 {
					conf.VUsMax = stage.Target
				}
			}
		}

		// If -d/--duration, -i/--iterations and -s/--stage are all unset, run to one iteration.
		if !conf.Duration.Valid && !conf.Iterations.Valid && len(conf.Stages) == 0 {
			conf.Iterations = null.IntFrom(1)
		}

		if conf.Iterations.Valid && conf.Iterations.Int64 < conf.VUsMax.Int64 {
			log.Warnf(
				"All iterations (%d in this test run) are shared between all VUs, so some of the %d VUs will not execute even a single iteration!",
				conf.Iterations.Int64, conf.VUsMax.Int64,
			)
		}

		// If duration is explicitly set to 0, it means run forever.
		if conf.Duration.Valid && conf.Duration.Duration == 0 {
			conf.Duration = types.NullDuration{}
		}

		// If summary trend stats are defined, update the UI to reflect them
		if len(conf.SummaryTrendStats) > 0 {
			ui.UpdateTrendColumns(conf.SummaryTrendStats)
		}

		// Write options back to the runner too.
		r.SetOptions(conf.Options)

		// Create a local executor wrapping the runner.
		fprintf(stdout, "%s executor\r", initBar.String())
		ex := local.New(r)
		if runNoSetup {
			ex.SetRunSetup(false)
		}
		if runNoTeardown {
			ex.SetRunTeardown(false)
		}

		// Create an engine.
		fprintf(stdout, "%s   engine\r", initBar.String())
		engine, err := core.NewEngine(ex, conf.Options)
		if err != nil {
			return err
		}

		// Configure the engine.
		if conf.NoThresholds.Valid {
			engine.NoThresholds = conf.NoThresholds.Bool
		}

		// Create a collector and assign it to the engine if requested.
		fprintf(stdout, "%s   collector\r", initBar.String())
		for _, out := range conf.Out {
			t, arg := parseCollector(out)
			collector, err := newCollector(t, arg, src, conf)
			if err != nil {
				return err
			}
			if err := collector.Init(); err != nil {
				return err
			}
			engine.Collectors = append(engine.Collectors, collector)
		}

		// Create an API server.
		fprintf(stdout, "%s   server\r", initBar.String())
		go func() {
			if err := api.ListenAndServe(address, engine); err != nil {
				log.WithError(err).Warn("Error from API server")
			}
		}()

		// Write the big banner.
		{
			out := "-"
			link := ""
			if engine.Collectors != nil {
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
			}

			fprintf(stdout, "  execution: %s\n", ui.ValueColor.Sprint("local"))
			fprintf(stdout, "     output: %s%s\n", ui.ValueColor.Sprint(out), ui.ExtraColor.Sprint(link))
			fprintf(stdout, "     script: %s\n", ui.ValueColor.Sprint(filename))
			fprintf(stdout, "\n")

			duration := ui.GrayColor.Sprint("-")
			iterations := ui.GrayColor.Sprint("-")
			if conf.Duration.Valid {
				duration = ui.ValueColor.Sprint(conf.Duration.Duration)
			}
			if conf.Iterations.Valid {
				iterations = ui.ValueColor.Sprint(conf.Iterations.Int64)
			}
			vus := ui.ValueColor.Sprint(conf.VUs.Int64)
			max := ui.ValueColor.Sprint(conf.VUsMax.Int64)

			leftWidth := ui.StrWidth(duration)
			if l := ui.StrWidth(vus); l > leftWidth {
				leftWidth = l
			}
			durationPad := strings.Repeat(" ", leftWidth-ui.StrWidth(duration))
			vusPad := strings.Repeat(" ", leftWidth-ui.StrWidth(vus))

			fprintf(stdout, "    duration: %s,%s iterations: %s\n", duration, durationPad, iterations)
			fprintf(stdout, "         vus: %s,%s max: %s\n", vus, vusPad, max)
			fprintf(stdout, "\n")
		}

		// Run the engine with a cancellable context.
		fprintf(stdout, "%s starting\r", initBar.String())
		ctx, cancel := context.WithCancel(context.Background())
		errC := make(chan error)
		go func() { errC <- engine.Run(ctx) }()

		// Trap Interrupts, SIGINTs and SIGTERMs.
		sigC := make(chan os.Signal, 1)
		signal.Notify(sigC, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(sigC)

		// If the user hasn't opted out: report usage.
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

		// Prepare a progress bar.
		progress := ui.ProgressBar{
			Width: 60,
			Left: func() string {
				if engine.Executor.IsPaused() {
					return "  paused"
				} else if engine.Executor.IsRunning() {
					return " running"
				} else {
					return "    done"
				}
			},
			Right: func() string {
				if endIt := engine.Executor.GetEndIterations(); endIt.Valid {
					return fmt.Sprintf("%d / %d", engine.Executor.GetIterations(), endIt.Int64)
				}
				precision := 100 * time.Millisecond
				atT := engine.Executor.GetTime()
				stagesEndT := lib.SumStages(engine.Executor.GetStages())
				endT := engine.Executor.GetEndTime()
				if !endT.Valid || (stagesEndT.Valid && endT.Duration > stagesEndT.Duration) {
					endT = stagesEndT
				}
				if endT.Valid {
					return fmt.Sprintf("%s / %s",
						(atT/precision)*precision,
						(time.Duration(endT.Duration)/precision)*precision,
					)
				}
				return ((atT / precision) * precision).String()
			},
		}

		// Ticker for progress bar updates. Less frequent updates for non-TTYs, none if quiet.
		updateFreq := 50 * time.Millisecond
		if !stdoutTTY {
			updateFreq = 1 * time.Second
		}
		ticker := time.NewTicker(updateFreq)
		if quiet || conf.HttpDebug.Valid && conf.HttpDebug.String != "" {
			ticker.Stop()
		}
	mainLoop:
		for {
			select {
			case <-ticker.C:
				if quiet || !stdoutTTY {
					l := log.WithFields(log.Fields{
						"t": engine.Executor.GetTime(),
						"i": engine.Executor.GetIterations(),
					})
					fn := l.Info
					if quiet {
						fn = l.Debug
					}
					if engine.Executor.IsPaused() {
						fn("Paused")
					} else {
						fn("Running")
					}
					break
				}

				var prog float64
				if endIt := engine.Executor.GetEndIterations(); endIt.Valid {
					prog = float64(engine.Executor.GetIterations()) / float64(endIt.Int64)
				} else {
					stagesEndT := lib.SumStages(engine.Executor.GetStages())
					endT := engine.Executor.GetEndTime()
					if !endT.Valid || (stagesEndT.Valid && endT.Duration > stagesEndT.Duration) {
						endT = stagesEndT
					}
					if endT.Valid {
						prog = float64(engine.Executor.GetTime()) / float64(endT.Duration)
					}
				}
				progress.Progress = prog
				fprintf(stdout, "%s\x1b[0K\r", progress.String())
			case err := <-errC:
				if err != nil {
					log.WithError(err).Error("Engine error")
				} else {
					log.Debug("Engine terminated cleanly")
				}
				cancel()
				break mainLoop
			case sig := <-sigC:
				log.WithField("sig", sig).Debug("Exiting in response to signal")
				cancel()
			}
		}
		if quiet || !stdoutTTY {
			e := log.WithFields(log.Fields{
				"t": engine.Executor.GetTime(),
				"i": engine.Executor.GetIterations(),
			})
			fn := e.Info
			if quiet {
				fn = e.Debug
			}
			fn("Test finished")
		} else {
			progress.Progress = 1
			fprintf(stdout, "%s\x1b[0K\n", progress.String())
		}

		// Warn if no iterations could be completed.
		if engine.Executor.GetIterations() == 0 {
			log.Warn("No data generated, because no script iterations finished, consider making the test duration longer")
		}

		// Print the end-of-test summary.
		if !quiet {
			fprintf(stdout, "\n")
			ui.Summarize(stdout, "", ui.SummaryData{
				Opts:    conf.Options,
				Root:    engine.Executor.GetRunner().GetDefaultGroup(),
				Metrics: engine.Metrics,
				Time:    engine.Executor.GetTime(),
			})
			fprintf(stdout, "\n")
		}

		if conf.Linger.Bool {
			log.Info("Linger set; waiting for Ctrl+C...")
			<-sigC
		}

		if engine.IsTainted() {
			return ExitCode{errors.New("some thresholds have failed"), 99}
		}
		return nil
	},
}

func init() {
	RootCmd.AddCommand(runCmd)

	runCmd.Flags().SortFlags = false
	runCmd.Flags().AddFlagSet(optionFlagSet())
	runCmd.Flags().AddFlagSet(runtimeOptionFlagSet(true))
	runCmd.Flags().AddFlagSet(configFlagSet())
	runCmd.Flags().StringVarP(&runType, "type", "t", runType, "override file `type`, \"js\" or \"archive\"")
	runCmd.Flags().BoolVar(&runNoSetup, "no-setup", runNoSetup, "don't run setup()")
	runCmd.Flags().BoolVar(&runNoTeardown, "no-teardown", runNoTeardown, "don't run teardown()")
}

// Reads a source file from any supported destination.
func readSource(src, pwd string, fs afero.Fs, stdin io.Reader) (*lib.SourceData, error) {
	if src == "-" {
		data, err := ioutil.ReadAll(stdin)
		if err != nil {
			return nil, err
		}
		return &lib.SourceData{Filename: "-", Data: data}, nil
	}
	abspath := filepath.Join(pwd, src)
	if ok, _ := afero.Exists(fs, abspath); ok {
		src = abspath
	}
	return loader.Load(fs, pwd, src)
}

// Creates a new runner.
func newRunner(src *lib.SourceData, typ string, fs afero.Fs, rtOpts lib.RuntimeOptions) (lib.Runner, error) {
	switch typ {
	case "":
		return newRunner(src, detectType(src.Data), fs, rtOpts)
	case typeJS:
		return js.New(src, fs, rtOpts)
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
