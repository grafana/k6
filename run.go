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

package main

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/ghodss/yaml"
	"github.com/loadimpact/k6/api"
	"github.com/loadimpact/k6/core"
	"github.com/loadimpact/k6/core/local"
	"github.com/loadimpact/k6/js"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/loader"
	"github.com/loadimpact/k6/simple"
	"github.com/loadimpact/k6/stats"
	"github.com/loadimpact/k6/stats/cloud"
	"github.com/loadimpact/k6/stats/influxdb"
	"github.com/loadimpact/k6/stats/json"
	"github.com/loadimpact/k6/ui"
	"github.com/pkg/errors"
	log "github.com/Sirupsen/logrus"
	"github.com/spf13/afero"
	"gopkg.in/guregu/null.v3"
	"gopkg.in/urfave/cli.v1"
)

const (
	TypeAuto    = "auto"
	TypeURL     = "url"
	TypeJS      = "js"
	TypeArchive = "archive"

	CollectorJSON     = "json"
	CollectorInfluxDB = "influxdb"
	CollectorCloud    = "cloud"
)

var urlRegex = regexp.MustCompile(`(?i)^https?://`)

var optionFlags = []cli.Flag{
	cli.Int64Flag{
		Name:  "vus, u",
		Usage: "virtual users to simulate",
		Value: 1,
	},
	cli.Int64Flag{
		Name:  "max, m",
		Usage: "max number of virtual users, if more than --vus",
	},
	cli.DurationFlag{
		Name:  "duration, d",
		Usage: "test duration, 0 to run until cancelled",
	},
	cli.Int64Flag{
		Name:  "iterations, i",
		Usage: "run a set number of iterations, multiplied by VU count",
	},
	cli.StringSliceFlag{
		Name:  "stage, s",
		Usage: "define a test stage, in the format time[:vus] (10s:100)",
	},
	cli.BoolFlag{
		Name:  "paused, p",
		Usage: "start test in a paused state",
	},
	cli.StringFlag{
		Name:  "type, t",
		Usage: "input type, one of: auto, url, js, archive",
		Value: "auto",
	},
	cli.BoolFlag{
		Name:  "linger, l",
		Usage: "linger after test completion",
	},
	cli.Int64Flag{
		Name:  "max-redirects",
		Usage: "follow at most n redirects",
		Value: 10,
	},
	cli.BoolFlag{
		Name:  "insecure-skip-tls-verify",
		Usage: "INSECURE: skip verification of TLS certificates",
	},
	cli.BoolFlag{
		Name:  "no-connection-reuse",
		Usage: "don't reuse connections between VU iterations",
	},
	cli.BoolFlag{
		Name:  "throw, w",
		Usage: "throw errors on failed requests",
	},
	cli.StringSliceFlag{
		Name:  "config, c",
		Usage: "read additional config files",
	},
	cli.BoolFlag{
		Name:   "no-usage-report",
		Usage:  "don't send heartbeat to k6 project on test execution",
		EnvVar: "K6_NO_USAGE_REPORT",
	},
}

var commandRun = cli.Command{
	Name:      "run",
	Usage:     "Starts running a load test",
	ArgsUsage: "url|filename",
	Flags: append(optionFlags,
		cli.BoolFlag{
			Name:  "quiet, q",
			Usage: "hide the progress bar",
		},
		cli.StringFlag{
			Name:   "out, o",
			Usage:  "output metrics to an external data store (format: type=uri)",
			EnvVar: "K6_OUT",
		},
	),
	Action: actionRun,
	Description: `Run starts a load test.

   This is the main entry point to k6, and will do two things:
   
   - Construct an Engine and provide it with a Runner, depending on the first
     argument and the --type flag, which is used to execute the test.
   
   - Start an a web server on the address specified by the global --address
     flag, which serves a web interface and a REST API for remote control.
   
   For ease of use, you may also pass initial status parameters (vus, max,
   duration) to 'run', which will be applied through a normal API call.`,
}

var commandArchive = cli.Command{
	Name:      "archive",
	Usage:     "Archives a test configuration",
	ArgsUsage: "url|filename",
	Flags: append(optionFlags,
		cli.StringFlag{
			Name:  "archive, a",
			Usage: "Filename for the archive",
			Value: "archive.tar",
		},
	),
	Action: actionArchive,
	Description: `

	`,
}

