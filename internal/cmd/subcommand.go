package cmd

import (
	"iter"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/ext"
	"go.k6.io/k6/subcommand"
)

// extensionSubcommands returns an iterator over all registered subcommand extensions
// that are not already defined in the given slice of commands.
func extensionSubcommands(gs *state.GlobalState, defined []*cobra.Command) iter.Seq[*cobra.Command] {
	already := make(map[string]struct{}, len(defined))
	for _, cmd := range defined {
		already[cmd.Name()] = struct{}{}
	}

	return func(yield func(*cobra.Command) bool) {
		for _, extension := range ext.Get(ext.SubcommandExtension) {
			if _, exists := already[extension.Name]; exists {
				gs.Logger.WithFields(logrus.Fields{"name": extension.Name, "path": extension.Path}).
					Warnf("subcommand already exists")
				continue
			}

			already[extension.Name] = struct{}{}

			if !yield(getCmdForExtension(extension, gs)) {
				break
			}
		}
	}
}

// getCmdForExtension gets a *cobra.Command for the given subcommand extension.
func getCmdForExtension(extension *ext.Extension, gs *state.GlobalState) *cobra.Command {
	ctor, ok := extension.Module.(subcommand.Constructor)
	if !ok {
		gs.Logger.WithFields(logrus.Fields{"name": extension.Name, "path": extension.Path}).
			Fatalf("subcommand's constructor does not implement the subcommand.Constructor")
	}

	cmd := ctor(gs)

	// Validate that the command's name matches the extension name.
	if cmd.Name() != extension.Name {
		gs.Logger.WithFields(logrus.Fields{"name": extension.Name, "path": extension.Path}).
			Fatalf("subcommand's command name (%s) does not match the extension name (%s)", cmd.Name(), extension.Name)
	}

	return cmd
}
