package main

import (
	"context"
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/api"
	"github.com/loadimpact/speedboat/js"
	"github.com/loadimpact/speedboat/lib"
	"github.com/loadimpact/speedboat/simple"
	"github.com/loadimpact/speedboat/stats"
	"github.com/loadimpact/speedboat/stats/influxdb"
	"gopkg.in/guregu/null.v3"
	"gopkg.in/urfave/cli.v1"
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
		cli.StringFlag{
			Name:  "out, o",
			Usage: "output metrics to an external data store",
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
		cli.Int64Flag{
			Name:  "vus, u",
			Usage: "override vus",
			Value: 10,
		},
		cli.Int64Flag{
			Name:  "max, m",
			Usage: "override vus-max",
		},
		cli.DurationFlag{
			Name:  "duration, d",
			Usage: "override duration",
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

	// Make the Runner
	filename := args[0]
	runnerType := cc.String("type")
	opts := lib.Options{}
	runner, err := makeRunner(filename, runnerType, &opts)
	if err != nil {
		log.WithError(err).Error("Couldn't create a runner")
		return err
	}

	// Collect arguments
	addr := cc.GlobalString("address")
	run := cc.Bool("run")
	quit := cc.Bool("quit")

	duration := cc.Duration("duration")
	if !cc.IsSet("duration") && opts.Duration.Valid {
		d, err := time.ParseDuration(opts.Duration.String)
		if err != nil {
			log.WithError(err).Error("Script exports invalid duration")
			return err
		}
		duration = d
	}

	vus := cc.Int64("vus")
	if !cc.IsSet("vus") && opts.VUs.Valid {
		vus = opts.VUs.Int64
	}

	max := cc.Int64("max")
	if !cc.IsSet("max") {
		if opts.VUsMax.Valid {
			max = opts.VUsMax.Int64
		} else {
			max = vus
		}
	}
	if vus > max {
		return cli.NewExitError(lib.ErrTooManyVUs.Error(), 1)
	}

	out := cc.String("out")

	// Make the metric collector, if requested.
	var collector stats.Collector
	if out != "" {
		c, err := makeCollector(out)
		if err != nil {
			log.WithError(err).Error("Couldn't create output")
			return err
		}
		collector = c
	}

	// Make the Engine
	engine, err := lib.NewEngine(runner)
	if err != nil {
		log.WithError(err).Error("Couldn't create the engine")
		return err
	}
	engineC, engineCancel := context.WithCancel(context.Background())
	engine.Collector = collector
	engine.Stages = []lib.Stage{lib.Stage{Duration: null.IntFrom(int64(duration))}}
	engine.Quit = quit

	// Make the API Server
	srv := &api.Server{
		Engine: engine,
		Info:   lib.Info{Version: cc.App.Version},
	}
	srvC, srvCancel := context.WithCancel(context.Background())

	// Make the Client
	cl, err := api.NewClient(addr)
	if err != nil {
		log.WithError(err).Error("Couldn't make a client; is the address valid?")
		return err
	}

	// Run the engine and API server in the background
	wg.Add(2)
	go func() {
		defer func() {
			log.Debug("Engine terminated")
			wg.Done()
		}()
		log.Debug("Starting engine...")
		if err := engine.Run(engineC); err != nil {
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

	// Wait for the API server to come online
	startTime := time.Now()
	for {
		if err := cl.Ping(); err != nil {
			if time.Since(startTime) < 1*time.Second {
				log.WithError(err).Debug("Waiting for API server to start...")
				time.Sleep(1 * time.Millisecond)
			} else {
				log.WithError(err).Warn("Connection to API server failed; retrying...")
				time.Sleep(1 * time.Second)
			}
			continue
		}
		break
	}

	log.Infof("Starting test - Web UI available at: http://%s/", addr)

	// Start the test with the desired state
	log.WithField("vus", vus).Debug("Configuring test...")
	status := lib.Status{
		Running: null.BoolFrom(run),
		VUs:     null.IntFrom(vus),
		VUsMax:  null.IntFrom(max),
	}
	if _, err := cl.UpdateStatus(status); err != nil {
		log.WithError(err).Error("Couldn't configure test")
	}
	if !run {
		log.Info("Use `speedboat start` to start your test, or pass `--run` to autostart")
	}

	// Wait for a signal or timeout before shutting down
	signals := make(chan os.Signal)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	log.Debug("Waiting for test to finish")
	select {
	case <-srvC.Done():
		log.Debug("API server terminated; shutting down...")
	case <-engineC.Done():
		log.Debug("Engine terminated; shutting down...")
	case sig := <-signals:
		log.WithField("signal", sig).Debug("Signal received; shutting down...")
	}

	// If API server is still available, write final metrics to stdout.
	// (An unavailable API server most likely means a port binding failure.)
	select {
	case <-srvC.Done():
	default:
		metricList, err := cl.Metrics()
		if err != nil {
			log.WithError(err).Error("Couldn't get metrics!")
			break
		}

		// Poor man's object sort.
		metrics := make(map[string]stats.Metric, len(metricList))
		keys := make([]string, len(metricList))
		for i, metric := range metricList {
			metrics[metric.Name] = metric
			keys[i] = metric.Name
		}
		sort.Strings(keys)

		for _, key := range keys {
			val := metrics[key].Humanize()
			if val == "0" {
				continue
			}
			fmt.Printf("%s: %s\n", key, val)
		}
	}

	// Shut down the API server and engine, wait for them to terminate before exiting
	srvCancel()
	engineCancel()
	wg.Wait()

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

	return dumpYAML(opts)
}
