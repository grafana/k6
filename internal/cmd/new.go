package cmd

import (
	"encoding/json"
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
	team           string
	env            string
	listTemplates  bool
	verbose        bool
}

func (c *newScriptCmd) flagSet() *pflag.FlagSet {
	flags := pflag.NewFlagSet("", pflag.ContinueOnError)
	flags.SortFlags = false
	flags.BoolVarP(&c.overwriteFiles, "force", "f", false, "overwrite existing files")
	flags.StringVar(&c.templateType, "template", "minimal", "template type (choices: minimal, protocol, browser, rest) or relative/absolute path to a custom template file") //nolint:lll
	flags.StringVar(&c.projectID, "project-id", "", "specify the Grafana Cloud project ID for the test")
	flags.StringVar(&c.projectID, "project", "", "alias for --project-id (for template rendering only)")
	flags.StringVar(&c.team, "team", "", "specify the team name for template rendering")
	flags.StringVar(&c.env, "env", "", "specify the environment name for template rendering")
	flags.BoolVar(&c.listTemplates, "list-templates", false, "list all available templates")
	flags.BoolVar(&c.verbose, "verbose", false, "show detailed template information in JSON format (used with --list-templates)")
	return flags
}

func (c *newScriptCmd) run(_ *cobra.Command, args []string) (err error) {
	// Initialize template manager
	tm, err := templates.NewTemplateManager(c.gs.FS, c.gs.UserOSConfigDir)
	if err != nil {
		return fmt.Errorf("error initializing template manager: %w", err)
	}

	// Handle --list-templates flag
	if c.listTemplates {
		if c.verbose {
			// Return JSON output with full metadata
			templatesWithInfo, err := tm.ListTemplatesWithInfo()
			if err != nil {
				return fmt.Errorf("error listing templates: %w", err)
			}

			// Create JSON output
			jsonData, err := json.MarshalIndent(templatesWithInfo, "", "  ")
			if err != nil {
				return fmt.Errorf("error marshaling templates to JSON: %w", err)
			}

			fmt.Fprintln(c.gs.Stdout, string(jsonData))
			return nil
		}

		// Standard list output with metadata if available
		templatesWithInfo, err := tm.ListTemplatesWithInfo()
		if err != nil {
			return fmt.Errorf("error listing templates: %w", err)
		}

		if len(templatesWithInfo) == 0 {
			fmt.Fprintln(c.gs.Stdout, "No templates available.")
			return nil
		}

		fmt.Fprintln(c.gs.Stdout, "Available templates:")
		for _, tmpl := range templatesWithInfo {
			typeInfo := ""
			if tmpl.Type != "new" && tmpl.Type != "" {
				typeInfo = fmt.Sprintf(" [%s]", tmpl.Type)
			}

			if tmpl.Metadata != nil && tmpl.Metadata.Description != "" {
				fmt.Fprintf(c.gs.Stdout, "  %s - %s%s\n", tmpl.Name, tmpl.Metadata.Description, typeInfo)
			} else {
				warning := ""
				if tmpl.Warning != "" {
					warning = " ⚠️"
				}
				fmt.Fprintf(c.gs.Stdout, "  %s (no metadata)%s%s\n", tmpl.Name, warning, typeInfo)
			}
		}
		return nil
	}

	target := defaultNewScriptName
	if len(args) > 0 {
		target = args[0]
	}

	// Validate template usage and show warnings if appropriate
	warning, err := tm.ValidateTemplateUsage(c.templateType, "new")
	if err != nil {
		return fmt.Errorf("error validating template: %w", err)
	}
	if warning != "" {
		fmt.Fprintln(c.gs.Stderr, warning)
	}

	// Prepare template arguments
	// TODO: Consider injecting --team and --env into options.tags automatically in the future.
	// For now, template authors may include these values manually if desired.
	// This supports future GCk6 use cases like team-level or environment-aware tagging.
	argsStruct := templates.TemplateArgs{
		ScriptName: target,
		ProjectID:  c.projectID,
		Project:    c.projectID, // Alias for ProjectID for template compatibility
		Team:       c.team,
		Env:        c.env,
	}

	// Check if this is a directory-based template
	if tm.IsDirectoryBasedTemplate(c.templateType) {
		// For directory-based templates, copy all files from the template directory
		if err := tm.CopyTemplateFiles(c.templateType, argsStruct, c.overwriteFiles, c.gs.Stdout); err != nil {
			return fmt.Errorf("error copying template files: %w", err)
		}

		if _, err := fmt.Fprintf(c.gs.Stdout, "New script created from template: %s\n", c.templateType); err != nil {
			return err
		}

		return nil
	}

	// For file-based templates and built-in templates, use the original single-file approach
	fileExists, err := fsext.Exists(c.gs.FS, target)
	if err != nil {
		return err
	}
	if fileExists && !c.overwriteFiles {
		return fmt.Errorf("%s already exists. Use the `--force` flag to overwrite it", target)
	}

	tmpl, err := tm.GetTemplate(c.templateType)
	if err != nil {
		return fmt.Errorf("error retrieving template: %w", err)
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
    $ {{.}} new --template rest

    # List all available templates
    $ {{.}} new --list-templates

    # List all templates with detailed metadata in JSON format
    $ {{.}} new --list-templates --verbose

    # Create a cloud-ready script with a specific project ID
    $ {{.}} new --project-id 12315

    # Create a script with team and environment context for template rendering
    $ {{.}} new --template rest --team platform --env staging

    # Use both project and team information
    $ {{.}} new --project 12315 --team platform --env production`[1:])

	initCmd := &cobra.Command{
		Use:   "new [file]",
		Short: "Create a new k6 script from a reusable template",
		Long: `Create and initialize a new k6 script using one of the predefined templates.

By default, the script will be named script.js unless a different name is specified.

Templates are searched in the following order:
1. ./templates/<name>/script.js (local to current working directory)
2. ~/.k6/templates/<name>/script.js (user-global templates)  
3. Built-in templates (minimal, protocol, browser, rest)

The --project/--project-id, --team, and --env flags are used only for template rendering
and are available as template variables ({{ .Project }}, {{ .Team }}, {{ .Env }}).
These flags do not affect test execution - they only influence the generated script content.

Use --list-templates to see all available templates.`,
		Example: exampleText,
		Args:    cobra.MaximumNArgs(1),
		RunE:    c.run,
	}

	initCmd.Flags().AddFlagSet(c.flagSet())
	return initCmd
}
