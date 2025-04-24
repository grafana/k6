// Package cmd is here to provide a way for xk6 to build a binary with added extensions
package cmd

import (
	"context"

	"go.k6.io/k6/cmd/state"
	internalcmd "go.k6.io/k6/internal/cmd"
)

// Execute executes the k6 command
// It only is exported here for backwards compatibility and the ability to use xk6 to build extended k6
func Execute() {
	gs := state.NewGlobalState(context.Background())

	if gs.Flags.BinaryProvisioning {
		internalcmd.NewLauncher(gs).Launch()
		return
	}

	// If Binary Provisioning is not enabled, continue with the regular k6 execution path

	// TODO: this is temporary defensive programming
	// The Launcher has already the support for this specific execution path, but we decided to play safe here.
	// After the v1.0 release, we want to fully delegate this control to the Launcher.
	gs.Logger.Debug("Binary Provisioning feature is disabled.")
	internalcmd.ExecuteWithGlobalState(gs)
}
