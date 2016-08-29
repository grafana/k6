package main

import (
	log "github.com/Sirupsen/logrus"
	"gopkg.in/urfave/cli.v1"
	"os"
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
		commandRun,
		commandStatus,
		commandScale,
		commandAbort,
	}
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "verbose, v",
			Usage: "show debug messages",
		},
		cli.StringFlag{
			Name:  "address, a",
			Usage: "address for the API",
			Value: "127.0.0.1:6565",
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
