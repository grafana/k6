package cmd

import (
	"errors"
	"fmt"
	"path"
	"text/template"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/lib/fsext"
)

const defaultNewScriptName = "script.js"

//nolint:gochecknoglobals
var (
	errFileExists            = errors.New("file already exists")
	defaultNewScriptTemplate = template.Must(template.New("init").Parse(`import http from 'k6/http';
import { sleep } from 'k6';

export const options = {
  // A number specifying the number of VUs to run concurrently.
  vus: 10,
  // A string specifying the total duration of the test run.
  duration: '30s',

  // The following section contains configuration options for execution of this
  // test script in Grafana Cloud.
  //
  // See https://grafana.com/docs/grafana-cloud/k6/get-started/run-cloud-tests-from-the-cli/
  // to learn about authoring and running k6 test scripts in Grafana Cloud.
  //
  // cloud: {
  //   // The ID of the project to which the test is assigned in the k6 Cloud UI.
  //   // By default tests are executed in default project.
  //   projectID: "",
  //   // The name of the test in the k6 Cloud UI.
  //   // Test runs with the same name will be grouped.
  //   name: "{{ .ScriptName }}"
  // },

  // Uncomment this section to enable the use of Browser API in your tests.
  //
  // See https://k6.io/docs/using-k6-browser/running-browser-tests/ to learn more
  // about using Browser API in your test scripts.
  //
  // scenarios: {
  //   // The scenario name appears in the result summary, tags, and so on.
  //   SCENARIO NAME: {
  //     // Mandatory parameter for browser-based tests.
  //     executor: 'shared-iterations',
  //     options: {
  //       browser: {
  //         // This is a mandatory parameter that instructs k6 to launch and
  //         // connect to a chromium-based browser, and use it to run UI-based
  //         // tests.
  //         type: 'chromium',
  //       },
  //     },
  //   },
  // }
};

// The function that defines VU logic.
//
// See https://k6.io/docs/examples/tutorials/get-started-with-k6/ to learn more
// about authoring k6 scripts.
//
export default function() {
  http.get('http://test.k6.io');
  sleep(1);
}
`))
)

type initScriptTemplateArgs struct {
	ScriptName string
}

// initCmd represents the `k6 init` command
type initCmd struct {
	gs             *state.GlobalState
	overwriteFiles bool
}

func (c *initCmd) flagSet() *pflag.FlagSet {
	flags := pflag.NewFlagSet("", pflag.ContinueOnError)
	flags.SortFlags = false
	flags.BoolVarP(&c.overwriteFiles, "force", "f", false, "Overwrite existing files")

	return flags
}

func (c *initCmd) run(cmd *cobra.Command, args []string) error { //nolint:revive
	target := defaultNewScriptName
	if len(args) > 0 {
		target = args[0]
	}

	fileExists, err := fsext.Exists(c.gs.FS, target)
	if err != nil {
		return err
	}

	if fileExists && !c.overwriteFiles {
		c.gs.Logger.Errorf("%s already exists", target)
		return errFileExists
	}

	fd, err := c.gs.FS.Create(target)
	if err != nil {
		return err
	}
	defer fd.Close() //nolint:errcheck

	if err := defaultNewScriptTemplate.Execute(fd, initScriptTemplateArgs{
		ScriptName: path.Base(target),
	}); err != nil {
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
  {{.}} init test.js

  # Overwrite existing test.js with a minimal k6 script
  {{.}} init -f test.js`[1:])

	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new k6 script.",
		Long: `Initialize a new k6 script.

This command will create a minimal k6 script in the current directory and
store it in the file specified by the first argument. If no argument is
provided, the script will be stored in script.js.

This command will not overwrite existing files.`,
		Example: exampleText,
		Args:    cobra.MaximumNArgs(1),
		RunE:    c.run,
	}
	initCmd.Flags().AddFlagSet(c.flagSet())

	return initCmd
}
