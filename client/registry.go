package client

import (
	"github.com/codegangsta/cli"
)

// All registered cli commands.
var GlobalCommands []cli.Command

// Register an application subcommand.
func RegisterCommand(cmd cli.Command) {
	GlobalCommands = append(GlobalCommands, cmd)
}
