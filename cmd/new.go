package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	templates "go.k6.io/k6/cmd/newtemplates"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/lib/fsext"
)

var (
	defaultNewScriptName = "script.js"
)

type newScriptCmd struct {
	gs             *state.GlobalState
	overwriteFiles bool
	templateType   string
	enableCloud    bool
	projectID      string
}

func (c *newScriptCmd) flagSet() *pflag.FlagSet {
	flags := pflag.NewFlagSet("", pflag.ContinueOnError)
	flags.SortFlags = false
	flags.BoolVarP(&c.overwriteFiles, "force", "f", false, "Overwrite existing files")
	flags.StringVar(&c.templateType, "template", "minimal", "Template type (choices: minimal, protocol, browser)")
	flags.BoolVar(&c.enableCloud, "cloud", false, "Enable cloud integration")
	flags.StringVar(&c.projectID, "project-id", "", "Specify the project ID for cloud integration")
	return flags
}

func (c *newScriptCmd) run(cmd *cobra.Command, args []string) error {
	target := defaultNewScriptName
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
		EnableCloud: c.enableCloud,
		ProjectID:   c.projectID,
	}

	if err := templates.ExecuteTemplate(fd, tmpl, argsStruct); err != nil {
		return err
	}

	fmt.Fprintf(c.gs.Stdout, "New script created: %s (%s template).\n", target, c.templateType)
	return nil
}

func getCmdNewScript(gs *state.GlobalState) *cobra.Command {
	c := &newScriptCmd{gs: gs}

	initCmd := &cobra.Command{
		Use:   "new [file]",
		Short: "Create a new k6 script",
		Long: `Create a new k6 script using one of the predefined templates.

By default, the script will be named script.js unless a different name is specified.`,
		Example: `
  # Create a minimal k6 script
  {{.}} new --template minimal

  # Overwrite an existing file with a protocol-based script
  {{.}} new -f --template protocol test.js`,
		Args: cobra.MaximumNArgs(1),
		RunE: c.run,
	}

	initCmd.Flags().AddFlagSet(c.flagSet())
	return initCmd
}
