package cmd

import (
	"fmt"
	"strings"

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
//  2. Cobra attempts to match <subcommand> with registered subcommands:
//     a. If <subcommand> is registered (exists), Cobra executes it normally
//     b. If <subcommand> is NOT registered (missing), Cobra falls through to the "x" command's RunE
//  3. The RunE function (catch-all handler):
//     a. Checks if help is requested (no args, "help", or flag starting with "-")
//     b. If help requested, displays the "x" command help showing available subcommands
//     c. Otherwise, extracts the subcommand name from args[0]
//     d. Calls dependenciesFromSubcommand to construct dependencies with "subcommand:<name>" format
//     e. Returns binaryIsNotSatisfyingDependenciesError with the dependencies
//  4. Error handling in root.go:
//     a. handleUnsatisfiedDependencies catches the error
//     b. Triggers k6build provisioner to build a custom binary with the extension
//     c. Executes the custom binary with the original arguments (including --help if present)
//
// Note: DisableFlagParsing is enabled to pass all arguments (including --help) to the provisioned
// binary unchanged. This ensures that "k6 x httpbin --help" will show httpbin's help after provisioning.
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
		// Disable flag parsing to pass all arguments unchanged to the provisioned binary.
		// This ensures flags like --help reach the extension subcommand after provisioning.
		DisableFlagParsing: true,

		RunE: func(cmd *cobra.Command, args []string) error {
			// Show help if: no args, explicit "help", or starts with a flag
			if len(args) == 0 || args[0] == "help" || strings.HasPrefix(args[0], "-") {
				return cmd.Help()
			}

			// Subcommand not found - trigger provisioning
			deps, err := dependenciesFromSubcommand(gs, args[0])
			if err != nil {
				return err
			}

			return binaryIsNotSatisfyingDependenciesError{
				deps: deps,
			}
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

// dependenciesFromSubcommand constructs a dependencies object for the given subcommand,
// potentially using the manifest file specified in the global state.
//
// Without a manifest, it returns a dependencies object with a single entry for the subcommand
// with nil version (indicating any version is acceptable).
//
// Dependency name is in the format "subcommand:<subcommand name>" to distinguish from
// other types of dependencies. The builder service can use this information to determine
// which subcommand is being run and provision the appropriate binary.
func dependenciesFromSubcommand(gs *state.GlobalState, subcommand string) (dependencies, error) {
	deps := dependencies{
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
