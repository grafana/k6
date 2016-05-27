package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/loadimpact/speedboat/loadtest"
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
	conf := loadtest.Config{}

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
		if err = conf.ParseYAML(bytes); err != nil {
			log.WithError(err).Fatal("Couldn't parse config file")
		}
	default:
		log.Fatal("Wrong number of arguments")
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
	}
	app.Before = func(c *cli.Context) error {
		configureLogging(c)
		return nil
	}
	app.Action = action
	app.Run(os.Args)
}
