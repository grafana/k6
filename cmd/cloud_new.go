package cmd

import (
	"fmt"

	templates "go.k6.io/k6/cmd/newtemplates"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/lib/fsext"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const defaultNewCloudScriptName = "script.js"

type cmdCloudNew struct {
	gs             *state.GlobalState
	overwriteFiles bool
	templateType   string
	projectID      string
}

func (c *cmdCloudNew) flagSet() *pflag.FlagSet {
	flags := pflag.NewFlagSet("", pflag.ContinueOnError)
	flags.SortFlags = false
	flags.BoolVarP(&c.overwriteFiles, "force", "f", false, "overwrite existing files")
	flags.StringVar(&c.templateType, "template", "minimal", "template type (choices: minimal, protocol, browser)")
	flags.StringVar(&c.projectID, "project-id", "", "specify the Grafana Cloud ProjectID for the test")
	return flags
}

func (c *cmdCloudNew) run(cmd *cobra.Command, args []string) error {
	target := defaultNewCloudScriptName
	if len(args) > 0 {
		target = args[0]
	}

	fileExists, err := fsext.Exists(c.gs.FS, target)
	if err != nil {
		return err
	}
	if fileExists && !c.overwriteFiles {
		return fmt.Errorf("%s already exists. Use the `--force` flag to overwrite", target)
	}

	fd, err := c.gs.FS.Create(target)
	if err != nil {
		return err
	}
	defer fd.Close()

	tmpl, err := templates.GetTemplate(c.templateType)
	if err != nil {
		return err
	}

	argsStruct := templates.TemplateArgs{
		ScriptName:  target,
		EnableCloud: true,
		ProjectID:   c.projectID,
	}

	if err := templates.ExecuteTemplate(fd, tmpl, argsStruct); err != nil {
		return err
	}

	fmt.Fprintf(c.gs.Stdout, "New cloud script created: %s (%s template).\n", target, c.templateType)
	return nil
}

func getCmdCloudNew(cloudCmd *cmdCloud) *cobra.Command {
	c := &cmdCloudNew{gs: cloudCmd.gs}

	exampleText := getExampleText(cloudCmd.gs, `
  # Create a minimal cloud-ready script
  $ {{.}} cloud new --template minimal

  # Create a protocol-based cloud script with a specific ProjectID
  $ {{.}} cloud new --template protocol --project-id my-project-id

  # Overwrite an existing file with a browser-based cloud script
  $ {{.}} cloud new -f --template browser test.js --project-id my-project-id`[1:])

	initCmd := &cobra.Command{
		Use:   "new [file]",
		Short: "Create a new k6 script",
		Long: `Create a new cloud-ready k6 script using one of the predefined templates.

By default, the script will be named script.js unless a different name is specified.`,
		Example: exampleText,
		Args:    cobra.MaximumNArgs(1),
		RunE:    c.run,
	}

	initCmd.Flags().AddFlagSet(c.flagSet())
	return initCmd
}
