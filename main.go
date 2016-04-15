package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/loadimpact/speedboat/aggregate"
	"github.com/loadimpact/speedboat/loadtest"
	"github.com/loadimpact/speedboat/runner"
	"github.com/loadimpact/speedboat/runner/simple"
	"golang.org/x/net/context"
	"io/ioutil"
	"os"
	"path"
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

func run(test loadtest.LoadTest, r runner.Runner) <-chan runner.Result {
	ch := make(chan runner.Result)

	go func() {
		defer close(ch)

		timeout := time.Duration(0)
		for _, stage := range test.Stages {
			timeout += stage.Duration
		}

		ctx, _ := context.WithTimeout(context.Background(), timeout)
		scale := make(chan int, 1)
		scale <- test.Stages[0].VUs.Start

		for res := range runner.Run(ctx, r, scale) {
			ch <- res
		}
	}()

	return ch
}

func action(c *cli.Context) {
	test, err := makeTest(c)
	if err != nil {
		log.WithError(err).Fatal("Configuration error")
	}

	r := simple.New()
	r.URL = test.URL

	stats := aggregate.Stats{}
	stats.Time.Values = make([]time.Duration, 30000000)[:0]
	for res := range aggregate.Aggregate(&stats, run(test, r)) {
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
	}

	log.WithField("results", stats.Results).Info("Finished")
	log.WithFields(log.Fields{
		"min": stats.Time.Min,
		"max": stats.Time.Max,
		"med": stats.Time.Med,
		"avg": stats.Time.Avg,
	}).Info("Time")

	bucketInterval := time.Duration(100) * time.Millisecond
	buckets := [][]time.Duration{}
	for _, t := range stats.Time.Values {
		b := int(t / bucketInterval)
		for len(buckets) <= b {
			buckets = append(buckets, []time.Duration{})
		}
		buckets[b] = append(buckets[b], t)
	}
	for i, bucket := range buckets {
		log.WithFields(log.Fields{
			"i":   i,
			"len": len(bucket),
		}).Info("Bucket")
	}
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
