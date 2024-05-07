package cmd

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/kataras/iris/v12"
	"github.com/liuxd6825/k6server/cmd/state"
	"github.com/liuxd6825/k6server/lib/fsext"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const defaultServerScriptName = "script.js"

// newScriptCmd represents the `k6 new` command
type newServerCmd struct {
	gs  *state.GlobalState
	app *iris.Application
}

func (c *newServerCmd) flagSet() *pflag.FlagSet {
	flags := pflag.NewFlagSet("", pflag.ContinueOnError)
	flags.SortFlags = false

	return flags
}

func (c *newServerCmd) run(cmd *cobra.Command, args []string) error { //nolint:revive
	target := defaultServerScriptName
	if len(args) > 0 {
		target = args[0]
	}

	fileExists, err := fsext.Exists(c.gs.FS, target)
	if err != nil {
		return err
	}

	if !fileExists {
		return fmt.Errorf("%s not exists", target)
	}

	valueColor := getColor(c.gs.Flags.NoColor || !c.gs.Stdout.IsTTY, color.Bold)
	printToStdout(c.gs, fmt.Sprintf(
		"Initialized a new k6 test script in %s. You can now execute it by running `%s run %s`.\n",
		valueColor.Sprint(target),
		c.gs.BinaryName,
		target,
	))

	return nil
}

func getCmdServer(_ *state.GlobalState) *cobra.Command {
	// versionCmd represents the version command.
	return &cobra.Command{
		Use:   "server",
		Short: "Show application version",
		Long:  `Show the application version and exit.`,
		Run: func(cmd *cobra.Command, _ []string) {
			root := cmd.Root()
			root.SetArgs([]string{"--server"})
			_ = root.Execute()
		},
	}
}
