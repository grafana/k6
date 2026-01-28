// Package subcommand provides functionality for registering k6 subcommand extensions.
//
// This package allows external modules to register new subcommands that will be
// available in the k6 CLI. Subcommand extensions are registered during
// package initialization and are called when the corresponding subcommand is invoked.
package subcommand

import (
	"github.com/spf13/cobra"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/ext"
)

// Constructor is a function type that creates a new cobra.Command for a subcommand extension.
// It receives a GlobalState instance that provides access to configuration, logging,
// file system, and other shared k6 runtime state. The returned Command will be
// integrated into k6's CLI as a subcommand.
//
// WARNING: The GlobalState parameter is read-only and must not be modified or altered
// in any way. Modifying the GlobalState can make k6 core unstable and lead to
// unpredictable behavior.
type Constructor func(*state.GlobalState) *cobra.Command

// RegisterExtension registers a subcommand extension with the given name and constructor function.
//
// The name parameter specifies the subcommand name that users will invoke (e.g., "k6 <name>").
// The constructor function will be called when k6 initializes to create the cobra.Command
// instance for this subcommand.
//
// This function must be called during package initialization (typically in an init() function)
// and will panic if a subcommand with the same name is already registered.
//
// The name parameter and the returned Command's Name() must match.
func RegisterExtension(name string, c Constructor) {
	ext.Register(name, ext.SubcommandExtension, c)
}
