package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/internal/cmd/templates"
	"go.k6.io/k6/lib/fsext"
)

func getCmdTemplate(gs *state.GlobalState) *cobra.Command {
	templateCmd := &cobra.Command{
		Use:   "template",
		Short: "Manage k6 script templates",
		Long: `Manage k6 script templates.

This command provides functionality to manage script templates for the k6 new command.`,
	}

	templateCmd.AddCommand(getCmdTemplateAdd(gs))
	return templateCmd
}

func getCmdTemplateAdd(gs *state.GlobalState) *cobra.Command {
	exampleText := getExampleText(gs, `
  # Promote an existing script to a reusable template
  $ {{.}} template add my-api-test ./my-api-test.js

  # Add a template from another location
  $ {{.}} template add custom-template /path/to/script.js`[1:])

	addCmd := &cobra.Command{
		Use:   "add <name> <path>",
		Short: "Promote a user script to a reusable template",
		Long: `Promote a user script to a reusable template.

This copies the specified script file to ~/.k6/templates/<name>/script.js, 
making it available as a template for the k6 new command.

The template name should not contain path separators and will be used to 
reference the template in future k6 new commands.`,
		Example: exampleText,
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			templateName := args[0]
			scriptPath := args[1]

			// Check if the script file exists
			exists, err := fsext.Exists(gs.FS, scriptPath)
			if err != nil {
				return fmt.Errorf("error checking script file: %w", err)
			}
			if !exists {
				return fmt.Errorf("script file %s does not exist", scriptPath)
			}

			// Initialize template manager
			tm, err := templates.NewTemplateManager(gs.FS, gs.UserOSConfigDir)
			if err != nil {
				return fmt.Errorf("error initializing template manager: %w", err)
			}

			// Create the template
			if err := tm.CreateUserTemplate(templateName, scriptPath); err != nil {
				return fmt.Errorf("error creating template: %w", err)
			}

			fmt.Fprintf(gs.Stdout, "Template '%s' created successfully.\n", templateName)
			fmt.Fprintf(gs.Stdout, "You can now use it with: k6 new --template %s\n", templateName)

			return nil
		},
	}

	return addCmd
}
