package main

import (
	"context"
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/ghodss/yaml"
	"github.com/loadimpact/speedboat/api"
	"github.com/loadimpact/speedboat/js"
	"github.com/loadimpact/speedboat/lib"
	"github.com/loadimpact/speedboat/simple"
	"github.com/loadimpact/speedboat/stats"
	"github.com/loadimpact/speedboat/stats/influxdb"
	"github.com/loadimpact/speedboat/ui"
	"gopkg.in/urfave/cli.v1"
	"io/ioutil"
	"net/url"
	"os"
	"os/signal"
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

var (
	ErrUnknownType = errors.New("Unable to infer type from argument; specify with -t/--type")
	ErrInvalidType = errors.New("Invalid type specified, see --help")
)

var commandRun = cli.Command{
	Name:      "run",
	Usage:     "Starts running a load test",
	ArgsUsage: "url|filename",
	Flags: []cli.Flag{
		cli.Int64Flag{
			Name:  "vus, u",
			Usage: "virtual users to simulate",
			Value: 10,
		},
		cli.Int64Flag{
			Name:  "max, m",
			Usage: "max number of virtual users, if more than --vus",
		},
		cli.DurationFlag{
			Name:  "duration, d",
			Usage: "test duration, 0 to run until cancelled",
			Value: 10 * time.Second,
		},
		cli.BoolFlag{
			Name:  "run, r",
			Usage: "start test immediately",
		},
		cli.StringFlag{
			Name:  "type, t",
			Usage: "input type, one of: auto, url, js",
			Value: "auto",
		},
		cli.BoolFlag{
			Name:  "quit, q",
			Usage: "quit immediately on test completion",
		},
		cli.BoolFlag{
			Name:  "quit-on-taint",
			Usage: "quit immediately if the test gets tainted",
		},
		cli.StringFlag{
			Name:  "out, o",
			Usage: "output metrics to an external data store",
		},
		cli.StringSliceFlag{
			Name:  "config, c",
			Usage: "read additional config files",
		},
	},
	Action: actionRun,
	Description: `Run starts a load test.

   This is the main entry point to Speedboat, and will do two things:
   
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

func guessType(filename string) string {
	switch {
	case strings.Contains(filename, "://"):
		return TypeURL
	case strings.HasSuffix(filename, ".js"):
		return TypeJS
	default:
		return ""
	}
}

func makeRunner(filename, t string, opts *lib.Options) (lib.Runner, error) {
	if t == TypeAuto {
		t = guessType(filename)
	}

	switch t {
	case "":
		return nil, ErrUnknownType
	case TypeURL:
		return simple.New(filename)
	case TypeJS:
		rt, err := js.New()
		if err != nil {
			return nil, err
		}

		exports, err := rt.Load(filename)
		if err != nil {
			return nil, err
		}

		if err := rt.ExtractOptions(exports, opts); err != nil {
			return nil, err
		}
		return js.NewRunner(rt, exports)
	default:
		return nil, ErrInvalidType
	}
}

func parseCollectorString(s string) (t string, u *url.URL, err error) {
	parts := strings.SplitN(s, "=", 2)
	if len(parts) != 2 {
		return "", nil, errors.New("Malformed output; must be in the form 'type=url'")
	}

	u, err = url.Parse(parts[1])
	if err != nil {
		return "", nil, err
	}

	return parts[0], u, nil
}

func makeCollector(s string) (stats.Collector, error) {
	t, u, err := parseCollectorString(s)
	if err != nil {
		return nil, err
	}

	switch t {
	case "influxdb":
		return influxdb.New(u)
	default:
		return nil, errors.New("Unknown output type: " + t)
	}
}

func actionRun(cc *cli.Context) error {
	wg := sync.WaitGroup{}

	args := cc.Args()
	if len(args) != 1 {
		return cli.NewExitError("Wrong number of arguments!", 1)
	}

	// Collect CLI arguments, most (not all) relating to options.
	addr := cc.GlobalString("address")
	out := cc.String("out")
	opts := lib.Options{
		Run:         cliBool(cc, "run"),
		VUs:         cliInt64(cc, "vus"),
		VUsMax:      cliInt64(cc, "vus-max"),
		Duration:    cliDuration(cc, "duration"),
		Quit:        cliBool(cc, "quit"),
		QuitOnTaint: cliBool(cc, "quit-on-taint"),
	}

	// Make the Runner, extract script-defined options.
	filename := args[0]
	runnerType := cc.String("type")
	runnerOpts := lib.Options{}
	runner, err := makeRunner(filename, runnerType, &runnerOpts)
	if err != nil {
		log.WithError(err).Error("Couldn't create a runner")
		return err
	}
	opts = opts.Apply(runnerOpts)

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

	// CLI options have defaults, which are set as invalid, but have potentially nonzero values.
	// Flipping the Valid flag for all invalid options thus applies all defaults.
	opts = opts.SetAllValid(true)

	// Make the metric collector, if requested.
	var collector stats.Collector
	collectorString := "-"
	if out != "" {
		c, err := makeCollector(out)
		if err != nil {
			log.WithError(err).Error("Couldn't create output")
			return err
		}
		collector = c
		collectorString = fmt.Sprint(collector)
	}

	// Make the Engine
	engine, err := lib.NewEngine(runner)
	if err != nil {
		log.WithError(err).Error("Couldn't create the engine")
		return err
	}
	engineC, engineCancel := context.WithCancel(context.Background())
	engine.Collector = collector

	// Make the API Server
	srv := &api.Server{
		Engine: engine,
		Info:   lib.Info{Version: cc.App.Version},
	}
	srvC, srvCancel := context.WithCancel(context.Background())

	// Run the engine and API server in the background
	wg.Add(2)
	go func() {
		defer func() {
			log.Debug("Engine terminated")
			wg.Done()
		}()
		log.Debug("Starting engine...")
		if err := engine.Run(engineC, opts); err != nil {
			log.WithError(err).Error("Engine Error")
		}
		engineCancel()
	}()
	go func() {
		defer func() {
			log.Debug("API Server terminated")
			wg.Done()
		}()
		log.WithField("addr", addr).Debug("API Server starting...")
		srv.Run(srvC, addr)
		srvCancel()
	}()

	// Print the banner!
	fmt.Printf("Welcome to Speedboat v%s!\n", cc.App.Version)
	fmt.Printf("\n")
	fmt.Printf("  execution: local\n")
	fmt.Printf("     output: %s\n", collectorString)
	fmt.Printf("     script: %s\n", filename)
	fmt.Printf("             ↳ duration: %s\n", opts.Duration.String)
	fmt.Printf("             ↳ vus: %d, max: %d\n", opts.VUs.Int64, opts.VUsMax.Int64)
	fmt.Printf("\n")
	fmt.Printf("  web ui: http://%s/\n", addr)
	fmt.Printf("\n")

	progressBar := ui.ProgressBar{Width: 60}
	fmt.Printf(" starting %s -- / --\r", progressBar.String())

	// Wait for a signal or timeout before shutting down
	signals := make(chan os.Signal)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	ticker := time.NewTicker(10 * time.Millisecond)

loop:
	for {
		select {
		case <-ticker.C:
			statusString := "running"
			if !engine.Status.Running.Bool {
				if engine.IsRunning() {
					statusString = "paused"
				} else {
					statusString = "stopping"
				}
			}

			atTime := time.Duration(engine.Status.AtTime.Int64)
			totalTime, finite := engine.TotalTime()
			progress := 0.0
			if finite {
				progress = float64(atTime) / float64(totalTime)
			}

			progressBar.Progress = progress
			fmt.Printf("%10s %s %10s / %s\r",
				statusString,
				progressBar.String(),
				atTime-(atTime%(100*time.Millisecond)),
				totalTime-(totalTime%(100*time.Millisecond)),
			)
		case <-srvC.Done():
			log.Debug("API server terminated; shutting down...")
			break loop
		case <-engineC.Done():
			log.Debug("Engine terminated; shutting down...")
			break loop
		case sig := <-signals:
			log.WithField("signal", sig).Debug("Signal received; shutting down...")
			break loop
		}
	}

	// Shut down the API server and engine.
	srvCancel()
	engineCancel()
	wg.Wait()

	// Test done, leave that status as the final progress bar!
	atTime := time.Duration(engine.Status.AtTime.Int64)
	progressBar.Progress = 1.0
	fmt.Printf("      done %s %10s / %s\n",
		progressBar.String(),
		atTime-(atTime%(100*time.Millisecond)),
		atTime-(atTime%(100*time.Millisecond)),
	)

	// Sort and print metrics.
	metrics := make(map[string]*stats.Metric, len(engine.Metrics))
	metricNames := make([]string, 0, len(engine.Metrics))
	for m, _ := range engine.Metrics {
		metrics[m.Name] = m
		metricNames = append(metricNames, m.Name)
	}
	sort.Strings(metricNames)

	for _, name := range metricNames {
		m := metrics[name]
		m.Sample = engine.Metrics[m].Format()
		val := metrics[name].Humanize()
		if val == "0" {
			continue
		}
		fmt.Printf("%s: %s\n", name, val)
	}

	if engine.Status.Tainted.Bool {
		return cli.NewExitError("", 99)
	}
	return nil
}

func actionInspect(cc *cli.Context) error {
	args := cc.Args()
	if len(args) != 1 {
		return cli.NewExitError("Wrong number of arguments!", 1)
	}
	filename := args[0]

	t := cc.String("type")
	if t == TypeAuto {
		t = guessType(filename)
	}

	var opts lib.Options
	switch t {
	case TypeJS:
		r, err := js.New()
		if err != nil {
			return cli.NewExitError(err.Error(), 1)
		}

		exports, err := r.Load(filename)
		if err != nil {
			return cli.NewExitError(err.Error(), 1)
		}
		if err := r.ExtractOptions(exports, &opts); err != nil {
			return cli.NewExitError(err.Error(), 1)
		}
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