var commandInspect = cli.Command{
	Name:      "inspect",
	Aliases:   []string{"i"},
	Usage:     "Merges and prints test configuration",
	ArgsUsage: "url|filename",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "type, t",
			Usage: "input type, one of: auto, url, js, archive",
			Value: "auto",
		},
	},
	Action: actionInspect,
}

func guessType(data []byte) string {
	// See if it looks like a URL.
	if urlRegex.Match(data) {
		return TypeURL
	}
	// See if it has a valid tar header.
	if _, err := tar.NewReader(bytes.NewReader(data)).Next(); err == nil {
		return TypeArchive
	}
	return TypeJS
}

func getSrcData(filename, pwd string, stdin io.Reader, fs afero.Fs) (*lib.SourceData, error) {
	if filename == "-" {
		data, err := ioutil.ReadAll(stdin)
		if err != nil {
			return nil, err
		}
		return &lib.SourceData{Filename: "-", Data: data}, nil
	}

	abspath := filepath.Join(pwd, filename)
	if ok, _ := afero.Exists(fs, abspath); ok {
		data, err := afero.ReadFile(fs, abspath)
		if err != nil {
			return nil, err
		}
		return &lib.SourceData{Filename: abspath, Data: data}, nil
	}

	return loader.Load(fs, pwd, filename)
}

func makeRunner(runnerType string, src *lib.SourceData, fs afero.Fs) (lib.Runner, error) {
	switch runnerType {
	case TypeAuto:
		return makeRunner(guessType(src.Data), src, fs)
	case TypeURL:
		u, err := url.Parse(strings.TrimSpace(string(src.Data)))
		if err != nil || u.Scheme == "" {
			return nil, errors.New("Failed to parse URL")
		}
		r, err := simple.New(u)
		if err != nil {
			return nil, err
		}
		return r, err
	case TypeJS:
		return js.New(src, fs)
	case TypeArchive:
		arc, err := lib.ReadArchive(bytes.NewReader(src.Data))
		if err != nil {
			return nil, err
		}
		switch arc.Type {
		case TypeJS:
			return js.NewFromArchive(arc)
		default:
			return nil, errors.Errorf("Invalid archive - unrecognized type: '%s'", arc.Type)
		}
	default:
		return nil, errors.New("Invalid type specified, see --help")
	}
}

func splitCollectorString(s string) (string, string) {
	parts := strings.SplitN(s, "=", 2)
	if len(parts) != 2 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}

func makeCollector(s string, conf Config, src *lib.SourceData, opts lib.Options, version string) (lib.Collector, error) {
	t, p := splitCollectorString(s)
	switch t {
	case CollectorInfluxDB:
		return influxdb.New(p, conf.Collectors.Get(t), opts)
	case CollectorJSON:
		return json.New(p, afero.NewOsFs(), opts)
	case CollectorCloud:
		return cloud.New(p, src, opts, version)
	default:
		return nil, errors.New("Unknown output type: " + t)
	}
}

func collectorOfType(t string) lib.Collector {
	switch t {
	case CollectorInfluxDB:
		return &influxdb.Collector{}
	case CollectorJSON:
		return &json.Collector{}
	case CollectorCloud:
		return &json.Collector{}
	default:
		return nil
	}
}

func getOptions(cc *cli.Context) (lib.Options, error) {
	var err error
	opts := lib.Options{
		Paused:                cliBool(cc, "paused"),
		VUs:                   cliInt64(cc, "vus"),
		VUsMax:                cliInt64(cc, "max"),
		Duration:              cliDuration(cc, "duration", &err),
		Iterations:            cliInt64(cc, "iterations"),
		Linger:                cliBool(cc, "linger"),
		MaxRedirects:          cliInt64(cc, "max-redirects"),
		InsecureSkipTLSVerify: cliBool(cc, "insecure-skip-tls-verify"),
		NoConnectionReuse:     cliBool(cc, "no-connection-reuse"),
		Throw:                 cliBool(cc, "throw"),
		NoUsageReport:         cliBool(cc, "no-usage-report"),
	}
	for _, s := range cc.StringSlice("stage") {
		stage, err := ParseStage(s)
		if err != nil {
			log.WithError(err).Error("Invalid stage specified")
			return opts, err
		}
		opts.Stages = append(opts.Stages, stage)
	}
	return opts, nil
}

