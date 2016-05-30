package main

import (
	"errors"
	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/loadimpact/speedboat"
	"github.com/loadimpact/speedboat/js"
	"github.com/loadimpact/speedboat/simple"
	"github.com/rcrowley/go-metrics"
	"golang.org/x/net/context"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	stdlog "log"
	"os"
	"strings"
	"time"
)

// Configure the global logger.
func configureLogging(c *cli.Context) {
	log.SetLevel(log.InfoLevel)
	if c.GlobalBool("verbose") {
		log.SetLevel(log.DebugLevel)
	}
}

func parse(cc *cli.Context) (conf Config, err error) {
	switch len(cc.Args()) {
	case 0:
		if !cc.IsSet("script") && !cc.IsSet("url") {
			return conf, errors.New("No config file, script or URL")
		}
	case 1:
		bytes, err := ioutil.ReadFile(cc.Args()[0])
		if err != nil {
			return conf, errors.New("Couldn't read config file")
		}
		if err := yaml.Unmarshal(bytes, &conf); err != nil {
			return conf, errors.New("Couldn't parse config file")
		}
	default:
		return conf, errors.New("Too many arguments!")
	}

	// Let commandline flags override config files
	if cc.IsSet("script") {
		conf.Script = cc.String("script")
	}
	if cc.IsSet("url") {
		conf.URL = cc.String("url")
	}
	if cc.IsSet("vus") {
		conf.VUs = cc.Int("vus")
	}
	if cc.IsSet("duration") {
		conf.Duration = cc.Duration("duration").String()
	}

	return conf, nil
}

func dumpTest(t *speedboat.Test) {
	log.WithFields(log.Fields{
		"script": t.Script,
		"url":    t.URL,
	}).Info("General")
	for i, stage := range t.Stages {
		log.WithFields(log.Fields{
			"#":        i,
			"duration": stage.Duration,
			"start":    stage.StartVUs,
			"end":      stage.EndVUs,
		}).Info("Stage")
	}
}

func headlessController(c context.Context, t *speedboat.Test) <-chan int {
	ch := make(chan int)

	go func() {
		defer close(ch)

		select {
		case ch <- t.VUsAt(0):
		case <-c.Done():
			return
		}

		startTime := time.Now()
		ticker := time.NewTicker(100 * time.Millisecond)
		for {
			select {
			case <-ticker.C:
				ch <- t.VUsAt(time.Since(startTime))
			case <-c.Done():
				return
			}
		}
	}()

	return ch
}

func action(cc *cli.Context) error {
	conf, err := parse(cc)
	if err != nil {
		log.WithError(err).Fatal("Invalid arguments; see --help")
	}

	t, err := conf.MakeTest()
	if err != nil {
		log.WithError(err).Fatal("Configuration error")
	}

	if cc.Bool("dump") {
		dumpTest(&t)
		return nil
	}

	// Inspect the test to find a suitable runner; additional ones can easily be added
	var runner speedboat.Runner
	switch {
	case t.URL != "":
		runner = simple.New()
	case strings.HasSuffix(t.Script, ".js"):
		src, err := ioutil.ReadFile(t.Script)
		if err != nil {
			log.WithError(err).Fatal("Couldn't read script")
		}
		runner = js.New(string(src))
	default:
		log.Fatal("No suitable runner found!")
	}

	// Global metrics
	mVUs := metrics.NewRegisteredGauge("vus", speedboat.Registry)

	// Output metrics appropriately
	go metrics.Log(speedboat.Registry, time.Second, stdlog.New(os.Stderr, "metrics: ", stdlog.Lmicroseconds))

	// Use a "headless controller" to scale VUs by polling the test ramp
	ctx, _ := context.WithTimeout(context.Background(), t.TotalDuration())
	vus := []context.CancelFunc{}
	for scale := range headlessController(ctx, &t) {
		for i := len(vus); i < scale; i++ {
			log.WithField("id", i).Debug("Spawning VU")
			vuCtx, cancel := context.WithCancel(ctx)
			vus = append(vus, cancel)
			go runner.RunVU(vuCtx, t, len(vus))
		}
		for i := len(vus); i > scale; i-- {
			log.WithField("id", i-1).Debug("Dropping VU")
			vus[i-1]()
			vus = vus[:i-1]
		}
		mVUs.Update(int64(len(vus)))
	}

	// Wait until the end of the test
	<-ctx.Done()

	return nil
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
		cli.BoolFlag{
			Name:  "dump",
			Usage: "Dump parsed test and exit",
		},
	}
	app.Before = func(c *cli.Context) error {
		configureLogging(c)
		return nil
	}
	app.Action = action
	app.Run(os.Args)
}
