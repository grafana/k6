package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/ext"
	"go.k6.io/k6/subcommand"
)

// getX creates the "x" command that serves as a namespace for extension-provided subcommands.
//
// Provisioning Workflow:
//
// The "x" command implements automatic binary provisioning for extension subcommands. When a user
// runs a subcommand that isn't available in the current k6 binary, the system automatically builds
// a custom k6 binary with the required extension.
//
// Execution Flow:
//
//  1. User runs: k6 x <subcommand> [args...]
//  2. PersistentPreRunE checks if the subcommand needs provisioning by calling needsProvisioningSubcommand
//  3. If provisioning is needed:
//     a. dependenciesFromSubcommand constructs a dependencies object with "subcommand:<name>" format
//     b. A binaryIsNotSatisfyingDependenciesError is returned with the dependencies
//     c. The error is caught by handleUnsatisfiedDependencies in root.go
//     d. A custom k6 binary with the extension is built via k6build provisioner
//     e. The custom binary is executed with the original arguments
//  4. If provisioning is not needed (subcommand already exists):
//     a. The registered extension subcommand is executed normally
//
// The "x" command itself (without subcommand) displays help showing all available extension subcommands.
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
			needs, subcommand, err := needsProvisioningSubcommand(cmd, args)
			if err != nil {
				return err
			}

			if !needs {
				return nil
			}

			deps, err := dependenciesFromSubcommand(gs, subcommand)
			if err != nil {
				return err
			}

			return binaryIsNotSatisfyingDependenciesError{
				deps: deps,
			}
		},

		RunE: func(cmd *cobra.Command, _ []string) error {
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

// needsProvisioningSubcommand checks if the given subcommand needs provisioning and
// returns the name of the subcommand to provision if needed.
func needsProvisioningSubcommand(cmd *cobra.Command, args []string) (bool, string, error) {
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
		if len(args) == 0 || args[0] == "help" {
			return false, "", nil
		}

		// provision args[0] required
		return true, args[0], nil
	}

	// nothing to do, already provisioned subcommand is being run
	return false, extCmd.Name(), nil
}

// dependenciesFromSubcommand constructs a dependencies object for the given subcommand,
// potentially using the manifest file specified in the global state.
//
// Dependency name is in the format "subcommand:<subcommand name>" to distinguish from
// other types of dependencies. The builder service can use this information to determine
// which subcommand is being run and provision the appropriate binary.
func dependenciesFromSubcommand(gs *state.GlobalState, subcommand string) (dependencies, error) {
	deps := dependencies{
		// Any version of the subcommand is fine,
		// the error is used as a signal for provisioning, not for unsatisfied dependencies.
		"subcommand:" + subcommand: nil,
	}

	m, err := parseManifest(gs.Flags.DependenciesManifest)
	if err != nil {
		return nil, err
	}

	err = deps.applyManifest(m)
	if err != nil {
		return nil, err
	}

	return deps, nil
}