func finalizeOptions(opts lib.Options) lib.Options {
	// If VUsMax is unspecified, default to either VUs or the highest Stage Target.
	if !opts.VUsMax.Valid {
		opts.VUsMax.Int64 = opts.VUs.Int64
		if len(opts.Stages) > 0 {
			for _, stage := range opts.Stages {
				if stage.Target.Valid && stage.Target.Int64 > opts.VUsMax.Int64 {
					opts.VUsMax = stage.Target
				}
			}
		}
	}

	// Default to 1 iteration if duration and stages are unspecified.
	if !opts.Duration.Valid && !opts.Iterations.Valid && len(opts.Stages) == 0 {
		opts.Iterations = null.IntFrom(1)
	}

	return opts
}

func readConfigFiles(cc *cli.Context, fs afero.Fs) (lib.Options, error) {
	var opts lib.Options
	for _, filename := range cc.StringSlice("config") {
		data, err := afero.ReadFile(fs, filename)
		if err != nil {
			return opts, err
		}

		var configOpts lib.Options
		if err := yaml.Unmarshal(data, &configOpts); err != nil {
			return opts, err
		}
		opts = opts.Apply(configOpts)
	}
	return opts, nil
}

func actionRun(cc *cli.Context) error {
	wg := sync.WaitGroup{}

	args := cc.Args()
	if len(args) != 1 {
		return cli.NewExitError("Wrong number of arguments!", 1)
	}

	pwd, err := os.Getwd()
	if err != nil {
		pwd = "/"
	}

	// Read the config file.
	conf, err := LoadConfig()
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	// Collect CLI arguments, most (not all) relating to options.
	addr := cc.GlobalString("address")
	out := cc.String("out")
	quiet := cc.Bool("quiet")
	cliOpts, err := getOptions(cc)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	opts := cliOpts

	// Make the Runner, extract script-defined options.
	arg := args[0]
	fs := afero.NewOsFs()
	src, err := getSrcData(arg, pwd, os.Stdin, fs)
	if err != nil {
		log.WithError(err).Error("Failed to parse input data")
		return err
	}
	runnerType := cc.String("type")
	if runnerType == TypeAuto {
		runnerType = guessType(src.Data)
	}
	runner, err := makeRunner(runnerType, src, fs)
	if err != nil {
		if errstr, ok := err.(fmt.Stringer); ok {
			log.Error(errstr.String())
		} else {
			log.WithError(err).Error("Couldn't create a runner")
		}
		return err
	}

	// Read config files.
	fileOpts, err := readConfigFiles(cc, fs)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	// Combine options in order, apply the final results.
	opts = finalizeOptions(opts.Apply(runner.GetOptions()).Apply(fileOpts).Apply(cliOpts))
	runner.ApplyOptions(opts)

	// Make the metric collector, if requested.
	var collector lib.Collector
	if out != "" {
		c, err := makeCollector(out, conf, src, opts, cc.App.Version)
		if err != nil {
			log.WithError(err).Error("Couldn't create output")
			return err
		}
		collector = c
	}

	fmt.Fprintln(color.Output, "")

	color.Cyan(`          /\      |‾‾|  /‾‾/  /‾/   `)
	color.Cyan(`     /\  /  \     |  |_/  /  / /   `)
	color.Cyan(`    /  \/    \    |      |  /  ‾‾\  `)
	color.Cyan(`   /          \   |  |‾\  \ | (_) | `)
	color.Cyan(`  / __________ \  |__|  \__\ \___/  Welcome to k6 v%s!`, cc.App.Version)

	collectorString := "-"
	if collector != nil {
		if err := collector.Init(); err != nil {
			return cli.NewExitError(err, 1)
		}
		collectorString = fmt.Sprint(collector)
	}

	fmt.Fprintln(color.Output, "")

	fmt.Fprintf(color.Output, "  execution: %s\n", color.CyanString("local"))
	fmt.Fprintf(color.Output, "     output: %s\n", color.CyanString(collectorString))
	fmt.Fprintf(color.Output, "     script: %s (%s)\n", color.CyanString(src.Filename), color.CyanString(runnerType))
	fmt.Fprintf(color.Output, "\n")
	fmt.Fprintf(color.Output, "   duration: %s, iterations: %s\n", color.CyanString(opts.Duration.String()), color.CyanString("%d", opts.Iterations.Int64))
	fmt.Fprintf(color.Output, "        vus: %s, max: %s\n", color.CyanString("%d", opts.VUs.Int64), color.CyanString("%d", opts.VUsMax.Int64))
	fmt.Fprintf(color.Output, "\n")
	fmt.Fprintf(color.Output, "    web ui: %s\n", color.CyanString("http://%s/", addr))
	fmt.Fprintf(color.Output, "\n")

	// Make the Engine
	engine, err := core.NewEngine(local.New(runner), opts)
	if err != nil {
		log.WithError(err).Error("Couldn't create the engine")
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	engine.Collector = collector

	// Send usage report, if we're allowed to
	if opts.NoUsageReport.Valid && !opts.NoUsageReport.Bool {
		go func() {
			conn, err := net.Dial("udp", "k6reports.loadimpact.com:6565")
			if err == nil {
				// This is a best-effort attempt to send a usage report. We don't want
				// to inconvenience users if this doesn't work, for whatever reason
				_, _ = conn.Write([]byte("nyoom"))
				_ = conn.Close()
			}
		}()
	}

	// Run the engine.
	wg.Add(1)
	go func() {
		defer func() {
			log.Debug("Engine terminated")
			wg.Done()
		}()
		log.Debug("Starting engine...")
		if err := engine.Run(ctx); err != nil {
			log.WithError(err).Error("Engine Error")
		}
		cancel()
	}()

	// Start the API server in the background.
	go func() {
		if err := api.ListenAndServe(addr, engine); err != nil {
			log.WithError(err).Error("Couldn't start API server!")
		}
	}()

	// Progress bar for TTYs.
	progressBar := ui.ProgressBar{Width: 60}
	if isTTY && !quiet {
		fmt.Fprintf(color.Output, " starting %s -- / --\r", progressBar.String())
	}

	// Wait for a signal or timeout before shutting down
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	// Print status at a set interval; less frequently on non-TTYs.
	tickInterval := 10 * time.Millisecond
	if !isTTY || quiet {
		tickInterval = 1 * time.Second
	}
	ticker := time.NewTicker(tickInterval)

loop:
	for {
		select {
		case <-ticker.C:
			if !engine.Executor.IsRunning() {
				break loop
			}

			statusString := "running"
			if engine.Executor.IsPaused() {
				statusString = "paused"
			}

			atTime := engine.Executor.GetTime()
			endTime := engine.Executor.GetEndTime()
			progress := 0.0
			if endTime.Valid {
				progress = float64(atTime) / float64(endTime.Duration)
			}

			if isTTY && !quiet {
				progressBar.Progress = progress
				fmt.Fprintf(color.Output, "%10s %s %10s / %s\r",
					statusString,
					progressBar.String(),
					roundDuration(atTime, 100*time.Millisecond),
					roundDuration(time.Duration(endTime.Duration), 100*time.Millisecond),
				)
			} else {
				fmt.Fprintf(color.Output, "[%-10s] %s / %s\n",
					statusString,
					roundDuration(atTime, 100*time.Millisecond),
					roundDuration(time.Duration(endTime.Duration), 100*time.Millisecond),
				)
			}
		case <-ctx.Done():
			log.Debug("Engine terminated; shutting down...")
			break loop
		case sig := <-signals:
			log.WithField("signal", sig).Debug("Signal received; shutting down...")
			break loop
		}
	}

	// Shut down the API server and engine.
	cancel()
	wg.Wait()

	// Test done, leave that status as the final progress bar!
	atTime := engine.Executor.GetTime()
	if isTTY && !quiet {
		progressBar.Progress = 1.0
		fmt.Fprintf(color.Output, "      done %s %10s / %s\n",
			progressBar.String(),
			roundDuration(atTime, 100*time.Millisecond),
			roundDuration(atTime, 100*time.Millisecond),
		)
	} else {
		fmt.Fprintf(color.Output, "[%-10s] %s / %s\n",
			"done",
			roundDuration(atTime, 100*time.Millisecond),
			roundDuration(atTime, 100*time.Millisecond),
		)
	}
	fmt.Fprintf(color.Output, "\n")

	// Print groups.
	var printGroup func(g *lib.Group, level int)
	printGroup = func(g *lib.Group, level int) {
		indent := strings.Repeat("  ", level)

		if g.Name != "" && g.Parent != nil {
			fmt.Fprintf(color.Output, "%s█ %s\n", indent, g.Name)
		}

		if len(g.Checks) > 0 {
			if g.Name != "" && g.Parent != nil {
				fmt.Fprintf(color.Output, "\n")
			}
			for _, check := range g.Checks {
				icon := "✓"
				statusColor := color.GreenString
				isCheckFailure := check.Fails > 0

				if isCheckFailure {
					icon = "✗"
					statusColor = color.RedString
				}

				fmt.Fprint(color.Output, statusColor("%s  %s %s\n",
					indent,
					icon,
					check.Name,
				))

				if isCheckFailure {
					fmt.Fprint(color.Output, statusColor("%s        %2.2f%% (%v/%v) \n",
						indent,
						100*(float64(check.Fails)/float64(check.Passes+check.Fails)),
						check.Fails,
						check.Passes+check.Fails,
					))
				}

			}
			fmt.Fprintf(color.Output, "\n")
		}
		if len(g.Groups) > 0 {
			if g.Name != "" && g.Parent != nil && len(g.Checks) > 0 {
				fmt.Fprintf(color.Output, "\n")
			}
			for _, g := range g.Groups {
				printGroup(g, level+1)
			}
		}
	}

	printGroup(engine.Executor.GetRunner().GetDefaultGroup(), 1)

	// Sort and print metrics.
	metricNames := make([]string, 0, len(engine.Metrics))
	metricNameWidth := 0
	for _, m := range engine.Metrics {
		metricNames = append(metricNames, m.Name)
		if l := len(m.Name); l > metricNameWidth {
			metricNameWidth = l
		}
	}
	sort.Strings(metricNames)

	for _, name := range metricNames {
		m := engine.Metrics[name]
		sample := m.Sink.Format()

		keys := make([]string, 0, len(sample))
		for k := range sample {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var val string
		switch len(keys) {
		case 0:
			continue
		case 1:
			for _, k := range keys {
				val = color.CyanString(m.HumanizeValue(sample[k]))
				if atTime > 1*time.Second && m.Type == stats.Counter && m.Contains != stats.Time {
					perS := m.HumanizeValue(sample[k] / float64(atTime/time.Second))
					val += " " + color.New(color.Faint, color.FgCyan).Sprintf("(%s/s)", perS)
				}
			}
		default:
			var parts []string
			for _, k := range keys {
				parts = append(parts, fmt.Sprintf("%s=%s", k, color.CyanString(m.HumanizeValue(sample[k]))))
			}
			val = strings.Join(parts, " ")
		}
		if val == "0" {
			continue
		}

		icon := " "
		if m.Tainted.Valid {
			if !m.Tainted.Bool {
				icon = color.GreenString("✓")
			} else {
				icon = color.RedString("✗")
			}
		}

		namePadding := strings.Repeat(".", metricNameWidth-len(name)+3)
		fmt.Fprintf(color.Output, "  %s %s%s %s\n",
			icon,
			name,
			color.New(color.Faint).Sprint(namePadding+":"),
			val,
		)
	}

	if opts.Linger.Bool {
		<-signals
	}

	if engine.IsTainted() {
		return cli.NewExitError("", 99)
	}
	return nil
}

func actionArchive(cc *cli.Context) error {
	args := cc.Args()
	if len(args) != 1 {
		return cli.NewExitError("Wrong number of arguments!", 1)
	}
	arg := args[0]

	pwd, err := os.Getwd()
	if err != nil {
		pwd = "/"
	}

	cliOpts, err := getOptions(cc)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	opts := cliOpts

	fs := afero.NewOsFs()
	src, err := getSrcData(arg, pwd, os.Stdin, fs)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	runnerType := cc.String("type")
	if runnerType == TypeAuto {
		runnerType = guessType(src.Data)
	}

	r, err := makeRunner(runnerType, src, fs)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	fileOpts, err := readConfigFiles(cc, fs)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	opts = finalizeOptions(opts.Apply(r.GetOptions()).Apply(fileOpts).Apply(cliOpts))
	r.ApplyOptions(opts)

	f, err := os.Create(cc.String("archive"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	defer func() { _ = f.Close() }()

	arc := r.MakeArchive()
	if err := arc.Write(f); err != nil {
		return cli.NewExitError(err, 1)
	}
	return nil
}

func actionInspect(cc *cli.Context) error {
	args := cc.Args()
	if len(args) != 1 {
		return cli.NewExitError("Wrong number of arguments!", 1)
	}
	arg := args[0]

	pwd, err := os.Getwd()
	if err != nil {
		pwd = "/"
	}

	fs := afero.NewOsFs()
	src, err := getSrcData(arg, pwd, os.Stdin, fs)
	if err != nil {
		return err
	}
	runnerType := cc.String("type")
	if runnerType == TypeAuto {
		runnerType = guessType(src.Data)
	}

	var opts lib.Options

	switch runnerType {
	case TypeJS:
		r, err := js.NewBundle(src, fs)
		if err != nil {
			return cli.NewExitError(err.Error(), 1)
		}
		opts = opts.Apply(r.Options)
	}

	return dumpYAML(opts)
}
