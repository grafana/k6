package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/lib/fsext"
)

const (
	defaultNewScriptName = "script.js"
	defaultNewScript     = `import http from 'k6/http';
import { sleep } from 'k6';

export default function () {
  http.get('https://grafana.com');
  sleep(1);
}
`
)

// initCmd represents the `k6 init` command
type initCmd struct {
	gs *state.GlobalState
}

func (c *initCmd) run(cmd *cobra.Command, args []string) error {
	target := defaultNewScriptName
	if len(args) > 0 {
		target = args[0]
	}

	fileExists, err := fsext.Exists(c.gs.FS, target)
	if err != nil {
		return err
	}

	if fileExists {
		c.gs.Logger.Errorf("%s already exists", target)
		return err
	}

	fd, err := c.gs.FS.Create(target)
	if err != nil {
		return err
	}
	defer fd.Close()

	_, err = fd.Write([]byte(defaultNewScript))
	if err != nil {
		return err
	}

	printToStdout(c.gs, fmt.Sprintf("Initialized a new k6 test script in %s.\n", target))
	printToStdout(c.gs, fmt.Sprintf("You can now execute it by running `%s run %s`.\n", c.gs.BinaryName, target))

	return nil
}

func getCmdInit(gs *state.GlobalState) *cobra.Command {
	c := &initCmd{gs: gs}

	exampleText := getExampleText(gs, `
  # Create a minimal k6 script in the current directory. By default, k6 creates script.js.
  {{.}} init

  # Create a minimal k6 script in the current directory and store it in test.js
  {{.}} init test.js`)

	return &cobra.Command{
		Use:   "init",
		Short: "Initialize a new k6 script.",
		Long: `Initialize a new k6 script.

This command will create a minimal k6 script in the current directory and
store it in the file specified by the first argument. If no argument is
provided, the script will be stored in script.js.

This command will not overwrite existing files.`,
		Example: exampleText,
		RunE:    c.run,
	}
}
