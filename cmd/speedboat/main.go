package main

import (
	"errors"
	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/loadimpact/speedboat"
	"github.com/loadimpact/speedboat/simple"
	"golang.org/x/net/context"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
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

	var runner speedboat.Runner
	switch {
	case t.URL != "":
		runner = simple.New()
	default:
		log.Fatal("No suitable runner found!")
	}

	ctx, _ := context.WithTimeout(context.Background(), t.TotalDuration())
	offset := time.Duration(0)
	for _, stage := range t.Stages {
		localOffset := offset
		go func() {
			time.Sleep(localOffset)
			c, _ := context.WithTimeout(ctx, stage.Duration)
			runner.RunVU(c, t)
		}()
		offset += stage.Duration
	}

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
