package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.k6.io/k6/cmd/state"
)

func getCmdInit(gs *state.GlobalState) *cobra.Command {
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
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("init called")
		},
	}
}
