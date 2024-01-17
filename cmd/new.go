package cmd

import (
	"fmt"
	"path"
	"text/template"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/lib/fsext"
)

const defaultNewScriptName = "script.js"

//nolint:gochecknoglobals
var defaultNewScriptTemplate = template.Must(template.New("new").Parse(`import http from 'k6/http';
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
  // to learn about authoring and running k6 test scripts in Grafana k6 Cloud.
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
  // See https://grafana.com/docs/k6/latest/using-k6-browser/running-browser-tests/ to learn more
  // about using Browser API in your test scripts.
  //
  // scenarios: {
  //   // The scenario name appears in the result summary, tags, and so on.
  //   // You can give the scenario any name, as long as each name in the script is unique.
  //   ui: {
  //     // Executor is a mandatory parameter for browser-based tests.
  //     // Shared iterations in this case tells k6 to reuse VUs to execute iterations.
  //     //
  //     // See https://grafana.com/docs/k6/latest/using-k6/scenarios/executors/ for other executor types.
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
// See https://grafana.com/docs/k6/latest/examples/get-started-with-k6/ to learn more
// about authoring k6 scripts.
//
export default function() {
  http.get('https://test.k6.io');
  sleep(1);
}
`))

type initScriptTemplateArgs struct {
	ScriptName string
}

// newScriptCmd represents the `k6 new` command
type newScriptCmd struct {
	gs             *state.GlobalState
	overwriteFiles bool
}

func (c *newScriptCmd) flagSet() *pflag.FlagSet {
	flags := pflag.NewFlagSet("", pflag.ContinueOnError)
	flags.SortFlags = false
	flags.BoolVarP(&c.overwriteFiles, "force", "f", false, "Overwrite existing files")

	return flags
}

func (c *newScriptCmd) run(cmd *cobra.Command, args []string) error { //nolint:revive
	target := defaultNewScriptName
	if len(args) > 0 {
		target = args[0]
	}

	fileExists, err := fsext.Exists(c.gs.FS, target)
	if err != nil {
		return err
	}

	if fileExists && !c.overwriteFiles {
		return fmt.Errorf("%s already exists, please use the `--force` flag if you want overwrite it", target)
	}

	fd, err := c.gs.FS.Create(target)
	if err != nil {
		return err
	}
	defer func() {
		_ = fd.Close() // we may think to check the error and log
	}()

	if err := defaultNewScriptTemplate.Execute(fd, initScriptTemplateArgs{
		ScriptName: path.Base(target),
	}); err != nil {
		return err
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

func getCmdNewScript(gs *state.GlobalState) *cobra.Command {
	c := &newScriptCmd{gs: gs}

	exampleText := getExampleText(gs, `
  # Create a minimal k6 script in the current directory. By default, k6 creates script.js.
  {{.}} new

  # Create a minimal k6 script in the current directory and store it in test.js
  {{.}} new test.js

  # Overwrite existing test.js with a minimal k6 script
  {{.}} new -f test.js`[1:])

	initCmd := &cobra.Command{
		Use:   "new",
		Short: "Create and initialize a new k6 script",
		Long: `Create and initialize a new k6 script.

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
