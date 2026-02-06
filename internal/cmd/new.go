package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/internal/cmd/templates"
	"go.k6.io/k6/lib/fsext"
)

const defaultNewScriptName = "script.js"

type newScriptCmd struct {
	gs             *state.GlobalState
	overwriteFiles bool
	templateType   string
	projectID      string
}

func (c *newScriptCmd) flagSet() *pflag.FlagSet {
	flags := pflag.NewFlagSet("", pflag.ContinueOnError)
	flags.SortFlags = false
	flags.BoolVarP(&c.overwriteFiles, "force", "f", false, "overwrite existing files")
	flags.StringVar(&c.templateType, "template", "minimal", "template type (choices: minimal, protocol, browser) or relative/absolute path to a custom template file") //nolint:lll
	flags.StringVar(&c.projectID, "project-id", "", "specify the Grafana Cloud project ID for the test")
	return flags
}

func (c *newScriptCmd) run(_ *cobra.Command, args []string) (err error) {
	target := defaultNewScriptName
	if len(args) > 0 {
		target = args[0]
	}

	fileExists, err := fsext.Exists(c.gs.FS, target)
	if err != nil {
		return err
	}
	if fileExists && !c.overwriteFiles {
		return fmt.Errorf("%s already exists. Use the `--force` flag to overwrite it", target)
	}

	// Initialize template manager and validate template before creating any files
	tm, err := templates.NewTemplateManager(c.gs.FS)
	if err != nil {
		return fmt.Errorf("error initializing template manager: %w", err)
	}

	tmpl, err := tm.GetTemplate(c.templateType)
	if err != nil {
		return fmt.Errorf("error retrieving template: %w", err)
	}

	// Prepare template arguments
	argsStruct := templates.TemplateArgs{
		ScriptName: target,
		ProjectID:  c.projectID,
	}

	// First render the template to a buffer to validate it
	var buf strings.Builder
	if err := templates.ExecuteTemplate(&buf, tmpl, argsStruct); err != nil {
		return fmt.Errorf("failed to execute template %s: %w", c.templateType, err)
	}

	// Only create the file after template rendering succeeds
	fd, err := c.gs.FS.Create(target)
	if err != nil {
		return err
	}

	defer func() {
		if cerr := fd.Close(); cerr != nil {
			if _, werr := fmt.Fprintf(c.gs.Stderr, "error closing file: %v\n", cerr); werr != nil {
				err = fmt.Errorf("error writing error message to stderr: %w", werr)
			} else {
				err = cerr
			}
		}
	}()

	// Write the rendered content to the file
	if _, err := io.WriteString(fd, buf.String()); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(c.gs.Stdout, "New script created: %s (%s template).\n", target, c.templateType); err != nil {
		return err
	}

	return nil
}

func getCmdNewScript(gs *state.GlobalState) *cobra.Command {
	c := &newScriptCmd{gs: gs}

	exampleText := getExampleText(c.gs, `
    # Create a new k6 script with the default template
    $ {{.}} new

    # Specify a file name when creating a script
    $ {{.}} new test.js

    # Overwrite an existing file
    $ {{.}} new -f test.js

    # Create a script using a specific template
    $ {{.}} new --template protocol

    # Create a cloud-ready script with a specific project ID
    $ {{.}} new --project-id 12315`[1:])

	initCmd := &cobra.Command{
		Use:   "new [file]",
		Short: "Create a test",
		Long: `Create and initialize a new k6 script using one of the predefined templates.

By default, the script will be named script.js unless a different name is specified.`,
		Example: exampleText,
		Args:    cobra.MaximumNArgs(1),
		RunE:    c.run,
	}

	initCmd.Flags().AddFlagSet(c.flagSet())
	return initCmd
}
