package main

import (
	log "github.com/Sirupsen/logrus"
	"gopkg.in/urfave/cli.v1"
	"os"
	"time"
)

func main() {
	// This won't be needed in cli v2
	cli.VersionFlag.Name = "version"
	cli.HelpFlag.Name = "help"
	cli.HelpFlag.Hidden = true

	app := cli.NewApp()
	app.Name = "speedboat"
	app.Usage = "a next generation load generator"
	app.Version = "0.2.0"
	app.Commands = []cli.Command{
		cli.Command{
			Name:      "run",
			Aliases:   []string{"r"},
			Usage:     "Starts running a load test",
			ArgsUsage: "url|filename",
			Flags: []cli.Flag{
				cli.IntFlag{
					Name:  "vus, u",
					Usage: "virtual users to simulate",
					Value: 10,
				},
				cli.DurationFlag{
					Name:  "duration, d",
					Usage: "test duration, 0 to run until cancelled",
					Value: 10 * time.Second,
				},
				cli.StringFlag{
					Name:  "type, t",
					Usage: "input type, one of: auto, url, js",
					Value: "auto",
				},
				cli.StringFlag{
					Name:  "address, a",
					Usage: "address to listen for requests on",
					Value: "127.0.0.1:6565",
				},
			},
			Action: actionRun,
		},
	}
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "verbose, v",
			Usage: "show debug messages",
		},
	}
	app.Before = func(cc *cli.Context) error {
		if cc.Bool("verbose") {
			log.SetLevel(log.DebugLevel)
		}

		return nil
	}
	if err := app.Run(os.Args); err != nil {
		os.Exit(1)
	}
}
