// Package subcommand provides functionality for registering k6 subcommand extensions.
//
// This package allows external modules to register new subcommands that will be
// available in the k6 CLI. Subcommand extensions are registered during
// package initialization and are called when the corresponding subcommand is invoked.
package subcommand

import (
	"context"

	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/ext"
)

// Constructor is a function type that creates a new subcommand extension instance.
// It takes a GlobalState parameter and returns a Subcommand implementation.
type Constructor func(gs *state.GlobalState) Subcommand

// Subcommand is the interface that all subcommand extensions must implement.
type Subcommand interface {
	// Description returns a short description of the subcommand.
	Description() string
	// Help returns the help text for the subcommand.
	// The args parameter contains any additional arguments passed with the help flag,
	// which allows subcommands with nested commands to provide contextual help.
	Help(args []string) string
	// Run executes the subcommand with the given args and context.
	Run(ctx context.Context, args []string) error
}

// RegisterExtension registers a subcommand extension with the given name and constructor function.
//
// The name parameter specifies the subcommand name that users will invoke (e.g., "k6 <name>").
// The constructor function will be called when k6 initializes to create the subcommand.Subcommand
// instance for this subcommand.
//
// This function must be called during package initialization (typically in an init() function)
// and will panic if a subcommand with the same name is already registered.
func RegisterExtension(name string, c Constructor) {
	ext.Register(name, ext.SubcommandExtension, c)
}
