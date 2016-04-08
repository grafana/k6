package client

import (
	"github.com/codegangsta/cli"
	"testing"
)

func TestRegisterCommand(t *testing.T) {
	oldGlobalCommands := GlobalCommands
	GlobalCommands = nil
	defer func() { GlobalCommands = oldGlobalCommands }()

	RegisterCommand(cli.Command{Name: "test"})
	if len(GlobalCommands) != 1 {
		t.Error("Command not registered")
	}
}
