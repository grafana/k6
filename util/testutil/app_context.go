package testutil

import (
	"github.com/codegangsta/cli"
	"strings"
)

func WithAppContext(args string, cmd cli.Command, fn func(c *cli.Context)) error {
	app := cli.NewApp()

	// Make a surrogate command, with the same invocation as cmd
	app.Commands = []cli.Command{
		cli.Command{
			Name:   "cmd",
			Flags:  cmd.Flags,
			Action: func(c *cli.Context) { fn(c) },
		},
	}

	// Don't print an usage message to stdout for invalid arguments
	app.OnUsageError = func(_ *cli.Context, err error, _ bool) error { return err }

	// Imitate os.Args by prepending a program name, and invoke the surrogate
	appArgs := []string{"program", "cmd"}
	for _, arg := range strings.Split(args, " ") {
		appArgs = append(appArgs, arg)
	}

	// Returns an error for invalid arguments
	return app.Run(appArgs)
}
