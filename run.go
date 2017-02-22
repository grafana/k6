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
	"context"
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/fatih/color"
	"github.com/ghodss/yaml"
	"github.com/loadimpact/k6/api"
	"github.com/loadimpact/k6/js"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/simple"
	"github.com/loadimpact/k6/stats"
	"github.com/loadimpact/k6/stats/influxdb"
	"github.com/loadimpact/k6/stats/json"
	"github.com/loadimpact/k6/ui"
	"gopkg.in/guregu/null.v3"
	"gopkg.in/urfave/cli.v1"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	TypeAuto = "auto"
	TypeURL  = "url"
	TypeJS   = "js"
)

var commandRun = cli.Command{
	Name:      "run",
	Usage:     "Starts running a load test",
	ArgsUsage: "url|filename",
	Flags: []cli.Flag{
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
		cli.BoolFlag{
			Name:  "paused, p",
			Usage: "start test in a paused state",
		},
		cli.StringFlag{
			Name:  "type, t",
			Usage: "input type, one of: auto, url, js",
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
		cli.StringFlag{
			Name:  "out, o",
			Usage: "output metrics to an external data store",
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
	},
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

var commandInspect = cli.Command{
	Name:      "inspect",
	Aliases:   []string{"i"},
	Usage:     "Merges and prints test configuration",
	ArgsUsage: "url|filename",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "type, t",
			Usage: "input type, one of: auto, url, js",
			Value: "auto",
		},
		cli.StringSliceFlag{
			Name:  "config, c",
			Usage: "read additional config files",
		},
	},
	Action: actionInspect,
}

func looksLikeURL(str []byte) bool {
	s := strings.ToLower(strings.TrimSpace(string(str)))
	match, _ := regexp.MatchString("^https?://", s)
	return match
}

func getSrcData(arg, t string) (*lib.SourceData, string, error) {
	srcdata := []byte("")
	runnerType := t
	filename := arg
	const cmdline = "[cmdline]"
	// special case name "-" will always cause src data to be read from file STDIN
	if arg == "-" {
		s, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return nil, "", err
		}
		srcdata = s
	} else {
		// Deduce how to get src data
		switch t {
		case TypeAuto:
			if looksLikeURL([]byte(arg)) { // always try to parse as URL string first
				srcdata = []byte(arg)
				runnerType = TypeURL
				filename = cmdline
			} else {
				// Otherwise, check if it is a file name and we can load the file
				s, err := ioutil.ReadFile(arg)
				srcdata = s
				if err != nil { // if we fail to open file, we assume the arg is JS code
					srcdata = []byte(arg)
					runnerType = TypeJS
					filename = cmdline
				}
			}
		case TypeURL:
			// We try to use TypeURL args as URLs directly first
			if looksLikeURL([]byte(arg)) {
				srcdata = []byte(arg)
				filename = cmdline
			} else { // if that didn’t work, we try to load a file with URLs
				s, err := ioutil.ReadFile(arg)
				if err != nil {
					return nil, "", err
				}
				srcdata = s
			}
		case TypeJS:
			// TypeJS args we try to use as file names first
			s, err := ioutil.ReadFile(arg)
			srcdata = s
			if err != nil { // and if that didn’t work, we assume the arg itself is JS code
				srcdata = []byte(arg)
				filename = cmdline
			}
		default:
			return nil, "", errors.New("Invalid type specified, see --help")
		}
	}
	// Now we should have some src data and in most cases a type also. If we
	// don’t have a type it means we read from STDIN or from a file and the user
	// specified type == TypeAuto. This means we need to try and auto-detect type
	if runnerType == TypeAuto {
		if looksLikeURL(srcdata) {
			runnerType = TypeURL
		} else {
			runnerType = TypeJS
		}
	}
	src := &lib.SourceData{
		Data:     srcdata,
		Filename: filename,
	}
	return src, runnerType, nil
}

func makeRunner(runnerType string, srcdata *lib.SourceData) (lib.Runner, error) {
	switch runnerType {
	case "":
		return nil, errors.New("Invalid type specified, see --help")
	case TypeURL:
		u, err := url.Parse(strings.TrimSpace(string(srcdata.Data)))
		if err != nil || u.Scheme == "" {
			return nil, errors.New("Failed to parse URL")
		}
		r, err := simple.New(u)
		if err != nil {
			return nil, err
		}
		return r, err
	case TypeJS:
		rt, err := js.New()
		if err != nil {
			return nil, err
		}
		exports, err := rt.Load(srcdata)
		if err != nil {
			return nil, err
		}
		r, err := js.NewRunner(rt, exports)
		if err != nil {
			return nil, err
		}
		return r, nil
	default:
		return nil, errors.New("Invalid type specified, see --help")
	}
}

func parseCollectorString(s string) (t, p string, err error) {
	parts := strings.SplitN(s, "=", 2)
	if len(parts) != 2 {
		return "", "", errors.New("Malformed output; must be in the form 'type=url'")
	}

	return parts[0], parts[1], nil
}

