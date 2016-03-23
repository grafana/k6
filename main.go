package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/loadimpact/speedboat/master"
	"github.com/loadimpact/speedboat/message"
	"github.com/loadimpact/speedboat/worker"
	"os"
)

// All registered commands.
var globalCommands []cli.Command

// All registered master handlers.
var globalHandlers []func(*master.Master, message.Message, chan message.Message) bool

// All registered worker processors.
var globalProcessors []func(*worker.Worker, message.Message, chan message.Message) bool

// Register an application subcommand.
func registerCommand(cmd cli.Command) {
	globalCommands = append(globalCommands, cmd)
}

// Register a master handler
func registerHandler(handler func(*master.Master, message.Message, chan message.Message) bool) {
	globalHandlers = append(globalHandlers, handler)
}

// Register a worker processor.
func registerProcessor(processor func(*worker.Worker, message.Message, chan message.Message) bool) {
	globalProcessors = append(globalProcessors, processor)
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
