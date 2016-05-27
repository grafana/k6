package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
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

func action(cc *cli.Context) error {
	conf := Config{}

	switch len(cc.Args()) {
	case 0:
		if !cc.IsSet("script") && !cc.IsSet("url") {
			log.Fatal("No config file, script or URL provided; see --help for usage")
		}
	case 1:
		bytes, err := ioutil.ReadFile(cc.Args()[0])
		if err != nil {
			log.WithError(err).Fatal("Couldn't read config file")
		}
		if err := yaml.Unmarshal(bytes, &conf); err != nil {
			log.WithError(err).Fatal("Couldn't parse config file")
		}
	default:
		log.Fatal("Too many arguments!")
	}

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

	t, err := conf.MakeTest()
	if err != nil {
		log.WithError(err).Fatal("Configuration error")
	}

	if cc.Bool("dump") {
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
