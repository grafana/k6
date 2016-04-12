package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/loadimpact/speedboat/loadtest"
	"github.com/loadimpact/speedboat/runner"
	"github.com/loadimpact/speedboat/util"
	"io/ioutil"
	"os"
	"path"
	"runtime/pprof"
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

	if err = test.Load(base); err != nil {
		return test, err
	}

	return test, nil
}

func action(c *cli.Context) {
	test, err := makeTest(c)
	if err != nil {
		log.WithError(err).Fatal("Configuration error")
	}

	r, err := util.GetRunner(test.Script)
	if err != nil {
		log.WithError(err).Fatal("Couldn't get a runner")
	}
	log.WithField("r", r).Info("Runner")

	err = r.Load(test.Script, test.Source)
	if err != nil {
		log.WithError(err).Fatal("Couldn't load script")
	}

	controlChannel := make(chan int, 1)
	currentVUs := test.Stages[0].VUs.Start
	controlChannel <- currentVUs

	startTime := time.Now()
	intervene := time.Tick(time.Duration(1) * time.Second)
	sequencer := runner.NewSequencer()
	results := runner.Run(r, controlChannel)
runLoop:
	for {
		select {
		case res := <-results:
			switch res := res.(type) {
			case runner.LogEntry:
				log.WithField("text", res.Text).Info("Test Log")
			case runner.Metric:
				log.WithField("d", res.Duration).Debug("Test Metric")
				sequencer.Add(res)
			case error:
				log.WithError(res).Error("Test Error")
			}
		case <-intervene:
			vus, stop := test.VUsAt(time.Since(startTime))
			if stop {
				break runLoop
			}
			if vus != currentVUs {
				delta := vus - currentVUs
				controlChannel <- delta
				currentVUs = vus
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

// Set up a CPU profile, if requested.
func startCPUProfile(c *cli.Context) {
	cpuProfile := c.String("cpuprofile")
	if cpuProfile != "" {
		f, err := os.Create(cpuProfile)
		if err != nil {
			log.WithError(err).Fatal("Couldn't create CPU profile file")
		}

		pprof.StartCPUProfile(f)
	}
}

// End an ongoing CPU profile.
func endCPUProfile(c *cli.Context) {
	pprof.StopCPUProfile()
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
			Usage: "Script to run",
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
			Name:  "cpuprofile",
			Usage: "Write a CPU profile to this file",
		},
	}
	app.Before = func(c *cli.Context) error {
		configureLogging(c)
		startCPUProfile(c)
		return nil
	}
	app.After = func(c *cli.Context) error {
		endCPUProfile(c)
		return nil
	}
	app.Action = action
	app.Run(os.Args)
}
