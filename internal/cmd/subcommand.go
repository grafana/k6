package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/ext"
	"go.k6.io/k6/subcommand"
)

func getX(gs *state.GlobalState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "x",
		Short: "Extension subcommands",
		Long: `Namespace for extension-provided subcommands.

This command serves as a parent for subcommands registered by k6 extensions,
allowing them to extend k6's functionality with custom commands.
`,
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: true,
		},

		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			needs, subcommand, err := needsProvisioning(cmd, args)
			if err != nil {
				return err
			}

			if needs {
				gs.Logger.WithField("subcommand", subcommand).Info("provisioning")
			}

			return nil
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(extensionSubcommands(gs)...)

	return cmd
}

// extensionSubcommands retrieves all subcommands provided by extensions.
func extensionSubcommands(gs *state.GlobalState) []*cobra.Command {
	commands := make([]*cobra.Command, 0)

	for _, extension := range ext.Get(ext.SubcommandExtension) {
		commands = append(commands, getCmdForExtension(extension, gs))
	}

	return commands
}

// getCmdForExtension gets a *cobra.Command for the given subcommand extension.
func getCmdForExtension(extension *ext.Extension, gs *state.GlobalState) *cobra.Command {
	ctor, ok := extension.Module.(subcommand.Constructor)
	if !ok {
		panic(fmt.Sprintf("invalid subcommand constructor: name: %s path: %s", extension.Name, extension.Path))
	}

	cmd := ctor(gs)

	// Validate that the command's name matches the extension name.
	if cmd.Name() != extension.Name {
		panic(fmt.Sprintf("subcommand name mismatch: command name: %s extension name: %s", cmd.Name(), extension.Name))
	}

	return cmd
}

func needsProvisioning(cmd *cobra.Command, args []string) (bool, string, error) {
	var (
		xCmd   *cobra.Command
		extCmd *cobra.Command
	)

	for c := cmd; c != nil; c = c.Parent() {
		if c.Name() == "x" {
			xCmd = c
			break
		}

		extCmd = c
	}

	// should not happen, only called from 'x' command PersistentPreRunE
	if xCmd == nil {
		return false, "", errors.New("'x' command not found in parent chain")
	}

	if cmd == xCmd {
		// x command itself is being run
		if len(args) == 0 {
			return false, "", nil
		}

		// provision args[0] required
		return true, args[0], nil
	}

	// nothing to do, already provisioned subcommand is being run
	return false, extCmd.Name(), nil
}
