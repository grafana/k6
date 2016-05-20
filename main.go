package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/loadimpact/speedboat/aggregate"
	"github.com/loadimpact/speedboat/loadtest"
	"github.com/loadimpact/speedboat/report"
	"github.com/loadimpact/speedboat/runner"
	"github.com/loadimpact/speedboat/runner/js"
	"github.com/loadimpact/speedboat/runner/simple"
	"golang.org/x/net/context"
	"io/ioutil"
	"os"
	"os/signal"
	"path"
	"runtime/debug"
	"time"
)

func makeTest(c *cli.Context) (test loadtest.LoadTest, err error) {
	base := ""
	conf := loadtest.NewConfig()
	if len(c.Args()) > 0 {
		filename := c.Args()[0]
		base = path.Dir(filename)
		data, err := ioutil.ReadFile(filename)
		if err != nil {
			return test, err
		}

		loadtest.ParseConfig(data, &conf)
	}

	if c.IsSet("script") {
		conf.Script = c.String("script")
		base = ""
	}
	if c.IsSet("url") {
		conf.URL = c.String("url")
	}
	if c.IsSet("duration") {
		conf.Duration = c.Duration("duration").String()
	}
	if c.IsSet("vus") {
		conf.VUs = c.Int("vus")
	}

	test, err = conf.Compile()
	if err != nil {
		return test, err
	}

	if test.Script != "" {
		srcb, err := ioutil.ReadFile(path.Join(base, test.Script))
		if err != nil {
			return test, err
		}
		test.Source = string(srcb)
	}

	return test, nil
}

func run(c context.Context, test loadtest.LoadTest, r runner.Runner) (<-chan runner.Result, chan int) {
	ch := make(chan runner.Result)
	scale := make(chan int, 1)

	go func() {
		defer close(ch)

		timeout := time.Duration(0)
		for _, stage := range test.Stages {
			timeout += stage.Duration
		}

		ctx, _ := context.WithTimeout(c, timeout)
		scale <- test.Stages[0].VUs.Start

		for res := range runner.Run(ctx, r, test, scale) {
			ch <- res
		}
	}()

	return ch, scale
}

func action(c *cli.Context) error {
	test, err := makeTest(c)
	if err != nil {
		log.WithError(err).Fatal("Configuration error")
	}

	r := runner.Runner(nil)
	if test.Script != "" {
		ext := path.Ext(test.Script)
		switch ext {
		case ".js":
			r = js.New()
		default:
			log.WithField("ext", ext).Fatal("No runner found")
		}
	} else {
		r = simple.New()
	}

	// Start the pipeline by just running requests
	ctx, cancel := context.WithCancel(context.Background())
	pipeline, scale := run(ctx, test, r)

	// Ramp VUs according to the test definition
	pipeline = runner.Ramp(&test, scale, pipeline)

	// Stick result aggregation onto it
	stats := aggregate.Stats{}
	stats.Time.Values = make([]time.Duration, 30000000)[:0]
	pipeline = aggregate.Aggregate(&stats, pipeline)

	// Log results to a file
	outFilename := c.String("out-file")
	if outFilename != "" {
		reporter := report.CSVReporter{}
		if outFilename != "-" {
			f, err := os.Create("results.csv")
			if err != nil {
				log.WithError(err).Fatal("Couldn't open log file")
			}
			pipeline = report.Report(reporter, f, pipeline)
		} else {
			pipeline = report.Report(reporter, os.Stdout, pipeline)
		}
	}

	// Listen for SIGINT (Ctrl+C)
	stop := make(chan os.Signal)
	signal.Notify(stop, os.Interrupt)

runLoop:
	for {
		select {
		case res, ok := <-pipeline:
			if !ok {
				break runLoop
			}

			if res.Abort {
				cancel()
			}

			switch {
			case res.Error != nil:
				l := log.WithError(res.Error)
				if res.Time != time.Duration(0) {
					l = l.WithField("t", res.Time)
				}
				l.Error("Error")
			case res.Text != "":
				l := log.WithField("text", res.Text)
				if res.Time != time.Duration(0) {
					l = l.WithField("t", res.Time)
				}
				l.Info("Log")
			default:
				// log.WithField("t", res.Time).Debug("Metric")
			}
		case <-stop:
			cancel()
		}
	}

	log.WithField("results", stats.Results).Info("Finished")
	log.WithFields(log.Fields{
		"min": stats.Time.Min,
		"max": stats.Time.Max,
		"med": stats.Time.Med,
		"avg": stats.Time.Avg,
	}).Info("Time")

	return nil
}

// Configure the global logger.
func configureLogging(c *cli.Context) {
	if c.GlobalBool("verbose") {
		log.SetLevel(log.DebugLevel)
	}
}

func main() {
	// Up the thread limit (default: 10.000)
	debug.SetMaxThreads(100000)
	// Up the stack size limit (default: 1GB)
	debug.SetMaxStack(3 * 1000000000)

	// Free up -v and -h for our own flags
	cli.VersionFlag.Name = "version"
	cli.HelpFlag.Name = "help, ?"

	// Bootstrap using action-registered commandline flags
	app := cli.NewApp()
	app.Name = "speedboat"
	app.Usage = "A next-generation load generator"
	app.Version = "0.0.1a1"
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "verbose, v",
			Usage: "More verbose output",
		},
		cli.StringFlag{
			Name:  "script, s",
			Usage: "Script to run",
		},
		cli.StringFlag{
			Name:  "url",
			Usage: "URL to test",
		},
		cli.IntFlag{
			Name:  "vus, u",
			Usage: "Number of VUs to simulate",
			Value: 10,
		},
		cli.DurationFlag{
			Name:  "duration, d",
			Usage: "Test duration",
			Value: time.Duration(10) * time.Second,
		},
		cli.StringFlag{
			Name:  "out-file, o",
			Usage: "Output raw metrics to a file",
		},
	}
	app.Before = func(c *cli.Context) error {
		configureLogging(c)
		return nil
	}
	app.Action = action
	app.Run(os.Args)
}