func makeCollector(s string, opts lib.Options) (lib.Collector, error) {
	t, p, err := parseCollectorString(s)
	if err != nil {
		return nil, err
	}

	switch t {
	case "influxdb":
		return influxdb.New(p, opts)
	case "json":
		return json.New(p, opts)
	default:
		return nil, errors.New("Unknown output type: " + t)
	}
}

func actionRun(cc *cli.Context) error {
	red := color.New(color.FgRed)
	green := color.New(color.FgGreen)

	wg := sync.WaitGroup{}

	args := cc.Args()
	if len(args) != 1 {
		return cli.NewExitError("Wrong number of arguments!", 1)
	}

	// Collect CLI arguments, most (not all) relating to options.
	addr := cc.GlobalString("address")
	out := cc.String("out")
	cliOpts := lib.Options{
		Paused:                cliBool(cc, "paused"),
		VUs:                   cliInt64(cc, "vus"),
		VUsMax:                cliInt64(cc, "max"),
		Duration:              cliDuration(cc, "duration"),
		Iterations:            cliInt64(cc, "iterations"),
		Linger:                cliBool(cc, "linger"),
		MaxRedirects:          cliInt64(cc, "max-redirects"),
		InsecureSkipTLSVerify: cliBool(cc, "insecure-skip-tls-verify"),
		NoUsageReport:         cliBool(cc, "no-usage-report"),
	}
	opts := cliOpts

	// Make the Runner, extract script-defined options.
	arg := args[0]
	t := cc.String("type")
	srcdata, runnerType, err := getSrcData(arg, t)
	if err != nil {
		log.WithError(err).Error("Failed to parse input data")
		return err
	}
	runner, err := makeRunner(runnerType, srcdata)
	if err != nil {
		log.WithError(err).Error("Couldn't create a runner")
		return err
	}
	opts = opts.Apply(runner.GetOptions())

	// Read config files.
	for _, filename := range cc.StringSlice("config") {
		data, err := ioutil.ReadFile(filename)
		if err != nil {
			return cli.NewExitError(err.Error(), 1)
		}

		var configOpts lib.Options
		if err := yaml.Unmarshal(data, &configOpts); err != nil {
			return cli.NewExitError(err.Error(), 1)
		}
		opts = opts.Apply(configOpts)
	}

	// CLI options override everything.
	opts = opts.Apply(cliOpts)

	// Default to 1 iteration if no duration is specified.
	if !opts.Duration.Valid && !opts.Iterations.Valid {
		opts.Iterations = null.IntFrom(1)
	}

	// Apply defaults.
	opts = opts.SetAllValid(true)

	// Make sure VUsMax defaults to VUs if not specified.
	if opts.VUsMax.Int64 == 0 {
		opts.VUsMax.Int64 = opts.VUs.Int64
	}

	// Update the runner's options.
	runner.ApplyOptions(opts)

	// Make the metric collector, if requested.
	var collector lib.Collector
	collectorString := "-"
	if out != "" {
		c, err := makeCollector(out, opts)
		if err != nil {
			log.WithError(err).Error("Couldn't create output")
			return err
		}
		collector = c
		collectorString = fmt.Sprint(collector)
	}

	fmt.Println("")

	color.Green(`          /\      |‾‾|  /‾‾/  /‾/   `)
	color.Green(`     /\  /  \     |  |_/  /  / /   `)
	color.Green(`    /  \/    \    |   _  |  /  ‾‾\  `)
	color.Green(`   /          \   |  | \  \ | (_) | `)
	color.Green(`  / __________ \  |__|  \__\ \___/  Welcome to k6 v%s!`, cc.App.Version)

	fmt.Println("")

	fmt.Printf("  execution: %s\n", color.CyanString("local"))
	fmt.Printf("     output: %s\n", color.CyanString(collectorString))
	fmt.Printf("     script: %s (%s)\n", color.CyanString(srcdata.Filename), color.CyanString(runnerType))
	fmt.Printf("\n")
	fmt.Printf("   duration: %s, iterations: %s\n", color.CyanString(opts.Duration.String), color.CyanString("%d", opts.Iterations.Int64))
	fmt.Printf("        vus: %s, max: %s\n", color.CyanString("%d", opts.VUs.Int64), color.CyanString("%d", opts.VUsMax.Int64))
	fmt.Printf("\n")
	fmt.Printf("    web ui: %s\n", color.CyanString("http://%s/", addr))
	fmt.Printf("\n")

	// Make the Engine
	engine, err := lib.NewEngine(runner, opts)
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
	if isTTY {
		fmt.Printf(" starting %s -- / --\r", progressBar.String())
	}

	// Wait for a signal or timeout before shutting down
	signals := make(chan os.Signal)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	// Print status at a set interval; less frequently on non-TTYs.
	tickInterval := 10 * time.Millisecond
	if !isTTY {
		tickInterval = 1 * time.Second
	}
	ticker := time.NewTicker(tickInterval)

loop:
	for {
		select {
		case <-ticker.C:
			if !engine.IsRunning() {
				break loop
			}

			statusString := "running"
			if engine.IsPaused() {
				statusString = "paused"
			}

			atTime := engine.AtTime()
			totalTime := engine.TotalTime()
			progress := 0.0
			if totalTime > 0 {
				progress = float64(atTime) / float64(totalTime)
			}

			if isTTY {
				progressBar.Progress = progress
				fmt.Printf("%10s %s %10s / %s\r",
					statusString,
					progressBar.String(),
					roundDuration(atTime, 100*time.Millisecond),
					roundDuration(totalTime, 100*time.Millisecond),
				)
			} else {
				fmt.Printf("[%-10s] %s / %s\n",
					statusString,
					roundDuration(atTime, 100*time.Millisecond),
					roundDuration(totalTime, 100*time.Millisecond),
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
	atTime := engine.AtTime()
	if isTTY {
		progressBar.Progress = 1.0
		fmt.Printf("      done %s %10s / %s\n",
			progressBar.String(),
			roundDuration(atTime, 100*time.Millisecond),
			roundDuration(atTime, 100*time.Millisecond),
		)
	} else {
		fmt.Printf("[%-10s] %s / %s\n",
			"done",
			roundDuration(atTime, 100*time.Millisecond),
			roundDuration(atTime, 100*time.Millisecond),
		)
	}
	fmt.Printf("\n")

	// Print groups.
	var printGroup func(g *lib.Group, level int)
	printGroup = func(g *lib.Group, level int) {
		indent := strings.Repeat("  ", level)

		if g.Name != "" && g.Parent != nil {
			fmt.Printf("%s█ %s\n", indent, g.Name)
		}

		if len(g.Checks) > 0 {
			if g.Name != "" && g.Parent != nil {
				fmt.Printf("\n")
			}
			for _, check := range g.Checks {
				icon := "✓"
				statusColor := green
				if check.Fails > 0 {
					icon = "✗"
					statusColor = red
				}
				statusColor.Printf("%s  %s %2.2f%% - %s\n",
					indent,
					icon,
					100*(float64(check.Passes)/float64(check.Passes+check.Fails)),
					check.Name,
				)
			}
			fmt.Printf("\n")
		}
		if len(g.Groups) > 0 {
			if g.Name != "" && g.Parent != nil && len(g.Checks) > 0 {
				fmt.Printf("\n")
			}
			for _, g := range g.Groups {
				printGroup(g, level+1)
			}
		}
	}

	printGroup(engine.Runner.GetDefaultGroup(), 1)

	// Sort and print metrics.
	metrics := make(map[string]*stats.Metric, len(engine.Metrics))
	metricNames := make([]string, 0, len(engine.Metrics))
	metricNameWidth := 0
	for m := range engine.Metrics {
		metrics[m.Name] = m
		metricNames = append(metricNames, m.Name)
		if l := len(m.Name); l > metricNameWidth {
			metricNameWidth = l
		}
	}
	sort.Strings(metricNames)

	for _, name := range metricNames {
		m := metrics[name]
		m.Sample = engine.Metrics[m].Format()
		val := metrics[name].Humanize()
		if val == "0" {
			continue
		}
		icon := " "
		if m.Tainted.Valid {
			if !m.Tainted.Bool {
				icon = green.Sprint("✓")
			} else {
				icon = red.Sprint("✗")
			}
		}

		// Hack some color in there.
		parts := strings.Split(val, ", ")
		newParts := make([]string, len(parts))
		for i, part := range parts {
			kv := strings.SplitN(part, "=", 2)
			switch len(kv) {
			case 1:
				newParts[i] = color.CyanString(kv[0])
			case 2:
				newParts[i] = fmt.Sprintf(
					"%s%s",
					color.New(color.Reset).Sprint(kv[0]+"="),
					color.New(color.FgCyan).Sprint(kv[1]),
				)
			}
		}
		val = strings.Join(newParts, ", ")

		namePadding := strings.Repeat(".", metricNameWidth-len(name)+3)
		fmt.Printf("  %s %s%s %s\n",
			icon,
			name,
			color.New(color.Faint).Sprint(namePadding+":"),
			color.New(color.FgCyan).Sprint(val),
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

func actionInspect(cc *cli.Context) error {
	args := cc.Args()
	if len(args) != 1 {
		return cli.NewExitError("Wrong number of arguments!", 1)
	}
	arg := args[0]
	t := cc.String("type")
	srcdata, runnerType, err := getSrcData(arg, t)
	if err != nil {
		return err
	}

	var opts lib.Options
	switch runnerType {
	case TypeJS:
		r, err := js.New()
		if err != nil {
			return cli.NewExitError(err.Error(), 1)
		}

		if _, err := r.Load(srcdata); err != nil {
			return cli.NewExitError(err.Error(), 1)
		}
		opts = opts.Apply(r.Options)
	}

	for _, filename := range cc.StringSlice("config") {
		data, err := ioutil.ReadFile(filename)
		if err != nil {
			return cli.NewExitError(err.Error(), 1)
		}

		var configOpts lib.Options
		if err := yaml.Unmarshal(data, &configOpts); err != nil {
			return cli.NewExitError(err.Error(), 1)
		}
		opts = opts.Apply(configOpts)
	}

	return dumpYAML(opts)
}
