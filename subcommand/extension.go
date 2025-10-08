// Package subcommand is a package to provide subcommand extension registration.
package subcommand

import (
	"github.com/spf13/cobra"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/ext"
)

// Constructor returns an instance of a cobra Command.
type Constructor func(*state.GlobalState) *cobra.Command

// RegisterExtension registers the given subcommand extension constructor.
// This function panics if a module with the same name is already registered.
func RegisterExtension(name string, c Constructor) {
	ext.Register(name, ext.SubcommandExtension, c)
}
