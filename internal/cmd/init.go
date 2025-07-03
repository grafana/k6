package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/internal/cmd/templates"
)

type initProjectCmd struct {
	gs          *state.GlobalState
	orgTemplate string
	name        string
	projectID   string
	team        string
	env         string
}

func (c *initProjectCmd) flagSet() *pflag.FlagSet {
	flags := pflag.NewFlagSet("", pflag.ContinueOnError)
	flags.SortFlags = false
	flags.StringVar(&c.orgTemplate, "org-template", "", "name of the directory-based template to use (required)")
	flags.StringVar(&c.name, "name", "", "name of the project folder to create")
	flags.StringVar(&c.projectID, "project-id", "", "Grafana Cloud project ID for the test")
	flags.StringVar(&c.team, "team", "", "specify the team name for template rendering")
	flags.StringVar(&c.env, "env", "", "specify the environment name for template rendering")
	return flags
}

func (c *initProjectCmd) run(_ *cobra.Command, _ []string) error {
	// TODO: This UX is functional but may need refinement.
	// Consider positional args (e.g., k6 init <template> <name>), a --vars flag, or --interactive mode.

	if c.orgTemplate == "" {
		return fmt.Errorf("--org-template is required")
	}

	// Initialize template manager
	tm, err := templates.NewTemplateManager(c.gs.FS, c.gs.UserOSConfigDir)
	if err != nil {
		return fmt.Errorf("error initializing template manager: %w", err)
	}

	// Validate template usage and show warnings if appropriate
	warning, err := tm.ValidateTemplateUsage(c.orgTemplate, "init")
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
	args := templates.TemplateArgs{
		ScriptName: "script.js", // Default script name for projects
		ProjectID:  c.projectID,
		Project:    c.projectID, // Alias for ProjectID for template compatibility
		Team:       c.team,
		Env:        c.env,
		Name:       c.name,
	}

	// Scaffold the project
	if err := tm.ScaffoldProject(c.orgTemplate, args, c.gs.Stdout); err != nil {
		return fmt.Errorf("error scaffolding project: %w", err)
	}

	return nil
}

func getCmdInit(gs *state.GlobalState) *cobra.Command {
	c := &initProjectCmd{gs: gs}

	exampleText := getExampleText(gs, `
    # Scaffold a project using an organizational template
    $ {{.}} init --org-template rest --name cart-service --team checkout

    # Create a project with team and environment context
    $ {{.}} init --org-template microservice --name user-api --team platform --env staging

    # Scaffold with GCk6 project ID for cloud integration
    $ {{.}} init --org-template full-stack --name e2e-tests --project-id 12345 --team qa --env prod

    # Use template's default project name
    $ {{.}} init --org-template api-template --team backend`[1:])

	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Scaffold a full k6 test project from an organizational template",
		Long: `Scaffold a full k6 test project from an organizational template.

This command creates a complete project directory structure from a directory-based template,
including multiple files such as test scripts, configuration files, documentation, and CI setup.
It's designed for platform teams to provide standardized onboarding experiences.

Templates are searched in the following order:
1. ./templates/<name>/ (local to current working directory)
2. <user-config-dir>/.k6/templates/<name>/ (user-global templates)
3. Built-in templates (if applicable)

The --project-id, --team, and --env flags are used only for template rendering
and are available as template variables ({{ .ProjectID }}, {{ .Team }}, {{ .Env }}, {{ .Name }}).
These flags do not affect test execution - they only influence the generated project content.

This is the recommended onboarding path for teams adopting GCk6.`,
		Example: exampleText,
		Args:    cobra.NoArgs,
		RunE:    c.run,
	}

	initCmd.Flags().AddFlagSet(c.flagSet())

	// Mark org-template as required
	if err := initCmd.MarkFlagRequired("org-template"); err != nil {
		panic(fmt.Sprintf("Failed to mark org-template flag as required: %v", err))
	}

	return initCmd
}
