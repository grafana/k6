package main

import (
	"errors"
	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/loadimpact/speedboat/loadtest"
	"github.com/loadimpact/speedboat/runner"
	"github.com/loadimpact/speedboat/runner/js"
	"io/ioutil"
	"os"
	"path"
	"time"
)

func getRunner(filename, url string) (runner.Runner, error) {
	// TODO: Implement a URL runner.
	if url != "" {
		return nil, nil
	}

	switch path.Ext(filename) {
	case ".js":
		return js.New()
	default:
		return nil, errors.New("No runner found")
	}
}

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

func action(c *cli.Context) {
	test, err := makeTest(c)
	if err != nil {
		log.WithError(err).Fatal("Configuration error")
	}

	r, err := getRunner(test.Script, test.URL)
	if err != nil {
		log.WithError(err).Fatal("Couldn't get a runner")
	}

	err = r.Load(test.Script, test.Source)
	if err != nil {
		log.WithError(err).Fatal("Couldn't load script")
	}

	// Write a number to the control channel to make the test scale to that many
	// VUs; close it to make the test terminate.
	controlChannel := make(chan int, 1)
	controlChannel <- test.Stages[0].VUs.Start

	sequencer := runner.NewSequencer()
	startTime := time.Now()

	intervene := time.NewTicker(time.Duration(1) * time.Second)
	results := runner.Run(r, controlChannel)
runLoop:
	for {
		select {
		case res, ok := <-results:
			// The results channel will be closed once all VUs are done.
			if !ok {
				break runLoop
			}
			switch res := res.(type) {
			case runner.LogEntry:
				log.WithField("text", res.Text).Info("Test Log")
			case runner.Metric:
				log.WithField("d", res.Duration).Debug("Test Metric")
				sequencer.Add(res)
			case error:
				log.WithError(res).Error("Test Error")
			}
		case <-intervene.C:
			vus, stop := test.VUsAt(time.Since(startTime))
			if stop {
				// Stop the timer, and let VUs gracefully terminate.
				intervene.Stop()
				close(controlChannel)
			} else {
				controlChannel <- vus
			}
		}
	}

	stats := sequencer.Stats()
	log.WithField("count", sequencer.Count()).Info("Results")
	log.WithFields(log.Fields{
		"min": stats.Duration.Min,
		"max": stats.Duration.Max,
		"avg": stats.Duration.Avg,
		"med": stats.Duration.Med,
	}).Info("Duration")
}

// Configure the global logger.
func configureLogging(c *cli.Context) {
	if c.GlobalBool("verbose") {
		log.SetLevel(log.DebugLevel)
	}
}

func main() {
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
			Usage: "Script to run (do not use with --url)",
		},
		cli.StringFlag{
			Name:  "url",
			Usage: "URL to test (do not use with --script)",
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
	}
	app.Before = func(c *cli.Context) error {
		configureLogging(c)
		return nil
	}
	app.Action = action
	app.Run(os.Args)
}
