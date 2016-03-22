package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"os"
)

// All registered commands.
var globalCommands []cli.Command

// Register an application subcommand.
func registerCommand(cmd cli.Command) {
	globalCommands = append(globalCommands, cmd)
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

	// Bootstrap using commandline flags
	app := cli.NewApp()
	app.Name = "speedboat"
	app.Usage = "A next-generation load generator"
	app.Version = "0.0.1a1"
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "verbose, v",
			Usage: "More verbose output",
		},
	}
	app.Commands = globalCommands
	app.Before = func(c *cli.Context) error {
		configureLogging(c)
		return nil
	}
	app.Run(os.Args)
}
