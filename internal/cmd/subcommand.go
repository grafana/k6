package cmd

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"go.k6.io/k6/v2/cmd/state"
	"go.k6.io/k6/v2/ext"
	"go.k6.io/k6/v2/subcommand"
)

// xStubAnnotation marks a cobra subcommand under `x` as a registry-sourced
// stub: it exists only to advertise an extension subcommand in `k6 x` help.
// Running it falls into the same provisioning path as an unregistered name.
const xStubAnnotation = "k6-x-stub"

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

Run "k6 x explore" to see the full list of official and community-provided subcommands.
`,
		// Disable flag parsing to pass all arguments unchanged to the provisioned binary.
		// This ensures flags like --help reach the extension subcommand after provisioning.
		DisableFlagParsing: true,

		RunE: func(cmd *cobra.Command, args []string) error {
			// Show help if: no args, explicit "help", or starts with a flag
			if len(args) == 0 || args[0] == "help" || strings.HasPrefix(args[0], "-") {
				return cmd.Help()
			}

			if !gs.Flags.AutoExtensionResolution {
				return cobra.NoArgs(cmd, args)
			}

			// Subcommand not found - trigger provisioning
			return buildExtensionDeps(gs, args[0])
		},
	}

	baked := extensionSubcommands(gs)
	cmd.AddCommand(baked...)
	cmd.AddCommand(registryStubs(gs, baked)...)

	return cmd
}

// registryStubs returns cobra stubs for registry-advertised subcommands not
// already provided by baked-in extensions. The stubs only exist so `k6 x`
// help can advertise the wider catalog; invoking one drops into the regular
// provisioning path. The function is a no-op outside `k6 x` invocations so
// other commands don't pay for the registry I/O.
func registryStubs(gs *state.GlobalState, baked []*cobra.Command) []*cobra.Command {
	if !gs.Flags.AutoExtensionResolution {
		return nil
	}

	args := gs.CmdArgs[1:]
	first := slices.IndexFunc(args, func(a string) bool {
		return a != cobra.ShellCompRequestCmd && a != cobra.ShellCompNoDescRequestCmd
	})
	if first < 0 || args[first] != "x" {
		return nil
	}

	subs, _ := readCachedCatalog(gs, catalogCachePath(gs))

	var stubs []*cobra.Command
	for _, r := range subs {
		if slices.ContainsFunc(baked, func(b *cobra.Command) bool { return b.Name() == r.Name }) {
			continue
		}
		stubs = append(stubs, &cobra.Command{
			Use:                r.Name,
			Short:              r.Short,
			DisableFlagParsing: true,
			Annotations:        map[string]string{xStubAnnotation: "true"},
			RunE: func(c *cobra.Command, _ []string) error {
				return buildExtensionDeps(gs, c.Name())
			},
		})
	}
	return stubs
}

// buildExtensionDeps returns a [binaryIsNotSatisfyingDependenciesError] for
// the given extension name if the required dependencies are not satisfied.
// It's used by both [getX] and extension completions check in root.go to
// trigger provisioning via [handleUnsatisfiedDependencies].
func buildExtensionDeps(gs *state.GlobalState, extName string) error {
	deps, err := dependenciesFromSubcommand(gs, extName)
	if err != nil {
		return err
	}
	// Will kickoff the provisioning flow in [handleUnsatisfiedDependencies].
	return binaryIsNotSatisfyingDependenciesError{deps: deps}
}

// extensionSubcommands retrieves all subcommands provided by extensions.
func extensionSubcommands(gs *state.GlobalState) []*cobra.Command {
	extensions := ext.Get(ext.SubcommandExtension)
	commands := make([]*cobra.Command, 0, len(extensions))

	for _, extension := range extensions {
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

// detectExtensionCompletion checks whether CmdArgs represent a completion
// request for an unregistered extension subcommand (e.g. k6 __complete x docs "").
// Returns true and the extension name when provisioning is needed.
func detectExtensionCompletion(root *cobra.Command, gs *state.GlobalState) (string, bool) {
	if !gs.Flags.AutoExtensionResolution {
		return "", false
	}

	args := gs.CmdArgs[1:]
	if len(args) == 0 || (args[0] != cobra.ShellCompRequestCmd && args[0] != cobra.ShellCompNoDescRequestCmd) {
		return "", false
	}

	cmd, remaining, err := root.Find(args[1:])
	if err != nil {
		return "", false
	}
	switch {
	case cmd.Annotations[xStubAnnotation] == "true" && len(remaining) >= 1:
		return cmd.Name(), true
	case cmd.Name() == "x" && len(remaining) >= 2:
		return remaining[0], true
	}
	return "", false
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
