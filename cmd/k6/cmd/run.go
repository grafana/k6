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
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/loadimpact/k6/api"
	"github.com/loadimpact/k6/core"
	"github.com/loadimpact/k6/core/local"
	"github.com/loadimpact/k6/js"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/loader"
	"github.com/loadimpact/k6/ui"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	null "gopkg.in/guregu/null.v3"
)

const (
	typeJS      = "js"
	typeArchive = "archive"
)

var (
	runType       string
	linger        bool
	noUsageReport bool
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
  k6 run -u 0 -s 10s:100 -s 60s -s 10s:0`[1:],
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, _ = BannerColor.Fprint(stdout, Banner+"\n\n")

		initBar := ui.ProgressBar{
			Width: 60,
			Left:  func() string { return "    init" },
		}

		// Create the Runner.
		fmt.Fprintf(stdout, "%s runner\r", initBar.String())
		pwd, err := os.Getwd()
		if err != nil {
			return err
		}
		filename := args[0]
		src, err := readSource(filename, pwd, afero.NewOsFs(), os.Stdin)
		if err != nil {
			return err
		}
		r, err := newRunner(src, runType, afero.NewOsFs())
		if err != nil {
			return err
		}

		fmt.Fprintf(stdout, "  execution: %s\n", ui.ValueColor.Sprint("local"))
		fmt.Fprintf(stdout, "     output: %s\n", ui.ValueColor.Sprint("-"))
		fmt.Fprintf(stdout, "     script: %s\n", ui.ValueColor.Sprint(filename))
		fmt.Fprintf(stdout, "\n")

		// Assemble options; start with the CLI-provided options to get shadowed (non-Valid)
		// defaults in there, override with Runner-provided ones, then merge the CLI opts in
		// on top to give them priority.
		fmt.Fprintf(stdout, "%s options\r", initBar.String())
		cliOpts, err := getOptions(cmd.Flags())
		if err != nil {
			return err
		}
		opts := cliOpts.Apply(r.GetOptions()).Apply(cliOpts)

		// If -m/--max isn't specified, figure out the max that should be needed.
		if !opts.VUsMax.Valid {
			opts.VUsMax = null.IntFrom(opts.VUs.Int64)
			for _, stage := range opts.Stages {
				if stage.Target.Valid && stage.Target.Int64 > opts.VUsMax.Int64 {
					opts.VUsMax = stage.Target
				}
			}
		}
		// If -d/--duration, -i/--iterations and -s/--stage are all unset, run to one iteration.
		if !opts.Duration.Valid && !opts.Iterations.Valid && opts.Stages == nil {
			opts.Iterations = null.IntFrom(1)
		}

		// Write options back to the runner too.
		r.ApplyOptions(opts)

		// Write the options banner.
		{
			duration := ui.GrayColor.Sprint("-")
			iterations := ui.GrayColor.Sprint("-")
			if opts.Duration.Valid {
				duration = ui.ValueColor.Sprint(opts.Duration.Duration)
			}
			if opts.Iterations.Valid {
				iterations = ui.ValueColor.Sprint(opts.Iterations.Int64)
			}
			vus := ui.ValueColor.Sprint(opts.VUs.Int64)
			max := ui.ValueColor.Sprint(opts.VUsMax.Int64)

			leftWidth := ui.StrWidth(duration)
			if l := ui.StrWidth(vus); l > leftWidth {
				leftWidth = l
			}
			durationPad := strings.Repeat(" ", leftWidth-ui.StrWidth(duration))
			vusPad := strings.Repeat(" ", leftWidth-ui.StrWidth(vus))

			fmt.Fprintf(stdout, "    duration: %s,%s iterations: %s\n", duration, durationPad, iterations)
			fmt.Fprintf(stdout, "         vus: %s,%s max: %s\n", vus, vusPad, max)
			fmt.Fprintf(stdout, "\n")
		}

		// Create an engine with a local executor, wrapping the Runner.
		fmt.Fprintf(stdout, "%s   engine\r", initBar.String())
		engine, err := core.NewEngine(local.New(r), opts)
		if err != nil {
			return err
		}

		// Create an API server.
		fmt.Fprintf(stdout, "%s   server\r", initBar.String())
		go func() {
			if err := api.ListenAndServe(address, engine); err != nil {
				log.WithError(err).Warn("Error from API server")
			}
		}()

		// Run the engine with a cancellable context.
		fmt.Fprintf(stdout, "%s starting\r", initBar.String())
		ctx, cancel := context.WithCancel(context.Background())
		errC := make(chan error)
		go func() { errC <- engine.Run(ctx) }()

		// Trap Interrupts, SIGINTs and SIGTERMs.
		sigC := make(chan os.Signal, 1)
		signal.Notify(sigC, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(sigC)

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
				if endT := engine.Executor.GetEndTime(); endT.Valid {
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
		if quiet {
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
				} else if endT := engine.Executor.GetEndTime(); endT.Valid {
					prog = float64(engine.Executor.GetTime()) / float64(endT.Duration)
				}
				progress.Progress = prog
				fmt.Fprintf(stdout, "%s\x1b[0K\r", progress.String())
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
			fmt.Fprintf(stdout, "%s\x1b[0K\n", progress.String())
		}

		// Print the end-of-test summary.
		if !quiet {
			fmt.Fprintf(stdout, "\n")
			ui.Summarize(stdout, "", ui.SummaryData{
				Opts:    opts,
				Root:    engine.Executor.GetRunner().GetDefaultGroup(),
				Metrics: engine.Metrics,
				Time:    engine.Executor.GetTime(),
			})
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(runCmd)

	runCmd.Flags().SortFlags = false
	registerOptions(runCmd.Flags())

	flags := pflag.NewFlagSet("", 0)
	flags.SortFlags = false
	flags.StringVarP(&runType, "type", "t", "", "override file `type`, \"js\" or \"archive\"")
	flags.BoolVarP(&linger, "linger", "l", false, "keep the API server alive past test end")
	flags.BoolVar(&noUsageReport, "no-usage-report", false, "don't send analytics to the maintainers")
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
func newRunner(src *lib.SourceData, typ string, fs afero.Fs) (lib.Runner, error) {
	switch typ {
	case "":
		if _, err := tar.NewReader(bytes.NewReader(src.Data)).Next(); err == nil {
			return newRunner(src, typeArchive, fs)
		}
		return newRunner(src, typeJS, fs)
	case typeJS:
		return js.New(src, fs)
	case typeArchive:
		arc, err := lib.ReadArchive(bytes.NewReader(src.Data))
		if err != nil {
			return nil, err
		}
		switch arc.Type {
		case typeJS:
			return js.NewFromArchive(arc)
		default:
			return nil, errors.Errorf("archive requests unsupported runner: %s", arc.Type)
		}
	default:
		return nil, errors.Errorf("unknown -t/--type: %s", typ)
	}
}
