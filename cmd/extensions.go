package cmd

import (
	"github.com/spf13/cobra"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/ext"
)

// Constructor returns an instance of a command extension module.
type Constructor func(*state.GlobalState) (*cobra.Command, error)

// RegisterExtension registers the given command extension constructor. This
// function panics if a module with the same name is already registered.
func RegisterExtension(name string, c Constructor) {
	ext.Register(name, ext.CommandExtension, c)
}
