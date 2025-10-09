package cmd

import (
	"iter"
	"strings"

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

	return wrapSubcommand(extension.Name, ctor(gs), gs)
}

// wrapSubcommand wraps a subcommand.Subcommand into a *cobra.Command.
func wrapSubcommand(name string, sc subcommand.Subcommand, gs *state.GlobalState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name,
		Short: sc.Description(),
		RunE: func(cmd *cobra.Command, args []string) error {
			return sc.Run(cmd.Context(), args)
		},
	}

	// Use a custom usage template that only shows the global flags section
	// when there are any global flags defined.
	cmd.SetUsageTemplate(subcommandUsageTemplate)

	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		var buff strings.Builder

		// remove the subcommand name from args
		if len(args) > 0 && args[0] == name {
			args = args[1:]
		}

		buff.WriteString(strings.TrimSpace(sc.Help(args)))
		buff.WriteString("\n\n")

		// display the usage of global flags
		buff.WriteString(cmd.UsageString())

		_, err := cmd.OutOrStdout().Write([]byte(buff.String()))
		if err != nil {
			gs.Logger.WithError(err).Error("failed to write help output")
		}
	})

	cmd.FParseErrWhitelist.UnknownFlags = true

	return cmd
}

// subcommandUsageTemplate is a cobra usage template that only shows the global flags section
// when there are any global flags defined.
const subcommandUsageTemplate = `{{if .HasAvailableInheritedFlags}}Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}
`
